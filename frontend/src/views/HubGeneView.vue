<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import SimpleTable from '../components/SimpleTable.vue'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'

const store = useWgcnaStore()

const hubRows = computed(() => (store.hub?.hub_genes ?? []).map((h) => ({
  gene: h.gene, kme: h.kme.toFixed(4), gs: h.gs.toFixed(4),
})))
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div>
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
        <SimpleTable :columns="[{ key: 'gene', label: t('hub.col.gene') }, { key: 'kme', label: t('hub.col.kme') }, { key: 'gs', label: t('hub.col.gs') }]" :rows="hubRows" filename="hub_gene" />
      </div>
      <p v-else class="text-muted">{{ t('hub.empty') }}</p>
  </SidebarDrawer>
</template>
