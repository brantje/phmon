import type {
  MapMonster,
  MapMonsterObservation,
} from '../../shared/types/live.ts'

export const DEFAULT_SHOW_NEARBY_MONSTER_NAMES = false

const MONSTER_TYPES: Record<
  number,
  {
    label: string
    scale: number
    party: boolean
    iconUrl: string
    partyBadgeUrl: string
  }
> = {
  0: {
    label: 'General',
    scale: 1,
    party: false,
    iconUrl: '/game-assets/monster-types/0_general.png',
    partyBadgeUrl: '',
  },
  1: {
    label: 'Champion',
    scale: 1.2,
    party: false,
    iconUrl: '/game-assets/monster-types/1_champion.png',
    partyBadgeUrl: '',
  },
  4: {
    label: 'Giant',
    scale: 1.5,
    party: false,
    iconUrl: '/game-assets/monster-types/4_giant.png',
    partyBadgeUrl: '',
  },
  16: {
    label: 'General (Party)',
    scale: 1,
    party: true,
    iconUrl: '/game-assets/monster-types/0_general.png',
    partyBadgeUrl: '/game-assets/monster-types/16_party_general.png',
  },
  17: {
    label: 'Champion (Party)',
    scale: 1.2,
    party: true,
    iconUrl: '/game-assets/monster-types/1_champion.png',
    partyBadgeUrl: '/game-assets/monster-types/17_party_champion.png',
  },
  20: {
    label: 'Giant (Party)',
    scale: 1.5,
    party: true,
    iconUrl: '/game-assets/monster-types/4_giant.png',
    partyBadgeUrl: '/game-assets/monster-types/20_party_giant.png',
  },
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
    iconUrl: known?.iconUrl || '',
    partyBadgeUrl: known?.partyBadgeUrl || '',
    unknown: !known,
  }
}

export function monsterHPFraction(monster: Pick<MapMonster, 'hp' | 'max_hp'>) {
  if (
    monster.hp == null ||
    !Number.isFinite(monster.hp) ||
    (monster.max_hp != null && !Number.isFinite(monster.max_hp))
  )
    return null
  const hp = Math.max(0, monster.hp)
  const maxHP = Math.max(0, monster.max_hp ?? 0)
  return maxHP > 0 ? Math.min(1, hp / maxHP) : hp > 0 ? 1 : 0
}

export function monsterHPBarFraction(
  monster: Pick<MapMonster, 'hp' | 'max_hp'>,
) {
  const fraction = monsterHPFraction(monster)
  return fraction == null || (monster.max_hp ?? 0) > 0 ? fraction : 0
}

export interface CurrentMapMonster extends MapMonster {
  observer: MapMonsterObservation
}

export function monsterMapKey(monster: MapMonster, server: string): string {
  const scope = server.trim().toLocaleLowerCase()
  const id = monster.id.trim()
  const normalizedID = /^\d+$/.test(id) ? id.replace(/^0+/, '') : id
  if (normalizedID) return JSON.stringify([scope, 'id', normalizedID])
  // The reference uses a precise position fallback only when no ID is available.
  return JSON.stringify([
    scope,
    'position',
    (monster.servername || monster.name || '').trim().toLocaleLowerCase(),
    monster.model_id ?? 0,
    monster.region,
    Math.round(monster.x * 2),
    Math.round(monster.y * 2),
  ])
}

export function dedupeCurrentMonsters(
  snapshots: MapMonsterObservation[],
): CurrentMapMonster[] {
  const monsters = new Map<string, CurrentMapMonster>()
  for (const observer of snapshots) {
    if (observer.status === 'unavailable') continue
    for (const monster of observer.monsters) {
      const key = monsterMapKey(monster, observer.server)
      const current = { ...monster, observer }
      const previous = monsters.get(key)
      if (
        !previous ||
        Date.parse(observer.observed_at) >
          Date.parse(previous.observer.observed_at) ||
        (Date.parse(observer.observed_at) ===
          Date.parse(previous.observer.observed_at) &&
          observer.session_id < previous.observer.session_id)
      )
        monsters.set(key, current)
    }
  }
  return [...monsters.values()]
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

export function monsterMapName(monster: MapMonster, visible: boolean): string {
  return visible ? monsterDisplayName(monster) : ''
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
