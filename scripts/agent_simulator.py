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

    if os.environ.get("PHMON_SIMULATOR_SCENARIO") == "character-lifecycle":
        deadline = time.time() + float(os.environ.get("PHMON_SIMULATOR_CONNECT_TIMEOUT", "30"))
        while time.time() < deadline and "Connected" not in worker.status:
            time.sleep(0.1)
        if "Connected" not in worker.status:
            worker.stop()
            worker.join(2.0)
            raise SystemExit("simulator could not establish backend connection")
        # Deterministic fixtures exercise identity reuse, state replacement,
        # explicit leave/switch and rejoin over the production PhMon.py worker.
        alpha = {"server": "Fixture Silkroad", "name": "FixtureAlpha", "guild": "FixtureGuild"}
        beta = {"server": "Fixture Silkroad", "name": "FixtureBeta", "guild": ""}
        worker.update_character(alpha, {"level": 75, "hp": 900, "hp_max": 1000, "mp": 400, "mp_max": 500, "current_exp": 1000, "max_exp": 5000, "sp": 250, "gold": 123456, "region": 25000, "zone": "Fixture Jangan", "x": 10.0, "y": 20.0, "z": 0.0, "botting": False})
        time.sleep(1.2)
        worker.update_character(alpha, {"level": 76, "hp": 950, "hp_max": 1000, "mp": 410, "mp_max": 500, "current_exp": 1800, "max_exp": 5000, "sp": 255, "gold": 123999, "region": 25000, "zone": "Fixture Jangan", "x": 11.0, "y": 21.0, "z": 0.0, "botting": True})
        time.sleep(1.2)
        worker.update_character(beta, {"level": 42, "hp": 300, "hp_max": 600, "mp": 700, "mp_max": 900, "current_exp": 12, "max_exp": 120, "sp": 17, "gold": 456, "region": 25200, "zone": "Fixture Donwhang", "x": 50.0, "y": 75.0, "z": 2.0, "botting": False})
        time.sleep(1.2)
        worker.leave_character()
        time.sleep(0.5)
        worker.update_character(alpha, {"level": 76, "hp": 950, "hp_max": 1000, "mp": 410, "mp_max": 500, "current_exp": 1800, "max_exp": 5000, "sp": 255, "gold": 123999, "region": 25000, "zone": "Fixture Jangan", "x": 11.0, "y": 21.0, "z": 0.0, "botting": True})
        time.sleep(1.2)
        worker.update_character(beta, {"level": 42, "hp": 300, "hp_max": 600, "mp": 700, "mp_max": 900, "current_exp": 12, "max_exp": 120, "sp": 17, "gold": 456, "region": 25200, "zone": "Fixture Donwhang", "x": 50.0, "y": 75.0, "z": 2.0, "botting": False})
        print("PASS deterministic character lifecycle fixture completed")

    if os.environ.get("PHMON_SIMULATOR_SCENARIO") == "stable-character":
        deadline = time.time() + float(os.environ.get("PHMON_SIMULATOR_CONNECT_TIMEOUT", "30"))
        while time.time() < deadline and "Connected" not in worker.status:
            time.sleep(0.1)
        if "Connected" not in worker.status:
            worker.stop()
            worker.join(2.0)
            raise SystemExit("simulator could not establish backend connection")
        worker.update_character(
            {
                "server": required("PHMON_SIMULATOR_SERVER"),
                "name": required("PHMON_SIMULATOR_CHARACTER"),
                "guild": "",
            },
            {"level": 75, "hp": 900, "region": 25000, "zone": "Fixture Jangan"},
        )
        print("PASS stable character session published")

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
