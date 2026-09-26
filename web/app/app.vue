<script setup lang="ts">
const { data, status, error, refresh } = await useFetch('/api/health', {
  retry: 0,
})
const ready = computed(() => !error.value && data.value?.status === 'ok')
</script>

<template>
  <UApp>
    <main
      class="mx-auto flex min-h-screen max-w-3xl flex-col justify-center px-6 py-16"
    >
      <div class="mb-8">
        <p
          class="mb-3 text-sm font-semibold tracking-widest text-primary uppercase"
        >
          ByteMonitor
        </p>
        <h1 class="text-4xl font-semibold tracking-tight text-highlighted">
          A foundation for your fleet.
        </h1>
        <p class="mt-4 text-lg text-muted">
          Self-hosted phBot monitoring and control.
        </p>
      </div>
      <UCard>
        <template #header>
          <div class="flex items-center justify-between gap-4">
            <h2 class="text-lg font-semibold">System status</h2>
            <UBadge color="neutral" variant="subtle">Local development</UBadge>
          </div>
        </template>
        <div aria-live="polite" role="status">
          <p
            class="text-xl font-medium"
            :class="ready ? 'text-success' : 'text-warning'"
          >
            {{
              status === 'pending'
                ? 'Checking connection…'
                : ready
                  ? 'All systems ready'
                  : 'Connection unavailable'
            }}
          </p>
          <p class="mt-2 text-muted">
            {{
              ready
                ? 'The backend is responding and PostgreSQL is connected.'
                : 'The backend or database is unavailable. Check the services and try again.'
            }}
          </p>
        </div>
        <template #footer>
          <UButton
            :loading="status === 'pending'"
            color="neutral"
            variant="outline"
            @click="refresh()"
            >Check again</UButton
          >
        </template>
      </UCard>
      <p class="mt-6 text-sm text-muted">
        Development foundation only. Agent connectivity is the next step.
      </p>
    </main>
  </UApp>
</template>
