<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getRegutraceRegulators, type RegutraceRegulatorRow } from '../api/client'
import { locale } from '../utils/i18n'

const props = defineProps<{ jobId: string; targetId: string }>()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const rows = ref<RegutraceRegulatorRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 25
const busy = ref(false)
const error = ref('')
const status = ref('')
const motif = ref('')
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
let loadSeq = 0

function statusLabel(value: string): string {
  switch (value) {
    case 'strict_rbh': return 'Strict RBH'
    case 'dbd_calibrated': return 'Calibrated DBD'
    case 'dbd_ambiguous': return 'Ambiguous DBD'
    case 'dbd_exploratory': return 'Exploratory DBD'
    case 'no_supported_motif': return 'No supported motif'
    default: return value
  }
}
function statusClass(value: string): string {
  if (value === 'strict_rbh') return 'text-bg-success'
  if (value === 'dbd_calibrated') return 'text-bg-primary'
  if (value === 'dbd_ambiguous') return 'text-bg-warning'
  if (value === 'dbd_exploratory') return 'text-bg-info'
  return 'text-bg-secondary'
}
function formatP(value?: number): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return value < 1e-3 ? value.toExponential(2) : value.toPrecision(3)
}
async function load() {
  if (!props.jobId || !props.targetId) return
  const seq = ++loadSeq
  busy.value = true
  error.value = ''
  try {
    const res = await getRegutraceRegulators(props.jobId, props.targetId, {
      limit: pageSize, offset: (page.value - 1) * pageSize,
      mapping_status: status.value || undefined, motif: motif.value || undefined,
    })
    if (seq !== loadSeq) return
    rows.value = res.rows
    total.value = res.total
  } catch (e) {
    if (seq === loadSeq) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === loadSeq) busy.value = false
  }
}
function search() { page.value = 1; void load() }
function changePage(delta: number) { page.value = Math.max(1, page.value + delta); void load() }
watch(() => [props.jobId, props.targetId], () => { page.value = 1; status.value = ''; motif.value = ''; void load() }, { immediate: true })
</script>

<template>
  <div class="card mb-3">
    <div class="card-body">
      <div class="d-flex align-items-start flex-wrap gap-2 mb-2">
        <div>
          <h6 class="mb-0">{{ text('Target → candidate regulators', 'Target → candidate regulators') }}</h6>
          <p class="small text-muted mb-0">{{ text('按当前 mapping mode 聚合；共享 PWM 的多个 TF 不是独立证据。', 'Aggregated under the current mapping mode; TFs sharing a PWM are not independent evidence.') }}</p>
        </div>
        <span class="badge text-bg-light border ms-auto">{{ total.toLocaleString() }} TFs</span>
      </div>
      <div class="d-flex flex-wrap gap-2 mb-2">
        <select v-model="status" class="form-select form-select-sm" style="width: 13rem" @change="search">
          <option value="">{{ text('全部映射状态', 'All mapping statuses') }}</option>
          <option value="strict_rbh">Strict RBH</option>
          <option value="dbd_calibrated">Calibrated DBD</option>
          <option value="dbd_ambiguous">Ambiguous DBD</option>
          <option value="dbd_exploratory">Exploratory DBD</option>
        </select>
        <input v-model="motif" class="form-control form-control-sm" style="width: 11rem" placeholder="Motif ID" @keyup.enter="search" />
        <button class="btn btn-sm btn-outline-primary" :disabled="busy" @click="search">{{ text('筛选', 'Filter') }}</button>
      </div>
      <div v-if="error" class="alert alert-warning small py-2">{{ error }}</div>
      <div class="table-responsive" style="max-height: 24rem; overflow: auto">
        <table class="table table-sm">
          <thead><tr><th>TF</th><th>best p</th><th>{{ text('位点', 'sites') }}</th><th>Motif</th><th>{{ text('状态', 'Status') }}</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.tf_id">
              <td><span class="fw-semibold">{{ row.tf_symbol }}</span><div class="small text-muted">{{ row.tf_id }} · {{ row.tf_family || '—' }}</div></td>
              <td style="white-space:nowrap">{{ formatP(row.best_p_value) }}</td>
              <td>{{ row.site_count }}</td>
              <td class="small">{{ (row.motif_ids ?? []).slice(0, 3).join(', ') }}<span v-if="(row.motif_ids ?? []).length > 3"> …</span></td>
              <td class="small">
                <span v-for="s in row.mapping_statuses" :key="s" class="badge me-1" :class="statusClass(s)">{{ statusLabel(s) }}</span>
                <div v-if="row.shared_motif_tfs > 1" class="text-warning-emphasis">{{ text('共享 PWM', 'shared PWM') }} × {{ row.shared_motif_tfs }}</div>
              </td>
            </tr>
            <tr v-if="!rows.length && !busy"><td colspan="5" class="small text-muted">{{ text('没有候选 regulator。', 'No candidate regulator.') }}</td></tr>
          </tbody>
        </table>
      </div>
      <div class="d-flex gap-2 align-items-center">
        <button class="btn btn-sm btn-outline-secondary" :disabled="page <= 1" @click="changePage(-1)">{{ text('上一页', 'Previous') }}</button>
        <span class="small">{{ page }} / {{ pageCount }} · {{ total.toLocaleString() }}</span>
        <button class="btn btn-sm btn-outline-secondary" :disabled="page >= pageCount" @click="changePage(1)">{{ text('下一页', 'Next') }}</button>
      </div>
    </div>
  </div>
</template>
