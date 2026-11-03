<script setup lang="ts">
import { computed } from 'vue'
import { useProjectStore } from '../stores/project'
import { locale } from '../utils/i18n'

const store = useProjectStore()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const TITLES: Record<string, [string, string]> = {
  filter: ['数据与过滤', 'Data & filtering'],
  soft_threshold: ['软阈值', 'Soft threshold'],
  network: ['网络与模块', 'Network & modules'],
  module_trait: ['模块与性状', 'Module–trait'],
  module_detail: ['模块详情', 'Module detail'],
  hub: ['候选基因', 'Hub genes'],
}
const runningStep = computed(() => {
  const pr = store.current?.progress?.step
  if (pr) return pr
  for (const [name, s] of Object.entries(store.current?.state.steps ?? {})) {
    if (s.status === 'running') return name
  }
  return ''
})
const title = computed(() => {
  const m = TITLES[runningStep.value]
  return m ? text(m[0], m[1]) : text('分析计算', 'Analysis')
})
const pct = computed(() => Math.round((store.current?.progress?.progress ?? 0) * 100))
const message = computed(() => store.current?.progress?.message ?? '')
</script>

<template>
  <div v-if="store.busy" class="prp-overlay">
    <div class="prp-modal card shadow-lg">
      <div class="card-body p-3">
        <div class="d-flex align-items-center gap-2 mb-2">
          <div class="spinner-border spinner-border-sm text-primary"></div>
          <span class="fw-semibold">{{ title }}</span>
          <span class="ms-auto small text-muted">{{ pct }}%</span>
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
.prp-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; z-index: 2000; }
.prp-modal { width: 360px; max-width: 90vw; }
</style>
