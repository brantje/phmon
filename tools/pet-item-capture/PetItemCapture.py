# -*- coding: utf-8 -*-
"""Temporary read-only phBot probe for pet inventory item packets.

Evidence gate for a verified pet item decoder. This plugin never injects
packets, never contacts PhMon and never reads chat or credentials. While armed
it records, into one bounded JSON file next to this plugin:

* the raw payload of incoming 0x30C8 and 0xB034 packets (opcode, length, hex),
* get_pets() and get_inventory() item rows sampled about once per second.

Arm it only in the chosen test session and remove it after the capture.
"""

import json
import os
import time

from phBot import get_inventory, get_pets, log
import QtBind

pName = 'Pet Item Capture'
pVersion = '0.1.0'

CAPTURE_SECONDS = 600.0
SAMPLE_INTERVAL_SECONDS = 1.0
MAX_PACKETS = 64
MAX_PACKET_BYTES = 4096
MAX_SNAPSHOTS = 400
MAX_ITEM_ROWS = 128
MAX_FILE_BYTES = 2 * 1024 * 1024
PET_OPCODES = (0x30C8, 0xB034)
# None writes beside this plugin file.
OUTPUT_DIRECTORY = None

_capture_until = 0.0
_next_sample_at = 0.0
_started_at = 0.0
_packets = []
_packet_total = 0
_snapshots = []
_last_signature = None
_pending_after_packet = False
_file_path = None


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


def _item_row(slot, item):
    if not isinstance(item, dict):
        return None
    return {'slot': slot, 'fields': _bounded(item)}


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
    for pet_id, pet in list(pets.items())[:8]:
        if not isinstance(pet, dict):
            continue
        result[str(pet_id)[:32]] = {
            'type': _bounded(pet.get('type')),
            'servername': _bounded(pet.get('servername')),
            'items': _rows(pet.get('items')),
        }
    return result


def _inventory_rows():
    try:
        inventory = get_inventory()
    except Exception as error:
        return {'error': type(error).__name__}
    if not isinstance(inventory, dict):
        return {'error': 'not_a_dict'}
    return _rows(inventory.get('items'))


def _snapshot(reason):
    return {
        't': round(time.monotonic() - _started_at, 3),
        'reason': reason,
        'pets': _pet_rows(),
        'inventory': _inventory_rows(),
    }


def arm_pet_capture():
    global _capture_until, _next_sample_at, _started_at, _packets, _packet_total
    global _snapshots, _last_signature, _pending_after_packet, _file_path
    _started_at = time.monotonic()
    _capture_until = _started_at + CAPTURE_SECONDS
    _next_sample_at = 0.0
    _packets = []
    _packet_total = 0
    _snapshots = []
    _last_signature = None
    _pending_after_packet = False
    _file_path = os.path.join(
        OUTPUT_DIRECTORY or os.path.dirname(os.path.abspath(__file__)),
        'pet-item-capture-%s.json' % time.strftime('%Y%m%dT%H%M%SZ', time.gmtime()))
    _snapshots.append(_snapshot('armed'))
    log('PetItemCapture armed for %d s; summon the pet, let it pick up items, then move one to the bag'
        % int(CAPTURE_SECONDS))


def stop_pet_capture():
    _finish('operator')


def handle_joymax(opcode, data):
    global _packet_total, _pending_after_packet
    if _capture_until and time.monotonic() < _capture_until and opcode in PET_OPCODES:
        _packet_total += 1
        _pending_after_packet = True
        if len(_packets) < MAX_PACKETS and isinstance(data, (bytes, bytearray)):
            payload = bytes(data)[:MAX_PACKET_BYTES]
            _packets.append({
                't': round(time.monotonic() - _started_at, 3),
                'opcode': '0x%04X' % opcode,
                'length': len(data),
                'truncated': len(data) > MAX_PACKET_BYTES,
                'hex': payload.hex(),
            })
    return True


def _write_file(reason):
    document = {
        'schema': 1, 'plugin': pName, 'version': pVersion, 'reason': reason,
        'packet_total': _packet_total, 'packets': _packets, 'snapshots': _snapshots,
    }
    text = json.dumps(document, separators=(',', ':'), sort_keys=True)
    while len(text.encode('utf-8')) > MAX_FILE_BYTES and document['snapshots']:
        document['snapshots'] = document['snapshots'][::2]
        document['thinned'] = True
        text = json.dumps(document, separators=(',', ':'), sort_keys=True)
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
    _last_signature = signature
    if len(_snapshots) < MAX_SNAPSHOTS:
        _snapshots.append(snapshot)


_gui = QtBind.init(__name__, pName)
QtBind.createLabel(_gui, 'Read-only capture of pet item packets; sends nothing', 10, 10)
QtBind.createButton(_gui, 'arm_pet_capture', 'Arm 10-minute capture', 10, 40)
QtBind.createButton(_gui, 'stop_pet_capture', 'Stop and write file', 10, 70)
