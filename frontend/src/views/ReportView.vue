<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t, locale } from '../utils/i18n'
import { exportReport, getReportStatus, pickFolder, type ReportStatus } from '../api/client'

const store = useWgcnaStore()

const enrichType = ref<'go' | 'kegg'>('go')
const reportDir = ref('')
const reportName = ref('')
const reportState = ref<ReportStatus | null>(null)
const reportRunning = ref(false)

const defaultReportName = computed(() => {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `rsWGCNA-${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`
})
const reportNameInvalid = computed(() => {
  const name = reportName.value.trim()
  if (name === '') return false
  return /[<>:"/\\|?*\u0000-\u001f]/.test(name) || /[. ]$/.test(name) || /^\.{1,2}$/.test(name) || /^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(\.|$)/i.test(name)
})
const resolvedReportPath = computed(() => {
  const base = reportDir.value.replace(/[\\/]+$/, '') || ''
  const name = reportName.value.trim() || defaultReportName.value
  return base ? `${base}/${name}/` : ''
})

async function pickFolderDir() {
  try {
    reportDir.value = await pickFolder()
  } catch { /* cancelled */ }
}

async function doExportReport() {
  if (!store.jobId || !reportDir.value || reportRunning.value || reportNameInvalid.value) return
  reportRunning.value = true
  reportState.value = null
  try {
    await exportReport(store.jobId, reportDir.value, reportName.value, enrichType.value, locale.value)
    for (let i = 0; i < 300; i++) {
      await new Promise((r) => setTimeout(r, 2000))
      const st = await getReportStatus(store.jobId)
      reportState.value = st
      if (st.state !== 'running') break
    }
  } finally {
    reportRunning.value = false
  }
}
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <h6 class="mb-2"><i class="bi bi-file-earmark-text me-1"></i>{{ t('report.title') }}</h6>

      <div class="mb-3">
        <label class="form-label small mb-1">{{ t('mo.enrichType') }}</label>
        <select v-model="enrichType" class="form-select form-select-sm">
          <option value="go">GO</option>
          <option value="kegg">KEGG</option>
        </select>
      </div>

      <hr />
      <div class="small text-muted">
        <div class="fw-semibold mb-1">{{ t('report.whatsIncluded') }}</div>
        <ul class="ps-3 mb-0 small">
          <li>{{ t('report.itemOverview') }}</li>
          <li>{{ t('report.itemNetwork') }}</li>
          <li>{{ t('report.itemModules') }}</li>
          <li>{{ t('report.itemEnrich') }}</li>
          <li>{{ t('report.itemProvenance') }}</li>
        </ul>
      </div>
    </template>

    <div class="p-3">
      <div class="d-flex align-items-center justify-content-between mb-3">
        <h4 class="mb-0 fw-semibold">{{ t('report.title') }}</h4>
        <span v-if="store.jobId" class="badge text-bg-light">{{ t('report.jobReady') }}</span>
        <span v-else class="badge text-bg-warning">{{ t('report.noJob') }}</span>
      </div>

      <div class="border rounded p-3 mb-3 bg-white" style="max-width: 860px">
        <div class="row g-2 align-items-end">
          <div class="col-5">
            <label class="form-label small mb-1">{{ t('mo.reportDir') }}</label>
            <div class="input-group input-group-sm">
              <input type="text" v-model="reportDir" class="form-control" :placeholder="t('mo.reportDirPlaceholder')" />
              <button class="btn btn-outline-secondary" type="button" @click="pickFolderDir"><i class="bi bi-folder2-open me-1"></i>{{ t('mo.browse') }}</button>
            </div>
          </div>
          <div class="col-4">
            <label class="form-label small mb-1">{{ t('mo.reportName') }}（{{ t('mo.optional') }}）</label>
            <input type="text" v-model="reportName" class="form-control form-control-sm" :class="{ 'is-invalid': reportNameInvalid }" :placeholder="defaultReportName" />
            <div v-if="reportNameInvalid" class="invalid-feedback">{{ t('mo.nameInvalid') }}</div>
          </div>
          <div class="col-3">
            <button class="btn btn-sm btn-primary w-100" :disabled="!store.jobId || reportRunning || !reportDir || reportNameInvalid" @click="doExportReport">
              <span v-if="reportRunning" class="spinner-border spinner-border-sm me-1"></span>{{ t('mo.startExport') }}
            </button>
          </div>
        </div>

        <div v-if="resolvedReportPath" class="small text-muted mt-2">
          <i class="bi bi-folder me-1"></i>{{ t('mo.resolvedPath') }}：<code>{{ resolvedReportPath }}</code>
        </div>

        <div v-if="reportState" class="small mt-3">
          <div v-if="reportState.state === 'running'" class="d-flex align-items-center gap-2 text-muted">
            <div class="spinner-border spinner-border-sm"></div>{{ t('mo.exporting') }} ({{ reportState.done.length }} {{ t('mo.modules') }})…
          </div>
          <div v-else-if="reportState.state === 'done'" class="text-success">
            <i class="bi bi-check-circle me-1"></i>{{ t('mo.exportDone') }} <code>{{ reportState.dir }}</code>
            <div class="mt-1">{{ t('report.openHint') }}</div>
          </div>
          <div v-else class="text-danger"><i class="bi bi-x-circle me-1"></i>{{ reportState.message }}</div>
        </div>
      </div>

      <div class="border rounded p-3 bg-white" style="max-width: 860px">
        <h6 class="mb-2">{{ t('report.structureTitle') }}</h6>
        <pre class="small mb-0" style="background:#f8f9fa;padding:0.75rem;border-radius:6px">项目名/
├── report/
│   ├── index.html      {{ t('report.structureHtml') }}
│   ├── report.qmd      {{ t('report.structureQmd') }}
│   └── design.css/.js
└── result/
    ├── 00_provenance/   {{ t('report.structureProv') }}
    ├── 02_filter … 07_hub/
    ├── module_overview.png
    └── overview_network.svg</pre>
      </div>
    </div>
  </SidebarDrawer>
</template>
