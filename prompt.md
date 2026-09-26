ou are working on a new self-hosted phBot monitoring and remote-control project.

Your task is to:

Inspect the repository and existing files first.

Create a canonical AGENTS.md at the repository root.

Put the complete architecture, engineering rules, implementation slices, dependencies, acceptance criteria, and project conventions into AGENTS.md.

Then immediately begin implementing Slice 0 only.

Do not start Slice 1 or later slices unless Slice 0 is completely finished and explicitly requested afterward.

Do not merely create a plan. After creating AGENTS.md, implement Slice 0.

Product objective

Build a completely self-hosted alternative to phMonitor for phBot.

Do not depend on phMonitor's backend, protocol, premium entitlement system, client implementation, or infrastructure.

Do not reverse engineer or bypass phMonitor's paid-access controls.

phMonitor may be used only as a product/feature reference.

The system should obtain its data directly from phBot using a custom phBot Python plugin.

The final architecture is:

phBot
  |
  | custom Python plugin
  | outbound HTTPS / WebSocket
  v
Go backend
  |
  +-- PostgreSQL
  +-- realtime connection management
  +-- command routing
  +-- event storage
  +-- analytics
  +-- heatmap aggregation
  +-- conditions / automation
  +-- scheduling
  |
  v
Nuxt + Nuxt UI frontend

There is intentionally no separate local Go/Windows agent.

The phBot plugin talks directly to the backend.

We are explicitly dropping game-client video/live-stream capture.

The "live" functionality we do want is live character state and a live game map based on phBot data.

Core architectural rule

This rule is non-negotiable:

The phBot plugin reports facts and executes commands.

The Go backend owns:
- policy
- history
- persistence
- aggregation
- analytics
- conditions
- scheduling
- authorization
- command lifecycle

Keep the phBot plugin deliberately thin.

Do not move business logic into the plugin merely because it is convenient.

The plugin should primarily:

collect
normalize
publish
receive commands
execute commands
report results

The plugin should not own:

analytics
heatmaps
historical calculations
conditions/rules
schedules
authorization policy
persistent business state

Apply DRY and YAGNI throughout the project.

Prefer the smallest maintainable design that supports the current slice.

Do not prematurely build infrastructure for later slices.

Repository structure

Unless the existing repository already has a better compatible structure, prefer:

/
├── AGENTS.md
├── plugin/
│   └── ByteMonitor.py
├── server/
│   ├── cmd/
│   └── internal/
├── web/
├── docker-compose.yml
└── .env.example

Names may be adapted if the repository already establishes conventions.

Do not restructure working code unnecessarily.

Technology choices

Backend:

Go
PostgreSQL
WebSocket
REST where appropriate

Frontend:

Nuxt
Nuxt UI
TypeScript

phBot integration:

Python phBot plugin

Do not introduce Redis unless a later demonstrated requirement makes it necessary.

Do not introduce Kafka, RabbitMQ, NATS, Kubernetes, microservices, CQRS, event sourcing, or other infrastructure without an actual requirement.

Start simple.

Connection model

Each running phBot plugin establishes its own outbound TLS connection to the backend.

Example:

phBot Alice ───────┐
phBot Bob ─────────┤
phBot Charlie ─────┼──> wss://monitor.example.com/agent
phBot Dave ────────┤
phBot Eve ─────────┘

There is no local broker.

Each plugin represents one active phBot instance.

The backend maps active agent/character identities to their current WebSocket connection.

Networking rule

Never perform slow blocking network operations directly inside latency-sensitive phBot callbacks.

Prefer:

phBot callback
    |
    v
in-memory outbound queue
    |
    v
network worker
    |
    v
backend connection

Callbacks should collect/queue state or events and return quickly.

Incoming commands should also pass through a controlled dispatch mechanism instead of arbitrary networking-thread execution.

Reconnection rule

The plugin must eventually tolerate complete backend outages.

Expected lifecycle:

CONNECTED
   |
backend unavailable
   v
DISCONNECTED
   |
retry with backoff
   v
CONNECTED
   |
send hello
send current/full state where required
resume normal event flow

Transient high-frequency observations may eventually be allowed to drop while disconnected.

Do not introduce durable local SQLite/event storage until a demonstrated requirement exists.

Command model

Commands must eventually use stable IDs.

Conceptually:

{
  "type": "command",
  "id": "cmd_...",
  "command": "bot.start",
  "expires_at": "..."
}

Plugin response:

{
  "type": "command.result",
  "id": "cmd_...",
  "status": "success"
}

The system must eventually distinguish:

queued
sent
acknowledged
completed
failed
expired

A successful WebSocket write must never be treated as equivalent to successful execution.

Commands that become unsafe or meaningless when delayed must support expiry.

Do not fully implement this until the command slice, but preserve the architecture so it remains possible.

Snapshot vs event model

Do not treat every piece of information as a periodically repeated full snapshot.

Use:

snapshots for current state
events for things that happened

Examples:

Snapshot/state:

HP
MP
XP
SP
gold
position
botting state
inventory
party
pets
training state

Events:

death
drop
unique spawn
teleport
level up
chat
alchemy result
disconnect/reconnect

Do not persist enormous repeated snapshots when a delta/event is sufficient.

Map and heatmap philosophy

Heatmaps are a first-class feature.

The plugin should report observations.

The backend should calculate density and historical aggregation.

For example:

plugin:
"I observed these monsters around this location at time T."

backend:
"Across observed samples, this spatial cell has average density X."

Do not calculate final heatmaps in the plugin.

Avoid naïve algorithms such as incrementing density once per second forever when a character stands still.

Density should eventually account for observation/sample count.

Useful future map layers include:

live characters
nearby monsters
drops
deaths
mob density
mob types
unique sightings
player movement
XP gain
gold/items gathered

Development methodology

Implement the project in vertical slices.

Each slice should leave an end-to-end piece of functionality working.

Do not implement all backend infrastructure first and postpone frontend/plugin integration indefinitely.

For every slice:

inspect existing architecture

define the smallest required domain/API changes

implement backend

implement plugin changes if required

implement frontend changes if required

add focused tests

update documentation

run relevant validation

report exactly what was completed

Do not silently expand the scope.

Slice plan

The following slice roadmap must be copied into AGENTS.md and treated as the canonical implementation plan.

Slice 0 — Skeleton and local development

Objective:

Establish the repository and development foundation without implementing product behavior yet.

Implement:

repository/application structure

Go backend skeleton

Nuxt + Nuxt UI frontend skeleton

PostgreSQL local service

Docker Compose

environment/config handling

.env.example

backend health endpoint

frontend connectivity to backend health endpoint

basic CI

formatting/lint/test commands

protocol/version placeholder/documentation where useful

Expected local stack:

Go server
PostgreSQL
Nuxt frontend

Acceptance criteria:

entire development stack starts locally

backend can connect to PostgreSQL

backend exposes a health/readiness endpoint

Nuxt can successfully call the backend

CI runs basic backend/frontend validation

configuration is documented

no Slice 1 product functionality is implemented yet

This is the slice to implement now.

Slice 1 — Agent registration and connectivity

Objective:

Connect phBot instances reliably to the backend.

Implement:

initial phBot Python plugin

outbound WebSocket connection

agent authentication token

stable agent_id

hello handshake

heartbeat

reconnect/backoff

backend active-agent registry

connected/disconnected status

last-seen timestamp

plugin version

phBot version

Nuxt page showing agents live

Acceptance criteria:

starting a phBot instance causes it to appear in the UI

stopping/disconnecting it updates the UI

reconnect works without manual intervention

agent authentication is enforced

Slice 2 — Character identity and core live stats

Objective:

Provide the first genuinely useful monitoring dashboard.

Implement:

joined-game detection

server identity

character identity

stable backend character record

current character state

level

HP/MP

XP/SP

gold

current position

botting/training state

full snapshot after join/reconnect

Nuxt character overview/dashboard

Acceptance criteria:

character appears after joining the game

current statistics update live

reconnect restores correct current state

character identity does not depend solely on an ephemeral socket connection

Slice 3 — Remote commands

Objective:

Safely control core phBot actions remotely.

Implement:

server-to-plugin command protocol

command IDs

acknowledgements

command results

command expiry

audit/history persistence

initial actions:

start bot

stop bot

disconnect

return scroll

Nuxt controls with pending/success/failure states

Acceptance criteria:

commands reach only the intended connected agent

frontend distinguishes sent from successfully executed

expired commands are not replayed unexpectedly

command history is inspectable

Slice 4 — Inventory, pets and party

Objective:

Expose important operational game state.

Implement:

character inventory

storage where cleanly available

pets

pet inventory

party members/state

delta/change handling where appropriate

Nuxt inventory view

Nuxt pet view

Nuxt party view

Acceptance criteria:

current inventory is visible

pet state/inventory is visible

party membership is visible

updates do not require blindly resending excessive full state when unnecessary

Slice 5 — Event pipeline

Objective:

Move from current-state monitoring to durable activity history.

Canonical events should include where available:

death

item drop

unique spawn

teleport

level-up

disconnect

reconnect

alchemy result

Implement:

event envelope/schema

plugin event publishing

nonblocking outbound queue

durable backend event storage

Nuxt activity timeline

basic event filtering

Acceptance criteria:

events survive page reload/backend querying

timeline ordering is reliable

event ingestion does not block normal phBot behavior

Slice 6 — Chat

Objective:

Provide remote chat visibility and sending.

Implement inbound chat where supported:

private

party

guild

union

general

other useful supported channels

Implement remote sending where supported.

Add:

persistent/appropriate history

per-character chat UI

command/result handling for outbound messages where necessary

Acceptance criteria:

incoming messages appear in the web UI

supported outbound chat can be sent remotely

messages are attributed to the correct character/channel

Slice 7 — Live map

Objective:

Show actual live game-world state without video capture.

Implement:

canonical coordinate model

character current position

position history where useful

map/region normalization

current character markers

optional nearby-monster overlay

optional nearby-drop overlay

Nuxt map component

legally usable/private map assets

Acceptance criteria:

live character position is visible on a map

movement updates correctly

region transitions are handled

map architecture supports future heatmap layers

Slice 8 — Mob observation and heatmap foundation

Objective:

Collect statistically meaningful mob-density data.

Plugin publishes compact observation batches containing:

observer character

timestamp

region

character/observer coordinates

nearby monster identity/model/type

nearby monster coordinates

Backend implements spatial aggregation.

Do not use naïve cumulative sightings alone.

Track enough information to derive values such as:

observation_samples
mob_observations
unique/identified mob observations where useful
mob types

Conceptual density:

density = mob observations / observation samples

Exact spatial model may evolve during implementation.

Acceptance criteria:

observations from multiple characters can contribute

standing still for a long period does not incorrectly create arbitrarily hot cells merely because time passed

backend can query spatial mob density by area/time range

Slice 9 — Heatmaps

Objective:

Expose the accumulated spatial analytics in the UI.

Implement heatmap rendering and filters.

Initial layers:

mob density

mob types

deaths

drops

unique sightings

player movement

Useful filters:

time range

region

mob type

character

server

Add backend pre-aggregation where justified for larger ranges.

Acceptance criteria:

heatmaps render from backend data

time filtering works

layers can be enabled/disabled

performance remains reasonable for accumulated historical data

This slice marks the target for the first complete phMonitor-replacement MVP.

Slice 10 — Conditions and automation

Objective:

Build a generic server-owned rules engine.

Concept:

WHEN conditions
THEN actions

Possible initial inputs:

HP/MP thresholds

disconnected

inventory full

bot stopped

death

unique seen

item dropped

Possible actions:

notification

Discord webhook

phBot command

Keep rule evaluation on the backend.

Do not implement condition logic inside individual plugins.

Acceptance criteria:

rules are persisted

rules evaluate deterministically

triggered actions are auditable

an unlimited number of self-hosted rules can be created subject only to practical resource limits

Slice 11 — Scheduling

Objective:

Support server-owned scheduled actions.

Initial actions may include:

start bot

stop bot

return scroll

disconnect

Implement:

persisted schedules

scheduler execution

command integration

missed-run semantics

expiration behavior

Nuxt schedule/calendar editor

Acceptance criteria:

schedules survive backend restart

commands execute against the intended agent/character

missed/late commands follow documented semantics

Slice 12 — Analytics

Objective:

Convert monitoring history into useful performance data.

Potential metrics:

XP/hour

SP/hour

gold/hour

deaths/hour

drops/hour

session duration

bot uptime

farming-area comparison

daily/weekly summaries

Implement historical charts in Nuxt.

Acceptance criteria:

metrics are derived from durable backend data

time-range queries work

calculations are documented/tested

dashboard remains usable over meaningful historical ranges

Slice 13 — Economy and item analytics

Objective:

Add item-centric historical/search functionality when the available phBot data supports it.

Potential functionality:

item acquisition history

valuable drop tracking

search across observed items

economy/stall information where available

price history where sufficiently reliable source data exists

Do not invent data that phBot cannot provide.

Do not overbuild this slice before confirming actual source capabilities.

Acceptance criteria should be defined based on the verified API/data available at implementation time.

Slice 14 — Hardening

Objective:

Make the system safe and maintainable for long-running self-hosted use.

Implement as justified:

token rotation

token revocation

user authentication/authorization

command authorization

rate limiting

schema migrations

protocol version negotiation

plugin compatibility handling

queue bounds

backpressure

observability

retention rules

high-volume map observation cleanup/aggregation

deployment documentation

upgrade procedures

Acceptance criteria:

operational failures are observable

high-volume data has bounded storage behavior

agent credentials can be revoked/rotated

version incompatibilities fail clearly

system can be upgraded predictably

Milestones

Treat these milestones as informational:

Slices 0-4
= first usable monitoring/control system

Slices 0-9
= real phMonitor replacement MVP

Slices 10-14
= advanced self-hosted platform

Do not jump ahead just because a later feature seems interesting.

Dependency chain

Preserve this broad dependency order:

foundation
   |
connectivity
   |
character state
   |
commands
   |
events
   |
map observations
   |
heatmaps
   |
conditions / scheduling / analytics

Later slices may reuse earlier abstractions.

Avoid introducing abstractions before at least one concrete use requires them.

Data ownership and boundaries

Backend is authoritative for:

agents
characters
command history
events
historical state
map observations
heatmap aggregates
rules
schedules
analytics
authentication/authorization

Plugin is authoritative only for what is currently happening inside its own phBot process and game session.

The plugin reports observations; it is not the long-term source of truth.

Security baseline

Even during early development:

never hardcode production credentials

use environment variables/secrets appropriately

do not log authentication tokens

bind database access appropriately

validate external input

authenticate agent connections once Slice 1 is reached

ensure one connected agent cannot impersonate another

eventually use TLS in real deployment

Do not implement an elaborate IAM system during Slice 0.

Testing philosophy

Tests should focus on behavior and boundaries.

Prefer:

Go unit tests for meaningful domain logic

HTTP/API tests

repository/database tests where valuable

frontend component/unit tests for meaningful behavior

protocol tests when the protocol exists

Avoid tests that merely duplicate implementation details.

Do not introduce huge testing frameworks solely to satisfy a coverage percentage.

Documentation rule

AGENTS.md is the canonical implementation/architecture guide.

When a later implementation decision materially changes the architecture or slice plan, update AGENTS.md.

Do not allow documentation to describe obsolete behavior.

For each completed slice, record:

status
major implementation decisions
important deviations from the original plan
known limitations
follow-up work deliberately deferred

Git/workflow expectations

Before modifying anything:

inspect current repository status

inspect existing conventions

inspect relevant existing files

avoid overwriting unrelated work

Keep changes scoped to the active slice.

Do not combine speculative future work with the current slice.

If git is configured and the environment expects commits, make logically grouped commits with clear messages.

Do not merge branches or PRs unless explicitly instructed.

Your immediate task

Perform the following now:

Step 1

Inspect the repository thoroughly enough to understand:

existing files

current stack

current conventions

whether any of the proposed structure already exists

package/tooling choices already made

Do not assume the repository is empty.

Step 2

Create or update root AGENTS.md.

It must contain:

product objective

architecture

core plugin/backend responsibility rule

networking/reconnect principles

command principles

snapshot/event principles

map/heatmap principles

engineering conventions

all Slices 0 through 14 above

milestones

dependency chain

testing expectations

documentation expectations

current implementation status

Make it useful to future coding agents, not merely a copy of this prompt.

Where repository reality differs from this proposed structure, document the actual chosen structure and why.

Step 3

Implement Slice 0 — Skeleton and local development.

Do not implement Slice 1.

Slice 0 must leave us with:

Go backend
PostgreSQL
Nuxt + Nuxt UI frontend
Docker Compose
environment configuration
health/readiness path
frontend -> backend connectivity
basic CI
documented local development commands

Use sensible current stable versions compatible with the existing repository/toolchain.

Keep the implementation minimal and maintainable.

Step 4

Validate Slice 0.

Run all relevant:

formatting

linting

tests

builds

Docker/config validation where available

Fix failures introduced by your changes.

Do not claim a command passed unless you actually ran it successfully.

Step 5

Update AGENTS.md to reflect the actual resulting repository and mark Slice 0 appropriately.

Do not mark future slices complete.

Step 6

Give a concise final report containing:

files/areas created or changed

important architecture decisions made

commands/tests run and results

any limitations

whether Slice 0 acceptance criteria are satisfied

the exact next slice, which must be Slice 1

Do not start Slice 1.
