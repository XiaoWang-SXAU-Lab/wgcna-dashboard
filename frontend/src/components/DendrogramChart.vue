<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { Dendrogram } from '../api/client'
import { buildDendrogramTree } from '../utils/dendrogram'
import EChart from './EChart.vue'

const props = defineProps<{
  genes: string[]
  colors: string[]
  dendrogram: Dendrogram
}>()

const option = computed<EChartsOption>(() => ({
  tooltip: { trigger: 'item', triggerOn: 'mousemove' },
  series: [
    {
      type: 'tree',
      data: [buildDendrogramTree(props.dendrogram, props.genes, props.colors)],
      orient: 'LR',
      layout: 'orthogonal',
      symbol: 'circle',
      symbolSize: 2,
      edgeShape: 'polyline',
      roam: true,
      initialTreeDepth: -1,
      label: { show: false },
      leaves: { label: { show: false } },
      emphasis: { focus: 'descendant' },
      expandAndCollapse: false,
      animationDuration: 200,
      animationDurationUpdate: 300,
    },
  ],
}))
</script>

<template>
  <EChart :option="option" height="520px" />
</template>
