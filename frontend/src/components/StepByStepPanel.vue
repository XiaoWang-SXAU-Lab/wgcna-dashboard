<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  createJob, getJob, getResult, runStep as runStepApi, uploadTrait,
  type DetailResult, type HubResult, type JobStatus, type ModulesResult, type MtaitResult, type SftResult,
} from '../api/client'
import SftPlot from './SftPlot.vue'
import TraitHeatmap from './TraitHeatmap.vue'
import EigengeneHeatmap from './EigengeneHeatmap.vue'
import ModuleGeneHeatmap from './ModuleGeneHeatmap.vue'
import GsMmScatter from './GsMmScatter.vue'
import DendrogramChart from './DendrogramChart.vue'
import SimpleTable from './SimpleTable.vue'
import { moduleColor } from '../utils/colors'
import { analyzeError } from '../utils/aiBus'

const STEPS = [
  { key: 'filter', name: '清洗过滤', icon: 'bi-funnel', desc: '归一化 + 低表达过滤 + MAD/VAR 选基因' },
  { key: 'soft_threshold', name: '软阈值 β', icon: 'bi-activity', desc: '无标度拟合 R² 与平均连通性' },
  { key: 'network', name: '网络与模块', icon: 'bi-diagram-3', desc: 'TOM + 层次聚类 + 模块检测' },
  { key: 'module_trait', name: '模块-性状', icon: 'bi-table', desc: '模块与性状的相关与显著性' },
  { key: 'module_detail', name: '模块深入', icon: 'bi-search', desc: 'GS × MM 散点与模块基因热图' },
  { key: 'hub', name: 'Hub 基因', icon: 'bi-stars', desc: '按 kME / GS 阈值筛选核心基因' },
]

const DEPS: Record<string, string[]> = {
  filter: [], soft_threshold: ['filter'], network: ['soft_threshold'],
  module_trait: ['network'], module_detail: ['module_trait'], hub: ['module_trait'],
}

const exprFile = ref<File | null>(null)
const traitFile = ref<File | null>(null)
const nKeep = ref(8000)
const normalizeMethod = ref('log10p1')
const rsquaredCut = ref(0.85)

const jobId = ref('')
const job = ref<JobStatus | null>(null)
const creating = ref(false)
const stepRunning = ref('')
const error = ref('')
const traitUploaded = ref(false)

const sft = ref<SftResult | null>(null)
const modules = ref<ModulesResult | null>(null)
const mtait = ref<MtaitResult | null>(null)
const detail = ref<DetailResult | null>(null)
const hub = ref<HubResult | null>(null)

function onExpr(e: Event) { exprFile.value = (e.target as HTMLInputElement).files?.[0] ?? null }
function onTrait(e: Event) { traitFile.value = (e.target as HTMLInputElement).files?.[0] ?? null }

function params() {
  return {
    filter: { normalize_method: normalizeMethod.value, n_keep: nKeep.value, filter_method: 'mad', sample_percentage: 0, cutoff: 1 },
    soft_threshold: { network_type: 'unsigned', rsquared_cut: rsquaredCut.value },
    network: { power: 6, network_type: 'unsigned', tom_type: 'signed', tom_denom: 'min', min_module_size: 30, merge_cut_height: 0.25, deep_split: 2, detect_cut_height: 0.995 },
  }
}

async function create() {
  if (!exprFile.value || creating.value) return
  creating.value = true; error.value = ''
  try {
    const { id } = await createJob(exprFile.value, params())
    jobId.value = id
    if (traitFile.value) {
      await uploadTrait(id, traitFile.value)
      traitUploaded.value = true
    }
    await refresh()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    creating.value = false
  }
}

async function refresh() {
  if (!jobId.value) return
  job.value = await getJob(jobId.value)
}

function isDone(s: string) { return job.value?.done_steps?.includes(s) ?? false }
function stepReady(s: string) {
  if (!jobId.value) return false
  if (s === 'module_trait' || s === 'module_detail' || s === 'hub') {
    if (!traitUploaded.value) return false
  }
  return (DEPS[s] ?? []).every(isDone)
}

function sleep(ms: number) { return new Promise((r) => setTimeout(r, ms)) }

async function execStep(step: string) {
  if (!jobId.value || stepRunning.value) return
  stepRunning.value = step; error.value = ''
  try {
    await runStepApi(jobId.value, step)
    for (;;) {
      await refresh()
      if (job.value?.status === 'error') break
      if (job.value?.done_steps?.includes(step)) break
      await sleep(1200)
    }
    await loadStep(step)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    stepRunning.value = ''
  }
}

async function loadStep(step: string) {
  try {
    if (step === 'soft_threshold') sft.value = await getResult<SftResult>(jobId.value, step)
    else if (step === 'network') modules.value = await getResult<ModulesResult>(jobId.value, step)
    else if (step === 'module_trait') mtait.value = await getResult<MtaitResult>(jobId.value, step)
    else if (step === 'module_detail') detail.value = await getResult<DetailResult>(jobId.value, step)
    else if (step === 'hub') hub.value = await getResult<HubResult>(jobId.value, step)
  } catch { /* not available */ }
}

function stepStatus(s: string) {
  if (isDone(s)) return 'done'
  if (stepRunning.value === s) return 'running'
  if (job.value?.status === 'error' && job.value.current_step === s) return 'error'
  return 'idle'
}

const sftRows = computed(() => (sft.value?.fit_indices ?? []).map((f) => ({
  power: f.power, rsq: f.rsq.toFixed(3), slope: f.slope.toFixed(3), mean_k: f.mean_k.toFixed(1), median_k: f.median_k.toFixed(1), max_k: f.max_k.toFixed(1),
})))

const moduleSizeRows = computed(() => {
  const sizes = modules.value?.module_sizes ?? {}
  return Object.entries(sizes).map(([m, n]) => ({ module: m, count: n, color: moduleColor(m) })).sort((a, b) => b.count - a.count)
})

const hubRows = computed(() => (hub.value?.hub_genes ?? []).map((h) => ({ gene: h.gene, kme: h.kme.toFixed(4), gs: h.gs.toFixed(4) })))
</script>

<template>
  <div>
    <!-- 上传 -->
    <div class="card mb-3">
      <div class="card-body">
        <h6 class="panel-title mb-3"><i class="bi bi-upload me-2"></i>输入数据</h6>
        <div class="row g-3">
          <div class="col-md-5">
            <label class="form-label">表达矩阵文件<span class="text-danger">*</span></label>
            <input class="form-control" type="file" accept=".tsv,.txt" @change="onExpr" />
          </div>
          <div class="col-md-5">
            <label class="form-label">性状文件（可选）</label>
            <input class="form-control" type="file" accept=".tsv,.txt" @change="onTrait" />
          </div>
          <div class="col-md-2">
            <label class="form-label">保留基因数</label>
            <input class="form-control" type="number" v-model.number="nKeep" min="100" />
          </div>
        </div>
        <div class="d-flex align-items-center gap-2 mt-3">
          <button class="btn btn-primary" :disabled="!exprFile || creating" @click="create">
            <i v-if="creating" class="bi bi-arrow-repeat spinner-border spinner-border-sm me-1"></i>
            <i v-else class="bi bi-plus-circle me-1"></i>
            {{ creating ? '创建中…' : '创建作业' }}
          </button>
          <small class="text-muted">创建后按顺序逐步运行，每步可单独查看结果</small>
        </div>
        <p v-if="error" class="text-danger mt-2 mb-0">
          <i class="bi bi-exclamation-triangle me-1"></i>{{ error }}
          <button class="btn btn-sm btn-outline-danger ms-2" @click="analyzeError(error)">
            <i class="bi bi-robot me-1"></i>AI 分析
          </button>
        </p>
      </div>
    </div>

    <!-- 步骤列表 -->
    <div v-if="jobId" class="d-flex flex-column gap-3">
      <div v-for="(s, i) in STEPS" :key="s.key" class="card">
        <div class="card-body">
          <div class="d-flex align-items-center gap-3">
            <span class="fs-5 fw-bold text-muted">{{ i + 1 }}</span>
            <i :class="['bi', s.icon, 'fs-5', 'text-primary']"></i>
            <div class="flex-grow-1">
              <div class="fw-semibold">{{ s.name }}</div>
              <div class="text-muted small">{{ s.desc }}</div>
            </div>
            <span v-if="stepStatus(s.key) === 'done'" class="badge text-bg-success">已完成</span>
            <span v-else-if="stepStatus(s.key) === 'running'" class="badge text-bg-primary">
              <span class="spinner-border spinner-border-sm me-1"></span>运行中
            </span>
            <span v-else-if="stepStatus(s.key) === 'error'" class="badge text-bg-danger">出错</span>
            <span v-else class="badge text-bg-light text-muted">待运行</span>
            <button
              class="btn btn-sm"
              :class="isDone(s.key) ? 'btn-outline-secondary' : 'btn-primary'"
              :disabled="!stepReady(s.key) || stepRunning !== ''"
              @click="execStep(s.key)"
            >
              {{ stepRunning === s.key ? '运行中…' : isDone(s.key) ? '重跑' : '运行' }}
            </button>
          </div>

          <!-- 该步结果 -->
          <div v-if="s.key === 'soft_threshold' && sft" class="mt-3">
            <SftPlot :data="sft" :rsquared-cut="rsquaredCut" />
            <SimpleTable
              class="mt-2"
              :columns="[{ key: 'power', label: 'Power' }, { key: 'rsq', label: 'R²' }, { key: 'slope', label: 'Slope' }, { key: 'mean_k', label: 'Mean k' }, { key: 'median_k', label: 'Median k' }, { key: 'max_k', label: 'Max k' }]"
              :rows="sftRows"
            />
          </div>

          <div v-if="s.key === 'network' && modules" class="mt-3">
            <div class="d-flex flex-wrap gap-2 mb-3">
              <span v-for="r in moduleSizeRows" :key="r.module" class="badge text-bg-light border">
                <span class="d-inline-block rounded-circle me-1" :style="{ width: '10px', height: '10px', background: r.color }"></span>
                {{ r.module }}（{{ r.count }}）
              </span>
            </div>
            <DendrogramChart :genes="modules.genes" :colors="modules.colors" :dendrogram="modules.dendrogram" />
          </div>

          <div v-if="s.key === 'module_trait' && mtait" class="mt-3">
            <div class="row g-3">
              <div class="col-lg-7"><TraitHeatmap :data="mtait" /></div>
              <div class="col-lg-5"><EigengeneHeatmap :data="mtait" /></div>
            </div>
          </div>

          <div v-if="s.key === 'module_detail' && detail" class="mt-3">
            <div class="row g-3">
              <div class="col-lg-6"><GsMmScatter :data="detail" /></div>
              <div class="col-lg-6"><ModuleGeneHeatmap :data="detail" /></div>
            </div>
          </div>

          <div v-if="s.key === 'hub' && hub" class="mt-3">
            <p class="text-muted small">模块 {{ hub.module }} × 性状 {{ hub.trait_name }}，共 {{ hub.hub_genes.length }} 个 hub 基因</p>
            <SimpleTable
              :columns="[{ key: 'gene', label: 'Gene' }, { key: 'kme', label: 'kME' }, { key: 'gs', label: 'GS' }]"
              :rows="hubRows"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
