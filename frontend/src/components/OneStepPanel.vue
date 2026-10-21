<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  createJob, getJob, getResult, runAll, uploadTrait,
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
import { t } from '../utils/i18n'

const STEPS = ['filter', 'soft_threshold', 'network', 'module_trait', 'module_detail', 'hub']
const STEP_KEY: Record<string, string> = {
  filter: 'di.title', soft_threshold: 'sft.title', network: 'net.title',
  module_trait: 'mt.title', module_detail: 'im.title', hub: 'hub.title',
}
function stepName(k: string): string { return t(STEP_KEY[k] ?? k) }

const exprFile = ref<File | null>(null)
const traitFile = ref<File | null>(null)
const nKeep = ref(8000)
const normalizeMethod = ref('log10p1')
const filterMethod = ref('mad')
const rsquaredCut = ref(0.85)
const minModuleSize = ref(30)

const running = ref(false)
const error = ref('')
const job = ref<JobStatus | null>(null)
const jobId = ref('')

const sft = ref<SftResult | null>(null)
const modules = ref<ModulesResult | null>(null)
const mtait = ref<MtaitResult | null>(null)
const detail = ref<DetailResult | null>(null)
const hub = ref<HubResult | null>(null)

function onExpr(e: Event) { exprFile.value = (e.target as HTMLInputElement).files?.[0] ?? null }
function onTrait(e: Event) { traitFile.value = (e.target as HTMLInputElement).files?.[0] ?? null }

function params() {
  return {
    filter: {
      normalize_method: normalizeMethod.value, n_keep: nKeep.value, filter_method: filterMethod.value,
      sample_percentage: 0, cutoff: 1,
    },
    soft_threshold: { network_type: 'unsigned', rsquared_cut: rsquaredCut.value },
    network: {
      power: 6, network_type: 'unsigned', tom_type: 'signed', tom_denom: 'min',
      min_module_size: minModuleSize.value, merge_cut_height: 0.25, deep_split: 2, detect_cut_height: 0.995,
    },
  }
}

function resetResults() {
  job.value = null; sft.value = null; modules.value = null; mtait.value = null; detail.value = null; hub.value = null
}

function sleep(ms: number) { return new Promise((r) => setTimeout(r, ms)) }

async function poll(id: string) {
  for (;;) {
    const j = await getJob(id)
    job.value = j
    if (j.status !== 'running') break
    await sleep(1200)
  }
  if (job.value?.status === 'done') await loadResults(id)
}

async function loadResults(id: string) {
  try { sft.value = await getResult<SftResult>(id, 'soft_threshold') } catch { /* not ready */ }
  try { modules.value = await getResult<ModulesResult>(id, 'network') } catch { /* not ready */ }
  try { mtait.value = await getResult<MtaitResult>(id, 'module_trait') } catch { /* not ready */ }
  try { detail.value = await getResult<DetailResult>(id, 'module_detail') } catch { /* not ready */ }
  try { hub.value = await getResult<HubResult>(id, 'hub') } catch { /* not ready */ }
}

async function submit() {
  if (!exprFile.value || running.value) return
  running.value = true; error.value = ''
  resetResults()
  try {
    const { id } = await createJob(exprFile.value, params())
    jobId.value = id
    if (traitFile.value) await uploadTrait(id, traitFile.value)
    await runAll(id)
    await poll(id)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    running.value = false
  }
}

const doneCount = computed(() => job.value?.done_steps?.length ?? 0)
const currentIdx = computed(() => STEPS.indexOf(job.value?.current_step ?? ''))
const overallProgress = computed(() => {
  if (!job.value) return 0
  if (job.value.status === 'done') return 100
  const frac = job.value.progress?.progress ?? 0
  return Math.round(((doneCount.value + (job.value.status === 'running' ? frac : 0)) / STEPS.length) * 100)
})

function stepClass(i: number) {
  if (!job.value) return ''
  const s = STEPS[i]
  if (job.value.done_steps?.includes(s)) return 'done'
  if (job.value.status === 'error' && job.value.current_step === s) return 'error'
  if (i === currentIdx.value && job.value.status === 'running') return 'active'
  return ''
}

const sftRows = computed(() => (sft.value?.fit_indices ?? []).map((f) => ({
  power: f.power, rsq: f.rsq.toFixed(3), slope: f.slope.toFixed(3),
  mean_k: f.mean_k.toFixed(1), median_k: f.median_k.toFixed(1), max_k: f.max_k.toFixed(1),
})))

const moduleSizeRows = computed(() => {
  const sizes = modules.value?.module_sizes ?? {}
  return Object.entries(sizes).map(([m, n]) => ({ module: m, count: n, color: moduleColor(m) })).sort((a, b) => b.count - a.count)
})

const mtraitRows = computed(() => {
  if (!mtait.value) return []
  const rows: Record<string, string>[] = []
  mtait.value.modules.forEach((m, i) => mtait.value!.traits.forEach((t, j) => {
    rows.push({
      module: m, trait: t,
      cor: (mtait.value!.cor[i]?.[j] ?? 0).toFixed(3),
      p: (mtait.value!.p[i]?.[j] ?? 1).toExponential(2),
    })
  }))
  return rows
})

const hubRows = computed(() => (hub.value?.hub_genes ?? []).map((h) => ({
  gene: h.gene, kme: h.kme.toFixed(4), gs: h.gs.toFixed(4),
})))

const geneRowData = computed(() => (modules.value?.genes ?? []).map((g, i) => ({
  gene: g, color: modules.value?.colors[i] ?? 'grey',
})))
</script>

<template>
  <div>
    <!-- 上传 + 参数 -->
    <div class="card mb-3">
      <div class="card-body">
        <h6 class="panel-title mb-3"><i class="bi bi-upload me-2"></i>输入数据</h6>
        <div class="row g-3">
          <div class="col-md-6">
            <label class="form-label">{{ t('di.expr') }}<span class="text-danger">*</span></label>
            <input class="form-control" type="file" accept=".tsv,.txt" @change="onExpr" />
          </div>
          <div class="col-md-6">
            <label class="form-label">{{ t('di.trait') }}</label>
            <input class="form-control" type="file" accept=".tsv,.txt" @change="onTrait" />
          </div>
          <div class="col-md-3">
            <label class="form-label">归一化方法</label>
            <select class="form-select" v-model="normalizeMethod">
              <option value="log10p1">log10(x+1)（FPKM/计数）</option>
              <option value="log">log(x)（丰度）</option>
              <option value="raw">不归一化（已标准化）</option>
            </select>
          </div>
          <div class="col-md-3">
            <label class="form-label">保留基因数（MAD top-N）</label>
            <input class="form-control" type="number" v-model.number="nKeep" min="100" />
          </div>
          <div class="col-md-3">
            <label class="form-label">基因筛选方法</label>
            <select class="form-select" v-model="filterMethod">
              <option value="mad">MAD（中位绝对偏差）</option>
              <option value="var">VAR（方差）</option>
            </select>
          </div>
          <div class="col-md-3">
            <label class="form-label">R² 阈值</label>
            <input class="form-control" type="number" v-model.number="rsquaredCut" min="0" max="1" step="0.05" />
          </div>
        </div>
        <div class="d-flex align-items-center gap-2 mt-3">
          <button class="btn btn-primary" :disabled="!exprFile || running" @click="submit">
            <i v-if="running" class="bi bi-arrow-repeat spinner-border spinner-border-sm me-1"></i>
            <i v-else class="bi bi-lightning-charge-fill me-1"></i>
            {{ running ? t('common.running') : t('os.start') }}
          </button>
          <small class="text-muted">{{ t('os.flow') }}</small>
        </div>
        <p v-if="error" class="text-danger mt-2 mb-0"><i class="bi bi-exclamation-triangle me-1"></i>{{ error }}</p>
      </div>
    </div>

    <!-- 进度条 -->
    <div v-if="job" class="card mb-3">
      <div class="card-body">
        <div class="d-flex justify-content-between mb-2">
          <span class="fw-semibold">分析进度</span>
          <span class="text-muted small">{{ overallProgress }}%</span>
        </div>
        <div class="progress" style="height: 10px">
          <div
            class="progress-bar"
            :class="job.status === 'error' ? 'bg-danger' : 'progress-bar-striped progress-bar-animated'"
            :style="{ width: overallProgress + '%' }"
          ></div>
        </div>
        <div class="d-flex gap-1 mt-2">
          <div v-for="(s, i) in STEPS" :key="s" :class="['step-pill', stepClass(i)]">
            {{ stepName(s) }}
          </div>
        </div>
        <p v-if="job.progress?.message" class="text-muted small mt-2 mb-0">
          <i class="bi bi-info-circle me-1"></i>{{ job.progress.message }}
        </p>
        <p v-if="job.status === 'error'" class="text-danger small mt-2 mb-0">
          <i class="bi bi-x-octagon me-1"></i>{{ job.error }}
          <button class="btn btn-sm btn-outline-danger ms-2" @click="analyzeError(job.error || '')">
            <i class="bi bi-robot me-1"></i>{{ t('common.ai') }}
          </button>
        </p>
      </div>
    </div>

    <!-- 结果 -->
    <div v-if="job?.status === 'done'" class="row g-3">
      <div class="col-12">
        <div class="card">
          <div class="card-header bg-white fw-semibold"><i class="bi bi-activity me-2"></i>{{ t('sft.title') }}</div>
          <div class="card-body">
            <div v-if="sft">
              <SftPlot :data="sft" :rsquared-cut="rsquaredCut" />
              <p class="text-muted small mt-2 mb-2">
                推荐 power = <b>{{ sft.power_estimate }}</b>（{{ sft.network_type }} 网络）
              </p>
              <SimpleTable
                :columns="[
                  { key: 'power', label: 'Power' }, { key: 'rsq', label: 'R²' }, { key: 'slope', label: 'Slope' },
                  { key: 'mean_k', label: 'Mean k' }, { key: 'median_k', label: 'Median k' }, { key: 'max_k', label: 'Max k' },
                ]"
                :rows="sftRows"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="col-12">
        <div class="card">
          <div class="card-header bg-white fw-semibold"><i class="bi bi-diagram-3 me-2"></i>模块树状图</div>
          <div class="card-body">
            <div v-if="modules" class="d-flex flex-wrap gap-2 mb-3">
              <span v-for="r in moduleSizeRows" :key="r.module" class="badge text-bg-light border">
                <span class="d-inline-block rounded-circle me-1" :style="{ width: '10px', height: '10px', background: r.color }"></span>
                {{ r.module }}（{{ r.count }}）
              </span>
            </div>
            <DendrogramChart v-if="modules" :genes="modules.genes" :colors="modules.colors" :dendrogram="modules.dendrogram" />
          </div>
        </div>
      </div>

      <div class="col-12">
        <div class="card">
          <div class="card-header bg-white fw-semibold"><i class="bi bi-table me-2"></i>{{ t('mt.title') }}</div>
          <div class="card-body">
            <div v-if="mtait" class="row g-3 mb-3">
              <div class="col-lg-7"><TraitHeatmap :data="mtait" /></div>
              <div class="col-lg-5"><EigengeneHeatmap :data="mtait" /></div>
            </div>
            <SimpleTable
              v-if="mtait"
              class="mt-3"
              :columns="[{ key: 'module', label: 'Module' }, { key: 'trait', label: 'Trait' }, { key: 'cor', label: 'Correlation' }, { key: 'p', label: 'p-value' }]"
              :rows="mtraitRows"
            />
          </div>
        </div>
      </div>

      <div class="col-12">
        <div class="card">
          <div class="card-header bg-white fw-semibold"><i class="bi bi-search me-2"></i>{{ t('im.title') }}</div>
          <div class="card-body">
            <div v-if="detail" class="row g-3">
              <div class="col-lg-6"><GsMmScatter :data="detail" /></div>
              <div class="col-lg-6"><ModuleGeneHeatmap :data="detail" /></div>
            </div>
          </div>
        </div>
      </div>

      <div class="col-12">
        <div class="card">
          <div class="card-header bg-white fw-semibold"><i class="bi bi-stars me-2"></i>{{ t('hub.title') }}</div>
          <div class="card-body">
            <p v-if="hub" class="text-muted small">
              模块 {{ hub.module }} × 性状 {{ hub.trait_name }}，共 {{ hub.hub_genes.length }} 个 hub 基因（|kME| &gt; 0.8 且 |GS| &gt; 0.5）
            </p>
            <SimpleTable
              v-if="hub"
              :columns="[{ key: 'gene', label: 'Gene' }, { key: 'kme', label: 'kME' }, { key: 'gs', label: 'GS' }]"
              :rows="hubRows"
            />
          </div>
        </div>
      </div>

      <div class="col-12">
        <div class="card">
          <div class="card-header bg-white fw-semibold"><i class="bi bi-list-ul me-2"></i>基因 → 模块</div>
          <div class="card-body p-0">
            <SimpleTable
              :columns="[{ key: 'gene', label: 'Gene' }, { key: 'color', label: 'Module' }]"
              :rows="geneRowData"
              scroll-height="500px"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
