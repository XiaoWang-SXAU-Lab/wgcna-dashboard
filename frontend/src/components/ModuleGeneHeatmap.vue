<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { DetailResult } from '../api/client'
import { moduleColor } from '../utils/colors'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'

const props = defineProps<{ data: DetailResult }>()

const lowColor = ref('#008000')
const midColor = ref('#000000')
const highColor = ref('#ff0000')

// Z-score each gene (row) across samples, matching R's scale() (sample sd, n-1).
function zscore(matrix: number[][]): number[][] {
  const nSamples = matrix[0]?.length ?? 0
  return matrix.map((row) => {
    let mean = 0
    for (const v of row) mean += v
    mean /= nSamples
    let ss = 0
    for (const v of row) ss += (v - mean) * (v - mean)
    const sd = Math.sqrt(ss / (nSamples - 1))
    return row.map((v) => (sd === 0 ? 0 : (v - mean) / sd))
  })
}

const option = computed<EChartsOption>(() => {
  const genes = props.data.genes
  const z = zscore(props.data.expression)
  const nSamples = z[0]?.length ?? 0
  const sampleLabels = props.data.samples?.length === nSamples
    ? props.data.samples
    : Array.from({ length: nSamples }, (_, i) => 'S' + (i + 1))
  const eigengene = props.data.eigengene ?? []

  const heat: [number, number, number][] = []
  z.forEach((row, gi) => row.forEach((v, si) => heat.push([si, gi, +v.toFixed(3)])))

  let maxAbs = 1
  for (const row of z) for (const v of row) maxAbs = Math.max(maxAbs, Math.abs(v))

  return {
    title: {
      text: props.data.module + ' ' + t('mgh.title'),
      left: 'center',
      textStyle: { fontSize: 14 },
    },
    tooltip: {
      formatter: (params: any) => {
        if (params.seriesType === 'heatmap') {
          const si = params.value[0] as number
          const gi = params.value[1] as number
          return genes[gi] + ' × ' + sampleLabels[si] + '<br/>z = ' + params.value[2]
        }
        const si = params.dataIndex as number
        return sampleLabels[si] + '<br/>Eigengene = ' + (eigengene[si] ?? 0).toFixed(4)
      },
    },
    grid: [
      { left: 120, right: 30, top: 44, height: '56%' },
      { left: 120, right: 30, top: '72%', height: '16%' },
    ],
    xAxis: [
      { type: 'category', data: sampleLabels, gridIndex: 0, axisLabel: { show: false }, axisTick: { show: false } },
      { type: 'category', data: sampleLabels, gridIndex: 1, axisLabel: { rotate: 90, fontSize: 10 } },
    ],
    yAxis: [
      { type: 'category', data: genes, gridIndex: 0, axisLabel: { fontSize: 9 } },
      { type: 'value', gridIndex: 1, axisLabel: { fontSize: 10 } },
    ],
    visualMap: {
      min: -maxAbs, max: maxAbs, calculable: true,
      orient: 'horizontal', left: 'center', bottom: 0,
      inRange: { color: [lowColor.value, midColor.value, highColor.value] },
      seriesIndex: 0,
    },
    toolbox: {
      right: 10, top: 10,
      feature: { saveAsImage: { title: t('comp.downloadPng'), name: 'module_gene_heatmap', pixelRatio: 2, backgroundColor: '#ffffff' } },
    },
    series: [
      {
        type: 'heatmap', xAxisIndex: 0, yAxisIndex: 0, data: heat,
        itemStyle: { borderColor: 'rgba(255,255,255,0.25)', borderWidth: 0.4 },
      },
      {
        type: 'bar', xAxisIndex: 1, yAxisIndex: 1,
        data: eigengene.map((v) => +v.toFixed(3)),
        itemStyle: { color: moduleColor(props.data.module) },
      },
    ],
  }
})
</script>

<template>
  <div>
    <div class="d-flex align-items-center gap-3 mb-2 flex-wrap">
      <span class="text-muted small">{{ t('comp.color') }}</span>
      <label class="d-flex align-items-center gap-1 small mb-0">{{ t('comp.low') }} <input type="color" v-model="lowColor" class="form-control form-control-color" /></label>
      <label class="d-flex align-items-center gap-1 small mb-0">{{ t('comp.mid') }} <input type="color" v-model="midColor" class="form-control form-control-color" /></label>
      <label class="d-flex align-items-center gap-1 small mb-0">{{ t('comp.high') }} <input type="color" v-model="highColor" class="form-control form-control-color" /></label>
    </div>
    <ResizableChart :option="option" />
  </div>
</template>
