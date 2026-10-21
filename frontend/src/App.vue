<script setup lang="ts">
import { computed, ref } from 'vue'
import { AgGridVue } from 'ag-grid-vue3'
import {
  createJob,
  getJob,
  getResult,
  runAll,
  type DetailResult,
  type JobStatus,
  type MtaitResult,
} from './api/client'
import DendrogramChart from './components/DendrogramChart.vue'
import GsMmScatter from './components/GsMmScatter.vue'
import TraitHeatmap from './components/TraitHeatmap.vue'
import ChatWidget from './components/ChatWidget.vue'
import 'ag-grid-community/styles/ag-grid.css'
import 'ag-grid-community/styles/ag-theme-quartz.css'

// Phase 1 minimal loop: upload expr -> submit -> run -> poll -> show module colors table.

const file = ref<File | null>(null)
const nKeep = ref(8000)
const job = ref<JobStatus | null>(null)
const submitting = ref(false)
const error = ref('')
const mtait = ref<MtaitResult | null>(null)
const detail = ref<DetailResult | null>(null)

const STEPS = ['filter', 'soft_threshold', 'network', 'module_trait', 'module_detail', 'hub']
const STEP_NAMES: Record<string, string> = {
  filter: '清洗过滤',
  soft_threshold: '软阈值 β',
  network: '网络与模块',
  module_trait: '模块-性状',
  module_detail: '模块深入',
  hub: 'hub 基因',
}

function defaultParams() {
  return {
    filter: {
      normalize_method: 'log10p1',
      n_keep: nKeep.value,
      filter_method: 'mad',
      sample_percentage: 0,
      cutoff: 1,
    },
    soft_threshold: { network_type: 'unsigned', rsquared_cut: 0.85 },
    network: {
      power: 6,
      network_type: 'unsigned',
      tom_type: 'signed',
      tom_denom: 'min',
      min_module_size: 30,
      merge_cut_height: 0.25,
      deep_split: 2,
      detect_cut_height: 0.995,
    },
  }
}

function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
}

async function submit() {
  if (!file.value || submitting.value) return
  submitting.value = true
  error.value = ''
  job.value = null
  try {
    const { id } = await createJob(file.value, defaultParams())
    await runAll(id)
    await poll(id)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    submitting.value = false
  }
}

async function poll(id: string) {
  for (;;) {
    const j = await getJob(id)
    job.value = j
    if (j.status !== 'running') break
    await new Promise((r) => setTimeout(r, 1500))
  }
  await loadResults(id)
}

async function loadResults(id: string) {
  mtait.value = null
  detail.value = null
  try {
    mtait.value = await getResult<MtaitResult>(id, 'module_trait')
  } catch {
    /* not run yet */
  }
  try {
    detail.value = await getResult<DetailResult>(id, 'module_detail')
  } catch {
    /* not run yet */
  }
}

const stepIndex = computed(() => STEPS.indexOf(job.value?.current_step ?? ''))

const columnDefs = [
  { field: 'gene', headerName: 'Gene', flex: 1, sortable: true },
  { field: 'color', headerName: 'Module', flex: 1, sortable: true },
]

const rowData = computed(() =>
  (job.value?.result?.genes ?? []).map((gene, i) => ({
    gene,
    color: job.value?.result?.colors[i] ?? 'grey',
  })),
)

const moduleSizes = computed(() => {
  const sizes = job.value?.result?.module_sizes ?? {}
  return Object.entries(sizes).sort((a, b) => b[1] - a[1])
})
</script>

<template>
  <div class="app">
    <header class="app-header">
      <h1>WGCNA Dashboard</h1>
      <p class="subtitle">加权基因共表达网络分析 · Weighted Gene Co-expression Network Analysis</p>
    </header>

    <!-- 提交区 -->
    <section class="panel">
      <label class="file-label">
        表达矩阵文件（genes × samples TSV）
        <input type="file" accept=".tsv,.txt" @change="onFile" />
      </label>
      <label class="field">
        保留基因数（MAD top-N）
        <input type="number" v-model.number="nKeep" min="100" />
      </label>
      <button :disabled="!file || submitting" @click="submit">
        {{ submitting ? '运行中…' : '开始分析' }}
      </button>
      <p v-if="error" class="error">{{ error }}</p>
    </section>

    <!-- 进度区 -->
    <section v-if="job" class="panel">
      <div class="stepper">
        <span
          v-for="(s, i) in STEPS"
          :key="s"
          :class="['step', { active: i === stepIndex, done: job.status === 'done' || i < stepIndex }]"
        >
          {{ STEP_NAMES[s] }}
        </span>
      </div>
      <p v-if="job.progress" class="progress-msg">{{ job.progress.message }}</p>
      <p v-if="job.status === 'error'" class="error">出错：{{ job.error }}</p>
    </section>

    <!-- 结果区 -->
    <section v-if="job?.status === 'done'" class="panel">
      <h2>模块概览</h2>
      <div class="module-chips">
        <span v-for="[m, n] in moduleSizes" :key="m" class="chip">{{ m }} ({{ n }})</span>
      </div>

      <template v-if="job.result?.dendrogram">
        <h2>模块树状图</h2>
        <DendrogramChart
          :genes="job.result.genes"
          :colors="job.result.colors"
          :dendrogram="job.result.dendrogram"
        />
      </template>

      <template v-if="mtait">
        <h2>模块-性状关联</h2>
        <TraitHeatmap :data="mtait" />
      </template>

      <template v-if="detail">
        <h2>基因显著性 × 模块隶属度（{{ detail.module }} / {{ detail.trait_name }}）</h2>
        <GsMmScatter :data="detail" />
      </template>

      <h2>基因 → 模块</h2>
      <ag-grid-vue
        class="ag-theme-quartz grid"
        :column-defs="columnDefs"
        :row-data="rowData"
        :default-col-def="{ resizable: true }"
        style="height: 480px"
      />
    </section>

    <ChatWidget :error-ctx="job?.error ?? ''" />
  </div>
</template>

<style scoped>
.app {
  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem;
}
.app-header h1 {
  margin: 0;
  font-size: 1.8rem;
}
.subtitle {
  color: #888;
  margin: 0.25rem 0 1.5rem;
}
.panel {
  border: 1px solid #e3e3e3;
  border-radius: 8px;
  padding: 1rem;
  margin-bottom: 1rem;
}
.file-label,
.field {
  display: block;
  margin-bottom: 0.75rem;
  font-size: 0.95rem;
}
button {
  padding: 0.5rem 1.2rem;
  font-size: 1rem;
  cursor: pointer;
}
button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.error {
  color: #d73027;
}
.stepper {
  display: flex;
  gap: 0.5rem;
}
.step {
  padding: 0.25rem 0.6rem;
  border-radius: 999px;
  background: #eee;
  color: #666;
  font-size: 0.85rem;
}
.step.active {
  background: #3182bd;
  color: #fff;
}
.step.done {
  background: #d1f2d1;
  color: #1a7f1a;
}
.progress-msg {
  color: #555;
  margin: 0.5rem 0 0;
}
.module-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-bottom: 1rem;
}
.chip {
  padding: 0.2rem 0.6rem;
  border-radius: 6px;
  background: #f0f4f8;
  font-size: 0.85rem;
}
.grid {
  width: 100%;
}
</style>
