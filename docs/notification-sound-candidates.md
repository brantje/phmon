# GreatestSRO notification sound candidates

The operator requested a bounded, local search for in-game sounds suitable for
PhMon's browser notification kinds on 2026-10-17. The existing read-only PK2
reader inspected the operator-supplied `Data.pk2`, `Media.pk2` and `Music.pk2`.
`Data.pk2` contains 2,982 WAV files (78 in `prim/snd/ui/`); `Media.pk2` has no
WAV/OGG files; `Music.pk2` contains 56 OGG background tracks. No source archive
was changed. This research does not change the main game-data exporter's
sound-excluded bundle contract or install sounds into PhMon.

The ignored, local review folder is `exports/notification-sound-review/`:

- `index.html` has click-to-play previews, volume control, search and a proposed
  assignment for every notification kind.
- `audio/` holds 87 byte-for-byte extracted WAV files (78 UI and 9 related effects).
- `manifest.json` records source entry, size, SHA-256, PCM format and duration;
  `recommendations.json` records the suggested mapping. All 87 files are readable
  mono PCM, and the review page refers only to files in the manifest.

Open `index.html` locally in a browser to audition. These raw client assets remain
ignored and must not be committed or redistributed by default. The preview page
does not send commands, notifications or audio to other people.

## Direct game-source evidence

The exported client table `web/public/game-assets/textdata/effectsound.txt`
maps `SND_DROPITEM/RARE` to `ui/itfly_rarebox.wav`, `SND_PICKUP` to
`ui/itpickup.wav`, `SND_LEVUP` to `ui/itlevelup.wav`, `SND_WARNING` to
`ui/biff.wav`, `SND_ELIXIR_USE/SUCCESS/FAILURE` to the matching `elixir_*.wav`
files, and `SND_COS_SUMMON` to `skill/csk_heal_ready_a.wav`. It also maps
`SND_DROPITEM/GOLD` to `ui/itgold.wav`. The `SND_DROPITEM/BAG` and `BOX` rows
and the `SND_WARP` row are commented out in this table; those files are audible
candidates, not evidence that the client currently plays them for those events.
The source table does not name a chat-message or unique-spawn cue.

## Proposed browser assignments

| Notification kinds | First audition | Basis / limit |
| --- | --- | --- |
| `drop.rare` | `ui/itfly_rarebox.wav` (2.44 s) | Direct rare-drop mapping. |
| `item.acquired`, `item.quantity_increased` | `ui/itpickup.wav` (0.28 s) | Direct pickup mapping; quantity increase can have other causes. |
| `character.level_up` | `ui/itlevelup.wav` (5.41 s) | Direct level-up mapping; needs a shorter browser edit if selected. |
| `character.attacked`, `job.thief_seen`, `pet.transport_died` | `ui/biff.wav` (0.31 s) | Direct warning cue, proposed for these PhMon alerts. |
| `alchemy.attempt` | `ui/elixir_use.wav` (2.99 s), then `ui/elixir_suc.wav` (4.86 s) or `ui/elixir_fail.wav` (3.54 s) by observed result | Direct game alchemy cues; result-based assignment would need a browser implementation. |
| `trade.completed` | `ui/itgold.wav` (0.60 s) | Direct gold-drop cue; proposed sale/completion analogue. |
| `drop.item` | `ui/itfly_bag.wav` (0.57 s) | Bag-drop row is commented out; verify by listening. |
| `pet.summoned` | `skill/csk_heal_ready_a.wav` (3.99 s) | Direct COS summon mapping; long for an alert. |
| `pet.dismissed` | `ui/buf_disappear.wav` (0.56 s) | Suggested disappearance effect. |
| `session.disconnected`, `trade.failed` | `ui/error.wav` (0.60 s) | Short error cue; event assignment is a suggestion. |
| `session.connected`, `job.hunter_trader_seen`, `chat.message_received`, party/academy joins | `ui/uiwinopen.wav` (0.30 s) | Soft, short opening cue; no dedicated client cue for these PhMon events was found. |
| party/academy leaves | `ui/uiwinclose.wav` (0.26 s) | Soft, short closing cue. |
| `session.joined_game` | `ui/gatein.wav` (1.80 s) | Portal sound; the warp table row is commented out. |
| `session.teleported` | `skill2/usk_wizard_light_teleport_a.wav` (0.74 s) | Named skill teleport effect, not a general teleport mapping. |
| `character.died` | `common/deathimpact_big_dirt.wav` (1.43 s) | Impact effect; audition against the shorter warning beep. |
| `world.unique_spawned`, `world.gm_spawned` | `ui/alarm_sound.wav` (4.89 s) | Attention cue; long and no direct source mapping found. |
| `alchemy.finished`, `academy.member_graduated` | `ui/eventcomplete.wav` (3.05 s) | Generic completion cue. |
| `item.transferred`, `item.quantity_decreased` | `ui/itbag.wav` (0.30 s) | Brief bag/inventory cue. |

The local `recommendations.json` covers the remaining exact kind-to-file rows and
notes. These assignments are **proposals**, not a claim that PhMon events trigger
the same sound in the game. Auditory selection remains for the operator; the
extractor verified file identity, format and duration, but did not judge how each
clip sounds. Before browser integration, decide which long cues to trim, set
per-kind volume/sound preferences, and handle overlapping events and browser
autoplay rules. The current PhMon browser still only has the existing optional
hidden-tab chat sound.
