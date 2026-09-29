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

export function dedupeCurrentMonsters(
  snapshots: MapMonsterObservation[],
): CurrentMapMonster[] {
  const byID = new Map<string, CurrentMapMonster>()
  for (const observer of snapshots) {
    if (observer.status === 'unavailable') continue
    for (const monster of observer.monsters) {
      const key = `${observer.server.toLowerCase()}\u0000${monster.region}\u0000${monster.id}`
      const previous = byID.get(key)
      if (
        !previous ||
        Date.parse(observer.observed_at) >
          Date.parse(previous.observer.observed_at)
      )
        byID.set(key, { ...monster, observer })
    }
  }
  return [...byID.values()]
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
