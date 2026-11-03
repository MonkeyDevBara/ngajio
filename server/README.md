# Ngajio server

One Go binary with SQLite (WAL). Only external dependency: `modernc.org/sqlite` (pure Go, no CGO).
Spec: `../docs/API.md`, `../docs/DECISIONS.md`.

## Build and test
```
cd server
make vet          # go vet ./...
make test         # go test -race ./...
make build        # linux/amd64 static binary -> bin/ngajio
make build-local  # native binary -> bin/ngajio-local
```

## Run locally with the dev login
```
make run-dev
# or:
ENV=dev DEV_LOGIN=1 LISTEN=127.0.0.1:8787 DB_PATH=/tmp/ngajio-dev/ngajio.db COOKIE_SECURE=false go run .
```
Try it:
```
curl -s localhost:8787/api/health
curl -s localhost:8787/api/config
curl -si -X POST localhost:8787/api/auth/dev \
  -H 'Content-Type: application/json' -H 'X-Ngajio: 1' -H 'Origin: http://localhost:8787' \
  -d '{"email":"ustaz@example.id"}'
```
Every POST needs `Content-Type: application/json`, `X-Ngajio: 1` and an `Origin` whose host equals `Host`
(or is listed in `ALLOWED_ORIGINS`), otherwise the answer is 403 `csrf`.

## Environment
| Variable | Default | Meaning |
|---|---|---|
| `ENV` | `prod` | `dev` or anything else. Dev-only switches need `dev`. |
| `LISTEN` | `127.0.0.1:8787` | Listen address. |
| `DB_PATH` | `/var/lib/ngajio/ngajio.db` | SQLite file. Created with mode 0600, its directory 0700. |
| `BACKUP_DIR` | `<dir of DB_PATH>/backup` | Backup folder (used by the backup job, later task). |
| `GOOGLE_CLIENT_ID` | empty | Returned by `/api/config`. When set, `POST /api/auth/google` verifies Google ID tokens; when empty it answers 503 `not_configured`. |
| `DEV_LOGIN` | off | `1` enables `POST /api/auth/dev`. The server refuses to start unless `ENV=dev` and `LISTEN` has a loopback host (`localhost`, `127.x`, `::1`). |
| `COOKIE_SECURE` | `true` | `false` is only allowed when `ENV=dev`. |
| `ALLOWED_ORIGINS` | empty | Comma-separated extra origins accepted by the CSRF check. |
| `TRUST_PROXY_HEADER` | `X-Real-IP` | Client IP header, trusted only from a loopback peer (per-IP rate limit on the auth endpoints: 10 per minute, burst 10). |

`.env.example` lists the same variables; the server reads real environment variables only (systemd `EnvironmentFile=`).

## Layout
`main.go` wiring and routes, `internal/config` env and guards, `internal/store` SQLite (`schema.sql`, migrations by `PRAGMA user_version`),
`internal/httpx` JSON, errors, middleware, client IP, `internal/auth` Google token verifier, sessions, logins and the auth middleware (`Require`, `UserID`), `internal/limit` in-memory token buckets.

The request log has only method, path, status and ms. Never log bodies, queries, cookies or personal data.
