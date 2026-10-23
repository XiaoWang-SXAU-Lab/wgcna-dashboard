<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { Dendrogram } from '../api/client'
import { layoutDendrogram } from '../utils/dendrogramLayout'
import EChart from './EChart.vue'
import { t } from '../utils/i18n'

export interface CategoryBand {
  name: string
  values: Record<string, string>
}

const props = defineProps<{
  samples: string[]
  tree: Dendrogram
  categories?: CategoryBand[]
  selected?: string[]
}>()
const emit = defineEmits<{ select: [name: string] }>()

const PALETTE = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b', '#e377c2', '#7f7f7f', '#bcbd22', '#17becf', '#aec7e8', '#ffbb78']

const order = computed(() => props.tree.order.map((i) => i - 1))
const sampleNames = computed(() => order.value.map((i) => props.samples[i] ?? ''))
const selectedSet = computed(() => new Set(props.selected ?? []))

const palettes = computed(() => (props.categories ?? []).map((c) => {
  const vals = Object.values(c.values)
  const uniq = [...new Set(vals)].sort()
  const m: Record<string, string> = {}
  uniq.forEach((v, i) => { m[v] = PALETTE[i % PALETTE.length] })
  return { name: c.name, values: c.values, palette: m }
}))

const option = computed<EChartsOption>(() => {
  const { merge, height } = props.tree
  const layout = layoutDendrogram(merge, height, order.value, 0.03)
  const n = layout.n
  const minH = layout.minH
  const maxH = layout.maxH
  const span = Math.max(maxH - minH, 1e-6)
  const lowestLeafY = minH - layout.hangScaled

  const leafTipY = new Array(n).fill(lowestLeafY)
  for (let i = 0; i < merge.length; i++) {
    const tip = height[i] - layout.hangScaled
    const a = merge[i][0]
    const b = merge[i][1]
    if (a < 0) leafTipY[-a - 1] = tip
    if (b < 0) leafTipY[-b - 1] = tip
  }

  const renderItem = (params: any, api: any): any => {
    const children: any[] = []
    for (const [x1, x2, y] of layout.hSegments) {
      const a = api.coord([x1, y])
      const b = api.coord([x2, y])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    }
    for (const [x, y1, y2] of layout.vSegments) {
      const a = api.coord([x, y1])
      const b = api.coord([x, y2])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    }
    return { type: 'group', children }
  }

  const cats = palettes.value
  const K = cats.length
  const grids: any[] = [{ left: 110, right: 16, top: '4%', bottom: K > 0 ? 48 + K * 30 : 44 }]
  const xAxes: any[] = [{ type: 'value', min: -0.5, max: n - 0.5, show: false, gridIndex: 0 }]
  const yAxes: any[] = [
    {
      type: 'value', min: lowestLeafY - 0.08 * span, max: maxH * 1.03,
      name: 'Height', nameLocation: 'middle', nameRotate: 90, nameGap: 55, gridIndex: 0,
      axisLabel: { formatter: (v: number) => (v < minH ? '' : v.toFixed(2)) },
      splitLine: { show: true, lineStyle: { color: '#eef1f4', type: 'dashed' } },
    },
  ]
  const series: any[] = [
    { type: 'custom', renderItem, data: [0], xAxisIndex: 0, yAxisIndex: 0, z: 1 },
    {
      type: 'scatter', xAxisIndex: 0, yAxisIndex: 0, z: 10,
      data: Array.from({ length: n }, (_, p) => ({ value: [p, leafTipY[order.value[p]]], name: sampleNames.value[p] })),
      symbolSize: 9,
      itemStyle: {
        color: (p: any) => (selectedSet.value.has(p.name) ? '#dc2626' : '#0d9488'),
        borderColor: '#ffffff',
        borderWidth: 1.5,
      },
      emphasis: { scale: 1.6 },
    },
  ]

  for (let i = 0; i < K; i++) {
    const gi = i + 1
    const band = cats[i]
    grids.push({ left: 110, right: 16, bottom: 10 + (K - 1 - i) * 30, height: 26 })
    xAxes.push({ type: 'value', min: -0.5, max: n - 0.5, show: false, gridIndex: gi })
    yAxes.push({
      type: 'value', min: 0, max: 1, gridIndex: gi,
      name: band.name, nameLocation: 'middle', nameGap: 12,
      nameRotate: 0,
      nameTextStyle: { fontSize: 11, color: '#4b5563', align: 'right' },
      axisLine: { show: false }, axisTick: { show: false }, axisLabel: { show: false }, splitLine: { show: false },
    })
    series.push({
      type: 'custom', data: [0], xAxisIndex: gi, yAxisIndex: gi, z: 5,
      renderItem: (params: any, api: any): any => {
        const children: any[] = []
        for (let p = 0; p < n; p++) {
          const name = sampleNames.value[p]
          const color = band.palette[band.values[name]] ?? '#d1d5db'
          const x0 = api.coord([p - 0.5, 0])[0]
          const x1 = api.coord([p + 0.5, 0])[0]
          const yTop = api.coord([p, 1])[1]
          const yBot = api.coord([p, 0])[1]
          children.push({ type: 'rect', shape: { x: x0, y: yTop, width: x1 - x0, height: yBot - yTop }, style: { fill: color } })
        }
        return { type: 'group', children }
      },
    })
  }

  return {
    grid: grids,
    xAxis: xAxes,
    yAxis: yAxes,
    tooltip: {
      trigger: 'item', confine: true, backgroundColor: '#0f172a', borderWidth: 0,
      textStyle: { color: '#fff', fontSize: 12 },
      formatter: (params: any) => {
        if (params.seriesType === 'scatter') {
          const name = params.name
          const parts = (props.categories ?? []).map((c) => `${c.name}: ${c.values[name] ?? ''}`)
          return name + (parts.length ? '<br/>' + parts.join('<br/>') : '')
        }
        return ''
      },
    },
    toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: t('comp.downloadPng'), name: 'sample_cluster', pixelRatio: 2, backgroundColor: '#ffffff' } } },
    series,
  }
})

const echartRef = ref<any>(null)

onMounted(() => {
  const c = echartRef.value?.getChart?.()
  if (!c) return
  c.on('click', (params: any) => {
    if (params.seriesType === 'scatter' && params.name) {
      emit('select', params.name)
    }
  })
})
</script>

<template>
  <div>
    <div class="border rounded bg-white p-1" style="resize: both; overflow: auto; height: 560px; min-height: 300px; min-width: 320px; max-width: 100%">
      <EChart ref="echartRef" :option="option" height="100%" />
    </div>
    <div v-if="palettes.length" class="mt-2">
      <div v-for="b in palettes" :key="b.name" class="d-flex flex-wrap gap-2 align-items-center mb-1">
        <span class="small text-muted fw-semibold" style="min-width:90px">{{ b.name }}</span>
        <span v-for="(color, value) in b.palette" :key="value" class="small d-flex align-items-center gap-1">
          <span class="d-inline-block" :style="{ width: '10px', height: '10px', background: color }"></span>{{ value || '—' }}
        </span>
      </div>
    </div>
    <div class="text-muted small mt-1"><i class="bi bi-arrows-angle-expand me-1"></i>{{ t('comp.resize') }} · {{ t('di.sampleTreeHint') }}</div>
  </div>
</template>
