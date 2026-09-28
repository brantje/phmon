# Character portrait model mapping

## Sources

- phBot documents `get_character_data()` as returning a `model` integer. PhMon
  collects that field when it is a non-boolean integer from 1 through `0xffffffff`;
  older plug-ins that omit it continue to send valid state frames.
  [phBot character API](https://plugins.phbot.org/phbot-api/character)
- The operator authorized static inspection of
  `%USERPROFILE%\Downloads\phMonitor-v0.5.0.exe`. Its
  `charPortraitAssetByModel` routine maps the model ranges below to portrait
  filenames. This records the mapping only; no executable or decompiled source is
  included in the repository.
- The existing portrait images were exported from the operator's local
  GreatestSRO `Media.pk2` and are served from PhMon's local
  `/game-assets/interface/character/` tree. No remote asset host is used.

## Verified model ranges

For each range, the portrait number is `model - first model + 1`. EU models have
two verified model ranges for the same 13 image files.

| Race | Gender | Model IDs | Portrait files |
| --- | --- | ---: | --- |
| Chinese | Man | 1907–1919 | `char_ch_man1.png`–`char_ch_man13.png` |
| Chinese | Woman | 1920–1932 | `char_ch_woman1.png`–`char_ch_woman13.png` |
| European | Man | 14717–14729, 14875–14887 | `char_eu_man1.png`–`char_eu_man13.png` |
| European | Woman | 14730–14742, 14888–14900 | `char_eu_woman1.png`–`char_eu_woman13.png` |

`tools/game-data-exporter/src/phmon_game_exporter/portrait_mapping.py` is the
independent implementation of these verified ranges. The exporter maps a model
only when the source entity code has the matching race and gender prefix and the
corresponding local portrait file exists. It records the join on the entity row and
in `audit/tables/portrait-model-joins.json`. Unknown IDs, code mismatches, and
missing files remain unmapped; no other entity art is treated as a character face.

The generated server profile's `character_portraits` catalog is keyed by the
exported numeric model ID. The API resolves it through the character's normalized
server-to-game-data profile, so a profile without a verified entry produces no
portrait URL and the UI uses initials.

## Runtime behavior

The plug-in reports `model` in character snapshot and state frames. The server
validates and persists it in `characters.model_id`. A newly claimed session clears
the prior model until a fresh observation arrives; ending a session preserves the
last observation for an offline character. Existing agents can omit the field.
Character, group, live, detail and death-event responses expose `model_id` and a
local `portrait_url` when the active profile resolves one.

`CharacterPortrait.vue` handles Stats, character detail and death/event identity.
It renders a same-origin local image and switches to the character's initial if the
model is unmapped, the URL is absent, or the image load fails.

## Coverage

Coverage counts for the active `gamedata-47c969ded0613d4c2a22` profile and visual
checks at 1440 × 1000, 1280 × 800 and 390 × 844 are recorded in
[`reference-parity.md`](../reference-parity.md). Other game-data profiles need their
own exported entity/model joins; unsupported models retain the initials fallback.
