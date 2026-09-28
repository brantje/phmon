export function useServerScope() {
  const serverScope = useCookie<string>('phmon-server-scope', {
    default: () => 'all',
    sameSite: 'lax',
    path: '/',
  })
  const { fleetCharacters } = useLiveData()
  const serverOptions = computed(() => {
    const servers = new Map<string, string>()
    for (const character of fleetCharacters.value) {
      const key = character.server.toLocaleLowerCase()
      if (!servers.has(key)) servers.set(key, character.server)
    }
    return [...servers.values()].sort((left, right) =>
      left.localeCompare(right),
    )
  })
  watch(serverOptions, (servers) => {
    if (
      serverScope.value !== 'all' &&
      !servers.some(
        (server) =>
          server.toLocaleLowerCase() === serverScope.value.toLocaleLowerCase(),
      )
    ) {
      serverScope.value = 'all'
    }
  })
  const matchesServer = (server: string) =>
    serverScope.value === 'all' ||
    server.toLocaleLowerCase() === serverScope.value.toLocaleLowerCase()

  return { serverScope, serverOptions, matchesServer }
}
