<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { DetailResult } from '../api/client'
import { pearson, linearRegression, corPvalueStudent } from '../utils/stats'
import { moduleColor } from '../utils/colors'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'

const props = defineProps<{ data: DetailResult }>()

const option = computed<EChartsOption>(() => {
  const xs = props.data.mm.map((m) => Math.abs(m))
  const ys = props.data.gs.map((g) => Math.abs(g))
  const points = xs.map((x, i) => [x, ys[i]])

  const cor = pearson(xs, ys)
  const p = corPvalueStudent(cor, xs.length)
  const corText = Number.isFinite(cor) ? cor.toFixed(3) : 'NA'
  const pText = Number.isFinite(p) ? (p < 0.001 ? p.toExponential(1) : p.toFixed(4)) : 'NA'

  const { slope, intercept } = linearRegression(xs, ys)
  const xmin = Math.min(...xs)
  const xmax = Math.max(...xs)
  const line = [[xmin, slope * xmin + intercept], [xmax, slope * xmax + intercept]]

  // 95% confidence interval band (gray, semi-transparent)
  const n = xs.length
  const xMean = xs.reduce((a, b) => a + b, 0) / n
  const sxx = xs.reduce((s, x) => s + (x - xMean) ** 2, 0) || 1
  const s2 = xs.reduce((s, x, i) => s + (ys[i] - (slope * x + intercept)) ** 2, 0) / Math.max(n - 2, 1)
  const tCrit = 1.96
  const se = (x: number) => Math.sqrt(s2 * (1 / n + (x - xMean) ** 2 / sxx))
  const N = 50
  const ciX = Array.from({ length: N }, (_, i) => xmin + ((xmax - xmin) * i) / (N - 1))
  const lowerData = ciX.map((x) => [x, slope * x + intercept - tCrit * se(x)])
  const bandData = ciX.map((x) => [x, 2 * tCrit * se(x)])

  return {
    title: {
      text: t('gs.title') + '（' + props.data.module + ' / ' + props.data.trait_name + '）',
      subtext: 'cor = ' + corText + '　p = ' + pText,
      left: 'center', textStyle: { fontSize: 14 }, subtextStyle: { fontSize: 12, color: '#6b7280' },
    },
    tooltip: {
      trigger: 'item',
      formatter: (params: any) => {
        const i = params.dataIndex as number
        const gene = props.data.genes[i] ?? ''
        const mm = Math.abs(props.data.mm[i] ?? 0)
        const gs = Math.abs(props.data.gs[i] ?? 0)
        return gene + '<br/>|MM| = ' + mm.toFixed(4) + '<br/>|GS| = ' + gs.toFixed(4)
      },
    },
    xAxis: { type: 'value', name: 'Module Membership (|MM|)', nameLocation: 'middle', nameGap: 32, min: 0 },
    yAxis: { type: 'value', name: 'Gene Significance (|GS|)', nameLocation: 'middle', nameRotate: 90, nameGap: 55, min: 0 },
    toolbox: {
      right: 10, top: 10,
      feature: { saveAsImage: { title: t('comp.downloadPng'), name: 'gs_mm_scatter', pixelRatio: 2, backgroundColor: '#ffffff' } },
    },
    series: [
      {
        type: 'scatter', data: points, symbolSize: 8,
        itemStyle: { color: moduleColor(props.data.module), opacity: 0.6 },
      },
      { type: 'line', data: lowerData, stack: 'ci', lineStyle: { opacity: 0 }, areaStyle: { opacity: 0 }, symbol: 'none', silent: true, z: 1 },
      { type: 'line', data: bandData, stack: 'ci', lineStyle: { opacity: 0 }, areaStyle: { color: 'rgba(128,128,128,0.3)' }, symbol: 'none', silent: true, z: 1 },
      { type: 'line', data: line, lineStyle: { color: '#e11d48', width: 2 }, symbol: 'none', silent: true, z: 10 },
    ],
  }
})
</script>

<template>
  <div>
    <ResizableChart :option="option" />
  </div>
</template>
