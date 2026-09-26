# Protocol status

No agent protocol, `/agent` endpoint or WebSocket implementation exists in Slice 0.
Slice 1 must define the first explicit wire version, authenticated identity, hello,
heartbeat, reconnect behavior and compatibility response before implementation.
Commands (Slice 3) need stable IDs, acknowledgement/result separation and expiry.
Snapshots and events are separate concepts; backend policy stays on the backend.
See [AGENTS.md](../AGENTS.md) for the canonical roadmap.

## Foundation HTTP contract

- `GET /healthz`: process liveness; HTTP 200, `{"status":"ok"}` even if PostgreSQL
  is unavailable.
- `GET /readyz`: a fresh PostgreSQL pool ping with a two-second deadline. HTTP 200,
  `{"status":"ok","database":"ok"}` or HTTP 503,
  `{"status":"unavailable","database":"unavailable"}`. No connection errors or
  credentials are returned. HEAD is also supported by Go's GET routes.
- Nuxt `GET /api/health`: calls Go `/readyz` server-side with a three-second timeout,
  no retries, and the same success/unavailable bodies and status codes. Transport
  failures become sanitized HTTP 503 responses. No business logic lives in Nuxt.
- Health responses use `Cache-Control: no-store`. There is no product API version
  yet; do not mistake these health routes for an agent protocol version.
