#!/usr/bin/env python3
"""Authenticated browser-relay -> Go -> PostgreSQL -> production plugin-worker smoke.

Use only a disposable local test database and a dedicated simulator credential.
The simulator's selected command adapter is fake and this script never touches phBot.
"""
from __future__ import annotations

import concurrent.futures
import http.cookiejar
import json
import os
import subprocess
import sys
import time
from socket import timeout as SocketTimeout
from http.cookies import SimpleCookie
from urllib.error import HTTPError, URLError
from urllib.parse import quote
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

def ingest_online_targets(targets_by_name, characters, character_names, server):
    if not isinstance(characters, list):
        return
    for item in characters:
        if not isinstance(item, dict):
            continue
        if (
            item.get("name") in character_names
            and item.get("server") == server
            and item.get("online")
            and item.get("session_id")
        ):
            targets_by_name[item["name"]] = item

def fetch_http_characters(opener, cookie, character_prefix, server):
    path = (
        WEB_URL
        + "/api/characters?q="
        + quote(character_prefix)
        + "&server="
        + quote(server)
    )
    try:
        code, _, body = request_json(opener, path, cookie=cookie)
    except (URLError, TimeoutError, SocketTimeout, OSError):
        return []
    if code != 200 or not isinstance(body, dict):
        return []
    characters = body.get("characters", [])
    return characters if isinstance(characters, list) else []

def main():
    # Set before importing the browser relay smoke's dependency-free RFC6455 client.
    os.environ["SMOKE_WEB_URL"] = WEB_URL
    sys.path.insert(0, os.path.join(ROOT, "scripts"))
    import live_smoke

    command_name = os.environ.get("PHMON_SMOKE_COMMAND", "bot.stop")
    if command_name not in ("bot.stop", "character.reverse_return"): raise RuntimeError("unsupported fixture command")
    reverse_args = {"type": int(os.environ.get("PHMON_SMOKE_REVERSE_TYPE", "0"))}
    if reverse_args["type"] >= 2: reverse_args["name"] = os.environ.get("PHMON_SMOKE_REVERSE_NAME", "Jangan")
    skip_third = command_name == "character.reverse_return" and os.environ.get("PHMON_SMOKE_SKIP_THIRD") == "true"
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
        agent_http = os.environ.get("PHMON_AGENT_URL", "ws://127.0.0.1:8081/agent").replace(
            "ws://", "http://", 1
        ).replace("wss://", "https://", 1).rsplit("/agent", 1)[0]
        ready_deadline = time.monotonic() + 30
        while time.monotonic() < ready_deadline:
            try:
                ready_code, _, ready_body = request_json(opener, agent_http + "/readyz")
            except (URLError, TimeoutError, SocketTimeout, OSError):
                ready_code, ready_body = None, None
            if ready_code == 200 and isinstance(ready_body, dict) and ready_body.get("status") == "ok":
                break
            time.sleep(0.25)
        else:
            raise RuntimeError("agent server did not report ready before command smoke")
        live = live_smoke.WebSocketClient(
            WEB_URL.replace("http://", "ws://", 1).replace("https://", "wss://", 1) + "/api/live",
            ORIGIN,
            cookie,
        ).connect()
        live.send_json({
            "type": "subscribe",
            "protocol_version": 1,
            "subscription_id": "command-targets",
            "revision": 1,
            "stream": "characters",
            "filter": {"q": character_prefix, "server": server},
        })

        targets_by_name = {}
        connect_timeout = float(os.environ.get("PHMON_SIMULATOR_CONNECT_TIMEOUT", "45"))

        def wait_for_character(character_name: str) -> None:
            deadline = time.monotonic() + connect_timeout
            last_http_poll = 0.0
            last_refresh = 0.0
            while time.monotonic() < deadline:
                now = time.monotonic()
                if now - last_refresh >= 1.0:
                    live.send_json({
                        "type": "refresh",
                        "protocol_version": 1,
                        "subscription_id": "command-targets",
                        "revision": 1,
                    })
                    last_refresh = now
                if now - last_http_poll >= 0.5:
                    ingest_online_targets(
                        targets_by_name,
                        fetch_http_characters(opener, cookie, character_prefix, server),
                        character_names,
                        server,
                    )
                    last_http_poll = now
                if character_name in targets_by_name:
                    return
                frame = receive_or_timeout(live, timeout=1)
                if frame is not None and frame.get("subscription_id") == "command-targets":
                    ingest_online_targets(
                        targets_by_name,
                        frame.get("data", {}).get("characters", []),
                        character_names,
                        server,
                    )
                if character_name in targets_by_name:
                    return
                if any(process.poll() is not None for process in simulators):
                    exited = next(process for process in simulators if process.poll() is not None)
                    output = (exited.stdout.read() if exited.stdout else "")[-2000:]
                    raise RuntimeError(
                        "a simulator exited before publishing a current session: " + output
                    )
            raise RuntimeError(
                "simulator session did not appear for " + character_name + " within "
                + str(int(connect_timeout))
                + "s"
            )

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
                "PHMON_SIMULATOR_CONNECT_TIMEOUT": str(int(connect_timeout)),
                "PHMON_SIMULATOR_BOT_STOP_RESULT": "false" if index == 1 else "true",
            })
            if skip_third and index == 2: env["PHMON_SIMULATOR_SKIP_REVERSE"] = "true"
            simulators.append(subprocess.Popen(
                [sys.executable, os.path.join(ROOT, "scripts", "agent_simulator.py")],
                cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
            ))
            wait_for_character(character_name)

        if len(targets_by_name) != len(character_names):
            missing = [name for name in character_names if name not in targets_by_name]
            raise RuntimeError(
                "not all simulator sessions appeared in the live character snapshot; missing "
                + ", ".join(missing)
                + "; observed "
                + ", ".join(sorted(targets_by_name))
            )
        targets = [targets_by_name[name] for name in character_names]

        live.send_json({"type":"subscribe","protocol_version":1,"subscription_id":"fanout-controls","revision":1,"stream":"controls","filter":{"character_ids":[target["character_id"] for target in targets]}})
        controls = None
        deadline = time.monotonic() + 15
        expected_by_id = {target["character_id"]: target for target in targets}
        while time.monotonic() < deadline:
            frame = receive_or_timeout(live, timeout=5)
            if frame is None or frame.get("subscription_id") != "fanout-controls":
                continue
            rows = frame.get("data", {}).get("targets", [])
            rows_by_id = {}
            for row in rows:
                character_id = row.get("character_id")
                target = expected_by_id.get(character_id)
                if target is None or character_id in rows_by_id:
                    rows_by_id = {}
                    break
                if (
                    row.get("character", {}).get("session_id") != target["session_id"]
                    or row.get("controls", {}).get("session_id") != target["session_id"]
                ):
                    rows_by_id = {}
                    break
                rows_by_id[character_id] = row
            if len(rows_by_id) == len(expected_by_id):
                controls = rows
                break
        if controls is None:
            raise RuntimeError("multi-character controls projection did not return matching sessions")

        if skip_third:
            third = targets[-1]
            third_caps = rows_by_id[third['character_id']]['controls']['capabilities'][command_name]
            if third_caps['supported']: raise RuntimeError('fixture skip target incorrectly reports capability')
            code,_,rejected = request_json(opener, WEB_URL + '/api/commands', 'POST', {
                'character_id': third['character_id'], 'expected_session_id': third['session_id'],
                'name': command_name, 'args': {'type':0}, 'confirmation':True, 'idempotency_key':'reverse-skipped-'+str(time.time_ns()),
            },cookie)
            if code != 422: raise RuntimeError('unsupported sibling was admitted: '+str(rejected))
            targets = targets[:-1]
            skipped_worker = simulators.pop()
            skipped_worker.terminate(); skipped_worker.communicate(timeout=5)
            print('PASS unsupported selected sibling skipped; backend also rejects an attempted admission')

        keys = ["command-smoke-" + str(time.time_ns()) + "-" + str(index) for index in range(len(targets))]
        accepted_by_key = {}
        def admit(target, key):
            code, _, accepted = request_json(opener, WEB_URL + "/api/commands", "POST", {
                "character_id": target["character_id"], "expected_session_id": target["session_id"],
                "name": command_name, "args": reverse_args if command_name=="character.reverse_return" else {},
                "confirmation": command_name=="character.reverse_return", "idempotency_key":key,
            },cookie)
            if code!=202 or not accepted.get("command_id"): raise RuntimeError("command admission failed: "+str(accepted))
            return key, {"command_id":accepted["command_id"],"target":target}
        with concurrent.futures.ThreadPoolExecutor(max_workers=len(targets)) as executor:
            futures=[executor.submit(admit,target,key) for target,key in zip(targets,keys)]
            for future in futures:
                key,accepted=future.result(); accepted_by_key[key]=accepted

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
            if command_name == "character.reverse_return" and completed.get("effective_args") != {"type": reverse_args["type"], "name": reverse_args.get("name", "")}:
                raise RuntimeError("Reverse return result changed the reviewed type or name")
            if index == 1 and completed.get("result_code") != "api_return_false":
                raise RuntimeError("failed fake adapter result omitted its result code")
            if not completed.get("finished_at"):
                raise RuntimeError("exact-key result omitted its finish timestamp")
        for simulator in simulators:
            output, _ = simulator.communicate(timeout=10)
            if "production worker invoked " + command_name + " once" not in output:
                raise RuntimeError("fake callback adapter did not confirm one invocation: " + output[-1000:])
        print("PASS authenticated simulator workers: set-based controls -> independent session-fenced admissions -> exact-key recovery of command IDs and execution evidence")
    finally:
        if live: live.close()
        for simulator in simulators:
            if simulator.poll() is None:
                simulator.terminate()
                try: simulator.wait(timeout=5)
                except subprocess.TimeoutExpired: simulator.kill(); simulator.wait(timeout=5)

if __name__ == "__main__":
    main()
