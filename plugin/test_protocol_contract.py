import copy
import unittest

from scripts.plugin_protocol_contract import protocol_contract_changed


class ProtocolContractTests(unittest.TestCase):
    def setUp(self):
        self.before = {
            "frames": {
                "capabilities": [
                    {
                        "type": "command.capabilities",
                        "commands": [
                            {"name": "bot.start", "supported": True, "reason": ""},
                        ],
                    }
                ],
                "state": [{"online": True}],
            }
        }
        self.after = copy.deepcopy(self.before)
        self.addition = {
            "name": "character.reverse_return",
            "supported": True,
            "reason": "",
            "modes": ["last_return", "named_location"],
        }

    def add(self, entry=None):
        capability = entry if entry is not None else self.addition
        self.after["frames"]["capabilities"][0]["commands"].append(capability)

    def test_new_capability_uses_existing_schema_without_protocol_bump(self):
        self.add()
        original = copy.deepcopy(self.after)
        self.assertFalse(protocol_contract_changed(self.before, self.after))
        self.assertEqual(self.after, original)

    def test_existing_capability_changes_and_removals_still_require_bump(self):
        self.after["frames"]["capabilities"][0]["commands"][0]["supported"] = False
        self.assertTrue(protocol_contract_changed(self.before, self.after))

        self.after = copy.deepcopy(self.before)
        self.after["frames"]["capabilities"][0]["commands"][0]["modes"] = ["changed"]
        self.assertTrue(protocol_contract_changed(self.before, self.after))

        self.after = copy.deepcopy(self.before)
        self.after["frames"]["capabilities"][0]["commands"] = []
        self.assertTrue(protocol_contract_changed(self.before, self.after))

    def test_duplicate_command_names_require_bump(self):
        self.add(dict(self.before["frames"]["capabilities"][0]["commands"][0]))
        self.assertTrue(protocol_contract_changed(self.before, self.after))

    def test_new_capability_cannot_add_fields_or_invalid_types(self):
        invalid_entries = (
            {"extra": True},
            {"supported": "true"},
            {"modes": "named_location"},
            {"modes": [1]},
            {"name": ""},
            {"name": "invalid-name"},
        )
        for override in invalid_entries:
            with self.subTest(override=override):
                self.after = copy.deepcopy(self.before)
                self.add({**self.addition, **override})
                self.assertTrue(protocol_contract_changed(self.before, self.after))

    def test_other_monitor_changes_and_missing_contracts_require_bump(self):
        self.add()
        self.after["frames"]["state"][0]["new_field"] = True
        self.assertTrue(protocol_contract_changed(self.before, self.after))
        self.assertTrue(protocol_contract_changed(self.before, {}))


if __name__ == "__main__":
    unittest.main()
