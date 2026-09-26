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
npm --prefix web run format
format_dirty=0
if ! git diff --quiet -- web; then
  git diff -- web
  format_dirty=1
fi
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run build
if [[ "$format_dirty" -ne 0 ]]; then
  echo "Frontend formatting changed files; commit the Prettier output." >&2
  exit 1
fi
docker compose --env-file .env.example config -q
