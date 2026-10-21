<script setup lang="ts">
import { ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import GsMmScatter from '../components/GsMmScatter.vue'
import ModuleGeneHeatmap from '../components/ModuleGeneHeatmap.vue'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'

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
        <h4 class="mb-1">{{ t('im.title') }}</h4>
        <p class="text-muted small mb-0">{{ t('im.desc') }}</p>
      </div>
      <p v-if="store.error" class="text-danger">{{ store.error }} <button class="btn btn-sm btn-outline-danger ms-2" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button></p>
      <div v-if="store.detail">
        <div class="row g-3">
          <div class="col-lg-6"><GsMmScatter :data="store.detail" /></div>
          <div class="col-lg-6"><ModuleGeneHeatmap :data="store.detail" /></div>
        </div>
      </div>
      <p v-else class="text-muted">{{ t('im.empty') }}</p>
  </SidebarDrawer>
</template>
