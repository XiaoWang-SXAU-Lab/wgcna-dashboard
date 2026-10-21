<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { MtaitResult } from '../api/client'
import { pearson } from '../utils/stats'
import { hclustAverage } from '../utils/hclust'
import { layoutDendrogram } from '../utils/dendrogramLayout'
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

  // correlation matrix (modules x modules)
  const cor: number[][] = []
  for (let i = 0; i < m; i++) {
    cor.push([])
    for (let j = 0; j < m; j++) {
      const a = eigengenes.map((row) => row[i] ?? 0)
      const b = eigengenes.map((row) => row[j] ?? 0)
      cor[i].push(pearson(a, b))
    }
  }
  // adjacency (1+cor)/2 in [0,1]
  const adjacency = cor.map((row) => row.map((v) => (1 + v) / 2))
  // dissimilarity + average-linkage clustering
  const dissim = adjacency.map((row) => row.map((v) => 1 - v))
  const hc = hclustAverage(dissim)
  const order = hc.order

  const reorderedModules = order.map((i) => modules[i])
  const reorderedAdj = order.map((i) => order.map((j) => adjacency[i][j]))

  const layout = layoutDendrogram(hc.merge, hc.height, order)
  const maxH = hc.height.length ? Math.max(...hc.height) : 1
  const leftX = (y: number): number => -(0.5 + y)
  const topY = (y: number): number => m - 0.5 + y

  const renderItem = (params: any, api: any): any => {
    const children: any[] = []
    // heatmap cells
    for (let i = 0; i < m; i++) {
      for (let j = 0; j < m; j++) {
        const v = reorderedAdj[i][j]
        const tl = api.coord([j - 0.5, i - 0.5])
        const br = api.coord([j + 0.5, i + 0.5])
        children.push({
          type: 'rect',
          shape: { x: tl[0], y: br[1], width: br[0] - tl[0], height: tl[1] - br[1] },
          style: { fill: adjColor(v) },
        })
      }
    }
    // top dendrogram
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
    // left dendrogram
    for (const [x1, x2, y] of layout.hSegments) {
      const a = api.coord([leftX(y), x1])
      const b = api.coord([leftX(y), x2])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#4b5563', lineWidth: 1 } })
    }
    for (const [x, y1, y2] of layout.vSegments) {
      const a = api.coord([leftX(y1), x])
      const b = api.coord([leftX(y2), x])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#4b5563', lineWidth: 1 } })
    }
    // labels
    for (let p = 0; p < m; p++) {
      const name = reorderedModules[p]
      const bp = api.coord([p, -0.8])
      children.push({ type: 'text', style: { text: name, x: bp[0], y: bp[1], fill: '#374151', fontSize: 11, textAlign: 'center', textVerticalAlign: 'top' } })
      const lp = api.coord([leftX(maxH) - 0.6, p])
      children.push({ type: 'text', style: { text: name, x: lp[0], y: lp[1], fill: '#374151', fontSize: 11, textAlign: 'right', textVerticalAlign: 'middle' } })
    }
    return { type: 'group', children }
  }

  return {
    grid: { left: 30, right: 30, top: 30, bottom: 70 },
    xAxis: { type: 'value', min: -(0.5 + maxH + 0.8), max: m - 0.5, show: false },
    yAxis: { type: 'value', min: -1.6, max: m - 0.5 + maxH + 0.6, show: false },
    toolbox: {
      right: 10, top: 10,
      feature: { saveAsImage: { title: '下载 PNG', name: 'eigengene_adjacency', pixelRatio: 2, backgroundColor: '#ffffff' } },
    },
    series: [{ type: 'custom', renderItem, data: [0] }],
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
