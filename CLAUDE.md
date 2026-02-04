# Sandbox Engine

Изолированные sandbox-среды с Claude Code CLI для hiring-задач и обучения.

## Архитектура

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Web UI        │────▶│  Sandbox Engine  │────▶│  Docker         │
│   (React)       │     │  (Go API)        │     │  Containers     │
└─────────────────┘     └──────────────────┘     └─────────────────┘
        │                       │                        │
        │ WebSocket             │                        │
        └───────────────────────┼────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │     Traefik           │
                    │  (Reverse Proxy)      │
                    └───────────────────────┘
```

## Структура проекта

```
sandbox-engine/
├── cmd/sandbox-engine/     # Entry point
├── internal/
│   ├── api/                # HTTP handlers, WebSocket terminal
│   ├── config/             # Configuration
│   ├── models/             # Domain models
│   ├── sandbox/            # Docker container management
│   ├── services/           # Business logic
│   ├── storage/            # PostgreSQL repository
│   └── templates/          # Template loader
├── docker/
│   ├── workspace/          # Workspace images (base, python, node, go)
│   └── services/           # Service images (postgres, redis)
├── deploy/                 # Docker Compose для prod
├── web/                    # React frontend (xterm.js terminal)
├── templates/              # YAML шаблоны sandbox-ов
└── migrations/             # SQL миграции
```

## Ключевые компоненты

### API Endpoints

- `POST /api/v1/sandboxes` — создать sandbox
- `GET /api/v1/sandboxes/{id}` — получить sandbox
- `DELETE /api/v1/sandboxes/{id}` — удалить sandbox
- `GET /api/v1/ws/terminal/{id}` — WebSocket терминал
- `GET /api/v1/templates` — список шаблонов

### Аутентификация

API использует API-ключи (таблица `api_clients`):
- Header: `Authorization: Bearer <key>` или `X-API-Key: <key>`
- WebSocket: query param `?token=<key>`

### Claude Code Auth

Credentials Claude Code сохраняются в Docker volume `claude-auth`:
- Volume mount: `/home/coder/.claude`
- При старте контейнера `entrypoint.sh` копирует `~/.claude/config.json` → `~/.claude.json`
- Это позволяет пропустить onboarding при повторных запусках

**Важные файлы:**
- `~/.claude.json` — конфиг с `hasCompletedOnboarding: true`
- `~/.claude/.credentials.json` — OAuth токены
- `~/.claude/config.json` — копия конфига в volume

## Локальная разработка

```bash
# Запуск с .env
cp .env.example .env
# Отредактировать .env

# Запуск
make run

# Или через Docker Compose
cd deploy && docker compose up -d
```

## Продакшен (Netcup)

```bash
# SSH на сервер
ssh terra@$(doppler secrets get NETCUP_IP --project servers --config dev --plain)

# Директория
cd /var/www/sandbox-engine

# Статус
docker compose -f deploy/docker-compose.prod.yml ps

# Логи
docker compose -f deploy/docker-compose.prod.yml logs -f sandbox-engine

# Пересборка образов
cd docker/workspace && ./build.sh
```

## Домены

- `terra-sandbox.ru` — Web UI
- `api.terra-sandbox.ru` — API
- `traefik.terra-sandbox.ru` — Traefik dashboard

## Шаблоны

Шаблоны в `templates/examples/`:
- `backend-python.yaml` — Python + FastAPI
- `backend-node.yaml` — Node.js
- `backend-go.yaml` — Go

Формат:
```yaml
name: backend-python
base_image: workspace-python:latest
services:
  - postgres
  - redis
resources:
  cpu_limit: "2"
  memory_limit: "4Gi"
ttl: 4h
expose:
  - container: 8000
    public: true
```

## Частые задачи

### Пересборка workspace образов

```bash
ssh terra@<server>
cd /var/www/sandbox-engine/docker/workspace
docker build -f Dockerfile.base -t workspace-base:latest .
docker build -f Dockerfile.python -t workspace-python:latest .
```

### Проверка Claude Code auth

```bash
# В контейнере sandbox
docker exec -u coder sandbox-<id> cat ~/.claude.json | grep hasCompletedOnboarding
docker exec -u coder sandbox-<id> ls -la ~/.claude/
```

### Очистка старых sandbox-ов

```bash
# Cleanup job запускается автоматически
# Или вручную:
docker ps --filter 'name=sandbox-' --format '{{.Names}}'
docker rm -f sandbox-<id>
```
