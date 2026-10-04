# -*- coding: utf-8 -*-
"""Temporary read-only phBot probe for a manual recall-point designation.

This plugin never injects packets. Arm it only in the chosen test session and
remove it after the bounded capture. It logs payloads only for the two candidate
teleporter opcodes; other packet records contain opcode and length only.
"""

import json
import time

from phBot import get_npcs, log
import QtBind

pName = 'Recall Point Capture'
pVersion = '0.1.1'

_capture_until = 0.0
_outgoing_count = 0
_incoming_count = 0
_MAX_OUTGOING = 48
_MAX_INCOMING = 48


def _gate_rows():
    try:
        rows = get_npcs() or {}
    except Exception:
        return []
    if not isinstance(rows, dict):
        return []
    gates = []
    for npc_id, row in rows.items():
        if not isinstance(row, dict):
            continue
        servername = row.get('servername')
        if not isinstance(servername, str) or not servername.startswith('GATE_'):
            continue
        gates.append({
            'id': str(npc_id),
            'name': str(row.get('name') or '')[:80],
            'servername': servername[:80],
            'region': row.get('region'),
        })
        if len(gates) >= 16:
            break
    return gates


def arm_recall_capture():
    global _capture_until, _outgoing_count, _incoming_count
    _capture_until = time.monotonic() + 15.0
    _outgoing_count = 0
    _incoming_count = 0
    log('RecallCapture armed for 15s; nearby gates=' + json.dumps(_gate_rows(), sort_keys=True))
    log('RecallCapture: manually choose Designate Recall Point now; this probe sends no packets')


def handle_silkroad(opcode, data):
    global _outgoing_count
    if time.monotonic() < _capture_until and _outgoing_count < _MAX_OUTGOING:
        if isinstance(opcode, int) and 0x7000 <= opcode <= 0x70FF:
            _outgoing_count += 1
            size = len(data) if isinstance(data, (bytes, bytearray)) else -1
            detail = ''
            if opcode in (0x7045, 0x7059) and 0 <= size <= 16:
                detail = ' payload=' + bytes(data).hex()
            log('RecallCapture outgoing opcode=0x%04X length=%d%s' % (opcode, size, detail))
    return True


def handle_joymax(opcode, data):
    global _incoming_count
    if time.monotonic() < _capture_until and _incoming_count < _MAX_INCOMING:
        if isinstance(opcode, int):
            _incoming_count += 1
            size = len(data) if isinstance(data, (bytes, bytearray)) else -1
            detail = ' payload=' + bytes(data).hex() if opcode == 0xB059 and size == 1 else ''
            log('RecallCapture incoming opcode=0x%04X length=%d%s' % (opcode, size, detail))
    return True


def event_loop():
    global _capture_until
    if _capture_until and time.monotonic() >= _capture_until:
        _capture_until = 0.0
        log('RecallCapture complete outgoing=%d incoming=%d' % (_outgoing_count, _incoming_count))


_gui = QtBind.init(__name__, pName)
QtBind.createLabel(_gui, 'Read-only manual capture; no packets sent', 10, 10)
QtBind.createButton(_gui, 'arm_recall_capture', 'Arm 15-second capture', 10, 40)
