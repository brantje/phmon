# -*- coding: utf-8 -*-
from __future__ import print_function

import base64
import calendar
import collections
import hashlib
import itertools
import json
import math
import os
import random
import re
import select
import socket
import ssl
import struct
import threading
import time
import uuid
try:
    import queue as _queue
except ImportError:  # pragma: no cover
    import Queue as _queue
try:
    from urllib.parse import urlparse
except ImportError:  # pragma: no cover - Python 2 is not supported, kept harmless for embedded variants.
    from urlparse import urlparse

pName = 'PhMon'
pVersion = '1.9.24'
pUrl = ''

PROTOCOL_VERSION = 16
EVENT_DIED = 7
EVENT_UNIQUE_SPAWN = 0
EVENT_HUNTER_SPAWN = 1
EVENT_THIEF_SPAWN = 2
EVENT_TRANSPORT_DIED = 3
EVENT_PLAYER_ATTACKING = 4
RECENT_PLAYER_ATTACK_SECONDS = 10.0
EVENT_RARE_DROP = 5
EVENT_ITEM_DROP = 6
EVENT_ALCHEMY_FINISHED = 8
EVENT_GM_SPAWNED = 9
EVENT_LEVEL_UP = 10
MAX_EVENT_CRITICAL_ITEMS = 512
MAX_EVENT_CRITICAL_BYTES = 8 * 1024 * 1024
MAX_EVENT_ORDINARY_ITEMS = 2048
MAX_EVENT_ORDINARY_BYTES = 16 * 1024 * 1024
MAX_EVENT_SPOOL_ITEMS = MAX_EVENT_CRITICAL_ITEMS + MAX_EVENT_ORDINARY_ITEMS
MAX_EVENT_SPOOL_BYTES = MAX_EVENT_CRITICAL_BYTES + MAX_EVENT_ORDINARY_BYTES
MAX_EVENT_BYTES = 40 * 1024
MAX_EVENT_PAYLOAD_BYTES = 32768
MAX_EVENT_BATCH_SIZE = 16
MAX_MONSTERS_PER_SNAPSHOT = 128
MAX_NPCS_PER_SNAPSHOT = 128
NPC_POLL_INTERVAL_SECONDS = 2.0
NPC_REFRESH_INTERVAL_SECONDS = 15.0
MAX_PLAYERS_PER_SNAPSHOT = 128
MAX_PLAYERS_SNAPSHOT_BYTES = 64 * 1024
PLAYER_POLL_INTERVAL_SECONDS = 1.0 # Changed by user do not change this value
PLAYER_REFRESH_INTERVAL_SECONDS = 2 # Changed by user do not change this value
_NPC_GATE_ROLE = re.compile(r'^GATE_[A-Za-z0-9_]+$')
_TELEPORT_PROBE_SYMBOL_RE = re.compile(
    r'(?:teleport|npc|gate|recall|location|script)',
    re.IGNORECASE,
)
MAX_TELEPORT_PROBE_PAIR_CALLS = 16
_MAX_TELEPORT_ROUTES_PER_GATE = 32
_MAX_TELEPORT_ROUTE_PROBE_CALLS = 64
_TELEPORT_ROUTE_CACHE_TTL_SECONDS = 120.0
_TELEPORT_PROBE_UNKNOWN_DEST = '__phmon_probe_unknown_destination__'
# Bounded labels probed via get_teleport_data; only successful pairs are published.
_TELEPORT_DESTINATION_CANDIDATES = (
    'Jangan', 'Hotan', 'Donwhang', 'Samarkand', 'Constantinople',
    'Alexandria (North)', 'Alexandria (South)',
    'Tiger Mountain', 'Baghdad', 'Alexandria', 'Constantinople Fortress',
    'GATE_CH', 'GATE_KT', 'GATE_HWANG', 'GATE_CA',
    # Special links use the exact phBot menu label, including the [teleport] suffix.
    'Soldier Choiyoung [teleport]', 'Jupiter Room',
)
_teleport_route_cache = {}
# Documented phBot examples (Hotan gate → Jangan); read-only in probe, optional operator script test.
_TELEPORT_PROBE_REFERENCE_PAIRS = (
    ('Hotan', 'Jangan', 'hotan_to_jangan'),
    ('GATE_KT', 'GATE_CH', 'gate_kt_to_jangan_gate'),
)
_HOTAN_JANGAN_SCRIPT_PAIRS = (
    ('Hotan', 'Jangan'),
    ('GATE_KT', 'GATE_CH'),
)
MAX_MOB_SPOOL_ITEMS = 2048
MAX_MOB_SPOOL_BYTES = 8 * 1024 * 1024
MOB_POLL_INTERVAL_SECONDS = 0.1
MOB_SAMPLE_INTERVAL_SECONDS = 60.0
MOB_OBSERVER_CELL_SIZE = 192.0
DEFAULT_HEARTBEAT_INTERVAL = 10
DEFAULT_HEARTBEAT_TIMEOUT = 30
POSITION_PUBLISH_INTERVAL_SECONDS = 0.2
POSITION_JITTER_TOLERANCE = 0.001
POSITION_COORDINATE_LIMIT = 10000000.0
POSITION_MOVEMENT_OPCODE = 0xB021
NETWORK_LOOP_MAX_WAIT_SECONDS = 0.05
MAX_MESSAGE_BYTES = 256 * 1024
MAX_RESOURCE_SLOTS = 2048
MAX_ITEM_TYPE_CACHE_ENTRIES = 4096
RESOURCE_SAMPLE_INTERVAL_SECONDS = 2.0
DROP_ENRICHMENT_SAMPLE_INTERVAL_SECONDS = 1.0
DROP_ENRICHMENT_WINDOW_SECONDS = 30.0
MAX_ITEM_PACKET_COUNT = 128
MAX_ITEM_PACKET_BYTES = 2 * 1024 * 1024
MAX_ITEM_PACKET_SIZE = 256 * 1024
MAX_ITEM_MAGIC_OPTIONS = 32
MAX_ITEM_CAPTURE_RECORDS = 100
MAX_ITEM_CAPTURE_BYTES = 2 * 1024 * 1024
# Enable only for a bounded, local diagnostic capture. Captured records contain
# parsed item fields or packet sizes only; the live callback never writes to disk.
ITEM_PACKET_CAPTURE_ENABLED = False
ITEM_DECODER_BUILD = 'vsro_1188_passive_r2'
PET_INVENTORY_DECODER_BUILD = 'vsro_pet_packet_probe_r1'
ITEM_API_EVIDENCE_VERSION = 2
ITEM_API_EVIDENCE_FIELDS = (
    'variance', 'magic_options', 'magic_option', 'blues', 'blue', 'options',
    'option', 'magic', 'advanced_elixir', 'attributes', 'attribute', 'attrs',
    'attr', 'white_stats', 'white_stat', 'whites', 'white', 'stats',
    'durability_max', 'phy_def_pwr', 'mag_def_pwr', 'parry_ratio',
    'phy_absorption', 'mag_absorption', 'phy_reinforce', 'mag_reinforce',
    # Named scalar fields observed in the live phBot 20.1.1 API schema. Preserve
    # their values as evidence before enabling backend units/range interpretation.
    'phys_def', 'mag_def', 'parry', 'block', 'critical', 'attack_rate',
    'max_durability', 'phys_atk_min', 'phys_atk_max', 'mag_atk_min', 'mag_atk_max',
    'phys_reinf_min', 'phys_reinf_max', 'mag_reinf_min', 'mag_reinf_max',
    'phys_absorb_min', 'phys_absorb_max', 'mag_absorb_min', 'mag_absorb_max',
)
_WEBSOCKET_GUID = '258EAFA5-E914-47DA-95CA-C5AB0DC85B11'
_ITEM_TYPE_CACHE = collections.OrderedDict()
_ITEM_TYPE_CACHE_LOCK = threading.Lock()
_ITEM_TYPE_OTHER = object()
_ITEM_TYPE_CACHE_MISS = object()
_ITEM_TYPE_FILTERS = {
    'Potion': 'ignore_potion_events',
    'Pill': 'ignore_pill_events',
}
MAX_WALK_WAYPOINTS = 256
WALK_ARRIVAL_TOLERANCE = 12.0
WALK_TIMEOUT_SECONDS = 300.0
# One native path-generation call per plugin, even across profile worker replacement.
_NAVIGATION_GENERATION_SLOT = threading.BoundedSemaphore(1)

try:
    from phBot import get_config_dir as _get_config_dir
    from phBot import get_config_path as _get_config_path
    from phBot import get_profile as _get_profile
    from phBot import get_version as _get_phbot_version
    from phBot import get_character_data as _get_character_data
    from phBot import get_position as _get_position
    from phBot import get_zone_name as _get_zone_name
    from phBot import log as _phbot_log
    _PHBOT_AVAILABLE = True
except ImportError:
    _PHBOT_AVAILABLE = False
    _get_config_dir = None
    _get_config_path = None
    _get_profile = None
    _get_phbot_version = None
    _get_character_data = None
    _get_position = None
    _get_zone_name = None
    _phbot_log = None

try:
    import QtBind as _QtBind
except ImportError:
    _QtBind = None

def _optional_phbot_api(name):
    try:
        import phBot
        return getattr(phBot, name, None)
    except Exception:
        return None

def _normalize_botting_status(value):
    if not isinstance(value, str):
        return None
    status = value.strip().lower()
    if status in ('botting', 'training'):
        return True
    if status == 'tracing':
        return False
    return None

def _read_trace_activity(status_getter=None):
    if status_getter is None:
        status_getter = _optional_phbot_api('get_status')
    if not callable(status_getter):
        return 'unknown', 'get_status_unavailable'
    try:
        value = status_getter()
    except Exception:
        return 'unknown', 'get_status_unavailable'
    if not isinstance(value, str):
        return 'unknown', 'get_status_unavailable'
    status = value.strip().lower()
    if status == 'tracing':
        return 'tracing', 'get_status'
    if status in ('botting', 'training'):
        return 'not_tracing', 'get_status'
    return 'unknown', 'get_status'


def _read_botting_state(character_data, status_getter=None, timing=None):
    """Prefer a boolean character field, then narrowly normalize optional status text."""
    if isinstance(character_data, dict):
        value = character_data.get('botting')
        if isinstance(value, bool):
            return value
    if status_getter is None:
        status_getter = _optional_phbot_api('get_status')
    if not callable(status_getter):
        return None
    try:
        value = timing.run('bot_status', status_getter) if timing else status_getter()
    except Exception:
        return None
    # None remains unknown until a supported runtime observation verifies that it
    # means stopped. Do not turn missing status into a guessed false value.
    return _normalize_botting_status(value)

_API_NAMES = ('start_bot','stop_bot','start_trace','stop_trace','get_status','get_position','get_monsters','get_npcs','get_teleport_data','inject_joymax',
              'generate_path','set_training_position','set_training_radius','set_training_area','get_training_area',
              'move_to','move_to_region','generate_script','start_script','stop_script','use_return_scroll','reverse_return','get_party','disconnect')

_CHAT_METHODS = {
    'general': ('All',),
    'private': ('Private',),
    'party': ('Party',),
    'guild': ('Guild',),
    'union': ('Union',),
    'global': ('Global',),
}

def _optional_chat_api():
    try:
        import phBotChat
        return phBotChat
    except Exception:
        return None

class PhBotAdapter(object):
    """Narrow allowlisted wrapper over documented phBot functions."""
    def __init__(self, functions=None, chat_functions=None):
        self.functions = functions or dict((name, _optional_phbot_api(name)) for name in _API_NAMES)
        if chat_functions is None:
            chat_api = _optional_chat_api()
            chat_functions = dict((channel, getattr(chat_api, methods[0], None) if chat_api is not None else None)
                                  for channel, methods in _CHAT_METHODS.items())
        self.chat_functions = dict(chat_functions)
        self.get_character_data = self.functions.get('get_character_data') or _optional_phbot_api('get_character_data')
        self.get_position = self.functions.get('get_position') or _optional_phbot_api('get_position')
    def has(self, name):
        if name == 'get_position': return callable(self.get_position)
        return callable(self.functions.get(name))
    def call(self, name, *args):
        function = self.functions.get(name)
        if not callable(function): raise RuntimeError('unsupported_runtime_primitive')
        return function(*args)
    def chat_modes(self):
        return [channel for channel in _CHAT_METHODS if callable(self.chat_functions.get(channel))]
    def send_chat(self, channel, text, recipient=None):
        function = self.chat_functions.get(channel)
        if not callable(function): raise RuntimeError('unsupported_channel')
        if channel == 'private': return function(recipient, text)
        return function(text)
    def character(self): return self.get_character_data() if callable(self.get_character_data) else None
    def position(self): return self.get_position() if callable(self.get_position) else None

def _result_json(value):
    try: return json.dumps(value, separators=(',', ':'), allow_nan=False)
    except Exception: return 'null'

def _number(value):
    return isinstance(value, (int, float)) and not isinstance(value, bool) and value == value and abs(value) != float('inf')

def _parse_generated_navigation_script(generated):
    """Validate once and return executable text plus script-free map geometry."""
    if not isinstance(generated, (list, tuple)) or not generated or len(generated) > 256:
        raise ValueError('invalid_path')
    lines = []
    instructions = []
    for index, line in enumerate(generated):
        if not isinstance(line, str) or len(line) > 256:
            raise ValueError('invalid_path')
        if re.fullmatch(r'walk,-?\d+(?:\.\d+)?,-?\d+(?:\.\d+)?,-?\d+(?:\.\d+)?', line):
            _, x_text, y_text, z_text = line.split(',')
            x, y, z = float(x_text), float(y_text), float(z_text)
            if not all(_number(value) and abs(value) <= 10000000 for value in (x, y, z)):
                raise ValueError('invalid_path')
            instructions.append({'index': index, 'kind': 'walk', 'x': x, 'y': y, 'z': z})
        elif re.fullmatch(r'wait,\d{1,6}', line):
            instructions.append({'index': index, 'kind': 'wait', 'duration_ms': int(line.split(',')[1])})
        elif re.fullmatch(r'teleport,[A-Za-z0-9_]+,[A-Za-z0-9_]+', line):
            instructions.append({'index': index, 'kind': 'teleport'})
        else:
            raise ValueError('invalid_path')
        lines.append(line)
    script = '\n'.join(lines)
    if len(script.encode('utf-8')) > 32768:
        raise ValueError('invalid_path')
    return script, instructions

def _valid_position_region(value):
    return (isinstance(value, int) and not isinstance(value, bool) and
            value != 0 and -32768 <= value <= 65535)


def _mob_region_matches(observer_region, monster_region):
    # Donwhang Stone Cave reports its character region as -32767 while
    # get_monsters() reports the same cave region as 32767.
    return (monster_region == observer_region or
            observer_region == -32767 and monster_region == 32767)


def _call_api(name):
    function = _optional_phbot_api(name)
    if not callable(function):
        return False, None
    try:
        return True, function()
    except Exception:
        return True, None


def _bounded_text(value, limit=256):
    if not isinstance(value, (str, int)) or isinstance(value, bool):
        return None
    result = str(value).strip()
    return result[:limit] if result else None


def _canonical_player_id(identifier):
    if isinstance(identifier, int) and not isinstance(identifier, bool):
        if 0 < identifier <= 4294967295:
            return str(identifier)
        return None
    if isinstance(identifier, str):
        text = identifier.strip()
        if not text or len(text) > 64:
            return None
        if text.isdigit():
            try:
                value = int(text, 10)
            except ValueError:
                return None
            if 0 < value <= 4294967295:
                return str(value)
    return None


def _bounded_player_text(value):
    if not isinstance(value, str):
        return None
    text = value.strip()
    if not text:
        return None
    while text and len(text.encode('utf-8')) > 64:
        text = text[:-1]
    return text or None


def collect_player_observation(api=None):
    """Return an explicitly classified, bounded get_players() observation."""
    function = (api or {}).get('get_players') if isinstance(api, dict) else None
    if not callable(function):
        if isinstance(api, dict):
            return 'unavailable', [], False
        function = _optional_phbot_api('get_players')
    if not callable(function):
        return 'unavailable', [], False
    try:
        raw = function()
    except Exception:
        return 'unavailable', [], False
    if raw is None or not isinstance(raw, dict):
        return 'unavailable', [], False
    truncated = len(raw) > MAX_PLAYERS_PER_SNAPSHOT
    players = []
    seen = set()
    zone_cache = {}
    for index, (identifier, value) in enumerate(raw.items()):
        if index >= MAX_PLAYERS_PER_SNAPSHOT:
            break
        if not isinstance(value, dict):
            truncated = True
            continue
        player_id = _canonical_player_id(identifier)
        name = _bounded_player_text(value.get('name'))
        x = value.get('x', value.get('X'))
        y = value.get('y', value.get('Y'))
        if (player_id is None or name is None or not _number(x) or not _number(y) or
                abs(x) > 1000000 or abs(y) > 1000000):
            truncated = True
            continue
        if player_id in seen:
            truncated = True
            continue
        level = value.get('level')
        if ('level' in value and level is not None and
                not (isinstance(level, int) and not isinstance(level, bool) and 1 <= level <= 255)):
            truncated = True
            continue
        region = value.get('region')
        if ('region' in value and region is not None and not _valid_position_region(region)):
            truncated = True
            continue
        row = {
            'player_id': player_id, 'name': name,
            'x': float(x), 'y': float(y),
        }
        for key in ('guild', 'grant'):
            detail = _bounded_player_text(value.get(key))
            if detail is not None:
                row[key] = detail
        if isinstance(value.get('dead'), bool):
            row['dead'] = value['dead']
        if isinstance(level, int) and not isinstance(level, bool) and 1 <= level <= 255:
            row['level'] = level
        if _valid_position_region(region):
            row['region'] = region
            zone = _cached_player_zone(region, zone_cache)
            if zone:
                row['zone'] = zone
        encoded = json.dumps(players + [row], separators=(',', ':'), sort_keys=True, allow_nan=False).encode('utf-8')
        if len(encoded) > MAX_PLAYERS_SNAPSHOT_BYTES:
            truncated = True
            break
        seen.add(player_id)
        players.append(row)
    return ('truncated' if truncated else 'observed'), players, truncated


def _player_snapshot_signature(status, region, observer_z, players):
    rows = []
    for player in players:
        rows.append((
            player.get('player_id'), player.get('name'), player.get('guild'), player.get('grant'),
            player.get('dead'), player.get('level'), player.get('region'), player.get('zone'),
            player.get('x'), player.get('y'),
        ))
    rows.sort()
    return (status, region, observer_z, tuple(rows))


def collect_monster_observation(api=None):
    """Return an explicitly classified, bounded get_monsters() observation."""
    function = (api or {}).get('get_monsters') if isinstance(api, dict) else None
    if not callable(function):
        function = _optional_phbot_api('get_monsters')
    if not callable(function):
        return 'unavailable', [], False
    try:
        raw = function()
    except Exception:
        return 'unavailable', [], False
    if raw is None or not isinstance(raw, dict):
        return 'unavailable', [], False
    entries = list(raw.items())
    truncated = len(entries) > MAX_MONSTERS_PER_SNAPSHOT
    monsters = []
    for monster_id, value in entries[:MAX_MONSTERS_PER_SNAPSHOT]:
        if not isinstance(value, dict):
            truncated = True
            continue
        identifier = _bounded_text(monster_id, 64)
        model = value.get('model')
        region = value.get('region')
        x = value.get('x', value.get('X'))
        y = value.get('y', value.get('Y'))
        z = value.get('z', value.get('Z'))
        if (identifier is None or model is not None and (not isinstance(model, int) or isinstance(model, bool) or model < 0 or model > 4294967295) or not _valid_position_region(region) or
                not _number(x) or not _number(y) or abs(x) > 1000000 or abs(y) > 1000000):
            truncated = True
            continue
        monster = {'id': identifier, 'region': region,
                   'x': float(x), 'y': float(y)}
        if model is not None:
            monster['model_id'] = model
        monster_type = _bounded_text(value.get('type'), 64)
        while monster_type and len(monster_type.encode('utf-8')) > 64:
            monster_type = monster_type[:-1]
        if monster_type is not None:
            monster['type'] = monster_type
        raw_type = value.get('type')
        if isinstance(raw_type, int) and not isinstance(raw_type, bool) and 0 <= raw_type <= 255:
            monster['type_code'] = raw_type
        for source, target in (('name', 'name'), ('servername', 'servername')):
            detail = _bounded_text(value.get(source), 128)
            while detail and len(detail.encode('utf-8')) > 128:
                detail = detail[:-1]
            if detail is not None:
                monster[target] = detail
        level = value.get('level')
        if isinstance(level, int) and not isinstance(level, bool) and 1 <= level <= 255:
            monster['level'] = level
        for source in ('hp', 'max_hp'):
            detail = value.get(source)
            if isinstance(detail, int) and not isinstance(detail, bool) and 0 <= detail <= 9007199254740991:
                monster[source] = detail
        attacking = value.get('attacking')
        if isinstance(attacking, (bool, int)):
            monster['attacking'] = attacking > 0
        if _number(z) and abs(z) <= 1000000:
            monster['z'] = float(z)
        monsters.append(monster)
    return ('truncated' if truncated else 'observed'), monsters, truncated


def _npc_role(servername):
    if isinstance(servername, str) and _NPC_GATE_ROLE.fullmatch(servername):
        return 'teleporter'
    return 'npc'


def collect_npc_observation(api=None):
    """Return an explicitly classified, bounded get_npcs() observation."""
    function = (api or {}).get('get_npcs') if isinstance(api, dict) else None
    if not callable(function):
        if isinstance(api, dict):
            return 'unavailable', [], False
        function = _optional_phbot_api('get_npcs')
    if not callable(function):
        return 'unavailable', [], False
    try:
        raw = function()
    except Exception:
        return 'unavailable', [], False
    if raw is None or not isinstance(raw, dict):
        return 'unavailable', [], False
    entries = list(raw.items())
    truncated = len(entries) > MAX_NPCS_PER_SNAPSHOT
    npcs = []
    for npc_id, value in entries[:MAX_NPCS_PER_SNAPSHOT]:
        if not isinstance(value, dict):
            truncated = True
            continue
        identifier = _bounded_text(npc_id, 64)
        model = value.get('model')
        region = value.get('region')
        x = value.get('x', value.get('X'))
        y = value.get('y', value.get('Y'))
        if (identifier is None or model is not None and (
                not isinstance(model, int) or isinstance(model, bool) or model < 0 or model > 4294967295) or
                not _valid_position_region(region) or not _number(x) or not _number(y) or
                abs(x) > 1000000 or abs(y) > 1000000):
            truncated = True
            continue
        servername = _bounded_text(value.get('servername'), 64)
        while servername and len(servername.encode('utf-8')) > 64:
            servername = servername[:-1]
        npc = {
            'id': identifier, 'role': _npc_role(servername), 'region': region,
            'x': float(x), 'y': float(y),
        }
        name = _bounded_text(value.get('name'), 64)
        while name and len(name.encode('utf-8')) > 64:
            name = name[:-1]
        if name is not None:
            npc['name'] = name
        if servername is not None:
            npc['servername'] = servername
        if model is not None:
            npc['model_id'] = model
        npcs.append(npc)
    return ('truncated' if truncated else 'observed'), npcs, truncated


def _teleport_probe_label(value, limit=64):
    if not isinstance(value, str):
        return None
    text = value.strip()
    if not text or ',' in text or '\n' in text or '\r' in text:
        return None
    return _bounded_text(text, limit)


def _teleport_route_cache_key(server, session_id, source_label, gate_key):
    return (
        (server or '').strip().lower(),
        (session_id or '').strip().lower(),
        source_label.lower(),
        gate_key,
    )


def _teleport_routes_for_gate(source_label, gate_servername, adapter, server='', session_id=''):
    """Discover live routes for one gate via bounded get_teleport_data probes."""
    if adapter is None or not adapter.has('get_teleport_data'):
        return []
    source_label = _teleport_probe_label(source_label)
    gate_key = gate_servername.strip().lower() if isinstance(gate_servername, str) else ''
    if not source_label:
        return []
    cache_key = _teleport_route_cache_key(server, session_id, source_label, gate_key)
    now = _monotonic()
    cached = _teleport_route_cache.get(cache_key)
    if cached and cached[0] > now:
        return list(cached[1])
    routes = []
    attempts = 0
    for candidate in _TELEPORT_DESTINATION_CANDIDATES:
        if attempts >= _MAX_TELEPORT_ROUTE_PROBE_CALLS or len(routes) >= _MAX_TELEPORT_ROUTES_PER_GATE:
            break
        destination_label = _teleport_probe_label(candidate)
        if not destination_label or destination_label.lower() == source_label.lower():
            continue
        attempts += 1
        try:
            observed = adapter.call('get_teleport_data', source_label, destination_label)
        except Exception:
            continue
        classification = _classify_teleport_data_result(observed)
        if classification.get('result') != 'tuple':
            continue
        entry = {'destination': destination_label}
        code = classification.get('code')
        if isinstance(code, int) and not isinstance(code, bool):
            entry['teleport_code'] = code
        routes.append(entry)
    routes.sort(key=lambda item: item['destination'].lower())
    _teleport_route_cache[cache_key] = (now + _TELEPORT_ROUTE_CACHE_TTL_SECONDS, tuple(routes))
    return list(routes)


def _attach_teleporter_routes(npc_list, adapter, server='', session_id=''):
    if not isinstance(npc_list, list) or adapter is None:
        return
    for npc in npc_list:
        if not isinstance(npc, dict) or npc.get('role') != 'teleporter':
            continue
        source = npc.get('name') or npc.get('servername')
        routes = _teleport_routes_for_gate(
            source, npc.get('servername'), adapter, server, session_id,
        )
        if routes:
            npc['teleport_routes'] = routes


def _classify_teleport_data_result(value):
    if value is None:
        return {'result': 'none'}
    if isinstance(value, tuple):
        elements = []
        for item in value:
            if isinstance(item, bool):
                elements.append({'type': 'bool'})
            elif isinstance(item, int):
                elements.append({'type': 'int', 'value': item})
            elif isinstance(item, float):
                elements.append({'type': 'float'})
            elif isinstance(item, str):
                bounded = _teleport_probe_label(item)
                elements.append({'type': 'str', 'length': len(bounded or '')})
            else:
                elements.append({'type': type(item).__name__})
        code = None
        if len(value) > 1 and isinstance(value[1], int) and not isinstance(value[1], bool):
            code = value[1]
        return {'result': 'tuple', 'length': len(value), 'elements': elements, 'code': code}
    return {'result': 'unexpected', 'type': type(value).__name__}


def _discover_teleport_probe_symbols(api_module):
    symbols = []
    if api_module is None:
        return symbols
    try:
        names = sorted(set(dir(api_module)))
    except Exception:
        return symbols
    for name in names:
        if name.startswith('_') or not _TELEPORT_PROBE_SYMBOL_RE.search(name):
            continue
        try:
            attr = getattr(api_module, name, None)
        except Exception:
            continue
        symbols.append({'name': name, 'callable': callable(attr)})
    return symbols


def _teleport_probe_gate_rows(npcs):
    gates = []
    if not isinstance(npcs, list):
        return gates
    for row in npcs:
        if not isinstance(row, dict) or row.get('role') != 'teleporter':
            continue
        gates.append({
            'id': row.get('id'),
            'name': row.get('name'),
            'servername': row.get('servername'),
        })
    return gates


def _teleport_probe_pair_plan(gates):
    pairs = []
    seen = set()

    def add(source, destination, gate_id=None, tag=None):
        source_label = _teleport_probe_label(source) if isinstance(source, str) else None
        destination_label = _teleport_probe_label(destination) if isinstance(destination, str) else None
        if destination == _TELEPORT_PROBE_UNKNOWN_DEST:
            destination_label = _TELEPORT_PROBE_UNKNOWN_DEST
        if source_label is None or destination_label is None:
            return
        key = (source_label, destination_label)
        if key in seen:
            return
        seen.add(key)
        entry = {
            'source': source_label,
            'destination': destination_label,
            'gate_id': gate_id,
        }
        if tag:
            entry['tag'] = tag
        pairs.append(entry)

    if gates:
        first = gates[0]
        control_source = first.get('name') or first.get('servername')
        add(control_source, _TELEPORT_PROBE_UNKNOWN_DEST, first.get('id'))
        gate_id = first.get('id')
        for source, destination, tag in _TELEPORT_PROBE_REFERENCE_PAIRS:
            add(source, destination, gate_id, tag=tag)
            if len(pairs) >= MAX_TELEPORT_PROBE_PAIR_CALLS:
                return pairs[:MAX_TELEPORT_PROBE_PAIR_CALLS]

    labels = []
    for gate in gates:
        for label in (gate.get('name'), gate.get('servername')):
            bounded = _teleport_probe_label(label) if isinstance(label, str) else None
            if bounded and bounded not in labels:
                labels.append(bounded)
    for index, source in enumerate(labels):
        for destination in labels[index + 1:]:
            add(source, destination)
            if len(pairs) >= MAX_TELEPORT_PROBE_PAIR_CALLS:
                return pairs[:MAX_TELEPORT_PROBE_PAIR_CALLS]
    for gate in gates:
        for label in (gate.get('name'), gate.get('servername')):
            add(label, _TELEPORT_PROBE_UNKNOWN_DEST, gate.get('id'))
            if len(pairs) >= MAX_TELEPORT_PROBE_PAIR_CALLS:
                return pairs[:MAX_TELEPORT_PROBE_PAIR_CALLS]
    return pairs[:MAX_TELEPORT_PROBE_PAIR_CALLS]


def probe_teleporter_capabilities(api=None, npcs=None, api_module=None):
    """Read-only teleporter investigation probe. Never injects packets or starts scripts."""
    result = {
        'status': 'ok',
        'plugin_version': pVersion,
        'npc_observation': 'unknown',
        'symbols': [],
        'gates': [],
        'pair_tests': [],
        'capabilities': {
            'get_npcs': False,
            'get_teleport_data': False,
            'start_script': False,
        },
        'enumeration': 'unsupported',
        'recall': 'unsupported',
        'execution': 'documented_script_command_unverified',
        'errors': [],
    }
    module = api_module
    if module is None and api is None:
        try:
            import phBot as module
        except Exception:
            module = None

    if isinstance(api, dict):
        get_npcs = api.get('get_npcs')
        get_teleport_data = api.get('get_teleport_data')
        start_script = api.get('start_script')
    else:
        get_npcs = _optional_phbot_api('get_npcs')
        get_teleport_data = _optional_phbot_api('get_teleport_data')
        start_script = _optional_phbot_api('start_script')

    result['capabilities']['get_npcs'] = callable(get_npcs)
    result['capabilities']['get_teleport_data'] = callable(get_teleport_data)
    result['capabilities']['start_script'] = callable(start_script)
    result['symbols'] = _discover_teleport_probe_symbols(module)

    if npcs is None and callable(get_npcs):
        npc_status, collected, _ = collect_npc_observation({'get_npcs': get_npcs})
        result['npc_observation'] = npc_status
        npcs = collected
    elif isinstance(npcs, list):
        result['npc_observation'] = 'provided'
    else:
        result['npc_observation'] = 'unavailable'
        npcs = []

    gates = _teleport_probe_gate_rows(npcs)
    result['gates'] = gates

    if not callable(get_teleport_data):
        result['status'] = 'unavailable'
        result['errors'].append('get_teleport_data_missing')
        return result

    pair_plan = _teleport_probe_pair_plan(gates)
    for planned in pair_plan:
        entry = {
            'source': planned['source'],
            'destination': planned['destination'],
            'gate_id': planned.get('gate_id'),
        }
        if planned.get('tag'):
            entry['tag'] = planned['tag']
        try:
            observed = get_teleport_data(planned['source'], planned['destination'])
        except Exception as error:
            entry['classification'] = {'result': 'error', 'type': error.__class__.__name__}
            result['pair_tests'].append(entry)
            continue
        entry['classification'] = _classify_teleport_data_result(observed)
        result['pair_tests'].append(entry)

    if len(result['pair_tests']) > MAX_TELEPORT_PROBE_PAIR_CALLS:
        result['errors'].append('pair_test_overflow')
        result['pair_tests'] = result['pair_tests'][:MAX_TELEPORT_PROBE_PAIR_CALLS]
    for test in result['pair_tests']:
        if test.get('tag') == 'hotan_to_jangan' and test.get('classification', {}).get('result') == 'tuple':
            result['execution'] = 'documented_script_command_verified_hotan_jangan'
            break
    return result


def summarize_teleport_probe(result):
    if not isinstance(result, dict):
        return 'Teleporter probe failed.'
    gates = len(result.get('gates') or [])
    pairs = len(result.get('pair_tests') or [])
    caps = result.get('capabilities') or {}
    teleport_api = 'yes' if caps.get('get_teleport_data') else 'no'
    return (
        'Teleporter probe: {gates} gate(s), get_teleport_data={api}, '
        '{pairs} pair test(s), enumeration unsupported.'
    ).format(gates=gates, api=teleport_api, pairs=pairs)


def _teleporter_source_matches(row, source_label):
    if not source_label or not isinstance(row, dict):
        return False
    name = row.get('name')
    servername = row.get('servername')
    if isinstance(name, str) and name == source_label:
        return True
    return isinstance(servername, str) and servername == source_label


def _session_teleporter_gate(npcs, gate_servername, source):
    if not isinstance(npcs, list):
        return None
    gate_servername = gate_servername.strip() if isinstance(gate_servername, str) else ''
    source_label = _teleport_probe_label(source) if isinstance(source, str) else None
    if not gate_servername or not source_label or not _NPC_GATE_ROLE.fullmatch(gate_servername):
        return None
    for row in npcs:
        if not isinstance(row, dict) or row.get('role') != 'teleporter':
            continue
        if row.get('servername') != gate_servername:
            continue
        if _teleporter_source_matches(row, source_label):
            return row
    return None


def _session_recall_point_gate(npcs, args):
    """Resolve one requested gate in this character's current NPC snapshot.

    The map combines other characters' observations; its runtime NPC id must
    never be reused here. A duplicate match is ambiguous and cannot be acted on.
    """
    if not isinstance(npcs, list) or not isinstance(args, dict):
        return None
    if set(args) not in (set(('gate_servername', 'region', 'x', 'y')),
                         set(('gate_servername', 'region', 'x', 'y', 'model_id'))):
        return None
    servername = args.get('gate_servername')
    region = args.get('region')
    x, y = args.get('x'), args.get('y')
    model = args.get('model_id')
    if (not isinstance(servername, str) or not _NPC_GATE_ROLE.fullmatch(servername) or
            not _valid_position_region(region) or not _number(x) or not _number(y) or
            abs(x) > 10000000 or abs(y) > 10000000 or
            (model is not None and (not isinstance(model, int) or isinstance(model, bool) or
                                    model <= 0 or model > 4294967295))):
        return None
    matches = []
    for row in npcs:
        if not isinstance(row, dict) or row.get('role') != 'teleporter':
            continue
        if row.get('servername') != servername or row.get('region') != region:
            continue
        if model is not None and row.get('model_id') != model:
            continue
        rx, ry = row.get('x'), row.get('y')
        if not _number(rx) or not _number(ry):
            continue
        if (float(rx) - float(x)) ** 2 + (float(ry) - float(y)) ** 2 > 64.0:
            continue
        identifier = row.get('id')
        if not isinstance(identifier, str) or not re.match(r'^[0-9]+$', identifier):
            continue
        npc_id = int(identifier)
        if not 0 < npc_id <= 4294967295:
            continue
        matches.append((row, npc_id))
        if len(matches) > 1:
            return None
    return matches[0] if matches else None


def _recall_point_support(phbot_version, identity, adapter):
    """Limit packet submission to the build and server observed in the manual capture."""
    if str(phbot_version).strip() not in ('20.1.3', '20.1.3.0'):
        return False, 'recall_point_build_unverified'
    if not isinstance(identity, dict) or identity.get('server') != 'Greatest':
        return False, 'recall_point_server_unverified'
    if not adapter.has('get_npcs') or not adapter.has('inject_joymax'):
        return False, 'unsupported_runtime_primitive'
    return True, ''


def _hotan_gate_row(npcs):
    for row in _teleport_probe_gate_rows(npcs):
        name = row.get('name')
        servername = row.get('servername')
        if isinstance(name, str) and name.lower() == 'hotan':
            return row
        if servername == 'GATE_KT':
            return row
    return None


def test_teleport_hotan_jangan():
    """Operator-only Hotan→Jangan check: get_teleport_data then one teleport script line."""
    if not _PHBOT_AVAILABLE:
        _set_gui_status('Hotan→Jangan test requires the phBot runtime.')
        return
    get_teleport_data = _optional_phbot_api('get_teleport_data')
    start_script = _optional_phbot_api('start_script')
    if not callable(get_teleport_data) or not callable(start_script):
        _set_gui_status('Hotan→Jangan test needs get_teleport_data and start_script.')
        return
    _, npcs, _ = collect_npc_observation()
    if _hotan_gate_row(npcs) is None:
        _set_gui_status('Hotan gate (GATE_KT) not in current get_npcs() snapshot.')
        return
    chosen = None
    for source, destination in _HOTAN_JANGAN_SCRIPT_PAIRS:
        try:
            observed = get_teleport_data(source, destination)
        except Exception as error:
            _log('PhMon Hotan→Jangan test get_teleport_data error: ' + error.__class__.__name__)
            continue
        if observed is not None and isinstance(observed, tuple):
            chosen = (source, destination, observed)
            break
    if chosen is None:
        _log('PhMon Hotan→Jangan test: get_teleport_data returned none for all pairs')
        _set_gui_status('No Hotan→Jangan route from get_teleport_data.')
        return
    source, destination, observed = chosen
    line = 'teleport,{0},{1}'.format(source, destination)
    try:
        started = start_script(line)
    except Exception as error:
        _set_gui_status('Hotan→Jangan start_script failed: ' + error.__class__.__name__)
        return
    code = observed[1] if len(observed) > 1 else None
    _log(
        'PhMon Hotan→Jangan test line=' + line
        + ' code=' + str(code)
        + ' start_script=' + str(started)
    )
    _set_gui_status(
        'Hotan→Jangan test: ' + line + ' (start_script=' + str(started) + ')'
    )


def probe_teleporters():
    """Operator-triggered read-only teleporter capability probe."""
    if not _PHBOT_AVAILABLE:
        _set_gui_status('Teleporter probe requires the phBot runtime.')
        return
    result = probe_teleporter_capabilities()
    summary = summarize_teleport_probe(result)
    try:
        payload = json.dumps(result, sort_keys=True, separators=(',', ':'))
    except Exception:
        payload = summary
    if len(payload) > 4000:
        payload = payload[:4000] + '...'
    _log('PhMon teleporter probe ' + payload)
    _set_gui_status(summary)


def _npc_snapshot_signature(status, region, npcs):
    rows = []
    for npc in npcs:
        route_sig = ()
        routes = npc.get('teleport_routes')
        if isinstance(routes, list):
            route_sig = tuple(
                (item.get('destination'), item.get('teleport_code'))
                for item in routes
                if isinstance(item, dict) and item.get('destination')
            )
        rows.append((
            npc.get('id'), npc.get('name'), npc.get('servername'), npc.get('model_id'),
            npc.get('role'), npc.get('region'), npc.get('x'), npc.get('y'), route_sig,
        ))
    rows.sort()
    return (status, region, tuple(rows))


def _cached_player_zone(region, cache):
    if region not in cache:
        cache[region] = _zone_name_for_region(region)
    return cache[region]


def _zone_name_for_region(region, limit=80):
    if not _valid_position_region(region) or not callable(_get_zone_name):
        return None
    try:
        value = _get_zone_name(region)
    except Exception:
        return None
    return _bounded_text(value, limit) if isinstance(value, str) else None


def _inventory_model_quantity(model):
    getter = _optional_phbot_api('get_inventory')
    if not callable(getter):
        return None
    try:
        inventory = getter()
    except Exception:
        return None
    if not isinstance(inventory, dict) or not isinstance(inventory.get('items'), list):
        return None
    items = inventory['items']
    start = 13 if len(items) >= 13 else 0
    total = 0
    for item in items[start:start + MAX_RESOURCE_SLOTS]:
        if not isinstance(item, dict) or item.get('model') != model:
            continue
        quantity = item.get('quantity', 1)
        total += quantity if isinstance(quantity, int) and not isinstance(quantity, bool) and quantity > 0 else 1
    return total


def _normalize_item(value):
    if value is None:
        return None
    if not isinstance(value, dict):
        return None
    item = {}
    for key in ('model', 'quantity', 'plus', 'durability'):
        field = value.get(key)
        if isinstance(field, int) and not isinstance(field, bool) and field >= 0:
            item[key] = field
        elif isinstance(field, float) and _number(field) and field >= 0:
            item[key] = field
    for key in ('name', 'servername'):
        field = _bounded_text(value.get(key))
        if field is not None:
            item[key] = field
    # Retain bounded source evidence without treating undocumented API fields as
    # interpreted stats. In particular, never send 64-bit rolls as JSON numbers
    # that a browser would round. Unknown semantics stay in api_fields.
    evidence = {}
    for key in ITEM_API_EVIDENCE_FIELDS:
        if key in value:
            field = _bounded_item_evidence(value[key])
            if field is not None:
                evidence[key] = field
    if evidence and len(json.dumps(evidence, separators=(',', ':')).encode('utf-8')) <= 8192:
        item['api_fields'] = evidence
    # Structural diagnostics expose missing field aliases without sending unknown
    # values. The backend must verify semantics before producing presentation.
    if item:
        item['api_evidence_version'] = ITEM_API_EVIDENCE_VERSION
        item['api_field_types'] = _item_api_field_types(value)
    return item or None


def _item_api_field_types(item):
    result = {}
    for key, value in itertools.islice(item.items(), 64):
        if not isinstance(key, str) or len(key) > 64 or not key.isidentifier():
            continue
        if any(word in key.lower() for word in ('secret', 'token', 'password', 'credential', 'auth', 'account')):
            continue
        kind = _item_evidence_type(value)
        shape = {'type': kind}
        if kind in ('dict', 'list', 'string'):
            shape['count'] = len(value)
        if kind == 'dict':
            shape['key_types'] = sorted(set(
                _item_evidence_type(k) for k in itertools.islice(value, 32)))
        result[key] = shape
        if len(json.dumps(result, separators=(',', ':')).encode('utf-8')) > 2048:
            del result[key]
            break
    return result


def _item_evidence_type(value):
    if value is None:
        return 'null'
    for kind, label in ((bool, 'boolean'), (int, 'integer'), (float, 'number'),
                        (str, 'string'), ((list, tuple), 'list'), (dict, 'dict')):
        if isinstance(value, kind):
            return label
    return 'unsupported'


def _bounded_item_evidence(value, depth=0):
    if depth > 3:
        return None
    if isinstance(value, bool) or value is None:
        return value
    if isinstance(value, int):
        return str(value) if abs(value) > 9007199254740991 else value
    if isinstance(value, float):
        return value if _number(value) else None
    if isinstance(value, str):
        return value[:256]
    if isinstance(value, (list, tuple)):
        return [_bounded_item_evidence(entry, depth + 1) for entry in value[:32]]
    if isinstance(value, dict):
        entries = list(itertools.islice(value.items(), 32))
        if any(isinstance(key, int) and not isinstance(key, bool) for key, _ in entries):
            # Integer option/attribute IDs are valid API evidence. Tagged entries
            # also preserve order and distinguish 9 from '9' without JSON key loss.
            pairs = []
            for key, entry in entries:
                kind = _item_evidence_type(key)
                if kind == 'integer' and -(2**63) <= key <= 2**64 - 1:
                    text = str(key)
                elif kind == 'string' and len(key) <= 64:
                    text = key
                else:
                    continue
                pairs.append({'key_type': kind, 'key': text,
                              'value': _bounded_item_evidence(entry, depth + 1)})
            return {'mapping_entries': pairs}
        return {key: _bounded_item_evidence(entry, depth + 1)
                for key, entry in entries
                if isinstance(key, str) and len(key) <= 64}
    return None


def _normalize_slots(items, capacity, source_offset=0, display_offset=0, maximum=MAX_RESOURCE_SLOTS):
    if not isinstance(items, (list, tuple)) or not isinstance(capacity, int) or isinstance(capacity, bool) or capacity < 0:
        return None
    capacity = min(capacity, maximum)
    slots = []
    for display_slot in range(capacity):
        source_slot = display_slot + source_offset
        item = _normalize_item(items[source_slot] if source_slot < len(items) else None)
        if item is None:
            slots.append(None)
        else:
            slots.append({
                'source_slot': source_slot,
                'displayed_slot': display_slot + display_offset,
                'item': item,
            })
    return slots


def _normalize_container(raw, resource_key):
    if not isinstance(raw, dict):
        availability = 'not_observed' if resource_key in ('storage', 'guild_storage', 'job_pouch') else 'unavailable'
        return {'availability': availability, 'reason': 'getter_unavailable_or_not_open'}
    items = raw.get('items')
    if not isinstance(items, (list, tuple)):
        return {'availability': 'unavailable', 'reason': 'invalid_api_shape'}
    raw_size = raw.get('size')
    if not isinstance(raw_size, int) or isinstance(raw_size, bool) or raw_size < 0:
        return {'availability': 'unavailable', 'reason': 'capacity_unavailable'}
    if raw_size > MAX_RESOURCE_SLOTS + 13:
        return {'availability': 'unavailable', 'reason': 'capacity_exceeds_limit'}
    if len(items) > raw_size:
        return {'availability': 'unavailable', 'reason': 'item_list_exceeds_reported_capacity'}
    slots = _normalize_slots(items, raw_size)
    if slots is None:
        return {'availability': 'unavailable', 'reason': 'invalid_api_shape'}
    used = sum(1 for entry in slots if entry is not None)
    return {
        'availability': 'observed',
        'capacity': raw_size,
        'used_slots': used,
        'slots': slots,
    }


def collect_resource_inputs(api=None):
    """Call phBot getters on its callback thread; return raw values for worker normalization."""
    inputs = {}
    for name in ('get_inventory', 'get_storage', 'get_guild_storage', 'get_job_pouch', 'get_pets', 'get_party', 'get_academy'):
        function = (api or {}).get(name) if isinstance(api, dict) else None
        if not callable(function):
            function = _optional_phbot_api(name)
        if not callable(function):
            inputs[name] = {'available': False, 'value': None}
            continue
        try:
            inputs[name] = {'available': True, 'value': function()}
        except Exception:
            inputs[name] = {'available': True, 'value': None}
    return inputs


def _reset_item_type_cache():
    with _ITEM_TYPE_CACHE_LOCK:
        _ITEM_TYPE_CACHE.clear()


def _cached_item_sort_type(server, model):
    if (not isinstance(server, str) or not server.strip() or
            not isinstance(model, int) or isinstance(model, bool) or model <= 0):
        return None
    key = (server.strip().lower(), model)
    with _ITEM_TYPE_CACHE_LOCK:
        if key not in _ITEM_TYPE_CACHE:
            return _ITEM_TYPE_CACHE_MISS
        value = _ITEM_TYPE_CACHE[key]
        _ITEM_TYPE_CACHE.move_to_end(key)
        return value


def _item_sort_type(server, model):
    """Resolve the two filtered item families using phBot's documented game data."""
    cached = _cached_item_sort_type(server, model)
    valid_identity = (
        isinstance(server, str) and bool(server.strip()) and
        isinstance(model, int) and not isinstance(model, bool)
    )
    key = (server.strip().lower(), model) if valid_identity else None
    if cached is not _ITEM_TYPE_CACHE_MISS:
        return cached if isinstance(cached, str) else None
    if key is None or model <= 0:
        return None
    getter = _optional_phbot_api('get_item')
    if not callable(getter):
        return None
    try:
        definition = getter(model)
    except Exception:
        return None
    if not isinstance(definition, dict):
        return None
    tid1 = definition.get('tid1')
    tid2 = definition.get('tid2')
    if (not isinstance(tid1, int) or isinstance(tid1, bool) or
            not isinstance(tid2, int) or isinstance(tid2, bool)):
        return None
    sort_type = None
    if tid1 == 3 and tid2 == 1:
        sort_type = 'Potion'
    elif tid1 == 3 and tid2 == 2:
        sort_type = 'Pill'
    with _ITEM_TYPE_CACHE_LOCK:
        _ITEM_TYPE_CACHE[key] = sort_type if sort_type is not None else _ITEM_TYPE_OTHER
        _ITEM_TYPE_CACHE.move_to_end(key)
        while len(_ITEM_TYPE_CACHE) > MAX_ITEM_TYPE_CACHE_ENTRIES:
            _ITEM_TYPE_CACHE.popitem(last=False)
    return sort_type


def _cache_resource_item_types(inputs, server):
    """Resolve item families on phBot's callback thread; never send this map."""
    if not _item_filters_enabled(_profile_config or {}):
        return
    models = set()

    def add_items(value):
        if not isinstance(value, dict) or len(models) >= MAX_RESOURCE_SLOTS:
            return
        items = value.get('items')
        if not isinstance(items, (list, tuple)):
            return
        for item in items:
            if not isinstance(item, dict):
                continue
            model = item.get('model')
            if isinstance(model, int) and not isinstance(model, bool) and model > 0:
                models.add(model)
                if len(models) >= MAX_RESOURCE_SLOTS:
                    return

    for api_name in ('get_inventory', 'get_storage', 'get_guild_storage', 'get_job_pouch'):
        result = inputs.get(api_name) if isinstance(inputs, dict) else None
        if isinstance(result, dict) and result.get('available') is True:
            add_items(result.get('value'))
    pets_result = inputs.get('get_pets') if isinstance(inputs, dict) else None
    pets = pets_result.get('value') if isinstance(pets_result, dict) and pets_result.get('available') is True else None
    if isinstance(pets, dict):
        for pet in pets.values():
            add_items(pet)
            if len(models) >= MAX_RESOURCE_SLOTS:
                break
    for model in models:
        _item_sort_type(server, model)


def _ignore_item_event(config, sort_type):
    setting = _ITEM_TYPE_FILTERS.get(sort_type)
    if setting is None:
        return False
    return config.get(setting, True) is True


def _item_filters_enabled(config):
    return any(config.get(setting, True) is True for setting in _ITEM_TYPE_FILTERS.values())


def normalize_resource_inputs(inputs, config_dir=None, server=None, locale=None):
    """Normalize bounded API data off the callback thread; never mutate game state."""
    def call(name):
        result = inputs.get(name) if isinstance(inputs, dict) else None
        if not isinstance(result, dict) or not isinstance(result.get('available'), bool):
            return False, None
        return result['available'], result.get('value')

    resources = {}
    available, raw = call('get_inventory')
    inventory = _normalize_container(raw, 'inventory') if available else {'availability': 'unavailable', 'reason': 'api_missing'}
    if inventory.get('availability') == 'observed':
        raw_items = raw.get('items', [])
        equipment_count = min(13, len(raw_items))
        # The 13 equipment-slot boundary is based on the supplied adapter and
        # remains marked as runtime-unverified in docs/phbot-capabilities.md.
        equipment_capacity = min(13, raw.get('size', 0))
        bag_capacity = max(0, raw.get('size', 0) - equipment_capacity)
        resources['equipment'] = {
            'availability': 'observed',
            'capacity': equipment_capacity,
            'used_slots': sum(1 for value in raw_items[:equipment_capacity] if _normalize_item(value) is not None),
            'slots': _normalize_slots(raw_items, equipment_capacity),
            'mapping_evidence': 'adapter_lead_runtime_unverified',
        }
        resources['inventory'] = {
            'availability': 'observed',
            'capacity': bag_capacity,
            'used_slots': sum(1 for value in raw_items[equipment_capacity:equipment_capacity + bag_capacity] if _normalize_item(value) is not None),
            'slots': _normalize_slots(raw_items, bag_capacity, source_offset=equipment_capacity),
            'gold': raw.get('gold') if isinstance(raw.get('gold'), int) and not isinstance(raw.get('gold'), bool) and raw.get('gold') >= 0 else None,
            'mapping_evidence': 'adapter_lead_runtime_unverified',
        }
    else:
        resources['inventory'] = inventory
        resources['equipment'] = dict(inventory)

    for resource_key, api_name in (('storage', 'get_storage'), ('guild_storage', 'get_guild_storage'), ('job_pouch', 'get_job_pouch')):
        available, raw = call(api_name)
        resource = _normalize_container(raw, resource_key) if available else {'availability': 'unavailable', 'reason': 'api_missing'}
        if resource_key == 'guild_storage' and resource.get('availability') == 'observed':
            gold = raw.get('gold') if isinstance(raw, dict) else None
            if isinstance(gold, int) and not isinstance(gold, bool) and gold >= 0:
                resource['gold'] = gold
        resources[resource_key] = resource

    available, raw_pets = call('get_pets')
    if not available:
        resources['pets'] = {'availability': 'unavailable', 'reason': 'api_missing', 'pets': []}
    elif raw_pets is None:
        resources['pets'] = {'availability': 'unavailable', 'reason': 'getter_returned_none', 'pets': []}
    elif isinstance(raw_pets, dict):
        pets = []
        for pet_index, (pet_id, raw_pet) in enumerate(raw_pets.items()):
            if pet_index >= 32:
                break
            if not isinstance(raw_pet, dict):
                continue
            inventory_available = isinstance(raw_pet.get('items'), (list, tuple))
            raw_items = raw_pet.get('items') if inventory_available else []
            pet = {'pet_id': _bounded_text(pet_id, 64), 'inventory_available': inventory_available}
            if inventory_available:
                pet['slots'] = _normalize_slots(raw_items, min(len(raw_items), MAX_RESOURCE_SLOTS))
            for key in ('name', 'servername', 'type'):
                value = _bounded_text(raw_pet.get(key), 100)
                if value is not None:
                    pet[key] = value
            for key in ('model', 'hp'):
                value = raw_pet.get(key)
                if isinstance(value, int) and not isinstance(value, bool) and value >= 0:
                    pet[key] = value
            if isinstance(raw_pet.get('mounted'), bool):
                pet['mounted'] = raw_pet['mounted']
            pets.append(pet)
        resources['pets'] = {'availability': 'observed', 'pets': pets}
    else:
        resources['pets'] = {'availability': 'unavailable', 'reason': 'invalid_api_shape', 'pets': []}

    available, raw_party = call('get_party')
    if not available or raw_party is None:
        resources['party'] = {'availability': 'unavailable' if not available else 'not_observed', 'members': []}
    elif isinstance(raw_party, dict):
        members = []
        for member_index, (party_id, entry) in enumerate(raw_party.items()):
            if member_index >= 32:
                break
            if not isinstance(entry, dict):
                continue
            member = {'party_id': _bounded_text(party_id, 64)}
            for key in ('name', 'guild'):
                value = _bounded_text(entry.get(key), 100)
                if value is not None:
                    member[key] = value
            for key in ('player_id', 'level'):
                value = entry.get(key)
                if isinstance(value, int) and not isinstance(value, bool) and value >= 0:
                    member[key] = value
            for axis in ('x', 'y'):
                value = entry.get(axis)
                if _number(value) and abs(value) <= 1000000:
                    member[axis] = float(value)
            for key in ('hp_percent', 'mp_percent'):
                value = entry.get(key)
                if isinstance(value, int) and not isinstance(value, bool) and 0 <= value <= 10:
                    member[key] = value * 10
            members.append(member)
        resources['party'] = {'availability': 'observed', 'members': members}
    else:
        resources['party'] = {'availability': 'unavailable', 'members': []}

    available, academy = call('get_academy')
    resources['academy'] = _normalize_academy(academy, available)
    resources['party_setup'] = {
        'availability': 'unavailable',
        'mode': 'read_only_unverified',
        'reason': 'configuration_reload_contract_not_verified',
    }
    protocol = detect_item_protocol(config_dir, server, locale)
    resources['item_enrichment'] = {
        'availability': 'unavailable',
        'protocol': protocol,
        'reason': 'passive_item_packet_decoder_not_enabled',
    }
    return resources


def collect_resources(api=None, config_dir=None, server=None, locale=None):
    """Synchronous fixture helper; production normalizes getter values on the worker."""
    return normalize_resource_inputs(collect_resource_inputs(api), config_dir, server, locale)


def _normalize_academy(value, api_available):
    if not api_available:
        return {'availability': 'unavailable', 'reason': 'api_missing'}
    if value is None:
        return {'availability': 'unavailable', 'reason': 'getter_returned_none'}
    if not isinstance(value, dict):
        return {'availability': 'unavailable', 'reason': 'invalid_api_shape'}
    members = []
    for member_id, raw_member in value.items():
        if member_id == 'id':
            continue
        if len(members) >= 32:
            break
        if not isinstance(raw_member, dict):
            continue
        member = {'member_id': _bounded_text(member_id, 64)}
        for key in ('name',):
            text = _bounded_text(raw_member.get(key), 100)
            if text is not None:
                member[key] = text
        for key in ('online', 'type', 'level'):
            field = raw_member.get(key)
            if isinstance(field, int) and not isinstance(field, bool) and field >= 0:
                member[key] = field
        for axis in ('x', 'y'):
            field = raw_member.get(axis)
            if _number(field):
                member[axis] = field
        members.append(member)
    academy_id = value.get('id')
    payload = {'members': members}
    if isinstance(academy_id, int) and not isinstance(academy_id, bool) and academy_id >= 0:
        payload['id'] = academy_id
    return {'availability': 'observed', 'value': payload}


def _event_resource_snapshot(resources):
    if not isinstance(resources, dict):
        return None
    pets_resource = resources.get('pets')

    def membership(resource, collection_path):
        if not isinstance(resource, dict) or resource.get('availability') != 'observed':
            return None
        value = resource
        for part in collection_path:
            value = value.get(part) if isinstance(value, dict) else None
        if not isinstance(value, list):
            return None
        result = {}
        for entry in value[:256]:
            if not isinstance(entry, dict):
                continue
            member_id = entry.get('party_id') or entry.get('member_id') or entry.get('name')
            if member_id is not None and str(member_id).strip():
                result[str(member_id)[:100]] = dict(entry)
        return result

    party = membership(resources.get('party'), ('members',))
    academy = membership(resources.get('academy'), ('value', 'members'))
    pets = None
    if isinstance(pets_resource, dict) and pets_resource.get('availability') == 'observed':
        pets = {}
        for pet in pets_resource.get('pets', [])[:32]:
            if not isinstance(pet, dict) or not pet.get('pet_id'):
                continue
            pets[str(pet['pet_id'])[:64]] = {key: value for key, value in pet.items() if key != 'slots'}

    containers = {}
    for key in ('equipment', 'inventory', 'storage', 'job_pouch'):
        container = resources.get(key)
        if not isinstance(container, dict) or container.get('availability') != 'observed':
            continue
        containers[key] = _event_item_quantities(container.get('slots', []))
    for pet in (pets_resource.get('pets', [])[:32] if pets is not None else []):
        if not isinstance(pet, dict) or not pet.get('pet_id') or not pet.get('inventory_available'):
            continue
        pet_key = 'pets:' + str(pet['pet_id'])[:64]
        containers[pet_key] = _event_item_quantities(pet.get('slots', []))
    return {'party': party, 'academy': academy, 'pets': pets, 'containers': containers}


def _event_item_quantities(slots):
    result = {}
    if not isinstance(slots, list):
        return result
    for row in slots[:MAX_RESOURCE_SLOTS]:
        if not isinstance(row, dict) or not isinstance(row.get('item'), dict):
            continue
        item = dict(row['item'])
        code = item.get('servername')
        model = item.get('model')
        if isinstance(code, str) and code:
            signature = 'code:' + code[:128]
        elif isinstance(model, int) and not isinstance(model, bool) and model >= 0:
            signature = 'model:' + str(model)
        else:
            continue
        quantity = item.get('quantity', 1)
        if not _number(quantity) or quantity < 0:
            quantity = 1
        slot = row.get('source_slot')
        record = result.setdefault(signature, {'quantity': 0, 'item': item, 'slots': [],
                                               'slot_items': {}})
        record['quantity'] += quantity
        if isinstance(slot, int) and not isinstance(slot, bool) and slot >= 0:
            record['slots'].append(slot)
            record['slot_items'][slot] = item
    return result


def _event_gain_observation(before_record, after_record, quantity):
    """Identify one gained instance without borrowing another copy's rolls."""
    if quantity != 1:
        return None
    previous = list((before_record or {}).get('slot_items', {}).values())
    unmatched = []
    for slot, item in after_record.get('slot_items', {}).items():
        try:
            index = previous.index(item)
        except ValueError:
            unmatched.append((slot, item))
        else:
            previous.pop(index)
    return unmatched[0] if len(unmatched) == 1 else None


def _event_gain_item(before_record, after_record, quantity):
    selected = _event_gain_observation(before_record, after_record, quantity)
    if selected:
        return selected[1], selected, False
    item = after_record.get('item') or {}
    if len(after_record.get('slot_items', {})) <= 1:
        return item, None, False
    # Multiple copies changed without one provable instance. Preserve only the
    # item identity, never the first copy's unrelated plus/whites/blues.
    identity = {key: item[key] for key in ('model', 'servername', 'name') if key in item}
    identity['quantity'] = quantity
    return identity, None, True


def _item_has_identity(item):
    return isinstance(item, dict) and (
        isinstance(item.get('servername'), str) and bool(item.get('servername')) or
        isinstance(item.get('model'), int) and not isinstance(item.get('model'), bool) and item.get('model') >= 0
    )


def _container_reference(key, record):
    reference = {'type': key.split(':', 1)[0]}
    if ':' in key:
        reference['id'] = key.split(':', 1)[1]
    slots = record.get('slots') if isinstance(record, dict) else None
    if isinstance(slots, list) and len(slots) == 1:
        reference['slot'] = slots[0]
    return reference


def detect_item_protocol(config_dir, server, locale):
    return detect_item_protocol_detail(config_dir, server, locale)[0]


def detect_item_protocol_detail(config_dir, server, locale):
    """Resolve vSRO variant from phBot's active installation config, never numeric version."""
    if locale != 22:
        return 'unknown', 'not_vsro_locale'
    if not isinstance(config_dir, str) or not config_dir.strip():
        return 'unknown', 'config_directory_unavailable'
    if not isinstance(server, str) or not server.strip():
        return 'unknown', 'server_unavailable'

    normalized_dir = os.path.normpath(config_dir)
    candidates = [os.path.join(normalized_dir, 'vSRO.json')]
    parent_config = os.path.dirname(normalized_dir)
    if parent_config and parent_config != normalized_dir:
        candidates.insert(0, os.path.join(parent_config, 'vSRO.json'))
    config_path = next((candidate for candidate in candidates if os.path.isfile(candidate)), None)
    if config_path is None:
        return 'unknown', 'vsro_config_not_found'
    try:
        with open(config_path, 'r') as stream:
            config = json.load(stream)
    except Exception:
        return 'unknown', 'vsro_config_unreadable'
    if isinstance(config, list):
        entries = config
    elif isinstance(config, dict):
        configured_servers = config.get('servers', config.get('Servers'))
        if isinstance(configured_servers, list):
            entries = configured_servers
        else:
            # phBot stores private-server profiles as a root mapping (for example,
            # {"GreatestSRO": {"servers": ["Greatest"], ...}}).
            entries = [value for value in config.values() if isinstance(value, dict)]
    else:
        return 'unknown', 'vsro_config_invalid_shape'

    server_key = server.strip().casefold()
    matches = []
    for entry in entries:
        if not isinstance(entry, dict):
            continue
        named_server = entry.get('server', entry.get('name', entry.get('Server', entry.get('Name'))))
        matches_by_name = (
            isinstance(named_server, str) and named_server.strip().casefold() == server_key
        )
        server_names = entry.get('servers', entry.get('Servers'))
        matches_by_list = (
            isinstance(server_names, list) and any(
                isinstance(name, str) and name.strip().casefold() == server_key
                for name in server_names
            )
        )
        if matches_by_name or matches_by_list:
            matches.append(entry)
    if not matches:
        return 'unknown', 'server_not_in_vsro_config'
    if len(matches) != 1:
        return 'unknown', 'ambiguous_server_config'

    matched = matches[0]
    selectors = []
    for key in ('protocol_variant', 'protocol'):
        if key in matched:
            value = matched.get(key)
            if not isinstance(value, str) or not value.strip():
                return 'unknown', 'malformed_protocol_selector'
            selectors.append(value.strip().casefold())
    if selectors and len(set(selectors)) != 1:
        return 'unknown', 'conflicting_protocol_selectors'
    selected_protocol = None
    if selectors:
        selected_protocol = 'vsro-1.188' if selectors[0] in ('1.188', 'vsro-1.188') else 'unknown'
        if selected_protocol == 'unknown':
            return 'unknown', 'unsupported_protocol_selector'

    # These phBot private-server type switches represent other client families or
    # vSRO variants. Do not silently interpret any enabled one as vSRO 1.188.
    other_types = (
        'black_rogue', 'black rogue', 'thsro', 'ecsro', 'csro silkroadr',
        'isro_private', 'jsro', 'rigid', 'mhtc',
    )
    for key in other_types:
        if key in matched:
            value = matched[key]
            if not isinstance(value, bool):
                return 'unknown', 'malformed_protocol_flags'
            if value:
                return 'unknown', 'unsupported_protocol_variant'

    flags = {
        '1.193': ('vsro_193', 'v1.193', 'v1.193_enabled'),
        '1.274': ('v1.274', 'vsro_274', 'v1.274_enabled'),
        '1.065': ('v1.065', 'vsro_065', 'v1.065_enabled'),
    }
    has_any_flag = any(key in matched for keys in flags.values() for key in keys)
    if not has_any_flag:
        if selected_protocol is not None:
            return selected_protocol, 'explicit_protocol_selector'
        return 'unknown', 'protocol_flags_unavailable'
    flag_selections = []
    for protocol, keys in flags.items():
        values = [matched[key] for key in keys if key in matched]
        if not values:
            return 'unknown', 'incomplete_protocol_flags'
        if any(not isinstance(value, bool) for value in values) or len(set(values)) != 1:
            return 'unknown', 'malformed_protocol_flags'
        flag_selections.append((protocol, values[0]))
    flagged = [protocol for protocol, enabled in flag_selections if enabled]
    if len(flagged) > 1:
        return 'unknown', 'conflicting_protocol_flags'
    flags_protocol = 'unknown' if flagged else 'vsro-1.188'
    if selected_protocol is not None and selected_protocol != flags_protocol:
        return 'unknown', 'conflicting_protocol_selector_and_flags'
    if flags_protocol == 'unknown':
        return 'unknown', 'unsupported_protocol_variant'
    return selected_protocol or flags_protocol, 'v1.188_selected_by_phbot_flags'


class ItemPacketError(ValueError):
    pass


class LittleEndianReader(object):
    """Bounds-checked reader for the small, allowlisted vSRO item updates."""
    def __init__(self, data):
        if not isinstance(data, (bytes, bytearray)) or len(data) > MAX_ITEM_PACKET_SIZE:
            raise ItemPacketError('invalid_or_oversized_packet')
        self.data = data
        self.offset = 0

    def _read(self, fmt):
        size = struct.calcsize(fmt)
        if self.offset + size > len(self.data):
            raise ItemPacketError('truncated_packet')
        value = struct.unpack_from(fmt, self.data, self.offset)[0]
        self.offset += size
        return value

    def u8(self): return self._read('<B')
    def u16(self): return self._read('<H')
    def u32(self): return self._read('<I')
    def u64(self): return self._read('<Q')

    def finish(self):
        if self.offset != len(self.data):
            raise ItemPacketError('unexpected_trailing_bytes')


def parse_item_stats_update(data):
    """Decode the corroborated 0x3040 update field order; reject unknown flags."""
    reader = LittleEndianReader(data)
    slot, flags = reader.u8(), reader.u8()
    if not flags or flags & 0x80:
        raise ItemPacketError('unsupported_update_flags')
    update = {'source_slot': slot, 'fields': {}}
    fields = update['fields']
    if flags & 0x01:
        fields['model'] = reader.u32()
        if fields['model'] == 0:
            raise ItemPacketError('invalid_model')
    if flags & 0x02:
        fields['plus'] = reader.u8()
    if flags & 0x04:
        fields['variance'] = str(reader.u64())
    if flags & 0x08:
        fields['quantity'] = reader.u16()
    if flags & 0x10:
        fields['durability'] = reader.u32()
    if flags & 0x40:
        fields['state'] = reader.u8()
    if flags & 0x20:
        count = reader.u8()
        if count > MAX_ITEM_MAGIC_OPTIONS:
            raise ItemPacketError('magic_option_count_exceeds_limit')
        fields['magic_options'] = [
            {'id': str(reader.u32()), 'value': str(reader.u32())}
            for _ in range(count)
        ]
        fields['magic_options_available'] = True
    reader.finish()
    return update


def parse_item_durability_update(data):
    reader = LittleEndianReader(data)
    slot, durability = reader.u8(), reader.u32()
    reader.finish()
    return {'source_slot': slot, 'fields': {'durability': durability}}



class PassiveItemTracker(object):
    """Bounded packet queue plus session-local, fail-closed item observations."""
    ALLOWED_OPCODES = (0x3040, 0x3052, 0x30C8, 0xB034)

    def __init__(self, capture_enabled=None):
        self.capture_enabled = ITEM_PACKET_CAPTURE_ENABLED if capture_enabled is None else bool(capture_enabled)
        self._lock = threading.Lock()
        self._queue = collections.deque()
        self._queued_bytes = 0
        self._overflow = False
        self._states = {}
        self._protocol = 'unknown'
        self._protocol_reason = 'not_resolved'
        self._epoch = 0
        self._sequence = 0
        self._last_invalidation = 'session_start'
        self._capture_records = []
        self._capture_bytes = 0
        self.capture_overflow = False
        self._pet_packet_count = 0
        self._pet_snapshot_packet_count = 0
        self._pet_packet_sizes = {
            '0x30C8': {'count': 0, 'min_bytes': None, 'max_bytes': None,
                       'last_bytes': None, 'last_sequence': None},
            '0xB034': {'count': 0, 'min_bytes': None, 'max_bytes': None,
                       'last_bytes': None, 'last_sequence': None},
        }

    def enqueue(self, opcode, data):
        if opcode not in self.ALLOWED_OPCODES:
            return False
        try:
            size = len(data)
            if size > MAX_ITEM_PACKET_SIZE:
                raise ValueError('oversized')
            payload = bytes(data)
        except Exception:
            with self._lock:
                self._overflow = True
                self._queue.clear()
                self._queued_bytes = 0
            return False
        with self._lock:
            if (len(self._queue) >= MAX_ITEM_PACKET_COUNT or
                    self._queued_bytes + len(payload) > MAX_ITEM_PACKET_BYTES):
                self._overflow = True
                self._queue.clear()
                self._queued_bytes = 0
                return False
            self._queue.append((opcode, payload))
            self._queued_bytes += len(payload)
        return True

    def _take_queue(self):
        with self._lock:
            events = list(self._queue)
            overflow = self._overflow
            self._queue.clear()
            self._queued_bytes = 0
            self._overflow = False
        return overflow, events

    def reset(self, reason='session_changed', protocol=None):
        self._states.clear()
        self._epoch += 1
        self._sequence = 0
        self._last_invalidation = reason
        self._pet_packet_count = 0
        self._pet_snapshot_packet_count = 0
        self._pet_packet_sizes = {
            '0x30C8': {'count': 0, 'min_bytes': None, 'max_bytes': None,
                       'last_bytes': None, 'last_sequence': None},
            '0xB034': {'count': 0, 'min_bytes': None, 'max_bytes': None,
                       'last_bytes': None, 'last_sequence': None},
        }
        if protocol is not None:
            self._protocol = protocol
        with self._lock:
            self._queue.clear()
            self._queued_bytes = 0
            self._overflow = False

    def set_protocol(self, protocol, reason=None):
        protocol = protocol if isinstance(protocol, str) else 'unknown'
        if not isinstance(reason, str) or not reason:
            reason = 'protocol_resolution_unavailable'
        self._protocol_reason = reason[:80]
        if protocol != self._protocol:
            self.reset('protocol_changed', protocol)

    def _invalidate(self, reason, slot=None):
        if slot is None:
            self._states.clear()
            self._epoch += 1
        else:
            self._states.pop(slot, None)
        self._last_invalidation = reason

    def _capture(self, record):
        if not self.capture_enabled:
            return
        encoded = json.dumps(record, separators=(',', ':'), sort_keys=True).encode('utf-8')
        if len(encoded) > MAX_ITEM_CAPTURE_BYTES:
            self.capture_overflow = True
            return
        while self._capture_records and (
                len(self._capture_records) >= MAX_ITEM_CAPTURE_RECORDS or
                self._capture_bytes + len(encoded) > MAX_ITEM_CAPTURE_BYTES):
            removed = self._capture_records.pop(0)
            self._capture_bytes -= len(json.dumps(removed, separators=(',', ':'), sort_keys=True).encode('utf-8'))
            self.capture_overflow = True
        self._capture_records.append(record)
        self._capture_bytes += len(encoded)

    def sanitized_capture(self):
        """Return bounded item observations and packet metadata without packet bytes."""
        return {'schema_version': 1, 'overflow': self.capture_overflow,
                'records': list(self._capture_records)}

    def _process(self, opcode, payload):
        self._sequence += 1
        if opcode in (0x30C8, 0xB034):
            opcode_name = '0x%04X' % opcode
            diagnostic = self._pet_packet_sizes[opcode_name]
            diagnostic['count'] = min(diagnostic['count'] + 1, 2147483647)
            packet_size = len(payload)
            diagnostic['min_bytes'] = packet_size if diagnostic['min_bytes'] is None else min(
                diagnostic['min_bytes'], packet_size)
            diagnostic['max_bytes'] = packet_size if diagnostic['max_bytes'] is None else max(
                diagnostic['max_bytes'], packet_size)
            diagnostic['last_bytes'] = packet_size
            diagnostic['last_sequence'] = str(self._sequence)
            if opcode == 0x30C8:
                self._pet_snapshot_packet_count = min(
                    self._pet_snapshot_packet_count + 1, 2147483647)
            else:
                self._pet_packet_count = min(self._pet_packet_count + 1, 2147483647)
                # This operation stream can change character inventory state;
                # discard character-slot enrichment until a verified handler
                # can reconcile the specific operation.
                self._invalidate('inventory_operation_unclassified')
            self._capture({'opcode': opcode_name, 'sequence': str(self._sequence),
                           'result': 'presence_and_size_only',
                           'packet_bytes': packet_size,
                           'decoder_build': PET_INVENTORY_DECODER_BUILD})
            return
        try:
            parsed = parse_item_stats_update(payload) if opcode == 0x3040 else parse_item_durability_update(payload)
        except ItemPacketError as error:
            slot = payload[0] if payload else None
            self._invalidate(str(error), slot)
            self._capture({'opcode': '0x%04X' % opcode, 'sequence': str(self._sequence), 'result': str(error)})
            return
        slot = parsed['source_slot']
        fields = parsed['fields']
        if self._protocol != 'vsro-1.188':
            self._invalidate('unsupported_protocol', slot)
            return
        state = self._states.get(slot)
        model = fields.get('model')
        if model is not None:
            # A RefObjID-bearing update starts a new item lifetime even when
            # the replacement has the same model in the same slot.
            state = {'model': model, 'fields': {}, 'sequence': self._sequence}
            self._states[slot] = state
        elif state is None:
            return
        state['fields'].update(fields)
        state['sequence'] = self._sequence
        self._capture({
            'opcode': '0x%04X' % opcode,
            'sequence': str(self._sequence),
            'source_slot': slot,
            'model': str(state['model']),
            'fields': dict(state['fields']),
        })

    def decorate(self, resources, session_id=None):
        overflow, events = self._take_queue()
        if overflow:
            self._invalidate('packet_queue_overflow')
        for opcode, payload in events:
            self._process(opcode, payload)

        attached = 0
        for container_key in ('equipment', 'inventory'):
            container = resources.get(container_key)
            if not isinstance(container, dict) or container.get('availability') != 'observed':
                continue
            slots = container.get('slots')
            if not isinstance(slots, list):
                continue
            for slot_entry in slots:
                if not isinstance(slot_entry, dict):
                    continue
                source_slot = slot_entry.get('source_slot')
                item = slot_entry.get('item')
                if isinstance(item, dict):
                    item.pop('instance', None)
                state = self._states.get(source_slot) if isinstance(source_slot, int) else None
                if not isinstance(item, dict):
                    if state is not None:
                        self._invalidate('api_slot_empty', source_slot)
                    continue
                if state is None:
                    continue
                if item.get('model') != state.get('model'):
                    self._invalidate('api_model_mismatch', source_slot)
                    continue
                api_plus = item.get('plus')
                packet_plus = state['fields'].get('plus')
                if api_plus is not None and packet_plus is not None and api_plus != packet_plus:
                    self._invalidate('api_enhancement_mismatch', source_slot)
                    continue
                observed_fields = state['fields']
                if not any(key in observed_fields for key in ('variance', 'durability', 'magic_options', 'plus')):
                    continue
                instance = {
                    'availability': 'observed',
                    'source': 'vsro_1188_packet',
                    'observation_id': '%s:%d:%d:%d' % (
                        session_id or 'unregistered', self._epoch,
                        state['sequence'], source_slot),
                    'capture_epoch': str(self._epoch),
                    'observation_sequence': str(state['sequence']),
                    'model': str(state['model']),
                    'magic_options_availability': (
                        'observed' if observed_fields.get('magic_options_available') else 'not_observed'),
                }
                for key in ('plus', 'variance', 'durability', 'quantity', 'state'):
                    if key in observed_fields:
                        value = observed_fields[key]
                        instance[key] = str(value) if key == 'variance' else value
                if observed_fields.get('magic_options_available'):
                    instance['magic_options'] = list(observed_fields.get('magic_options', []))
                item['instance'] = instance
                attached += 1

        resources['item_enrichment'] = {
            'availability': 'observed' if attached else 'not_observed',
            'protocol': self._protocol,
            'protocol_reason': self._protocol_reason,
            'source': 'vsro_1188_packet',
            'decoder_build': ITEM_DECODER_BUILD,
            'api_evidence_version': ITEM_API_EVIDENCE_VERSION,
            'observed_items': attached,
            'reason': None if attached else self._last_invalidation,
            'capture_overflow': self.capture_overflow,
            'pet_inventory_decoder_build': PET_INVENTORY_DECODER_BUILD,
            'pet_inventory_packet_count': self._pet_packet_count,
            'pet_snapshot_packet_count': self._pet_snapshot_packet_count,
            'pet_inventory_status': (
                'candidate_layout_unverified'
                if self._pet_snapshot_packet_count else 'not_observed'),
            'pet_packet_probe': {
                'mode': 'presence_and_size_only',
                'packet_bytes_retained': False,
                'opcodes': {
                    key: dict(value) for key, value in self._pet_packet_sizes.items()
                },
            },
        }
        return resources

    def persist_sanitized_capture(self, path):
        """Write only parsed item observations and packet metadata off the callback."""
        if not self.capture_enabled or not isinstance(path, str) or not path:
            return False
        payload = json.dumps(self.sanitized_capture(), separators=(',', ':'), sort_keys=True)
        if len(payload.encode('utf-8')) > MAX_ITEM_CAPTURE_BYTES:
            raise ValueError('sanitized item capture exceeds size limit')
        directory = os.path.dirname(path)
        if directory and not os.path.isdir(directory):
            os.makedirs(directory)
        temporary = path + '.tmp'
        with open(temporary, 'w') as stream:
            stream.write(payload)
        os.replace(temporary, path)
        return True

def _utc_epoch(value):
    if (not isinstance(value, str) or len(value) < 20 or value[-1] != 'Z'
            or value[19] not in ('Z', '.')):
        raise ValueError('invalid server timestamp')
    epoch = calendar.timegm(time.strptime(value[:19], '%Y-%m-%dT%H:%M:%S'))
    if value[19] == 'Z':
        if len(value) != 20:
            raise ValueError('invalid server timestamp')
        return float(epoch)
    fraction = value[20:-1]
    if not fraction or len(fraction) > 9 or not fraction.isdigit():
        raise ValueError('invalid server timestamp')
    return float(epoch) + int(fraction) / float(10 ** len(fraction))


def _log(message):
    text = 'Plugin: PhMon ' + str(message)
    if _phbot_log is not None:
        _phbot_log(text)
    else:
        print(text)


class _CallbackTiming:
    """Local duration evidence only; never record arguments or API return data."""
    def __init__(self):
        self.started = _monotonic()
        self.stages = []

    def run(self, stage, function, *args):
        started = _monotonic()
        try:
            return function(*args)
        finally:
            self.stages.append((stage, max(0.0, _monotonic() - started)))

    def report(self):
        elapsed = max(0.0, _monotonic() - self.started)
        if elapsed < 0.5:
            return
        slowest = sorted(self.stages, key=lambda entry: entry[1], reverse=True)[:4]
        _log('event_loop slow: %.0f ms; %s' % (
            elapsed * 1000.0,
            ', '.join('%s=%.0f ms' % (stage, duration * 1000.0) for stage, duration in slowest)))


def _navigation_stage(stage, function, *args):
    # A start line identifies an API that has not returned when phBot emits its
    # callback watchdog warning. Keep script text, coordinates and errors local.
    _log('navigation %s started' % stage)
    started = _monotonic()
    outcome = 'raised'
    try:
        result = function(*args)
        outcome = 'returned'
        return result
    finally:
        _log('navigation %s %s in %.0f ms' % (
            stage, outcome, max(0.0, _monotonic() - started) * 1000.0))


def _utc_now():
    return time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())


def _worker_utc_now(worker):
    # Use the authenticated hello clock offset so occurrence timestamps can be
    # checked against the server-owned character-session interval.
    offset = getattr(worker, '_server_clock_offset', 0.0) if worker is not None else 0.0
    try:
        epoch = time.time() + float(offset)
    except Exception:
        epoch = time.time()
    return time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(epoch))


def _monotonic():
    if hasattr(time, 'monotonic'):
        return time.monotonic()
    return time.time()


def _validate_agent_id(agent_id):
    if not isinstance(agent_id, str) or len(agent_id) != 36:
        return False
    parts = agent_id.split('-')
    if [len(part) for part in parts] != [8, 4, 4, 4, 12]:
        return False
    if agent_id.lower() != agent_id:
        return False
    try:
        int(''.join(parts), 16)
    except ValueError:
        return False
    return True


def validate_config(config):
    if not isinstance(config, dict):
        raise ValueError('configuration must be a mapping')
    backend_url = config.get('backend_url', '')
    agent_id = config.get('agent_id', '')
    agent_token = config.get('agent_token', '')

    if not isinstance(backend_url, str) or len(backend_url) > 2048:
        raise ValueError('backend_url must be a WebSocket URL')
    if any(ch.isspace() for ch in backend_url):
        raise ValueError('backend_url must not contain whitespace')
    parsed = urlparse(backend_url)
    if parsed.scheme not in ('ws', 'wss') or not parsed.hostname:
        raise ValueError('backend_url must use ws:// or wss://')
    if parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError('backend_url must not contain credentials, query parameters, or fragments')
    if parsed.path not in ('', '/', '/agent'):
        raise ValueError('backend_url path must be /agent')
    if not _validate_agent_id(agent_id):
        raise ValueError('agent_id must be a lowercase UUID')
    if not isinstance(agent_token, str) or not agent_token or len(agent_token) > 256:
        raise ValueError('agent_token is missing or invalid')
    if any(ch.isspace() for ch in agent_token):
        raise ValueError('agent_token must not contain whitespace')

    normalized = dict(config)
    if parsed.path in ('', '/'):
        normalized['backend_url'] = backend_url.rstrip('/') + '/agent'
    for key in _CONFIG_OPTIONAL_BOOLEAN_KEYS:
        value = config.get(key, True)
        if isinstance(value, str) and value.lower() in ('true', 'false'):
            value = value.lower() == 'true'
        if not isinstance(value, bool):
            raise ValueError(key + ' must be true or false')
        normalized[key] = value
    return normalized


_CONFIG_KEYS = ('backend_url', 'agent_id', 'agent_token')
_CONFIG_OPTIONAL_BOOLEAN_KEYS = ('ignore_potion_events', 'ignore_pill_events')
_CONFIG_ALLOWED_KEYS = frozenset(_CONFIG_KEYS + _CONFIG_OPTIONAL_BOOLEAN_KEYS)


def _profile_settings_path(config_dir, bot_config_path, bot_profile):
    if not bot_config_path:
        raise ValueError('active phBot profile is unavailable')
    profile_file = str(bot_config_path).replace('\\', '/').rsplit('/', 1)[-1]
    player_config = os.path.splitext(profile_file)[0]
    if not player_config:
        raise ValueError('active phBot profile is invalid')

    bot_profile = '' if bot_profile is None else str(bot_profile)
    if bot_profile:
        safe_profile = ''.join(
            ch if ch.isalnum() or ch in ('-', '_') else '_'
            for ch in bot_profile
        )[:48].strip('_')
        digest = hashlib.sha256(bot_profile.encode('utf-8')).hexdigest()[:8]
        profile_key = (safe_profile or 'profile') + '-' + digest
    else:
        profile_key = 'default'

    return os.path.join(
        config_dir,
        pName,
        player_config + '.' + profile_key + '.cfg',
    )


def _death_spool_path(config_dir, agent_id, settings_path):
    if (not isinstance(config_dir, str) or not config_dir.strip() or
            not _validate_agent_id(agent_id) or
            not isinstance(settings_path, str) or not settings_path.strip()):
        return None
    profile_identity = os.path.normcase(os.path.abspath(settings_path)).encode('utf-8')
    profile_key = hashlib.sha256(profile_identity).hexdigest()[:16]
    filename = 'death-events-' + agent_id + '-' + profile_key + '.json'
    return os.path.join(config_dir, pName, filename)


def _mob_spool_path(config_dir, agent_id, settings_path):
    if (not isinstance(config_dir, str) or not config_dir.strip() or
            not _validate_agent_id(agent_id) or
            not isinstance(settings_path, str) or not settings_path.strip()):
        return None
    profile_identity = os.path.normcase(os.path.abspath(settings_path)).encode('utf-8')
    profile_key = hashlib.sha256(profile_identity).hexdigest()[:16]
    filename = 'mob-observations-' + agent_id + '-' + profile_key + '.json'
    return os.path.join(config_dir, pName, filename)


def load_saved_config(path):
    values = {}
    with open(path, 'r') as handle:
        for raw_line in handle:
            line = raw_line.rstrip('\r\n')
            if not line:
                continue
            if '=' not in line:
                raise ValueError('invalid saved configuration')
            key, value = line.split('=', 1)
            if key not in _CONFIG_ALLOWED_KEYS or key in values:
                raise ValueError('invalid saved configuration')
            values[key] = value
    if not set(_CONFIG_KEYS).issubset(values):
        raise ValueError('saved configuration is incomplete')
    return validate_config(values)


def save_saved_config(path, config):
    normalized = validate_config(config)
    directory = os.path.dirname(path)
    if not os.path.isdir(directory):
        try:
            os.makedirs(directory)
        except OSError:
            if not os.path.isdir(directory):
                raise
    temporary = path + '.tmp'
    with open(temporary, 'w') as handle:
        for key in _CONFIG_KEYS + _CONFIG_OPTIONAL_BOOLEAN_KEYS:
            value = normalized[key]
            handle.write(key + '=' + (str(value).lower() if isinstance(value, bool) else value) + '\n')
    os.replace(temporary, path)
    return normalized


def config_from_gui_values(backend_url, agent_id, agent_token, saved_config=None,
                           ignore_potion_events=True, ignore_pill_events=True):
    backend_url = str(backend_url).strip()
    agent_id = str(agent_id).strip()
    agent_token = str(agent_token).strip()

    if not agent_token and saved_config is not None:
        if (
            backend_url == saved_config.get('backend_url') and
            agent_id == saved_config.get('agent_id')
        ):
            agent_token = saved_config.get('agent_token', '')

    if not agent_token:
        raise ValueError('agent token is required for this identity')

    return validate_config({
        'backend_url': backend_url,
        'agent_id': agent_id,
        'agent_token': agent_token,
        'ignore_potion_events': ignore_potion_events,
        'ignore_pill_events': ignore_pill_events,
    })


def _expected_accept(key):
    raw = (key + _WEBSOCKET_GUID).encode('ascii')
    return base64.b64encode(hashlib.sha1(raw).digest()).decode('ascii')


def _mask_payload(payload, mask_key):
    data = bytearray(payload)
    for index in range(len(data)):
        data[index] ^= mask_key[index % 4]
    return bytes(data)


def _encode_client_frame(opcode, payload, mask_key=None):
    if isinstance(payload, str):
        payload = payload.encode('utf-8')
    if mask_key is None:
        mask_key = os.urandom(4)
    if len(mask_key) != 4:
        raise ValueError('mask key must be four bytes')
    length = len(payload)
    if length > MAX_MESSAGE_BYTES:
        raise ValueError('WebSocket message exceeds limit')
    header = bytearray([0x80 | (opcode & 0x0f)])
    if length < 126:
        header.append(0x80 | length)
    elif length <= 0xffff:
        header.append(0x80 | 126)
        header.extend(struct.pack('!H', length))
    else:
        header.append(0x80 | 127)
        header.extend(struct.pack('!Q', length))
    return bytes(header) + mask_key + _mask_payload(payload, mask_key)


class WebSocketClosed(Exception):
    pass


class WebSocketClient(object):
    def __init__(self, url, token, connect_timeout=10.0):
        self.url = url
        self.token = token
        self.connect_timeout = connect_timeout
        self._socket = None
        self._buffer = b''
        self._send_lock = threading.Lock()

    def connect(self):
        parsed = urlparse(self.url)
        secure = parsed.scheme == 'wss'
        port = parsed.port or (443 if secure else 80)
        path = parsed.path or '/agent'
        if parsed.query:
            path += '?' + parsed.query

        raw_socket = socket.create_connection((parsed.hostname, port), self.connect_timeout)
        if secure:
            context = ssl.create_default_context()
            raw_socket = context.wrap_socket(raw_socket, server_hostname=parsed.hostname)
        raw_socket.settimeout(self.connect_timeout)
        self._socket = raw_socket

        key = base64.b64encode(os.urandom(16)).decode('ascii')
        default_port = 443 if secure else 80
        host = parsed.hostname if port == default_port else parsed.hostname + ':' + str(port)
        request = (
            'GET ' + path + ' HTTP/1.1\r\n'
            'Host: ' + host + '\r\n'
            'Upgrade: websocket\r\n'
            'Connection: Upgrade\r\n'
            'Sec-WebSocket-Key: ' + key + '\r\n'
            'Sec-WebSocket-Version: 13\r\n'
            'Authorization: Bearer ' + self.token + '\r\n'
            '\r\n'
        ).encode('ascii')
        raw_socket.sendall(request)
        response = self._read_http_headers()
        status_line = response[0]
        headers = response[1]
        if ' 101 ' not in status_line:
            raise WebSocketClosed('WebSocket upgrade rejected')
        if headers.get('upgrade', '').lower() != 'websocket':
            raise WebSocketClosed('invalid WebSocket upgrade response')
        if 'upgrade' not in headers.get('connection', '').lower():
            raise WebSocketClosed('invalid WebSocket connection response')
        if headers.get('sec-websocket-accept', '') != _expected_accept(key):
            raise WebSocketClosed('invalid WebSocket accept value')
        raw_socket.settimeout(None)
        return self

    def _read_http_headers(self):
        data = b''
        while b'\r\n\r\n' not in data:
            chunk = self._socket.recv(4096)
            if not chunk:
                raise WebSocketClosed('connection closed during WebSocket upgrade')
            data += chunk
            if len(data) > 16384:
                raise WebSocketClosed('WebSocket upgrade response is too large')
        header_bytes, self._buffer = data.split(b'\r\n\r\n', 1)
        lines = header_bytes.decode('iso-8859-1').split('\r\n')
        headers = {}
        for line in lines[1:]:
            if ':' not in line:
                continue
            name, value = line.split(':', 1)
            headers[name.strip().lower()] = value.strip()
        return lines[0], headers

    def _recv_exact(self, length, sock=None):
        if sock is None:
            sock = self._socket
        if sock is None:
            raise WebSocketClosed('WebSocket is not connected')
        chunks = []
        if self._buffer:
            take = self._buffer[:length]
            chunks.append(take)
            self._buffer = self._buffer[len(take):]
            length -= len(take)
        while length > 0:
            chunk = sock.recv(length)
            if not chunk:
                raise WebSocketClosed('WebSocket closed')
            chunks.append(chunk)
            length -= len(chunk)
        return b''.join(chunks)

    def _wait_for_frame_start(self, sock, timeout):
        if self._buffer:
            return True
        pending = getattr(sock, 'pending', None)
        if callable(pending) and pending() > 0:
            return True
        if timeout is None:
            return True
        readable, _, _ = select.select([sock], [], [], timeout)
        return bool(readable)

    def _read_frame(self, timeout=None):
        sock = self._socket
        if sock is None:
            raise WebSocketClosed('WebSocket is not connected')
        if not self._wait_for_frame_start(sock, timeout):
            return None

        sock.settimeout(self.connect_timeout)
        try:
            first, second = struct.unpack('!BB', self._recv_exact(2, sock))
            fin = bool(first & 0x80)
            rsv = first & 0x70
            opcode = first & 0x0f
            masked = bool(second & 0x80)
            length = second & 0x7f
            if rsv or not fin:
                raise WebSocketClosed('fragmented or extended frames are unsupported')
            if masked:
                raise WebSocketClosed('server frames must not be masked')
            if length == 126:
                length = struct.unpack('!H', self._recv_exact(2, sock))[0]
            elif length == 127:
                length = struct.unpack('!Q', self._recv_exact(8, sock))[0]
            if length > MAX_MESSAGE_BYTES:
                raise WebSocketClosed('WebSocket message exceeds limit')
            payload = self._recv_exact(length, sock)
            return opcode, payload
        except socket.timeout:
            raise WebSocketClosed('timeout while receiving WebSocket frame')
        finally:
            try:
                sock.settimeout(None)
            except Exception:
                pass

    def send_json(self, value):
        payload = json.dumps(value, separators=(',', ':'))
        self._send_frame(0x1, payload.encode('utf-8'))

    def receive_json(self, timeout=None):
        while True:
            frame = self._read_frame(timeout)
            if frame is None:
                return None
            opcode, payload = frame
            if opcode == 0x1:
                try:
                    return json.loads(payload.decode('utf-8'))
                except (ValueError, UnicodeDecodeError):
                    raise WebSocketClosed('invalid JSON message')
            if opcode == 0x8:
                raise WebSocketClosed('server closed WebSocket')
            if opcode == 0x9:
                self._send_frame(0xA, payload)
                continue
            if opcode == 0xA:
                continue
            raise WebSocketClosed('unsupported WebSocket frame')

    def _send_frame(self, opcode, payload):
        if self._socket is None:
            raise WebSocketClosed('WebSocket is not connected')
        frame = _encode_client_frame(opcode, payload)
        with self._send_lock:
            self._socket.sendall(frame)

    def close(self):
        sock = self._socket
        if sock is None:
            return
        try:
            sock.shutdown(socket.SHUT_RDWR)
        except Exception:
            pass
        try:
            sock.close()
        except Exception:
            pass
        self._socket = None


class ReconnectBackoff(object):
    def __init__(self, random_function=None):
        self._base = 1.0
        self._random = random_function or random.uniform

    def reset(self):
        self._base = 1.0

    def next_delay(self):
        delay = self._base * self._random(0.75, 1.0)
        self._base = min(self._base * 2.0, 30.0)
        return delay


class EventSpool(object):
    """Atomic bounded spool for canonical discrete activity events."""
    def __init__(self, path, max_items=MAX_EVENT_SPOOL_ITEMS, max_bytes=MAX_EVENT_SPOOL_BYTES):
        self.path = path
        self.max_items = max_items
        self.max_bytes = max_bytes
        self._lock = threading.Lock()
        self._items = []
        if path:
            self._load()

    def _load(self):
        try:
            with open(self.path, 'rb') as handle:
                raw = handle.read(self.max_bytes + 1)
            if len(raw) > self.max_bytes:
                raise ValueError('spool exceeds size limit')
            value = json.loads(raw.decode('utf-8'))
            if not isinstance(value, list) or len(value) > self.max_items:
                raise ValueError('invalid spool')
            normalized = [_upgrade_legacy_death_event(item) for item in value]
            self._items = [item for item in normalized if _valid_event_spool_item(item)]
            if normalized != value:
                with self._lock:
                    self._save_locked()
        except IOError:
            self._items = []
        except Exception as error:
            _log('activity event spool could not be loaded (' + error.__class__.__name__ + ')')
            self._items = []

    def _save_locked(self):
        if not self.path:
            return False
        directory = os.path.dirname(self.path)
        if directory and not os.path.isdir(directory):
            os.makedirs(directory)
        temporary = self.path + '.tmp'
        raw = json.dumps(self._items, separators=(',', ':'), sort_keys=True).encode('utf-8')
        if len(raw) > self.max_bytes:
            return False
        with open(temporary, 'wb') as handle:
            handle.write(raw)
            handle.flush()
            try:
                os.fsync(handle.fileno())
            except Exception:
                pass
        replace = getattr(os, 'replace', None)
        if replace:
            replace(temporary, self.path)
        else:  # pragma: no cover - Python 3.4 has os.replace; kept for embedded variants.
            if os.path.exists(self.path):
                os.remove(self.path)
            os.rename(temporary, self.path)
        return True

    def add(self, item):
        if not self.path:
            return False
        if not _valid_event_spool_item(item):
            return False
        with self._lock:
            if any(existing.get('event_id') == item.get('event_id') for existing in self._items):
                return True
            if not self._has_capacity(item):
                return False
            self._items.append(dict(item))
            try:
                if self._save_locked():
                    return True
            except Exception as error:
                _log('activity event spool write failed (' + error.__class__.__name__ + ')')
            self._items.pop()
            return False

    def acknowledge(self, event_id):
        with self._lock:
            previous = self._items
            self._items = [item for item in previous if item.get('event_id') != event_id]
            if len(previous) == len(self._items):
                return True
            try:
                if self._save_locked():
                    return True
            except Exception as error:
                _log('activity event spool acknowledgement failed (' + error.__class__.__name__ + ')')
            self._items = previous
            return False

    def bind_session(self, event_id, character_id, session_id, sequence=None):
        if not _validate_agent_id(character_id) or not _validate_agent_id(session_id):
            return False
        with self._lock:
            for index, item in enumerate(self._items):
                if item.get('event_id') != event_id:
                    continue
                if item.get('character_id') and item.get('session_id') and (
                        item['character_id'] != character_id or item['session_id'] != session_id):
                    return False
                previous = self._items
                updated = [dict(value) for value in previous]
                updated[index]['character_id'] = character_id
                updated[index]['session_id'] = session_id
                if sequence is not None and updated[index].get('sequence') is None:
                    updated[index]['sequence'] = sequence
                updated[index]['deferred_session_binding'] = False
                payload = updated[index].get('payload')
                if isinstance(payload, dict) and payload.get('session_binding') == 'deferred':
                    payload = dict(payload)
                    payload.pop('session_binding', None)
                    updated[index]['payload'] = payload
                if updated[index] == item:
                    return True
                self._items = updated
                try:
                    if self._save_locked():
                        return True
                except Exception as error:
                    _log('activity event session binding failed (' + error.__class__.__name__ + ')')
                self._items = previous
                return False
        return False

    def pending(self):
        with self._lock:
            return [dict(item) for item in self._items]

    def _has_capacity(self, item):
        raw = json.dumps(item, separators=(',', ':'), sort_keys=True).encode('utf-8')
        total_raw = json.dumps(self._items, separators=(',', ':'), sort_keys=True).encode('utf-8')
        if len(self._items) >= self.max_items or len(total_raw) + len(raw) > self.max_bytes:
            return False
        critical = _event_is_critical(item)
        current = [entry for entry in self._items if _event_is_critical(entry) == critical]
        current_bytes = sum(len(json.dumps(entry, separators=(',', ':'), sort_keys=True).encode('utf-8')) for entry in current)
        if critical:
            return len(current) < MAX_EVENT_CRITICAL_ITEMS and current_bytes + len(raw) <= MAX_EVENT_CRITICAL_BYTES
        return len(current) < MAX_EVENT_ORDINARY_ITEMS and current_bytes + len(raw) <= MAX_EVENT_ORDINARY_BYTES


def _upgrade_legacy_death_event(item):
    if not isinstance(item, dict):
        return item
    if item.get('source_ref') == 'EVENT_DIED' and 'kind' not in item:
        item = dict(item)
        item['schema_version'] = 1
        item['kind'] = 'character.died'
        item['category'] = 'character'
    return item


def _valid_event_spool_item(item):
    if not isinstance(item, dict):
        return False
    character_id = item.get('character_id')
    session_id = item.get('session_id')
    if (not _validate_agent_id(item.get('event_id')) or
            not isinstance(item.get('schema_version'), int) or isinstance(item.get('schema_version'), bool) or item.get('schema_version') != 1 or
            not isinstance(item.get('kind'), str) or len(item.get('kind')) > 80 or
            not isinstance(item.get('category'), str) or len(item.get('category')) > 40 or
            not isinstance(item.get('source'), str) or len(item.get('source')) > 80 or
            not isinstance(item.get('source_ref'), str) or len(item.get('source_ref')) > 120 or
            not isinstance(item.get('payload'), dict) or
            type(item.get('deferred_session_binding', False)) is not bool):
        return False
    if (character_id is None) != (session_id is None):
        return False
    if character_id is not None and (
            not _validate_agent_id(character_id) or not _validate_agent_id(session_id)):
        return False
    server = item.get('server')
    character_name = item.get('character_name')
    if (server is None) != (character_name is None):
        return False
    if (server is not None and (not isinstance(server, str) or len(server) > 100) or
            character_name is not None and (not isinstance(character_name, str) or len(character_name) > 64)):
        return False
    if item.get('sequence') is not None and (not isinstance(item.get('sequence'), int) or isinstance(item.get('sequence'), bool) or item.get('sequence') <= 0):
        return False
    if item.get('item_model') is not None and (not isinstance(item.get('item_model'), int) or isinstance(item.get('item_model'), bool) or item.get('item_model') < 0):
        return False
    if item.get('item_code') is not None and (not isinstance(item.get('item_code'), str) or len(item.get('item_code')) > 128):
        return False
    if item.get('dedupe_key') is not None and (not isinstance(item.get('dedupe_key'), str) or len(item.get('dedupe_key')) > 160):
        return False
    try:
        _utc_epoch(item.get('occurred_at'))
        encoded = json.dumps(item, separators=(',', ':'), sort_keys=True).encode('utf-8')
        payload = json.dumps(item.get('payload'), separators=(',', ':'), sort_keys=True).encode('utf-8')
    except Exception:
        return False
    return len(encoded) <= MAX_EVENT_BYTES and len(payload) <= MAX_EVENT_PAYLOAD_BYTES


def _valid_mob_sample(sample):
    if not isinstance(sample, dict):
        return False
    if (not _validate_agent_id(sample.get('sample_id')) or
            not _validate_agent_id(sample.get('character_id')) or
            not _validate_agent_id(sample.get('session_id')) or
            not _valid_position_region(sample.get('region')) or
            sample.get('area_id') != 'region:' + str(sample.get('region')) or
            sample.get('floor_id') != 'unmapped' or not isinstance(sample.get('observer'), dict) or
            not isinstance(sample.get('monsters'), list) or len(sample['monsters']) > MAX_MONSTERS_PER_SNAPSHOT):
        return False
    try:
        _utc_epoch(sample.get('sampled_at'))
        for axis in ('x', 'y'):
            value = sample['observer'].get(axis)
            if not _number(value) or abs(value) > 1000000:
                return False
        observer_z = sample['observer'].get('z')
        if observer_z is not None and (not _number(observer_z) or abs(observer_z) > 1000000):
            return False
        seen = set()
        for monster in sample['monsters']:
            if (not isinstance(monster, dict) or not isinstance(monster.get('id'), str) or
                    not monster['id'] or len(monster['id']) > 64 or monster['id'] in seen or
                    not _valid_position_region(monster.get('region')) or
                    not _mob_region_matches(sample['region'], monster.get('region')) or
                    not _number(monster.get('x')) or abs(monster['x']) > 1000000 or
                    not _number(monster.get('y')) or abs(monster['y']) > 1000000):
                return False
            seen.add(monster['id'])
            model = monster.get('model_id')
            if model is not None and (not isinstance(model, int) or isinstance(model, bool) or model < 0 or model > 4294967295):
                return False
            if monster.get('z') is not None and (not _number(monster['z']) or abs(monster['z']) > 1000000):
                return False
            if monster.get('type') is not None and (not isinstance(monster['type'], str) or len(monster['type']) > 64):
                return False
            if monster.get('type_code') is not None and (not isinstance(monster['type_code'], int) or isinstance(monster['type_code'], bool) or monster['type_code'] < 0 or monster['type_code'] > 255):
                return False
            for detail in ('name', 'servername'):
                if monster.get(detail) is not None and (not isinstance(monster[detail], str) or len(monster[detail]) > 128):
                    return False
            if monster.get('level') is not None and (not isinstance(monster['level'], int) or isinstance(monster['level'], bool) or monster['level'] < 1 or monster['level'] > 255):
                return False
            for detail in ('hp', 'max_hp'):
                if monster.get(detail) is not None and (not isinstance(monster[detail], int) or isinstance(monster[detail], bool) or monster[detail] < 0 or monster[detail] > 9007199254740991):
                    return False
            if monster.get('attacking') is not None and not isinstance(monster['attacking'], bool):
                return False
        encoded = json.dumps(sample, separators=(',', ':'), sort_keys=True, allow_nan=False).encode('utf-8')
    except Exception:
        return False
    return len(encoded) <= 64 * 1024


class MobObservationSpool(object):
    """Profile-scoped atomic spool; an item is removed only after a terminal ACK."""
    def __init__(self, path, max_items=MAX_MOB_SPOOL_ITEMS, max_bytes=MAX_MOB_SPOOL_BYTES):
        self.path = path
        self.max_items = max_items
        self.max_bytes = max_bytes
        self._lock = threading.Lock()
        self._items = []
        if path:
            self._load()

    def _load(self):
        try:
            with open(self.path, 'rb') as handle:
                raw = handle.read(self.max_bytes + 1)
            if len(raw) > self.max_bytes:
                raise ValueError('spool exceeds size limit')
            value = json.loads(raw.decode('utf-8'))
            if not isinstance(value, list) or len(value) > self.max_items:
                raise ValueError('invalid spool')
            self._items = [item for item in value if _valid_mob_sample(item)]
            if self._items != value:
                with self._lock:
                    self._save_locked()
        except IOError:
            self._items = []
        except Exception as error:
            _log('mob observation spool could not be loaded (' + error.__class__.__name__ + ')')
            self._items = []

    def _save_locked(self):
        if not self.path:
            return False
        directory = os.path.dirname(self.path)
        if directory and not os.path.isdir(directory):
            os.makedirs(directory)
        temporary = self.path + '.tmp'
        raw = json.dumps(self._items, separators=(',', ':'), sort_keys=True, allow_nan=False).encode('utf-8')
        if len(raw) > self.max_bytes:
            return False
        with open(temporary, 'wb') as handle:
            handle.write(raw)
            handle.flush()
            try:
                os.fsync(handle.fileno())
            except Exception:
                pass
        replace = getattr(os, 'replace', None)
        if replace:
            replace(temporary, self.path)
        else:  # pragma: no cover
            if os.path.exists(self.path):
                os.remove(self.path)
            os.rename(temporary, self.path)
        return True

    def add(self, sample):
        if not self.path or not _valid_mob_sample(sample):
            return False
        with self._lock:
            if any(item.get('sample_id') == sample['sample_id'] for item in self._items):
                return True
            encoded = json.dumps(sample, separators=(',', ':'), sort_keys=True, allow_nan=False).encode('utf-8')
            current = sum(len(json.dumps(item, separators=(',', ':'), sort_keys=True).encode('utf-8')) for item in self._items)
            if len(self._items) >= self.max_items or current + len(encoded) > self.max_bytes:
                return False
            self._items.append(dict(sample))
            try:
                if self._save_locked():
                    return True
            except Exception as error:
                _log('mob observation spool write failed (' + error.__class__.__name__ + ')')
            self._items.pop()
            return False

    def pending(self):
        with self._lock:
            return [dict(item) for item in self._items]

    def acknowledge(self, sample_id):
        with self._lock:
            previous = self._items
            self._items = [item for item in previous if item.get('sample_id') != sample_id]
            if len(previous) == len(self._items):
                return True
            try:
                if self._save_locked():
                    return True
            except Exception as error:
                _log('mob observation spool acknowledgement failed (' + error.__class__.__name__ + ')')
            self._items = previous
            return False


# Compatibility alias: existing installations and tests may still use this name.
DeathEventSpool = EventSpool


class AgentWorker(object):
    def __init__(self, config, phbot_version, websocket_factory=None, api_adapter=None):
        self.config = validate_config(config)
        self.config['death_spool_path'] = config.get('death_spool_path') if isinstance(config, dict) else None
        self.config['mob_spool_path'] = config.get('mob_spool_path') if isinstance(config, dict) else None
        self.phbot_version = str(phbot_version)
        self.websocket_factory = websocket_factory or WebSocketClient
        self.api = api_adapter or PhBotAdapter()
        self.stop_event = threading.Event()
        self._thread = None
        self._socket = None
        self._socket_lock = threading.Lock()
        self.status = 'Connecting to PhMon backend...'
        self._samples = _queue.Queue(maxsize=1)
        self._resource_samples = _queue.Queue(maxsize=8)
        self._resource_sample_gap = False
        self._event_baseline_required = True
        self._event_diff_session = None
        self._event_diff_identity = None
        self._event_diff = None
        self._pending_drop_lock = threading.Lock()
        self._pending_drop_events = []
        self._event_samples = _queue.Queue(maxsize=256)
        self._death_samples = self._event_samples
        self._death_spool = DeathEventSpool(self.config.get('death_spool_path'))
        self._map_samples = _queue.Queue(maxsize=1)
        self._mob_samples = _queue.Queue(maxsize=32)
        self._mob_spool = MobObservationSpool(self.config.get('mob_spool_path'))
        self._pending_mob_spool_sample = None
        self._mob_retry_at = {}
        self._latest_map_observation = None
        self._last_map_sent_at = None
        self._npc_samples = _queue.Queue(maxsize=1)
        self._latest_npc_observation = None
        self._last_npc_sent_at = None
        self._player_samples = _queue.Queue(maxsize=1)
        self._latest_player_observation = None
        self._last_player_sent_at = None
        self._event_sequence_lock = threading.Lock()
        self._event_sequences = {}
        self._alchemy_items_lock = threading.Lock()
        self._latest_alchemy_items = {}
        for pending_event in self._death_spool.pending():
            if pending_event.get('session_id') and isinstance(pending_event.get('sequence'), int):
                key = pending_event['session_id']
                self._event_sequences[key] = max(self._event_sequences.get(key, 0), pending_event['sequence'])
        self._death_retry_at = {}
        self._death_context_lock = threading.Lock()
        self._last_death_context = None
        self._latest_sample = None
        self._position_lock = threading.Lock()
        self._latest_position = None
        self._position_revision = 0
        self._position_sequence = 0
        self._position_sequence_session = None
        self._position_authority_epoch = 0
        self._position_authority_character_id = None
        self._position_authority_session_id = None
        self._position_authority_identity = None
        self.character_id = None
        self._current_identity = None
        self._rejected_identity = None
        self.session_id = None
        self._commands = _queue.Queue(maxsize=16)
        self._outgoing = _queue.Queue(maxsize=32)
        self._dedup = set()
        self._dedup_order = []
        self._profile_epoch = 0
        self._active_walk = None
        self._navigation_job_lock = threading.Lock()
        self._navigation_job = None
        self._navigation_lock = threading.Lock()
        self._navigation_sequence = 0
        self._latest_navigation_route = None
        self._navigation_route_sent_at = 0.0
        self._last_navigation_evidence = None
        self._active_navigation = None
        self._trace_requested_name = None
        self._server_clock_offset = 0.0
        self._last_control_sample_session = None
        self._last_control_sample_at = 0.0
        self._resource_revision = 0
        self._resource_baseline_required = True
        self._confirmed_resources = None
        self._latest_resources = None
        self._latest_resources_identity = None
        self._item_tracker = PassiveItemTracker()

    def update_map_monsters(self, identity, status, region, monsters, sample=None, observer_z=None):
        observation = {
            'identity': dict(identity), 'status': status, 'region': region,
            'monsters': [dict(monster) for monster in monsters], 'observed_at': _worker_utc_now(self),
        }
        if _number(observer_z) and abs(observer_z) <= 1000000:
            observation['observer_z'] = float(observer_z)
        try:
            self._map_samples.put_nowait(observation)
        except _queue.Full:
            try: self._map_samples.get_nowait()
            except _queue.Empty: pass
            try: self._map_samples.put_nowait(observation)
            except _queue.Full: pass
        if sample is not None:
            try:
                self._mob_samples.put_nowait(dict(sample))
            except _queue.Full:
                self.status = 'Mob observation queue is full; a sample needs operator attention.'
                _log('mob observation was not queued because its bounded worker queue is full')
                return False
        return True

    def update_map_npcs(self, identity, status, region, npcs, observer_z=None):
        observation = {
            'identity': dict(identity), 'status': status, 'region': region,
            'npcs': [dict(npc) for npc in npcs], 'observed_at': _worker_utc_now(self),
        }
        if _number(observer_z) and abs(observer_z) <= 1000000:
            observation['observer_z'] = float(observer_z)
        try:
            self._npc_samples.put_nowait(observation)
        except _queue.Full:
            try: self._npc_samples.get_nowait()
            except _queue.Empty: pass
            try: self._npc_samples.put_nowait(observation)
            except _queue.Full: pass
        return True

    def clear_map_npcs(self):
        while True:
            try: self._npc_samples.get_nowait()
            except _queue.Empty: break
        self._latest_npc_observation = None
        self._last_npc_sent_at = None

    def update_map_players(self, identity, status, region, players, observer_z=None):
        observation = {
            'identity': dict(identity), 'status': status, 'region': region,
            'players': [dict(player) for player in players], 'observed_at': _worker_utc_now(self),
        }
        if _number(observer_z) and abs(observer_z) <= 1000000:
            observation['observer_z'] = float(observer_z)
        try:
            self._player_samples.put_nowait(observation)
        except _queue.Full:
            try: self._player_samples.get_nowait()
            except _queue.Empty: pass
            try: self._player_samples.put_nowait(observation)
            except _queue.Full: pass
        return True

    def clear_map_players(self):
        while True:
            try: self._player_samples.get_nowait()
            except _queue.Empty: break
        self._latest_player_observation = None
        self._last_player_sent_at = None

    def _spool_queued_mob_samples(self):
        while True:
            sample = self._pending_mob_spool_sample
            if sample is None:
                try: sample = self._mob_samples.get_nowait()
                except _queue.Empty: break
            if not self._mob_spool.add(sample):
                self._pending_mob_spool_sample = sample
                self.status = 'Mob observation spool is unavailable or full; a sample needs operator attention.'
                _log('mob observation was not sent because durable local spooling failed')
                break
            self._pending_mob_spool_sample = None

    def _flush_map_observations(self, client):
        self._spool_queued_mob_samples()
        while True:
            try: self._latest_map_observation = self._map_samples.get_nowait()
            except _queue.Empty: break
        observation = self._latest_map_observation
        if (observation is not None and self.character_id is not None and self.session_id is not None and
                self._identity_key(observation.get('identity')) == self._identity_key(self._current_identity) and
                observation.get('observed_at') != self._last_map_sent_at):
            snapshot = {
                'status': observation['status'], 'character_id': self.character_id,
                'session_id': self.session_id, 'observed_at': observation['observed_at'],
                'region': observation['region'], 'monsters': observation['monsters'],
                'truncated': observation['status'] == 'truncated',
            }
            if observation.get('observer_z') is not None:
                snapshot['observer_z'] = observation['observer_z']
            client.send_json({'type': 'map.monsters', 'protocol_version': PROTOCOL_VERSION,
                              'map_snapshot': snapshot})
            self._last_map_sent_at = observation['observed_at']
        self._flush_map_npcs(client)
        self._flush_map_players(client)
        self._flush_navigation_route(client)
        now = _monotonic()
        for sample in self._mob_spool.pending():
            sample_id = sample.get('sample_id')
            if self._mob_retry_at.get(sample_id, 0.0) > now:
                continue
            client.send_json({'type': 'mob.sample', 'protocol_version': PROTOCOL_VERSION, 'sample': sample})
            self._mob_retry_at[sample_id] = now + 3.0
            break

    def _flush_map_npcs(self, client):
        while True:
            try: self._latest_npc_observation = self._npc_samples.get_nowait()
            except _queue.Empty: break
        observation = self._latest_npc_observation
        if (observation is None or self.character_id is None or self.session_id is None or
                self._identity_key(observation.get('identity')) != self._identity_key(self._current_identity) or
                observation.get('observed_at') == self._last_npc_sent_at):
            return
        snapshot = {
            'status': observation['status'], 'character_id': self.character_id,
            'session_id': self.session_id, 'observed_at': observation['observed_at'],
            'region': observation['region'], 'npcs': observation['npcs'],
            'truncated': observation['status'] == 'truncated',
        }
        if observation.get('observer_z') is not None:
            snapshot['observer_z'] = observation['observer_z']
        client.send_json({'type': 'map.npcs', 'protocol_version': PROTOCOL_VERSION,
                          'map_snapshot': snapshot})
        self._last_npc_sent_at = observation['observed_at']

    def _flush_map_players(self, client):
        while True:
            try: self._latest_player_observation = self._player_samples.get_nowait()
            except _queue.Empty: break
        observation = self._latest_player_observation
        if (observation is None or self.character_id is None or self.session_id is None or
                self._identity_key(observation.get('identity')) != self._identity_key(self._current_identity) or
                observation.get('observed_at') == self._last_player_sent_at):
            return
        snapshot = {
            'status': observation['status'], 'character_id': self.character_id,
            'session_id': self.session_id, 'observed_at': observation['observed_at'],
            'region': observation['region'], 'players': observation['players'],
            'truncated': observation['status'] == 'truncated',
        }
        if observation.get('observer_z') is not None:
            snapshot['observer_z'] = observation['observer_z']
        client.send_json({'type': 'map.players', 'protocol_version': PROTOCOL_VERSION,
                          'map_snapshot': snapshot})
        self._last_player_sent_at = observation['observed_at']

    def update_character(self, identity, state):
        self._replace_sample({'identity': dict(identity), 'state': dict(state)})

    def update_position(self, position, observed_at=None, identity=None):
        """Keep only the newest coordinate owned by the registered character epoch."""
        if not isinstance(position, dict):
            return False
        identity = dict(identity) if isinstance(identity, dict) else None
        with self._position_lock:
            if (self._position_authority_character_id is None or
                    self._position_authority_session_id is None or
                    self.character_id != self._position_authority_character_id or
                    self.session_id != self._position_authority_session_id):
                return False
            if identity is not None and identity != self._position_authority_identity:
                return False
            self._position_revision += 1
            self._latest_position = {
                'position': dict(position),
                'observed_at': observed_at or _worker_utc_now(self),
                'authority_epoch': self._position_authority_epoch,
            }
        return True

    def clear_position(self):
        with self._position_lock:
            self._position_revision += 1
            self._latest_position = None

    def invalidate_position_authority(self):
        """Fence callback observations before character/session ownership changes."""
        with self._position_lock:
            self._position_authority_epoch += 1
            self._position_revision += 1
            self._latest_position = None
            self._position_authority_character_id = None
            self._position_authority_session_id = None
            self._position_authority_identity = None

    def _adopt_position_session(self, session_id, identity=None):
        identity = dict(identity) if isinstance(identity, dict) else None
        with self._position_lock:
            changed = (
                self._position_authority_character_id != self.character_id or
                self._position_authority_session_id != session_id or
                self._position_authority_identity != identity
            )
            if changed:
                self._position_authority_epoch += 1
                self._position_revision += 1
                self._latest_position = None
            self._position_authority_character_id = self.character_id
            self._position_authority_session_id = session_id
            self._position_authority_identity = identity
            if self._position_sequence_session != session_id:
                self._position_sequence_session = session_id
                self._position_sequence = 0

    def _flush_realtime_position(self, client):
        with self._position_lock:
            sample = self._latest_position
            if sample is None:
                return False
            if (sample.get('authority_epoch') != self._position_authority_epoch or
                    self.character_id != self._position_authority_character_id or
                    self.session_id != self._position_authority_session_id or
                    self.character_id is None or self.session_id is None):
                if self._latest_position is sample:
                    self._position_revision += 1
                    self._latest_position = None
                return False
            session_id = self._position_authority_session_id
            character_id = self._position_authority_character_id
            if self._position_sequence_session != session_id:
                self._position_sequence_session = session_id
                self._position_sequence = 0
            sequence = self._position_sequence + 1
            revision = self._position_revision
            authority_epoch = self._position_authority_epoch
        client.send_json({
            'type': 'character.position',
            'protocol_version': PROTOCOL_VERSION,
            'character_id': character_id,
            'session_id': session_id,
            'sequence': sequence,
            'position': sample['position'],
            'sent_at': sample['observed_at'],
        })
        with self._position_lock:
            if (self._position_authority_epoch == authority_epoch and
                    self._position_sequence_session == session_id):
                self._position_sequence = sequence
            if self._position_revision == revision and self._latest_position is sample:
                self._latest_position = None
        return True

    def update_resources(self, identity, resources, position=None):
        self._queue_resource_sample({'identity': dict(identity), 'resources': resources,
                                     'position': dict(position) if isinstance(position, dict) else None,
                                     'observed_at': _worker_utc_now(self)})

    def update_resource_inputs(self, identity, inputs, config_dir, locale, position=None):
        self._queue_resource_sample({
            'identity': dict(identity),
            'inputs': inputs,
            'config_dir': config_dir,
            'locale': locale,
            'position': dict(position) if isinstance(position, dict) else None,
            'observed_at': _worker_utc_now(self),
        })

    def latest_resource_item_at_slot(self, slot):
        with self._alchemy_items_lock:
            item = self._latest_alchemy_items.get(slot)
            return dict(item) if isinstance(item, dict) else None

    def _update_alchemy_items(self, resources):
        indexed = {}
        for key in ('equipment', 'inventory'):
            container = resources.get(key) if isinstance(resources, dict) else None
            if not isinstance(container, dict) or container.get('availability') != 'observed':
                continue
            for row in container.get('slots', []):
                if (isinstance(row, dict) and isinstance(row.get('source_slot'), int) and
                        isinstance(row.get('item'), dict)):
                    indexed[row['source_slot']] = dict(row['item'])
        with self._alchemy_items_lock:
            self._latest_alchemy_items = indexed

    def _derive_resource_events(self, identity, resources, sample):
        identity_key = self._identity_key(identity)
        if identity_key != self._event_diff_identity or self.session_id != self._event_diff_session:
            self._event_baseline_required = True
            self._event_diff_identity = identity_key
            self._event_diff_session = self.session_id
            with self._pending_drop_lock:
                self._pending_drop_events = [entry for entry in self._pending_drop_events
                                             if entry['session_id'] == self.session_id and
                                             entry['identity_key'] == identity_key]
        if self._resource_sample_gap:
            self._resource_sample_gap = False
            self._event_baseline_required = True
            with self._pending_drop_lock:
                self._pending_drop_events = []
        current = _event_resource_snapshot(resources)
        if current is None:
            self._event_baseline_required = True
            self._event_diff = None
            with self._pending_drop_lock:
                self._pending_drop_events = []
            return
        previous = self._event_diff
        if self._event_baseline_required or previous is None:
            self._event_diff = current
            self._event_baseline_required = False
            with self._pending_drop_lock:
                self._pending_drop_events = []
            return
        observed_at = sample.get('observed_at') if isinstance(sample, dict) else None
        position = sample.get('position') if isinstance(sample, dict) else None

        for group, join_kind, leave_kind, ref in (
                ('party', 'party.member_joined', 'party.member_left', 'party'),
                ('academy', 'academy.member_joined', 'academy.member_left', 'academy'),
                ('pets', 'pet.summoned', 'pet.dismissed', 'pet')):
            before = previous.get(group)
            after = current.get(group)
            if before is None or after is None:
                continue
            for key in sorted(set(after) - set(before)):
                self._emit_state_change(identity, join_kind, ref, key, after[key], observed_at, position)
            for key in sorted(set(before) - set(after)):
                self._emit_state_change(identity, leave_kind, ref, key, before[key], observed_at, position)

        old_containers = previous.get('containers', {})
        new_containers = current.get('containers', {})
        common = set(old_containers) & set(new_containers)
        changed = set(old_containers) ^ set(new_containers)
        # A newly available or dismissed container has no comparable baseline.
        # Suppress its item models on this sample, since moving one of them into
        # a stable container could otherwise look like an acquisition.
        uncertain = set()
        for key in changed:
            uncertain.update((old_containers.get(key) or new_containers.get(key) or {}).keys())
        self._emit_item_deltas(identity,
                               {key: old_containers[key] for key in common},
                               {key: new_containers[key] for key in common},
                               observed_at, position, uncertain)
        self._event_diff = current

    def _emit_state_change(self, identity, kind, source_ref, key, value, occurred_at, position):
        payload = {'member_id': key, 'member': value}
        if source_ref == 'pet':
            payload = {'pet_id': key, 'pet': value}
        self._queue_derived_event(identity, kind, source_ref, payload, occurred_at, position)

    def _emit_item_deltas(self, identity, before, after, occurred_at, position, uncertain=()):
        signatures = set()
        for container in before.values(): signatures.update(container)
        for container in after.values(): signatures.update(container)
        def inventory_slots(containers):
            slots = {}
            for record in containers.get('inventory', {}).values():
                slots.update(record.get('slot_items', {}))
            return slots

        before_slots = inventory_slots(before)
        after_slots = inventory_slots(after)
        new_model_slots = {}
        for slot, slot_item in after_slots.items():
            # Only a formerly empty bag slot can identify one particular item.
            if slot in before_slots:
                continue
            model = slot_item.get('model')
            if isinstance(model, int) and not isinstance(model, bool) and model > 0:
                new_model_slots.setdefault(model, []).append((slot, slot_item))
        for signature in sorted(signatures):
            if signature in uncertain:
                continue
            old_total = sum(entries.get(signature, {}).get('quantity', 0) for entries in before.values())
            new_total = sum(entries.get(signature, {}).get('quantity', 0) for entries in after.values())
            if old_total == new_total:
                gains, losses = [], []
                for container_key in before:
                    old = before[container_key].get(signature, {}).get('quantity', 0)
                    new = after[container_key].get(signature, {}).get('quantity', 0)
                    if new > old: gains.append((container_key, new - old, after[container_key][signature]))
                    elif old > new: losses.append((container_key, old - new, before[container_key][signature]))
                if len(gains) == 1 and len(losses) == 1 and gains[0][1] == losses[0][1]:
                    source_key, quantity, source_item = losses[0]
                    dest_key, _, dest_item = gains[0]
                    item = dest_item.get('item') or source_item.get('item') or {}
                    if _item_has_identity(item):
                        payload = {'item': item, 'quantity_delta': quantity,
                                   'source_container': _container_reference(source_key, source_item),
                                   'destination_container': _container_reference(dest_key, dest_item),
                                   'method': 'observed_container_delta'}
                        self._queue_derived_event(identity, 'item.transferred', 'item_container', payload,
                                                  occurred_at, position, item)
                elif gains or losses:
                    for container_key, quantity, record in gains:
                        item, selected, ambiguous = _event_gain_item(
                            before[container_key].get(signature), record, quantity)
                        if _item_has_identity(item):
                            payload = {'item': item, 'quantity_delta': quantity,
                                       'destination_container': _container_reference(container_key, record),
                                       'acquisition_method': 'unknown'}
                            if selected:
                                payload['destination_container']['slot'] = selected[0]
                            if ambiguous:
                                payload['item_instance_unobserved'] = True
                            self._queue_derived_event(identity, 'item.quantity_increased', 'item_container', payload,
                                                      occurred_at, position, item)
                    for container_key, quantity, record in losses:
                        item = record.get('item') or {}
                        if _item_has_identity(item):
                            payload = {'item': item, 'quantity_delta': quantity,
                                       'source_container': _container_reference(container_key, record)}
                            self._queue_derived_event(identity, 'item.quantity_decreased', 'item_container', payload,
                                                      occurred_at, position, item)
                continue

            positive, negative = (new_total - old_total, 0) if new_total > old_total else (0, old_total - new_total)
            deltas = []
            for container_key in before:
                old = before[container_key].get(signature, {}).get('quantity', 0)
                new = after[container_key].get(signature, {}).get('quantity', 0)
                if new > old: deltas.append(('gain', container_key, new - old, after[container_key][signature]))
                elif old > new: deltas.append(('loss', container_key, old - new, before[container_key][signature]))
            changes = [entry for entry in deltas if entry[0] == ('gain' if positive else 'loss')]
            remaining = positive or negative
            for direction, container_key, quantity, record in changes:
                amount = min(quantity, remaining)
                remaining -= amount
                if direction == 'gain':
                    item, selected, ambiguous = _event_gain_item(
                        before[container_key].get(signature), record, amount)
                else:
                    item, selected, ambiguous = record.get('item') or {}, None, False
                if not amount or not _item_has_identity(item):
                    continue
                if direction == 'gain':
                    kind = 'item.acquired' if old_total == 0 else 'item.quantity_increased'
                    payload = {'item': item, 'quantity_delta': amount,
                               'destination_container': _container_reference(container_key, record)}
                    if selected:
                        payload['destination_container']['slot'] = selected[0]
                    if ambiguous:
                        payload['item_instance_unobserved'] = True
                    payload['acquisition_method'] = 'unknown'
                    model = item.get('model')
                    candidates = new_model_slots.get(model, [])
                    if (container_key == 'inventory' and amount == 1 and len(changes) == 1 and
                            isinstance(model, int) and len(candidates) == 1 and
                            candidates[0][0] in record.get('slots', []) and
                            candidates[0][1].get('quantity', 1) == 1):
                        slot, observed_item = candidates[0]
                        inventory_quantity = sum(
                            entry.get('quantity', 0)
                            for entry in after.get('inventory', {}).values()
                            if (entry.get('item') or {}).get('model') == model)
                        drop_event_id = self._claim_drop_for_inventory_gain(identity, observed_item,
                                                                              inventory_quantity)
                        if drop_event_id:
                            payload['item'] = observed_item
                            payload['destination_container'] = {'type': 'inventory', 'slot': slot}
                            payload['drop_event_id'] = drop_event_id
                            payload.pop('item_instance_unobserved', None)
                            item = observed_item
                else:
                    kind = 'item.quantity_decreased'
                    payload = {'item': item, 'quantity_delta': amount,
                               'source_container': _container_reference(container_key, record)}
                self._queue_derived_event(identity, kind, 'item_container', payload, occurred_at, position, item)

    def register_drop_for_enrichment(self, identity, model, event_id, inventory_quantity):
        if (not isinstance(identity, dict) or not isinstance(model, int) or model <= 0 or
                not _validate_agent_id(event_id) or not _validate_agent_id(self.session_id) or
                not isinstance(inventory_quantity, int) or inventory_quantity < 0 or
                self._identity_key(identity) != self._identity_key(self._current_identity)):
            return False
        with self._pending_drop_lock:
            now = _monotonic()
            self._pending_drop_events = [entry for entry in self._pending_drop_events
                                         if entry['expires_at'] > now and entry['session_id'] == self.session_id]
            if len(self._pending_drop_events) >= 32:
                return False
            self._pending_drop_events.append({
                'event_id': event_id, 'model': model, 'session_id': self.session_id,
                'identity_key': self._identity_key(identity),
                'inventory_quantity': inventory_quantity,
                'expires_at': now + DROP_ENRICHMENT_WINDOW_SECONDS,
            })
        return True

    def has_pending_drop_enrichment(self, identity):
        with self._pending_drop_lock:
            now = _monotonic()
            key = self._identity_key(identity)
            self._pending_drop_events = [entry for entry in self._pending_drop_events
                                         if entry['expires_at'] > now and entry['session_id'] == self.session_id and
                                         entry['identity_key'] == key]
            return bool(self._pending_drop_events)

    def _claim_drop_for_inventory_gain(self, identity, item, inventory_quantity):
        model = item.get('model') if isinstance(item, dict) else None
        if not isinstance(model, int) or isinstance(model, bool) or model <= 0:
            return None
        with self._pending_drop_lock:
            now = _monotonic()
            key = self._identity_key(identity)
            self._pending_drop_events = [entry for entry in self._pending_drop_events
                                         if entry['expires_at'] > now and entry['session_id'] == self.session_id and
                                         entry['identity_key'] == key]
            matches = [entry for entry in self._pending_drop_events if entry['model'] == model]
            if len(matches) != 1 or inventory_quantity <= matches[0]['inventory_quantity']:
                return None
            selected = matches[0]
            self._pending_drop_events.remove(selected)
            return selected['event_id']

    def _queue_derived_event(self, identity, kind, source_ref, payload, occurred_at, position, item=None):
        event = {
            'event_id': str(uuid.uuid4()), 'schema_version': 1, 'kind': kind,
            'category': kind.split('.', 1)[0], 'occurred_at': occurred_at or _worker_utc_now(self),
            'source': 'phbot.state_diff', 'source_ref': source_ref, 'payload': payload,
        }
        if isinstance(position, dict):
            region = position.get('region')
            if _valid_position_region(region):
                event['region'] = region
                zone = _zone_name_for_region(region)
                if zone:
                    event['zone'] = zone
            for axis in ('x', 'y', 'z'):
                value = position.get(axis)
                if _number(value) and abs(value) <= 1000000:
                    event[axis] = float(value)
        if isinstance(item, dict):
            model = item.get('model')
            code = item.get('servername')
            if isinstance(model, int) and not isinstance(model, bool) and model >= 0: event['item_model'] = model
            if isinstance(code, str) and code: event['item_code'] = code[:128]
        self.queue_event(identity, event)

    def capture_joymax_packet(self, opcode, data):
        """Copy only allowlisted packets; decoding is performed by the network worker."""
        return self._item_tracker.enqueue(opcode, data)

    def _queue_verified_pet_pickup(self, identity, item, quantity_delta, pet_id,
                                  source_slot, packet_observation, position=None):
        """Queue a receipt emitted only by a validated pet operation reconciler."""
        if (self._current_identity is None or not isinstance(identity, dict) or
                self._identity_key(identity) != self._identity_key(self._current_identity) or
                not _validate_agent_id(self.character_id) or not _validate_agent_id(self.session_id) or
                not isinstance(item, dict) or not _item_has_identity(item) or
                not isinstance(quantity_delta, int) or isinstance(quantity_delta, bool) or
                quantity_delta <= 0 or quantity_delta > 1000000000 or
                not isinstance(pet_id, str) or not pet_id or len(pet_id) > 64 or
                not isinstance(source_slot, int) or isinstance(source_slot, bool) or
                source_slot < 0 or source_slot > 65535 or
                not isinstance(packet_observation, dict)):
            return False
        observation_id = packet_observation.get('observation_id')
        sequence = packet_observation.get('sequence')
        decoder = packet_observation.get('decoder_version')
        if (not isinstance(observation_id, str) or not observation_id or len(observation_id) > 160 or
                not isinstance(sequence, str) or not sequence.isdigit() or not sequence.strip('0') or
                len(sequence) > 20 or not isinstance(decoder, str) or not decoder or len(decoder) > 64):
            return False
        try:
            snapshot = json.loads(json.dumps(item, separators=(',', ':'), ensure_ascii=False))
            payload = {
                'item': snapshot,
                'quantity_delta': quantity_delta,
                'destination_container': {'type': 'pets', 'id': pet_id, 'slot': source_slot},
                'acquisition_method': 'pet_pickup',
                'packet_observation': {
                    'observation_id': observation_id,
                    'sequence': sequence,
                    'decoder_version': decoder,
                },
            }
            if len(json.dumps(payload, separators=(',', ':'), ensure_ascii=False).encode('utf-8')) > MAX_EVENT_PAYLOAD_BYTES:
                return False
        except Exception:
            return False
        model = item.get('model')
        code = item.get('servername')
        session = self.session_id
        stable_identity = session + ':' + observation_id
        event_id = str(uuid.uuid5(uuid.NAMESPACE_URL, 'phmon:pet-pickup:' + stable_identity))
        event = {
            'event_id': event_id, 'schema_version': 1,
            'kind': 'item.acquired', 'category': 'item',
            'occurred_at': packet_observation.get('occurred_at') or _worker_utc_now(self),
            'source': 'joymax.pet_inventory', 'source_ref': '0xB034',
            'payload': payload,
            'dedupe_key': 'pet-pickup:' + hashlib.sha256(stable_identity.encode('utf-8')).hexdigest(),
        }
        if isinstance(model, int) and not isinstance(model, bool) and model >= 0:
            event['item_model'] = model
        if isinstance(code, str) and code:
            event['item_code'] = code[:128]
        if isinstance(position, dict):
            region = position.get('region')
            if _valid_position_region(region):
                event['region'] = region
            for axis in ('x', 'y', 'z'):
                value = position.get(axis)
                if _number(value) and abs(value) <= 1000000:
                    event[axis] = float(value)
        return self.queue_event(identity, event)

    def queue_death_event(self, identity, event):
        if isinstance(event, dict):
            event = dict(event)
            event.setdefault('schema_version', 1)
            event.setdefault('kind', 'character.died')
            event.setdefault('category', 'character')
        return self.queue_event(identity, event)

    def queue_event(self, identity, event):
        if not isinstance(event, dict) or not event.get('kind') or not event.get('category'):
            _log('activity event is missing its canonical type')
            return False
        if (isinstance(identity, dict) and self._current_identity is not None and
                self._identity_key(identity) != self._identity_key(self._current_identity)):
            _log('activity event was rejected because its character identity is no longer active')
            return False
        if _item_filters_enabled(self.config):
            sort_type = event.get('_phmon_item_sort_type')
            if not isinstance(sort_type, str) and isinstance(identity, dict):
                sort_type = _cached_item_sort_type(identity.get('server'), event.get('item_model'))
            if _ignore_item_event(self.config, sort_type):
                return False
        record = dict(event)
        record.pop('_phmon_item_sort_type', None)
        if isinstance(identity, dict):
            server = identity.get('server')
            character_name = identity.get('name')
            record['server'] = str(server or '').strip()[:100] or None
            record['character_name'] = str(character_name or '').strip()[:64] or None
        elif record.get('server') is not None or record.get('character_name') is not None:
            record['server'] = str(record.get('server') or '').strip()[:100] or None
            record['character_name'] = str(record.get('character_name') or '').strip()[:64] or None
        context = None
        if (isinstance(identity, dict) and self._current_identity is not None and
                self._identity_key(identity) == self._identity_key(self._current_identity) and
                _validate_agent_id(self.character_id) and _validate_agent_id(self.session_id)):
            context = {'character_id': self.character_id, 'session_id': self.session_id}
        if context is None and isinstance(identity, dict):
            with self._death_context_lock:
                previous = self._last_death_context
                if previous and self._identity_key(identity) == self._identity_key(previous['identity']):
                    context = {
                        'character_id': previous['character_id'],
                        'session_id': previous['session_id'],
                    }
        if context:
            record.update(context)
            record['deferred_session_binding'] = False
            if record.get('sequence') is None:
                record['sequence'] = self._next_event_sequence(context['session_id'])
        else:
            record['character_id'] = None
            record['session_id'] = None
            record['deferred_session_binding'] = bool(record.get('character_name') and record.get('server'))
            if record['deferred_session_binding']:
                payload = record.get('payload')
                if isinstance(payload, dict):
                    payload = dict(payload)
                    payload['session_binding'] = 'deferred'
                    record['payload'] = payload
        try:
            self._event_samples.put_nowait(record)
            return True
        except _queue.Full:
            self.status = 'Activity event queue is full; an occurrence needs operator attention.'
            _log('activity event was not queued because the event queue is full')
            return False

    def _next_event_sequence(self, session_id):
        with self._event_sequence_lock:
            sequence = self._event_sequences.get(session_id, 0) + 1
            self._event_sequences[session_id] = sequence
            return sequence

    def _queue_resource_sample(self, sample):
        try:
            self._resource_samples.put_nowait(sample)
        except _queue.Full:
            try:
                self._resource_samples.get_nowait()
            except _queue.Empty:
                pass
            try:
                self._resource_samples.put_nowait(sample)
            except _queue.Full:
                pass
            self._resource_sample_gap = True
            self.status = 'Resource sampling fell behind; event baseline will be reset.'

    def leave_character(self):
        if self._active_walk is not None:
            self._finish_walk('unknown', 'character_left_during_walk', 'unverified')
        self._discard_pending_commands('character_left')
        self.invalidate_position_authority()
        self._replace_sample({'leave': True})

    def _replace_sample(self, value):
        try:
            self._samples.put_nowait(value)
        except _queue.Full:
            try:
                self._samples.get_nowait()
            except _queue.Empty:
                pass
            try:
                self._samples.put_nowait(value)
            except _queue.Full:
                pass

    def start(self):
        if self._thread is not None and self._thread.is_alive():
            return
        self._thread = threading.Thread(target=self._run, name='PhMon-network')
        self._thread.daemon = True
        self._thread.start()

    def join(self, timeout=None):
        if self._thread is not None:
            self._thread.join(timeout)

    def stop(self):
        self.stop_event.set()
        self._cancel_navigation_generation('plugin_stopped')
        with self._socket_lock:
            client = self._socket
        if client is not None:
            try: client.close()
            except Exception: pass

    def _set_socket(self, value):
        with self._socket_lock:
            self._socket = value

    def _clear_navigation_route(self):
        self._cancel_navigation_generation('navigation_session_changed')
        with self._navigation_lock:
            self._navigation_sequence = 0
            self._latest_navigation_route = None
            self._navigation_route_sent_at = 0.0
        self._last_navigation_evidence = None
        self._active_navigation = None

    def _run(self):
        backoff = ReconnectBackoff()
        while not self.stop_event.is_set():
            self._spool_queued_events()
            self._spool_queued_mob_samples()
            client = None
            try:
                self.status = 'Connecting to PhMon backend...'
                client = self.websocket_factory(self.config['backend_url'], self.config['agent_token'])
                self._set_socket(client)
                client.connect()
                client.send_json({
                    'type': 'hello',
                    'protocol_version': PROTOCOL_VERSION,
                    'agent_id': self.config['agent_id'],
                    'plugin_version': pVersion,
                    'phbot_version': self.phbot_version,
                    'sent_at': _utc_now(),
                })
                ack = client.receive_json(timeout=10.0)
                interval = self._validate_ack(ack)
                if PROTOCOL_VERSION >= 3:
                    client.send_json(self._capability_frame())
                backoff.reset()
                self.status = 'Connected to PhMon backend.'
                _log('connected to backend')
                next_heartbeat = _monotonic() + interval
                self._clear_navigation_route()
                self.invalidate_position_authority()
                self.character_id = None
                self.session_id = None
                self._current_identity = None
                self._rejected_identity = None
                self._resource_revision = 0
                self._resource_baseline_required = True
                self._confirmed_resources = None
                self._latest_resources = None
                self._latest_resources_identity = None
                self._last_map_sent_at = None
                self._last_npc_sent_at = None
                self._last_player_sent_at = None
                self._item_tracker.reset('backend_reconnect')
                # Callbacks may have queued a leave while the backend was
                # unavailable. Apply the newest queued fact before replaying
                # the last sample so a departed character is never resurrected.
                self._restore_latest_sample(client)

                while not self.stop_event.is_set():
                    try:
                        sample = self._samples.get_nowait()
                        if sample.get('leave'):
                            if self.character_id is not None:
                                client.send_json({'type':'character.left','protocol_version':PROTOCOL_VERSION,'character_id':self.character_id,'sent_at':_utc_now()})
                            self.character_id = None
                            self.session_id = None
                            self._current_identity = None
                            self._clear_navigation_route()
                            self._latest_sample = None
                            self.clear_position()
                            self._latest_resources = None
                            self._latest_resources_identity = None
                            self._latest_map_observation = None
                            self._last_map_sent_at = None
                            self.clear_map_npcs()
                            self.clear_map_players()
                            self._resource_baseline_required = True
                            self._confirmed_resources = None
                            self._item_tracker.reset('character_left')

                        else:
                            self._latest_sample = sample
                            self._publish_sample(client, sample, False)
                    except _queue.Empty:
                        pass
                    self._flush_death_events(client)
                    self._flush_realtime_position(client)
                    self._flush_map_observations(client)
                    try:
                        resource_sample = self._resource_samples.get_nowait()
                        if self.character_id is not None and resource_sample.get('identity') == self._current_identity:
                            if 'inputs' in resource_sample:
                                identity = resource_sample['identity']
                                protocol, protocol_reason = detect_item_protocol_detail(
                                    resource_sample.get('config_dir'),
                                    identity.get('server'),
                                    resource_sample.get('locale'),
                                )
                                self._item_tracker.set_protocol(protocol, protocol_reason)
                                resources = normalize_resource_inputs(
                                    resource_sample['inputs'],
                                    resource_sample.get('config_dir'),
                                    identity.get('server'),
                                    resource_sample.get('locale'),
                                )
                            else:
                                resources = resource_sample['resources']
                            resources = self._item_tracker.decorate(resources, self.session_id)
                            self._derive_resource_events(resource_sample['identity'], resources, resource_sample)
                            self._update_alchemy_items(resources)
                            self._latest_resources = resources
                            self._latest_resources_identity = self._identity_key(resource_sample['identity'])
                            self._send_resource_snapshot(client, self._latest_resources, refresh_party=True)
                    except _queue.Empty:
                        pass
                    if (self._latest_resources is not None and
                            self._latest_resources_identity == self._identity_key(self._current_identity)
                            if self._current_identity is not None else False):
                        self._item_tracker.decorate(self._latest_resources, self.session_id)
                        self._send_resource_snapshot(client, self._latest_resources)
                    now = _monotonic()
                    if now >= next_heartbeat:
                        client.send_json({
                            'type': 'heartbeat',
                            'protocol_version': PROTOCOL_VERSION,
                            'sent_at': _utc_now(),
                        })
                        next_heartbeat = now + interval
                    wait = max(0.001, min(next_heartbeat - now, NETWORK_LOOP_MAX_WAIT_SECONDS))
                    self._drain_server_messages(client, wait)
                    self._flush_results(client)
            except Exception as error:
                if not self.stop_event.is_set():
                    self.status = 'Backend unavailable; retrying...'
                    _log('backend unavailable (' + error.__class__.__name__ + '); reconnecting')
            finally:
                # Commands and results belong to the socket generation that
                # accepted them. Never execute or publish them on a reconnect.
                self._clear_pending_commands()
                while True:
                    try: self._outgoing.get_nowait()
                    except _queue.Empty: break
                while True:
                    try: self._resource_samples.get_nowait()
                    except _queue.Empty: break
                if client is not None:
                    client.close()
                self._set_socket(None)
                self.invalidate_position_authority()
                self.character_id = None
                self._current_identity = None
                self.session_id = None
                self._resource_revision = 0
                self._resource_baseline_required = True
                self._confirmed_resources = None
                self._latest_resources = None
                self._latest_resources_identity = None
                self._item_tracker.reset('backend_reconnect')

            if not self.stop_event.is_set():
                delay = backoff.next_delay()
                deadline = _monotonic() + delay
                while not self.stop_event.is_set():
                    self._spool_queued_events()
                    self._spool_queued_mob_samples()
                    remaining = deadline - _monotonic()
                    if remaining <= 0 or self.stop_event.wait(min(0.25, remaining)):
                        break

    def _drain_server_messages(self, client, timeout, max_frames=32):
        # One read per iteration used to fall behind the acknowledgements from
        # several outgoing frames. Commands then waited behind an ever-growing
        # backlog. Drain a bounded burst; retain time for sampling and heartbeats.
        for index in range(max_frames):
            message = client.receive_json(timeout=timeout if index == 0 else 0.001)
            if message is None:
                return index
            self._handle_server_message(message)
        return max_frames

    def _publish_sample(self, client, sample, snapshot):
        identity, state = sample['identity'], sample['state']
        identity_key = self._identity_key(identity)
        if self._rejected_identity == identity_key:
            return
        if self._rejected_identity is not None and self._rejected_identity != identity_key:
            self._rejected_identity = None
        if identity != self._current_identity or self.character_id is None:
            self.invalidate_position_authority()
            if self._current_identity is not None and identity != self._current_identity:
                self._discard_pending_commands('character_changed')
            if self.character_id is not None and self._current_identity is not None:
                previous_key = (self._current_identity.get('server','').lower(), self._current_identity.get('name','').lower())
                next_key = (identity.get('server','').lower(), identity.get('name','').lower())
                if previous_key != next_key:
                    client.send_json({'type':'character.left','protocol_version':PROTOCOL_VERSION,'character_id':self.character_id,'session_id':self.session_id,'sent_at':_worker_utc_now(self)})
                    self.character_id = None
                    self.session_id = None
            self._profile_epoch += 1
            self._clear_navigation_route()
            self._trace_requested_name = None
            self._item_tracker.reset('character_or_profile_changed')
            client.send_json({'type':'character.identify','protocol_version':PROTOCOL_VERSION,'server':identity['server'],'name':identity['name'],'guild':identity.get('guild',''),'sent_at':_worker_utc_now(self)})
            reply = self._wait_for_registration(client)
            if not isinstance(reply,dict) or reply.get('type')!='character.registered' or reply.get('protocol_version')!=PROTOCOL_VERSION or not _validate_agent_id(reply.get('character_id')) or not _validate_agent_id(reply.get('session_id')):
                raise WebSocketClosed('character registration rejected')
            self.character_id = reply['character_id']
            self.session_id = reply['session_id']
            self._adopt_position_session(self.session_id, identity)
            self._current_identity = identity
            # The first capability frame is sent before character registration.
            # Rescope recall support now that the game server is known.
            client.send_json(self._capability_frame())
            with self._death_context_lock:
                self._last_death_context = {
                    'identity': dict(identity),
                    'character_id': self.character_id,
                    'session_id': self.session_id,
                }
            self._resource_revision = 0
            self._resource_baseline_required = True
            self._confirmed_resources = None
            snapshot = True
        client.send_json({'type':'character.snapshot' if snapshot else 'character.state','protocol_version':PROTOCOL_VERSION,'character_id':self.character_id,'session_id':self.session_id,'state':state,'sent_at':_worker_utc_now(self)})

    def _flush_navigation_route(self, client):
        with self._navigation_lock:
            route = self._latest_navigation_route
            if (route is None or self.character_id != route.get('character_id') or
                    self.session_id != route.get('session_id')):
                return False
            now = _monotonic()
            if now - self._navigation_route_sent_at < 5.0:
                return False
            payload = dict(route)
            self._navigation_route_sent_at = now
        # The server accepts route evidence only after it has completed the
        # matching command. Preserve frame order on this socket.
        self._flush_results(client)
        client.send_json({'type': 'navigation.route', 'protocol_version': PROTOCOL_VERSION, 'route': payload})
        return True

    def _publish_navigation_route(self, message):
        evidence = self._last_navigation_evidence
        self._last_navigation_evidence = None
        if (not isinstance(evidence, dict) or not self.character_id or not self.session_id or
                message.get('character_id') != self.character_id or message.get('session_id') != self.session_id):
            return False
        with self._navigation_lock:
            sequence = self._navigation_sequence + 1
            route = {
                'schema_version': 1,
                'command_id': message.get('command_id'),
                'character_id': self.character_id,
                'session_id': self.session_id,
                'route_sequence': sequence,
                'invoked_at': evidence['invoked_at'],
                'instructions': evidence['instructions'],
            }
            if evidence.get('source') is not None:
                source = evidence['source']
                route['source'] = {
                    'region': source['region'], 'x': source['x'], 'y': source['y'],
                    'z': source['z'], 'observed_at': source['observed_at'],
                }
            try:
                encoded_size = len(json.dumps({
                    'type': 'navigation.route', 'protocol_version': PROTOCOL_VERSION, 'route': route,
                }, separators=(',', ':'), allow_nan=False).encode('utf-8'))
            except Exception:
                return False
            if encoded_size > 64 * 1024:
                return False
            self._navigation_sequence = sequence
            self._latest_navigation_route = route
            self._navigation_route_sent_at = 0.0
            self._active_navigation = {
                'command_id': message.get('command_id'),
                'route_sequence': sequence,
                'epoch': self._profile_epoch,
            }
        return True

    def _send_resource_snapshot(self, client, value, refresh_party=False):
        if not isinstance(value, dict) or self.character_id is None or self.session_id is None:
            return False
        full = self._resource_baseline_required or self._confirmed_resources is None
        resource_values = dict(value)
        # Serialize a complete immutable comparison view before emitting any
        # chunks. The resource objects may be mutated in place by the collector
        # between polls, so retaining references would hide nested changes.
        try:
            serialized_resources = {
                key: json.dumps(resource, sort_keys=True, separators=(',', ':'), allow_nan=False)
                for key, resource in resource_values.items()
            }
        except Exception:
            return False
        changed = resource_values if full else {}
        if not full:
            previous = self._confirmed_resources or {}
            for key, resource_json in serialized_resources.items():
                if resource_json != previous.get(key):
                    changed[key] = resource_values[key]
            # Party membership can stay unchanged for hours. Refresh this bounded
            # observation so controls can distinguish a fresh check from stale data.
            if refresh_party and 'party' in resource_values:
                changed['party'] = resource_values['party']
            if not changed:
                return True

        base_fields = {
            'type': 'resource.snapshot' if full else 'resource.delta',
            'protocol_version': PROTOCOL_VERSION,
            'character_id': self.character_id,
            'session_id': self.session_id,
            'revision': self._resource_revision + 1,
            'base_revision': 0 if full else self._resource_revision,
            'full': full,
            'sent_at': _utc_now(),
        }
        chunks = []
        current_chunk = {}
        for key in sorted(changed):
            candidate = dict(current_chunk)
            candidate[key] = changed[key]
            probe = dict(base_fields)
            probe.update({'chunk_index': 0, 'chunk_count': 1, 'resources': candidate})
            try:
                encoded_size = len(json.dumps(probe, separators=(',', ':'), allow_nan=False).encode('utf-8'))
            except Exception:
                return False
            if encoded_size > MAX_MESSAGE_BYTES - 512:
                if not current_chunk:
                    _log('a resource observation exceeds the per-frame limit; waiting for a smaller observation')
                    return False
                chunks.append(current_chunk)
                current_chunk = {key: changed[key]}
                probe['resources'] = current_chunk
                try:
                    encoded_size = len(json.dumps(probe, separators=(',', ':'), allow_nan=False).encode('utf-8'))
                except Exception:
                    return False
                if encoded_size > MAX_MESSAGE_BYTES - 512:
                    _log('a resource observation exceeds the per-frame limit; waiting for a smaller observation')
                    return False
            else:
                current_chunk = candidate
        if current_chunk:
            chunks.append(current_chunk)
        if not chunks or len(chunks) > 12:
            _log('resource snapshot exceeds the bounded chunk limit')
            return False
        frames = []
        for index, resources in enumerate(chunks):
            frame = dict(base_fields)
            frame.update({'chunk_index': index, 'chunk_count': len(chunks), 'resources': resources})
            try:
                encoded = json.dumps(frame, separators=(',', ':'), allow_nan=False).encode('utf-8')
            except Exception:
                return False
            if len(encoded) > MAX_MESSAGE_BYTES:
                return False
            frames.append(frame)
        for frame in frames:
            client.send_json(frame)
        self._resource_revision += 1
        self._resource_baseline_required = False
        self._confirmed_resources = serialized_resources
        return True

    def _handle_character_rejected(self, message):
        if message.get('protocol_version') != PROTOCOL_VERSION or message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
            raise WebSocketClosed('invalid character rejection')
        self._discard_pending_commands('session_superseded')
        self._clear_navigation_route()
        self._rejected_identity = self._identity_key(self._current_identity)
        self.invalidate_position_authority()
        self.character_id = None
        self.session_id = None
        self.status = 'Character observation superseded; waiting for a new character observation.'

    @staticmethod
    def _identity_key(identity):
        if not isinstance(identity, dict):
            return None
        return (identity.get('server', '').lower(), identity.get('name', '').lower())

    def _restore_latest_sample(self, client):
        # Drain pending callbacks before replaying state after reconnect. The
        # bounded queue contains the most recent callback fact, including leave.
        pending_sample = None
        while True:
            try:
                pending_sample = self._samples.get_nowait()
            except _queue.Empty:
                break
        if pending_sample is not None:
            self._latest_sample = None if pending_sample.get('leave') else pending_sample
        if self._latest_sample is not None:
            self._publish_sample(client, self._latest_sample, True)

    def _validate_ack(self, ack):
        if not isinstance(ack, dict) or ack.get('type') != 'hello.ack':
            raise WebSocketClosed('hello acknowledgement required')
        if ack.get('protocol_version') != PROTOCOL_VERSION:
            raise WebSocketClosed('unsupported protocol version')
        interval = ack.get('heartbeat_interval_seconds', DEFAULT_HEARTBEAT_INTERVAL)
        timeout = ack.get('heartbeat_timeout_seconds', DEFAULT_HEARTBEAT_TIMEOUT)
        if not isinstance(interval, int) or interval < 1:
            raise WebSocketClosed('invalid heartbeat interval')
        if not isinstance(timeout, int) or timeout <= interval:
            raise WebSocketClosed('invalid heartbeat timeout')
        try:
            self._server_clock_offset = _utc_epoch(ack.get('server_time')) - time.time()
        except Exception:
            if ack.get('protocol_version') >= 3:
                raise WebSocketClosed('server clock evidence required')
        return interval

    def _wait_for_registration(self, client):
        deadline = _monotonic() + 5.0
        while _monotonic() < deadline:
            message = client.receive_json(timeout=max(0.05, min(0.5, deadline - _monotonic())))
            if message is None: continue
            if not isinstance(message,dict): raise WebSocketClosed('invalid server application frame')
            if message.get('type') == 'character.registered': return message
            if message.get('type') == 'command.execute':
                # The newly claimed server session is not yet active in this worker.
                # Reject it as known-unexecuted, then keep waiting for registration.
                self._accept_command(message)
                self._flush_results(client)
                continue
            if message.get('type') == 'command.revoke':
                self._revoke_session(message)
                continue
            raise WebSocketClosed('unexpected frame during character registration')
        raise WebSocketClosed('character registration timed out')

    def _handle_server_message(self, message):
        if not isinstance(message,dict): raise WebSocketClosed('invalid server application frame')
        if message.get('type') == 'character.rejected': self._handle_character_rejected(message)
        elif message.get('type') == 'event.batch.ack':
            if message.get('protocol_version') != PROTOCOL_VERSION or not isinstance(message.get('results'), list) or len(message['results']) > MAX_EVENT_BATCH_SIZE:
                raise WebSocketClosed('invalid event batch acknowledgement')
            seen = set()
            for result in message['results']:
                if (not isinstance(result, dict) or not _validate_agent_id(result.get('event_id')) or
                        result.get('status') not in ('persisted', 'rejected', 'retry') or result['event_id'] in seen):
                    raise WebSocketClosed('invalid event batch acknowledgement')
                seen.add(result['event_id'])
                event_id = result['event_id']
                if result['status'] in ('persisted', 'rejected'):
                    self._death_spool.acknowledge(event_id)
                    self._death_retry_at.pop(event_id, None)
                    if result['status'] == 'rejected':
                        _log('activity event rejected by backend (' + str(result.get('reason', 'unknown'))[:80] + ')')
                else:
                    self._death_retry_at[event_id] = _monotonic() + 3.0
        elif message.get('type') == 'event.ack':
            if (message.get('protocol_version') != PROTOCOL_VERSION or
                    not _validate_agent_id(message.get('event_id')) or
                    message.get('status') not in ('persisted', 'rejected', 'retry')):
                raise WebSocketClosed('invalid event acknowledgement')
            if message.get('status') in ('persisted', 'rejected'):
                self._death_spool.acknowledge(message['event_id'])
                self._death_retry_at.pop(message['event_id'], None)
                if message.get('status') == 'rejected':
                    _log('death event rejected by backend (' + str(message.get('reason', 'unknown'))[:80] + ')')
            else:
                self._death_retry_at[message['event_id']] = _monotonic() + 3.0
        elif message.get('type') == 'mob.sample.ack':
            sample_id = message.get('sample_id')
            if (message.get('protocol_version') != PROTOCOL_VERSION or
                    not _validate_agent_id(sample_id) or
                    message.get('status') not in ('persisted', 'rejected', 'retry')):
                raise WebSocketClosed('invalid mob sample acknowledgement')
            if message['status'] in ('persisted', 'rejected'):
                self._mob_spool.acknowledge(sample_id)
                self._mob_retry_at.pop(sample_id, None)
                if message['status'] == 'rejected':
                    _log('mob observation rejected by backend (' + str(message.get('reason', 'unknown'))[:80] + ')')
            else:
                self._mob_retry_at[sample_id] = _monotonic() + 3.0
        elif message.get('type') == 'command.execute': self._accept_command(message)
        elif message.get('type') == 'command.revoke': self._revoke_session(message)
        elif message.get('type') == 'resource.ack':
            if (message.get('protocol_version') != PROTOCOL_VERSION or message.get('character_id') != self.character_id or
                    message.get('session_id') != self.session_id or not isinstance(message.get('revision'), int) or
                    message.get('revision') > self._resource_revision):
                raise WebSocketClosed('invalid resource acknowledgement')
        elif message.get('type') == 'resource.resync':
            if message.get('protocol_version') != PROTOCOL_VERSION or message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
                raise WebSocketClosed('invalid resource resynchronization request')
            if self._latest_resources is not None and self._latest_resources_identity == self._identity_key(self._current_identity):
                self._resource_baseline_required = True
                self.update_resources(self._current_identity, self._latest_resources)
        elif message.get('type') != 'character.registered': raise WebSocketClosed('unexpected server application message')

    def _capability_frame(self):
        mapping = {
            'bot.start': ('start_bot', 'unsupported_runtime_primitive'),
            'bot.stop': ('stop_bot', 'unsupported_runtime_primitive'),
            'trace.start': ('start_trace', 'unsupported_runtime_primitive'),
            'trace.stop': ('stop_trace', 'unsupported_runtime_primitive'),
            'training.area.set': (None, 'unsupported_runtime_primitive'),
            'training.radius.set': ('set_training_radius', 'unsupported_runtime_primitive'),
            'character.walk': ('move_to_region', 'unsupported_runtime_primitive'),
            'character.move_to': ('move_to', 'unsupported_runtime_primitive'),
            'character.navigate': ('generate_script', 'unsupported_runtime_primitive'),
            'character.navigate.stop': ('stop_script', 'unsupported_runtime_primitive'),
            'character.teleport': (None, 'unsupported_runtime_primitive'),
            'character.recall_point.designate': (None, 'recall_point_server_unverified'),
            'character.return': ('use_return_scroll', 'unsupported_runtime_primitive'),
            'character.reverse_return': ('reverse_return', 'unsupported_runtime_primitive'),
            'character.disconnect': ('disconnect', 'unsupported_runtime_primitive'),
            'client.clientless': (None, 'unsupported_runtime_primitive'),
        }
        commands = []
        for name, (function, reason) in mapping.items():
            supported = bool(function and self.api.has(function))
            extra = {}
            if name == 'training.area.set':
                modes=[]
                if self.api.has('set_training_position'):
                    modes.append('position')
                    if self.api.has('get_position'): modes.append('current_position')
                if self.api.has('set_training_area'): modes.append('named')
                supported=bool(modes); extra['modes']=modes
            if name == 'training.radius.set': supported = self.api.has('set_training_radius') and self.api.has('get_training_area')
            if name == 'character.walk':
                supported = all(self.api.has(symbol) for symbol in ('generate_path', 'move_to_region', 'get_position'))
            if name == 'character.navigate':
                supported = self.api.has('generate_script') and self.api.has('start_script')
            if name == 'character.navigate.stop':
                supported = self.api.has('stop_script') and self.api.has('start_script')
            if name == 'character.reverse_return':
                extra['modes'] = ['last_return', 'last_death'] if supported else []
                if supported and self.api.has('get_party'): extra['modes'].append('party_member')
                if supported: extra['modes'].append('named_location')
            if name == 'character.teleport':
                supported = all(self.api.has(symbol) for symbol in ('get_npcs', 'get_teleport_data', 'start_script'))
            if name == 'character.recall_point.designate':
                supported, reason = _recall_point_support(self.phbot_version, self._current_identity, self.api)
            commands.append({'name': name, 'supported': supported, 'reason': '' if supported else reason})
            if extra: commands[-1].update(extra)
        chat_modes = self.api.chat_modes()
        commands.append({'name': 'chat.send', 'supported': bool(chat_modes),
                         'reason': '' if chat_modes else 'unsupported_runtime_primitive',
                         'modes': chat_modes})
        return {'type': 'agent.capabilities', 'protocol_version': PROTOCOL_VERSION, 'schema_version': 1, 'commands': commands}

    def _accept_command(self, message):
        command_id = message.get('command_id')
        if (message.get('protocol_version') != PROTOCOL_VERSION or not isinstance(command_id, str) or
                len(command_id) != 40 or not command_id.startswith('cmd_') or
                not _validate_agent_id(message.get('character_id')) or not _validate_agent_id(message.get('session_id')) or
                not isinstance(message.get('name'), str) or not isinstance(message.get('args'), dict)):
            raise WebSocketClosed('invalid or stale command target')
        if message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
            self._queue_result(self._base_result(message,'failed','session_not_active','unverified'))
            return
        if command_id in self._dedup:
            return
        _log('command %s %s received' % (command_id, message['name']))
        if self._outgoing.qsize() > 29:
            self._queue_result(self._base_result(message, 'failed', 'result_queue_full', 'unverified'))
            return
        ttl = message.get('ttl_ms')
        if not isinstance(ttl, int) or isinstance(ttl, bool) or ttl <= 0 or ttl > 10000:
            self._queue_result(self._base_result(message, 'failed', 'command_expired', 'unverified'))
            return
        try:
            remaining = min(ttl / 1000.0, _utc_epoch(message.get('expires_at')) - (time.time() + self._server_clock_offset)) - 0.25
        except Exception:
            remaining = 0.0
        if remaining <= 0:
            self._queue_result(self._base_result(message, 'failed', 'command_expired', 'unverified'))
            return
        try:
            self._commands.put_nowait({'message': dict(message), 'deadline': _monotonic() + remaining, 'epoch': self._profile_epoch})
        except _queue.Full:
            self._queue_result(self._base_result(message, 'failed', 'command_queue_full', 'unverified'))
            return
        self._remember_command(command_id)
        _log('command %s queued (remaining=%.0f ms)' % (command_id, remaining * 1000.0))
        self._queue_result({'type':'command.ack','protocol_version':PROTOCOL_VERSION,'command_id':command_id,'character_id':self.character_id,'session_id':self.session_id})

    def _remember_command(self, command_id):
        self._dedup.add(command_id); self._dedup_order.append(command_id)
        while len(self._dedup_order) > 256:
            old = self._dedup_order.pop(0); self._dedup.discard(old)

    def _queue_result(self, frame):
        if frame.get('type') == 'command.result':
            reason = frame.get('reason') or 'none'
            # Native exception strings can contain arguments or local details.
            # Log only bounded reason codes, never arbitrary exception text.
            if not isinstance(reason, str) or not re.match(r'^[a-z][a-z0-9_]{0,63}$', reason):
                reason = 'api_error'
            _log('command %s result=%s reason=%s verification=%s' % (
                frame.get('command_id'), frame.get('status'), reason,
                frame.get('verification')))
        try: self._outgoing.put_nowait(frame)
        except _queue.Full: self.status = 'Command result queue is full; backend may show an unknown outcome.'

    def _flush_results(self, client):
        while True:
            try: frame = self._outgoing.get_nowait()
            except _queue.Empty: return
            client.send_json(frame)

    def _spool_queued_events(self):
        while True:
            try:
                event = self._event_samples.get_nowait()
            except _queue.Empty:
                break
            if not self._death_spool.add(event):
                self.status = 'Activity event spool is unavailable or full; an occurrence needs operator attention.'
                _log('activity event was not sent because durable local spooling failed')

    def _flush_events(self, client):
        self._spool_queued_events()
        now = _monotonic()
        batch = []
        batch_ids = []
        batch_size = 128
        for event in self._death_spool.pending():
            event_id = event.get('event_id')
            if self._death_retry_at.get(event_id, 0.0) > now:
                continue
            already_bound = (
                _validate_agent_id(event.get('character_id')) and
                _validate_agent_id(event.get('session_id'))
            )
            needs_binding = not already_bound and (
                event.get('deferred_session_binding') or
                (event.get('character_name') and not _validate_agent_id(event.get('session_id')))
            )
            if needs_binding:
                identity = {'server': event.get('server'), 'name': event.get('character_name')}
                if (self._current_identity is None or
                        self._identity_key(identity) != self._identity_key(self._current_identity) or
                        not _validate_agent_id(self.character_id) or not _validate_agent_id(self.session_id)):
                        continue
                character_id = self.character_id
                session_id = self.session_id
            elif already_bound:
                character_id = event['character_id']
                session_id = event['session_id']
            else:
                character_id = None
                session_id = None
            if character_id and session_id:
                sequence = event.get('sequence')
                if sequence is None:
                    sequence = self._next_event_sequence(session_id)
                if not self._death_spool.bind_session(event_id, character_id, session_id, sequence):
                    self.status = 'Activity event could not be bound to a registered character session.'
                    continue
                event['character_id'] = character_id
                event['session_id'] = session_id
                event['sequence'] = sequence
                event['deferred_session_binding'] = False
                payload = event.get('payload')
                if isinstance(payload, dict) and payload.get('session_binding') == 'deferred':
                    payload = dict(payload)
                    payload.pop('session_binding', None)
                    event['payload'] = payload
            wire = {
                key: event.get(key) for key in (
                    'event_id', 'schema_version', 'kind', 'category', 'character_id', 'session_id',
                    'occurred_at', 'sequence', 'source', 'source_ref', 'dedupe_key', 'region', 'zone',
                    'x', 'y', 'z', 'item_model', 'item_code', 'payload')
                if event.get(key) is not None
            }
            if event.get('server'):
                wire['server'] = event['server']
            if event.get('character_name'):
                wire['character'] = event['character_name']
            encoded_size = len(json.dumps(wire, separators=(',', ':'), sort_keys=True).encode('utf-8'))
            if batch and (len(batch) >= MAX_EVENT_BATCH_SIZE or batch_size + encoded_size > MAX_MESSAGE_BYTES - 1024):
                break
            if encoded_size + batch_size > MAX_MESSAGE_BYTES - 1024:
                self.status = 'Activity event exceeds the transport frame limit.'
                _log('activity event could not be sent because it exceeds the transport frame limit')
                self._death_retry_at[event_id] = now + 30.0
                continue
            batch.append(wire)
            batch_ids.append(event_id)
            batch_size += encoded_size
        if batch:
            client.send_json({
                'type': 'event.batch',
                'protocol_version': PROTOCOL_VERSION,
                'sent_at': _utc_now(),
                'events': batch,
            })
            for event_id in batch_ids:
                self._death_retry_at[event_id] = now + 3.0

    def _flush_death_events(self, client):
        # Compatibility name retained for callers during the event-pipeline rollout.
        return self._flush_events(client)

    def _base_result(self, message, status, code, verification):
        return {'type':'command.result','protocol_version':PROTOCOL_VERSION,'command_id':message.get('command_id'),
                'character_id':message.get('character_id'),'session_id':message.get('session_id'),
                'status':status,'reason':code,'verification':verification}

    def _revoke_session(self, message):
        if message.get('protocol_version') != PROTOCOL_VERSION or message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
            return
        self._discard_pending_commands('session_superseded')
        self._clear_navigation_route()

    def _clear_pending_commands(self):
        while True:
            try: self._commands.get_nowait()
            except _queue.Empty: break

    def _discard_pending_commands(self, reason):
        self._cancel_navigation_generation(reason)
        while True:
            try: item=self._commands.get_nowait()
            except _queue.Empty: break
            self._queue_result(self._base_result(item['message'],'failed',reason,'unverified'))

    def process_one_command(self, current_identity=None, current_region=None):
        """Invoke at most one validated command from phBot's event_loop callback."""
        if self._navigation_job is not None:
            return self._process_navigation_generation(current_identity)
        if self.stop_event.is_set():
            return False
        try: item = self._commands.get_nowait()
        except _queue.Empty: return False
        message = item['message']; name = message['name']; args = message['args']
        _log('command %s %s callback started' % (message['command_id'], name))
        if item['epoch'] != self._profile_epoch or message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
            self._queue_result(self._base_result(message, 'failed', 'stale_session', 'unverified')); return True
        if _monotonic() >= item['deadline']:
            self._queue_result(self._base_result(message, 'failed', 'command_expired', 'unverified')); return True
        expected_identity = self._current_identity
        if current_identity is not None and self._identity_key(current_identity) != self._identity_key(expected_identity):
            self._queue_result(self._base_result(message, 'failed', 'character_changed', 'unverified')); return True
        try:
            if name == 'character.navigate':
                self._start_navigation_generation(item, current_identity)
                return True
            if name == 'character.walk':
                self._start_walk(message, args, current_region)
                return True
            outcome, effective, observed, verification = self._invoke(name, args, current_region)
            status = 'failed' if outcome is False else 'completed'
            result = self._base_result(message, status, 'api_return_false' if outcome is False else '', verification)
            result['api_return'] = outcome
            result['effective_args'] = effective
            if observed is not None: result['observed_after'] = observed
            self._queue_result(result)
            if name != 'character.move_to':
                self._queue_control_state(message)
            if name == 'character.navigate' and outcome is not False:
                self._publish_navigation_route(message)
        except Exception as error:
            self._queue_result(self._base_result(message, 'failed', str(error)[:64] or 'api_error', 'unverified'))
        return True

    @staticmethod
    def _navigation_args(args):
        if set(args) != set(('region', 'x', 'y', 'z')):
            raise ValueError('invalid_arguments')
        region = args.get('region')
        if (not isinstance(region, int) or isinstance(region, bool) or region == 0 or
                region < -32768 or region > 65535 or
                not all(_number(args.get(axis)) and abs(args[axis]) <= 10000000 for axis in ('x', 'y', 'z'))):
            raise ValueError('invalid_arguments')
        return region

    def _start_navigation_generation(self, item, current_identity):
        args = dict(item['message']['args'])
        region = self._navigation_args(args)
        if not self.api.has('generate_script') or not self.api.has('start_script'):
            raise ValueError('unsupported_runtime_primitive')
        if not _NAVIGATION_GENERATION_SLOT.acquire(False):
            raise ValueError('navigation_generation_busy')
        job = {'item': item, 'identity': dict(current_identity or self._current_identity or {}),
               'done': threading.Event(), 'generated': None, 'error': None, 'cancel_reason': None}

        def generate():
            try:
                # This read-only native API is the sole off-callback phBot call.
                # It must never run on the transport thread or invoke a script.
                job['generated'] = _navigation_stage('generate_script', self.api.call, 'generate_script',
                                                    region, float(args['x']), float(args['y']), float(args['z']))
            except Exception:
                job['error'] = 'path_generation_failed'
            finally:
                _NAVIGATION_GENERATION_SLOT.release()
                job['done'].set()

        job['thread'] = threading.Thread(target=generate, name='PhMon-navigation-generation')
        job['thread'].daemon = True
        with self._navigation_job_lock:
            self._navigation_job = job
        try:
            job['thread'].start()
        except Exception:
            with self._navigation_job_lock:
                self._navigation_job = None
            _NAVIGATION_GENERATION_SLOT.release()
            raise ValueError('path_generation_failed')

    def _cancel_navigation_generation(self, reason):
        with self._navigation_job_lock:
            job = self._navigation_job
            if job is None or job['cancel_reason'] is not None:
                return
            job['cancel_reason'] = reason
        self._queue_result(self._base_result(job['item']['message'], 'failed', reason, 'unverified'))
        # Native generation cannot be interrupted safely. Retain its occupied
        # slot until it returns; never join, replay, or start an extra generator.

    def _process_navigation_generation(self, current_identity):
        with self._navigation_job_lock:
            job = self._navigation_job
        if job is None:
            return False
        item = job['item']
        message = item['message']
        expected = job['identity']
        identity = current_identity if current_identity is not None else self._current_identity
        reason = None
        if self.stop_event.is_set():
            reason = 'plugin_stopped'
        elif (item['epoch'] != self._profile_epoch or message['character_id'] != self.character_id or
              message['session_id'] != self.session_id):
            reason = 'stale_session'
        elif (self._identity_key(identity) != self._identity_key(expected) or
              (identity or {}).get('profile_key') != expected.get('profile_key')):
            reason = 'character_changed'
        elif _monotonic() >= item['deadline']:
            reason = 'command_expired'
        if reason:
            self._cancel_navigation_generation(reason)
        if not job['done'].is_set():
            return False
        # Serialize cancellation against the final fence and script invocation.
        # The native generator never needs this lock and callback never joins it.
        with self._navigation_job_lock:
            try:
                if job['cancel_reason'] is not None:
                    return True
                if (item['epoch'] != self._profile_epoch or message['character_id'] != self.character_id or
                        message['session_id'] != self.session_id or self.stop_event.is_set()):
                    self._queue_result(self._base_result(message, 'failed', 'stale_session', 'unverified'))
                    return True
                if job['error']:
                    raise ValueError(job['error'])
                outcome, effective, observed, verification = self._finish_navigation(message['args'], job['generated'], message['command_id'])
                result = self._base_result(message, 'failed' if outcome is False else 'completed',
                                           'api_return_false' if outcome is False else '', verification)
                result['api_return'] = outcome
                result['effective_args'] = effective
                self._queue_result(result)
                self._queue_control_state(message)
                if outcome is not False:
                    self._publish_navigation_route(message)
            except Exception as error:
                self._queue_result(self._base_result(message, 'failed', str(error)[:64] or 'api_error', 'unverified'))
            finally:
                self._navigation_job = None
        return True

    def _navigation_debug_status(self):
        # Optional installed-runtime observation. None/unknown text has no
        # inferred meaning and must not become a script admission condition.
        if not self.api.has('get_status'):
            return 'unavailable'
        try:
            status = self.api.call('get_status')
        except Exception:
            return 'read_failed'
        if status is None:
            return 'none'
        if isinstance(status, str):
            label = status.strip().lower()
            if label in ('botting', 'training', 'tracing', 'stopped', 'idle', 'walking', 'script', 'running'):
                return label
            return 'other_text'
        return 'other_type'

    def _finish_navigation(self, args, generated, command_id='local'):
        self._last_navigation_evidence = None
        region = self._navigation_args(args)
        if generated is False: raise ValueError('path_rate_limited_or_not_in_game')
        if generated is None: raise ValueError('path_not_found')
        script, instructions = _navigation_stage('validate_script', _parse_generated_navigation_script, generated)
        source = None
        source_time = _utc_now()
        try:
            position = _navigation_stage('source_position', self.api.position)
            if (isinstance(position, dict) and _valid_position_region(position.get('region')) and
                    all(_number(position.get(axis)) and abs(position[axis]) <= 10000000 for axis in ('x', 'y', 'z'))):
                source = {'region': int(position['region']), 'x': float(position['x']),
                          'y': float(position['y']), 'z': float(position['z']), 'observed_at': source_time}
        except Exception:
            source = None
        counts = collections.Counter(instruction['kind'] for instruction in instructions)
        encoded = script.encode('utf-8')
        status_before = _navigation_stage('debug_status_before', self._navigation_debug_status)
        _log('navigation %s prepared steps=%d walk=%d wait=%d teleport=%d bytes=%d sha256=%s '
             'trailing_lf=%s source_region=%s source_z=%s target_region=%s target_z=%s status=%s' % (
                 command_id, len(instructions), counts['walk'], counts['wait'], counts['teleport'],
                 len(encoded), hashlib.sha256(encoded).hexdigest()[:16], script.endswith('\n'),
                 source.get('region') if source else 'unknown', source.get('z') if source else 'unknown',
                 region, float(args['z']), status_before))
        invoked_at = _utc_now()
        try:
            result=_navigation_stage('start_script', self.api.call, 'start_script', script)
        except Exception:
            _log('navigation %s start_script raised; status_after=%s' % (
                command_id, _navigation_stage('debug_status_after', self._navigation_debug_status)))
            raise ValueError('script_start_failed') from None
        returned = 'false' if result is False else 'true' if result is True else 'none' if result is None else 'other'
        return_type = ('NoneType' if result is None else 'bool' if isinstance(result, bool)
                       else 'int' if isinstance(result, int) else 'float' if isinstance(result, float)
                       else 'str' if isinstance(result, str) else 'other')
        _log('navigation %s start_script result=%s result_type=%s status_after=%s' % (
            command_id, returned, return_type, _navigation_stage('debug_status_after', self._navigation_debug_status)))
        if result is not False:
            self._last_navigation_evidence = {'instructions': instructions, 'source': source,
                                              'invoked_at': invoked_at}
        return result,{'region':region,'x':float(args['x']),'y':float(args['y']),'z':float(args['z']),'route_steps':len(instructions)},None,'api_confirmed' if isinstance(result,bool) else 'unverified'

    def _start_walk(self, message, args, current_region):
        if set(args) != set(('region', 'x', 'y', 'z')):
            raise ValueError('invalid_arguments')
        if self._active_walk is not None:
            raise ValueError('walk_already_active')
        if not all(self.api.has(symbol) for symbol in ('generate_path', 'move_to_region', 'get_position')):
            raise ValueError('unsupported_runtime_primitive')
        region = args.get('region')
        if (not isinstance(region, int) or isinstance(region, bool) or region <= 0 or region != current_region or
                not all(_number(args.get(axis)) and abs(args[axis]) <= 10000000 for axis in ('x', 'y', 'z'))):
            raise ValueError('invalid_arguments')
        generated = self.api.call('generate_path', float(args['x']), float(args['y']))
        if generated is False:
            raise ValueError('path_rate_limited_or_not_in_game')
        if generated is None:
            raise ValueError('path_not_found')
        if not isinstance(generated, (list, tuple)) or not generated or len(generated) > MAX_WALK_WAYPOINTS:
            raise ValueError('invalid_path')
        points = []
        for point in generated:
            if not isinstance(point, (list, tuple)):
                raise ValueError('invalid_path')
            if len(point) == 2:
                point_region, x, y = region, point[0], point[1]
            elif len(point) == 3:
                point_region, x, y = point[0], point[1], point[2]
            else:
                raise ValueError('invalid_path')
            if (not isinstance(point_region, int) or isinstance(point_region, bool) or point_region != region or
                    not _number(x) or not _number(y) or abs(x) > 10000000 or abs(y) > 10000000):
                raise ValueError('invalid_path')
            points.append({'region': region, 'x': float(x), 'y': float(y), 'z': float(args['z'])})

        route = {
            'message': message,
            'identity': self._identity_key(self._current_identity),
            'epoch': self._profile_epoch,
            'session_id': self.session_id,
            'region': region,
            'points': points,
            'index': 0,
            'destination': {'region': region, 'x': float(args['x']), 'y': float(args['y']), 'z': float(args['z'])},
            'deadline': _monotonic() + WALK_TIMEOUT_SECONDS,
        }
        self._active_walk = route
        route['effective_args'] = dict(route['destination'])
        try:
            self.api.call('move_to_region', region, points[0]['x'], points[0]['y'], points[0]['z'])
        except Exception:
            self._finish_walk('unknown', 'walk_waypoint_dispatch_failed', 'unverified')

    def process_walk_step(self, current_identity=None, current_region=None):
        """Advance one path waypoint from phBot's callback; never called by the network worker."""
        route = self._active_walk
        if route is None:
            return False
        message = route['message']
        if (route['epoch'] != self._profile_epoch or route['session_id'] != self.session_id or
                self._identity_key(current_identity) != route['identity']):
            self._finish_walk('unknown', 'walk_target_changed', 'unverified')
            return True
        if current_region != route['region']:
            self._finish_walk('unknown', 'walk_region_changed', 'unverified')
            return True
        if _monotonic() >= route['deadline']:
            self._finish_walk('unknown', 'walk_timeout', 'unverified')
            return True
        position = self.api.position()
        if (not isinstance(position, dict) or position.get('region') != route['region'] or
                not _number(position.get('x')) or not _number(position.get('y'))):
            self._finish_walk('unknown', 'walk_position_unavailable', 'unverified')
            return True
        point = route['points'][route['index']]
        if math.hypot(float(position['x']) - point['x'], float(position['y']) - point['y']) > WALK_ARRIVAL_TOLERANCE:
            return False
        route['index'] += 1
        if route['index'] >= len(route['points']):
            destination = route['destination']
            reached = math.hypot(float(position['x']) - destination['x'], float(position['y']) - destination['y']) <= WALK_ARRIVAL_TOLERANCE
            self._finish_walk('completed' if reached else 'unknown', '' if reached else 'destination_not_observed',
                              'observed' if reached else 'unverified', position)
            return True
        next_point = route['points'][route['index']]
        try:
            self.api.call('move_to_region', next_point['region'], next_point['x'], next_point['y'], next_point['z'])
        except Exception:
            self._finish_walk('unknown', 'walk_waypoint_dispatch_failed', 'unverified')
        return True

    def _finish_walk(self, status, reason, verification, observed=None):
        route = self._active_walk
        if route is None:
            return
        self._active_walk = None
        message = route['message']
        result = self._base_result(message, status, reason, verification)
        result['api_return'] = None
        result['effective_args'] = route['effective_args']
        result['route_waypoints'] = len(route['points'])
        if observed is not None:
            result['observed_after'] = {'x': float(observed['x']), 'y': float(observed['y']), 'region': int(observed['region'])}
        self._queue_result(result)
        self._queue_control_state(message)

    def _invoke(self, name, args, current_region):
        def exact(allowed):
            if set(args) - set(allowed): raise ValueError('invalid_arguments')
        if name == 'character.move_to':
            if set(args) != set(('x', 'y', 'z')) or not all(
                    _number(args.get(axis)) and abs(args[axis]) <= 10000000 for axis in ('x', 'y', 'z')):
                raise ValueError('invalid_arguments')
            if not self.api.has('move_to'):
                raise ValueError('unsupported_runtime_primitive')
            # Explicit operator test: no path generation, position readback,
            # bot/script mutation or arrival wait. None is the documented return.
            result = self.api.call('move_to', float(args['x']), float(args['y']), float(args['z']))
            return result, dict(args), None, 'unverified'
        empty = ('bot.start','bot.stop','trace.stop','character.return','character.disconnect')
        if name in empty:
            exact(())
            function = {'bot.start':'start_bot','bot.stop':'stop_bot','trace.stop':'stop_trace','character.return':'use_return_scroll','character.disconnect':'disconnect'}[name]
            result = self.api.call(function)
            if name == 'trace.stop' and result is not False:
                self._trace_requested_name = None
            return result, {}, None, 'api_confirmed' if isinstance(result,bool) else 'unverified'
        if name == 'character.reverse_return':
            exact(('type', 'name'))
            kind = args.get('type')
            value = args.get('name', '')
            if (type(kind) is not int or kind < 0 or kind > 3 or not isinstance(value, str) or
                    any(ord(c) < 32 or 127 <= ord(c) <= 159 for c in value) or (kind < 2 and value)):
                raise ValueError('invalid_arguments')
            value = value.strip()
            if len(value.encode('utf-8')) > 100 or (kind < 2 and value) or (kind >= 2 and not value):
                raise ValueError('invalid_arguments')
            if not self.api.has('reverse_return'): raise ValueError('unsupported_runtime_primitive')
            if kind == 2:
                if not self.api.has('get_party'): raise ValueError('party_unavailable')
                identity = self._current_identity or {}
                if value.casefold() == str(identity.get('name', '')).casefold(): raise ValueError('party_self_target')
                party = self.api.call('get_party')
                if not isinstance(party, dict): raise ValueError('party_unavailable')
                matches = [row.get('name', '').strip() for row in list(party.values())[:32]
                           if isinstance(row, dict) and isinstance(row.get('name'), str) and row['name'].strip().casefold() == value.casefold()]
                if not matches: raise ValueError('party_member_not_found')
                value = matches[0]
                if len(value.encode('utf-8')) > 100 or any(ord(c) < 32 or 127 <= ord(c) <= 159 for c in value):
                    raise ValueError('party_member_not_found')
            outcome = self.api.call('reverse_return', kind, value)
            if not isinstance(outcome, bool): raise ValueError('invalid_api_result')
            return outcome, {'type': kind, 'name': value}, None, 'api_confirmed'
        if name == 'trace.start':
            exact(('name',)); value=args.get('name')
            if not isinstance(value,str) or not value.strip() or len(value.strip().encode('utf-8'))>64: raise ValueError('invalid_arguments')
            leader=value.strip()
            result=self.api.call('start_trace',leader)
            if result is not False:
                self._trace_requested_name = leader
            return result,{'name':leader},None,'api_confirmed' if isinstance(result,bool) else 'unverified'
        if name == 'character.navigate.stop':
            exact(('command_id', 'route_sequence'))
            command_id = args.get('command_id')
            sequence = args.get('route_sequence')
            if (not isinstance(command_id, str) or not command_id.startswith('cmd_') or len(command_id) != 40 or
                    not isinstance(sequence, int) or isinstance(sequence, bool) or sequence <= 0):
                raise ValueError('invalid_arguments')
            token = self._active_navigation
            if (not isinstance(token, dict) or token.get('command_id') != command_id or
                    token.get('route_sequence') != sequence or token.get('epoch') != self._profile_epoch):
                raise ValueError('route_token_mismatch')
            if not self.api.has('stop_script'):
                raise ValueError('unsupported_runtime_primitive')
            result = self.api.call('stop_script')
            self._active_navigation = None
            status = 'failed' if result is False else 'completed'
            return result, {'command_id': command_id, 'route_sequence': sequence}, None, 'api_confirmed' if isinstance(result, bool) else 'unverified'
        if name == 'chat.send':
            exact(('channel','text','recipient'))
            channel=args.get('channel'); value=args.get('text'); recipient=args.get('recipient')
            if not isinstance(channel,str) or channel not in _CHAT_METHODS or not isinstance(value,str):
                raise ValueError('invalid_arguments')
            if recipient is not None and not isinstance(recipient,str):
                raise ValueError('invalid_arguments')
            recipient=(recipient or '').strip()
            try:
                encoded=value.encode('utf-8')
                recipient_bytes=recipient.encode('utf-8')
            except Exception:
                raise ValueError('invalid_arguments')
            if (not value.strip() or len(encoded)>2048 or b'\x00' in encoded or
                    len(recipient_bytes)>64 or b'\x00' in recipient_bytes or
                    (channel == 'private') != bool(recipient)):
                raise ValueError('invalid_arguments')
            if channel not in self.api.chat_modes():
                raise ValueError('unsupported_channel')
            result=self.api.send_chat(channel,value,recipient or None)
            effective={'channel':channel,'text':value}
            if recipient: effective['recipient']=recipient
            return result,effective,None,'api_confirmed' if isinstance(result,bool) else 'unverified'
        if name == 'training.area.set':
            exact(('mode','name','region','x','y','z')); mode=args.get('mode')
            if mode == 'current_position':
                position=self.api.position()
                if not isinstance(position,dict) or not all(_number(position.get(k)) and abs(position.get(k))<=10000000 for k in ('x','y','z')) or not isinstance(position.get('region'),int) or isinstance(position.get('region'),bool) or position.get('region') < -32768 or position.get('region') > 65535 or position.get('region')==0: raise ValueError('position_unavailable')
                region=int(position['region']); coords=[float(position[k]) for k in ('x','y','z')]
            elif mode == 'position':
                if not isinstance(args.get('region'),int) or isinstance(args.get('region'),bool) or args.get('region') < -32768 or args.get('region') > 65535 or args.get('region')==0 or not all(_number(args.get(k)) and abs(args[k])<=10000000 for k in ('x','y','z')): raise ValueError('invalid_arguments')
                region=args['region']; coords=[float(args[k]) for k in ('x','y','z')]
            elif mode == 'named':
                value=args.get('name')
                if not isinstance(value,str) or not value.strip() or len(value.strip().encode('utf-8'))>100: raise ValueError('invalid_arguments')
                result=self.api.call('set_training_area',value.strip()); area=self.api.call('get_training_area') if self.api.has('get_training_area') else None; return result,{'mode':'named','name':value.strip()},self._safe_area(area),'api_confirmed' if isinstance(result,bool) else 'unverified'
            else: raise ValueError('invalid_arguments')
            result=self.api.call('set_training_position',region,*coords)
            area=self.api.call('get_training_area') if self.api.has('get_training_area') else None
            return result,{'mode':mode,'region':region,'x':coords[0],'y':coords[1],'z':coords[2]},self._safe_area(area),'api_confirmed' if isinstance(result,bool) else 'unverified'
        if name == 'training.radius.set':
            exact(('radius',)); radius=args.get('radius')
            if not _number(radius) or radius<1 or radius>10000: raise ValueError('invalid_arguments')
            area=self.api.call('get_training_area') if self.api.has('get_training_area') else None
            if not isinstance(area,dict): raise ValueError('training_area_unavailable')
            result=self.api.call('set_training_radius',float(radius)); observed=self.api.call('get_training_area') if self.api.has('get_training_area') else None
            confirmed=isinstance(observed,dict) and observed.get('radius')==float(radius)
            return result,{'radius':float(radius)},self._safe_area(observed),'observed' if confirmed else ('api_confirmed' if isinstance(result,bool) else 'unverified')
        if name == 'character.teleport':
            exact(('source', 'destination', 'gate_servername'))
            source = _teleport_probe_label(args.get('source'))
            destination = _teleport_probe_label(args.get('destination'))
            gate_servername = args.get('gate_servername')
            if (not source or not destination or not isinstance(gate_servername, str) or
                    not _NPC_GATE_ROLE.fullmatch(gate_servername.strip())):
                raise ValueError('invalid_arguments')
            gate_servername = gate_servername.strip()
            npc_status, npcs, _ = collect_npc_observation()
            if npc_status != 'observed':
                raise ValueError('teleporter_gate_not_observed')
            gate = _session_teleporter_gate(npcs, gate_servername, source)
            if gate is None:
                raise ValueError('teleporter_gate_not_observed')
            observed = self.api.call('get_teleport_data', source, destination)
            if _classify_teleport_data_result(observed).get('result') != 'tuple':
                raise ValueError('teleport_route_unavailable')
            line = 'teleport,{0},{1}'.format(source, destination)
            result = self.api.call('start_script', line)
            code = observed[1] if isinstance(observed, tuple) and len(observed) > 1 else None
            effective = {
                'source': source,
                'destination': destination,
                'gate_servername': gate_servername,
                'gate_npc_id': gate.get('id'),
                'script_line': line,
                'teleport_code': code,
            }
            return result, effective, None, 'api_confirmed' if isinstance(result, bool) else 'unverified'
        if name == 'character.recall_point.designate':
            supported, reason = _recall_point_support(self.phbot_version, self._current_identity, self.api)
            if not supported:
                raise ValueError(reason)
            npc_status, npcs, _ = collect_npc_observation(
                {'get_npcs': self.api.functions.get('get_npcs')})
            if npc_status != 'observed':
                raise ValueError('recall_point_gate_not_observed')
            resolved = _session_recall_point_gate(npcs, args)
            if resolved is None:
                raise ValueError('recall_point_gate_not_observed')
            gate, npc_id = resolved
            # phBot 20.1.3 on Greatest sent exactly 0x7059 + this live gate ID
            # for two successful manual Hotan designations. No select packet.
            result = self.api.call('inject_joymax', 0x7059, struct.pack('<I', npc_id), False)
            effective = {
                'gate_servername': gate['servername'], 'region': gate['region'],
                'x': gate['x'], 'y': gate['y'], 'gate_npc_id': gate['id'],
            }
            if 'model_id' in gate:
                effective['model_id'] = gate['model_id']
            # The B059 response byte has not been captured/interpreted for a
            # PhMon-issued request, so packet submission is not save proof.
            return result, effective, None, 'unverified'
        if name == 'character.navigate':
            raise ValueError('navigation_requires_callback_path')
        if name == 'character.walk':
            raise ValueError('walk_requires_callback_path')
        raise ValueError('unsupported_command')

    @staticmethod
    def _safe_area(area):
        if not isinstance(area,dict): return None
        safe={}
        for key in ('region','x','y','z','radius'):
            value=area.get(key)
            if key=='region':
                if _valid_position_region(value):
                    safe['training_region']=value
                    zone = _zone_name_for_region(value, 100)
                    if zone:
                        safe['training_zone']=zone
            elif _number(value): safe['training_'+key]=float(value)
        return safe

    def _queue_control_state(self, message):
        area=None
        try:
            if self.api.has('get_training_area'): area=self.api.call('get_training_area')
        except Exception: area=None
        safe=self._safe_area(area) or {}
        activity_state, activity_source = _read_trace_activity()
        state={
            'training_available':bool(isinstance(area,dict)),
            'observed_at':_utc_now(),
            'activity_state': activity_state,
            'activity_source': activity_source,
        }
        if self._trace_requested_name:
            state['trace_requested_name'] = self._trace_requested_name
        state.update(safe)
        self._queue_result({'type':'character.control_state','protocol_version':PROTOCOL_VERSION,'character_id':message.get('character_id'),
                            'session_id':message.get('session_id'),'control_state':state})

    def report_control_state(self, force=False):
        """Queue current training readback from the phBot callback, never network I/O."""
        if self.character_id is None or self.session_id is None:
            return False
        now = _monotonic()
        if (not force and self._last_control_sample_session == self.session_id
                and now - self._last_control_sample_at < 30.0):
            return False
        self._queue_control_state({
            'character_id': self.character_id,
            'session_id': self.session_id,
        })
        self._last_control_sample_session = self.session_id
        self._last_control_sample_at = now
        return True


_worker = None
_active_settings_path = None
_profile_config = None
_gui = None
_gui_backend_url = None
_gui_agent_id = None
_gui_agent_token = None
_gui_ignore_potions = None
_gui_ignore_pills = None
_gui_status = None
_last_character_signature = None
_last_character_sample_at = 0.0
_last_resources_sample_at = 0.0
_last_position_sample_at = 0.0
_last_position_publish_at = 0.0
_last_position_observed = None
_last_position_session_id = None
_position_sample_lock = threading.Lock()
_last_monster_poll_at = 0.0
_last_mob_cell_samples = {}
_last_npc_poll_at = 0.0
_last_npc_region = None
_last_npc_signature = None
_last_npc_publish_at = 0.0
_npc_sample_forced = False
_last_player_poll_at = 0.0
_last_player_region = None
_last_player_observer_z = None
_last_player_signature = None
_last_player_publish_at = 0.0
_player_sample_forced = False


def _reset_npc_sample_state():
    global _last_npc_poll_at, _last_npc_region, _last_npc_signature, _last_npc_publish_at, _npc_sample_forced
    _last_npc_poll_at = 0.0
    _last_npc_region = None
    _last_npc_signature = None
    _last_npc_publish_at = 0.0
    _npc_sample_forced = False


def _reset_player_sample_state():
    global _last_player_poll_at, _last_player_region, _last_player_observer_z
    global _last_player_signature, _last_player_publish_at, _player_sample_forced
    _last_player_poll_at = 0.0
    _last_player_region = None
    _last_player_observer_z = None
    _last_player_signature = None
    _last_player_publish_at = 0.0
    _player_sample_forced = False
_death_callback_active = False
_recent_player_attack = None
_phbot_connected_state = None
_pending_callback_events = _queue.Queue(maxsize=64)
_pending_callback_overflow = False
# None means the plugin loaded after phBot may already have joined. In that case
# event_loop can establish presence once get_character_data() returns a character.
_character_joined = None


def _set_gui_status(message):
    if _QtBind is not None and _gui is not None and _gui_status is not None:
        _QtBind.setText(_gui, _gui_status, str(message))


def _set_gui_config(config=None):
    if _QtBind is None or _gui is None:
        return
    config = config or {}
    _QtBind.setText(_gui, _gui_backend_url, config.get('backend_url', ''))
    _QtBind.setText(_gui, _gui_agent_id, config.get('agent_id', ''))
    # QtBind has no documented password field. Never keep the persisted token visible.
    _QtBind.setText(_gui, _gui_agent_token, '')
    if _gui_ignore_potions is not None:
        _QtBind.setChecked(_gui, _gui_ignore_potions, config.get('ignore_potion_events', True))
    if _gui_ignore_pills is not None:
        _QtBind.setChecked(_gui, _gui_ignore_pills, config.get('ignore_pill_events', True))


def item_filter_changed(state):
    _set_gui_status('Click Save & Connect to apply item filters.')


def _stop_worker():
    global _worker
    if _worker is not None:
        _worker.stop()
        _worker = None


def _start_worker(config):
    global _worker
    _stop_worker()
    config = dict(config)
    try:
        config_dir = _get_config_dir() if callable(_get_config_dir) else None
        if isinstance(config_dir, str) and config_dir.strip():
            settings_path = _active_settings_path
            if not settings_path:
                settings_path = _current_settings_path()
            config['death_spool_path'] = _death_spool_path(
                config_dir, config['agent_id'], settings_path)
            config['mob_spool_path'] = _mob_spool_path(
                config_dir, config['agent_id'], settings_path)
        else:
            config['death_spool_path'] = None
            config['mob_spool_path'] = None
    except Exception:
        config['death_spool_path'] = None
        config['mob_spool_path'] = None
    try:
        version = _get_phbot_version()
    except Exception:
        version = 'unknown'
    _worker = AgentWorker(config, version)
    _worker.start()


def _current_settings_path():
    bot_profile = _get_profile()
    if bot_profile is None:
        return None
    bot_config_path = _get_config_path()
    if not bot_config_path:
        return None
    return _profile_settings_path(
        _get_config_dir(),
        bot_config_path,
        bot_profile,
    )


def _load_active_profile(force=False):
    global _active_settings_path
    global _profile_config

    path = _current_settings_path()
    if path is None:
        _set_gui_status('Join the game to select a bot profile.')
        return
    if not force and path == _active_settings_path:
        return

    _active_settings_path = path
    _profile_config = None
    _stop_worker()
    _reset_item_type_cache()

    try:
        config = load_saved_config(path)
    except IOError:
        _set_gui_config()
        _set_gui_status('Configure this bot profile, then click Save & Connect.')
        return
    except Exception as error:
        _set_gui_config()
        _set_gui_status('Saved profile is invalid; enter the settings again.')
        _log('profile configuration invalid (' + error.__class__.__name__ + ')')
        return

    _profile_config = config
    _set_gui_config(config)
    _set_gui_status('Profile loaded. Connecting to PhMon backend...')
    _start_worker(config)


def save_config():
    global _active_settings_path
    global _profile_config

    path = _current_settings_path()
    if path is None:
        _set_gui_status('Join the game before saving PhMon settings.')
        return

    saved = _profile_config if path == _active_settings_path else None
    try:
        config = config_from_gui_values(
            _QtBind.text(_gui, _gui_backend_url),
            _QtBind.text(_gui, _gui_agent_id),
            _QtBind.text(_gui, _gui_agent_token),
            saved,
            _QtBind.isChecked(_gui, _gui_ignore_potions),
            _QtBind.isChecked(_gui, _gui_ignore_pills),
        )
        save_saved_config(path, config)
    except Exception as error:
        _set_gui_status('Invalid settings. Check URL, agent ID and token.')
        _log('configuration not saved (' + error.__class__.__name__ + ')')
        return

    _active_settings_path = path
    _profile_config = config
    _set_gui_config(config)
    _set_gui_status('Settings saved for this bot profile. Connecting to PhMon backend...')
    _start_worker(config)


def connected():
    # phBot calls this when its client connects to the game server.
    global _character_joined, _phbot_connected_state
    already_connected = _phbot_connected_state is True
    _phbot_connected_state = True
    if not already_connected:
        _character_joined = False
        if _worker is not None and hasattr(_worker, 'invalidate_position_authority'):
            _worker.invalidate_position_authority()
        _lifecycle_event('session.connected', 'connected')


def disconnected():
    global _last_character_signature, _last_position_sample_at, _last_position_publish_at, _last_position_observed, _last_position_session_id, _character_joined, _death_callback_active, _recent_player_attack, _phbot_connected_state
    already_disconnected = _phbot_connected_state is False
    _phbot_connected_state = False
    _last_character_signature = None
    _last_position_sample_at = 0.0
    _last_position_publish_at = 0.0
    _last_position_observed = None
    _last_position_session_id = None
    _character_joined = False
    _death_callback_active = False
    _recent_player_attack = None
    if not already_disconnected:
        _lifecycle_event('session.disconnected', 'disconnected', include_identity=_worker is not None)
    if _worker is not None:
        _worker.leave_character()
    _reset_npc_sample_state()
    _reset_player_sample_state()
    _set_gui_status('SRO client disconnected. Waiting for login...')


def joined_game():
    # This callback runs after the player selects a character.
    global _last_character_signature, _last_position_sample_at, _last_position_publish_at, _last_position_observed, _last_position_session_id, _character_joined, _death_callback_active, _recent_player_attack
    already_joined = _character_joined is True
    _last_character_signature = None
    _last_position_sample_at = 0.0
    _last_position_publish_at = 0.0
    _last_position_observed = None
    _last_position_session_id = None
    if _worker is not None and hasattr(_worker, 'invalidate_position_authority'):
        _worker.invalidate_position_authority()
    _character_joined = True
    _death_callback_active = False
    _recent_player_attack = None
    # joined_game runs before phBot has loaded character data; keep this agent-scoped.
    _reset_npc_sample_state()
    _reset_player_sample_state()
    if not already_joined:
        _lifecycle_event('session.joined_game', 'joined_game')


def teleported():
    global _npc_sample_forced, _player_sample_forced, _last_position_sample_at, _last_position_publish_at, _last_position_observed
    _npc_sample_forced = True
    _last_position_sample_at = 0.0
    _last_position_publish_at = 0.0
    _last_position_observed = None
    _player_sample_forced = True
    if _worker is not None and hasattr(_worker, 'clear_position'):
        _worker.clear_position()
    if _worker is not None and hasattr(_worker, '_cancel_navigation_generation'):
        _worker._cancel_navigation_generation('character_teleported')
    _lifecycle_event('session.teleported', 'teleported', include_identity=True)


def _normalized_realtime_position(position):
    if not isinstance(position, dict):
        return None
    region = position.get('region')
    if not _valid_position_region(region):
        return None
    normalized = {'region': int(region)}
    for axis in ('x', 'y', 'z'):
        value = position.get(axis)
        if (not isinstance(value, (int, float)) or isinstance(value, bool) or
                not math.isfinite(float(value)) or abs(float(value)) > POSITION_COORDINATE_LIMIT):
            return None
        normalized[axis] = float(value)
    return normalized


def _position_meaningfully_changed(previous, current):
    if previous is None or previous.get('region') != current.get('region'):
        return True
    return any(
        abs(float(previous[axis]) - float(current[axis])) > POSITION_JITTER_TOLERANCE
        for axis in ('x', 'y', 'z')
    )


def _publish_realtime_position(position, now=None, identity=None):
    global _last_position_publish_at, _last_position_observed, _last_position_session_id
    if _worker is None:
        return False
    normalized = _normalized_realtime_position(position)
    if normalized is None:
        return False
    now = _monotonic() if now is None else now
    with _position_sample_lock:
        session_id = getattr(_worker, 'session_id', None)
        session_changed = bool(session_id) and session_id != _last_position_session_id
        region_changed = (
            _last_position_observed is not None and
            _last_position_observed.get('region') != normalized.get('region')
        )
        if not session_changed and not _position_meaningfully_changed(_last_position_observed, normalized):
            return False
        if (not session_changed and not region_changed and _last_position_publish_at > 0.0 and
                now - _last_position_publish_at < POSITION_PUBLISH_INTERVAL_SECONDS):
            return False
        observed_at = _worker_utc_now(_worker)
        accepted = (
            _worker.update_position(normalized, observed_at, identity)
            if identity is not None
            else _worker.update_position(normalized, observed_at)
        )
        if not accepted:
            return False
        _last_position_publish_at = now
        _last_position_observed = dict(normalized)
        if session_id:
            _last_position_session_id = session_id
        return True


def _sample_realtime_position(now=None):
    """Fast packet-triggered sampler; throttle before calling get_position()."""
    global _last_position_sample_at
    if not _PHBOT_AVAILABLE or _worker is None or _character_joined is False or not callable(_get_position):
        return False
    now = _monotonic() if now is None else now
    with _position_sample_lock:
        if now - _last_position_sample_at < POSITION_PUBLISH_INTERVAL_SECONDS:
            return False
        _last_position_sample_at = now
    try:
        position = _get_position()
    except Exception:
        return False
    return _publish_realtime_position(position, now)


def handle_joymax(opcode, data):
    """Observe server packets passively and always forward them to phBot."""
    if opcode == POSITION_MOVEMENT_OPCODE:
        try:
            _sample_realtime_position()
        except Exception:
            # Position monitoring must never interfere with packet forwarding.
            pass
    if _worker is not None:
        try:
            _worker.capture_joymax_packet(opcode, data)
        except Exception:
            # A monitoring failure must never interfere with the game packet.
            pass
    try:
        _observe_unique_notice(opcode, data)
    except Exception:
        # A monitoring failure must never interfere with the game packet.
        pass
    return True


_CRITICAL_EVENT_KINDS = (
    'character.died', 'drop.rare', 'alchemy.attempt', 'alchemy.finished', 'world.unique_spawned',
)


def _event_is_critical(item):
    return isinstance(item, dict) and (
        item.get('kind') in _CRITICAL_EVENT_KINDS or
        (item.get('kind') == 'item.acquired' and item.get('source') == 'joymax.pet_inventory')
    )


def _unique_notice(opcode, data):
    """Read a unique spawn or kill notice. Other packets stay untouched."""
    try:
        opcode = int(opcode)
    except Exception:
        return None
    if opcode != 0x300C:
        return None
    try:
        raw = bytes(data or b'')
    except Exception:
        return None
    if len(raw) < 6 or len(raw) > 256 or raw[0] not in (5, 6):
        return None
    model = struct.unpack_from('<I', raw, 2)[0]
    if model < 1 or model > 4294967295:
        return None
    killer = ''
    if raw[0] == 6 and len(raw) >= 8:
        killer_len = struct.unpack_from('<H', raw, 6)[0]
        end = 8 + killer_len
        if 0 < killer_len <= 64 and end <= len(raw):
            killer = raw[8:end].decode('utf-8', 'ignore')
            killer = ''.join(ch for ch in killer if ch.isprintable()).strip()[:64]
    return {'notice': 'spawn' if raw[0] == 5 else 'kill', 'model': int(model), 'killer': killer}


def _observe_unique_notice(opcode, data):
    parsed = _unique_notice(opcode, data)
    if parsed is None:
        return False
    try:
        character = _get_character_data()
    except Exception:
        character = None
    identity = None
    if isinstance(character, dict):
        server = str(character.get('server', '') or '').strip()[:100]
        name = str(character.get('name', '') or '').strip()[:64]
        if server and name:
            identity = {'server': server, 'name': name}
    if identity is None:
        return False
    payload = {'model': parsed['model'], 'notice': parsed['notice']}
    if parsed['killer']:
        payload['killer'] = parsed['killer']
    # The notice is server-wide. The observing character is attribution, not a map point.
    return _queue_canonical_event(
        'world.unique_spawned', 'world', 'joymax.unique_notice', '0x300C', payload, identity,
        position=False, dedupe_key='unique-notice:%s:%d:%d' % (parsed['notice'], parsed['model'], int(time.time()) // 15),
    )


def _thief_spawn_payload(data):
    """Attach the thief's own position when get_players has one matching name."""
    name = (_bounded_text(data, 512) or '').strip()
    payload = {'value': name, 'plugin_version': pVersion, 'position_source': 'observer'}
    _status, players, _truncated = collect_player_observation()
    folded = name.casefold()
    matches = [player for player in players
               if folded and isinstance(player.get('name'), str) and player['name'].casefold() == folded]
    if len(matches) != 1:
        return payload
    player = matches[0]
    region = player.get('region')
    x = player.get('x')
    y = player.get('y')
    if not _valid_position_region(region) or not _number(x) or not _number(y) or abs(x) > 1000000 or abs(y) > 1000000:
        return payload
    thief_name = player['name'].strip()
    while thief_name and len(thief_name.encode('utf-8')) > 64:
        thief_name = thief_name[:-1]
    thief_name = thief_name.strip()
    if not thief_name:
        return payload
    payload['position_source'] = 'thief'
    payload['thief'] = {'name': thief_name, 'region': int(region), 'x': float(x), 'y': float(y)}
    return payload


def handle_event(event_type, data):
    """Normalize documented event callbacks and enqueue without disk/network work."""
    global _death_callback_active, _recent_player_attack
    try:
        event_type = int(event_type)
    except Exception:
        return
    mapping = {
        EVENT_UNIQUE_SPAWN: ('world.unique_spawned', 'world', 'EVENT_UNIQUE_SPAWN'),
        EVENT_HUNTER_SPAWN: ('job.hunter_trader_seen', 'job', 'EVENT_HUNTER_SPAWN'),
        EVENT_THIEF_SPAWN: ('job.thief_seen', 'job', 'EVENT_THIEF_SPAWN'),
        EVENT_TRANSPORT_DIED: ('pet.transport_died', 'pet', 'EVENT_TRANSPORT_DIED'),
        EVENT_PLAYER_ATTACKING: ('character.attacked', 'character', 'EVENT_PLAYER_ATTACKING'),
        EVENT_RARE_DROP: ('drop.rare', 'drop', 'EVENT_RARE_DROP'),
        EVENT_ITEM_DROP: ('drop.item', 'drop', 'EVENT_ITEM_DROP'),
        EVENT_DIED: ('character.died', 'character', 'EVENT_DIED'),
        EVENT_ALCHEMY_FINISHED: ('alchemy.finished', 'alchemy', 'EVENT_ALCHEMY_FINISHED'),
        EVENT_GM_SPAWNED: ('world.gm_spawned', 'world', 'EVENT_GM_SPAWNED'),
        EVENT_LEVEL_UP: ('character.level_up', 'character', 'EVENT_LEVEL_UP'),
    }
    mapped = mapping.get(event_type)
    if mapped is None or event_type == EVENT_DIED and _death_callback_active:
        return
    try:
        character = _get_character_data()
    except Exception:
        character = None
    identity = None
    if isinstance(character, dict):
        server = str(character.get('server', '') or '').strip()[:100]
        name = str(character.get('name', '') or '').strip()[:64]
        if server and name:
            identity = {'server': server, 'name': name}
    kind, category, source_ref = mapped
    if identity is None:
        _log('phBot event callback had no character identity')
        return
    if event_type == EVENT_PLAYER_ATTACKING:
        attacker = (_bounded_text(data, 128) or '').strip()
        if attacker:
            _recent_player_attack = {
                'identity': (server.casefold(), name.casefold()),
                'name': attacker,
                'at': _monotonic(),
            }
        else:
            _recent_player_attack = None
    payload = {}
    item_model = None
    if event_type in (EVENT_ITEM_DROP, EVENT_RARE_DROP):
        try:
            item_model = int(str(data).strip())
        except Exception:
            _log('drop callback supplied an invalid item model')
            return
        if item_model < 0 or item_model > 2147483647:
            return
        payload = {'model': item_model}
    elif event_type == EVENT_LEVEL_UP:
        try:
            level = int(str(data).strip())
        except Exception:
            _log('level-up callback supplied an invalid level')
            return
        if level < 1 or level > 255:
            return
        payload = {'level': level}
    elif event_type == EVENT_DIED:
        payload = {
            'cause': 'monster_environment',
            'reason_type': 'monster_or_environment',
            'reason_value': 'Monster / environment',
        }
        attack = _recent_player_attack
        if attack and attack['identity'] == (server.casefold(), name.casefold()):
            age = _monotonic() - attack['at']
            if 0 <= age <= RECENT_PLAYER_ATTACK_SECONDS:
                payload = {
                    'cause': attack['name'],
                    'reason_type': 'attacker',
                    'reason_value': attack['name'],
                }
    elif event_type == EVENT_ALCHEMY_FINISHED:
        payload = {}
    elif event_type == EVENT_THIEF_SPAWN:
        payload = _thief_spawn_payload(data)
    else:
        payload = {'value': _bounded_text(data, 512) or ''}
    drop_event_id = str(uuid.uuid4()) if item_model is not None else None
    if _queue_canonical_event(kind, category, 'phbot.callback', source_ref, payload, identity,
                              item_model=item_model, event_id=drop_event_id):
        if event_type == EVENT_DIED:
            _death_callback_active = True
            _recent_player_attack = None
        if drop_event_id and _worker is not None:
            inventory_quantity = _inventory_model_quantity(item_model)
            if inventory_quantity is not None:
                _worker.register_drop_for_enrichment(identity, item_model, drop_event_id,
                                                     inventory_quantity)


def _safe_callback_text(value, maximum=2048):
    try:
        raw = str(value if value is not None else '').encode('utf-8')
    except Exception:
        return ''
    if len(raw) > maximum:
        raw = raw[:maximum]
    return raw.decode('utf-8', 'ignore')


def _queue_canonical_event(kind, category, source, source_ref, payload, identity=None,
                           item_model=None, item_code=None, position=True, dedupe_key=None,
                           event_id=None):
    event = {
        'event_id': event_id or str(uuid.uuid4()),
        'schema_version': 1,
        'kind': kind,
        'category': category,
        'occurred_at': _worker_utc_now(_worker) if _worker is not None else _utc_now(),
        'source': source,
        'source_ref': source_ref,
        'payload': payload if isinstance(payload, dict) else {},
    }
    if item_model is not None:
        event['item_model'] = item_model
        if isinstance(identity, dict) and _item_filters_enabled(_profile_config or {}):
            sort_type = _item_sort_type(identity.get('server'), item_model)
            event['_phmon_item_sort_type'] = sort_type
            if _ignore_item_event(_profile_config or {}, sort_type):
                return False
    if item_code:
        event['item_code'] = str(item_code)[:128]
    if dedupe_key:
        event['dedupe_key'] = str(dedupe_key)[:160]
    if position:
        try:
            observed_position = _get_position()
        except Exception:
            observed_position = None
        if isinstance(observed_position, dict):
            region = observed_position.get('region')
            if _valid_position_region(region):
                event['region'] = region
                zone = _zone_name_for_region(region)
                if zone:
                    event['zone'] = zone
            for axis in ('x', 'y', 'z'):
                value = observed_position.get(axis)
                if isinstance(value, (int, float)) and not isinstance(value, bool) and -1000000 <= value <= 1000000:
                    event[axis] = float(value)
    if _worker is not None:
        return _worker.queue_event(identity, event)
    global _pending_callback_overflow
    try:
        _pending_callback_events.put_nowait((dict(identity) if isinstance(identity, dict) else None, event))
        return True
    except _queue.Full:
        _pending_callback_overflow = True
        _log('activity callback queue is full; an occurrence needs operator attention')
        return False


def _lifecycle_event(kind, source_ref, include_identity=False):
    identity = None
    if include_identity:
        try:
            character = _get_character_data()
        except Exception:
            character = None
        if isinstance(character, dict) and character.get('server') and character.get('name'):
            identity = {'server': character['server'], 'name': character['name']}
    session = _worker.session_id if _worker is not None else None
    dedupe = ('lifecycle:' + str(session) + ':' + source_ref) if session and kind != 'session.teleported' else None
    return _queue_canonical_event(kind, 'session', 'phbot.lifecycle_callback', source_ref, {}, identity,
                                  position=include_identity, dedupe_key=dedupe)


def handle_chat(chat_type, player, message):
    """Preserve raw type and normalize the verified phBot channel values."""
    try:
        character = _get_character_data()
    except Exception:
        character = None
    identity = None
    if isinstance(character, dict) and character.get('server') and character.get('name'):
        identity = {'server': character['server'], 'name': character['name']}
    if identity is None:
        _log('phBot chat callback had no character identity')
        return
    sender = _bounded_text(player, 64)
    raw_type = _bounded_text(chat_type, 64)
    known_channels = {
        'all': 'general', 'general': 'general', 'private': 'private',
        'party': 'party', 'guild': 'guild', 'union': 'union', 'global': 'global',
        '1': 'general', '2': 'private', '4': 'party', '5': 'guild', '6': 'global',
    }
    channel = known_channels.get(raw_type.strip().lower()) if isinstance(raw_type, str) else None
    payload = {
        'channel': channel or 'unknown',
        'raw_type': raw_type,
        'sender': sender,
        'recipient': identity.get('name') if sender and identity else None,
        'direction': 'inbound',
        'message': _safe_callback_text(message, 2048),
    }
    _queue_canonical_event('chat.message_received', 'chat', 'phbot.chat_callback', 'handle_chat', payload, identity)


def alchemy_update(slot, success, plus):
    """Capture the documented callback values and a cached item snapshot if known."""
    if not isinstance(slot, int) or isinstance(slot, bool) or slot < 0 or slot > 4096:
        return
    success_value = success if isinstance(success, bool) else None
    plus_value = plus if isinstance(plus, int) and not isinstance(plus, bool) and 0 <= plus <= 255 else None
    payload = {'slot': slot, 'success': success_value, 'plus': plus_value}
    identity = None
    try:
        character = _get_character_data()
    except Exception:
        character = None
    if isinstance(character, dict) and character.get('server') and character.get('name'):
        identity = {'server': character['server'], 'name': character['name']}
    if identity is None:
        _log('phBot alchemy callback had no character identity')
        return
    item = _worker.latest_resource_item_at_slot(slot) if hasattr(_worker, 'latest_resource_item_at_slot') else None
    item_model = item.get('model') if item else None
    item_code = item.get('servername') if item else None
    if item:
        payload['item'] = item
    _queue_canonical_event('alchemy.attempt', 'alchemy', 'phbot.alchemy_callback', 'alchemy_update', payload,
                           identity, item_model=item_model, item_code=item_code)


def _item_snapshot_at_inventory_slot(resources, slot):
    if not isinstance(resources, dict):
        return None
    for key in ('equipment', 'inventory'):
        container = resources.get(key)
        if not isinstance(container, dict) or container.get('availability') != 'observed':
            continue
        for row in container.get('slots', []):
            if isinstance(row, dict) and row.get('source_slot') == slot and isinstance(row.get('item'), dict):
                return dict(row['item'])
    return None
def event_loop():
    # phBot calls this every 500 ms. Keep UI updates and profile detection here;
    # the worker owns backend I/O and only publishes its latest status string.
    timing = _CallbackTiming()
    try:
        try:
            timing.run('profile_sync', _load_active_profile)
        except Exception as error:
            _log('profile sync failed (' + error.__class__.__name__ + ')')
        if _worker is not None:
            timing.run('callback_events', _drain_pending_callback_events)
            timing.run('gui_status', _set_gui_status, _worker.status)
            _sample_character(timing)
    finally:
        timing.report()


def _drain_pending_callback_events():
    global _pending_callback_overflow
    if _worker is None:
        return
    while True:
        try:
            identity, event = _pending_callback_events.get_nowait()
        except _queue.Empty:
            break
        _worker.queue_event(identity, event)
    if _pending_callback_overflow:
        _worker.status = 'Activity callback queue overflowed before the event worker was available.'
        _pending_callback_overflow = False


def _sample_character(timing=None):
    global _last_character_signature, _last_character_sample_at, _last_resources_sample_at, _character_joined, _death_callback_active, _recent_player_attack
    if not _PHBOT_AVAILABLE or _worker is None or _character_joined is False:
        return
    timing = timing or _CallbackTiming()
    try:
        data = timing.run('character_data', _get_character_data)
    except Exception:
        data = None
    if not isinstance(data, dict) or not data.get('name') or not data.get('server'):
        return
    # phBot may load/reload a plugin after joined_game() already fired. A complete
    # get_character_data() identity is the documented signal that character data
    # has finished loading, so recover without waiting for another callback.
    if _character_joined is None:
        _character_joined = True
    state = {}
    for source in ('level','hp','hp_max','mp','mp_max','current_exp','max_exp','sp','gold','region'):
        value = data.get(source)
        if source == 'region' and _valid_position_region(value):
            state[source] = value
        elif source != 'region' and isinstance(value, (int,float)) and not isinstance(value,bool) and value >= 0:
            state[source] = int(value)
    model = data.get('model')
    if isinstance(model, int) and not isinstance(model, bool) and 1 <= model <= 0xffffffff:
        state['model'] = model
    if isinstance(data.get('dead'), bool):
        state['dead'] = data['dead']
        if data['dead'] is False:
            if _death_callback_active:
                _recent_player_attack = None
            _death_callback_active = False
    try:
        position = timing.run('position', _get_position)
    except Exception:
        position = None
    if isinstance(position, dict):
        for axis in ('x','y','z'):
            value = position.get(axis)
            if isinstance(value,(int,float)) and not isinstance(value,bool):
                state[axis] = float(value)
        region = position.get('region')
        if _valid_position_region(region):
            state['region'] = int(region)
    if isinstance(state.get('region'),int):
        zone = timing.run('zone_name', _zone_name_for_region, state['region'], 100)
        if zone:
            state['zone'] = zone
    state['botting'] = _read_botting_state(data, timing=timing)
    try:
        active_profile = timing.run('profile', _get_profile) if callable(_get_profile) else None
    except Exception:
        active_profile = None
    profile_key = _bounded_text(active_profile, 128)
    identity = {
        'server':str(data['server']).strip()[:100],
        'name':str(data['name']).strip()[:64],
        # Local-only identity fence: a profile switch must clear a route even
        # when phBot reports the same game server and character name.
        'profile_key':profile_key,
        'guild':(
            str(data['guild']).strip()[:100]
            if isinstance(data.get('guild'), str)
            else None
        ),
    }
    if not identity['server'] or not identity['name']:
        return
    now = _monotonic()
    _publish_realtime_position(position, now, identity)
    signature_state = dict(state)
    for axis in ('x', 'y', 'z'):
        signature_state.pop(axis, None)
    signature = json.dumps([identity,signature_state],sort_keys=True,separators=(',',':'))
    resource_interval = (DROP_ENRICHMENT_SAMPLE_INTERVAL_SECONDS
                         if _worker.has_pending_drop_enrichment(identity)
                         else RESOURCE_SAMPLE_INTERVAL_SECONDS)
    if now - _last_resources_sample_at >= resource_interval:
        try:
            config_dir = timing.run('config_dir', _get_config_dir) if callable(_get_config_dir) else None
        except Exception:
            config_dir = None
        try:
            locale_getter = _optional_phbot_api('get_locale')
            locale = timing.run('locale', locale_getter) if callable(locale_getter) else None
        except Exception:
            locale = None
        try:
            inputs = timing.run('resource_collect', collect_resource_inputs)
            timing.run('item_type_lookup', _cache_resource_item_types, inputs, identity['server'])
            timing.run('resource_publish', _worker.update_resource_inputs, identity, inputs, config_dir, locale, position)
            _last_resources_sample_at = now
        except Exception as error:
            _log('resource collection failed (' + error.__class__.__name__ + ')')
    if signature != _last_character_signature or now-_last_character_sample_at >= 5.0:
        timing.run('character_publish', _worker.update_character, identity,state)
        _last_character_signature = signature
        _last_character_sample_at = now
    timing.run('monsters', _sample_monsters, identity, state, position, now)
    timing.run('npcs', _sample_npcs, identity, state, position, now)
    timing.run('players', _sample_players, identity, state, position, now)
    if hasattr(_worker, 'report_control_state'):
        try: timing.run('controls', _worker.report_control_state)
        except Exception as error: _log('control-state sample failed (' + error.__class__.__name__ + ')')
    # Mutations run only on phBot's event_loop callback, after refreshing identity
    # and region. The network worker only validates/enqueues command frames.
    if hasattr(_worker, 'process_one_command'):
        try: timing.run('command', _worker.process_one_command, identity, state.get('region'))
        except Exception as error: _log('command callback failed (' + error.__class__.__name__ + ')')
    if hasattr(_worker, 'process_walk_step'):
        try: timing.run('walk_progress', _worker.process_walk_step, identity, state.get('region'))
        except Exception as error: _log('walk callback failed (' + error.__class__.__name__ + ')')


def _sample_monsters(identity, state, position, now=None):
    global _last_monster_poll_at, _last_mob_cell_samples
    if _worker is None:
        return False
    now = _monotonic() if now is None else now
    if now - _last_monster_poll_at < MOB_POLL_INTERVAL_SECONDS:
        return False
    _last_monster_poll_at = now
    status, monsters, truncated = collect_monster_observation()
    region = state.get('region') if isinstance(state, dict) else None
    if not _valid_position_region(region):
        return False
    matching = [monster for monster in monsters if _mob_region_matches(region, monster.get('region'))]
    if len(matching) != len(monsters):
        truncated = True
        status = 'truncated'
    if truncated:
        status = 'truncated'
    elif status == 'observed':
        status = 'observed'
    worker_character = getattr(_worker, 'character_id', None)
    worker_session = getattr(_worker, 'session_id', None)
    current_identity = getattr(_worker, '_current_identity', None)
    sample = None
    x = position.get('x') if isinstance(position, dict) else None
    y = position.get('y') if isinstance(position, dict) else None
    if (status == 'observed' and worker_character and worker_session and
            _worker._identity_key(identity) == _worker._identity_key(current_identity) and
            _number(x) and _number(y) and abs(x) <= 1000000 and abs(y) <= 1000000):
        cell = (int(math.floor(float(x) / MOB_OBSERVER_CELL_SIZE)),
                int(math.floor(float(y) / MOB_OBSERVER_CELL_SIZE)))
        throttle_key = (worker_session, region, 'unmapped', cell[0], cell[1])
        last_sampled = _last_mob_cell_samples.get(throttle_key)
        if last_sampled is None or now - last_sampled >= MOB_SAMPLE_INTERVAL_SECONDS:
            sampled_at = _worker_utc_now(_worker)
            observer = {'x': float(x), 'y': float(y)}
            z = position.get('z') if isinstance(position, dict) else None
            if _number(z) and abs(z) <= 1000000:
                observer['z'] = float(z)
            sample = {
                'sample_id': str(uuid.uuid4()), 'character_id': worker_character,
                'session_id': worker_session, 'area_id': 'region:' + str(region),
                'floor_id': 'unmapped', 'region': region, 'sampled_at': sampled_at,
                'observer': observer, 'monsters': matching,
            }
            if _worker.update_map_monsters(identity, status, region, matching, sample, position.get('z')):
                _last_mob_cell_samples[throttle_key] = now
            else:
                return False
            return True
    _worker.update_map_monsters(identity, status, region, matching,
                                observer_z=position.get('z') if isinstance(position, dict) else None)
    if len(_last_mob_cell_samples) > 4096:
        oldest = sorted(_last_mob_cell_samples.items(), key=lambda item: item[1])[:2048]
        for key, _ in oldest:
            _last_mob_cell_samples.pop(key, None)
    return True


def _sample_npcs(identity, state, position, now=None):
    global _last_npc_poll_at, _last_npc_region, _last_npc_signature, _last_npc_publish_at, _npc_sample_forced
    if _worker is None:
        return False
    now = _monotonic() if now is None else now
    region = state.get('region') if isinstance(state, dict) else None
    if not _valid_position_region(region):
        return False
    forced = _npc_sample_forced or _last_npc_region != region
    if not forced and now - _last_npc_poll_at < NPC_POLL_INTERVAL_SECONDS:
        return False
    _last_npc_poll_at = now
    _npc_sample_forced = False
    status, npcs, truncated = collect_npc_observation()
    matching = [npc for npc in npcs if _mob_region_matches(region, npc.get('region'))]
    if len(matching) != len(npcs):
        truncated = True
    if truncated:
        status = 'truncated'
    if _worker is not None:
        server_name = identity.get('server') if isinstance(identity, dict) else ''
        _attach_teleporter_routes(
            matching,
            getattr(_worker, 'api', None),
            server_name if isinstance(server_name, str) else '',
            getattr(_worker, 'session_id', '') or '',
        )
    signature = _npc_snapshot_signature(status, region, matching)
    changed = signature != _last_npc_signature
    refresh_due = (_last_npc_publish_at == 0.0 or
                   now - _last_npc_publish_at >= NPC_REFRESH_INTERVAL_SECONDS)
    _last_npc_region = region
    if not forced and not changed and not refresh_due:
        return False
    observer_z = position.get('z') if isinstance(position, dict) else None
    _worker.update_map_npcs(identity, status, region, matching, observer_z=observer_z)
    _last_npc_signature = signature
    _last_npc_publish_at = now
    return True


def _sample_players(identity, state, position, now=None):
    global _last_player_poll_at, _last_player_region, _last_player_observer_z
    global _last_player_signature, _last_player_publish_at, _player_sample_forced
    if _worker is None:
        return False
    now = _monotonic() if now is None else now
    region = state.get('region') if isinstance(state, dict) else None
    if not _valid_position_region(region):
        return False
    observer_z = position.get('z') if isinstance(position, dict) else None
    normalized_z = float(observer_z) if _number(observer_z) and abs(observer_z) <= 1000000 else None
    forced = (_player_sample_forced or _last_player_region != region or
              _last_player_observer_z != normalized_z)
    if not forced and now - _last_player_poll_at < PLAYER_POLL_INTERVAL_SECONDS:
        return False
    _last_player_poll_at = now
    _player_sample_forced = False
    status, players, truncated = collect_player_observation()
    matching = [player for player in players
                if player.get('region') is None or _mob_region_matches(region, player.get('region'))]
    if len(matching) != len(players):
        truncated = True
    if truncated:
        status = 'truncated'
    if any(not player.get('zone') and not _valid_position_region(player.get('region'))
           for player in matching):
        observer_zone = _zone_name_for_region(region)
        if observer_zone:
            for player in matching:
                if not player.get('zone') and not _valid_position_region(player.get('region')):
                    player['zone'] = observer_zone
    signature = _player_snapshot_signature(status, region, normalized_z, matching)
    changed = signature != _last_player_signature
    refresh_due = (_last_player_publish_at == 0.0 or
                   now - _last_player_publish_at >= PLAYER_REFRESH_INTERVAL_SECONDS)
    _last_player_region = region
    _last_player_observer_z = normalized_z
    if not forced and not changed and not refresh_due:
        return False
    _worker.update_map_players(identity, status, region, matching, observer_z=observer_z)
    _last_player_signature = signature
    _last_player_publish_at = now
    return True


def finished():
    _stop_worker()


if _PHBOT_AVAILABLE and _QtBind is not None:
    _gui = _QtBind.init(__name__, pName)
    _QtBind.createLabel(_gui, 'Loaded: ' + pName + ' v' + pVersion + ' (pet packet probe r1)', 390, 10)
    _QtBind.createLabel(_gui, 'Ignore item events', 390, 42)
    _gui_ignore_potions = _QtBind.createCheckBox(_gui, 'item_filter_changed', 'Potions', 390, 66)
    _gui_ignore_pills = _QtBind.createCheckBox(_gui, 'item_filter_changed', 'Pills', 390, 91)
    _QtBind.setChecked(_gui, _gui_ignore_potions, True)
    _QtBind.setChecked(_gui, _gui_ignore_pills, True)
    _QtBind.createLabel(_gui, 'Apply changes with Save & Connect.', 390, 118)
    _QtBind.createLabel(_gui, 'Backend WebSocket URL', 10, 10)
    _gui_backend_url = _QtBind.createLineEdit(_gui, '', 10, 30, 360, 20)
    _QtBind.createLabel(_gui, 'Agent ID', 10, 60)
    _gui_agent_id = _QtBind.createLineEdit(_gui, '', 10, 80, 360, 20)
    _QtBind.createLabel(_gui, 'Agent token (paste to set or replace)', 10, 110)
    _gui_agent_token = _QtBind.createLineEdit(_gui, '', 10, 130, 360, 20)
    _QtBind.createButton(_gui, 'save_config', 'Save & Connect', 10, 165)
    _QtBind.createButton(_gui, 'probe_teleporters', 'Probe teleporters', 10, 195)
    _QtBind.createButton(_gui, 'test_teleport_hotan_jangan', 'Test Hotan→Jangan', 180, 195)
    _gui_status = _QtBind.createLabel(
        _gui,
        'Join the game to select a bot profile.',
        10,
        260,
    )
    try:
        _load_active_profile(force=True)
    except Exception as error:
        _log('profile sync failed (' + error.__class__.__name__ + ')')
elif _PHBOT_AVAILABLE:
    _log('QtBind GUI API is unavailable; PhMon cannot be configured')
