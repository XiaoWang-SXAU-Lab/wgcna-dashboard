<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import cytoscape from 'cytoscape'
import fcose from 'cytoscape-fcose'
import type { NetworkResult, NetNode } from '../api/client'
import { mixHex } from '../utils/colors'

cytoscape.use(fcose)

export type NetLayout = 'fcose' | 'cose' | 'circle' | 'concentric' | 'radial' | 'breadthfirst'
export type SizeBasis = 'degree' | 'kme' | 'gs' | 'fixed'
export type LabelMode = 'all' | 'hub' | 'none'
export type NodeColorMode = 'kme' | 'gs' | 'degree' | 'hub' | 'module'

const props = defineProps<{
  network: NetworkResult
  layout: NetLayout
  sizeBasis: SizeBasis
  nodeSizeMin: number
  nodeSizeMax: number
  edgeLowColor: string
  edgeHighColor: string
  edgeWidth: number
  labelSize: number
  labelMode: LabelMode
  geneMap: Record<string, string>
  colorMode: NodeColorMode
  nodeLowColor: string
  nodeHighColor: string
  hubNodeColor: string
  hubBorderColor: string
  hubBorderWidth: number
}>()

const container = ref<HTMLElement | null>(null)
let cy: cytoscape.Core | null = null
let timer: ReturnType<typeof setTimeout> | undefined
const tooltip = ref<HTMLElement | null>(null)

const isGradient = computed(() => ['kme', 'gs', 'degree'].includes(props.colorMode))
const colorLabel = computed(() => ({ kme: 'kME', gs: 'GS', degree: 'degree', hub: 'hub', module: 'module' } as Record<string, string>)[props.colorMode])
const sizeLabel = computed(() => ({ degree: 'degree', kme: 'kME', gs: 'GS', fixed: 'fixed' } as Record<string, string>)[props.sizeBasis])

function metric(n: NetNode): number {
  if (props.sizeBasis === 'degree') return n.degree
  if (props.sizeBasis === 'kme') return n.kme
  if (props.sizeBasis === 'gs') return n.gs
  return 1
}

function colorMetric(n: NetNode): number {
  if (props.colorMode === 'gs') return n.gs
  if (props.colorMode === 'degree') return n.degree
  return n.kme
}

function fillFor(n: NetNode, cmin: number, cmax: number): string {
  if (props.colorMode === 'module') return n.color
  if (props.colorMode === 'hub') return n.is_hub ? props.hubNodeColor : props.nodeLowColor
  const t = cmax === cmin ? 0.5 : (colorMetric(n) - cmin) / (cmax - cmin)
  return mixHex(props.nodeLowColor, props.nodeHighColor, t)
}

function buildElements() {
  const { nodes, edges } = props.network
  const values = nodes.map(metric)
  const vmin = Math.min(...values, 0)
  const vmax = Math.max(...values, 1)
  const scale = (v: number) =>
    vmax === vmin
      ? (props.nodeSizeMin + props.nodeSizeMax) / 2
      : props.nodeSizeMin + ((v - vmin) / (vmax - vmin)) * (props.nodeSizeMax - props.nodeSizeMin)

  const cvals = nodes.map(colorMetric)
  const cmin = Math.min(...cvals, 0)
  const cmax = Math.max(...cvals, 1)

  return [
    ...nodes.map((n) => ({
      data: {
        id: n.id,
        label: props.geneMap[n.id] || n.id,
        size: scale(metric(n)),
        color: n.color,
        kme: n.kme,
        gs: n.gs,
        degree: n.degree,
        isHub: n.is_hub,
        fill: fillFor(n, cmin, cmax),
      },
    })),
    ...edges.map((e) => ({
      data: { id: `${e.source}::${e.target}`, source: e.source, target: e.target, weight: e.weight },
    })),
  ]
}

function edgeRange() {
  const ws = props.network.edges.map((e) => e.weight)
  if (!ws.length) return { min: 0, max: 1 }
  return { min: Math.min(...ws), max: Math.max(...ws) }
}

function buildStyle(): any[] {
  const showLabel = (el: any) => {
    if (props.labelMode === 'all') return true
    if (props.labelMode === 'none') return false
    return el.data('isHub')
  }
  const { min: wmin, max: wmax } = edgeRange()
  const spread = wmax - wmin
  const edgeColor = spread > 1e-9
    ? 'mapData(weight, ' + wmin + ', ' + wmax + ', ' + props.edgeLowColor + ', ' + props.edgeHighColor + ')'
    : props.edgeHighColor
  const edgeWidthStyle = spread > 1e-9
    ? 'mapData(weight, ' + wmin + ', ' + wmax + ', 0.5, ' + props.edgeWidth + ')'
    : props.edgeWidth / 2

  return [
    {
      selector: 'node',
      style: {
        'background-color': 'data(fill)',
        'border-color': (el: any) => (el.data('isHub') ? props.hubBorderColor : '#ffffff'),
        'border-width': (el: any) => (el.data('isHub') ? props.hubBorderWidth : 1.5),
        'width': 'data(size)',
        'height': 'data(size)',
        'label': (el: any) => (showLabel(el) ? el.data('label') : ''),
        'font-size': props.labelSize,
        'color': '#1f2937',
        'text-valign': 'bottom',
        'text-margin-y': 4,
        'text-wrap': 'ellipsis',
        'text-max-width': '80px',
      },
    },
    {
      selector: 'edge',
      style: {
        'line-color': edgeColor,
        'line-opacity': 0.45,
        'width': edgeWidthStyle,
        'curve-style': 'bezier',
        'target-arrow-shape': 'none',
      },
    },
  ]
}

function radialPositions(): Record<string, { x: number; y: number }> {
  const nodes = props.network.nodes
  const edges = props.network.edges
  const hubs = nodes.filter((n) => n.is_hub)
  const hubIds = new Set(hubs.map((h) => h.id))
  const pos: Record<string, { x: number; y: number }> = {}

  // Assign each neighbor to the hub it connects to with the strongest edge.
  const best = new Map<string, string>()
  const bestW = new Map<string, number>()
  for (const e of edges) {
    const aIsHub = hubIds.has(e.source)
    const bIsHub = hubIds.has(e.target)
    if (aIsHub === bIsHub) continue
    const hub = aIsHub ? e.source : e.target
    const nb = aIsHub ? e.target : e.source
    if (e.weight > (bestW.get(nb) ?? -1)) {
      bestW.set(nb, e.weight)
      best.set(nb, hub)
    }
  }

  const rH = Math.max(90, (hubs.length * (props.nodeSizeMax + 14)) / (2 * Math.PI) + 30)
  const rN = rH + 280
  hubs.forEach((h, i) => {
    const a = (2 * Math.PI * i) / hubs.length - Math.PI / 2
    pos[h.id] = { x: rH * Math.cos(a), y: rH * Math.sin(a) }
  })

  const groups = new Map<string, string[]>()
  for (const n of nodes) {
    if (hubIds.has(n.id)) continue
    const key = best.get(n.id) ?? ''
    const arr = groups.get(key) ?? []
    arr.push(n.id)
    groups.set(key, arr)
  }

  for (const [hubId, ids] of groups) {
    const hi = hubId ? hubs.findIndex((h) => h.id === hubId) : -1
    const base = hi >= 0 ? (2 * Math.PI * hi) / hubs.length - Math.PI / 2 : 0
    const arc = hubs.length <= 1 ? 2 * Math.PI * 0.94 : Math.max((2 * Math.PI) / hubs.length * 0.92, 0.4)
    ids.forEach((id, j) => {
      const off = ids.length <= 1 ? 0 : (j / (ids.length - 1) - 0.5) * arc
      pos[id] = { x: rN * Math.cos(base + off), y: rN * Math.sin(base + off) }
    })
  }
  return pos
}

function layoutOptions(): any {
  const name = props.layout
  if (name === 'circle') return { name: 'circle' }
  if (name === 'concentric') {
    return { name: 'concentric', level: (n: any) => (n.data('isHub') ? 0 : 1), minNodeSpacing: Math.max(16, props.nodeSizeMax + 6) }
  }
  if (name === 'radial') return { name: 'preset', positions: radialPositions(), fit: true, padding: 40 }
  if (name === 'breadthfirst') {
    return {
      name: 'breadthfirst',
      roots: cy ? cy.nodes().filter((n) => n.data('isHub')) : undefined,
      directed: false,
      spacingFactor: 1.2,
      padding: 40,
    }
  }
  if (name === 'cose') return { name: 'cose', animate: true, padding: 40, nodeRepulsion: 8000 }
  return { name: 'fcose', quality: 'default', animate: true, padding: 40, randomize: true }
}

function render() {
  if (!cy) return
  cy.elements().remove()
  cy.add(buildElements())
  cy.style(buildStyle())
  cy.layout(layoutOptions()).run()
}

function relayout() {
  if (!cy) return
  cy.layout(layoutOptions()).run()
}

function esc(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' } as Record<string, string>)[c])
}

function eventPos(e: any): { x: number; y: number } {
  if (e.renderedPosition) return e.renderedPosition
  const t = e.target
  if (t && typeof t.renderedPosition === 'function') return t.renderedPosition()
  if (t && typeof t.renderedMidpoint === 'function') return t.renderedMidpoint()
  return { x: 0, y: 0 }
}

function showTooltip(e: any, html: string) {
  if (!tooltip.value) return
  const p = eventPos(e)
  tooltip.value.innerHTML = html
  tooltip.value.style.display = 'block'
  tooltip.value.style.left = p.x + 14 + 'px'
  tooltip.value.style.top = p.y + 14 + 'px'
}

function hideTooltip() {
  if (tooltip.value) tooltip.value.style.display = 'none'
}

onMounted(() => {
  if (!container.value) return
  cy = cytoscape({
    container: container.value,
    wheelSensitivity: 0.2,
    minZoom: 0.1,
    maxZoom: 4,
  })
  render()
  cy.on('mouseover', 'node', (e: any) => {
    const d = e.target.data()
    showTooltip(e, '<div class="fw-semibold">' + esc(d.label) + '</div><div>kME ' + d.kme.toFixed(3) + ' · GS ' + d.gs.toFixed(3) + ' · degree ' + d.degree + '</div>')
  })
  cy.on('mouseout', 'node', hideTooltip)
  cy.on('mouseover', 'edge', (e: any) => {
    const d = e.target.data()
    showTooltip(e, '<div>' + esc(d.source) + ' ↔ ' + esc(d.target) + '</div><div>weight ' + d.weight.toFixed(3) + '</div>')
  })
  cy.on('mouseout', 'edge', hideTooltip)
})

onBeforeUnmount(() => {
  if (cy) {
    cy.destroy()
    cy = null
  }
})

// debounce full re-render on param change (sliders)
watch(() => [props.network, props.sizeBasis, props.nodeSizeMin, props.nodeSizeMax, props.edgeLowColor, props.edgeHighColor, props.edgeWidth, props.labelSize, props.labelMode, props.geneMap, props.colorMode, props.nodeLowColor, props.nodeHighColor, props.hubNodeColor, props.hubBorderColor, props.hubBorderWidth], () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(render, 250)
})

// layout change re-runs the layout without rebuilding elements
watch(() => props.layout, () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(relayout, 150)
})

function zoomIn() { cy?.zoom(cy.zoom() * 1.3) }
function zoomOut() { cy?.zoom(cy.zoom() / 1.3) }
function fit() { cy?.fit(undefined, 40) }
</script>

<template>
  <div class="position-relative border rounded bg-white" style="height: calc(100vh - 190px); min-height: 480px">
    <div ref="container" class="w-100 h-100"></div>
    <div class="position-absolute top-0 end-0 m-2 d-flex flex-column gap-1">
      <button class="btn btn-sm btn-light border shadow-sm" @click="zoomIn" title="zoom in"><i class="bi bi-plus-lg"></i></button>
      <button class="btn btn-sm btn-light border shadow-sm" @click="zoomOut" title="zoom out"><i class="bi bi-dash-lg"></i></button>
      <button class="btn btn-sm btn-light border shadow-sm" @click="fit" title="fit / center"><i class="bi bi-arrows-angle-contract"></i></button>
    </div>
    <div ref="tooltip" class="position-absolute rounded" style="display:none; pointer-events:none; z-index:10; background:rgba(17,24,39,0.92); color:#fff; padding:6px 9px; font-size:12px; line-height:1.35; max-width:260px"></div>
    <div class="position-absolute bottom-0 start-0 m-2 p-2 bg-white border rounded shadow-sm small" style="max-width:320px; opacity:0.92">
      <div class="fw-semibold mb-1">Legend</div>

      <div class="text-uppercase text-muted fw-semibold" style="font-size:10px">Node</div>
      <div class="d-flex align-items-center gap-2 mb-1 mt-1">
        <span class="text-muted text-nowrap" style="width:42px">size</span>
        <span class="text-nowrap">{{ sizeLabel }}</span>
        <span class="d-flex align-items-center gap-1">
          <span class="d-inline-block rounded-circle border" :style="{ width: Math.min(nodeSizeMin, 36) + 'px', height: Math.min(nodeSizeMin, 36) + 'px', background: '#64748b' }"></span>
          <span class="text-muted">→</span>
          <span class="d-inline-block rounded-circle border" :style="{ width: Math.min(nodeSizeMax, 44) + 'px', height: Math.min(nodeSizeMax, 44) + 'px', background: '#64748b' }"></span>
        </span>
        <span class="text-muted text-nowrap">{{ nodeSizeMin }} → {{ nodeSizeMax }}</span>
      </div>
      <div class="d-flex align-items-center gap-2 mb-2">
        <span class="text-muted text-nowrap" style="width:42px">color</span>
        <span class="text-nowrap">{{ colorLabel }}</span>
        <span v-if="isGradient" class="flex-grow-1 d-inline-block rounded" :style="{ height: '8px', background: 'linear-gradient(to right, ' + nodeLowColor + ', ' + nodeHighColor + ')' }"></span>
        <span v-else-if="colorMode === 'hub'" class="d-flex align-items-center gap-1">
          <span class="d-inline-block rounded-circle" :style="{ width: '10px', height: '10px', background: hubNodeColor }"></span>
          <span class="text-muted text-nowrap">hub</span>
          <span class="d-inline-block rounded-circle" :style="{ width: '10px', height: '10px', background: nodeLowColor }"></span>
          <span class="text-muted text-nowrap">other</span>
        </span>
        <span v-else class="text-muted text-nowrap">module</span>
      </div>

      <div class="text-uppercase text-muted fw-semibold" style="font-size:10px">Edge</div>
      <div class="d-flex align-items-center gap-2 mb-1 mt-1">
        <span class="text-muted text-nowrap" style="width:42px">size</span>
        <span class="text-nowrap">weight</span>
        <span class="d-inline-block" :style="{ width: '64px', height: Math.min(edgeWidth, 10) + 'px', background: '#64748b', clipPath: 'polygon(0 50%, 100% 0, 100% 100%)', opacity: 0.7 }"></span>
      </div>
      <div class="d-flex align-items-center gap-2">
        <span class="text-muted text-nowrap" style="width:42px">color</span>
        <span class="text-nowrap">weight</span>
        <span class="flex-grow-1 d-inline-block rounded" :style="{ height: '8px', background: 'linear-gradient(to right, ' + edgeLowColor + ', ' + edgeHighColor + ')' }"></span>
      </div>
    </div>
  </div>
</template>
