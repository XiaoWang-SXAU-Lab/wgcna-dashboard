<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { Dendrogram } from '../api/client'
import { moduleColor } from '../utils/colors'
import { layoutDendrogram } from '../utils/dendrogramLayout'
import { t } from '../utils/i18n'
import EChart from './EChart.vue'

const props = defineProps<{ genes: string[]; colors: string[]; dendrogram: Dendrogram }>()

const option = computed<EChartsOption>(() => {
  const { merge, height } = props.dendrogram
  // worker hclust order is 1-based (1..n): convert to 0-based.
  const order = props.dendrogram.order.map((i) => i - 1)
  const layout = layoutDendrogram(merge, height, order, 0.03)
  const n = layout.n
  const minH = layout.minH
  const maxH = layout.maxH
  const lowestLeafY = minH - layout.hangScaled
  const barH = (maxH - minH) * 0.3

  const renderItem = (params: any, api: any): any => {
    const children: any[] = []
    // module color band cells (below the hanging leaves)
    for (let p = 0; p < n; p++) {
      const geneIdx = order[p]
      const color = moduleColor(props.colors[geneIdx])
      const tl = api.coord([p - 0.5, lowestLeafY])
      const br = api.coord([p + 0.5, lowestLeafY - barH])
      children.push({
        type: 'rect',
        shape: { x: tl[0], y: tl[1], width: br[0] - tl[0], height: br[1] - tl[1] },
        style: { fill: color },
      })
    }
    // top & bottom border of the color band
    const bTop1 = api.coord([-0.5, lowestLeafY])
    const bTop2 = api.coord([n - 0.5, lowestLeafY])
    children.push({ type: 'line', shape: { x1: bTop1[0], y1: bTop1[1], x2: bTop2[0], y2: bTop2[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    const bBot1 = api.coord([-0.5, lowestLeafY - barH])
    const bBot2 = api.coord([n - 0.5, lowestLeafY - barH])
    children.push({ type: 'line', shape: { x1: bBot1[0], y1: bBot1[1], x2: bBot2[0], y2: bBot2[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    // dendrogram edges (rectangle style, hanging leaves)
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
    // 'Module color' label to the left of the color band
    const lp = api.coord([0, lowestLeafY - barH / 2])
    children.push({ type: 'text', style: { text: 'Module color', x: 10, y: lp[1], fill: '#6b7280', fontSize: 11, textAlign: 'left', textVerticalAlign: 'middle' } })
    return { type: 'group', children }
  }

  return {
    grid: { left: 80, right: 16, top: 20, bottom: 20 },
    xAxis: { type: 'value', min: -0.5, max: n - 0.5, show: false },
    yAxis: {
      type: 'value',
      min: lowestLeafY - barH,
      max: maxH * 1.03,
      name: 'Height',
      nameLocation: 'middle',
      nameRotate: 90,
      nameGap: 55,
      axisLabel: { formatter: (v: number) => (v < minH ? '' : v.toFixed(2)) },
      splitLine: { show: true, lineStyle: { color: '#eef1f4', type: 'dashed' } },
    },
    toolbox: {
      right: 10,
      top: 10,
      feature: {
        saveAsImage: {
          title: t('comp.downloadPng'),
          name: 'module_dendrogram',
          pixelRatio: 2,
          backgroundColor: '#ffffff',
        },
      },
    },
    series: [{ type: 'custom', renderItem, data: [0] }],
  }
})
</script>

<template>
  <div>
    <div
      class="border rounded bg-white p-1"
      style="resize: both; overflow: auto; height: 520px; min-height: 300px; min-width: 320px; max-width: 100%"
    >
      <EChart :option="option" height="100%" />
    </div>
    <div class="text-muted small mt-1">
      <i class="bi bi-arrows-angle-expand me-1"></i>{{ t('comp.resize') }}
    </div>
  </div>
</template>
