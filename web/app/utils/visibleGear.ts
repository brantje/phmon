export type VisibleGearEntry = {
  ref_item_id: number
  opt_level?: number | null
  slot?: number | null
  item?: Record<string, unknown>
}

export type PlayerEnrichment = {
  source: string
  model_ref_id?: number | null
  gear?: VisibleGearEntry[]
  avatar_gear?: VisibleGearEntry[]
  level?: number | null
  job?: number | null
  job_level?: number | null
  race?: string | null
  guild?: string | null
  position?: {
    region: number
    x: number
    y: number
    z: number
  } | null
  job_item_ids?: number[]
  identity_links?: Array<Record<string, unknown>>
  gear_observed_at?: string | null
  avatar_gear_observed_at?: string | null
  level_observed_at?: string | null
  position_observed_at?: string | null
  field_evidence?: Record<
    string,
    {
      observed_at: string
      profile: string
      source_opcode: number
      reason: string
      source_identity_kind?: string
      source_name?: string
      association?: {
        alias: string
        status: string
        reason: string
        observed_at: string
      } | null
    }
  >
}

export type PlayerDetailRecord = {
  enrichment?: PlayerEnrichment | null
} & Record<string, unknown>

export function visibleGearItemRecord(
  entry: VisibleGearEntry,
): Record<string, unknown> {
  const base =
    entry.item && typeof entry.item === 'object'
      ? { ...entry.item }
      : { model: entry.ref_item_id, visible_equipment_only: true }
  const item: Record<string, unknown> = {
    ...base,
    model: entry.ref_item_id,
    visible_equipment_only: true,
  }
  if (entry.opt_level != null) {
    item.plus = entry.opt_level
  }
  const metadata = item.metadata
  const hasName =
    metadata &&
    typeof metadata === 'object' &&
    typeof (metadata as Record<string, unknown>).name === 'string' &&
    String((metadata as Record<string, unknown>).name).trim() !== ''
  if (!hasName) {
    item.servername = `Item #${entry.ref_item_id}`
  }
  return item
}

export function visibleGearLabel(entry: VisibleGearEntry): string {
  if (entry.slot != null) {
    return `Visible item (game slot ${entry.slot})`
  }
  return 'Visible item'
}
