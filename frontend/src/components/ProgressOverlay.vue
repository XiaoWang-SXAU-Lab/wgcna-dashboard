<script setup lang="ts">
import { computed } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { t } from '../utils/i18n'

const store = useWgcnaStore()

const STEP_TITLE: Record<string, string> = {
  filter: 'di.title', soft_threshold: 'sft.title', network: 'net.title',
  module_trait: 'mt.title', module_detail: 'im.title', hub: 'hub.title',
}

const title = computed(() => t(STEP_TITLE[store.stepRunning] ?? store.stepRunning))
const pct = computed(() => Math.round((store.job?.progress?.progress ?? 0) * 100))
const message = computed(() => store.job?.progress?.message ?? '')
</script>

<template>
  <div v-if="store.stepRunning" class="progress-overlay">
    <div class="progress-modal card shadow-lg">
      <div class="card-body p-3">
        <div class="d-flex align-items-center gap-2 mb-2">
          <div class="spinner-border spinner-border-sm text-primary"></div>
          <span class="fw-semibold">{{ title }}</span>
        </div>
        <div class="progress" style="height: 8px">
          <div class="progress-bar progress-bar-striped progress-bar-animated" :style="{ width: pct + '%' }"></div>
        </div>
        <div class="text-muted small mt-2 text-truncate">{{ message || '…' }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.progress-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; z-index: 2000; }
.progress-modal { width: 360px; max-width: 90vw; }
</style>
