<script setup lang="ts">
import { computed } from 'vue'
import { locale } from '../utils/i18n'

const props = defineProps<{
  matrix?: number[][]
  height?: number
  title?: string
}>()

const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const bases = ['A', 'C', 'G', 'T']
const colors = ['#2f8f5b', '#2f6fb0', '#d9822b', '#c0392b']

const columns = computed(() => {
  const matrix = props.matrix
  if (!matrix || matrix.length !== 4 || !matrix[0]?.length) return []
  const length = matrix[0].length
  return Array.from({ length }, (_, position) => {
    const probabilities = matrix.map((row) => Math.max(0, row[position] ?? 0))
    const total = probabilities.reduce((sum, value) => sum + value, 0)
    const normalized = total > 0 ? probabilities.map((value) => value / total) : [0.25, 0.25, 0.25, 0.25]
    const entropy = normalized.reduce((sum, value) => sum + (value > 0 ? -value * Math.log2(value) : 0), 0)
    const information = Math.max(0, 2 - entropy)
    return { position, probabilities: normalized, information }
  })
})

const svgWidth = computed(() => Math.max(180, columns.value.length * 38 + 46))
const svgHeight = computed(() => props.height ?? 190)
const plotBottom = computed(() => svgHeight.value - 34)
const bitsToPx = computed(() => (plotBottom.value - 20) / 2)

function letterHeight(probability: number, information: number): number {
  return Math.max(0, probability * information * bitsToPx.value)
}
function stackedHeight(column: { probabilities: number[]; information: number }, base: number): number {
  let offset = 0
  for (let index = 0; index < base; index++) {
    offset += letterHeight(column.probabilities[index], column.information)
  }
  return offset
}
function columnTitle(column: { position: number; probabilities: number[]; information: number }): string {
  const values = column.probabilities.map((value, index) => `${bases[index]}=${value.toFixed(3)}`).join(' ')
  return `position ${column.position + 1}, IC=${column.information.toFixed(3)} bits, ${values}`
}
</script>

<template>
  <div class="rt-logo">
    <div v-if="title" class="small text-muted mb-1">{{ title }}</div>
    <div v-if="!columns.length" class="small text-muted">{{ text('没有可用的 PWM 矩阵；不会生成装饰性 logo。', 'No PWM matrix is available; no decorative logo is generated.') }}</div>
    <svg v-else :viewBox="`0 0 ${svgWidth} ${svgHeight}`" role="img" :aria-label="text('根据真实 PWM 矩阵计算的 sequence logo', 'Sequence logo computed from the actual PWM matrix')">
      <line :x1="36" :x2="svgWidth - 8" :y1="plotBottom" :y2="plotBottom" stroke="#5b5b5b" stroke-width="1" />
      <line :x1="36" :x2="36" :y1="20" :y2="plotBottom" stroke="#5b5b5b" stroke-width="1" />
      <text x="4" :y="24" font-size="9" fill="#5b5b5b">2 bits</text>
      <text x="4" :y="plotBottom" font-size="9" fill="#5b5b5b">0</text>
      <g v-for="column in columns" :key="column.position">
        <title>{{ columnTitle(column) }}</title>
        <line
          :x1="54 + column.position * 38" :x2="54 + column.position * 38"
          :y1="20" :y2="plotBottom" stroke="#eeeeee" stroke-width="1"
        />
        <text
          :x="54 + column.position * 38"
          :y="plotBottom + 14"
          font-size="9"
          text-anchor="middle"
          fill="#5b5b5b"
        >{{ column.position + 1 }}</text>
        <template v-for="(probability, base) in column.probabilities" :key="base">
          <text
            v-if="probability > 0.001"
            :x="54 + column.position * 38"
            :y="plotBottom - stackedHeight(column, base) - 1"
            :font-size="Math.max(7, letterHeight(probability, column.information))"
            text-anchor="middle"
            :fill="colors[base]"
            font-weight="600"
            opacity="0.92"
          >{{ bases[base] }}</text>
        </template>
      </g>
    </svg>
  </div>
</template>



<style scoped>
.rt-logo svg {
  width: 100%;
  max-width: 100%;
  height: auto;
  background: #fcfcfb;
  border: 1px solid #e4e4e4;
  border-radius: 4px;
}
</style>
