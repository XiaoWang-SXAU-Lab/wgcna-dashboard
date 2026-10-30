<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { waitForRun } from '../utils/pollJob'
import { useWgcnaStore } from '../stores/wgcna'
import { t } from '../utils/i18n'
import {
  getOverview, getOverviewNetwork, runCluster, getCluster, runOverviewNetwork, overviewExportUrl, plotUrl,
  runClusterElbow, getClusterElbow,
  type OverviewData, type OverviewNetwork, type ClusterResult, type ClusterElbow,
} from '../api/client'
import * as echarts from 'echarts'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import OverviewSettingsSidebar, { type Tab } from '../components/overview/OverviewSettingsSidebar.vue'
import OverviewNetworkPanel from '../components/overview/OverviewNetworkPanel.vue'

const store = useWgcnaStore()

const tab = ref<Tab>('overview')

// ---- overview settings ----
const enrichType = ref<'go' | 'kegg'>('go')
const onto = ref('')
const topN = ref(5)
const maxCountOn = ref(true)
const lowColor = ref('#08519c')
const midColor = ref('#ffffff')
const highColor = ref('#a50f15')

// ---- network settings ----
const networkMode = ref<'intra' | 'inter' | 'combined'>('intra')
const percentile = ref(10)
const maxNodes = ref(2000)
const minPerModule = ref(20)
const intraPerGene = ref(8)
const interPerPair = ref(20)
const interBudget = ref(300)
const hideGrey = ref(true)
const hideIsolated = ref(true)
const hubPerModule = ref(10)
const geneSpacing = ref(1.0)
const moduleSpacing = ref(1.0)
const showLabels = ref(false)
const showModuleLabels = ref(true)
const nodeScale = ref(1)
const edgeOpacity = ref(0.15)
const networkDirty = ref(false)

// ---- cluster settings ----
const clusterMethod = ref<'kmeans' | 'cmeans'>('kmeans')
const clusterK = ref(0)
const clusterSeed = ref(42)
const clusterKmax = ref(12)
const clusterEnrichType = ref<'go' | 'kegg'>('go')
const clusterOnto = ref('')

// ---- table settings ----
const tableMode = ref<'modules' | 'clusters'>('modules')
const selectedModule = ref('')
const selectedCluster = ref('')
const geneSearch = ref('')

// ---- data ----
const overview = ref<OverviewData | null>(null)
const network = ref<OverviewNetwork | null>(null)
const cluster = ref<ClusterResult | null>(null)
const loading = ref(false)
const networkLoading = ref(false)
const clusterRunning = ref(false)
const overviewImgLoading = ref(false)
const showOverviewImg = ref(false)
const overviewImgError = ref(false)
const clusterImgError = ref(false)
const clusterImgLoading = ref(false)
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

watch(overviewUrl, () => { overviewImgError.value = false })
watch(clusterOverviewUrl, () => { clusterImgError.value = false })
function plotFailed(clusterPlot: boolean) {
  if (clusterPlot) { clusterImgLoading.value = false; clusterImgError.value = true }
  else { overviewImgLoading.value = false; overviewImgError.value = true }
  store.notifyError(new Error('Plot could not be generated. Check plot settings and the R plotting environment, then retry.'))
}

async function loadOverview() {
  if (!store.jobId || !store.isDone('network')) return
  loading.value = true
  try {
    overview.value = await getOverview(store.jobId, enrichType.value, onto.value, 0.05, topN.value)
    if (!selectedModule.value && overview.value.module_order.length) {
      selectedModule.value = overview.value.module_order[0]
    }
  } catch (e) { store.notifyError(e, 'network') } finally {
    loading.value = false
  }
}

// fetch combined (both edge types) for inter/combined so the layout is shared
// across modes; intra mode fetches intra-only (light, no cross-edge processing).
function fetchMode(): 'intra' | 'combined' {
  return networkMode.value === 'intra' ? 'intra' : 'combined'
}

async function fetchNetwork() {
  if (!store.jobId || networkLoading.value) return
  networkLoading.value = true
  try {
    network.value = await getOverviewNetwork(store.jobId, fetchMode(), maxNodes.value, minPerModule.value, intraPerGene.value, interPerPair.value, interBudget.value, hideGrey.value)
  } catch (e) { store.notifyError(e, 'overview_network') } finally {
    networkLoading.value = false
  }
}

async function awaitAux(id: string, runId: string) {
  const status = await waitForRun(id, runId, status => { if (store.jobId === id) store.job = status }, lost => { store.connectionLost = lost })
  return status.status === 'done'
}
async function buildNetwork() {
  if (!store.jobId || store.stepRunning || networkLoading.value) return
  const id = store.jobId
  networkLoading.value = true
  store.stepRunning = 'overview_network'
  try {
    const { run_id } = await runOverviewNetwork(id, percentile.value)
    network.value = null
    if (await awaitAux(id, run_id)) {
      network.value = await getOverviewNetwork(id, fetchMode(), maxNodes.value, minPerModule.value, intraPerGene.value, interPerPair.value, interBudget.value, hideGrey.value)
      networkDirty.value = false
    }
  } catch (e) { store.notifyError(e, 'overview_network') } finally {
    store.connectionLost = false
    store.stepRunning = ''
    networkLoading.value = false
  }
}
async function runClustering() {
  if (!store.jobId || store.stepRunning || clusterRunning.value) return
  const id = store.jobId
  clusterRunning.value = true
  store.stepRunning = 'cluster'
  try {
    const { run_id } = await runCluster(id, clusterMethod.value, clusterK.value, clusterSeed.value)
    cluster.value = null
    if (await awaitAux(id, run_id)) {
      cluster.value = await getCluster(id)
      selectedCluster.value = cluster.value.cluster_order[0] ?? ''
    }
  } catch (e) { store.notifyError(e, 'cluster') } finally {
    store.connectionLost = false
    store.stepRunning = ''
    clusterRunning.value = false
  }
}
async function computeElbow() {
  if (!store.jobId || store.stepRunning || elbowRunning.value) return
  const id = store.jobId
  elbowRunning.value = true
  store.stepRunning = 'cluster_elbow'
  try {
    const { run_id } = await runClusterElbow(id, clusterMethod.value, clusterSeed.value, clusterKmax.value)
    elbow.value = null
    if (await awaitAux(id, run_id)) {
      elbow.value = await getClusterElbow(id)
      clusterK.value = elbow.value.suggested_k
      await nextTick()
      renderElbow()
    }
  } catch (e) { store.notifyError(e, 'cluster_elbow') } finally {
    store.connectionLost = false
    store.stepRunning = ''
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

function resetNetworkSettings() {
  networkMode.value = 'intra'
  percentile.value = 10
  maxNodes.value = 2000
  minPerModule.value = 20
  intraPerGene.value = 8
  interPerPair.value = 20
  interBudget.value = 300
  hideGrey.value = true
  hideIsolated.value = true
  hubPerModule.value = 10
  geneSpacing.value = 1.0
  moduleSpacing.value = 1.0
  showLabels.value = false
  showModuleLabels.value = true
  nodeScale.value = 1
  edgeOpacity.value = 0.15
}

// ---- table ----
const tableGenes = computed(() => {
  const q = geneSearch.value.trim().toLowerCase()
  let rows: { gene: string; group: string; kme: number | null }[] = []
  if (tableMode.value === 'modules' && overview.value) {
    const m = selectedModule.value
    rows = overview.value.genes
      .map((g, i) => ({ gene: g, group: overview.value!.modules[i], kme: overview.value!.kme[g] ?? null }))
      .filter((r) => r.group === m)
  } else if (tableMode.value === 'clusters' && cluster.value) {
    const c = selectedCluster.value
    rows = cluster.value.genes
      .map((g, i) => ({ gene: g, group: cluster.value!.clusters[i], kme: cluster.value!.score[i] ?? null }))
      .filter((r) => r.group === c)
  }
  if (q) rows = rows.filter((r) => r.gene.toLowerCase().includes(q))
  return rows
})

const xlsxUrl = computed(() => store.jobId ? overviewExportUrl(store.jobId, tableMode.value) : '#')

const moduleOptions = computed(() => overview.value?.module_order ?? [])
const clusterOptions = computed(() => cluster.value?.cluster_order ?? [])

// ---- watches ----
watch([enrichType, onto, topN, maxCountOn], () => {
  showOverviewImg.value = false
  overviewImgLoading.value = false
  loadOverview()
})
watch([lowColor, midColor, highColor], () => {
  showOverviewImg.value = false
  overviewImgLoading.value = false
})
watch([clusterEnrichType, clusterOnto, topN, maxCountOn, lowColor, midColor, highColor], () => {
  if (cluster.value) clusterImgLoading.value = true
})
// construction setting change → mark stale (no silent stale results)
watch(percentile, () => {
  if (network.value) networkDirty.value = true
})
// filtering/sampling setting change → re-fetch (re-sample) from existing files, debounced
let reSampleTimer: ReturnType<typeof setTimeout> | undefined
watch([networkMode, maxNodes, intraPerGene, interPerPair, interBudget, hideGrey], () => {
  if (!network.value) return
  clearTimeout(reSampleTimer)
  reSampleTimer = setTimeout(() => fetchNetwork(), 400)
})

onBeforeUnmount(() => { clearTimeout(reSampleTimer); elbowChart.value?.dispose() })

onMounted(() => {
  if (store.jobId) loadOverview()
})
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <OverviewSettingsSidebar
        v-model:tab="tab"
        v-model:enrich-type="enrichType" v-model:onto="onto" v-model:top-n="topN" v-model:max-count-on="maxCountOn"
        v-model:low-color="lowColor" v-model:mid-color="midColor" v-model:high-color="highColor"
        v-model:percentile="percentile" v-model:max-nodes="maxNodes" v-model:network-mode="networkMode"
        v-model:intra-per-gene="intraPerGene" v-model:inter-per-pair="interPerPair" v-model:inter-budget="interBudget"
        v-model:hide-grey="hideGrey" v-model:hide-isolated="hideIsolated" v-model:hub-per-module="hubPerModule"
        v-model:gene-spacing="geneSpacing" v-model:module-spacing="moduleSpacing"
        v-model:show-labels="showLabels" v-model:show-module-labels="showModuleLabels"
        v-model:node-scale="nodeScale" v-model:edge-opacity="edgeOpacity" v-model:network-dirty="networkDirty"
        v-model:cluster-method="clusterMethod" v-model:cluster-k="clusterK" v-model:cluster-seed="clusterSeed"
        v-model:cluster-kmax="clusterKmax" v-model:cluster-enrich-type="clusterEnrichType" v-model:cluster-onto="clusterOnto"
        v-model:table-mode="tableMode" v-model:selected-module="selectedModule" v-model:selected-cluster="selectedCluster"
        v-model:gene-search="geneSearch"
        :module-options="moduleOptions" :cluster-options="clusterOptions"
        @build-network="buildNetwork" @compute-elbow="computeElbow" @run-clustering="runClustering" @reset-network-settings="resetNetworkSettings"
      />
    </template>

    <div class="d-flex flex-column h-100">
      <div class="d-flex justify-content-end mb-2">
        <a :href="xlsxUrl" class="btn btn-sm btn-outline-success"><i class="bi bi-file-earmark-excel me-1"></i>{{ t('mo.downloadXlsx') }}</a>
      </div>
      <div class="flex-grow-1" style="min-height: 0">
        <!-- overview -->
        <div v-if="tab === 'overview'" class="h-100 overflow-auto">
          <div class="alert alert-light border small text-muted mb-3"><i class="bi bi-info-circle me-1"></i>{{ t('mo.overviewDesc') }}</div>
          <div v-if="loading" class="text-center text-muted py-5"><div class="spinner-border"></div></div>
          <div v-else-if="overview">
            <div v-if="!overview.enrich_ok" class="alert alert-warning py-2 small">{{ t('mo.enrichMissing') }}</div>
            <div v-if="!showOverviewImg" class="text-center py-5">
              <button class="btn btn-primary" @click="showOverviewImg = true; overviewImgError = false; overviewImgLoading = true">
                <i class="bi bi-play-circle me-1"></i>{{ t('mo.showFigure') }}
              </button>
              <div class="small text-muted mt-2">{{ t('mo.generating') }}</div>
            </div>
            <div v-else class="border rounded p-2 bg-white position-relative">
              <img v-if="!overviewImgError" :src="overviewUrl" class="w-100" alt="module overview" @load="overviewImgLoading = false" @error="plotFailed(false)" />
              <div v-if="overviewImgError" class="alert alert-warning mb-0">{{ t('common.error') }} <button class="btn btn-sm btn-outline-secondary" @click="overviewImgError = false; overviewImgLoading = true">{{ t('common.rerun') }}</button></div>
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
        <div v-else-if="tab === 'network'" class="h-100">
          <OverviewNetworkPanel
            :network="network" :loading="networkLoading" :network-mode="networkMode"
            :hub-per-module="hubPerModule" :gene-spacing="geneSpacing" :module-spacing="moduleSpacing"
            :hide-isolated="hideIsolated" :show-labels="showLabels" :show-module-labels="showModuleLabels"
            :node-scale="nodeScale" :edge-opacity="edgeOpacity"
            @build="buildNetwork" @export="() => {}"
          />
        </div>

        <!-- cluster -->
        <div v-else-if="tab === 'cluster'" class="h-100 overflow-auto">
          <div class="alert alert-light border small text-muted mb-3"><i class="bi bi-info-circle me-1"></i>{{ t('mo.clusterDesc') }}</div>
          <div v-if="elbow" class="border rounded p-3 bg-white mb-3">
            <div class="small text-muted mb-1">
              {{ clusterMethod === 'cmeans' ? t('mo.cmeansHint') : t('mo.elbowHint') }}
              <span class="badge text-bg-primary ms-1">k = {{ elbow.suggested_k }}</span>
            </div>
            <div ref="elbowEl" style="height: 260px"></div>
          </div>
          <div v-if="elbowRunning" class="text-center text-muted py-4"><div class="spinner-border"></div></div>

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
            <div class="border rounded p-2 bg-white mt-2 position-relative">
              <img v-if="!clusterImgError" :src="clusterOverviewUrl" class="w-100" alt="cluster overview" @load="clusterImgLoading = false" @error="plotFailed(true)" />
              <div v-if="clusterImgError" class="alert alert-warning mb-0">{{ t('common.error') }} <button class="btn btn-sm btn-outline-secondary" @click="clusterImgError = false; clusterImgLoading = true">{{ t('common.rerun') }}</button></div>
              <div v-if="clusterImgLoading" class="position-absolute top-0 start-0 w-100 h-100 d-flex flex-column align-items-center justify-content-center" style="background: rgba(255,255,255,0.72); z-index: 2">
                <div class="spinner-border text-primary"></div>
                <div class="small text-muted mt-2">{{ t('mo.generating') }}</div>
              </div>
            </div>
          </div>
          <div v-else class="text-center text-muted py-5">{{ t('mo.noData') }}</div>
        </div>

        <!-- table -->
        <div v-else class="h-100 overflow-auto">
          <div class="table-responsive border rounded bg-white" style="max-height: 100%">
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
          <div v-if="tableGenes.length === 0" class="text-center text-muted py-4 small">{{ t('mo.noData') }}</div>
        </div>
      </div>
    </div>
  </SidebarDrawer>
</template>
