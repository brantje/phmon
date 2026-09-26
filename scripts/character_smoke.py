#!/usr/bin/env python3
"""Assert the deterministic character fixture reached the real API path.

Fixture records are produced only by scripts/agent_simulator.py in test stacks.
"""
import json
import os
import time
from urllib.request import urlopen

web = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005")
deadline = time.time() + float(os.environ.get("CHARACTER_SMOKE_TIMEOUT", "30"))
last_state = "no fixture records"
while time.time() < deadline:
    try:
        response = urlopen(web + "/api/characters?q=Fixture", timeout=5)
        with response:
            body = json.loads(response.read().decode())
        characters = {item["name"]: item for item in body.get("characters", [])}
        alpha = characters.get("FixtureAlpha")
        beta = characters.get("FixtureBeta")
        if alpha is not None and beta is not None:
            if alpha["character_id"] == beta["character_id"]:
                raise SystemExit("distinct fixture characters share an identity")
            if not alpha["online"] and beta["online"] and beta.get("level") == 42 and beta.get("gold") == 456 and beta.get("zone") == "Fixture Donwhang":
                print("PASS fixture identity, switched presence and current state")
                raise SystemExit(0)
            last_state = "Alpha=%s Beta=%s level=%s" % (alpha["online"], beta["online"], beta.get("level"))
    except (OSError, ValueError) as error:
        last_state = str(error)
    time.sleep(0.5)
raise SystemExit("fixture state did not converge: " + last_state)
