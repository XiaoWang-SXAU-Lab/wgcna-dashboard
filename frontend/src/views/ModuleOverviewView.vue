<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import {
  getOverview, getOverviewNetwork, runCluster, getCluster, runOverviewNetwork, overviewExportUrl, plotUrl,
  type OverviewData, type OverviewNetwork, type ClusterResult,
} from '../api/client'
import * as echarts from 'echarts'
import 'echarts-gl'

const store = useWgcnaStore()

type Tab = 'overview' | 'network' | 'cluster' | 'table'
const tab = ref<Tab>('overview')

// ---- controls ----
const enrichType = ref<'go' | 'kegg'>('go')
const topN = ref(5)
const percentile = ref(10)
const clusterMethod = ref<'kmeans' | 'cmeans'>('kmeans')
const clusterK = ref(0)
const clusterSeed = ref(42)

// ---- data ----
const overview = ref<OverviewData | null>(null)
const network = ref<OverviewNetwork | null>(null)
const cluster = ref<ClusterResult | null>(null)
const loading = ref(false)
const networkLoading = ref(false)
const clusterRunning = ref(false)
const selectedModule = ref('')
const selectedCluster = ref('')

const overviewUrl = computed(() =>
  store.jobId ? plotUrl(store.jobId, 'overview', { mode: 'png', type: enrichType.value, top: topN.value, fdr: 0.05, width: 18, height: 13 }) : '')

const overviewExportUrlPng = computed(() =>
  store.jobId ? plotUrl(store.jobId, 'overview', { mode: 'svg', type: enrichType.value, top: topN.value, fdr: 0.05 }) : '')

const networkExportUrl = computed(() =>
  store.jobId ? plotUrl(store.jobId, 'overview-network', { mode: 'svg', per_module: 2000 }) : '')

async function loadOverview() {
  if (!store.jobId) return
  loading.value = true
  try {
    overview.value = await getOverview(store.jobId, enrichType.value, 0.05, topN.value)
    if (!selectedModule.value && overview.value.module_order.length) {
      selectedModule.value = overview.value.module_order[0]
    }
  } finally {
    loading.value = false
  }
}

async function loadNetwork() {
  if (!store.jobId) return
  networkLoading.value = true
  try {
    await runOverviewNetwork(store.jobId, percentile.value)
    network.value = await getOverviewNetwork(store.jobId, 300)
  } finally {
    networkLoading.value = false
  }
}

async function runClustering() {
  if (!store.jobId || clusterRunning.value) return
  clusterRunning.value = true
  try {
    await runCluster(store.jobId, clusterMethod.value, clusterK.value, clusterSeed.value)
    // poll until done
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

// ---- network ECharts (limited preview) ----
const netChart = ref<echarts.ECharts | null>(null)
const netEl = ref<HTMLElement | null>(null)
const showNetworkGraph = ref(false)

function renderNetwork() {
  if (!netEl.value || !network.value) return
  if (!netChart.value) netChart.value = echarts.init(netEl.value)
  // limit nodes by degree (top 1500) + edges among them
  const nodes = [...network.value.nodes].sort((a, b) => b.degree - a.degree).slice(0, 1500)
  const ids = new Set(nodes.map((n) => n.id))
  const edges = network.value.edges.filter((e) => ids.has(e.source) && ids.has(e.target)).slice(0, 8000)
  const cat = [...new Set(nodes.map((n) => n.color))]
  netChart.value.setOption({
    animation: false,
    series: [{
      type: 'graphGL',
      layout: 'force',
      force: { repulsion: 80, edgeLength: 20, gravity: 0.1 },
      roam: true,
      data: nodes.map((n) => ({ id: n.id, name: n.id, symbolSize: Math.max(2, Math.sqrt(n.degree) * 0.6), itemStyle: { color: n.color }, category: cat.indexOf(n.color) })),
      links: edges.map((e) => ({ source: e.source, target: e.target, lineStyle: { opacity: 0.15, width: 0.4 } })),
      categories: cat.map((c) => ({ name: c, itemStyle: { color: c } })),
      label: { show: false },
      lineStyle: { color: '#b0bec5', curveness: 0 },
    }],
  })
  showNetworkGraph.value = true
}

watch([enrichType, topN], loadOverview)
onMounted(() => {
  if (store.jobId) loadOverview()
})
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <h6 class="mb-2"><i class="bi bi-grid-1x2 me-1"></i>{{ t('mo.title') }}</h6>

      <div class="mb-3">
        <label class="form-label small mb-1">{{ t('mo.enrichType') }}</label>
        <select v-model="enrichType" class="form-select form-select-sm">
          <option value="go">GO</option>
          <option value="kegg">KEGG</option>
        </select>
      </div>
      <div class="mb-3">
        <label class="form-label small mb-1">{{ t('mo.topN') }}</label>
        <input type="number" v-model.number="topN" class="form-control form-control-sm" min="1" max="30" />
      </div>
      <div class="mb-3">
        <label class="form-label small mb-1">{{ t('mo.percentile') }} (%)</label>
        <input type="number" v-model.number="percentile" class="form-control form-control-sm" min="1" max="100" />
      </div>

      <hr />
      <div class="mb-2">
        <label class="form-label small mb-1">{{ t('mo.clusterMethod') }}</label>
        <select v-model="clusterMethod" class="form-select form-select-sm">
          <option value="kmeans">KMeans</option>
          <option value="cmeans">{{ t('mo.cmeans') }}</option>
        </select>
      </div>
      <div class="mb-2">
        <label class="form-label small mb-1">{{ t('mo.k') }} (0 = auto)</label>
        <input type="number" v-model.number="clusterK" class="form-control form-control-sm" min="0" max="50" />
      </div>
      <div class="mb-3">
        <label class="form-label small mb-1">{{ t('mo.seed') }}</label>
        <input type="number" v-model.number="clusterSeed" class="form-control form-control-sm" min="0" />
      </div>
      <button class="btn btn-sm btn-primary w-100" :disabled="clusterRunning" @click="runClustering">
        <span v-if="clusterRunning" class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.runCluster') }}
      </button>
    </template>

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
        <div v-if="loading" class="text-center text-muted py-5"><div class="spinner-border"></div></div>
        <div v-else-if="overview">
          <div v-if="!overview.enrich_ok" class="alert alert-warning py-2 small">{{ t('mo.enrichMissing') }}</div>
          <div class="border rounded p-2 bg-white">
            <img :src="overviewUrl" class="w-100" alt="module overview" />
          </div>
          <div class="mt-2 d-flex gap-2">
            <a :href="overviewExportUrlPng" class="btn btn-sm btn-outline-primary"><i class="bi bi-download me-1"></i>SVG</a>
          </div>
        </div>
        <div v-else class="text-center text-muted py-5">{{ t('mo.noData') }}</div>
      </div>

      <!-- network -->
      <div v-if="tab === 'network'">
        <div class="mb-2 d-flex gap-2 align-items-center">
          <button class="btn btn-sm btn-primary" :disabled="networkLoading" @click="loadNetwork">
            <span v-if="networkLoading" class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.buildNetwork') }}
          </button>
          <button v-if="network" class="btn btn-sm btn-outline-secondary" @click="renderNetwork">{{ t('mo.renderNetwork') }}</button>
          <a v-if="network" :href="networkExportUrl" class="btn btn-sm btn-outline-primary"><i class="bi bi-download me-1"></i>SVG</a>
        </div>
        <div v-if="network" class="small text-muted mb-2">
          {{ network.nodes.length }} {{ t('mo.nodes') }} · {{ network.edges.length }} {{ t('mo.edges') }}
        </div>
        <div v-if="showNetworkGraph" ref="netEl" style="height: 560px" class="border rounded bg-white"></div>
        <div v-else class="text-center text-muted py-5">{{ t('mo.networkHint') }}</div>
      </div>

      <!-- cluster -->
      <div v-if="tab === 'cluster'">
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
          <div v-if="cluster.metrics?.wss?.length" class="small text-muted">
            WSS: {{ cluster.metrics.wss.map((x) => Math.round(x)).join(', ') }}
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
  </SidebarDrawer>
</template>
