# Feedoku 2.0 — Redesign Plan

AI-native RSS reader: subscribe to feeds, and let AI summarize, rank, dedupe and brief you — then ask questions across everything you follow.

---

## 1. Where the project is today

| Area | State |
|---|---|
| **Go backend** | Does not compile. `cmd/main.go` is missing (Makefile, Dockerfile and `.air.toml` all point at it). `internal/auth` is imported but doesn't exist. `sql/queries/users.sql` targets a `users` table with `password_hash`/`api_key` columns that the schema doesn't have, and handlers call `GetPostsForUser` / `models.ConvertDatabase*` that were never written. |
| **Database** | One migration: better-auth tables (`user`, `session`, `account`, `verification`). No feeds, posts or subscriptions tables. |
| **Feed fetching** | Only an `RSSFeed` XML struct and a date parser. No scraper, no Atom support. |
| **Kafka** | Mentioned in README, never implemented (just a start script). |
| **Redis** | Client init only, unused. |
| **Frontend** | Next 15 + React 19 + Tailwind v4 + shadcn. Landing page, sign-in/sign-up with better-auth (email/password). No app shell, no reader, middleware is a no-op. Sign-in copy still says "UI component framework". |

**Conclusion:** the only reusable parts are the frontend scaffold (Next/Tailwind/shadcn/better-auth) and the auth schema. The backend is effectively a rewrite.

---

## 2. Target architecture

```
                ┌──────────────── Next.js (www/) ────────────────┐
 Browser ──────►│ App Router UI · better-auth (sessions) · BFF   │
                └──────────────┬─────────────────────────────────┘
                               │ HTTP (session token forwarded)
                ┌──────────────▼──────────── Go (cmd/api) ───────┐
                │ REST API · auth via better-auth session table  │
                │ SSE stream for chat + digest generation        │
                └──────────────┬─────────────────────────────────┘
                               │
   ┌───────────────────────────▼──────────────────────┐
   │ Postgres 16 + pgvector   (single source of truth, │
   │ also the job queue via FOR UPDATE SKIP LOCKED)    │
   └───────────────────────────▲──────────────────────┘
                               │
                ┌──────────────┴──────────── Go (cmd/worker) ────┐
                │ Feed poller → parse RSS/Atom → readability     │
                │ extract → AI enrichment jobs (summary, tags,   │
                │ embedding, clustering) → daily digests         │
                └──────────────┬─────────────────────────────────┘
                               │
        LLM gateway (Groq → OpenRouter fallback, OpenAI-compatible) · Voyage embeddings
```

Key decisions:
- **Drop Kafka.** One Postgres job table with `SKIP LOCKED` handles this scale with zero extra infra. Can be swapped later.
- **Redis optional** — rate limiting + caching hot feed lists; not required to run.
- **better-auth stays the auth owner.** Go validates requests by looking up the session token in the shared `session` table (no separate JWT system).
- **pgvector** for semantic search / RAG / "for you" ranking — keeps everything in one DB.
- **docker-compose** for Postgres (pgvector) so `make db` works on a fresh machine. Redis is left out until something needs it.

---

## 3. AI features

| # | Feature | What the user sees | How |
|---|---|---|---|
| A1 | **Article summaries** | TL;DR + 3 key points on every article card; "Explain like I'm new to this" toggle | On ingest, worker calls the LLM with extracted article text; JSON output `{tldr, key_points[], topics[], reading_time, sentiment}` validated in Go. Backfills are throttled through the rate limiter. |
| A2 | **Auto-tagging & topics** | Articles tagged (AI, Startups, Security…); smart folders built from tags | Same enrichment call as A1 returns `topics[]` from a fixed taxonomy + free-form entities. |
| A3 | **Story clustering / dedupe** | "Covered by 5 sources" — one card per story, expandable | Embedding similarity (cosine > threshold within 48h window) → `story_clusters`. |
| A4 | **Daily AI Briefing** | A morning digest page: top stories across *your* feeds, grouped by theme, with citations back to articles | Scheduled per-user job; the LLM writes digest from that day's cluster summaries. Optional email later. |
| A5 | **Ask your feeds (RAG chat)** | Chat panel: "What did people say about the new GPU launches this week?" → streamed answer with cited articles | Embed query → pgvector top-k (filtered to user's subscriptions + date range) → LLM with numbered source snippets, told to cite `[n]`; citations mapped back to articles → SSE stream. |
| A6 | **Semantic search** | Search box that understands meaning, not just keywords | Hybrid: Postgres full-text (`tsvector`) + vector similarity, merged with RRF. |
| A7 | **"For You" ranking** | Default feed sorted by relevance to you, not just recency | User interest vector = weighted mean of embeddings of articles read/saved/upvoted; score = similarity × recency decay. No LLM call needed. |
| A8 | **Smart onboarding / feed discovery** | Paste any site URL → we find its feed. Or type interests ("rust, climate tech") → suggested feeds | Feed autodiscovery (`<link rel=alternate>`) in Go; LLM suggests sources from a curated catalog by interest. |

MVP AI scope = **A1, A2, A5, A6**. Then A4, A3, A7, A8.

### Model & provider decisions (free-tier first)

All LLM calls go through one small Go `llm` package that speaks the **OpenAI-compatible chat API**, so the provider is just `base_url + api_key + model` in env. Any of Groq, OpenRouter or Vercel AI Gateway can be swapped in without code changes.

| Use | Provider | Model (env-configurable) | Free-tier budget* |
|---|---|---|---|
| Enrichment (summary + tags), every article | **Groq** | `openai/gpt-oss-20b` | ~1,000–14,400 req/day, 30 RPM |
| Briefing + Ask chat | **Groq** | `openai/gpt-oss-120b` | ~1,000 req/day, 30 RPM |
| Fallback when Groq 429s | **OpenRouter** | a `:free` model | 20 RPM; 50 req/day (1,000/day after a one-time $10 credit purchase) |
| Embeddings (search, RAG, dedupe, For You) | **Voyage AI** | `voyage-4-lite` @ 1024 dims | large one-time free token grant (reported 200M) |

\* Free-tier numbers vary between sources and change often. Check the providers' own pages. The design doesn't depend on exact numbers.

Why: Groq has the most generous free daily limits and is very fast (good for streamed chat). OpenRouter's free tier is too small to be primary but works as a fallback. Vercel AI Gateway is mainly a router over paid providers, so it adds nothing on a $0 budget. It can still be plugged in later through the same interface. Voyage's free grant covers millions of articles; Gemini embeddings (free, rate-limited) is the backup option.

How we stay inside free limits:
- **Worker-side rate limiter** per provider (token bucket ≈ 25 RPM) plus a fallback chain Groq → OpenRouter → mark job `deferred`, retried later. Nothing breaks when quota runs out; summaries just arrive later.
- **Enrich once per article, not per user.** AI output lives on the shared `article_ai` row, so 100 users on the same feed still cost one call.
- **Truncate input** to ~3k tokens of extracted text; ask for **JSON output** and validate it in Go; retry once on bad JSON.
- **Batch embeddings** (up to 128 texts per Voyage call).
- **The briefing reuses stored summaries**, so it costs one LLM call per user per day.
- **For You + dedupe use only vectors**, with no LLM calls.
- Per-user daily quota for chat (e.g. 30 questions/day) stored in `ai_usage`.
- Embedding dimension is fixed at 1024 in the schema. Switching embedding provider means re-embedding (a script will do this).

---

## 4. Data model (new migrations)

```
feeds            id, url (unique), site_url, title, description, favicon,
                 etag, last_modified, last_fetched_at, next_fetch_at, error_count
subscriptions    user_id, feed_id, folder, custom_title, created_at
articles         id, feed_id, guid (unique per feed), url, title, author,
                 published_at, content_html, content_text, image_url,
                 search_tsv (generated tsvector)
article_ai       article_id, tldr, key_points jsonb, topics text[], entities jsonb,
                 sentiment, reading_minutes, model, embedding vector(N), cluster_id,
                 status (pending|done|failed), updated_at
user_article     user_id, article_id, read_at, saved, rating (-1/0/1)
story_clusters   id, centroid vector(N), title, first_seen, last_seen
digests          id, user_id, date, content jsonb (sections + citations), model
chat_threads / chat_messages   for A5 history
jobs             id, kind, payload jsonb, run_at, attempts, locked_at, last_error
ai_usage         user_id, day, input_tokens, output_tokens
```

Keep better-auth tables as-is. Use goose for migrations + sqlc for queries (already set up).

---

## 5. API surface (Go)

```
GET    /v1/me
GET    /v1/feeds                      POST /v1/feeds {url}  (autodiscovers)
DELETE /v1/feeds/:id                  POST /v1/feeds/opml   (import)
GET    /v1/articles?feed=&topic=&sort=for_you|latest&cursor=
GET    /v1/articles/:id               POST /v1/articles/:id/state {read,saved,rating}
GET    /v1/search?q=                  (hybrid)
GET    /v1/briefing/today             POST /v1/briefing/regenerate
POST   /v1/chat (SSE)                 GET  /v1/chat/threads
GET    /v1/discover?interests=
```

---

## 6. Frontend redesign (www/)

Keep: Next 16, Tailwind v4, shadcn, better-auth. Tooling: oxlint + oxfmt. Font: Sora (UI), Geist Mono (code). Add: TanStack Query, `nuqs` for URL state, `cmdk` command palette.

Screens:
1. **Landing** — rewrite around the AI angle (briefing preview, chat demo), fix copy.
2. **Auth** — sign-in/up + Google/GitHub OAuth; real `middleware.ts` protecting `/app/*`.
3. **Onboarding** — pick interests → suggested feeds → paste URLs / import OPML.
4. **App shell** (3-pane reader): sidebar (Briefing, For You, Latest, Saved, Topics, Feeds/folders) · article list (cards with AI TL;DR, topic chips, "5 sources" badge) · reader pane (clean article, AI key points, related stories).
5. **Briefing** — editorial layout of today's digest with inline citations.
6. **Ask** — chat panel (slide-over from anywhere, ⌘J) with streamed answers + source cards.
7. **Search** — ⌘K palette with semantic results.
8. **Settings** — feeds management, AI preferences (summary length, briefing time), usage.

Design direction: calm, reading-first (serif article body, generous whitespace), dark/light, keyboard-first (j/k navigation, s to save, o to open).

---

## 7. Phased roadmap

| Phase | Deliverable | "Done" means |
|---|---|---|
| **0. Foundations** ✅ | docker-compose (pg+pgvector), env cleanup, delete dead code, `cmd/api` + `cmd/worker` that compile, chi router, session-based auth middleware, CI (go build/vet/test + next build/lint), upgrade all deps (Next 16, React 19.3, better-auth 1.7, Zod 4, TS 7), oxlint + oxfmt, Sora font | `make db migrate-up` + `make api` + `make web` run the stack; `/v1/me` works with a better-auth session (verified end to end) |
| **1. Core reader** ✅ | Feed add/autodiscover, RSS+Atom parse (gofeed), conditional GET, readability extraction, poll scheduler, articles API, app shell + reader UI, read/save state, OPML import | Add a feed in the UI → articles appear within a minute and are readable |
| **2. AI enrichment** ✅ | Job queue, LLM enrichment (Groq + fallback) (A1+A2), embeddings, usage tracking, UI for summaries/topics | New articles show TL;DR + topics automatically |
| **3. Search & Ask** | Hybrid search (A6), RAG chat with SSE + citations (A5) | Ask a question, get a streamed cited answer |
| **4. Briefing & ranking** | Clustering (A3), daily briefing (A4), For You (A7), onboarding (A8) | Morning briefing generated per user; For You is default view |
| **5. Polish & ship** | Landing rewrite, keyboard shortcuts, empty/error states, rate limits, tests, Dockerfiles, deploy guide | Deployable with one compose file |

Each phase ends in a working, demoable app.

---

## 8. Decisions log

| Decision | Choice |
|---|---|
| Backend | Go: `cmd/api` + `cmd/worker` (rewrite); Next.js for UI + better-auth |
| LLM | Groq free tier (gpt-oss-20b bulk, gpt-oss-120b chat/briefing), OpenRouter `:free` fallback, behind an OpenAI-compatible interface |
| Embeddings | Voyage AI `voyage-4-lite` (free grant), Gemini as backup |
| MVP AI scope | All four: summaries + tags, Ask (RAG chat + semantic search), daily briefing, For You + dedupe |
| Queue | Postgres `SKIP LOCKED` (Kafka dropped) |

| Visual design | Palette from you.brainfogagency.com (warm paper / ink canvas, indigo primary, lavender accent, hairline translucent borders, glass panels), mapped onto shadcn tokens in `globals.css`; Sora UI font; serif reading pane |
| Frontend tooling | bun, oxlint + oxfmt, TypeScript 7 `tsc` (with TS 6 API aliased for tools), Sora font |

Still open (sensible defaults assumed): deploy via **docker-compose** (self-host) first.
