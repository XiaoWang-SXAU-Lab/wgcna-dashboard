<!--
  Network graph renderer (Cytoscape). Layout strategies include `ring` (hub inner /
  neighbor outer) and `ringTopN` (top-N concentric rings).

  Attribution: `ringTopN` is inspired by the circular module-network visualization
  in hdWGCNA (Morabito et al., Cell Reports Methods 2023,
  DOI 10.1016/j.crmeth.2023.100498; GPLv3). rsWGCNA re-implements it independently
  as a layout over its own shared scientific network; interactive filtering,
  deterministic sparsification, and provenance tracking are rsWGCNA-specific.
-->
<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import cytoscape from 'cytoscape'
import fcose from 'cytoscape-fcose'
import type { NetworkResult, NetNode } from '../api/client'
import { mixHex } from '../utils/colors'
import { locale } from '../utils/i18n'

cytoscape.use(fcose)

const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en

export type NetLayout = 'fcose' | 'cose' | 'circle' | 'concentric' | 'radial' | 'breadthfirst' | 'ring' | 'ringTopN'
export type SizeBasis = 'degree' | 'kme' | 'gs' | 'tomStrength' | 'kwithin' | 'fixed'
export type LabelMode = 'all' | 'hub' | 'none'
export type NodeColorMode = 'kme' | 'gs' | 'degree' | 'tomStrength' | 'kwithin' | 'hub' | 'module'

const props = defineProps<{
  network: NetworkResult
  layout: NetLayout
  sizeBasis: SizeBasis
  nodeSizeMin: number
  nodeSizeMax: number
  edgeLowColor: string
  edgeHighColor: string
  edgeWidth: number
  edgeColorBy?: 'weight' | 'stability'
  labelSize: number
  labelMode: LabelMode
  geneMap: Record<string, string>
  colorMode: NodeColorMode
  nodeLowColor: string
  nodeHighColor: string
  hubNodeColor: string
  hubBorderColor: string
  nodeBorderColor: string
  nodeBorderWidth: number
  ringTopN?: number
  ringInnerN?: number
}>()

const emit = defineEmits<{
  (e: 'select', payload: { kind: 'node' | 'edge'; data: Record<string, any> }): void
  (e: 'clear'): void
}>()

const container = ref<HTMLElement | null>(null)
let cy: cytoscape.Core | null = null
let timer: ReturnType<typeof setTimeout> | undefined
const tooltip = ref<HTMLElement | null>(null)
const searchInput = ref('')
const selectedNode = ref<string | null>(null)

const isGradient = computed(() => ['kme', 'gs', 'degree', 'tomStrength', 'kwithin'].includes(props.colorMode))
const colorLabel = computed(() => ({ kme: 'kME', gs: 'GS', degree: 'degree', tomStrength: 'TOM strength', kwithin: 'kWithin', hub: 'hub', module: 'module' } as Record<string, string>)[props.colorMode])
const sizeLabel = computed(() => ({ degree: 'degree', kme: 'kME', gs: 'GS', tomStrength: 'TOM strength', kwithin: 'kWithin', fixed: 'fixed' } as Record<string, string>)[props.sizeBasis])
const edgeMetricLabel = computed(() => props.edgeColorBy === 'stability' ? 'stability' : 'weight')

// degree = the degree in the *currently displayed* graph. In Hub-centric mode
// this makes hubs clearly larger (many neighbours) and non-hubs smaller; in
// Sparse/Full it reflects the displayed edge budget. This is a display-layer
// metric, distinct from the worker's full-subgraph degree (n.degree) shown in
// the tooltip.
const displayDegree = computed(() => {
  const m = new Map<string, number>()
  for (const n of props.network.nodes) m.set(n.id, 0)
  for (const e of props.network.edges) {
    m.set(e.source, (m.get(e.source) ?? 0) + 1)
    m.set(e.target, (m.get(e.target) ?? 0) + 1)
  }
  return m
})

function tomStrengthOf(n: NetNode): number | undefined {
  return n.extracted_tom_strength ?? n.kwithin
}

function metric(n: NetNode): number {
  if (props.sizeBasis === 'degree') return displayDegree.value.get(n.id) ?? n.degree
  if (props.sizeBasis === 'tomStrength') return tomStrengthOf(n) ?? NaN
  if (props.sizeBasis === 'kwithin') return n.module_kwithin ?? NaN
  if (props.sizeBasis === 'kme') return n.kme
  if (props.sizeBasis === 'gs') return n.gs
  return 1
}

function colorMetric(n: NetNode): number {
  if (props.colorMode === 'gs') return n.gs
  if (props.colorMode === 'degree') return displayDegree.value.get(n.id) ?? n.degree
  if (props.colorMode === 'tomStrength') return tomStrengthOf(n) ?? NaN
  if (props.colorMode === 'kwithin') return n.module_kwithin ?? NaN
  return n.kme
}

// Missing (non-finite) metric value: node size falls to the midpoint (see scale
// below); node color falls back to the low color here.
function fillFor(n: NetNode, cmin: number, cmax: number): string {
  if (props.colorMode === 'module') return n.color
  if (props.colorMode === 'hub') return n.is_seed ? props.hubNodeColor : props.nodeLowColor
  const cv = colorMetric(n)
  if (!Number.isFinite(cv)) return props.nodeLowColor
  const t = cmax === cmin ? 0.5 : (cv - cmin) / (cmax - cmin)
  return mixHex(props.nodeLowColor, props.nodeHighColor, t)
}

// min/max over the finite metric values. Metrics with a natural zero baseline
// (kME/GS/degree) EXTEND the range to include 0 and at least 1; TOM strength uses
// the actual min/max because it has no meaningful zero baseline.
function rangeOf(values: number[], hasZeroBaseline: boolean): { min: number; max: number } {
  const finite = values.filter((v) => Number.isFinite(v))
  if (!finite.length) return { min: 0, max: 1 }
  let mn = Math.min(...finite)
  let mx = Math.max(...finite)
  if (hasZeroBaseline) {
    mn = Math.min(mn, 0)
    mx = Math.max(mx, 1)
  }
  return { min: mn, max: mx }
}

function computeVisuals() {
  const { nodes } = props.network
  const sizeHasZero = ['kme', 'gs', 'degree'].includes(props.sizeBasis)
  const colorHasZero = ['kme', 'gs', 'degree'].includes(props.colorMode)
  const { min: vmin, max: vmax } = rangeOf(nodes.map(metric), sizeHasZero)
  const scale = (v: number) =>
    !Number.isFinite(v) || vmax === vmin
      ? (props.nodeSizeMin + props.nodeSizeMax) / 2
      : props.nodeSizeMin + ((v - vmin) / (vmax - vmin)) * (props.nodeSizeMax - props.nodeSizeMin)
  const { min: cmin, max: cmax } = rangeOf(nodes.map(colorMetric), colorHasZero)
  return { scale, cmin, cmax }
}

function buildElements() {
  const { nodes, edges } = props.network
  const { scale, cmin, cmax } = computeVisuals()
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
        tomStrength: tomStrengthOf(n),
        kwithin: n.module_kwithin,
        isHub: n.is_seed,
        isHubSci: n.is_hub,
        fill: fillFor(n, cmin, cmax),
      },
    })),
    ...edges.map((e) => ({
      data: { id: `${e.source}::${e.target}`, source: e.source, target: e.target, weight: e.weight, stability: e.stability ?? 0, hasStability: e.stability != null },
    })),
  ]
}

// Update visual data (size/fill/label) in place, keeping node positions.
function applyVisuals() {
  if (!cy) return
  const { scale, cmin, cmax } = computeVisuals()
  const nodeMap = new Map(cy.nodes().map((n) => [n.id(), n]))
  for (const n of props.network.nodes) {
    const el = nodeMap.get(n.id)
    if (!el) continue
    el.data('label', props.geneMap[n.id] || n.id)
    el.data('size', scale(metric(n)))
    el.data('fill', fillFor(n, cmin, cmax))
    el.data('isHub', n.is_seed)
    el.data('isHubSci', n.is_hub)
  }
  cy.style(buildStyle())
}

function edgeValue(e: { weight: number; stability?: number }): number {
  return props.edgeColorBy === 'stability' ? (e.stability ?? e.weight) : e.weight
}

function edgeRange() {
  const ws = props.network.edges.map(edgeValue)
  if (!ws.length) return { min: 0, max: 1 }
  return { min: Math.min(...ws), max: Math.max(...ws) }
}

function buildStyle(): any[] {
  const showLabel = (el: any) => {
    if (props.labelMode === 'all') return true
    if (props.labelMode === 'none') return false
    return el.data('isHub')
  }
  const edgeMetric = props.edgeColorBy === 'stability' ? 'stability' : 'weight'
  const { min: wmin, max: wmax } = edgeRange()
  const spread = wmax - wmin
  const edgeColor = spread > 1e-9
    ? 'mapData(' + edgeMetric + ', ' + wmin + ', ' + wmax + ', ' + props.edgeLowColor + ', ' + props.edgeHighColor + ')'
    : props.edgeHighColor
  const edgeOpacity = spread > 1e-9
    ? 'mapData(' + edgeMetric + ', ' + wmin + ', ' + wmax + ', 0.12, 0.8)'
    : 0.6
  const edgeWidthStyle = spread > 1e-9
    ? 'mapData(' + edgeMetric + ', ' + wmin + ', ' + wmax + ', 0.5, ' + props.edgeWidth + ')'
    : props.edgeWidth / 2

  return [
    {
      selector: 'node',
      style: {
        'background-color': 'data(fill)',
        'border-color': (el: any) => (el.data('isHub') ? props.hubBorderColor : props.nodeBorderColor),
        'border-width': props.nodeBorderWidth,
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
        'line-opacity': edgeOpacity,
        'width': edgeWidthStyle,
        'curve-style': 'bezier',
        'target-arrow-shape': 'none',
      },
    },
    {
      selector: '.dimmed',
      style: { 'opacity': 0.12 },
    },
    {
      selector: 'node.selected',
      style: { 'border-color': '#0f172a', 'border-width': 4 },
    },
    {
      selector: 'node.neighbor',
      style: { 'border-color': '#f59e0b', 'border-width': 3 },
    },
  ]
}

function radialPositions(): Record<string, { x: number; y: number }> {
  const nodes = props.network.nodes
  const edges = props.network.edges
  const hubs = nodes.filter((n) => n.is_seed)
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

function ringPositions(): Record<string, { x: number; y: number }> {
  const hubs = props.network.nodes.filter((n) => n.is_seed).sort((a, b) => b.kme - a.kme)
  const others = props.network.nodes.filter((n) => !n.is_seed).sort((a, b) => b.kme - a.kme)
  const pos: Record<string, { x: number; y: number }> = {}
  const rH = Math.max(70, (hubs.length * (props.nodeSizeMax + 12)) / (2 * Math.PI) + 24)
  const rN = rH + 200
  hubs.forEach((h, i) => {
    const a = (2 * Math.PI * i) / Math.max(1, hubs.length) - Math.PI / 2
    pos[h.id] = { x: rH * Math.cos(a), y: rH * Math.sin(a) }
  })
  others.forEach((n, j) => {
    const a = (2 * Math.PI * j) / Math.max(1, others.length) - Math.PI / 2
    pos[n.id] = { x: rN * Math.cos(a), y: rN * Math.sin(a) }
  })
  return pos
}

// hdWGCNA ModuleNetworkPlot-inspired concentric rings: top-N genes by kME, with
// the top `ringInnerN` in the centre, the next tier in a middle ring, and the
// remaining genes in the outermost ring. Layout only — same nodes/edges.
function ringTopNPositions(): Record<string, { x: number; y: number }> {
  const nodes = [...props.network.nodes].sort((a, b) => b.kme - a.kme)
  const topN = Math.max(1, props.ringTopN ?? 25)
  const innerN = Math.min(Math.max(1, props.ringInnerN ?? 10), topN)
  const inner = nodes.slice(0, innerN)
  const middle = nodes.slice(innerN, topN)
  const outer = nodes.slice(topN)
  const pos: Record<string, { x: number; y: number }> = {}
  const place = (list: { id: string }[], radius: number) => {
    list.forEach((n, i) => {
      const a = (2 * Math.PI * i) / Math.max(1, list.length) - Math.PI / 2
      pos[n.id] = { x: radius * Math.cos(a), y: radius * Math.sin(a) }
    })
  }
  const r0 = Math.max(36, (inner.length * (props.nodeSizeMax + 10)) / (2 * Math.PI) + 16)
  place(inner, r0)
  place(middle, r0 + 130)
  place(outer, r0 + 260)
  return pos
}

function layoutOptions(): any {
  const name = props.layout
  if (name === 'circle') return { name: 'circle' }
  if (name === 'concentric') {
    return { name: 'concentric', level: (n: any) => (n.data('isHub') ? 0 : 1), minNodeSpacing: Math.max(16, props.nodeSizeMax + 6) }
  }
  if (name === 'radial') return { name: 'preset', positions: radialPositions(), fit: true, padding: 40 }
  if (name === 'ring') return { name: 'preset', positions: ringPositions(), fit: true, padding: 40 }
  if (name === 'ringTopN') return { name: 'preset', positions: ringTopNPositions(), fit: true, padding: 40 }
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
  selectedNode.value = null
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
  cy.on('layoutstop pan zoom dragfree', () => container.value?.dispatchEvent(new CustomEvent('report-chart-rendered', { bubbles: true })))
  render()
  cy.on('mouseover', 'node', (e: any) => {
    const d = e.target.data()
    const rows = ['kME ' + d.kme.toFixed(3), 'GS ' + d.gs.toFixed(3), 'degree ' + (displayDegree.value.get(d.id) ?? d.degree)]
    if (d.tomStrength != null) rows.push('TOM strength ' + d.tomStrength.toFixed(2))
    if (d.kwithin != null) rows.push('kWithin ' + d.kwithin.toFixed(2))
    showTooltip(e, '<div class="fw-semibold">' + esc(d.label) + '</div><div>' + rows.join(' · ') + '</div><div>hub ' + (d.isHubSci ? 'yes' : 'no') + ' · seed ' + (d.isHub ? 'yes' : 'no') + '</div>')
  })
  cy.on('mouseout', 'node', hideTooltip)
  cy.on('mouseover', 'edge', (e: any) => {
    const d = e.target.data()
    const stabLine = d.hasStability ? ' · stability ' + d.stability.toFixed(3) : ''
    showTooltip(e, '<div>' + esc(d.source) + ' ↔ ' + esc(d.target) + '</div><div>weight ' + d.weight.toFixed(3) + stabLine + '</div>')
  })
  cy.on('mouseout', 'edge', hideTooltip)
  cy.on('tap', 'node', (e: any) => {
    highlightNode(e.target.id())
    emit('select', { kind: 'node', data: e.target.data() })
  })
  cy.on('tap', 'edge', (e: any) => emit('select', { kind: 'edge', data: e.target.data() }))
  cy.on('tap', (e: any) => { if (e.target === cy) { clearHighlight(); emit('clear') } })
})

onBeforeUnmount(() => {
  if (cy) {
    cy.destroy()
    cy = null
  }
})

// debounce full re-render on param change (sliders)
// network change: full re-render + layout (fresh positions)
watch(() => props.network, () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(render, 250)
})

// visual-only changes: restyle in place, keep node positions (e.g. after expand)
watch(() => [props.sizeBasis, props.nodeSizeMin, props.nodeSizeMax, props.edgeLowColor, props.edgeHighColor, props.edgeWidth, props.edgeColorBy, props.labelSize, props.labelMode, props.geneMap, props.colorMode, props.nodeLowColor, props.nodeHighColor, props.hubNodeColor, props.hubBorderColor, props.nodeBorderColor, props.nodeBorderWidth], () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(applyVisuals, 250)
})

// layout change re-runs the layout without rebuilding elements
watch(() => props.layout, () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(relayout, 150)
})

function zoomIn() { cy?.zoom(cy.zoom() * 1.3) }
function zoomOut() { cy?.zoom(cy.zoom() / 1.3) }
function fit() { cy?.fit(undefined, 40) }

// --- selection / search (P2) ---------------------------------------------

function clearHighlight() {
  selectedNode.value = null
  if (!cy) return
  cy.elements().removeClass('dimmed selected neighbor')
}

function highlightNode(id: string) {
  if (!cy) return
  const node = cy.getElementById(id)
  if (!node || node.length === 0) return
  selectedNode.value = id
  cy.elements().addClass('dimmed')
  const nb = node.closedNeighborhood()
  nb.removeClass('dimmed')
  node.removeClass('dimmed').addClass('selected')
  nb.nodes().not(node).addClass('neighbor')
  if (nb.length) cy.animate({ fit: { eles: nb, padding: 60 } }, { duration: 320 })
}

function searchNode() {
  const q = searchInput.value.trim().toLowerCase()
  if (!q) return
  const hit = props.network.nodes.find((n) => n.id.toLowerCase().includes(q) || (props.geneMap[n.id] ?? '').toLowerCase().includes(q))
  if (hit) highlightNode(hit.id)
}

function resetHighlight() {
  clearHighlight()
  cy?.fit(undefined, 40)
}

// Gephi-style radial expansion/contraction: move node positions (layout), not the view.
function radialScale(factor: number) {
  if (!cy) return
  const nodes = cy.nodes()
  if (nodes.length === 0) return
  const bb = cy.elements().boundingBox()
  const centerX = (bb.x1 + bb.x2) / 2
  const centerY = (bb.y1 + bb.y2) / 2
  nodes.forEach((n) => {
    const p = n.position()
    n.position({ x: centerX + (p.x - centerX) * factor, y: centerY + (p.y - centerY) * factor })
  })
}
function expand() { radialScale(1.4) }
function contract() { radialScale(1 / 1.4) }
</script>

<template>
  <div class="position-relative border rounded bg-white" style="height: calc(100vh - 190px); min-height: 480px">
    <div ref="container" class="w-100 h-100"></div>
    <div class="position-absolute top-0 end-0 m-2 d-flex flex-column gap-1 align-items-end">
      <div class="input-group input-group-sm shadow-sm" style="width: 180px">
        <input v-model="searchInput" class="form-control" placeholder="search gene" @keyup.enter="searchNode" />
        <button class="btn btn-light border" @click="searchNode" title="search"><i class="bi bi-search"></i></button>
      </div>
      <button v-if="selectedNode !== null" class="btn btn-sm btn-light border shadow-sm" @click="resetHighlight" title="reset highlight"><i class="bi bi-x-circle me-1"></i>reset</button>
      <button class="btn btn-sm btn-light border shadow-sm" @click="zoomIn" title="zoom in"><i class="bi bi-plus-lg"></i></button>
      <button class="btn btn-sm btn-light border shadow-sm" @click="zoomOut" title="zoom out"><i class="bi bi-dash-lg"></i></button>
      <button class="btn btn-sm btn-light border shadow-sm" @click="fit" title="fit / center"><i class="bi bi-bounding-box"></i></button>
      <button class="btn btn-sm btn-light border shadow-sm" @click="expand" title="expand (spread nodes)"><i class="bi bi-arrows-angle-expand"></i></button>
      <button class="btn btn-sm btn-light border shadow-sm" @click="contract" title="contract (pull nodes)"><i class="bi bi-arrows-angle-contract"></i></button>
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
        <span class="text-nowrap">{{ edgeMetricLabel }}</span>
        <span class="d-inline-block" :style="{ width: '64px', height: Math.min(edgeWidth, 10) + 'px', background: '#64748b', clipPath: 'polygon(0 50%, 100% 0, 100% 100%)', opacity: 0.7 }"></span>
      </div>
      <div class="d-flex align-items-center gap-2">
        <span class="text-muted text-nowrap" style="width:42px">color</span>
        <span class="text-nowrap">{{ edgeMetricLabel }}</span>
        <span class="flex-grow-1 d-inline-block rounded" :style="{ height: '8px', background: 'linear-gradient(to right, ' + edgeLowColor + ', ' + edgeHighColor + ')' }"></span>
      </div>

      <div class="text-muted mt-2" style="font-size:10px">{{ text('色阶为当前视图相对刻度', 'Color scale is relative to the current view') }}</div>
    </div>
  </div>
</template>
