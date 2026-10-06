#!/usr/bin/env python3
"""Disposable authenticated navigation flow using fixture adapters.

Requires an isolated Compose project/database and never invokes a real phBot API.
"""
from __future__ import annotations

import ast
import json
import os
import re
import select
import subprocess
import sys
import time
from socket import timeout as SocketTimeout
from http.cookies import SimpleCookie
from urllib.error import HTTPError
from urllib.request import Request, build_opener

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WEB_URL = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:53005").rstrip("/")
ORIGIN = os.environ.get("SMOKE_ORIGIN", WEB_URL)
OUTDOOR_SEAM = os.environ.get("PHMON_NAVIGATION_SMOKE_OUTDOOR_SEAM") == "1"
EXPECTED_SCRIPT = "\n".join(
    tuple("walk," + str(x) + ",1080,0" for x in (6410, 6450, 6500, 6540, 6580, 6650))
    if OUTDOOR_SEAM else ("walk,6429,1088,0", "walk,6430,1090,0")
)
DESTINATION = {"region": 25001, "x": 6650, "y": 1080, "z": 0} if OUTDOOR_SEAM else {
    "region": 25000, "x": 6430, "y": 1090, "z": 0,
}
LAST_NAVIGATION_ROUTES: list[dict] = []


def plugin_protocol_version() -> int:
    path = os.path.join(ROOT, "plugin", "PhMon.py")
    with open(path, encoding="utf-8") as handle:
        tree = ast.parse(handle.read(), filename=path)
    for node in tree.body:
        if not isinstance(node, ast.Assign):
            continue
        for target in node.targets:
            if isinstance(target, ast.Name) and target.id == "PROTOCOL_VERSION" and isinstance(node.value, ast.Constant):
                return int(node.value.value)
    raise RuntimeError("plugin PROTOCOL_VERSION was not found")


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
        raw = error.read().decode("utf-8")
        return error.code, error.headers, json.loads(raw) if raw else {}
    raw = response.read()
    return response.status, response.headers, json.loads(raw.decode("utf-8")) if raw else {}


def receive_or_timeout(client, timeout=5):
    try:
        return client.receive_json(timeout=timeout)
    except (TimeoutError, SocketTimeout):
        return None


def wait_for_subscription(live, subscription_id, predicate, timeout=30):
    global LAST_NAVIGATION_ROUTES
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        frame = receive_or_timeout(live, min(5, max(0.1, deadline - time.monotonic())))
        if frame is None or frame.get("subscription_id") != subscription_id:
            continue
        data = frame.get("data", {})
        if subscription_id == "navigation-map":
            LAST_NAVIGATION_ROUTES = [
                {key: route.get(key) for key in ("command_id", "status", "reason", "sequence", "updated_at")}
                for route in data.get("navigation", [])
            ]
        value = predicate(data)
        if value is not None:
            return value
    raise RuntimeError("timed out waiting for live " + subscription_id)


def wait_for_fixture_arrival(live):
    """Pause for browser inspection while continuing live heartbeat replies."""
    global LAST_NAVIGATION_ROUTES
    print("NAVIGATION_ROUTE_READY; send 'arrive' to continue the fixture", flush=True)
    while True:
        readable, _, _ = select.select([sys.stdin], [], [], 1.0)
        if readable:
            if sys.stdin.readline().strip() != "arrive":
                raise RuntimeError("interactive fixture did not receive the arrival instruction")
            return
        frame = receive_or_timeout(live, 1.0)
        if frame is None or frame.get("subscription_id") != "navigation-map":
            continue
        data = frame.get("data", {})
        LAST_NAVIGATION_ROUTES = [
            {key: route.get(key) for key in ("command_id", "status", "reason", "route_sequence", "updated_at")}
            for route in data.get("navigation", [])
        ]


def position_history_count(project, character_id):
    if not re.fullmatch(r"[a-zA-Z0-9_-]+", project):
        raise RuntimeError("invalid disposable Compose project name")
    if not re.fullmatch(r"[0-9a-fA-F-]{36}", character_id):
        raise RuntimeError("invalid simulator character ID")
    user = os.environ.get("PHMON_SMOKE_DB_USER", "phmon")
    database = os.environ.get("PHMON_SMOKE_DB_NAME", "phmon")
    sql = "SELECT count(*) FROM character_position_samples WHERE character_id='" + character_id + "'::uuid"
    result = subprocess.run(
        ["docker", "compose", "-p", project, "exec", "-T", "postgres", "psql", "-U", user,
         "-d", database, "-Atqc", sql],
        cwd=ROOT, check=True, capture_output=True, text=True, timeout=10,
    )
    return int(result.stdout.strip())


def main():
    os.environ["SMOKE_WEB_URL"] = WEB_URL
    sys.path.insert(0, os.path.join(ROOT, "scripts"))
    import live_smoke

    server = os.environ.get("PHMON_SIMULATOR_SERVER", "Greatest")
    prefix = "Issue27_" + str(time.time_ns())
    character_name = prefix + "_Navigator"
    project = required("PHMON_SMOKE_COMPOSE_PROJECT")
    simulator = None
    live = None
    try:
        opener = build_opener()
        code, headers, _ = request_json(
            opener, WEB_URL + "/api/auth/login", "POST",
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

        code, _, credential = request_json(opener, WEB_URL + "/api/agents/credentials", "POST", {}, cookie)
        if code != 201 or not credential.get("agent_id") or not credential.get("agent_token"):
            raise RuntimeError("disposable simulator credential creation failed")
        env = dict(os.environ)
        env.update({
            "PHMON_AGENT_URL": os.environ.get("PHMON_AGENT_URL", "ws://127.0.0.1:58081/agent"),
            "PHMON_AGENT_ID": credential["agent_id"],
            "PHMON_AGENT_TOKEN": credential["agent_token"],
            "PHMON_SIMULATOR_SCENARIO": "navigation",
            "PHMON_SIMULATOR_SERVER": server,
            "PHMON_SIMULATOR_CHARACTER": character_name,
            "PHMON_SIMULATOR_EXPECTED_SCRIPT": EXPECTED_SCRIPT,
            "PHMON_SIMULATOR_DESTINATION_REGION": str(DESTINATION["region"]),
            "PHMON_SIMULATOR_DESTINATION_X": str(DESTINATION["x"]),
            "PHMON_SIMULATOR_DESTINATION_Y": str(DESTINATION["y"]),
            "PHMON_SIMULATOR_PROGRESS_REGION": "25001",
            "PHMON_SIMULATOR_PROGRESS_X": "6540",
            "PHMON_SIMULATOR_PROGRESS_Y": "1080",
            "PHMON_SIMULATOR_CONNECT_TIMEOUT": "30",
            "PHMON_SIMULATOR_COMMAND_TIMEOUT": "60",
        })
        simulator = subprocess.Popen(
            [sys.executable, os.path.join(ROOT, "scripts", "agent_simulator.py")],
            cwd=ROOT, env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT, text=True, bufsize=1,
        )

        live_url = WEB_URL.replace("http://", "ws://", 1).replace("https://", "wss://", 1) + "/api/live"
        live = live_smoke.WebSocketClient(live_url, ORIGIN, cookie).connect()
        live.send_json({
            "type": "subscribe", "protocol_version": 1, "subscription_id": "navigation-character",
            "revision": 1, "stream": "characters", "filter": {"q": prefix},
        })
        character = wait_for_subscription(
            live, "navigation-character",
            lambda data: next((row for row in data.get("characters", [])
                               if row.get("name") == character_name and row.get("server") == server
                               and row.get("online") and row.get("session_id")), None),
        )
        live.send_json({
            "type": "subscribe", "protocol_version": 1, "subscription_id": "navigation-controls",
            "revision": 1, "stream": "controls", "filter": {"character_ids": [character["character_id"]]},
        })
        protocol_version = plugin_protocol_version()
        controls = wait_for_subscription(
            live, "navigation-controls",
            lambda data: next((target.get("controls") for target in data.get("targets", [])
                               if target.get("character_id") == character["character_id"]
                               and target.get("character", {}).get("session_id") == character["session_id"]
                               and target.get("controls", {}).get("agent_protocol_version") == protocol_version), None),
        )
        if not controls.get("capabilities", {}).get("character.navigate", {}).get("supported"):
            raise RuntimeError("current plugin protocol fixture did not report supported generated-script navigation")

        position_count_before_command = position_history_count(project, character["character_id"])

        key = "issue27-navigation-" + str(time.time_ns())
        code, _, accepted = request_json(opener, WEB_URL + "/api/commands", "POST", {
            "character_id": character["character_id"],
            "expected_session_id": character["session_id"],
            "name": "character.navigate",
            "args": DESTINATION,
            "confirmation": False,
            "idempotency_key": key,
        }, cookie)
        if code != 202 or not accepted.get("command_id"):
            raise RuntimeError("simulator navigation command admission failed: HTTP " + str(code))

        live.send_json({
            "type": "subscribe", "protocol_version": 1, "subscription_id": "navigation-command",
            "revision": 1, "stream": "commands", "filter": {"idempotency_keys": [key]},
        })
        live.send_json({
            "type": "subscribe", "protocol_version": 1, "subscription_id": "navigation-map",
            "revision": 1, "stream": "map", "filter": {
                "server": server, "area": "world", "floor": "world", "region": 0,
            },
        })
        completed = wait_for_subscription(
            live, "navigation-command",
            lambda data: next((item for item in data.get("commands", [])
                               if item.get("idempotency_key") == key and item.get("state") == "completed"), None),
            timeout=60,
        )
        if completed.get("command_id") != accepted["command_id"] or not completed.get("finished_at"):
            raise RuntimeError("durable command completion did not match the admitted request")
        route_snapshot = wait_for_subscription(
            live, "navigation-map",
            lambda data: next((route for route in data.get("navigation", [])
                               if route.get("command_id") == accepted["command_id"]), None),
            timeout=60,
        )
        if route_snapshot.get("status") == "arrived":
            raise RuntimeError("script acceptance was incorrectly presented as arrival")
        if route_snapshot.get("destination", {}).get("x") != DESTINATION["x"]:
            raise RuntimeError("route destination was not derived from frozen command arguments")
        command_finished = completed["finished_at"]
        count_before_route_progress = position_history_count(project, character["character_id"])
        # The route frame/store update itself must never write position history.
        if count_before_route_progress != position_count_before_command:
            raise RuntimeError("transient route state created a position-history row")

        if simulator.stdin is None:
            raise RuntimeError("simulator input pipe is unavailable")
        if OUTDOOR_SEAM:
            blocks = route_snapshot.get("blocks", [])
            if len(blocks) != 1 or len(blocks[0].get("points", [])) != 6:
                raise RuntimeError("outdoor tile seam split the initial walk block")
            # Plugin observation timestamps have second precision while route
            # invocation timestamps may have sub-second precision. Ensure this
            # controlled progress sample is strictly newer than route admission.
            time.sleep(1.1)
            simulator.stdin.write("progress\n")
            simulator.stdin.flush()
            remaining = wait_for_subscription(
                live, "navigation-map",
                lambda data: next((route for route in data.get("navigation", [])
                                   if route.get("command_id") == accepted["command_id"]
                                   and route.get("status") == "moving"), None),
            )
            blocks = remaining.get("blocks", [])
            if (len(blocks) != 1 or [point["x"] for point in blocks[0]["points"]] != [6580, 6650]
                    or remaining.get("current_anchor", {}).get("region") != 25001):
                raise RuntimeError("outdoor seam progress or remaining connector is incorrect")
            # Plugin observation timestamps have second precision and the
            # route store ignores duplicate/out-of-order samples. Ensure the
            # controlled arrival is a distinct fresh observation after progress.
            time.sleep(1.1)
        if os.environ.get("PHMON_NAVIGATION_SMOKE_HOLD_FOR_ARRIVAL") == "1":
            wait_for_fixture_arrival(live)
        simulator.stdin.write("arrive\n")
        simulator.stdin.flush()
        try:
            arrived = wait_for_subscription(
                live, "navigation-map",
                lambda data: next((route for route in data.get("navigation", [])
                                   if route.get("command_id") == accepted["command_id"]
                                   and route.get("status") == "arrived"), None),
                timeout=30,
            )
        except RuntimeError as error:
            if simulator and simulator.poll() is None:
                simulator.terminate()
                try:
                    output, _ = simulator.communicate(timeout=5)
                except subprocess.TimeoutExpired:
                    simulator.kill()
                    output, _ = simulator.communicate(timeout=5)
            else:
                output = ""
            raise RuntimeError(
                str(error) + "; last route states=" + json.dumps(LAST_NAVIGATION_ROUTES)
                + "; simulator output=" + output[-1200:]
            ) from error
        if arrived.get("updated_at", "") <= command_finished:
            raise RuntimeError("arrival observation did not occur after durable command completion")
        simulator.stdin.write("stop\n")
        simulator.stdin.flush()
        output, _ = simulator.communicate(timeout=10)
        if simulator.returncode != 0 or "PASS fixture navigation command" not in output:
            raise RuntimeError("production plugin fixture failed: " + output[-1200:])
        print("PASS authenticated v8 admission -> production plugin worker -> transient route snapshot -> later observed arrival; no route-only position-history write"
              + ("; continuous outdoor seam and skipped-waypoint progress" if OUTDOOR_SEAM else ""))
    finally:
        if live:
            live.close()
        if simulator and simulator.poll() is None:
            simulator.terminate()
            try:
                simulator.wait(timeout=5)
            except subprocess.TimeoutExpired:
                simulator.kill()
                simulator.wait(timeout=5)


if __name__ == "__main__":
    main()
