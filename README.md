# Lookfar

Self-hosted watcher agents you can leave running. Describe what to watch in plain language. Agents check pages, feeds and repos on a schedule, tell you only what matters, ask before doing anything with side effects, and keep a memory your other AI tools can use through MCP.

> 🚧 **Pivot in progress.** Phase 0 of [PLAN.md](PLAN.md) is done: the Go API and the SvelteKit app are in place, with sign-in, API-token auth and an empty inbox. Sources, agents and MCP land in the next phases.

## Stack

| Part | Tech |
|---|---|
| Web | SvelteKit 3, Svelte 5 (runes), Tailwind v4, better-auth, bun, oxlint + oxfmt |
| API | Go 1.26, chi, pgx, sqlc (`cmd/api`) |
| Worker | Go (`cmd/worker`): job queue and, later, source checks and agent runs |
| Database | PostgreSQL 17 + pgvector, migrations via goose |
| AI | Groq / OpenRouter free tiers (OpenAI-compatible) by default, Voyage AI embeddings |

better-auth (in SvelteKit) owns sign-up and sessions. The Go API checks the signed `better-auth.session_token` cookie with the shared `BETTER_AUTH_SECRET` and looks the session up in Postgres, or accepts a personal API token (`Authorization: Bearer lf_...`). The browser calls `/api/v1/*` on the SvelteKit origin, which proxies to the Go API. The first account becomes the admin; sign-up then closes unless `ALLOW_SIGNUP=true`.

## Getting started

Prerequisites: Go 1.26+, bun, Docker, make.

```bash
cp .env.example .env                 # set BETTER_AUTH_SECRET (openssl rand -base64 32)
cp web/.env.example web/.env         # use the same BETTER_AUTH_SECRET
make db                              # Postgres + pgvector in Docker
make migrate-up
cd web && bun install && cd ..
```

Run each in its own terminal:

```bash
make api      # Go API on :8080 (live reload)
make worker   # background worker
make web      # SvelteKit on :5173
```

Open http://localhost:5173/signup. After signing up you land on `/app`, which is served using data from the Go API.

## Common tasks

| Command | What it does |
|---|---|
| `make migration name=add_feeds` | new goose migration in `sql/schema` |
| `make generate` | regenerate sqlc code from `sql/queries` (downloads sqlc into `bin/`) |
| `make check` | gofmt, vet, tests, oxlint, svelte-check |
| `cd web && bun run format:fix` | format the frontend with oxfmt |
| `docker compose --profile app up --build` | run Postgres, API and worker all in Docker |

## License

MIT. See [LICENSE](LICENSE).
