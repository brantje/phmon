#!/usr/bin/env python3
"""Disposable fixture flow using the production plugin collector/worker.

Requires an explicitly configured local test instance. Creates only simulator
credentials and synthetic sightings; never imports phBot or invokes a bot command.
"""
import importlib.util
import json
import os
import sys
import time
from http.cookies import SimpleCookie
from urllib.parse import quote, urlparse
from urllib.request import build_opener

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(ROOT, 'scripts'))
from command_smoke import request_json

spec = importlib.util.spec_from_file_location('player_registry_fixture_plugin', os.path.join(ROOT, 'plugin', 'PhMon.py'))
plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plugin)


def wait(predicate, description, timeout=20):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = predicate()
        if result:
            return result
        time.sleep(.1)
    raise RuntimeError('timed out: ' + description)


def main():
    web = os.environ.get('SMOKE_WEB_URL', '').rstrip('/')
    agent = os.environ.get('PHMON_AGENT_URL', '')
    if urlparse(web).hostname not in ('127.0.0.1', 'localhost') or urlparse(agent).hostname not in ('127.0.0.1', 'localhost'):
        raise SystemExit('Set SMOKE_WEB_URL and PHMON_AGENT_URL to a disposable loopback test instance.')
    secret = os.environ['OPERATOR_ACCESS_SECRET']
    opener = build_opener()
    code, headers, _ = request_json(opener, web + '/api/auth/login', 'POST', {'secret': secret})
    if code != 200:
        raise RuntimeError('fixture operator login failed')
    cookies = SimpleCookie(); cookies.load(headers.get('Set-Cookie', ''))
    cookie = 'phmon_operator=' + cookies['phmon_operator'].value
    server = 'Fixture Player Registry ' + str(time.time_ns())
    workers = []
    credentials = []
    try:
        for index in range(2):
            code, _, credential = request_json(opener, web + '/api/agents/credentials', 'POST', {}, cookie)
            if code != 201:
                raise RuntimeError('fixture credential creation failed')
            credentials.append(credential)
            worker = plugin.AgentWorker({'backend_url': agent, 'agent_id': credential['agent_id'], 'agent_token': credential['agent_token']}, 'synthetic-player-registry-fixture')
            workers.append(worker); worker.start()
            identity = {'server': server, 'name': 'FixtureObserver' + str(index), 'guild': 'Fixture'}
            worker.update_character(identity, {'level': 110, 'region': 25000, 'x': 10., 'y': 20., 'z': 0., 'hp': 100, 'dead': False})
            wait(lambda: worker.session_id, 'fixture observer session')
            raw = {'7': {'name': 'FixtureNormal', 'guild': 'Fixture Guild', 'level': 110, 'region': 25000, 'x': 20., 'y': 30.}, '8': {'name': 'FixtureAlias', 'level': 110, 'region': 25000, 'x': 40., 'y': 50.}}
            status, rows, truncated = plugin.collect_player_observation({'get_players': lambda: raw})
            if status != 'observed' or truncated:
                raise RuntimeError('fixture collector failed')
            worker.update_map_players(identity, status, 25000, rows, observer_z=0.)
        def page(expected=2):
            code, _, data = request_json(opener, web + '/api/players?server=' + quote(server), cookie=cookie)
            return data if code == 200 and data.get('total') == expected else None
        result = wait(page, 'two durable records from two observers')
        if any(p['name'] is not None or p['resolved'] for p in result['players']):
            raise RuntimeError('unclassified fixture name became normal or resolved')
        def observers_committed():
            code, _, history = request_json(opener, web + '/api/players/' + result['players'][0]['id'] + '/observations', cookie=cookie)
            sessions = {row.get('observer_session_id') for row in history.get('items', [])} if code == 200 else set()
            return len(sessions - {None}) == 2
        wait(observers_committed, 'both observer attributions committed')
        old_session = workers[0].session_id
        for worker in workers:
            worker.stop(); worker.join(3)
        workers.clear()
        persisted = wait(page, 'registry after observers disconnect')
        player_id = persisted['players'][0]['id']
        code, _, detail = request_json(opener, web + '/api/players/' + player_id, cookie=cookie)
        if code != 200 or detail['id'] != player_id:
            raise RuntimeError('persistent profile unavailable')
        # Reconnect through the same production worker. A reused runtime ID with
        # a new alias must not rename the original durable player.
        credential = credentials[0]
        worker = plugin.AgentWorker({'backend_url': agent, 'agent_id': credential['agent_id'], 'agent_token': credential['agent_token']}, 'synthetic-player-registry-fixture')
        workers.append(worker); worker.start()
        identity = {'server': server, 'name': 'FixtureObserver0', 'guild': 'Fixture'}
        worker.update_character(identity, {'level': 110, 'region': 25000, 'x': 10., 'y': 20., 'z': 0., 'hp': 100, 'dead': False})
        wait(lambda: worker.session_id and worker.session_id != old_session, 'new observer session after reconnect')
        raw = {'7': {'name': 'FixtureReusedRuntime', 'level': 110, 'region': 25000, 'x': 20., 'y': 30.}}
        status, rows, _ = plugin.collect_player_observation({'get_players': lambda: raw})
        worker.update_map_players(identity, status, 25000, rows, observer_z=0.)
        wait(lambda: page(3), 'runtime ID reuse creates independent identity')
        code, _, original = request_json(opener, web + '/api/players/' + player_id, cookie=cookie)
        if code != 200 or original['observed_name'] == 'FixtureReusedRuntime':
            raise RuntimeError('runtime ID became a permanent identity key')
        print(json.dumps({'result': 'PASS', 'fixture_server': server, 'player_id': player_id, 'players': 3, 'observing_agents': 2, 'reconnect': True, 'runtime_id_reuse': True, 'equipment_runtime_verified': False}))
    finally:
        for worker in workers:
            worker.stop(); worker.join(3)
        for credential in credentials:
            request_json(opener, web + '/api/agents/' + credential['agent_id'], 'DELETE', cookie=cookie)


if __name__ == '__main__':
    main()
