<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { ScaleFreeCheck } from '../api/client'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'

const props = defineProps<{ data: ScaleFreeCheck }>()

// Plot 1: soft connectivity histogram (k vs frequency).
const hist = computed<EChartsOption>(() => ({
  title: { text: `${t('sft.connHist')} (power = ${props.data.power})`, left: 'center', textStyle: { fontSize: 14 } },
  tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
  grid: { left: 60, right: 20, top: 50, bottom: 40 },
  xAxis: { type: 'category', name: 'k', nameLocation: 'middle', nameGap: 28, data: props.data.hist_mids.map((m) => m.toFixed(1)) },
  yAxis: { type: 'value', name: t('sft.freq') },
  series: [{
    type: 'bar',
    data: props.data.hist_counts,
    itemStyle: { color: '#94a3b8', borderColor: '#334155', borderWidth: 0.5 },
    barCategoryGap: '5%',
  }],
}))

// Plot 2: log-log regression (log10(k) vs log10(p(k))) + fitted line.
const loglog = computed<EChartsOption>(() => {
  const { log_k: x, log_p: y, intercept, slope, adj_r2 } = props.data
  const x0 = Math.min(...x)
  const x1 = Math.max(...x)
  const line = [[x0, intercept + slope * x0], [x1, intercept + slope * x1]]
  return {
    title: {
      text: `${t('sft.scaleCheckTitle')}\nscale R²=${adj_r2.toFixed(2)}, slope=${slope.toFixed(2)}`,
      left: 'center',
      textStyle: { fontSize: 13 },
    },
    tooltip: { trigger: 'item' },
    grid: { left: 60, right: 20, top: 60, bottom: 40 },
    xAxis: { type: 'value', name: 'log10(k)' },
    yAxis: { type: 'value', name: 'log10(p(k))' },
    series: [
      {
        type: 'scatter',
        data: x.map((v, i) => [v, y[i]]),
        symbolSize: 9,
        itemStyle: { color: '#5470c6', borderColor: '#1e40af', borderWidth: 1 },
      },
      {
        type: 'line',
        data: line,
        symbol: 'none',
        lineStyle: { color: '#e11d48', width: 2 },
      },
    ],
  }
})
</script>

<template>
  <div class="row g-3">
    <div class="col-md-6"><ResizableChart :option="hist" /></div>
    <div class="col-md-6"><ResizableChart :option="loglog" /></div>
  </div>
</template>
