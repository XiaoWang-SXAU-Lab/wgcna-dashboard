<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { MtaitResult } from '../api/client'
import { pearson } from '../utils/stats'
import { hclustAverage } from '../utils/hclust'
import { layoutDendrogram } from '../utils/dendrogramLayout'
import { moduleColor } from '../utils/colors'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'

const props = defineProps<{ data: MtaitResult }>()

const lowColor = ref('#313695')
const midColor = ref('#ffffff')
const highColor = ref('#a50026')

function hexToRgb(hex: string): number[] {
  const h = hex.replace('#', '')
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)]
}

function adjColor(v: number): string {
  const mix = (a: number[], b: number[], t: number): string => {
    const c = a.map((x, i) => Math.round(x + (b[i] - x) * t))
    return 'rgb(' + c[0] + ',' + c[1] + ',' + c[2] + ')'
  }
  const lo = hexToRgb(lowColor.value)
  const mid = hexToRgb(midColor.value)
  const hi = hexToRgb(highColor.value)
  if (v < 0.5) return mix(lo, mid, v / 0.5)
  return mix(mid, hi, (v - 0.5) / 0.5)
}

const option = computed<EChartsOption>(() => {
  const modules = props.data.modules
  const eigengenes = props.data.eigengenes
  const m = modules.length

  const cor: number[][] = []
  for (let i = 0; i < m; i++) {
    cor.push([])
    for (let j = 0; j < m; j++) {
      const a = eigengenes.map((row) => row[i] ?? 0)
      const b = eigengenes.map((row) => row[j] ?? 0)
      cor[i].push(pearson(a, b))
    }
  }
  const adjacency = cor.map((row) => row.map((v) => (1 + v) / 2))
  const dissim = adjacency.map((row) => row.map((v) => 1 - v))
  const hc = hclustAverage(dissim)
  const order = hc.order

  const reorderedModules = order.map((i) => modules[i])
  const reorderedAdj = order.map((i) => order.map((j) => adjacency[i][j]))

  const layout = layoutDendrogram(hc.merge, hc.height, order)
  const maxH = hc.height.length ? Math.max(...hc.height) : 1

  const bandH = 0.3   // top color band height (data units)
  const bandW = 0.3   // left color band width
  const bandGap = 0.1 // gap between color band and heatmap
  const topY = (y: number): number => m - 0.5 + bandH + y

  const renderItem = (params: any, api: any): any => {
    const children: any[] = []
    // heatmap cells (x in [0,m-1], y in [0,m-1])
    for (let i = 0; i < m; i++) {
      for (let j = 0; j < m; j++) {
        const v = reorderedAdj[i][j]
        const bl = api.coord([j - 0.5, i - 0.5])
        const tr = api.coord([j + 0.5, i + 0.5])
        children.push({ type: 'rect', shape: { x: bl[0], y: tr[1], width: tr[0] - bl[0], height: bl[1] - tr[1] }, style: { fill: adjColor(v) } })
      }
    }
    // top dendrogram only (x-axis clustering)
    for (const [x1, x2, y] of layout.hSegments) {
      const a = api.coord([x1, topY(y)])
      const b = api.coord([x2, topY(y)])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#4b5563', lineWidth: 1 } })
    }
    for (const [x, y1, y2] of layout.vSegments) {
      const a = api.coord([x, topY(y1)])
      const b = api.coord([x, topY(y2)])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#4b5563', lineWidth: 1 } })
    }
    // top color band (above the heatmap, one block per column)
    for (let p = 0; p < m; p++) {
      const mc = moduleColor(reorderedModules[p])
      const bl = api.coord([p - 0.5, m - 0.5])
      const tr = api.coord([p + 0.5, m - 0.5 + bandH])
      children.push({ type: 'rect', shape: { x: bl[0], y: tr[1], width: tr[0] - bl[0], height: bl[1] - tr[1] }, style: { fill: mc, stroke: '#333', lineWidth: 0.6 } })
    }
    // left color band (one block per row, no dendrogram, no labels)
    for (let p = 0; p < m; p++) {
      const mc = moduleColor(reorderedModules[p])
      const bl = api.coord([-0.5 - bandW - bandGap, p - 0.5])
      const tr = api.coord([-0.5 - bandGap, p + 0.5])
      children.push({ type: 'rect', shape: { x: bl[0], y: tr[1], width: tr[0] - bl[0], height: bl[1] - tr[1] }, style: { fill: mc, stroke: '#333', lineWidth: 0.6 } })
    }
    return { type: 'group', children }
  }

  return {
    grid: { left: 40, right: 30, top: 30, bottom: 80 },
    xAxis: { type: 'value', min: -1.2, max: m - 0.5, show: false },
    yAxis: { type: 'value', min: -2.2, max: m - 0.5 + bandH + maxH + 0.5, show: false },
    toolbox: {
      right: 10, top: 10,
      feature: { saveAsImage: { title: t('comp.downloadPng'), name: 'eigengene_adjacency', pixelRatio: 2, backgroundColor: '#ffffff' } },
    },
    series: [
      { type: 'custom', renderItem, data: [0] },
      {
        type: 'scatter',
        data: reorderedModules.map((name, i) => ({ value: [i, -1.3], name })),
        symbolSize: 0,
        label: { show: true, position: 'bottom', rotate: 45, formatter: (p: any) => p.data.name, color: '#374151', fontSize: 11 },
        silent: true,
        z: 3,
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
