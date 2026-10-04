import assert from 'node:assert/strict'
import test from 'node:test'
import {
  collectMapActionNotifications,
  mapActionNotification,
} from '../app/utils/mapActionNotifications.ts'
import type {
  FanOutChild,
  FanOutCommandName,
  FanOutOperation,
} from '../app/utils/commandFanOut.ts'

function operation(
  name: FanOutCommandName,
  children: Partial<FanOutChild>[] = [{}],
): FanOutOperation {
  return {
    operationID: 'action-1',
    command: { name, label: name, impact: 'routine', buildArgs: () => ({}) },
    scopeKey: 'fixture',
    selectedCount: children.length,
    state: 'settled',
    children: children.map((child, i) => ({
      characterID: String(i),
      characterName: `Fixture${i + 1}`,
      server: 'fixture',
      sessionID: 's',
      scopeKey: 'fixture',
      submission: 'accepted',
      executionState: 'completed',
      apiReturn: true,
      ...child,
    })),
  }
}

test('map notifications report each demonstrated action and pluralize actual successes', () => {
  const examples: [FanOutCommandName, string][] = [
    ['bot.start', 'Bot started for 4 characters'],
    ['bot.stop', 'Bot stopped for 4 characters'],
    ['character.return', 'Return scroll used by 4 characters'],
    ['character.navigate', 'Navigation sent to 4 characters'],
    [
      'character.disconnect',
      'Disconnected Fixture1, Fixture2, Fixture3, Fixture4',
    ],
  ]
  for (const [name, message] of examples) {
    assert.deepEqual(mapActionNotification(operation(name, [{}, {}, {}, {}])), {
      message,
      tone: 'success',
    })
  }
  for (const count of [1, 3]) {
    assert.equal(
      mapActionNotification(
        operation(
          'trace.start',
          Array.from({ length: count }, () => ({ args: { name: 'Kyrra' } })),
        ),
      )?.message,
      `${count} character${count === 1 ? '' : 's'} tracing Kyrra`,
    )
  }
  assert.equal(
    mapActionNotification(operation('trace.stop'))?.message,
    'Trace stopped for 1 character',
  )
})

test('pending, prepared and cancelled commands never announce completion', () => {
  for (const state of ['prepared', 'cancelled'] as const) {
    assert.equal(
      mapActionNotification({ ...operation('bot.start'), state }),
      null,
    )
  }
  for (const child of [
    { submission: 'ready' as const },
    { submission: 'submitting' as const },
    { executionState: 'queued' as const },
    { executionState: 'sent' as const },
  ]) {
    assert.equal(mapActionNotification(operation('bot.start', [child])), null)
  }
})

test('recall designation reports a sent request without claiming the point was saved', () => {
  assert.deepEqual(
    mapActionNotification(operation('character.recall_point.designate')),
    {
      message: 'Recall-point request sent to 1 character; save unverified',
      tone: 'warning',
    },
  )
})

test('partial results count only completed commands and expose failure, skips and uncertainty', () => {
  const result = mapActionNotification(
    operation('bot.start', [
      {},
      { submission: 'skipped' },
      { submission: 'rejected' },
      { executionState: 'expired' },
      { executionState: 'failed' },
      { apiReturn: false },
      { submission: 'uncertain' },
    ]),
  )
  assert.deepEqual(result, {
    message:
      'Bot started for 1 character · 4 failed · 1 skipped · 1 outcome unknown',
    tone: 'warning',
  })
  assert.equal(
    mapActionNotification(operation('bot.start', [{ apiReturn: false }]))
      ?.message,
    'bot.start · 1 failed',
  )
  assert.equal(
    mapActionNotification(
      operation('character.disconnect', [{ apiReturn: null }]),
    )?.message,
    'Disconnected Fixture1',
  )
})

test('uncertain admissions remain visible until their actual outcome arrives', () => {
  assert.deepEqual(
    mapActionNotification(
      operation('bot.start', [
        { submission: 'uncertain' },
        { executionState: 'queued' },
      ]),
    ),
    { message: 'bot.start · 1 outcome unknown · 1 pending', tone: 'warning' },
  )
})

test('live replacement snapshots never replay the same outcome; later resolution does notify', () => {
  const announced = new Map<string, string>()
  const uncertain = operation('bot.start', [{ submission: 'uncertain' }])
  assert.equal(collectMapActionNotifications([uncertain], announced).length, 1)
  for (let i = 0; i < 10; i++) {
    assert.equal(
      collectMapActionNotifications(
        [
          {
            ...uncertain,
            command: { ...uncertain.command },
            children: uncertain.children.map((child) => ({ ...child })),
          },
        ],
        announced,
      ).length,
      0,
    )
  }
  assert.equal(
    collectMapActionNotifications([operation('bot.start')], announced).length,
    1,
  )
  assert.equal(
    collectMapActionNotifications([operation('bot.start')], announced).length,
    0,
  )
  collectMapActionNotifications([], announced)
  assert.equal(announced.size, 0)
})
