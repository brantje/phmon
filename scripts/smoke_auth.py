"""Shared operator login helpers for local stack smoke scripts."""
from __future__ import annotations

import json
import os
from http.cookies import SimpleCookie
from urllib.error import HTTPError
from urllib.request import Request, urlopen


def operator_secret(root: str) -> str:
    secret = os.environ.get("OPERATOR_ACCESS_SECRET")
    if secret:
        return secret
    try:
        with open(os.path.join(root, ".env"), encoding="utf-8") as env_file:
            for line in env_file:
                key, separator, value = line.partition("=")
                if separator and key.strip() == "OPERATOR_ACCESS_SECRET":
                    return value.strip().strip("\"'")
    except OSError:
        pass
    raise RuntimeError("OPERATOR_ACCESS_SECRET is required (set it or configure .env)")


def login_cookie(web_url: str, origin: str, root: str) -> str:
    cookie_name = os.environ.get("NUXT_OPERATOR_COOKIE_NAME", "phmon_operator")
    request = Request(
        web_url.rstrip("/") + "/api/auth/login",
        data=json.dumps({"secret": operator_secret(root)}).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Origin": origin,
            "Sec-Fetch-Site": "same-origin",
        },
        method="POST",
    )
    try:
        with urlopen(request, timeout=8) as response:
            body = json.loads(response.read().decode("utf-8"))
            set_cookie = response.headers.get("Set-Cookie", "")
    except HTTPError as error:
        raise RuntimeError("operator login failed: HTTP {}".format(error.code)) from error
    if not body.get("authenticated"):
        raise RuntimeError("operator login did not establish a session")
    parsed = SimpleCookie()
    parsed.load(set_cookie)
    if cookie_name not in parsed:
        raise RuntimeError("operator login did not issue the configured session cookie")
    return "{}={}".format(cookie_name, parsed[cookie_name].value)
