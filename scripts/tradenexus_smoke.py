#!/usr/bin/env python3
"""Smoke the unauthenticated TradeNexus WebSocket without an operator cookie.

Set TRADENEXUS_URL to the Go server origin, for example http://127.0.0.1:8081.
The script connects two raw clients: one subscribes, the other reports, then
checks the broadcast, ack, a validation error, and the report rate limit.
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
import secrets
import socket
import ssl
import struct
import sys
from urllib.parse import urlparse


class SmokeError(RuntimeError):
    pass


class Client:
    def __init__(self, url: str) -> None:
        parsed = urlparse(url)
        if parsed.scheme not in {"ws", "wss", "http", "https"}:
            raise SmokeError(f"unsupported URL {url}")
        secure = parsed.scheme in {"wss", "https"}
        port = parsed.port or (443 if secure else 80)
        host = parsed.hostname or "127.0.0.1"
        raw = socket.create_connection((host, port), timeout=5)
        if secure:
            raw = ssl.create_default_context().wrap_socket(raw, server_hostname=host)
        raw.settimeout(5)
        self.sock = raw
        self.buffer = b""
        path = parsed.path or "/tradenexus"
        key = base64.b64encode(secrets.token_bytes(16)).decode("ascii")
        header_host = host if parsed.port is None else f"{host}:{parsed.port}"
        request = (
            f"GET {path} HTTP/1.1\r\n"
            f"Host: {header_host}\r\n"
            "Upgrade: websocket\r\n"
            "Connection: Upgrade\r\n"
            f"Sec-WebSocket-Key: {key}\r\n"
            "Sec-WebSocket-Version: 13\r\n"
            "Origin: http://advanced-auto-trade.local\r\n\r\n"
        )
        raw.sendall(request.encode("ascii"))
        head = b""
        while b"\r\n\r\n" not in head:
            chunk = raw.recv(4096)
            if not chunk:
                raise SmokeError("closed during upgrade")
            head += chunk
        header, self.buffer = head.split(b"\r\n\r\n", 1)
        if b" 101 " not in header.split(b"\r\n", 1)[0]:
            raise SmokeError(header.split(b"\r\n", 1)[0].decode("ascii", "replace"))
        accept = base64.b64encode(
            hashlib.sha1((key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").encode()).digest()
        )
        if accept not in header:
            raise SmokeError("invalid websocket accept")

    def send(self, payload: dict) -> None:
        body = json.dumps(payload).encode("utf-8")
        mask = secrets.token_bytes(4)
        masked = bytes(value ^ mask[index % 4] for index, value in enumerate(body))
        length = len(body)
        if length < 126:
            header = struct.pack("!BB", 0x81, 0x80 | length)
        elif length < 65536:
            header = struct.pack("!BBH", 0x81, 0x80 | 126, length)
        else:
            header = struct.pack("!BBQ", 0x81, 0x80 | 127, length)
        self.sock.sendall(header + mask + masked)

    def recv(self) -> dict:
        while True:
            frame = self._frame()
            opcode, payload = frame
            if opcode == 0x9:
                self.sock.sendall(struct.pack("!BB", 0x8A, 0x80) + secrets.token_bytes(4))
                continue
            if opcode == 0x8:
                raise SmokeError("server closed the socket")
            if opcode != 0x1:
                continue
            return json.loads(payload.decode("utf-8"))

    def _frame(self) -> tuple[int, bytes]:
        header = self._exact(2)
        opcode = header[0] & 0x0F
        length = header[1] & 0x7F
        if length == 126:
            length = struct.unpack("!H", self._exact(2))[0]
        elif length == 127:
            length = struct.unpack("!Q", self._exact(8))[0]
        return opcode, self._exact(length)

    def _exact(self, length: int) -> bytes:
        chunks = []
        if self.buffer:
            take = self.buffer[:length]
            chunks.append(take)
            self.buffer = self.buffer[len(take) :]
            length -= len(take)
        while length:
            chunk = self.sock.recv(length)
            if not chunk:
                raise SmokeError("socket closed")
            chunks.append(chunk)
            length -= len(chunk)
        return b"".join(chunks)

    def close(self) -> None:
        self.sock.close()


def main() -> int:
    base = os.environ.get("TRADENEXUS_URL", "http://127.0.0.1:8081").rstrip("/")
    url = base + "/tradenexus"
    listener = Client(url)
    publisher = Client(url)
    try:
        hello = listener.recv()
        if hello.get("type") != "hello" or hello.get("v") != 1:
            raise SmokeError(f"listener hello: {hello}")
        publisher.recv()
        listener.send({"v": 1, "type": "subscribe", "servers": ["Greatest", "greatest"]})
        subscribed = listener.recv()
        if subscribed.get("type") != "subscribed" or subscribed.get("servers") != ["Greatest"]:
            raise SmokeError(f"subscribe: {subscribed}")
        snapshot = listener.recv()
        if snapshot.get("type") != "thieves" or snapshot.get("sightings") != []:
            raise SmokeError(f"initial thieves snapshot: {snapshot}")
        publisher.send(
            {
                "v": 1,
                "type": "thief.report",
                "ref": "smoke-1",
                "server": "Greatest",
                "thief": {"name": "Bandit123"},
                "position": {"region": 24999, "x": 1234.5, "y": 678.9, "z": 12},
                "position_source": "thief",
                "reporter": {"name": "trader1", "app": "AdvancedAutoTrade", "version": "1.0.0"},
            }
        )
        ack = publisher.recv()
        if ack.get("type") != "ack" or ack.get("ref") != "smoke-1" or not ack.get("sighting_id"):
            raise SmokeError(f"ack: {ack}")
        seen = listener.recv()
        if seen.get("type") != "thief.sighting" or seen.get("sighting_id") != ack["sighting_id"]:
            raise SmokeError(f"broadcast: {seen}")
        if seen.get("origin") != "external" or seen["thief"]["name"] != "Bandit123":
            raise SmokeError(f"broadcast body: {seen}")
        publisher.send({"v": 1, "type": "thief.report", "ref": "bad", "server": "Greatest"})
        error = publisher.recv()
        if error.get("type") != "error" or error.get("ref") != "bad" or error.get("code") != "invalid_message":
            raise SmokeError(f"error frame: {error}")
        limited = None
        for index in range(110):
            publisher.send(
                {
                    "v": 1,
                    "type": "thief.report",
                    "ref": f"rate-{index}",
                    "server": "Other",
                    "thief": {"name": "Rate"},
                }
            )
            frame = publisher.recv()
            while frame.get("type") == "thief.sighting":
                frame = publisher.recv()
            if frame.get("code") == "rate_limited":
                limited = frame
                break
            if frame.get("type") != "ack":
                raise SmokeError(f"unexpected rate frame: {frame}")
        if limited is None or not str(limited.get("ref", "")).startswith("rate-"):
            raise SmokeError("rate limit was not returned")
    finally:
        listener.close()
        publisher.close()
    print("tradenexus smoke passed")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (SmokeError, OSError, json.JSONDecodeError, TimeoutError) as exc:
        print(f"tradenexus smoke failed: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
