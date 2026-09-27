#!/usr/bin/env python3
"""Verify a running stack; creates and removes one temporary group on healthy runs."""
import json
import os
from urllib.error import HTTPError
from urllib.request import Request, urlopen

backend = os.environ.get("SMOKE_BACKEND_URL", "http://127.0.0.1:8081")
web = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005")
unavailable = os.environ.get("EXPECT_UNAVAILABLE") == "1"


def request(url, data=None, method=None):
    try:
        request = Request(
            url,
            data=data,
            headers={"Content-Type": "application/json"} if data is not None else {},
            method=method or ("POST" if data is not None else "GET"),
        )
        response = urlopen(request, timeout=10)
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
assert "Dashboard" in body, "Missing dashboard"
assert 'href="/stats"' in body, "Missing Stats navigation link"
code, _, body = request(web + "/stats")
assert code == 200, code
assert "Stats" in body, "Missing Stats page"
assert "Characters" in body, "Missing character list on Stats page"
print("PASS dashboard and Stats pages render")

for path, key in (("/api/agents", "agents"), ("/api/characters", "characters"), ("/api/groups", "groups")):
    code, _, body = request(web + path)
    expected_api_code = 503 if unavailable else 200
    assert code == expected_api_code, (path, code, body)
    response = json.loads(body)
    records = response.get(key) if key != "agents" else response.get("agents")
    assert isinstance(records, list), (path, body)
    if unavailable and path == "/api/agents":
        assert response.get("status") == "unavailable", (path, body)
    if unavailable and path == "/api/characters":
        assert response.get("status") == "unavailable", (path, body)
    print("PASS {}: HTTP {}, {} records".format(path, code, len(records)))

unknown_id = "00000000-0000-4000-8000-000000000000"
code, _, _ = request(web + "/api/characters/" + unknown_id)
expected_detail = 503 if unavailable else 404
assert code == expected_detail, ("character detail status", code)
print("PASS character detail status is HTTP {}".format(code))

oversized = b'{"name":"' + b'x' * 5000 + b'"}'
code, _, _ = request(web + "/api/groups", oversized)
assert code == 413, ("oversized group body status", code)
print("PASS oversized group body rejected with HTTP 413")

if not unavailable:
    group_name = "PhMon smoke {}".format(os.getpid())
    code, _, body = request(
        web + "/api/groups",
        json.dumps({"name": group_name}).encode(),
    )
    assert code == 201, ("group create status", code, body)
    group_id = json.loads(body)["group_id"]
    code, _, body = request(
        web + "/api/groups/" + group_id,
        json.dumps({"name": group_name + " renamed"}).encode(),
        method="PATCH",
    )
    assert code == 204, ("group rename status", code, body)
    code, _, body = request(
        web + "/api/groups/" + group_id,
        method="DELETE",
    )
    assert code == 204, ("group delete status", code, body)
    print("PASS group proxy preserves HTTP 201/204 semantics")
