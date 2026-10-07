<script setup lang="ts">
import {
  Chart,
  registerables,
  type ChartConfiguration,
  type ChartDataset,
} from 'chart.js'
import type { AnalyticsPoint } from '~~/shared/types/live'
import { buildAnalyticsTimeBuckets } from '~/utils/analyticsTimeBuckets'

Chart.register(...registerables)

const props = withDefaults(
  defineProps<{
    title: string
    description: string
    points: readonly AnalyticsPoint[]
    emptyLabel?: string
    categoryLabels?: string[]
    variant?: 'category' | 'time'
    chartType?: 'bar' | 'line'
    unit?: 'count' | 'percent' | 'gold'
    zeroFillMissing?: boolean
    from?: string
    to?: string
    bucket?: 'hour' | 'day' | 'week'
    timezone?: string
  }>(),
  {
    emptyLabel: '',
    categoryLabels: () => [],
    variant: 'category',
    chartType: 'bar',
    unit: 'count',
    zeroFillMissing: true,
    from: '',
    to: '',
    bucket: 'day',
    timezone: 'UTC',
  },
)

const canvas = ref<HTMLCanvasElement>()
const chartID = `analytics-chart-${useId().replace(/\W/g, '')}`
const selectedIndex = ref(0)
const selectedSeries = ref(0)
let chart: Chart | undefined

const palette = [
  '#82a9d8',
  '#d7bd77',
  '#86b7a1',
  '#d28d82',
  '#a99bd4',
  '#78b7c4',
  '#d29dbd',
  '#b9c686',
]

function seriesColor(name: string) {
  let hash = 0
  for (const character of name)
    hash = (hash * 31 + character.charCodeAt(0)) >>> 0
  return palette[hash % palette.length]!
}

function bucketLabel(bucket: string, unit: 'hour' | 'day' | 'week') {
  const [date = '', time = ''] = bucket.split(' ')
  if (unit === 'hour') return `${date.slice(5)} ${time.slice(0, 5)}`
  if (unit === 'week') return `Week of ${date}`
  return date
}

const model = computed(() => {
  const category = props.variant === 'category'
  const unit = props.bucket
  const pointByKey = new Map<string, AnalyticsPoint[]>()
  for (const point of props.points) {
    const key = category ? point.label : point.bucket
    const current = pointByKey.get(key) || []
    current.push(point)
    pointByKey.set(key, current)
  }

  let keys: string[]
  if (category) {
    keys = props.categoryLabels.length
      ? props.categoryLabels
      : [...new Set(props.points.map((point) => point.label))]
  } else {
    const generated = new Set(
      buildAnalyticsTimeBuckets(props.from, props.to, unit, props.timezone),
    )
    for (const key of pointByKey.keys()) generated.add(key)
    keys = [...generated].sort()
  }

  const seriesNames = category
    ? [props.title]
    : [...new Set(props.points.map((point) => point.series || props.title))]
  if (!seriesNames.length && keys.length) seriesNames.push(props.title)
  let goldOrigin = 0n
  if (props.unit === 'gold') {
    const exactValues = props.points
      .map((point) => point.value)
      .filter((value) => /^-?\d+$/.test(value))
      .map((value) => BigInt(value))
    if (exactValues.length)
      goldOrigin = exactValues.reduce((minimum, value) =>
        value < minimum ? value : minimum,
      )
  }
  const labels = keys.map((key) => (category ? key : bucketLabel(key, unit)))
  const datasets = seriesNames.map((seriesName) => {
    const sourceName = category ? undefined : seriesName
    const exact = keys.map((key) => {
      const matching = (pointByKey.get(key) || []).find((point) =>
        category ? true : (point.series || props.title) === sourceName,
      )
      return matching
    })
    const data = exact.map((point) => {
      if (!point) return category || props.zeroFillMissing ? 0 : null
      if (props.unit === 'gold') {
        if (!/^-?\d+$/.test(point.value)) return null
        return Number(BigInt(point.value) - goldOrigin)
      }
      const value = Number(point.value)
      return Number.isFinite(value) ? value : null
    })
    const color = seriesColor(seriesName)
    const dataset: ChartDataset<'bar' | 'line', (number | null)[]> = {
      label: seriesName,
      data,
      borderColor: color,
      backgroundColor: props.chartType === 'line' ? `${color}22` : `${color}d9`,
      borderWidth: props.chartType === 'line' ? 2 : 1,
      pointRadius: props.chartType === 'line' ? 2 : 0,
      pointHoverRadius: 5,
      tension: 0.18,
      spanGaps: false,
      ...(props.chartType === 'bar'
        ? { borderRadius: 2, maxBarThickness: category ? 42 : 18 }
        : {}),
    }
    return { dataset, exact }
  })

  return {
    keys,
    labels,
    seriesNames,
    datasets,
    goldOrigin: goldOrigin.toString(),
  }
})

const selectedPoint = computed(() => {
  const points = model.value.datasets[selectedSeries.value]?.exact || []
  return points[selectedIndex.value]
})
const selectedValue = computed(() => {
  const point = selectedPoint.value
  if (!point)
    return props.zeroFillMissing ? '0 occurrences' : 'No recorded value'
  if (props.unit === 'percent') return `${point.value}%`
  if (props.unit === 'gold') return `${point.value} gold`
  return `${point.value} occurrences`
})

function numberLabel(value: number) {
  if (props.unit === 'percent')
    return `${value.toLocaleString(undefined, { maximumFractionDigits: 1 })}%`
  if (props.unit === 'gold') {
    try {
      return `${(BigInt(model.value.goldOrigin) + BigInt(Math.round(value))).toLocaleString()} gold`
    } catch {
      return 'Gold'
    }
  }
  return Math.round(value).toLocaleString()
}

function showPoint(index: number, series = selectedSeries.value) {
  selectedIndex.value = Math.max(
    0,
    Math.min(index, model.value.keys.length - 1),
  )
  selectedSeries.value = Math.max(
    0,
    Math.min(series, model.value.datasets.length - 1),
  )
}

function buildChart() {
  if (!canvas.value || !model.value.keys.length) {
    chart?.destroy()
    chart = undefined
    return
  }
  const datasets = model.value.datasets.map(({ dataset }) => dataset)
  const config: ChartConfiguration<'bar' | 'line', (number | null)[], string> =
    {
      type: props.chartType,
      data: { labels: model.value.labels, datasets },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        interaction: { mode: 'index', intersect: false },
        plugins: {
          legend: {
            display: datasets.length > 1,
            labels: { color: '#aab7c9', boxWidth: 10, boxHeight: 10 },
          },
          tooltip: {
            callbacks: {
              title(items) {
                const index = items[0]?.dataIndex ?? 0
                return model.value.labels[index] || ''
              },
              label(item) {
                const point =
                  model.value.datasets[item.datasetIndex]?.exact[item.dataIndex]
                if (!point) return `${item.dataset.label}: no observation`
                const unitValue =
                  props.unit === 'percent'
                    ? `${point.value}%`
                    : props.unit === 'gold'
                      ? `${point.value} gold`
                      : `${point.value} occurrences`
                return `${item.dataset.label}: ${unitValue}${point.detail ? ` · ${point.detail}` : ''}`
              },
            },
          },
        },
        scales: {
          x: {
            ticks: {
              color: '#8f9db0',
              maxTicksLimit: 8,
              maxRotation: 0,
              autoSkip: true,
            },
            grid: { color: 'rgba(134, 155, 184, 0.12)' },
          },
          y: {
            beginAtZero: props.unit !== 'gold',
            ticks: {
              color: '#8f9db0',
              precision: props.unit === 'count' ? 0 : undefined,
              callback(value) {
                return numberLabel(Number(value))
              },
            },
            grid: { color: 'rgba(134, 155, 184, 0.12)' },
          },
        },
        onClick(_event, elements) {
          const element = elements[0]
          if (element) showPoint(element.index, element.datasetIndex)
        },
        onHover(_event, elements) {
          const element = elements[0]
          if (element) showPoint(element.index, element.datasetIndex)
        },
      },
    }
  chart?.destroy()
  chart = new Chart(canvas.value, config)
  showPoint(Math.min(selectedIndex.value, model.value.keys.length - 1))
}

watch(
  () => [
    props.points,
    props.variant,
    props.chartType,
    props.unit,
    props.zeroFillMissing,
    props.from,
    props.to,
    props.bucket,
    props.timezone,
    props.title,
    props.categoryLabels,
  ],
  () => nextTick(buildChart),
  { deep: true },
)
onMounted(buildChart)
onBeforeUnmount(() => chart?.destroy())
</script>

<template>
  <section class="panel analytics-chart-panel" :aria-label="title">
    <header class="panel-header compact">
      <div>
        <h2>{{ title }}</h2>
        <p>{{ description }}</p>
      </div>
    </header>
    <div v-if="model.keys.length" class="analytics-chart-wrap">
      <div class="analytics-chart-canvas">
        <canvas
          ref="canvas"
          role="img"
          :aria-label="`${title}. ${points.length} recorded observations.`"
          :aria-describedby="`${chartID}-selected`"
        />
      </div>
      <div class="chart-inspector">
        <label>
          Inspect a value
          <input
            type="range"
            min="0"
            :max="Math.max(0, model.keys.length - 1)"
            :value="selectedIndex"
            :aria-label="`${title} value selector`"
            @input="
              showPoint(Number(($event.target as HTMLInputElement).value))
            "
          />
        </label>
        <p :id="`${chartID}-selected`" aria-live="polite">
          <strong>{{ model.labels[selectedIndex] || 'No bucket' }}</strong>
          <span v-if="model.datasets.length > 1">
            · {{ model.seriesNames[selectedSeries] }}</span
          >
          · {{ selectedValue }}
          <small v-if="selectedPoint?.detail">
            · {{ selectedPoint.detail }}</small
          >
        </p>
      </div>
      <details class="chart-data-table">
        <summary>Accessible numerical data</summary>
        <div class="chart-table-scroll">
          <table>
            <caption>
              {{
                title
              }}
              ·
              {{
                unit === 'count' ? 'occurrences' : unit
              }}
            </caption>
            <thead>
              <tr>
                <th scope="col">Bucket</th>
                <th scope="col">Series</th>
                <th scope="col">Value</th>
                <th scope="col">Evidence</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(key, index) in model.keys" :key="key">
                <th scope="row">{{ model.labels[index] }}</th>
                <td>
                  {{
                    model.seriesNames.length > 1
                      ? model.seriesNames.join(', ')
                      : title
                  }}
                </td>
                <td>
                  {{
                    model.datasets
                      .map(
                        (entry) =>
                          entry.exact[index]?.value ??
                          (zeroFillMissing ? '0' : 'No observation'),
                      )
                      .join(' · ')
                  }}
                </td>
                <td>
                  {{
                    model.datasets
                      .map((entry) => entry.exact[index]?.detail)
                      .filter(Boolean)
                      .join(' · ') || 'Recorded value'
                  }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </details>
    </div>
    <div v-else class="analytics-chart-empty" role="status">
      <span aria-hidden="true">⌁</span>
      <p>{{ emptyLabel || 'No recorded observations in this range.' }}</p>
    </div>
  </section>
</template>

<style scoped>
.analytics-chart-panel {
  min-width: 0;
  min-height: 330px;
}
.analytics-chart-wrap {
  padding: 12px;
}
.analytics-chart-canvas {
  position: relative;
  height: 230px;
  width: 100%;
}
.chart-inspector {
  display: grid;
  grid-template-columns: minmax(120px, 0.7fr) minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  margin-top: 6px;
  color: #9baabe;
  font-size: 11px;
}
.chart-inspector label {
  display: grid;
  gap: 3px;
}
.chart-inspector input {
  width: 100%;
  accent-color: #82a9d8;
  min-height: 24px;
}
.chart-inspector p {
  margin: 0;
  overflow-wrap: anywhere;
}
.chart-inspector strong {
  color: #d8e4f3;
  font-weight: 500;
}
.chart-inspector small {
  color: #8898ad;
}
.chart-data-table {
  margin-top: 5px;
  color: #9baabe;
  font-size: 11px;
}
.chart-data-table summary {
  cursor: pointer;
  width: fit-content;
  padding: 5px 0;
}
.chart-table-scroll {
  max-height: 180px;
  overflow: auto;
}
.chart-data-table table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}
.chart-data-table caption {
  text-align: left;
  padding: 5px;
  color: #c6d3e3;
}
.chart-data-table th,
.chart-data-table td {
  padding: 5px 7px;
  border-bottom: 1px solid #263448;
}
.chart-data-table th {
  color: #aab7c9;
  font-weight: 500;
}
.analytics-chart-empty {
  min-height: 230px;
  display: grid;
  place-content: center;
  text-align: center;
  color: #93a0b2;
}
.analytics-chart-empty span {
  color: #71849e;
  font-size: 30px;
  line-height: 1;
}
.analytics-chart-empty p {
  margin: 8px 0;
}
@media (max-width: 640px) {
  .analytics-chart-panel {
    min-height: 310px;
  }
  .analytics-chart-canvas {
    height: 210px;
  }
  .chart-inspector {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
</style>
