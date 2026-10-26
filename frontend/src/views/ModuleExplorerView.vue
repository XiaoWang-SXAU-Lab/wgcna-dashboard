<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import GsMmScatter from '../components/GsMmScatter.vue'
import ModuleGeneHeatmap from '../components/ModuleGeneHeatmap.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
import EnrichmentPanel from '../components/EnrichmentPanel.vue'
import SimpleTable from '../components/SimpleTable.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'
import type { NetLayout, SizeBasis, LabelMode, NodeColorMode } from '../components/NetworkGraph.vue'
import { getAnnotation, type AnnotationRow } from '../api/client'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import StepStatusBadge from '../components/StepStatusBadge.vue'
import { t } from '../utils/i18n'

type Tab = 'detail' | 'network' | 'annotation' | 'go' | 'kegg'

const store = useWgcnaStore()
const tab = ref<Tab>('detail')

const traitOptions = computed(() => store.mtait?.traits ?? [])
const moduleOptions = computed(() => {
  const sizes = store.modules?.module_sizes ?? {}
  return Object.keys(sizes).sort((a, b) => (sizes[b] ?? 0) - (sizes[a] ?? 0))
})

// --- network visualization params ---
const layout = ref<NetLayout>('fcose')
const sizeBasis = ref<SizeBasis>('degree')
const nodeSizeMin = ref(5)
const nodeSizeMax = ref(30)
const edgeLowColor = ref('#cbd5e1')
const edgeHighColor = ref('#e11d48')
const edgeWidth = ref(6)
const labelSize = ref(10)
const labelMode = ref<LabelMode>('hub')
const colorMode = ref<NodeColorMode>('kme')
const nodeLowColor = ref('#dbeafe')
const nodeHighColor = ref('#1d4ed8')
const hubNodeColor = ref('#e11d48')
const hubBorderColor = ref('#0f172a')
const hubBorderWidth = ref(3)

const gradientColor = computed(() => ['kme', 'gs', 'degree'].includes(colorMode.value))
const netNodeCount = computed(() => store.hub?.network.nodes.length ?? 0)
const netEdgeCount = computed(() => store.hub?.network.edges.length ?? 0)
const largeNetwork = computed(() => netNodeCount.value > 1200 || netEdgeCount.value > 4000)

const geneMap = ref<Record<string, string>>({})
const showMap = ref(false)
const showExportGsmm = ref(false)
const showExportHeatmap = ref(false)

const annotationRows = ref<AnnotationRow[]>([])
const annotationLoading = ref(false)

// gene_id -> annotation (from uploaded annotation.tsv)
const annMap = computed(() => {
  const m: Record<string, AnnotationRow> = {}
  for (const r of annotationRows.value) m[r.gene_id] = r
  return m
})

// gene -> kME/GS (from module_detail result for the selected module+trait)
const detailMap = computed(() => {
  const m: Record<string, { kme: number; gs: number }> = {}
  const d = store.detail
  if (d) d.genes.forEach((g, i) => { m[g] = { kme: d.mm[i] ?? 0, gs: d.gs[i] ?? 0 } })
  return m
})

const hubRows = computed(() => (store.hub?.hub_genes ?? []).map((h) => {
  const a = annMap.value[h.gene]
  return {
    gene: h.gene,
    symbol: a?.symbol || '',
    gene_name: a?.gene_name || '',
    description: a?.description || '',
    kme: h.kme.toFixed(4),
    gs: h.gs.toFixed(4),
  }
}))

const annotationRowsDisplay = computed(() => annotationRows.value.map((r) => {
  const d = detailMap.value[r.gene_id]
  return {
    ...r,
    kme: d ? d.kme.toFixed(4) : '',
    gs: d ? d.gs.toFixed(4) : '',
  }
}))

function onTraitChange() {
  if (!store.selectedTrait) return
  store.runStep('module_detail')
}

function onModuleChange() {
  if (!store.selectedModule) return
  store.runStep('module_detail')
}

async function loadAnnotation() {
  if (!store.jobId || !store.selectedModule) return
  annotationLoading.value = true
  try {
    annotationRows.value = await getAnnotation(store.jobId, store.selectedModule)
    // 自动用 symbol 填充网络节点的标签（用户手动填过的保留）。
    for (const r of annotationRows.value) {
      if (r.symbol && !geneMap.value[r.gene_id]) {
        geneMap.value[r.gene_id] = r.symbol
      }
    }
  } catch {
    annotationRows.value = []
  } finally {
    annotationLoading.value = false
  }
}

watch(() => store.selectedModule, (m) => {
  if (m) loadAnnotation()
})
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div class="d-flex flex-column gap-2">
        <label v-if="traitOptions.length" class="form-label small mb-0">{{ t('im.trait') }}</label>
        <select v-if="traitOptions.length" v-model="store.selectedTrait" @change="onTraitChange" class="form-select form-select-sm">
          <option v-for="tr in traitOptions" :key="tr" :value="tr">{{ tr }}</option>
        </select>
        <label v-if="moduleOptions.length" class="form-label small mb-0">{{ t('im.module') }}</label>
        <select v-if="moduleOptions.length" v-model="store.selectedModule" @change="onModuleChange" class="form-select form-select-sm">
          <option v-for="m in moduleOptions" :key="m" :value="m">{{ m }}</option>
        </select>
        <button class="btn btn-primary w-100" :disabled="!store.stepReady('module_detail') || store.stepRunning !== ''" @click="store.runStep('module_detail')">
          {{ store.stepRunning === 'module_detail' ? t('common.running') : store.isDone('module_detail') ? t('common.rerun') : t('common.run') }}
        </button>

        <template v-if="tab === 'network'">
          <hr class="my-1" />
          <span class="text-muted small fw-semibold">{{ t('hub.filter') }}</span>
          <label class="form-label small mb-0">{{ t('hub.kmeCutoff') }}: <b>{{ store.kmeCutoff.toFixed(2) }}</b></label>
          <input type="range" class="form-range" min="0" max="1" step="0.01" v-model.number="store.kmeCutoff" />
          <label class="form-label small mb-0">{{ t('hub.gsCutoff') }}: <b>{{ store.gsCutoff.toFixed(2) }}</b></label>
          <input type="range" class="form-range" min="0" max="1" step="0.01" v-model.number="store.gsCutoff" />
          <label class="form-label small mb-0">{{ t('hub.threshold') }}: <b>{{ store.weightThreshold.toFixed(2) }}</b></label>
          <input type="range" class="form-range" min="0" max="0.5" step="0.005" v-model.number="store.weightThreshold" />
          <label class="form-label small mb-0">{{ t('hub.maxHubs') }}: <b>{{ store.maxHubs }}</b></label>
          <input type="number" class="form-control form-control-sm" min="1" max="200" v-model.number="store.maxHubs" />
          <label class="form-label small mb-0">{{ t('hub.topNNeighbors') }}: <b>{{ store.topNNeighbors }}</b></label>
          <input type="number" class="form-control form-control-sm" min="1" max="100" v-model.number="store.topNNeighbors" />

          <hr class="my-1" />
          <span class="text-muted small fw-semibold">{{ t('hub.style') }}</span>
          <label class="form-label small mb-0">{{ t('hub.layout') }}</label>
          <select v-model="layout" class="form-select form-select-sm">
            <option value="fcose">ForceAtlas2 (Gephi)</option>
            <option value="cose">Fruchterman-Reingold</option>
            <option value="radial">Hub-and-spoke radial</option>
            <option value="circle">Circular</option>
            <option value="concentric">Concentric</option>
            <option value="breadthfirst">Hierarchy</option>
          </select>
          <label class="form-label small mb-0">{{ t('hub.sizeBasis') }}</label>
          <select v-model="sizeBasis" class="form-select form-select-sm">
            <option value="kme">kME</option>
            <option value="degree">degree</option>
            <option value="gs">GS</option>
            <option value="fixed">fixed</option>
          </select>
          <label class="form-label small mb-0">{{ t('hub.nodeSizeMin') }}: <b>{{ nodeSizeMin }}</b></label>
          <input type="range" class="form-range" min="3" max="40" step="1" v-model.number="nodeSizeMin" />
          <label class="form-label small mb-0">{{ t('hub.nodeSizeMax') }}: <b>{{ nodeSizeMax }}</b></label>
          <input type="range" class="form-range" min="10" max="80" step="1" v-model.number="nodeSizeMax" />
          <div class="row g-2">
            <div class="col-6">
              <label class="form-label small mb-0">{{ t('hub.edgeLowColor') }}</label>
              <input type="color" class="form-control form-control-color" v-model="edgeLowColor" />
            </div>
            <div class="col-6">
              <label class="form-label small mb-0">{{ t('hub.edgeHighColor') }}</label>
              <input type="color" class="form-control form-control-color" v-model="edgeHighColor" />
            </div>
          </div>
          <label class="form-label small mb-0">{{ t('hub.edgeWidth') }}: <b>{{ edgeWidth }}</b></label>
          <input type="range" class="form-range" min="1" max="16" step="1" v-model.number="edgeWidth" />
          <label class="form-label small mb-0">{{ t('hub.labelSize') }}: <b>{{ labelSize }}</b></label>
          <input type="range" class="form-range" min="6" max="20" step="1" v-model.number="labelSize" />
          <label class="form-label small mb-0">{{ t('hub.labelMode') }}</label>
          <select v-model="labelMode" class="form-select form-select-sm">
            <option value="hub">{{ t('hub.labelHub') }}</option>
            <option value="all">{{ t('hub.labelAll') }}</option>
            <option value="none">{{ t('hub.labelNone') }}</option>
          </select>
          <label class="form-label small mb-0">{{ t('hub.nodeColor') }}</label>
          <select v-model="colorMode" class="form-select form-select-sm">
            <option value="kme">{{ t('hub.nodeColorKme') }}</option>
            <option value="gs">{{ t('hub.nodeColorGs') }}</option>
            <option value="degree">{{ t('hub.nodeColorDegree') }}</option>
            <option value="hub">{{ t('hub.nodeColorHub') }}</option>
            <option value="module">{{ t('hub.nodeColorModule') }}</option>
          </select>
          <div v-if="gradientColor" class="row g-2">
            <div class="col-6">
              <label class="form-label small mb-0">{{ t('hub.nodeLowColor') }}</label>
              <input type="color" class="form-control form-control-color" v-model="nodeLowColor" />
            </div>
            <div class="col-6">
              <label class="form-label small mb-0">{{ t('hub.nodeHighColor') }}</label>
              <input type="color" class="form-control form-control-color" v-model="nodeHighColor" />
            </div>
          </div>
          <div v-if="colorMode === 'hub'" class="row g-2">
            <div class="col-6">
              <label class="form-label small mb-0">{{ t('hub.hubNodeColor') }}</label>
              <input type="color" class="form-control form-control-color" v-model="hubNodeColor" />
            </div>
          </div>
          <div class="row g-2">
            <div class="col-6">
              <label class="form-label small mb-0">{{ t('hub.hubBorderColor') }}</label>
              <input type="color" class="form-control form-control-color" v-model="hubBorderColor" />
            </div>
            <div class="col-6">
              <label class="form-label small mb-0">{{ t('hub.hubBorderWidth') }}</label>
              <input type="number" class="form-control form-control-sm" min="1" max="8" v-model.number="hubBorderWidth" />
            </div>
          </div>

          <hr class="my-1" />
          <button class="btn btn-sm btn-outline-secondary" @click="showMap = !showMap">
            <i class="bi bi-tags me-1"></i>{{ t('hub.geneMap') }}
          </button>
          <div v-if="showMap" class="border rounded p-2" style="max-height: 200px; overflow: auto">
            <div v-for="h in store.hub?.hub_genes ?? []" :key="h.gene" class="d-flex align-items-center gap-2 mb-1">
              <code class="small flex-shrink-0" style="max-width: 120px; overflow: hidden; text-overflow: ellipsis">{{ h.gene }}</code>
              <input v-model="geneMap[h.gene]" class="form-control form-control-sm" :placeholder="t('hub.labelPlaceholder')" />
            </div>
          </div>

          <button class="btn btn-primary w-100" :disabled="!store.stepReady('hub') || store.stepRunning !== ''" @click="store.runStep('hub')">
            {{ store.stepRunning === 'hub' ? t('common.running') : store.isDone('hub') ? t('common.rerun') : t('common.run') }}
          </button>
        </template>
      </div>
    </template>

    <div class="mb-3">
      <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('me.title') }} <StepStatusBadge step="module_detail" /></h4>
      <p class="text-muted small mb-0">{{ t('me.desc') }}</p>
    </div>
    <div v-if="store.error" class="alert alert-danger d-flex align-items-center gap-2 py-2">
      <i class="bi bi-exclamation-triangle-fill"></i>
      <span class="flex-grow-1">{{ store.error }}</span>
      <button class="btn btn-sm btn-outline-danger" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button>
    </div>

    <ul class="nav nav-tabs mb-3">
      <li class="nav-item"><a class="nav-link" :class="{ active: tab === 'detail' }" @click="tab = 'detail'">{{ t('me.tabDetail') }}</a></li>
      <li class="nav-item"><a class="nav-link" :class="{ active: tab === 'network' }" @click="tab = 'network'">{{ t('me.tabNetwork') }}</a></li>
      <li class="nav-item"><a class="nav-link" :class="{ active: tab === 'annotation' }" @click="tab = 'annotation'">{{ t('me.tabAnnotation') }}</a></li>
      <li class="nav-item"><a class="nav-link" :class="{ active: tab === 'go' }" @click="tab = 'go'">{{ t('me.tabGo') }}</a></li>
      <li class="nav-item"><a class="nav-link" :class="{ active: tab === 'kegg' }" @click="tab = 'kegg'">{{ t('me.tabKegg') }}</a></li>
    </ul>

    <template v-if="tab === 'detail'">
      <div v-if="store.detail">
        <div class="d-flex gap-2 mb-2">
          <button class="btn btn-sm btn-outline-primary" @click="showExportGsmm = true"><i class="bi bi-download me-1"></i>{{ t('im.exportGsmm') }}</button>
          <button class="btn btn-sm btn-outline-primary" @click="showExportHeatmap = true"><i class="bi bi-download me-1"></i>{{ t('im.exportHeatmap') }}</button>
        </div>
        <div class="row g-3">
          <div class="col-lg-6"><GsMmScatter :data="store.detail" /></div>
          <div class="col-lg-6"><ModuleGeneHeatmap :data="store.detail" /></div>
        </div>
        <PlotDownloadWidget kind="gsmm" :show="showExportGsmm" @close="showExportGsmm = false" />
        <PlotDownloadWidget kind="geneheatmap" :show="showExportHeatmap" @close="showExportHeatmap = false" />
      </div>
      <p v-else class="text-muted">{{ t('im.empty') }}</p>
    </template>

    <template v-else-if="tab === 'network'">
      <div v-if="store.hub">
        <p class="text-muted small mb-1">{{ store.hub.module }} × {{ store.hub.trait_name }} · {{ store.hub.hub_genes.length }} {{ t('hub.count') }}</p>
        <p class="text-muted small">{{ t('hub.network') }}：{{ store.hub.network.nodes.length }} {{ t('hub.nodes') }} / {{ store.hub.network.edges.length }} {{ t('hub.edges') }}</p>
        <div v-if="largeNetwork" class="alert alert-warning py-2 small">
          <i class="bi bi-exclamation-triangle-fill me-1"></i>
          {{ t('hub.largeNetwork') }}（{{ netNodeCount }} {{ t('hub.nodes') }} / {{ netEdgeCount }} {{ t('hub.edges') }}）
        </div>
        <NetworkGraph
          :network="store.hub.network"
          :layout="layout"
          :size-basis="sizeBasis"
          :node-size-min="nodeSizeMin"
          :node-size-max="nodeSizeMax"
          :edge-low-color="edgeLowColor"
          :edge-high-color="edgeHighColor"
          :edge-width="edgeWidth"
          :label-size="labelSize"
          :label-mode="labelMode"
          :gene-map="geneMap"
          :color-mode="colorMode"
          :node-low-color="nodeLowColor"
          :node-high-color="nodeHighColor"
          :hub-node-color="hubNodeColor"
          :hub-border-color="hubBorderColor"
          :hub-border-width="hubBorderWidth"
        />
        <SimpleTable class="mt-3" :columns="[
          { key: 'gene', label: t('hub.col.gene') },
          { key: 'symbol', label: 'Symbol' },
          { key: 'gene_name', label: 'Gene name' },
          { key: 'description', label: 'Description' },
          { key: 'kme', label: t('hub.col.kme') },
          { key: 'gs', label: t('hub.col.gs') },
        ]" :rows="hubRows" filename="hub_gene" />
      </div>
      <p v-else class="text-muted">{{ t('hub.empty') }}</p>
    </template>

    <template v-else-if="tab === 'annotation'">
      <div v-if="annotationLoading" class="text-muted small">{{ t('common.running') }}</div>
      <SimpleTable v-else :columns="[
        { key: 'gene_id', label: 'Gene ID' },
        { key: 'symbol', label: 'Symbol' },
        { key: 'gene_name', label: 'Gene name' },
        { key: 'description', label: 'Description' },
        { key: 'kme', label: 'kME' },
        { key: 'gs', label: 'GS' },
      ]" :rows="annotationRowsDisplay" scroll-height="560px" filename="module_annotation" />
    </template>

    <template v-else-if="tab === 'go'">
      <EnrichmentPanel type="go" />
    </template>

    <template v-else>
      <EnrichmentPanel type="kegg" />
    </template>
  </SidebarDrawer>
</template>
