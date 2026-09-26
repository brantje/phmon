export interface AgentView {
  agent_id: string
  connected: boolean
  connected_at?: string
  first_seen_at?: string
  last_seen_at?: string
  last_connected_at?: string
  last_disconnected_at?: string
  protocol_version?: number
  plugin_version?: string
  phbot_version?: string
}

export interface AgentListResponse {
  status: 'ok' | 'unavailable'
  agents: AgentView[]
}
