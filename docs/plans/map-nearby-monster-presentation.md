# Plan: nearby monster icons, names, and normalized rank labels

Baseline: `main` at `2d8d1e3bc9a914a5117de01e12186ad8f0d68847` (PR #44 merged).
Working branch: `fix/map-monster-icons`.

## Requested behavior

- Keep the existing live monster HP bubbles and their current HP/attacking/size behavior.
- Add the appropriate monster rank icon to each live monster bubble.
- For party monsters, compose the matching rank icon with the shared party badge exported by PR #44; do not treat the party badge as rank artwork.
- Add an opt-in **Show nearby monster names** control under the **Current nearby monsters** layer entry.
- When that control is enabled, show the monster name next to/below the existing bubble. Default it off so the map remains uncluttered unless requested.
- Do not show user-facing monster UI text labeled `Type` / `Mob type`. Present the normalized rank name directly.
- Normalize the six supported values as:
  - 0 → General
  - 1 → Champion
  - 4 → Giant
  - 16 → General (Party)
  - 17 → Champion (Party)
  - 20 → Giant (Party)
- Keep internal transport/query field names such as `type_code`, `monster_type`, and the heatmap layer id `mob_types` unchanged.

## Implementation steps

1. **Centralize monster presentation metadata**
   - Extend `web/app/utils/mapMarkerPresentation.ts` so the existing numeric type-code mapping owns:
     - normalized label;
     - scale;
     - party flag;
     - rank icon URL;
     - optional party badge URL.
   - Reuse the committed PR #44 assets under `/game-assets/monster-types/`.
   - Keep unknown type codes safe: retain the HP bubble, show no fabricated icon, and use a neutral unknown label only where text is required.

2. **Preserve the bubble and layer in artwork**
   - Update `web/app/components/MapCanvas.vue` so monster marker content remains `phmon-map-monster-bubble`.
   - Add the rank icon inside the bubble rather than replacing it.
   - For party variants, overlay the shared party badge in a small corner while retaining the corresponding General/Champion/Giant rank icon.
   - Keep the current HP ring, attacking highlight, party styling, marker scaling, animation, popup behavior, hit target, and z-order unchanged unless a small CSS adjustment is required to fit the artwork.

3. **Add opt-in monster name labels**
   - Add `showNearbyMonsterNames = ref(false)` in `web/app/pages/map.vue`.
   - Under the existing **Current nearby monsters** layer toggle, add a subordinate checkbox labeled **Show nearby monster names**.
   - Pass that presentation state into marker rendering without changing live-map payloads.
   - When enabled, render `monsterDisplayName(monster)` as a compact map label attached to the monster marker; when disabled, render no map name label.
   - The popup and side list continue showing the monster name regardless of the map-label toggle.

4. **Remove user-facing monster “Type” wording**
   - Popup: remove `Type` as a row label and surface the normalized rank directly, for example in the monster subtitle/header or a rank row labeled `Rank`.
   - Nearby monsters list: change `Lv. X · Type: Champion · ...` to `Lv. X · Champion · ...`.
   - Historical filter: rename `Mob type` to `Monster rank` and display normalized rank labels while preserving the raw `monster_type` query value.
   - Historical layer label: rename `Mob type sightings` to `Monster rank sightings`.
   - Audit monster-facing frontend copy so no visible monster classification uses the word `Type`.

5. **Keep historical/API compatibility**
   - Do not migrate stored observations or rename API fields.
   - Convert historical facet values to display labels only at the frontend boundary.
   - Query/reset requests must continue sending the original raw `monster_type` value so existing historical data and reset semantics remain compatible.

6. **Tests**
   - Extend `web/tests/mapMarkerPresentation.test.ts` for all six normalized names and icon mappings.
   - Verify party variants map to the correct rank icon plus shared party badge.
   - Verify unknown values never claim a known icon/rank.
   - Add/extend map UI tests to verify:
     - name labels default off;
     - **Show nearby monster names** enables them;
     - the existing bubble remains present;
     - the correct icon(s) are rendered;
     - popup/list/filter text uses normalized rank names and no monster-facing `Type` wording.
   - Keep existing dedupe, scaling, HP and map placement tests green.

7. **Visual verification**
   - Check General, Champion, Giant and all three party variants on the live map.
   - Verify names remain readable without hiding HP state or causing excessive overlap.
   - Verify at the project’s standard desktop/mobile widths and with names both on and off.

## Expected files

Primary:
- `web/app/utils/mapMarkerPresentation.ts`
- `web/app/components/MapCanvas.vue`
- `web/app/pages/map.vue`
- `web/app/utils/mapHeatmap.ts`
- `web/tests/mapMarkerPresentation.test.ts`

Potential additional frontend test files only if needed for the toggle/rendering coverage.

No plugin, Go server, database migration, protocol-version, or exported-asset change should be required.
