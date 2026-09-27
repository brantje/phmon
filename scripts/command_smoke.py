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
    character = os.environ.get("PHMON_SIMULATOR_CHARACTER", "Slice3_" + str(int(time.time())))
    simulator = None
    simulator_output = None
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
        env = dict(os.environ)
        env.update({
            "PHMON_AGENT_URL": os.environ.get("PHMON_AGENT_URL", "ws://127.0.0.1:8081/agent"),
            "PHMON_AGENT_ID": credential["agent_id"],
            "PHMON_AGENT_TOKEN": credential["agent_token"],
            "PHMON_SIMULATOR_SCENARIO": "commands",
            "PHMON_SIMULATOR_SERVER": server,
            "PHMON_SIMULATOR_CHARACTER": character,
            "PHMON_SIMULATOR_COMMAND_TIMEOUT": "60",
            "PHMON_SIMULATOR_CONNECT_TIMEOUT": "30",
        })
        simulator = subprocess.Popen([sys.executable, os.path.join(ROOT, "scripts", "agent_simulator.py")], cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)

        live = live_smoke.WebSocketClient(WEB_URL.replace("http://", "ws://", 1).replace("https://", "wss://", 1) + "/api/live", ORIGIN, cookie).connect()
        live.send_json({"type":"subscribe","protocol_version":1,"subscription_id":"command-target","revision":1,"stream":"characters","filter":{"q":character}})
        target = None
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            frame = receive_or_timeout(live, timeout=5)
            if frame is None:
                continue
            data = frame.get("data", {})
            for item in data.get("characters", []):
                if item.get("name") == character and item.get("server") == server and item.get("online") and item.get("session_id"):
                    target = item; break
            if target: break
            if simulator.poll() is not None:
                raise RuntimeError("simulator exited before publishing a current session")
        if not target:
            raise RuntimeError("no simulator session appeared in the live character snapshot")

        key = "command-smoke-" + str(time.time_ns())
        code, _, accepted = request_json(opener, WEB_URL + "/api/commands", "POST", {
            "character_id": target["character_id"],
            "expected_session_id": target["session_id"],
            "name": "bot.stop", "args": {}, "confirmation": False,
            "idempotency_key": key,
        }, cookie)
        if code != 202 or not accepted.get("command_id"):
            raise RuntimeError("command admission failed: HTTP " + str(code) + " " + str(accepted))

        live.send_json({"type":"subscribe","protocol_version":1,"subscription_id":"command-history","revision":1,"stream":"commands","filter":{"character_id":target["character_id"],"limit":25}})
        deadline = time.monotonic() + 45
        completed = None
        while time.monotonic() < deadline:
            frame = receive_or_timeout(live, timeout=5)
            if frame is None:
                continue
            items = frame.get("data", {}).get("commands", [])
            completed = next((item for item in items if item.get("command_id") == accepted["command_id"] and item.get("state") == "completed"), None)
            if completed: break
            if simulator.poll() is not None:
                simulator_output, _ = simulator.communicate()
                if "production worker invoked bot.stop once" not in simulator_output:
                    raise RuntimeError("simulator exited without fake adapter success: " + simulator_output[-1000:])
        if not completed or completed.get("verification") != "api_confirmed" or completed.get("api_return") is not True:
            raise RuntimeError("authoritative completed result did not arrive over /api/live")
        if simulator_output is None:
            simulator_output, _ = simulator.communicate(timeout=5)
        if "production worker invoked bot.stop once" not in simulator_output:
            raise RuntimeError("fake callback adapter did not confirm one invocation")
        print("PASS authenticated /api/live command smoke: durable admission -> one fake callback invocation -> api_confirmed result")
    finally:
        if live: live.close()
        if simulator is not None and simulator.poll() is None:
            simulator.terminate()
            try: simulator.wait(timeout=5)
            except subprocess.TimeoutExpired: simulator.kill(); simulator.wait(timeout=5)

if __name__ == "__main__":
    main()
