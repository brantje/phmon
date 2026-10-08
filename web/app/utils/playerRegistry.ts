import type {
  PlayerEquipment,
  PlayerRecord,
} from '../../shared/types/players.ts'

export const playerJobs = [
  'trader',
  'thief',
  'hunter',
  'none',
  'unknown',
] as const
export const playerSlots = [
  'head',
  'chest',
  'shoulder',
  'hands',
  'legs',
  'feet',
  'weapon',
  'shield',
  'earring',
  'necklace',
  'ring_left',
  'ring_right',
  'job',
  'avatar_head',
  'avatar_body',
  'avatar_attachment',
  'avatar_flag',
] as const
export const playerLabel = (
  player: Pick<PlayerRecord, 'name' | 'observed_name'>,
) => player.name || player.observed_name || 'Unknown'
export const playerJobLabel = (job?: string | null) =>
  job && playerJobs.includes(job as (typeof playerJobs)[number])
    ? job[0]!.toUpperCase() + job.slice(1)
    : 'Unknown'
export const equipmentLabel = (equipment?: PlayerEquipment | null) =>
  (equipment?.last_availability || equipment?.availability) ===
  'observed_complete'
    ? 'Complete'
    : (equipment?.last_availability || equipment?.availability) ===
        'observed_partial'
      ? 'Partial'
      : 'Unavailable'
export const safePlayerReturn = (value: unknown): string =>
  typeof value === 'string' &&
  /^\/players(?:\?[^#]*)?$/.test(value) &&
  !value.includes('\\') &&
  !/[\r\n]/.test(value)
    ? value
    : '/players'
export function playerRelativeTime(value: string, now = Date.now()): string {
  const time = Date.parse(value)
  if (!Number.isFinite(time)) return 'Unknown'
  const seconds = Math.max(0, Math.floor((now - time) / 1000))
  if (seconds < 60) return `${seconds}s ago`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}

export function normalizePlayerQuery(
  query: Record<string, unknown>,
  fallbackServer = 'all',
): Record<string, string> {
  const out: Record<string, string> = {}
  const read = (key: string) =>
    typeof query[key] === 'string' ? String(query[key]).trim() : ''
  for (const [key, max] of [
    ['server', 100],
    ['q', 64],
    ['guild', 64],
  ] as const) {
    const value = read(key)
    if (value && value.length <= max && !value.includes('\0')) out[key] = value
  }
  if (!Object.hasOwn(query, 'server') && fallbackServer !== 'all')
    out.server = fallbackServer
  for (const key of ['min_level', 'max_level']) {
    const value = read(key)
    if (/^\d{1,3}$/.test(value) && Number(value) >= 1 && Number(value) <= 255)
      out[key] = String(Number(value))
  }
  if (
    out.min_level &&
    out.max_level &&
    Number(out.min_level) > Number(out.max_level)
  ) {
    delete out.min_level
    delete out.max_level
  }
  const choices: Record<string, readonly string[]> = {
    job: playerJobs,
    seen: ['1h', '24h', '7d', '30d', 'custom'],
    identity: ['resolved', 'unresolved'],
    equipment: ['complete', 'partial', 'unavailable'],
    sort: ['last_seen', 'name', 'level', 'guild', 'job'],
    direction: ['asc', 'desc'],
  }
  for (const [key, values] of Object.entries(choices)) {
    const value = read(key)
    if (values.includes(value)) out[key] = value
  }
  for (const key of ['from', 'to']) {
    const value = read(key)
    if (value && value.length <= 40 && Number.isFinite(Date.parse(value)))
      out[key] = value
  }
  if (out.from && out.to && Date.parse(out.from) > Date.parse(out.to)) {
    delete out.from
    delete out.to
  }
  const size = read('limit')
  if (/^\d{1,3}$/.test(size) && Number(size) >= 1 && Number(size) <= 100)
    out.limit = size
  const cursor = read('cursor')
  if (cursor.length <= 1024 && /^[A-Za-z0-9_-]+$/.test(cursor))
    out.cursor = cursor
  return out
}
