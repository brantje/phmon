import assert from 'node:assert/strict'
import test from 'node:test'
import {
  chartPoints,
  formatModel,
  jobLabel,
  jobbingLabel,
  registryPlayerName,
  safePlayersReturn,
  type LevelSnapshot,
} from '../app/utils/playerRegistry.ts'

test('player and job names stay independent', () => {
  assert.equal(
    registryPlayerName({
      observed_name: 'DarkWizard',
      player_name: 'DarkWizard',
      job_name: null,
    }),
    'DarkWizard',
  )
  assert.equal(
    registryPlayerName({
      observed_name: 'SecretHunter',
      player_name: null,
      job_name: 'SecretHunter',
    }),
    '',
  )
  assert.equal(
    registryPlayerName({
      observed_name: 'UnknownWarrior',
      player_name: null,
      job_name: null,
    }),
    'UnknownWarrior',
  )
})

test('job role and jobbing state use separate labels', () => {
  assert.equal(jobLabel('hunter'), 'Hunter')
  assert.equal(jobLabel('none'), 'None')
  assert.equal(jobLabel(null), 'Unknown')
  assert.equal(jobbingLabel(false), 'No')
  assert.equal(jobbingLabel(true), 'Yes')
  assert.equal(jobbingLabel(null), 'Unknown')
})

test('model fallback keeps the numeric id', () => {
  assert.equal(
    formatModel('CHAR_CH_MAN_ADVENTURER', 1907),
    'CHAR_CH_MAN_ADVENTURER (1907)',
  )
  assert.equal(formatModel('', 1907), 'Unknown (1907)')
  assert.equal(formatModel('', null), 'Unknown')
})

test('progression points do not invent missing levels', () => {
  const snapshots = [
    snapshot(103, '2026-10-06T00:00:00Z'),
    snapshot(100, '2026-10-01T00:00:00Z'),
    snapshot(106, '2026-10-09T00:00:00Z'),
  ]
  const points = chartPoints(snapshots)
  assert.deepEqual(
    points.map((point) => point.level),
    [100, 103, 106],
  )
})

test('player profile return path stays on the registry list', () => {
  assert.equal(
    safePlayersReturn('/players?server=Greatest&job=hunter'),
    '/players?server=Greatest&job=hunter',
  )
  assert.equal(safePlayersReturn('/players/abc'), '/players')
  assert.equal(safePlayersReturn('https://example.test/players'), '/players')
})

function snapshot(level: number, at: string): LevelSnapshot {
  return {
    id: String(level),
    level,
    first_seen_at: at,
    last_seen_at: at,
    observed_name: 'DarkWizard',
    source: 'map.players',
  }
}
