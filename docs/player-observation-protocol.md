# Passive player evidence: research and enablement gate

Investigated 2026-10-07/08. This document is evidence for the focused
[player registry plan](plans/player-registry-job-identities.md), not an enabled
packet contract. Agent protocol remains **18**, plugin becomes **1.9.30**.

The [follow-up enablement plan](plans/player-protocol-enablement.md) specifies
implementation of profile gates, verified decoders, acknowledged observations and
review-only/automatic transition stages. All of those follow-up increments are
planned; the evidence requirements below still apply.

## Source baseline

- [Official Players API](https://plugins.phbot.org/phbot-api/players): getter is
  labeled disabled and its example contains item models/plus. The installed
  phBot 20.1.2 Greatest evidence from 2026-10-01 contains no `items`. These are
  different observations; neither proves current equipment collection.
- [RSBot pinned tree](https://github.com/myildirimofficial/RSBot/tree/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5):
  primary implementation, inspected at this exact commit. It branches by client
  family; those branches are not target-server validation.
- [SilkroadDoc packet index](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/Agent-packets):
  identifies the entity equip/unequip messages. Repository baseline is pinned at
  `8473d441e69dcb66a6b638b01e5448c5f74398e4`; the wiki page observed during research
  says last edited 2025-04-16. The wiki is corroboration, not a pinned decoder.
- [SilkroadSecurityAPI vSRO188 implementation](https://github.com/ducksoup-sro/ducksoup/blob/0919317d39cf3ebcd5fef087ed1338996a148e9d/SilkroadSecurityAPI/VSRO188/Security.cs):
  packet framing/security and incoming packet transfer. Its security layer does
  not establish player identity fields. phBot already supplies callback payloads;
  PhMon adds no proxy, handshake, injection or alternate security implementation.

## Opcode investigation

All entries below remain **disabled for registry decoding**. Capture includes
these server-to-client callbacks only when explicitly started locally.

| Opcode | Pinned primary implementation and finding | Unresolved target evidence |
| --- | --- | --- |
| `0x3015` | [Single spawn](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Entity/EntitySingleSpawnResponse.cs) dispatches the shared entity parser. | Entity discrimination, version-specific prefix/tail, exact consumption and name classification. |
| `0x3016` | [Single despawn](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Entity/EntitySingleDespawnResponse.cs) reads a runtime `uint32`; transport removal is also handled. | Exact target payload and lifecycle/incarnation behavior. |
| `0x3017` | [Group begin](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Entity/EntityGroupSpawnBeginResponse.cs): operation byte and little-endian `uint16` count. | Target envelope and count semantics. Diagnostic envelope inspection is not entity decoding. |
| `0x3019` | [Group data](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Entity/EntityGroupSpawnDataResponse.cs) appends segments. | Complete mixed-entity multipart captures. Individual segments are not independent spawns. |
| `0x3018` | [Group end](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Entity/EntityGroupSpawnEndResponse.cs): operation 1 parses spawns; 2 reads runtime-ID despawns. | Exact count consumption, trailing bytes and incomplete sequences. |
| `0x3038` | SilkroadDoc index identifies entity equip. No applicable handler/layout was found in the pinned RSBot Entity/Inventory handlers. | Entity versus slot IDs, model/plus fields, full payload and supported client version. No guessed decoder. |
| `0x3039` | SilkroadDoc index identifies entity unequip. Same pinned-source layout gap. | Removal semantics and explicit observed-empty slot evidence. |
| `0x3013` | [Character data](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Character/CharacterDataResponse.cs) appends the controlled character's chunked data. | Own-character data cannot establish other-player equipment or a cross-mode stable identifier. Its enclosing chunk messages are not in this bounded capture. |
| `0x3040` | [Inventory item update](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Inventory/InventoryUpdateItemResponse.cs): controlled inventory slot, flags, optional fields including unsigned variance/magic values. Existing PhMon item tracker remains unchanged. | No runtime entity ID in this layout; never attach controlled-character item stats to another player. |

[SpawnedPlayer](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/Spawn/SpawnedPlayer.cs)
reads visible item models/plus and a separate avatar collection. Static definitions
identify job suits. It then reads entity details, name and job byte, with client
family branches for job rank/level, guild and stalls. This parser does not expose
other-player variance, durability or blues in its visible equipment list. The
[JobType enum](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/JobType.cs)
maps 0/1/2/3 to none/trade/thief/hunter in this implementation. **Job level is not
character level.** Item-model dictionaries do not prove slot positions or empty
jewelry slots. Unknown models can affect consumption; a future decoder must fail
closed instead of trying to continue with assumed field sizes.

No source or fixture proves a stable character identifier surviving job changes.
Runtime IDs stay scoped to server, observer session, epoch and incarnation. No
identity hash/profile or transition auto-link policy is enabled for Greatest.

## Bounded local capture

The existing `handle_joymax()` callback calls the worker's capture admission. It
only copies allowlisted bytes; conversion/envelope work and export use the existing
worker. Buttons are **Capture players (15 s)** and **Save local capture** in the
local plugin. Capture is off by default; the internal maximum duration is 30 s.

Budgets: 64 KiB per callback packet, 128 records, 2 MiB total payload, 256 KiB per
assembled diagnostic group, count at most 128. Hex records occupy at most 4 MiB
plus bounded object overhead. Callback and worker share the same total budget.
Malformed groups, overflow and session/connection changes invalidate continuity.
Segment sequence numbers are retained rather than duplicated assembled payloads.
Missing snapshots, truncation, overflow and timeout never establish despawns.

Export creates a unique `player-capture-*.json` in the existing local spool
folder. No raw packet bytes are sent to the backend. Reports say
`decoder_enabled: false` and `sanitized: false`. The operator must inspect/redact
names, addresses, credentials and unrelated sensitive payloads before sharing.
No raw runtime capture or invented sanitized fixture is committed here.

## Equipment observation foundation

Backend-only normalized equipment describes source/profile, coverage, latest
attempt availability, slot states and original per-slot/per-field timestamps.
Unsigned 64-bit variance and magic values are decimal strings. Known slots remain
visible after an unavailable/partial attempt, with their original evidence time.
Changing model or enhancement invalidates old instance attributes. Existing item
enrichment/tooltips accept any independently verified item representation; static
catalog ranges never become fabricated instance rolls.

`gear-v1` uses SHA-256 in sorted slot order, excluding observation times,
durability and the enriched item presentation adapter. Only normalized observed
model/plus/variance/magic values enter gear hashes. `identity-v1` requires a backend-verified
matching comparison-profile version/model and four known normal slots; it excludes job/avatar gear
and transient stats. Partial comparison checks overlap and conflicts. These
internal synthetic tests do not authorize agent-controlled verification flags.

No `player.observations` message was introduced: there is no verified new packet
source to negotiate yet. A future addition must use stable evidence IDs, bounded
batches, session/generation fencing, commit acknowledgements and exact replay.
Legacy `map.players` remains unchanged and has no persistence acknowledgement.

## Required evidence to unblock

For each exact target client/server profile, provide sanitized actual callback
captures plus independently observed screen/API facts for complete single/group
spawns, explicit despawns, multipart groups, equip/unequip, reconnect/teleport,
runtime-ID reuse, normal→Trader/Hunter/Thief and reversals. Record phBot/client
version, dataset/profile version, slot/model/plus coverage, any instance attributes,
job classification and world/cave coordinate scope. Include distinct players with
identical gear and competing transitions. Verify exact field consumption and
unsupported variants before enabling a decoder or automatic transition rule.
