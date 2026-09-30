#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
(
  cd server
  unformatted=$(gofmt -l .)
  if [[ -n "$unformatted" ]]; then
    echo "Run gofmt on: $unformatted" >&2
    exit 1
  fi
  go vet ./...
  go test -race ./...
  go build -o bin/server ./cmd/server
  go build -o bin/phmonctl ./cmd/phmonctl
)
python3 -m unittest discover -s plugin -p 'test_*.py'
python3 scripts/live_transport_audit.py
npm --prefix web run format:check
npm --prefix web run test:unit
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run build
docker compose --env-file .env.example config -q
