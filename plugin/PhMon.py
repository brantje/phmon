# -*- coding: utf-8 -*-
from __future__ import print_function

import base64
import hashlib
import json
import os
import random
import select
import socket
import ssl
import struct
import threading
import time
try:
    from urllib.parse import urlparse
except ImportError:  # pragma: no cover - Python 2 is not supported, kept harmless for embedded variants.
    from urlparse import urlparse

pName = 'PhMon'
pVersion = '1.0.0'
pUrl = ''

PROTOCOL_VERSION = 1
DEFAULT_HEARTBEAT_INTERVAL = 10
DEFAULT_HEARTBEAT_TIMEOUT = 30
MAX_MESSAGE_BYTES = 8192
_WEBSOCKET_GUID = '258EAFA5-E914-47DA-95CA-C5AB0DC85B11'

try:
    from phBot import get_config_dir as _get_config_dir
    from phBot import get_config_path as _get_config_path
    from phBot import get_profile as _get_profile
    from phBot import get_version as _get_phbot_version
    from phBot import log as _phbot_log
    _PHBOT_AVAILABLE = True
except ImportError:
    _PHBOT_AVAILABLE = False
    _get_config_dir = None
    _get_config_path = None
    _get_profile = None
    _get_phbot_version = None
    _phbot_log = None

try:
    import QtBind as _QtBind
except ImportError:
    _QtBind = None


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
    def __init__(self, config, phbot_version, websocket_factory=None):
        self.config = validate_config(config)
        self.phbot_version = str(phbot_version)
        self.websocket_factory = websocket_factory or WebSocketClient
        self.stop_event = threading.Event()
        self._thread = None
        self._socket = None
        self._socket_lock = threading.Lock()

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

    def _set_socket(self, value):
        with self._socket_lock:
            self._socket = value

    def _run(self):
        backoff = ReconnectBackoff()
        while not self.stop_event.is_set():
            client = None
            try:
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
                backoff.reset()
                _log('connected to backend')
                next_heartbeat = _monotonic() + interval

                while not self.stop_event.is_set():
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
                        raise WebSocketClosed('unexpected server application message')
            except Exception as error:
                if not self.stop_event.is_set():
                    _log('backend unavailable (' + error.__class__.__name__ + '); reconnecting')
            finally:
                if client is not None:
                    client.close()
                self._set_socket(None)

            if not self.stop_event.is_set():
                self.stop_event.wait(backoff.next_delay())

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
        return interval


_worker = None
_active_settings_path = None
_profile_config = None
_gui = None
_gui_backend_url = None
_gui_agent_id = None
_gui_agent_token = None
_gui_status = None


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
    bot_config_path = _get_config_path()
    if not bot_config_path:
        return None
    bot_profile = _get_profile()
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
    _set_gui_status('Profile loaded. Token is stored but hidden.')
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
    _set_gui_status('Saved for this bot profile. Connecting...')
    _start_worker(config)


def joined_game():
    _load_active_profile(force=True)


def event_loop():
    # Profile changes can occur without reloading the plugin. The callback is cheap:
    # it only compares phBot's active config path unless the profile changed.
    try:
        _load_active_profile()
    except Exception as error:
        _log('profile sync failed (' + error.__class__.__name__ + ')')


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
