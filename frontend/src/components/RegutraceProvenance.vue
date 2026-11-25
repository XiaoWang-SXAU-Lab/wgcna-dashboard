<script setup lang="ts">
import { ref } from 'vue'
import { getRegutraceProvenance, type RegutraceProvenance } from '../api/client'
import { locale } from '../utils/i18n'

const props = defineProps<{ jobId: string }>()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const busy = ref(false)
const error = ref('')
const data = ref<RegutraceProvenance | null>(null)
let loadSeq = 0

async function load() {
  if (!props.jobId) return
  const seq = ++loadSeq
  busy.value = true
  error.value = ''
  try {
    const res = await getRegutraceProvenance(props.jobId)
    if (seq === loadSeq) data.value = res
  } catch (e) {
    if (seq === loadSeq) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === loadSeq) busy.value = false
  }
}
function pretty(value: unknown): string {
  try { return JSON.stringify(value, null, 2) } catch { return String(value) }
}
</script>

<template>
  <div class="card mb-3">
    <div class="card-body">
      <div class="d-flex align-items-start flex-wrap gap-2 mb-2">
        <div>
          <h6 class="mb-0">{{ text('可复现性与诊断', 'Provenance and diagnostics') }}</h6>
          <p class="small text-muted mb-0">{{ text('输入校验和、参考包、扫描 manifest、统计限制和警告。', 'Input checksums, reference pack, scan manifest, statistical limitations and warnings.') }}</p>
        </div>
        <button class="btn btn-sm btn-outline-secondary ms-auto" :disabled="busy" @click="load">
          {{ busy ? text('读取中…', 'Loading…') : text('加载 provenance', 'Load provenance') }}
        </button>
      </div>
      <div v-if="error" class="alert alert-warning small py-2">{{ error }}</div>
      <div v-if="data">
        <div class="row g-3">
          <div class="col-12 col-lg-6">
            <div class="small text-muted mb-1">{{ text('软件与输入', 'Software and inputs') }}</div>
            <table class="table table-sm small">
              <tbody>
                <tr><th class="text-muted">Tool</th><td>{{ data.tool || '—' }} {{ data.tool_version || '' }}</td></tr>
                <tr><th class="text-muted">Job</th><td class="font-monospace">{{ data.job_id }}</td></tr>
                <tr v-for="input in data.inputs" :key="input.role + input.name">
                  <th class="text-muted">{{ input.role }}</th>
                  <td>{{ input.name }} <span class="text-muted">· {{ input.size?.toLocaleString() ?? '—' }} bytes</span>
                    <div class="font-monospace text-break small">{{ input.sha256 || 'checksum unavailable' }}</div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="col-12 col-lg-6">
            <div class="small text-muted mb-1">{{ text('参考包与统计限制', 'Reference pack and limitations') }}</div>
            <div v-if="data.reference_pack?.version" class="small mb-2">
              <b>{{ data.reference_pack.key }}</b> · {{ data.reference_pack.version }}<span v-if="data.reference_pack.species"> · {{ data.reference_pack.species }}</span>
            </div>
            <ul class="small ps-3">
              <li v-for="limitation in data.limitations" :key="limitation">{{ limitation }}</li>
            </ul>
            <div v-if="data.warnings?.length" class="small text-warning-emphasis">
              <div class="fw-semibold">{{ text('警告', 'Warnings') }}</div>
              <ul class="ps-3 mb-0"><li v-for="warning in data.warnings" :key="warning">{{ warning }}</li></ul>
            </div>
          </div>
        </div>
        <div class="row g-3 mt-1">
          <div class="col-12 col-lg-6">
            <div class="small text-muted mb-1">{{ text('运行指标', 'Analysis metrics') }}</div>
            <pre class="small bg-light border rounded p-2 mb-0" style="max-height: 16rem; overflow: auto">{{ pretty(data.analysis) }}</pre>
          </div>
          <div class="col-12 col-lg-6">
            <div class="small text-muted mb-1">scan manifest</div>
            <pre class="small bg-light border rounded p-2 mb-0" style="max-height: 16rem; overflow: auto">{{ pretty(data.scan_manifest ?? { note: 'scan manifest not available for this job' }) }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
