import importlib.util
import json
import os
import socket
import tempfile
import threading
import time
import unittest

MODULE_PATH = os.path.join(os.path.dirname(__file__), 'PhMon.py')
spec = importlib.util.spec_from_file_location('phmon_plugin', MODULE_PATH)
plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plugin)

AGENT_ID = '11111111-2222-4333-8444-555555555555'


class ConfigTests(unittest.TestCase):
    def test_valid_config_normalizes_agent_path(self):
        config = plugin.validate_config({
            'backend_url': 'ws://127.0.0.1:8081',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_token',
        })
        self.assertEqual(config['backend_url'], 'ws://127.0.0.1:8081/agent')

    def test_invalid_config_rejects_credentials_and_bad_identity(self):
        cases = [
            {'backend_url': 'http://127.0.0.1/agent', 'agent_id': AGENT_ID, 'agent_token': 'x'},
            {'backend_url': 'ws://user:pass@127.0.0.1/agent', 'agent_id': AGENT_ID, 'agent_token': 'x'},
            {'backend_url': 'ws://127.0.0.1/agent?token=x', 'agent_id': AGENT_ID, 'agent_token': 'x'},
            {'backend_url': 'ws://127.0.0.1/agent', 'agent_id': 'not-an-id', 'agent_token': 'x'},
            {'backend_url': 'ws://127.0.0.1/agent', 'agent_id': AGENT_ID, 'agent_token': 'has space'},
        ]
        for value in cases:
            with self.assertRaises(ValueError):
                plugin.validate_config(value)

    def test_load_config(self):
        handle, path = tempfile.mkstemp()
        os.close(handle)
        try:
            with open(path, 'w') as stream:
                json.dump({'backend_url': 'wss://example.test/agent', 'agent_id': AGENT_ID, 'agent_token': 'token'}, stream)
            loaded = plugin.load_config(path)
            self.assertEqual(loaded['agent_id'], AGENT_ID)
        finally:
            os.unlink(path)


class WebSocketFrameTests(unittest.TestCase):
    def _server_text_frame(self, payload):
        return bytes(bytearray([0x81, len(payload)])) + payload

    def test_client_frame_is_masked(self):
        frame = plugin._encode_client_frame(0x1, b'hello', b'\x01\x02\x03\x04')
        self.assertEqual(frame[0], 0x81)
        self.assertEqual(frame[1], 0x85)
        self.assertEqual(frame[2:6], b'\x01\x02\x03\x04')
        self.assertEqual(plugin._mask_payload(frame[6:], frame[2:6]), b'hello')

    def test_expected_accept_matches_rfc_example(self):
        self.assertEqual(
            plugin._expected_accept('dGhlIHNhbXBsZSBub25jZQ=='),
            's3pPLMBiTxaQ9kYGzzhZRbK+xOo=',
        )

    def test_reads_server_text_and_answers_ping(self):
        client_sock, server_sock = socket.socketpair()
        try:
            ws = plugin.WebSocketClient('ws://localhost/agent', 'token')
            ws._socket = client_sock
            payload = b'{"type":"hello.ack"}'
            server_sock.sendall(bytes(bytearray([0x89, 0x01])) + b'x')
            server_sock.sendall(self._server_text_frame(payload))
            value = ws.receive_json(timeout=1)
            self.assertEqual(value['type'], 'hello.ack')
            pong = server_sock.recv(64)
            self.assertEqual(pong[0] & 0x0f, 0x0a)
            self.assertTrue(pong[1] & 0x80)
        finally:
            client_sock.close()
            server_sock.close()

    def test_rejects_masked_server_frame(self):
        client_sock, server_sock = socket.socketpair()
        try:
            ws = plugin.WebSocketClient('ws://localhost/agent', 'token')
            ws._socket = client_sock
            server_sock.sendall(plugin._encode_client_frame(0x1, b'{}', b'1234'))
            with self.assertRaises(plugin.WebSocketClosed):
                ws.receive_json(timeout=1)
        finally:
            client_sock.close()
            server_sock.close()

    def test_invalid_json_is_rejected(self):
        client_sock, server_sock = socket.socketpair()
        try:
            ws = plugin.WebSocketClient('ws://localhost/agent', 'token')
            ws._socket = client_sock
            server_sock.sendall(self._server_text_frame(b'not-json'))
            with self.assertRaisesRegex(plugin.WebSocketClosed, 'invalid JSON message'):
                ws.receive_json(timeout=1)
        finally:
            client_sock.close()
            server_sock.close()

    def test_oversized_server_frame_is_rejected_before_payload(self):
        client_sock, server_sock = socket.socketpair()
        try:
            ws = plugin.WebSocketClient('ws://localhost/agent', 'token')
            ws._socket = client_sock
            server_sock.sendall(
                bytes(bytearray([0x81, 126])) +
                plugin.struct.pack('!H', plugin.MAX_MESSAGE_BYTES + 1)
            )
            with self.assertRaisesRegex(plugin.WebSocketClosed, 'WebSocket message exceeds limit'):
                ws.receive_json(timeout=1)
        finally:
            client_sock.close()
            server_sock.close()

    def test_no_data_poll_keeps_stream_parseable(self):
        client_sock, server_sock = socket.socketpair()
        try:
            ws = plugin.WebSocketClient('ws://localhost/agent', 'token')
            ws._socket = client_sock
            self.assertIsNone(ws.receive_json(timeout=0.01))

            payload = b'{"type":"hello.ack"}'
            server_sock.sendall(self._server_text_frame(payload))
            self.assertEqual(ws.receive_json(timeout=1)['type'], 'hello.ack')
        finally:
            client_sock.close()
            server_sock.close()

    def test_partial_frame_timeout_fails_connection(self):
        client_sock, server_sock = socket.socketpair()
        try:
            ws = plugin.WebSocketClient('ws://localhost/agent', 'token', connect_timeout=0.05)
            ws._socket = client_sock
            server_sock.sendall(b'\x81')
            with self.assertRaisesRegex(plugin.WebSocketClosed, 'timeout while receiving WebSocket frame'):
                ws.receive_json(timeout=1)
        finally:
            client_sock.close()
            server_sock.close()

    def test_segmented_server_frame_is_reassembled(self):
        client_sock, server_sock = socket.socketpair()
        sender = None
        try:
            ws = plugin.WebSocketClient('ws://localhost/agent', 'token', connect_timeout=0.5)
            ws._socket = client_sock
            payload = b'{"type":"hello.ack"}'
            frame = self._server_text_frame(payload)
            chunks = [frame[:1], frame[1:2], frame[2:7], frame[7:]]

            def send_chunks():
                for chunk in chunks:
                    server_sock.sendall(chunk)
                    time.sleep(0.01)

            sender = threading.Thread(target=send_chunks)
            sender.start()
            self.assertEqual(ws.receive_json(timeout=1)['type'], 'hello.ack')
            sender.join(1)
            self.assertFalse(sender.is_alive())
        finally:
            if sender is not None:
                sender.join(1)
            client_sock.close()
            server_sock.close()


class BackoffTests(unittest.TestCase):
    def test_backoff_caps_and_resets(self):
        backoff = plugin.ReconnectBackoff(lambda low, high: 1.0)
        self.assertEqual([backoff.next_delay() for _ in range(7)], [1.0, 2.0, 4.0, 8.0, 16.0, 30.0, 30.0])
        backoff.reset()
        self.assertEqual(backoff.next_delay(), 1.0)

    def test_ack_validation(self):
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture')
        interval = worker._validate_ack({
            'type': 'hello.ack',
            'protocol_version': 1,
            'heartbeat_interval_seconds': 10,
            'heartbeat_timeout_seconds': 30,
        })
        self.assertEqual(interval, 10)
        with self.assertRaises(plugin.WebSocketClosed):
            worker._validate_ack({'type': 'hello.ack', 'protocol_version': 2})


if __name__ == '__main__':
    unittest.main()
