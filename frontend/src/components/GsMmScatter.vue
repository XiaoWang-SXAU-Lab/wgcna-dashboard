<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { DetailResult } from '../api/client'
import EChart from './EChart.vue'

const props = defineProps<{ data: DetailResult }>()

const option = computed<EChartsOption>(() => ({
  tooltip: { trigger: 'item' },
  xAxis: { type: 'value', name: 'Gene Significance (GS)' },
  yAxis: { type: 'value', name: 'Module Membership (MM)' },
  series: [
    {
      type: 'scatter',
      data: props.data.gs.map((g, i) => [g, props.data.mm[i]]),
      symbolSize: 5,
      itemStyle: { opacity: 0.6 },
    },
  ],
}))
</script>

<template>
  <EChart :option="option" height="420px" />
</template>
