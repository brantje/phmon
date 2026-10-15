# Unidentified nearby-player admission — 2026-10-12

## Cause and observed evidence

Read-only production inspection after the requested cleanup found four recreated
unclassified player records and seven observations, all from `map.players`.
None had `player_name`, `job_name` or `is_jobbing` evidence; no pending enrichment
existed at inspection. Real player names are excluded from this report/fixtures.

The collector obtains a nearby displayed `name` from `get_players()`. The observed
runtime supplies name, guild, level and position but no normal-name/alias
discriminator. The official [Players API](https://plugins.phbot.org/phbot-api/players),
checked again on 2026-10-12, documents a generic `name` without a job-mode field.
Runtime evidence in `docs/phbot-capabilities.md` establishes that this API works
on the observed runtime despite its documented disabled notice.

`Registry.ApplyLive` previously persisted every nonempty displayed name. The
shared identity projection avoids assuming those generic names are normal player
names. Without native classification or accepted proxy evidence, both display
fields are Unknown. Deletion removes the tracking rows, but the next nearby
snapshot re-creates them through unconditional admission. This identifies a
registry admission problem; it does not establish a ph-proxy rejection or a
missing sender upload.

## Implemented admission rule

`Registry.ApplyLive` now validates all observations and admits only names with:

- Explicit native active-job state, either true or false.
- An existing resolved tracked identity, including accepted server-scoped alias links.
- A registered own-character identity in that server.
- Non-expired accepted pending enrichment with direct normal-name evidence, when
  the promoter is installed. Legacy v1 verified evidence is supported.

Job affiliation and job level alone cannot classify a name. An association on an
individual alias-attributed field is not a global alias link. Exact normalized
names remain server-scoped. Admission of a staged normal name triggers existing
enrichment promotion; a subsequent nearby snapshot can admit its related alias
record. Proxy-only names still do not create tracked records by themselves.

The independent live-map store receives every validated sighting. Skipped sightings
create no saved registry row, observation or level snapshot. Later classification
starts history at that actual later observation. Subsequent omission of
classification does not prevent updating a known record. Existing unresolved
historical records remain intact and receive no further unidentified snapshot
history. Explicitly attributed tracking/imports through `Apply` stay unchanged.
No native field or timestamp is filled from inferred proxy identity values.

## Validation and live-delivery boundary

Synthetic PostgreSQL tests cover repeated unidentified names, affiliation-only
names, native normal/alias classification, own characters, server isolation, atomic
validation, preserved legacy history, omitted classification, later classification,
expired pending evidence, v1 promotion, field-association isolation and staged
Magik/xDont with seven gear entries and two original records displayed as one.

No sender/configuration changes or bot operations were performed. At the operator's
subsequent request, the local Compose server/web containers were rebuilt and
restarted; the server is healthy and applied the current migrations. This verifies
that the admission code builds and starts locally, but not how a new natural
phBot sighting is classified on the Windows sender/runtime. Existing sightings
remain on the live map; observe naturally occurring map updates to complete that
runtime check.
