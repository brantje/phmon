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
  session_id?: string
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
  dead?: boolean | null
}

export interface ActivityEvent {
  event_id: string
  schema_version: number
  kind: string
  category: string
  agent_id: string
  character_id: string
  session_id: string
  server: string
  character: string
  occurred_at: string
  received_at: string
  source: string
  source_ref: string
  sequence?: number
  dedupe_key?: string
  item_model?: number
  item_code?: string
  region?: number
  x?: number
  y?: number
  z?: number
  payload: Record<string, unknown>
}

export interface EventPage {
  events: ActivityEvent[]
  total: number
  next_cursor?: string
  alchemy_summary?: AlchemySummary
}

export interface ChatMessage {
  message_id: string
  event_id?: string
  command_id?: string
  echo_event_id?: string
  character_id: string
  session_id?: string
  server: string
  character: string
  channel: string
  direction: 'inbound' | 'outbound'
  raw_type?: string
  sender?: string
  peer_name?: string
  peer_key?: string
  message: string
  state: string
  occurred_at: string
}

export interface ChatContact {
  server: string
  character_id: string
  character: string
  peer_name: string
  peer_key: string
  last_message_id: string
  last_message: string
  last_message_at: string
  last_direction: 'inbound' | 'outbound'
  unread: number
}

export interface ChatPage {
  messages: ChatMessage[]
  older_cursor?: string
  newer_cursor?: string
  has_older: boolean
}

export interface ChatSnapshot {
  channel: string
  contacts: ChatContact[]
  page: ChatPage
  unread_by_channel: Record<string, number>
}

export interface AlchemySummary {
  attempts: number
  successes: number
  failures: number
  highest_plus?: number
}

export interface CharacterGroup {
  group_id: string
  name: string
  members: CharacterView[]
}

export type LiveStream =
  | 'agents'
  | 'characters'
  | 'character'
  | 'groups'
  | 'commands'
  | 'controls'
  | 'resources'
  | 'events'
  | 'chat'

export interface LiveFilter {
  q?: string
  group_id?: string
  character_id?: string
  command_name?: string
  command_state?: string
  limit?: number
  resource_keys?: string[]
  server?: string
  kind?: string
  category?: string
  item?: string
  from?: string
  to?: string
  cursor?: string
  channel?: string
  peer?: string
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

export interface ResourceObservation {
  resource_key: string
  availability: 'observed' | 'unavailable' | 'not_observed'
  payload: Record<string, unknown>
  observed_at?: string
  checked_at?: string
}

export interface CharacterResourcesView {
  character_id: string
  revision: number
  resources: Record<string, ResourceObservation>
  updated_at?: string
}

export interface GroupsSnapshot {
  groups: CharacterGroup[]
}

export interface RemoteCommand {
  command_id: string
  character_id: string
  session_id: string
  name: string
  args: Record<string, unknown>
  state:
    | 'queued'
    | 'dispatching'
    | 'sent'
    | 'acknowledged'
    | 'completed'
    | 'failed'
    | 'expired'
    | 'unknown'
  created_at: string
  expires_at: string
  message?: string
  verification?: 'api_confirmed' | 'observed' | 'unverified'
  api_return?: unknown
  effective_args?: Record<string, unknown>
  observed_after?: Record<string, unknown>
}

export interface CommandsSnapshot {
  character_id: string
  commands: RemoteCommand[]
}
export interface ControlsSnapshot {
  character_id: string
  session_id: string
  capabilities: Record<
    string,
    { supported: boolean; reason?: string; modes?: string[] }
  >
  training?: {
    session_id: string
    training_available: boolean
    training_region?: number
    training_x?: number
    training_y?: number
    training_z?: number
    training_radius?: number
    observed_at?: string
  }
}

export type LiveConnectionState =
  'idle' | 'connecting' | 'syncing' | 'current' | 'stale' | 'reconnecting'
