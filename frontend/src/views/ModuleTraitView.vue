<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useProjectStore } from '../stores/project'
import type { MtaitResult } from '../api/client'
import { getSampleInfo } from '../api/projects'
import { locale } from '../utils/i18n'
import TraitHeatmap from '../components/TraitHeatmap.vue'
import EigengeneHeatmap from '../components/EigengeneHeatmap.vue'
import SimpleTable from '../components/SimpleTable.vue'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'

const store = useProjectStore()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const mtrait = computed(() => store.results.module_trait as MtaitResult | undefined)
const showExportMtrait = ref(false)
const showExportEigengene = ref(false)
const sampleColumns = ref<string[]>([])

watch(() => store.current?.state.inputs.sample_info?.sha256, async () => {
  if (!store.id || !store.current?.state.inputs.sample_info) { sampleColumns.value = []; return }
  try {
    const infos = await getSampleInfo(store.id)
    sampleColumns.value = (infos.length ? Object.keys(infos[0].attrs) : []).filter((c) => !['sample_id', 'sample_name', 'species'].includes(c))
  } catch { sampleColumns.value = [] }
}, { immediate: true })

const hasTrait = computed(() => !!store.current?.state.inputs.trait)
const hasSampleInfo = computed(() => !!store.current?.state.inputs.sample_info)
const traitReady = computed(() => store.draft.module_trait.trait_source === 'uploaded' ? hasTrait.value : hasSampleInfo.value)
const canRun = computed(() => store.current?.validity.filter === '' && store.current?.validity.network === '' && traitReady.value && !store.busy)

const kmeRows = computed(() => {
  if (!mtrait.value?.kme || !mtrait.value.genes) return []
  const mods = mtrait.value.modules
  return mtrait.value.genes.map((g, i) => {
    const row: Record<string, string> = { gene: g }
    mods.forEach((m, j) => { row[m] = (mtrait.value!.kme[i]?.[j] ?? 0).toFixed(4) })
    return row
  })
})
const kmeColumns = computed(() => [
  { key: 'gene', label: text('基因', 'Gene') },
  ...(mtrait.value?.modules ?? []).map((m) => ({ key: m, label: 'MM.' + m })),
])
const mtraitRows = computed(() => {
  if (!mtrait.value) return []
  const rows: Record<string, string>[] = []
  mtrait.value.modules.forEach((m, i) => mtrait.value!.traits.forEach((tr, j) => {
    rows.push({ module: m, trait: tr, cor: (mtrait.value!.cor[i]?.[j] ?? 0).toFixed(3), p: (mtrait.value!.p[i]?.[j] ?? 1).toExponential(2) })
  }))
  return rows
})
</script>

<template>
  <div v-if="!store.current" class="text-center py-5">
    <i class="bi bi-folder2-open fs-1 text-muted"></i>
    <h4 class="mt-3">{{ text('尚未打开项目', 'No project open') }}</h4>
    <p class="text-muted">{{ text('请先新建或打开项目并完成过滤、软阈值与网络，再做模块-性状关联。', 'Open or create a project and finish filtering + soft threshold + network before module–trait analysis.') }}</p>
    <router-link to="/projects" class="btn btn-primary mt-2">{{ text('前往项目', 'Go to projects') }}</router-link>
  </div>
  <SidebarDrawer v-else>
    <template #sidebar>
      <div class="vstack gap-2">
        <div>
          <label class="form-label small mb-0">{{ text('性状来源', 'Trait source') }}</label>
          <select class="form-select form-select-sm" v-model="store.draft.module_trait.trait_source">
            <option value="uploaded" :disabled="!hasTrait">{{ text('表型表', 'Trait table') }}</option>
            <option value="sample_info" :disabled="!hasSampleInfo">{{ text('样本信息', 'Sample information') }}</option>
          </select>
        </div>
        <div v-if="store.draft.module_trait.trait_source === 'sample_info'">
          <label class="form-label small mb-0">{{ text('样本信息列名', 'Sample information column') }}</label>
          <select class="form-select form-select-sm" v-model="store.draft.module_trait.trait_column">
            <option v-for="c in sampleColumns" :key="c" :value="c">{{ c }}</option>
          </select>
        </div>
        <p v-if="store.draft.module_trait.trait_source === 'uploaded' && !hasTrait" class="alert alert-warning small mb-0">{{ text('尚未导入表型表', 'No trait table imported') }}</p>
        <button class="btn btn-primary w-100" :disabled="!canRun" @click="store.run('module_trait')">{{ text('运行此步骤', 'Run step') }}</button>
      </div>
    </template>
    <div class="mb-3 d-flex align-items-start gap-2">
      <div class="flex-grow-1"><h4 class="mb-1">{{ text('模块与性状', 'Module–trait') }}</h4><p class="text-muted small mb-0">{{ text('关联模块与性状，找出显著相关的模块。', 'Correlate modules with traits to find significant associations.') }}</p></div>
      <div v-if="mtrait" class="d-flex gap-2">
        <button class="btn btn-sm btn-outline-primary" @click="showExportMtrait = true"><i class="bi bi-download me-1"></i>{{ text('导出热图', 'Export heatmap') }}</button>
        <button class="btn btn-sm btn-outline-primary" @click="showExportEigengene = true"><i class="bi bi-download me-1"></i>{{ text('导出特征基因', 'Export eigengene') }}</button>
      </div>
    </div>
    <div v-if="store.error" class="alert alert-danger">{{ store.error }}</div>
    <div v-if="mtrait">
      <div class="row g-3 mb-3">
        <div class="col-lg-7"><TraitHeatmap :data="mtrait" /></div>
        <div class="col-lg-5"><EigengeneHeatmap :data="mtrait" /></div>
      </div>
      <SimpleTable :columns="[{ key: 'module', label: text('模块', 'Module') }, { key: 'trait', label: text('性状', 'Trait') }, { key: 'cor', label: 'cor' }, { key: 'p', label: 'p' }]" :rows="mtraitRows" filename="module_trait" />
      <h6 class="mt-4 mb-2">{{ text('模块内基因与模块的相关性 (kME)', 'kME') }}</h6>
      <SimpleTable :columns="kmeColumns" :rows="kmeRows" filename="kme" />
    </div>
    <p v-else class="text-muted">{{ text('完成网络与模块识别后运行，这里会显示模块-性状关联。', 'Run after network detection to see module–trait associations.') }}</p>
    <PlotDownloadWidget kind="mtrait" :show="showExportMtrait" @close="showExportMtrait = false" />
    <PlotDownloadWidget kind="eigengene" :show="showExportEigengene" @close="showExportEigengene = false" />
  </SidebarDrawer>
</template>
