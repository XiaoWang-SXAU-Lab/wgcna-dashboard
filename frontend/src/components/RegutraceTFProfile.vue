<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  getRegutraceTFTargets,
  type RegutraceMotifPolicy,
  type RegutracePolicyInfo,
  type RegutraceTFTargetRow,
  type RegutraceTFView,
} from '../api/client'
import { locale } from '../utils/i18n'
import RegutraceSequenceLogo from './RegutraceSequenceLogo.vue'

const props = defineProps<{
  view: RegutraceTFView
  jobId: string
  selectedMotif?: string
}>()
const emit = defineEmits<{
  (e: 'open-target', target: string): void
  (e: 'open-motif', motif: string): void
}>()

const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const targetRows = ref<RegutraceTFTargetRow[]>([])
const targetTotal = ref(0)
const targetPage = ref(1)
const targetPageSize = 25
const targetBusy = ref(false)
const targetError = ref('')
const targetMotif = ref('')
const targetStatus = ref('')
const targetPolicy = ref<RegutraceMotifPolicy>('all')
const policyInfo = ref<RegutracePolicyInfo | null>(null)
const targetMaxP = ref('')
const showSequence = ref(false)
let targetSeq = 0

const mappingStatus = computed(() => props.view.mapping_status || 'not_computed')
const mappingLabel = computed(() => statusLabel(mappingStatus.value))
const mappingClass = computed(() => statusClass(mappingStatus.value))
const mappings = computed(() => props.view.motif_mappings ?? [])
const primaryMapping = computed(() => {
  if (props.selectedMotif) {
    return mappings.value.find((mapping) => mapping.motif_id === props.selectedMotif) ?? mappings.value[0]
  }
  return mappings.value[0]
})
const proteinSequence = computed(() => {
  const protein = props.view.proteins?.find((item) => item.protein_id === props.view.default_transcript) ??
    props.view.proteins?.[0]
  return protein?.sequence ?? ''
})
const targetPageCount = computed(() => Math.max(1, Math.ceil(targetTotal.value / targetPageSize)))
const statuses = computed(() => [...new Set(mappings.value.map((mapping) => mapping.mapping_status))])

function statusClass(value: string): string {
  switch (value) {
    case 'strict_rbh': return 'text-bg-success'
    case 'dbd_calibrated': return 'text-bg-primary'
    case 'dbd_ambiguous': return 'text-bg-warning'
    case 'dbd_exploratory': return 'text-bg-info'
    case 'no_supported_motif': return 'text-bg-secondary'
    default: return 'text-bg-light border'
  }
}
function statusLabel(value: string): string {
  switch (value) {
    case 'strict_rbh': return text('Strict RBH', 'Strict RBH')
    case 'dbd_calibrated': return text('Calibrated DBD', 'Calibrated DBD')
    case 'dbd_ambiguous': return text('Ambiguous DBD', 'Ambiguous DBD')
    case 'dbd_exploratory': return text('Exploratory DBD', 'Exploratory DBD')
    case 'no_supported_motif': return text('无支持 motif', 'No supported motif')
    case 'not_computed': return text('未计算', 'Not computed')
    default: return value
  }
}
function policyLabel(value: RegutraceMotifPolicy): string {
  switch (value) {
    case 'best': return text('每组最佳支持（推荐扩展候选）', 'Best-supported per group (recommended extended)')
    case 'reps': return text('每组代表 motif', 'Representative per specificity group')
    default: return text('全部候选 motif', 'All candidate motifs')
  }
}
function testabilityLabel(value?: string): string {
  switch (value) {
    case 'significant_hits': return text('有显著位点', 'Significant hits')
    case 'no_significant_hits': return text('无可达位点', 'No significant hits')
    case 'threshold_unattainable': return text('阈值不可达', 'Threshold unattainable')
    case 'not_scanned': return text('未扫描', 'Not scanned')
    case 'not_computed': return text('可测试性未计算', 'Testability not computed')
    default: return value || '—'
  }
}
function formatP(value?: number): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return value < 1e-3 || value >= 1e4 ? value.toExponential(2) : value.toPrecision(3)
}
function formatNumber(value?: number): string {
  return value == null ? '—' : value.toLocaleString()
}

async function loadTargets() {
  if (!props.jobId) return
  const seq = ++targetSeq
  targetBusy.value = true
  targetError.value = ''
  try {
    const maxP = targetMaxP.value.trim() === '' ? undefined : Number(targetMaxP.value)
    const res = await getRegutraceTFTargets(props.jobId, props.view.tf_id, {
      limit: targetPageSize,
      offset: (targetPage.value - 1) * targetPageSize,
      motif: targetMotif.value || undefined,
      mapping_status: targetStatus.value || undefined,
      motif_policy: targetPolicy.value,
      max_p: Number.isFinite(maxP as number) ? maxP : undefined,
    })
    if (seq !== targetSeq) return
    targetRows.value = res.rows
    targetTotal.value = res.total
    policyInfo.value = res.motif_policy ?? null
  } catch (e) {
    if (seq === targetSeq) targetError.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === targetSeq) targetBusy.value = false
  }
}
function searchTargets() {
  targetPage.value = 1
  void loadTargets()
}
function changePage(delta: number) {
  targetPage.value = Math.max(1, targetPage.value + delta)
  void loadTargets()
}

watch(() => [props.view.tf_id, props.selectedMotif], () => {
  targetMotif.value = props.selectedMotif ?? ''
  targetPage.value = 1
  targetStatus.value = ''
  showSequence.value = false
  void loadTargets()
}, { immediate: true })
</script>

<template>
  <div class="card mb-3">
    <div class="card-body">
      <div class="d-flex align-items-start flex-wrap gap-2 mb-2">
        <div>
          <h6 class="mb-0">{{ text('TF 概览与候选靶标', 'TF overview and candidate targets') }}</h6>
          <div class="small text-muted">
            {{ view.tf_id }} · {{ view.tf_family || text('未分家族', 'unassigned family') }} ·
            <span class="badge" :class="mappingClass">{{ mappingLabel }}</span>
          </div>
        </div>
        <div class="ms-auto small text-end">
          <div>{{ text('候选 target', 'Candidate targets') }}: <b>{{ formatNumber(view.target_count) }}</b></div>
          <div v-if="view.shared_pwm_tf_count > 1" class="text-warning-emphasis">
            {{ text('共享 PWM 的 TF', 'TFs sharing PWM') }}: {{ view.shared_pwm_tf_count }}
          </div>
        </div>
      </div>

      <div v-if="view.diagnostic_reason" class="alert alert-warning small py-2">
        {{ view.diagnostic_reason }}
      </div>
      <div v-if="!mappings.length" class="alert alert-light border small py-2">
        {{ text('没有可用的 TF–motif 分配，因此没有 target 面板。可在下方 motif registry 查看该家族是否有实验 motif 资源；这不代表该 TF 有 motif。', 'No TF–motif assignment, so there is no target panel. The motif registry below may show a family-associated experimental resource; that does not mean this TF has a motif.') }}
      </div>

      <div v-if="mappings.length" class="row g-3">
        <div class="col-12 col-xl-5">
          <div class="small text-muted mb-1">{{ text('Motif 分配', 'Motif assignments') }}</div>
          <div class="table-responsive" style="max-height: 17rem; overflow: auto">
            <table class="table table-sm align-middle mb-0">
              <thead><tr><th>Motif</th><th>{{ text('状态', 'Status') }}</th><th>{{ text('可测试性', 'Testability') }}</th><th>p floor</th></tr></thead>
              <tbody>
                <tr v-for="mapping in mappings" :key="`${mapping.motif_id}-${mapping.reference_tf_id}`" role="button" @click="emit('open-motif', mapping.motif_id)">
                  <td>
                    <span class="font-monospace">{{ mapping.motif_id }}</span>
                    <div class="small text-muted">{{ mapping.motif_name }}</div>
                    <div v-if="mapping.shared_pwm_tf_count && mapping.shared_pwm_tf_count > 1" class="small text-warning-emphasis">
                      {{ text('共享 PWM', 'shared PWM') }} × {{ mapping.shared_pwm_tf_count }}
                    </div>
                  </td>
                  <td>
                    <span class="badge" :class="statusClass(mapping.mapping_status)"
                          :title="[mapping.mapping_method, mapping.mapping_confidence].filter(Boolean).join(' · ')">
                      {{ statusLabel(mapping.mapping_status) }}
                    </span>
                    <span v-if="mapping.domain_mode === 'single_domain_fallback'"
                          class="badge text-bg-secondary ms-1"
                          :title="text('完整 DBD 架构提取失败，使用单域比对回退；不按 full-architecture 优先', 'Full DBD architecture extraction failed; single-domain alignment fallback is used and is not ranked as full architecture')">
                      {{ text('单域比对回退', 'single-domain alignment fallback') }}
                    </span>
                  </td>
                  <td class="small">{{ testabilityLabel(mapping.testability_status) }}</td>
                  <td class="small">{{ formatP(mapping.min_pvalue) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="primaryMapping?.matrix?.length" class="mt-2">
            <RegutraceSequenceLogo :matrix="primaryMapping.matrix" :height="170" :title="`${primaryMapping.motif_id} · ${text('实际 PWM', 'actual PWM')}`" />
          </div>
        </div>
        <div class="col-12 col-xl-7">
          <div class="small text-muted mb-1">{{ text('候选靶标（按 best p 排序，分页）', 'Candidate targets (best p, paginated)') }}</div>
          <div class="row g-1 align-items-end mb-2">
            <div class="col-6 col-md-2">
              <label class="form-label small mb-0">Motif</label>
              <select v-model="targetMotif" class="form-select form-select-sm" @change="searchTargets">
                <option value="">{{ text('全部', 'All') }}</option>
                <option v-for="mapping in mappings" :key="mapping.motif_id" :value="mapping.motif_id">{{ mapping.motif_id }}</option>
              </select>
            </div>
            <div class="col-6 col-md-3">
              <label class="form-label small mb-0">{{ text('Motif 选择策略', 'Motif policy') }}</label>
              <select v-model="targetPolicy" class="form-select form-select-sm" @change="searchTargets">
                <option value="all">{{ policyLabel('all') }}</option>
                <option value="best">{{ policyLabel('best') }}</option>
                <option value="reps">{{ policyLabel('reps') }}</option>
              </select>
            </div>
            <div class="col-6 col-md-2">
              <label class="form-label small mb-0">{{ text('映射状态', 'Mapping status') }}</label>
              <select v-model="targetStatus" class="form-select form-select-sm" @change="searchTargets">
                <option value="">{{ text('全部', 'All') }}</option>
                <option v-for="status in statuses" :key="status" :value="status">{{ statusLabel(status) }}</option>
              </select>
            </div>
            <div class="col-6 col-md-2">
              <label class="form-label small mb-0">max p</label>
              <input v-model="targetMaxP" class="form-control form-control-sm" placeholder="any" @keyup.enter="searchTargets" />
            </div>
            <div class="col-6 col-md-2">
              <button class="btn btn-sm btn-outline-primary w-100" :disabled="targetBusy" @click="searchTargets">{{ text('筛选', 'Filter') }}</button>
            </div>
          </div>
          <div v-if="policyInfo" class="small text-muted mb-2">
            {{ policyInfo.selected_motifs }} motif(s) ·
            {{ text('固定 strict/calibrated', 'fixed strict/calibrated') }}: {{ policyInfo.fixed_mappings }} ·
            {{ text('探索性入选', 'exploratory selected') }}: {{ policyInfo.exploratory_selected }}
            <span v-if="policyInfo.uncoordinated_motifs"> · {{ text('无坐标排除', 'without coordinates') }}: {{ policyInfo.uncoordinated_motifs }}</span>
            <span v-if="policyInfo.fallback" class="text-warning-emphasis"> · {{ text('已回退', 'fallback') }}</span>
            <div v-if="policyInfo.note" class="text-warning-emphasis">{{ policyInfo.note }}</div>
          </div>
          <div v-if="targetError" class="alert alert-warning small py-2">{{ targetError }}</div>
          <div v-if="targetBusy" class="small text-muted"><span class="spinner-border spinner-border-sm me-2"></span>{{ text('读取 target…', 'Loading targets…') }}</div>
          <div v-else class="table-responsive" style="max-height: 21rem; overflow: auto">
            <table class="table table-sm align-middle" style="min-width: 620px">
              <thead><tr><th>Target</th><th style="white-space:nowrap">best p</th><th>{{ text('位点', 'sites') }}</th><th>Motif</th><th>{{ text('状态', 'Status') }}</th><th></th></tr></thead>
              <tbody>
                <tr v-for="row in targetRows" :key="row.target_id">
                  <td><span class="fw-semibold">{{ row.target_symbol }}</span><div class="small text-muted">{{ row.target_id }}</div></td>
                  <td class="small" style="white-space:nowrap">{{ formatP(row.best_p_value) }}</td>
                  <td>{{ row.site_count }}</td>
                  <td class="small" :title="(row.motif_ids ?? []).join(', ')">
                    {{ (row.motif_ids ?? []).length }} motif(s)
                    <div class="text-muted">{{ (row.motif_ids ?? []).slice(0, 2).join(', ') }}<span v-if="(row.motif_ids ?? []).length > 2"> …</span></div>
                  </td>
                  <td class="small">
                    <span v-for="status in row.mapping_statuses" :key="status" class="badge me-1" :class="statusClass(status)">{{ statusLabel(status) }}</span>
                    <div v-if="row.shared_motif_tfs > 1" class="text-warning-emphasis">{{ text('共享 PWM', 'shared PWM') }} × {{ row.shared_motif_tfs }}</div>
                  </td>
                  <td><button class="btn btn-sm btn-outline-secondary" @click="emit('open-target', row.target_id)">{{ text('位点', 'Sites') }}</button></td>
                </tr>
                <tr v-if="!targetRows.length"><td colspan="6" class="small text-muted">{{ text('没有符合条件的候选 target。', 'No candidate target passes the filters.') }}</td></tr>
              </tbody>
            </table>
          </div>
          <div class="d-flex gap-2 align-items-center">
            <button class="btn btn-sm btn-outline-secondary" :disabled="targetPage <= 1" @click="changePage(-1)">{{ text('上一页', 'Previous') }}</button>
            <span class="small">{{ targetPage }} / {{ targetPageCount }} · {{ targetTotal.toLocaleString() }}</span>
            <button class="btn btn-sm btn-outline-secondary" :disabled="targetPage >= targetPageCount" @click="changePage(1)">{{ text('下一页', 'Next') }}</button>
          </div>
        </div>
      </div>

      <div v-if="proteinSequence" class="mt-3">
        <button class="btn btn-sm btn-outline-secondary" @click="showSequence = !showSequence">
          {{ showSequence ? text('隐藏蛋白序列', 'Hide protein sequence') : text('查看蛋白序列', 'Show protein sequence') }}
        </button>
        <pre v-if="showSequence" class="small bg-light border rounded p-2 mt-2 mb-0" style="white-space: pre-wrap; max-height: 12rem; overflow: auto">{{ proteinSequence }}</pre>
      </div>
      <p class="small text-muted mt-2 mb-0">
        {{ text('Motif 策略只改变当前 target 视图：全部候选保留在库中，strict/calibrated 映射始终保留，ambiguous 不进入 best/reps 视图。全量 evidence/background 导出保留全部原始候选；policy 只影响 target/regulator 查询和 v3 tf_motif_policy 表。best/reps 的排序只使用 architecture、coverage、identity 和 bitscore，不使用 DNA-contact、reciprocity 或 full-length similarity。所有 PWM promoter 命中都是计算候选（evidence level D）；映射状态和位点 p 值都不是实验验证或真实调控概率。', 'The motif policy changes only the current target view: all candidate motifs remain stored, strict/calibrated mappings are always retained, and ambiguous mappings do not enter best/reps views. Full evidence/background exports retain all original candidates; policy-specific access is the target/regulator query and the v3 tf_motif_policy table. best/reps ranking uses only architecture, coverage, identity and bitscore; it does not use DNA-contact residues, reciprocity or full-length similarity. All PWM promoter hits are computational candidates (evidence level D); mapping status and site p-values are not experimental validation or true regulatory probabilities.') }}
      </p>
    </div>
  </div>
</template>
