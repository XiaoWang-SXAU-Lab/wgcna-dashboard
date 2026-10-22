<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import TraitHeatmap from '../components/TraitHeatmap.vue'
import EigengeneHeatmap from '../components/EigengeneHeatmap.vue'
import SimpleTable from '../components/SimpleTable.vue'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'

const store = useWgcnaStore()

const mtraitRows = computed(() => {
  if (!store.mtait) return []
  const rows: Record<string, string>[] = []
  store.mtait.modules.forEach((m, i) => store.mtait!.traits.forEach((tr, j) => {
    rows.push({
      module: m, trait: tr,
      cor: (store.mtait!.cor[i]?.[j] ?? 0).toFixed(3),
      p: (store.mtait!.p[i]?.[j] ?? 1).toExponential(2),
    })
  }))
  return rows
})
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div>
        <p v-if="!store.traitUploaded" class="alert alert-warning small">{{ t('mt.warn') }}</p>
        <button class="btn btn-primary w-100" :disabled="!store.stepReady('module_trait') || store.stepRunning !== ''" @click="store.runStep('module_trait')">
          {{ store.stepRunning === 'module_trait' ? t('common.running') : store.isDone('module_trait') ? t('common.rerun') : t('common.run') }}
        </button>
      </div>
    </template>
      <div class="mb-3">
        <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('mt.title') }} <StepStatusBadge step="module_trait" /></h4>
        <p class="text-muted small mb-0">{{ t('mt.desc') }}</p>
      </div>
      <div v-if="store.error" class="alert alert-danger d-flex align-items-center gap-2 py-2">
        <i class="bi bi-exclamation-triangle-fill"></i>
        <span class="flex-grow-1">{{ store.error }}</span>
        <button class="btn btn-sm btn-outline-danger" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button>
      </div>
      <div v-if="store.mtait">
        <div class="row g-3 mb-3">
          <div class="col-lg-7"><TraitHeatmap :data="store.mtait" /></div>
          <div class="col-lg-5"><EigengeneHeatmap :data="store.mtait" /></div>
        </div>
        <SimpleTable :columns="[{ key: 'module', label: t('mt.col.module') }, { key: 'trait', label: t('mt.col.trait') }, { key: 'cor', label: t('mt.col.cor') }, { key: 'p', label: t('mt.col.p') }]" :rows="mtraitRows" filename="module_trait" />
      </div>
      <p v-else class="text-muted">{{ t('mt.empty') }}</p>
  </SidebarDrawer>
</template>
