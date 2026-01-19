# billfold-gin-gorm-api

Go/Gin REST API for transactional payments and credits, built on MariaDB (GORM) with InfluxDB request metrics and Sentry error tracking — a reference implementation of the patterns needed for peak-load, no-data-loss payment services (venue/event wallet-style credit systems).

**Live:** `https://billfold.ddns.net/api`

## Stack

Go, Gin, GORM (MariaDB), InfluxDB, Sentry, Swagger/OpenAPI annotations, Docker, GitLab CI.

## Setup

```bash
go mod tidy
cp .env.example .env
go run main.go
```

## Structure

```
billfold-gin-gorm-api/
  main.go
  config/       # DB (MariaDB), InfluxDB, Sentry, migrations
  models/       # User, Post, Tag, Wallet, Transaction
  handlers/     # Auth, CRUD, payment/credit handlers
  routes/       # Route registration
  middleware/   # Auth (JWT), DB transaction wrapper, request metrics, Sentry
  Dockerfile
  .gitlab-ci.yml
```

## Payments & credits

`models/payment.go` and `handlers/payment_handler.go` implement the transactional core:

- `Wallet.Balance` is only ever mutated inside a DB transaction alongside a `Transaction` ledger row — balance and ledger can't drift apart even on a mid-request crash.
- Row locking (`SELECT ... FOR UPDATE`) on the wallet row prevents lost updates when concurrent requests hit the same wallet under peak load.
- Every adjustment requires a client-supplied `idempotency_key`, so retried requests (timeouts, load-balancer retries) are safely no-ops instead of double-applying.
- `/payments/*` routes require a valid JWT (`middleware.RequireAuth`) so balance mutations can't be triggered by unauthenticated callers.

## API Reference

Base URL: `https://billfold.ddns.net/api/v1` (local dev: `http://localhost:8080/api/v1`)

All bodies are JSON. Authenticated routes require `Authorization: Bearer <token>`.

### Auth

**`POST /auth/register`**

```json
// request
{ "username": "alice", "email": "alice@example.com", "password": "at-least-8-chars" }

// 201 response
{ "token": "eyJ...", "user": { "ID": 1, "username": "alice", "email": "alice@example.com", "is_active": true } }
```

**`POST /auth/login`**

```json
// request
{ "username": "alice", "password": "at-least-8-chars" }

// 200 response
{ "token": "eyJ...", "user": { "ID": 1, "username": "alice", "email": "alice@example.com" } }
```

### Payments (auth required)

**`POST /payments/adjust`** — credit or debit a wallet. Transactional, row-locked, idempotent.

```json
// request
{
  "user_id": 1,
  "type": "credit",              // "credit" | "debit"
  "amount_cents": 5000,
  "idempotency_key": "order-1234-attempt-1",  // required, 8-100 chars, unique per logical operation
  "reference": "top-up via card"  // optional
}

// 200 response
{
  "message": "Balance adjusted successfully",
  "transaction": {
    "ID": 1, "wallet_id": 1, "type": "credit",
    "amount_cents": 5000, "idempotency_key": "order-1234-attempt-1",
    "reference": "top-up via card"
  }
}
```

Re-sending the same `idempotency_key` returns the original transaction instead of applying it twice. Debits that would overdraw the wallet return `422` with `{"error": "insufficient balance"}`.

**`GET /payments/wallets/:user_id`**

```json
// 200 response
{ "ID": 1, "user_id": 1, "balance_cents": 5000 }
```

### Users

- `POST /users` — create (public registration endpoint; `auth/register` is preferred for login-capable accounts)
- `GET /users?page=&limit=&active=&search=` — paginated list
- `GET /users/:id`
- `PATCH /users/:id` — partial update (`username`, `email`, `bio`, `is_active`)
- `DELETE /users/:id?hard=true` — soft delete by default, `hard=true` permanently deletes
- `POST /users/:id/restore` — undo a soft delete

### Posts

- `POST /posts` — `{ "title", "content", "user_id", "tags": ["..."] }`
- `GET /posts?published=&author_id=&tag=` — filterable list
- `GET /posts/:id` — includes author, tags, and threaded replies
- `POST /posts/transfer` — `{ "post_id", "new_user_id" }`, transactional ownership transfer

### Health

- `GET /health` → `{"status":"healthy"}` — liveness, always 200 once the process is up
- `GET /ready` → `{"status":"ready"}` or `503 {"status":"unready"}` — pings MariaDB

> Note: `/health` and `/ready` are unversioned, at the API root (`/api/health`, not `/api/v1/health`), and there is no handler at bare `/api` or `/api/v1` — hitting those paths directly returns 404 by design.

## Production hardening

- Graceful shutdown on SIGINT/SIGTERM: in-flight requests (including payment adjustments) are drained before the process exits, with a 20s deadline.
- Per-request timeout (15s) via `middleware.TimeoutMiddleware` so a stalled DB call can't hold a connection open indefinitely.
- CORS is locked to `ALLOWED_ORIGINS` (comma-separated), not wildcard.
- `config.RequireEnv()` fails fast at startup if `JWT_SECRET`/DB config is missing, instead of failing silently on the first request.
- SQL query logging drops to warn-only when `GIN_MODE=release`.

## Ops

- `Dockerfile` — multi-stage build, static binary on Alpine (for containerized hosts; the current production deploy runs natively, see below)
- `.gitlab-ci.yml` — test (`go vet` + `go test -race`) → build → Docker image push
- `docker-compose.prod.yml` — API + MariaDB + InfluxDB, for a Docker-based single-host deploy

### Current production deploy (billfold.ddns.net)

Running natively on the host (no Docker) as a launchd service:

- Binary built with `go build -o billfold-api .`, run via `~/Library/LaunchAgents/com.billfold.api.plist` (`RunAtLoad` + `KeepAlive`, auto-restarts on crash)
- MariaDB via Homebrew, local `billfold` database, dedicated `billfold_app` user
- nginx (Homebrew, `/opt/homebrew/etc/nginx/servers/billfold.conf`) reverse-proxies `billfold.ddns.net/api/*` → `127.0.0.1:8080`, HTTP→HTTPS redirect
- TLS via Let's Encrypt, issued with the webroot method (`certbot certonly --webroot -w /opt/homebrew/var/www/certbot -d billfold.ddns.net`), matching how sibling sites on this host are configured

To redeploy after code changes:

```bash
go build -o billfold-api .
launchctl kickstart -k gui/$(id -u)/com.billfold.api
```

InfluxDB and Sentry are not yet wired in production — see below.

## Known gaps

- **InfluxDB metrics**: the client library (`influxdata/influxdb-client-go/v2`) targets the v2 HTTP API; the InfluxDB installed on the production host is v3 (different wire protocol). `INFLUX_URL` is left unset, so `config.RecordRequestLatency` no-ops safely. Needs either a v2-compatible client swap or a v3-installed instance to activate.
- **Sentry**: `SENTRY_DSN` is unset in production; error tracking is inactive until a DSN is provided.
