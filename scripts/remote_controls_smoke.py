#!/usr/bin/env python3
"""Disposable Issue #35 flow through the production plugin worker and fake APIs.

Requires an isolated Compose project/database and never invokes a real phBot API.
"""
from __future__ import annotations

import concurrent.futures
import json
import os
import subprocess
import sys
import time
import uuid
from http.cookies import SimpleCookie
from socket import timeout as SocketTimeout
from urllib.error import HTTPError
from urllib.request import Request, build_opener

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WEB_URL = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:53005").rstrip("/")
ORIGIN = os.environ.get("SMOKE_ORIGIN", WEB_URL)


def required(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise SystemExit(name + " is required")
    return value


def request_json(url, method="GET", body=None, cookie=""):
    headers = {"Origin": ORIGIN, "Accept": "application/json"}
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body).encode("utf-8")
    if cookie:
        headers["Cookie"] = cookie
    request = Request(url, data=data, headers=headers, method=method)
    try:
        response = build_opener().open(request, timeout=5)
    except HTTPError as error:
        raw = error.read().decode("utf-8")
        return error.code, error.headers, json.loads(raw) if raw else {}
    raw = response.read()
    return response.status, response.headers, json.loads(raw.decode("utf-8")) if raw else {}


def receive_or_timeout(client, timeout=5):
    try:
        return client.receive_json(timeout=timeout)
    except (TimeoutError, SocketTimeout):
        return None


def main():
    os.environ["SMOKE_WEB_URL"] = WEB_URL
    sys.path.insert(0, os.path.join(ROOT, "scripts"))
    import live_smoke

    server = os.environ.get("PHMON_SIMULATOR_SERVER", "Fixture Remote Controls")
    prefix = "Remote35_" + str(time.time_ns())
    replacement_name = prefix + "_Replacement"
    names = [prefix + "_Alpha", prefix + "_Bravo", prefix + "_Charlie", replacement_name]
    positions = [(6410.0, 1080.0), (6520.0, 1140.0), (6675.0, 1225.0), (6800.0, 1300.0)]
    simulators = []
    live = None
    try:
        code, headers, _ = request_json(
            WEB_URL + "/api/auth/login", "POST",
            {"secret": required("OPERATOR_ACCESS_SECRET")},
        )
        if code != 200:
            raise RuntimeError("operator login failed: HTTP " + str(code))
        parsed = SimpleCookie()
        parsed.load(headers.get("Set-Cookie", ""))
        cookie_name = os.environ.get("NUXT_OPERATOR_COOKIE_NAME", "phmon_operator")
        if cookie_name not in parsed:
            raise RuntimeError("operator login did not issue the configured session cookie")
        cookie = cookie_name + "=" + parsed[cookie_name].value

        code, _, credential = request_json(
            WEB_URL + "/api/agents/credentials", "POST", {}, cookie,
        )
        if code != 201 or not credential.get("agent_id") or not credential.get("agent_token"):
            raise RuntimeError("simulator credential creation failed: HTTP " + str(code))

        for index, (name, position) in enumerate(zip(names, positions)):
            env = dict(os.environ)
            env.update({
                "PHMON_AGENT_URL": os.environ.get("PHMON_AGENT_URL", "ws://127.0.0.1:8081/agent"),
                "PHMON_AGENT_ID": credential["agent_id"],
                "PHMON_AGENT_TOKEN": credential["agent_token"],
                "PHMON_SIMULATOR_SCENARIO": "remote-controls",
                "PHMON_SIMULATOR_SERVER": server,
                "PHMON_SIMULATOR_CHARACTER": name,
                "PHMON_SIMULATOR_X": str(position[0]),
                "PHMON_SIMULATOR_Y": str(position[1]),
                "PHMON_SIMULATOR_Z": str(index * 3),
                "PHMON_SIMULATOR_NO_TRAINING_AREA": "false",
                "PHMON_SIMULATOR_FALSE_ACTION": "start_bot" if index == 1 else "",
                "PHMON_SIMULATOR_MAX_COMMANDS": ("8", "3", "4", "2")[index],
                "PHMON_SIMULATOR_COMMAND_TIMEOUT": "180",
                "PHMON_SIMULATOR_CONNECT_TIMEOUT": "30",
            })
            if index == 3:
                env["PHMON_SIMULATOR_UNSUPPORTED_TRAINING_MODES"] = "named"
                env["PHMON_SIMULATOR_REPLACE_SESSION_AFTER"] = "1"
            simulators.append(subprocess.Popen(
                [sys.executable, os.path.join(ROOT, "scripts", "agent_simulator.py")],
                cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
            ))

        live = live_smoke.WebSocketClient(
            WEB_URL.replace("http://", "ws://", 1).replace("https://", "wss://", 1) + "/api/live",
            ORIGIN, cookie,
        ).connect()
        live.send_json({
            "type": "subscribe", "protocol_version": 1,
            "subscription_id": "remote-control-characters", "revision": 1,
            "stream": "characters", "filter": {"q": prefix},
        })
        targets_by_name = {}
        deadline = time.monotonic() + 35
        while time.monotonic() < deadline and len(targets_by_name) < len(names):
            frame = receive_or_timeout(live, 3)
            if frame is None or frame.get("subscription_id") != "remote-control-characters":
                continue
            for item in frame.get("data", {}).get("characters", []):
                if item.get("name") in names and item.get("server") == server and item.get("online") and item.get("session_id"):
                    targets_by_name[item["name"]] = item
            if any(process.poll() is not None for process in simulators):
                raise RuntimeError("a remote-control fixture exited before publishing its session")
        if len(targets_by_name) != len(names):
            raise RuntimeError("not all fixture sessions appeared in the live fleet snapshot")
        targets = [targets_by_name[name] for name in names[:3]]
        replacement_target = targets_by_name[replacement_name]
        all_targets = [*targets, replacement_target]
        targets_by_id = {target["character_id"]: target for target in all_targets}

        live.send_json({
            "type": "subscribe", "protocol_version": 1,
            "subscription_id": "remote-control-controls", "revision": 1,
            "stream": "controls",
            "filter": {"character_ids": [target["character_id"] for target in all_targets]},
        })
        controls = None
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            frame = receive_or_timeout(live, 3)
            if frame is None or frame.get("subscription_id") != "remote-control-controls":
                continue
            rows = frame.get("data", {}).get("targets", [])
            by_id = {row.get("character_id"): row for row in rows}
            if set(by_id) != set(targets_by_id):
                continue
            if any(
                by_id[target_id].get("controls", {}).get("session_id") != target["session_id"]
                or by_id[target_id].get("character", {}).get("session_id") != target["session_id"]
                or not by_id[target_id].get("controls", {}).get("training", {}).get("observed_at")
                for target_id, target in targets_by_id.items()
            ):
                continue
            controls = by_id
            break
        if controls is None:
            raise RuntimeError("matching-session capability/readback projection did not arrive")
        for target_id, row in controls.items():
            capabilities = row.get("controls", {}).get("capabilities", {})
            if not capabilities.get("bot.start", {}).get("supported"):
                raise RuntimeError("fixture did not advertise bot.start capability")
            if capabilities.get("client.clientless", {}).get("supported"):
                raise RuntimeError("fixture incorrectly advertised Clientless support")
            area = capabilities.get("training.area.set", {})
            target = targets_by_id[target_id]
            modes = set(area.get("modes", []))
            if target["name"] == replacement_name:
                if "current_position" not in modes or "named" in modes:
                    raise RuntimeError("fixture did not omit only the configured named training-area mode")
            elif not {"current_position", "named"}.issubset(modes):
                raise RuntimeError("fixture did not advertise both supported training-area modes")
            if not row.get("controls", {}).get("training", {}).get("training_available"):
                raise RuntimeError(
                    "fixture active training area readback was not available: "
                    + json.dumps(row.get("controls"), sort_keys=True)
                )

        def run_wave(action, args, confirmation, wave_targets, expected_states):
            subscription_id = "remote-result-" + uuid.uuid4().hex
            keys = ["remote35-" + uuid.uuid4().hex for _ in wave_targets]
            accepted = {}

            def submit_one(target, key):
                return request_json(WEB_URL + "/api/commands", "POST", {
                    "character_id": target["character_id"],
                    "expected_session_id": target["session_id"],
                    "name": action,
                    "args": args,
                    "confirmation": confirmation,
                    "idempotency_key": key,
                }, cookie)

            with concurrent.futures.ThreadPoolExecutor(max_workers=len(wave_targets)) as pool:
                responses = list(pool.map(lambda pair: submit_one(*pair), zip(wave_targets, keys)))
            for target, key, (status, _, value) in zip(wave_targets, keys, responses):
                if status != 202 or not value.get("command_id"):
                    raise RuntimeError("" + action + " admission failed: HTTP " + str(status) + " " + str(value))
                accepted[key] = {"target": target, "command_id": value["command_id"]}

            live.send_json({
                "type": "subscribe", "protocol_version": 1,
                "subscription_id": subscription_id, "revision": 1,
                "stream": "commands", "filter": {"idempotency_keys": keys},
            })
            results = {}
            deadline = time.monotonic() + 55
            while time.monotonic() < deadline and len(results) < len(keys):
                frame = receive_or_timeout(live, 3)
                if frame is None or frame.get("subscription_id") != subscription_id:
                    continue
                for result in frame.get("data", {}).get("commands", []):
                    key = result.get("idempotency_key")
                    if key in accepted and result.get("state") in ("completed", "failed"):
                        results[key] = result
            if len(results) != len(keys):
                raise RuntimeError("timed out waiting for exact command results for " + action)
            ordered = []
            for target, key, expected in zip(wave_targets, keys, expected_states):
                result = results[key]
                if result.get("command_id") != accepted[key]["command_id"]:
                    raise RuntimeError("live result did not match the admitted command ID")
                if result.get("character_id") != target["character_id"] or result.get("session_id") != target["session_id"]:
                    raise RuntimeError("live result crossed a character/session boundary")
                if result.get("state") != expected:
                    raise RuntimeError(action + " returned unexpected state: " + str(result.get("state")))
                ordered.append(result)
            return ordered

        start_results = run_wave(
            "bot.start", {}, False, targets,
            ["completed", "failed", "completed"],
        )
        if start_results[1].get("result_code") != "api_return_false" or start_results[1].get("api_return") is not False:
            raise RuntimeError("independent false fake adapter outcome was not preserved")
        if any(result.get("verification") != "api_confirmed" for index, result in enumerate(start_results) if index != 1):
            raise RuntimeError("successful fake bot.start results omitted API verification")

        position_results = run_wave(
            "training.area.set", {"mode": "current_position"}, False, targets,
            ["completed"] * len(targets),
        )
        observed_positions = []
        for result, (x, y) in zip(position_results, positions):
            effective = result.get("effective_args", {})
            if effective.get("mode") != "current_position" or effective.get("x") != x or effective.get("y") != y:
                raise RuntimeError("current-position command did not use its own execution-time character position")
            observed_positions.append((effective.get("x"), effective.get("y")))
        if len(set(observed_positions)) != len(targets):
            raise RuntimeError("identical current-position requests did not retain distinct effective coordinates")

        named = run_wave(
            "training.area.set", {"mode": "named", "name": "Fixture Area"},
            False, [targets[0]], ["completed"],
        )[0]
        if named.get("effective_args") != {"mode": "named", "name": "Fixture Area"}:
            raise RuntimeError("named-area result omitted its independent effective arguments")
        if named.get("observed_after", {}).get("training_x") != positions[0][0]:
            raise RuntimeError("named-area readback did not report the fixture's selected profile state")

        radius = run_wave(
            "training.radius.set", {"radius": 137.5}, False,
            [targets[0]], ["completed"],
        )[0]
        if radius.get("effective_args") != {"radius": 137.5} or radius.get("observed_after", {}).get("training_radius") != 137.5:
            raise RuntimeError("radius command omitted its separate effective/readback evidence")
        if radius.get("verification") != "observed":
            raise RuntimeError("radius readback was not marked observed")

        stopped = run_wave("bot.stop", {}, False, [targets[0]], ["completed"])[0]
        if stopped.get("api_return") is not True:
            raise RuntimeError("fake bot.stop did not retain its API return")

        trace_results = run_wave(
            "trace.start", {"name": "FixtureNuker"}, False, targets,
            ["completed"] * len(targets),
        )
        if any(result.get("effective_args") != {"name": "FixtureNuker"} for result in trace_results):
            raise RuntimeError("eligible trace targets did not receive the same player name")

        trace_stopped = run_wave("trace.stop", {}, False, [targets[0]], ["completed"])[0]
        if trace_stopped.get("api_return") is not True:
            raise RuntimeError("fake trace.stop did not retain its API return")

        returned = run_wave(
            "character.return", {}, True, [targets[0]], ["completed"],
        )[0]
        if returned.get("verification") != "api_confirmed" or returned.get("api_return") is not True:
            raise RuntimeError("Return Scroll did not preserve its verified fake adapter result")

        disconnected = run_wave(
            "character.disconnect", {}, True, [targets[2]], ["completed"],
        )[0]
        if disconnected.get("verification") != "unverified" or disconnected.get("result_code"):
            raise RuntimeError("void-return fake Disconnect was presented as an observed transition")

        run_wave("bot.start", {}, False, [replacement_target], ["completed"])
        replacement_session = None
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline and replacement_session is None:
            frame = receive_or_timeout(live, 3)
            if frame is None or frame.get("subscription_id") != "remote-control-characters":
                continue
            for item in frame.get("data", {}).get("characters", []):
                if (
                    item.get("character_id") == replacement_target["character_id"]
                    and item.get("online")
                    and item.get("session_id")
                    and item.get("session_id") != replacement_target["session_id"]
                ):
                    replacement_session = item["session_id"]
                    break
        if replacement_session is None:
            raise RuntimeError("controlled fixture did not publish its replacement session")
        stale_status, _, stale_response = request_json(
            WEB_URL + "/api/commands", "POST", {
                "character_id": replacement_target["character_id"],
                "expected_session_id": replacement_target["session_id"],
                "name": "bot.stop",
                "args": {},
                "confirmation": False,
                "idempotency_key": "remote35-stale-" + uuid.uuid4().hex,
            }, cookie,
        )
        if stale_status != 409 or stale_response.get("error") != "stale_session":
            raise RuntimeError("an old prepared session was not rejected after controlled replacement")
        run_wave(
            "bot.stop", {}, False,
            [{**replacement_target, "session_id": replacement_session}], ["completed"],
        )
        for process in simulators:
            output, _ = process.communicate(timeout=8)
            if "production command validation and worker" not in output:
                raise RuntimeError("production worker did not finish fixture calls cleanly: " + output[-1200:])
            if "REMOTE_CONTROL_SESSION_REPLACED" not in output and process is simulators[3]:
                raise RuntimeError("controlled replacement fixture did not report a new session")
            if '"disconnect"' in output and 'REMOTE_CONTROL_CALL ["disconnect"]' not in output:
                raise RuntimeError("fixture Disconnect evidence was not a local void-return call")
            if "client.clientless" in output:
                raise RuntimeError("unsupported Clientless appeared in fake API calls")
        print("PASS Issue #35 fixture: current capabilities, independent fan-out outcomes, execution-time positions, named/radius readback, intent flag, void Disconnect and unsupported Clientless")
    finally:
        if live:
            live.close()
        for process in simulators:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)


if __name__ == "__main__":
    main()
