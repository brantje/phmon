import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import time
import types
import unittest


class PetItemCaptureTests(unittest.TestCase):
    def load(self, pets, inventory, lines):
        phbot = types.ModuleType('phBot')
        phbot.get_pets = lambda: pets()
        phbot.get_inventory = lambda: inventory()
        phbot.log = lines.append
        qtbind = types.ModuleType('QtBind')
        qtbind.init = lambda *_args: object()
        qtbind.createLabel = lambda *_args: None
        qtbind.createButton = lambda *_args: None
        self.before = {name: sys.modules.get(name) for name in ('phBot', 'QtBind')}
        sys.modules['phBot'] = phbot
        sys.modules['QtBind'] = qtbind
        path = Path(__file__).with_name('PetItemCapture.py')
        spec = importlib.util.spec_from_file_location('pet_capture_test', path)
        probe = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(probe)
        return probe

    def tearDown(self):
        for name, module in getattr(self, 'before', {}).items():
            if module is None:
                sys.modules.pop(name, None)
            else:
                sys.modules[name] = module

    def test_probe_is_unarmed_by_default_and_records_only_pet_packets(self):
        lines = []
        probe = self.load(lambda: {}, lambda: {'items': []}, lines)
        self.assertTrue(probe.handle_joymax(0x30C8, b'\x01\x02'))
        self.assertEqual(probe._packets, [])
        with tempfile.TemporaryDirectory() as directory:
            probe.OUTPUT_DIRECTORY = directory
            probe.arm_pet_capture()
            self.assertTrue(probe.handle_joymax(0x30C8, b'\x01\x02\x03'))
            self.assertTrue(probe.handle_joymax(0xB034, b'\xAA' * (probe.MAX_PACKET_BYTES + 10)))
            self.assertTrue(probe.handle_joymax(0x3040, b'secret'))
            self.assertTrue(probe.handle_silkroad(0x7034, b'\x04\x05'))
            self.assertTrue(probe.handle_silkroad(0x7025, b'secret'))
            self.assertEqual([p['opcode'] for p in probe._packets], ['0x30C8', '0xB034', '0x7034'])
            self.assertEqual([p['direction'] for p in probe._packets],
                             ['server_to_client', 'server_to_client', 'client_to_server'])
            self.assertEqual(probe._packets[0]['hex'], '010203')
            self.assertTrue(probe._packets[1]['truncated'])
            self.assertEqual(len(probe._packets[1]['hex']), probe.MAX_PACKET_BYTES * 2)
            probe.stop_pet_capture()
            written = list(Path(directory).glob('pet-item-capture-*.json'))
            self.assertEqual(len(written), 1)
            document = json.loads(written[0].read_text())
            self.assertEqual(document['packet_total'], 3)
            self.assertEqual(document['packet_counts'], {
                'server_to_client:0x30C8': 1,
                'server_to_client:0xB034': 1,
                'client_to_server:0x7034': 1,
            })
            self.assertNotIn('secret', written[0].read_text())

    def test_unchanged_inventory_is_not_resampled_and_expiration_is_ignored(self):
        lines = []
        state = {'exp': -100, 'plus': 0}

        def inventory():
            return {'items': [
                {'model': 1, 'name': 'Spear', 'plus': state['plus'], 'expiration': state['exp']},
                {'model': 2, 'name': 'Bolt', 'quantity': 5, 'expiration': state['exp']},
            ]}
        probe = self.load(lambda: {}, inventory, lines)
        with tempfile.TemporaryDirectory() as directory:
            probe.OUTPUT_DIRECTORY = directory
            probe.arm_pet_capture()
            self.assertEqual(len(probe._snapshots[0]['inventory']), 2)
            self.assertTrue(any('no pet at arm time' in line for line in lines))
            for _ in range(3):
                state['exp'] -= 1
                probe._next_sample_at = 0.0
                probe.event_loop()
            self.assertEqual(len(probe._snapshots), 1)
            state['plus'] = 5
            probe._next_sample_at = 0.0
            probe.event_loop()
            self.assertEqual(len(probe._snapshots), 2)
            changed = probe._snapshots[1]['inventory']
            self.assertEqual([row['slot'] for row in changed], [0])
            self.assertNotIn('expiration', json.dumps(changed))

    def test_file_limit_reports_omitted_packets(self):
        probe = self.load(lambda: {}, lambda: {'items': []}, [])
        with tempfile.TemporaryDirectory() as directory:
            probe.OUTPUT_DIRECTORY = directory
            probe.arm_pet_capture()
            probe.MAX_FILE_BYTES = 1200
            for _ in range(8):
                probe.handle_joymax(0xB034, b'\xAB' * 100)
            probe.stop_pet_capture()
            written = list(Path(directory).glob('pet-item-capture-*.json'))
            self.assertEqual(len(written), 1)
            self.assertLessEqual(written[0].stat().st_size, probe.MAX_FILE_BYTES)
            document = json.loads(written[0].read_text())
            self.assertEqual(document['packet_total'], 8)
            self.assertGreater(document['packets_omitted_for_size'], 0)

    def test_disconnect_writes_capture_once(self):
        lines = []
        probe = self.load(lambda: {}, lambda: {'items': []}, lines)
        with tempfile.TemporaryDirectory() as directory:
            probe.OUTPUT_DIRECTORY = directory
            probe.arm_pet_capture()
            probe.handle_joymax(0x30C8, b'\x01')
            probe.disconnected()
            probe.finished()
            written = list(Path(directory).glob('pet-item-capture-*.json'))
            self.assertEqual(len(written), 1)
            self.assertEqual(json.loads(written[0].read_text())['reason'], 'disconnected')
            self.assertTrue(any('no 0xB034 item operation' in line for line in lines))

    def test_move_only_capture_warns_about_missing_pet_pickup(self):
        lines = []
        probe = self.load(lambda: {}, lambda: {'items': []}, lines)
        with tempfile.TemporaryDirectory() as directory:
            probe.OUTPUT_DIRECTORY = directory
            probe.arm_pet_capture()
            probe.handle_joymax(0xB034, b'\x01\x10\x01\x00')
            probe.handle_joymax(0xB034, b'\x01\x1a\x01\x00')
            probe.stop_pet_capture()
            document = json.loads(next(Path(directory).glob('pet-item-capture-*.json')).read_text())
            self.assertEqual(document['packet_counts']['server_to_client:0xB034:0x10'], 1)
            self.assertEqual(document['packet_counts']['server_to_client:0xB034:0x1A'], 1)
            self.assertTrue(any('no candidate floor-to-pet' in line for line in lines))

    def test_pet_pickup_subtype_is_counted_without_decoding(self):
        lines = []
        probe = self.load(lambda: {}, lambda: {'items': []}, lines)
        with tempfile.TemporaryDirectory() as directory:
            probe.OUTPUT_DIRECTORY = directory
            probe.arm_pet_capture()
            probe.handle_joymax(0xB034, b'\x01\x11\x01\x00')
            probe.stop_pet_capture()
            document = json.loads(next(Path(directory).glob('pet-item-capture-*.json')).read_text())
            self.assertEqual(document['packet_counts']['server_to_client:0xB034:0x11'], 1)
            self.assertFalse(any('no candidate floor-to-pet' in line for line in lines))

    def test_packet_count_is_bounded_and_item_rows_are_primitives_only(self):
        lines = []
        item = {'name': 'Spear', 'model': 4115, 'plus': 5,
                'blues': {70: 6, 76: 6}, 'whites': {8: 100}, 'handle': object()}
        probe = self.load(lambda: {7: {'type': 'pick', 'items': [None, item]}},
                          lambda: {'items': [item]}, lines)
        with tempfile.TemporaryDirectory() as directory:
            probe.OUTPUT_DIRECTORY = directory
            probe.arm_pet_capture()
            for _ in range(probe.MAX_PACKETS + 20):
                probe.handle_joymax(0xB034, b'\x00')
            self.assertEqual(len(probe._packets), probe.MAX_PACKETS)
            self.assertEqual(probe._packet_total, probe.MAX_PACKETS + 20)
            self.assertEqual(probe._packet_counts['server_to_client:0xB034'], probe.MAX_PACKETS + 20)
            probe.event_loop()
            snapshot = probe._snapshots[-1]
            row = snapshot['pets']['7']['items'][0]
            self.assertEqual(row['slot'], 1)
            pairs = dict((key, value) for key, value in row['fields']['pairs'])
            self.assertEqual(pairs['name'], 'Spear')
            self.assertEqual(pairs['plus'], 5)
            self.assertEqual(pairs['blues']['pairs'], [[70, 6], [76, 6]])
            self.assertEqual(pairs['handle'], 'object')
            self.assertEqual(snapshot['pets']['7']['type'], 'pick')
            self.assertNotIn('object object', json.dumps(snapshot))
            probe._capture_until = time.monotonic() - 1
            probe.event_loop()
            self.assertTrue(any('wrote pet-item-capture-' in line for line in lines))


    def test_pet_stat_leads_only_kept_for_current_pet_ids(self):
        lines = []
        pets = {'77': {'type': 'wolf', 'servername': 'COS_P_WOLF_001', 'hp': 5, 'level': 3, 'items': []}}
        probe = self.load(lambda: pets, lambda: {'items': []}, lines)
        with tempfile.TemporaryDirectory() as directory:
            probe.OUTPUT_DIRECTORY = directory
            probe.arm_pet_capture()
            self.assertEqual(probe._snapshots[0]['pets']['77']['level'], 3)
            self.assertTrue(probe.handle_joymax(0x3057, (77).to_bytes(4, 'little') + b'\x01'))
            self.assertTrue(probe.handle_joymax(0x3057, (99).to_bytes(4, 'little') + b'\x01'))
            self.assertTrue(probe.handle_joymax(0x3057, b'\x01'))
            self.assertEqual([p['opcode'] for p in probe._packets], ['0x3057'])
            for _ in range(40):
                probe.handle_joymax(0x30C9, (77).to_bytes(4, 'little'))
            self.assertEqual(probe._lead_counts[0x30C9], probe.MAX_LEAD_PACKETS_PER_OPCODE)
            probe.stop_pet_capture()


if __name__ == '__main__':
    unittest.main()
