# Issue #32 — Teleporter capability investigation

Date: 2026-10-02  
Scope: investigation spike for [#32](https://github.com/brantje/phmon/issues/32); implementation contract for [#33](https://github.com/brantje/phmon/issues/33).  
Parent: [#28](https://github.com/brantje/phmon/issues/28). Live gate observations: [#24](https://github.com/brantje/phmon/issues/24) / protocol v9 `map.npcs`.

This document is the decision-complete contract. It does not authorize map UI, fan-out commands, or live teleports in this spike.

## Executive summary

| Question | Chosen answer |
| --- | --- |
| How is a nearby teleporter identified? | Per-session `get_npcs()` row with `servername` matching `^GATE_[A-Za-z0-9_]+$`; runtime dictionary key is that character’s NPC id (PhMon `map.npcs` role `teleporter`). |
| How are destinations enumerated? | **Unsupported.** No public phBot API or inspected community plugin lists a gate’s live menu. |
| How is a destination resolved? | `get_teleport_data(source, destination)` when the operator or UI supplies both names; tuple index `1` is the reference teleport id when present. `None` means no route on this server. |
| How is a teleport executed (PhMon plan)? | One validated script line `teleport,{source},{destination}` via `start_script` after pair resolution and live-gate checks. **Verified once** on operator phBot (plugin 1.9.3, Hotan→Jangan); broader routes/servers and PhMon remote commands remain unimplemented. |
| Designate Recall Point | **Unsupported** for PhMon until an authorized capture validates community opcode `0x7059`. |
| Persistent teleporter catalog | **Rejected** (exporter graph, phBot locale SQLite, city lists). |

## Live teleporter identity (already implemented)

PhMon plugin `collect_npc_observation()` and Go `server/internal/npcs` already implement issue #24:

- Source: documented [`get_npcs()`](https://plugins.phbot.org/phbot-api/npc).
- Teleporter classification: `servername` matches `GATE_*` (same rule as phBot’s Jangan example `GATE_CH`).
- Runtime id: the dictionary key from `get_npcs()` for that session only.
- Cross-session map deduplication merges markers for display; **#33 must never reuse another character’s runtime id**.

Required session inputs for any later action:

- Current character session / `expected_session_id`
- Observer’s own `map.npcs` row for the chosen logical gate (id, name, servername, region, position freshness)
- Explicit destination label (name or server name) because enumeration is unsupported

## Public phBot API

| API | Role | Sends packets? |
| --- | --- | --- |
| [`get_teleport_data(source, destination)`](https://plugins.phbot.org/phbot-api/teleport) | Resolve a known pair; returns `None` or a tuple whose second value is the teleport code/id | No |
| [`get_npcs()`](https://plugins.phbot.org/phbot-api/npc) | Nearby NPCs and gates; keys are runtime ids | No |
| [`teleport,source,destination`](https://guide.phbot.org/phbot/script-commands) script command | Documented in-game teleporter use | Via phBot script engine (not raw plugin injection in PhMon plan) |
| [`start_script(str)`](https://plugins.phbot.org/phbot-api/script) | Execute script text | Yes (bounded catalog in PhMon) |
| `inject_joymax` / `inject_silkroad` | Packet injection | Yes — **out of scope for PhMon implementation** |
| `recall` script command | Recalls **pick pet** | Not Designate Recall Point |

phBot’s [map guide](https://guide.phbot.org/phbot/map) documents a right-click menu with Teleport, **Designate Recall Point**, and Jupiter room entries. No plugin API in the public index exposes that menu tree or grouping.

## Community plugin evidence (third-party, not PhMon behavior)

These patterns explain how players automate teleports today. PhMon records them; it does not copy packet injection.

### Destination enumeration

Inspected plugins (**xControl**, **EnterVicious**, **xNPC**, **Vette1123/phbot-plugins**) **do not** enumerate a gate’s destination list. They require the caller to already know `source` and `destination` strings (chat command `TP A B`, hardcoded dungeon pair, or script `teleport,Jangan,Donwhang`).

**xNPC** only lists nearby NPC names, server names, models, and unique ids for operator reference.

**xBattleInfinity** reads phBot’s locale SQLite `teleport` table (`SELECT * FROM teleport WHERE servername LIKE ?`). That is packaged client data, locale-specific (`iSRO.db3`, `TRSRO.db3`, or server-matched file), and equivalent to a static catalog. Issue #33 forbids introducing a persistent PhMon catalog; this table is not the live gate menu.

### Pair resolution and execution (community)

Typical flow in [xControl `inject_teleport`](https://github.com/JellyBitz/phBot-xPlugins/blob/master/xControl.py):

1. `t = get_teleport_data(source, destination)` — if `None`, abort.
2. Find `source` in `get_npcs()` by `name` or `servername`; use dict key as NPC unique id.
3. `inject_joymax(0x7045, struct.pack('<I', npc_id), False)` — select object.
4. After ~2s: `inject_joymax(0x705A, struct.pack('<IBI', npc_id, 2, t[1]), False)` — teleport type `2` + reference id.

[EnterVicious](https://github.com/Bunker141/Phbot-Plugins/blob/master/EnterVicious.py) uses the same `0x7045` / `0x705A` sequence plus dungeon-specific `0x3080` confirmation.

Archived [SilkroadDoc `AGENT_TELEPORT_USE`](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/AGENT_TELEPORT_USE) describes `0x705A` payload: uint NPC unique id, byte teleport type (`2` = named destination + uint `RefTeleportID`).

[ProjectHax thread](https://forum.projecthax.com/t/get-teleport-data-does-not-return-anything/7854): manual UI emits `0x7045` then `0x705A`; `get_teleport_data` returned nothing on a custom map.

### Designate Recall Point (community, unverified for PhMon)

[xControl `RECALL #Town`](https://github.com/JellyBitz/phBot-xPlugins/blob/master/xControl.py): resolve NPC display name via `get_npcs()`, then `inject_joymax(0x7059, struct.pack('I', npcUID), False)`.

ElitePvPers opcode lists name `0x7059` as `CLIENT_SAVE_PLACE`. This spike has **no PhMon-authorized packet capture** on the operator’s phBot build. Recall stays **unsupported** in PhMon until capture + version scoping exist.

### PhMon chosen execution path (issue #33)

Prefer the **documented script command** over copying `0x7045` / `0x705A`:

1. Confirm target character session and capability (`get_npcs`, `get_teleport_data`, `start_script`).
2. Confirm the logical gate is still present in that session’s live `map.npcs` snapshot (matching id or name/servername).
3. `get_teleport_data(source, destination)` must return a tuple (destination supplied explicitly by UI).
4. Build exactly one line: `teleport,{source},{destination}` with comma-free, bounded labels (reject commas/newlines).
5. Pass only that line to `start_script` through the existing audited command lifecycle.

Community packet sequence remains the cross-check for debugging, not the production implementation.

## Labels and groups

phBot’s map UI shows destination **groups/submenus**. No inspected API or plugin reproduces grouping. PhMon must not invent groups. If a future phBot release exposes a callable menu API, revisit enumeration; until then, flat explicit destination strings only.

## Static game-data exporter (rejected as menu)

GreatestSRO `teleportdata.txt` / `teleportlink.txt` are exported in `tools/game-data-exporter` for reference ids, codes, names, and endpoint links. Fees, eligibility, submenu layout, and live private-server drift are unresolved. **Not** used for issue #33 destination menus.

## Capability probe (plugin 1.9.2)

Operator action: PhMon QtBind **Probe teleporters** (read-only).

- Lists `phBot` module symbol **names** matching `teleport|npc|gate|recall|location|script` (no calls except `get_teleport_data`).
- Reports `get_npcs` / `get_teleport_data` / `start_script` presence.
- From current `GATE_*` rows, runs at most **16** `get_teleport_data` calls including one unknown-destination control pair.
- Never calls `inject_joymax`, `start_script`, `generate_script`, or movement/disconnect APIs.

Results are written to the phBot log and a short status line. No backend frame or database row.

**Runtime gate:** Read-only probe and one operator **Test Hotan→Jangan** on installed phBot (below). PhMon remote `#33` commands and packet injection were not part of this spike.

### Operator live probe — 2026-10-02 10:53 UTC

Captured from phBot log after **Probe teleporters** on plugin **1.9.2** while standing at the Hotan gate (`phMonitorAdapter` was also loaded; probe output is from PhMon only).

```json
{
  "capabilities": {
    "get_npcs": true,
    "get_teleport_data": true,
    "start_script": true
  },
  "enumeration": "unsupported",
  "errors": [],
  "execution": "documented_script_command_unverified",
  "gates": [
    {
      "id": "4",
      "name": "Hotan",
      "servername": "GATE_KT"
    }
  ],
  "npc_observation": "observed",
  "pair_tests": [
    {
      "classification": { "result": "none" },
      "destination": "__phmon_probe_unknown_destination__",
      "gate_id": "4",
      "source": "Hotan"
    },
    {
      "classification": { "result": "none" },
      "destination": "GATE_KT",
      "gate_id": null,
      "source": "Hotan"
    },
    {
      "classification": { "result": "none" },
      "destination": "__phmon_probe_unknown_destination__",
      "gate_id": "4",
      "source": "GATE_KT"
    }
  ],
  "plugin_version": "1.9.2",
  "recall": "unsupported",
  "status": "ok",
  "symbols": [
    { "callable": true, "name": "generate_script" },
    { "callable": true, "name": "get_gateway" },
    { "callable": true, "name": "get_npc_goods" },
    { "callable": true, "name": "get_npcs" },
    { "callable": true, "name": "get_teleport_data" },
    { "callable": true, "name": "set_training_script" },
    { "callable": true, "name": "start_script" },
    { "callable": true, "name": "stop_script" }
  ]
}
```

**Interpretation (not a teleport attempt):**

- Confirms **no destination-list / recall / menu** symbol appeared in the probe’s name filter; only pair-resolution and script helpers (`get_teleport_data`, `start_script`, `generate_script`, etc.).
- One live gate: runtime id **`4`**, display name **Hotan**, server name **`GATE_KT`** — matches PhMon’s `GATE_*` teleporter rule.
- All three automated `get_teleport_data` checks returned **`none`**: expected for the unknown-destination control; **`Hotan` → `GATE_KT`** is not a valid player destination (that string is the gate’s own server name, not a target city). Issue #33 must use an explicit destination such as another town name (`Jangan`, `Donwhang`, …) and treat `none` as “no route” before any script line runs.
- **`get_gateway`** appeared in symbol discovery only; this spike does not call it. Revisit only if a future probe authorizes safe read-only inspection.

### Operator script test — 2026-10-02 10:57 UTC (plugin 1.9.3)

QtBind **Test Hotan→Jangan** at the Hotan gate (`GATE_KT`):

```text
PhMon Hotan→Jangan test line=teleport,Hotan,Jangan code=1 start_script=True
Script: Teleporting
```

**Interpretation:**

- `get_teleport_data('Hotan', 'Jangan')` returned a tuple whose second element was **`1`** (reference teleport id for this server build).
- `start_script('teleport,Hotan,Jangan')` returned **`True`**; phBot’s script engine logged **`Teleporting`** (~2s later).
- This validates the **documented script-command execution path** for this single pair on the operator’s Greatest/runtime profile. It does not verify other destinations, `GATE_KT`/`GATE_CH` form, multi-character fan-out, or arrival/position readback.
- Issue #33 should reuse the same sequence: live gate present → `get_teleport_data` → one bounded `teleport,source,destination` line → interpret `start_script` result and optional `teleported()` / position change separately.

**Still open for #33:** remote command type, eligibility UI, per-character gate ids, additional destination strings, and recall (`0x7059`).

## Fail-closed rules

- Missing `get_npcs` or `get_teleport_data` → teleporter actions disabled.
- Gate not in current session snapshot → skip character (#33 eligibility).
- `get_teleport_data` → `None` → skip; do not guess codes or destinations.
- Another character’s runtime NPC id or resolved tuple → never used.
- Custom/private servers may return `None` for valid-looking names (documented forum case).

## Issue #33 implementation contract (not in this PR)

- Reuse [#29](https://github.com/brantje/phmon/issues/29) action targets and [#30](https://github.com/brantje/phmon/issues/30) fan-out; one `character.*` command per eligible child; no `group.teleport`.
- UI may show destination only as **operator-entered or pre-known string** (same as `TP source dest`), with eligibility preview (online, session, gate observed, pair resolves).
- **Omit** Designate Recall Point and Jupiter room until supported mechanisms exist.
- **No** durable teleporter/NPC catalog tables.

## Evidence index

| Artifact | Use |
| --- | --- |
| [phBot Teleport API](https://plugins.phbot.org/phbot-api/teleport) | Pair resolution |
| [phBot NPC API](https://plugins.phbot.org/phbot-api/npc) | Live gates |
| [Script commands — teleport](https://guide.phbot.org/phbot/script-commands) | PhMon execution plan |
| [phBot map guide](https://guide.phbot.org/phbot/map) | UI features without API |
| [xControl.py](https://github.com/JellyBitz/phBot-xPlugins/blob/master/xControl.py) | TP / RECALL / injection pattern |
| [SilkroadDoc AGENT_TELEPORT_USE](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/AGENT_TELEPORT_USE) | `0x705A` layout |
| `plugin/PhMon.py` `probe_teleporter_capabilities` | Operator probe |
| `docs/phbot-capabilities.md` § Teleporter investigation | Ledger entry |
