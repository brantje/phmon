# Agent protocol v1

Slice 1 defines the first PhMon agent wire protocol. Commands, character snapshots
and events are deliberately outside this version's implemented message set.

## Transport and authentication

- Endpoint: GET /agent upgraded to WebSocket.
- Production deployments use wss:// with normal certificate validation. Plain ws://
  is for trusted local development only.
- Every upgrade request carries Authorization: Bearer <agent-token>.
- Tokens are created per agent through the dashboard or `phmonctl agent create`.
  Both use the same server-side generator/store path. PostgreSQL stores only a
  SHA-256 token hash. A token is permanently bound to one stable agent_id.
- Tokens never belong in URLs, logs or QR codes. The credential-creation response is
  the sole browser-visible exception: it returns the newly generated plaintext token
  once with `Cache-Control: no-store`. Existing tokens are never retrievable.
- The server rejects a missing/invalid token before WebSocket upgrade.
- The first application message must be hello within 5 seconds. The agent_id in
  hello must match the identity authenticated by the bearer token.
- Application messages are JSON text frames and are limited to 8 KiB.

Protocol version: 1.

## hello

Agent to server:

    {
      "type": "hello",
      "protocol_version": 1,
      "agent_id": "9d63c35b-1ff2-4d76-a17c-10514af2f664",
      "plugin_version": "1.0.0",
      "phbot_version": "21.1.9",
      "sent_at": "2026-09-26T14:00:00Z"
    }

plugin_version and phbot_version are bounded to 64 characters. sent_at is diagnostic
agent time; server-owned timestamps remain authoritative.

A valid hello authenticates the durable agent, updates first/last-seen and version
metadata, and installs the socket as that agent's active session. A newer valid
session for the same agent supersedes the older socket. Cleanup from the old socket
must not remove the newer active session.

Server to agent:

    {
      "type": "hello.ack",
      "protocol_version": 1,
      "heartbeat_interval_seconds": 10,
      "heartbeat_timeout_seconds": 30
    }

Unsupported protocol versions close with an explicit policy/unsupported-data reason.
Malformed or identity-mismatched hello messages are rejected.

## heartbeat

After hello acknowledgement, the plugin sends:

    {
      "type": "heartbeat",
      "protocol_version": 1,
      "sent_at": "2026-09-26T14:00:10Z"
    }

A valid heartbeat refreshes the server-owned last_seen_at value. If no valid
application message arrives for 30 seconds, the server closes the session and marks
the agent disconnected. WebSocket ping/pong control frames may exist at the transport
layer but do not replace the application heartbeat.

## reconnect

The plugin reconnects automatically after transport/backend failure. Backoff begins
near 1 second, doubles to a 30-second cap, adds bounded jitter, and resets after a
successful hello/ack. On reconnect the plugin sends hello again before normal
messages. There is no durable local event store in Slice 1.

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

## Foundation health contract

- GET /healthz: process liveness; HTTP 200 with status ok even if PostgreSQL becomes
  unavailable after startup.
- GET /readyz: fresh PostgreSQL ping with a two-second deadline.
- Nuxt GET /api/health proxies readiness server-side with a bounded timeout.
- Health errors are sanitized and no credentials are returned.
