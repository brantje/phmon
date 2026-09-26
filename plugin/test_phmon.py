import importlib.util
import os
import socket
import tempfile
import threading
import time
import unittest
from unittest.mock import Mock, patch

MODULE_PATH = os.path.join(os.path.dirname(__file__), 'PhMon.py')
spec = importlib.util.spec_from_file_location('phmon_plugin', MODULE_PATH)
plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plugin)

AGENT_ID = '11111111-2222-4333-8444-555555555555'


class ConfigTests(unittest.TestCase):
    def test_profile_path_is_unavailable_until_phbot_reports_login(self):
        with patch.object(plugin, '_get_profile', return_value=None):
            self.assertIsNone(plugin._current_settings_path())

    def test_connected_callback_loads_saved_profile(self):
        with patch.object(plugin, '_load_active_profile') as load_profile:
            plugin.connected()
        load_profile.assert_called_once_with()

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

    def test_profile_settings_path_is_scoped_to_active_bot_profile(self):
        config_dir = os.path.join('C:', 'phBot', 'Config')
        first = plugin._profile_settings_path(
            config_dir,
            'C:\\phBot\\Config\\Venus_Alice.json',
            'Farm',
        )
        second = plugin._profile_settings_path(
            config_dir,
            'C:\\phBot\\Config\\Venus_Bob.json',
            'Farm',
        )
        alternate = plugin._profile_settings_path(
            config_dir,
            'C:\\phBot\\Config\\Venus_Alice.json',
            'Trade',
        )
        default = plugin._profile_settings_path(
            config_dir,
            'C:\\phBot\\Config\\Venus_Alice.json',
            '',
        )
        self.assertIn(os.path.join('PhMon', 'Venus_Alice.Farm-'), first)
        self.assertTrue(first.endswith('.cfg'))
        self.assertTrue(default.endswith(os.path.join('PhMon', 'Venus_Alice.default.cfg')))
        self.assertNotEqual(first, second)
        self.assertNotEqual(first, alternate)
        self.assertNotEqual(first, default)

    def test_saved_profile_config_round_trip(self):
        root = tempfile.mkdtemp()
        path = os.path.join(root, 'PhMon', 'Venus_Alice.Farm.cfg')
        config = {
            'backend_url': 'wss://example.test/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_secret',
        }
        try:
            saved = plugin.save_saved_config(path, config)
            loaded = plugin.load_saved_config(path)
            self.assertEqual(loaded, saved)
            with open(path, 'r') as stream:
                content = stream.read()
            self.assertNotIn('{', content)
            self.assertIn('agent_token=phm_secret', content)
        finally:
            if os.path.exists(path):
                os.unlink(path)
            directory = os.path.dirname(path)
            if os.path.isdir(directory):
                os.rmdir(directory)
            os.rmdir(root)

    def test_gui_config_reuses_hidden_token_only_for_same_identity(self):
        saved = plugin.validate_config({
            'backend_url': 'wss://example.test/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_secret',
        })
        current = plugin.config_from_gui_values(
            saved['backend_url'],
            saved['agent_id'],
            '',
            saved,
        )
        self.assertEqual(current['agent_token'], 'phm_secret')

        with self.assertRaisesRegex(ValueError, 'agent token is required'):
                plugin.config_from_gui_values(
                    'wss://other.example.test/agent',
                    saved['agent_id'],
                    '',
                    saved,
                )


class CharacterCollectorTests(unittest.TestCase):
    def test_recovers_character_when_plugin_loads_after_join_callback(self):
        worker = Mock()
        previous = (
            plugin._worker,
            plugin._character_joined,
            plugin._last_character_signature,
            plugin._last_character_sample_at,
        )
        try:
            plugin._worker = worker
            plugin._character_joined = None
            plugin._last_character_signature = None
            plugin._last_character_sample_at = 0
            with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                    patch.object(plugin, '_get_character_data', return_value={
                        'server': 'Test Server', 'name': 'nuker1', 'guild': 'Test Guild',
                        'level': 90, 'hp': 100, 'hp_max': 200, 'mp': 300,
                        'mp_max': 400, 'current_exp': 5, 'max_exp': 10,
                        'sp': 6, 'gold': 7, 'region': 25000,
                    }), \
                    patch.object(plugin, '_get_position', return_value={
                        'region': 25000, 'x': 1.0, 'y': 2.0, 'z': 3.0,
                    }), \
                    patch.object(plugin, '_get_zone_name', return_value='Jangan'):
                plugin._sample_character()

            worker.update_character.assert_called_once()
            identity, state = worker.update_character.call_args.args
            self.assertEqual(identity['name'], 'nuker1')
            self.assertEqual(identity['server'], 'Test Server')
            self.assertEqual(state['zone'], 'Jangan')
            self.assertTrue(plugin._character_joined)
        finally:
            (plugin._worker, plugin._character_joined,
             plugin._last_character_signature, plugin._last_character_sample_at) = previous

    def test_does_not_resurrect_character_after_disconnect(self):
        worker = Mock()
        with patch.object(plugin, '_worker', worker), \
                patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                patch.object(plugin, '_character_joined', False), \
                patch.object(plugin, '_get_character_data', return_value={
                    'server': 'Test Server', 'name': 'nuker1',
                }):
            plugin._sample_character()
        worker.update_character.assert_not_called()


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

    def test_tls_pending_bytes_skip_socket_select(self):
        class PendingSocket:
            def pending(self):
                return 2

        with patch.object(plugin.select, 'select', side_effect=AssertionError('select called')):
            self.assertTrue(plugin.WebSocketClient('ws://localhost/agent', 'token')
                            ._wait_for_frame_start(PendingSocket(), 1))

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
            'protocol_version': 2,
            'heartbeat_interval_seconds': 10,
            'heartbeat_timeout_seconds': 30,
        })
        self.assertEqual(interval, 10)
        with self.assertRaises(plugin.WebSocketClosed):
            worker._validate_ack({'type': 'hello.ack', 'protocol_version': 1})

    def test_character_messages_resolve_identity_then_use_explicit_id(self):
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture')

        class Transport:
            sent = []
            def send_json(self, value): self.sent.append(value)
            def receive_json(self, timeout=None):
                return {'type':'character.registered','protocol_version':2,'character_id':AGENT_ID}

        transport = Transport()
        sample = {'identity':{'server':'Silkroad','name':'Alpha','guild':''},'state':{'level':110,'hp':500,'botting':None}}
        worker._publish_sample(transport, sample, False)
        worker._publish_sample(transport, sample, False)
        self.assertEqual(transport.sent[0]['type'], 'character.identify')
        self.assertEqual(transport.sent[1]['type'], 'character.snapshot')
        self.assertEqual(transport.sent[1]['character_id'], AGENT_ID)
        self.assertEqual(transport.sent[2]['type'], 'character.state')
        self.assertEqual(transport.sent[2]['character_id'], AGENT_ID)


class WorkerStopTests(unittest.TestCase):
    def test_stop_worker_signals_without_joining_callback(self):
        worker = Mock()
        original = plugin._worker
        plugin._worker = worker
        try:
            plugin._stop_worker()
        finally:
            plugin._worker = original

        worker.stop.assert_called_once_with()
        worker.join.assert_not_called()

    def test_event_loop_displays_worker_connection_state(self):
        worker = Mock()
        worker.status = 'Connected to PhMon backend.'
        original = plugin._worker
        plugin._worker = worker
        try:
            with patch.object(plugin, '_load_active_profile'), patch.object(
                plugin, '_set_gui_status'
            ) as set_status:
                plugin.event_loop()
        finally:
            plugin._worker = original

        set_status.assert_called_once_with('Connected to PhMon backend.')


if __name__ == '__main__':
    unittest.main()
