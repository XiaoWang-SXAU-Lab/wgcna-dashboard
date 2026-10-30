<script setup lang="ts">
import { formatNumber } from '../utils/formatNumber'
import ReportFigure from '../components/ReportFigure.vue'
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import TraitHeatmap from '../components/TraitHeatmap.vue'
import EigengeneHeatmap from '../components/EigengeneHeatmap.vue'
import SimpleTable from '../components/SimpleTable.vue'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'

const store = useWgcnaStore()
const showExportMtrait = ref(false)
const showExportEigengene = ref(false)

const kmeRows = computed(() => {
  if (!store.mtait?.kme || !store.mtait.genes) return []
  const mods = store.mtait.modules
  return store.mtait.genes.map((g, i) => {
    const row: Record<string, string> = { gene: g }
    mods.forEach((m, j) => { row[m] = formatNumber(store.mtait!.kme[i]?.[j], 4) })
    return row
  })
})
const kmeColumns = computed(() => [
  { key: 'gene', label: t('mt.col.gene') },
  ...(store.mtait?.modules ?? []).map((m) => ({ key: m, label: 'MM.' + m })),
])

const mtraitRows = computed(() => {
  if (!store.mtait) return []
  const rows: Record<string, string>[] = []
  store.mtait.modules.forEach((m, i) => store.mtait!.traits.forEach((tr, j) => {
    rows.push({
      module: m, trait: tr,
      cor: formatNumber(store.mtait!.cor[i]?.[j], 3),
      p: formatNumber(store.mtait!.p[i]?.[j], 2, true),
    })
  }))
  return rows
})
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div class="vstack gap-2">
        <div>
          <label class="form-label small mb-0">{{ t('mt.traitSource') }}</label>
          <select class="form-select form-select-sm" v-model="store.traitSource">
            <option value="uploaded" :disabled="!store.traitUploaded">{{ t('mt.traitSourceUploaded') }}</option>
            <option value="sample_info" :disabled="!store.sampleInfo">{{ t('mt.traitSourceSampleInfo') }}</option>
          </select>
        </div>
        <div v-if="store.traitSource === 'sample_info'">
          <label class="form-label small mb-0">{{ t('mt.traitColumn') }}</label>
          <select class="form-select form-select-sm" v-model="store.traitColumn">
            <option v-for="c in store.sampleColumns" :key="c" :value="c">{{ c }}</option>
          </select>
        </div>
        <p v-if="store.traitSource === 'uploaded' && !store.traitUploaded" class="alert alert-warning small mb-0">{{ t('mt.warn') }}</p>
        <button class="btn btn-primary w-100" :disabled="!store.stepReady('module_trait') || store.stepRunning !== ''" @click="store.runStep('module_trait')">
          {{ store.stepRunning === 'module_trait' ? t('common.running') : store.isDone('module_trait') ? t('common.rerun') : t('common.run') }}
        </button>
      </div>
    </template>
      <div class="mb-3 d-flex align-items-start gap-2">
        <div class="flex-grow-1">
          <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('mt.title') }} <StepStatusBadge step="module_trait" /></h4>
          <p class="text-muted small mb-0">{{ t('mt.desc') }}</p>
        </div>
        <div v-if="store.mtait" class="d-flex gap-2">
          <button class="btn btn-sm btn-outline-primary" @click="showExportMtrait = true"><i class="bi bi-download me-1"></i>{{ t('mt.exportHeatmap') }}</button>
          <button class="btn btn-sm btn-outline-primary" @click="showExportEigengene = true"><i class="bi bi-download me-1"></i>{{ t('mt.exportEigengene') }}</button>
        </div>
      </div>
      <div v-if="store.error" class="alert alert-danger d-flex align-items-center gap-2 py-2">
        <i class="bi bi-exclamation-triangle-fill"></i>
        <span class="flex-grow-1">{{ store.error }}</span>
        <button class="btn btn-sm btn-outline-danger" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button>
      </div>
      <div v-if="store.mtait">
        <div class="row g-3 mb-3">
          <div class="col-lg-7"><ReportFigure figure-key="05_module_trait/mtrait.png"><TraitHeatmap :data="store.mtait" /></ReportFigure></div>
          <div class="col-lg-5"><EigengeneHeatmap :data="store.mtait" /></div>
        </div>
        <SimpleTable :columns="[{ key: 'module', label: t('mt.col.module') }, { key: 'trait', label: t('mt.col.trait') }, { key: 'cor', label: t('mt.col.cor') }, { key: 'p', label: t('mt.col.p') }]" :rows="mtraitRows" filename="module_trait" />
        <h6 class="mt-4 mb-2">{{ t('mt.kme') }}</h6>
        <SimpleTable :columns="kmeColumns" :rows="kmeRows" filename="kme" />
        <PlotDownloadWidget kind="mtrait" :show="showExportMtrait" @close="showExportMtrait = false" />
        <PlotDownloadWidget kind="eigengene" :show="showExportEigengene" @close="showExportEigengene = false" />
      </div>
      <p v-else class="text-muted">{{ t('mt.empty') }}</p>
  </SidebarDrawer>
</template>
