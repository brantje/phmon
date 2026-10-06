# TradeNexus protocol v1

TradeNexus is an open WebSocket on the PhMon server at `GET /tradenexus`. It
relays thief sightings between AdvancedAutoTrade clients and PhMon. The socket
does not use operator cookies or agent tokens. PhMon stores every accepted
sighting and broadcasts it to clients subscribed to that game server.

TradeNexus itself does not remember sightings for clients that connect later.
A new subscriber receives only sightings published after it subscribes.

## Transport

- Endpoint: `GET /tradenexus`, upgraded to WebSocket.
- Any `Origin` is accepted. Clients are expected to be non-browser bots as well
  as browsers. The route sets no cookies.
- Frames are UTF-8 JSON text, at most 4096 bytes. Larger frames are rejected.
- Every frame includes `"v": 1` and a `type`. Unknown fields are ignored.
- Keepalive is WebSocket ping/pong. The server pings about every 30 seconds.
- Unknown `type` values return `error` with code `unsupported_type` and leave
  the connection open. Five consecutive invalid frames close the connection.
- Limits, per immediate peer address: 25 connections, 5000 connections total,
  25 subscribed servers, and 100 `thief.report` frames per 5 seconds.
  `rate_limited` is not an invalid frame. The peer cap uses the dialing
  address, so clients behind one reverse proxy share it.

## Client to server

Subscribe. This replaces the connection's server list. Matching is
case-insensitive. Names are trimmed, duplicates collapse, and 1–25 names of
1–100 characters are required.

```json
{ "v": 1, "type": "subscribe", "servers": ["Greatest"] }
```

Report a thief. `ref` is optional, at most 64 bytes, and is echoed only to the
sender. `origin` sent by a client is ignored. A report does not require a
subscription. The sender receives its own broadcast only when it is subscribed
to that server, and can ignore the echo by comparing `sighting_id` with `ack`.

```json
{
  "v": 1,
  "type": "thief.report",
  "ref": "client-chosen-id-123",
  "server": "Greatest",
  "thief": { "name": "Bandit123" },
  "position": { "region": 24999, "x": 1234.5, "y": 678.9, "z": 12.0 },
  "position_source": "thief",
  "reporter": { "name": "trader1", "app": "AdvancedAutoTrade", "version": "1.0.0" },
  "observed_at": "2026-10-06T19:10:00Z"
}
```

## Server to client

```json
{
  "v": 1,
  "type": "hello",
  "server_time": "2026-10-06T19:10:00Z",
  "limits": { "max_frame_bytes": 4096, "max_servers": 25, "reports_per_5s": 100 }
}
```

```json
{ "v": 1, "type": "subscribed", "servers": ["Greatest"] }
```

```json
{ "v": 1, "type": "ack", "ref": "client-chosen-id-123", "sighting_id": "8c1f..." }
```

```json
{
  "v": 1,
  "type": "error",
  "ref": "client-chosen-id-123",
  "code": "invalid_message",
  "message": "thief.name is required"
}
```

```json
{
  "v": 1,
  "type": "thief.sighting",
  "sighting_id": "8c1f...",
  "server": "Greatest",
  "thief": { "name": "Bandit123" },
  "position": { "region": 24999, "x": 1234.5, "y": 678.9, "z": 12.0 },
  "position_source": "thief",
  "reporter": { "name": "nuker1", "app": "PhMon", "version": "1.9.24" },
  "origin": "phmon",
  "observed_at": "2026-10-06T19:10:00Z",
  "received_at": "2026-10-06T19:10:00.120Z"
}
```

`origin` is `phmon` or `external`. `position` is omitted when the reporter had
no coordinates. `reporter` is omitted when the client sent none.

## Field rules

- `server`: 1–100 characters, matched case-insensitively, stored as trimmed.
- `thief.name`: required, trimmed, 1–64 bytes, no control characters.
- `position`: optional. `region` uses PhMon's existing map range, -32768
  through 65535 excluding 0. Outdoor regions such as 25735 do not fit in a
  signed 16-bit integer, so this wider range is the contract. `x` and `y` are
  finite and within ±1,000,000. `z` is optional with the same numeric bounds.
  Cave placement still needs `z`; a cave region without `z` is stored but not
  drawn.
- `position_source`: `thief`, `observer`, or `unknown`. `thief` requires
  `position`. Omitted means `unknown` when there is no position.
- `reporter.name`, `reporter.app`, and `reporter.version`: optional, each at
  most 64 bytes.
- `observed_at`: RFC3339. A missing value, or one more than 5 minutes from
  server time, is replaced by `received_at`.
- `ref`: optional, at most 64 bytes, echoed only to the sender.

Error codes: `invalid_message`, `unsupported_version`, `unsupported_type`,
`too_large`, `rate_limited`. `not_subscribed_required` is reserved and is not
enforced.

## PhMon relay

A newly stored `job.thief_seen` event is published with `origin` `phmon`, the
observing character as `reporter.name`, app `PhMon`, and the plugin version
from the event payload. Replays of an already stored event are not published
again. The plugin attaches the thief's own region and coordinates when
`get_players()` returns exactly one case-insensitive name match; otherwise the
sighting uses the observer's position and `position_source` `observer`.
