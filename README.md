# PhMon

A self-hosted phBot monitoring and remote-control project. Slice 0 and the Slice 1
implementation are complete. The operator has manually verified real phBot → PhMon
plugin/backend connectivity; the phBot build, embedded Python version and data API
behavior are not recorded yet. Same-viewport reference comparison remains open.
Slice 2 adds server-scoped durable character identity, live state/sessions, groups
and character overview/detail surfaces. Remote commands remain in Slice 3.
[AGENTS.md](AGENTS.md) is the canonical Slice 0–15 roadmap. The target is applicable
feature, layout and style parity with [the phMonitor demo](https://phmonitor.com/demo).
The active implementation scope is Slice 2 only; see the completion ledger in
[AGENTS.md](AGENTS.md).

## Start the local stack

Requires Docker Engine and Docker Compose v2+ (with `--wait` support).

```sh
cp .env.example .env
docker compose up --build -d --wait --wait-timeout 180
```

Open **http://127.0.0.1:3005** on the host, or **http://<host-LAN-IP>:3005** from
another device on the same network. Find the host address with `hostname -I` on
Linux or `ipconfig` on Windows/macOS. If the page does not load, allow inbound TCP
port 3005 through the host firewall for your private LAN. The dashboard shows
character presence and current stats, with the agent connections and backend
readiness available below. It refreshes automatically and retains the last received
records with a stale indicator during temporary outages.

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

## Connect a phBot agent

Provision one stable identity/token pair for each phBot account/profile that should
appear as its own PhMon agent. The normal path is the dashboard's **Create credential**
action. It generates a new agent ID/token through Nuxt -> Go -> PostgreSQL and displays
the plaintext token only in that one response. Save it immediately.

The CLI remains available for headless/operator workflows:

```sh
docker compose exec server phmonctl agent create
```

Both paths use the same server-side credential generator and PostgreSQL stores only
the token's SHA-256 hash. Copy `plugin/PhMon.py` into phBot's Plugins directory and
reload the plugin. In the phBot **Plugins -> PhMon** tab, enter the backend WebSocket
URL, provisioned agent ID and token, then click **Save & Connect**.

PhMon stores these values separately for the active phBot player/profile under
`Config/PhMon/<active-profile>.cfg`. The active player configuration returned by
`get_config_path()` and the explicit profile name returned by `get_profile()` are
both part of the storage key. PhMon never edits phBot's own player JSON. This lets
multiple accounts/characters and multiple named profiles for the same character keep
distinct PhMon identities and credentials in one phBot installation.

The token is persisted locally because reconnects must be unattended, but after a
profile is loaded or saved the GUI token field is cleared instead of continuously
displaying the secret. Keep the phBot Config directory private. Leaving the token
field blank preserves the saved token only while the backend URL and agent ID are
unchanged; changing either identity field requires pasting the token again.

The agent should appear on the dashboard after the authenticated hello succeeds. The
plugin reconnects automatically after backend loss and never puts credentials in the
URL. The default Compose binding keeps port 8081 on loopback. For a phBot host on
another machine, terminate TLS in front of the Go backend and configure a reachable
`wss://` URL; do not expose cleartext bearer authentication to an untrusted network.
A deliberate trusted-LAN development setup may override `SERVER_BIND_ADDR`, but
`ws://` is development-only.
See [plugin/README.md](plugin/README.md) and
[docs/phbot-capabilities.md](docs/phbot-capabilities.md).

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

All examples are **local development only**. The web UI is reachable on the LAN by
default; PostgreSQL and the authenticated agent/API port bind to loopback. Set
`WEB_BIND_ADDR=127.0.0.1` when host-only web access is sufficient. Remote agents
should connect through operator-managed TLS termination using `wss://`; overriding
`SERVER_BIND_ADDR` is intended only for an explicitly trusted development network.
The defaults avoid common 3000/8080/5432 conflicts. This is not a public deployment:
user authentication is not implemented yet; only the agent WebSocket is
token-authenticated. In particular, any user who can reach the current web UI can use
the Create credential action, so keep the UI on a trusted network until user
authentication/authorization is implemented.

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
| `NUXT_BACKEND_URL`                    | `http://127.0.0.1:8081` | Private Nuxt server URL; Compose uses `http://server:8081`                                       |
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
binaries; the stdlib-only phBot transport tests; frontend Prettier, ESLint, type
checking and production build; and Compose config validation. Without
`TEST_DATABASE_URL`, PostgreSQL integration tests explicitly skip; unit/API/protocol
tests still run. CI additionally provisions an agent credential through the Nuxt web
endpoint and proves connect → backend restart → automatic reconnect → disconnect
through the real Go/PostgreSQL/Nuxt stack.

With the full Compose stack running, verify outage and recovery:

```sh
python3 scripts/smoke.py
docker compose stop postgres
EXPECT_UNAVAILABLE=1 python3 scripts/smoke.py
docker compose up -d --wait --wait-timeout 120
python3 scripts/smoke.py
```

CI runs validation plus complete Docker build/start, authenticated agent lifecycle,
database outage and recovery sequences. The Slice 1 completion pass observed both
hosted jobs green after the frame-safe transport regression tests were added. For
manual development without phBot, the same plugin transport can be exercised with
`scripts/agent_simulator.py`; simulator success is fixture coverage and is never
reported as real phBot runtime validation.

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
same-origin API.

## Architecture and references

Go uses standard-library HTTP handlers, pgx and embedded transactional migrations.
PostgreSQL stores durable agents, server-scoped characters, current state, character
sessions, groups and membership. `/agent` is the authenticated protocol-v2 WebSocket.
One agent may keep multiple authenticated sockets active; each socket receives its
own connection generation and can only update the character sessions it owns. The
agent remains connected until its last socket closes.
Character identity is the case-folded character name within a case-folded server
name; it assumes Silkroad character names are unique per game server and does not
claim a universal game identifier. Startup ends stale sessions but retains identity
and last state. `GET /api/characters?q=&group_id=` searches name, guild, server and
zone; `/api/characters/{character_id}` is the stable detail route. Groups are
operator-managed metadata under `/api/groups` and never affect identity/routing.
`GET /api/agents` exposes safe presentation fields. Credential creation returns a
new token once with no-store semantics; existing tokens cannot be retrieved.

Nuxt keeps browser access same-origin through health, agent, character and group API
proxies plus the credential-creation endpoint.
`plugin/PhMon.py` uses only Python standard-library networking, performs no backend
I/O in phBot callbacks, and reconnects on a worker thread. It samples documented
`get_character_data()`, `get_position()` and `get_zone_name(region)` APIs; botting
state remains unknown because the official Botting API lists mutations but no
read-only state getter. See [docs/protocol.md](docs/protocol.md) and
[docs/phbot-capabilities.md](docs/phbot-capabilities.md). Remote commands, inventory,
events, analytics and later roadmap areas remain in their designated slices.

Version/setup references: [Go releases](https://go.dev/dl/),
[Nuxt installation](https://nuxt.com/docs/4.x/getting-started/installation),
[Nuxt UI setup](https://ui.nuxt.com/docs/getting-started/installation/nuxt),
[Nuxt runtime configuration](https://nuxt.com/docs/4.x/guide/going-further/runtime-config),
and [PostgreSQL support](https://www.postgresql.org/support/versioning/).
