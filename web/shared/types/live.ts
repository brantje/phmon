import type { AgentView } from './agent'

export const LIVE_PROTOCOL_VERSION = 1 as const

export interface CharacterView {
  character_id: string
  server: string
  name: string
  guild?: string
  zone?: string
  online: boolean
  agent_id?: string
  session_started_at?: string
  last_activity_at?: string
  state_updated_at?: string
  level?: number
  hp?: number
  hp_max?: number
  mp?: number
  mp_max?: number
  current_exp?: number
  max_exp?: number
  sp?: number
  gold?: number
  region?: number
  x?: number
  y?: number
  z?: number
  botting?: boolean | null
}

export interface CharacterGroup {
  group_id: string
  name: string
  members: CharacterView[]
}

export type LiveStream = 'agents' | 'characters' | 'character' | 'groups'

export interface LiveFilter {
  q?: string
  group_id?: string
  character_id?: string
}

export interface LiveClientFrame {
  type: 'subscribe' | 'unsubscribe' | 'refresh' | 'heartbeat'
  protocol_version: typeof LIVE_PROTOCOL_VERSION
  subscription_id?: string
  revision?: number
  stream?: LiveStream
  filter?: LiveFilter
}

export interface LiveServerFrame {
  type:
    | 'snapshot'
    | 'subscription.unavailable'
    | 'subscription.rejected'
    | 'heartbeat'
  protocol_version: number
  subscription_id?: string
  revision?: number
  stream?: LiveStream
  reason?: string
  sent_at?: string
  data?: unknown
}

export interface AgentsSnapshot {
  agents: AgentView[]
}

export interface CharactersSnapshot {
  characters: CharacterView[]
}

export interface CharacterSnapshot {
  character: CharacterView | null
}

export interface GroupsSnapshot {
  groups: CharacterGroup[]
}

export type LiveConnectionState =
  | 'idle'
  | 'connecting'
  | 'syncing'
  | 'current'
  | 'stale'
  | 'reconnecting'
