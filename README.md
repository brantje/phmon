# PhMon

PhMon is a self-hosted monitoring and remote-control dashboard for phBot / Silkroad Online.
It consists of a Go backend, PostgreSQL, a Nuxt frontend and a phBot plugin. The project
is independently implementing the applicable monitoring/control feature set and general
information architecture of the phMonitor reference while keeping all data and control
inside the operator's own infrastructure.

> **Project status:** active development. `main` now contains substantial implementation
> through the Slice 7-9 map/mob/heatmap work, plus later map-control foundations. Some
> earlier slices still have runtime/capability gaps, so the authoritative implementation
> status remains [AGENTS.md](AGENTS.md), not the slice number alone.

Useful project references:

- [AGENTS.md](AGENTS.md) — canonical architecture, slice roadmap and completion ledger.
- [docs/phbot-capabilities.md](docs/phbot-capabilities.md) — verified phBot APIs/runtime limitations.
- [docs/protocol.md](docs/protocol.md) — agent/backend/live protocol contracts.
- [docs/reference-parity.md](docs/reference-parity.md) — reference UI/feature comparison evidence.

## Current feature set

Current `main` includes:

- **Agent management and connectivity** — provisioned agent credentials, automatic reconnect,
  concurrent phBot connections/profiles, plugin/phBot version reporting and capability-aware sessions.
- **Live character monitoring** — server-scoped character identity, online/session state, level,
  HP/MP, XP/SP, gold, zone/position, groups and character detail views.
- **Resources and character data** — inventory/equipment plus supported storage, guild storage,
  job pouch, pet, party and academy observations where the active phBot/runtime exposes them.
- **Remote commands** — audited, idempotent, session-fenced controls for supported bot, trace,
  training-area/radius, movement/navigation, return/disconnect/clientless and chat operations.
  Unsupported runtime primitives stay disabled instead of being guessed.
- **Multi-character control foundations** — shared character/group target selection, command fan-out
  and per-character results for supported actions.
- **Events and chat** — durable event ingestion for verified phBot events/state transitions, live/history
  surfaces, inbound chat, channel/contact views and capability-gated outbound chat.
- **Live map** — locally served exported game maps/assets, character markers, party-member markers,
  recent death/drop overlays, nearby monster observations, cave/floor-aware projection and live layer state.
- **Mob history and heatmaps** — bounded historical monster sampling, density/heatmap queries, facets,
  filters and scoped reset controls. Live nearby monsters remain separate from historical density.

The bundled phBot plugin is currently **1.9.23** and reports agent transport **protocol 16**.

## Roadmap

The README keeps this intentionally high level; [AGENTS.md](AGENTS.md) is the canonical source for
acceptance criteria, blockers and exact completion state.

| Slice | Area | README status |
| --- | --- | --- |
| 0-1 | Local stack, application shell, agent registration/connectivity | Core implemented |
| 2 | Character identity, groups and live stats | Core implemented; parity/data gaps tracked in `AGENTS.md` |
| 2.5 | Standalone game-data/asset exporter | Implemented; dataset/profile coverage continues |
| 3 | Remote commands | Core implemented with capability/session fencing |
| 4 | Inventory, equipment, pets, party and related resources | Substantial implementation; verified-runtime gaps remain |
| 5 | Canonical event pipeline | Core implemented |
| 6 | Chat | Core implemented |
| 7-9 | Live map, mob observations and historical heatmaps | Core implemented; map/world-control work continues |
| 10 | Conditions / automation | Planned |
| 11 | Scheduling | Planned |
| 12 | Analytics | Planned |
| 13 | Economy and item analytics | Planned |
| 14 | Hardening | Planned |
| 15 | Demo-parity completion and final acceptance | Planned |

### Active/open feature issues

The current issue backlog is concentrated around richer map/world interaction:

- [#24 — Show / use NPCs](https://github.com/brantje/phmon/issues/24)
- [#25 — Show and edit training areas](https://github.com/brantje/phmon/issues/25)
- [#27 — Right-click navigate](https://github.com/brantje/phmon/issues/27)
- [#28 — Multi-character/group map control and live world interactions](https://github.com/brantje/phmon/issues/28)
- [#31 — Inspect nearby shop goods on demand](https://github.com/brantje/phmon/issues/31)
- [#32 — Investigate teleporter destinations, recall point and execution](https://github.com/brantje/phmon/issues/32)
- [#33 — Multi-character teleporter/recall actions](https://github.com/brantje/phmon/issues/33)
- [#34 — Reverse Return character/group actions](https://github.com/brantje/phmon/issues/34)
- [#35 — Apply multi-character targeting to existing remote controls](https://github.com/brantje/phmon/issues/35)
- [#36 — Show other nearby players](https://github.com/brantje/phmon/issues/36)

Recent foundations for this program are already merged/closed: **#23 party members on the map**,
**#29 map action target selection**, and **#30 multi-character command fan-out/result summaries**.

## Install and run

Requirements: Docker Engine and Docker Compose v2+ with `--wait` support.

```sh
git clone https://github.com/brantje/phmon.git
cd phmon
cp .env.example .env

# Set OPERATOR_ACCESS_SECRET in .env to a unique high-entropy value.
# For trusted-LAN plain HTTP, also configure the insecure-HTTP/origin settings in .env.

docker compose up --build -d --wait --wait-timeout 180
```

Open **http://127.0.0.1:3005** locally, or `http://<host-LAN-IP>:3005` from another
device when the web binding/firewall permits it. The first build requires internet access for
container images and dependencies; the running stack does not depend on phMonitor services.

Useful checks:

```sh
docker compose ps
docker compose logs -f
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:3005/api/health
```

Stop the stack with `docker compose down`. The named PostgreSQL volume is preserved;
`docker compose down -v` deletes the local PhMon database.

## Add a phBot agent

1. In PhMon open **Settings -> Agents** and choose **Create credential**. Save the returned
   agent ID and token immediately; the plaintext token is only shown in that response.
2. Copy [`plugin/PhMon.py`](plugin/PhMon.py) into phBot's `Plugins` directory and reload the plugin.
3. In phBot open **Plugins -> PhMon**, enter the backend WebSocket URL, agent ID and token, then
   choose **Save & Connect**.
4. The agent should appear online in **Settings -> Agents** after the authenticated hello succeeds.

For a same-host development setup the backend agent endpoint is normally
`ws://127.0.0.1:8081/agent`. For phBot running on another machine, put TLS/reverse-proxy
termination in front of the backend and use a reachable `wss://.../agent` endpoint; do not expose
cleartext bearer credentials to an untrusted network.

Credentials can also be created from the CLI:

```sh
docker compose exec server phmonctl agent create
```

A credential represents a logical PhMon agent and may intentionally be reused by multiple
phBot profiles/connections that should belong to that same logical agent. Use separate credentials
when they should appear as separate agents. Per-profile plugin configuration is stored under
`Config/PhMon/` without modifying phBot's own player JSON. See [plugin/README.md](plugin/README.md).

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
environment (Compose does this). That value is private relay configuration: browsers
connect to same-origin `/api/live` and never receive the backend address. Host Go
needs a restart after edits.

Any reverse proxy in front of Nuxt must allow WebSocket upgrade traffic on
`/api/live`. Do not route browser `/api/live` directly to Go; Nuxt is the
same-origin relay and the Go listener stays on the trusted/private backend boundary.

## Configuration

All examples are **local development only**. The web UI is reachable on the LAN by
default; PostgreSQL and the authenticated agent/API port bind to loopback. Set
`WEB_BIND_ADDR=127.0.0.1` when host-only web access is sufficient. Remote agents
should connect through operator-managed TLS termination using `wss://`; overriding
`SERVER_BIND_ADDR` is intended only for an explicitly trusted development network.
The defaults avoid common 3000/8080/5432 conflicts. This setup is for a trusted
development/LAN network. Browser monitoring, mutations and credential creation
require the operator session. Keep the UI on a trusted network and use operator-managed
TLS for remote agents; the configured secret is not a substitute for a protected
network deployment.

| Variable                              | Default/example         | Purpose                                                                                          |
| ------------------------------------- | ----------------------- | ------------------------------------------------------------------------------------------------ |
| `POSTGRES_USER`                       | `phmon`                 | Initial local database user                                                                      |
| `POSTGRES_PASSWORD`                   | `phmon_local_only`      | Required by Compose; local-only password                                                         |
| `POSTGRES_DB`                         | `phmon`                 | Initial database name                                                                            |
| `POSTGRES_PORT`                       | `5435`                  | Host port for Compose PostgreSQL                                                                 |
| `SERVER_PORT`                         | `8081`                  | Host port for Compose Go                                                                         |
| `SERVER_BIND_ADDR`                    | `127.0.0.1`             | Host address for the agent/API port; override only for an explicitly trusted development network |
| `WEB_PORT`                            | `3005`                  | Host port for Compose Nuxt                                                                       |
| `WEB_BIND_ADDR`                       | `0.0.0.0`               | Host address for the Compose web UI; use `127.0.0.1` for host-only access                        |
| `HTTP_ADDR`                           | `127.0.0.1:8081`        | Host Go listener; Compose uses `0.0.0.0:8081`                                                    |
| `DATABASE_URL`                        | See `.env.example`      | Required host Go PostgreSQL URL                                                                  |
| `NUXT_BACKEND_URL`                    | `http://127.0.0.1:8081` | Private Nuxt relay upstream; Compose uses `http://server:8081`; never exposed to browsers        |
| `NUXT_PUBLIC_INSTANCE_URL`            | Unset                   | Optional reachable browser-facing origin for the mobile QR/copy panel                            |
| `TEST_DATABASE_URL`                   | Unset                   | Enables real PostgreSQL Go integration test                                                      |
| `SMOKE_BACKEND_URL` / `SMOKE_WEB_URL` | Local defaults above    | Smoke-test target overrides                                                                      |
| `EXPECT_UNAVAILABLE`                  | Unset                   | Set `1` for database-outage smoke test                                                           |

When changing ports, update the corresponding host URL too; Compose internal ports
stay fixed. Change the Nuxt host dev port with `npm --prefix web run dev -- --port
YOUR_PORT`. For local simplicity, use URL-safe alphanumeric/underscore database
credentials: Compose constructs its internal database URL from these values. Custom
host URLs must percent-encode special characters. PostgreSQL initialization values
apply only to a new volume; changing `.env` does not change existing database users.
Never commit `.env` or use these example credentials in production. If PhMon is
usually opened as `http://localhost:3005` but the mobile QR must work from another
device, set `NUXT_PUBLIC_INSTANCE_URL` to the reachable HTTPS or LAN origin. Loopback
origins are detected and are not offered as mobile QR targets.

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

`check.sh` runs Go formatting verification, vet, race-enabled tests and both Go
binaries; the stdlib-only phBot transport tests; the source-level WebSocket-only live
transport audit; frontend Prettier, ESLint, type checking and production build; and
Compose config validation. Without
`TEST_DATABASE_URL`, PostgreSQL integration tests explicitly skip; unit/API/protocol
tests still run. CI additionally provisions an agent credential through the Nuxt web
endpoint and proves connect → backend restart → automatic reconnect → disconnect
through the real Go/PostgreSQL/Nuxt stack. `scripts/live_smoke.py` exercises the
same-origin Nuxt WebSocket relay, initial snapshots, manual refresh, subscription
filter/revision handling, character detail when available, cross-client group changes
and cross-origin rejection without using HTTP live reads.

`scripts/command_smoke.py` logs in using `OPERATOR_ACCESS_SECRET`, provisions a
throwaway agent, then submits `bot.stop` to the production plugin worker running with
a fake adapter. It verifies one callback invocation and the authoritative result on
the same-origin `/api/live` stream. It creates fixture records and must run only on a
disposable local/test database; it never connects to phBot.

`scripts/remote_controls_smoke.py` runs the Issue #35 multi-character flow on the
same disposable stack. It uses three production plugin workers with fake bot,
trace, return, disconnect and training adapters to verify independent results,
execution-time positions, named-area/radius readback and unsupported Clientless
capability. It never invokes a real phBot API.

With the full Compose stack running, verify outage and recovery:

```sh
python3 scripts/smoke.py
python3 scripts/live_smoke.py
docker compose stop postgres
EXPECT_UNAVAILABLE=1 python3 scripts/smoke.py
docker compose up -d --wait --wait-timeout 120
python3 scripts/smoke.py
```

CI runs validation plus complete Docker build/start, authenticated agent lifecycle,
browser-facing live WebSocket coverage, database outage and recovery sequences. Live
browser traffic must not perform GET reads against the diagnostic agent, character or
group endpoints during startup, filtering, refresh, actions or recovery. CI includes frame-safe transport regression coverage in addition to the stack and protocol checks. For
manual development without phBot, the same plugin transport can be exercised with
`scripts/agent_simulator.py`; simulator success is fixture coverage and is never
reported as real phBot runtime validation.

Real phBot runtime validation is tracked separately from simulator/fixture coverage.
Runtime-gated or server-specific features must remain marked unsupported/unverified until
they are exercised against the active phBot/private-server combination.

To run its explicit fixture character lifecycle scenario against a local test stack:

```sh
PHMON_AGENT_URL=ws://127.0.0.1:8081/agent \
PHMON_AGENT_ID='<agent-id>' PHMON_AGENT_TOKEN='<agent-token>' \
PHMON_SIMULATOR_SCENARIO=character-lifecycle \
python3 scripts/agent_simulator.py
```

This creates clearly named `FixtureAlpha`/`FixtureBeta` records. Run this only
against a development/test database; fixture data is not a production monitoring
source. `python3 scripts/character_smoke.py` verifies the switched state through the
retained diagnostic HTTP API; `python3 scripts/live_smoke.py` verifies the browser
WebSocket path.

## Architecture and references

The Go service is the backend authority for authentication, agent sessions, character identity,
commands, resource observations, events and historical map analytics. PostgreSQL stores durable
state and history. The phBot plugin uses a versioned authenticated `/agent` WebSocket transport
(currently protocol 7) and keeps backend I/O off phBot callbacks by dispatching through its worker/
event-loop boundary.

One logical agent may have multiple authenticated phBot connections. Character/session ownership
is generation-fenced so an old or disconnected socket cannot overwrite a newer live session.
Current resource/map observations use replacement/freshness semantics; historical events and
heatmap samples are stored separately where the relevant slice explicitly requires durability.

Nuxt is the browser-facing boundary. Live browser state uses one same-origin `/api/live` WebSocket
relayed by Nuxt to Go; normal HTTP remains for authentication, configuration/mutations, readiness
and historical queries. Reverse proxies must allow WebSocket upgrades on `/api/live`.

Remote actions use a bounded command catalog with operator authentication, idempotency, current-
session fencing, capability checks, durable audit/results and explicit confirmation for
consequential actions. Multi-character operations fan out into ordinary per-character commands
instead of creating a separate group execution identity.

See [docs/protocol.md](docs/protocol.md), [docs/phbot-capabilities.md](docs/phbot-capabilities.md)
and [AGENTS.md](AGENTS.md) for the detailed contracts and remaining capability gaps.

## Operator authentication

PhMon uses a separate operator control-plane session. Set a high-entropy
`OPERATOR_ACCESS_SECRET`, list browser origins in `OPERATOR_ALLOWED_ORIGINS`, and
keep the default `phmon_operator` cookie name unless the matching Nuxt private
runtime setting is changed too. Plain HTTP origins are rejected by default. For an
isolated trusted LAN without TLS, set `OPERATOR_ALLOW_INSECURE_HTTP=true` and list
the exact LAN origin (for example `http://192.168.10.25:3005`). This sends the
operator session cookie without encryption; use only on a trusted network. HTTPS
origins always receive a Secure cookie.

The browser sends the access secret only to the same-origin login endpoint. Go stores
only a hash of the opaque eight-hour session token in bounded process memory, so a
backend restart requires sign-in again. The session cookie is HttpOnly and
SameSite=Strict; HTTPS origins always receive a Secure cookie. Agent bearer
credentials are a separate trust boundary and cannot authenticate operator APIs.
