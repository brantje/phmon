#!/usr/bin/env python3
"""Deterministic development harness for the Slice 1 agent protocol.

This is fixture tooling, not a production monitoring source and not proof that the
same plugin has been validated inside a real phBot process.
"""
import os
import signal
import sys
import time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(ROOT, 'plugin'))

import PhMon  # noqa: E402


def required(name):
    value = os.environ.get(name)
    if not value:
        raise SystemExit(name + ' is required')
    return value


def main():
    config = {
        'backend_url': required('PHMON_AGENT_URL'),
        'agent_id': required('PHMON_AGENT_ID'),
        'agent_token': required('PHMON_AGENT_TOKEN'),
    }
    worker = PhMon.AgentWorker(config, 'simulator-fixture')
    stopping = [False]

    def stop(_signum=None, _frame=None):
        if stopping[0]:
            return
        stopping[0] = True
        worker.stop()

    signal.signal(signal.SIGINT, stop)
    signal.signal(signal.SIGTERM, stop)
    worker.start()

    run_seconds = float(os.environ.get('PHMON_SIMULATOR_RUN_SECONDS', '0'))
    deadline = time.time() + run_seconds if run_seconds > 0 else None
    try:
        while not stopping[0] and (deadline is None or time.time() < deadline):
            time.sleep(0.2)
    finally:
        stop()
        worker.join(3.0)
    return 0


if __name__ == '__main__':
    sys.exit(main())
