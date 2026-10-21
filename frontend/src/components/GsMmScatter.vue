<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { DetailResult } from '../api/client'
import { pearson, linearRegression, corPvalueStudent } from '../utils/stats'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'

const props = defineProps<{ data: DetailResult }>()
const pointColor = ref('#2563eb')

const option = computed<EChartsOption>(() => {
  // Match the Shiny app's getverboseplot: x = |MM|, y = |GS|, with an lm line.
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
  const line = [
    [xmin, slope * xmin + intercept],
    [xmax, slope * xmax + intercept],
  ]

  return {
    title: {
      text: t('gs.title') + '（' + props.data.module + ' / ' + props.data.trait_name + '）',
      subtext: 'cor = ' + corText + '　p = ' + pText,
      left: 'center',
      textStyle: { fontSize: 14 },
      subtextStyle: { fontSize: 12, color: '#6b7280' },
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
    xAxis: { type: 'value', name: 'Module Membership (|MM|)', min: 0 },
    yAxis: { type: 'value', name: 'Gene Significance (|GS|)', min: 0 },
    toolbox: {
      right: 10, top: 10,
      feature: { saveAsImage: { title: '下载 PNG', name: 'gs_mm_scatter', pixelRatio: 2, backgroundColor: '#ffffff' } },
    },
    series: [
      {
        type: 'scatter',
        data: points,
        symbolSize: 8,
        itemStyle: { color: pointColor.value, opacity: 0.55 },
      },
      {
        type: 'line',
        data: line,
        lineStyle: { color: '#e11d48', width: 2 },
        symbol: 'none',
        silent: true,
        z: 10,
      },
    ],
  }
})
</script>

<template>
  <div>
    <div class="d-flex align-items-center gap-2 mb-2">
      <span class="text-muted small">{{ t('comp.pointColor') }}</span>
      <input type="color" v-model="pointColor" class="form-control form-control-color" />
    </div>
    <ResizableChart :option="option" />
  </div>
</template>
