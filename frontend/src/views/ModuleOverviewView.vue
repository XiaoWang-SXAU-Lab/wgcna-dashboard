<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { t } from '../utils/i18n'
import {
  getOverview, getOverviewNetwork, runCluster, getCluster, runOverviewNetwork, overviewExportUrl, plotUrl,
  runClusterElbow, getClusterElbow,
  type OverviewData, type OverviewNetwork, type ClusterResult, type ClusterElbow,
} from '../api/client'
import * as echarts from 'echarts'
import 'echarts-gl'
import { moduleAwareLayout, type LayoutResult } from '../utils/networkLayout'

const store = useWgcnaStore()

type Tab = 'overview' | 'network' | 'cluster' | 'table'
const tab = ref<Tab>('overview')

// ---- controls ----
const enrichType = ref<'go' | 'kegg'>('go')
const onto = ref('')   // GO ontology: '' = ALL, 'bp' / 'cc' / 'mf'
const topN = ref(5)
const maxCountOn = ref(true)   // drop terms with gene count > 500 (default on)
const lowColor = ref('#08519c')
const midColor = ref('#ffffff')
const highColor = ref('#a50f15')
const percentile = ref(10)
const clusterMethod = ref<'kmeans' | 'cmeans'>('kmeans')
const clusterK = ref(0)
const clusterSeed = ref(42)
const clusterKmax = ref(12)
const clusterEnrichType = ref<'go' | 'kegg'>('go')
const clusterOnto = ref('')

// ---- data ----
const overview = ref<OverviewData | null>(null)
const network = ref<OverviewNetwork | null>(null)
const cluster = ref<ClusterResult | null>(null)
const loading = ref(false)
const networkLoading = ref(false)
const clusterRunning = ref(false)
const overviewImgLoading = ref(false)
const clusterImgLoading = ref(false)
const selectedModule = ref('')
const selectedCluster = ref('')
const elbow = ref<ClusterElbow | null>(null)
const elbowRunning = ref(false)
const elbowEl = ref<HTMLElement | null>(null)
const elbowChart = ref<echarts.ECharts | null>(null)

const overviewUrl = computed(() =>
  store.jobId ? plotUrl(store.jobId, 'overview', { mode: 'png', type: enrichType.value, onto: onto.value, max_count: maxCountOn.value ? 500 : 0, low: lowColor.value, mid: midColor.value, high: highColor.value, top: topN.value, fdr: 0.05, width: 18, height: 13 }) : '')

const overviewExportUrlPng = computed(() =>
  store.jobId ? plotUrl(store.jobId, 'overview', { mode: 'svg', type: enrichType.value, onto: onto.value, max_count: maxCountOn.value ? 500 : 0, low: lowColor.value, mid: midColor.value, high: highColor.value, top: topN.value, fdr: 0.05 }) : '')

const clusterOverviewUrl = computed(() =>
  store.jobId && cluster.value ? plotUrl(store.jobId, 'overview-cluster', { mode: 'png', type: clusterEnrichType.value, onto: clusterOnto.value, max_count: maxCountOn.value ? 500 : 0, low: lowColor.value, mid: midColor.value, high: highColor.value, top: topN.value, fdr: 0.05, width: 18, height: 13 }) : '')

const networkExportUrl = computed(() =>
  store.jobId ? plotUrl(store.jobId, 'overview-network', { mode: 'svg', per_module: 2000 }) : '')

async function loadOverview() {
  if (!store.jobId) return
  loading.value = true
  try {
    overview.value = await getOverview(store.jobId, enrichType.value, onto.value, 0.05, topN.value)
    if (!selectedModule.value && overview.value.module_order.length) {
      selectedModule.value = overview.value.module_order[0]
    }
  } finally {
    loading.value = false
  }
}

async function loadNetwork() {
  if (!store.jobId || networkLoading.value) return
  networkLoading.value = true
  try {
    await runOverviewNetwork(store.jobId, percentile.value)
    // poll until the network computation finishes (files appear)
    for (let i = 0; i < 240; i++) {
      await new Promise((r) => setTimeout(r, 1000))
      try {
        network.value = await getOverviewNetwork(store.jobId, 300)
        break
      } catch {
        /* still computing */
      }
    }
    layoutCache.value = null
    showNetworkGraph.value = false
  } finally {
    networkLoading.value = false
  }
}

async function runClustering() {
  if (!store.jobId || clusterRunning.value) return
  clusterRunning.value = true
  try {
    await runCluster(store.jobId, clusterMethod.value, clusterK.value, clusterSeed.value)
    for (let i = 0; i < 120; i++) {
      await new Promise((r) => setTimeout(r, 1000))
      try {
        cluster.value = await getCluster(store.jobId)
        break
      } catch {
        /* still running */
      }
    }
    if (cluster.value?.cluster_order.length && !selectedCluster.value) {
      selectedCluster.value = cluster.value.cluster_order[0]
    }
  } finally {
    clusterRunning.value = false
  }
}

async function computeElbow() {
  if (!store.jobId || elbowRunning.value) return
  elbowRunning.value = true
  try {
    await runClusterElbow(store.jobId, clusterMethod.value, clusterSeed.value, clusterKmax.value)
    for (let i = 0; i < 120; i++) {
      await new Promise((r) => setTimeout(r, 1000))
      try {
        elbow.value = await getClusterElbow(store.jobId)
        break
      } catch {
        /* still computing */
      }
    }
    if (elbow.value) {
      clusterK.value = elbow.value.suggested_k
      await nextTick()
      renderElbow()
    }
  } finally {
    elbowRunning.value = false
  }
}

function renderElbow() {
  if (!elbowEl.value || !elbow.value) return
  if (!elbowChart.value) elbowChart.value = echarts.init(elbowEl.value)
  const e = elbow.value
  if (e.method === 'cmeans') {
    elbowChart.value.setOption({
      tooltip: { trigger: 'axis' },
      legend: { data: ['Xie-Beni', 'Partition coefficient'] },
      xAxis: { type: 'category', name: 'k', data: e.ks.map(String) },
      yAxis: [
        { type: 'value', name: 'Xie-Beni (min = best)' },
        { type: 'value', name: 'PC', min: 0, max: 1 },
      ],
      series: [
        { name: 'Xie-Beni', type: 'line', data: e.xb, markPoint: { data: [{ type: 'min' }] } },
        { name: 'Partition coefficient', type: 'line', yAxisIndex: 1, data: e.pc },
      ],
    })
  } else {
    elbowChart.value.setOption({
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', name: 'k', data: e.ks.map(String) },
      yAxis: { type: 'value', name: 'WSS' },
      series: [{ type: 'line', data: e.wss, markPoint: { data: [{ coord: [e.ks.indexOf(e.suggested_k), e.wss[e.ks.indexOf(e.suggested_k)]], value: 'elbow' }] } }],
    })
  }
}

// ---- table ----
const tableMode = ref<'modules' | 'clusters'>('modules')
const tableGenes = computed(() => {
  if (tableMode.value === 'modules' && overview.value) {
    const m = selectedModule.value
    return overview.value.genes
      .map((g, i) => ({ gene: g, group: overview.value!.modules[i], kme: overview.value!.kme[g] ?? null }))
      .filter((r) => r.group === m)
  }
  if (tableMode.value === 'clusters' && cluster.value) {
    const c = selectedCluster.value
    return cluster.value.genes
      .map((g, i) => ({ gene: g, group: cluster.value!.clusters[i], kme: cluster.value!.score[i] ?? null }))
      .filter((r) => r.group === c)
  }
  return []
})

const xlsxUrl = computed(() => store.jobId ? overviewExportUrl(store.jobId, tableMode.value) : '#')

// ---- network ECharts (module-aware hub-centered, fixed coordinates) ----
const netChart = ref<echarts.ECharts | null>(null)
const netEl = ref<HTMLElement | null>(null)
const showNetworkGraph = ref(false)
const layoutCache = ref<LayoutResult | null>(null)

async function renderNetwork() {
  if (!network.value) return
  showNetworkGraph.value = true
  await nextTick()
  if (!netEl.value) return
  if (!netChart.value) netChart.value = echarts.init(netEl.value)

  if (!layoutCache.value) {
    layoutCache.value = moduleAwareLayout(network.value.nodes, network.value.edges)
  }
  const layout = layoutCache.value
  const nodeById = new Map(network.value.nodes.map((n) => [n.id, n]))

  const nodes = network.value.nodes.map((n) => {
    const p = layout.positions.get(n.id)!
    const isHub = layout.hubs.has(n.id)
    const size = isHub ? 6 + n.kme * 12 : 3 + Math.min(n.kme, 0.6) * 10
    return {
      id: n.id,
      name: n.id,
      x: p.x,
      y: p.y,
      symbolSize: Math.max(2.5, size),
      itemStyle: {
        color: n.color,
        borderColor: isHub ? '#111' : n.color,
        borderWidth: isHub ? 2 : 0.4,
        opacity: 0.95,
      },
      module: n.color,
      kme: n.kme,
    }
  })
  const edges = network.value.edges.map((e) => ({
    source: e.source,
    target: e.target,
    lineStyle: { opacity: 0.12, width: 0.5 },
  }))

  netChart.value.setOption({
    animation: false,
    tooltip: {
      formatter: (p: any) => {
        const d = p.data as any
        if (!d || d.source != null) return ''
        const n = nodeById.get(d.id)
        return `<b>${d.id}</b><br/>module: ${d.module}<br/>|kME|: ${d.kme?.toFixed(3) ?? '—'}`
      },
    },
    series: [{
      type: 'graphGL',
      layout: 'none',
      roam: true,
      data: nodes,
      links: edges,
      categories: [...new Set(network.value.nodes.map((n) => n.color))].map((c) => ({ name: c, itemStyle: { color: c } })),
      label: { show: false },
      emphasis: { focus: 'adjacency', label: { show: false } },
      lineStyle: { color: '#9aa5b1' },
    }],
  })
}

watch([enrichType, onto, topN, maxCountOn], () => {
  overviewImgLoading.value = true
  loadOverview()
})
watch([lowColor, midColor, highColor], () => {
  overviewImgLoading.value = true
})
watch([clusterEnrichType, clusterOnto, topN, maxCountOn, lowColor, midColor, highColor], () => {
  if (cluster.value) clusterImgLoading.value = true
})
onMounted(() => {
  if (store.jobId) loadOverview()
})
</script>

<template>
  <div class="p-3">
    <div class="d-flex align-items-center justify-content-between mb-3">
      <h4 class="mb-0 fw-semibold">{{ t('mo.title') }}</h4>
      <a :href="xlsxUrl" class="btn btn-sm btn-outline-success"><i class="bi bi-file-earmark-excel me-1"></i>{{ t('mo.downloadXlsx') }}</a>
    </div>

    <ul class="nav nav-tabs mb-3">
      <li class="nav-item"><button class="nav-link" :class="{ active: tab === 'overview' }" @click="tab = 'overview'">{{ t('mo.overview') }}</button></li>
      <li class="nav-item"><button class="nav-link" :class="{ active: tab === 'network' }" @click="tab = 'network'">{{ t('mo.network') }}</button></li>
      <li class="nav-item"><button class="nav-link" :class="{ active: tab === 'cluster' }" @click="tab = 'cluster'">{{ t('mo.cluster') }}</button></li>
      <li class="nav-item"><button class="nav-link" :class="{ active: tab === 'table' }" @click="tab = 'table'">{{ t('mo.table') }}</button></li>
    </ul>

    <!-- overview figure -->
    <div v-if="tab === 'overview'">
      <div class="d-flex gap-2 align-items-end mb-3">
        <div>
          <label class="form-label small mb-1">{{ t('mo.enrichType') }}</label>
          <select v-model="enrichType" class="form-select form-select-sm">
            <option value="go">GO</option>
            <option value="kegg">KEGG</option>
          </select>
        </div>
        <div v-if="enrichType === 'go'">
          <label class="form-label small mb-1">{{ t('mo.ontology') }}</label>
          <select v-model="onto" class="form-select form-select-sm">
            <option value="">ALL</option>
            <option value="bp">BP</option>
            <option value="cc">CC</option>
            <option value="mf">MF</option>
          </select>
        </div>
        <div>
          <label class="form-label small mb-1">{{ t('mo.topN') }}</label>
          <input type="number" v-model.number="topN" class="form-control form-control-sm" min="1" max="30" style="width: 90px" />
        </div>
        <div class="form-check ms-1" style="padding-top: 1.4rem">
          <input type="checkbox" id="maxCountToggle" v-model="maxCountOn" class="form-check-input" />
          <label for="maxCountToggle" class="form-check-label small">{{ t('mo.dropBroadTerms') }}</label>
        </div>
      </div>
      <div class="d-flex gap-2 align-items-end mb-3">
        <div>
          <label class="form-label small mb-1">{{ t('mo.heatmapColors') }}</label>
          <div class="d-flex gap-2 align-items-center">
            <span class="small text-muted">low</span>
            <input type="color" v-model="lowColor" class="form-control form-control-color form-control-sm" title="low" />
            <span class="small text-muted">mid</span>
            <input type="color" v-model="midColor" class="form-control form-control-color form-control-sm" title="mid" />
            <span class="small text-muted">high</span>
            <input type="color" v-model="highColor" class="form-control form-control-color form-control-sm" title="high" />
          </div>
        </div>
      </div>

      <div v-if="loading" class="text-center text-muted py-5"><div class="spinner-border"></div></div>
      <div v-else-if="overview">
        <div v-if="!overview.enrich_ok" class="alert alert-warning py-2 small">{{ t('mo.enrichMissing') }}</div>
        <div class="border rounded p-2 bg-white position-relative">
          <img :src="overviewUrl" class="w-100" alt="module overview" @load="overviewImgLoading = false" @error="overviewImgLoading = false" />
          <div v-if="overviewImgLoading" class="position-absolute top-0 start-0 w-100 h-100 d-flex flex-column align-items-center justify-content-center" style="background: rgba(255,255,255,0.72); z-index: 2">
            <div class="spinner-border text-primary"></div>
            <div class="small text-muted mt-2">{{ t('mo.generating') }}</div>
          </div>
        </div>
        <div class="mt-2 d-flex gap-2">
          <a :href="overviewExportUrlPng" class="btn btn-sm btn-outline-primary"><i class="bi bi-download me-1"></i>SVG</a>
        </div>
      </div>
      <div v-else class="text-center text-muted py-5">{{ t('mo.noData') }}</div>
    </div>

    <!-- network -->
    <div v-if="tab === 'network'">
      <div class="mb-2 d-flex gap-2 align-items-end">
        <div>
          <label class="form-label small mb-1">{{ t('mo.percentile') }} (%)</label>
          <input type="number" v-model.number="percentile" class="form-control form-control-sm" min="1" max="100" style="width: 90px" />
        </div>
        <button class="btn btn-sm btn-primary" :disabled="networkLoading" @click="loadNetwork">
          <span v-if="networkLoading" class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.buildNetwork') }}
        </button>
        <button v-if="network" class="btn btn-sm btn-outline-secondary" @click="renderNetwork">{{ t('mo.renderNetwork') }}</button>
        <a v-if="network" :href="networkExportUrl" class="btn btn-sm btn-outline-primary"><i class="bi bi-download me-1"></i>SVG</a>
      </div>
      <div v-if="networkLoading" class="small text-muted mb-2"><span class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.generating') }}</div>
      <div v-if="network" class="small text-muted mb-2">
        {{ network.nodes.length }} {{ t('mo.nodes') }} · {{ network.edges.length }} {{ t('mo.edges') }}
      </div>
      <div v-if="showNetworkGraph" ref="netEl" style="height: 560px" class="border rounded bg-white"></div>
      <div v-else class="text-center text-muted py-5">{{ t('mo.networkHint') }}</div>
    </div>

    <!-- cluster -->
    <div v-if="tab === 'cluster'">
      <div class="d-flex gap-2 align-items-end mb-3">
        <div>
          <label class="form-label small mb-1">{{ t('mo.clusterMethod') }}</label>
          <select v-model="clusterMethod" class="form-select form-select-sm">
            <option value="kmeans">KMeans</option>
            <option value="cmeans">{{ t('mo.cmeans') }}</option>
          </select>
        </div>
        <div>
          <label class="form-label small mb-1">{{ t('mo.seed') }}</label>
          <input type="number" v-model.number="clusterSeed" class="form-control form-control-sm" min="0" style="width: 90px" />
        </div>
        <div>
          <label class="form-label small mb-1">{{ t('mo.kmax') }}</label>
          <input type="number" v-model.number="clusterKmax" class="form-control form-control-sm" min="3" max="30" style="width: 90px" />
        </div>
        <button class="btn btn-sm btn-outline-primary" :disabled="elbowRunning" @click="computeElbow">
          <span v-if="elbowRunning" class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.computeElbow') }}
        </button>
      </div>

      <div v-if="elbow" class="border rounded p-3 bg-white mb-3">
        <div class="small text-muted mb-1">
          {{ clusterMethod === 'cmeans' ? t('mo.cmeansHint') : t('mo.elbowHint') }}
          <span class="badge text-bg-primary ms-1">k = {{ elbow.suggested_k }}</span>
        </div>
        <div ref="elbowEl" style="height: 260px"></div>
        <div class="d-flex gap-2 align-items-end mt-3">
          <div>
            <label class="form-label small mb-1">{{ t('mo.k') }}</label>
            <input type="number" v-model.number="clusterK" class="form-control form-control-sm" min="2" :max="clusterKmax" style="width: 90px" />
          </div>
          <button class="btn btn-sm btn-primary" :disabled="clusterRunning || clusterK < 2" @click="runClustering">
            <span v-if="clusterRunning" class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.runCluster') }}
          </button>
        </div>
      </div>
      <div v-else-if="elbowRunning" class="text-center text-muted py-4"><div class="spinner-border"></div></div>

      <div v-if="clusterRunning" class="text-center text-muted py-5"><div class="spinner-border"></div></div>
      <div v-else-if="cluster" class="border rounded p-3 bg-white">
        <div v-if="cluster.degenerate" class="alert alert-warning py-2 small">{{ t('mo.degenerate') }}</div>
        <div class="mb-2">
          <span class="badge text-bg-primary me-2">{{ cluster.method }}</span>
          <span class="badge text-bg-secondary">k = {{ cluster.k }}</span>
        </div>
        <div class="mb-2 small text-muted">
          {{ t('mo.clusterSizes') }}:
          <span v-for="c in cluster.cluster_order" :key="c" class="badge text-bg-light me-1">
            {{ c }} ({{ cluster.clusters.filter((x) => x === c).length }})
          </span>
        </div>
        <div v-if="cluster.metrics?.wss?.length" class="small text-muted mb-2">
          WSS: {{ cluster.metrics.wss.map((x) => Math.round(x)).join(', ') }}
        </div>
        <div class="d-flex gap-2 align-items-end mb-2">
          <div>
            <label class="form-label small mb-1">{{ t('mo.enrichType') }}</label>
            <select v-model="clusterEnrichType" class="form-select form-select-sm">
              <option value="go">GO</option>
              <option value="kegg">KEGG</option>
            </select>
          </div>
          <div v-if="clusterEnrichType === 'go'">
            <label class="form-label small mb-1">{{ t('mo.ontology') }}</label>
            <select v-model="clusterOnto" class="form-select form-select-sm">
              <option value="">ALL</option>
              <option value="bp">BP</option>
              <option value="cc">CC</option>
              <option value="mf">MF</option>
            </select>
          </div>
        </div>
        <div class="border rounded p-2 bg-white mt-2 position-relative">
          <img :src="clusterOverviewUrl" class="w-100" alt="cluster overview" @load="clusterImgLoading = false" @error="clusterImgLoading = false" />
          <div v-if="clusterImgLoading" class="position-absolute top-0 start-0 w-100 h-100 d-flex flex-column align-items-center justify-content-center" style="background: rgba(255,255,255,0.72); z-index: 2">
            <div class="spinner-border text-primary"></div>
            <div class="small text-muted mt-2">{{ t('mo.generating') }}</div>
          </div>
        </div>
      </div>
      <div v-else class="text-center text-muted py-5">{{ t('mo.noData') }}</div>
    </div>

    <!-- table -->
    <div v-if="tab === 'table'">
      <div class="mb-2 d-flex gap-2 align-items-center">
        <select v-model="tableMode" class="form-select form-select-sm" style="width: auto">
          <option value="modules">{{ t('mo.modules') }}</option>
          <option value="clusters">{{ t('mo.clusters') }}</option>
        </select>
        <select v-if="tableMode === 'modules'" v-model="selectedModule" class="form-select form-select-sm" style="width: auto">
          <option v-for="m in overview?.module_order ?? []" :key="m" :value="m">{{ m }}</option>
        </select>
        <select v-else v-model="selectedCluster" class="form-select form-select-sm" style="width: auto">
          <option v-for="c in cluster?.cluster_order ?? []" :key="c" :value="c">{{ c }}</option>
        </select>
        <span class="small text-muted">{{ tableGenes.length }} {{ t('mo.genes') }}</span>
      </div>
      <div class="table-responsive border rounded bg-white" style="max-height: 560px">
        <table class="table table-sm table-hover mb-0">
          <thead class="table-light sticky-top">
            <tr><th>Gene</th><th>{{ t('mo.group') }}</th><th>kME / score</th></tr>
          </thead>
          <tbody>
            <tr v-for="r in tableGenes.slice(0, 500)" :key="r.gene">
              <td class="font-monospace small">{{ r.gene }}</td>
              <td>{{ r.group }}</td>
              <td>{{ r.kme != null ? r.kme.toFixed(3) : '' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
