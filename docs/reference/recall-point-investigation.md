# Designate Recall Point evidence — 2026-10-04

## Current finding

The action is visible in the [official phBot map guide](https://guide.phbot.org/phbot/map) under a teleporter. The [official plugin API index](https://plugins.phbot.org/phbot-api) has no dedicated recall-point function. Its [teleport API](https://plugins.phbot.org/phbot-api/teleport) only resolves known source/destination pairs and sends no packets. The [script command](https://guide.phbot.org/phbot/script-commands) named `recall` recalls a pick pet. The [packet injection API](https://plugins.phbot.org/phbot-api/packet-injection) documents a generic `inject_joymax(opcode, data, encrypted)` function, without an opcode or payload for this action.

The operator's phBot Stable installation is `%LOCALAPPDATA%\Programs\phBot Stable`, file version **20.1.3.0** on 2026-10-04. Four phBot processes and game clients were running. This inspection was read-only; no character was selected or operated. The previous live symbol-name probe on plugin 1.9.2 found no callable recall/menu API. The community `0x7059` / 4-byte NPC-ID proposal in [Issue #32](issue32-teleporter-investigation.md) remains unverified on this installation.

Static byte searches of the operator-authorized `%USERPROFILE%\Downloads\phMonitor-v0.5.0.exe` for `Designate Recall Point`, `designateRecall`, `recallPoint`, `recall_point`, `0x7059`, `28761` and `CLIENT_SAVE_PLACE` returned no readable hit. This does not prove the executable lacks that behavior. Searches of the installed phBot executable for the English label in UTF-8 and UTF-16LE likewise returned no hit; it may be compressed or localized.

## Capture path

`tools/recall-point-capture/RecallPointCapture.py` is a temporary read-only phBot plugin. It arms for 15 seconds, records a bounded `get_npcs()` gate list, candidate outgoing `0x7045`/`0x7059` packet bytes and other outgoing 0x70xx opcode/length metadata, and incoming opcode/length metadata. It always forwards packets and never sends one. It is **not installed** as of this entry.

The official [callback documentation](https://plugins.phbot.org/phbot-api/events) says `handle_silkroad` receives packets from the game client. A manual action initiated by phBot's own map may bypass that callback. An empty capture would therefore be inconclusive. The next evidence needed is an operator-designated test character/gate, one manual designation, the capture lines if observable, and the visible phBot/game result. If the callback cannot observe the action, choose another authorized capture path rather than guessing packet bytes.

## Implementation gate

Until a capture or dedicated API establishes versioned semantics, `character.recall_point.designate` must remain unsupported in the plugin capability frame. The Go catalog and frontend intent may validate the proposed narrow gate identity and confirmation, but neither may send an unverified opcode or claim the recall point was saved. No current-recall-point state should be stored without a verified readback.
