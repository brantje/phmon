#!/usr/bin/env python3
"""Exercise the browser-facing live WebSocket through the Nuxt same-origin relay."""

from __future__ import annotations

import base64
import hashlib
import json
import os
import secrets
import socket
import ssl
import struct
import time
from dataclasses import dataclass
from urllib.error import HTTPError
from urllib.parse import urlparse
from urllib.request import Request, urlopen

WEB_URL = os.environ.get("SMOKE_WEB_URL", "http://127.0.0.1:3005").rstrip("/")
LIVE_PROTOCOL_VERSION = 1
MAX_FRAME = 1024 * 1024


class WebSocketError(RuntimeError):
    pass


def _masked_frame(opcode: int, payload: bytes) -> bytes:
    mask = secrets.token_bytes(4)
    length = len(payload)
    first = 0x80 | opcode
    if length < 126:
        header = struct.pack("!BB", first, 0x80 | length)
    elif length < 65536:
        header = struct.pack("!BBH", first, 0x80 | 126, length)
    else:
        header = struct.pack("!BBQ", first, 0x80 | 127, length)
    masked = bytes(value ^ mask[index % 4] for index, value in enumerate(payload))
    return header + mask + masked


@dataclass
class Handshake:
    status: int
    headers: dict[str, str]


class WebSocketClient:
    def __init__(self, url: str, origin: str):
        self.url = url
        self.origin = origin
        self.sock: socket.socket | ssl.SSLSocket | None = None
        self.buffer = b""

    def connect(self) -> "WebSocketClient":
        parsed = urlparse(self.url)
        secure = parsed.scheme == "wss"
        port = parsed.port or (443 if secure else 80)
        raw = socket.create_connection((parsed.hostname, port), timeout=5)
        if secure:
            raw = ssl.create_default_context().wrap_socket(raw, server_hostname=parsed.hostname)
        raw.settimeout(5)
        self.sock = raw

        key = base64.b64encode(secrets.token_bytes(16)).decode("ascii")
        path = parsed.path or "/"
        if parsed.query:
            path += "?" + parsed.query
        host = parsed.hostname
        if parsed.port and parsed.port != (443 if secure else 80):
            host += ":" + str(parsed.port)
        request = (
            f"GET {path} HTTP/1.1\r\n"
            f"Host: {host}\r\n"
            "Upgrade: websocket\r\n"
            "Connection: Upgrade\r\n"
            f"Sec-WebSocket-Key: {key}\r\n"
            "Sec-WebSocket-Version: 13\r\n"
            f"Origin: {self.origin}\r\n"
            "\r\n"
        ).encode("ascii")
        raw.sendall(request)
        handshake = self._read_handshake()
        if handshake.status != 101:
            self.close()
            raise WebSocketError(f"upgrade rejected with HTTP {handshake.status}")
        expected = base64.b64encode(
            hashlib.sha1((key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").encode("ascii")).digest()
        ).decode("ascii")
        if handshake.headers.get("sec-websocket-accept") != expected:
            self.close()
            raise WebSocketError("invalid Sec-WebSocket-Accept")
        raw.settimeout(None)
        return self

    def _read_handshake(self) -> Handshake:
        assert self.sock is not None
        data = b""
        while b"\r\n\r\n" not in data:
            chunk = self.sock.recv(4096)
            if not chunk:
                raise WebSocketError("connection closed during upgrade")
            data += chunk
            if len(data) > 16384:
                raise WebSocketError("oversized upgrade response")
        head, self.buffer = data.split(b"\r\n\r\n", 1)
        lines = head.decode("iso-8859-1").split("\r\n")
        parts = lines[0].split()
        status = int(parts[1])
        headers: dict[str, str] = {}
        for line in lines[1:]:
            if ":" in line:
                name, value = line.split(":", 1)
                headers[name.strip().lower()] = value.strip()
        return Handshake(status, headers)

    def _recv_exact(self, length: int) -> bytes:
        assert self.sock is not None
        chunks: list[bytes] = []
        if self.buffer:
            take = self.buffer[:length]
            chunks.append(take)
            self.buffer = self.buffer[len(take) :]
            length -= len(take)
        while length:
            chunk = self.sock.recv(length)
            if not chunk:
                raise WebSocketError("WebSocket closed")
            chunks.append(chunk)
            length -= len(chunk)
        return b"".join(chunks)

    def _read_frame(self, timeout: float) -> tuple[int, bytes]:
        assert self.sock is not None
        self.sock.settimeout(timeout)
        try:
            first, second = struct.unpack("!BB", self._recv_exact(2))
            if first & 0x70 or not first & 0x80:
                raise WebSocketError("unsupported fragmented/extended frame")
            opcode = first & 0x0F
            masked = bool(second & 0x80)
            if masked:
                raise WebSocketError("server frame must not be masked")
            length = second & 0x7F
            if length == 126:
                length = struct.unpack("!H", self._recv_exact(2))[0]
            elif length == 127:
                length = struct.unpack("!Q", self._recv_exact(8))[0]
            if length > MAX_FRAME:
                raise WebSocketError("oversized server frame")
            return opcode, self._recv_exact(length)
        finally:
            try:
                self.sock.settimeout(None)
            except OSError:
                pass

    def send_json(self, value: dict) -> None:
        if self.sock is None:
            raise WebSocketError("not connected")
        payload = json.dumps(value, separators=(",", ":")).encode("utf-8")
        self.sock.sendall(_masked_frame(0x1, payload))

    def receive_json(self, timeout: float = 5) -> dict:
        deadline = time.monotonic() + timeout
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError("timed out waiting for live frame")
            opcode, payload = self._read_frame(remaining)
            if opcode == 0x1:
                value = json.loads(payload.decode("utf-8"))
                if value.get("type") == "heartbeat":
                    self.send_json({"type": "heartbeat", "protocol_version": LIVE_PROTOCOL_VERSION})
                    continue
                return value
            if opcode == 0x8:
                raise WebSocketError("server closed WebSocket")
            if opcode == 0x9:
                assert self.sock is not None
                self.sock.sendall(_masked_frame(0xA, payload))
                continue
            if opcode == 0xA:
                continue
            raise WebSocketError(f"unsupported opcode {opcode}")

    def close(self) -> None:
        sock, self.sock = self.sock, None
        if sock is None:
            return
        try:
            sock.sendall(_masked_frame(0x8, struct.pack("!H", 1000)))
        except OSError:
            pass
        try:
            sock.close()
        except OSError:
            pass


def live_url() -> str:
    parsed = urlparse(WEB_URL)
    scheme = "wss" if parsed.scheme == "https" else "ws"
    return f"{scheme}://{parsed.netloc}/api/live"


def subscribe(client: WebSocketClient, sub_id: str, revision: int, stream: str, filter_: dict | None = None) -> None:
    frame = {
        "type": "subscribe",
        "protocol_version": LIVE_PROTOCOL_VERSION,
        "subscription_id": sub_id,
        "revision": revision,
        "stream": stream,
    }
    if filter_:
        frame["filter"] = filter_
    client.send_json(frame)


def wait_for(client: WebSocketClient, predicate, timeout: float = 8) -> dict:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        frame = client.receive_json(max(0.1, deadline - time.monotonic()))
        if predicate(frame):
            return frame
    raise TimeoutError("matching live frame not received")


def snapshot(client: WebSocketClient, sub_id: str, revision: int, timeout: float = 8) -> dict:
    frame = wait_for(
        client,
        lambda item: item.get("type") == "snapshot"
        and item.get("subscription_id") == sub_id
        and item.get("revision") == revision,
        timeout,
    )
    if frame.get("protocol_version") != LIVE_PROTOCOL_VERSION:
        raise AssertionError(f"wrong live protocol version: {frame}")
    return frame.get("data") or {}


def http_json(path: str, method: str, body: dict | None = None):
    data = None if body is None else json.dumps(body).encode("utf-8")
    request = Request(
        WEB_URL + path,
        data=data,
        method=method,
        headers={
            "Content-Type": "application/json",
            "Origin": WEB_URL,
            "Sec-Fetch-Site": "same-origin",
        },
    )
    try:
        with urlopen(request, timeout=8) as response:
            payload = response.read()
            return response.status, json.loads(payload) if payload else None
    except HTTPError as exc:
        payload = exc.read()
        raise AssertionError(f"{method} {path} failed: HTTP {exc.code} {payload!r}") from exc


def assert_cross_origin_rejected() -> None:
    try:
        WebSocketClient(live_url(), "https://evil.example").connect()
    except WebSocketError as exc:
        if "HTTP 403" not in str(exc):
            raise AssertionError(f"unexpected cross-origin rejection: {exc}") from exc
        return
    raise AssertionError("cross-origin browser WebSocket unexpectedly connected")


def group_ids(data: dict) -> dict[str, str]:
    return {
        item["name"]: item["group_id"]
        for item in data.get("groups", [])
        if isinstance(item, dict) and "name" in item and "group_id" in item
    }


def main() -> None:
    assert_cross_origin_rejected()

    first = WebSocketClient(live_url(), WEB_URL).connect()
    second = WebSocketClient(live_url(), WEB_URL).connect()
    try:
        subscribe(first, "agents", 1, "agents")
        subscribe(first, "fleet", 1, "characters")
        subscribe(first, "groups", 1, "groups")
        subscribe(second, "groups", 1, "groups")

        agents = snapshot(first, "agents", 1)
        fleet = snapshot(first, "fleet", 1)
        snapshot(first, "groups", 1)
        snapshot(second, "groups", 1)
        if not isinstance(agents.get("agents"), list):
            raise AssertionError("agents snapshot has wrong shape")
        if not isinstance(fleet.get("characters"), list):
            raise AssertionError("character snapshot has wrong shape")

        # Manual refresh is a protocol message, not an HTTP read.
        first.send_json(
            {
                "type": "refresh",
                "protocol_version": LIVE_PROTOCOL_VERSION,
                "subscription_id": "agents",
                "revision": 1,
            }
        )
        snapshot(first, "agents", 1)

        # A newer filter revision wins. A delayed old revision is explicitly rejected.
        subscribe(first, "filtered", 2, "characters", {"q": "Fixture"})
        filtered = snapshot(first, "filtered", 2)
        if not isinstance(filtered.get("characters"), list):
            raise AssertionError("filtered character snapshot has wrong shape")
        subscribe(first, "filtered", 1, "characters", {"q": "obsolete"})
        rejected = wait_for(
            first,
            lambda item: item.get("type") == "subscription.rejected"
            and item.get("subscription_id") == "filtered"
            and item.get("revision") == 1,
        )
        if rejected.get("reason") != "obsolete_revision":
            raise AssertionError(f"wrong obsolete-subscription response: {rejected}")

        characters = fleet.get("characters") or []
        if characters:
            character_id = characters[0].get("character_id")
            if character_id:
                subscribe(
                    first,
                    "detail",
                    1,
                    "character",
                    {"character_id": character_id},
                )
                detail = snapshot(first, "detail", 1)
                if (detail.get("character") or {}).get("character_id") != character_id:
                    raise AssertionError("detail subscription returned wrong identity")

        # Both browser sockets must receive group replacements after the HTTP action.
        # The action response is used only for cleanup identity; UI live state comes
        # from the two WebSocket snapshots below.
        name = "live-smoke-" + secrets.token_hex(5)
        status, created = http_json("/api/groups", "POST", {"name": name})
        if status != 201 or not created or not created.get("group_id"):
            raise AssertionError(f"group creation failed: {status} {created}")
        group_id = created["group_id"]

        first_group_update = wait_for(
            first,
            lambda item: item.get("type") == "snapshot"
            and item.get("subscription_id") == "groups"
            and name in group_ids(item.get("data") or {}),
        )
        second_group_update = wait_for(
            second,
            lambda item: item.get("type") == "snapshot"
            and item.get("subscription_id") == "groups"
            and name in group_ids(item.get("data") or {}),
        )
        if group_ids(first_group_update["data"]).get(name) != group_id:
            raise AssertionError("first client received wrong group identity")
        if group_ids(second_group_update["data"]).get(name) != group_id:
            raise AssertionError("second client received wrong group identity")

        status, _ = http_json(f"/api/groups/{group_id}", "DELETE")
        if status != 204:
            raise AssertionError(f"group deletion failed: {status}")
        wait_for(
            first,
            lambda item: item.get("type") == "snapshot"
            and item.get("subscription_id") == "groups"
            and name not in group_ids(item.get("data") or {}),
        )
        wait_for(
            second,
            lambda item: item.get("type") == "snapshot"
            and item.get("subscription_id") == "groups"
            and name not in group_ids(item.get("data") or {}),
        )

        print(
            "Live WebSocket smoke passed: initial snapshots, refresh, filtering/revisions, "
            "detail when available, cross-client group changes and cross-origin rejection."
        )
    finally:
        first.close()
        second.close()


if __name__ == "__main__":
    main()
