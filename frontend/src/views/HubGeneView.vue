<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import SimpleTable from '../components/SimpleTable.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
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

const layout = ref<'force' | 'circular' | 'none'>('force')
const nodeSize = ref(24)
const edgeWidth = ref(6)

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
        <label class="form-label small mb-0">{{ t('hub.kmeCutoff') }}: <b>{{ store.kmeCutoff.toFixed(2) }}</b></label>
        <input type="range" class="form-range" min="0" max="1" step="0.01" v-model.number="store.kmeCutoff" />
        <label class="form-label small mb-0">{{ t('hub.gsCutoff') }}: <b>{{ store.gsCutoff.toFixed(2) }}</b></label>
        <input type="range" class="form-range" min="0" max="1" step="0.01" v-model.number="store.gsCutoff" />
        <hr class="my-1" />
        <label class="form-label small mb-0">{{ t('hub.threshold') }}: <b>{{ store.weightThreshold.toFixed(2) }}</b></label>
        <input type="range" class="form-range" min="0" max="0.5" step="0.005" v-model.number="store.weightThreshold" />
        <label class="form-label small mb-0">{{ t('hub.layout') }}</label>
        <select v-model="layout" class="form-select form-select-sm">
          <option value="force">force</option>
          <option value="circular">circular</option>
          <option value="none">none</option>
        </select>
        <label class="form-label small mb-0">{{ t('hub.nodeSize') }}: <b>{{ nodeSize }}</b></label>
        <input type="range" class="form-range" min="4" max="48" step="1" v-model.number="nodeSize" />
        <label class="form-label small mb-0">{{ t('hub.edgeWidth') }}: <b>{{ edgeWidth }}</b></label>
        <input type="range" class="form-range" min="1" max="16" step="1" v-model.number="edgeWidth" />
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
        <p class="text-muted small">{{ store.hub.module }} × {{ store.hub.trait_name }} · {{ store.hub.hub_genes.length }} {{ t('hub.count') }}</p>
        <div class="mb-3">
          <h6 class="mb-2">{{ t('hub.network') }}（{{ store.hub.network.nodes.length }} {{ t('hub.nodes') }} / {{ store.hub.network.edges.length }} {{ t('hub.edges') }}）</h6>
          <NetworkGraph :network="store.hub.network" :layout="layout" :node-size="nodeSize" :edge-width="edgeWidth" />
        </div>
        <SimpleTable :columns="[{ key: 'gene', label: t('hub.col.gene') }, { key: 'kme', label: t('hub.col.kme') }, { key: 'gs', label: t('hub.col.gs') }]" :rows="hubRows" filename="hub_gene" />
      </div>
      <p v-else class="text-muted">{{ t('hub.empty') }}</p>
  </SidebarDrawer>
</template>
