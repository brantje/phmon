#!/usr/bin/env python3
"""Fixture-only carried/bank smoke against an isolated PhMon stack.

Uses the real plugin callback and agent worker with deterministic getter adapters.
Never run this against a production monitoring instance or a real phBot process.
"""
import json
import os
import signal
import tempfile
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path

from agent_simulator import PhMon, ROOT, wait_until
import live_smoke
from live_smoke import WebSocketClient, http_json, live_url, snapshot, subscribe
from smoke_auth import login_cookie


def main():
    web = os.environ.get('SMOKE_WEB_URL', 'http://127.0.0.1:3006').rstrip('/')
    live_smoke.WEB_URL = web
    live_smoke.OPERATOR_COOKIE = login_cookie(web, web, ROOT)
    status, credential = http_json('/api/agents/credentials', 'POST', {})
    assert status == 201, 'fixture credential creation failed'
    config = {'backend_url': os.environ.get('PHMON_AGENT_URL', 'ws://127.0.0.1:8082/agent'),
              'agent_id': credential['agent_id'], 'agent_token': credential['agent_token']}
    run_seconds = float(os.environ.get('STORAGE_GOLD_RUN_SECONDS', '80'))
    identity = {'server': 'Fixture Bank', 'name': 'BankFixture', 'guild': 'Fixture Bank Guild'}
    balances = {'gold': 1000, 'bank': 4000}
    getters = {
        'get_inventory': lambda: {'size': 13, 'items': [None]*13, 'gold': balances['gold']},
        'get_storage': lambda: {'size': 0, 'items': [], 'gold': balances['bank']},
        'get_guild_storage': lambda: {'size': 0, 'items': [], 'gold': 2000},
    }
    worker = PhMon.AgentWorker(config, 'storage-gold-fixture', api_adapter=PhMon.PhBotAdapter({}))
    worker._phbot_version = 'fixture'
    PhMon._worker = worker
    PhMon._PHBOT_AVAILABLE = True
    PhMon._character_joined = True
    PhMon._get_character_data = lambda: dict(identity, gold=balances['gold'], level=80, sp=100)
    PhMon._get_position = lambda: None
    PhMon._optional_phbot_api = lambda name: getters.get(name)
    PhMon._last_character_signature = None
    PhMon._last_resources_sample_at = 0
    stopped = [False]
    signal.signal(signal.SIGTERM, lambda *_: stopped.__setitem__(0, True))
    client = None
    with tempfile.TemporaryDirectory(prefix='phmon-bank-fixture-') as directory:
        config['death_spool_path'] = os.path.join(directory, 'events.json')
        worker.start()
        try:
            wait_until(lambda: 'Connected' in worker.status, 20, 'fixture connection')
            PhMon._sample_character()
            wait_until(lambda: worker.character_id and worker.session_id, 20, 'fixture character')
            start = time.monotonic()
            # Keep callbacks active while resource baseline/ack catches up.
            while time.monotonic()-start < 4:
                PhMon._sample_character()
                time.sleep(.5)
            status, view = http_json('/api/characters/'+worker.character_id, 'GET')
            assert status == 200 and view['gold'] == 1000 and view['storage_gold'] == 4000, view
            status, group = http_json('/api/groups', 'POST', {'name': 'Fixture bank group ' + credential['agent_id'][:8]})
            assert status == 201, group
            status, _ = http_json('/api/groups/'+group['group_id']+'/members/'+worker.character_id, 'PUT')
            assert status == 204, status
            ready = {'character_id': worker.character_id, 'group_id': group['group_id'], 'server': identity['server']}
            if os.environ.get('STORAGE_GOLD_READY_FILE'):
                Path(os.environ['STORAGE_GOLD_READY_FILE']).write_text(json.dumps(ready))
            print('READY fixture bank display', json.dumps(ready), flush=True)
            while not stopped[0] and time.monotonic()-start < run_seconds:
                if time.monotonic()-start >= 30:
                    balances.update(gold=500, bank=4500)
                PhMon._sample_character()
                time.sleep(.5)
            if stopped[0]:
                return
            status, view = http_json('/api/characters/'+worker.character_id, 'GET')
            assert status == 200 and view['gold'] == 500 and view['storage_gold'] == 4500, view
            client = WebSocketClient(live_url(), web, live_smoke.OPERATOR_COOKIE).connect()
            now = datetime.now(timezone.utc)
            filter_ = {'view': 'performance', 'server': identity['server'], 'character_id': worker.character_id,
                       'from': (now-timedelta(minutes=5)).isoformat(), 'to': (now+timedelta(seconds=5)).isoformat(), 'timezone': 'UTC'}
            subscribe(client, 'bank-rates', 1, 'analytics', filter_)
            data = snapshot(client, 'bank-rates', 1)
            rate = data['performance']['rates']['gold']
            assert rate['has_rate'] and rate['delta_exact'] == '0' and rate['per_hour'] == 0, rate
            filter_.update(view='economy', balance_scope='characters')
            subscribe(client, 'bank-balances', 1, 'analytics', filter_)
            data = snapshot(client, 'bank-balances', 1)
            assert data['breakdown'][0]['value'] == '5000', data
            print('PASS fixture plugin → Go → PostgreSQL → live combined balances and zero transfer rate', flush=True)
        finally:
            if client:
                client.close()
            worker.stop()
            worker.join(3)


if __name__ == '__main__':
    main()
