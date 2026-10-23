# Lookfar: plan

> **Lookfar** (formerly Feedoku): agents that look far, so you don't have to.
> The previous AI RSS reader plan is in git history (`git show 4f5e775:PLAN.md`).

**One line:** agents you can leave running. You describe what to watch in plain language. Agents check pages, feeds and repos on a schedule, tell you only what matters, ask before doing anything with side effects, and keep a memory your other AI tools can use through MCP.

---

## 1. Why pivot

AI RSS readers are a solved, crowded category: summaries, "chat with your feeds" and daily digests ship in Feedly, Inoreader, Readwise and others. What people still lack in 2026 is **background AI they can trust**: agents that run unattended, cost little, can't do damage on their own, and show their work.

The product combines three pillars:

| Pillar | What it is | Role |
|---|---|---|
| **A. Watcher agents** | Scheduled agents with a mission, sources, tools and a budget. They detect change, judge relevance and write reports. | The core product, MVP lead |
| **B. Context hub + MCP** | Everything agents read and write is stored, embedded and searchable, and exposed to Claude, Cursor, ChatGPT etc. through an MCP server. | Memory for agents, the reason to keep data here |
| **C. Approvals inbox** | One inbox for reports, alerts, agent questions and action approvals, from our agents and from external agents (via MCP or webhook). | The trust layer: nothing outward-facing happens without a yes |

### Example agents (templates shipped in-app)

- **Competitor pricing:** "Check these 4 pricing pages every Monday. Report only real price, plan or limit changes, not copy edits."
- **Dependency radar:** "Watch Go, Postgres and pgx releases. Tell me only about breaking changes or security fixes that affect a Go + Postgres 17 app."
- **Opportunity scout:** "Watch these grant and job boards and these RSS feeds. Alert me about remote Go roles over $X or grants for open-source infra."
- **Story tracker:** "Follow this news topic across my feeds. Send a short 'what changed since last time' each evening."

### Product principles

1. **No change, no tokens.** Deterministic change detection runs first and costs nothing. A small model triages relevance. The main model only runs when something likely matters.
2. **Quiet by default.** Agents report only above a significance threshold. One useful alert beats twenty digests.
3. **Read freely, act with permission.** In v1 every side-effecting tool goes through the approval inbox, without exception.
4. **Show the work.** Every run has a step-by-step timeline: what was fetched, what changed, what the model saw, which tools ran, and what it cost.
5. **Own your stack.** Self-host with one `docker compose up`. Bring your own LLM keys, with a free-tier default so it works on day one.

---

## 2. Decisions so far

| Topic | Decision |
|---|---|
| Hosting | **Self-host first**: single compose file, one user or a small team (invite-only). Code stays multi-user safe so a hosted version is possible later. |
| LLM access | **BYOK + free default**: users add Anthropic / OpenAI / OpenRouter / any OpenAI-compatible keys. The existing Groq → OpenRouter free chain is the zero-config fallback. |
| MVP lead | **Watcher agents**, with the run log and approvals inbox built in from the start. |
| Agent autonomy (v1) | **Read + notify; actions gated.** Fetch, diff, search and report run freely. Email, webhooks and other outward actions wait for approval. |
| Notifications (v1) | **In-app inbox + web push (PWA) + email (SMTP).** |
| Frontend | **SvelteKit 2 + Svelte 5 (runes)**, Tailwind v4, **shadcn-svelte**, bun. |
| SvelteKit role | **UI + auth + BFF**: better-auth runs in SvelteKit, and `/api/v1/*` is proxied to Go. Go owns all business logic. |
| Backend | Keep **Go** (`cmd/api` + `cmd/worker`), Postgres 17 + pgvector, goose, sqlc, Postgres `SKIP LOCKED` job queue. |
| Name | **Lookfar.** Heed, Perch, Vigil, Hark and Outpost were rejected because each is already an AI-agent or dev-infra product. A search in Oct 2026 found no software named Lookfar (domains and GitHub org not yet registered). |

---

## 3. Architecture

```
                  ┌──────────────── SvelteKit (web/) ──────────────────┐
 Browser / PWA ──►│ UI · better-auth (sessions) · /api/v1 proxy · SW   │
                  └───────────────┬────────────────────────────────────┘
                                  │ HTTP + SSE (session cookie forwarded)
 Claude / Cursor /                │
 other agents ── MCP (token) ──┐  │
 webhooks ──── REST (token) ───┤  │
                  ┌────────────▼──▼──────────── Go (cmd/api) ───────────┐
                  │ REST API · MCP server (/mcp) · SSE run streams       │
                  │ auth: better-auth session cookie OR API token        │
                  └───────────────┬──────────────────────────────────────┘
                                  │
          ┌───────────────────────▼────────────────────────────┐
          │ Postgres 17 + pgvector                              │
          │ sources · snapshots · agents · runs · inbox ·       │
          │ documents/chunks (memory) · jobs (SKIP LOCKED)      │
          └───────────────────────▲────────────────────────────┘
                                  │
                  ┌───────────────┴──────────── Go (cmd/worker) ────────┐
                  │ Scheduler → source checks → change detection        │
                  │ → triage (small model) → agent run (tool loop)      │
                  │ → report / inbox item → notify (push, email)        │
                  │ embedding + memory indexing jobs                    │
                  └───────────────┬─────────────────────────────────────┘
                                  │
           LLM adapters: OpenAI-compatible (Groq, OpenRouter, OpenAI, local) · Anthropic native
           Embeddings: Voyage (default) · OpenAI-compatible
```

### What we reuse from the current codebase

| Existing package | Fate |
|---|---|
| `internal/auth` | **Keep as is.** It verifies the better-auth cookie and doesn't care whether Next or SvelteKit set it. Add API-token auth next to it. |
| `internal/jobs` | **Keep.** It becomes the backbone for source checks, agent runs, resumes and notifications. |
| `internal/llm` | **Extend.** Add tool calling, an Anthropic native adapter, per-user credentials, cost accounting. The fallback chain and rate limiter stay. |
| `internal/llm/voyage.go`, `internal/enrich/embed.go` | **Keep** for memory embeddings. |
| `internal/feed` (fetch, parse, discover, extract) | **Keep, move** into `internal/source` as the RSS/Atom and page fetchers. The SSRF guard and conditional GET are reused for every fetch. |
| `internal/ingest`, `internal/enrich` (summaries) | **Replace.** Per-article enrichment is replaced by change detection + triage + agent runs. |
| `internal/httpapi/reader.go` | **Replace** with agents / runs / inbox / sources handlers. |
| `internal/config`, `logging`, `postgres` | **Keep**, add new settings. |
| `www/` (Next.js) | **Replace** with `web/` (SvelteKit). Carry over design tokens and `STYLE_GUIDE.md`. |
| `sql/schema/003_reader.sql`, `004_ai.sql` | **Replace** with a fresh migration series (nothing is deployed). `001_better_auth.sql` and `002_extensions.sql` stay. |

---

## 4. Core concepts

### Source
A thing to watch, owned by a user and shared across that user's agents.

| Kind | How it's checked | What a "change" is |
|---|---|---|
| `rss` | gofeed, conditional GET | new or updated entries |
| `page` | HTTP GET → readability or CSS selector → normalized text | text diff after normalization (whitespace, dates, tracking params stripped) |
| `github` | GitHub REST (releases, tags, optional issues/PRs by label); optional token | new releases or items |
| `json` | HTTP GET + JSONPath | changed values |

Each check stores a **snapshot** (content hash + normalized text or items). If the hash matches the last snapshot, the check stops and no LLM runs. Otherwise a structured **diff** is stored and the agents attached to that source are triggered.

Page sources include **noise filters**: ignore selectors, "ignore numbers that look like timestamps", and a minimum diff size, so cookie banners and "updated 3 min ago" don't wake agents.

### Agent
| Field | Meaning |
|---|---|
| `mission` | Natural-language instructions: what to watch for and what counts as important |
| `sources` | Attached sources |
| `trigger` | `on_change` (default), `cron` (e.g. Monday 9:00), or both |
| `tools` | Allowed tool set (see §5) |
| `allowed_domains` | Extra domains `fetch_url` may reach beyond its sources |
| `models` | Triage model profile + main model profile |
| `budget` | Max cost per run and per month; the run stops cleanly when exceeded |
| `notify` | Channels + minimum significance (`low` / `medium` / `high`) |
| `memory` | Notes the agent keeps between runs (e.g. "Pro plan was $49 on 2026-10-01") |

### Run
One execution of an agent: trigger → triage → (maybe) main loop → output. States:

```
queued → triaging → skipped (not relevant, cheap exit)
                  → running ⇄ waiting_user (question / approval in inbox)
                            → succeeded | failed | budget_exceeded | cancelled
```

Every step (LLM call, tool call, tool result, approval, note) is a persisted `run_step` with tokens, cost and duration. That gives the UI timeline and makes runs **resumable**: when you answer an approval, a `resume_run` job continues from the last step.

### Inbox item
| Kind | Created by | Your response |
|---|---|---|
| `report` | Agent `report` tool | Read, rate (👍/👎 trains the significance threshold), dismiss |
| `alert` | High-significance report | Same as report, plus push |
| `question` | Agent `ask_user` tool or external agent via MCP | Free-text or choice answer; the run resumes |
| `approval` | Gated action (send email, webhook…) or external agent | Approve / deny / edit-then-approve; the run resumes |

Approvals always show **exactly** what will happen (recipient, full payload). They expire after a configurable time (default 48h → treated as denied).

---

## 5. Agent runtime

### Pipeline per trigger
1. **Detect** (no LLM): fetch → normalize → hash → diff. Unchanged → stop.
2. **Triage** (small/cheap model, ~1 call): given mission + diff, return `{relevant: bool, significance, reason}` as validated JSON. Not relevant → run ends as `skipped`, logged at ~zero cost.
3. **Act** (main model, tool loop): system prompt = mission + agent memory + diff + recent reports. Loop up to `max_steps` (default 12), checking the budget before every LLM call.
4. **Output**: the agent must end with `report` (or `nothing_to_report`). The report becomes a document in memory and an inbox item, and notifications fire if significance ≥ the agent's threshold.

### Tools (v1)
| Tool | Side effects | Gated |
|---|---|---|
| `get_diff`, `get_snapshot(source, at?)` | none | no |
| `fetch_url(url)` (SSRF-guarded, domain allowlist, size cap) | none | no |
| `search_memory(query, filters)` | none | no |
| `web_search(query)` (optional, see open decisions) | none | no |
| `remember(key, value)` / `forget(key)` | agent's own memory | no |
| `report(title, body_md, significance, citations[])` | inbox item | no |
| `ask_user(question, choices?)` | inbox item, pauses run | — |
| `send_email(to, subject, body)` | outward | **yes** |
| `post_webhook(endpoint_id, payload)` (pre-registered endpoints only) | outward | **yes** |

Later: `create_github_issue`, `post_slack`, user-defined HTTP tools, and policy-based auto-approval.

### Safety model
- **All fetched content is untrusted.** It's wrapped and labeled as data in prompts, and the system prompt says instructions inside it must be ignored.
- **Injection can't cause side effects on its own:** every outward tool is gated, and the approval card shows the real payload. Webhooks can only go to endpoints the user registered.
- `fetch_url` reuses the existing SSRF guard (no private IPs unless `ALLOW_PRIVATE_SOURCES`), honors the per-agent domain allowlist, and caps response size and time.
- Per-run and per-month budgets; a global kill switch (pause all agents).
- LLM keys and channel secrets are encrypted at rest with AES-256-GCM under `APP_SECRET_KEY`.

### LLM layer changes (`internal/llm`)
- `Request` gains `Tools []ToolDef` and `ToolChoice`; `Response` gains `ToolCalls []ToolCall`.
- Two adapters behind one interface: **OpenAI-compatible** (Groq, OpenRouter, OpenAI, Ollama/LM Studio) and **Anthropic Messages API** (native tool use and prompt caching).
- **Model profiles** per user: `{credential, model, role: triage|main|embed}`. Agents pick profiles; the free Groq chain is the built-in default profile.
- **Cost accounting**: a price table (per model, editable in settings) → `cost_micros` on every step, rolled up per run, agent, day and user.

### Agent quality: evals
Agents are only useful if triage is right. Ship an eval harness from phase 2:
- `testdata/evals/*.yaml`: mission + before/after snapshot + expected `{relevant, min_significance}`.
- `make eval` runs them against a chosen model profile and prints precision/recall + cost.
- CI runs them with recorded responses; live runs are optional with a key.
- 👍/👎 on reports can be exported as new eval cases.

---

## 6. Context hub + MCP

### Memory
Everything worth recalling becomes a **document**: feed entries, page snapshots (latest per source), reports, agent notes, and later manual saves and uploads. Documents are chunked (~800 tokens), embedded (Voyage, 1024 dims) and indexed for **hybrid search** (Postgres `tsvector` + pgvector, merged with reciprocal rank fusion). Agents use it through `search_memory`, and you use it through the search page and ⌘K.

### MCP server
Served by the Go API at `/mcp` (Streamable HTTP transport, official Go MCP SDK), authenticated with **personal API tokens** (scoped, revocable, hashed at rest).

| MCP tool | Scope |
|---|---|
| `search_memory(query, since?, source?)` | `memory:read` |
| `list_reports(agent?, since?)`, `get_report(id)` | `memory:read` |
| `list_agents()`, `get_agent(id)`, `run_agent(id)` | `agents:read` / `agents:run` |
| `inbox_ask(question, choices?)` → item id | `inbox:write` |
| `inbox_request_approval(title, details)` → item id | `inbox:write` |
| `inbox_get_response(id)` (poll; returns pending/answer) | `inbox:write` |

The `inbox_*` tools are pillar C for **external** agents: a Claude Code session or a script can ask you to approve something, and you answer from your phone. The same endpoints are also plain REST (`POST /v1/inbox`) for webhooks and scripts.

---

## 7. Data model (fresh migration series)

Keep `001_better_auth.sql` and `002_extensions.sql`. New tables (all user-owned rows carry `user_id` and are filtered by it in every query):

```
sources              id, user_id, kind, url, title, config jsonb (selector, ignore rules, jsonpath),
                     interval, next_check_at, etag, last_modified, error_count, last_error, paused
snapshots            id, source_id, fetched_at, content_hash, content jsonb (text | items),
                     diff jsonb (vs previous), size_bytes
agents               id, user_id, name, mission, trigger jsonb (on_change, cron), tools text[],
                     allowed_domains text[], triage_profile_id, main_profile_id,
                     budget_run_micros, budget_month_micros, notify jsonb, min_significance,
                     status (active|paused), created_at, updated_at
agent_sources        agent_id, source_id
agent_notes          agent_id, key, value, updated_at
runs                 id, agent_id, trigger (change|cron|manual|mcp), snapshot_id, status,
                     significance, report_document_id, input_tokens, output_tokens,
                     cost_micros, error, started_at, finished_at
run_steps            run_id, seq, kind (llm|tool_call|tool_result|approval|note), payload jsonb,
                     input_tokens, output_tokens, cost_micros, duration_ms, created_at
inbox_items          id, user_id, run_id?, origin (agent|mcp|api), kind (report|alert|question|approval),
                     title, body_md, payload jsonb, status (open|approved|denied|answered|dismissed|expired),
                     response jsonb, rating, expires_at, created_at, resolved_at
documents            id, user_id, kind (feed_item|snapshot|report|note|upload), source_id?, run_id?,
                     title, url, text, search_tsv, created_at
chunks               document_id, idx, text, embedding vector(1024)   -- HNSW index
llm_credentials      id, user_id, provider, base_url, label, key_enc, created_at
model_profiles       id, user_id, name, credential_id (null = built-in free chain), model, role
model_prices         model, input_per_mtok_micros, output_per_mtok_micros   -- editable
notification_targets id, user_id, kind (email|webpush), config_enc, verified
push_subscriptions   id, user_id, endpoint, keys_enc, user_agent, created_at
webhook_endpoints    id, user_id, name, url, secret_enc            -- the only targets post_webhook may use
api_tokens           id, user_id, name, token_hash, scopes text[], last_used_at, revoked_at
usage_daily          day, user_id, agent_id?, requests, input_tokens, output_tokens, cost_micros
jobs                 (unchanged from today)
```

---

## 8. Frontend (SvelteKit)

### Stack
SvelteKit 2, Svelte 5 runes, TypeScript, Tailwind v4, **shadcn-svelte** (Bits UI), lucide-svelte, bun, `svelte-check`. Design tokens and fonts come from the current `globals.css` / `STYLE_GUIDE.md`. better-auth runs through its SvelteKit handler (`svelteKitHandler` in `hooks.server.ts`) against the same Postgres tables. `/api/v1/[...path]/+server.ts` proxies to Go and forwards the cookie. Live run logs use SSE. `adapter-node` builds the production image.

### Screens
| Route | Purpose |
|---|---|
| `/` | Landing: "agents you can leave running", live demo of a run timeline |
| `/signin`, `/signup`, `/invite/[code]` | Auth (email/password; OAuth later). First user becomes admin. |
| `/app/onboarding` | 1) Model: use free default or paste a key. 2) Pick a template agent. 3) Enable push. |
| `/app` (Inbox) | **Home.** Reports, alerts, questions and approvals; filters; approve/deny inline; keyboard-first (j/k, a approve, d deny, e edit) |
| `/app/agents` | List with status, last run, 30-day cost, success/skip ratio |
| `/app/agents/new`, `/app/agents/[id]` | Editor: mission, sources, trigger, tools, budget, notify; **"Dry run"** on the latest diff |
| `/app/runs/[id]` | Timeline: diff → triage verdict → each LLM/tool step (expandable payloads) → output; tokens and cost per step; live via SSE |
| `/app/sources`, `/app/sources/[id]` | Sources and snapshot history with side-by-side **diff viewer**; selector picker preview |
| `/app/memory` | Hybrid search over everything; ⌘K anywhere |
| `/app/settings/*` | Models & keys, notifications, webhooks, API tokens / MCP setup (copy-paste config for Claude Desktop / Claude Code), usage & budgets, members/invites |

### PWA
Manifest + service worker (SvelteKit `service-worker.ts`). The Go worker sends **web push** with VAPID (`webpush-go`). Approval notifications deep-link to the inbox item, so approve/deny works from the phone.

---

## 9. Roadmap

Each phase ends in a working, demoable app.

| Phase | Deliverables | Done means |
|---|---|---|
| **0. Pivot foundations** | Rename to Lookfar (Go module `github.com/codebyaadi/lookfar`, compose service/DB names, env prefixes, README, repo); scaffold `web/` (SvelteKit + shadcn-svelte + better-auth); port design tokens; delete `www/`; fresh migration series; move `internal/feed` → `internal/source`; remove reader/enrich code; API-token auth; update Makefile, compose, Dockerfile, CI | Sign up in SvelteKit → `/app` shows an empty inbox from Go `/v1/me` + `/v1/inbox` |
| **1. Sources & change detection** | `rss`/`page`/`github` sources, normalization + noise filters, snapshots, diffs, scheduler, Sources UI + diff viewer. **No LLM yet.** | Add a pricing page; edit it locally; see a clean diff in the UI |
| **2. Agent runtime** | LLM tool calling (OpenAI-compatible + Anthropic), encrypted BYOK credentials, model profiles, price table, run state machine + steps, triage → act pipeline, budgets, read-only tools, `report`, agent editor, run timeline (SSE), templates, eval harness | A template agent watching a page writes a sensible report on a real change and skips a noise change, with cost shown |
| **3. Inbox & notifications** | Inbox items, `ask_user`, gated `send_email` + `post_webhook`, resume after answer, expiry, SMTP email, PWA + web push, significance thresholds + 👍/👎 | An agent proposes a webhook call → push on phone → approve → run resumes and succeeds |
| **4. Memory & MCP** | Documents + chunks + embeddings, hybrid search, `search_memory` tool, memory page + ⌘K, MCP server with scoped tokens, external `inbox_*` tools, REST inbox API | Claude Desktop connects to `/mcp`, finds last week's reports, and asks you for an approval that you answer in the inbox |
| **5. Polish & ship** | Landing page, one-file compose (postgres + api + worker + web), backups doc, invites, rate limits, empty/error states, tests, docs | Fresh VPS → `docker compose up` → working Lookfar instance in under 10 minutes |

**Later:** Telegram/ntfy/Slack, policy-based auto-approval, headless-browser sources, user-defined HTTP tools, newsletter inbox source, hosted multi-tenant version.

---

## 10. Open decisions

Each has a default the plan assumes. Change any of them before its phase starts.

| # | Decision | Options | Default | Needed by |
|---|---|---|---|---|
| 1 | **Domain + GitHub** | lookfar.dev · lookfar.app · getlookfar.com; repo rename vs new repo | Check and register before Phase 5 launch; rename this repo (GitHub redirects old URLs) | Phase 0 |
| 2 | **Web search tool** | None in v1 · SearXNG (self-hosted, compose profile) · Tavily / Brave API (BYOK) | Pluggable interface; SearXNG optional profile + BYOK Tavily/Brave | Phase 2 |
| 3 | **JS-rendered pages** | Plain HTTP + readability only · optional headless Chromium (`chromedp` or a Browserless container) | Plain HTTP in v1; Chromium as an optional compose profile later | Phase 1 / later |
| 4 | **Frontend lint/format** | oxlint (TS) + Prettier w/ `prettier-plugin-svelte` · ESLint + `eslint-plugin-svelte` + Prettier · oxfmt if it supports `.svelte` by then | oxlint + Prettier + svelte-check | Phase 0 |
| 5 | **Migrations** | Fresh series (drop reader tables) · keep and migrate forward | Fresh series (nothing deployed) | Phase 0 |
| 6 | **Sign-up mode** | First user = admin, then invite-only · open sign-up | Invite-only | Phase 0 |
| 7 | **Embeddings default** | Voyage (current) · OpenAI-compatible BYOK · local (Ollama) | Voyage, OpenAI-compatible as BYOK option, 1024 dims fixed | Phase 4 |
| 8 | **Frontend data layer** | SvelteKit `load` + form actions only · plus `@tanstack/svelte-query` for client polling/caching | `load` + form actions, with SSE for live data; add svelte-query only if needed | Phase 0 |
| 9 | **Monorepo layout** | `web/` beside Go at root (as today) · `apps/web` + `apps/api` workspace | `web/` at root | Phase 0 |

---

## 11. Risks

| Risk | Mitigation |
|---|---|
| Noisy page diffs wake agents constantly | Normalization, ignore selectors, minimum diff size, triage step; diffs viewable so users can tune filters |
| Free-tier models are weak at tool use | Triage works fine on small models; the main loop recommends a BYOK model; evals show quality per model |
| Prompt injection from watched pages | Untrusted-content wrapping, all outward tools gated, webhook targets pre-registered, approval shows the real payload |
| Cost surprises with BYOK | Per-run and monthly budgets checked before every call, cost shown per step, global pause |
| Sites blocking bots | Respectful intervals, conditional GET, honest User-Agent; headless browser only as an opt-in |
| Scope creep across three pillars | Strict phase order: A → C → B; each phase demoable before the next starts |
