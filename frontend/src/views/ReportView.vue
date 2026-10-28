<script setup lang="ts">
import { prepareReportFigures } from '../utils/prepareReportFigures'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useWgcnaStore } from '../stores/wgcna'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t, locale } from '../utils/i18n'
import { exportReport, getReportStatus, pickFolder, type ReportStatus } from '../api/client'

const store = useWgcnaStore()
const route = useRoute()
// A report deep link can reopen an existing on-disk job after an app restart.
const jobId = computed(() => store.jobId || (typeof route.query.job === 'string' && /^[a-zA-Z0-9_-]+$/.test(route.query.job) ? route.query.job : ''))
const enrichType = ref<'go' | 'kegg'>('go')
const reportDir = ref('')
const reportName = ref('')
const reportState = ref<ReportStatus | null>(null)
const reportRunning = ref(false)
const error = ref('')
const previewVersion = ref(0)
let mounted = true
let generation = 0
const pad = (n: number) => String(n).padStart(2, '0')
function newReportName() {
  const d = new Date()
  return `rsWGCNA-${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`
}
const defaultReportName = ref(newReportName())
const reportNameInvalid = computed(() => {
  const name = reportName.value.trim()
  return name !== '' && (/[<>:"/\\|?*\u0000-\u001f]/.test(name) || /[. ]$/.test(name) || /^\.{1,2}$/.test(name) || /^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(\.|$)/i.test(name))
})
const resolvedReportPath = computed(() => {
  const base = reportDir.value.trim().replace(/[\\/]+$/, '')
  const separator = base.includes('\\') ? '\\' : '/'
  return base ? `${base}${separator}${reportName.value.trim() || defaultReportName.value}${separator}` : ''
})
const previewUrl = computed(() => reportState.value?.state === 'done' && reportState.value.preview_url ? `${reportState.value.preview_url}?v=${previewVersion.value}` : '')
async function pickFolderDir() {
  try { reportDir.value = await pickFolder() } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
}
async function poll(id: string, token: number) {
  while (mounted && token === generation) {
    const st = await getReportStatus(id)
    if (!mounted || token !== generation) return
    reportState.value = st
    if (st.state !== 'running') {
      reportRunning.value = false
      previewVersion.value = Date.now()
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 1500))
  }
}
async function restoreReport() {
  const token = ++generation
  reportState.value = null
  reportRunning.value = false
  error.value = ''
  if (!jobId.value) return
  const id = jobId.value
  try {
    const st = await getReportStatus(id)
    if (!mounted || token !== generation) return
    reportState.value = st
    previewVersion.value = Date.now()
    if (st.state === 'running') { reportRunning.value = true; await poll(id, token) }
  } catch (e) {
    if (reportRunning.value && token === generation) {
      reportRunning.value = false
      error.value = e instanceof Error ? e.message : String(e)
    }
  }
}
async function doExportReport() {
  if (!jobId.value || !reportDir.value.trim() || reportRunning.value || reportNameInvalid.value) return
  const id = jobId.value, token = ++generation
  reportRunning.value = true
  error.value = ''
  try {
    reportState.value = { state: 'running', dir: '', message: '准备图表快照…', done: [] }
    await prepareReportFigures(id, enrichType.value, text => { if (reportState.value) reportState.value.message = text })
    const result = await exportReport(id, reportDir.value.trim(), reportName.value.trim() || defaultReportName.value, enrichType.value, locale.value)
    reportState.value = { state: 'running', dir: result.dir, message: '', done: [] }
    await poll(id, token)
    if (!reportName.value.trim()) defaultReportName.value = newReportName()
  } catch (e) {
    if (token === generation) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (token === generation) reportRunning.value = false
  }
}
watch(jobId, restoreReport, { immediate: true })
onBeforeUnmount(() => { mounted = false; generation++ })
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <h6 class="mb-2"><i class="bi bi-file-earmark-text me-1"></i>{{ t('report.title') }}</h6>
      <div class="mb-3">
        <label class="form-label small mb-1">{{ t('mo.enrichType') }}</label>
        <select v-model="enrichType" class="form-select form-select-sm" :disabled="reportRunning"><option value="go">GO</option><option value="kegg">KEGG</option></select>
      </div>
      <div class="small text-muted mb-3">{{ t('report.autoDetails') }}</div>
      <hr />
      <div class="small text-muted">
        <div class="fw-semibold mb-1">{{ t('report.whatsIncluded') }}</div>
        <ul class="ps-3 mb-0 small">
          <li>{{ t('report.itemOverview') }}</li><li>{{ t('report.itemNetwork') }}</li><li>{{ t('report.itemModules') }}</li><li>{{ t('report.itemEnrich') }}</li><li>{{ t('report.itemCluster') }}</li><li>{{ t('report.itemProvenance') }}</li>
        </ul>
      </div>
    </template>
    <div class="p-3">
      <div class="d-flex align-items-center justify-content-between mb-3">
        <h4 class="mb-0 fw-semibold">{{ t('report.title') }}</h4>
        <span class="badge" :class="jobId ? 'text-bg-light' : 'text-bg-warning'">{{ jobId ? t('report.jobReady') : t('report.noJob') }}</span>
      </div>
      <div class="border rounded p-3 mb-3 bg-white">
        <div class="row g-2 align-items-end">
          <div class="col-12 col-lg-5">
            <label class="form-label small mb-1">{{ t('mo.reportDir') }}</label>
            <div class="input-group input-group-sm">
              <input type="text" v-model="reportDir" class="form-control" :disabled="reportRunning" :placeholder="t('mo.reportDirPlaceholder')" />
              <button class="btn btn-outline-secondary" type="button" :disabled="reportRunning" @click="pickFolderDir"><i class="bi bi-folder2-open me-1"></i>{{ t('mo.browse') }}</button>
            </div>
          </div>
          <div class="col-12 col-lg-4">
            <label class="form-label small mb-1">{{ t('mo.reportName') }} ({{ t('mo.optional') }})</label>
            <input type="text" v-model="reportName" :disabled="reportRunning" class="form-control form-control-sm" :class="{ 'is-invalid': reportNameInvalid }" :placeholder="defaultReportName" />
            <div v-if="reportNameInvalid" class="invalid-feedback">{{ t('mo.nameInvalid') }}</div>
          </div>
          <div class="col-12 col-lg-3">
            <button class="btn btn-sm btn-primary w-100" :disabled="!jobId || reportRunning || !reportDir.trim() || reportNameInvalid" @click="doExportReport">
              <span v-if="reportRunning" class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.startExport') }}
            </button>
          </div>
        </div>
        <div v-if="resolvedReportPath" class="small text-muted mt-2">{{ t('mo.resolvedPath') }}: <code class="text-break">{{ reportRunning ? reportState?.dir || resolvedReportPath : resolvedReportPath }}</code></div>
        <div v-if="error" class="alert alert-danger small mt-3 mb-0">{{ error }}</div>
        <div v-if="reportState" class="small mt-3" aria-live="polite">
          <div v-if="reportState.state === 'running'" class="text-muted">
            <div class="d-flex align-items-center gap-2"><span class="spinner-border spinner-border-sm"></span>{{ reportState.message || t('mo.exporting') }}</div>
            <div v-if="reportState.total" class="mt-1">{{ reportState.done?.length || 0 }} / {{ reportState.total }} {{ t('mo.modules') }}</div>
          </div>
          <div v-else-if="reportState.state === 'done'" class="text-success"><i class="bi bi-check-circle me-1"></i>{{ t('mo.exportDone') }} <code class="text-break">{{ reportState.dir }}</code></div>
          <div v-else class="text-danger"><i class="bi bi-x-circle me-1"></i>{{ reportState.message }}</div>
          <details v-if="reportState.warnings?.length" class="alert alert-warning mt-2 mb-0">
            <summary>{{ t('report.warnings') }} ({{ reportState.warnings.length }})</summary>
            <ul class="mb-0 mt-2"><li v-for="warning in reportState.warnings" :key="warning" class="text-break">{{ warning }}</li></ul>
          </details>
        </div>
      </div>
      <section class="border rounded bg-white overflow-hidden">
        <div class="d-flex align-items-center justify-content-between gap-2 border-bottom p-3">
          <h6 class="mb-0">{{ t('report.preview') }}</h6>
          <div v-if="previewUrl" class="d-flex gap-2">
            <button class="btn btn-sm btn-outline-secondary" @click="previewVersion = Date.now()">{{ t('report.reloadPreview') }}</button>
            <a class="btn btn-sm btn-outline-primary" :href="previewUrl" target="_blank" rel="noopener">{{ t('report.openFull') }}</a>
          </div>
        </div>
        <iframe v-if="previewUrl" :key="previewUrl" :src="previewUrl" :title="t('report.preview')" sandbox="allow-scripts allow-downloads allow-modals" class="report-preview"></iframe>
        <div v-else class="text-center text-muted p-5">{{ reportRunning ? t('report.previewRunning') : t('report.previewEmpty') }}</div>
      </section>
      <details class="small text-muted mt-3"><summary>{{ t('report.structureTitle') }}</summary>
        <pre class="p-3 bg-light rounded mt-2">{{ t('report.bundleStructure') }}</pre>
      </details>
    </div>
  </SidebarDrawer>
</template>

<style scoped>
.report-preview { display: block; width: 100%; height: 80vh; min-height: 680px; border: 0; background: #fff; }
</style>
