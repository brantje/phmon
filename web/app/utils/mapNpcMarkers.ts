import type { MapNpc } from '../../shared/types/live.ts'
import type { MapProfile } from '../../shared/types/map.ts'
import { worldPositionToRaster, type RasterPosition } from './mapCoordinates.ts'

export const NPC_MARKER_ICON = '/game-assets/interface/minimap/mm_sign_npc.png'

export interface NpcMapMarker {
  id: string
  label: string
  kind: 'npc'
  position: RasterPosition
  npc: MapNpc
}

export function npcDisplayLabel(
  npc: Pick<MapNpc, 'name' | 'servername' | 'model_id'>,
): string {
  const name = npc.name?.trim()
  if (name) return name
  const serverName = npc.servername?.trim()
  if (serverName) return serverName
  if (npc.model_id != null) return `NPC ${npc.model_id}`
  return 'NPC'
}

export function npcRoleLabel(role: MapNpc['role'] | undefined): string {
  return role === 'teleporter' ? 'Teleporter' : 'NPC'
}

export function npcMapMarkers(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  npcs: MapNpc[],
  enabled = true,
): NpcMapMarker[] {
  if (!enabled) return []
  const seen = new Set<string>()
  const markers: NpcMapMarker[] = []
  for (const npc of npcs) {
    if (!npc.id || seen.has(npc.id)) continue
    const position = worldPositionToRaster(
      profile,
      areaID,
      floorID,
      npc.region,
      npc.x,
      npc.y,
      npc.observer_z,
    )
    if (!position) continue
    seen.add(npc.id)
    markers.push({
      id: npc.id,
      label: npcDisplayLabel(npc),
      kind: 'npc',
      position,
      npc,
    })
  }
  return markers
}
