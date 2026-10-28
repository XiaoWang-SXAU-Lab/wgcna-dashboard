<script setup lang="ts">
import { t } from '../../utils/i18n'

export type Tab = 'overview' | 'network' | 'cluster' | 'table'

const tab = defineModel<Tab>('tab', { required: true })

// overview
const enrichType = defineModel<'go' | 'kegg'>('enrichType', { required: true })
const onto = defineModel<string>('onto', { required: true })
const topN = defineModel<number>('topN', { required: true })
const maxCountOn = defineModel<boolean>('maxCountOn', { required: true })
const lowColor = defineModel<string>('lowColor', { required: true })
const midColor = defineModel<string>('midColor', { required: true })
const highColor = defineModel<string>('highColor', { required: true })

// network
const networkMode = defineModel<'intra' | 'inter' | 'combined'>('networkMode', { required: true })
const percentile = defineModel<number>('percentile', { required: true })
const maxNodes = defineModel<number>('maxNodes', { required: true })
const intraPerGene = defineModel<number>('intraPerGene', { required: true })
const interPerPair = defineModel<number>('interPerPair', { required: true })
const interBudget = defineModel<number>('interBudget', { required: true })
const hideGrey = defineModel<boolean>('hideGrey', { required: true })
const hideIsolated = defineModel<boolean>('hideIsolated', { required: true })
const hubPerModule = defineModel<number>('hubPerModule', { required: true })
const spread = defineModel<number>('spread', { required: true })
const showLabels = defineModel<boolean>('showLabels', { required: true })
const showModuleLabels = defineModel<boolean>('showModuleLabels', { required: true })
const nodeScale = defineModel<number>('nodeScale', { required: true })
const edgeOpacity = defineModel<number>('edgeOpacity', { required: true })
const networkDirty = defineModel<boolean>('networkDirty', { required: true })

// cluster
const clusterMethod = defineModel<'kmeans' | 'cmeans'>('clusterMethod', { required: true })
const clusterK = defineModel<number>('clusterK', { required: true })
const clusterSeed = defineModel<number>('clusterSeed', { required: true })
const clusterKmax = defineModel<number>('clusterKmax', { required: true })
const clusterEnrichType = defineModel<'go' | 'kegg'>('clusterEnrichType', { required: true })
const clusterOnto = defineModel<string>('clusterOnto', { required: true })

// table
const tableMode = defineModel<'modules' | 'clusters'>('tableMode', { required: true })
const selectedModule = defineModel<string>('selectedModule', { required: true })
const selectedCluster = defineModel<string>('selectedCluster', { required: true })
const geneSearch = defineModel<string>('geneSearch', { required: true })

defineProps<{
  moduleOptions: string[]
  clusterOptions: string[]
  networkDirtyLabel?: string
}>()

const emit = defineEmits<{
  (e: 'build-network'): void
  (e: 'compute-elbow'): void
  (e: 'run-clustering'): void
  (e: 'reset-network-settings'): void
}>()
</script>

<template>
  <div class="h-100 d-flex flex-column">
    <!-- tab switcher -->
    <div class="nav nav-pills flex-column gap-1 mb-3">
      <button v-for="(t2, key) in (['overview','network','cluster','table'] as Tab[])" :key="key"
        class="nav-link text-start" :class="{ active: tab === t2 }" @click="tab = t2">
        <i class="bi me-1" :class="{ overview: 'bi-grid-1x2', network: 'bi-diagram-3', cluster: 'bi-bounding-box-circles', table: 'bi-table' }[t2]"></i>
        {{ t('mo.' + t2) }}
      </button>
    </div>

    <div class="flex-grow-1 overflow-auto">
      <!-- Overview -->
      <template v-if="tab === 'overview'">
        <div class="mb-2 fw-semibold small text-muted text-uppercase">{{ t('mo.enrichment') }}</div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.enrichType') }}</label>
          <select v-model="enrichType" class="form-select form-select-sm">
            <option value="go">GO</option>
            <option value="kegg">KEGG</option>
          </select>
        </div>
        <div v-if="enrichType === 'go'" class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.ontology') }}</label>
          <select v-model="onto" class="form-select form-select-sm">
            <option value="">ALL</option>
            <option value="bp">BP</option>
            <option value="cc">CC</option>
            <option value="mf">MF</option>
          </select>
        </div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.topN') }}</label>
          <input type="number" v-model.number="topN" class="form-control form-control-sm" min="1" max="30" />
        </div>
        <div class="form-check mb-3">
          <input type="checkbox" v-model="maxCountOn" class="form-check-input" id="ovMaxCount" />
          <label class="form-check-label small" for="ovMaxCount">{{ t('mo.dropBroadTerms') }}</label>
        </div>
        <div class="mb-2 fw-semibold small text-muted text-uppercase">{{ t('mo.heatmapColors') }}</div>
        <div class="d-flex gap-2 align-items-center mb-3">
          <span class="small text-muted">low</span><input type="color" v-model="lowColor" class="form-control form-control-color form-control-sm" />
          <span class="small text-muted">mid</span><input type="color" v-model="midColor" class="form-control form-control-color form-control-sm" />
          <span class="small text-muted">high</span><input type="color" v-model="highColor" class="form-control form-control-color form-control-sm" />
        </div>
      </template>

      <!-- Network -->
      <template v-else-if="tab === 'network'">
        <div class="mb-2 fw-semibold small text-muted text-uppercase">{{ t('mo.networkConstruction') }}</div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.networkMode') }}</label>
          <select v-model="networkMode" class="form-select form-select-sm">
            <option value="intra">{{ t('mo.modeIntra') }}</option>
            <option value="inter">{{ t('mo.modeInter') }}</option>
            <option value="combined">{{ t('mo.modeCombined') }}</option>
          </select>
        </div>
        <div v-if="networkMode !== 'intra'" class="alert alert-warning py-1 px-2 small mb-2">{{ t('mo.interModeHint') }}</div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.percentile') }} (%)</label>
          <input type="number" v-model.number="percentile" class="form-control form-control-sm" min="1" max="100" />
        </div>
        <button class="btn btn-sm w-100 mb-3" :class="networkDirty ? 'btn-warning' : 'btn-primary'" @click="emit('build-network')">
          {{ networkDirty ? t('mo.rebuildNetwork') : t('mo.buildNetwork') }}
        </button>

        <div class="mb-2 fw-semibold small text-muted text-uppercase">{{ t('mo.graphFiltering') }}</div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.maxNodes') }}</label>
          <input type="number" v-model.number="maxNodes" class="form-control form-control-sm" min="100" max="8000" />
        </div>
        <div v-if="networkMode === 'intra' || networkMode === 'combined'" class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.intraPerGene') }}</label>
          <input type="number" v-model.number="intraPerGene" class="form-control form-control-sm" min="1" max="50" />
        </div>
        <div v-if="networkMode === 'inter' || networkMode === 'combined'" class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.interPerPair') }}</label>
          <input type="number" v-model.number="interPerPair" class="form-control form-control-sm" min="1" max="100" />
        </div>
        <div v-if="networkMode === 'inter' || networkMode === 'combined'" class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.interBudget') }}</label>
          <input type="number" v-model.number="interBudget" class="form-control form-control-sm" min="10" max="5000" />
        </div>
        <div class="form-check mb-1">
          <input type="checkbox" v-model="hideGrey" class="form-check-input" id="netGrey" />
          <label class="form-check-label small" for="netGrey">{{ t('mo.hideGrey') }}</label>
        </div>
        <div class="form-check mb-3">
          <input type="checkbox" v-model="hideIsolated" class="form-check-input" id="netIsolated" />
          <label class="form-check-label small" for="netIsolated">{{ t('mo.hideIsolated') }}</label>
        </div>

        <div class="mb-2 fw-semibold small text-muted text-uppercase">{{ t('mo.layout') }}</div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.hubPerModule') }}</label>
          <input type="number" v-model.number="hubPerModule" class="form-control form-control-sm" min="1" max="50" />
        </div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.spread') }}: <b>{{ spread.toFixed(1) }}</b></label>
          <input type="range" v-model.number="spread" class="form-range" min="0.5" max="2.5" step="0.1" />
        </div>

        <div class="mb-2 fw-semibold small text-muted text-uppercase">{{ t('mo.visual') }}</div>
        <div class="form-check mb-1">
          <input type="checkbox" v-model="showLabels" class="form-check-input" id="netLabels" />
          <label class="form-check-label small" for="netLabels">{{ t('mo.showGeneLabels') }}</label>
        </div>
        <div class="form-check mb-1">
          <input type="checkbox" v-model="showModuleLabels" class="form-check-input" id="netModLabels" />
          <label class="form-check-label small" for="netModLabels">{{ t('mo.showModuleLabels') }}</label>
        </div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.nodeScale') }}</label>
          <input type="range" v-model.number="nodeScale" class="form-range" min="0.5" max="2" step="0.1" />
        </div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.edgeOpacity') }}</label>
          <input type="range" v-model.number="edgeOpacity" class="form-range" min="0.03" max="0.5" step="0.01" />
        </div>
        <button class="btn btn-sm btn-outline-secondary w-100" @click="emit('reset-network-settings')">{{ t('mo.resetSettings') }}</button>
      </template>

      <!-- Clustering -->
      <template v-else-if="tab === 'cluster'">
        <div class="mb-2 fw-semibold small text-muted text-uppercase">{{ t('mo.clustering') }}</div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.clusterMethod') }}</label>
          <select v-model="clusterMethod" class="form-select form-select-sm">
            <option value="kmeans">KMeans</option>
            <option value="cmeans">{{ t('mo.cmeans') }}</option>
          </select>
        </div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.seed') }}</label>
          <input type="number" v-model.number="clusterSeed" class="form-control form-control-sm" min="0" />
        </div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.kmax') }}</label>
          <input type="number" v-model.number="clusterKmax" class="form-control form-control-sm" min="3" max="30" />
        </div>
        <button class="btn btn-sm btn-outline-primary w-100 mb-2" @click="emit('compute-elbow')">{{ t('mo.computeElbow') }}</button>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.k') }}</label>
          <input type="number" v-model.number="clusterK" class="form-control form-control-sm" min="2" :max="clusterKmax" />
        </div>
        <button class="btn btn-sm btn-primary w-100 mb-3" @click="emit('run-clustering')">{{ t('mo.runCluster') }}</button>

        <div class="mb-2 fw-semibold small text-muted text-uppercase">{{ t('mo.enrichment') }}</div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.enrichType') }}</label>
          <select v-model="clusterEnrichType" class="form-select form-select-sm">
            <option value="go">GO</option>
            <option value="kegg">KEGG</option>
          </select>
        </div>
        <div v-if="clusterEnrichType === 'go'" class="mb-3">
          <label class="form-label small mb-1">{{ t('mo.ontology') }}</label>
          <select v-model="clusterOnto" class="form-select form-select-sm">
            <option value="">ALL</option>
            <option value="bp">BP</option>
            <option value="cc">CC</option>
            <option value="mf">MF</option>
          </select>
        </div>
      </template>

      <!-- Table -->
      <template v-else>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.dataSource') }}</label>
          <select v-model="tableMode" class="form-select form-select-sm">
            <option value="modules">{{ t('mo.modules') }}</option>
            <option value="clusters">{{ t('mo.clusters') }}</option>
          </select>
        </div>
        <div v-if="tableMode === 'modules'" class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.modules') }}</label>
          <select v-model="selectedModule" class="form-select form-select-sm">
            <option v-for="m in moduleOptions" :key="m" :value="m">{{ m }}</option>
          </select>
        </div>
        <div v-else class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.clusters') }}</label>
          <select v-model="selectedCluster" class="form-select form-select-sm">
            <option v-for="c in clusterOptions" :key="c" :value="c">{{ c }}</option>
          </select>
        </div>
        <div class="mb-2">
          <label class="form-label small mb-1">{{ t('mo.geneSearch') }}</label>
          <input type="text" v-model="geneSearch" class="form-control form-control-sm" :placeholder="t('mo.geneSearchPlaceholder')" />
        </div>
      </template>
    </div>
  </div>
</template>
