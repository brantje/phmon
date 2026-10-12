# PhMon enrichment v2 contract

This is the implemented **ph-proxy sender contract** and specification for the
separate PhMon receiver. It does not claim deployed backend support or successful
live ingestion. V1 (`docs/protocol.md` ph-proxy section) remains supported.

## Transport

- `POST /api/integrations/ph-proxy/v2/enrichments` — agent bearer token, `schema_version: 2`
- `GET /api/integrations/ph-proxy/v2/status?server=` — operator session

Validation, staging, idempotency and merge semantics match v1 unless noted in
`server/internal/phproxy` v2 validators. Profile `silkroad-equipment-v1` only.

See the operator ph-proxy repository for wire examples and sender configuration.
