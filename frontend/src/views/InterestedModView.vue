<script setup lang="ts">
import { ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import GsMmScatter from '../components/GsMmScatter.vue'
import ModuleGeneHeatmap from '../components/ModuleGeneHeatmap.vue'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'

const store = useWgcnaStore()
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div>
        <button class="btn btn-primary w-100" :disabled="!store.stepReady('module_detail') || store.stepRunning !== ''" @click="store.runStep('module_detail')">
          {{ store.stepRunning === 'module_detail' ? t('common.running') : store.isDone('module_detail') ? t('common.rerun') : t('common.run') }}
        </button>
      </div>
    </template>
      <div class="mb-3">
        <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('im.title') }} <StepStatusBadge step="module_detail" /></h4>
        <p class="text-muted small mb-0">{{ t('im.desc') }}</p>
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
      </div>
      <p v-else class="text-muted">{{ t('im.empty') }}</p>
  </SidebarDrawer>
</template>
