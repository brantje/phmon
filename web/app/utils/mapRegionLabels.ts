export function uniqueRegionOptionLabels(
  regions: number[],
  zoneNameForRegion: (region: number) => string,
): Map<number, string> {
  const labels = regions.map(
    (region) => [region, zoneNameForRegion(region)] as const,
  )
  const counts = new Map<string, number>()

  for (const [, label] of labels)
    counts.set(label, (counts.get(label) || 0) + 1)

  return new Map(
    labels.map(([region, label]) => [
      region,
      counts.get(label)! > 1 ? `${label} · ${region}` : label,
    ]),
  )
}
