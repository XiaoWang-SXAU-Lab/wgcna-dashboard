<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import GsMmScatter from '../components/GsMmScatter.vue'
import ModuleGeneHeatmap from '../components/ModuleGeneHeatmap.vue'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'

const store = useWgcnaStore()

const traitOptions = computed(() => store.mtait?.traits ?? [])

const moduleOptions = computed(() => {
  const sizes = store.modules?.module_sizes ?? {}
  return Object.keys(sizes).sort((a, b) => (sizes[b] ?? 0) - (sizes[a] ?? 0))
})

const selTrait = ref('')
function onTraitChange() {
  if (!selTrait.value) return
  store.selectedTrait = selTrait.value
  store.runStep('module_detail')
}

const selModule = ref('')
const showExportGsmm = ref(false)
const showExportHeatmap = ref(false)
function onModuleChange() {
  if (!selModule.value) return
  store.selectedModule = selModule.value
  store.runStep('module_detail')
}
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div class="d-flex flex-column gap-2">
        <label v-if="traitOptions.length" class="form-label small mb-0">{{ t('im.trait') }}</label>
        <select v-if="traitOptions.length" v-model="selTrait" @change="onTraitChange" class="form-select form-select-sm">
          <option v-for="tr in traitOptions" :key="tr" :value="tr">{{ tr }}</option>
        </select>
        <label v-if="moduleOptions.length" class="form-label small mb-0">{{ t('im.module') }}</label>
        <select v-if="moduleOptions.length" v-model="selModule" @change="onModuleChange" class="form-select form-select-sm">
          <option v-for="m in moduleOptions" :key="m" :value="m">{{ m }}</option>
        </select>
        <button class="btn btn-primary w-100" :disabled="!store.stepReady('module_detail') || store.stepRunning !== ''" @click="store.runStep('module_detail')">
          {{ store.stepRunning === 'module_detail' ? t('common.running') : store.isDone('module_detail') ? t('common.rerun') : t('common.run') }}
        </button>
      </div>
    </template>
      <div class="mb-3 d-flex align-items-start gap-2">
        <div class="flex-grow-1">
          <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('im.title') }} <StepStatusBadge step="module_detail" /></h4>
          <p class="text-muted small mb-0">{{ t('im.desc') }}</p>
        </div>
        <div v-if="store.detail" class="d-flex gap-2">
          <button class="btn btn-sm btn-outline-primary" @click="showExportGsmm = true"><i class="bi bi-download me-1"></i>{{ t('im.exportGsmm') }}</button>
          <button class="btn btn-sm btn-outline-primary" @click="showExportHeatmap = true"><i class="bi bi-download me-1"></i>{{ t('im.exportHeatmap') }}</button>
        </div>
      </div>
      <div v-if="store.error" class="alert alert-danger d-flex align-items-center gap-2 py-2">
        <i class="bi bi-exclamation-triangle-fill"></i>
        <span class="flex-grow-1">{{ store.error }}</span>
        <button class="btn btn-sm btn-outline-danger" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button>
      </div>
      <div v-if="store.detail">
        <div class="row g-3">
          <div class="col-lg-6"><GsMmScatter :data="store.detail" /></div>
          <div class="col-lg-6"><ModuleGeneHeatmap :data="store.detail" /></div>
        </div>
        <PlotDownloadWidget kind="gsmm" :show="showExportGsmm" @close="showExportGsmm = false" />
        <PlotDownloadWidget kind="geneheatmap" :show="showExportHeatmap" @close="showExportHeatmap = false" />
      </div>
      <p v-else class="text-muted">{{ t('im.empty') }}</p>
  </SidebarDrawer>
</template>
