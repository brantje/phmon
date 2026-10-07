<script setup lang="ts">
import type { AnalyticsPoint } from '~~/shared/types/live'

const props = defineProps<{
  title: string
  description: string
  points: readonly AnalyticsPoint[]
  emptyLabel?: string
}>()

const width = 760
const height = 228
const inset = { left: 36, right: 12, top: 12, bottom: 34 }
const plotWidth = width - inset.left - inset.right
const plotHeight = height - inset.top - inset.bottom
const values = computed(() =>
  props.points.map((point) => {
    const value = Number(point.value)
    return Number.isFinite(value) ? value : 0
  }),
)
const maxValue = computed(() => Math.max(0, ...values.value))
const minValue = computed(() => Math.min(0, ...values.value))
const chartBars = computed(() => {
  const count = props.points.length
  if (!count) return []
  const range = maxValue.value - minValue.value || 1
  const slot = plotWidth / count
  const baseline = inset.top + (maxValue.value / range) * plotHeight
  return props.points.map((point, index) => {
    const value = values.value[index] || 0
    const valueY = inset.top + ((maxValue.value - value) / range) * plotHeight
    const y = Math.min(baseline, valueY)
    const barHeight = Math.max(1, Math.abs(valueY - baseline))
    return {
      ...point,
      x: inset.left + index * slot + Math.max(1, slot * 0.12),
      y,
      width: Math.max(1, slot * 0.76),
      height: barHeight,
      baseline,
      showLabel:
        count <= 12 ||
        index === 0 ||
        index === count - 1 ||
        index % Math.ceil(count / 7) === 0,
    }
  })
})
const ticks = computed(() => {
  const max = maxValue.value
  const min = minValue.value
  if (min < 0) return [max, 0, min]
  return [max, max / 2, 0]
})
</script>

<template>
  <section class="panel analytics-chart-panel" :aria-label="title">
    <header class="panel-header compact">
      <div>
        <h2>{{ title }}</h2>
        <p>{{ description }}</p>
      </div>
    </header>
    <div v-if="chartBars.length" class="analytics-chart-wrap">
      <svg
        class="analytics-chart"
        :viewBox="`0 0 ${width} ${height}`"
        role="img"
        :aria-label="`${title}. ${points.length} recorded buckets.`"
      >
        <title>{{ title }}</title>
        <desc>
          {{ description }} Values are recorded occurrences or observed balance
          changes.
        </desc>
        <g v-for="(tick, index) in ticks" :key="`${tick}-${index}`">
          <line
            :x1="inset.left"
            :x2="width - inset.right"
            :y1="inset.top + (index * plotHeight) / (ticks.length - 1)"
            :y2="inset.top + (index * plotHeight) / (ticks.length - 1)"
            class="chart-gridline"
          />
          <text
            :x="inset.left - 8"
            :y="inset.top + (index * plotHeight) / (ticks.length - 1) + 4"
            text-anchor="end"
            class="chart-axis-label"
          >
            {{
              Number(tick).toLocaleString(undefined, {
                maximumFractionDigits: 1,
              })
            }}
          </text>
        </g>
        <line
          v-if="minValue < 0"
          :x1="inset.left"
          :x2="width - inset.right"
          :y1="chartBars[0]?.baseline"
          :y2="chartBars[0]?.baseline"
          class="chart-zero-line"
        />
        <g
          v-for="(bar, index) in chartBars"
          :key="`${bar.bucket}-${bar.series || ''}-${index}`"
        >
          <rect
            :x="bar.x"
            :y="bar.y"
            :width="bar.width"
            :height="bar.height"
            :class="['chart-bar', { 'is-negative': Number(bar.value) < 0 }]"
            rx="2"
          >
            <title>
              {{ bar.label }}{{ bar.series ? ` · ${bar.series}` : '' }}:
              {{ Number(bar.value).toLocaleString() }}
            </title>
          </rect>
          <text
            v-if="bar.showLabel"
            :x="bar.x + bar.width / 2"
            :y="height - 10"
            text-anchor="middle"
            class="chart-axis-label chart-x-label"
          >
            {{ bar.label.slice(0, 12) }}
          </text>
        </g>
      </svg>
      <table class="sr-only">
        <caption>
          {{
            title
          }}
          values
        </caption>
        <thead>
          <tr>
            <th>Bucket</th>
            <th>Series</th>
            <th>Value</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(point, index) in points"
            :key="`${point.bucket}-${index}`"
          >
            <td>{{ point.label }}</td>
            <td>{{ point.series || title }}</td>
            <td>{{ point.value }}</td>
          </tr>
        </tbody>
      </table>
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
  padding: 14px 12px 4px;
  overflow-x: auto;
}
.analytics-chart {
  display: block;
  width: 100%;
  min-width: 420px;
  height: auto;
  overflow: visible;
}
.chart-gridline {
  stroke: rgba(134, 155, 184, 0.17);
  stroke-width: 1;
}
.chart-zero-line {
  stroke: rgba(254, 246, 195, 0.58);
  stroke-width: 1;
}
.chart-axis-label {
  fill: #8f9db0;
  font:
    11px 'Segoe UI',
    Tahoma,
    Arial,
    sans-serif;
}
.chart-bar {
  fill: #688db8;
  opacity: 0.88;
}
.chart-bar:hover {
  fill: #9cbbdf;
  opacity: 1;
}
.chart-bar.is-negative {
  fill: #d18d78;
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
    min-height: 280px;
  }
  .analytics-chart-wrap {
    padding-inline: 6px;
  }
}
</style>
