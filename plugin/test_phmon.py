import importlib.util
import json
import os
import socket
import struct
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


class MobObservationTests(unittest.TestCase):
    def test_current_monster_polling_runs_at_one_tenth_second_interval(self):
        previous_worker = plugin._worker
        previous_poll = plugin._last_monster_poll_at
        previous_cells = plugin._last_mob_cell_samples

        class WorkerStub:
            character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
            session_id = 'ffffffff-1111-4222-8333-444444444444'
            _current_identity = {'name': 'Alpha'}

            def __init__(self):
                self.snapshots = []

            def _identity_key(self, identity):
                return identity.get('name')

            def update_map_monsters(self, identity, status, region, monsters, sample=None):
                self.snapshots.append((status, region, monsters))
                return True

        worker = WorkerStub()
        identity = {'name': 'Alpha'}
        try:
            plugin._worker = worker
            plugin._last_monster_poll_at = float('-inf')
            plugin._last_mob_cell_samples = {}
            with patch.object(plugin, 'collect_monster_observation',
                              return_value=('observed', [], False)) as collect:
                plugin._sample_monsters(identity, {'region': 25273}, {'x': 10, 'y': 20}, now=20)
                plugin._sample_monsters(identity, {'region': 25273}, {'x': 10, 'y': 20}, now=20.099)
                plugin._sample_monsters(identity, {'region': 25273}, {'x': 10, 'y': 20}, now=20.1)

            self.assertEqual(plugin.MOB_POLL_INTERVAL_SECONDS, 0.1)
            self.assertEqual(collect.call_count, 2)
            self.assertEqual(len(worker.snapshots), 2)
        finally:
            plugin._worker = previous_worker
            plugin._last_monster_poll_at = previous_poll
            plugin._last_mob_cell_samples = previous_cells

    def test_monster_collector_distinguishes_missing_empty_and_truncated(self):
        self.assertEqual(plugin.collect_monster_observation({'get_monsters': lambda: None}),
                         ('unavailable', [], False))
        self.assertEqual(plugin.collect_monster_observation({'get_monsters': lambda: {}}),
                         ('observed', [], False))
        raw = {
            str(index): {'model': 300, 'type': 'Tiger', 'region': 25273,
                         'x': index, 'y': index + 1, 'z': 4}
            for index in range(plugin.MAX_MONSTERS_PER_SNAPSHOT + 1)
        }
        status, monsters, truncated = plugin.collect_monster_observation(
            {'get_monsters': lambda: raw})
        self.assertEqual(status, 'truncated')
        self.assertTrue(truncated)
        self.assertEqual(len(monsters), plugin.MAX_MONSTERS_PER_SNAPSHOT)
        self.assertEqual(monsters[0]['model_id'], 300)
        self.assertEqual(monsters[0]['type'], 'Tiger')

    def test_monster_collector_preserves_bounded_popup_and_hp_ring_fields(self):
        raw = {46296: {'model': 1933, 'type': 20, 'name': 'Eldimmu',
                       'servername': 'MOB_EU_ELDIMMU', 'region': 25735,
                       'x': 48.8, 'y': 1550.7, 'level': 72,
                       'hp': 7515, 'max_hp': 9000,
                       'attacking': 1}}
        status, monsters, truncated = plugin.collect_monster_observation(
            {'get_monsters': lambda: raw})
        self.assertEqual((status, truncated), ('observed', False))
        self.assertEqual(monsters[0]['type_code'], 20)
        self.assertEqual(monsters[0]['name'], 'Eldimmu')
        self.assertEqual(monsters[0]['servername'], 'MOB_EU_ELDIMMU')
        self.assertEqual(monsters[0]['level'], 72)
        self.assertEqual(monsters[0]['hp'], 7515)
        self.assertEqual(monsters[0]['max_hp'], 9000)
        self.assertTrue(monsters[0]['attacking'])

    def test_monster_collector_bounds_text_fields_by_utf8_bytes(self):
        raw = {'1': {'model': 1933, 'type': '界' * 64, 'name': '虎' * 128,
                     'servername': '龍' * 128, 'region': 25735,
                     'x': 48.8, 'y': 1550.7}}
        _, monsters, _ = plugin.collect_monster_observation(
            {'get_monsters': lambda: raw})
        self.assertLessEqual(len(monsters[0]['type'].encode('utf-8')), 64)
        self.assertLessEqual(len(monsters[0]['name'].encode('utf-8')), 128)
        self.assertLessEqual(len(monsters[0]['servername'].encode('utf-8')), 128)

    def test_monster_collector_ignores_invalid_or_undocumented_level_values(self):
        for value in (0, 256, True, '72'):
            raw = {'1': {'model': 1933, 'region': 25735, 'x': 1, 'y': 2,
                         'level': value}}
            _, monsters, _ = plugin.collect_monster_observation(
                {'get_monsters': lambda raw=raw: raw})
            self.assertNotIn('level', monsters[0])

    def test_spool_reloads_pending_sample_and_removes_only_after_ack(self):
        sample = {
            'sample_id': '00000000-0000-4000-8000-000000000001',
            'character_id': 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee',
            'session_id': 'ffffffff-1111-4222-8333-444444444444',
            'area_id': 'region:25273', 'floor_id': 'unmapped', 'region': 25273,
            'sampled_at': '2026-09-29T00:00:00Z',
            'observer': {'x': 192, 'y': -1, 'z': 0}, 'monsters': [],
        }
        with tempfile.TemporaryDirectory() as directory:
            path = os.path.join(directory, 'mob-samples.json')
            spool = plugin.MobObservationSpool(path)
            self.assertTrue(spool.add(sample))
            recovered = plugin.MobObservationSpool(path)
            self.assertEqual(recovered.pending(), [sample])
            self.assertTrue(recovered.acknowledge(sample['sample_id']))
            self.assertEqual(plugin.MobObservationSpool(path).pending(), [])

    def test_stationary_sampling_throttles_per_session_cell_and_keeps_empty_samples(self):
        previous_worker = plugin._worker
        previous_poll = plugin._last_monster_poll_at
        previous_cells = plugin._last_mob_cell_samples

        class WorkerStub:
            character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
            session_id = 'ffffffff-1111-4222-8333-444444444444'
            _current_identity = {'name': 'Alpha'}

            def __init__(self):
                self.samples = []

            def _identity_key(self, identity):
                return identity.get('name')

            def update_map_monsters(self, identity, status, region, monsters, sample=None):
                if sample is not None:
                    self.samples.append(sample)
                return True

        worker = WorkerStub()
        identity = {'name': 'Alpha'}
        try:
            plugin._worker = worker
            plugin._last_monster_poll_at = float('-inf')
            plugin._last_mob_cell_samples = {}
            with patch.object(plugin, 'collect_monster_observation', return_value=('observed', [], False)):
                plugin._sample_monsters(identity, {'region': 25273}, {'x': 10, 'y': 20}, now=20)
                plugin._sample_monsters(identity, {'region': 25273}, {'x': 10, 'y': 20}, now=35)
                plugin._sample_monsters(identity, {'region': 25273}, {'x': 10, 'y': 20}, now=80)
            self.assertEqual(len(worker.samples), 2)
            self.assertEqual(worker.samples[0]['monsters'], [])
            self.assertEqual(worker.samples[0]['area_id'], 'region:25273')
            self.assertEqual(worker.samples[0]['floor_id'], 'unmapped')
            self.assertEqual(worker.samples[0]['observer'], {'x': 10.0, 'y': 20.0})
        finally:
            plugin._worker = previous_worker
            plugin._last_monster_poll_at = previous_poll
            plugin._last_mob_cell_samples = previous_cells

    def test_worker_sends_scoped_current_snapshot_and_durable_sample_then_acks(self):
        with tempfile.TemporaryDirectory() as directory:
            worker = plugin.AgentWorker(
                {'backend_url': 'ws://127.0.0.1:1', 'agent_id': AGENT_ID,
                 'agent_token': 'fixture-token',
                 'mob_spool_path': os.path.join(directory, 'mob-samples.json')},
                'fixture',
            )
            identity = {'server': 'greatest', 'name': 'Alpha', 'guild': ''}
            worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
            worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
            worker._current_identity = identity
            sample = {
                'sample_id': '00000000-0000-4000-8000-000000000001',
                'character_id': worker.character_id, 'session_id': worker.session_id,
                'area_id': 'region:25273', 'floor_id': 'unmapped', 'region': 25273,
                'sampled_at': plugin._utc_now(), 'observer': {'x': 10, 'y': 20},
                'monsters': [],
            }
            self.assertTrue(worker.update_map_monsters(identity, 'observed', 25273, [], sample))

            class Client:
                def __init__(self):
                    self.sent = []

                def send_json(self, message):
                    self.sent.append(message)

            client = Client()
            worker._flush_map_observations(client)
            self.assertEqual([message['type'] for message in client.sent],
                             ['map.monsters', 'mob.sample'])
            self.assertEqual(client.sent[0]['map_snapshot']['status'], 'observed')
            self.assertEqual(client.sent[0]['map_snapshot']['monsters'], [])
            self.assertEqual(client.sent[1]['sample']['floor_id'], 'unmapped')
            self.assertEqual(len(worker._mob_spool.pending()), 1)
            worker._handle_server_message({
                'type': 'mob.sample.ack', 'protocol_version': plugin.PROTOCOL_VERSION,
                'sample_id': sample['sample_id'], 'status': 'persisted',
            })
            self.assertEqual(worker._mob_spool.pending(), [])


class ConfigTests(unittest.TestCase):
    def test_profile_path_is_unavailable_until_phbot_reports_login(self):
        with patch.object(plugin, '_get_profile', return_value=None):
            self.assertIsNone(plugin._current_settings_path())

    def test_connected_callback_only_queues_lifecycle_without_profile_io(self):
        previous = (plugin._worker, plugin._phbot_connected_state, plugin._character_joined)
        plugin._worker = None
        try:
            with patch.object(plugin, '_load_active_profile') as load_profile:
                plugin.connected()
            load_profile.assert_not_called()
            identity, event = plugin._pending_callback_events.get_nowait()
            self.assertIsNone(identity)
            self.assertEqual(event['kind'], 'session.connected')
        finally:
            plugin._worker, plugin._phbot_connected_state, plugin._character_joined = previous
            while True:
                try:
                    plugin._pending_callback_events.get_nowait()
                except plugin._queue.Empty:
                    break

    def test_repeated_connected_callback_keeps_joined_character_sampling(self):
        previous = (plugin._worker, plugin._phbot_connected_state, plugin._character_joined)
        try:
            plugin._worker = None
            plugin._phbot_connected_state = True
            plugin._character_joined = True
            plugin.connected()
            self.assertTrue(plugin._character_joined)
        finally:
            plugin._worker, plugin._phbot_connected_state, plugin._character_joined = previous

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

    def test_death_spool_path_is_scoped_to_profile_settings(self):
        config_dir = os.path.join('C:', 'phBot', 'Config')
        first_profile = os.path.join(config_dir, 'PhMon', 'Alice.Farm.cfg')
        second_profile = os.path.join(config_dir, 'PhMon', 'Bob.Farm.cfg')
        first = plugin._death_spool_path(config_dir, AGENT_ID, first_profile)
        self.assertEqual(first, plugin._death_spool_path(config_dir, AGENT_ID, first_profile))
        self.assertNotEqual(first, plugin._death_spool_path(config_dir, AGENT_ID, second_profile))
        self.assertIn('death-events-' + AGENT_ID + '-', first)

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


class ResourceCollectorTests(unittest.TestCase):
    def test_item_evidence_is_bounded_lossless_and_not_guessed(self):
        result = plugin._normalize_item({'model': 1, 'variance': 2**64-1,
            'magic_options': [{'id': 3, 'value': 5}], 'stats': {'unknown': 12},
            'credential': 'must not leave process'})
        self.assertEqual(result['api_fields']['variance'], '18446744073709551615')
        self.assertNotIn('credential', result['api_fields'])
        self.assertNotIn('phy_def_pwr', result)
        self.assertEqual(len(plugin._bounded_item_evidence(list(range(100)))), 32)

    def test_integer_option_and_attribute_keys_survive_json_without_collisions(self):
        raw = {'model': 71, 'blues': {9: 3, '9': 5, 0: 0, 2**64-1: 2**64-1},
               'white_stats': {0: 9, 9: 12}, 'unknown_item_field': 'not exported',
               'credential': 'must not leave process'}
        item = json.loads(json.dumps(plugin._normalize_item(raw)))
        self.assertEqual(item['api_fields']['blues']['mapping_entries'], [
            {'key_type': 'integer', 'key': '9', 'value': 3},
            {'key_type': 'string', 'key': '9', 'value': 5},
            {'key_type': 'integer', 'key': '0', 'value': 0},
            {'key_type': 'integer', 'key': '18446744073709551615',
             'value': '18446744073709551615'},
        ])
        self.assertEqual(item['api_fields']['white_stats']['mapping_entries'][0],
                         {'key_type': 'integer', 'key': '0', 'value': 9})
        self.assertEqual(item['api_field_types']['blues'],
                         {'type': 'dict', 'count': 4, 'key_types': ['integer', 'string']})
        self.assertEqual(item['api_field_types']['unknown_item_field']['type'], 'string')
        self.assertNotIn('unknown_item_field', item['api_fields'])
        self.assertNotIn('credential', item['api_field_types'])
        self.assertNotIn('not exported', json.dumps(item))
        self.assertEqual(raw['blues'][9], 3, 'source observations stay unchanged')
        tracker = plugin.PassiveItemTracker()
        tracker.decorate({'inventory': {'availability': 'observed', 'slots': [
            {'source_slot': 13, 'item': item}]}}, 'session')
        self.assertNotIn('instance', item, 'unverified API evidence is not trusted instance data')

    def test_api_evidence_distinguishes_missing_empty_zero_and_bounds_maps(self):
        missing = plugin._normalize_item({'model': 71})
        empty = plugin._normalize_item({'model': 71, 'blues': {}, 'attributes': None})
        zero = plugin._normalize_item({'model': 71, 'blues': {9: 0}})
        self.assertNotIn('api_fields', missing)
        self.assertEqual(empty['api_fields']['blues'], {})
        self.assertEqual(empty['api_field_types']['attributes']['type'], 'null')
        self.assertEqual(zero['api_fields']['blues']['mapping_entries'][0]['value'], 0)
        bounded = plugin._bounded_item_evidence({key: key for key in range(100)})
        self.assertEqual(len(bounded['mapping_entries']), 32)
        fields = plugin._item_api_field_types({'field_'+str(i): i for i in range(100)})
        self.assertLessEqual(len(fields), 64)
        self.assertLessEqual(len(json.dumps(fields, separators=(',', ':')).encode('utf-8')), 2048)

    def test_observed_runtime_scalar_fields_are_retained_without_interpretation(self):
        raw = {'model': 4247, 'max_durability': 77, 'phys_def': 55,
               'mag_def': 72, 'parry': 23, 'phys_reinf_min': 13.872,
               'phys_reinf_max': 13.872, 'mag_absorb_min': 16.322,
               'expiration': 12345}
        result = plugin._normalize_item(raw)
        for key in ('max_durability', 'phys_def', 'mag_def', 'parry',
                    'phys_reinf_min', 'phys_reinf_max', 'mag_absorb_min'):
            self.assertEqual(result['api_fields'][key], raw[key])
        self.assertNotIn('expiration', result['api_fields'])
        self.assertNotIn('phys_def', result)

    def test_collect_resources_preserves_slots_and_separates_equipment(self):
        api = {
            'get_inventory': lambda: {
                'size': 16,
                'gold': 9876543210123,
                'items': [{'model': 7, 'name': 'Sword', 'servername': 'ITEM_SWORD', 'quantity': 1, 'plus': 5, 'durability': 12}] + [None] * 12 + [None, {'model': 2, 'quantity': 200}, None],
            },
            'get_storage': lambda: {'size': 4, 'items': [None] * 4},
            'get_guild_storage': lambda: None,
            'get_job_pouch': lambda: {'size': 1, 'items': [None]},
            'get_pets': lambda: {123: {'name': 'Wolf', 'type': 'wolf', 'items': [None, {'model': 10, 'quantity': 2}]}},
            'get_party': lambda: {55: {'name': 'Ally', 'hp_percent': 8, 'mp_percent': 10, 'player_id': 0}},
            'get_academy': lambda: {
                'id': 237,
                6699: {'online': 1, 'type': 0, 'x': 1.5, 'y': 2.5, 'level': 110, 'name': 'AcademyMember'},
            },
        }
        result = plugin.collect_resources(api=api)
        self.assertEqual(result['equipment']['slots'][0]['item']['model'], 7)
        self.assertEqual(result['inventory']['capacity'], 3)
        self.assertIsNone(result['inventory']['slots'][0])
        self.assertEqual(result['inventory']['slots'][1]['source_slot'], 14)
        self.assertEqual(result['inventory']['slots'][1]['displayed_slot'], 1)
        self.assertEqual(result['inventory']['gold'], 9876543210123)
        self.assertEqual(result['storage']['availability'], 'observed')
        self.assertEqual(result['storage']['used_slots'], 0)
        self.assertEqual(result['guild_storage']['availability'], 'not_observed')
        self.assertEqual(result['pets']['pets'][0]['pet_id'], '123')
        self.assertEqual(result['pets']['pets'][0]['slots'][1]['item']['quantity'], 2)
        self.assertEqual(result['party']['members'][0]['hp_percent'], 80)
        self.assertEqual(result['party']['members'][0]['mp_percent'], 100)
        self.assertEqual(result['academy'], {
            'availability': 'observed',
            'value': {
                'id': 237,
                'members': [{
                    'member_id': '6699', 'name': 'AcademyMember', 'online': 1,
                    'type': 0, 'level': 110, 'x': 1.5, 'y': 2.5,
                }],
            },
        })
        self.assertEqual(result['party_setup']['mode'], 'read_only_unverified')

    def test_guild_storage_preserves_reported_gold(self):
        result = plugin.collect_resources(api={
            'get_guild_storage': lambda: {
                'size': 2,
                'gold': 4567890123,
                'items': [None, {'model': 12, 'quantity': 3}],
            },
        })
        self.assertEqual(result['guild_storage']['availability'], 'observed')
        self.assertEqual(result['guild_storage']['gold'], 4567890123)
        self.assertEqual(result['guild_storage']['slots'][1]['item']['model'], 12)

    def test_callback_collection_keeps_raw_values_for_worker_normalization(self):
        inventory = {'size': 0, 'gold': 17, 'items': []}
        inputs = plugin.collect_resource_inputs({'get_inventory': lambda: inventory})
        self.assertIs(inputs['get_inventory']['value'], inventory)
        self.assertTrue(inputs['get_inventory']['available'])
        result = plugin.normalize_resource_inputs(inputs)
        self.assertEqual(result['inventory']['availability'], 'observed')
        self.assertEqual(result['inventory']['capacity'], 0)

    def test_missing_apis_and_unopened_storage_are_not_empty_observations(self):
        result = plugin.collect_resources(api={})
        self.assertEqual(result['inventory']['availability'], 'unavailable')
        self.assertEqual(result['storage']['availability'], 'unavailable')
        self.assertEqual(result['party']['availability'], 'unavailable')

    def test_protocol_detection_ignores_numeric_server_version(self):
        root = tempfile.mkdtemp()
        try:
            path = os.path.join(root, 'vSRO.json')
            with open(path, 'w') as stream:
                import json
                json.dump({'servers': [{'name': 'Greatest', 'version': 296, 'v1.065': False, 'v1.274': False, 'vsro_193': False}]}, stream)
            self.assertEqual(plugin.detect_item_protocol(root, 'Greatest', 22), 'vsro-1.188')
            self.assertEqual(plugin.detect_item_protocol(root, 'Unknown', 22), 'unknown')
            self.assertEqual(plugin.detect_item_protocol(root, 'Greatest', 18), 'unknown')
        finally:
            os.unlink(path)
            os.rmdir(root)

    def test_protocol_detection_rejects_malformed_conflicting_and_ambiguous_config(self):
        root = tempfile.mkdtemp()
        path = os.path.join(root, 'vSRO.json')
        try:
            entry = {'name': 'Greatest', 'v1.065': False, 'v1.274': False, 'vsro_193': False}
            with open(path, 'w') as stream:
                json.dump({'servers': [entry]}, stream)
            self.assertEqual(plugin.detect_item_protocol(root, 'Greatest', 22), 'vsro-1.188')
            entry['v1.274'] = 'false'
            with open(path, 'w') as stream:
                json.dump({'servers': [entry]}, stream)
            self.assertEqual(plugin.detect_item_protocol(root, 'Greatest', 22), 'unknown')
            entry['v1.274'] = True
            entry['protocol_variant'] = '1.188'
            with open(path, 'w') as stream:
                json.dump({'servers': [entry]}, stream)
            self.assertEqual(plugin.detect_item_protocol(root, 'Greatest', 22), 'unknown')
            with open(path, 'w') as stream:
                json.dump({'servers': [{'name': 'Greatest', 'protocol': 'vsro-1.188'}]}, stream)
            self.assertEqual(plugin.detect_item_protocol(root, 'Greatest', 22), 'vsro-1.188')
            with open(path, 'w') as stream:
                json.dump({'servers': [{'name': 'Greatest', 'protocol': '1.188'},
                                      {'name': 'Greatest', 'protocol': '1.188'}]}, stream)
            self.assertEqual(plugin.detect_item_protocol(root, 'Greatest', 22), 'unknown')
        finally:
            os.unlink(path)
            os.rmdir(root)

    def test_protocol_detection_reads_phbot_profile_next_to_config_directory(self):
        root = tempfile.mkdtemp()
        config_dir = os.path.join(root, 'Config')
        os.mkdir(config_dir)
        path = os.path.join(root, 'vSRO.json')
        try:
            with open(path, 'w') as stream:
                json.dump({
                    'GreatestSRO': {
                        'servers': ['Greatest'],
                        'version': 296,
                        'v1.065': False,
                        'v1.274': False,
                        'vsro_193': False,
                    },
                }, stream)

            self.assertEqual(
                plugin.detect_item_protocol_detail(config_dir, 'Greatest', 22),
                ('vsro-1.188', 'v1.188_selected_by_phbot_flags'),
            )
            self.assertEqual(
                plugin.detect_item_protocol_detail(config_dir, 'Other', 22),
                ('unknown', 'server_not_in_vsro_config'),
            )
        finally:
            os.unlink(path)
            os.rmdir(config_dir)
            os.rmdir(root)


class PassiveItemPacketTests(unittest.TestCase):
    @staticmethod
    def stats_packet(slot=13, flags=0x35, model=71, plus=5,
                     variance=2**64 - 1, durability=57, options=None):
        options = options if options is not None else [(9, 3), (9, 3)]
        payload = bytearray([slot, flags])
        if flags & 0x01:
            payload.extend(struct.pack('<I', model))
        if flags & 0x02:
            payload.extend(struct.pack('<B', plus))
        if flags & 0x04:
            payload.extend(struct.pack('<Q', variance))
        if flags & 0x08:
            payload.extend(struct.pack('<H', 200))
        if flags & 0x10:
            payload.extend(struct.pack('<I', durability))
        if flags & 0x40:
            payload.extend(struct.pack('<B', 0))
        if flags & 0x20:
            payload.append(len(options))
            for option_id, value in options:
                payload.extend(struct.pack('<II', option_id, value))
        return bytes(payload)

    def test_item_stats_reader_preserves_uint64_and_duplicate_options(self):
        packet = self.stats_packet(flags=0x3F)
        parsed = plugin.parse_item_stats_update(packet)
        self.assertEqual(parsed['source_slot'], 13)
        self.assertEqual(parsed['fields']['model'], 71)
        self.assertEqual(parsed['fields']['plus'], 5)
        self.assertEqual(parsed['fields']['variance'], '18446744073709551615')
        self.assertEqual(parsed['fields']['durability'], 57)
        self.assertEqual(parsed['fields']['magic_options'], [
            {'id': '9', 'value': '3'}, {'id': '9', 'value': '3'}])

    def test_zero_magic_options_are_observed_empty_and_durability_packet_is_exact(self):
        parsed = plugin.parse_item_stats_update(self.stats_packet(flags=0x21, options=[]))
        self.assertEqual(parsed['fields']['magic_options'], [])
        self.assertTrue(parsed['fields']['magic_options_available'])
        self.assertEqual(plugin.parse_item_durability_update(struct.pack('<BI', 2, 0)), {
            'source_slot': 2, 'fields': {'durability': 0}})
        with self.assertRaises(plugin.ItemPacketError):
            plugin.parse_item_durability_update(struct.pack('<BI', 2, 0) + b'\x00')

    def test_truncated_unknown_and_trailing_item_packets_fail_closed(self):
        for packet in (b'\x01', b'\x01\x80', b'\x01\x02',
                       self.stats_packet(flags=0x02) + b'\x00'):
            with self.subTest(packet=packet):
                with self.assertRaises(plugin.ItemPacketError):
                    plugin.parse_item_stats_update(packet)
        with self.assertRaises(plugin.ItemPacketError):
            plugin.parse_item_stats_update(bytes([0, 0x20, 33]))

    def test_instance_is_session_and_model_bound_and_inventory_operation_invalidates(self):
        tracker = plugin.PassiveItemTracker()
        tracker.set_protocol('vsro-1.188', 'v1.188_selected_by_phbot_flags')
        tracker.enqueue(0x3040, self.stats_packet())
        resources = {'equipment': {'availability': 'observed', 'slots': []},
                     'inventory': {'availability': 'observed', 'slots': [
                         {'source_slot': 13, 'item': {'model': 71, 'plus': 5}}]}}
        tracker.decorate(resources, 'session-one')
        observed = resources['inventory']['slots'][0]['item']['instance']
        self.assertEqual(observed['source'], 'vsro_1188_packet')
        self.assertEqual(observed['magic_options_availability'], 'observed')
        self.assertTrue(observed['observation_id'].startswith('session-one:'))

        # RefObjID begins a new lifetime, including a replacement with the
        # same model and enhancement. Old variance/blues must not survive it.
        tracker.enqueue(0x3040, self.stats_packet(flags=0x01, model=71))
        same_model_replacement = {'equipment': {'availability': 'observed', 'slots': []},
                                  'inventory': {'availability': 'observed', 'slots': [
                                      {'source_slot': 13, 'item': {'model': 71, 'plus': 5}}]}}
        tracker.decorate(same_model_replacement, 'session-one')
        self.assertNotIn('instance', same_model_replacement['inventory']['slots'][0]['item'])

        # Any inventory operation is unclassified in this decoder, so cached
        # instance fields cannot migrate to a same-model replacement in the slot.
        tracker.enqueue(0xB034, b'\x00')
        tracker.enqueue(0x3040, self.stats_packet(flags=0x04, variance=5))
        replacement = {'equipment': {'availability': 'observed', 'slots': []},
                       'inventory': {'availability': 'observed', 'slots': [
                           {'source_slot': 13, 'item': {'model': 71, 'plus': 5}}]}}
        tracker.decorate(replacement, 'session-one')
        self.assertNotIn('instance', replacement['inventory']['slots'][0]['item'])

        tracker.enqueue(0x3040, self.stats_packet(flags=0x05, variance=5))
        tracker.decorate(replacement, 'session-one')
        self.assertEqual(replacement['inventory']['slots'][0]['item']['instance']['variance'], '5')

        changed_model = {'equipment': {'availability': 'observed', 'slots': []},
                         'inventory': {'availability': 'observed', 'slots': [
                             {'source_slot': 13, 'item': {'model': 72, 'plus': 5}}]}}
        tracker.decorate(changed_model, 'session-one')
        self.assertNotIn('instance', changed_model['inventory']['slots'][0]['item'])

    def test_parser_build_is_reported_before_any_item_packet_is_observed(self):
        tracker = plugin.PassiveItemTracker()
        tracker.set_protocol('vsro-1.188', 'v1.188_selected_by_phbot_flags')
        resources = {
            'equipment': {'availability': 'observed', 'slots': []},
            'inventory': {'availability': 'observed', 'slots': []},
        }

        tracker.decorate(resources, 'session-one')

        self.assertEqual(resources['item_enrichment']['availability'], 'not_observed')
        self.assertEqual(resources['item_enrichment']['protocol'], 'vsro-1.188')
        self.assertEqual(resources['item_enrichment']['protocol_reason'], 'v1.188_selected_by_phbot_flags')
        self.assertEqual(resources['item_enrichment']['source'], 'vsro_1188_packet')
        self.assertEqual(resources['item_enrichment']['decoder_build'], 'vsro_1188_passive_r2')
        self.assertEqual(resources['item_enrichment']['observed_items'], 0)

    def test_queue_overflow_invalidates_all_item_instances(self):
        tracker = plugin.PassiveItemTracker()
        tracker.set_protocol('vsro-1.188')
        tracker.enqueue(0x3040, self.stats_packet())
        resources = {'equipment': {'availability': 'observed', 'slots': []},
                     'inventory': {'availability': 'observed', 'slots': [
                         {'source_slot': 13, 'item': {'model': 71, 'plus': 5}}]}}
        tracker.decorate(resources, 'session-one')
        self.assertIn('instance', resources['inventory']['slots'][0]['item'])
        for _ in range(plugin.MAX_ITEM_PACKET_COUNT + 1):
            tracker.enqueue(0xB034, b'\x00')
        tracker.decorate(resources, 'session-one')
        self.assertNotIn('instance', resources['inventory']['slots'][0]['item'])
        self.assertEqual(resources['item_enrichment']['reason'], 'packet_queue_overflow')

    def test_protocol_change_and_api_empty_slot_remove_enrichment(self):
        tracker = plugin.PassiveItemTracker()
        tracker.set_protocol('vsro-1.188')
        tracker.enqueue(0x3040, self.stats_packet())
        resources = {'equipment': {'availability': 'observed', 'slots': []},
                     'inventory': {'availability': 'observed', 'slots': [
                         {'source_slot': 13, 'item': {'model': 71, 'plus': 5}}]}}
        tracker.decorate(resources, 'session-one')
        empty = {'equipment': {'availability': 'observed', 'slots': []},
                 'inventory': {'availability': 'observed', 'slots': [
                     {'source_slot': 13, 'item': None}]}}
        tracker.decorate(empty, 'session-one')
        resources['inventory']['slots'][0]['item'] = None
        tracker.decorate(resources, 'session-one')
        self.assertIsNone(resources['inventory']['slots'][0]['item'])
        tracker.set_protocol('unknown')
        self.assertEqual(tracker._states, {})

    def test_diagnostic_capture_persists_only_bounded_sanitized_item_fields(self):
        tracker = plugin.PassiveItemTracker(capture_enabled=True)
        tracker.set_protocol('vsro-1.188')
        tracker.enqueue(0x3040, self.stats_packet())
        tracker.decorate({'equipment': {'availability': 'observed', 'slots': []},
                          'inventory': {'availability': 'observed', 'slots': []}}, 'session-one')
        root = tempfile.mkdtemp()
        path = os.path.join(root, 'item-capture.json')
        try:
            self.assertTrue(tracker.persist_sanitized_capture(path))
            with open(path, 'r') as stream:
                saved = json.load(stream)
            with open(path, 'r') as stream:
                contents = stream.read()
            self.assertEqual(len(saved['records']), 1)
            self.assertEqual(saved['records'][0]['fields']['variance'], '18446744073709551615')
            self.assertNotIn('session-one', contents)
            self.assertLessEqual(os.path.getsize(path), plugin.MAX_ITEM_CAPTURE_BYTES)
        finally:
            os.unlink(path)
            os.rmdir(root)


class ResourceTransportTests(unittest.TestCase):
    def setUp(self):
        self.worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_test_token',
        }, 'fixture')
        self.worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        self.worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        self.frames = []
        self.client = type('Client', (), {'send_json': lambda _, frame: self.frames.append(frame)})()


    def test_resource_baseline_then_revision_checked_delta(self):
        original = {
            'inventory': {'availability': 'observed', 'slots': []},
            'pets': {'availability': 'observed', 'pets': []},
        }
        self.assertTrue(self.worker._send_resource_snapshot(self.client, original))
        self.assertTrue(self.frames[0]['full'])
        self.assertEqual(self.frames[0]['revision'], 1)
        self.assertEqual(self.frames[0]['chunk_count'], 1)
        self.assertTrue(self.worker._send_resource_snapshot(self.client, original))
        self.assertEqual(len(self.frames), 1, 'unchanged observations should not create a revision')

        updated = dict(original)
        updated['inventory'] = {'availability': 'observed', 'slots': [None]}
        self.assertTrue(self.worker._send_resource_snapshot(self.client, updated))
        delta = self.frames[1]
        self.assertFalse(delta['full'])
        self.assertEqual(delta['type'], 'resource.delta')
        self.assertEqual(delta['revision'], 2)
        self.assertEqual(delta['base_revision'], 1)
        self.assertEqual(list(delta['resources']), ['inventory'])

    def test_nested_in_place_item_mutation_emits_resource_delta(self):
        current = {
            'inventory': {
                'availability': 'observed',
                'slots': [{'item': {'model': 100, 'api_fields': {'blues': {'7': 1}}}}],
            },
        }
        self.assertTrue(self.worker._send_resource_snapshot(self.client, current))
        current['inventory']['slots'][0]['item']['api_fields']['blues']['7'] = 2
        self.assertTrue(self.worker._send_resource_snapshot(self.client, current))
        self.assertEqual(len(self.frames), 2)
        self.assertEqual(self.frames[1]['type'], 'resource.delta')
        self.assertEqual(list(self.frames[1]['resources']), ['inventory'])
        self.assertEqual(
            self.frames[1]['resources']['inventory']['slots'][0]['item']['api_fields']['blues']['7'],
            2,
        )

    def test_large_baseline_is_split_into_bounded_atomic_chunks(self):
        value = {
            'inventory': {'availability': 'observed', 'sample': 'a' * 142000},
            'storage': {'availability': 'observed', 'sample': 'b' * 142000},
        }
        self.assertTrue(self.worker._send_resource_snapshot(self.client, value))
        self.assertEqual(len(self.frames), 2)
        self.assertEqual({frame['chunk_count'] for frame in self.frames}, {2})
        self.assertEqual({frame['chunk_index'] for frame in self.frames}, {0, 1})
        self.assertTrue(all(len(plugin.json.dumps(frame, separators=(',', ':')).encode('utf-8')) <= plugin.MAX_MESSAGE_BYTES for frame in self.frames))

    def test_resync_forces_a_new_full_baseline(self):
        current = {'inventory': {'availability': 'observed', 'slots': []}}
        self.worker._send_resource_snapshot(self.client, current)
        self.worker._latest_resources = current
        self.worker._latest_resources_identity = self.worker._identity_key({'server': 'S', 'name': 'C'})
        self.worker._current_identity = {'server': 'S', 'name': 'C'}
        self.worker._handle_server_message({
            'type': 'resource.resync',
            'protocol_version': plugin.PROTOCOL_VERSION,
            'character_id': self.worker.character_id,
            'session_id': self.worker.session_id,
        })
        self.worker._send_resource_snapshot(self.client, current)
        self.assertTrue(self.frames[-1]['full'])
        self.assertEqual(self.frames[-1]['revision'], 2)

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


class DeathEventTransportTests(unittest.TestCase):
    def setUp(self):
        self.root = tempfile.mkdtemp()
        self.spool_path = os.path.join(self.root, 'deaths.json')
        self.identity = {'server': 'Silkroad', 'name': 'Alpha'}
        self.worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_test_token',
            'death_spool_path': self.spool_path,
        }, 'fixture')
        self.worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        self.worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        self.worker._current_identity = self.identity
        self.frames = []
        self.client = type('Client', (), {'send_json': lambda _, frame: self.frames.append(frame)})()

    def tearDown(self):
        for path in (
            self.spool_path,
            self.spool_path + '.tmp',
            self.spool_path + '.capacity',
            self.spool_path + '.capacity.tmp',
        ):
            if os.path.exists(path):
                os.unlink(path)
        os.rmdir(self.root)

    def death(self, event_id='00000000-0000-4000-8000-000000000001'):
        return {
            'event_id': event_id,
            'occurred_at': plugin._utc_now(),
            'source': 'phbot.callback',
            'source_ref': 'EVENT_DIED',
            'schema_version': 1,
            'kind': 'character.died',
            'category': 'character',
            'payload': {'cause': 'unknown'},
        }

    def test_event_spool_enforces_independent_item_and_byte_reserves(self):
        with patch.object(plugin, 'MAX_EVENT_CRITICAL_ITEMS', 1), \
                patch.object(plugin, 'MAX_EVENT_ORDINARY_ITEMS', 2):
            spool = plugin.EventSpool(self.spool_path)
            critical = self.death()
            self.assertTrue(spool.add(critical))
            second_critical = dict(critical, event_id='00000000-0000-4000-8000-000000000002')
            self.assertFalse(spool.add(second_critical))

            ordinary = {
                'event_id': '00000000-0000-4000-8000-000000000003',
                'occurred_at': plugin._utc_now(),
                'schema_version': 1,
                'kind': 'session.connected',
                'category': 'session',
                'source': 'phbot.lifecycle_callback',
                'source_ref': 'connected',
                'payload': {},
            }
            self.assertTrue(spool.add(ordinary))
            second_ordinary = dict(ordinary, event_id='00000000-0000-4000-8000-000000000004')
            self.assertTrue(spool.add(second_ordinary))
            third_ordinary = dict(ordinary, event_id='00000000-0000-4000-8000-000000000005')
            self.assertFalse(spool.add(third_ordinary))

        with patch.object(plugin, 'MAX_EVENT_CRITICAL_BYTES', 512), \
                patch.object(plugin, 'MAX_EVENT_ORDINARY_BYTES', 512):
            byte_spool = plugin.EventSpool(self.spool_path + '.capacity')
            large_critical = self.death('00000000-0000-4000-8000-000000000006')
            large_critical['payload'] = {'cause': 'x' * 700}
            self.assertFalse(byte_spool.add(large_critical))

            large_ordinary = dict(ordinary, event_id='00000000-0000-4000-8000-000000000007')
            large_ordinary['payload'] = {'detail': 'x' * 700}
            self.assertFalse(byte_spool.add(large_ordinary))

    def test_spool_write_failure_is_reported_and_event_is_not_sent(self):
        self.assertTrue(self.worker.queue_death_event(self.identity, self.death()))
        with patch.object(plugin.EventSpool, '_save_locked', side_effect=OSError('disk full')), \
                patch.object(plugin, '_log') as log:
            self.worker._flush_events(self.client)

        self.assertIn('spool is unavailable or full', self.worker.status)
        self.assertEqual(self.worker._death_spool.pending(), [])
        self.assertEqual(self.frames, [])
        self.assertTrue(any('durable local spooling failed' in call.args[0] for call in log.call_args_list))

    def test_worker_spools_queued_events_during_reconnect_backoff(self):
        event = self.death('00000000-0000-4000-8000-000000000009')
        worker = self.worker

        class DisconnectedClient:
            def connect(inner_self):
                worker.queue_event(self.identity, event)
                raise OSError('backend unavailable')

            def close(inner_self):
                return None

        worker.websocket_factory = lambda *_args: DisconnectedClient()
        worker.start()
        deadline = time.monotonic() + 2.0
        try:
            while not worker._death_spool.pending() and time.monotonic() < deadline:
                time.sleep(0.01)
            pending = worker._death_spool.pending()
            self.assertEqual(len(pending), 1)
            self.assertEqual(pending[0]['event_id'], event['event_id'])
        finally:
            worker.stop()
            worker.join(2.0)

    def test_spool_replays_same_stable_event_until_persisted_ack(self):
        event = self.death()
        self.assertTrue(self.worker.queue_death_event(self.identity, event))
        self.worker._flush_death_events(self.client)
        self.assertEqual(len(self.frames), 1)
        self.assertEqual(self.frames[0]['type'], 'event.batch')
        self.assertEqual(len(self.frames[0]['events']), 1)
        wire_event = self.frames[0]['events'][0]
        self.assertEqual(wire_event['event_id'], event['event_id'])
        self.assertEqual(wire_event['character_id'], self.worker.character_id)
        self.assertEqual(wire_event['session_id'], self.worker.session_id)
        self.assertEqual(wire_event['sequence'], 1)

        recovered = plugin.DeathEventSpool(self.spool_path)
        self.assertEqual(recovered.pending()[0]['event_id'], event['event_id'])
        retry_worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_test_token',
            'death_spool_path': self.spool_path,
        }, 'fixture')
        replay = []
        retry_worker._flush_death_events(type('Client', (), {'send_json': lambda _, frame: replay.append(frame)})())
        self.assertEqual(replay[0]['events'][0]['event_id'], event['event_id'])
        retry_worker._handle_server_message({
            'type': 'event.batch.ack', 'protocol_version': plugin.PROTOCOL_VERSION,
            'results': [{'event_id': event['event_id'], 'status': 'retry'}],
        })
        self.assertEqual(len(retry_worker._death_spool.pending()), 1)
        retry_worker._handle_server_message({
            'type': 'event.batch.ack', 'protocol_version': plugin.PROTOCOL_VERSION,
            'results': [{'event_id': event['event_id'], 'status': 'persisted'}],
        })
        self.assertEqual(retry_worker._death_spool.pending(), [])

    def test_dead_snapshot_does_not_create_an_occurrence_and_switch_is_fenced(self):
        self.worker.update_character(self.identity, {'dead': True})
        self.worker._flush_death_events(self.client)
        self.assertEqual(self.frames, [])

        other_identity = {'server': 'Silkroad', 'name': 'Beta'}
        self.assertFalse(self.worker.queue_death_event(other_identity, self.death()))
        self.assertEqual(self.worker._death_samples.qsize(), 0)

        self.worker._current_identity = other_identity
        self.worker.character_id = 'bbbbbbbb-cccc-4ddd-8eee-ffffffffffff'
        self.worker.session_id = 'eeeeeeee-ffff-4111-8222-333333333333'
        self.assertTrue(self.worker.queue_death_event(other_identity, self.death(
            '00000000-0000-4000-8000-000000000002')))
        self.worker._flush_death_events(self.client)
        sent = self.frames[-1]['events'][0]
        self.assertEqual(sent['character_id'], self.worker.character_id)
        self.assertEqual(sent['session_id'], self.worker.session_id)

    def test_disconnect_uses_last_registered_context_for_callback(self):
        self.worker._last_death_context = {
            'identity': dict(self.identity),
            'character_id': self.worker.character_id,
            'session_id': self.worker.session_id,
        }
        self.worker._current_identity = None
        self.worker.character_id = None
        self.worker.session_id = None
        self.assertTrue(self.worker.queue_death_event(self.identity, self.death()))
        self.worker._flush_death_events(self.client)
        self.assertEqual(len(self.frames), 1)
        self.assertEqual(self.frames[0]['events'][0]['character_id'], 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee')

    def test_deferred_callback_waits_for_matching_session_and_binds_before_send(self):
        other_identity = {'server': 'Silkroad', 'name': 'OfflineAlpha'}
        self.worker._current_identity = None
        self.worker.character_id = None
        self.worker.session_id = None
        event = self.death()
        self.assertTrue(self.worker.queue_death_event(other_identity, event))
        self.worker._flush_death_events(self.client)
        self.assertEqual(self.frames, [])
        pending = self.worker._death_spool.pending()[0]
        self.assertTrue(pending['deferred_session_binding'])
        self.assertIsNone(pending['character_id'])

        self.worker._current_identity = other_identity
        self.worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        self.worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        self.worker._flush_death_events(self.client)
        self.assertEqual(len(self.frames), 1)
        frame = self.frames[0]['events'][0]
        self.assertEqual(frame['character_id'], self.worker.character_id)
        self.assertEqual(frame['session_id'], self.worker.session_id)
        self.assertEqual(frame['payload'], {'cause': 'unknown'})
        persisted = self.worker._death_spool.pending()[0]
        self.assertEqual(persisted['character_id'], self.worker.character_id)
        self.assertEqual(persisted['session_id'], self.worker.session_id)
        self.assertFalse(persisted['deferred_session_binding'])
        self.assertNotIn('session_binding', persisted['payload'])

        original_session = frame['session_id']
        original_sequence = frame['sequence']
        self.worker.session_id = '11111111-2222-4333-8444-555555555555'
        self.worker._death_retry_at[frame['event_id']] = 0
        self.worker._flush_death_events(self.client)
        replay = self.frames[-1]['events'][0]
        self.assertEqual(replay['event_id'], frame['event_id'])
        self.assertEqual(replay['session_id'], original_session)
        self.assertEqual(replay['sequence'], original_sequence)

    def test_death_callback_is_deduplicated_until_alive_or_character_switch(self):
        previous_worker = plugin._worker
        previous_active = plugin._death_callback_active
        received = []
        fake_worker = type('Worker', (), {
            'session_id': None,
            'queue_event': lambda _, identity, event: received.append((identity, event)) or True,
        })()
        try:
            plugin._worker = fake_worker
            plugin._death_callback_active = False
            with patch.object(plugin, '_get_character_data', return_value={'server': 'Silkroad', 'name': 'Alpha'}), \
                    patch.object(plugin, '_get_position', return_value={'region': 25273, 'x': 1, 'y': 2, 'z': 3}), \
                    patch.object(plugin, '_load_active_profile'):
                plugin.handle_event(plugin.EVENT_DIED, '')
                plugin.handle_event(plugin.EVENT_DIED, '')
                deaths = [entry for entry in received if entry[1]['kind'] == 'character.died']
                self.assertEqual(len(deaths), 1)
                self.assertEqual(deaths[0][1]['source_ref'], 'EVENT_DIED')
                self.assertEqual(deaths[0][1]['payload']['cause'], 'unknown')
                plugin.joined_game()
                plugin.handle_event(plugin.EVENT_DIED, '')
                deaths = [entry for entry in received if entry[1]['kind'] == 'character.died']
                self.assertEqual(len(deaths), 2)
                self.assertNotEqual(deaths[0][1]['event_id'], deaths[1][1]['event_id'])
        finally:
            plugin._worker = previous_worker
            plugin._death_callback_active = previous_active


class ResourceEventDerivationTests(unittest.TestCase):
    def setUp(self):
        self.worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_test_token',
        }, 'fixture')
        self.identity = {'server': 'Silkroad', 'name': 'Alpha'}
        self.worker._current_identity = self.identity
        self.worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        self.worker.session_id = 'ffffffff-1111-4222-8333-444444444444'

    def item(self, quantity=1):
        return {'model': 77, 'servername': 'ITEM_ETC_TEST', 'name': 'Test Item', 'quantity': quantity}

    def resources(self, bag=None, pet=None, job=None, storage=None):
        def container(items):
            return {
                'availability': 'observed',
                'slots': [
                    {'source_slot': slot, 'item': dict(item)}
                    for slot, item in (items or [])
                ],
            }
        pets = []
        if pet is not None:
            pets.append({
                'pet_id': '1234', 'name': 'Pick Pet', 'type': 'pick',
                'inventory_available': True,
                'slots': [{'source_slot': 0, 'item': dict(pet)}],
            })
        return {
            'equipment': container([]),
            'inventory': container(bag),
            'pets': {'availability': 'observed', 'pets': pets},
            'party': {'availability': 'observed', 'members': []},
            'academy': {'availability': 'observed', 'value': {'members': []}},
            **({'job_pouch': container(job)} if job is not None else {}),
            **({'storage': container(storage)} if storage is not None else {}),
        }

    def observe(self, resources, now=None):
        self.worker._derive_resource_events(self.identity, resources, {
            'observed_at': now or plugin._utc_now(),
            'position': {'region': 25273, 'x': 10.0, 'y': 20.0, 'z': 0.0},
        })

    def drain(self):
        output = []
        while True:
            try:
                output.append(self.worker._event_samples.get_nowait())
            except plugin._queue.Empty:
                return output

    def test_baseline_and_bag_split_merge_do_not_create_item_occurrences(self):
        self.observe(self.resources(bag=[(13, self.item(4))]))
        self.observe(self.resources(bag=[(14, self.item(1)), (15, self.item(3))]))
        self.assertEqual(self.drain(), [])

    def test_bag_and_job_pouch_positive_quantity_deltas_keep_unknown_cause(self):
        self.observe(self.resources(bag=[] , job=[]))
        self.observe(self.resources(bag=[(13, self.item(5))], job=[(0, self.item(2))]))
        events = self.drain()
        self.assertEqual(len(events), 2)
        by_container = {event['payload']['destination_container']['type']: event for event in events}
        self.assertEqual(by_container['inventory']['kind'], 'item.acquired')
        self.assertEqual(by_container['inventory']['payload']['quantity_delta'], 5)
        self.assertEqual(by_container['inventory']['payload']['acquisition_method'], 'unknown')
        self.assertEqual(by_container['job_pouch']['payload']['quantity_delta'], 2)
        self.assertEqual(by_container['inventory']['region'], 25273)

    def test_opening_storage_establishes_baseline_and_later_gain_is_recorded(self):
        self.observe(self.resources(bag=[]))
        self.observe(self.resources(bag=[], storage=[(0, self.item(9))]))
        self.assertEqual(self.drain(), [])
        self.observe(self.resources(bag=[], storage=[(0, self.item(11))]))
        event = self.drain()[0]
        self.assertEqual(event['payload']['quantity_delta'], 2)
        self.assertEqual(event['payload']['destination_container']['type'], 'storage')
        self.assertEqual(event['payload']['acquisition_method'], 'unknown')

    def test_pet_summon_sets_inventory_baseline_then_pet_to_bag_is_transfer(self):
        self.observe(self.resources(bag=[]))
        self.observe(self.resources(bag=[], pet=self.item(3)))
        events = self.drain()
        self.assertEqual([event['kind'] for event in events], ['pet.summoned'])
        self.observe(self.resources(bag=[(13, self.item(3))], pet=None))
        # Dismissal changes the owned-container set; that sample is a new baseline.
        dismissed = self.drain()
        self.assertEqual([event['kind'] for event in dismissed], ['pet.dismissed'])
        self.assertNotIn('item.acquired', [event['kind'] for event in dismissed])

        self.observe(self.resources(bag=[], pet=self.item(3)))
        self.drain()
        self.observe(self.resources(bag=[(13, self.item(3))], pet=self.item(0)))
        transfer = self.drain()[0]
        self.assertEqual(transfer['kind'], 'item.transferred')
        self.assertEqual(transfer['payload']['source_container']['type'], 'pets')
        self.assertEqual(transfer['payload']['destination_container']['type'], 'inventory')
        self.assertEqual(transfer['payload']['quantity_delta'], 3)

    def test_sampling_gap_resets_baseline_after_quantity_change(self):
        self.observe(self.resources(bag=[(13, self.item(1))]))
        self.worker._resource_sample_gap = True
        self.observe(self.resources(bag=[(13, self.item(20))]))
        self.assertEqual(self.drain(), [])


class CanonicalCallbackTests(unittest.TestCase):
    def setUp(self):
        self.previous_worker = plugin._worker
        self.identity = {'server': 'Silkroad', 'name': 'Alpha'}
        self.root = tempfile.mkdtemp()
        self.spool_path = os.path.join(self.root, 'events.json')
        self.worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_test_token',
            'death_spool_path': self.spool_path,
        }, 'fixture')
        self.worker._current_identity = self.identity
        self.worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        self.worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        self.worker._latest_alchemy_items = {
            13: {'model': 77, 'servername': 'ITEM_TEST', 'name': 'Observed item', 'quantity': 1},
        }
        plugin._worker = self.worker

    def tearDown(self):
        plugin._worker = self.previous_worker
        for path in (self.spool_path, self.spool_path + '.tmp', self.spool_path + '.capacity', self.spool_path + '.capacity.tmp'):
            if os.path.exists(path):
                os.unlink(path)
        os.rmdir(self.root)

    def callback(self, fn, *args):
        with patch.object(plugin, '_get_character_data', return_value=self.identity), \
                patch.object(plugin, '_get_position', return_value={'region': 25273, 'x': 10, 'y': 20, 'z': 0}):
            fn(*args)
        return self.worker._event_samples.get_nowait()

    def test_handle_event_keeps_drop_kinds_separate_and_does_not_invent_instance_data(self):
        rare = self.callback(plugin.handle_event, plugin.EVENT_RARE_DROP, '77')
        normal = self.callback(plugin.handle_event, plugin.EVENT_ITEM_DROP, '78')
        self.assertEqual(rare['kind'], 'drop.rare')
        self.assertEqual(normal['kind'], 'drop.item')
        self.assertEqual(rare['item_model'], 77)
        self.assertNotIn('item', rare['payload'])
        self.assertEqual(normal['payload'], {'model': 78})

    def test_chat_and_alchemy_callbacks_use_canonical_queue_and_keep_raw_fields(self):
        chat = self.callback(plugin.handle_chat, 'party', None, 'hello' * 500)
        self.assertEqual(chat['kind'], 'chat.message_received')
        self.assertEqual(chat['payload']['channel'], 'party')
        self.assertEqual(chat['payload']['raw_type'], 'party')
        self.assertEqual(len(chat['payload']['message']), 2048)
        self.assertEqual(chat['sequence'], 1)

        alchemy = self.callback(plugin.alchemy_update, 13, True, 7)
        self.assertEqual(alchemy['kind'], 'alchemy.attempt')
        self.assertEqual(alchemy['payload']['success'], True)
        self.assertEqual(alchemy['payload']['plus'], 7)
        self.assertEqual(alchemy['payload']['item']['servername'], 'ITEM_TEST')
        self.assertEqual(alchemy['item_code'], 'ITEM_TEST')
        self.assertEqual(alchemy['sequence'], 2)

    def test_chat_callback_maps_verified_numeric_types_and_preserves_other_types(self):
        for raw_type, channel in ((1, 'general'), (2, 'private'), (4, 'party'), (5, 'guild'), (6, 'global')):
            with self.subTest(raw_type=raw_type):
                numeric = self.callback(plugin.handle_chat, raw_type, 'Beta', 'chat text')
                self.assertEqual(numeric['payload']['channel'], channel)
                self.assertEqual(numeric['payload']['raw_type'], str(raw_type))

        unknown = self.callback(plugin.handle_chat, 99, 'Beta', 'other text')
        explicit = self.callback(plugin.handle_chat, ' Private ', 'Beta', 'private text')
        self.assertEqual(unknown['payload']['channel'], 'unknown')
        self.assertEqual(unknown['payload']['raw_type'], '99')
        self.assertEqual(explicit['payload']['channel'], 'private')
        self.assertEqual(explicit['payload']['sender'], 'Beta')

    def test_chat_capability_and_dispatch_use_only_injected_documented_methods(self):
        calls = []
        adapter = plugin.PhBotAdapter({}, {
            'general': lambda text: calls.append(('all', text)) or True,
            'private': lambda name, text: calls.append(('private', name, text)) or False,
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        capabilities = {item['name']: item for item in worker._capability_frame()['commands']}
        self.assertEqual(capabilities['chat.send']['modes'], ['general', 'private'])
        self.assertTrue(capabilities['chat.send']['supported'])
        self.assertEqual(worker._invoke('chat.send', {'channel': 'general', 'text': 'hello'}, None)[:2],
                         (True, {'channel': 'general', 'text': 'hello'}))
        private = worker._invoke('chat.send', {'channel': 'private', 'text': 'hello', 'recipient': 'Beta'}, None)
        self.assertFalse(private[0])
        self.assertEqual(private[1]['recipient'], 'Beta')
        self.assertEqual(calls, [('all', 'hello'), ('private', 'Beta', 'hello')])
        with self.assertRaises(ValueError):
            worker._invoke('chat.send', {'channel': 'guild', 'text': 'hello'}, None)
        with self.assertRaises(ValueError):
            worker._invoke('chat.send', {'channel': 'private', 'text': 'hello'}, None)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        frame = {
            'type': 'command.execute', 'protocol_version': plugin.PROTOCOL_VERSION,
            'command_id': 'cmd_00000000-0000-4000-8000-000000000005',
            'character_id': AGENT_ID, 'session_id': worker.session_id,
            'name': 'chat.send', 'args': {'channel': 'general', 'text': 'callback dispatch'},
            'ttl_ms': 10000,
            'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time() + 10)),
        }
        worker._accept_command(frame)
        worker._outgoing.get_nowait()  # queued acknowledgement
        self.assertTrue(worker.process_one_command(worker._current_identity, None))
        result = worker._outgoing.get_nowait()
        self.assertEqual(result['status'], 'completed')
        self.assertEqual(result['verification'], 'api_confirmed')
        self.assertEqual(calls[-1], ('all', 'callback dispatch'))

    def test_alchemy_finished_is_a_distinct_completion_occurrence(self):
        event = self.callback(plugin.handle_event, plugin.EVENT_ALCHEMY_FINISHED, '')
        self.assertEqual(event['kind'], 'alchemy.finished')
        self.assertEqual(event['payload'], {})

    def test_handle_event_buffers_when_worker_is_not_ready(self):
        previous_worker = plugin._worker
        previous_pending = plugin._pending_callback_events
        plugin._worker = None
        plugin._pending_callback_events = plugin._queue.Queue()
        try:
            with patch.object(plugin, '_get_character_data', return_value=self.identity), \
                    patch.object(plugin, '_get_position', return_value={'region': 25273, 'x': 10, 'y': 20, 'z': 0}):
                plugin.handle_event(plugin.EVENT_LEVEL_UP, '42')
            identity, event = plugin._pending_callback_events.get_nowait()
            self.assertEqual(identity, self.identity)
            self.assertEqual(event['kind'], 'character.level_up')
            self.assertEqual(event['payload'], {'level': 42})
        finally:
            plugin._worker = previous_worker
            plugin._pending_callback_events = previous_pending

    def test_chat_and_alchemy_callbacks_buffer_when_worker_is_not_ready(self):
        previous_worker = plugin._worker
        previous_pending = plugin._pending_callback_events
        plugin._worker = None
        plugin._pending_callback_events = plugin._queue.Queue()
        try:
            with patch.object(plugin, '_get_character_data', return_value=self.identity), \
                    patch.object(plugin, '_get_position', return_value={'region': 25273, 'x': 10, 'y': 20, 'z': 0}):
                plugin.handle_chat('party', 'Beta', 'hello')
                plugin.alchemy_update(13, True, 7)
            buffered = [plugin._pending_callback_events.get_nowait() for _ in range(2)]
            self.assertEqual([event['kind'] for _, event in buffered], [
                'chat.message_received', 'alchemy.attempt',
            ])
            self.assertTrue(all(identity == self.identity for identity, _ in buffered))
        finally:
            plugin._worker = previous_worker
            plugin._pending_callback_events = previous_pending

    def test_each_teleport_has_its_own_occurrence_identity(self):
        captured = []
        fake_worker = type('Worker', (), {
            'session_id': self.worker.session_id,
            'queue_event': lambda _, identity, event: captured.append((identity, event)) or True,
        })()
        previous_worker = plugin._worker
        try:
            plugin._worker = fake_worker
            with patch.object(plugin, '_get_character_data', return_value=self.identity), \
                    patch.object(plugin, '_get_position', return_value={'region': 25273, 'x': 10, 'y': 20, 'z': 0}):
                plugin.teleported()
                plugin.teleported()
            self.assertEqual(len(captured), 2)
            self.assertEqual([entry[1]['kind'] for entry in captured], ['session.teleported'] * 2)
            self.assertNotEqual(captured[0][1]['event_id'], captured[1][1]['event_id'])
            self.assertNotIn('dedupe_key', captured[0][1])
            self.assertNotIn('dedupe_key', captured[1][1])
        finally:
            plugin._worker = previous_worker

    def test_deferred_empty_payload_callbacks_drop_internal_binding_marker(self):
        callbacks = (
            (plugin.teleported, ()),
            (plugin.handle_event, (plugin.EVENT_ALCHEMY_FINISHED, '')),
        )
        for callback, args in callbacks:
            self.worker._current_identity = None
            self.worker.character_id = None
            self.worker.session_id = None
            event = self.callback(callback, *args)
            self.assertEqual(event['payload']['session_binding'], 'deferred')
            self.worker._event_samples.put_nowait(event)

            self.worker._flush_events(type('Client', (), {'send_json': lambda *_args: None})())
            self.assertEqual(self.worker._death_spool.pending()[-1]['payload']['session_binding'], 'deferred')

            self.worker._current_identity = self.identity
            self.worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
            self.worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
            frames = []
            client = type('Client', (), {'send_json': lambda _, frame: frames.append(frame)})()
            self.worker._flush_events(client)
            wire = next(entry for entry in frames[-1]['events'] if entry['event_id'] == event['event_id'])
            self.assertEqual(wire['payload'], {})
            self.worker._death_spool.acknowledge(event['event_id'])
            self.worker._death_retry_at.pop(event['event_id'], None)


class CharacterCollectorTests(unittest.TestCase):
    def test_dead_state_is_sent_only_when_phbot_returns_a_boolean(self):
        previous = (
            plugin._worker,
            plugin._character_joined,
            plugin._last_character_signature,
            plugin._last_character_sample_at,
            plugin._last_resources_sample_at,
            plugin._death_callback_active,
        )
        worker = Mock()
        try:
            plugin._worker = worker
            plugin._character_joined = True
            plugin._last_character_signature = None
            plugin._last_character_sample_at = 0
            plugin._last_resources_sample_at = time.monotonic()
            plugin._death_callback_active = True
            with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                    patch.object(plugin, '_get_position', return_value=None), \
                    patch.object(plugin, '_get_zone_name', return_value=None):
                for raw_value, expected in ((True, True), (False, False), ('unknown', None)):
                    data = {'server': 'Silkroad', 'name': 'Alpha', 'model': 1907}
                    if raw_value != 'unknown':
                        data['dead'] = raw_value
                    plugin._last_character_signature = None
                    with patch.object(plugin, '_get_character_data', return_value=data):
                        plugin._sample_character()
                    state = worker.update_character.call_args.args[1]
                    self.assertEqual(state['model'], 1907)
                    if expected is None:
                        self.assertNotIn('dead', state)
                    else:
                        self.assertIs(state.get('dead'), expected)
                    if expected is False:
                        self.assertFalse(plugin._death_callback_active)
        finally:
            (plugin._worker, plugin._character_joined,
             plugin._last_character_signature, plugin._last_character_sample_at,
             plugin._last_resources_sample_at, plugin._death_callback_active) = previous

    def test_invalid_character_models_are_omitted_from_state(self):
        previous = (
            plugin._worker,
            plugin._character_joined,
            plugin._last_character_signature,
            plugin._last_character_sample_at,
            plugin._last_resources_sample_at,
        )
        worker = Mock()
        try:
            plugin._worker = worker
            plugin._character_joined = True
            plugin._last_character_sample_at = 0
            plugin._last_resources_sample_at = time.monotonic()
            with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                    patch.object(plugin, '_get_position', return_value=None), \
                    patch.object(plugin, '_get_zone_name', return_value=None):
                for model in (0, -1, 4294967296, True, 1907.0, '1907'):
                    plugin._last_character_signature = None
                    with patch.object(
                        plugin, '_get_character_data',
                        return_value={'server': 'Silkroad', 'name': 'Alpha', 'model': model},
                    ):
                        plugin._sample_character()
                    state = worker.update_character.call_args.args[1]
                    self.assertNotIn('model', state, repr(model))
        finally:
            (plugin._worker, plugin._character_joined,
             plugin._last_character_signature, plugin._last_character_sample_at,
             plugin._last_resources_sample_at) = previous

    def test_character_rejection_stops_repeated_stale_updates(self):
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_test',
        }, 'fixture-phbot')
        identity = {'server': 'Greatest', 'name': 'nuker1'}
        worker.character_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = identity
        worker._handle_character_rejected({
            'type': 'character.rejected',
            'protocol_version': plugin.PROTOCOL_VERSION,
            'character_id': worker.character_id,
            'reason': 'not_current_session',
        })

        client = Mock()
        worker._publish_sample(client, {'identity': identity, 'state': {'hp': 1}}, False)

        self.assertIsNone(worker.character_id)
        self.assertEqual(worker._rejected_identity, ('greatest', 'nuker1'))
        client.send_json.assert_not_called()

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
                bytes(bytearray([0x81, 127])) +
                plugin.struct.pack('!Q', plugin.MAX_MESSAGE_BYTES + 1)
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
            'protocol_version': plugin.PROTOCOL_VERSION,
            'server_time': plugin._utc_now(),
            'heartbeat_interval_seconds': 10,
            'heartbeat_timeout_seconds': 30,
        })
        self.assertEqual(interval, 10)
        with self.assertRaises(plugin.WebSocketClosed):
            worker._validate_ack({'type': 'hello.ack', 'protocol_version': 1})
        fractional_ack = dict({
            'type': 'hello.ack',
            'protocol_version': plugin.PROTOCOL_VERSION,
            'server_time': plugin._utc_now()[:-1] + '.123456789Z',
            'heartbeat_interval_seconds': 10,
            'heartbeat_timeout_seconds': 30,
        })
        self.assertEqual(worker._validate_ack(fractional_ack), 10)
        with self.assertRaises(ValueError):
            plugin._utc_epoch(plugin._utc_now()[:-1] + '.1234567890Z')

    def test_character_messages_resolve_identity_then_use_explicit_id(self):
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture')

        class Transport:
            def __init__(self): self.sent = []
            def send_json(self, value): self.sent.append(value)
            def receive_json(self, timeout=None):
                return {'type':'character.registered','protocol_version':plugin.PROTOCOL_VERSION,'character_id':AGENT_ID,'session_id':'22222222-3333-4444-8555-666666666666'}

        transport = Transport()
        sample = {'identity':{'server':'Silkroad','name':'Alpha','guild':''},'state':{'level':110,'hp':500,'botting':None}}
        worker._publish_sample(transport, sample, False)
        worker._publish_sample(transport, sample, False)
        self.assertEqual(transport.sent[0]['type'], 'character.identify')
        self.assertEqual(transport.sent[0]['guild'], '')
        self.assertEqual(transport.sent[1]['type'], 'character.snapshot')
        self.assertEqual(transport.sent[1]['character_id'], AGENT_ID)
        self.assertEqual(transport.sent[2]['type'], 'character.state')
        self.assertEqual(transport.sent[2]['character_id'], AGENT_ID)
        self.assertEqual(transport.sent[1]['session_id'], '22222222-3333-4444-8555-666666666666')

        unknown_guild = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture')
        transport = Transport()
        unknown_guild._publish_sample(transport, {
            'identity': {'server': 'Silkroad', 'name': 'Beta', 'guild': None},
            'state': {'level': 1},
        }, True)
        self.assertIsNone(transport.sent[0]['guild'])

    def test_callback_dispatch_is_allowlisted_deduplicated_and_truthful(self):
        calls = []
        adapter = plugin.PhBotAdapter({'stop_bot': lambda: calls.append('stop') or False})
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        frame = {'type':'command.execute','protocol_version':plugin.PROTOCOL_VERSION,'command_id':'cmd_00000000-0000-4000-8000-000000000001',
                 'character_id':AGENT_ID,'session_id':worker.session_id,'name':'bot.stop','args':{},'ttl_ms':10000,'expires_at':plugin._utc_now()}
        frame['expires_at'] = time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time()+10))
        worker._accept_command(frame)
        worker._accept_command(frame)
        self.assertTrue(worker.process_one_command({'server':'Silkroad','name':'Alpha'}, 25000))
        self.assertFalse(worker.process_one_command({'server':'Silkroad','name':'Alpha'}, 25000))
        self.assertEqual(calls, ['stop'])
        ack = worker._outgoing.get_nowait()
        result = worker._outgoing.get_nowait()
        self.assertEqual(ack['type'], 'command.ack')
        self.assertEqual(result['status'], 'failed')
        self.assertIs(result['api_return'], False)
        self.assertEqual(result['verification'], 'api_confirmed')

    def test_walk_generates_and_follows_a_bounded_path_before_reporting_observed_arrival(self):
        calls = []
        generated = []
        position = {'region': 25000, 'x': 0.0, 'y': 0.0}
        adapter = plugin.PhBotAdapter({
            'generate_path': lambda x, y: generated.append((x, y)) or [(1, 2), (3, 4)],
            'move_to_region': lambda *args: calls.append(args),
            'get_position': lambda: dict(position),
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        frame = {'type':'command.execute','protocol_version':plugin.PROTOCOL_VERSION,'command_id':'cmd_00000000-0000-4000-8000-000000000002',
                 'character_id':AGENT_ID,'session_id':worker.session_id,'name':'character.walk',
                 'args':{'region':25000,'x':1,'y':2,'z':3},'ttl_ms':10000,'expires_at':time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time()+10))}
        worker._accept_command(frame)
        worker.process_one_command({'server':'Silkroad','name':'Alpha'}, 25001)
        self.assertEqual(calls, [])
        self.assertEqual(generated, [])
        self.assertEqual(worker._outgoing.get_nowait()['type'], 'command.ack')
        rejected = worker._outgoing.get_nowait()
        self.assertEqual(rejected['status'], 'failed')
        self.assertEqual(rejected['reason'], 'invalid_arguments')

        frame['command_id'] = 'cmd_00000000-0000-4000-8000-000000000003'
        worker._accept_command(frame)
        worker.process_one_command({'server':'Silkroad','name':'Alpha'}, 25000)
        self.assertEqual(worker._outgoing.get_nowait()['type'], 'command.ack')
        self.assertEqual(generated, [(1.0, 2.0)])
        self.assertEqual(calls, [(25000, 1.0, 2.0, 3.0)])
        self.assertTrue(worker.process_walk_step({'server':'Silkroad','name':'Alpha'}, 25000))
        self.assertEqual(calls[-1], (25000, 3.0, 4.0, 3.0))
        position.update({'x': 3.0, 'y': 4.0})
        self.assertTrue(worker.process_walk_step({'server':'Silkroad','name':'Alpha'}, 25000))
        result = worker._outgoing.get_nowait()
        self.assertEqual(result['status'], 'completed')
        self.assertEqual(result['verification'], 'observed')
        self.assertEqual(result['route_waypoints'], 2)
        self.assertEqual(result['observed_after'], {'x': 3.0, 'y': 4.0, 'region': 25000})

    def test_walk_rejects_cave_or_cross_region_waypoint_paths(self):
        calls = []
        adapter = plugin.PhBotAdapter({
            'generate_path': lambda x, y: [(25001, x, y)],
            'move_to_region': lambda *args: calls.append(args),
            'get_position': lambda: {'region': 25000, 'x': 0, 'y': 0},
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        frame = {'type':'command.execute','protocol_version':plugin.PROTOCOL_VERSION,'command_id':'cmd_00000000-0000-4000-8000-000000000004',
                 'character_id':AGENT_ID,'session_id':worker.session_id,'name':'character.walk',
                 'args':{'region':25000,'x':1,'y':2,'z':3},'ttl_ms':10000,
                 'expires_at':time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time()+10))}
        worker._accept_command(frame)
        worker.process_one_command({'server':'Silkroad','name':'Alpha'}, 25000)
        worker._outgoing.get_nowait()
        rejected = worker._outgoing.get_nowait()
        self.assertEqual(rejected['status'], 'failed')
        self.assertEqual(rejected['reason'], 'invalid_path')
        self.assertEqual(calls, [])

    def test_walk_stops_advancing_when_original_session_is_superseded(self):
        calls = []
        adapter = plugin.PhBotAdapter({
            'generate_path': lambda x, y: [(20, 20), (40, 40)],
            'move_to_region': lambda *args: calls.append(args),
            'get_position': lambda: {'region': 25000, 'x': 20, 'y': 20},
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        frame = {'type':'command.execute','protocol_version':plugin.PROTOCOL_VERSION,'command_id':'cmd_00000000-0000-4000-8000-000000000005',
                 'character_id':AGENT_ID,'session_id':worker.session_id,'name':'character.walk',
                 'args':{'region':25000,'x':40,'y':40,'z':0},'ttl_ms':10000,
                 'expires_at':time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time()+10))}
        worker._accept_command(frame)
        worker.process_one_command({'server':'Silkroad','name':'Alpha'}, 25000)
        worker._outgoing.get_nowait()  # acknowledged
        worker.session_id = '33333333-4444-4555-8666-777777777777'
        self.assertTrue(worker.process_walk_step({'server':'Silkroad','name':'Alpha'}, 25000))
        result = worker._outgoing.get_nowait()
        self.assertEqual(result['status'], 'unknown')
        self.assertEqual(result['reason'], 'walk_target_changed')
        self.assertEqual(len(calls), 1)

    def test_capability_probe_reports_only_complete_documented_primitives(self):
        adapter = plugin.PhBotAdapter({
            'set_training_position': lambda *args: True,
            'set_training_radius': lambda radius: True,
            'get_training_area': lambda: {'radius': 50.0, 'path': 'private.txt'},
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        frame = worker._capability_frame()
        caps = {item['name']: item for item in frame['commands']}
        self.assertEqual(caps['training.area.set']['modes'], ['position'])
        self.assertTrue(caps['training.radius.set']['supported'])
        self.assertFalse(caps['character.walk']['supported'])
        self.assertFalse(caps['client.clientless']['supported'])
        self.assertEqual(worker._safe_area({'region': 25000, 'x': 1, 'path': 'secret'}), {'training_region': 25000, 'training_x': 1.0})

    def test_control_state_is_initial_session_scoped_and_rate_limited(self):
        adapter = plugin.PhBotAdapter({
            'get_training_area': lambda: {
                'region': 25000, 'x': 12.5, 'y': 9, 'z': 0, 'radius': 50,
                'path': 'must-not-leave-client.txt',
            },
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'

        self.assertTrue(worker.report_control_state())
        self.assertFalse(worker.report_control_state())
        frame = worker._outgoing.get_nowait()
        self.assertEqual(frame['type'], 'character.control_state')
        self.assertEqual(frame['session_id'], worker.session_id)
        self.assertEqual(frame['control_state']['training_radius'], 50.0)
        self.assertNotIn('path', frame['control_state'])

        worker.session_id = '33333333-3333-4444-8555-666666666666'
        self.assertTrue(worker.report_control_state())

    def test_queued_leave_prevents_reconnect_snapshot_resurrection(self):
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture')
        sample = {'identity': {'server': 'Silkroad', 'name': 'Alpha', 'guild': ''}, 'state': {'level': 110}}
        worker._latest_sample = sample
        worker.leave_character()

        class Transport:
            sent = []
            def send_json(self, value): self.sent.append(value)

        transport = Transport()
        worker._restore_latest_sample(transport)
        self.assertEqual(transport.sent, [])
        self.assertIsNone(worker._latest_sample)


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
