interface MasteryEntry {
  id: number
  level: number
  name: string
}

function normalizeMastery(value: unknown): MasteryEntry | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const raw = value as Record<string, unknown>
  const id = raw.id
  const level = raw.level
  const name = typeof raw.name === 'string' ? raw.name.trim() : ''
  if (
    typeof id !== 'number' ||
    !Number.isSafeInteger(id) ||
    id <= 0 ||
    typeof level !== 'number' ||
    !Number.isInteger(level) ||
    level <= 0 ||
    level > 255 ||
    !name
  )
    return null
  return { id, level, name: name.slice(0, 64) }
}

export function masterySummary(value: unknown): string {
  if (!Array.isArray(value)) return ''
  return value
    .map(normalizeMastery)
    .filter((entry): entry is MasteryEntry => entry !== null)
    .sort((left, right) => right.level - left.level || left.id - right.id)
    .slice(0, 2)
    .map((entry) => entry.name)
    .join(' · ')
}
