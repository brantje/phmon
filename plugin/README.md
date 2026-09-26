# phBot integration (planned)

Slice 0 intentionally contains no executable plugin. Slice 1 will introduce
`PhMon.py`, connecting each phBot instance directly to the Go backend using
an outbound authenticated WebSocket. Follow the callback/worker boundary and
backend-owned business logic rules in [AGENTS.md](../AGENTS.md).
