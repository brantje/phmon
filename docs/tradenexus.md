# TradeNexus protocol v1

TradeNexus is an open WebSocket on the PhMon server at `GET /tradenexus`. It
relays thief sightings between AdvancedAutoTrade clients and PhMon, and accepts
finished trade reports from those clients. The socket does not use operator
cookies or agent tokens. PhMon stores every accepted sighting and broadcasts it
to clients subscribed to that game server. Trade reports are stored and
acknowledged only to the sender.

The WebSocket hub keeps the latest sighting per thief name for each server in
memory for **10 minutes** from `received_at`. When a client sends `subscribe`,
the server replies with `subscribed` and then a `thieves` snapshot of every
active sighting for the subscribed servers. Live `thief.sighting` frames still
follow for new reports. A process restart clears the hub memory; PostgreSQL
history and the operator map layer use separate retention rules.

## Transport

- Endpoint: `GET /tradenexus`, upgraded to WebSocket.
- Any `Origin` is accepted. Clients are expected to be non-browser bots as well
  as browsers. The route sets no cookies.
- Frames are UTF-8 JSON text. Subscribe and `thief.report` frames are at most
  4096 bytes. A `trade.report` frame may be 16384 bytes so its waypoint list
  fits. Larger frames are rejected.
- Every frame includes `"v": 1` and a `type`. Unknown fields are ignored.
- Keepalive is WebSocket ping/pong. The server pings about every 30 seconds.
- Unknown `type` values return `error` with code `unsupported_type` and leave
  the connection open. Five consecutive invalid frames close the connection.
- Limits, per immediate peer address: 25 connections, 5000 connections total,
  25 subscribed servers, and 100 `thief.report` or `trade.report` frames per 5 seconds.
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
  "reporter": {
    "name": "trader1",
    "app": "AdvancedAutoTrade",
    "version": "1.0.0"
  },
  "observed_at": "2026-10-06T19:10:00Z"
}
```

Report a finished trade. `ref` is required and uses the same 1–64 byte text rule
as a thief ref. AdvancedAutoTrade sends `aat-<unix>-<counter>`; PhMon does not
parse that shape. The sender receives `ack` with `trade_id`. A repeat of the
same server and `ref` returns the original `trade_id` and does not change the
stored row. Subscribers do not receive the report. `reporter.app` and
`reporter.version` match thief reports. `reporter.name` is required.

```json
{
  "v": 1,
  "type": "trade.report",
  "ref": "aat-1728460800-3",
  "server": "Server Name",
  "outcome": "success",
  "reason": "sold",
  "route": { "from": "Jangan", "to": "Donwhang" },
  "waypoints": [
    { "name": "chau_approach", "x": 37643, "y": 7342 },
    { "name": "chau_mid", "x": 37643, "y": 7342 },
    { "name": "doji_approach", "x": 37643, "y": 7342 }
  ],
  "goods": [{ "name": "Silk", "quantity": 120 }],
  "gold": 45000,
  "duration_s": 842,
  "stars": "Max",
  "reporter": {
    "app": "AdvancedAutoTrade",
    "version": "1.0.0",
    "name": "CharName"
  },
  "finished_at": "2026-10-09T07:03:00Z"
}
```

Required: `v`, `type`, `ref`, `server`, `outcome`, `reason`, `route.from`,
`route.to`, `waypoints`, `reporter.name`, and `finished_at`.

- `outcome` is `success`, `failed`, or `cancelled`.
- `reason` belongs to that outcome: success is `sold` or `already_empty`; failed
  is `thief`, `navigation`, or `error`; cancelled is `cancelled`.
- Towns are `Jangan`, `Donwhang`, `Hotan`, `Samarkand`, `Constantinople`, and
  `Alexandria`. Matching ignores case. `from` and `to` must differ.
- `waypoints` is the graph nodes the trip arrived at, in arrival order, at most
  200 objects. Each `name` is a `ROUTE_NODES` id: 1–64 characters, lowercase
  letters, digits, and underscores. `x` and `y` are required finite numbers
  within ±1,000,000. An empty list means the inter-town leg arrived at no graph
  node. PhMon stores the coordinates and does not keep its own copy of the
  graph. The node being walked when the trip stops is left off.
- `goods` is optional, at most 16 items. Each name uses the text rule and each
  quantity is an integer from 1 to 99999. Omit it when the transport reading is
  missing. `already_empty` may send `[]` and cannot send a non-empty list.
- `gold` is an optional signed 32-bit integer, gold now minus trip-start gold.
- `duration_s` is optional whole seconds from 0 to 86400.
- `stars` is optional: `"1"` through `"5"` or `"Max"`.
- `detail` is optional, only for `navigation` and `error`, at most 256 bytes.
- `thief.name` is required for reason `thief` and rejected otherwise.
- `transport` is an optional display label using the name text rule.
- `finished_at` is UTC RFC3339. A value more than 5 minutes from server time is
  stored as `received_at`, the same replacement used for `observed_at`.

Trade and thief reports share the 100-frames-per-5-seconds counter. The limit
error says `too many reports`.

## Server to client

```json
{
  "v": 1,
  "type": "hello",
  "server_time": "2026-10-06T19:10:00Z",
  "limits": {
    "max_frame_bytes": 4096,
    "max_trade_frame_bytes": 16384,
    "max_servers": 25,
    "reports_per_5s": 100
  }
}
```

```json
{ "v": 1, "type": "subscribed", "servers": ["Greatest"] }
```

```json
{
  "v": 1,
  "type": "thieves",
  "sightings": [],
  "truncated": false
}
```

Sent immediately after `subscribed`. Each `sightings` entry uses the same JSON
shape as `thief.sighting` below, ordered oldest `received_at` first. An empty
list still means the snapshot finished. `truncated` is true when a subscribed
server hit the 256-name active cap and dropped older thieves. Outbound snapshot
frames are not limited to 4096 bytes; inbound client frames remain capped.

```json
{
  "v": 1,
  "type": "ack",
  "ref": "client-chosen-id-123",
  "sighting_id": "8c1f..."
}
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

A trade ack uses `trade_id` instead of `sighting_id`:

```json
{ "v": 1, "type": "ack", "ref": "aat-1728460800-3", "trade_id": "8c1f..." }
```

Accepted trades are listed for the signed-in operator at `GET /api/trade-reports`
and on Events → Trades. The list is filtered by server, finished time, and
reporter, route, transport, thief, outcome, or reason. Waypoint coordinates are
returned with each row and are not drawn on the map. Rows older than
`TRADENEXUS_RETENTION_DAYS` are deleted with thief sightings, using
`received_at`.

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
