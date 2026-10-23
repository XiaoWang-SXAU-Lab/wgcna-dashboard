<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import SimpleTable from '../components/SimpleTable.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
import type { NetLayout, SizeBasis, LabelMode } from '../components/NetworkGraph.vue'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'

const store = useWgcnaStore()

const traitOptions = computed(() => store.mtait?.traits ?? [])
const moduleOptions = computed(() => {
  const sizes = store.modules?.module_sizes ?? {}
  return Object.keys(sizes).sort((a, b) => (sizes[b] ?? 0) - (sizes[a] ?? 0))
})

// --- network visualization params ---
const layout = ref<NetLayout>('fcose')
const sizeBasis = ref<SizeBasis>('kme')
const nodeSizeMin = ref(5)
const nodeSizeMax = ref(30)
const edgeLowColor = ref('#cbd5e1')
const edgeHighColor = ref('#e11d48')
const edgeWidth = ref(6)
const labelSize = ref(10)
const labelMode = ref<LabelMode>('hub')

// --- gene label mapping ---
const geneMap = ref<Record<string, string>>({})
const showMap = ref(false)

const hubRows = computed(() => (store.hub?.hub_genes ?? []).map((h) => ({
  gene: h.gene, kme: h.kme.toFixed(4), gs: h.gs.toFixed(4),
})))
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div class="d-flex flex-column gap-2">
        <label v-if="traitOptions.length" class="form-label small mb-0">{{ t('im.trait') }}</label>
        <select v-if="traitOptions.length" v-model="store.selectedTrait" class="form-select form-select-sm">
          <option v-for="tr in traitOptions" :key="tr" :value="tr">{{ tr }}</option>
        </select>
        <label v-if="moduleOptions.length" class="form-label small mb-0">{{ t('im.module') }}</label>
        <select v-if="moduleOptions.length" v-model="store.selectedModule" class="form-select form-select-sm">
          <option v-for="m in moduleOptions" :key="m" :value="m">{{ m }}</option>
        </select>

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
          <option value="circle">Circular</option>
          <option value="concentric">Concentric</option>
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
      </div>
    </template>
      <div class="mb-3">
        <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('hub.title') }} <StepStatusBadge step="hub" /></h4>
        <p class="text-muted small mb-0">{{ t('hub.desc') }}</p>
      </div>
      <div v-if="store.error" class="alert alert-danger d-flex align-items-center gap-2 py-2">
        <i class="bi bi-exclamation-triangle-fill"></i>
        <span class="flex-grow-1">{{ store.error }}</span>
        <button class="btn btn-sm btn-outline-danger" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button>
      </div>
      <div v-if="store.hub">
        <p class="text-muted small mb-1">{{ store.hub.module }} × {{ store.hub.trait_name }} · {{ store.hub.hub_genes.length }} {{ t('hub.count') }}</p>
        <p class="text-muted small">{{ t('hub.network') }}：{{ store.hub.network.nodes.length }} {{ t('hub.nodes') }} / {{ store.hub.network.edges.length }} {{ t('hub.edges') }}</p>
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
          :kme-cutoff="store.kmeCutoff"
          :gene-map="geneMap"
        />
        <SimpleTable class="mt-3" :columns="[{ key: 'gene', label: t('hub.col.gene') }, { key: 'kme', label: t('hub.col.kme') }, { key: 'gs', label: t('hub.col.gs') }]" :rows="hubRows" filename="hub_gene" />
      </div>
      <p v-else class="text-muted">{{ t('hub.empty') }}</p>
  </SidebarDrawer>
</template>
