export function useRegistryServers() {
  const registryServers = useState<string[]>(
    'player-registry-servers',
    () => [],
  )
  const { data } = useAsyncData(
    'player-registry-server-options',
    () => $fetch<{ servers: string[] }>('/api/players/servers'),
    { server: false, lazy: true },
  )
  watch(
    data,
    (value) => {
      if (value) registryServers.value = value.servers
    },
    { immediate: true },
  )
  return registryServers
}
