<script setup lang="ts">
import { computed } from 'vue'
import { fmtBytes } from '@/lib/format'

export type DonutSlice = {
  label: string
  value: number
  color: string
  /** 图例展示文案；不传则按 valueFormat 格式化 */
  display?: string
}

const props = withDefaults(
  defineProps<{
    slices: DonutSlice[]
    size?: number
    thickness?: number
    centerLabel?: string
    centerValue?: string
    /** bytes：容量；raw：原样数字 */
    valueFormat?: 'bytes' | 'raw'
  }>(),
  { size: 180, thickness: 22, valueFormat: 'raw' },
)

function formatValue(n: number, display?: string) {
  if (display != null && display !== '') return display
  if (props.valueFormat === 'bytes') return fmtBytes(n)
  if (Number.isInteger(n)) return String(n)
  return n.toFixed(1)
}

const total = computed(() => props.slices.reduce((s, x) => s + Math.max(0, Number(x.value) || 0), 0))

const arcs = computed(() => {
  const r = (props.size - props.thickness) / 2
  const c = 2 * Math.PI * r
  const cx = props.size / 2
  const cy = props.size / 2
  let offset = 0
  const t = total.value
  if (t <= 0) {
    return [
      {
        label: '无数据',
        color: 'var(--color-line)',
        dash: `${c}`,
        gap: '0',
        offset: 0,
        value: 0,
        text: '—',
        pct: 0,
        r,
        c,
        cx,
        cy,
      },
    ]
  }
  return props.slices
    .filter((s) => s.value > 0)
    .map((s) => {
      const len = (s.value / t) * c
      const item = {
        label: s.label,
        color: s.color,
        dash: `${len} ${c - len}`,
        gap: String(c - len),
        offset,
        value: s.value,
        text: formatValue(s.value, s.display),
        pct: (s.value / t) * 100,
        r,
        c,
        cx,
        cy,
      }
      offset -= len
      return item
    })
})

const legend = computed(() =>
  props.slices.map((s) => ({
    label: s.label,
    color: s.color,
    text: formatValue(s.value, s.display),
  })),
)
</script>

<template>
  <div class="flex flex-wrap items-center gap-5">
    <svg :width="size" :height="size" class="shrink-0" role="img" :aria-label="centerLabel || '饼图'">
      <g :transform="`rotate(-90 ${size / 2} ${size / 2})`">
        <circle
          v-for="(a, i) in arcs"
          :key="i"
          :cx="a.cx"
          :cy="a.cy"
          :r="a.r"
          fill="none"
          :stroke="a.color"
          :stroke-width="thickness"
          :stroke-dasharray="a.dash"
          :stroke-dashoffset="a.offset"
          stroke-linecap="butt"
        >
          <title>{{ a.label }}: {{ a.text }} ({{ a.pct.toFixed(1) }}%)</title>
        </circle>
      </g>
      <text
        v-if="centerValue || centerLabel"
        :x="size / 2"
        :y="size / 2"
        text-anchor="middle"
        dominant-baseline="central"
        class="fill-fg"
      >
        <tspan v-if="centerValue" :x="size / 2" dy="-0.35em" class="text-base font-semibold">
          {{ centerValue }}
        </tspan>
        <tspan
          v-if="centerLabel"
          :x="size / 2"
          :dy="centerValue ? '1.35em' : '0'"
          class="fill-muted text-xs"
        >
          {{ centerLabel }}
        </tspan>
      </text>
    </svg>
    <ul class="min-w-[9rem] space-y-2 text-sm">
      <li v-for="s in legend" :key="s.label" class="flex items-center gap-2.5 text-muted">
        <span class="inline-block h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ background: s.color }" />
        <span class="text-fg">{{ s.label }}</span>
        <span class="ml-auto font-mono tabular-nums text-fg/80">{{ s.text }}</span>
      </li>
    </ul>
  </div>
</template>
