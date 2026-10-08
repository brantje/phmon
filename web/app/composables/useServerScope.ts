export function useServerScope() {
  const serverScope = useCookie<string>('phmon-server-scope', {
    default: () => 'all',
    sameSite: 'lax',
    path: '/',
  })
  const { fleetCharacters, groups } = useLiveData()
  const registryServers = useState<string[]>(
    'player-registry-servers',
    () => [],
  )
  const serverOptions = computed(() => {
    const servers = new Map<string, string>()
    for (const character of fleetCharacters.value) {
      const key = character.server.toLocaleLowerCase()
      if (!servers.has(key)) servers.set(key, character.server)
    }
    for (const server of registryServers.value) {
      const key = server.toLocaleLowerCase()
      if (!servers.has(key)) servers.set(key, server)
    }
    if (
      serverScope.value !== 'all' &&
      !servers.has(serverScope.value.toLocaleLowerCase())
    ) {
      servers.set(serverScope.value.toLocaleLowerCase(), serverScope.value)
    }
    return [...servers.values()].sort((left, right) =>
      left.localeCompare(right),
    )
  })
  const matchesServer = (server: string) =>
    serverScope.value === 'all' ||
    server.toLocaleLowerCase() === serverScope.value.toLocaleLowerCase()

  const scopedGroups = computed(() =>
    groups.value.flatMap((group) => {
      if (serverScope.value === 'all') return [group]
      const members = group.members.filter((member) =>
        matchesServer(member.server),
      )
      // Empty groups have no server identity yet, so keep them available for
      // management in the selected scope. Hide groups owned only by another
      // server and show only matching members in cross-server groups.
      return members.length > 0 || group.members.length === 0
        ? [{ ...group, members }]
        : []
    }),
  )

  return { serverScope, serverOptions, matchesServer, scopedGroups }
}
