# Lookfar

Self-hosted watcher agents you can leave running. Describe what to watch in plain language. Agents check pages, feeds and repos on a schedule, tell you only what matters, ask before doing anything with side effects, and keep a memory your other AI tools can use through MCP.

> 🚧 **Pivot in progress.** The project is moving from an AI RSS reader to watcher agents + approvals inbox + MCP context hub, with a SvelteKit frontend. See [PLAN.md](PLAN.md) for the new architecture and roadmap. The code below still describes the old reader (formerly Feedoku) until Phase 0 of the new plan lands, which also renames modules and services to Lookfar.

## Stack (current code)

| Part | Tech |
|---|---|
| Web | Next.js 16, React 19, Tailwind v4, shadcn/ui, better-auth, bun, oxlint + oxfmt (moving to SvelteKit 2 + Svelte 5 + shadcn-svelte) |
| API | Go 1.26, chi, pgx, sqlc (`cmd/api`) |
| Worker | Go (`cmd/worker`): feed polling and AI jobs |
| Database | PostgreSQL 17 + pgvector, migrations via goose |
| AI | Groq / OpenRouter free tiers (OpenAI-compatible), Voyage AI embeddings |

better-auth (in Next.js) owns sign-up and sessions. The Go API checks the signed `better-auth.session_token` cookie with the shared `BETTER_AUTH_SECRET` and looks the session up in Postgres. The browser calls `/api/v1/*` on the Next.js origin, and Next rewrites those calls to the Go API.

## Getting started

Prerequisites: Go 1.26+, bun, Docker, make.

```bash
cp .env.example .env                 # set BETTER_AUTH_SECRET (openssl rand -base64 32)
cp www/.env.example www/.env.local   # use the same BETTER_AUTH_SECRET
make db                              # Postgres + pgvector in Docker
make migrate-up
cd www && bun install && cd ..
```

Run each in its own terminal:

```bash
make api      # Go API on :8080 (live reload)
make worker   # background worker
make web      # Next.js on :3000
```

Open http://localhost:3000/signup. After signing up you land on `/app`, which is served using data from the Go API.

## Common tasks

| Command | What it does |
|---|---|
| `make migration name=add_feeds` | new goose migration in `sql/schema` |
| `make generate` | regenerate sqlc code from `sql/queries` (downloads sqlc into `bin/`) |
| `make check` | gofmt, vet, tests, oxlint, typecheck |
| `cd www && bun run format:fix` | format the frontend with oxfmt |
| `docker compose --profile app up --build` | run Postgres, API and worker all in Docker |

## License

MIT. See [LICENSE](LICENSE).
