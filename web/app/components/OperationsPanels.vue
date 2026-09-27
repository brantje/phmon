<script setup lang="ts">
const backendReady = useBackendHealth()
</script>

<template>
  <section class="lower-grid">
    <article class="panel operations-panel">
      <div class="panel-header compact">
        <div>
          <h2>Operations</h2>
          <p>Slice 0 health stays available as an operational diagnostic.</p>
        </div>
      </div>
      <dl class="operation-list">
        <div>
          <dt>API / database</dt>
          <dd :class="backendReady ? 'text-ok' : 'text-warning'">
            {{ backendReady ? 'Ready' : 'Unavailable' }}
          </dd>
        </div>
        <div>
          <dt>Agent protocol</dt>
          <dd>v2</dd>
        </div>
        <div>
          <dt>Session reconciliation</dt>
          <dd>3 seconds</dd>
        </div>
      </dl>
    </article>

    <article class="panel next-panel">
      <div class="panel-header compact">
        <div>
          <h2>Presence model</h2>
          <p>
            Agent connection and joined character sessions are tracked
            separately.
          </p>
        </div>
      </div>
      <dl class="operation-list">
        <div>
          <dt>Character identity</dt>
          <dd>Server + character name</dd>
        </div>
        <div>
          <dt>Session recovery</dt>
          <dd>Full snapshot on reconnect</dd>
        </div>
        <div>
          <dt>Training state</dt>
          <dd>Unknown until API getter is verified</dd>
        </div>
      </dl>
    </article>
  </section>
</template>
