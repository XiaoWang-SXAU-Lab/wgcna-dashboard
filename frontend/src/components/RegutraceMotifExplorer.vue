<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  getRegutraceMotif,
  getRegutraceMotifs,
  type RegutraceMotifDetail,
  type RegutraceMotifSummary,
} from '../api/client'
import { locale } from '../utils/i18n'
import RegutraceSequenceLogo from './RegutraceSequenceLogo.vue'

const props = defineProps<{ jobId: string }>()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en

const query = ref('')
const family = ref('')
const species = ref('')
const status = ref('')
const page = ref(1)
const pageSize = 25
const total = ref(0)
const rows = ref<RegutraceMotifSummary[]>([])
const busy = ref(false)
const error = ref('')
const detail = ref<RegutraceMotifDetail | null>(null)
const detailBusy = ref(false)
const detailError = ref('')
let listSeq = 0
let detailSeq = 0

const statusOptions = [
  { value: '', label: text('全部状态', 'All statuses') },
  { value: 'significant_hits', label: text('有显著位点', 'Significant hits') },
  { value: 'no_significant_hits', label: text('无可达位点', 'No significant hits') },
  { value: 'threshold_unattainable', label: text('阈值不可达', 'Threshold unattainable') },
  { value: 'not_scanned', label: text('未扫描', 'Not scanned') },
  { value: 'not_computed', label: text('未计算', 'Not computed') },
]

async function load() {
  const seq = ++listSeq
  busy.value = true
  error.value = ''
  try {
    const res = await getRegutraceMotifs(props.jobId, {
      q: query.value.trim(), family: family.value.trim(), species: species.value.trim(),
      status: status.value, limit: pageSize, offset: (page.value - 1) * pageSize,
    })
    if (seq !== listSeq) return
    rows.value = res.rows
    total.value = res.total
  } catch (e) {
    if (seq === listSeq) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === listSeq) busy.value = false
  }
}

async function open(motifId: string) {
  const seq = ++detailSeq
  detailBusy.value = true
  detailError.value = ''
  detail.value = null
  try {
    const res = await getRegutraceMotif(props.jobId, motifId)
    if (seq === detailSeq) detail.value = res
  } catch (e) {
    if (seq === detailSeq) detailError.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === detailSeq) detailBusy.value = false
  }
}

function search() {
  page.value = 1
  void load()
}
function nextPage(delta: number) {
  page.value = Math.max(1, page.value + delta)
  void load()
}
function formatP(value?: number): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return value < 1e-3 || value >= 1e4 ? value.toExponential(2) : value.toPrecision(3)
}
function statusClass(value: string): string {
  if (value === 'significant_hits') return 'text-bg-success'
  if (value === 'no_significant_hits') return 'text-bg-info'
  if (value === 'threshold_unattainable') return 'text-bg-warning'
  if (value === 'not_scanned') return 'text-bg-secondary'
  return 'text-bg-light border'
}
function statusLabel(value: string): string {
  const found = statusOptions.find((option) => option.value === value)
  return found?.label ?? value
}

onMounted(load)
</script>

<template>
  <div class="card mb-3">
    <div class="card-body">
      <div class="d-flex align-items-start flex-wrap gap-2 mb-2">
        <div>
          <h6 class="mb-0">{{ text('Motif / cis-element registry', 'Motif / cis-element registry') }}</h6>
          <p class="small text-muted mb-0">
            {{ text('浏览参考库中的实验 motif。family-associated 资源不等于某个 TF 的个体 motif 分配。', 'Browse experimental motifs in the reference pack. A family-associated resource is not an individual TF assignment.') }}
          </p>
        </div>
        <span class="badge text-bg-light border ms-auto">{{ total.toLocaleString() }} motifs</span>
      </div>
      <div class="row g-2 align-items-end mb-2">
        <div class="col-12 col-md-4">
          <label class="form-label small mb-1">{{ text('搜索 motif / family / donor', 'Search motif / family / donor') }}</label>
          <input v-model="query" class="form-control form-control-sm" :placeholder="text('MA0001 / MYB / Arabidopsis', 'MA0001 / MYB / Arabidopsis')" @keyup.enter="search" />
        </div>
        <div class="col-6 col-md-2">
          <label class="form-label small mb-1">{{ text('家族', 'Family') }}</label>
          <input v-model="family" class="form-control form-control-sm" @keyup.enter="search" />
        </div>
        <div class="col-6 col-md-2">
          <label class="form-label small mb-1">{{ text('donor 物种', 'Donor species') }}</label>
          <input v-model="species" class="form-control form-control-sm" @keyup.enter="search" />
        </div>
        <div class="col-6 col-md-2">
          <label class="form-label small mb-1">{{ text('可测试性', 'Testability') }}</label>
          <select v-model="status" class="form-select form-select-sm" @change="search">
            <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </div>
        <div class="col-6 col-md-2">
          <button class="btn btn-primary btn-sm w-100" :disabled="busy" @click="search">
            <i class="bi bi-search me-1"></i>{{ busy ? text('查询中…', 'Searching…') : text('查询', 'Search') }}
          </button>
        </div>
      </div>
      <div v-if="error" class="alert alert-danger small py-2">{{ error }}</div>
      <div class="table-responsive" style="max-height: 25rem; overflow: auto">
        <table class="table table-sm align-middle">
          <thead><tr>
            <th>Motif</th><th>{{ text('家族', 'Family') }}</th><th>{{ text('物种', 'Species') }}</th>
            <th>{{ text('长度', 'Length') }}</th><th>IC</th><th>{{ text('映射 TF', 'Mapped TFs') }}</th>
            <th>{{ text('可测试性', 'Testability') }}</th><th>p floor</th>
          </tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" role="button" @click="open(row.id)">
              <td>
                <span class="font-monospace">{{ row.id }}</span>
                <div class="small text-muted">{{ row.name }}</div>
                <span v-if="row.family_resource" class="badge text-bg-light border">{{ text('family 资源', 'family resource') }}</span>
              </td>
              <td>{{ row.family || '—' }}</td>
              <td class="small">{{ (row.donor_species ?? []).join(', ') || '—' }}</td>
              <td>{{ row.length }}</td>
              <td>{{ row.information_content.toFixed(2) }}</td>
              <td class="small" :title="(row.mapped_tfs ?? []).join(', ')">
                <template v-if="row.mapped_tfs?.length">
                  {{ row.mapped_tfs.length }} TF(s)
                  <div class="text-muted">{{ row.mapped_tfs.slice(0, 3).join(', ') }}<span v-if="row.mapped_tfs.length > 3"> …</span></div>
                </template>
                <span v-else class="text-muted">—</span>
              </td>
              <td style="white-space:nowrap"><span class="badge" :class="statusClass(row.testability_status)">{{ statusLabel(row.testability_status) }}</span></td>
              <td class="small" style="white-space:nowrap">{{ formatP(row.min_pvalue) }}</td>
            </tr>
            <tr v-if="!rows.length && !busy"><td colspan="8" class="text-muted small">{{ text('没有匹配的 motif。', 'No matching motif.') }}</td></tr>
          </tbody>
        </table>
      </div>
      <div class="d-flex gap-2 align-items-center">
        <button class="btn btn-sm btn-outline-secondary" :disabled="page <= 1" @click="nextPage(-1)">{{ text('上一页', 'Previous') }}</button>
        <span class="small">{{ page }} / {{ Math.max(1, Math.ceil(total / pageSize)) }}</span>
        <button class="btn btn-sm btn-outline-secondary" :disabled="page >= Math.ceil(total / pageSize)" @click="nextPage(1)">{{ text('下一页', 'Next') }}</button>
      </div>

      <div v-if="detailBusy" class="small text-muted mt-3"><span class="spinner-border spinner-border-sm me-2"></span>{{ text('读取 motif…', 'Loading motif…') }}</div>
      <div v-else-if="detailError" class="alert alert-warning small mt-3 mb-0">{{ detailError }}</div>
      <div v-else-if="detail" class="border-top mt-3 pt-3">
        <div class="d-flex align-items-start flex-wrap gap-2">
          <div>
            <h6 class="mb-0">{{ detail.id }} <span class="text-muted fw-normal">{{ detail.name }}</span></h6>
            <div class="small text-muted">
              {{ detail.family || text('未分家族', 'unassigned family') }} · {{ (detail.donor_species ?? []).join(', ') || '—' }}
              · {{ detail.length }} bp · IC {{ detail.information_content.toFixed(2) }} bits
            </div>
          </div>
          <span class="badge ms-auto" :class="statusClass(detail.testability_status)">{{ statusLabel(detail.testability_status) }}</span>
        </div>
        <p v-if="detail.note" class="small text-warning-emphasis mt-2 mb-2">{{ detail.note }}</p>
        <div class="row g-3 mt-1">
          <div class="col-12 col-lg-7">
            <RegutraceSequenceLogo :matrix="detail.matrix" :title="text('由实际 PWM 矩阵计算', 'Computed from the actual PWM matrix')" />
          </div>
          <div class="col-12 col-lg-5">
            <table class="table table-sm small mb-2">
              <tbody>
                <tr><th class="text-muted">{{ text('PWM hash', 'PWM hash') }}</th><td class="font-monospace text-break">{{ detail.pwm_hash || '—' }}</td></tr>
                <tr><th class="text-muted">{{ text('最小可达 p', 'Minimum attainable p') }}</th><td>{{ formatP(detail.min_pvalue) }}</td></tr>
                <tr><th class="text-muted">{{ text('请求阈值', 'Requested cutoff') }}</th><td>{{ formatP(detail.requested_pvalue) }}</td></tr>
                <tr><th class="text-muted">{{ text('背景', 'Background') }}</th><td>{{ detail.background_mode || '—' }}<span v-if="detail.background?.length"> · {{ detail.background.map((v) => v.toFixed(3)).join(', ') }}</span></td></tr>
                <tr><th class="text-muted">{{ text('共享 PWM TF 数', 'TFs sharing PWM') }}</th><td>{{ detail.shared_pwm_tf_count }}</td></tr>
                <tr><th class="text-muted">{{ text('靶基因数', 'Target genes') }}</th><td>{{ detail.target_count.toLocaleString() }}</td></tr>
              </tbody>
            </table>
            <div class="small text-muted">{{ text('TF 映射', 'TF mappings') }}</div>
            <div v-if="!detail.tfs.length" class="small text-muted">{{ text('没有个体 TF 映射；这是 family-associated 资源。', 'No individual TF mapping; this is a family-associated resource.') }}</div>
            <div v-for="tf in detail.tfs" :key="tf.tf_id" class="small">
              <span class="fw-semibold">{{ tf.tf_symbol || tf.tf_id }}</span>
              <span class="text-muted"> · {{ tf.tf_family || '—' }} · {{ tf.mapping_status }}</span>
            </div>
            <div v-if="detail.specificity_groups?.length" class="small mt-1">
              <span class="text-muted">{{ text('specificity group', 'specificity group') }}:</span> {{ detail.specificity_groups.join(', ') }}
            </div>
          </div>
        </div>
        <div v-if="detail.registry?.length" class="table-responsive mt-2">
          <table class="table table-sm small">
            <thead><tr><th>Donor</th><th>{{ text('来源', 'Source') }}</th><th>{{ text('方法', 'Method') }}</th><th>{{ text('物种', 'Species') }}</th><th>{{ text('许可', 'License') }}</th></tr></thead>
            <tbody><tr v-for="(entry, index) in detail.registry" :key="index">
              <td>{{ entry.donor_id || '—' }}</td><td>{{ entry.source || '—' }} {{ entry.source_version || '' }}</td>
              <td>{{ entry.method || '—' }}</td><td>{{ entry.species || '—' }}</td><td>{{ entry.license || '—' }}</td>
            </tr></tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
