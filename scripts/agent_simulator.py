#!/usr/bin/env python3
"""Deterministic development harness for the Slice 1 agent protocol.

This is fixture tooling, not a production monitoring source and not proof that the
same plugin has been validated inside a real phBot process.
"""
import os
import signal
import shutil
import sys
import tempfile
import time
import uuid

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(ROOT, 'plugin'))

import PhMon  # noqa: E402


def required(name):
    value = os.environ.get(name)
    if not value:
        raise SystemExit(name + ' is required')
    return value


def wait_until(predicate, timeout, description):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if predicate():
            return
        time.sleep(0.05)
    raise SystemExit("simulator timed out waiting for " + description)


def run_death_events(worker, config, workers, stopping, spool_directory):
    timeout = float(os.environ.get("PHMON_SIMULATOR_CONNECT_TIMEOUT", "30"))
    wait_until(lambda: "Connected" in worker.status, timeout, "backend connection")
    ack_statuses = []
    original_handler = worker._handle_server_message

    def record_ack(message):
        result = original_handler(message)
        if isinstance(message, dict) and message.get("type") == "event.ack":
            ack_statuses.append(message.get("status"))
        return result

    worker._handle_server_message = record_ack

    old_values = {
        "worker": PhMon._worker,
        "available": PhMon._PHBOT_AVAILABLE,
        "joined": PhMon._character_joined,
        "signature": PhMon._last_character_signature,
        "sample_at": PhMon._last_character_sample_at,
        "resources_at": PhMon._last_resources_sample_at,
        "death_active": PhMon._death_callback_active,
        "character_getter": PhMon._get_character_data,
        "position_getter": PhMon._get_position,
        "zone_getter": PhMon._get_zone_name,
    }
    api = {"server": "Fixture Silkroad", "name": "DeathFixtureAlpha", "guild": ""}
    position = {"region": 25000, "x": 10.0, "y": 20.0, "z": 0.0}
    PhMon._worker = worker
    PhMon._PHBOT_AVAILABLE = True
    PhMon._character_joined = True
    PhMon._death_callback_active = False
    PhMon._get_character_data = lambda: dict(api)
    PhMon._get_position = lambda: dict(position)
    PhMon._get_zone_name = lambda _region: "Fixture Jangan"

    def sample(dead_marker):
        if dead_marker is None:
            api.pop("dead", None)
        else:
            api["dead"] = dead_marker
        PhMon._last_character_signature = None
        PhMon._last_resources_sample_at = PhMon._monotonic()
        PhMon._sample_character()
        wait_until(
            lambda: worker._latest_sample is not None
            and (
                "dead" not in worker._latest_sample.get("state", {})
                if dead_marker is None
                else worker._latest_sample.get("state", {}).get("dead") is dead_marker
            ),
            timeout,
            "character state sample",
        )

    try:
        sample(False)
        wait_until(lambda: worker.character_id and worker.session_id, timeout, "character session registration")
        first_character_id, first_session_id = worker.character_id, worker.session_id

        sample(True)
        time.sleep(0.25)
        if ack_statuses:
            raise SystemExit("a dead-state snapshot created a death event")

        PhMon.handle_event(PhMon.EVENT_DIED, "")
        PhMon.handle_event(PhMon.EVENT_DIED, "")
        wait_until(lambda: len(ack_statuses) >= 1, timeout, "durable death acknowledgement")
        if ack_statuses[0] != "persisted":
            raise SystemExit("death event was not durably persisted: " + str(ack_statuses[0]))
        if len(worker._death_spool.pending()) != 0:
            raise SystemExit("persisted death event remained in the spool")
        PhMon.handle_event(PhMon.EVENT_DIED, "")
        time.sleep(0.25)
        if len(ack_statuses) != 1:
            raise SystemExit("repeated callback was not coalesced")

        sample(False)
        PhMon.handle_event(PhMon.EVENT_DIED, "")
        wait_until(lambda: len(ack_statuses) >= 2, timeout, "second death after alive transition")
        if ack_statuses[1] != "persisted":
            raise SystemExit("second death event was not persisted")

        sample(None)
        if "dead" in worker._latest_sample.get("state", {}):
            raise SystemExit("missing phBot death field did not remain unknown")

        api["name"] = "DeathFixtureBeta"
        api["dead"] = False
        PhMon._last_character_signature = None
        PhMon._sample_character()
        wait_until(
            lambda: worker._current_identity is not None
            and worker._current_identity.get("name") == "DeathFixtureBeta"
            and worker.character_id != first_character_id
            and worker.session_id != first_session_id,
            timeout,
            "character session switch",
        )
        stale_event = {
            "event_id": str(uuid.uuid4()),
            "occurred_at": PhMon._worker_utc_now(worker),
            "source": "phbot.callback",
            "source_ref": "EVENT_DIED",
            "payload": {"cause": "unknown"},
            # Pair the old session with the newly switched character. This is
            # rejected immediately by the ownership fence without waiting for
            # the server's five-minute clock-skew window to expire.
            "character_id": worker.character_id,
            "session_id": first_session_id,
        }
        if not worker._death_spool.add(stale_event):
            raise SystemExit("could not stage stale-session fixture")
        wait_until(lambda: len(ack_statuses) >= 3, timeout, "stale-session rejection")
        if ack_statuses[2] != "rejected":
            raise SystemExit("stale-session death event was not rejected")

        second_config = dict(config)
        second_config["death_spool_path"] = os.path.join(spool_directory, "socket-two.json")
        second_worker = PhMon.AgentWorker(second_config, "simulator-fixture")
        workers.append(second_worker)
        second_worker.start()
        wait_until(lambda: "Connected" in second_worker.status, timeout, "second authenticated socket")
        second_worker.update_character(
            {"server": "Fixture Silkroad", "name": "DeathFixtureGamma", "guild": ""},
            {"level": 50, "dead": False},
        )
        wait_until(lambda: second_worker.character_id and second_worker.session_id, timeout, "second socket character")
        if worker.character_id == second_worker.character_id:
            raise SystemExit("concurrent simulator sockets shared character identity")

        replay = {
            "event_id": str(uuid.uuid4()),
            "occurred_at": PhMon._worker_utc_now(worker),
            "source": "phbot.callback",
            "source_ref": "EVENT_DIED",
            "payload": {"cause": "unknown"},
            "character_id": worker.character_id,
            "session_id": worker.session_id,
        }
        if not worker._death_spool.add(replay):
            raise SystemExit("could not stage reconnect replay fixture")
        worker._death_retry_at[replay["event_id"]] = PhMon._monotonic() + 60.0
        current_socket = worker._socket
        if current_socket is None:
            raise SystemExit("worker had no socket for reconnect fixture")
        current_socket.close()
        wait_until(
            lambda: "Connected" in worker.status and worker.session_id != replay["session_id"],
            timeout,
            "reconnected character session",
        )
        worker._death_retry_at.pop(replay["event_id"], None)
        wait_until(lambda: len(ack_statuses) >= 4, timeout, "spool replay acknowledgement")
        if ack_statuses[3] != "persisted":
            raise SystemExit("spooled death did not replay after reconnect")

        print("PASS fixture alive/dead/unknown transitions, callback deduplication, character/session fencing, two sockets, reconnect, and spool replay")
        return 0
    finally:
        stopping[0] = True
        for current in workers:
            current.stop()
        for current in workers:
            current.join(3.0)
        PhMon._worker = old_values["worker"]
        PhMon._PHBOT_AVAILABLE = old_values["available"]
        PhMon._character_joined = old_values["joined"]
        PhMon._last_character_signature = old_values["signature"]
        PhMon._last_character_sample_at = old_values["sample_at"]
        PhMon._last_resources_sample_at = old_values["resources_at"]
        PhMon._death_callback_active = old_values["death_active"]
        PhMon._get_character_data = old_values["character_getter"]
        PhMon._get_position = old_values["position_getter"]
        PhMon._get_zone_name = old_values["zone_getter"]


def run_map_observations(worker, stopping):
    """Exercise v7 live snapshots, samples, event overlays and replay."""
    timeout = float(os.environ.get("PHMON_SIMULATOR_CONNECT_TIMEOUT", "30"))
    wait_until(lambda: "Connected" in worker.status, timeout, "backend connection")
    sample_acks = []
    event_results = []
    original_handler = worker._handle_server_message

    def record_ack(message):
        result = original_handler(message)
        if isinstance(message, dict):
            if message.get("type") == "mob.sample.ack":
                sample_acks.append({"sample_id": message.get("sample_id"),
                                    "status": message.get("status")})
            elif message.get("type") == "event.batch.ack":
                event_results.extend(message.get("results", []))
            elif message.get("type") == "event.ack":
                event_results.append(message)
        return result

    worker._handle_server_message = record_ack
    previous = {
        "worker": PhMon._worker,
        "character_getter": PhMon._get_character_data,
        "position_getter": PhMon._get_position,
        "death_active": PhMon._death_callback_active,
    }
    identity = {"server": "greatest", "name": "MapFixtureAlpha", "guild": "Fixture"}
    api = dict(identity)
    position = {"region": 25273, "x": 10.0, "y": 20.0, "z": 0.0}
    state = {"level": 50, "dead": False, "region": 25273, "zone": "Fixture Jangan",
             "x": 10.0, "y": 20.0, "z": 0.0, "botting": False}
    monster = {"id": "fixture-monster-1", "model_id": 500, "type": "Tiger",
               "region": 25273, "x": 30.0, "y": 40.0}
    PhMon._worker = worker
    PhMon._get_character_data = lambda: dict(api)
    PhMon._get_position = lambda: dict(position)
    PhMon._death_callback_active = False

    def sample(sample_id, monsters, x):
        return {
            "sample_id": sample_id,
            "character_id": worker.character_id,
            "session_id": worker.session_id,
            "area_id": "region:25273",
            "floor_id": "unmapped",
            "region": 25273,
            "sampled_at": PhMon._utc_now(),
            "observer": {"x": float(x), "y": 20.0, "z": 0.0},
            "monsters": monsters,
        }

    try:
        worker.update_character(identity, state)
        wait_until(lambda: worker.character_id and worker.session_id, timeout,
                   "fixture character registration")
        wait_until(
            lambda: worker._current_identity is not None
            and worker._identity_key(worker._current_identity) == worker._identity_key(identity),
            timeout,
            "fixture character identity",
        )
        first_session = worker.session_id
        first_sample = sample(str(uuid.uuid4()), [monster], 10.0)
        worker.update_map_monsters(identity, "observed", 25273, [monster], first_sample)
        wait_until(
            lambda: any(entry["sample_id"] == first_sample["sample_id"] for entry in sample_acks),
            timeout,
            "first mob sample commit",
        )
        first_ack = next(entry for entry in sample_acks if entry["sample_id"] == first_sample["sample_id"])
        if first_ack["status"] != "persisted":
            raise SystemExit("mob sample was not persisted: " + str(first_ack["status"]))

        # Let the server-side Slice 9 movement sampler cross its bounded
        # two-second interval before moving to another observer cell.
        time.sleep(2.1)
        # Moving to another observer cell allows the observed-empty snapshot to
        # contribute a zero to the historical denominator.
        position.update({"x": 250.0, "y": 20.0})
        state.update({"x": 250.0, "y": 20.0})
        worker.update_character(identity, state)
        wait_until(
            lambda: worker._latest_sample is not None
            and worker._latest_sample.get("state", {}).get("x") == 250.0,
            timeout,
            "moved observer position",
        )
        empty_sample = sample(str(uuid.uuid4()), [], 250.0)
        worker.update_map_monsters(identity, "observed", 25273, [], empty_sample)
        wait_until(
            lambda: any(entry["sample_id"] == empty_sample["sample_id"] for entry in sample_acks),
            timeout,
            "empty mob sample commit",
        )
        empty_ack = next(entry for entry in sample_acks if entry["sample_id"] == empty_sample["sample_id"])
        if empty_ack["status"] != "persisted":
            raise SystemExit("empty mob sample was not persisted: " + str(empty_ack["status"]))

        worker.update_map_monsters(identity, "unavailable", 25273, [])
        truncated = [dict(monster, id="fixture-truncated-" + str(index))
                     for index in range(PhMon.MAX_MONSTERS_PER_SNAPSHOT)]
        worker.update_map_monsters(identity, "truncated", 25273, truncated)
        PhMon.handle_event(PhMon.EVENT_ITEM_DROP, "500")
        PhMon.handle_event(PhMon.EVENT_DIED, "")
        PhMon.handle_event(PhMon.EVENT_UNIQUE_SPAWN, "FixtureUnique")
        wait_until(lambda: len(event_results) >= 3, timeout, "death, drop and unique event commits")
        if any(entry.get("status") != "persisted" for entry in event_results[:3]):
            raise SystemExit("map analytics event was not persisted: " + str(event_results[:3]))

        # Reconnect and replay a committed sample ID. The server should return a
        # terminal persisted acknowledgement without adding another denominator.
        current_socket = worker._socket
        if current_socket is None:
            raise SystemExit("worker had no socket for sample replay fixture")
        current_socket.close()
        wait_until(
            lambda: "Connected" in worker.status and worker.session_id != first_session,
            timeout,
            "reconnected fixture session",
        )
        if not worker._mob_spool.add(first_sample):
            raise SystemExit("could not stage committed sample replay")
        wait_until(
            lambda: sum(entry["sample_id"] == first_sample["sample_id"] for entry in sample_acks) >= 2,
            timeout,
            "idempotent sample replay",
        )
        replay_acks = [entry for entry in sample_acks if entry["sample_id"] == first_sample["sample_id"]]
        if replay_acks[-1]["status"] != "persisted":
            raise SystemExit("sample replay was not acknowledged: " + str(replay_acks[-1]["status"]))
        print("PASS protocol v7 movement, current/empty/unavailable/truncated monster states, death/drop/unique events, reconnect and idempotent sample replay")
        return 0
    finally:
        stopping[0] = True
        worker.stop()
        worker.join(3.0)
        PhMon._worker = previous["worker"]
        PhMon._get_character_data = previous["character_getter"]
        PhMon._get_position = previous["position_getter"]
        PhMon._death_callback_active = previous["death_active"]


def main():
    scenario = os.environ.get("PHMON_SIMULATOR_SCENARIO")
    spool_directory = tempfile.mkdtemp(prefix="phmon-agent-simulator-") if scenario in ("death-events", "map-observations") else None
    config = {
        'backend_url': required('PHMON_AGENT_URL'),
        'agent_id': required('PHMON_AGENT_ID'),
        'agent_token': required('PHMON_AGENT_TOKEN'),
    }
    if spool_directory:
        config['death_spool_path'] = os.path.join(spool_directory, "socket-one.json")
        if scenario == "map-observations":
            config['mob_spool_path'] = os.path.join(spool_directory, "mob-samples.json")
    fake_calls = []
    api = PhMon.PhBotAdapter({'stop_bot': lambda: fake_calls.append('bot.stop') or True}) if scenario == 'commands' else None
    worker = PhMon.AgentWorker(config, 'simulator-fixture', api_adapter=api)
    workers = [worker]
    stopping = [False]

    def stop(_signum=None, _frame=None):
        if stopping[0]:
            return
        stopping[0] = True
        for current in workers:
            current.stop()

    signal.signal(signal.SIGINT, stop)
    signal.signal(signal.SIGTERM, stop)
    worker.start()

    if scenario == "death-events":
        try:
            return run_death_events(worker, config, workers, stopping, spool_directory)
        finally:
            if spool_directory and os.path.isdir(spool_directory):
                shutil.rmtree(spool_directory, ignore_errors=True)

    if scenario == "map-observations":
        try:
            return run_map_observations(worker, stopping)
        finally:
            if spool_directory and os.path.isdir(spool_directory):
                shutil.rmtree(spool_directory, ignore_errors=True)

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

    if scenario == "commands":
        deadline = time.time() + float(os.environ.get("PHMON_SIMULATOR_CONNECT_TIMEOUT", "30"))
        while time.time() < deadline and "Connected" not in worker.status:
            time.sleep(0.1)
        if "Connected" not in worker.status:
            worker.stop(); worker.join(2.0)
            raise SystemExit("simulator could not establish backend connection")
        identity = {"server": required("PHMON_SIMULATOR_SERVER"), "name": required("PHMON_SIMULATOR_CHARACTER"), "guild": ""}
        worker.update_character(identity, {"level":75,"hp":900,"region":25000,"zone":"Fixture Jangan","x":10.0,"y":20.0,"z":0.0,"botting":None})
        callback_deadline = time.time() + float(os.environ.get("PHMON_SIMULATOR_COMMAND_TIMEOUT", "60"))
        try:
            while time.time() < callback_deadline and not fake_calls and not stopping[0]:
                if worker.character_id and worker.session_id:
                    worker.process_one_command(identity, 25000)
                time.sleep(0.5)
            # The phBot callback only queues the result. Give the production
            # network worker time to flush it before the fixture closes the socket.
            flush_deadline = time.time() + 3.0
            while fake_calls and not worker._outgoing.empty() and time.time() < flush_deadline:
                time.sleep(0.05)
            if fake_calls:
                time.sleep(0.25)
        finally:
            stop(); worker.join(3.0)
        if fake_calls != ['bot.stop']:
            raise SystemExit("simulator did not invoke exactly one fake bot.stop adapter")
        print("PASS production worker invoked bot.stop once through fake callback adapter")
        return 0

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
