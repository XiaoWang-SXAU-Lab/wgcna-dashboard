<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { MtaitResult } from '../api/client'
import EChart from './EChart.vue'

const props = defineProps<{ data: MtaitResult }>()

const option = computed<EChartsOption>(() => {
  const { modules, traits, cor } = props.data
  const heat: [number, number, number][] = []
  cor.forEach((row, i) => row.forEach((v, j) => heat.push([j, i, +v.toFixed(4)])))
  return {
    tooltip: {
      formatter: (p: any) => {
        const [xj, yi, v] = p.value as [number, number, number]
        return `${modules[yi]} × ${traits[xj]}: ${v}`
      },
    },
    grid: { left: 120, right: 30, bottom: 90, top: 20 },
    xAxis: { type: 'category', data: traits, axisLabel: { rotate: 45 } },
    yAxis: { type: 'category', data: modules },
    visualMap: {
      min: -1,
      max: 1,
      calculable: true,
      orient: 'horizontal',
      left: 'center',
      bottom: 0,
      inRange: { color: ['#4575b4', '#ffffff', '#d73027'] },
    },
    series: [
      {
        type: 'heatmap',
        data: heat,
        label: {
          show: true,
          formatter: (p: any) => (p.value as number[])[2].toFixed(2),
          fontSize: 10,
        },
      },
    ],
  }
})
</script>

<template>
  <EChart :option="option" height="420px" />
</template>
