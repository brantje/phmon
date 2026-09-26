# Agent protocol v1

Slice 1 defines the first PhMon agent wire protocol. Commands, character snapshots
and events are deliberately outside this version's implemented message set.

## Transport and authentication

- Endpoint: GET /agent upgraded to WebSocket.
- Production deployments use wss:// with normal certificate validation. Plain ws://
  is for trusted local development only.
- Every upgrade request carries Authorization: Bearer <agent-token>.
- Tokens are created per agent with phmonctl agent create. PostgreSQL stores only a
  SHA-256 token hash. A token is permanently bound to one stable agent_id.
- Tokens never belong in URLs, logs, QR codes or browser-visible API responses.
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

## HTTP read model

GET /api/agents returns safe presentation fields for agents that have connected at
least once: stable id, connected state, first/last seen, connect/disconnect times,
current connection start when active, protocol version, plugin version and phBot
version. Credential hashes and tokens are never returned. Responses are no-store.

## Foundation health contract

- GET /healthz: process liveness; HTTP 200 with status ok even if PostgreSQL becomes
  unavailable after startup.
- GET /readyz: fresh PostgreSQL ping with a two-second deadline.
- Nuxt GET /api/health proxies readiness server-side with a bounded timeout.
- Health errors are sanitized and no credentials are returned.
