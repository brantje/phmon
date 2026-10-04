# Designate Recall Point evidence — 2026-10-04

## Current finding

The action is visible in the [official phBot map guide](https://guide.phbot.org/phbot/map) under a teleporter. The [official plugin API index](https://plugins.phbot.org/phbot-api) has no dedicated recall-point function. Its [teleport API](https://plugins.phbot.org/phbot-api/teleport) only resolves known source/destination pairs and sends no packets. The [script command](https://guide.phbot.org/phbot/script-commands) named `recall` recalls a pick pet. The [packet injection API](https://plugins.phbot.org/phbot-api/packet-injection) documents a generic `inject_joymax(opcode, data, encrypted)` function, without an opcode or payload for this action.

The operator's phBot Stable installation is `%LOCALAPPDATA%\Programs\phBot Stable`, file version **20.1.3.0** on 2026-10-04. The previous live symbol-name probe on plugin 1.9.2 found no callable recall/menu API. The community `0x7059` / 4-byte NPC-ID proposal in [Issue #32](issue32-teleporter-investigation.md) has now been confirmed for the observed Greatest session, subject to the result limitation below.

Static byte searches of the operator-authorized `%USERPROFILE%\Downloads\phMonitor-v0.5.0.exe` for `Designate Recall Point`, `designateRecall`, `recallPoint`, `recall_point`, `0x7059`, `28761` and `CLIENT_SAVE_PLACE` returned no readable hit. This does not prove the executable lacks that behavior. Searches of the installed phBot executable for the English label in UTF-8 and UTF-16LE likewise returned no hit; it may be compressed or localized.

## Capture path

`tools/recall-point-capture/RecallPointCapture.py` is a temporary read-only phBot plugin. It arms for 15 seconds, records a bounded `get_npcs()` gate list, candidate outgoing `0x7045`/`0x7059` packet bytes and other outgoing 0x70xx opcode/length metadata, and incoming opcode/length metadata. It always forwards packets and never sends one. The operator installed it and ran two manual Hotan captures on Auren at Greatest.

At 21:04:45 the probe listed `id=4`, `servername=GATE_KT`, `region=23687`. The manual action sent `0x7059` with `04000000` and received `0xB059` length 1. An earlier `0x7045` length 38 was also seen in this window. At 21:11:26 the operator repeated the manual designation: the **only** outgoing 0x70xx packet was `0x7059` with `04000000`, followed by `0xB059` length 1. This shows no `0x7045` selection is required for this action in the tested session. The operator reported the in-game message that the recall point was updated; phBot showed no separate message. The `0xB059` payload was not logged, so its one-byte success/failure semantics remain unknown.

The official [callback documentation](https://plugins.phbot.org/phbot-api/events) says `handle_silkroad` receives packets from the game client. Here it observed the manual phBot map action. The [packet injection API](https://plugins.phbot.org/phbot-api/packet-injection) documents `inject_joymax(opcode, data, encrypted)`; the captured request establishes the specific opcode and payload for this tested build/server. The [community xControl implementation](https://github.com/JellyBitz/phBot-xPlugins/blob/master/xControl.py) independently uses `get_npcs()` and `0x7059` with a four-byte gate ID, without preceding selection.

## Implementation gate

PhMon may submit the captured fixed request only on phBot **20.1.3** and server **Greatest**, after resolving the exact gate in the target character's fresh `get_npcs()` snapshot. Other builds/servers remain unsupported. `inject_joymax` submission is recorded as `completed` with verification `unverified`; the UI says the request was sent and does not claim the recall point was saved. The manual `0xB059` response and game message verify the manual test, not a later PhMon-issued command. A response-byte capture or readback is needed to classify automated saves as confirmed. No current-recall-point state is stored. The next runtime check is one PhMon-issued command after the updated plugin is installed, with its command result and game message observed.
