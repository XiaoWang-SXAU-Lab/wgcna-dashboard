<script setup lang="ts">
import { formatNumber } from '../utils/formatNumber'
import ReportFigure from '../components/ReportFigure.vue'
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import SftPlot from '../components/SftPlot.vue'
import SftCheckChart from '../components/SftCheckChart.vue'
import SimpleTable from '../components/SimpleTable.vue'
import { analyzeError } from '../utils/aiBus'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'

const store = useWgcnaStore()

const showExport = ref(false)
const showExportCheck = ref(false)

const rows = computed(() => (store.sft?.fit_indices ?? []).map((f) => ({
  power: f.power, rsq: formatNumber(f.rsq, 3), slope: formatNumber(f.slope, 3),
  mean_k: formatNumber(f.mean_k, 1), median_k: formatNumber(f.median_k, 1), max_k: formatNumber(f.max_k, 1),
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
              <option value="signed hybrid">signed hybrid</option>
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
          <button class="btn btn-sm btn-outline-primary ms-auto" @click="showExport = true">
            <i class="bi bi-download me-1"></i>{{ t('plot.export') }}
          </button>
        </div>
        <ReportFigure figure-key="03_soft_threshold/sft.png"><SftPlot :data="store.sft" :rsquared-cut="store.rsquaredCut" /></ReportFigure>
        <template v-if="store.sft.scale_free_check">
          <div class="d-flex align-items-center mt-4 mb-2">
            <h6 class="fw-semibold mb-0"><i class="bi bi-graph-up me-1"></i>{{ t('sft.checkTitle') }}</h6>
            <button class="btn btn-sm btn-outline-primary ms-auto" @click="showExportCheck = true">
              <i class="bi bi-download me-1"></i>{{ t('plot.export') }}
            </button>
          </div>
          <SftCheckChart :data="store.sft.scale_free_check" />
        </template>
        <SimpleTable :columns="[{ key: 'power', label: t('sft.col.power') }, { key: 'rsq', label: t('sft.col.rsq') }, { key: 'slope', label: t('sft.col.slope') }, { key: 'mean_k', label: t('sft.col.meanK') }, { key: 'median_k', label: t('sft.col.medianK') }, { key: 'max_k', label: t('sft.col.maxK') }]" :rows="rows" filename="sft" />
        <PlotDownloadWidget kind="sft" :rsq-cut="store.rsquaredCut" :show="showExport" @close="showExport = false" />
        <PlotDownloadWidget kind="sft-check" :show="showExportCheck" @close="showExportCheck = false" />
      </div>
      <p v-else class="text-muted">{{ t('sft.empty') }}</p>
  </SidebarDrawer>
</template>
