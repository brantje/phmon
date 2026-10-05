import importlib.util
import json
import os
import socket
import struct
import tempfile
import threading
import time
import unittest
import uuid
from types import SimpleNamespace
from unittest.mock import Mock, patch

MODULE_PATH = os.path.join(os.path.dirname(__file__), 'PhMon.py')
spec = importlib.util.spec_from_file_location('phmon_plugin', MODULE_PATH)
plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plugin)

AGENT_ID = '11111111-2222-4333-8444-555555555555'


class PlayerObservationTests(unittest.TestCase):
    def test_collect_player_observation_classifies_missing_none_empty_and_unavailable(self):
        self.assertEqual(plugin.collect_player_observation({}), ('unavailable', [], False))
        self.assertEqual(plugin.collect_player_observation({'get_players': None}), ('unavailable', [], False))
        self.assertEqual(plugin.collect_player_observation({'get_players': lambda: None}), ('unavailable', [], False))
        self.assertEqual(plugin.collect_player_observation({'get_players': lambda: {}}), ('observed', [], False))

    def test_collect_player_observation_normalizes_decimal_string_ids_and_fields(self):
        raw = {
            '08654977': {'name': 'Nearby', 'guild': 'Guild', 'grant': 'Member',
                         'dead': False, 'level': 71, 'region': 26244, 'x': 10.5, 'y': 20.0,
                         'items': ['PRIVATE'], 'z': 3.0},
        }
        status, players, truncated = plugin.collect_player_observation({'get_players': lambda: raw})
        self.assertEqual(status, 'observed')
        self.assertFalse(truncated)
        self.assertEqual(players, [{
            'player_id': '8654977', 'name': 'Nearby', 'guild': 'Guild', 'grant': 'Member',
            'dead': False, 'level': 71, 'region': 26244, 'x': 10.5, 'y': 20.0,
        }])
        self.assertNotIn('items', players[0])
        self.assertNotIn('z', players[0])
        self.assertNotIn('zone', players[0])

    def test_collect_player_observation_resolves_zone_once_per_region(self):
        raw = {
            '1': {'name': 'A', 'region': 26753, 'x': 1, 'y': 2},
            '2': {'name': 'B', 'region': 26753, 'x': 3, 'y': 4},
            '3': {'name': 'C', 'x': 5, 'y': 6},
        }
        with patch.object(plugin, '_get_zone_name', return_value=' Taklamakan ') as get_zone:
            status, players, truncated = plugin.collect_player_observation({'get_players': lambda: raw})
        self.assertEqual(status, 'observed')
        self.assertFalse(truncated)
        self.assertEqual([player.get('zone') for player in players], ['Taklamakan', 'Taklamakan', None])
        get_zone.assert_called_once_with(26753)

    def test_collect_player_observation_omits_zone_when_lookup_fails(self):
        raw = {'7': {'name': 'Nearby', 'region': 26753, 'x': 1, 'y': 2}}
        with patch.object(plugin, '_get_zone_name', side_effect=RuntimeError('lookup failed')):
            status, players, truncated = plugin.collect_player_observation({'get_players': lambda: raw})
        self.assertEqual(status, 'observed')
        self.assertFalse(truncated)
        self.assertEqual(players[0]['region'], 26753)
        self.assertNotIn('zone', players[0])

    def test_collect_player_observation_marks_malformed_duplicate_and_bounds(self):
        valid = {'name': 'Player', 'x': 1, 'y': 2}
        raw = {1: valid, 2: None, 3: dict(valid, x=float('nan')), 4: dict(valid, name=''),
               5: dict(valid, level=0), 6: dict(valid, region=True), '01': dict(valid, x=3, y=4)}
        status, players, truncated = plugin.collect_player_observation({'get_players': lambda: raw})
        self.assertEqual(status, 'truncated')
        self.assertTrue(truncated)
        self.assertEqual(len(players), 1)
        self.assertEqual(players[0]['x'], 1.0)

    def test_player_snapshot_signature_includes_zone(self):
        base = {'player_id': '1', 'name': 'A', 'region': 26753, 'x': 1.0, 'y': 2.0}
        named = plugin._player_snapshot_signature('observed', 26753, 0.0, [dict(base, zone='Taklamakan')])
        missing = plugin._player_snapshot_signature('observed', 26753, 0.0, [dict(base)])
        self.assertNotEqual(named, missing)

    def test_sample_players_names_regionless_rows_from_the_observer_region(self):
        previous = (
            plugin._worker, plugin._last_player_poll_at, plugin._last_player_region,
            plugin._last_player_observer_z, plugin._last_player_signature,
            plugin._last_player_publish_at, plugin._player_sample_forced,
        )
        plugin._reset_player_sample_state()
        worker = Mock()
        worker.update_map_players = Mock(return_value=True)
        plugin._worker = worker
        identity = {'server': 'Greatest', 'name': 'Observer'}
        state = {'region': 26753}
        position = {'z': 96.0}
        row = {'player_id': '7', 'name': 'Nearby', 'x': 1.0, 'y': 2.0}
        with patch.object(plugin, 'collect_player_observation', return_value=('observed', [row], False)), \
                patch.object(plugin, '_get_zone_name', return_value='Taklamakan') as get_zone:
            self.assertTrue(plugin._sample_players(identity, state, position, 100.0))
        sent = worker.update_map_players.call_args.args[3]
        self.assertEqual(sent[0]['zone'], 'Taklamakan')
        get_zone.assert_called_once_with(26753)
        plugin._worker, plugin._last_player_poll_at, plugin._last_player_region, \
            plugin._last_player_observer_z, plugin._last_player_signature, \
            plugin._last_player_publish_at, plugin._player_sample_forced = previous

    def test_player_snapshot_signature_includes_observer_z(self):
        players = [{'player_id': '1', 'name': 'A', 'x': 1.0, 'y': 2.0}]
        first = plugin._player_snapshot_signature('observed', 25000, -6.0, players)
        second = plugin._player_snapshot_signature('observed', 25000, 12.0, players)
        self.assertNotEqual(first, second)

    def test_sample_players_respects_poll_refresh_and_forces_on_region_change(self):
        previous = (
            plugin._worker, plugin._last_player_poll_at, plugin._last_player_region,
            plugin._last_player_observer_z, plugin._last_player_signature,
            plugin._last_player_publish_at, plugin._player_sample_forced,
        )
        plugin._reset_player_sample_state()
        worker = Mock()
        worker.update_map_players = Mock(return_value=True)
        plugin._worker = worker
        identity = {'server': 'Greatest', 'name': 'Observer'}
        state = {'region': 25000}
        position = {'z': -6.0}
        now = 100.0
        with patch.object(plugin, 'collect_player_observation', return_value=('observed', [], False)):
            self.assertTrue(plugin._sample_players(identity, state, position, now))
            self.assertFalse(plugin._sample_players(identity, state, position, now + 1.0))
            self.assertTrue(plugin._sample_players(identity, state, position, now + 16.0))
            state['region'] = 25001
            self.assertTrue(plugin._sample_players(identity, state, position, now + 17.0))
        plugin._worker, plugin._last_player_poll_at, plugin._last_player_region, \
            plugin._last_player_observer_z, plugin._last_player_signature, \
            plugin._last_player_publish_at, plugin._player_sample_forced = previous

    def test_flush_map_players_requires_matching_identity(self):
        worker = plugin.AgentWorker({'backend_url': 'ws://127.0.0.1/agent', 'agent_id': AGENT_ID,
                                     'agent_token': 'token'}, '20.1.2')
        worker.character_id = AGENT_ID
        worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        worker._current_identity = {'server': 'Greatest', 'name': 'Observer'}
        client = Mock()
        worker.update_map_players(worker._current_identity, 'observed', 25000,
                                  [{'player_id': '7', 'name': 'Nearby', 'x': 1.0, 'y': 2.0}],
                                  observer_z=-6.0)
        worker._flush_map_players(client)
        self.assertEqual(client.send_json.call_count, 1)
        frame = client.send_json.call_args.args[0]
        self.assertEqual(frame['type'], 'map.players')
        self.assertEqual(frame['protocol_version'], plugin.PROTOCOL_VERSION)
        worker._current_identity = {'server': 'Greatest', 'name': 'Other'}
        worker.update_map_players(worker._current_identity, 'observed', 25000, [], observer_z=-6.0)
        worker._flush_map_players(client)
        self.assertEqual(client.send_json.call_count, 1)


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

            def update_map_monsters(self, identity, status, region, monsters, sample=None, observer_z=None):
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

    def test_monster_collector_accepts_signed_cave_regions(self):
        raw = {'7': {'model': 1933, 'region': -32767, 'x': -24300, 'y': 20, 'z': -9}}
        status, monsters, truncated = plugin.collect_monster_observation(
            {'get_monsters': lambda: raw})
        self.assertEqual((status, truncated), ('observed', False))
        self.assertEqual(monsters[0]['region'], -32767)

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

            def update_map_monsters(self, identity, status, region, monsters, sample=None, observer_z=None):
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

    def test_cave_monsters_match_donwhang_unsigned_alias_and_carry_observer_z(self):
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

            def update_map_monsters(self, identity, status, region, monsters, sample=None, observer_z=None):
                self.snapshots.append((status, region, monsters, sample, observer_z))
                return True

        worker = WorkerStub()
        try:
            plugin._worker = worker
            plugin._last_monster_poll_at = float('-inf')
            plugin._last_mob_cell_samples = {}
            monster = {'id': '7', 'region': 32767, 'x': -24300.0, 'y': 20.0}
            with patch.object(plugin, 'collect_monster_observation', return_value=('observed', [monster], False)):
                plugin._sample_monsters({'name': 'Alpha'}, {'region': -32767},
                                        {'x': -24300, 'y': 20, 'z': -9}, now=20)
            self.assertEqual(len(worker.snapshots), 1)
            status, region, monsters, sample, observer_z = worker.snapshots[0]
            self.assertEqual((status, region, monsters, observer_z), ('observed', -32767, [monster], -9))
            self.assertEqual(sample['region'], -32767)
            self.assertEqual(sample['monsters'], [monster])
            self.assertEqual(sample['observer']['z'], -9.0)
            self.assertTrue(plugin._valid_mob_sample(sample))
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
            self.assertTrue(worker.update_map_monsters(identity, 'observed', 25273, [], sample, observer_z=-9))

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
            self.assertEqual(client.sent[0]['map_snapshot']['observer_z'], -9.0)
            self.assertEqual(client.sent[1]['sample']['floor_id'], 'unmapped')
            self.assertEqual(len(worker._mob_spool.pending()), 1)
            worker._handle_server_message({
                'type': 'mob.sample.ack', 'protocol_version': plugin.PROTOCOL_VERSION,
                'sample_id': sample['sample_id'], 'status': 'persisted',
            })
            self.assertEqual(worker._mob_spool.pending(), [])


class NPCObservationTests(unittest.TestCase):
    def setUp(self):
        self.previous = (
            plugin._worker, plugin._last_npc_poll_at, plugin._last_npc_region,
            plugin._last_npc_signature, plugin._last_npc_publish_at, plugin._npc_sample_forced,
        )
        plugin._reset_npc_sample_state()

    def tearDown(self):
        (plugin._worker, plugin._last_npc_poll_at, plugin._last_npc_region,
         plugin._last_npc_signature, plugin._last_npc_publish_at, plugin._npc_sample_forced) = self.previous

    def test_collector_distinguishes_missing_empty_exception_and_role(self):
        self.assertEqual(plugin.collect_npc_observation({'get_npcs': lambda: None}),
                         ('unavailable', [], False))
        self.assertEqual(plugin.collect_npc_observation({'get_npcs': lambda: {}}),
                         ('observed', [], False))

        def explode():
            raise RuntimeError('npc api failed')

        self.assertEqual(plugin.collect_npc_observation({'get_npcs': explode}),
                         ('unavailable', [], False))
        self.assertEqual(plugin.collect_npc_observation({}), ('unavailable', [], False))
        raw = {
            10: {'name': 'Jangan', 'servername': 'GATE_CH', 'model': 2094,
                 'region': 25000, 'x': 6461.4, 'y': 1097.4},
            273: {'name': 'Herbalist Yangyun', 'servername': 'NPC_CH_POTION',
                  'model': 2005, 'region': 25000, 'x': 6494.4, 'y': 1100.7},
            'bad': {'name': 'Dropped', 'region': 0, 'x': 1, 'y': 2},
        }
        status, npcs, truncated = plugin.collect_npc_observation({'get_npcs': lambda: raw})
        self.assertEqual((status, truncated), ('truncated', True))
        self.assertEqual([npc['id'] for npc in npcs], ['10', '273'])
        self.assertEqual(npcs[0]['role'], 'teleporter')
        self.assertEqual(npcs[1]['role'], 'npc')
        self.assertNotIn('servername', plugin.collect_npc_observation({
            'get_npcs': lambda: {1: {'region': 25000, 'x': 1, 'y': 2}},
        })[1][0])

    def test_collector_bounds_rows_and_text(self):
        raw = {
            str(index): {'name': 'N', 'servername': 'NPC_TEST', 'model': 1,
                         'region': 25000, 'x': index, 'y': index}
            for index in range(plugin.MAX_NPCS_PER_SNAPSHOT + 1)
        }
        status, npcs, truncated = plugin.collect_npc_observation({'get_npcs': lambda: raw})
        self.assertEqual(status, 'truncated')
        self.assertTrue(truncated)
        self.assertEqual(len(npcs), plugin.MAX_NPCS_PER_SNAPSHOT)
        long_name = '名' * 80
        _, bounded, _ = plugin.collect_npc_observation({
            'get_npcs': lambda: {7: {'name': long_name, 'servername': 'S' * 80,
                                     'region': 25000, 'x': 1, 'y': 2}},
        })
        self.assertLessEqual(len(bounded[0]['name'].encode('utf-8')), 64)
        self.assertLess(len(bounded[0]['name']), 64)
        self.assertEqual(len(bounded[0]['servername']), 64)
        self.assertLessEqual(len(bounded[0]['servername'].encode('utf-8')), 64)
        self.assertEqual(bounded[0]['role'], 'npc')

    def test_sampler_filters_region_suppresses_unchanged_and_refreshes(self):
        class WorkerStub:
            def __init__(self):
                self.snapshots = []

            def update_map_npcs(self, identity, status, region, npcs, observer_z=None):
                self.snapshots.append((status, region, npcs, observer_z))
                return True

        worker = WorkerStub()
        plugin._worker = worker
        seen = {'region': 25000}

        def collect():
            return ('observed', [{
                'id': '10', 'name': 'Jangan', 'servername': 'GATE_CH', 'role': 'teleporter',
                'region': seen['region'], 'x': 10.0, 'y': 20.0,
            }, {
                'id': '99', 'name': 'Elsewhere', 'servername': 'NPC_OTHER', 'role': 'npc',
                'region': 25273, 'x': 1.0, 'y': 2.0,
            }], False)

        with patch.object(plugin, 'collect_npc_observation', side_effect=lambda: collect()):
            self.assertTrue(plugin._sample_npcs({'name': 'Alpha'}, {'region': 25000},
                                                 {'x': 1, 'y': 2, 'z': 3}, now=10))
            self.assertFalse(plugin._sample_npcs({'name': 'Alpha'}, {'region': 25000},
                                                  {'x': 1, 'y': 2, 'z': 3}, now=11.9))
            self.assertFalse(plugin._sample_npcs({'name': 'Alpha'}, {'region': 25000},
                                                  {'x': 1, 'y': 2, 'z': 3}, now=12))
            self.assertTrue(plugin._sample_npcs({'name': 'Alpha'}, {'region': 25000},
                                                 {'x': 1, 'y': 2, 'z': 3}, now=25))
            seen['region'] = 32767
            self.assertTrue(plugin._sample_npcs({'name': 'Alpha'}, {'region': -32767},
                                                 {'x': 1, 'y': 2, 'z': -9}, now=25))
        self.assertEqual(len(worker.snapshots), 3)
        status, region, npcs, observer_z = worker.snapshots[0]
        self.assertEqual((status, region, observer_z), ('truncated', 25000, 3))
        self.assertEqual([npc['id'] for npc in npcs], ['10'])
        self.assertEqual(worker.snapshots[2][1], -32767)
        self.assertEqual(worker.snapshots[2][2][0]['region'], 32767)

    def test_teleport_forces_an_immediate_resample(self):
        class WorkerStub:
            def __init__(self):
                self.calls = 0

            def update_map_npcs(self, identity, status, region, npcs, observer_z=None):
                self.calls += 1
                return True

        plugin._worker = WorkerStub()
        with patch.object(plugin, 'collect_npc_observation', return_value=('observed', [], False)):
            plugin._sample_npcs({'name': 'Alpha'}, {'region': 25000}, {'z': 0}, now=10)
            plugin._npc_sample_forced = True
            self.assertTrue(plugin._sample_npcs({'name': 'Alpha'}, {'region': 25000}, {'z': 0}, now=10))
        self.assertEqual(plugin._worker.calls, 2)
        plugin._worker = None
        plugin._npc_sample_forced = False
        plugin.teleported()
        self.assertTrue(plugin._npc_sample_forced)
        plugin._pending_callback_events.get_nowait()

    def test_worker_sends_npc_snapshot_and_clears_it_with_the_session(self):
        worker = plugin.AgentWorker(
            {'backend_url': 'ws://127.0.0.1:1', 'agent_id': AGENT_ID, 'agent_token': 'fixture-token'},
            'fixture',
        )
        identity = {'server': 'greatest', 'name': 'Alpha', 'guild': ''}
        worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        worker._current_identity = identity
        npc = {'id': '10', 'name': 'Jangan', 'servername': 'GATE_CH', 'role': 'teleporter',
               'region': 25000, 'x': 1.0, 'y': 2.0}
        self.assertTrue(worker.update_map_npcs(identity, 'observed', 25000, [npc], observer_z=0))

        class Client:
            def __init__(self):
                self.sent = []

            def send_json(self, message):
                self.sent.append(message)

        client = Client()
        worker._flush_map_npcs(client)
        self.assertEqual(client.sent[0]['type'], 'map.npcs')
        self.assertEqual(client.sent[0]['protocol_version'], plugin.PROTOCOL_VERSION)
        self.assertEqual(client.sent[0]['map_snapshot']['npcs'], [npc])
        worker._flush_map_npcs(client)
        self.assertEqual(len(client.sent), 1)
        worker.update_map_npcs(identity, 'observed', 25000, [])
        worker.clear_map_npcs()
        worker._flush_map_npcs(client)
        self.assertEqual(len(client.sent), 1)
        self.assertIsNone(worker._latest_npc_observation)


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
        self.assertTrue(config['ignore_potion_events'])
        self.assertTrue(config['ignore_pill_events'])

    def test_filter_config_accepts_independent_boolean_values(self):
        config = plugin.validate_config({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_token',
            'ignore_potion_events': False,
            'ignore_pill_events': 'false',
        })
        self.assertFalse(config['ignore_potion_events'])
        self.assertFalse(config['ignore_pill_events'])
        config['ignore_pill_events'] = 'maybe'
        with self.assertRaises(ValueError):
            plugin.validate_config(config)

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
            'ignore_potion_events': False,
            'ignore_pill_events': True,
        }
        try:
            saved = plugin.save_saved_config(path, config)
            loaded = plugin.load_saved_config(path)
            self.assertEqual(loaded, saved)
            with open(path, 'r') as stream:
                content = stream.read()
            self.assertNotIn('{', content)
            self.assertIn('agent_token=phm_secret', content)
            self.assertIn('ignore_potion_events=false', content)
            self.assertIn('ignore_pill_events=true', content)
        finally:
            if os.path.exists(path):
                os.unlink(path)
            directory = os.path.dirname(path)
            if os.path.isdir(directory):
                os.rmdir(directory)
                os.rmdir(root)

    def test_legacy_saved_profile_defaults_item_filters_on(self):
        root = tempfile.mkdtemp()
        path = os.path.join(root, 'legacy.cfg')
        try:
            with open(path, 'w') as stream:
                stream.write('backend_url=ws://example.test/agent\n')
                stream.write('agent_id=' + AGENT_ID + '\n')
                stream.write('agent_token=phm_secret\n')
            config = plugin.load_saved_config(path)
            self.assertTrue(config['ignore_potion_events'])
            self.assertTrue(config['ignore_pill_events'])
        finally:
            if os.path.exists(path):
                os.unlink(path)
            os.rmdir(root)

    def test_item_type_lookup_matches_phbot_type_ids_and_caches_other_types(self):
        plugin._reset_item_type_cache()
        definitions = {
            10: {'tid1': 3, 'tid2': 1, 'tid3': 1},
            11: {'tid1': 3, 'tid2': 2, 'tid3': 6},
            12: {'tid1': 3, 'tid2': 5, 'tid3': 0},
        }
        getter = Mock(side_effect=lambda model: definitions[model])
        with patch.object(plugin, '_optional_phbot_api', return_value=getter):
            self.assertEqual(plugin._item_sort_type('Silkroad', 10), 'Potion')
            self.assertEqual(plugin._item_sort_type('Silkroad', 11), 'Pill')
            self.assertIsNone(plugin._item_sort_type('Silkroad', 12))
            self.assertIsNone(plugin._item_sort_type('Silkroad', 12))
        self.assertEqual(getter.call_count, 3)
        plugin._reset_item_type_cache()

    def test_resource_item_type_lookup_covers_inventory_and_pet_slots(self):
        plugin._reset_item_type_cache()
        inputs = {
            'get_inventory': {'available': True, 'value': {'items': [{'model': 10}, {'model': 12}]}},
            'get_pets': {'available': True, 'value': {'123': {'items': [{'model': 11}]}}},
        }
        definitions = {
            10: {'tid1': 3, 'tid2': 1},
            11: {'tid1': 3, 'tid2': 2},
            12: {'tid1': 3, 'tid2': 5},
        }
        getter = Mock(side_effect=lambda model: definitions[model])
        with patch.object(plugin, '_optional_phbot_api', return_value=getter):
            plugin._cache_resource_item_types(inputs, 'Silkroad')
        self.assertEqual(plugin._cached_item_sort_type('Silkroad', 10), 'Potion')
        self.assertEqual(plugin._cached_item_sort_type('Silkroad', 11), 'Pill')
        self.assertEqual(getter.call_count, 3)
        plugin._reset_item_type_cache()


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
            'get_party': lambda: {55: {'name': 'Ally', 'guild': 'Guild', 'level': 110, 'hp_percent': 8, 'mp_percent': 10, 'player_id': 0, 'x': 123.5, 'y': -456}},
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
        self.assertEqual(result['party']['members'][0]['player_id'], 0)
        self.assertEqual(result['party']['members'][0]['x'], 123.5)
        self.assertEqual(result['party']['members'][0]['y'], -456.0)
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

    def test_pet_inventories_keep_per_pet_slots_empty_and_missing_distinct(self):
        result = plugin.collect_resources(api={
            'get_pets': lambda: {
                101: {'name': 'Wolf', 'type': 'wolf', 'items': [None, {
                    'model': 70, 'servername': 'ITEM_ATTACK', 'quantity': 1,
                    'plus': 0, 'durability': 0,
                    'blues': {0: 0, 3: 12},
                }]},
                102: {'name': 'Porter', 'type': 'transport', 'items': [None, {
                    'model': 71, 'servername': 'ITEM_TRANSPORT', 'quantity': 5,
                }]},
                103: {'name': 'Companion', 'type': 'fellow', 'items': []},
                104: {'name': 'Picker', 'type': 'pick'},
            },
        })['pets']
        by_id = {pet['pet_id']: pet for pet in result['pets']}
        self.assertEqual(by_id['101']['slots'][1]['source_slot'], 1)
        self.assertEqual(by_id['101']['slots'][1]['item']['plus'], 0)
        self.assertEqual(by_id['101']['slots'][1]['item']['durability'], 0)
        blues = by_id['101']['slots'][1]['item']['api_fields']['blues']
        self.assertEqual([entry['key'] for entry in blues['mapping_entries']], ['0', '3'])
        self.assertEqual(by_id['102']['slots'][1]['source_slot'], 1)
        self.assertEqual(by_id['102']['slots'][1]['item']['model'], 71)
        self.assertEqual(by_id['103']['inventory_available'], True)
        self.assertEqual(by_id['103']['slots'], [])
        self.assertEqual(by_id['104']['inventory_available'], False)
        self.assertNotIn('slots', by_id['104'])

    def test_party_normalization_rejects_malformed_nonfinite_and_out_of_range_coordinates(self):
        result = plugin.collect_resources(api={
            'get_party': lambda: {
                1: {'name': 'Valid', 'player_id': 101, 'x': 12, 'y': -34.5},
                2: {'name': 'NaN', 'player_id': 102, 'x': float('nan'), 'y': 1},
                3: {'name': 'Inf', 'player_id': 103, 'x': 1, 'y': float('inf')},
                4: {'name': 'String', 'player_id': 104, 'x': '2', 'y': 3},
                5: {'name': 'Huge', 'player_id': 105, 'x': 1000001, 'y': -1000001},
            },
        })
        by_name = dict((member['name'], member) for member in result['party']['members'])
        self.assertEqual(by_name['Valid']['x'], 12.0)
        self.assertEqual(by_name['Valid']['y'], -34.5)
        self.assertNotIn('x', by_name['NaN'])
        self.assertEqual(by_name['NaN']['y'], 1.0)
        self.assertEqual(by_name['Inf']['x'], 1.0)
        self.assertNotIn('y', by_name['Inf'])
        self.assertNotIn('x', by_name['String'])
        self.assertEqual(by_name['String']['y'], 3.0)
        self.assertNotIn('x', by_name['Huge'])
        self.assertNotIn('y', by_name['Huge'])

    def test_party_empty_and_member_bound_are_preserved(self):
        empty = plugin.collect_resources(api={'get_party': lambda: {}})
        self.assertEqual(empty['party'], {'availability': 'observed', 'members': []})
        bounded = plugin.collect_resources(api={
            'get_party': lambda: dict((index, {'name': 'M%s' % index, 'player_id': index}) for index in range(40)),
        })
        self.assertEqual(len(bounded['party']['members']), 32)

    def test_guild_storage_preserves_valid_api_gold(self):
        for gold in (0, 4567890123):
            with self.subTest(gold=gold):
                result = plugin.collect_resources(api={
                    'get_guild_storage': lambda gold=gold: {
                        'size': 0,
                        'gold': gold,
                        'items': [],
                    },
                })
                self.assertEqual(result['guild_storage']['availability'], 'observed')
                self.assertEqual(result['guild_storage']['gold'], gold)

    def test_guild_storage_rejects_invalid_api_gold(self):
        for gold in (True, -1, 1.5, '123'):
            with self.subTest(gold=gold):
                result = plugin.collect_resources(api={
                    'get_guild_storage': lambda gold=gold: {
                        'size': 0,
                        'gold': gold,
                        'items': [],
                    },
                })
                self.assertEqual(result['guild_storage']['availability'], 'observed')
                self.assertNotIn('gold', result['guild_storage'])

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
        self.assertEqual(resources['item_enrichment']['pet_inventory_status'], 'not_observed')
        self.assertEqual(resources['item_enrichment']['pet_inventory_decoder_build'],
                         'vsro_pet_packet_probe_r1')
        self.assertFalse(resources['item_enrichment']['pet_packet_probe']['packet_bytes_retained'])

    def test_pet_packet_probe_reports_counts_and_lengths_without_packet_bytes(self):
        tracker = plugin.PassiveItemTracker(capture_enabled=True)
        tracker.enqueue(0x30C8, b'pet-snapshot')
        tracker.enqueue(0xB034, b'private-packet-bytes')
        resources = {'equipment': {'availability': 'observed', 'slots': []},
                     'inventory': {'availability': 'observed', 'slots': []}}
        tracker.decorate(resources, 'session-one')
        self.assertEqual(resources['item_enrichment']['pet_inventory_packet_count'], 1)
        self.assertEqual(resources['item_enrichment']['pet_snapshot_packet_count'], 1)
        self.assertEqual(resources['item_enrichment']['pet_inventory_status'], 'candidate_layout_unverified')
        probe = resources['item_enrichment']['pet_packet_probe']
        self.assertEqual(probe['mode'], 'presence_and_size_only')
        self.assertFalse(probe['packet_bytes_retained'])
        self.assertEqual(probe['opcodes']['0x30C8']['last_bytes'], len(b'pet-snapshot'))
        self.assertEqual(probe['opcodes']['0xB034']['last_bytes'], len(b'private-packet-bytes'))
        self.assertEqual(probe['opcodes']['0xB034']['count'], 1)
        capture = tracker.sanitized_capture()
        self.assertEqual(capture['records'][0]['result'], 'presence_and_size_only')
        self.assertNotIn('private-packet-bytes', json.dumps(capture))
        self.assertNotIn('pet-snapshot', json.dumps(capture))

        # 0xB034 carries general inventory operations, so its presence alone
        # cannot be labelled as pet traffic without decoding its operation.
        operation_only = plugin.PassiveItemTracker()
        operation_only.enqueue(0xB034, b'operation')
        operation_resources = {'equipment': {'availability': 'observed', 'slots': []},
                               'inventory': {'availability': 'observed', 'slots': []}}
        operation_only.decorate(operation_resources, 'session-one')
        self.assertEqual(operation_resources['item_enrichment']['pet_inventory_status'], 'not_observed')

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
        event['zone'] = 'Jangan'
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
        self.assertEqual(wire_event['zone'], 'Jangan')

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
        self.assertEqual(replay[0]['events'][0]['zone'], 'Jangan')
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
        previous_attack = plugin._recent_player_attack
        received = []
        fake_worker = type('Worker', (), {
            'session_id': None,
            'queue_event': lambda _, identity, event: received.append((identity, event)) or True,
        })()
        try:
            plugin._worker = fake_worker
            plugin._death_callback_active = False
            plugin._recent_player_attack = None
            with patch.object(plugin, '_get_character_data', return_value={'server': 'Silkroad', 'name': 'Alpha'}), \
                    patch.object(plugin, '_get_position', return_value={'region': 25273, 'x': 1, 'y': 2, 'z': 3}), \
                    patch.object(plugin, '_load_active_profile'):
                plugin.handle_event(plugin.EVENT_DIED, '')
                plugin.handle_event(plugin.EVENT_DIED, '')
                deaths = [entry for entry in received if entry[1]['kind'] == 'character.died']
                self.assertEqual(len(deaths), 1)
                self.assertEqual(deaths[0][1]['source_ref'], 'EVENT_DIED')
                self.assertEqual(deaths[0][1]['payload']['cause'], 'monster_environment')
                self.assertEqual(deaths[0][1]['payload']['reason_type'], 'monster_or_environment')
                plugin.joined_game()
                plugin.handle_event(plugin.EVENT_DIED, '')
                deaths = [entry for entry in received if entry[1]['kind'] == 'character.died']
                self.assertEqual(len(deaths), 2)
                self.assertNotEqual(deaths[0][1]['event_id'], deaths[1][1]['event_id'])
        finally:
            plugin._worker = previous_worker
            plugin._death_callback_active = previous_active
            plugin._recent_player_attack = previous_attack

    def test_death_records_recent_player_attacker_and_consumes_the_observation(self):
        received = []
        fake_worker = type('Worker', (), {
            'session_id': None,
            'queue_event': lambda _, identity, event: received.append(event) or True,
        })()
        with patch.object(plugin, '_worker', fake_worker), \
                patch.object(plugin, '_death_callback_active', False), \
                patch.object(plugin, '_recent_player_attack', None), \
                patch.object(plugin, '_get_character_data', return_value={'server': 'Silkroad', 'name': 'Alpha'}), \
                patch.object(plugin, '_get_position', return_value=None):
            with patch.object(plugin, '_monotonic', return_value=100.0):
                plugin.handle_event(plugin.EVENT_PLAYER_ATTACKING, '  Rival  ')
            with patch.object(plugin, '_monotonic', return_value=109.9):
                plugin.handle_event(plugin.EVENT_DIED, '')
            deaths = [event for event in received if event['kind'] == 'character.died']
            self.assertEqual(len(deaths), 1)
            self.assertEqual(deaths[0]['payload'], {
                'cause': 'Rival', 'reason_type': 'attacker', 'reason_value': 'Rival',
            })
            self.assertIsNone(plugin._recent_player_attack)

    def test_death_does_not_attribute_expired_or_other_character_attack(self):
        received = []
        fake_worker = type('Worker', (), {
            'session_id': None,
            'queue_event': lambda _, identity, event: received.append(event) or True,
        })()
        current = {'server': 'Silkroad', 'name': 'Alpha'}
        with patch.object(plugin, '_worker', fake_worker), \
                patch.object(plugin, '_death_callback_active', False), \
                patch.object(plugin, '_recent_player_attack', None), \
                patch.object(plugin, '_get_character_data', side_effect=lambda: current), \
                patch.object(plugin, '_get_position', return_value=None):
            with patch.object(plugin, '_monotonic', return_value=100.0):
                plugin.handle_event(plugin.EVENT_PLAYER_ATTACKING, 'Rival')
            with patch.object(plugin, '_monotonic', return_value=110.1):
                plugin.handle_event(plugin.EVENT_DIED, '')
            self.assertEqual(received[-1]['payload']['reason_type'], 'monster_or_environment')

            plugin._death_callback_active = False
            with patch.object(plugin, '_monotonic', return_value=200.0):
                plugin.handle_event(plugin.EVENT_PLAYER_ATTACKING, 'Rival')
            current = {'server': 'Silkroad', 'name': 'Beta'}
            with patch.object(plugin, '_monotonic', return_value=201.0):
                plugin.handle_event(plugin.EVENT_DIED, '')
            self.assertEqual(received[-1]['payload']['reason_type'], 'monster_or_environment')

            plugin._death_callback_active = False
            current = {'server': 'Silkroad', 'name': 'Alpha'}
            with patch.object(plugin, '_monotonic', return_value=300.0):
                plugin.handle_event(plugin.EVENT_PLAYER_ATTACKING, 'Rival')
                plugin.handle_event(plugin.EVENT_PLAYER_ATTACKING, '')
                plugin.handle_event(plugin.EVENT_DIED, '')
            self.assertEqual(received[-1]['payload']['reason_type'], 'monster_or_environment')


class ResourceEventDerivationTests(unittest.TestCase):
    def setUp(self):
        with plugin._ITEM_TYPE_CACHE_LOCK:
            self.previous_item_type_cache = plugin.collections.OrderedDict(plugin._ITEM_TYPE_CACHE)
            plugin._ITEM_TYPE_CACHE.clear()
        self.worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_test_token',
        }, 'fixture')
        self.identity = {'server': 'Silkroad', 'name': 'Alpha'}
        self.worker._current_identity = self.identity
        self.worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        self.worker.session_id = 'ffffffff-1111-4222-8333-444444444444'

    def tearDown(self):
        with plugin._ITEM_TYPE_CACHE_LOCK:
            plugin._ITEM_TYPE_CACHE.clear()
            plugin._ITEM_TYPE_CACHE.update(self.previous_item_type_cache)

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

    def test_unique_drop_links_the_observed_inventory_item_and_its_api_evidence(self):
        self.observe(self.resources(bag=[]))
        drop_id = '8d16ab39-10b8-4b26-80aa-a4be01e10f18'
        self.assertTrue(self.worker.register_drop_for_enrichment(self.identity, 77, drop_id, 0))
        item = self.item()
        item['plus'] = 3
        item['api_fields'] = {'blues': {'9': 5}, 'attributes': {'1': 72}}
        self.observe(self.resources(bag=[(13, item)]))
        event = self.drain()[0]
        self.assertEqual(event['kind'], 'item.acquired')
        self.assertEqual(event['payload']['drop_event_id'], drop_id)
        self.assertEqual(event['payload']['item']['api_fields'], item['api_fields'])
        self.assertEqual(event['payload']['acquisition_method'], 'unknown')
        self.assertFalse(self.worker.has_pending_drop_enrichment(self.identity))

    def test_drop_links_new_slot_when_same_model_is_already_owned(self):
        existing = self.item()
        existing['api_fields'] = {'blues': {'9': 1}}
        self.observe(self.resources(bag=[(13, existing)]))
        drop_id = '8d16ab39-10b8-4b26-80aa-a4be01e10f18'
        self.assertTrue(self.worker.register_drop_for_enrichment(self.identity, 77, drop_id, 1))
        dropped = self.item()
        dropped['api_fields'] = {'blues': {'9': 5}}
        self.observe(self.resources(bag=[(13, existing), (14, dropped)]))
        event = self.drain()[0]
        self.assertEqual(event['kind'], 'item.quantity_increased')
        self.assertEqual(event['payload']['drop_event_id'], drop_id)
        self.assertEqual(event['payload']['destination_container'], {'type': 'inventory', 'slot': 14})
        self.assertEqual(event['payload']['item']['api_fields'], dropped['api_fields'])

    def test_party_recipient_gain_keeps_its_own_rolls_without_drop_callback(self):
        old = self.item()
        old['api_fields'] = {'blues': {'9': 1}}
        received = self.item()
        received['api_fields'] = {'blues': {'9': 5}, 'attributes': {'10': 25}}
        baseline = self.resources(bag=[(13, old)])
        baseline['pets'] = {'availability': 'unavailable', 'reason': 'getter_returned_none'}
        self.observe(baseline)
        after = self.resources(bag=[(13, old), (14, received)])
        after['pets'] = {'availability': 'unavailable', 'reason': 'getter_returned_none'}
        self.observe(after)
        events = self.drain()
        self.assertEqual(len(events), 1)
        self.assertEqual(events[0]['kind'], 'item.quantity_increased')
        self.assertEqual(events[0]['payload']['item']['api_fields'], received['api_fields'])
        self.assertEqual(events[0]['payload']['destination_container'],
                         {'type': 'inventory', 'slot': 14})
        self.assertNotIn('drop_event_id', events[0]['payload'])

    def test_unrelated_container_appearance_does_not_hide_recipient_gain(self):
        self.observe(self.resources(bag=[]))
        other = dict(self.item(), model=88, servername='ITEM_OTHER')
        self.observe(self.resources(bag=[(14, self.item())], storage=[(0, other)]))
        events = self.drain()
        self.assertEqual(len(events), 1)
        self.assertEqual(events[0]['kind'], 'item.acquired')
        self.assertEqual(events[0]['payload']['destination_container'],
                         {'type': 'inventory', 'slot': 14})

    def test_appearing_container_same_model_cannot_invent_an_acquisition(self):
        self.observe(self.resources(bag=[]))
        self.observe(self.resources(bag=[(14, self.item())], storage=[(0, self.item())]))
        self.assertEqual(self.drain(), [])

    def test_ambiguous_same_model_gain_never_borrows_an_older_copys_rolls(self):
        old = self.item()
        old['api_fields'] = {'blues': {'9': 1}}
        changed = self.item()
        changed['api_fields'] = {'blues': {'9': 2}}
        received = self.item()
        received['api_fields'] = {'blues': {'9': 5}}
        self.observe(self.resources(bag=[(13, old)]))
        self.observe(self.resources(bag=[(13, changed), (14, received)]))
        event = self.drain()[0]
        self.assertTrue(event['payload']['item_instance_unobserved'])
        self.assertNotIn('api_fields', event['payload']['item'])
        self.assertNotIn('slot', event['payload']['destination_container'])

    def test_drop_does_not_link_when_two_same_model_slots_appear(self):
        self.observe(self.resources(bag=[(13, self.item())]))
        self.assertTrue(self.worker.register_drop_for_enrichment(
            self.identity, 77, '8d16ab39-10b8-4b26-80aa-a4be01e10f18', 1))
        self.observe(self.resources(bag=[(13, self.item()), (14, self.item()), (15, self.item())]))
        self.assertNotIn('drop_event_id', self.drain()[0]['payload'])

    def test_ambiguous_same_model_drops_are_not_linked(self):
        self.observe(self.resources(bag=[]))
        self.assertTrue(self.worker.register_drop_for_enrichment(self.identity, 77,
                        '8d16ab39-10b8-4b26-80aa-a4be01e10f18', 0))
        self.assertTrue(self.worker.register_drop_for_enrichment(self.identity, 77,
                        '21a71980-c560-4bc8-b75b-8d1230d3ba9f', 0))
        self.observe(self.resources(bag=[(13, self.item())]))
        event = self.drain()[0]
        self.assertNotIn('drop_event_id', event['payload'])

    def test_two_new_inventory_items_with_same_model_do_not_claim_one_drop(self):
        self.observe(self.resources(bag=[]))
        self.assertTrue(self.worker.register_drop_for_enrichment(self.identity, 77,
                        '8d16ab39-10b8-4b26-80aa-a4be01e10f18', 0))
        other = self.item()
        other['servername'] = 'ITEM_ETC_TEST_VARIANT'
        self.observe(self.resources(bag=[(13, self.item()), (14, other)]))
        for event in self.drain():
            self.assertNotIn('drop_event_id', event['payload'])

    def test_drop_pending_before_first_resource_baseline_is_discarded(self):
        self.assertTrue(self.worker.register_drop_for_enrichment(self.identity, 77,
                        '8d16ab39-10b8-4b26-80aa-a4be01e10f18', 0))
        self.observe(self.resources(bag=[]))
        self.assertFalse(self.worker.has_pending_drop_enrichment(self.identity))
        self.observe(self.resources(bag=[(13, self.item())]))
        self.assertNotIn('drop_event_id', self.drain()[0]['payload'])

    def test_preexisting_gain_in_stale_resource_baseline_cannot_link_drop(self):
        self.observe(self.resources(bag=[]))
        self.assertTrue(self.worker.register_drop_for_enrichment(self.identity, 77,
                        '8d16ab39-10b8-4b26-80aa-a4be01e10f18', 1))
        self.observe(self.resources(bag=[(13, self.item())]))
        self.assertNotIn('drop_event_id', self.drain()[0]['payload'])

    def test_callback_inventory_baseline_counts_only_bag_items(self):
        items = [None] * 13 + [self.item(1), self.item(2)]
        items[0] = self.item(1)
        with patch.object(plugin, '_optional_phbot_api', return_value=lambda: {'items': items}):
            self.assertEqual(plugin._inventory_model_quantity(77), 3)
        with patch.object(plugin, '_optional_phbot_api', return_value=None):
            self.assertIsNone(plugin._inventory_model_quantity(77))

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

    def test_ignored_potion_loss_advances_baseline_and_other_items_still_emit(self):
        with plugin._ITEM_TYPE_CACHE_LOCK:
            plugin._ITEM_TYPE_CACHE[('silkroad', 77)] = 'Potion'
        other_before = {'model': 88, 'servername': 'ITEM_ETC_OTHER', 'quantity': 2}
        other_after = dict(other_before, quantity=3)
        self.observe(self.resources(bag=[(13, self.item(2)), (14, other_before)]))
        self.observe(self.resources(bag=[(13, self.item(1)), (14, other_after)]))
        events = self.drain()
        self.assertEqual(len(events), 1)
        self.assertEqual(events[0]['item_model'], 88)
        self.assertEqual(events[0]['kind'], 'item.quantity_increased')
        self.observe(self.resources(bag=[(13, self.item(1)), (14, other_after)]))
        self.assertEqual(self.drain(), [])

    def test_ignored_potion_transfer_does_not_emit_item_event(self):
        with plugin._ITEM_TYPE_CACHE_LOCK:
            plugin._ITEM_TYPE_CACHE[('silkroad', 77)] = 'Potion'
        self.observe(self.resources(bag=[(13, self.item(2))], storage=[]))
        self.observe(self.resources(bag=[(13, self.item(1))], storage=[(0, self.item(1))]))
        self.assertEqual(self.drain(), [])

    def test_queue_event_filters_all_item_event_kinds_but_keeps_unknown_types(self):
        with plugin._ITEM_TYPE_CACHE_LOCK:
            plugin._ITEM_TYPE_CACHE[('silkroad', 77)] = 'Potion'
            plugin._ITEM_TYPE_CACHE[('silkroad', 78)] = 'Pill'
        for index, kind in enumerate((
                'drop.item', 'drop.rare', 'item.acquired', 'item.quantity_increased',
                'item.quantity_decreased', 'item.transferred', 'alchemy.attempt')):
            with self.subTest(kind=kind):
                self.assertFalse(self.worker.queue_event(self.identity, {
                    'event_id': str(uuid.uuid4()), 'schema_version': 1,
                    'kind': kind, 'category': kind.split('.', 1)[0],
                    'item_model': 77 if index % 2 == 0 else 78,
                    'source': 'fixture', 'payload': {},
                }))
        unknown = {
            'event_id': str(uuid.uuid4()), 'schema_version': 1,
            'kind': 'item.acquired', 'category': 'item', 'item_model': 99,
            'source': 'fixture', 'payload': {},
        }
        self.assertTrue(self.worker.queue_event(self.identity, unknown))
        self.assertEqual(self.drain()[0]['item_model'], 99)

    def test_filter_options_are_independent_and_unchecked_type_is_recorded(self):
        with plugin._ITEM_TYPE_CACHE_LOCK:
            plugin._ITEM_TYPE_CACHE[('silkroad', 77)] = 'Potion'
            plugin._ITEM_TYPE_CACHE[('silkroad', 78)] = 'Pill'
        self.worker.config['ignore_potion_events'] = False
        potion_event = {
            'event_id': str(uuid.uuid4()), 'schema_version': 1,
            'kind': 'item.acquired', 'category': 'item', 'item_model': 77,
            'source': 'fixture', 'payload': {},
        }
        pill_event = dict(potion_event, event_id=str(uuid.uuid4()), item_model=78)
        self.assertTrue(self.worker.queue_event(self.identity, potion_event))
        self.assertFalse(self.worker.queue_event(self.identity, pill_event))
        queued = self.drain()
        self.assertEqual(len(queued), 1)
        self.assertEqual(queued[0]['item_model'], 77)

    def test_verified_pet_pickup_receipt_keeps_snapshot_and_delta_separate(self):
        worker = self.worker
        item = self.item(4)
        receipt = {
            'observation_id': 'session-fixture:seq-12:pet-1234:slot-3',
            'sequence': '12', 'decoder_version': 'pet-fixture-v1',
            'occurred_at': '2026-10-03T12:00:00Z',
        }
        position = {'region': 25273, 'x': 1.0, 'y': 2.0, 'z': 0.0}
        self.assertTrue(worker._queue_verified_pet_pickup(
            self.identity, item, 2, '1234', 3, receipt, position))
        item['quantity'] = 99
        event = worker._event_samples.get_nowait()
        self.assertEqual(event['source'], 'joymax.pet_inventory')
        self.assertEqual(event['source_ref'], '0xB034')
        self.assertEqual(event['payload']['item']['quantity'], 4)
        self.assertEqual(event['payload']['quantity_delta'], 2)
        self.assertEqual(event['payload']['destination_container'],
                         {'type': 'pets', 'id': '1234', 'slot': 3})
        self.assertTrue(plugin._event_is_critical(event))
        self.assertTrue(worker._queue_verified_pet_pickup(
            self.identity, self.item(4), 2, '1234', 3, receipt, position))
        retry = worker._event_samples.get_nowait()
        self.assertEqual(retry['event_id'], event['event_id'])
        self.assertEqual(retry['payload'], event['payload'])
        worker.session_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        self.assertTrue(worker._queue_verified_pet_pickup(
            self.identity, self.item(4), 2, '1234', 3, receipt, position))
        next_session = worker._event_samples.get_nowait()
        self.assertNotEqual(next_session['event_id'], event['event_id'])
        self.assertNotEqual(next_session['dedupe_key'], event['dedupe_key'])

    def test_pet_receipt_rejects_unbounded_or_nonpositive_delta(self):
        invalids = [
            (0, '1234', 0, {'observation_id': 'x', 'sequence': '1', 'decoder_version': 'fixture-v1'}),
            (1, '', 0, {'observation_id': 'x', 'sequence': '1', 'decoder_version': 'fixture-v1'}),
            (1, '1234', -1, {'observation_id': 'x', 'sequence': '1', 'decoder_version': 'fixture-v1'}),
            (1, '1234', 0, {'observation_id': 'x', 'sequence': '0', 'decoder_version': 'fixture-v1'}),
        ]
        for quantity, pet_id, slot, evidence in invalids:
            with self.subTest(quantity=quantity, pet_id=pet_id, slot=slot):
                self.assertFalse(self.worker._queue_verified_pet_pickup(
                    self.identity, self.item(1), quantity, pet_id, slot, evidence))
        self.assertEqual(self.drain(), [])

    def test_pet_receipt_requires_a_current_session(self):
        self.worker.session_id = None
        self.assertFalse(self.worker._queue_verified_pet_pickup(
            self.identity, self.item(1), 1, '1234', 0,
            {'observation_id': 'x', 'sequence': '1', 'decoder_version': 'fixture-v1'}))
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

    def test_ignored_potion_drop_never_starts_inventory_enrichment(self):
        with plugin._ITEM_TYPE_CACHE_LOCK:
            previous_cache = plugin.collections.OrderedDict(plugin._ITEM_TYPE_CACHE)
            plugin._ITEM_TYPE_CACHE.clear()
            plugin._ITEM_TYPE_CACHE[('silkroad', 77)] = 'Potion'
        previous_config = plugin._profile_config
        try:
            plugin._profile_config = None
            with patch.object(plugin, '_get_character_data', return_value=self.identity), \
                    patch.object(plugin, '_get_position', return_value={'region': 25273, 'x': 10, 'y': 20, 'z': 0}):
                plugin.handle_event(plugin.EVENT_ITEM_DROP, '77')
            self.assertTrue(self.worker._event_samples.empty())
            self.assertEqual(self.worker._pending_drop_events, [])
        finally:
            plugin._profile_config = previous_config
            with plugin._ITEM_TYPE_CACHE_LOCK:
                plugin._ITEM_TYPE_CACHE.clear()
                plugin._ITEM_TYPE_CACHE.update(previous_cache)

    def test_verified_pet_pickup_receipt_replays_exactly_from_durable_spool(self):
        evidence = {
            'observation_id': 'session-fixture:sequence-19:pet-1234:slot-7',
            'sequence': '19', 'decoder_version': 'simulator-pet-v1',
            'occurred_at': '2026-10-03T12:00:00Z',
        }
        item = {'model': 847, 'servername': 'ITEM_TEST', 'quantity': 3, 'plus': 0}
        self.assertTrue(self.worker._queue_verified_pet_pickup(
            self.identity, item, 1, '1234', 7, evidence,
            {'region': 25273, 'x': 1.0, 'y': 2.0, 'z': 0.0}))
        frames = []
        client = type('Client', (), {'send_json': lambda _, frame: frames.append(frame)})()
        self.worker._flush_events(client)
        original = frames[-1]['events'][0]
        self.assertEqual(original['source'], 'joymax.pet_inventory')
        self.assertEqual(original['payload']['quantity_delta'], 1)
        self.assertEqual(original['payload']['item']['quantity'], 3)
        self.assertEqual(len(plugin.DeathEventSpool(self.spool_path).pending()), 1)

        replay_worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'phm_test_token',
            'death_spool_path': self.spool_path,
        }, 'fixture')
        retry_frames = []
        replay_worker._flush_events(type(
            'Client', (), {'send_json': lambda _, frame: retry_frames.append(frame)})())
        self.assertEqual(retry_frames[0]['events'][0], original)
        replay_worker._handle_server_message({
            'type': 'event.batch.ack', 'protocol_version': plugin.PROTOCOL_VERSION,
            'results': [{'event_id': original['event_id'], 'status': 'persisted'}],
        })
        self.assertEqual(plugin.DeathEventSpool(self.spool_path).pending(), [])

    def test_canonical_event_captures_bounded_zone_for_observed_region(self):
        with patch.object(plugin, '_get_zone_name', return_value=' Jangan ') as get_zone:
            event = self.callback(plugin.handle_event, plugin.EVENT_PLAYER_ATTACKING, 'mob')

        self.assertEqual(event['zone'], 'Jangan')
        get_zone.assert_called_once_with(25273)

    def test_canonical_event_omits_missing_or_failed_zone_lookup(self):
        with patch.object(plugin, '_get_zone_name', side_effect=RuntimeError('lookup failed')):
            event = self.callback(plugin.handle_event, plugin.EVENT_DIED, '')
        self.assertNotIn('zone', event)

    def test_zone_lookup_rejects_missing_api_and_invalid_regions(self):
        with patch.object(plugin, '_get_zone_name', return_value='Jangan') as get_zone:
            self.assertIsNone(plugin._zone_name_for_region(0))
            self.assertIsNone(plugin._zone_name_for_region(True))
            self.assertIsNone(plugin._zone_name_for_region(70000))
        get_zone.assert_not_called()

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

    def test_unique_notice_records_spawn_and_kill_without_observer_position(self):
        spawn = struct.pack('<BBI', 5, 0, 1954)
        event = self.callback(plugin.handle_joymax, 0x300C, spawn)
        self.assertEqual(event['kind'], 'world.unique_spawned')
        self.assertEqual(event['source'], 'joymax.unique_notice')
        self.assertEqual(event['source_ref'], '0x300C')
        self.assertEqual(event['payload'], {'model': 1954, 'notice': 'spawn'})
        self.assertNotIn('region', event)
        killer = 'nuker1'.encode('ascii')
        kill = struct.pack('<BBIH', 6, 0, 1954, len(killer)) + killer
        killed = self.callback(plugin.handle_joymax, 0x300C, kill)
        self.assertEqual(killed['payload'], {'model': 1954, 'notice': 'kill', 'killer': 'nuker1'})
        with patch.object(plugin, '_get_character_data', return_value=self.identity):
            plugin.handle_joymax(0x3040, b'')
            plugin.handle_joymax(0x300C, struct.pack('<BBI', 1, 0, 1954))
        self.assertTrue(self.worker._event_samples.empty())

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
    def test_botting_normalizes_only_known_status_values(self):
        cases = (
            ('botting', True),
            ('training', True),
            ('tracing', False),
            (' Botting ', True),
            ('stopped', None),
            ('idle', None),
            ('running', None),
            (None, None),
            (True, None),
            (1, None),
        )
        for value, expected in cases:
            with self.subTest(value=value):
                self.assertIs(plugin._normalize_botting_status(value), expected)

    def test_boolean_character_data_has_precedence_over_optional_status(self):
        for value in (True, False):
            with self.subTest(value=value):
                with patch.object(plugin, '_optional_phbot_api') as resolve:
                    self.assertIs(
                        plugin._read_botting_state({'botting': value}),
                        value,
                    )
                resolve.assert_not_called()

    def test_non_boolean_character_data_falls_back_without_guessing(self):
        getter = Mock(return_value='training')
        self.assertIs(
            plugin._read_botting_state({'botting': 'true'}, getter),
            True,
        )
        getter.assert_called_once_with()
        with patch.object(plugin, '_optional_phbot_api', return_value=None):
            self.assertIs(plugin._read_botting_state({}, None), None)
        self.assertIs(plugin._read_botting_state({}, lambda: None), None)
        self.assertIs(plugin._read_botting_state({}, lambda: 'unknown'), None)

        def unavailable_status():
            raise RuntimeError('status unavailable')

        self.assertIs(plugin._read_botting_state({}, unavailable_status), None)

    def test_character_sample_publishes_normalized_optional_status(self):
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
            plugin._last_character_signature = None
            plugin._last_character_sample_at = 0
            plugin._last_resources_sample_at = time.monotonic()
            with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                    patch.object(plugin, '_get_character_data', return_value={
                        'server': 'Greatest', 'name': 'Alpha', 'botting': 'unknown',
                    }), \
                    patch.object(plugin, '_get_position', return_value=None), \
                    patch.object(plugin, '_get_profile', return_value=None), \
                    patch.object(plugin, '_optional_phbot_api', return_value=lambda: 'tracing'), \
                    patch.object(plugin, '_sample_monsters'), \
                    patch.object(plugin, '_sample_npcs'):
                plugin._sample_character()

            state = worker.update_character.call_args.args[1]
            self.assertIs(state['botting'], False)
        finally:
            (plugin._worker, plugin._character_joined,
             plugin._last_character_signature, plugin._last_character_sample_at,
             plugin._last_resources_sample_at) = previous

    def test_profile_identity_fence_is_local_and_captured_for_session_replacement(self):
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
            plugin._last_character_signature = None
            plugin._last_character_sample_at = 0
            plugin._last_resources_sample_at = time.monotonic()
            with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                    patch.object(plugin, '_get_character_data', return_value={
                        'server': 'Greatest', 'name': 'nuker1',
                    }), \
                    patch.object(plugin, '_get_position', return_value=None), \
                    patch.object(plugin, '_get_profile', return_value='Greatest_Farm'):
                plugin._sample_character()
            identity = worker.update_character.call_args.args[0]
            self.assertEqual(identity['profile_key'], 'Greatest_Farm')
        finally:
            (plugin._worker, plugin._character_joined,
             plugin._last_character_signature, plugin._last_character_sample_at,
             plugin._last_resources_sample_at) = previous

    def test_signed_cave_region_from_get_position_is_preserved(self):
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
            plugin._last_character_signature = None
            plugin._last_character_sample_at = 0
            plugin._last_resources_sample_at = time.monotonic()
            position = {'region': -32767, 'x': -24294.0, 'y': -91.0, 'z': 0.0}
            with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                    patch.object(plugin, '_get_character_data', return_value={
                        'server': 'Greatest', 'name': 'nuker1', 'region': -32767,
                    }), \
                    patch.object(plugin, '_get_position', return_value=position), \
                    patch.object(plugin, '_get_zone_name', return_value='Donwhang Stone Cave'):
                plugin._sample_character()

            state = worker.update_character.call_args.args[1]
            self.assertEqual(state['region'], -32767)
            self.assertEqual(state['x'], -24294.0)
            self.assertEqual(state['y'], -91.0)
            self.assertEqual(state['z'], 0.0)
            self.assertEqual(state['zone'], 'Donwhang Stone Cave')
        finally:
            (plugin._worker, plugin._character_joined,
             plugin._last_character_signature, plugin._last_character_sample_at,
             plugin._last_resources_sample_at) = previous

    def test_invalid_signed_position_regions_are_omitted(self):
        for region in (0, -32769, 65536, True):
            with self.subTest(region=region):
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
                    plugin._last_character_signature = None
                    plugin._last_character_sample_at = 0
                    plugin._last_resources_sample_at = time.monotonic()
                    with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                            patch.object(plugin, '_get_character_data', return_value={
                                'server': 'Greatest', 'name': 'nuker1',
                            }), \
                            patch.object(plugin, '_get_position', return_value={
                                'region': region, 'x': -24294.0, 'y': -91.0, 'z': 0.0,
                            }), \
                            patch.object(plugin, '_get_zone_name', return_value=None):
                        plugin._sample_character()
                    state = worker.update_character.call_args.args[1]
                    self.assertNotIn('region', state)
                finally:
                    (plugin._worker, plugin._character_joined,
                     plugin._last_character_signature, plugin._last_character_sample_at,
                     plugin._last_resources_sample_at) = previous

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


class RealtimePositionTests(unittest.TestCase):
    def setUp(self):
        self.previous = (
            plugin._worker,
            plugin._character_joined,
            plugin._last_position_sample_at,
            plugin._last_position_publish_at,
            plugin._last_position_observed,
            plugin._last_position_session_id,
            plugin._last_character_signature,
            plugin._last_character_sample_at,
            plugin._last_resources_sample_at,
        )
        plugin._last_position_sample_at = 0.0
        plugin._last_position_publish_at = 0.0
        plugin._last_position_observed = None
        plugin._last_position_session_id = None

    def tearDown(self):
        (
            plugin._worker,
            plugin._character_joined,
            plugin._last_position_sample_at,
            plugin._last_position_publish_at,
            plugin._last_position_observed,
            plugin._last_position_session_id,
            plugin._last_character_signature,
            plugin._last_character_sample_at,
            plugin._last_resources_sample_at,
        ) = self.previous

    def test_packet_trigger_throttles_before_get_position(self):
        worker = Mock()
        worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        worker.update_position = Mock(return_value=True)
        plugin._worker = worker
        plugin._character_joined = True
        get_position = Mock(side_effect=[
            {'region': 25000, 'x': 1, 'y': 2, 'z': 3},
            {'region': 25000, 'x': 2, 'y': 2, 'z': 3},
        ])
        with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                patch.object(plugin, '_get_position', get_position):
            self.assertTrue(plugin._sample_realtime_position(100.0))
            self.assertFalse(plugin._sample_realtime_position(100.1))
            self.assertTrue(plugin._sample_realtime_position(100.201))
        self.assertEqual(get_position.call_count, 2)
        self.assertEqual(worker.update_position.call_count, 2)

    def test_stationary_and_jitter_positions_do_not_republish(self):
        worker = Mock()
        worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        worker.update_position = Mock(return_value=True)
        plugin._worker = worker
        plugin._character_joined = True
        get_position = Mock(side_effect=[
            {'region': 25000, 'x': 1.0, 'y': 2.0, 'z': 3.0},
            {'region': 25000, 'x': 1.0005, 'y': 2.0, 'z': 3.0},
            {'region': 25001, 'x': 1.0005, 'y': 2.0, 'z': 3.0},
        ])
        with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                patch.object(plugin, '_get_position', get_position):
            self.assertTrue(plugin._sample_realtime_position(100.0))
            self.assertFalse(plugin._sample_realtime_position(100.3))
            self.assertTrue(plugin._sample_realtime_position(100.6))
        self.assertEqual(worker.update_position.call_count, 2)

    def test_malformed_position_is_ignored(self):
        worker = Mock()
        worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        worker.update_position = Mock(return_value=True)
        plugin._worker = worker
        plugin._character_joined = True
        with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                patch.object(plugin, '_get_position', return_value={
                    'region': 25000, 'x': float('nan'), 'y': 2, 'z': 3,
                }):
            self.assertFalse(plugin._sample_realtime_position(100.0))
        worker.update_position.assert_not_called()

    def test_worker_position_slot_is_latest_only_and_sequence_is_session_scoped(self):
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1/agent',
            'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, '20.1.2')
        worker.character_id = 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee'
        first_session = 'ffffffff-1111-4222-8333-444444444444'
        second_session = '22222222-3333-4444-8555-666666666666'
        worker.session_id = first_session
        worker._adopt_position_session(first_session)
        for x in range(100):
            worker.update_position(
                {'region': 25000, 'x': float(x), 'y': 2.0, 'z': 3.0},
                '2026-10-05T20:00:00Z',
            )
        client = Mock()
        self.assertTrue(worker._flush_realtime_position(client))
        first = client.send_json.call_args.args[0]
        self.assertEqual(first['type'], 'character.position')
        self.assertEqual(first['sequence'], 1)
        self.assertEqual(first['position']['x'], 99.0)
        self.assertIsNone(worker._latest_position)

        worker._adopt_position_session(first_session)
        worker.update_position(
            {'region': 25000, 'x': 100.0, 'y': 2.0, 'z': 3.0},
            '2026-10-05T20:00:01Z',
        )
        worker._flush_realtime_position(client)
        self.assertEqual(client.send_json.call_args.args[0]['sequence'], 2)

        worker.session_id = second_session
        worker._adopt_position_session(second_session)
        worker.update_position(
            {'region': 25000, 'x': 101.0, 'y': 2.0, 'z': 3.0},
            '2026-10-05T20:00:02Z',
        )
        worker._flush_realtime_position(client)
        self.assertEqual(client.send_json.call_args.args[0]['sequence'], 1)

    def test_handle_joymax_only_samples_and_never_has_a_websocket_client(self):
        calls = []

        class CallbackWorker:
            session_id = 'ffffffff-1111-4222-8333-444444444444'

            def update_position(self, position, observed_at=None):
                calls.append(('position', position))
                return True

            def capture_joymax_packet(self, opcode, data):
                calls.append(('packet', opcode))
                return True

        plugin._worker = CallbackWorker()
        plugin._character_joined = True
        with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                patch.object(plugin, '_get_position', return_value={
                    'region': 25000, 'x': 1.0, 'y': 2.0, 'z': 3.0,
                }), \
                patch.object(plugin, '_observe_unique_notice'):
            self.assertTrue(plugin.handle_joymax(plugin.POSITION_MOVEMENT_OPCODE, b''))
        self.assertEqual([entry[0] for entry in calls], ['position', 'packet'])

    def test_xy_only_movement_does_not_churn_character_state(self):
        worker = Mock()
        worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        worker.update_position = Mock(return_value=True)
        plugin._worker = worker
        plugin._character_joined = True
        plugin._last_character_signature = None
        plugin._last_character_sample_at = 0.0
        plugin._last_resources_sample_at = time.monotonic()
        character = {
            'server': 'Greatest', 'name': 'Alpha', 'level': 110,
            'hp': 1000, 'hp_max': 1000, 'region': 25000,
        }
        with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                patch.object(plugin, '_get_character_data', return_value=character), \
                patch.object(plugin, '_get_position', side_effect=[
                    {'region': 25000, 'x': 1.0, 'y': 2.0, 'z': 3.0},
                    {'region': 25000, 'x': 2.0, 'y': 3.0, 'z': 3.0},
                ]), \
                patch.object(plugin, '_get_zone_name', return_value='Jangan'), \
                patch.object(plugin, '_sample_monsters'), \
                patch.object(plugin, '_sample_npcs'), \
                patch.object(plugin, '_sample_players'):
            plugin._sample_character()
            plugin._last_position_publish_at = 0.0
            plugin._sample_character()
        self.assertEqual(worker.update_character.call_count, 1)
        self.assertEqual(worker.update_position.call_count, 2)

    def test_non_position_state_change_still_publishes(self):
        worker = Mock()
        worker.session_id = 'ffffffff-1111-4222-8333-444444444444'
        worker.update_position = Mock(return_value=True)
        plugin._worker = worker
        plugin._character_joined = True
        plugin._last_character_signature = None
        plugin._last_character_sample_at = 0.0
        plugin._last_resources_sample_at = time.monotonic()
        characters = [
            {'server': 'Greatest', 'name': 'Alpha', 'hp': 1000, 'region': 25000},
            {'server': 'Greatest', 'name': 'Alpha', 'hp': 900, 'region': 25000},
        ]
        with patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                patch.object(plugin, '_get_character_data', side_effect=characters), \
                patch.object(plugin, '_get_position', return_value={
                    'region': 25000, 'x': 1.0, 'y': 2.0, 'z': 3.0,
                }), \
                patch.object(plugin, '_get_zone_name', return_value='Jangan'), \
                patch.object(plugin, '_sample_monsters'), \
                patch.object(plugin, '_sample_npcs'), \
                patch.object(plugin, '_sample_players'):
            plugin._sample_character()
            plugin._sample_character()
        self.assertEqual(worker.update_character.call_count, 2)


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


class CommandTransportDrainTests(unittest.TestCase):
    def worker(self, api=None):
        worker = plugin.AgentWorker({'backend_url': 'ws://127.0.0.1:8081/agent',
                                    'agent_id': AGENT_ID, 'agent_token': 'private-token'},
                                   'fixture', api_adapter=plugin.PhBotAdapter(api or {}))
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        return worker

    def test_ack_backlog_is_drained_and_command_received_in_same_tick(self):
        worker = self.worker()
        received = []
        worker._handle_server_message = received.append
        frames = [{'type': 'mob.sample.ack'} for _ in range(20)]
        frames.append({'type': 'command.execute'})
        client = Mock()
        client.receive_json.side_effect = frames + [None]
        self.assertEqual(worker._drain_server_messages(client, 0.25), 21)
        self.assertEqual(received[-1]['type'], 'command.execute')
        self.assertEqual(client.receive_json.call_args_list[0].kwargs['timeout'], 0.25)
        self.assertTrue(all(call.kwargs['timeout'] == 0.001 for call in client.receive_json.call_args_list[1:]))

    def test_continuous_frames_cannot_starve_outbound_sampling(self):
        worker = self.worker()
        worker._handle_server_message = Mock()
        client = Mock()
        client.receive_json.return_value = {'type': 'mob.sample.ack'}
        self.assertEqual(worker._drain_server_messages(client, 0.25), 32)
        self.assertEqual(client.receive_json.call_count, 32)

    def test_direct_move_calls_only_move_to_once_and_logs_outcome(self):
        move = Mock(return_value=None)
        generation = Mock()
        position = Mock()
        worker = self.worker({'move_to': move, 'generate_script': generation, 'get_position': position})
        frame = {'protocol_version': plugin.PROTOCOL_VERSION,
                 'command_id': 'cmd_00000000-0000-4000-8000-000000000001',
                 'character_id': worker.character_id, 'session_id': worker.session_id,
                 'name': 'character.move_to', 'args': {'x': 10, 'y': 20, 'z': 0},
                 'ttl_ms': 10000,
                 'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time() + 10))}
        with patch.object(plugin, '_log') as log:
            worker._accept_command(frame)
            worker.process_one_command()
            worker._accept_command(frame)
        move.assert_called_once_with(10.0, 20.0, 0.0)
        generation.assert_not_called()
        position.assert_not_called()
        lines = [call.args[0] for call in log.call_args_list]
        self.assertTrue(any('received' in line for line in lines))
        self.assertTrue(any('callback started' in line for line in lines))
        self.assertTrue(any('result=completed' in line for line in lines))
        self.assertFalse(any('private-token' in line for line in lines))
        self.assertIsNone(worker._latest_navigation_route)

    def test_direct_move_rejects_nonfinite_or_extra_arguments(self):
        move = Mock()
        worker = self.worker({'move_to': move})
        for args in ({'x': float('nan'), 'y': 20, 'z': 0}, {'x': 10, 'y': 20},
                     {'x': 10, 'y': 20, 'z': 0, 'region': 25000}, {'x': True, 'y': 20, 'z': 0}):
            with self.assertRaisesRegex(ValueError, 'invalid_arguments'):
                worker._invoke('character.move_to', args, None)
        move.assert_not_called()

    def test_result_logs_reason_codes_without_native_exception_text(self):
        worker = self.worker()
        with patch.object(plugin, '_log') as log:
            worker._queue_result({'type': 'command.result', 'command_id': 'cmd-test',
                                 'status': 'failed', 'reason': 'walk,10,20,0 private-token',
                                 'verification': 'unverified'})
            worker._queue_result({'type': 'command.result', 'command_id': 'cmd-test',
                                 'status': 'failed', 'reason': 'api_return_false',
                                 'verification': 'api_confirmed'})
        self.assertIn('reason=api_error', log.call_args_list[0].args[0])
        self.assertNotIn('private-token', log.call_args_list[0].args[0])
        self.assertIn('reason=api_return_false', log.call_args_list[1].args[0])


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
        self.assertEqual(transport.sent[1]['type'], 'agent.capabilities')
        self.assertEqual(transport.sent[2]['type'], 'character.snapshot')
        self.assertEqual(transport.sent[2]['character_id'], AGENT_ID)
        self.assertEqual(transport.sent[3]['type'], 'character.state')
        self.assertEqual(transport.sent[3]['character_id'], AGENT_ID)
        self.assertEqual(transport.sent[2]['session_id'], '22222222-3333-4444-8555-666666666666')

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
        self.assertFalse(caps['character.navigate']['supported'])
        with patch.object(plugin, '_get_zone_name', return_value='Jangan'):
            self.assertEqual(worker._safe_area({'region': 25000, 'x': 1, 'path': 'secret'}), {
                'training_region': 25000, 'training_zone': 'Jangan', 'training_x': 1.0,
            })

    def test_cave_navigation_uses_explicit_signed_region_and_reports_phbot_result(self):
        calls = []
        reject_script = []
        adapter = plugin.PhBotAdapter({
            'generate_script': lambda *args: calls.append(('generate', args)) or ['walk,-24272.5,-93.5,0', 'wait,500'],
            'start_script': lambda script: calls.append(('start', script)) or (False if reject_script else True),
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        frame = {'type':'command.execute','protocol_version':plugin.PROTOCOL_VERSION,
                 'command_id':'cmd_00000000-0000-4000-8000-000000000006',
                 'character_id':AGENT_ID,'session_id':worker.session_id,
                 'name':'character.navigate',
                 'args':{'region':-32767,'x':-24272.5,'y':-93.5,'z':0},
                 'ttl_ms':10000,'expires_at':time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time()+10))}
        worker._accept_command(frame)
        self.assertTrue(worker.process_one_command({'server':'Silkroad','name':'Alpha'}, -32767))
        if worker._navigation_job is not None:
            worker._navigation_job['thread'].join(timeout=1)
            self.assertTrue(worker.process_one_command(worker._current_identity, -32767))
        self.assertEqual(worker._outgoing.get_nowait()['type'], 'command.ack')
        result = worker._outgoing.get_nowait()
        self.assertEqual(result['status'], 'completed')
        self.assertEqual(result['verification'], 'api_confirmed')
        self.assertEqual(calls[0], ('generate', (-32767, -24272.5, -93.5, 0.0)))
        self.assertEqual(calls[1], ('start', 'walk,-24272.5,-93.5,0\nwait,500'))
        route = worker._latest_navigation_route
        self.assertEqual(route['schema_version'], 1)
        self.assertEqual(route['command_id'], frame['command_id'])
        self.assertEqual(route['route_sequence'], 1)
        self.assertEqual(route['instructions'], [
            {'index': 0, 'kind': 'walk', 'x': -24272.5, 'y': -93.5, 'z': 0.0},
            {'index': 1, 'kind': 'wait', 'duration_ms': 500},
        ])
        self.assertNotIn('walk,-24272.5', json.dumps(route))
        self.assertEqual(worker._outgoing.get_nowait()['type'], 'character.control_state')

        frame['command_id'] = 'cmd_00000000-0000-4000-8000-000000000008'
        reject_script.append(True)
        worker._accept_command(frame)
        worker.process_one_command({'server':'Silkroad','name':'Alpha'}, -32767)
        if worker._navigation_job is not None:
            worker._navigation_job['thread'].join(timeout=1)
            self.assertTrue(worker.process_one_command(worker._current_identity, -32767))
        worker._outgoing.get_nowait()
        rejected = worker._outgoing.get_nowait()
        self.assertEqual(rejected['status'], 'failed')
        self.assertEqual(rejected['reason'], 'api_return_false')
        self.assertIs(rejected['api_return'], False)
        self.assertEqual(worker._latest_navigation_route['route_sequence'], 1)
        self.assertEqual(worker._outgoing.get_nowait()['type'], 'character.control_state')

        frame['command_id'] = 'cmd_00000000-0000-4000-8000-000000000007'
        frame['args']['region'] = 0
        worker._accept_command(frame)
        worker.process_one_command({'server':'Silkroad','name':'Alpha'}, -32767)
        if worker._navigation_job is not None:
            worker._navigation_job['thread'].join(timeout=1)
            self.assertTrue(worker.process_one_command(worker._current_identity, -32767))
        worker._outgoing.get_nowait()
        self.assertEqual(worker._outgoing.get_nowait()['reason'], 'invalid_arguments')
        self.assertEqual(len(calls), 4)

    def test_navigation_command_result_is_flushed_before_route_frame(self):
        adapter = plugin.PhBotAdapter({
            'generate_script': lambda *args: ['walk,10,20,0'],
            'start_script': lambda script: True,
            'get_position': lambda: {'region': 25000, 'x': 1, 'y': 2, 'z': 0},
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        frame = {
            'type': 'command.execute', 'protocol_version': plugin.PROTOCOL_VERSION,
            'command_id': 'cmd_00000000-0000-4000-8000-000000000009',
            'character_id': AGENT_ID, 'session_id': worker.session_id,
            'name': 'character.navigate', 'args': {'region': 25000, 'x': 10, 'y': 20, 'z': 0},
            'ttl_ms': 10000,
            'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time() + 10)),
        }
        worker._accept_command(frame)
        self.assertTrue(worker.process_one_command(worker._current_identity, 25000))
        if worker._navigation_job is not None:
            worker._navigation_job['thread'].join(timeout=1)
            self.assertTrue(worker.process_one_command(worker._current_identity, -32767))

        sent = []

        class CaptureClient:
            def send_json(self, value):
                sent.append(value)

        self.assertTrue(worker._flush_navigation_route(CaptureClient()))
        frame_types = [item['type'] for item in sent]
        self.assertEqual(frame_types, [
            'command.ack', 'command.result', 'character.control_state', 'navigation.route',
        ])
        self.assertEqual(sent[1]['status'], 'completed')
        self.assertEqual(sent[3]['route']['command_id'], frame['command_id'])

    def test_navigation_route_parser_rejects_unsafe_scripts_without_partial_output(self):
        script, route = plugin._parse_generated_navigation_script([
            'walk,-12.5,20,0', 'wait,500', 'teleport,GATE_ONE,GATE_TWO', 'walk,40,50,-2.25',
        ])
        self.assertIn('teleport,GATE_ONE,GATE_TWO', script)
        self.assertEqual([item['kind'] for item in route], ['walk', 'wait', 'teleport', 'walk'])
        self.assertEqual(route[2], {'index': 2, 'kind': 'teleport'})
        oversized_frame = 'teleport,' + ('A' * 120) + ',' + ('B' * 120)
        self.assertEqual(len(oversized_frame), 250)
        for invalid in (
            ['walk,nan,1,2'], ['walk,10000001,1,2'], ['walk,1,2,3', 'exec,unsafe'],
            ['wait,1000000'], ['teleport,PRIVATE-NAME,TARGET'], [], ['walk,1,2,3'] * 257,
            ['walk,' + ('1' * 257) + ',1,1'], [oversized_frame] * 256,
        ):
            with self.subTest(invalid=invalid[:1]):
                with self.assertRaises(ValueError):
                    plugin._parse_generated_navigation_script(invalid)

    def test_navigation_route_is_cleared_on_profile_replacement_and_revocation(self):
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=plugin.PhBotAdapter({}))
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {
            'server': 'Greatest', 'name': 'Alpha', 'profile_key': 'ProfileOne',
        }
        worker._navigation_sequence = 1
        worker._latest_navigation_route = {'route_sequence': 1}
        registered = {
            'type': 'character.registered', 'protocol_version': plugin.PROTOCOL_VERSION,
            'character_id': '33333333-4444-4555-8666-777777777777',
            'session_id': '44444444-5555-4666-8777-888888888888',
        }
        sent = []
        client = type('Client', (), {'send_json': lambda _, frame: sent.append(frame)})()
        with patch.object(worker, '_wait_for_registration', return_value=registered):
            worker._publish_sample(client, {
                'identity': {'server': 'Greatest', 'name': 'Alpha', 'profile_key': 'ProfileTwo'},
                'state': {'region': 25000, 'x': 1.0, 'y': 2.0, 'z': 0.0},
            }, False)
        self.assertIsNone(worker._latest_navigation_route)
        self.assertEqual(worker._navigation_sequence, 0)
        self.assertEqual(worker.session_id, registered['session_id'])

        worker._latest_navigation_route = {'route_sequence': 1}
        worker._navigation_sequence = 1
        worker._revoke_session({
            'type': 'command.revoke', 'protocol_version': plugin.PROTOCOL_VERSION,
            'character_id': worker.character_id, 'session_id': worker.session_id,
        })
        self.assertIsNone(worker._latest_navigation_route)
        self.assertEqual(worker._navigation_sequence, 0)

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

        with patch.object(plugin, '_get_zone_name', return_value='Jangan'):
            self.assertTrue(worker.report_control_state())
        self.assertFalse(worker.report_control_state())
        frame = worker._outgoing.get_nowait()
        self.assertEqual(frame['type'], 'character.control_state')
        self.assertEqual(frame['session_id'], worker.session_id)
        self.assertEqual(frame['control_state']['training_radius'], 50.0)
        self.assertEqual(frame['control_state']['training_zone'], 'Jangan')
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


class NavigationGenerationTests(unittest.TestCase):
    def setUp(self):
        self.started = threading.Event()
        self.release = threading.Event()
        self.calls = []
        self.callback_thread = threading.get_ident()

        def generate(*args):
            self.calls.append(('generate', threading.get_ident()))
            self.started.set()
            if not self.release.wait(2):
                raise RuntimeError('test generator was not released')
            return ['walk,10,20,0']

        self.worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID, 'agent_token': 'token',
        }, 'fixture', api_adapter=plugin.PhBotAdapter({
            'generate_script': generate,
            'get_position': lambda: {'region': 25000, 'x': 1, 'y': 2, 'z': 0},
            'start_script': lambda script: self.calls.append(('start', threading.get_ident(), script)) or True,
        }))
        self.worker.character_id = AGENT_ID
        self.worker.session_id = '22222222-3333-4444-8555-666666666666'
        self.identity = {'server': 'Silkroad', 'name': 'Alpha', 'profile_key': 'One'}
        self.worker._current_identity = dict(self.identity)
        self.frame = {
            'type': 'command.execute', 'protocol_version': plugin.PROTOCOL_VERSION,
            'command_id': 'cmd_00000000-0000-4000-8000-000000000001',
            'character_id': AGENT_ID, 'session_id': self.worker.session_id,
            'name': 'character.navigate', 'args': {'region': 25000, 'x': 10, 'y': 20, 'z': 0},
            'ttl_ms': 10000,
            'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time() + 10)),
        }

    def tearDown(self):
        self.release.set()
        job = self.worker._navigation_job
        if job is not None:
            job['thread'].join(timeout=1)
            self.assertFalse(job['thread'].is_alive())

    def begin(self):
        self.worker._accept_command(self.frame)
        started_at = time.monotonic()
        self.assertTrue(self.worker.process_one_command(self.identity, 25000))
        self.assertLess(time.monotonic() - started_at, 0.2)
        self.assertTrue(self.started.wait(1))
        self.assertEqual(self.worker._outgoing.get_nowait()['type'], 'command.ack')

    def finish(self):
        self.release.set()
        self.worker._navigation_job['thread'].join(timeout=1)
        self.assertTrue(self.worker.process_one_command(self.identity, 25000))

    def test_native_rejection_logs_bounded_metadata_and_status_without_retry(self):
        start = Mock(return_value=False)
        statuses = Mock(side_effect=[None, 'private-token unexpected status'])
        worker = self.worker
        worker.api.functions['start_script'] = start
        worker.api.functions['get_status'] = statuses
        lines = ['walk,10,20,0', 'wait,500', 'walk,30,40,0']
        with patch.object(plugin, '_log') as log:
            outcome = worker._finish_navigation(self.frame['args'], lines, self.frame['command_id'])
        self.assertIs(outcome[0], False)
        start.assert_called_once_with('\n'.join(lines))
        self.assertEqual(statuses.call_count, 2)
        messages = [call.args[0] for call in log.call_args_list]
        prepared = next(line for line in messages if ' prepared ' in line)
        self.assertIn(self.frame['command_id'], prepared)
        self.assertIn('steps=3 walk=2 wait=1 teleport=0', prepared)
        self.assertIn('sha256=', prepared)
        self.assertIn('trailing_lf=False', prepared)
        self.assertIn('status=none', prepared)
        self.assertTrue(any('result=false result_type=bool status_after=other_text' in line for line in messages))
        self.assertFalse(any('walk,10,20,0' in line or 'private-token' in line for line in messages))
        self.assertIsNone(worker._last_navigation_evidence)

    def test_start_script_exception_is_redacted_and_suppressed(self):
        worker = self.worker
        worker.api.functions['start_script'] = Mock(
            side_effect=RuntimeError('private script body'),
        )
        with patch.object(plugin, '_log') as log, self.assertRaisesRegex(
            ValueError, 'script_start_failed',
        ) as raised:
            worker._finish_navigation(
                self.frame['args'], ['walk,10,20,0'], self.frame['command_id'],
            )
        self.assertTrue(raised.exception.__suppress_context__)
        self.assertFalse(any('private script body' in call.args[0] for call in log.call_args_list))

    def test_optional_debug_status_failure_does_not_block_script_execution(self):
        worker = self.worker
        start = Mock(return_value=True)
        worker.api.functions['start_script'] = start
        worker.api.functions['get_status'] = Mock(side_effect=RuntimeError('private detail'))
        with patch.object(plugin, '_log'):
            outcome = worker._finish_navigation(self.frame['args'], ['walk,10,20,0'])
        self.assertIs(outcome[0], True)
        start.assert_called_once_with('walk,10,20,0')

    def test_blocked_generation_keeps_callback_sampling_and_result_transport_responsive(self):
        self.begin()
        self.worker._accept_command(self.frame)  # Exact duplicate does not create a generator.
        with patch.object(plugin, '_worker', self.worker), \
                patch.object(plugin, '_PHBOT_AVAILABLE', True), \
                patch.object(plugin, '_character_joined', True), \
                patch.object(plugin, '_get_character_data', return_value={'server': 'Silkroad', 'name': 'Alpha'}), \
                patch.object(plugin, '_get_position', return_value={'region': 25000, 'x': 1, 'y': 2, 'z': 0}), \
                patch.object(plugin, '_get_profile', return_value='One'), \
                patch.object(plugin, '_get_zone_name', return_value='Fixture'), \
                patch.object(plugin, '_last_resources_sample_at', plugin._monotonic()), \
                patch.object(plugin, '_sample_monsters'), \
                patch.object(plugin, '_last_character_signature', None):
            started_at = time.monotonic()
            plugin._sample_character()
            self.assertLess(time.monotonic() - started_at, 0.2)
            self.assertIsNotNone(self.worker._samples.get_nowait())
        sent = []
        self.worker._flush_results(type('Client', (), {'send_json': lambda _, frame: sent.append(frame)})())
        self.assertEqual(len(self.calls), 1)
        self.assertNotEqual(self.calls[0][1], self.callback_thread)
        self.finish()
        self.assertEqual(self.calls[1], ('start', self.callback_thread, 'walk,10,20,0'))
        self.assertEqual(len(self.calls), 2)
        self.assertEqual(self.worker._latest_navigation_route['route_sequence'], 1)

    def test_late_generation_never_invokes_after_lifecycle_or_expiry_change(self):
        changes = {
            'expiry': lambda: self.worker._navigation_job['item'].update(deadline=float('-inf')),
            'session': lambda: setattr(self.worker, 'session_id', AGENT_ID),
            'epoch': lambda: setattr(self.worker, '_profile_epoch', 1),
            'profile': lambda: self.identity.update(profile_key='Two'),
            'revoke': lambda: self.worker._revoke_session(self.frame),
            'leave': lambda: self.worker.leave_character(),
            'stop': lambda: self.worker.stop(),
            'teleport': lambda: self.worker._cancel_navigation_generation('character_teleported'),
        }
        for label, change in changes.items():
            with self.subTest(change=label):
                self.setUp()
                try:
                    self.begin()
                    change()
                    self.assertFalse(self.worker.process_one_command(self.identity, 25000))
                    result = self.worker._outgoing.get_nowait()
                    self.assertEqual(result['status'], 'failed')
                    self.finish()
                    self.assertEqual(len(self.calls), 1)
                    self.assertIsNone(self.worker._latest_navigation_route)
                    self.assertTrue(self.worker._outgoing.empty())
                finally:
                    self.tearDown()

    def test_profile_replacement_cannot_spawn_a_second_blocked_generator(self):
        self.begin()
        self.worker.stop()
        replacement = plugin.AgentWorker(self.worker.config, 'fixture', api_adapter=self.worker.api)
        replacement.character_id = self.frame['character_id']
        replacement.session_id = self.frame['session_id']
        replacement._current_identity = self.identity
        replacement._accept_command(self.frame)
        self.assertTrue(replacement.process_one_command(self.identity, 25000))
        self.assertEqual(replacement._outgoing.get_nowait()['type'], 'command.ack')
        self.assertEqual(replacement._outgoing.get_nowait()['reason'], 'navigation_generation_busy')
        self.assertIsNone(replacement._navigation_job)
        self.assertEqual(len(self.calls), 1)

    def test_generation_failure_or_invalid_route_never_starts_a_script(self):
        for generated, reason in [(None, 'path_not_found'), (False, 'path_rate_limited_or_not_in_game'),
                                  (['walk,nan,20,0'], 'invalid_path')]:
            with self.subTest(generated=generated):
                self.setUp()
                try:
                    self.worker.api.functions['generate_script'] = lambda *args: generated
                    self.worker._accept_command(self.frame)
                    self.worker.process_one_command(self.identity, 25000)
                    self.finish()
                    self.assertEqual(self.worker._outgoing.get_nowait()['type'], 'command.ack')
                    result = self.worker._outgoing.get_nowait()
                    self.assertEqual(result['status'], 'failed')
                    self.assertEqual(result['reason'], reason)
                    self.assertEqual(self.calls, [])
                finally:
                    self.tearDown()


class CallbackTimingTests(unittest.TestCase):
    def test_navigation_stage_reports_elapsed_time_and_preserves_return(self):
        for result in (False, None, True, ['walk,10,20,0', 'teleport,PRIVATE_GATE,PRIVATE_TARGET']):
            with self.subTest(result=result), patch.object(plugin, '_log') as log, \
                    patch.object(plugin, '_monotonic', side_effect=[1.0, 11.25]):
                api = Mock(return_value=result)
                self.assertIs(plugin._navigation_stage('generate_script', api, 'private argument'), result)
                api.assert_called_once_with('private argument')
                self.assertEqual([call.args[0] for call in log.call_args_list], [
                    'navigation generate_script started',
                    'navigation generate_script returned in 10250 ms',
                ])

    def test_navigation_stage_reports_failure_without_logging_exception_data(self):
        with patch.object(plugin, '_log') as log, \
                patch.object(plugin, '_monotonic', side_effect=[1.0, 9.0]):
            with self.assertRaisesRegex(ValueError, 'private script'):
                plugin._navigation_stage('start_script', Mock(side_effect=ValueError('private script')))
            self.assertEqual([call.args[0] for call in log.call_args_list], [
                'navigation start_script started', 'navigation start_script raised in 8000 ms',
            ])

    def test_callback_reports_slowest_stage_even_when_it_raises(self):
        with patch.object(plugin, '_log') as log, \
                patch.object(plugin, '_monotonic', side_effect=[0, 0, 0.2, 0.2, 10.2, 10.3]):
            timing = plugin._CallbackTiming()
            timing.run('resource_collect', lambda: None)
            with self.assertRaises(ValueError):
                timing.run('command', Mock(side_effect=ValueError('private value')))
            timing.report()
            log.assert_called_once_with('event_loop slow: 10300 ms; command=10000 ms, resource_collect=200 ms')

    def test_fast_callback_is_silent(self):
        with patch.object(plugin, '_log') as log, \
                patch.object(plugin, '_monotonic', side_effect=[0, 0.499]):
            plugin._CallbackTiming().report()
            log.assert_not_called()

    def test_event_loop_reports_sampling_delay_in_finally(self):
        clock = [0.0]

        def fail_sample(timing):
            def fail():
                clock[0] += 10
                raise RuntimeError('private runtime details')
            timing.run('resource_collect', fail)

        with patch.object(plugin, '_worker', Mock()), \
                patch.object(plugin, '_load_active_profile'), \
                patch.object(plugin, '_drain_pending_callback_events'), \
                patch.object(plugin, '_set_gui_status'), \
                patch.object(plugin, '_sample_character', side_effect=fail_sample), \
                patch.object(plugin, '_monotonic', side_effect=lambda: clock[0]), \
                patch.object(plugin, '_log') as log:
            with self.assertRaises(RuntimeError):
                plugin.event_loop()
            self.assertIn('resource_collect=10000 ms', log.call_args.args[0])
            self.assertNotIn('private', log.call_args.args[0])


class Issue57NavigationAndTraceTests(unittest.TestCase):
    def test_read_trace_activity_maps_status_values(self):
        self.assertEqual(plugin._read_trace_activity(lambda: 'tracing'), ('tracing', 'get_status'))
        self.assertEqual(plugin._read_trace_activity(lambda: 'botting'), ('not_tracing', 'get_status'))
        self.assertEqual(plugin._read_trace_activity(lambda: 'stopped'), ('unknown', 'get_status'))
        self.assertEqual(plugin._read_trace_activity(lambda: None), ('unknown', 'get_status_unavailable'))

    def test_navigate_stop_requires_matching_active_token(self):
        worker = plugin.AgentWorker(
            {
                'backend_url': 'ws://127.0.0.1/agent',
                'agent_id': AGENT_ID,
                'agent_token': 'token',
            },
            '20.1.2',
        )
        worker.api = Mock()
        worker.api.has = Mock(return_value=True)
        worker.api.call = Mock(return_value=True)
        worker._active_navigation = {
            'command_id': 'cmd_00000000-0000-4000-8000-000000000001',
            'route_sequence': 1,
            'epoch': worker._profile_epoch,
        }
        args = {
            'command_id': 'cmd_00000000-0000-4000-8000-000000000001',
            'route_sequence': 2,
        }
        with self.assertRaises(ValueError):
            worker._invoke('character.navigate.stop', args, 25000)
        worker.api.call.assert_not_called()
        args['route_sequence'] = 1
        result, effective, _, verification = worker._invoke(
            'character.navigate.stop', args, 25000
        )
        worker.api.call.assert_called_once_with('stop_script')
        self.assertTrue(result)
        self.assertIsNone(worker._active_navigation)


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


class RecallPointGateTests(unittest.TestCase):
    def setUp(self):
        self.args = {
            'gate_servername': 'GATE_KT', 'region': 25000,
            'x': 30.0, 'y': 40.0, 'model_id': 2094,
        }
        self.npcs = [{
            'id': '4', 'role': 'teleporter', 'servername': 'GATE_KT',
            'region': 25000, 'x': 30.0, 'y': 40.0, 'model_id': 2094,
        }]

    def test_resolves_only_the_current_sessions_unique_gate(self):
        row, npc_id = plugin._session_recall_point_gate(self.npcs, self.args)
        self.assertEqual(row['servername'], 'GATE_KT')
        self.assertEqual(npc_id, 4)
        self.assertEqual(plugin._session_recall_point_gate(self.npcs, dict(self.args, x=38.01)), None)

    def test_rejects_missing_wrong_or_ambiguous_gate(self):
        self.assertIsNone(plugin._session_recall_point_gate([], self.args))
        self.assertIsNone(plugin._session_recall_point_gate(self.npcs, dict(self.args, region=1)))
        self.assertIsNone(plugin._session_recall_point_gate(self.npcs, dict(self.args, model_id=999)))
        self.assertIsNone(plugin._session_recall_point_gate(self.npcs * 2, self.args))
        self.assertIsNone(plugin._session_recall_point_gate(self.npcs, dict(self.args, opcode=28761)))
        self.assertIsNone(plugin._session_recall_point_gate(self.npcs, dict(self.args, gate_servername='NPC_KT')))
        self.assertIsNone(plugin._session_recall_point_gate([dict(self.npcs[0], id='bad')], self.args))

    def test_live_submission_uses_only_the_current_gate_id(self):
        packets = []
        adapter = plugin.PhBotAdapter({
            'get_npcs': lambda: {4: {
                'name': 'Hotan', 'servername': 'GATE_KT', 'model': 2094,
                'region': 25000, 'x': 30.0, 'y': 40.0,
            }},
            'inject_joymax': lambda *packet: packets.append(packet),
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'fixture',
        }, '20.1.3', api_adapter=adapter)
        worker._current_identity = {'server': 'Greatest', 'name': 'Auren'}
        caps = {row['name']: row for row in worker._capability_frame()['commands']}
        self.assertTrue(caps['character.recall_point.designate']['supported'])
        result, effective, observed, verification = worker._invoke(
            'character.recall_point.designate', self.args, 25000)
        self.assertIsNone(result)
        self.assertIsNone(observed)
        self.assertEqual(verification, 'unverified')
        self.assertEqual(effective['gate_npc_id'], '4')
        self.assertEqual(packets, [(0x7059, b'\x04\x00\x00\x00', False)])

        packets.clear()
        with self.assertRaisesRegex(ValueError, 'recall_point_gate_not_observed'):
            worker._invoke('character.recall_point.designate', dict(self.args, x=38.01), 25000)
        self.assertEqual(packets, [])
        with self.assertRaisesRegex(ValueError, 'recall_point_gate_not_observed'):
            worker._invoke('character.recall_point.designate', dict(self.args, opcode=0x7059), 25000)
        self.assertEqual(packets, [])

    def test_submission_is_limited_to_observed_build_server_and_api(self):
        adapter = plugin.PhBotAdapter({
            'get_npcs': lambda: {}, 'inject_joymax': lambda *_: None,
        })
        for version, server, expected in (
                ('20.1.2', 'Greatest', 'recall_point_build_unverified'),
                ('20.1.3', 'Other', 'recall_point_server_unverified')):
            worker = plugin.AgentWorker({
                'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
                'agent_token': 'fixture',
            }, version, api_adapter=adapter)
            worker._current_identity = {'server': server, 'name': 'Auren'}
            caps = {row['name']: row for row in worker._capability_frame()['commands']}
            self.assertEqual(caps['character.recall_point.designate']['reason'], expected)
            with self.assertRaisesRegex(ValueError, expected):
                worker._invoke('character.recall_point.designate', self.args, 25000)

        supported, reason = plugin._recall_point_support(
            '20.1.3', {'server': 'Greatest'}, plugin.PhBotAdapter({'get_npcs': lambda: {}}))
        self.assertFalse(supported)
        self.assertEqual(reason, 'unsupported_runtime_primitive')

    def test_audited_command_submits_once_and_reports_unverified(self):
        packets = []
        adapter = plugin.PhBotAdapter({
            'get_npcs': lambda: {4: {
                'name': 'Hotan', 'servername': 'GATE_KT', 'model': 2094,
                'region': 25000, 'x': 30.0, 'y': 40.0,
            }},
            'inject_joymax': lambda *packet: packets.append(packet),
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'fixture',
        }, '20.1.3', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        identity = {'server': 'Greatest', 'name': 'Auren'}
        worker._current_identity = identity
        frame = {
            'type': 'command.execute', 'protocol_version': plugin.PROTOCOL_VERSION,
            'command_id': 'cmd_00000000-0000-4000-8000-000000000097',
            'character_id': AGENT_ID, 'session_id': worker.session_id,
            'name': 'character.recall_point.designate', 'args': self.args,
            'ttl_ms': 10000,
            'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time() + 10)),
        }
        worker._accept_command(frame)
        worker._accept_command(frame)
        worker.process_one_command(identity, 25000)
        ack = worker._outgoing.get_nowait()
        result = worker._outgoing.get_nowait()
        self.assertEqual(ack['type'], 'command.ack')
        self.assertEqual(result['status'], 'completed')
        self.assertEqual(result['verification'], 'unverified')
        self.assertEqual(result['effective_args']['gate_npc_id'], '4')
        self.assertEqual(packets, [(0x7059, b'\x04\x00\x00\x00', False)])


class TeleporterProbeTests(unittest.TestCase):
    def test_probe_never_injects_and_caps_pair_calls(self):
        calls = []

        def get_teleport_data(source, destination):
            calls.append((source, destination))
            if destination == plugin._TELEPORT_PROBE_UNKNOWN_DEST:
                return None
            return (1, 42)

        def inject_joymax(*_args, **_kwargs):
            raise AssertionError('inject_joymax must not run during probe')

        npcs = [{
            'id': '10', 'role': 'teleporter', 'name': 'Jangan', 'servername': 'GATE_CH',
        }, {
            'id': '11', 'role': 'teleporter', 'name': 'Donwhang', 'servername': 'GATE_DW',
        }]
        api = {
            'get_npcs': lambda: {},
            'get_teleport_data': get_teleport_data,
            'inject_joymax': inject_joymax,
        }
        result = plugin.probe_teleporter_capabilities(api=api, npcs=npcs)
        self.assertLessEqual(len(calls), plugin.MAX_TELEPORT_PROBE_PAIR_CALLS)
        self.assertEqual(result['enumeration'], 'unsupported')
        self.assertTrue(result['capabilities']['get_teleport_data'])
        self.assertTrue(any(
            test['classification']['result'] == 'tuple' and test['classification']['code'] == 42
            for test in result['pair_tests']
        ))
        self.assertTrue(any(
            test['classification']['result'] == 'none'
            for test in result['pair_tests']
        ))

    def test_probe_reports_missing_teleport_api(self):
        result = plugin.probe_teleporter_capabilities(api={'get_npcs': lambda: {}}, npcs=[])
        self.assertEqual(result['status'], 'unavailable')
        self.assertIn('get_teleport_data_missing', result['errors'])

    def test_probe_classifies_teleport_errors(self):
        def explode(*_args, **_kwargs):
            raise RuntimeError('boom')

        result = plugin.probe_teleporter_capabilities(
            api={'get_teleport_data': explode},
            npcs=[{'id': '1', 'role': 'teleporter', 'name': 'Jangan', 'servername': 'GATE_CH'}],
        )
        self.assertEqual(result['pair_tests'][0]['classification']['result'], 'error')
        self.assertEqual(result['pair_tests'][0]['classification']['type'], 'RuntimeError')

    def test_summarize_teleport_probe(self):
        text = plugin.summarize_teleport_probe({
            'gates': [{'id': '10'}],
            'pair_tests': [{'classification': {'result': 'none'}}],
            'capabilities': {'get_teleport_data': True},
        })
        self.assertIn('1 gate', text)
        self.assertIn('enumeration unsupported', text)

    def test_probe_marks_execution_when_hotan_jangan_resolves(self):
        result = plugin.probe_teleporter_capabilities(
            api={'get_teleport_data': lambda s, d: (1, 7) if s == 'Hotan' and d == 'Jangan' else None},
            npcs=[{
                'id': '4', 'role': 'teleporter', 'name': 'Hotan', 'servername': 'GATE_KT',
            }],
        )
        self.assertEqual(result['execution'], 'documented_script_command_verified_hotan_jangan')

    def test_probe_includes_hotan_jangan_reference_pairs(self):
        gates = [{'id': '4', 'name': 'Hotan', 'servername': 'GATE_KT'}]
        result = plugin.probe_teleporter_capabilities(
            api={'get_teleport_data': lambda _s, _d: None},
            npcs=[{
                'id': '4', 'role': 'teleporter', 'name': 'Hotan', 'servername': 'GATE_KT',
            }],
        )
        tags = [test.get('tag') for test in result['pair_tests']]
        self.assertIn('hotan_to_jangan', tags)
        self.assertIn('gate_kt_to_jangan_gate', tags)
        hotan_jangan = next(
            test for test in result['pair_tests'] if test.get('tag') == 'hotan_to_jangan'
        )
        self.assertEqual(hotan_jangan['source'], 'Hotan')
        self.assertEqual(hotan_jangan['destination'], 'Jangan')

    def test_hotan_jangan_test_requires_gate(self):
        with patch.object(plugin, '_PHBOT_AVAILABLE', True), patch.object(
            plugin, 'collect_npc_observation', return_value=('observed', [], False)
        ), patch.object(plugin, '_optional_phbot_api', return_value=lambda *_a, **_k: None), patch.object(
            plugin, '_set_gui_status'
        ) as set_status:
            plugin.test_teleport_hotan_jangan()
        set_status.assert_called()
        self.assertIn('GATE_KT', set_status.call_args[0][0])

    def test_hotan_jangan_test_runs_script_when_pair_resolves(self):
        scripts = []

        def start_script(line):
            scripts.append(line)
            return True

        npcs = [{
            'id': '4', 'role': 'teleporter', 'name': 'Hotan', 'servername': 'GATE_KT',
        }]
        with patch.object(plugin, '_PHBOT_AVAILABLE', True), patch.object(
            plugin, 'collect_npc_observation', return_value=('observed', npcs, False)
        ), patch.object(
            plugin, '_optional_phbot_api',
            side_effect=lambda name: {
                'get_teleport_data': lambda s, d: (1, 99) if s == 'Hotan' and d == 'Jangan' else None,
                'start_script': start_script,
            }.get(name),
        ), patch.object(plugin, '_set_gui_status'), patch.object(plugin, '_log'):
            plugin.test_teleport_hotan_jangan()
        self.assertEqual(scripts, ['teleport,Hotan,Jangan'])

    def test_character_teleport_command_requires_gate_and_route(self):
        scripts = []
        adapter = plugin.PhBotAdapter({
            'get_teleport_data': lambda s, d: (1, 3) if s == 'Hotan' and d == 'Jangan' else None,
            'start_script': lambda line: scripts.append(line) or True,
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        npcs = [{
            'id': '4', 'role': 'teleporter', 'name': 'Hotan', 'servername': 'GATE_KT',
            'region': 25000, 'x': 30, 'y': 40,
        }]
        frame = {
            'type': 'command.execute', 'protocol_version': plugin.PROTOCOL_VERSION,
            'command_id': 'cmd_00000000-0000-4000-8000-000000000099',
            'character_id': AGENT_ID, 'session_id': worker.session_id,
            'name': 'character.teleport',
            'args': {'source': 'Hotan', 'destination': 'Jangan', 'gate_servername': 'GATE_KT'},
            'ttl_ms': 10000,
            'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time() + 10)),
        }
        with patch.object(plugin, 'collect_npc_observation', return_value=('observed', npcs, False)):
            worker._accept_command(frame)
            worker.process_one_command({'server': 'Silkroad', 'name': 'Alpha'}, 25000)
        worker._outgoing.get_nowait()
        result = worker._outgoing.get_nowait()
        self.assertEqual(result['status'], 'completed')
        self.assertEqual(scripts, ['teleport,Hotan,Jangan'])
        self.assertEqual(result['effective_args']['teleport_code'], 3)

    def test_session_teleporter_gate_requires_source_match(self):
        npcs = [{
            'id': '4', 'role': 'teleporter', 'name': 'Hotan', 'servername': 'GATE_KT',
        }]
        self.assertIsNone(plugin._session_teleporter_gate(npcs, 'GATE_KT', 'Jangan'))
        self.assertEqual(
            plugin._session_teleporter_gate(npcs, 'GATE_KT', 'Hotan')['id'],
            '4',
        )
        self.assertEqual(
            plugin._session_teleporter_gate(npcs, 'GATE_KT', 'GATE_KT')['id'],
            '4',
        )

    def test_character_teleport_rejects_non_tuple_route(self):
        adapter = plugin.PhBotAdapter({
            'get_teleport_data': lambda *_args: False,
            'start_script': lambda *_args: True,
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Silkroad', 'name': 'Alpha'}
        npcs = [{
            'id': '4', 'role': 'teleporter', 'name': 'Hotan', 'servername': 'GATE_KT',
        }]
        frame = {
            'type': 'command.execute', 'protocol_version': plugin.PROTOCOL_VERSION,
            'command_id': 'cmd_00000000-0000-4000-8000-000000000098',
            'character_id': AGENT_ID, 'session_id': worker.session_id,
            'name': 'character.teleport',
            'args': {'source': 'Hotan', 'destination': 'Jangan', 'gate_servername': 'GATE_KT'},
            'ttl_ms': 10000,
            'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time() + 10)),
        }
        with patch.object(plugin, 'collect_npc_observation', return_value=('observed', npcs, False)):
            worker._accept_command(frame)
            worker.process_one_command({'server': 'Silkroad', 'name': 'Alpha'}, 25000)
        worker._outgoing.get_nowait()
        result = worker._outgoing.get_nowait()
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(result['reason'], 'teleport_route_unavailable')

    def test_capability_reports_character_teleport_when_apis_present(self):
        adapter = plugin.PhBotAdapter({
            'get_npcs': lambda: {},
            'get_teleport_data': lambda *_args: None,
            'start_script': lambda *_args: True,
        })
        worker = plugin.AgentWorker({
            'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID,
            'agent_token': 'token',
        }, 'fixture', api_adapter=adapter)
        caps = {item['name']: item for item in worker._capability_frame()['commands']}
        self.assertTrue(caps['character.teleport']['supported'])

    def test_discover_teleport_routes_for_gate(self):
        plugin._teleport_route_cache.clear()

        def get_teleport_data(source, destination):
            if source == 'Hotan' and destination == 'Jangan':
                return (1, 7)
            if source == 'Hotan' and destination == 'Samarkand':
                return (1, 2)
            if source == 'Jangan' and destination == 'Soldier Choiyoung [teleport]':
                return (1, 4)
            return None

        adapter = plugin.PhBotAdapter({
            'get_npcs': lambda: {},
            'get_teleport_data': get_teleport_data,
            'start_script': lambda *_args: True,
        })
        routes = plugin._teleport_routes_for_gate('Hotan', 'GATE_KT', adapter)
        self.assertEqual(
            [item['destination'] for item in routes],
            ['Jangan', 'Samarkand'],
        )
        self.assertEqual(routes[0]['teleport_code'], 7)
        jangan = plugin._teleport_routes_for_gate('Jangan', 'GATE_CH', adapter)
        self.assertIn(
            'Soldier Choiyoung [teleport]',
            [item['destination'] for item in jangan],
        )

    def test_teleport_route_cache_does_not_cross_server_or_session(self):
        plugin._teleport_route_cache.clear()
        calls = []

        def get_teleport_data(source, destination):
            calls.append((source, destination))
            if destination == 'Jangan':
                return (1, 7)
            return None

        adapter = plugin.PhBotAdapter({
            'get_npcs': lambda: {},
            'get_teleport_data': get_teleport_data,
            'start_script': lambda *_args: True,
        })
        plugin._teleport_routes_for_gate('Hotan', 'GATE_KT', adapter, 'Greatest', 'session-a')
        calls.clear()
        plugin._teleport_routes_for_gate('Hotan', 'GATE_KT', adapter, 'Servar', 'session-b')
        self.assertIn(('Hotan', 'Jangan'), calls)
        calls.clear()
        plugin._teleport_routes_for_gate('Hotan', 'GATE_KT', adapter, 'Greatest', 'session-c')
        self.assertIn(('Hotan', 'Jangan'), calls)
        calls.clear()
        plugin._teleport_routes_for_gate('Hotan', 'GATE_KT', adapter, 'Greatest', 'session-a')
        self.assertEqual(calls, [])

    def test_default_adapter_exposes_get_teleport_data_for_capability_gate(self):
        teleport = lambda *_args: None

        def fake_optional(name):
            if name == 'get_teleport_data':
                return teleport
            return None

        with patch.object(plugin, '_optional_phbot_api', side_effect=fake_optional):
            adapter = plugin.PhBotAdapter()
        self.assertTrue(adapter.has('get_teleport_data'))

class ReverseReturnTests(unittest.TestCase):
    def worker(self, result=True, party=None, symbols=True):
        self.calls = []
        def reverse(kind, name):
            self.calls.append((kind, name))
            if isinstance(result, Exception): raise result
            return result
        adapter = plugin.PhBotAdapter({'reverse_return': reverse, 'get_party': lambda: party} if symbols else {})
        worker = plugin.AgentWorker({'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID, 'agent_token': 'fixture'}, 'fixture', api_adapter=adapter)
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        worker._current_identity = {'server': 'Fixture', 'name': 'Alpha'}
        return worker

    def test_modes_and_current_party_validation(self):
        worker = self.worker(party={1: {'name': 'Member'}})
        self.assertEqual(worker._invoke('character.reverse_return', {'type': 0}, None), (True, {'type': 0, 'name': ''}, None, 'api_confirmed'))
        worker._invoke('character.reverse_return', {'type': 1}, None)
        worker._invoke('character.reverse_return', {'type': 2, 'name': ' member '}, None)
        self.assertEqual(self.calls, [(0, ''), (1, ''), (2, 'Member')])
        for name, reason in [('Missing', 'party_member_not_found'), ('Alpha', 'party_self_target')]:
            with self.assertRaisesRegex(ValueError, reason): worker._invoke('character.reverse_return', {'type': 2, 'name': name}, None)
        self.assertEqual(len(self.calls), 3)

    def test_bad_arguments_never_execute(self):
        worker = self.worker()
        for args in [{}, {'type': True}, {'type': 1.0}, {'type': -1}, {'type': 4}, {'type': 0, 'name': None}, {'type': 0, 'name': 'Member'}, {'type': 0, 'name': ' '}, {'type': 2}, {'type': 2, 'name': 'Member\n'}, {'type': 0, 'extra': True}, {'type': 2, 'name': 'é'*51}]:
            with self.assertRaisesRegex(ValueError, 'invalid_arguments'): worker._invoke('character.reverse_return', args, None)
        for args in [{'type': 3}, {'type': 3, 'name': ' '}, {'type': 3, 'name': 'Jangan\n'}, {'type': 3, 'name': 'é'*51}]:
            with self.assertRaisesRegex(ValueError, 'invalid_arguments'): worker._invoke('character.reverse_return', args, None)
        self.assertEqual(self.calls, [])

    def test_named_location_uses_callback_api_without_party_dependency(self):
        worker = self.worker()
        worker.api = plugin.PhBotAdapter({'reverse_return': lambda kind, name: self.calls.append((kind, name)) or True})
        self.assertEqual(worker._invoke('character.reverse_return', {'type': 3, 'name': ' Jangan '}, None), (True, {'type': 3, 'name': 'Jangan'}, None, 'api_confirmed'))
        self.assertEqual(self.calls, [(3, 'Jangan')])
        for result in [True, False]:
            worker = self.worker(result=result)
            self.assertIs(worker._invoke('character.reverse_return', {'type': 3, 'name': 'Hotan'}, None)[0], result)
        for result in [None, 1, 'true']:
            with self.assertRaisesRegex(ValueError, 'invalid_api_result'):
                self.worker(result=result)._invoke('character.reverse_return', {'type': 3, 'name': 'Hotan'}, None)
        with self.assertRaisesRegex(RuntimeError, 'native failure'):
            self.worker(result=RuntimeError('native failure'))._invoke('character.reverse_return', {'type': 3, 'name': 'Hotan'}, None)

    def test_native_result_is_authoritative(self):
        for outcome in [True, False]:
            worker = self.worker(result=outcome)
            self.assertIs(worker._invoke('character.reverse_return', {'type': 0}, None)[0], outcome)
        for outcome in [None, 1, 'true']:
            worker = self.worker(result=outcome)
            with self.assertRaisesRegex(ValueError, 'invalid_api_result'): worker._invoke('character.reverse_return', {'type': 0}, None)
        worker = self.worker(result=RuntimeError('native failure'))
        with self.assertRaisesRegex(RuntimeError, 'native failure'): worker._invoke('character.reverse_return', {'type': 0}, None)

    def test_capability_modes_require_primitives(self):
        worker = self.worker(symbols=False)
        caps = {entry['name']: entry for entry in worker._capability_frame()['commands']}
        self.assertFalse(caps['character.reverse_return']['supported'])
        worker = self.worker()
        caps = {entry['name']: entry for entry in worker._capability_frame()['commands']}
        self.assertEqual(caps['character.reverse_return']['modes'], ['last_return', 'last_death', 'party_member', 'named_location'])
        worker.api = plugin.PhBotAdapter({'reverse_return': lambda *_: True})
        caps = {entry['name']: entry for entry in worker._capability_frame()['commands']}
        self.assertEqual(caps['character.reverse_return']['modes'], ['last_return', 'last_death', 'named_location'])

    def test_default_adapter_discovers_optional_reverse_and_party_apis(self):
        reverse = lambda *_: True
        party = lambda: {}
        with patch.object(plugin, '_optional_phbot_api', side_effect=lambda name: {'reverse_return': reverse, 'get_party': party}.get(name)):
            adapter = plugin.PhBotAdapter()
        self.assertTrue(adapter.has('reverse_return'))
        self.assertTrue(adapter.has('get_party'))

    def test_callback_result_and_session_fencing(self):
        for outcome in [True, False]:
            worker = self.worker(result=outcome)
            frame = {'type': 'command.execute', 'protocol_version': plugin.PROTOCOL_VERSION, 'command_id': 'cmd_00000000-0000-4000-8000-000000000099', 'character_id': AGENT_ID, 'session_id': worker.session_id, 'name': 'character.reverse_return', 'args': {'type': 0}, 'ttl_ms': 10000, 'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time()+10))}
            worker._accept_command(frame)
            worker.process_one_command(worker._current_identity, None)
            worker._outgoing.get_nowait()
            result = worker._outgoing.get_nowait()
            self.assertEqual(result['status'], 'completed' if outcome else 'failed')
            self.assertIs(result['api_return'], outcome)
            self.assertEqual(result['verification'], 'api_confirmed')
            worker._accept_command(frame)
            worker.process_one_command(worker._current_identity, None)
            self.assertEqual(len(self.calls), 1)
        worker = self.worker()
        frame['session_id'] = worker.session_id
        worker._accept_command(frame)
        worker.session_id = AGENT_ID
        worker.process_one_command(worker._current_identity, None)
        self.assertEqual(self.calls, [])

    def command(self, worker, args, **changes):
        frame = {'type': 'command.execute', 'protocol_version': plugin.PROTOCOL_VERSION,
                 'command_id': 'cmd_00000000-0000-4000-8000-000000000098',
                 'character_id': AGENT_ID, 'session_id': worker.session_id,
                 'name': 'character.reverse_return', 'args': args, 'ttl_ms': 10000,
                 'expires_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(time.time()+10))}
        frame.update(changes)
        worker._accept_command(frame)
        return frame

    def test_callback_errors_and_nonboolean_results_are_failed(self):
        for outcome in [None, 1, 'true', RuntimeError('native failure')]:
            worker = self.worker(result=outcome)
            self.command(worker, {'type': 0})
            worker.process_one_command(worker._current_identity, None)
            worker._outgoing.get_nowait()
            result = worker._outgoing.get_nowait()
            self.assertEqual(result['status'], 'failed')
            self.assertEqual(len(self.calls), 1)

    def test_party_changed_after_admission_does_not_use_scroll(self):
        party = {1: {'name': 'Member'}}
        worker = self.worker(party=party)
        self.command(worker, {'type': 2, 'name': 'Member'})
        party.clear()
        worker.process_one_command(worker._current_identity, None)
        worker._outgoing.get_nowait()
        result = worker._outgoing.get_nowait()
        self.assertEqual(result['reason'], 'party_member_not_found')
        self.assertEqual(self.calls, [])

    def test_expired_command_does_not_use_scroll(self):
        worker = self.worker()
        self.command(worker, {'type': 1}, expires_at='2000-01-01T00:00:00Z')
        worker.process_one_command(worker._current_identity, None)
        self.assertEqual(self.calls, [])
        messages = []
        while not worker._outgoing.empty():
            messages.append(worker._outgoing.get_nowait())
        self.assertTrue(any(m.get('status') == 'failed' and m.get('reason') == 'command_expired' for m in messages))

class ReverseReturnPartyFreshnessTests(unittest.TestCase):
    def test_unchanged_party_is_refreshed_without_resending_inventory(self):
        worker = plugin.AgentWorker({'backend_url': 'ws://127.0.0.1:8081/agent', 'agent_id': AGENT_ID, 'agent_token': 'fixture'}, 'fixture', api_adapter=plugin.PhBotAdapter({}))
        worker.character_id = AGENT_ID
        worker.session_id = '22222222-3333-4444-8555-666666666666'
        frames = []
        client = SimpleNamespace(send_json=frames.append)
        resources = {'party': {'availability': 'observed', 'members': [{'name': 'Member'}]}, 'inventory': {'availability': 'observed', 'capacity': 0, 'used_slots': 0, 'slots': []}}
        self.assertTrue(worker._send_resource_snapshot(client, resources, refresh_party=True))
        self.assertTrue(worker._send_resource_snapshot(client, resources))
        self.assertEqual(len(frames), 1, 'cached resources must not refresh party checked_at')
        self.assertTrue(worker._send_resource_snapshot(client, resources, refresh_party=True))
        self.assertEqual(frames[-1]['type'], 'resource.delta')
        self.assertEqual(set(frames[-1]['resources']), {'party'})
        self.assertEqual(frames[-1]['revision'], 2)


if __name__ == '__main__':
    unittest.main()
