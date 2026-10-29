<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { EChartsOption } from 'echarts'
import { getEnrich, getExpression, getGOVersion, getSampleInfo, updateGO, type EnrichResult, type ExpressionResult, type GOVersionInfo, type SampleInfo } from '../api/client'
import { useWgcnaStore } from '../stores/wgcna'
import ResizableChart from './ResizableChart.vue'
import SimpleTable from './SimpleTable.vue'
import PlotDownloadWidget from './PlotDownloadWidget.vue'
import { makePoints, optionFromPoints, ratioFrac, type BubblePoint, type Source } from '../utils/enrichmentChart'
import ReportFigure from './ReportFigure.vue'
import { hclustAverage } from '../utils/hclust'
import { t } from '../utils/i18n'
const store = useWgcnaStore()
const props = defineProps<{ type: 'go' | 'kegg' }>()

const goResults = ref<EnrichResult[]>([])
const keggResults = ref<EnrichResult[]>([])
const loading = ref(false)
const error = ref('')
const fdr = ref(0.05)
const top = ref(0)
const maxCountOn = ref(true)   // drop terms with gene count > 500 (default on)
const goVersion = ref<GOVersionInfo | null>(null)
const updatingGO = ref(false)
const exportType = ref<'go' | 'kegg'>('go')
const heatExportMode = ref<'png' | 'svg' | 'pdf'>('png')

const selected = ref<{ termId: string; description: string; source: Source; genes: string[]; count: number; qvalue: number } | null>(null)
const selectedTermId = ref('')
const expr = ref<ExpressionResult | null>(null)
const sampleInfos = ref<SampleInfo[]>([])
const PALETTE = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b', '#e377c2', '#7f7f7f', '#bcbd22', '#17becf', '#aec7e8', '#ffbb78']
const showExport = ref(false)
const goOntology = ref<'ALL' | 'MF' | 'BP' | 'CC'>('ALL')

function openExport(type: 'go' | 'kegg') {
  exportType.value = type
  showExport.value = true
}

const heatExportUrl = computed(() => {
  if (!store.jobId || !selected.value?.genes?.length) return '#'
  return `/api/jobs/${store.jobId}/plots/enrich-heatmap?genes=${encodeURIComponent(selected.value.genes.join(','))}&mode=${heatExportMode.value}`
})

async function checkGO() {
  try {
    goVersion.value = await getGOVersion()
  } catch {
    goVersion.value = null
  }
}

async function doUpdateGO() {
  updatingGO.value = true
  try {
    await updateGO()
    await checkGO()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    updatingGO.value = false
  }
}

function sampleCategory(sample: string): string {
  const s = sampleInfos.value.find((x) => x.sample_id === sample)
  if (!s) return ''
  return s.attrs['group'] || s.attrs['genotype'] || s.attrs['condition'] || ''
}

function clusterOrder(rows: ExpressionResult['rows']): number[] {
  const n = rows.length
  const z = rows.map((r) => {
    const ns = r.values.length || 1
    const mean = r.values.reduce((a, b) => a + b, 0) / ns
    const ss = r.values.reduce((s, v) => s + (v - mean) * (v - mean), 0)
    const sd = Math.sqrt(ss / (ns - 1 || 1))
    return r.values.map((v) => (sd === 0 ? 0 : (v - mean) / sd))
  })
  const dissim = z.map((zi, i) => z.map((zj, j) => {
    if (i === j) return 0
    let s = 0
    for (let k = 0; k < zi.length; k++) s += (zi[k] - zj[k]) ** 2
    return Math.sqrt(s)
  }))
  if (n <= 1) return [0]
  return hclustAverage(dissim).order
}

async function loadSampleInfo() {
  if (!store.jobId) return
  try {
    sampleInfos.value = await getSampleInfo(store.jobId)
  } catch {
    sampleInfos.value = []
  }
}

const filteredGo = computed(() => goOntology.value === 'ALL' ? goResults.value : goResults.value.filter((r) => r.ontology === goOntology.value))
const goOption = computed(() => optionFromPoints(makePoints(filteredGo.value, 'GO'), 'GO ' + t('enrich.title'), '460px'))
const keggOption = computed(() => optionFromPoints(makePoints(keggResults.value, 'KEGG'), 'KEGG ' + t('enrich.title'), '460px'))

const enrichColumns = [
  { key: 'id', label: 'ID' },
  { key: 'description', label: 'Description' },
  { key: 'gene_ratio', label: 'GeneRatio' },
  { key: 'bg_ratio', label: 'BgRatio' },
  { key: 'p_adjust', label: 'p.adjust' },
  { key: 'count', label: 'Count' },
]
function enrichRows(results: EnrichResult[], source: Source) {
  return results.map((r) => ({
    id: r.id,
    description: r.description,
    _source: source,
    gene_ratio: r.gene_ratio,
    bg_ratio: r.bg_ratio,
    p_adjust: r['p.adjust'].toExponential(2),
    count: r.count,
  }))
}
const goTableRows = computed(() => enrichRows(filteredGo.value, 'GO'))
const keggTableRows = computed(() => enrichRows(keggResults.value, 'KEGG'))

async function load() {
  if (!store.jobId || !store.selectedModule) return
  loading.value = true
  error.value = ''
  selected.value = null
  expr.value = null
  try {
    const res = await getEnrich(store.jobId, store.selectedModule, props.type, fdr.value, top.value, maxCountOn.value ? 500 : 0)
    if (props.type === 'go') goResults.value = res
    else keggResults.value = res
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function selectTerm(d: BubblePoint) {
  if (!d || !d.genes?.length) return
  selectedTermId.value = d.termId
  selected.value = { termId: d.termId, description: d.description, source: d.source, genes: d.genes, count: d.value[2], qvalue: d.value[3] }
  getExpression(store.jobId, d.genes)
    .then((e) => { expr.value = e })
    .catch((e) => { error.value = e instanceof Error ? e.message : String(e) })
}

function onChartClick(params: any) {
  const d = params.data as BubblePoint | undefined
  if (d) selectTerm(d)
}

function onTableRowClick(row: Record<string, any>) {
  const results = row._source === 'GO' ? filteredGo.value : keggResults.value
  const r = results.find((x) => x.id === row.id)
  if (!r) return
  const d: BubblePoint = {
    value: [ratioFrac(r.gene_ratio), r.description, r.count, r.qvalue],
    termId: r.id,
    description: r.description,
    genes: r.gene_id,
    source: row._source as Source,
  }
  selectTerm(d)
}

const geneRows = computed(() => (expr.value?.rows ?? []).map((r) => ({
  gene: r.gene,
  symbol: r.symbol || '',
  gene_name: r.gene_name || '',
  mean: r.mean.toFixed(1),
})))

const heatOption = computed<EChartsOption>(() => {
  if (!expr.value || !expr.value.rows.length) return {}
  const nS = expr.value.samples.length || 1
  const zAll = expr.value.rows.map((r) => {
    const mean = r.values.reduce((a, b) => a + b, 0) / nS
    const ss = r.values.reduce((s, v) => s + (v - mean) * (v - mean), 0)
    const sd = Math.sqrt(ss / (nS - 1 || 1))
    return r.values.map((v) => (sd === 0 ? 0 : (v - mean) / sd))
  })
  const order = clusterOrder(expr.value.rows)
  const rows = order.map((i) => expr.value!.rows[i])
  const z = order.map((i) => zAll[i])
  const origSamples = expr.value.samples
  const geneLabels = rows.map((r) => r.symbol || r.gene)

  // x-axis: sort samples by group (stable), keeping biological replicates of
  // the same group together. Only the y-axis (genes) is clustered.
  const sampleIdx = origSamples
    .map((s, i) => ({ i, g: sampleCategory(s) }))
    .sort((a, b) => a.g.localeCompare(b.g) || a.i - b.i)
  const samples = sampleIdx.map((x) => origSamples[x.i])
  const idxMap = sampleIdx.map((x) => x.i)

  const heat: [number, number, number][] = []
  z.forEach((row, gi) => idxMap.forEach((origSi, newSi) => heat.push([newSi, gi, +row[origSi].toFixed(3)])))
  let maxAbs = 1
  z.forEach((row) => row.forEach((v) => (maxAbs = Math.max(maxAbs, Math.abs(v)))))

  const cats = samples.map((s) => sampleCategory(s))
  const uniq = [...new Set(cats)].sort()
  const pal: Record<string, string> = {}
  uniq.forEach((v, i) => { pal[v] = PALETTE[i % PALETTE.length] })
  const n = samples.length

  return {
    title: { text: (selected.value?.description ?? '') + ' (' + rows.length + ' genes)', left: 'center', top: 2, textStyle: { fontSize: 13 } },
    tooltip: {
      formatter: (p: any) => {
        if (p.seriesType === 'heatmap') {
          const si = p.value[0] as number
          const gi = p.value[1] as number
          return geneLabels[gi] + ' × ' + samples[si] + '<br/>z = ' + p.value[2]
        }
        return ''
      },
    },
    grid: [
      { left: 170, right: 40, top: 64, bottom: 120 },
      { left: 170, right: 40, bottom: 10, height: 16 },
    ],
    xAxis: [
      { type: 'category', data: samples, gridIndex: 0, axisLabel: { rotate: 90, fontSize: 9 } },
      { type: 'value', min: -0.5, max: n - 0.5, gridIndex: 1, show: false },
    ],
    yAxis: [
      { type: 'category', data: geneLabels, gridIndex: 0, axisLabel: { fontSize: 9 } },
      { type: 'value', min: 0, max: 1, gridIndex: 1, show: false },
    ],
    visualMap: {
      min: -maxAbs, max: maxAbs, calculable: true, orient: 'horizontal', left: 'center', top: 28,
      inRange: { color: ['#2166ac', '#ffffff', '#b2182b'] },
    },
    series: [
      { type: 'heatmap', data: heat, xAxisIndex: 0, yAxisIndex: 0, itemStyle: { borderColor: 'rgba(255,255,255,0.25)', borderWidth: 0.4 } },
      {
        type: 'custom', data: [0], xAxisIndex: 1, yAxisIndex: 1, z: 5,
        renderItem: (params: any, api: any): any => {
          const children: any[] = []
          for (let si = 0; si < n; si++) {
            const color = pal[cats[si]] ?? '#d1d5db'
            const x0 = api.coord([si - 0.5, 0])[0]
            const x1 = api.coord([si + 0.5, 0])[0]
            const yTop = api.coord([si, 1])[1]
            const yBot = api.coord([si, 0])[1]
            children.push({ type: 'rect', shape: { x: x0, y: yTop, width: x1 - x0, height: yBot - yTop }, style: { fill: color } })
          }
          return { type: 'group', children }
        },
      },
    ],
  }
})

watch(() => store.selectedModule, (m) => { if (m) load() })
onMounted(() => { if (store.selectedModule) load(); checkGO(); loadSampleInfo() })
</script>

<template>
  <div class="mt-4">
    <div class="d-flex align-items-center gap-3 flex-wrap mb-2">
      <label class="small mb-0 d-flex align-items-center gap-1">{{ t('enrich.fdr') }}
        <input type="number" class="form-control form-control-sm" style="width:80px" v-model.number="fdr" step="0.01" min="0.01" max="1" />
      </label>
      <label class="small mb-0 d-flex align-items-center gap-1">{{ t('enrich.top') }}
        <input type="number" class="form-control form-control-sm" style="width:72px" v-model.number="top" min="0" max="1000" title="0 = all" />
      </label>
      <label class="small mb-0 d-flex align-items-center gap-1">
        <input type="checkbox" v-model="maxCountOn" class="form-check-input me-1" />{{ t('mo.dropBroadTerms') }}
      </label>
      <button class="btn btn-sm btn-primary" :disabled="loading || !store.selectedModule" @click="load"><i class="bi bi-arrow-repeat me-1"></i>{{ t('enrich.run') }}</button>
      <span v-if="props.type === 'go' && goVersion" class="small text-muted">
        GO {{ goVersion.current }}
        <button v-if="goVersion.update_available" class="btn btn-sm btn-outline-warning py-0 px-1 ms-1" :disabled="updatingGO" @click="doUpdateGO">
          {{ updatingGO ? '…' : t('enrich.update') + ' ' + goVersion.latest }}
        </button>
      </span>
    </div>

    <div v-if="error" class="alert alert-warning py-2 small">{{ error }}</div>
    <div v-if="loading" class="text-muted small py-2">{{ t('common.running') }}</div>
    <template v-else>
      <div v-if="props.type === 'go' && goResults.length" class="border rounded p-3">
        <div class="d-flex align-items-center gap-2 mb-2">
          <div class="btn-group btn-group-sm">
            <button class="btn" :class="goOntology === 'ALL' ? 'btn-primary' : 'btn-outline-primary'" @click="goOntology = 'ALL'">ALL</button>
            <button class="btn" :class="goOntology === 'MF' ? 'btn-primary' : 'btn-outline-primary'" @click="goOntology = 'MF'">MF</button>
            <button class="btn" :class="goOntology === 'BP' ? 'btn-primary' : 'btn-outline-primary'" @click="goOntology = 'BP'">BP</button>
            <button class="btn" :class="goOntology === 'CC' ? 'btn-primary' : 'btn-outline-primary'" @click="goOntology = 'CC'">CC</button>
          </div>
          <button class="btn btn-sm btn-outline-primary ms-auto" @click="openExport('go')"><i class="bi bi-download me-1"></i>GO</button>
        </div>
        <div class="row g-3">
          <div class="col-lg-7"><ReportFigure :figure-key="`06_module_detail/${store.selectedModule}/enrichment/go.png`"><ResizableChart :option="goOption" @chart-click="onChartClick" /></ReportFigure></div>
          <div class="col-lg-5"><SimpleTable :columns="enrichColumns" :rows="goTableRows" :filename="'go_enrich'" scroll-height="460px" :selected-key="selectedTermId" row-key="id" @row-click="onTableRowClick" /></div>
        </div>
      </div>

      <div v-if="props.type === 'kegg' && keggResults.length" class="border rounded p-3">
        <div class="d-flex align-items-center gap-2 mb-2">
          <button class="btn btn-sm btn-outline-primary ms-auto" @click="openExport('kegg')"><i class="bi bi-download me-1"></i>KEGG</button>
        </div>
        <div class="row g-3">
          <div class="col-lg-7"><ReportFigure :figure-key="`06_module_detail/${store.selectedModule}/enrichment/kegg.png`"><ResizableChart :option="keggOption" @chart-click="onChartClick" /></ReportFigure></div>
          <div class="col-lg-5"><SimpleTable :columns="enrichColumns" :rows="keggTableRows" :filename="'kegg_enrich'" scroll-height="460px" :selected-key="selectedTermId" row-key="id" @row-click="onTableRowClick" /></div>
        </div>
      </div>
    </template>

    <div v-if="selected" class="mt-3">
      <h6 class="fw-semibold">{{ selected.source }} · {{ selected.termId }} · {{ selected.description }}（{{ selected.genes.length }} genes）</h6>
      <SimpleTable :columns="[
        { key: 'gene', label: 'Gene' },
        { key: 'symbol', label: 'Symbol' },
        { key: 'gene_name', label: 'Gene name' },
        { key: 'mean', label: 'Mean expr' },
      ]" :rows="geneRows" scroll-height="320px" :filename="selected.termId + '_genes'" />
      <div class="d-flex align-items-center gap-2 my-2">
        <span class="text-muted small">{{ t('plot.export') }} · ComplexHeatmap</span>
        <select v-model="heatExportMode" class="form-select form-select-sm" style="width:90px">
          <option value="png">PNG</option>
          <option value="svg">SVG</option>
          <option value="pdf">PDF</option>
        </select>
        <a class="btn btn-sm btn-outline-secondary" :href="heatExportUrl" download><i class="bi bi-download me-1"></i>{{ t('plot.download') }}</a>
      </div>
      <ResizableChart :option="heatOption" />
    </div>
    <PlotDownloadWidget kind="enrich" :type="exportType" :module="store.selectedModule" :fdr="fdr" :top="top" :ontology="goOntology" :show="showExport" @close="showExport = false" />
  </div>
</template>
