# -*- coding: utf-8 -*-
from __future__ import print_function

import base64
import calendar
import hashlib
import json
import math
import os
import random
import select
import socket
import ssl
import struct
import threading
import time
try:
    import queue as _queue
except ImportError:  # pragma: no cover
    import Queue as _queue
try:
    from urllib.parse import urlparse
except ImportError:  # pragma: no cover - Python 2 is not supported, kept harmless for embedded variants.
    from urlparse import urlparse

pName = 'PhMon'
pVersion = '1.1.2'
pUrl = ''

PROTOCOL_VERSION = 3
DEFAULT_HEARTBEAT_INTERVAL = 10
DEFAULT_HEARTBEAT_TIMEOUT = 30
MAX_MESSAGE_BYTES = 8192
_WEBSOCKET_GUID = '258EAFA5-E914-47DA-95CA-C5AB0DC85B11'
MAX_WALK_WAYPOINTS = 256
WALK_ARRIVAL_TOLERANCE = 12.0
WALK_TIMEOUT_SECONDS = 300.0

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

_API_NAMES = ('start_bot','stop_bot','start_trace','stop_trace','get_position','generate_path','set_training_position',
              'set_training_radius','set_training_area','get_training_area','move_to_region',
              'use_return_scroll','disconnect')

class PhBotAdapter(object):
    """Narrow allowlisted wrapper over documented phBot functions."""
    def __init__(self, functions=None):
        self.functions = functions or dict((name, _optional_phbot_api(name)) for name in _API_NAMES)
        self.get_character_data = self.functions.get('get_character_data') or _optional_phbot_api('get_character_data')
        self.get_position = self.functions.get('get_position') or _optional_phbot_api('get_position')
    def has(self, name):
        if name == 'get_position': return callable(self.get_position)
        return callable(self.functions.get(name))
    def call(self, name, *args):
        function = self.functions.get(name)
        if not callable(function): raise RuntimeError('unsupported_runtime_primitive')
        return function(*args)
    def character(self): return self.get_character_data() if callable(self.get_character_data) else None
    def position(self): return self.get_position() if callable(self.get_position) else None

def _result_json(value):
    try: return json.dumps(value, separators=(',', ':'), allow_nan=False)
    except Exception: return 'null'

def _number(value):
    return isinstance(value, (int, float)) and not isinstance(value, bool) and value == value and abs(value) != float('inf')

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


def _utc_now():
    return time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())


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
    return normalized


_CONFIG_KEYS = ('backend_url', 'agent_id', 'agent_token')


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
            if key not in _CONFIG_KEYS or key in values:
                raise ValueError('invalid saved configuration')
            values[key] = value
    if set(values) != set(_CONFIG_KEYS):
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
        for key in _CONFIG_KEYS:
            handle.write(key + '=' + normalized[key] + '\n')
    os.replace(temporary, path)
    return normalized


def config_from_gui_values(backend_url, agent_id, agent_token, saved_config=None):
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


class AgentWorker(object):
    def __init__(self, config, phbot_version, websocket_factory=None, api_adapter=None):
        self.config = validate_config(config)
        self.phbot_version = str(phbot_version)
        self.websocket_factory = websocket_factory or WebSocketClient
        self.api = api_adapter or PhBotAdapter()
        self.stop_event = threading.Event()
        self._thread = None
        self._socket = None
        self._socket_lock = threading.Lock()
        self.status = 'Connecting to PhMon backend...'
        self._samples = _queue.Queue(maxsize=1)
        self._latest_sample = None
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
        self._server_clock_offset = 0.0
        self._last_control_sample_session = None
        self._last_control_sample_at = 0.0

    def update_character(self, identity, state):
        self._replace_sample({'identity': dict(identity), 'state': dict(state)})

    def leave_character(self):
        if self._active_walk is not None:
            self._finish_walk('unknown', 'character_left_during_walk', 'unverified')
        self._discard_pending_commands('character_left')
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
        with self._socket_lock:
            client = self._socket
        if client is not None:
            try: client.close()
            except Exception: pass

    def _set_socket(self, value):
        with self._socket_lock:
            self._socket = value

    def _run(self):
        backoff = ReconnectBackoff()
        while not self.stop_event.is_set():
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
                self.character_id = None
                self._current_identity = None
                self._rejected_identity = None
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
                            self._latest_sample = None

                        else:
                            self._latest_sample = sample
                            self._publish_sample(client, sample, False)
                        continue
                    except _queue.Empty:
                        pass
                    now = _monotonic()
                    if now >= next_heartbeat:
                        client.send_json({
                            'type': 'heartbeat',
                            'protocol_version': PROTOCOL_VERSION,
                            'sent_at': _utc_now(),
                        })
                        next_heartbeat = now + interval
                        continue
                    wait = min(next_heartbeat - now, 1.0)
                    message = client.receive_json(timeout=wait)
                    if message is not None:
                        self._handle_server_message(message)
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
                if client is not None:
                    client.close()
                self._set_socket(None)
                self.character_id = None
                self._current_identity = None
                self.session_id = None

            if not self.stop_event.is_set():
                self.stop_event.wait(backoff.next_delay())

    def _publish_sample(self, client, sample, snapshot):
        identity, state = sample['identity'], sample['state']
        identity_key = self._identity_key(identity)
        if self._rejected_identity == identity_key:
            return
        if self._rejected_identity is not None and self._rejected_identity != identity_key:
            self._rejected_identity = None
        if identity != self._current_identity or self.character_id is None:
            if self._current_identity is not None and identity != self._current_identity:
                self._discard_pending_commands('character_changed')
            if self.character_id is not None and self._current_identity is not None:
                previous_key = (self._current_identity.get('server','').lower(), self._current_identity.get('name','').lower())
                next_key = (identity.get('server','').lower(), identity.get('name','').lower())
                if previous_key != next_key:
                    client.send_json({'type':'character.left','protocol_version':PROTOCOL_VERSION,'character_id':self.character_id,'sent_at':_utc_now()})
                    self.character_id = None
            self._profile_epoch += 1
            client.send_json({'type':'character.identify','protocol_version':PROTOCOL_VERSION,'server':identity['server'],'name':identity['name'],'guild':identity.get('guild',''),'sent_at':_utc_now()})
            reply = self._wait_for_registration(client)
            if not isinstance(reply,dict) or reply.get('type')!='character.registered' or reply.get('protocol_version')!=PROTOCOL_VERSION or not _validate_agent_id(reply.get('character_id')) or not _validate_agent_id(reply.get('session_id')):
                raise WebSocketClosed('character registration rejected')
            self.character_id = reply['character_id']
            self.session_id = reply['session_id']
            self._current_identity = identity
            snapshot = True
        client.send_json({'type':'character.snapshot' if snapshot else 'character.state','protocol_version':PROTOCOL_VERSION,'character_id':self.character_id,'session_id':self.session_id,'state':state,'sent_at':_utc_now()})

    def _handle_character_rejected(self, message):
        if message.get('protocol_version') != PROTOCOL_VERSION or message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
            raise WebSocketClosed('invalid character rejection')
        self._discard_pending_commands('session_superseded')
        self._rejected_identity = self._identity_key(self._current_identity)
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
        elif message.get('type') == 'command.execute': self._accept_command(message)
        elif message.get('type') == 'command.revoke': self._revoke_session(message)
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
            'character.return': ('use_return_scroll', 'unsupported_runtime_primitive'),
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
            commands.append({'name': name, 'supported': supported, 'reason': '' if supported else reason})
            if extra: commands[-1].update(extra)
        return {'type': 'agent.capabilities', 'protocol_version': 3, 'schema_version': 1, 'commands': commands}

    def _accept_command(self, message):
        command_id = message.get('command_id')
        if (message.get('protocol_version') != 3 or not isinstance(command_id, str) or
                len(command_id) != 40 or not command_id.startswith('cmd_') or
                not _validate_agent_id(message.get('character_id')) or not _validate_agent_id(message.get('session_id')) or
                not isinstance(message.get('name'), str) or not isinstance(message.get('args'), dict)):
            raise WebSocketClosed('invalid or stale command target')
        if message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
            self._queue_result(self._base_result(message,'failed','session_not_active','unverified'))
            return
        if command_id in self._dedup:
            return
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
        self._queue_result({'type':'command.ack','protocol_version':3,'command_id':command_id,'character_id':self.character_id,'session_id':self.session_id})

    def _remember_command(self, command_id):
        self._dedup.add(command_id); self._dedup_order.append(command_id)
        while len(self._dedup_order) > 256:
            old = self._dedup_order.pop(0); self._dedup.discard(old)

    def _queue_result(self, frame):
        try: self._outgoing.put_nowait(frame)
        except _queue.Full: self.status = 'Command result queue is full; backend may show an unknown outcome.'

    def _flush_results(self, client):
        while True:
            try: frame = self._outgoing.get_nowait()
            except _queue.Empty: return
            client.send_json(frame)

    def _base_result(self, message, status, code, verification):
        return {'type':'command.result','protocol_version':3,'command_id':message.get('command_id'),
                'character_id':message.get('character_id'),'session_id':message.get('session_id'),
                'status':status,'reason':code,'verification':verification}

    def _revoke_session(self, message):
        if message.get('protocol_version') != 3 or message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
            return
        self._discard_pending_commands('session_superseded')

    def _clear_pending_commands(self):
        while True:
            try: self._commands.get_nowait()
            except _queue.Empty: break

    def _discard_pending_commands(self, reason):
        while True:
            try: item=self._commands.get_nowait()
            except _queue.Empty: break
            self._queue_result(self._base_result(item['message'],'failed',reason,'unverified'))

    def process_one_command(self, current_identity=None, current_region=None):
        """Invoke at most one validated command from phBot's event_loop callback."""
        try: item = self._commands.get_nowait()
        except _queue.Empty: return False
        message = item['message']; name = message['name']; args = message['args']
        if item['epoch'] != self._profile_epoch or message.get('character_id') != self.character_id or message.get('session_id') != self.session_id:
            self._queue_result(self._base_result(message, 'failed', 'stale_session', 'unverified')); return True
        if _monotonic() >= item['deadline']:
            self._queue_result(self._base_result(message, 'failed', 'command_expired', 'unverified')); return True
        expected_identity = self._current_identity
        if current_identity is not None and self._identity_key(current_identity) != self._identity_key(expected_identity):
            self._queue_result(self._base_result(message, 'failed', 'character_changed', 'unverified')); return True
        try:
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
            self._queue_control_state(message)
        except Exception as error:
            self._queue_result(self._base_result(message, 'failed', str(error)[:64] or 'api_error', 'unverified'))
        return True

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
        empty = ('bot.start','bot.stop','trace.stop','character.return','character.disconnect')
        if name in empty:
            exact(())
            function = {'bot.start':'start_bot','bot.stop':'stop_bot','trace.stop':'stop_trace','character.return':'use_return_scroll','character.disconnect':'disconnect'}[name]
            result = self.api.call(function)
            return result, {}, None, 'api_confirmed' if isinstance(result,bool) else 'unverified'
        if name == 'trace.start':
            exact(('name',)); value=args.get('name')
            if not isinstance(value,str) or not value.strip() or len(value.strip().encode('utf-8'))>64: raise ValueError('invalid_arguments')
            result=self.api.call('start_trace',value.strip()); return result,{'name':value.strip()},None,'api_confirmed' if isinstance(result,bool) else 'unverified'
        if name == 'training.area.set':
            exact(('mode','name','region','x','y','z')); mode=args.get('mode')
            if mode == 'current_position':
                position=self.api.position()
                if not isinstance(position,dict) or not all(_number(position.get(k)) and abs(position.get(k))<=10000000 for k in ('x','y','z')) or not isinstance(position.get('region'),int) or isinstance(position.get('region'),bool) or position.get('region')<=0: raise ValueError('position_unavailable')
                region=int(position['region']); coords=[float(position[k]) for k in ('x','y','z')]
            elif mode == 'position':
                if not isinstance(args.get('region'),int) or isinstance(args.get('region'),bool) or args.get('region')<=0 or current_region!=args.get('region') or not all(_number(args.get(k)) and abs(args[k])<=10000000 for k in ('x','y','z')): raise ValueError('invalid_arguments')
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
                if isinstance(value,int) and not isinstance(value,bool): safe['training_region']=value
            elif _number(value): safe['training_'+key]=float(value)
        return safe

    def _queue_control_state(self, message):
        area=None
        try:
            if self.api.has('get_training_area'): area=self.api.call('get_training_area')
        except Exception: area=None
        safe=self._safe_area(area) or {}
        state={'training_available':bool(isinstance(area,dict)),'observed_at':_utc_now()}
        state.update(safe)
        self._queue_result({'type':'character.control_state','protocol_version':3,'character_id':message.get('character_id'),
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
_gui_status = None
_last_character_signature = None
_last_character_sample_at = 0.0
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


def _stop_worker():
    global _worker
    if _worker is not None:
        _worker.stop()
        _worker = None


def _start_worker(config):
    global _worker
    _stop_worker()
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
    global _character_joined
    _character_joined = False
    _load_active_profile()


def disconnected():
    global _last_character_signature, _character_joined
    _last_character_signature = None
    _character_joined = False
    if _worker is not None:
        _worker.leave_character()
    _set_gui_status('SRO client disconnected. Waiting for login...')


def joined_game():
    # This callback runs after the player selects a character.
    global _last_character_signature, _character_joined
    _last_character_signature = None
    _character_joined = True
    _load_active_profile()


def event_loop():
    # phBot calls this every 500 ms. Keep UI updates and profile detection here;
    # the worker owns backend I/O and only publishes its latest status string.
    try:
        _load_active_profile()
    except Exception as error:
        _log('profile sync failed (' + error.__class__.__name__ + ')')
    if _worker is not None:
        _set_gui_status(_worker.status)
        _sample_character()


def _sample_character():
    global _last_character_signature, _last_character_sample_at, _character_joined
    if not _PHBOT_AVAILABLE or _worker is None or _character_joined is False:
        return
    try:
        data = _get_character_data()
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
        if isinstance(value, (int,float)) and not isinstance(value,bool) and value >= 0:
            state[source] = int(value)
    try:
        position = _get_position()
    except Exception:
        position = None
    if isinstance(position, dict):
        for axis in ('x','y','z'):
            value = position.get(axis)
            if isinstance(value,(int,float)) and not isinstance(value,bool):
                state[axis] = float(value)
        region = position.get('region')
        if isinstance(region,int) and not isinstance(region,bool) and region >= 0:
            state['region'] = region
    if isinstance(state.get('region'),int):
        try:
            zone = _get_zone_name(state['region'])
            if isinstance(zone,str) and zone.strip():
                state['zone'] = zone.strip()[:100]
        except Exception:
            pass
    # Official Botting docs expose start/stop mutations but no state getter.
    state['botting'] = None
    identity = {
        'server':str(data['server']).strip()[:100],
        'name':str(data['name']).strip()[:64],
        'guild':(
            str(data['guild']).strip()[:100]
            if isinstance(data.get('guild'), str)
            else None
        ),
    }
    if not identity['server'] or not identity['name']:
        return
    signature = json.dumps([identity,state],sort_keys=True,separators=(',',':'))
    now = _monotonic()
    if signature != _last_character_signature or now-_last_character_sample_at >= 5.0:
        _worker.update_character(identity,state)
        _last_character_signature = signature
        _last_character_sample_at = now
    if hasattr(_worker, 'report_control_state'):
        try: _worker.report_control_state()
        except Exception as error: _log('control-state sample failed (' + error.__class__.__name__ + ')')
    # Mutations run only on phBot's event_loop callback, after refreshing identity
    # and region. The network worker only validates/enqueues command frames.
    if hasattr(_worker, 'process_one_command'):
        try: _worker.process_one_command(identity, state.get('region'))
        except Exception as error: _log('command callback failed (' + error.__class__.__name__ + ')')
    if hasattr(_worker, 'process_walk_step'):
        try: _worker.process_walk_step(identity, state.get('region'))
        except Exception as error: _log('walk callback failed (' + error.__class__.__name__ + ')')


def finished():
    _stop_worker()


if _PHBOT_AVAILABLE and _QtBind is not None:
    _gui = _QtBind.init(__name__, pName)
    _QtBind.createLabel(_gui, 'Backend WebSocket URL', 10, 10)
    _gui_backend_url = _QtBind.createLineEdit(_gui, '', 10, 30, 360, 20)
    _QtBind.createLabel(_gui, 'Agent ID', 10, 60)
    _gui_agent_id = _QtBind.createLineEdit(_gui, '', 10, 80, 360, 20)
    _QtBind.createLabel(_gui, 'Agent token (paste to set or replace)', 10, 110)
    _gui_agent_token = _QtBind.createLineEdit(_gui, '', 10, 130, 360, 20)
    _QtBind.createButton(_gui, 'save_config', 'Save & Connect', 10, 165)
    _gui_status = _QtBind.createLabel(
        _gui,
        'Join the game to select a bot profile.',
        10,
        200,
    )
    try:
        _load_active_profile(force=True)
    except Exception as error:
        _log('profile sync failed (' + error.__class__.__name__ + ')')
elif _PHBOT_AVAILABLE:
    _log('QtBind GUI API is unavailable; PhMon cannot be configured')
