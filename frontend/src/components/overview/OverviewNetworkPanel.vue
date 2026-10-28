<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import * as echarts from 'echarts'
import 'echarts-gl'
import { t } from '../../utils/i18n'
import { moduleAwareLayout, type LayoutResult } from '../../utils/networkLayout'
import type { OverviewNetwork } from '../../api/client'

const props = defineProps<{
  network: OverviewNetwork | null
  loading: boolean
  networkMode: 'intra' | 'inter' | 'combined'
  hubPerModule: number
  geneSpacing: number
  moduleSpacing: number
  hideIsolated: boolean
  showLabels: boolean
  showModuleLabels: boolean
  nodeScale: number
  edgeOpacity: number
}>()

const emit = defineEmits<{
  (e: 'build'): void
  (e: 'export'): void
}>()

const el = ref<HTMLElement | null>(null)
const chart = ref<echarts.ECharts | null>(null)
const layout = ref<LayoutResult | null>(null)
const fullscreen = ref(false)
const layoutSeed = ref(0)
const exportFormat = ref<'cyjs' | 'gexf'>('cyjs')

// display edges: filter the fetched (intra or combined) edges by the mode
const displayEdges = computed(() => {
  if (!props.network) return []
  const all = props.network.edges
  if (props.networkMode === 'intra') return all.filter((e) => e.type === 'intra')
  if (props.networkMode === 'inter') return all.filter((e) => e.type === 'inter')
  return all
})

// display nodes: hide isolated (degree-zero in the displayed edges)
const displayNodes = computed(() => {
  if (!props.network) return []
  const all = props.network.nodes
  if (!props.hideIsolated) return all
  const deg = new Map<string, number>()
  for (const e of displayEdges.value) {
    deg.set(e.source, (deg.get(e.source) ?? 0) + 1)
    deg.set(e.target, (deg.get(e.target) ?? 0) + 1)
  }
  return all.filter((n) => (deg.get(n.id) ?? 0) > 0)
})

const intraCount = computed(() => displayEdges.value.filter((e) => e.type === 'intra').length)
const interCount = computed(() => displayEdges.value.filter((e) => e.type === 'inter').length)

// bridge genes: top k_cross (only relevant for inter/combined)
const bridgeIds = computed(() => {
  if (!props.network || props.networkMode === 'intra') return new Set<string>()
  const sorted = [...props.network.nodes].filter((n) => n.cross_degree > 0).sort((a, b) => b.k_cross - a.k_cross)
  return new Set(sorted.slice(0, 20).map((n) => n.id))
})

// layout from the FULL fetched network (shared across modes; inter/combined
// share one combined layout so nodes don't jump when toggling)
function computeLayout() {
  if (!props.network) return
  const connected = new Set(props.network.edges.flatMap((e) => [e.source, e.target]))
  const nodes = props.hideIsolated ? props.network.nodes.filter((n) => connected.has(n.id)) : props.network.nodes
  layout.value = moduleAwareLayout(nodes, props.network.edges, 70, props.hubPerModule, props.geneSpacing, props.moduleSpacing, layoutSeed.value)
}

function relayout() {
  layoutSeed.value = Math.floor(Math.random() * 100000)
  computeLayout()
  render()
}

// WGCNA module color name → hex, for GEXF viz:color
const WGCNA_HEX: Record<string, string> = {
  turquoise: '#40E0D0', blue: '#0000FF', brown: '#A52A2A', yellow: '#FFD700',
  green: '#008000', red: '#FF0000', black: '#000000', pink: '#FFC0CB',
  magenta: '#FF00FF', purple: '#800080', greenyellow: '#ADFF2F', tan: '#D2B48C',
  salmon: '#FA8072', cyan: '#00FFFF', midnightblue: '#191970', grey: '#808080',
}
function hexToRgb(hex: string): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex)
  if (!m) return '0,0,0'
  const v = parseInt(m[1], 16)
  return `${(v >> 16) & 255},${(v >> 8) & 255},${v & 255}`
}

function xmlAttribute(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&apos;',
  })[character]!)
}

// export the current view as Cytoscape JSON (.cyjs) or Gephi GEXF (.gexf).
// Node label = gene symbol when available, else gene ID; hub/bridge/module are
// kept as attributes so the user can re-style/colour in Cytoscape/Gephi.
function exportView() {
  if (!props.network || !layout.value) return
  const nodeById = new Map(props.network.nodes.map((n) => [n.id, n]))
  const nodeRows = displayNodes.value.map((n) => {
    const p = layout.value!.positions.get(n.id) ?? { x: 0, y: 0 }
    return {
      id: n.id,
      label: n.symbol || n.id,
      module: n.color,
      x: +p.x.toFixed(3),
      y: +p.y.toFixed(3),
      kme: +n.kme.toFixed(6),
      k_within: +n.k_within.toFixed(4),
      k_cross: +n.k_cross.toFixed(4),
      cross_degree: n.cross_degree,
      is_hub: layout.value!.hubs.has(n.id),
      is_bridge: bridgeIds.value.has(n.id),
    }
  })
  const edgeRows = displayEdges.value.map((e) => ({
    source: e.source,
    target: e.target,
    source_module: nodeById.get(e.source)?.color ?? '',
    target_module: nodeById.get(e.target)?.color ?? '',
    weight: e.weight,
    type: e.type,
  }))

  let text = ''
  let ext = ''
  let mime = ''

  if (exportFormat.value === 'gexf') {
    ext = 'gexf'
    mime = 'application/xml'
    const nodesXml = nodeRows.map((n) => {
      const rgb = hexToRgb(WGCNA_HEX[n.module] ?? '#808080')
      return `      <node id="${xmlAttribute(n.id)}" label="${xmlAttribute(n.label)}">\n` +
        `        <attvalues>\n` +
        `          <attvalue for="0" value="${xmlAttribute(n.module)}"/>\n` +
        `          <attvalue for="1" value="${n.is_hub}"/>\n` +
        `          <attvalue for="2" value="${n.is_bridge}"/>\n` +
        `          <attvalue for="3" value="${n.kme}"/>\n` +
        `          <attvalue for="4" value="${n.k_cross}"/>\n` +
        `        </attvalues>\n` +
        `        <viz:color r="${rgb.split(',')[0]}" g="${rgb.split(',')[1]}" b="${rgb.split(',')[2]}"/>\n` +
        `        <viz:position x="${n.x}" y="${n.y}" z="0"/>\n` +
        `      </node>`
    }).join('\n')
    const edgesXml = edgeRows.map((e, i) =>
      `      <edge id="e${i}" source="${xmlAttribute(e.source)}" target="${xmlAttribute(e.target)}" weight="${e.weight}">\n` +
      `        <attvalues><attvalue for="5" value="${e.type}"/></attvalues>\n` +
      `      </edge>`
    ).join('\n')
    text = `<?xml version="1.0" encoding="UTF-8"?>\n` +
      `<gexf xmlns="http://www.gexf.net/1.3" xmlns:viz="http://www.gexf.net/1.3/viz" version="1.3">\n` +
      `  <graph mode="static" defaultedgetype="undirected">\n` +
      `    <attributes class="node">\n` +
      `      <attribute id="0" title="module" type="string"/>\n` +
      `      <attribute id="1" title="is_hub" type="boolean"/>\n` +
      `      <attribute id="2" title="is_bridge" type="boolean"/>\n` +
      `      <attribute id="3" title="kme" type="double"/>\n` +
      `      <attribute id="4" title="k_cross" type="double"/>\n` +
      `    </attributes>\n` +
      `    <attributes class="edge">\n` +
      `      <attribute id="5" title="type" type="string"/>\n` +
      `    </attributes>\n` +
      `    <nodes>\n${nodesXml}\n    </nodes>\n` +
      `    <edges>\n${edgesXml}\n    </edges>\n` +
      `  </graph>\n</gexf>\n`
  } else {
    ext = 'cyjs'
    mime = 'application/json'
    text = JSON.stringify({
      elements: {
        nodes: nodeRows.map((n) => ({ data: n, position: { x: n.x, y: n.y } })),
        edges: edgeRows.map((e) => ({ data: e })),
      },
      data: { name: `rsWGCNA ${props.networkMode} network`, weight_type: props.network.weight_type },
    }, null, 2)
  }

  const blob = new Blob([text], { type: mime })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `network_${props.networkMode}.${ext}`
  a.click()
  URL.revokeObjectURL(a.href)
}

function render() {
  if (!el.value || !props.network || !layout.value) return
  if (!chart.value) chart.value = echarts.init(el.value)

  const nodes = displayNodes.value.map((n) => {
    const p = layout.value!.positions.get(n.id) ?? { x: 0, y: 0 }
    const isHub = layout.value!.hubs.has(n.id)
    const isBridge = bridgeIds.value.has(n.id)
    const size = (isHub ? 6 + n.kme * 12 : 3 + Math.min(n.kme, 0.6) * 10) * props.nodeScale
    return {
      id: n.id,
      name: n.id,
      x: p.x,
      y: p.y,
      symbolSize: Math.max(2, size),
      itemStyle: {
        color: n.color,
        borderColor: isBridge ? '#d97706' : isHub ? '#111' : n.color,
        borderWidth: isBridge || isHub ? 2 : 0.4,
        opacity: 0.95,
      },
      module: n.color,
      kme: n.kme,
      kCross: n.k_cross,
      crossDegree: n.cross_degree,
      isBridge,
      label: { show: props.showLabels, fontSize: 8, color: '#334' },
    }
  })
  const edges = displayEdges.value.map((e) => ({
    source: e.source,
    target: e.target,
    lineStyle: {
      opacity: props.edgeOpacity,
      width: 0.5,
      color: e.type === 'inter' ? '#b45309' : '#9aa5b1',
    },
  }))

  // module labels: text at each module's centroid (data → pixel)
  const graphic: any[] = []
  if (props.showModuleLabels && props.networkMode !== 'inter') {
    const centers = new Map<string, { x: number; y: number; n: number }>()
    for (const n of displayNodes.value) {
      const p = layout.value!.positions.get(n.id)
      if (!p) continue
      const c = centers.get(n.color) ?? { x: 0, y: 0, n: 0 }
      c.x += p.x
      c.y += p.y
      c.n++
      centers.set(n.color, c)
    }
    for (const [mod, c] of centers) {
      if (c.n === 0) continue
      try {
        const px = chart.value!.convertToPixel({ seriesIndex: 0 }, [c.x / c.n, c.y / c.n])
        if (px && px.length >= 2 && Number.isFinite(px[0]) && Number.isFinite(px[1])) {
          graphic.push({
            type: 'text',
            left: px[0] - 20,
            top: px[1] - 8,
            style: { text: mod, fill: '#333', fontSize: 11, fontWeight: 'bold' },
          })
        }
      } catch {
        /* convertToPixel may fail for graphGL before setOption; skip labels */
      }
    }
  }

  chart.value.setOption({
    animation: false,
    tooltip: {
      formatter: (p: any) => {
        const d = p.data as any
        if (!d || d.source != null) return ''
        return `<b>${d.id}</b><br/>module: ${d.module}<br/>|kME|: ${d.kme?.toFixed(3) ?? '—'}<br/>kCross: ${d.kCross?.toFixed(2) ?? '—'} (deg ${d.crossDegree ?? 0})`
      },
    },
    graphic,
    series: [{
      type: 'graphGL',
      layout: 'none',
      roam: true,
      data: nodes,
      links: edges,
      categories: [...new Set(displayNodes.value.map((n) => n.color))].map((c) => ({ name: c, itemStyle: { color: c } })),
      label: { show: false },
      emphasis: { focus: 'adjacency', label: { show: props.showLabels } },
      lineStyle: { color: '#9aa5b1' },
    }],
  })
}

// watch display/styling changes: re-render without recomputing layout
watch(() => [props.showLabels, props.showModuleLabels, props.nodeScale, props.edgeOpacity, props.networkMode], render)
// watch layout/filtering changes: recompute layout + render
watch(
  () => [props.network, props.hubPerModule, props.hideIsolated, props.geneSpacing, props.moduleSpacing],
  async () => {
    computeLayout()
    await nextTick()
    render()
  },
  { immediate: true },
)

function fitView() {
  chart.value?.dispatchAction({ type: 'restore' })
  chart.value?.resize()
}
function fullscreenToggle() {
  const c = el.value?.parentElement as HTMLElement | null
  if (!c) return
  if (!document.fullscreenElement) {
    c.requestFullscreen?.()
    fullscreen.value = true
  } else {
    document.exitFullscreen?.()
    fullscreen.value = false
  }
}
function onFsChange() {
  fullscreen.value = !!document.fullscreenElement
  nextTick(() => chart.value?.resize())
}
document.addEventListener('fullscreenchange', onFsChange)

onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFsChange)
  chart.value?.dispose()
  chart.value = null
})
</script>

<template>
  <div class="d-flex flex-column h-100">
    <!-- toolbar -->
    <div class="d-flex flex-wrap align-items-center gap-2 pb-2 mb-2 border-bottom">
      <button class="btn btn-sm btn-primary" :disabled="loading" @click="emit('build')">
        <span v-if="loading" class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.buildNetwork') }}
      </button>
      <button class="btn btn-sm btn-outline-secondary" :disabled="!network" @click="fitView"><i class="bi bi-arrows-fullscreen me-1"></i>{{ t('mo.fitView') }}</button>
      <button class="btn btn-sm btn-outline-secondary" :disabled="!network" @click="relayout"><i class="bi bi-shuffle me-1"></i>{{ t('mo.relayout') }}</button>
      <button class="btn btn-sm btn-outline-secondary" :disabled="!network" @click="fullscreenToggle"><i class="bi bi-fullscreen me-1"></i>{{ t('mo.fullscreen') }}</button>
      <select v-model="exportFormat" class="form-select form-select-sm" style="width: auto">
        <option value="cyjs">Cytoscape (.cyjs)</option>
        <option value="gexf">Gephi (.gexf)</option>
      </select>
      <button class="btn btn-sm btn-outline-secondary" :disabled="!network" @click="exportView"><i class="bi bi-download me-1"></i>{{ t('mo.exportView') }}</button>
      <span v-if="network" class="small text-muted ms-auto" :title="t('mo.nodeCountHint')">
        {{ displayNodes.length }} / {{ network.nodes.length }} {{ t('mo.nodes') }}
        <span v-if="networkMode !== 'inter'"> · {{ intraCount }} intra</span>
        <span v-if="networkMode !== 'intra'"> · {{ interCount }} inter</span>
      </span>
    </div>

    <!-- chart -->
    <div class="flex-grow-1 position-relative border rounded bg-white" style="min-height: 520px">
      <div v-if="loading" class="position-absolute top-50 start-50 translate-middle text-center text-muted">
        <div class="spinner-border text-primary mb-2"></div>
        <div class="small">{{ t('mo.generating') }}</div>
      </div>
      <div v-else-if="!network" class="position-absolute top-50 start-50 translate-middle text-center text-muted">
        <i class="bi bi-diagram-3 fs-3 d-block mb-2"></i>
        <div class="small">{{ t('mo.networkHint') }}</div>
      </div>
      <div v-show="network && !loading" ref="el" class="w-100 h-100"></div>
    </div>
  </div>
</template>
