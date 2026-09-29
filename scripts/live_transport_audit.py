#!/usr/bin/env python3
"""Fail CI if browser live-data code regresses to HTTP/SSE transport."""

from __future__ import annotations

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
APP = ROOT / "web" / "app"

errors: list[str] = []
sources = {
    path: path.read_text(encoding="utf-8")
    for path in APP.rglob("*")
    if path.suffix in {".ts", ".vue"}
}

live_endpoint = re.compile(r"/api/(?:agents(?:\b|/)|characters(?:\b|/)|groups(?:\b|/))")

for path, text in sources.items():
    rel = path.relative_to(ROOT)

    for match in re.finditer(r"(?:useFetch|useAsyncData)\s*(?:<[^>]+>)?\s*\([^)]{0,400}", text, re.S):
        if live_endpoint.search(match.group(0)):
            errors.append(f"{rel}: live data must not use useFetch/useAsyncData")

    if re.search(r"new\s+EventSource\s*\(", text):
        errors.append(f"{rel}: SSE/EventSource is prohibited for live data")

    if re.search(r"['\"]\/api\/characters(?:\/|['\"])", text):
        errors.append(f"{rel}: browser character reads/actions must not use HTTP in this slice")

    allowed_agent_actions: set[tuple[int, int]] = set()
    for action in re.finditer(
        r"\$fetch(?:<[^>]+>)?\s*\(\s*(?P<quote>['\"])(?P<endpoint>/api/agents(?:/[^'\"]*)?)(?P=quote)",
        text,
    ):
        endpoint = action.group("endpoint")
        window = text[action.start() : action.start() + 500]
        method = re.search(r"method\s*:\s*['\"](POST|PATCH|PUT|DELETE)['\"]", window)
        verb = method.group(1) if method else ""
        if (endpoint == "/api/agents/credentials" and verb == "POST") or (
            endpoint.startswith("/api/agents/") and verb == "DELETE"
        ):
            allowed_agent_actions.add(action.span("endpoint"))

    for match in re.finditer(
        r"['\"](?P<endpoint>\/api\/agents(?:\/[^'\"]*)?)['\"]",
        text,
    ):
        if match.span("endpoint") in allowed_agent_actions:
            continue
        endpoint = match.group("endpoint")
        errors.append(f"{rel}: {endpoint} is a prohibited browser live HTTP read")

    for match in re.finditer(r"\$fetch(?:<[^>]+>)?\s*\(\s*['\"]\/api\/groups", text):
        window = text[match.start() : match.start() + 500]
        method = re.search(r"method\s*:\s*(?:isMember\s*\?\s*)?['\"](POST|PATCH|PUT|DELETE)['\"]", window)
        dynamic_member_method = "method: isMember ? 'DELETE' : 'PUT'" in window
        if not method and not dynamic_member_method:
            errors.append(f"{rel}: group HTTP access must be a mutation, never a live read")

    for retired in (
        "refreshAgents",
        "refreshCharacters",
        "refreshFleetCharacters",
        "refreshGroups",
        "refreshCharacterDetail",
    ):
        if retired in text:
            errors.append(f"{rel}: retired HTTP live refresh helper {retired} reappeared")

composable = APP / "composables" / "useLiveData.ts"
composable_text = sources.get(composable, "")
if "new WebSocket(browserLiveURL())" not in composable_text:
    errors.append("web/app/composables/useLiveData.ts: shared WebSocket connection missing")
if "'/api/live'" not in composable_text:
    errors.append("web/app/composables/useLiveData.ts: same-origin /api/live endpoint missing")

if errors:
    print("Live transport contract audit failed:", file=sys.stderr)
    for error in errors:
        print(f"  - {error}", file=sys.stderr)
    raise SystemExit(1)

print("Live transport contract audit passed: no browser HTTP/SSE live-data reads found.")
