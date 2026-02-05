# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Sandbox Engine** — Go service that creates isolated Docker-based development environments with web terminal access. Part of the Terra ecosystem alongside **assessment-service** (orchestrator) and **proxy-api** (API key management).

## Commands

```bash
make build          # CGO_ENABLED=0 go build → bin/sandbox-engine
make dev            # go run ./cmd/sandbox-engine
make test           # go test -v -race -coverprofile=coverage.out ./...
make test-short     # go test -v -short ./...
make lint           # golangci-lint run ./...
make lint-fix       # golangci-lint run --fix ./...
make services-up    # Start postgres + redis + traefik (docker compose)
make services-down  # Stop dev services

# Web UI (React)
cd web && npm run dev    # Vite dev server (:5173)
cd web && npm run build  # Production build

# Workspace Docker images
cd docker/workspace
docker build -f Dockerfile.base -t workspace-base:latest .
docker build -f Dockerfile.python -t workspace-python:latest .
# Tag for FROM reference:
docker tag workspace-base:latest ghcr.io/terra-clan/sandbox-engine/workspace-base:latest
```

## Architecture

### Startup chain (`cmd/sandbox-engine/main.go`)
```
config.Load() (env vars)
→ storage.MigrateFromDSN() (auto-migrate on startup)
→ storage.NewPostgresRepository() (pgx pool)
→ services.NewRegistry() + Register("postgres"|"redis")
→ templates.NewLoader().LoadFromDir()
→ sandbox.NewManager() (Docker client + all deps)
→ cleanup.NewCleaner().Start() (background TTL reaper)
→ api.NewServer() → http.ListenAndServe(:8080)
```

### Two access patterns

1. **Direct sandbox** (legacy): `POST /api/v1/sandboxes` → container created immediately → WebSocket terminal at `/api/v1/ws/terminal/{id}?token=API_KEY`

2. **Lazy session** (primary): `POST /api/v1/sessions` → no container yet → candidate opens `/join/{token}` → clicks Start → `POST /api/v1/join/{token}/activate` → container provisions → WebSocket at `/api/v1/ws/session-terminal/{id}?session_token=TOKEN`

### Session lifecycle
```
ready → provisioning → active → expired
  └──────────────────→ failed
```
- `ready`: created by admin, no container
- `provisioning`: candidate clicked Start, sandbox spinning up (background goroutine polls up to 30s)
- `active`: container running, TTL started from activation time
- `ActivateSession` is idempotent — re-calling returns current state without duplicate containers

### Key packages

| Package | Role |
|---------|------|
| `internal/api/` | Chi router, handlers, auth middleware, WebSocket terminal proxy |
| `internal/sandbox/` | `Manager` interface + `DockerManager` — container CRUD, session CRUD, async provisioning |
| `internal/storage/` | `Repository` interface + PostgreSQL impl (pgx), auto-migrations |
| `internal/services/` | `Provider` interface + postgres/redis providers — per-sandbox DB/keyspace isolation |
| `internal/templates/` | YAML template loader from `TEMPLATES_DIR` |
| `internal/cleanup/` | Background worker deletes expired sandboxes on interval |
| `internal/models/` | Domain types: Sandbox, Session, Template, ApiClient |

### Web UI (`web/`)

React 19 + Vite 6 + Tailwind + xterm.js + Framer Motion + react-router-dom

Routes:
- `/join/:token` → `JoinPage` (welcome → provisioning animation → workspace)
- `/*` → `WorkspaceRoute` (legacy: `?sandbox=ID&token=API_KEY`)

### Workspace Docker images (`docker/workspace/`)

Multi-layer:
- `Dockerfile.base` — debian:bookworm-slim + Node.js 20 + Claude Code CLI + `coder` user (uid 1000)
- `Dockerfile.python` — extends base + Python 3.12 + poetry/ruff/pytest
- `Dockerfile.node`, `Dockerfile.go` — language variants
- `entrypoint.sh` — restores Claude auth from volume, fixes permissions

Claude Code auth mounted via Docker volume `claude-auth:/home/coder/.claude` (in `manager.go:createContainer`).

## Authentication

- **Admin API**: `Authorization: Bearer sk_...` or `X-API-Key` header → `api_clients` table → permissions (`sandboxes:read/write`, `sessions:read/write`, `templates:read`)
- **WebSocket (admin)**: `?token=API_KEY` query param
- **Join endpoints** (`/join/{token}`, `/ws/session-terminal/{id}`): public, session token (48-char hex) is the auth

## Environment Variables

All have defaults (see `internal/config/config.go`):
- `DATABASE_DSN` — PostgreSQL connection string
- `REDIS_ADDRESS`, `REDIS_PASSWORD`
- `DOCKER_HOST` — Docker socket (`npipe:////./pipe/dockerDesktopLinuxEngine` on Windows)
- `DOCKER_NETWORK` — network for containers (default: `sandbox-network`)
- `DOCKER_PULL_POLICY` — `if-not-present` | `never` | `always`
- `TRAEFIK_ENABLED` — generate Traefik labels on containers
- `SANDBOX_DOMAIN` — domain for routing (e.g., `terra-sandbox.ru`)
- `TEMPLATES_DIR` — path to YAML templates
- `CLEANUP_INTERVAL` — TTL cleanup frequency (default: `5m`)

## Dev services

`make services-up` runs `docker/services/docker-compose.yml`:
- PostgreSQL 16 (`:5432`, user: `sandbox`, pass: `sandbox_secret`, db: `sandbox_engine`)
- Redis 7 (`:6379`, pass: `redis_secret`)
- Traefik v3 (`:80/:443`, dashboard `:8081`)

## Deployment

Production on Netcup via `deploy/docker-compose.prod.yml` + nginx (`deploy/nginx/`).
Domains: `terra-sandbox.ru` (UI), `api.terra-sandbox.ru` (API).

```bash
ssh terra@$(doppler secrets get NETCUP_IP --project servers --config dev --plain)
cd /var/www/sandbox-engine
docker compose -f deploy/docker-compose.prod.yml logs -f sandbox-engine
```

## Common pitfalls

- **CRLF on Windows**: Scripts in `docker/workspace/` must have LF endings. CRLF causes `exec format error` in Linux containers.
- **Background goroutines**: Provisioning runs async. Never use `r.Context()` — use `context.Background()` (request context cancels when response is sent).
- **Docker on Windows**: Use `DOCKER_HOST=npipe:////./pipe/dockerDesktopLinuxEngine`.
- **Workspace image FROM**: `Dockerfile.python` references `ghcr.io/terra-clan/sandbox-engine/workspace-base:latest`. For local dev, tag your build: `docker tag workspace-base:latest ghcr.io/terra-clan/sandbox-engine/workspace-base:latest`.
