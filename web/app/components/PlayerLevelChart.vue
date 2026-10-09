<script setup lang="ts">
import { Chart, registerables, type ChartConfiguration } from 'chart.js'
import {
  chartPoints,
  jobLabel,
  type LevelSnapshot,
} from '~/utils/playerRegistry'

Chart.register(...registerables)

const props = defineProps<{ snapshots: LevelSnapshot[] }>()
const canvas = ref<HTMLCanvasElement>()
let chart: Chart | undefined

const points = computed(() => chartPoints(props.snapshots))

function render() {
  chart?.destroy()
  chart = undefined
  if (!canvas.value || points.value.length === 0) return
  const config: ChartConfiguration = {
    type: 'line',
    data: {
      labels: points.value.map((point) =>
        point.first_seen_at.slice(0, 16).replace('T', ' '),
      ),
      datasets: [
        {
          label: 'Observed level',
          data: points.value.map((point) => point.level),
          borderColor: '#d7bd77',
          backgroundColor: '#d7bd7733',
          pointRadius: 4,
          pointHoverRadius: 6,
          borderWidth: 2,
          tension: 0,
          spanGaps: false,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label(item: { dataIndex: number }) {
              const point = points.value[item.dataIndex]
              if (!point) return ''
              const job = point.job ? jobLabel(point.job) : 'Unknown job'
              const jobLevel =
                point.job_level != null ? ` job level ${point.job_level}` : ''
              return `Level ${point.level} · ${job}${jobLevel}`
            },
          },
        },
      },
      scales: {
        x: {
          ticks: { color: '#8e9caf', maxRotation: 0, autoSkip: true },
          grid: { color: '#1b2839' },
        },
        y: {
          ticks: { color: '#8e9caf', precision: 0 },
          grid: { color: '#1b2839' },
          title: { display: true, text: 'Character level', color: '#8e9caf' },
        },
      },
    },
  }
  chart = new Chart(canvas.value, config)
}

watch(points, async () => {
  await nextTick()
  render()
})
onMounted(render)
onBeforeUnmount(() => chart?.destroy())
</script>

<template>
  <div class="player-level-chart" aria-label="Level progression chart">
    <p v-if="points.length === 0" class="player-empty">
      No observed character levels yet.
    </p>
    <div v-else class="player-level-chart-canvas">
      <canvas
        ref="canvas"
        role="img"
        aria-label="Observed character levels over time"
      />
    </div>
    <p class="player-note">
      Each point is the first time that level was observed. The line only
      connects those samples. Missing levels are not filled in.
    </p>
  </div>
</template>
