# Slice 4 item-instance evidence matrix

Updated 2026-09-28 for plugin 1.2.6 live API evidence and the Slice 4 audit. This
matrix separates API facts, published protocol references, implementation behavior
and open real-runtime checks. Synthetic fixtures prove parser and state logic only;
they are not phBot runtime evidence.

## Shared SRO inventory assets and definitions — 2026-09-28

SRO inventory presentation is shared across game servers. Items expose the stable
`servername` code, so the backend indexes the local catalog by that code and reuses
the complete static item presentation on servers without a mapped profile. This
includes artwork, names, classification/type IDs, rarity/seal, requirements and
reference-stat ranges. It also uses shared option definitions to interpret verified
live item observations. Where multiple configured catalogs disagree on a field or
option ID, that conflicting field is omitted from the shared index. Numeric model
IDs remain profile-specific and are never used to guess cross-server metadata.

Live values still come only from the current character's phBot item observation;
shared definitions provide their labels and reference ranges. Static definitions do
not create inventory, ownership or historical item events. The current local SRO
catalog has 14,227 icon entries, all resolving to files under
`web/public/game-assets`. The assets are local and are not server-owned character
or inventory data.

The shared-definition fallback is deployed to `10.25`. The backend `/readyz`
endpoint and web `/api/health` both returned `status=ok,database=ok`; a bundled
item icon returned HTTP 200. PostgreSQL stayed healthy and was not restarted.
`go test ./internal/resources` verifies that a different server/model ID still gets
the shared rarity and reference-based API stat calculation by item code. Authenticated
visual confirmation against a live Servar inventory remains open.

| Area | Evidence and implementation | Status / limit |
| --- | --- | --- |
| Basic and extended item fields | phBot documents model, server name, name, quantity, plus, durability, capacity and `None` empty slots: <https://plugins.phbot.org/phbot-api/inventory>. Live 20.1.1/plugin 1.2.6 observations add typed white/blue maps, current/max durability, attack/defense, parry/block/critical/rate and reinforcement/absorption fields. | 14 sanitized real items are checked in. 21 API field names were observed in the live agent data. Exact per-model presence varies; absent stays unavailable. |
| Passive packet callback | phBot documents `handle_joymax(opcode, data)` for server packets: <https://plugins.phbot.org/phbot-api/events>. The plugin only copies allowlisted opcodes into a bounded queue and returns `True`. | Callback does no decoding, disk I/O or network I/O. Runtime scheduling and exact callback data type still need phBot confirmation. |
| vSRO protocol baseline | SilkroadDoc README identifies the collected protocol as vSRO 1.188: <https://github.com/DummkopfOfHachtenduden/SilkroadDoc>. The packet index lists 0x3040 item-stat update, 0x3052 durability update and 0xB034 inventory operation. | The indexed packet pages available in the checked-out copy do not contain full field layouts. The parser below is therefore marked corroborated, not runtime-verified. |
| 0x3040 item-stat update | Bounded little-endian parser follows the pinned RSBot implementation: slot, flags, then RefObjID, plus, unsigned 64-bit variance, quantity, durability, state and ordered `(option ID, value)` pairs for set flags: <https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Inventory/InventoryUpdateItemResponse.cs>. | Corroborating implementation is not proof for the Greatest runtime. Unknown flag 0x80, truncation, excess options and trailing bytes reject the observation. Runtime packet fixture remains required before claiming correct live parsing. |
| 0x3052 durability update | Bounded parser follows the pinned RSBot handler (slot byte plus unsigned 32-bit durability): <https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Inventory/InventoryUpdateDurabilityResponse.cs>. | Corroborating implementation only; a vSRO 1.188 runtime fixture is still required. |
| Container changes | 0xB034 is listed as `AGENT_INVENTORY_OPERATION` in SilkroadDoc. This implementation does not parse its operation subtypes; any packet invalidates every cached item enrichment. Queue overflow, malformed updates, session/profile changes and API model/plus/empty-slot conflicts also invalidate. | Fail-closed invalidation is implemented and fixture-tested. Movement/transfer-aware retention is intentionally unavailable until operation layouts are verified. Storage and guild-storage packet snapshots are not decoded. This is not live-validated safe invalidation. |
| Observation identity | Updates are tied to the active agent session, tracker epoch, packet sequence, source slot and RefObjID, then reconciled against the current API model and enhancement before being attached to bag/equipment slots. | Slot/model matches alone never seed a new item. Replacement safety is fixture-tested. Natural runtime ordering and same-model replacement behavior remain open. The 0xB034 path discards cached enrichment; it does not retain item identity through moves. |
| Variance storage | The 64-bit variance stays a decimal string. Family field positions and 5-bit slots follow the pinned RSBot `ItemAttributesInfo`: <https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/ItemAttributesInfo.cs>. | Backend computes only `floor(roll × 100 / 31)` as roll quality with explicit armor, shield, weapon and accessory positions. The field map needs a matching vSRO fixture before live confirmation. |
| Reference ranges | The Greatest bundle was rebuilt from the local client archives and validated; the backend compact catalog carries family-specific reference ranges as decimal strings for 9,334 of 14,238 item records. | Table ranges alone do not establish the absolute roll/enhancement formula. Absolute combat values and maximum durability remain unavailable. |
| Magic options | The rebuilt Greatest catalog contains 615 raw option records from the unambiguous `magicoption.txt` table; 298 have raw ranges. Packet values retain ordered decimal-string IDs and values, including duplicates and confirmed empty arrays. | Zero option labels uniquely joined to the available localized object keys; none of the 615 records has a verified value scale or unit. The backend stores source option IDs/ranges but omits named blue values from presentation. |
| Diagnostic capture | `ITEM_PACKET_CAPTURE_ENABLED` defaults off. When enabled, retained records are parsed item-only observations, capped at 100 records / 2 MiB, and can be written from a worker context. Credentials, login traffic and unrelated packet sections are never captured. | No real capture fixture is checked in. Tests verify the sanitized writer and limits; actual runtime capture is pending a parser-enabled plugin payload and naturally arriving item traffic. |

## Live runtime check — 2026-09-27 20:49 UTC

The authenticated LAN Stats page recovered to current data. The backend reported two
agents on plugin 1.2.2 / phBot 20.1.1 / agent protocol v4, both seen within the last
minute, and four online characters. It had 239 durable item rows, with the newest
inventory/equipment observation at 20:48:34 UTC; 68 rows retained raw `api_fields.blues`.
No item row contained an `instance` object.

All four latest `item_enrichment` observations reported outer availability
`not_observed` and retained a payload with protocol `unknown` and reason
`passive_item_packet_decoder_not_enabled`. A later database read at 21:03 UTC showed
two agents reporting plugin 1.2.2 / phBot 20.1.1 / protocol v4, 242 item rows and zero
`instance` objects.

The retained fallback payload cannot identify the loaded file: resource persistence
updates availability for non-observed resources but intentionally keeps their prior
payload. The deployed checkout also held a pre-parser `plugin/PhMon.py`, while the
workspace copy contained `PassiveItemTracker`; the operator's active phBot file is
still unverified. Plugin 1.2.3 reports `decoder_build=vsro_1188_passive_r1` in QtBind
and in enrichment telemetry. The backend now persists that diagnostic even when no
item packet has been observed. Runtime confirmation of this build is pending. No
instance rows means percentages, rolled stats and blues are still not confirmed live;
raw API blues remain untrusted and are intentionally not promoted to tooltip fields.

## Live runtime check — 2026-09-27 21:24 UTC

After the operator re-uploaded the parser release, all four active characters reported
plugin 1.2.3, phBot 20.1.1 and agent protocol v4. Their latest diagnostic had
`decoder_build=vsro_1188_passive_r1`, but `protocol=unknown`, no
`protocol_reason`, zero observed items, and zero persisted item rows with an
`instance` object. API-backed item collection was current (251 item rows total).
This proves the parser build and resource transport were active; it does not prove an
item packet was decoded.

The local active phBot installation was then checked read-only. `get_config_dir()`'s
directory is `Config`, while the selected `vSRO.json` is beside it. That JSON uses a
root profile map (`GreatestSRO`) whose `servers` list contains `Greatest`; its
`version=296` is ignored, and the explicit vSRO variant booleans select the 1.188
baseline. The prior detector searched inside `Config` and expected a different shape,
so it failed closed. The new detector now reads this shape and locally resolves it as
`vsro-1.188` with reason `v1.188_selected_by_phbot_flags`.

Plugin 1.2.4 (`vsro_1188_passive_r2`) includes this path/shape correction and emits
the protocol-resolution reason. It has not yet been transferred or confirmed in the
live agent. The backend diagnostic persistence fix is already deployed. No phMonitor
executable or private client protocol was inspected; the detector was corrected from
the installed phBot config and public phBot documentation.

## Runtime gate

Verification at 2026-09-27 21:28:42 UTC: all four active characters now report
plugin 1.2.4 / phBot 20.1.1, `decoder_build=vsro_1188_passive_r2`,
`protocol=vsro-1.188`, and `protocol_reason=v1.188_selected_by_phbot_flags`.
Heartbeats were within four seconds of the check. The config detection fix is
confirmed live. There were 239 current item rows and zero instance rows.
`nuker1` reported `inventory_operation_unclassified`, proving an inventory-operation
packet reached the tracker but was deliberately invalidated because that layout is
not implemented. The other characters reported `protocol_changed`, with no attached
instance observations. Upload, transport and protocol detection are verified; richer
item values are not. Full inventory snapshots/movements and formula/blue semantics
remain implementation gaps, not upload problems. No game action was issued.

The 1.2.4 protocol gate is now passed. The subsequent operator-authorized static
inspection found a reference API-backed tooltip path and a concrete PhMon bug:
integer dictionary keys were discarded during API evidence serialization. The
reference also reads additional attribute aliases that PhMon omitted. See
[item-tooltip-investigation.md](reference/item-tooltip-investigation.md) for exact
artifact identification, observations and formula discrepancies.

Next load plugin 1.2.5 and confirm `api_evidence_version=2`. Inspect API item field
types, attribute maps and option IDs before requiring packet-only enrichment.
Prefer verified API observations and use passive packets for missing fields.
Compare observed armor, accessory, weapon, shield and stackable details with source
evidence; retain independent formula and rounding checks. No item movement, storage
opening, alchemy or character reconnect is authorized merely to generate fixtures.
Slice 4 rolled details remain open.


## 2026-09-27: plugin 1.2.5 API evidence confirmed

Read-only live verification at 21:44:58 UTC confirmed all four characters on
plugin 1.2.5, phBot 20.1.1 and vSRO 1.188. All 244 current items retained evidence
schema 2. Rich data is arriving: `whites` contains integer attribute IDs and integer
percentages, and `blues` contains integer option IDs and values. The prior loss of
integer dictionary keys was the primary blocker for these fields.

Sanitized actual observations are checked in as
`server/internal/resources/testdata/phbot-20.1.1-api-items.json` (14 items; no
credentials, character/session identifiers or unrelated traffic). Python Casque
reports defense rolls 12/19, parry 22, reinforcement 3/32, durability 9, Int 3 and
MP 5; Tiger Bone Coronet reports 61/32, 45, 0/9, 0, Steady 2 and Parry 5%.
Flame Platinum Necklace reports absorption rolls 6/12; Copper Ring reports 0/0.
These match the supplied screenshots. Source observations remain unchanged.

Backend presentation now recognizes these observed API maps only with evidence
schema/type/count validation and an exact dataset/model/code match. Verified white
mappings currently cover Chinese armor/protector and accessories. Four dataset
option codes have verified labels/scales: MATTR_INT, MATTR_MP, MATTR_SOLID and
MATTR_ER. Other families/options remain explicit gaps; unknown raw values are
preserved without invented presentation. Missing and confirmed-empty options remain
distinct. Presentation is recomputed per observation, never retained by slot.

The API field schema also exposes `phys_def`, `mag_def`, `parry`, `block`,
`critical`, `attack_rate`, `max_durability`, attack/reinforcement/absorption min/max
fields. Plugin 1.2.6 adds those exact field names to bounded raw evidence collection.
Their real values, scaling and enhancement/blue effects still require verification;
this release does not promote them to trusted absolute stats. The collector test
values are synthetic and are not runtime evidence. No game action was performed.

## 2026-09-28: plugin 1.2.6 named values and complete observed-blue display

The live database confirmed plugin 1.2.6 on connected phBot 20.1.1 agents.
Across retained items, `api_fields` contains 21 distinct keys: whites, blues,
current/max durability, physical/magical defense, parry, block, critical, attack
rate, and physical/magical attack, reinforcement and absorption minimum/maximum
values. The backend reads these typed fields for the corresponding equipment
families. It keeps an observed scalar even when the white-roll map is missing or
invalid; it does not attach an unobserved percentage.

For Python Casque, the live API reported physical/magical defense integer floors
54/73, parry 23, maximum durability 77 and reinforcement 13.872/18.233. The
dataset's reference range combined with observed white rolls 3/32 produces
54.8/73.3 at one-decimal display precision, matching the in-game screenshot.
The live Phoenix Horn Spear reported physical attack 375–435, magical attack
640–757, attack rate 124, critical 4, durability 64/64 and reinforcement
88.516–105.358 / 152.742–186.728. Those plugin numbers are used directly;
reinforcement displays at one decimal. No packet observation was needed here.

Every observed blue entry is displayed individually in source order. Known
dataset option IDs/codes receive verified in-game labels; an unfamiliar code is
shown literally with its ID/raw value rather than silently dropped or assigned a
guessed meaning. The API does not provide blue roll-quality percentages. None of
the retained 1.2.6 items reports Advanced elixir eligibility or maximum magic
option capacity, so those screenshot lines cannot be shown as observed facts.
The `MATTR_REPAIR` option has an explicit label when it is actually observed.
This is an available-data improvement, not evidence that all Slice 4 item semantics
or the passive packet paths are complete.

## 2026-09-28 Slice 4 re-audit

### What is backed by current evidence

- Live API-backed item observations preserve ordered typed whites and blues and the
  named scalar fields; item enrichment requires the configured dataset and an exact
  model/code match. The backend renders each observed blue separately, retains
  duplicates/order and shows an unresolved code plus raw option ID/value when its
  text meaning is not resolved. White percent fields are rendered as roll quality.
- Captured in-game/API comparisons cover Python Casque, Tiger Bone Coronet, Flame
  Platinum Necklace, Copper Ring and Phoenix Horn Spear. These provide live evidence
  for armor/protector, accessory and weapon fields; they do not establish every
  family, server variant, blue label/scale or enhancement modifier. Shield-family
  and other unmapped live details remain open.
- Absolute attack/defense/reinforcement/absorption values are only shown from exact
  typed API scalar inputs or a matching reference plus supported white roll. Current
  item reference data is not sufficient to derive every formula. Missing inputs or
  modifier evidence must continue to produce unavailable fields. Blue roll-quality
  percentages are not returned by the current getter and are not calculated.
- The code contains strict little-endian readers for 0x3040 and 0x3052 based on
  pinned third-party corroboration. SilkroadDoc's 1.188 index names opcodes but does
  not provide the complete layouts here. No real Greatest packet fixture verifies
  the offsets, flags or field meanings, so do not promote those decoders to verified.
- Unknown 0xB034 inventory-operation subtypes invalidate cached packet enrichment.
  Malformed/truncated packets, overflow, profile/session changes, empty slots and
  model/plus conflicts also fail closed. Unit fixtures show same-slot replacement
  does not inherit a prior packet record after these triggers. No runtime evidence
  verifies event ordering, dropped-packet behavior or safe invalidation on actual
  item change. Movement/transfer retention is not implemented.
- The item observation's session/epoch/slot/model/sequence fields prevent a slot
  alone from creating enrichment. API observation data stays separate from packet
  data. Pet containers are API-backed only; no pet packet parsing was added.

### Still not observable or verified

The current live API does not report maximum magic-option capacity or a standalone
Advanced elixir-eligibility flag. The in-game “Able to use Advanced elixir.” line is
only represented when the actual observed option maps to a supported `MATTR_REPAIR`
definition; the plugin does not provide an independent static eligibility field.
Option IDs with dataset codes but no verified in-game label/unit/scale remain
diagnostic raw values. Additional reference captures of those option types, or a
trusted versioned name/scale source, are needed before assigning presentation meaning.

Equipment separation is a separate unresolved data-contract issue. The official
inventory API describes a flat list and `size` but does not say the first 13 entries
are equipment. The implementation leaves that split marked
`adapter_lead_runtime_unverified`; runtime item family or a matching screenshot alone
does not establish each source index. Required evidence is a documented getter slot
contract or a sanitized same-session mapping of raw getter slots to independently
observed equipment positions and bag positions.

### Visual inspection record

At the available browser size, the operator LAN reference displayed two characters
in the “nukers” group with populated inventories and equipment. The deployed PhMon
tab displayed four characters in its “Nukers 4” group, showed stale data after the
backend restart, and reported `0/0 equipped` with no equipment observation for the
first character. The tab did retain inventory counts/icons and the Party Setup
read-only blocker. A temporary 1440×1000 browser viewport could be applied to the
reference tab, but not to the deployed PhMon tab; the displayed captures are
therefore not a same-viewport acceptance comparison. No current plugin/browser
reload, character action or item operation was performed to force fresh data.

## Pet item-detail and pickup evidence — 2026-10-03

| Path | Current implementation | Evidence boundary |
| --- | --- | --- |
| Pet membership and slots | `get_pets()` continues to define current pet IDs, type, provided inventory, occupied slots and basic item facts. Pet item rows share the existing resource persistence and item resolver. | Operator reports phBot 20.1.2/plugin 1.9.12/protocol 13 and a Pick pet with 56 slots/five basic items. No sanitized raw object is checked in. Missing inventory, empty inventory, unavailable observations, stale retained rows and basic contents awaiting detail remain distinct. |
| Item detail merge | Resource and event item details merge verified fields individually. Ordered blue values retain duplicates and zeros; API and packet conflicts are not silently replaced. Historical event snapshots are enriched from the saved occurrence without reading current item state. | Go resource tests cover active-profile rarity classification, API/packet partial merges, duplicate blue options and immutable historical payloads. Numeric option meanings, unsupported family formulas and max durability remain unavailable without evidence. |
| Pet packet layouts | Workspace plugin **1.9.14** / protocol **14** reports per-session `0x30C8` and `0xB034` packet counts and min/max/latest sizes in `item_enrichment`; the Pet tab displays them. The probe retains no payload bytes and attaches no pet packet detail. `0xB034` continues to invalidate character item enrichment. | This is presence evidence only, not a decoder. Packet branch, rental/binding layout, supported protocol and pet-family coverage remain unverified against the installed runtime. Pinned RSBot code is corroboration only. Pick, Transport, Fellow and other reported types still need separate runtime evidence. |
| Pickup event contract | The worker helper freezes the item snapshot and carries pet ID/slot, positive quantity delta, packet observation ID/sequence and decoder version through the durable exact-retry spool. Go validates protocol-14 `item.acquired` source semantics. | Tests cover receipt bounds, frozen snapshots and retry behavior. Production decoding is gated off, so no runtime pickup receipt is currently emitted; startup snapshots and state differences do not become pet-pickup events. |
| Normal/Rare feeds | Migration `000022` stores nullable profile-derived classification/version. REST and live filters include classified pet receipts only on explicit opt-in for Normal/Rare feeds, before count/pagination. The UI opts in on both drop tabs; unknown class remains in All. | Disposable PostgreSQL integration verifies callback rows plus rare/normal/unknown pet records, combined counts, cursor pagination and filter rejection outside the two feeds. It uses synthetic test identities and is not a real pickup. |
| Callback drop detail | Plugin 1.9.16 reads a fresh inventory model count at a drop callback and briefly resamples resources. A unique new same-model inventory slot can link its recorded item snapshot through `drop_event_id` on `item.acquired` or `item.quantity_increased`. The server verifies the same agent/session/character/model and a 30-second window. A missing explicit link can use a unique unlinked `item.acquired` within three seconds and nearby observed position, with no competing drop. The 2026-10-03 18:44:10Z necklace callback has a same-session/model/position acquisition one second later with white rolls 87%/32% and absolute absorption 23.4/23.1. | Neither association proves pickup cause. Ground-only or ambiguous drops still lack exact rolls; a historical callback can only gain details if its observed inventory item was also saved. The necklace item has no observed blue options. Native runtime explanation of its missing explicit link and PostgreSQL integration of the read-time fallback remain open. |

The current browser check shows the existing Pet tab rendering a mounted Horse with
an explicitly empty inventory and a Pick inventory with two occupied slots, 56
reported slots, owner/type/ID and pending detail status. At 1440×1000, 1280×800 and
390×844 there was no document-width overflow and no browser console error. A later
connection recovery gap left event rows in the intentional loading state, so the
combined row contents are evidenced by the disposable PostgreSQL test, not by that
browser feed capture. The public demo's 2026-10-03 Rare Drops table exposes Item,
Time, Character, Location and Map; Normal Drops exposes Item, Time, Character and
Location. The demo's Stats view had no character and therefore did not expose a Pet
tab for fresh comparison. The historical supplied Stats capture remains the layout
baseline. The screenshots from this task were stored under `/tmp` and are not added
to the repository because they contain live local character/item data.

Full detailed inventory and pickup acceptance remains open until matching Windows/
phBot packet evidence and naturally observed pickup/retry behavior are verified. The
1.9.14 upload is limited to a passive count/size probe so the live packet families
can be confirmed before enabling a parser.
