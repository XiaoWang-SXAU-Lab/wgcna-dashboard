<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import SimpleTable from '../components/SimpleTable.vue'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'

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
        <h4 class="mb-1">{{ t('hub.title') }}</h4>
        <p class="text-muted small mb-0">{{ t('hub.desc') }}</p>
      </div>
      <p v-if="store.error" class="text-danger">{{ store.error }} <button class="btn btn-sm btn-outline-danger ms-2" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button></p>
      <div v-if="store.hub">
        <p class="text-muted small">{{ store.hub.module }} × {{ store.hub.trait_name }} · {{ store.hub.hub_genes.length }} {{ t('hub.count') }}</p>
        <SimpleTable :columns="[{ key: 'gene', label: t('hub.col.gene') }, { key: 'kme', label: t('hub.col.kme') }, { key: 'gs', label: t('hub.col.gs') }]" :rows="hubRows" filename="hub_gene" />
      </div>
      <p v-else class="text-muted">{{ t('hub.empty') }}</p>
  </SidebarDrawer>
</template>
