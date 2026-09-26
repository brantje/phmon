# Agent protocol v2

Slice 1 introduced authenticated agent connectivity (v1). Slice 2 evolves that
contract to v2 and adds character identity registration, snapshots, state updates and
leave messages. Commands and game events remain outside the implemented message set.

## Transport and authentication

- Endpoint: GET /agent upgraded to WebSocket.
- Production deployments use wss:// with normal certificate validation. Plain ws://
  is for trusted local development only.
- Every upgrade request carries Authorization: Bearer <agent-token>.
- Tokens are created per logical agent through the dashboard or `phmonctl agent create`.
  Both use the same server-side generator/store path. PostgreSQL stores only a
  SHA-256 token hash. A token is permanently bound to one stable agent_id.
- Multiple concurrent phBot sockets/profiles may intentionally reuse the same
  agent ID/token when they belong to one logical agent. Use separate credentials
  when they should be separate logical agents; credentials are not per-socket.
- Tokens never belong in URLs, logs or QR codes. The credential-creation response is
  the sole browser-visible exception: it returns the newly generated plaintext token
  once with `Cache-Control: no-store`. Existing tokens are never retrievable.
- The server rejects a missing/invalid token before WebSocket upgrade.
- The first application message must be hello within 5 seconds. The agent_id in
  hello must match the identity authenticated by the bearer token.
- Application messages are JSON text frames and are limited to 8 KiB.

Protocol version: 2. Version 1 agents are rejected with an explicit unsupported
protocol close reason because the character identity/state contract is required.

## hello

Agent to server:

    {
      "type": "hello",
      "protocol_version": 2,
      "agent_id": "9d63c35b-1ff2-4d76-a17c-10514af2f664",
      "plugin_version": "1.1.0",
      "phbot_version": "21.1.9",
      "sent_at": "2026-09-26T14:00:00Z"
    }

plugin_version and phbot_version are bounded to 64 characters. sent_at is diagnostic
agent time; server-owned timestamps remain authoritative.

A valid hello authenticates the durable agent and updates first/last-seen and version
metadata. An agent may have multiple active WebSocket connections and observe
multiple characters at the same time. Each socket receives a distinct server-owned
connection generation; reconnecting or closing one socket must not cancel another
socket for the same agent. The agent is connected while any authenticated socket is
active.

Server to agent:

    {
      "type": "hello.ack",
      "protocol_version": 2,
      "heartbeat_interval_seconds": 10,
      "heartbeat_timeout_seconds": 30
    }

Unsupported protocol versions close with an explicit policy/unsupported-data reason.
Malformed or identity-mismatched hello messages are rejected.

## heartbeat

After hello acknowledgement, the plugin sends:

    {
      "type": "heartbeat",
      "protocol_version": 2,
      "sent_at": "2026-09-26T14:00:10Z"
    }

A valid heartbeat refreshes the server-owned last_seen_at value. If no valid
application message arrives for 30 seconds, the server closes that socket and ends
only character sessions owned by its connection generation. The agent is marked
disconnected only when its last authenticated socket closes. WebSocket ping/pong
control frames may exist at the transport layer but do not replace the application
heartbeat.

## Character identity and registration

Agent identity and character identity are separate UUIDs. Before the backend has
resolved a character, the plugin sends an authenticated registration request:

    {"type":"character.identify","protocol_version":2,"server":"Example Silkroad",
     "name":"CharacterName","guild":"Optional guild name","sent_at":"2026-09-26T14:00:11Z"}

The backend trims and case-folds server and name for a durable identity key, assuming
character names are unique within one game server. `guild` is optional: omitted means
guild was unavailable, while an explicit empty string means the plugin observed no
current guild and clears saved guild metadata. It does not use agent ID,
connection generation, guild, or undocumented player/account ID stability. This is
the only character-related request before the backend issues an ID and establishes a
live session claim for that character on this connection generation. The backend
responds:

    {"type":"character.registered","protocol_version":2,"character_id":"<uuid>"}

The character UUID is stable across agents, reconnects and backend restarts. Every
subsequent character-scoped message MUST include `character_id`; the server never
routes state from connection identity alone. An agent can register multiple game
characters over time or multiplex several by sending explicit IDs.

## Character snapshots, updates and presence

After identification on join or reconnect, the plugin sends the complete available
snapshot. Identification establishes character-level authority; a snapshot cannot
open a session or retake a character after another generation has taken authority.
An explicit later identify may transfer authority. Delayed writes from a generation
that lost ownership are rejected/fenced. This is per character, so one generation
may continue observing B while another owns A.

    {"type":"character.snapshot","protocol_version":2,"character_id":"<uuid>",
     "sent_at":"2026-09-26T14:00:12Z","state":{"level":110,"hp":32207,"mp":8744,
     "current_exp":34360711,"max_exp":4044607839,"sp":997208910,"gold":99920423940,
     "region":25000,"zone":"Jangan","x":6428.2,"y":1086.7,"z":-32.6,"botting":null}}

Changed observations use `character.state` with the same explicit ID and state
shape. Missing fields mean unavailable/not observed; null botting means the verified
phBot API has no read-only state getter. Values are validated before persistence.
Server-owned state/session/activity timestamps are authoritative; plugin `sent_at` is
diagnostic only.

`character.left` carries protocol version, explicit character ID and diagnostic
`sent_at`. The plugin sends it before identifying a switched character. A full
`character.snapshot` replaces all current state fields, clearing unavailable values
to unknown instead of carrying them from an earlier session. `character.state` is
patch-oriented and preserves fields omitted from that delta. Updates are accepted
only for that active agent/generation/session. Socket close and heartbeat
expiry close sessions owned by that generation; backend process startup closes all
persisted live sessions with a distinct end reason. After
reconnect a character starts offline and becomes online after explicit identification;
the full snapshot then restores authoritative current state. Backend restart retains durable records/state and never resurrects
live sessions.

The backend fences writes with the authenticated agent ID and socket's connection
generation, which is stored on character sessions. A socket cannot modify another
socket's session, and at most one live observer is represented per character; a new
authorized observer for the same character supersedes the old session. Different
characters observed over different sockets for one agent can remain online together.

## reconnect

The plugin reconnects automatically after transport/backend failure. Backoff begins
near 1 second, doubles to a 30-second cap, adds bounded jitter, and resets after a
successful hello/ack. On reconnect the plugin sends hello, resolves its observed
characters and publishes full snapshots. There is no durable local event store.

## HTTP agent API

GET /api/agents returns safe presentation fields for agents that have connected at
least once: stable id, connected state, first/last seen, connect/disconnect times,
current connection start when active, protocol version, plugin version and phBot
version. Stored credential hashes/tokens are never returned.

POST /api/agents/credentials creates one new stable agent identity/token pair using
the same domain generator/store path as phmonctl. The plaintext token is returned in
that creation response only; PostgreSQL persists only its SHA-256 hash. Both endpoints
are no-store.

Nuxt exposes same-origin equivalents. Slice 1 still has no human-user authentication,
so browser credential provisioning is for a trusted deployment only until later auth
work owns that boundary.

Slice 2 adds `GET /api/characters?q=&group_id=` for name/guild/server/zone search,
`GET /api/characters/{character_id}` for stable details, and persisted groups under
`/api/groups` with explicit `/members/{character_id}` operations. Responses never
contain agent credential material. Human-user authentication remains unimplemented,
so group edits and credential provisioning require a trusted network.

## Foundation health contract

- GET /healthz: process liveness; HTTP 200 with status ok even if PostgreSQL becomes
  unavailable after startup.
- GET /readyz: fresh PostgreSQL ping with a two-second deadline.
- Nuxt GET /api/health proxies readiness server-side with a bounded timeout.
- Health errors are sanitized and no credentials are returned.
