import importlib.util
from pathlib import Path
import sys
import time
import types
import unittest


class RecallCaptureTests(unittest.TestCase):
    def test_probe_is_unarmed_by_default_and_only_logs_bounded_candidate_payloads(self):
        lines = []
        phbot = types.ModuleType('phBot')
        phbot.get_npcs = lambda: {
            4: {'name': 'Hotan', 'servername': 'GATE_KT', 'region': 25000},
        }
        phbot.log = lines.append
        qtbind = types.ModuleType('QtBind')
        qtbind.init = lambda *_args: object()
        qtbind.createLabel = lambda *_args: None
        qtbind.createButton = lambda *_args: None
        before = {name: sys.modules.get(name) for name in ('phBot', 'QtBind')}
        sys.modules['phBot'] = phbot
        sys.modules['QtBind'] = qtbind
        try:
            path = Path(__file__).with_name('RecallPointCapture.py')
            spec = importlib.util.spec_from_file_location('recall_probe_test', path)
            probe = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(probe)
            self.assertTrue(probe.handle_silkroad(0x7059, b'\x04\x00\x00\x00'))
            self.assertFalse(lines)
            probe.arm_recall_capture()
            self.assertTrue(probe.handle_silkroad(0x7059, b'\x04\x00\x00\x00'))
            self.assertTrue(probe.handle_silkroad(0x7045, b'\x04\x00\x00\x00'))
            self.assertTrue(probe.handle_silkroad(0x7020, b'secret'))
            self.assertTrue(probe.handle_joymax(0xB059, b'secret'))
            self.assertTrue(probe.handle_joymax(0xB059, b'\x01'))
            self.assertIn('payload=04000000', '\n'.join(lines))
            self.assertIn('incoming opcode=0xB059 length=1 payload=01', '\n'.join(lines))
            self.assertNotIn('secret', '\n'.join(lines))
            probe._capture_until = time.monotonic() - 1
            probe.event_loop()
            self.assertTrue(any('complete outgoing=3 incoming=2' in line for line in lines))
        finally:
            for name, module in before.items():
                if module is None:
                    sys.modules.pop(name, None)
                else:
                    sys.modules[name] = module


if __name__ == '__main__':
    unittest.main()
