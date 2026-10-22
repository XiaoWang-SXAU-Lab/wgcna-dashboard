<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import SftPlot from '../components/SftPlot.vue'
import SimpleTable from '../components/SimpleTable.vue'
import { analyzeError } from '../utils/aiBus'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'

const store = useWgcnaStore()

const rows = computed(() => (store.sft?.fit_indices ?? []).map((f) => ({
  power: f.power, rsq: f.rsq.toFixed(3), slope: f.slope.toFixed(3),
  mean_k: f.mean_k.toFixed(1), median_k: f.median_k.toFixed(1), max_k: f.max_k.toFixed(1),
})))
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div>
        <h6 class="fw-bold mb-3">{{ t('param.parameters') }}</h6>
        <div class="vstack gap-2">
          <div>
            <label class="form-label small mb-1">{{ t('di.rsq') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.rsquaredCut" min="0" max="1" step="0.05" />
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.networkType') }}</label>
            <select class="form-select form-select-sm" v-model="store.sftNetworkType">
              <option value="unsigned">unsigned</option>
              <option value="signed">signed</option>
            </select>
          </div>
          <button class="btn btn-primary w-100 mt-2" :disabled="!store.stepReady('soft_threshold') || store.stepRunning !== ''" @click="store.runStep('soft_threshold')">
            {{ store.stepRunning === 'soft_threshold' ? t('common.running') : store.isDone('soft_threshold') ? t('common.rerun') : t('common.run') }}
          </button>
        </div>
      </div>
    </template>
      <div class="mb-3">
        <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('sft.title') }} <StepStatusBadge step="soft_threshold" /></h4>
        <p class="text-muted small mb-0">{{ t('sft.desc') }}</p>
      </div>
      <div v-if="store.error" class="alert alert-danger d-flex align-items-center gap-2 py-2">
        <i class="bi bi-exclamation-triangle-fill"></i>
        <span class="flex-grow-1">{{ store.error }}</span>
        <button class="btn btn-sm btn-outline-danger" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button>
      </div>
      <div v-if="store.sft">
        <div class="d-flex align-items-center gap-2 mb-2">
          <span class="text-muted small">
            {{ t('sft.recommend') }} <b>{{ store.sft.power_estimate }}</b>（{{ store.sft.network_type }}）
            <span v-if="store.sft.power_source === 'experience'" class="badge text-bg-warning ms-1">{{ t('sft.experiencePower') }}</span>
          </span>
        </div>
        <SftPlot :data="store.sft" :rsquared-cut="store.rsquaredCut" />
        <SimpleTable :columns="[{ key: 'power', label: t('sft.col.power') }, { key: 'rsq', label: t('sft.col.rsq') }, { key: 'slope', label: t('sft.col.slope') }, { key: 'mean_k', label: t('sft.col.meanK') }, { key: 'median_k', label: t('sft.col.medianK') }, { key: 'max_k', label: t('sft.col.maxK') }]" :rows="rows" filename="sft" />
        <PlotDownloadWidget kind="sft" :rsq-cut="store.rsquaredCut" class="mt-3" />
      </div>
      <p v-else class="text-muted">{{ t('sft.empty') }}</p>
  </SidebarDrawer>
</template>
