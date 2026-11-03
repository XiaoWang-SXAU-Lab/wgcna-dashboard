<script setup lang="ts">
import { computed, ref } from 'vue'
import { useProjectStore } from '../stores/project'
import type { SftResult } from '../api/client'
import { locale } from '../utils/i18n'
import SftPlot from '../components/SftPlot.vue'
import SftCheckChart from '../components/SftCheckChart.vue'
import SimpleTable from '../components/SimpleTable.vue'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'

const store = useProjectStore()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const sft = computed(() => store.results.soft_threshold as SftResult | undefined)
const showExport = ref(false)
const showExportCheck = ref(false)
const canRun = computed(() => store.current?.validity.filter === '' && !store.busy)
const rows = computed(() => (sft.value?.fit_indices ?? []).map((f) => ({
  power: f.power, rsq: f.rsq.toFixed(3), slope: f.slope.toFixed(3),
  mean_k: f.mean_k.toFixed(1), median_k: f.median_k.toFixed(1), max_k: f.max_k.toFixed(1),
})))
</script>

<template>
  <div v-if="!store.current" class="text-center py-5">
    <i class="bi bi-folder2-open fs-1 text-muted"></i>
    <h4 class="mt-3">{{ text('尚未打开项目', 'No project open') }}</h4>
    <p class="text-muted">{{ text('请先新建或打开项目并完成数据过滤，再计算软阈值。', 'Open or create a project and finish filtering before soft-threshold analysis.') }}</p>
    <router-link to="/projects" class="btn btn-primary mt-2">{{ text('前往项目', 'Go to projects') }}</router-link>
  </div>
  <SidebarDrawer v-else>
    <template #sidebar>
      <fieldset :disabled="store.busy" class="vstack gap-3">
        <label>{{ text('无标度拟合阈值', 'Scale-free fit threshold') }}<input v-model.number="store.draft.soft_threshold.rsquared_cut" type="number" min="0" max="1" step="0.05" class="form-control" /></label>
        <label>{{ text('网络类型', 'Network type') }}
          <select v-model="store.draft.soft_threshold.network_type" class="form-select"><option>unsigned</option><option>signed</option><option>signed hybrid</option></select>
        </label>
      </fieldset>
      <button class="btn btn-primary w-100 mt-3" :disabled="!canRun" @click="store.run('soft_threshold')">{{ text('运行此步骤', 'Run step') }}</button>
    </template>
    <div class="mb-3"><h4 class="mb-1">{{ text('软阈值', 'Soft threshold') }}</h4><p class="text-muted small mb-0">{{ text('拟合无标度拓扑，选择软阈值 β。', 'Fit scale-free topology and choose the soft threshold β.') }}</p></div>
    <div v-if="store.error" class="alert alert-danger">{{ store.error }}</div>
    <p v-if="store.busy" role="status" class="alert alert-info">{{ text('正在计算，完成后自动保存。', 'Running. Results will be saved on completion.') }}</p>
    <template v-else-if="sft">
      <div class="d-flex align-items-center gap-2 mb-3">
        <span class="text-muted small">{{ text('推荐 β：', 'Recommended β: ') }}<b>{{ sft.power_estimate }}</b>（{{ sft.network_type }}）</span>
        <button class="btn btn-sm btn-outline-primary ms-auto" @click="showExport = true"><i class="bi bi-download me-1"></i>{{ text('导出高清图', 'Export') }}</button>
      </div>
      <SftPlot :data="sft" :rsquared-cut="Number(store.draft.soft_threshold.rsquared_cut)" />
      <template v-if="sft.scale_free_check">
        <div class="d-flex align-items-center mt-4 mb-2">
          <h6 class="fw-semibold mb-0">{{ text('无标度检验', 'Scale-free check') }}</h6>
          <button class="btn btn-sm btn-outline-primary ms-auto" @click="showExportCheck = true"><i class="bi bi-download me-1"></i>{{ text('导出高清图', 'Export') }}</button>
        </div>
        <SftCheckChart :data="sft.scale_free_check" />
      </template>
      <SimpleTable class="mt-4" :columns="[{ key: 'power', label: 'β' }, { key: 'rsq', label: 'R²' }, { key: 'slope', label: 'Slope' }, { key: 'mean_k', label: 'mean k' }, { key: 'median_k', label: 'median k' }, { key: 'max_k', label: 'max k' }]" :rows="rows" filename="sft" />
    </template>
    <p v-else class="text-muted">{{ text('完成数据过滤后运行，这里会显示软阈值结果。', 'Run after filtering to see the soft-threshold results.') }}</p>
    <PlotDownloadWidget kind="sft" :rsq-cut="Number(store.draft.soft_threshold.rsquared_cut)" :show="showExport" @close="showExport = false" />
    <PlotDownloadWidget kind="sft-check" :show="showExportCheck" @close="showExportCheck = false" />
  </SidebarDrawer>
</template>

<style scoped>
fieldset { border: 0; min-width: 0; padding: 0; }
label { font-size: .875rem; }
</style>
