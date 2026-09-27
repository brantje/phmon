#!/usr/bin/env python3
"""Wait for one provisioned test agent to reach the expected UI-visible state."""
import json
import os
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from smoke_auth import login_cookie

web = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005")
agent_id = os.environ.get("EXPECT_AGENT_ID")
expected_connected = os.environ.get("EXPECT_AGENT_CONNECTED", "1") == "1"
timeout = float(os.environ.get("AGENT_SMOKE_TIMEOUT", "45"))
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
operator_cookie = login_cookie(web, web, root)

if not agent_id:
    raise SystemExit("EXPECT_AGENT_ID is required")

last_error = None
deadline = time.time() + timeout
while time.time() < deadline:
    try:
        request = Request(web + "/api/agents", headers={"Cookie": operator_cookie})
        response = urlopen(request, timeout=5)
        with response:
            body = json.loads(response.read().decode())
        if body.get("status") != "ok":
            raise AssertionError("agent API status is not ok")
        match = next(
            (
                agent
                for agent in body.get("agents", [])
                if agent.get("agent_id") == agent_id
            ),
            None,
        )
        if (
            match is not None
            and bool(match.get("connected")) == expected_connected
        ):
            state = "connected" if expected_connected else "disconnected"
            print("PASS agent {} is {}".format(agent_id, state))
            raise SystemExit(0)
        last_error = "agent state was {}".format(
            None if match is None else match.get("connected")
        )
    except (HTTPError, URLError, ValueError, AssertionError) as error:
        last_error = str(error)
    time.sleep(0.5)

raise SystemExit("agent state did not converge: " + str(last_error))
