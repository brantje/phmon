#!/usr/bin/env python3
"""Authenticated browser-relay -> Go -> PostgreSQL -> production plugin-worker smoke.

Use only a disposable local test database and a dedicated simulator credential.
The simulator's bot.stop adapter is fake and this script never touches phBot.
"""
from __future__ import annotations

import http.cookiejar
import json
import os
import subprocess
import sys
import time
from socket import timeout as SocketTimeout
from http.cookies import SimpleCookie
from urllib.error import HTTPError
from urllib.request import Request, build_opener

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WEB_URL = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005").rstrip("/")
ORIGIN = os.environ.get("SMOKE_ORIGIN", WEB_URL)

def required(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise SystemExit(name + " is required")
    return value

def request_json(opener, url, method="GET", body=None, cookie=""):
    headers = {"Origin": ORIGIN, "Accept": "application/json"}
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body).encode("utf-8")
    if cookie:
        headers["Cookie"] = cookie
    request = Request(url, data=data, headers=headers, method=method)
    try:
        response = opener.open(request, timeout=5)
    except HTTPError as error:
        return error.code, error.headers, json.loads(error.read().decode("utf-8"))
    raw = response.read()
    return response.status, response.headers, json.loads(raw.decode("utf-8")) if raw else {}

def receive_or_timeout(client, timeout=5):
    try:
        return client.receive_json(timeout=timeout)
    except (TimeoutError, SocketTimeout):
        return None

def main():
    # Set before importing the browser relay smoke's dependency-free RFC6455 client.
    os.environ["SMOKE_WEB_URL"] = WEB_URL
    sys.path.insert(0, os.path.join(ROOT, "scripts"))
    import live_smoke

    server = os.environ.get("PHMON_SIMULATOR_SERVER", "Fixture Slice3")
    character_prefix = os.environ.get("PHMON_SIMULATOR_CHARACTER", "Slice3_" + str(int(time.time_ns())))
    character_names = [character_prefix + "_" + str(index) for index in range(3)]
    simulators = []
    live = None
    try:
        opener = build_opener()
        code, headers, _ = request_json(opener, WEB_URL + "/api/auth/login", "POST", {"secret": required("OPERATOR_ACCESS_SECRET")})
        if code != 200:
            raise RuntimeError("operator login failed: HTTP " + str(code))
        raw_cookie = headers.get("Set-Cookie", "")
        parsed = SimpleCookie(); parsed.load(raw_cookie)
        cookie_name = os.environ.get("NUXT_OPERATOR_COOKIE_NAME", "phmon_operator")
        if cookie_name not in parsed:
            raise RuntimeError("login did not issue the configured operator cookie")
        cookie = cookie_name + "=" + parsed[cookie_name].value

        code, _, credential = request_json(opener, WEB_URL + "/api/agents/credentials", "POST", {}, cookie)
        if code != 201 or not credential.get("agent_id") or not credential.get("agent_token"):
            raise RuntimeError("simulator credential creation failed: HTTP " + str(code))
        for index, character_name in enumerate(character_names):
            env = dict(os.environ)
            env.update({
                "PHMON_AGENT_URL": os.environ.get("PHMON_AGENT_URL", "ws://127.0.0.1:8081/agent"),
                "PHMON_AGENT_ID": credential["agent_id"],
                "PHMON_AGENT_TOKEN": credential["agent_token"],
                "PHMON_SIMULATOR_SCENARIO": "commands",
                "PHMON_SIMULATOR_SERVER": server,
                "PHMON_SIMULATOR_CHARACTER": character_name,
                "PHMON_SIMULATOR_COMMAND_TIMEOUT": "60",
                "PHMON_SIMULATOR_CONNECT_TIMEOUT": "30",
                "PHMON_SIMULATOR_BOT_STOP_RESULT": "false" if index == 1 else "true",
            })
            simulators.append(subprocess.Popen(
                [sys.executable, os.path.join(ROOT, "scripts", "agent_simulator.py")],
                cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
            ))

        live = live_smoke.WebSocketClient(WEB_URL.replace("http://", "ws://", 1).replace("https://", "wss://", 1) + "/api/live", ORIGIN, cookie).connect()
        live.send_json({"type":"subscribe","protocol_version":1,"subscription_id":"command-targets","revision":1,"stream":"characters","filter":{"q":character_prefix}})
        targets_by_name = {}
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline and len(targets_by_name) < len(character_names):
            frame = receive_or_timeout(live, timeout=5)
            if frame is None:
                continue
            if frame.get("subscription_id") != "command-targets":
                continue
            data = frame.get("data", {})
            for item in data.get("characters", []):
                if item.get("name") in character_names and item.get("server") == server and item.get("online") and item.get("session_id"):
                    targets_by_name[item["name"]] = item
            if any(process.poll() is not None for process in simulators):
                raise RuntimeError("a simulator exited before publishing a current session")
        if len(targets_by_name) != len(character_names):
            raise RuntimeError("not all simulator sessions appeared in the live character snapshot")
        targets = [targets_by_name[name] for name in character_names]

        live.send_json({"type":"subscribe","protocol_version":1,"subscription_id":"fanout-controls","revision":1,"stream":"controls","filter":{"character_ids":[target["character_id"] for target in targets]}})
        controls = None
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            frame = receive_or_timeout(live, timeout=5)
            if frame is None or frame.get("subscription_id") != "fanout-controls":
                continue
            rows = frame.get("data", {}).get("targets", [])
            if len(rows) == len(targets) and all(
                row.get("character", {}).get("session_id") == row.get("controls", {}).get("session_id")
                and row.get("character_id") == row.get("character", {}).get("character_id")
                for row in rows
            ):
                controls = rows
                break
        if controls is None:
            raise RuntimeError("multi-character controls projection did not return matching sessions")

        keys = ["command-smoke-" + str(time.time_ns()) + "-" + str(index) for index in range(len(targets))]
        accepted_by_key = {}
        for target, key in zip(targets, keys):
            code, _, accepted = request_json(opener, WEB_URL + "/api/commands", "POST", {
                "character_id": target["character_id"],
                "expected_session_id": target["session_id"],
                "name": "bot.stop", "args": {}, "confirmation": False,
                "idempotency_key": key,
            }, cookie)
            if code != 202 or not accepted.get("command_id"):
                raise RuntimeError("command admission failed: HTTP " + str(code) + " " + str(accepted))
            accepted_by_key[key] = {"command_id": accepted["command_id"], "target": target}

        live.send_json({"type":"subscribe","protocol_version":1,"subscription_id":"command-exact-results","revision":1,"stream":"commands","filter":{"idempotency_keys":keys}})
        completed_by_key = {}
        deadline = time.monotonic() + 60
        while time.monotonic() < deadline and len(completed_by_key) < len(keys):
            frame = receive_or_timeout(live, timeout=5)
            if frame is None or frame.get("subscription_id") != "command-exact-results":
                continue
            items = frame.get("data", {}).get("commands", [])
            for item in items:
                key = item.get("idempotency_key")
                if key in accepted_by_key and item.get("state") in ("completed", "failed"):
                    completed_by_key[key] = item
        for index, (key, admitted) in enumerate(accepted_by_key.items()):
            completed = completed_by_key.get(key)
            target = admitted["target"]
            if not completed or completed.get("command_id") != admitted["command_id"]:
                raise RuntimeError("exact-key results did not recover the admitted command ID")
            if completed.get("character_id") != target["character_id"] or completed.get("session_id") != target["session_id"]:
                raise RuntimeError("exact-key result crossed a character or session boundary")
            expected_state = "failed" if index == 1 else "completed"
            expected_return = index != 1
            if completed.get("state") != expected_state:
                raise RuntimeError("a sibling execution outcome did not remain independent")
            if completed.get("verification") != "api_confirmed" or completed.get("api_return") is not expected_return:
                raise RuntimeError("authoritative result did not preserve fake adapter evidence")
            if index == 1 and completed.get("result_code") != "api_return_false":
                raise RuntimeError("failed fake adapter result omitted its result code")
            if not completed.get("finished_at"):
                raise RuntimeError("exact-key result omitted its finish timestamp")
        for simulator in simulators:
            output, _ = simulator.communicate(timeout=10)
            if "production worker invoked bot.stop once" not in output:
                raise RuntimeError("fake callback adapter did not confirm one invocation: " + output[-1000:])
        print("PASS three authenticated simulator workers: set-based controls -> independent session-fenced admissions -> exact-key recovery of command IDs and execution evidence")
    finally:
        if live: live.close()
        for simulator in simulators:
            if simulator.poll() is None:
                simulator.terminate()
                try: simulator.wait(timeout=5)
                except subprocess.TimeoutExpired: simulator.kill(); simulator.wait(timeout=5)

if __name__ == "__main__":
    main()
