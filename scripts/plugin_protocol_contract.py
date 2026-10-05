#!/usr/bin/env python3
"""Generate a deterministic fingerprint of PhMon's outbound monitor contract."""

from __future__ import print_function

import argparse
import ast
import copy
import contextlib
import importlib.util
import io
import json
import os
import queue
import re
import tempfile


MASKED_KEYS = {
    "agent_id",
    "character_id",
    "command_id",
    "event_id",
    "occurred_at",
    "observed_at",
    "phbot_version",
    "plugin_version",
    "protocol_version",
    "sample_id",
    "sent_at",
    "session_id",
}


def load_plugin(path):
    spec = importlib.util.spec_from_file_location("phmon_contract_target", path)
    if spec is None or spec.loader is None:
        raise RuntimeError("unable to load plugin")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def canonical(value, key=None):
    if key in MASKED_KEYS:
        return "<%s>" % key
    if isinstance(value, dict):
        return {
            str(item_key): canonical(item_value, str(item_key))
            for item_key, item_value in sorted(value.items(), key=lambda item: str(item[0]))
        }
    if isinstance(value, (list, tuple)):
        return [canonical(item) for item in value]
    if isinstance(value, float):
        if value != value:
            return "<nan>"
        if value == float("inf"):
            return "<inf>"
        if value == float("-inf"):
            return "<-inf>"
    return value


def expression_shape(node, key=None):
    if isinstance(node, ast.Dict):
        result = {}
        for raw_key, raw_value in zip(node.keys, node.values):
            if isinstance(raw_key, ast.Constant) and isinstance(raw_key.value, str):
                result[raw_key.value] = expression_shape(raw_value, raw_key.value)
            else:
                result["<dynamic-key>"] = "<expr>"
        return result
    if isinstance(node, (ast.List, ast.Tuple)):
        return [expression_shape(item) for item in node.elts]
    if isinstance(node, ast.Constant):
        if key == "type":
            return node.value
        return "<%s>" % type(node.value).__name__
    if isinstance(node, ast.IfExp):
        return {
            "if_true": expression_shape(node.body, key),
            "if_false": expression_shape(node.orelse, key),
        }
    return "<expr>"


def direct_send_surface(path):
    with open(path, "r", encoding="utf-8") as handle:
        tree = ast.parse(handle.read(), filename=path)
    found = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call) or not node.args:
            continue
        func = node.func
        if not isinstance(func, ast.Attribute) or func.attr != "send_json":
            continue
        if isinstance(node.args[0], ast.Dict):
            found.append(expression_shape(node.args[0]))
    return sorted(found, key=lambda item: json.dumps(item, sort_keys=True))


def resource_fixture(plugin):
    item = {
        "model": 1001,
        "quantity": 2,
        "plus": 7,
        "durability": 123,
        "name": "Contract Sword",
        "servername": "ITEM_CONTRACT_SWORD",
    }
    for index, field in enumerate(getattr(plugin, "ITEM_API_EVIDENCE_FIELDS", ())):
        item.setdefault(field, index + 1)

    inputs = {
        "get_inventory": {
            "available": True,
            "value": {"size": 14, "gold": 123456, "items": [copy.deepcopy(item) for _ in range(14)]},
        },
        "get_storage": {
            "available": True,
            "value": {"size": 1, "items": [copy.deepcopy(item)]},
        },
        "get_guild_storage": {
            "available": True,
            "value": {"size": 1, "items": [copy.deepcopy(item)]},
        },
        "get_job_pouch": {
            "available": True,
            "value": {"size": 1, "items": [copy.deepcopy(item)]},
        },
        "get_pets": {
            "available": True,
            "value": {
                123: {
                    "name": "Contract Wolf",
                    "servername": "COS_CONTRACT_WOLF",
                    "type": "wolf",
                    "model": 42,
                    "hp": 9000,
                    "mounted": False,
                    "items": [copy.deepcopy(item)],
                }
            },
        },
        "get_party": {
            "available": True,
            "value": {
                55: {
                    "name": "Contract Ally",
                    "guild": "Contract Guild",
                    "player_id": 500,
                    "level": 110,
                    "x": 123,
                    "y": -456,
                    "hp_percent": 8,
                    "mp_percent": 9,
                },
                56: {
                    "name": "Contract Far Ally",
                    "player_id": 501,
                    "x": 1000001,
                    "y": -1000001,
                },
            },
        },
        "get_academy": {
            "available": True,
            "value": {
                "id": 237,
                6699: {
                    "online": 1,
                    "type": 0,
                    "x": 1.5,
                    "y": 2.5,
                    "level": 110,
                    "name": "Contract Academy",
                },
            },
        },
    }
    return plugin.normalize_resource_inputs(inputs, None, "Greatest", 22)


def monster_fixture(plugin):
    status, monsters, truncated = plugin.collect_monster_observation(
        {
            "get_monsters": lambda: {
                100: {
                    "model": 200,
                    "region": 25000,
                    "x": 10,
                    "y": 20.5,
                    "z": -3,
                    "type": 4,
                    "name": "Contract Mob",
                    "servername": "MOB_CONTRACT",
                    "level": 110,
                    "hp": 500,
                    "max_hp": 1000,
                    "attacking": 1,
                },
                101: {
                    "model": 201,
                    "region": 25000,
                    "x": 1000001,
                    "y": 20,
                },
            }
        }
    )
    return {"status": status, "monsters": monsters, "truncated": truncated}


class CaptureClient(object):
    def __init__(self, protocol_version):
        self.sent = []
        self.protocol_version = protocol_version

    def send_json(self, value):
        self.sent.append(copy.deepcopy(value))

    def receive_json(self, timeout=None):
        return {
            "type": "character.registered",
            "protocol_version": self.protocol_version,
            "character_id": "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
            "session_id": "ffffffff-1111-4222-8333-444444444444",
        }


class MemorySpool(object):
    def __init__(self, items=None):
        self.items = list(items or [])

    def add(self, item):
        self.items.append(copy.deepcopy(item))
        return True

    def pending(self):
        return [copy.deepcopy(item) for item in self.items]

    def acknowledge(self, identifier):
        return True

    def bind_session(self, event_id, character_id, session_id, sequence):
        return True


def adapter_fixture(plugin):
    def yes(*args):
        return True

    functions = {
        name: yes
        for name in getattr(plugin, "_API_NAMES", ())
    }
    functions["get_position"] = lambda: {"region": 25000, "x": 12.5, "y": 9.5, "z": 0}
    functions["get_training_area"] = lambda: {
        "radius": 50,
        "region": 25000,
        "x": 12.5,
        "y": 9.5,
        "z": 0,
        "path": "contract.txt",
    }
    chat = {
        "general": lambda text: True,
        "private": lambda recipient, text: True,
        "party": lambda text: True,
        "guild": lambda text: True,
        "union": lambda text: True,
        "global": lambda text: True,
    }
    return plugin.PhBotAdapter(functions, chat)


def worker_fixture(plugin, adapter=None, **paths):
    config = {
        "backend_url": "ws://127.0.0.1:8081/agent",
        "agent_id": "11111111-2222-4333-8444-555555555555",
        "agent_token": "contract-token",
    }
    config.update(paths)
    return plugin.AgentWorker(config, "contract-phbot", api_adapter=adapter or adapter_fixture(plugin))


def character_sample_fixture(plugin):
    worker = worker_fixture(plugin)
    previous = {
        "_PHBOT_AVAILABLE": getattr(plugin, "_PHBOT_AVAILABLE", False),
        "_worker": getattr(plugin, "_worker", None),
        "_character_joined": getattr(plugin, "_character_joined", None),
        "_get_character_data": getattr(plugin, "_get_character_data", None),
        "_get_position": getattr(plugin, "_get_position", None),
        "_get_zone_name": getattr(plugin, "_get_zone_name", None),
        "_last_character_signature": getattr(plugin, "_last_character_signature", None),
        "_last_character_sample_at": getattr(plugin, "_last_character_sample_at", 0.0),
        "_last_resources_sample_at": getattr(plugin, "_last_resources_sample_at", 0.0),
        "_last_monster_poll_at": getattr(plugin, "_last_monster_poll_at", 0.0),
    }
    try:
        plugin._PHBOT_AVAILABLE = True
        plugin._worker = worker
        plugin._character_joined = True
        plugin._get_character_data = lambda: {
            "server": "Greatest",
            "name": "Contract Character",
            "guild": "Contract Guild",
            "level": 110,
            "hp": 1000,
            "hp_max": 1200,
            "mp": 800,
            "mp_max": 900,
            "current_exp": 123,
            "max_exp": 456,
            "sp": 789,
            "gold": 123456,
            "region": 25000,
            "model": 1907,
            "dead": False,
        }
        plugin._get_position = lambda: {"region": 25000, "x": 12, "y": 34.5, "z": -6}
        plugin._get_zone_name = lambda region: "Contract Zone"
        now = plugin._monotonic()
        plugin._last_character_signature = None
        plugin._last_character_sample_at = 0.0
        plugin._last_resources_sample_at = now
        plugin._last_monster_poll_at = now
        plugin._sample_character()
        return worker._samples.get_nowait()
    finally:
        for name, value in previous.items():
            setattr(plugin, name, value)


def frame_fixtures(plugin, resources, monsters):
    frames = {}

    client = CaptureClient(plugin.PROTOCOL_VERSION)
    worker = worker_fixture(plugin)
    worker._publish_sample(
        client,
        {
            "identity": {"server": "Greatest", "name": "Contract Character", "guild": "Contract Guild"},
            "state": {"level": 110, "region": 25000, "x": 12.0, "y": 34.0, "botting": None},
        },
        True,
    )
    worker._publish_sample(
        client,
        {
            "identity": {"server": "Greatest", "name": "Contract Character", "guild": "Contract Guild"},
            "state": {"level": 111, "region": 25000, "x": 13.0, "y": 35.0, "botting": None},
        },
        False,
    )
    frames["character"] = client.sent

    position_client = CaptureClient(plugin.PROTOCOL_VERSION)
    position_worker = worker_fixture(plugin)
    position_worker.character_id = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
    position_worker.session_id = "ffffffff-1111-4222-8333-444444444444"
    position_worker._adopt_position_session(position_worker.session_id)
    position_worker.update_position(
        {"region": 25000, "x": 12.5, "y": 34.5, "z": -6.0},
        "2026-01-01T00:00:00Z",
    )
    position_worker._flush_realtime_position(position_client)
    frames["positions"] = position_client.sent

    resource_client = CaptureClient(plugin.PROTOCOL_VERSION)
    resource_worker = worker_fixture(plugin)
    resource_worker.character_id = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
    resource_worker.session_id = "ffffffff-1111-4222-8333-444444444444"
    resource_worker._send_resource_snapshot(resource_client, resources)
    changed = copy.deepcopy(resources)
    if isinstance(changed.get("inventory"), dict):
        changed["inventory"]["gold"] = 654321
    resource_worker._send_resource_snapshot(resource_client, changed)
    frames["resources"] = resource_client.sent

    map_client = CaptureClient(plugin.PROTOCOL_VERSION)
    map_worker = worker_fixture(plugin)
    map_worker.character_id = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
    map_worker.session_id = "ffffffff-1111-4222-8333-444444444444"
    map_worker._current_identity = {"server": "Greatest", "name": "Contract Character"}
    map_worker._latest_map_observation = {
        "identity": dict(map_worker._current_identity),
        "status": "observed",
        "region": 25000,
        "monsters": copy.deepcopy(monsters),
        "observed_at": "2026-01-01T00:00:00Z",
        "observer_z": -6.0,
    }
    sample = {
        "sample_id": "99999999-8888-4777-8666-555555555555",
        "character_id": map_worker.character_id,
        "session_id": map_worker.session_id,
        "area_id": "region:25000",
        "floor_id": "unmapped",
        "region": 25000,
        "sampled_at": "2026-01-01T00:00:00Z",
        "observer": {"x": 12.0, "y": 34.0, "z": -6.0},
        "monsters": copy.deepcopy(monsters),
    }
    map_worker._mob_spool = MemorySpool([sample])
    map_worker._latest_npc_observation = {
        "identity": dict(map_worker._current_identity),
        "status": "observed",
        "region": 25000,
        "npcs": [{
            "id": "10",
            "name": "Jangan",
            "servername": "GATE_CH",
            "model_id": 2094,
            "role": "teleporter",
            "region": 25000,
            "x": 30.0,
            "y": 40.0,
        }],
        "observed_at": "2026-01-01T00:00:00Z",
        "observer_z": -6.0,
    }
    map_worker._latest_player_observation = {
        "identity": dict(map_worker._current_identity),
        "status": "observed",
        "region": 25000,
        "players": [{
            "player_id": "8654977",
            "name": "Nearby",
            "guild": "Guild",
            "grant": "Member",
            "dead": False,
            "level": 71,
            "region": 25000,
            "zone": "Jangan",
            "x": 30.0,
            "y": 40.0,
        }],
        "observed_at": "2026-01-01T00:00:00Z",
        "observer_z": -6.0,
    }
    map_worker._flush_map_observations(map_client)
    frames["map"] = map_client.sent

    command_worker = worker_fixture(plugin)
    command_worker.character_id = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
    command_worker.session_id = "ffffffff-1111-4222-8333-444444444444"
    command_worker._current_identity = {"server": "Greatest", "name": "Contract Character"}
    command = {
        "type": "command.execute",
        "protocol_version": plugin.PROTOCOL_VERSION,
        "command_id": "cmd_00000000-0000-4000-8000-000000000001",
        "character_id": command_worker.character_id,
        "session_id": command_worker.session_id,
        "name": "bot.stop",
        "args": {},
        "expires_at": "2099-01-01T00:00:00Z",
        "ttl_ms": 10000,
    }
    command_worker._accept_command(command)
    command_worker.process_one_command(command_worker._current_identity, 25000)
    command_frames = []
    while True:
        try:
            command_frames.append(command_worker._outgoing.get_nowait())
        except queue.Empty:
            break
    frames["commands"] = command_frames
    frames["capabilities"] = [command_worker._capability_frame()]

    event_worker = worker_fixture(plugin)
    event = {
        "event_id": "77777777-6666-4555-8444-333333333333",
        "schema_version": 1,
        "kind": "contract.event",
        "category": "contract",
        "character_id": "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
        "session_id": "ffffffff-1111-4222-8333-444444444444",
        "occurred_at": "2026-01-01T00:00:00Z",
        "sequence": 1,
        "source": "contract",
        "source_ref": "fixture",
        "dedupe_key": "contract:1",
        "region": 25000,
        "zone": "Contract Zone",
        "x": 12.0,
        "y": 34.0,
        "z": -6.0,
        "item_model": 1001,
        "item_code": "ITEM_CONTRACT_SWORD",
        "payload": {"contract": True, "nested": {"value": 1}},
        "server": "Greatest",
        "character_name": "Contract Character",
    }
    event_worker._death_spool = MemorySpool([event])
    event_client = CaptureClient(plugin.PROTOCOL_VERSION)
    event_worker._flush_events(event_client)
    frames["events"] = event_client.sent

    return frames


def build_contract(plugin, path):
    resources = resource_fixture(plugin)
    monster = monster_fixture(plugin)
    character_sample = character_sample_fixture(plugin)
    frames = frame_fixtures(plugin, resources, monster["monsters"])
    event_resource = (
        plugin._event_resource_snapshot(resources)
        if hasattr(plugin, "_event_resource_snapshot")
        else "<missing>"
    )
    return canonical(
        {
            "direct_send_literals": direct_send_surface(path),
            "character_sample": character_sample,
            "normalized_resources": resources,
            "monster_observation": monster,
            "event_resource_snapshot": event_resource,
            "frames": frames,
        }
    )


def protocol_contract_changed(before, after):
    """Require a bump for monitor changes, allowing additive capability entries.

    Protocol v3 receivers ignore well-formed command names they do not implement.
    New entries in its existing schema are compatible; removals, changed existing
    entries and all other frame changes remain guarded.
    """
    if before == after:
        return False
    normalized = copy.deepcopy(after)
    try:
        old_frames = before["frames"]["capabilities"]
        new_frames = normalized["frames"]["capabilities"]
        if len(old_frames) != len(new_frames):
            return True
        for old_frame, new_frame in zip(old_frames, new_frames):
            old_commands = old_frame["commands"]
            commands = new_frame["commands"]
            known = {entry["name"] for entry in old_commands}
            names = [entry["name"] for entry in commands]
            if len(known) != len(old_commands) or len(set(names)) != len(names):
                return True
            for entry in commands:
                if entry["name"] in known:
                    continue
                if (not isinstance(entry["name"], str) or not entry["name"]
                        or len(entry["name"]) > 64
                        or re.fullmatch(
                            r"[a-z][a-z0-9]*(?:[._][a-z0-9]+)*", entry["name"]
                        ) is None
                        or set(entry) - {"name", "supported", "reason", "modes"}
                        or type(entry.get("supported")) is not bool
                        or not isinstance(entry.get("reason"), str)):
                    return True
                if "modes" in entry and (not isinstance(entry["modes"], list)
                        or any(not isinstance(mode, str) for mode in entry["modes"])):
                    return True
            new_frame["commands"] = [entry for entry in commands if entry["name"] in known]
    except (KeyError, TypeError):
        return True
    return normalized != before


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("plugin")
    args = parser.parse_args()
    path = os.path.abspath(args.plugin)
    # Plugin callbacks may log while the contract harness exercises them. Keep
    # those diagnostics out of stdout so callers can consume the JSON directly.
    with contextlib.redirect_stdout(io.StringIO()):
        plugin = load_plugin(path)
        contract = build_contract(plugin, path)
    print(json.dumps(contract, sort_keys=True, separators=(",", ":"), allow_nan=False))


if __name__ == "__main__":
    main()
