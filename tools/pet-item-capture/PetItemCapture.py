# -*- coding: utf-8 -*-
"""Temporary read-only phBot probe for pet inventory item packets.

Evidence gate for a verified pet item decoder. This plugin never injects
packets, never contacts PhMon and never reads chat or credentials. While armed
it records, into one bounded JSON file next to this plugin:

* incoming 0x30C8 / 0xB034 pet data and item operations, and outgoing
  0x7034 item-move requests (direction, opcode, length, bounded hex),
* get_pets() and get_inventory() item rows sampled about once per second,
  including each pet's type, model, name, mounted flag and reported hp/level-like
  primitive fields (any pet class: attack, fellow, pick, transport, horse, wolf),
* lead stat-update packets (0x3057 / 0x30C9) but ONLY when their first four
  bytes equal the ID of a currently summoned pet; each lead opcode is capped.

Arm it only in the chosen test session and remove it after the capture.
"""

import json
import os
import time

from phBot import get_inventory, get_pets, log
import QtBind

pName = 'Pet Item Capture'
pVersion = '0.5.0'

CAPTURE_SECONDS = 600.0
SAMPLE_INTERVAL_SECONDS = 1.0
MAX_PACKETS = 64
MAX_PACKET_BYTES = 32768
MAX_SNAPSHOTS = 400
MAX_ITEM_ROWS = 128
MAX_FILE_BYTES = 2 * 1024 * 1024
SERVER_OPCODES = (0x30C8, 0xB034)
CLIENT_OPCODES = (0x7034,)
# Unverified leads for pet HP/level updates. Recorded only when the packet
# starts with a current pet ID (little-endian u32), so entity traffic of
# other units is never kept.
PET_STAT_LEAD_OPCODES = (0x3057, 0x30C9)
# Cap per packet *shape* (opcode, length, bytes 4..6) so frequent HP updates
# cannot crowd out a rarer update type such as hunger.
MAX_LEAD_PACKETS_PER_OPCODE = 6
MAX_LEAD_SHAPES = 64
# Any other opcode whose first four bytes are a current pet ID is kept too
# (bounded), because the real hunger update opcode is not yet known.
# Opcode counts only (no bytes) for all other server packets.
_seen_opcodes = {}
# None writes beside this plugin file.
OUTPUT_DIRECTORY = None

_capture_until = 0.0
_next_sample_at = 0.0
_started_at = 0.0
_packets = []
_packet_total = 0
_packet_counts = {}
_snapshots = []
_last_signature = None
_pending_after_packet = False
_file_path = None
_last_inventory = None
_pet_ids = set()
_lead_counts = {}


def _bounded(value, depth=0):
    if depth > 4:
        return None
    if value is None or isinstance(value, (bool, int, float)):
        return value
    if isinstance(value, str):
        return value[:200]
    if isinstance(value, (list, tuple)):
        return [_bounded(entry, depth + 1) for entry in list(value)[:32]]
    if isinstance(value, dict):
        result = []
        for key, entry in list(value.items())[:64]:
            result.append([_bounded(key, depth + 1), _bounded(entry, depth + 1)])
        return {'pairs': result}
    return str(type(value).__name__)


# Fields that change every sample and say nothing about the item instance.
VOLATILE_FIELDS = ('expiration',)


def _item_row(slot, item):
    if not isinstance(item, dict):
        return None
    stable = dict((key, value) for key, value in item.items()
                  if key not in VOLATILE_FIELDS)
    return {'slot': slot, 'fields': _bounded(stable)}


def _rows(items):
    rows = []
    if not isinstance(items, (list, tuple)):
        return rows
    for slot, item in enumerate(items):
        if len(rows) >= MAX_ITEM_ROWS:
            break
        row = _item_row(slot, item)
        if row is not None and row['fields']:
            rows.append(row)
    return rows


def _pet_rows():
    try:
        pets = get_pets()
    except Exception as error:
        return {'error': type(error).__name__}
    if not isinstance(pets, dict):
        return {'error': 'not_a_dict'}
    result = {}
    ids = set()
    for pet_id, pet in list(pets.items())[:8]:
        if not isinstance(pet, dict):
            continue
        row = {
            'type': _bounded(pet.get('type')),
            'servername': _bounded(pet.get('servername')),
            'items': _rows(pet.get('items')),
        }
        for key, value in list(pet.items())[:32]:
            if key != 'items' and key not in row and isinstance(value, (int, float, bool, str)):
                row[str(key)[:32]] = _bounded(value)
        result[str(pet_id)[:32]] = row
        if str(pet_id).isdigit():
            ids.add(int(pet_id))
    _pet_ids.clear()
    _pet_ids.update(ids)
    return result


def _inventory_rows():
    try:
        inventory = get_inventory()
    except Exception as error:
        return {'error': type(error).__name__}
    if not isinstance(inventory, dict):
        return {'error': 'not_a_dict'}
    return _rows(inventory.get('items'))


def _inventory_changes(rows):
    """Return only the slots whose stable fields changed since the last sample."""
    global _last_inventory
    if not isinstance(rows, list):
        return rows
    current = dict((row['slot'], json.dumps(row['fields'], sort_keys=True)) for row in rows)
    if _last_inventory is None:
        changed = rows
    else:
        changed = [row for row in rows if _last_inventory.get(row['slot']) != current[row['slot']]]
        for slot in _last_inventory:
            if slot not in current:
                changed.append({'slot': slot, 'fields': None, 'removed': True})
    _last_inventory = current
    return changed


def _snapshot(reason):
    return {
        't': round(time.monotonic() - _started_at, 3),
        'reason': reason,
        'pets': _pet_rows(),
        'inventory': _inventory_changes(_inventory_rows()),
    }


def _first_pets_marker():
    return _snapshots[-1]['pets'] if _snapshots else None


def arm_pet_capture():
    global _capture_until, _next_sample_at, _started_at, _packets, _packet_total, _packet_counts
    global _snapshots, _last_signature, _pending_after_packet, _file_path, _last_inventory, _lead_counts
    _started_at = time.monotonic()
    _capture_until = _started_at + CAPTURE_SECONDS
    _next_sample_at = 0.0
    _packets = []
    _packet_total = 0
    _packet_counts = {}
    _snapshots = []
    _last_signature = None
    _pending_after_packet = False
    _last_inventory = None
    _lead_counts = {}
    _seen_opcodes.clear()
    _pet_ids.clear()
    _file_path = os.path.join(
        OUTPUT_DIRECTORY or os.path.dirname(os.path.abspath(__file__)),
        'pet-item-capture-%s.json' % time.strftime('%Y%m%dT%H%M%SZ', time.gmtime()))
    first = _snapshot('armed')
    _snapshots.append(first)
    pets = first['pets']
    if not pets or 'error' in pets:
        log('PetItemCapture: no pet at arm time; summon the test pet during this capture.')
    log('PetItemCapture armed for %d s; summon the pet, let it pick up items, then move one to the bag'
        % int(CAPTURE_SECONDS))


def stop_pet_capture():
    _finish('operator')


def disconnected():
    _finish('disconnected')


def finished():
    _finish('unloaded')


def _capture_packet(direction, opcode, data):
    global _packet_total, _pending_after_packet
    allowed = SERVER_OPCODES if direction == 'server_to_client' else CLIENT_OPCODES
    pet_id_match = (isinstance(data, (bytes, bytearray)) and len(data) >= 4 and
                    int.from_bytes(bytes(data[:4]), 'little') in _pet_ids)
    if (_capture_until and time.monotonic() < _capture_until and
            direction == 'server_to_client' and opcode not in SERVER_OPCODES):
        if opcode not in PET_STAT_LEAD_OPCODES:
            _seen_opcodes['0x%04X' % opcode] = _seen_opcodes.get('0x%04X' % opcode, 0) + 1
        shape = (opcode, len(data) if pet_id_match else 0,
                 bytes(data[4:7]).hex() if pet_id_match else '')
        if (pet_id_match and (shape in _lead_counts or len(_lead_counts) < MAX_LEAD_SHAPES) and
                _lead_counts.get(shape, 0) < MAX_LEAD_PACKETS_PER_OPCODE):
            _lead_counts[shape] = _lead_counts.get(shape, 0) + 1
            key = 'server_to_client:0x%04X' % opcode
            _packet_counts[key] = _packet_counts.get(key, 0) + 1
            _packets.append({
                't': round(time.monotonic() - _started_at, 3),
                'direction': direction, 'opcode': '0x%04X' % opcode,
                'length': len(data), 'truncated': len(data) > 512,
                'hex': bytes(data)[:512].hex(), 'pet_stat_lead': True,
            })
            _pending_after_packet = True
        return
    if _capture_until and time.monotonic() < _capture_until and opcode in allowed:
        _packet_total += 1
        key = '%s:0x%04X' % (direction, opcode)
        _packet_counts[key] = _packet_counts.get(key, 0) + 1
        if direction == 'server_to_client' and opcode == 0xB034 and isinstance(data, (bytes, bytearray)):
            # The first byte is the operation result. Count only successful
            # operation subtypes; this is a diagnostic, not an item decoder.
            if len(data) >= 2 and data[0] == 1:
                subtype_key = key + ':0x%02X' % data[1]
                _packet_counts[subtype_key] = _packet_counts.get(subtype_key, 0) + 1
        _pending_after_packet = True
        if len(_packets) < MAX_PACKETS and isinstance(data, (bytes, bytearray)):
            payload = bytes(data)[:MAX_PACKET_BYTES]
            _packets.append({
                't': round(time.monotonic() - _started_at, 3),
                'direction': direction,
                'opcode': '0x%04X' % opcode,
                'length': len(data),
                'truncated': len(data) > MAX_PACKET_BYTES,
                'hex': payload.hex(),
            })


def handle_joymax(opcode, data):
    _capture_packet('server_to_client', opcode, data)
    return True


def handle_silkroad(opcode, data):
    _capture_packet('client_to_server', opcode, data)
    return True


def _write_file(reason):
    document = {
        'schema': 1, 'plugin': pName, 'version': pVersion, 'reason': reason,
        'packet_total': _packet_total, 'packet_counts': _packet_counts,
        'packets': _packets, 'snapshots': _snapshots,
        'other_server_opcode_counts': _seen_opcodes,
    }
    text = json.dumps(document, separators=(',', ':'), sort_keys=True)
    while len(text.encode('utf-8')) > MAX_FILE_BYTES and len(document['snapshots']) > 2:
        snapshots = document['snapshots']
        middle = snapshots[1:-1]
        document['snapshots'] = [snapshots[0]] + (middle[::2] if len(middle) > 1 else []) + [snapshots[-1]]
        document['thinned'] = True
        text = json.dumps(document, separators=(',', ':'), sort_keys=True)
    # Packet payloads alone can exceed the file budget. Preserve the first pet
    # snapshot when possible and report every omission rather than writing a
    # file larger than the advertised cap.
    while len(text.encode('utf-8')) > MAX_FILE_BYTES and document['packets']:
        document['packets'].pop()
        document['packets_omitted_for_size'] = document.get('packets_omitted_for_size', 0) + 1
        text = json.dumps(document, separators=(',', ':'), sort_keys=True)
    if len(text.encode('utf-8')) > MAX_FILE_BYTES:
        raise ValueError('capture exceeds file limit')
    with open(_file_path, 'w') as handle:
        handle.write(text)


def _finish(reason):
    global _capture_until
    if not _capture_until:
        return
    _capture_until = 0.0
    try:
        _write_file(reason)
        log('PetItemCapture wrote %s (%d packets seen, %d kept, %d snapshots)'
            % (os.path.basename(_file_path), _packet_total, len(_packets), len(_snapshots)))
        if not _packet_counts.get('server_to_client:0xB034'):
            log('PetItemCapture: no 0xB034 item operation was seen; this file cannot verify pet pickup or pet-to-bag movement.')
        elif not _packet_counts.get('server_to_client:0xB034:0x11'):
            log('PetItemCapture: no candidate floor-to-pet 0xB034 subtype 0x11 was seen; pet moves alone cannot verify item detail bytes.')
    except Exception as error:
        log('PetItemCapture could not write its file: %s' % type(error).__name__)


def event_loop():
    global _next_sample_at, _last_signature, _pending_after_packet
    if not _capture_until:
        return
    now = time.monotonic()
    if now >= _capture_until:
        _finish('timeout')
        return
    if now < _next_sample_at and not _pending_after_packet:
        return
    reason = 'after_packet' if _pending_after_packet else 'interval'
    _pending_after_packet = False
    _next_sample_at = now + SAMPLE_INTERVAL_SECONDS
    snapshot = _snapshot(reason)
    signature = json.dumps([snapshot['pets'], snapshot['inventory']], sort_keys=True)
    if signature == _last_signature and reason == 'interval':
        return
    if reason == 'interval' and not snapshot['inventory'] and snapshot['pets'] == _first_pets_marker():
        return
    _last_signature = signature
    if len(_snapshots) < MAX_SNAPSHOTS:
        _snapshots.append(snapshot)


_gui = QtBind.init(__name__, pName)
QtBind.createLabel(_gui, 'Read-only capture of pet item packets; sends nothing', 10, 10)
QtBind.createButton(_gui, 'arm_pet_capture', 'Arm 10-minute capture', 10, 40)
QtBind.createButton(_gui, 'stop_pet_capture', 'Stop and write file', 10, 70)
