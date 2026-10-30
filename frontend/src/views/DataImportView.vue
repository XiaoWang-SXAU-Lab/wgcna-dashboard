<script setup lang="ts">
import ReportFigure from '../components/ReportFigure.vue'
import { computed, ref, watch } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { getSampleInfo, type SampleInfo } from '../api/client'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'
import SampleTreeChart from '../components/SampleTreeChart.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'
import type { CategoryBand } from '../components/SampleTreeChart.vue'

const store = useWgcnaStore()

type UploadKind = 'exprFile' | 'traitFile' | 'sampleInfoFile' | 'annotationFile' | 'gene2goFile' | 'gene2keggFile'
async function selectFile(kind: UploadKind, e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return // Cancelling the system picker preserves the selection.
  store.invalidateInputs()
  store[kind] = file
  input.value = '' // Selecting the same corrected file must trigger change.
  await store.checkInputs()
}
async function clearFile(kind: UploadKind) {
  store.invalidateInputs()
  store[kind] = null
  if ([store.exprFile, store.traitFile, store.sampleInfoFile, store.annotationFile, store.gene2goFile, store.gene2keggFile].some(Boolean)) await store.checkInputs()
}
function onExpr(e: Event) { void selectFile('exprFile', e) }
function onTrait(e: Event) { void selectFile('traitFile', e) }
function onSampleInfo(e: Event) { void selectFile('sampleInfoFile', e) }
function onAnnotation(e: Event) { void selectFile('annotationFile', e) }
function onGene2go(e: Event) { void selectFile('gene2goFile', e) }
function onGene2kegg(e: Event) { void selectFile('gene2keggFile', e) }
watch(() => store.dataType, () => {
  if (store.exprFile && !store.created) void store.checkInputs()
})

function fileName(file: File | null, demoName: string): string {
  if (store.demoLoaded) return `demo · ${demoName}`
  return file ? file.name : t('di.chooseFile')
}

async function createAndFilter() {
  if (store.creatingJob || store.validatingInputs) return
  if (!store.created) await store.createJobAndUpload()
  if (store.created) await store.runStep('filter')
}

async function loadDemoData() {
  await store.loadDemo()
}

const selectedOutliers = ref<string[]>([])
const sampleInfos = ref<SampleInfo[]>([])
const attrColumns = ref<string[]>([])
const colorAttrs = ref<string[]>([])
const EXCLUDE_ATTRS = new Set(['sample_id', 'sample_name', 'species'])

watch([() => store.jobId, () => store.filterResult], async ([id]) => {
  if (!id) {
    sampleInfos.value = []
    attrColumns.value = []
    colorAttrs.value = []
    return
  }
  try {
    const infos = await getSampleInfo(id)
    sampleInfos.value = infos
    const cols = (infos.length ? Object.keys(infos[0].attrs) : []).filter((c) => !EXCLUDE_ATTRS.has(c))
    attrColumns.value = cols
    colorAttrs.value = cols.includes('group') ? ['group'] : (cols.length ? [cols[0]] : [])
  } catch {
    sampleInfos.value = []
    attrColumns.value = []
    colorAttrs.value = []
  }
})

const categories = computed<CategoryBand[]>(() => {
  if (!colorAttrs.value.length || !sampleInfos.value.length) return []
  return colorAttrs.value.map((attr) => {
    const values: Record<string, string> = {}
    for (const s of sampleInfos.value) values[s.sample_id] = s.attrs[attr] ?? ''
    return { name: attr, values }
  })
})

const showTreeExport = ref(false)

async function removeOutliers() {
  await store.removeSamples(selectedOutliers.value)
  selectedOutliers.value = []
}

function onSelectSample(name: string) {
  const i = selectedOutliers.value.indexOf(name)
  if (i >= 0) selectedOutliers.value.splice(i, 1)
  else selectedOutliers.value.push(name)
}
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div>
        <div class="border rounded p-2 mb-2">
          <div class="small fw-semibold mb-1">{{ t('di.demo') }}</div>
          <div class="d-flex gap-2">
            <button class="btn btn-sm btn-primary flex-fill" :disabled="store.stepRunning !== '' || store.creatingJob" @click="loadDemoData">
              <i class="bi bi-magic me-1"></i>{{ t('di.loadDemo') }}
            </button>
            <a class="btn btn-sm btn-outline-secondary flex-fill" href="/api/demo/download" download>
              <i class="bi bi-download me-1"></i>{{ t('di.downloadDemo') }}
            </a>
          </div>
          <div v-if="store.demoLoaded" class="small text-success mt-1"><i class="bi bi-check-circle me-1"></i>{{ t('di.demoLoaded') }}</div>
        </div>
        <div v-if="store.validatingInputs" class="alert alert-info small py-2" role="status">{{ t('di.validating') }}</div>
        <button v-else-if="store.inputValidation" class="btn btn-sm w-100 mb-2" :class="store.inputValidation.valid ? 'btn-outline-success' : 'btn-outline-danger'" @click="store.showInputValidation = true">
          {{ store.inputValidation.valid ? t('di.validationPassed') : t('di.validationBlocked') }}
        </button>
        <h6 class="fw-bold mb-3">{{ t('param.parameters') }}</h6>
        <div class="vstack gap-2">
          <div>
            <label class="form-label small mb-1">{{ t('di.expr') }}<span class="text-danger">*</span></label>
            <button v-if="store.exprFile" type="button" class="btn btn-link btn-sm p-0 float-end" :disabled="store.stepRunning !== '' || store.creatingJob" @click="clearFile('exprFile')">{{ t('di.clearFile') }}</button>
            <label class="btn btn-sm btn-outline-secondary w-100 text-truncate mb-0">
              <i class="bi bi-upload me-1"></i>{{ fileName(store.exprFile, 'expr.tsv') }}
              <input type="file" :disabled="store.stepRunning !== '' || store.creatingJob" accept=".tsv,.txt,.csv,.xlsx,.xls" class="visually-hidden" @change="onExpr" />
            </label>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.trait') }}</label>
            <button v-if="store.traitFile" type="button" class="btn btn-link btn-sm p-0 float-end" :disabled="store.stepRunning !== '' || store.creatingJob" @click="clearFile('traitFile')">{{ t('di.clearFile') }}</button>
            <label class="btn btn-sm btn-outline-secondary w-100 text-truncate mb-0">
              <i class="bi bi-upload me-1"></i>{{ fileName(store.traitFile, 'trait.tsv') }}
              <input type="file" :disabled="store.stepRunning !== '' || store.creatingJob" accept=".tsv,.txt,.csv,.xlsx,.xls" class="visually-hidden" @change="onTrait" />
            </label>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.sampleInfo') }}</label>
            <button v-if="store.sampleInfoFile" type="button" class="btn btn-link btn-sm p-0 float-end" :disabled="store.stepRunning !== '' || store.creatingJob" @click="clearFile('sampleInfoFile')">{{ t('di.clearFile') }}</button>
            <label class="btn btn-sm btn-outline-secondary w-100 text-truncate mb-0">
              <i class="bi bi-upload me-1"></i>{{ fileName(store.sampleInfoFile, 'sample_info.tsv') }}
              <input type="file" :disabled="store.stepRunning !== '' || store.creatingJob" accept=".tsv,.txt,.csv" class="visually-hidden" @change="onSampleInfo" />
            </label>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.annotation') }}</label>
            <button v-if="store.annotationFile" type="button" class="btn btn-link btn-sm p-0 float-end" :disabled="store.stepRunning !== '' || store.creatingJob" @click="clearFile('annotationFile')">{{ t('di.clearFile') }}</button>
            <label class="btn btn-sm btn-outline-secondary w-100 text-truncate mb-0">
              <i class="bi bi-upload me-1"></i>{{ fileName(store.annotationFile, 'annotation.tsv') }}
              <input type="file" :disabled="store.stepRunning !== '' || store.creatingJob" accept=".tsv,.txt,.csv" class="visually-hidden" @change="onAnnotation" />
            </label>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.gene2go') }}</label>
            <button v-if="store.gene2goFile" type="button" class="btn btn-link btn-sm p-0 float-end" :disabled="store.stepRunning !== '' || store.creatingJob" @click="clearFile('gene2goFile')">{{ t('di.clearFile') }}</button>
            <label class="btn btn-sm btn-outline-secondary w-100 text-truncate mb-0">
              <i class="bi bi-upload me-1"></i>{{ fileName(store.gene2goFile, 'gene2go.tsv') }}
              <input type="file" :disabled="store.stepRunning !== '' || store.creatingJob" accept=".tsv,.txt,.csv" class="visually-hidden" @change="onGene2go" />
            </label>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.gene2kegg') }}</label>
            <button v-if="store.gene2keggFile" type="button" class="btn btn-link btn-sm p-0 float-end" :disabled="store.stepRunning !== '' || store.creatingJob" @click="clearFile('gene2keggFile')">{{ t('di.clearFile') }}</button>
            <label class="btn btn-sm btn-outline-secondary w-100 text-truncate mb-0">
              <i class="bi bi-upload me-1"></i>{{ fileName(store.gene2keggFile, 'gene2kegg.tsv') }}
              <input type="file" :disabled="store.stepRunning !== '' || store.creatingJob" accept=".tsv,.txt,.csv" class="visually-hidden" @change="onGene2kegg" />
            </label>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.dataType') }}</label>
            <select class="form-select form-select-sm" v-model="store.dataType">
              <option value="count">{{ t('di.dataTypeCount') }}</option>
              <option value="expected_count">{{ t('di.dataTypeExpectedCount') }}</option>
              <option value="normalized_count">{{ t('di.dataTypeNormalizedCount') }}</option>
              <option value="peak_area">{{ t('di.dataTypePeakArea') }}</option>
              <option value="protein_abundance">{{ t('di.dataTypeProteinAbundance') }}</option>
            </select>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.nKeep') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.nKeep" min="100" />
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.filterMethod') }}</label>
            <select class="form-select form-select-sm" v-model="store.filterMethod">
              <option value="mad">MAD</option>
              <option value="var">VAR</option>
            </select>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.samplePct') }}: <b>{{ store.samplePercentage }}%</b></label>
            <input class="form-range" type="range" v-model.number="store.samplePercentage" min="0" max="100" step="5" />
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.cutoff') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.cutoff" step="0.1" />
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.normalize') }}</label>
            <select class="form-select form-select-sm" v-model="store.normalizeMethod">
              <option v-for="o in store.normalizeOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
            </select>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('di.threads') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.threads" min="1" max="32" />
          </div>
          <button class="btn btn-primary w-100 mt-2" :disabled="(store.creatingJob || (!store.created && (!store.exprFile || store.validatingInputs || store.inputValidation?.valid === false))) || store.stepRunning !== ''" @click="createAndFilter">
            {{ store.stepRunning ? t('common.running') : store.isDone('filter') ? t('di.rerun') : t('di.run') }}
          </button>
        </div>
      </div>
    </template>
      <div class="mb-3">
        <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('di.title') }} <StepStatusBadge step="filter" /></h4>
        <p class="text-muted small mb-0">{{ t('di.desc') }}</p>
      </div>
      <div v-if="store.error" class="alert alert-danger d-flex align-items-center gap-2 py-2">
        <i class="bi bi-exclamation-triangle-fill"></i>
        <span class="flex-grow-1">{{ store.error }}</span>
        <button class="btn btn-sm btn-outline-danger" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button>
      </div>

      <div v-if="store.excludedSamples.length" class="alert alert-warning small d-flex align-items-center gap-2">
        <span class="flex-grow-1">{{ t('di.excludedSamples') }}: {{ store.excludedSamples.join(', ') }}</span>
        <button class="btn btn-sm btn-outline-secondary text-nowrap" :disabled="store.stepRunning !== ''" @click="store.restoreSamples()">{{ t('di.restoreSamples') }}</button>
      </div>
      <template v-if="store.filterResult">
        <div class="card mb-3">
          <div class="card-header bg-white fw-semibold"><i class="bi bi-clipboard-data me-2"></i>{{ t('di.report') }}</div>
          <div class="card-body py-2 d-flex flex-wrap gap-2">
            <span class="badge text-bg-light border">{{ t('di.genesBefore') }}: {{ store.filterResult.n_genes_before }}</span>
            <span class="badge text-bg-light border">{{ t('di.genesAfterLow') }}: {{ store.filterResult.n_genes_after_low }}</span>
            <span class="badge text-bg-success">{{ t('di.genesFinal') }}: {{ store.filterResult.n_genes_final }}</span>
            <span class="badge text-bg-light border">{{ t('di.samples') }}: {{ store.filterResult.n_samples }}</span>
          </div>
        </div>

        <div class="mb-3">
          <h6 class="fw-semibold mb-2"><i class="bi bi-diagram-3 me-1"></i>{{ t('di.sampleTree') }}</h6>
          <div v-if="attrColumns.length" class="d-flex align-items-center gap-2 mb-2">
            <label class="form-label small mb-0 text-nowrap">{{ t('di.colorBy') }}</label>
            <div class="d-flex flex-wrap gap-2">
              <label v-for="c in attrColumns" :key="c" class="form-check form-check-inline small mb-0">
                <input class="form-check-input" type="checkbox" :value="c" v-model="colorAttrs" />
                <span class="form-check-label">{{ c }}</span>
              </label>
            </div>
          </div>
          <div class="mb-2">
            <button class="btn btn-sm btn-outline-primary" @click="showTreeExport = true">
              <i class="bi bi-download me-1"></i>{{ t('plot.export') }}
            </button>
          </div>
          <ReportFigure figure-key="02_filter/sample_tree.png"><SampleTreeChart :samples="store.filterResult.samples" :tree="store.filterResult.sample_tree" :categories="categories" :selected="selectedOutliers" @select="onSelectSample" /></ReportFigure>
          <PlotDownloadWidget kind="sample-tree" :attrs="colorAttrs.join(',')" :show="showTreeExport" @close="showTreeExport = false" />
        </div>

        <div class="card">
          <div class="card-header bg-white fw-semibold"><i class="bi bi-trash me-2"></i>{{ t('di.removeOutlier') }}</div>
          <div class="card-body">
            <p class="text-muted small">{{ t('di.selectSamples') }}</p>
            <div class="d-flex flex-wrap gap-2">
              <label v-for="s in store.filterResult.samples" :key="s" class="form-check mb-0">
                <input class="form-check-input" type="checkbox" :value="s" v-model="selectedOutliers" />
                <span class="form-check-label small">{{ s }}</span>
              </label>
            </div>
            <button class="btn btn-danger mt-3" :disabled="selectedOutliers.length === 0 || store.stepRunning !== ''" @click="removeOutliers">
              <i class="bi bi-arrow-repeat me-1"></i>{{ t('di.removeBtn') }}
            </button>
          </div>
        </div>
      </template>

      <div v-else class="alert alert-info small">
        <span v-if="store.isDone('filter')" class="badge text-bg-success me-2">{{ t('di.done') }} ✓</span>
        <span v-else-if="store.created" class="badge text-bg-light text-muted me-2">{{ t('di.created') }}</span>
        {{ t('di.hint') }}
      </div>
  </SidebarDrawer>
</template>
