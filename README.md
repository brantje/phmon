# ByteMonitor

A self-hosted phBot monitoring and remote-control project. **Slice 0 only:** a Go
backend, PostgreSQL, and Nuxt + Nuxt UI development foundation. No agents, game data
or remote commands are implemented. [AGENTS.md](AGENTS.md) is the canonical guide
and complete Slice 0–15 roadmap. The target is applicable feature, layout and style
parity with [the phMonitor demo](https://phmonitor.com/demo). The next implementation
slice is **Slice 1 — Agent registration and connectivity**. When asked to implement
AGENTS.md, continue through the remaining roadmap under its autonomous execution
contract; a documentation-only request does not start implementation.

## Start the local stack

Requires Docker Engine and Docker Compose v2+ (with `--wait` support).

```sh
cp .env.example .env
docker compose up --build -d --wait --wait-timeout 180
```

Open **http://127.0.0.1:3005**. “All systems ready” means Nuxt successfully called
Go and Go successfully pinged PostgreSQL. “Check again” makes a fresh request.

```sh
docker compose ps
docker compose logs -f
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8081/readyz
curl -fsS http://127.0.0.1:3005/api/health
python3 scripts/smoke.py
docker compose down
```

`down` preserves the named database volume. `docker compose down -v` **deletes this
project's local database**. Compose runs built images; run `up --build` after source
changes, or use the host workflow below for frontend hot reload. Dependencies and
container images require internet access during initial install/build; the running
stack has no external-service dependency. UI fonts are system fonts.

## Host development with frontend hot reload

Use Node **24.20.0** (`nvm use`), Go **1.27.1**, npm, Docker Compose, and Python 3 for
the optional stack smoke test. Go's automatic toolchain download can satisfy the
module's version on older Go 1.21+ installations. Do not use the host's Node 20.

```sh
cp .env.example .env  # first setup only; preserve your existing settings
nvm use
npm --prefix web ci
docker compose up -d --wait postgres
```

If the complete Compose stack is already running, first stop its application
services (`docker compose stop web server`) to free the ports. In one terminal:

```sh
set -a
. ./.env
set +a
cd server
go run ./cmd/server
```

In another terminal, from the repository root:

```sh
nvm use
npm --prefix web run dev
```

Go reads the process environment; it does not load `.env` itself. Nuxt dev explicitly
loads the root `.env`. Built Nuxt must receive `NUXT_BACKEND_URL` in its process
environment (Compose does this). Host Go needs a restart after edits.

## Configuration

All examples are **local development only**. Ports bind to loopback. The defaults
avoid common 3000/8080/5432 conflicts. This foundation is not a public deployment:
TLS, user auth, and product authorization are not implemented.

| Variable | Default/example | Purpose |
| --- | --- | --- |
| `POSTGRES_USER` | `phmon` | Initial local database user |
| `POSTGRES_PASSWORD` | `phmon_local_only` | Required by Compose; local-only password |
| `POSTGRES_DB` | `phmon` | Initial database name |
| `POSTGRES_PORT` | `5435` | Host port for Compose PostgreSQL |
| `SERVER_PORT` | `8081` | Host port for Compose Go |
| `WEB_PORT` | `3005` | Host port for Compose Nuxt |
| `HTTP_ADDR` | `127.0.0.1:8081` | Host Go listener; Compose uses `0.0.0.0:8081` |
| `DATABASE_URL` | See `.env.example` | Required host Go PostgreSQL URL |
| `NUXT_BACKEND_URL` | `http://127.0.0.1:8081` | Private Nuxt server URL; Compose uses `http://server:8081` |
| `TEST_DATABASE_URL` | Unset | Enables real PostgreSQL Go integration test |
| `SMOKE_BACKEND_URL` / `SMOKE_WEB_URL` | Local defaults above | Smoke-test target overrides |
| `EXPECT_UNAVAILABLE` | Unset | Set `1` for database-outage smoke test |

When changing ports, update the corresponding host URL too; Compose internal ports
stay fixed. Change the Nuxt host dev port with `npm --prefix web run dev -- --port
YOUR_PORT`. For local simplicity, use URL-safe alphanumeric/underscore database
credentials: Compose constructs its internal database URL from these values. Custom
host URLs must percent-encode special characters. PostgreSQL initialization values
apply only to a new volume; changing `.env` does not change existing database users.
Never commit `.env` or use these example credentials in production.

## Validation

```sh
nvm use
npm --prefix web ci
bash scripts/check.sh
# Include a real database check (after starting PostgreSQL):
TEST_DATABASE_URL='postgres://phmon:phmon_local_only@127.0.0.1:5435/phmon?sslmode=disable' bash scripts/check.sh
# Format source:
(cd server && gofmt -w .)
npm --prefix web run format
```

`check.sh` runs Go formatting verification, vet, race-enabled tests and build;
frontend Prettier, ESLint, type checking and production build; and Compose config
validation. Without `TEST_DATABASE_URL`, the Go database integration test explicitly
skips; unit/API tests still run. Frontend behavior is tested through the real stack
smoke test, with no separate frontend unit framework for this skeleton.

With the full Compose stack running, verify outage and recovery:

```sh
python3 scripts/smoke.py
docker compose stop postgres
EXPECT_UNAVAILABLE=1 python3 scripts/smoke.py
docker compose up -d --wait --wait-timeout 120
python3 scripts/smoke.py
```

CI runs both validation and a complete Docker build/start/smoke/outage/recovery
sequence. A local pass is not a claim that hosted GitHub Actions has run.

## Architecture and references

Go uses standard-library HTTP handlers and a pgx pool; `/healthz` is liveness and
`/readyz` checks PostgreSQL with a bounded deadline. The process stays alive during
database outages so readiness can recover. The Nuxt server route is a thin,
same-origin health transport; it holds no domain policy or database connection.
See [the HTTP contract and protocol status](docs/protocol.md).

There are no database tables or migrations yet, and `plugin/` is documentation only.
Commands, WebSockets, identity, authentication, analytics and other product behavior
remain in their designated future slices.

Version/setup references: [Go releases](https://go.dev/dl/),
[Nuxt installation](https://nuxt.com/docs/4.x/getting-started/installation),
[Nuxt UI setup](https://ui.nuxt.com/docs/getting-started/installation/nuxt),
[Nuxt runtime configuration](https://nuxt.com/docs/4.x/guide/going-further/runtime-config),
and [PostgreSQL support](https://www.postgresql.org/support/versioning/).
