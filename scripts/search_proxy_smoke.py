#!/usr/bin/env python3
"""Read-only checks for the authenticated Item Search and Economy web proxies."""
import json
import os
from urllib.error import HTTPError
from urllib.request import Request, urlopen

from smoke_auth import login_cookie

web = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005").rstrip("/")
root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def read(path, cookie=""):
    request = Request(web + path, headers={"Cookie": cookie} if cookie else {})
    try:
        response = urlopen(request, timeout=8)
    except HTTPError as error:
        response = error
    with response:
        return response.status, response.headers, json.loads(response.read())


for path in ("/api/items", "/api/economy/offers"):
    status, _, _ = read(path)
    assert status == 401, (path, "unauthenticated status", status)

cookie = login_cookie(web, web, root)
for path, key in (("/api/items", "items"), ("/api/economy/offers", "offers")):
    status, headers, body = read(path + "?limit=1", cookie)
    assert status == 200, (path, status, body)
    assert headers.get("Cache-Control") == "no-store", path
    assert isinstance(body.get(key), list), (path, body)
    assert isinstance(body.get("total"), int), (path, body)
    assert len(body[key]) <= 1, (path, "limit was not forwarded")
    status, _, _ = read(path + "?limit=101", cookie)
    assert status == 400, (path, "validation status", status)
    print("PASS {}: authenticated response, query limit, no-store and HTTP 400/401".format(path))
