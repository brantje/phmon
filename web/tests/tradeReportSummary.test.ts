import assert from 'node:assert/strict'
import test from 'node:test'
import type { TradeReport } from '../shared/types/live.ts'
import {
  tradeOutcomeLabel,
  tradeReportSummary,
  tradeRouteLabel,
} from '../app/utils/tradeReportSummary.ts'

function report(overrides: Partial<TradeReport> = {}): TradeReport {
  return {
    trade_id: 'trade-1',
    server: 'Greatest',
    ref: 'aat-1728460800-3',
    outcome: 'success',
    reason: 'sold',
    route: { from: 'Jangan', to: 'Donwhang' },
    waypoints: [
      { name: 'chau_approach', x: 37643, y: 7342 },
      { name: 'chau_mid', x: 37000, y: 7000 },
      { name: 'doji_approach', x: 36000, y: 6800 },
    ],
    goods: [{ name: 'Silk', quantity: 120 }],
    gold: 45000,
    duration_s: 842,
    stars: 'Max',
    transport: 'Horse',
    reporter: { name: 'CharName', app: 'AdvancedAutoTrade', version: '1.0.0' },
    finished_at: '2026-10-09T07:03:00Z',
    received_at: '2026-10-09T07:03:01Z',
    ...overrides,
  }
}

test('trade rows label the outcome, route and compact summary', () => {
  const row = report()
  assert.equal(tradeOutcomeLabel(row), 'success · sold')
  assert.equal(tradeRouteLabel(row), 'Jangan → Donwhang')
  assert.equal(
    tradeReportSummary(row),
    'Silk × 120 · 45,000 gold · 14m 2s · Max stars · 3 waypoints · Horse',
  )
})

test('failed trades keep the thief and detail without plotting coordinates', () => {
  const row = report({
    outcome: 'failed',
    reason: 'thief',
    goods: [{ name: 'Silk', quantity: 40 }],
    gold: -1200,
    duration_s: 12,
    stars: '2',
    thief: { name: 'Bandit' },
    transport: undefined,
    waypoints: [],
  })
  assert.equal(tradeOutcomeLabel(row), 'failed · thief')
  assert.equal(
    tradeReportSummary(row),
    'Silk × 40 · -1,200 gold · 12s · 2 stars · 0 waypoints · Bandit',
  )
  assert.equal(tradeReportSummary(row).includes('37643'), false)
})

test('navigation detail is included and a single waypoint stays singular', () => {
  const row = report({
    outcome: 'failed',
    reason: 'navigation',
    goods: undefined,
    gold: undefined,
    duration_s: undefined,
    stars: undefined,
    transport: undefined,
    detail: 'path blocked',
    waypoints: [{ name: 'chau_approach', x: 1, y: 2 }],
  })
  assert.equal(tradeReportSummary(row), '1 waypoint · path blocked')
})
