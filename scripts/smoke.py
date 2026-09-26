#!/usr/bin/env python3
"""Verify a running stack. Uses only the Python standard library; never mutates data."""
import json
import os
from urllib.error import HTTPError
from urllib.request import urlopen

backend = os.environ.get("SMOKE_BACKEND_URL", "http://127.0.0.1:8081")
web = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005")
unavailable = os.environ.get("EXPECT_UNAVAILABLE") == "1"


def request(url):
    try:
        response = urlopen(url, timeout=10)
    except HTTPError as error:
        response = error
    with response:
        return response.status, response.headers, response.read().decode()


for url, expected_code, expected in [
    (backend + "/healthz", 200, {"status": "ok"}),
    (
        backend + "/readyz",
        503 if unavailable else 200,
        {"status": "unavailable", "database": "unavailable"}
        if unavailable
        else {"status": "ok", "database": "ok"},
    ),
    (
        web + "/api/health",
        503 if unavailable else 200,
        {"status": "unavailable", "database": "unavailable"}
        if unavailable
        else {"status": "ok", "database": "ok"},
    ),
]:
    code, headers, body = request(url)
    assert code == expected_code, (url, code, body)
    assert json.loads(body) == expected, (url, body)
    assert headers.get("Cache-Control") == "no-store", url
    print("PASS {}: HTTP {}".format(url, code))

code, _, body = request(web + "/")
assert code == 200, code
assert "PhMon" in body, "Missing application brand"
assert "Agent connections" in body, "Missing Slice 1 agent screen"
print("PASS page renders PhMon agent shell")
