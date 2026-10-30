<script setup lang="ts">
import { computed } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { t, locale } from '../utils/i18n'

const store = useWgcnaStore()

const STEP_TITLE: Record<string, string> = {
  filter: 'di.title', soft_threshold: 'sft.title', network: 'net.title',
  module_trait: 'mt.title', module_detail: 'im.title', hub: 'hub.title',
  cluster: 'mo.clustering', cluster_elbow: 'mo.clustering', overview_network: 'mo.network',
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
        <div class="text-muted small mt-2" style="overflow-wrap: anywhere">{{ message || '…' }}</div>
        <div v-if="store.connectionLost" class="alert alert-warning small mt-2 mb-0" role="status">{{ locale === 'zh' ? '暂时无法连接后台，正在重连。计算可能仍在进行；恢复连接后会继续更新。' : 'Reconnecting to the backend. Computation may still be running; progress will resume when connected.' }}</div>
        <button class="btn btn-sm btn-outline-secondary mt-3" :disabled="store.cancelling" @click="store.cancelCurrent()">{{ store.cancelling ? (locale === 'zh' ? '正在取消…' : 'Cancelling…') : (locale === 'zh' ? '取消计算' : 'Cancel analysis') }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.progress-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; z-index: 2000; }
.progress-modal { width: 360px; max-width: 90vw; }
</style>
