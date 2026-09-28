export function formatUnsignedCount(value: unknown): string | null {
  if (
    typeof value !== 'number' &&
    !(typeof value === 'string' && /^\d+$/.test(value))
  )
    return null
  if (typeof value === 'number' && (!Number.isSafeInteger(value) || value < 0))
    return null
  try {
    return BigInt(value)
      .toString()
      .replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  } catch {
    return null
  }
}

export function formatScaledUnsigned(
  raw: unknown,
  scale: unknown,
  precision: unknown,
): string | null {
  if (
    typeof raw !== 'string' ||
    !/^\d+$/.test(raw) ||
    typeof scale !== 'number' ||
    !Number.isSafeInteger(scale) ||
    scale < 1 ||
    typeof precision !== 'number' ||
    !Number.isInteger(precision) ||
    precision < 0 ||
    precision > 6
  )
    return null

  try {
    const factor = 10n ** BigInt(precision)
    const divisor = BigInt(scale)
    const numerator = BigInt(raw) * factor
    const rounded = (numerator + divisor / 2n) / divisor
    const whole = formatUnsignedCount((rounded / factor).toString())
    if (whole === null || precision === 0) return whole
    const fraction = (rounded % factor)
      .toString()
      .padStart(precision, '0')
      .replace(/0+$/, '')
    return fraction ? `${whole}.${fraction}` : whole
  } catch {
    return null
  }
}
