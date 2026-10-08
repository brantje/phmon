export interface PlayerLocation {
  region: number
  x: number
  y: number
  z?: number
  zone?: string
  observed_at: string
  scope?: string
}
export interface PlayerEquipmentSlot {
  slot: string
  state: 'unknown' | 'empty' | 'occupied'
  model_id?: number
  plus?: number
  variance?: string
  durability?: number
  magic_options?: Record<string, string>
  item?: Record<string, unknown>
  observed_at: string
  field_times?: Record<string, string>
}
export interface PlayerEquipment {
  last_availability?: string
  last_attempt_at?: string
  availability: 'observed_complete' | 'observed_partial' | 'unavailable'
  source: string
  profile: string
  slots: PlayerEquipmentSlot[]
  observed_at: string
  identity_verified: boolean
  character_model_id?: number
}
export interface PlayerRecord {
  id: string
  server_key: string
  server: string
  name: string | null
  observed_name: string
  level: number | null
  guild_name: string | null
  job: string | null
  job_name: string | null
  gear: PlayerEquipment | null
  gear_hash: string | null
  identity_gear_hash: string | null
  location: PlayerLocation | null
  first_seen_at: string
  last_seen_at: string
  resolved: boolean
  revision: number
  source_ids: string[]
}
export interface PlayerPage {
  players: PlayerRecord[]
  total: number
  next_cursor?: string
  previous_cursor?: string
  servers: string[]
  ingestion: {
    pending: number
    overflow: number
    last_error?: string
    last_commit_at?: string
  }
}
export interface PlayerAlias {
  id: string
  player_id: string
  alias_name: string
  alias_type: string
  job_type: string | null
  match_method: string
  confirmation_status: string
  first_seen_at: string
  last_seen_at: string
}
export interface PlayerObservation {
  id: string
  player_id: string
  server: string
  observed_name: string
  name_type: string
  job_type?: string
  level?: number
  location?: PlayerLocation
  source: string
  source_ref: string
  observed_at: string
  last_seen_at: string
  observer_agent_id?: string
  observer_character_id?: string
  observer_session_id?: string
  runtime_entity_id?: string
  runtime_epoch: string
  equipment?: PlayerEquipment
  evidence?: Record<string, unknown>
}
export interface PlayerEquipmentHistory {
  id: string
  player_id: string
  observation_id: string
  last_evidence: PlayerObservation
  equipment: PlayerEquipment
  gear_hash: string
  identity_gear_hash: string | null
  first_seen_at: string
  last_seen_at: string
}
export interface PlayerLink {
  id: string
  server_key: string
  canonical_player_id: string
  linked_player_id: string
  decision: string
  method: string
  confidence_score: number | null
  evidence_json: Record<string, unknown>
  status: string
  created_at: string
  decided_at: string | null
  decided_by: string | null
  revision: number
}
export interface PlayerHistoryPage<T> {
  items: T[]
  next_cursor?: string
}
