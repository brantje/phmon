# PhMon phBot plugin

PhMon.py is the phBot-side connector for the self-hosted PhMon backend. Each running
phBot instance owns one stable agent identity and makes its own outbound WebSocket
connection. The plugin reports connectivity facts; durable identity, authentication,
history and future command policy remain server-owned.

## Install

1. Provision a credential on the PhMon server:

       docker compose exec server phmonctl agent create

   The command prints an agent_id and agent_token once. PostgreSQL stores only the
   token hash. Keep the plaintext token private.

2. Copy plugin/PhMon.py into the phBot Plugins directory.
3. Create PhMon.json in the phBot Config directory returned by phBot's
   get_config_dir() API:

       {
         "backend_url": "wss://phmon.example.internal/agent",
         "agent_id": "11111111-2222-4333-8444-555555555555",
         "agent_token": "phm_replace_with_the_provisioned_token"
       }

4. Reload the plugin or restart phBot. The agent should appear in PhMon after the
   authenticated hello handshake succeeds.

The default Compose binding keeps the Go agent listener on loopback. Use ws:// only
when the plugin runs on the same host or on an explicitly trusted development network
with a deliberate SERVER_BIND_ADDR override. For remote agents, terminate TLS in
front of Go and configure wss://; the plugin uses normal system certificate
validation and does not provide an insecure TLS bypass.

Do not put the token in the backend URL. The plugin rejects URL credentials, query
parameters and fragments so secrets do not leak into logs, browser history or proxy
access logs.

## Runtime behavior

The plugin uses only Python standard-library networking. Public phBot documentation
verifies socket support but does not document a bundled third-party WebSocket client,
so no pip install inside phBot is required.

A dedicated worker thread owns WebSocket connect/read/write work. It sends the
protocol-v1 hello, follows the server heartbeat interval and reconnects automatically
with bounded exponential backoff and jitter. finished() only signals shutdown and
closes the worker socket; latency-sensitive phBot callbacks never wait for backend
I/O.

The plugin stores no durable event queue. Later slices will add current-state and
event messages over the same authenticated connection.

## Simulator

For development without a Windows/phBot process:

    PHMON_AGENT_URL='ws://127.0.0.1:8081/agent' \
    PHMON_AGENT_ID='...' \
    PHMON_AGENT_TOKEN='...' \
    python3 scripts/agent_simulator.py

The simulator imports the exact same transport/worker implementation as the plugin
and reports simulator-fixture as its phBot version. It is test tooling only and is
never evidence that real phBot integration has been validated.
