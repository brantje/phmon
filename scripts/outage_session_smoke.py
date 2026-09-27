#!/usr/bin/env python3
"""Exercise live WebSocket cleanup across a real Compose PostgreSQL outage.

Two production plugin workers share one logical agent. PostgreSQL is stopped,
one socket closes while writes cannot persist, PostgreSQL returns without a Go
restart, and reconciliation must close only the dead generation's character.
"""
import json
import os
import signal
import subprocess
import sys
import time
import uuid
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from smoke_auth import login_cookie


web = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005")
server = os.environ.get("SMOKE_SERVER_URL", "http://127.0.0.1:8081")
agent_url = os.environ.get("PHMON_AGENT_URL", "ws://127.0.0.1:8081/agent")
simulators = []
recovered = False
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
operator_cookie = login_cookie(web, web, root)
compose_project = os.environ.get("PHMON_COMPOSE_PROJECT")
if not compose_project:
    raise SystemExit("PHMON_COMPOSE_PROJECT must name the disposable Compose test stack")
compose = ["docker", "compose", "--project-name", compose_project]


def request_json(url, data=None, headers=None):
    request_headers = {
        "Origin": web,
        "Sec-Fetch-Site": "same-origin",
        "Cookie": operator_cookie,
        **(headers or {}),
    }
    request = Request(
        url,
        data=data,
        headers=request_headers,
        method="GET" if data is None else "POST",
    )
    with urlopen(request, timeout=5) as response:
        return response.status, json.loads(response.read().decode())


def wait_for(description, predicate, timeout=45):
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        try:
            last = predicate()
            if last:
                print("PASS " + description)
                return
        except (OSError, ValueError, HTTPError, URLError) as error:
            last = str(error)
        time.sleep(0.3)
    raise RuntimeError("timed out waiting for " + description + ": " + str(last))


def stop_process(process):
    if process.poll() is None:
        process.send_signal(signal.SIGTERM)
        try:
            process.wait(timeout=8)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=3)


def main():
    global recovered
    token_response = Request(
        web + "/api/agents/credentials",
        data=b"{}",
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urlopen(token_response, timeout=10) as response:
        credential = json.loads(response.read().decode())
    agent_id, token = credential["agent_id"], credential["agent_token"]
    suffix = uuid.uuid4().hex[:10]
    names = ["OutageA" + suffix, "OutageB" + suffix]
    for name in names:
        env = os.environ.copy()
        env.update({
            "PHMON_AGENT_URL": agent_url,
            "PHMON_AGENT_ID": agent_id,
            "PHMON_AGENT_TOKEN": token,
            "PHMON_SIMULATOR_SCENARIO": "stable-character",
            "PHMON_SIMULATOR_SERVER": "Fixture Outage " + suffix,
            "PHMON_SIMULATOR_CHARACTER": name,
        })
        simulators.append(subprocess.Popen([sys.executable, "scripts/agent_simulator.py"], env=env))

    def both_online():
        _, body = request_json(web + "/api/characters?q=" + names[0])
        first = next((row for row in body.get("characters", []) if row["name"] == names[0]), None)
        _, body = request_json(web + "/api/characters?q=" + names[1])
        second = next((row for row in body.get("characters", []) if row["name"] == names[1]), None)
        _, agents = request_json(web + "/api/agents")
        agent = next((row for row in agents.get("agents", []) if row["agent_id"] == agent_id), None)
        return bool(first and first["online"] and second and second["online"] and agent and agent.get("active_connections") == 2)

    wait_for("two shared-credential sockets publish two online characters", both_online)
    subprocess.run(compose + ["stop", "postgres"], check=True)
    wait_for("database readiness reports outage", lambda: _status(server + "/readyz") == 503)
    stop_process(simulators[0])

    outage_env = os.environ.copy()
    outage_env["EXPECT_UNAVAILABLE"] = "1"
    subprocess.run([sys.executable, "scripts/smoke.py"], env=outage_env, check=True)
    subprocess.run(compose + ["up", "-d", "--wait", "--wait-timeout", "120", "postgres"], check=True)

    def recovered_characters():
        _, a_body = request_json(web + "/api/characters?q=" + names[0])
        _, b_body = request_json(web + "/api/characters?q=" + names[1])
        a = next((row for row in a_body.get("characters", []) if row["name"] == names[0]), None)
        b = next((row for row in b_body.get("characters", []) if row["name"] == names[1]), None)
        _, agent_body = request_json(web + "/api/agents")
        agent = next((row for row in agent_body.get("agents", []) if row["agent_id"] == agent_id), None)
        return bool(a and not a["online"] and b and b["online"] and agent and agent["connected"] and agent["active_connections"] == 1)

    wait_for("recovery closes only dead A while live B remains online and agent connected", recovered_characters)
    subprocess.run([sys.executable, "scripts/smoke.py"], check=True)
    recovered = True


def _status(url):
    try:
        with urlopen(url, timeout=3) as response:
            return response.status
    except HTTPError as error:
        return error.code


try:
    main()
finally:
    for process in simulators:
        stop_process(process)
    if not recovered:
        subprocess.run(compose + ["up", "-d", "--wait", "--wait-timeout", "120", "postgres"], check=False)
