#!/usr/bin/env python3
"""Hold a browser-facing live subscription across PostgreSQL outage/recovery."""

from __future__ import annotations

import os
import pathlib
import time

from live_smoke import LIVE_PROTOCOL_VERSION, WEB_URL, WebSocketClient, live_url, snapshot, subscribe, wait_for
from smoke_auth import login_cookie

READY_FILE = pathlib.Path(os.environ.get("LIVE_RECOVERY_READY_FILE", "/tmp/phmon-live-recovery-ready"))
STALE_FILE = pathlib.Path(os.environ.get("LIVE_RECOVERY_STALE_FILE", "/tmp/phmon-live-recovery-stale"))
TIMEOUT = float(os.environ.get("LIVE_RECOVERY_TIMEOUT", "90"))


def main() -> None:
    for marker in (READY_FILE, STALE_FILE):
        try:
            marker.unlink()
        except FileNotFoundError:
            pass

    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    operator_cookie = login_cookie(WEB_URL, WEB_URL, root)
    client = WebSocketClient(live_url(), WEB_URL, operator_cookie).connect()
    try:
        subscribe(client, "groups", 1, "groups")
        initial = snapshot(client, "groups", 1, timeout=15)
        if not isinstance(initial.get("groups"), list):
            raise AssertionError("initial groups snapshot has wrong shape")

        READY_FILE.write_text("ready\n", encoding="utf-8")

        unavailable = wait_for(
            client,
            lambda item: item.get("type") == "subscription.unavailable"
            and item.get("protocol_version") == LIVE_PROTOCOL_VERSION
            and item.get("subscription_id") == "groups"
            and item.get("revision") == 1,
            timeout=TIMEOUT,
        )
        if unavailable.get("reason") != "temporarily_unavailable":
            raise AssertionError(f"unexpected outage frame: {unavailable}")
        STALE_FILE.write_text("stale\n", encoding="utf-8")

        recovered = snapshot(client, "groups", 1, timeout=TIMEOUT)
        if not isinstance(recovered.get("groups"), list):
            raise AssertionError("recovery snapshot has wrong shape")

        print(
            "Live recovery smoke passed: the same browser-facing WebSocket received "
            "subscription.unavailable during PostgreSQL outage and a fresh snapshot "
            "after recovery without an agent event or HTTP fallback."
        )
    finally:
        client.close()


if __name__ == "__main__":
    main()
