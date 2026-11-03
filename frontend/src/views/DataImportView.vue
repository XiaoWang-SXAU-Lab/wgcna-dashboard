<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useProjectStore } from '../stores/project'
import type { FilterResult, SampleInfo } from '../api/client'
import { getSampleInfo } from '../api/projects'
import { locale } from '../utils/i18n'
import SampleTreeChart, { type CategoryBand } from '../components/SampleTreeChart.vue'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'

const store = useProjectStore()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const selectedOutliers = ref<string[]>([])
const showTreeExport = ref(false)
const sampleInfos = ref<SampleInfo[]>([])
const attrColumns = ref<string[]>([])
const colorAttrs = ref<string[]>([])
const EXCLUDE_ATTRS = new Set(['sample_id', 'sample_name', 'species'])
async function loadSampleInfo() {
  if (!store.id || !store.current?.state.inputs.sample_info) {
    sampleInfos.value = []; attrColumns.value = []; colorAttrs.value = []
    return
  }
  try {
    const infos = await getSampleInfo(store.id)
    sampleInfos.value = infos
    const cols = (infos.length ? Object.keys(infos[0].attrs) : []).filter((c) => !EXCLUDE_ATTRS.has(c))
    attrColumns.value = cols
    colorAttrs.value = cols.includes('group') ? ['group'] : (cols.length ? [cols[0]] : [])
  } catch {
    sampleInfos.value = []; attrColumns.value = []; colorAttrs.value = []
  }
}
watch(() => store.current?.state.inputs.sample_info?.sha256, () => { void loadSampleInfo() }, { immediate: true })
const categories = computed<CategoryBand[]>(() => {
  if (!colorAttrs.value.length || !sampleInfos.value.length) return []
  return colorAttrs.value.map((attr) => {
    const values: Record<string, string> = {}
    for (const s of sampleInfos.value) values[s.sample_id] = s.attrs[attr] ?? ''
    return { name: attr, values }
  })
})
const roles = computed(() => [
  ['expr', text('表达矩阵（必需）', 'Expression matrix (required)')],
  ['trait', text('表型数据', 'Trait table')],
  ['sample_info', text('样本信息', 'Sample information')],
  ['annotation', text('基因注释', 'Gene annotation')],
  ['gene2go', 'Gene → GO'],
  ['gene2kegg', 'Gene → KEGG'],
])
const filter = computed(() => store.results.filter as FilterResult | undefined)
const canRun = computed(() => !!store.current?.state.inputs.expr && !store.busy)
const demoLoading = ref(false)
async function loadDemo() {
  demoLoading.value = true
  try { await store.loadDemo() } catch (e) { store.error = String(e) } finally { demoLoading.value = false }
}

async function upload(role: string, event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) { try { await store.importFile(role, file) } catch (e) { store.error = String(e) } }
  input.value = ''
}
function toggleOutlier(sample: string) {
  selectedOutliers.value = selectedOutliers.value.includes(sample) ? selectedOutliers.value.filter(x => x !== sample) : [...selectedOutliers.value, sample]
}
async function excludeSelected() {
  store.draft.filter.exclude_samples = [...new Set([...(store.draft.filter.exclude_samples as string[]), ...selectedOutliers.value])]
  selectedOutliers.value = []
  await store.run('filter')
}
</script>

<template>
  <div v-if="!store.current" class="text-center py-5">
    <i class="bi bi-folder2-open fs-1 text-muted"></i>
    <h4 class="mt-3">{{ text('尚未打开项目', 'No project open') }}</h4>
    <p class="text-muted">{{ text('请先新建或打开一个分析项目，再进行数据导入与过滤。', 'Open or create a project before importing data and filtering.') }}</p>
    <router-link to="/projects" class="btn btn-primary mt-2">{{ text('前往项目', 'Go to projects') }}</router-link>
  </div>
  <SidebarDrawer v-else>
    <template #sidebar>
      <div class="border rounded p-2 mb-3">
        <div class="small fw-semibold mb-1">{{ text('演示数据', 'Demo data') }}</div>
        <div class="d-flex gap-2">
          <button class="btn btn-sm btn-primary flex-fill" :disabled="store.busy || demoLoading" @click="loadDemo">{{ text('加载演示数据', 'Load demo data') }}</button>
          <a class="btn btn-sm btn-outline-secondary flex-fill" href="/api/demo/download" download :title="text('下载演示数据', 'Download demo data')"><i class="bi bi-download"></i></a>
        </div>
        <div class="small text-muted mt-1">{{ text('拟南芥热胁迫转录记忆（GSE162434），导入表达矩阵、表型与注释。', 'Arabidopsis heat-stress memory (GSE162434): imports expression, trait and annotation.') }}</div>
      </div>
      <fieldset :disabled="store.busy" class="vstack gap-3">
        <label v-for="[role, label] in roles" :key="role" class="small">{{ label }} <i v-if="store.current.state.inputs[role]" class="bi bi-check-circle text-success"></i>
          <input type="file" accept=".tsv,.txt,.csv,.xlsx,.xls" class="form-control form-control-sm mt-1" @change="upload(role, $event)" />
        </label>
        <label>{{ text('数据类型', 'Data type') }}
          <select v-model="store.draft.filter.data_type" class="form-select">
            <option value="count">Raw counts</option>
            <option value="normalized_count">FPKM / TPM</option>
            <option value="protein_abundance">Protein abundance</option>
            <option value="peak_area">Peak area</option>
          </select>
        </label>
        <label>{{ text('归一化', 'Normalization') }}
          <select v-model="store.draft.filter.normalize_method" class="form-select">
            <option v-if="store.draft.filter.data_type === 'count'" value="vst">VST</option>
            <option value="log2p1">log2(x+1)</option>
            <option value="log2">log2(x)</option>
            <option value="raw">{{ text('不变换', 'No transformation') }}</option>
          </select>
        </label>
        <label>{{ text('保留基因数', 'Genes to keep') }}<input v-model.number="store.draft.filter.n_keep" type="number" min="1" class="form-control" /></label>
        <label>{{ text('基因筛选', 'Gene selection') }}
          <select v-model="store.draft.filter.filter_method" class="form-select"><option value="mad">MAD</option><option value="var">Variance</option></select>
        </label>
        <label>{{ text('表达阈值', 'Expression cutoff') }}<input v-model.number="store.draft.filter.cutoff" type="number" min="0" class="form-control" /></label>
        <label>{{ text('满足阈值的样本比例（0–1）', 'Sample fraction above cutoff (0–1)') }}<input v-model.number="store.draft.filter.sample_percentage" type="number" min="0" max="1" step="0.1" class="form-control" /></label>
        <button v-if="(store.draft.filter.exclude_samples as string[]).length" class="btn btn-outline-secondary" @click="store.draft.filter.exclude_samples = []">{{ text('清除排除样本', 'Clear excluded samples') }} ({{ (store.draft.filter.exclude_samples as string[]).length }})</button>
      </fieldset>
      <button class="btn btn-primary w-100 mt-3" :disabled="!canRun" @click="store.run('filter')">{{ text('运行此步骤', 'Run step') }}</button>
    </template>
    <div class="mb-3"><h4 class="mb-1">{{ text('数据导入与过滤', 'Data import & filtering') }}</h4><p class="text-muted small mb-0">{{ text('上传表达矩阵并设置过滤参数，然后运行。', 'Upload an expression matrix, set filtering parameters, then run.') }}</p></div>
    <div v-if="store.error" class="alert alert-danger">{{ store.error }}</div>
    <p v-if="store.busy" role="status" class="alert alert-info">{{ text('正在计算，完成后自动保存。', 'Running. Results will be saved on completion.') }}</p>
    <template v-else-if="filter">
      <div class="card mb-3"><div class="card-header bg-white fw-semibold">{{ text('过滤结果', 'Filter results') }}</div>
        <div class="card-body py-2 d-flex flex-wrap gap-2">
          <span class="badge text-bg-light border">{{ text('过滤前基因', 'Genes before') }}: {{ filter.n_genes_before }}</span>
          <span class="badge text-bg-light border">{{ text('低表达过滤后', 'After low-expression') }}: {{ filter.n_genes_after_low }}</span>
          <span class="badge text-bg-success">{{ text('最终基因', 'Final genes') }}: {{ filter.n_genes_final }}</span>
          <span class="badge text-bg-light border">{{ text('样本', 'Samples') }}: {{ filter.n_samples }}</span>
        </div>
      </div>
      <div class="d-flex align-items-center gap-2 mb-2">
        <h6 class="fw-semibold mb-0">{{ text('样本聚类树', 'Sample tree') }}</h6>
        <button class="btn btn-sm btn-outline-primary ms-auto" @click="showTreeExport = true"><i class="bi bi-download me-1"></i>{{ text('导出高清图', 'Export') }}</button>
      </div>
      <div v-if="attrColumns.length" class="d-flex align-items-center gap-2 mb-2">
        <label class="form-label small mb-0 text-nowrap">{{ text('按字段着色', 'Color by') }}</label>
        <div class="d-flex flex-wrap gap-2">
          <label v-for="c in attrColumns" :key="c" class="form-check form-check-inline small mb-0">
            <input class="form-check-input" type="checkbox" :value="c" v-model="colorAttrs" />
            <span class="form-check-label">{{ c }}</span>
          </label>
        </div>
      </div>
      <SampleTreeChart :samples="filter.samples" :tree="filter.sample_tree" :categories="categories" :selected="selectedOutliers" @select="toggleOutlier" />
      <div class="card mt-3"><div class="card-header bg-white fw-semibold">{{ text('排除离群样本', 'Exclude outlier samples') }}</div>
        <div class="card-body">
          <p class="text-muted small">{{ text('勾选要排除的样本，然后重新过滤。', 'Select samples to exclude, then rerun filtering.') }}</p>
          <div class="d-flex flex-wrap gap-2">
            <label v-for="s in filter.samples" :key="s" class="form-check mb-0"><input class="form-check-input" type="checkbox" :value="s" v-model="selectedOutliers" /><span class="form-check-label small">{{ s }}</span></label>
          </div>
          <button class="btn btn-danger mt-3" :disabled="!selectedOutliers.length || store.busy" @click="excludeSelected">{{ text('排除选中样本并重新过滤', 'Exclude selected and rerun') }}</button>
        </div>
      </div>
    </template>
    <p v-else class="text-muted">{{ text('导入表达矩阵并运行，结果会保存在当前项目中。', 'Import an expression matrix and run. Results are saved in this project.') }}</p>
    <PlotDownloadWidget kind="sample-tree" :attrs="colorAttrs.join(',')" :show="showTreeExport" @close="showTreeExport = false" />
  </SidebarDrawer>
</template>

<style scoped>
fieldset { border: 0; min-width: 0; padding: 0; }
label { font-size: .875rem; }
</style>
