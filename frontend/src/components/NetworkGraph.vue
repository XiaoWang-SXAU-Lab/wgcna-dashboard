<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import cytoscape from 'cytoscape'
import fcose from 'cytoscape-fcose'
import type { NetworkResult, NetNode } from '../api/client'

cytoscape.use(fcose)

export type NetLayout = 'fcose' | 'cose' | 'circle' | 'concentric'
export type SizeBasis = 'degree' | 'kme' | 'gs' | 'fixed'
export type LabelMode = 'all' | 'hub' | 'none'

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
  kmeCutoff: number
  geneMap: Record<string, string>
}>()

const container = ref<HTMLElement | null>(null)
let cy: cytoscape.Core | null = null
let timer: ReturnType<typeof setTimeout> | undefined

function metric(n: NetNode): number {
  if (props.sizeBasis === 'degree') return n.degree
  if (props.sizeBasis === 'kme') return n.kme
  if (props.sizeBasis === 'gs') return n.gs
  return 1
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

  return [
    ...nodes.map((n) => ({
      data: {
        id: n.id,
        label: props.geneMap[n.id] || n.id,
        size: scale(metric(n)),
        color: n.color,
        kme: n.kme,
        isHub: n.kme > props.kmeCutoff,
      },
    })),
    ...edges.map((e) => ({
      data: { id: `${e.source}::${e.target}`, source: e.source, target: e.target, weight: e.weight },
    })),
  ]
}

function buildStyle(): any[] {
  const showLabel = (el: any) => {
    if (props.labelMode === 'all') return true
    if (props.labelMode === 'none') return false
    return el.data('isHub')
  }
  return [
    {
      selector: 'node',
      style: {
        'background-color': 'data(color)',
        'border-color': '#ffffff',
        'border-width': 1.5,
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
        'line-color': 'mapData(weight, 0, 1, ' + props.edgeLowColor + ', ' + props.edgeHighColor + ')',
        'line-opacity': 0.45,
        'width': 'mapData(weight, 0, 1, 0.5, ' + props.edgeWidth + ')',
        'curve-style': 'bezier',
        'target-arrow-shape': 'none',
      },
    },
  ]
}

function layoutOptions(): any {
  const name = props.layout
  if (name === 'circle') return { name: 'circle' }
  if (name === 'concentric') return { name: 'concentric', minNodeSpacing: 20 }
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

onMounted(() => {
  if (!container.value) return
  cy = cytoscape({
    container: container.value,
    wheelSensitivity: 0.2,
    minZoom: 0.1,
    maxZoom: 4,
  })
  render()
})

onBeforeUnmount(() => {
  if (cy) {
    cy.destroy()
    cy = null
  }
})

// debounce full re-render on param change (sliders)
watch(() => [props.network, props.sizeBasis, props.nodeSizeMin, props.nodeSizeMax, props.edgeLowColor, props.edgeHighColor, props.edgeWidth, props.labelSize, props.labelMode, props.kmeCutoff, props.geneMap], () => {
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
  </div>
</template>
