export type PlayerJob = 'none' | 'trader' | 'thief' | 'hunter' | 'unknown'

export type PlayerRecord = {
  id: string
  server: string
  observed_name: string
  player_name?: string | null
  job_name?: string | null
  level?: number | null
  guild_name?: string | null
  job?: PlayerJob | null
  job_level?: number | null
  is_jobbing?: boolean | null
  model_id?: number | null
  model_name?: string
  last_region?: number | null
  last_zone?: string
  last_x?: number | null
  last_y?: number | null
  last_z?: number | null
  first_seen_at: string
  last_seen_at: string
  progression?: PlayerProgress | null
  display_identity?: DisplayIdentity | null
  display_fallback?: {
    level?: number | null
    job?: string | null
    job_level?: number | null
    guild?: string | null
    model_id?: number | null
    model_name?: string | null
  } | null
}

export type DisplayIdentity = {
  canonical_id: string
  player_name?: string | null
  job_alias?: string | null
  aliases: string[]
  related_record_ids: string[]
  source: string
  status: 'inferred' | 'conflict' | 'observed' | 'unknown'
  first_seen_at: string
  last_seen_at: string
}

export type PlayerProgress = {
  first_observed_level?: number | null
  latest_observed_level?: number | null
  highest_observed_level?: number | null
  distinct_levels_observed: number
  latest_level_change?: number | null
  latest_level_change_at?: string | null
  note: string
}

export type LevelSnapshot = {
  id: string
  level: number
  first_seen_at: string
  last_seen_at: string
  observed_name: string
  player_name?: string | null
  job_name?: string | null
  guild_name?: string | null
  job?: PlayerJob | null
  job_level?: number | null
  is_jobbing?: boolean | null
  model_id?: number | null
  model_name?: string
  region?: number | null
  zone?: string
  x?: number | null
  y?: number | null
  z?: number | null
  source: string
}

export function jobLabel(job?: string | null) {
  switch (job) {
    case 'none':
      return 'None'
    case 'trader':
      return 'Trader'
    case 'thief':
      return 'Thief'
    case 'hunter':
      return 'Hunter'
    case 'unknown':
      return 'Unknown'
    default:
      return 'Unknown'
  }
}

export function jobbingLabel(value?: boolean | null) {
  if (value === true) return 'Yes'
  if (value === false) return 'No'
  return 'Unknown'
}

type PlayerIdentityInput = {
  player_name?: string | null
  job_name?: string | null
  observed_name: string
  is_jobbing?: boolean | null
  display_identity?: DisplayIdentity | null
}

export function registryPlayerName(player: PlayerIdentityInput) {
  if (player.display_identity) return player.display_identity.player_name || ''
  if (player.player_name) return player.player_name
  if (!player.job_name && player.is_jobbing === false)
    return player.observed_name
  return ''
}

export function registryJobAlias(player: PlayerIdentityInput) {
  if (player.display_identity) return player.display_identity.job_alias || ''
  if (player.job_name) return player.job_name
  if (player.is_jobbing === true) return player.observed_name
  return ''
}

export function identityStatusLabel(player: PlayerIdentityInput) {
  if (player.display_identity?.status === 'inferred')
    return 'Inferred · ph-proxy'
  if (player.display_identity?.status === 'conflict')
    return 'Unresolved mapping'
  return ''
}

export function textOrUnknown(value?: string | number | null) {
  if (value === undefined || value === null || value === '') return 'Unknown'
  return String(value)
}

export function formatPlayerLocation(
  region?: number | null,
  zone?: string | null,
  x?: number | null,
  y?: number | null,
) {
  const place = zone?.trim() || (region != null ? `Region ${region}` : '')
  const coordinates =
    x != null && y != null ? `${trimCoordinate(x)}, ${trimCoordinate(y)}` : ''
  if (place && coordinates) return `${place} (${coordinates})`
  return place || coordinates || 'Unknown'
}

export function formatModel(name?: string | null, id?: number | null) {
  if (name) return id != null ? `${name} (${id})` : name
  if (id != null) return `Unknown (${id})`
  return 'Unknown'
}

export function chartPoints(snapshots: readonly LevelSnapshot[]) {
  return [...snapshots].sort((left, right) => {
    const time = left.first_seen_at.localeCompare(right.first_seen_at)
    return time || left.level - right.level
  })
}

export function safePlayersReturn(value: unknown) {
  if (typeof value !== 'string') return '/players'
  if (!value.startsWith('/players') || value.startsWith('/players/'))
    return '/players'
  if (
    value.includes('://') ||
    value.startsWith('//') ||
    value.includes('\\') ||
    value.includes('\n')
  ) {
    return '/players'
  }
  return value
}

function trimCoordinate(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(1)
}
