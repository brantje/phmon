import type {
  MapMonster,
  MapMonsterObservation,
} from '../../shared/types/live.ts'

const MONSTER_TYPES: Record<
  number,
  { label: string; scale: number; party: boolean }
> = {
  0: { label: 'General', scale: 1, party: false },
  1: { label: 'Champion', scale: 1.2, party: false },
  4: { label: 'Giant', scale: 1.5, party: false },
  16: { label: 'Party General', scale: 1, party: true },
  17: { label: 'Party Champion', scale: 1.2, party: true },
  20: { label: 'Party Giant', scale: 1.5, party: true },
}

export function monsterTypePresentation(
  monster: Pick<MapMonster, 'type' | 'type_code'>,
) {
  const code =
    monster.type_code ??
    (monster.type && /^\d{1,3}$/.test(monster.type)
      ? Number(monster.type)
      : undefined)
  const known = code == null ? undefined : MONSTER_TYPES[code]
  return {
    code,
    label:
      known?.label ||
      (code == null ? monster.type || 'Unknown' : `Unknown (${code})`),
    scale: known?.scale || 1,
    party: known?.party || false,
    unknown: !known,
  }
}

export function monsterHPFraction(monster: Pick<MapMonster, 'hp' | 'max_hp'>) {
  if (
    monster.hp == null ||
    monster.max_hp == null ||
    !Number.isFinite(monster.hp) ||
    !Number.isFinite(monster.max_hp) ||
    monster.max_hp <= 0
  )
    return null
  return Math.max(0, Math.min(1, monster.hp / monster.max_hp))
}

export interface CurrentMapMonster extends MapMonster {
  observer: MapMonsterObservation
}

const CROSS_OBSERVER_MATCH_DISTANCE = 8

function monsterIdentities(monster: MapMonster): string[] {
  const identities: string[] = []
  if (monster.model_id != null) identities.push(`model:${monster.model_id}`)
  if (monster.servername)
    identities.push(`server:${monster.servername.trim().toLocaleLowerCase()}`)
  if (monster.name && !/^\d+$/.test(monster.name))
    identities.push(`name:${monster.name.trim().toLocaleLowerCase()}`)
  return identities
}

function monsterTypeCode(monster: MapMonster): number | undefined {
  if (monster.type_code != null) return monster.type_code
  return monster.type && /^\d{1,3}$/.test(monster.type)
    ? Number(monster.type)
    : undefined
}

export function dedupeCurrentMonsters(
  snapshots: MapMonsterObservation[],
): CurrentMapMonster[] {
  const monsters: CurrentMapMonster[] = []
  for (const observer of snapshots) {
    if (observer.status === 'unavailable') continue
    for (const monster of observer.monsters) {
      const identities = monsterIdentities(monster)
      const matchIndex = identities.length
        ? monsters.findIndex((candidate) => {
            if (
              candidate.observer.server.toLocaleLowerCase() !==
                observer.server.toLocaleLowerCase() ||
              candidate.region !== monster.region ||
              (monsterTypeCode(candidate) != null &&
                monsterTypeCode(monster) != null &&
                monsterTypeCode(candidate) !== monsterTypeCode(monster)) ||
              !monsterIdentities(candidate).some((identity) =>
                identities.includes(identity),
              ) ||
              candidate.observer.session_id === observer.session_id
            )
              return false
            return (
              Math.hypot(candidate.x - monster.x, candidate.y - monster.y) <=
              CROSS_OBSERVER_MATCH_DISTANCE
            )
          })
        : -1
      const current = { ...monster, observer }
      if (matchIndex < 0) {
        monsters.push(current)
        continue
      }

      const previous = monsters[matchIndex]
      if (
        Date.parse(observer.observed_at) >
        Date.parse(previous.observer.observed_at)
      )
        monsters[matchIndex] = current
    }
  }
  return monsters
}

export function monsterDisplayName(monster: MapMonster): string {
  const name = monster.name?.trim()
  if (name && !/^\d+$/.test(name)) return name

  const servername = monster.servername?.trim()
  if (servername) {
    const readable = servername
      .replace(/^MOB_[A-Z]{2}_/i, '')
      .replace(/^MOB_/i, '')
      .replace(/_/g, ' ')
      .toLocaleLowerCase()
      .replace(/\b\w/g, (letter) => letter.toLocaleUpperCase())
    if (readable) return readable
  }
  return 'Unknown monster'
}

export function localMapAsset(
  url: string | undefined,
  kind: 'portrait' | 'item',
) {
  const prefix =
    kind === 'portrait'
      ? '/game-assets/interface/character/'
      : '/game-assets/icon/'
  return url?.startsWith(prefix) &&
    !url.includes('..') &&
    !/[\\?#]/.test(url) &&
    ![...url].some((character) => character.charCodeAt(0) < 32)
    ? url
    : ''
}
