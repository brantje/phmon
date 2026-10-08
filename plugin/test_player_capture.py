"""Synthetic diagnostic envelopes, not captured target-server packet fixtures."""
import importlib.util
import json
import os
import struct
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('player_capture_test_plugin', os.path.join(os.path.dirname(__file__), 'PhMon.py'))
plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plugin)


class PlayerCaptureTest(unittest.TestCase):
    def test_export_with_bare_spool_filename(self):
        worker = object.__new__(plugin.AgentWorker)
        worker.config = {'death_spool_path': 'events.json'}
        worker._player_capture_export = True
        worker._player_capture = plugin.PassivePlayerCapture()
        worker._player_capture.begin({'server': 'Synthetic Fixture'}, 15)
        worker._player_capture.enqueue(0x3015, b'fixture')
        with tempfile.TemporaryDirectory() as directory:
            with patch.object(plugin.os, 'getcwd', return_value=directory), patch.object(plugin, '_log') as log:
                worker._export_player_capture_if_requested()
            files = os.listdir(directory)
            self.assertEqual(len(files), 1)
            self.assertTrue(files[0].startswith('player-capture-'))
            self.assertTrue(files[0].endswith('.json'))
            with open(os.path.join(directory, files[0])) as stream:
                report = json.load(stream)
            self.assertEqual(report['records'][0]['payload_hex'], b'fixture'.hex())
            self.assertFalse(report['decoder_enabled'])
            self.assertFalse(report['sanitized'])
            self.assertIn('saved', log.call_args[0][0])
            self.assertFalse(worker._player_capture_export)

    def test_disabled_and_allowlisted_only(self):
        capture = plugin.PassivePlayerCapture()
        self.assertFalse(capture.enqueue(0x3015, b'fixture'))
        self.assertTrue(capture.begin({'server': 'Synthetic Fixture'}, 15))
        self.assertFalse(capture.enqueue(0x7001, b'not-server-observation'))
        self.assertTrue(capture.enqueue(0x3015, b'fixture'))
        report = capture.report()
        self.assertFalse(report['decoder_enabled'])
        self.assertFalse(report['sanitized'])
        self.assertEqual(report['records'][0]['payload_hex'], b'fixture'.hex())

    def test_group_segments_preserve_exact_order(self):
        capture = plugin.PassivePlayerCapture()
        capture.begin({}, 15)
        for opcode, payload in [(0x3017, struct.pack('<BH', 1, 2)), (0x3019, b'first'), (0x3019, b'second'), (0x3018, b'')]:
            self.assertTrue(capture.enqueue(opcode, payload))
        report = capture.report()
        self.assertEqual(report['groups'][0]['segments'], [2, 3])
        self.assertEqual(report['groups'][0]['count'], 2)
        self.assertEqual(report['groups'][0]['bytes'], 11)

    def test_malformed_or_incomplete_groups_never_become_lifecycle(self):
        capture = plugin.PassivePlayerCapture()
        with patch.object(plugin, '_monotonic', return_value=10):
            capture.begin({}, 1)
            capture.enqueue(0x3017, b'bad-envelope')
            capture.drain()
            self.assertEqual(capture.report()['groups'], [])
            capture.begin({}, 1)
            capture.enqueue(0x3017, struct.pack('<BH', 1, 1))
            capture.drain()
        with patch.object(plugin, '_monotonic', return_value=12):
            self.assertEqual(capture.report()['reason'], 'incomplete_group')
            self.assertFalse(capture.enqueue(0x3019, b'late'))

    def test_overflow_and_reset_are_bounded(self):
        capture = plugin.PassivePlayerCapture()
        capture.begin({}, 15)
        for _ in range(capture.MAX_RECORDS):
            self.assertTrue(capture.enqueue(0x3015, b'1'))
        self.assertFalse(capture.enqueue(0x3015, b'overflow'))
        self.assertEqual(capture.report()['reason'], 'overflow')
        capture.reset('disconnect')
        self.assertFalse(capture.enqueue(0x3015, b'old session'))
        capture.begin({'session_id': 'new'}, 15)
        self.assertTrue(capture.enqueue(0x3016, b'new incarnation'))
        self.assertEqual(capture.report()['records'][0]['sequence'], 1)
        self.assertFalse(capture.begin({}, 31))

    def test_oversized_and_unexpected_payload(self):
        capture = plugin.PassivePlayerCapture()
        capture.begin({}, 15)
        self.assertFalse(capture.enqueue(0x3015, b'x' * (capture.MAX_PACKET + 1)))
        self.assertEqual(capture.report()['reason'], 'invalid_or_oversized_packet')


if __name__ == '__main__':
    unittest.main()
