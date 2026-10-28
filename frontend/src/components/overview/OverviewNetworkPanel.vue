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
  spread: number
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
  layout.value = moduleAwareLayout(props.network.nodes, props.network.edges, 70, props.hubPerModule, props.spread)
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

  chart.value.setOption({
    animation: false,
    tooltip: {
      formatter: (p: any) => {
        const d = p.data as any
        if (!d || d.source != null) return ''
        return `<b>${d.id}</b><br/>module: ${d.module}<br/>|kME|: ${d.kme?.toFixed(3) ?? '—'}<br/>kCross: ${d.kCross?.toFixed(2) ?? '—'} (deg ${d.crossDegree ?? 0})`
      },
    },
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
watch(() => [props.showLabels, props.nodeScale, props.edgeOpacity, props.networkMode], render)
// watch layout/filtering changes: recompute layout + render
watch(
  () => [props.network, props.hubPerModule, props.hideIsolated, props.spread],
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
      <button class="btn btn-sm btn-outline-secondary" :disabled="!network" @click="fullscreenToggle"><i class="bi bi-fullscreen me-1"></i>{{ t('mo.fullscreen') }}</button>
      <button class="btn btn-sm btn-outline-secondary" :disabled="!network" @click="emit('export')"><i class="bi bi-download me-1"></i>{{ t('mo.exportView') }}</button>
      <span v-if="network" class="small text-muted ms-auto">
        {{ displayNodes.length }} {{ t('mo.nodes') }}
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
