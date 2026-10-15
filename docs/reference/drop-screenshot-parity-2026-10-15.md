# Drop table, compact tooltip and Global Offers correction — 2026-10-15

This is a focused correction to the operator's supplied phMonitor screenshots,
not full Slice 13 acceptance. It supersedes the double-card presentation in the
earlier [tooltip regression repair](item-tooltip-regression-2026-10-15.md).

## Supplied reference and concrete differences

The operator supplied the Rare Drops table both with and without a Leopard Casque
popup, the corresponding PhMon captures, and the unwanted first half of PhMon's
popup. The earlier Gem Necklace capture is the tooltip baseline. The reference
has approximately 67 px rows, 45 px item frames, 13 px table text, a 36 px table
header, 39 px tabs, two 184 px text filters and two 150 px date filters. Dates are
readable local calendar dates; map actions have a small gold square treatment.

PhMon had 42 px rows, 32 px icons, 11 px text, workspace-wide filter inputs,
raw UTC timestamps and an extra 'All rare drops' heading. Its popup repeated the
same item above and below an explanation of the observation relationship.

The live public demo was opened in a real browser at 1440×1000 and returned HTTP
200. Its recurring overlay blocked the navigation attempt; the operator's supplied
captures remained the visual baseline, without treating the connection popup as
an application requirement or backend investigation.

## Final presentation

- Rare/Normal Drop tables use the measured reference dimensions above, a corner
  count badge, compact filters, local calendar date formatting and gold map
  actions. The extra 'All rare/normal drops' text is removed. Mobile retains
  horizontal scrolling inside the table without whole-page overflow.
- The shared popup renders exactly one item model: name/seal/classification,
  available stats/rolls and durability, requirements, then cyan blue options.
  The repeated metadata/warning block, observation heading, long explanation and
  visible event UUID are removed. The default tooltip is 286 px wide with 12 px
  body text and a 15 px name. Stat grouping follows the reference's armor order.
- When the displayed source is a separate inventory observation, a small footer
  retains the meaningful 'pickup unverified' qualifier. Full source metadata stays
  in the response and source description; it is not promoted into the callback's
  canonical item, drop/acquisition metrics or durable history.
- Static catalog ranges retain a short 'Catalog ranges · rolls unavailable' label.
  Missing item values are not generated from the catalog or a different copy.
- The Gem Necklace with 48% / 9% rolls and freezing/electric-shock blues was found
  in retained history. Its sanitized item-only snapshot is
  `web/tests/fixtures/gem-necklace-tooltip.json`. The API reports physical absorption
  27.541934967041016 and magical absorption 27.348386764526367; PhMon displays 27.5
  and 27.3 with the observed rolls. The reference's inconsistent leading values
  and extra unlabeled raw floats are not copied as a new calculation.

## Concrete pet-data gap, still open

The reported Lepus Soul Fiend drop is callback
`ce7a12c1-df44-4f0b-b7f4-42e0ff9ca592`. Its separate pet observation is
`90bfbda4-b978-4e2f-a906-0b09b65d31b5`, one second later. The retained item and
current pet row contain durability 82 and basic identity/quantity only. Their
`api_field_types` contains name, model, quantity, remaining, durability, expiration
and servername; it contains no whites, blues or absolute attack fields.

The normalizer uses the same item evidence collector for bag and pet slots. This
specific missing detail is not a renderer omission. A different earlier bag copy
of model 11142 has rich fields, but its values cannot identify this pet item.
The installed probe still reports `vsro_pet_packet_probe_r1`, and production pet
packet decoding is gated off. General packet counts are not item layout proof.

The [documented pet getter](https://plugins.phbot.org/phbot-api/pets) provides basic
pet item examples. Completion requires per-instance runtime API evidence or
sanitized native pet packet fixtures that verify the relevant layout and identity
before enabling a decoder. No pet was moved and no character command was issued
to manufacture evidence. The compact card displays the observed durability and
an unavailable status; full pet-stat parity remains unclaimed.

## Related functional repairs

The table's item-name filter previously could not match a model-only callback.
The existing item catalog now resolves names to server/model keys before the
Events count and page query. It never assumes a numeric model identifies the same
item on another server and does not attach instance values.

The dashboard's Global Offers 'LATER' card now consumes the canonical retained
Global offer query over the shared `/api/live` socket (`offers` stream, server and
bounded limit only). It shows WTB/WTS/WTT, sender, text, observer attribution,
canonical source links, loading/empty/error/stale states and an Economy link.
Economy's sidebar entry now navigates to its implemented page. Independently
retained same-text observer messages remain separate; no new broadcast identity,
confirmed transaction or realized price is invented.

## Executed checks and artifacts

- PostgreSQL-backed race suites for Events, resources, HTTP API and chat passed
  using an isolated `phmon_drop_parity_checks` database, removed after validation.
  Tests cover catalog-name callback searches, server/model fencing and the new
  bounded offers subscription. Go vet passed for these packages and server main.
- Frontend unit tests, Nuxt typecheck, changed-file lint/Prettier, production Docker
  builds, whitespace and live transport audits passed. Existing HTML void-element
  style warnings remain; there are no changed-file lint errors.
- `scripts/drop_parity_browser_check.mjs` and the updated item detail browser check
  passed at 1440×1000, 1280×800 and 390×844 using retained real observations. They
  verify one popup identity, source stats/blues, the actual Gem Necklace, the basic
  pet gap, keyboard/touch/Escape, tooltip bounds, table dimensions, name filtering,
  live Global offers/server scoping and no overflow/browser exceptions.
- The existing Stack CI browser audit now requires a loaded live Global Offers
  card without 'LATER'. This updated remote CI gate has not been claimed executed
  before a commit/push. The read-only proxy and live transport checks passed locally.

Screenshots and JSON results: `/tmp/phmon-drop-parity-20261015`, including
`rare-drops-*`, `rare-tooltip-*`, `gem-necklace-*`, `pet-basic-item-*`,
`global-offers-*`, `parity-results.json` and `results.json`. Local server/web were
rebuilt/recreated; PostgreSQL and historical records were retained. No merge,
remote publication, notification, plugin/protocol change or bot command occurred.

Final frontend count: 266 passing tests. A final web rebuild hit disk capacity;
pruning only unused Docker build cache older than 15 minutes reclaimed 6.449 GB.
No application volume/container or retained history was removed. The focused
PostgreSQL test database was also removed after its checks.

The retry built successfully. The final running web image passed health and all
three viewport browser checks again, including Item Search/Economy regression
flows (770 current slots / 5,422 retained offer messages at the final check).
