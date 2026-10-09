import type { TradeReport } from '~~/shared/types/live'

export function tradeOutcomeLabel(report: TradeReport) {
  return `${report.outcome} · ${report.reason}`
}

export function tradeRouteLabel(report: TradeReport) {
  return `${report.route.from} → ${report.route.to}`
}

export function formatTradeGold(value: number) {
  const sign = value < 0 ? '-' : ''
  const digits = Math.abs(value)
    .toString()
    .replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return `${sign}${digits} gold`
}

export function formatTradeDuration(seconds: number) {
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  if (minutes === 0) return `${rest}s`
  return `${minutes}m ${rest}s`
}

export function tradeReportSummary(report: TradeReport) {
  const parts: string[] = []
  if (report.goods && report.goods.length > 0) {
    parts.push(
      report.goods.map((good) => `${good.name} × ${good.quantity}`).join(', '),
    )
  }
  if (typeof report.gold === 'number') parts.push(formatTradeGold(report.gold))
  if (typeof report.duration_s === 'number') {
    parts.push(formatTradeDuration(report.duration_s))
  }
  if (report.stars) {
    parts.push(report.stars === 'Max' ? 'Max stars' : `${report.stars} stars`)
  }
  const count = report.waypoints?.length ?? 0
  parts.push(`${count} waypoint${count === 1 ? '' : 's'}`)
  if (report.transport) parts.push(report.transport)
  if (report.thief?.name) parts.push(report.thief.name)
  if (report.detail) parts.push(report.detail)
  return parts.join(' · ')
}
