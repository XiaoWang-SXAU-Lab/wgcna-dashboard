<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { Dendrogram } from '../api/client'
import { layoutDendrogram } from '../utils/dendrogramLayout'
import EChart from './EChart.vue'
import { t } from '../utils/i18n'

const props = defineProps<{ samples: string[]; tree: Dendrogram }>()
const emit = defineEmits<{ select: [name: string] }>()

const order = computed(() => props.tree.order.map((i) => i - 1))
const sampleNames = computed(() => order.value.map((i) => props.samples[i] ?? ''))

const option = computed<EChartsOption>(() => {
  const { merge, height } = props.tree
  const layout = layoutDendrogram(merge, height, order.value, 0.03)
  const n = layout.n
  const minH = layout.minH
  const maxH = layout.maxH
  const span = Math.max(maxH - minH, 1e-6)
  const lowestLeafY = minH - layout.hangScaled

  // Tip y of each leaf (0-based index): its parent merge height minus hang.
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

  return {
    grid: { left: 60, right: 16, top: 20, bottom: 30 },
    xAxis: { type: 'value', min: -0.5, max: n - 0.5, show: false },
    yAxis: {
      type: 'value', min: lowestLeafY - 0.08 * span, max: maxH * 1.03,
      name: 'Height', nameLocation: 'middle', nameGap: 40,
      axisLabel: { formatter: (v: number) => (v < minH ? '' : v.toFixed(2)) },
      splitLine: { show: true, lineStyle: { color: '#eef1f4', type: 'dashed' } },
    },
    tooltip: {
      trigger: 'item', confine: true, backgroundColor: '#0f172a', borderWidth: 0,
      textStyle: { color: '#fff', fontSize: 12 },
      formatter: (params: any) => (params.seriesType === 'scatter' ? params.name : ''),
    },
    toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: '下载 PNG', name: 'sample_cluster', pixelRatio: 2, backgroundColor: '#ffffff' } } },
    series: [
      { type: 'custom', renderItem, data: [0], xAxisIndex: 0, yAxisIndex: 0, z: 1 },
      {
        type: 'scatter', xAxisIndex: 0, yAxisIndex: 0, z: 10,
        data: Array.from({ length: n }, (_, p) => ({ value: [p, leafTipY[order.value[p]]], name: sampleNames.value[p] })),
        symbolSize: 9,
        itemStyle: { color: '#0d9488', borderColor: '#ffffff', borderWidth: 1.5 },
        emphasis: { scale: 1.6, itemStyle: { color: '#f59e0b' } },
      },
    ],
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
    <div class="border rounded bg-white p-1" style="resize: both; overflow: auto; height: 520px; min-height: 300px; min-width: 320px; max-width: 100%">
      <EChart ref="echartRef" :option="option" height="100%" />
    </div>
    <div class="text-muted small mt-1"><i class="bi bi-arrows-angle-expand me-1"></i>{{ t('comp.resize') }} · 点击圆点选择样本，悬停显示样本名</div>
  </div>
</template>
