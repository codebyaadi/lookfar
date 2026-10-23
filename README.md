<p align="center">
  <img src="docs/images/banner.png" alt="Lookfar: agents that look far, so you don't have to" width="100%" />
</p>

<p align="center">
  <b>Self-hosted watcher agents you can leave running.</b><br />
  Describe what to watch in plain language. Lookfar checks pages, feeds and repos on a schedule, tells you only what matters, and asks before it does anything on your behalf.
</p>

<p align="center">
  <a href="https://lookfar.codebyaadi.com">Website</a> ·
  <a href="PLAN.md">Roadmap</a> ·
  <a href="STYLE_GUIDE.md">Style guide</a> ·
  <a href="LICENSE">MIT license</a>
</p>

> **Early days.** Lookfar is being built in the open, phase by phase. Accounts, personal API tokens, and watching pages, RSS feeds and GitHub releases (with stored snapshots and readable diffs) work today. The agent runtime, approvals, memory and MCP are next, in that order. Every feature below is labelled **Live** or **Coming soon**, and [PLAN.md](PLAN.md) has the full design.

---

## Why Lookfar

Page monitors fire on every cookie banner. AI agents re-read the web on every run and burn tokens on nothing. And agents that can act on their own are hard to trust. Lookfar is built around fixing those three problems:

- **No change, no tokens.** Every run starts with a free, deterministic check: fetch, normalise, hash, diff (this part is live today). A small model triages only what changed, and your main model runs only when it likely matters.
- **Quiet by default.** Agents report above a significance threshold you set. One useful alert beats twenty digests.
- **Read freely, act with permission.** Anything with side effects (email, webhooks) waits in your inbox with the exact payload shown. Nothing happens without a yes.
- **Show the work.** Every run keeps a step-by-step timeline of what was fetched, what changed, what the model saw, which tools ran, and what each step cost.
- **Own your stack.** One `docker compose up`, your own Postgres, your own LLM keys, and a memory your other AI tools can use over MCP.

## What you get

| | | |
| --- | --- | --- |
| **Live** | Accounts and sign-in | better-auth with email and password. The first account becomes the admin, then sign-up closes unless you set `ALLOW_SIGNUP=true`. |
| **Live** | Personal API tokens | `Authorization: Bearer lf_…`, stored as a SHA-256 hash and revocable. A management UI is coming. |
| **Live** | Inbox API and shell | `GET /v1/inbox` and the `/app` inbox page, ready for reports, alerts, questions and approvals. |
| **Live** | Job queue | Postgres `FOR UPDATE SKIP LOCKED` queue with retries and rate-limit deferral. |
| **Live** | Sources and change detection | Web pages (visible text or a CSS selector), RSS and Atom feeds, and GitHub releases (falling back to tags). Text is normalised and noise is ignored: cookie banners, hidden elements, "5 minutes ago" and clock text, plus your own selectors and regex rules. Each real change is stored as a snapshot with a line diff, viewable unified or side by side. |
| **Live** | Scheduler | The worker queues due checks as jobs. Failures are recorded and retried with exponential backoff, and one check never runs twice at once for the same source. |
| Coming soon | Watcher agents | A mission, sources, a trigger, tools and a budget. Triage, then act, then report. |
| Coming soon | Approvals inbox | Approve, deny or edit before anything outward-facing happens, with push and email notifications. |
| Coming soon | Memory and MCP | Hybrid search over everything agents read and write, exposed to Claude, Cursor and others at `/mcp`. |

### Starter agents

- **Competitor pricing:** "Check these 4 pricing pages every Monday. Report only real price, plan or limit changes, not copy edits."
- **Dependency radar:** "Watch Go, Postgres and pgx releases. Tell me only about breaking changes or security fixes that affect a Go + Postgres 17 app."
- **Opportunity scout:** "Watch these grant and job boards. Alert me about remote Go roles over $150k or grants for open-source infra."
- **Story tracker:** "Follow this topic across my feeds. Send a short 'what changed since last time' each evening."

## Built with care

- **Fetched content is untrusted data.** Prompts label it as such, and instructions inside a page are ignored. Because every outward tool is gated, an injection still cannot cause a side effect on its own.
- **The SSRF guard is on by default.** Fetches refuse private, loopback and link-local addresses unless you set `ALLOW_PRIVATE_SOURCES=true` for local testing, and the address is checked at connect time.
- **API tokens are hashed at rest** and only shown once when created.
- **Budgets are checked before every model call** (planned with the agent runtime): per-run and monthly caps, plus a global pause.
- **Sign-up is closed by default** after the first account.

## Stack

SvelteKit 3 and Svelte 5 (runes) · Tailwind v4 · better-auth · bun · oxlint and oxfmt · Go 1.26 with chi, pgx and sqlc · PostgreSQL 17 with pgvector · goose migrations · Groq and OpenRouter free tiers (OpenAI-compatible) by default, Voyage AI for embeddings.

```
cmd/
  api/        HTTP API (chi): /v1/me, /v1/inbox, /v1/sources, token and session auth
  worker/     Scheduler and job runner (runs source checks)
internal/
  auth/       better-auth session cookie and API-token authentication
  httpapi/    routes and handlers
  jobs/       Postgres job queue
  llm/        OpenAI-compatible provider chain, rate limiter, Voyage client
  source/     SSRF-guarded fetcher, page/feed/GitHub checkers, normalisation, diff engine
  store/      sqlc-generated queries
sql/
  schema/     goose migrations
  queries/    sqlc queries
web/          SvelteKit app: landing page, auth, inbox, sources and diff viewer, API proxy
```

better-auth runs inside SvelteKit and owns sign-up and sessions. The Go API verifies the signed `better-auth.session_token` cookie with the shared `BETTER_AUTH_SECRET` and looks the session up in Postgres, or accepts a personal API token. The browser talks to `/api/v1/*` on the SvelteKit origin, which proxies to the Go API.

## Run it locally

Requires Go 1.26+, [Bun](https://bun.sh), Docker and `make` (or run the commands the Makefile wraps).

```bash
cp .env.example .env            # set BETTER_AUTH_SECRET (openssl rand -base64 32)
cp web/.env.example web/.env    # use the same BETTER_AUTH_SECRET
docker compose up -d postgres   # Postgres 17 + pgvector
make migrate-up
(cd web && bun install)
```

Run each in its own terminal:

```bash
make api      # Go API on :8080 (live reload)
make worker   # background worker
make web      # SvelteKit on :5173
```

| Service | URL |
| --- | --- |
| Web | http://localhost:5173 |
| API | http://localhost:8080 (`/healthz`) |

Open http://localhost:5173/signup, create the first account, and you land on `/app`. Open **Sources**, add a page, a feed or a GitHub repo, and Lookfar takes a baseline snapshot straight away. The worker then re-checks it on its schedule and stores a diff whenever the content really changes.

To try it on a page you control, serve one locally and set `ALLOW_PRIVATE_SOURCES=true` in `.env` (local testing only: it switches off the guard that blocks private addresses).

> If port 5432 is already taken, set `POSTGRES_PORT=5433` in `.env` and update `DATABASE_URL` to match. Without `make` (for example on Windows), run `go tool -modfile=tools/go.mod goose -dir sql/schema postgres "$DATABASE_URL" up` and `go run ./cmd/api`.

### Commands

| Command | What it does |
| --- | --- |
| `make api` / `make worker` / `make web` | Run a service with reload |
| `make migrate-up` / `make migrate-down` | Apply or roll back migrations |
| `make migration name=add_sources` | Create a migration in `sql/schema` |
| `make generate` | Regenerate sqlc code from `sql/queries` |
| `make check` | gofmt, vet, Go tests, oxlint and svelte-check |
| `cd web && bun run format:fix` | Format the frontend with oxfmt |

The integration tests (sources, the HTTP API) need a migrated Postgres and `DATABASE_URL`; without one they skip themselves.

Settings you may want for sources, all optional:

| Setting | What it does |
| --- | --- |
| `GITHUB_TOKEN` | No scopes needed. Raises the GitHub API rate limit for release sources |
| `CHECK_CONCURRENCY` | Source checks the worker runs at once (default 4) |
| `WORKER_TICK` | How often the worker looks for due checks (default 30s) |
| `ALLOW_PRIVATE_SOURCES` | Allow watching private and loopback addresses. Local testing only |

CI runs gofmt, a sqlc drift check, vet, a migration up, down and up cycle against Postgres, the Go tests (including the integration tests), the frontend lint, format, type check and build.

## Self-host it

```bash
cp .env.example .env     # set BETTER_AUTH_SECRET and BETTER_AUTH_URL
docker compose --profile app up -d
```

This starts Postgres, the API, the worker and the web app (on port 3000). Apply the migrations once against the database before first use. The `app` profile is new and has not been exercised end to end yet, so expect rough edges and please open an issue.

The web app can also be deployed to Vercel (`adapter-vercel` is used automatically on Vercel). It then needs a hosted Postgres and a hosted API, set through `DATABASE_URL` and `API_URL`.

## Things to know

- **Docker and the Go toolchain.** Running goose through `go tool` downloads many database drivers on first use, which can be slow on a flaky network. Applying `sql/schema/*.sql` with `psql` also works.
- **Free-tier models are weak at tool use.** Triage works fine on small models, but the main agent loop will recommend a model of your own.
- **Headless-browser sources are not planned for v1.** Pages are fetched over plain HTTP and extracted as readable text or a selector.
- **The roadmap is strict about order.** Watcher agents first, then the approvals inbox, then memory and MCP. Each phase ends in something you can run.

## Roadmap

| Phase | Deliverable | Status |
| --- | --- | --- |
| 0. Foundations | Rename, SvelteKit app, fresh migrations, API-token auth, landing page | Done |
| 1. Sources and change detection | RSS, page and GitHub sources, snapshots, diffs, scheduler, diff viewer | Done |
| 2. Agent runtime | Tool calling, triage then act, budgets, run timeline, evals | Next |
| 3. Inbox and notifications | `ask_user`, gated email and webhook, push, resume after answer | Planned |
| 4. Memory and MCP | Embeddings, hybrid search, MCP server with scoped tokens | Planned |
| 5. Polish and ship | One-file compose, backups doc, invites, rate limits | Planned |

## License

MIT. See [LICENSE](LICENSE).
