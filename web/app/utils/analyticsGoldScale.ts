/** Bars show absolute balances; lines offset balances to preserve small changes. */
export function analyticsGoldOrigin(
  values: readonly string[],
  chartType: 'bar' | 'line',
): bigint {
  if (chartType === 'bar') return 0n
  const exact = values
    .filter((value) => /^-?\d+$/.test(value))
    .map((value) => BigInt(value))
  return exact.length
    ? exact.reduce((minimum, value) => (value < minimum ? value : minimum))
    : 0n
}
