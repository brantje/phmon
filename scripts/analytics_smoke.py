#!/usr/bin/env python3
"""Exercise analytics against fixture observations sent through the agent worker.

This starts only a local protocol simulator. It does not connect to phBot or
operate a game character; fixture observations are confined to the smoke stack.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import time
from datetime import datetime, timedelta, timezone

from live_smoke import WebSocketClient, http_json, live_url, snapshot, subscribe
from smoke_auth import login_cookie

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WEB_URL = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005").rstrip("/")


def find_fixture_character(data: dict, name: str) -> dict | None:
    return next(
        (row for row in data.get("characters", []) if row.get("name") == name),
        None,
    )


def progress_data(frame: dict) -> dict:
    return (frame.get("data") or {}).get("performance") or {}


def main() -> None:
    live_smoke_cookie = login_cookie(WEB_URL, WEB_URL, ROOT)
    # The shared HTTP helper reads this module variable for its same-origin
    # authenticated request headers.
    import live_smoke

    live_smoke.OPERATOR_COOKIE = live_smoke_cookie
    status, credential = http_json("/api/agents/credentials", "POST", {})
    if status != 201 or not credential or not credential.get("agent_id") or not credential.get("agent_token"):
        raise SystemExit(f"could not provision fixture agent: {status} {credential}")

    character = "AnalyticsFixture"
    server = "Fixture Silkroad"
    simulator_env = os.environ.copy()
    simulator_env.update(
        {
            "PHMON_AGENT_URL": os.environ.get(
                "PHMON_AGENT_URL", "ws://127.0.0.1:8081/agent"
            ),
            "PHMON_AGENT_ID": credential["agent_id"],
            "PHMON_AGENT_TOKEN": credential["agent_token"],
            "PHMON_SIMULATOR_SCENARIO": "analytics-history",
            "PHMON_SIMULATOR_SERVER": server,
            "PHMON_SIMULATOR_CHARACTER": character,
            "PHMON_SIMULATOR_RUN_SECONDS": "100",
        }
    )
    simulator = subprocess.Popen(
        [sys.executable, os.path.join(ROOT, "scripts", "agent_simulator.py")],
        cwd=ROOT,
        env=simulator_env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    client: WebSocketClient | None = None
    observer: WebSocketClient | None = None
    try:
        client = WebSocketClient(live_url(), WEB_URL, live_smoke_cookie).connect()
        subscribe(client, "analytics-fleet", 1, "characters")
        fleet = snapshot(client, "analytics-fleet", 1)
        fixture = find_fixture_character(fleet, character)
        deadline = time.monotonic() + 30
        while fixture is None and time.monotonic() < deadline:
            frame = client.receive_json(timeout=min(5, max(0.1, deadline - time.monotonic())))
            if frame.get("type") == "snapshot" and frame.get("subscription_id") == "analytics-fleet":
                fixture = find_fixture_character(frame.get("data") or {}, character)
        if fixture is None:
            raise AssertionError("fixture character did not arrive through the live character stream")
        if fixture.get("server") != server:
            raise AssertionError(f"fixture character has wrong server scope: {fixture}")

        now = datetime.now(timezone.utc)
        from_at = (now - timedelta(hours=23)).isoformat(timespec="seconds").replace("+00:00", "Z")
        to_at = (now + timedelta(minutes=3)).isoformat(timespec="seconds").replace("+00:00", "Z")
        death_filter = {
            "server": server,
            "character_id": fixture["character_id"],
            "view": "deaths",
            "from": from_at,
            "to": to_at,
            "timezone": "UTC",
            "group_by": "character",
        }
        subscribe(client, "analytics-deaths", 1, "analytics", death_filter)
        deaths = snapshot(client, "analytics-deaths", 1)
        deadline = time.monotonic() + 30
        while deaths.get("total") != "1" and time.monotonic() < deadline:
            client.send_json(
                {
                    "type": "refresh",
                    "protocol_version": 1,
                    "subscription_id": "analytics-deaths",
                    "revision": 1,
                }
            )
            try:
                frame = client.receive_json(timeout=3)
            except TimeoutError:
                continue
            if frame.get("type") == "snapshot" and frame.get("subscription_id") == "analytics-deaths":
                deaths = frame.get("data") or {}
        if deaths.get("total") != "1":
            raise AssertionError(f"production callback did not produce one canonical death: {deaths}")
        total_metric = next(
            (metric for metric in deaths.get("summary", []) if metric.get("key") == "total_deaths"),
            None,
        )
        if not total_metric or total_metric.get("value") != "1":
            raise AssertionError(f"death summary did not match the canonical total: {deaths}")
        if sum(int(point.get("value", "0")) for point in deaths.get("time_series", [])) != 1:
            raise AssertionError(f"death chart did not match the canonical total: {deaths}")
        occurrences = deaths.get("occurrences", [])
        if len(occurrences) != 1 or occurrences[0].get("kind") != "character.died":
            raise AssertionError(f"death table did not contain exactly one callback: {occurrences}")
        event_id = occurrences[0].get("event_id")
        leader = next(
            (metric for metric in deaths.get("summary", []) if metric.get("key") == "most_deaths_character"),
            None,
        )
        if (
            not event_id
            or not leader
            or leader.get("value") != character
            or fixture["character_id"] not in leader.get("href", "")
        ):
            raise AssertionError(f"death summary did not preserve the character drill-down: {deaths}")

        observer = WebSocketClient(live_url(), WEB_URL, live_smoke_cookie).connect()
        subscribe(observer, "analytics-deaths-second-client", 1, "analytics", death_filter)
        second_deaths = snapshot(observer, "analytics-deaths-second-client", 1)
        second_occurrences = second_deaths.get("occurrences", [])
        if second_deaths.get("total") != "1" or len(second_occurrences) != 1 or second_occurrences[0].get("event_id") != event_id:
            raise AssertionError(f"independent live clients disagreed on the canonical death: {deaths} / {second_deaths}")

        subscribe(
            client,
            "analytics-progress",
            1,
            "analytics",
            {
                "server": server,
                "character_id": fixture["character_id"],
                "view": "performance",
                "from": from_at,
                "to": to_at,
                "timezone": "UTC",
            },
        )

        deadline = time.monotonic() + 105
        latest = None
        while time.monotonic() < deadline:
            try:
                frame = client.receive_json(timeout=5)
            except TimeoutError:
                client.send_json(
                    {
                        "type": "refresh",
                        "protocol_version": 1,
                        "subscription_id": "analytics-progress",
                        "revision": 1,
                    }
                )
                continue
            if frame.get("type") != "snapshot" or frame.get("subscription_id") != "analytics-progress":
                continue
            latest = frame.get("data") or {}
            performance = progress_data(frame)
            xp_rate = (performance.get("rates") or {}).get("xp") or {}
            if xp_rate.get("has_rate"):
                if latest.get("status") not in ("available", "limited"):
                    raise AssertionError(f"rate arrived with unexpected snapshot status: {latest.get('status')}")
                if performance.get("character_id") != fixture["character_id"]:
                    raise AssertionError("analytics returned a different character identity")
                if performance.get("coverage_seconds", 0) < 60 or performance.get("botting_seconds", 0) <= 0:
                    raise AssertionError(f"performance did not include observed interval coverage: {performance}")
                break
            client.send_json(
                {
                    "type": "refresh",
                    "protocol_version": 1,
                    "subscription_id": "analytics-progress",
                    "revision": 1,
                }
            )
        else:
            raise AssertionError(f"no eligible rate history arrived; last snapshot={latest}")

        status, reset = http_json(
            "/api/analytics/rate-resets",
            "POST",
            {
                "character_id": fixture["character_id"],
                "character_name": character,
                "idempotency_key": "analytics-smoke-reset-0001",
            },
        )
        if status != 200 or not reset or reset.get("rates_only") is not True:
            raise AssertionError(f"rate reset did not persist through the authenticated API: {status} {reset}")

        client.send_json(
            {
                "type": "refresh",
                "protocol_version": 1,
                "subscription_id": "analytics-progress",
                "revision": 1,
            }
        )
        refreshed = None
        reset_deadline = time.monotonic() + 12
        while time.monotonic() < reset_deadline:
            frame = client.receive_json(timeout=min(5, max(0.1, reset_deadline - time.monotonic())))
            if frame.get("type") == "snapshot" and frame.get("subscription_id") == "analytics-progress":
                refreshed = frame.get("data") or {}
                rate = (((refreshed.get("performance") or {}).get("rates") or {}).get("xp") or {})
                if not rate.get("has_rate") and rate.get("eligible_seconds", 0) < 60:
                    break
        else:
            raise AssertionError(f"rate reset did not change the live baseline: {refreshed}")
        print(
            "PASS plugin callback death analytics across two live clients, Progress rates, "
            "and durable rate reset"
        )
    finally:
        if observer is not None:
            observer.close()
        if client is not None:
            client.close()
        if simulator.poll() is None:
            simulator.terminate()
        try:
            output, _ = simulator.communicate(timeout=8)
        except subprocess.TimeoutExpired:
            simulator.kill()
            output, _ = simulator.communicate(timeout=5)
        if simulator.returncode not in (0, -15) and output:
            sys.stderr.write(output)
        if simulator.returncode not in (0, -15):
            raise SystemExit(f"analytics fixture simulator failed with {simulator.returncode}")


if __name__ == "__main__":
    main()
