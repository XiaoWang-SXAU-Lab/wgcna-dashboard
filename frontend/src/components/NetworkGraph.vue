<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { NetworkResult } from '../api/client'
import EChart from './EChart.vue'

const props = defineProps<{ network: NetworkResult; layout: 'force' | 'circular' | 'none'; nodeSize: number; edgeWidth: number }>()

const option = computed<EChartsOption>(() => {
  const { nodes, edges } = props.network
  const degree = new Map<string, number>()
  for (const e of edges) {
    degree.set(e.source, (degree.get(e.source) ?? 0) + 1)
    degree.set(e.target, (degree.get(e.target) ?? 0) + 1)
  }
  const maxDeg = Math.max(1, ...Array.from(degree.values()))
  return {
    tooltip: {
      formatter: (params: any) => {
        if (params.dataType === 'edge') {
          return params.data.source + ' — ' + params.data.target + '<br/>weight = ' + Number(params.data.weight ?? 0).toFixed(4)
        }
        return params.data.id + '<br/>degree = ' + (degree.get(params.data.id) ?? 0)
      },
    },
    series: [{
      type: 'graph',
      layout: props.layout,
      data: nodes.map((n) => ({
        id: n.id, name: n.id,
        symbolSize: 6 + (degree.get(n.id) ?? 0) / maxDeg * props.nodeSize,
        itemStyle: { color: n.color },
      })),
      links: edges.map((e) => ({
        source: e.source, target: e.target, weight: e.weight,
        lineStyle: { width: 0.5 + e.weight * props.edgeWidth, opacity: 0.35, curveness: 0.1 },
      })),
      force: { repulsion: 260, gravity: 0.08, edgeLength: 45 },
      roam: true,
      draggable: true,
      label: { show: false },
      emphasis: { focus: 'adjacency', lineStyle: { opacity: 0.9, width: 2 } },
      edgeSymbol: ['none', 'none'],
    }],
  }
})
</script>

<template>
  <div class="border rounded bg-white p-1" style="height: 560px">
    <EChart :option="option" height="100%" />
  </div>
</template>
