# Load .env (if present) and export it to every recipe.
ifneq (,$(wildcard .env))
include .env
export
endif

GO_PKGS    := ./cmd/... ./internal/...
TOOL       := go tool -modfile=tools/go.mod
SQLC_VER   := 1.31.1
SQLC       := bin/sqlc
MIGRATIONS := sql/schema

.PHONY: help
help:
	@echo "Setup"
	@echo "  make db            start Postgres (pgvector) in Docker"
	@echo "  make migrate-up    apply migrations       make migrate-down  roll back one"
	@echo "  make migration name=add_feeds   create a new migration"
	@echo "Develop"
	@echo "  make api           run API with live reload (air)"
	@echo "  make worker        run the background worker"
	@echo "  make web           run the Next.js app"
	@echo "Quality"
	@echo "  make generate      regenerate sqlc code"
	@echo "  make check         gofmt + vet + test + web lint/typecheck"
	@echo "  make build         build api + worker binaries into bin/"

## --- infrastructure -------------------------------------------------------

.PHONY: db db-down
db:
	docker compose up -d postgres

db-down:
	docker compose down

## --- migrations -----------------------------------------------------------

.PHONY: migrate-up migrate-down migrate-status migration
migrate-up:
	$(TOOL) goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" up

migrate-down:
	$(TOOL) goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down

migrate-status:
	$(TOOL) goose -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" status

migration:
	@test -n "$(name)" || (echo "usage: make migration name=<snake_case>"; exit 1)
	$(TOOL) goose -dir $(MIGRATIONS) -s create $(name) sql

## --- codegen --------------------------------------------------------------

$(SQLC):
	@mkdir -p bin
	curl -sSfL https://github.com/sqlc-dev/sqlc/releases/download/v$(SQLC_VER)/sqlc_$(SQLC_VER)_linux_amd64.tar.gz | tar -xz -C bin sqlc

.PHONY: generate
generate: $(SQLC)
	$(SQLC) generate

## --- run ------------------------------------------------------------------

.PHONY: api worker web
api:
	$(TOOL) air

worker:
	go run ./cmd/worker

web:
	cd www && bun run dev

## --- quality --------------------------------------------------------------

.PHONY: fmt check test build clean
fmt:
	gofmt -s -w cmd internal

test:
	go test $(GO_PKGS)

check:
	@test -z "$$(gofmt -l cmd internal)" || (echo "gofmt needed:"; gofmt -l cmd internal; exit 1)
	go vet $(GO_PKGS)
	go test $(GO_PKGS)
	cd www && bun run lint && bun run typecheck

build:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api
	CGO_ENABLED=0 go build -o bin/worker ./cmd/worker

clean:
	rm -rf tmp bin/api bin/worker
