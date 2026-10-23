<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { EChartsOption } from 'echarts'
import { getEnrich, getExpression, getGOVersion, getSampleInfo, updateGO, type EnrichResult, type ExpressionResult, type GOVersionInfo, type SampleInfo } from '../api/client'
import { useWgcnaStore } from '../stores/wgcna'
import EChart from './EChart.vue'
import SimpleTable from './SimpleTable.vue'
import { mixHex } from '../utils/colors'
import { hclustAverage } from '../utils/hclust'
import { t } from '../utils/i18n'

type Source = 'GO' | 'KEGG'

interface BubblePoint {
  value: [number, string, number, number]
  termId: string
  description: string
  genes: string[]
  source: Source
}

const store = useWgcnaStore()

const goResults = ref<EnrichResult[]>([])
const keggResults = ref<EnrichResult[]>([])
const loading = ref(false)
const error = ref('')
const fdr = ref(0.05)
const top = ref(20)
const mode = ref<'separate' | 'combined'>('separate')
const exportMode = ref<'png' | 'svg' | 'pdf'>('png')
const goVersion = ref<GOVersionInfo | null>(null)
const updatingGO = ref(false)

const selected = ref<{ termId: string; description: string; source: Source; genes: string[]; count: number; qvalue: number } | null>(null)
const expr = ref<ExpressionResult | null>(null)
const sampleInfos = ref<SampleInfo[]>([])
const PALETTE = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b', '#e377c2', '#7f7f7f', '#bcbd22', '#17becf', '#aec7e8', '#ffbb78']

function exportUrl(type: 'go' | 'kegg'): string {
  if (!store.jobId || !store.selectedModule) return '#'
  return `/api/jobs/${store.jobId}/plots/enrich?module=${encodeURIComponent(store.selectedModule)}&type=${type}&fdr=${fdr.value}&top=${top.value}&mode=${exportMode.value}`
}

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

function ratioFrac(r: string): number {
  const [k, n] = r.split('/').map(Number)
  return n > 0 ? k / n : 0
}

function sigColor(q: number): string {
  const t = q >= 0.1 ? 0 : 1 - q / 0.1
  return mixHex('#2166ac', '#b2182b', t)
}

function makePoints(results: EnrichResult[], source: Source): BubblePoint[] {
  return results.map((r) => ({
    value: [ratioFrac(r.gene_ratio), r.description, r.count, r.qvalue],
    termId: r.id,
    description: r.description,
    genes: r.gene_id,
    source,
  }))
}

function optionFromPoints(points: BubblePoint[], title: string, height: string): EChartsOption {
  const maxCount = Math.max(1, ...points.map((p) => p.value[2]))
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: {
      trigger: 'item',
      formatter: (p: any) => {
        const d = p.data as BubblePoint
        return `<b>${d.termId}</b> (${d.source})<br/>${d.description}<br/>GeneRatio ${d.value[0].toFixed(3)} · Count ${d.value[2]}<br/>p.adjust ${d.value[3].toExponential(2)}`
      },
    },
    grid: { left: 300, right: 40, top: 42, bottom: 36 },
    xAxis: { type: 'value', name: 'GeneRatio', min: 0 },
    yAxis: {
      type: 'category',
      data: points.map((p) => p.description),
      axisLabel: { width: 280, overflow: 'truncate', fontSize: 11 },
    },
    series: [
      {
        type: 'scatter',
        data: points,
        symbolSize: (val: number[]) => 8 + (val[2] / maxCount) * 26,
        itemStyle: { color: (p: any) => sigColor(p.value[3]), opacity: 0.85 },
      },
    ],
  }
}

const goOption = computed(() => optionFromPoints(makePoints(goResults.value, 'GO'), 'GO ' + t('enrich.title'), '460px'))
const keggOption = computed(() => optionFromPoints(makePoints(keggResults.value, 'KEGG'), 'KEGG ' + t('enrich.title'), '460px'))
const combinedOption = computed(() =>
  optionFromPoints([...makePoints(goResults.value, 'GO'), ...makePoints(keggResults.value, 'KEGG')], 'GO + KEGG ' + t('enrich.title'), '540px'),
)

async function load() {
  if (!store.jobId || !store.selectedModule) return
  loading.value = true
  error.value = ''
  selected.value = null
  expr.value = null
  try {
    const [go, kegg] = await Promise.all([
      getEnrich(store.jobId, store.selectedModule, 'go', fdr.value, top.value),
      getEnrich(store.jobId, store.selectedModule, 'kegg', fdr.value, top.value),
    ])
    goResults.value = go
    keggResults.value = kegg
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function onChartClick(params: any) {
  const d = params.data as BubblePoint | undefined
  if (!d || !d.genes?.length) return
  selected.value = { termId: d.termId, description: d.description, source: d.source, genes: d.genes, count: d.value[2], qvalue: d.value[3] }
  try {
    expr.value = await getExpression(store.jobId, d.genes)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
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
  const samples = expr.value.samples
  const geneLabels = rows.map((r) => r.symbol || r.gene)

  const heat: [number, number, number][] = []
  z.forEach((row, gi) => row.forEach((v, si) => heat.push([si, gi, +v.toFixed(3)])))
  let maxAbs = 1
  z.forEach((row) => row.forEach((v) => (maxAbs = Math.max(maxAbs, Math.abs(v)))))

  const cats = samples.map((s) => sampleCategory(s))
  const uniq = [...new Set(cats)].sort()
  const pal: Record<string, string> = {}
  uniq.forEach((v, i) => { pal[v] = PALETTE[i % PALETTE.length] })
  const n = samples.length

  return {
    title: { text: (selected.value?.description ?? '') + ' (' + rows.length + ' genes)', left: 'center', textStyle: { fontSize: 13 } },
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
      { left: 170, right: 30, top: 46, bottom: 56 },
      { left: 170, right: 30, bottom: 16, height: 22 },
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
      min: -maxAbs, max: maxAbs, calculable: true, orient: 'horizontal', left: 'center', bottom: 40,
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
      <h6 class="fw-semibold mb-0"><i class="bi bi-bar-chart me-1"></i>{{ t('enrich.title') }}</h6>
      <label class="small mb-0 d-flex align-items-center gap-1">{{ t('enrich.fdr') }}
        <input type="number" class="form-control form-control-sm" style="width:80px" v-model.number="fdr" step="0.01" min="0.01" max="1" />
      </label>
      <label class="small mb-0 d-flex align-items-center gap-1">{{ t('enrich.top') }}
        <input type="number" class="form-control form-control-sm" style="width:72px" v-model.number="top" min="5" max="100" />
      </label>
      <div class="btn-group btn-group-sm">
        <button class="btn" :class="mode === 'separate' ? 'btn-primary' : 'btn-outline-primary'" @click="mode = 'separate'">{{ t('enrich.separate') }}</button>
        <button class="btn" :class="mode === 'combined' ? 'btn-primary' : 'btn-outline-primary'" @click="mode = 'combined'">{{ t('enrich.combined') }}</button>
      </div>
      <button class="btn btn-sm btn-primary" :disabled="loading || !store.selectedModule" @click="load"><i class="bi bi-arrow-repeat me-1"></i>{{ t('enrich.run') }}</button>
      <span v-if="goVersion" class="small text-muted">
        GO {{ goVersion.current }}
        <button v-if="goVersion.update_available" class="btn btn-sm btn-outline-warning py-0 px-1 ms-1" :disabled="updatingGO" @click="doUpdateGO">
          {{ updatingGO ? '…' : t('enrich.update') + ' ' + goVersion.latest }}
        </button>
      </span>
    </div>

    <div v-if="!loading && (goResults.length || keggResults.length)" class="d-flex align-items-center gap-2 mb-2">
      <span class="text-muted small">{{ t('plot.export') }}</span>
      <select v-model="exportMode" class="form-select form-select-sm" style="width:90px">
        <option value="png">PNG</option>
        <option value="svg">SVG</option>
        <option value="pdf">PDF</option>
      </select>
      <a class="btn btn-sm btn-outline-secondary" :href="exportUrl('go')" download>GO</a>
      <a class="btn btn-sm btn-outline-secondary" :href="exportUrl('kegg')" download>KEGG</a>
    </div>

    <div v-if="error" class="alert alert-warning py-2 small">{{ error }}</div>
    <div v-if="loading" class="text-muted small py-2">{{ t('common.running') }}</div>
    <template v-else>
      <div v-if="mode === 'separate'" class="row g-3">
        <div class="col-lg-6"><EChart :option="goOption" height="460px" @chart-click="onChartClick" /></div>
        <div class="col-lg-6"><EChart :option="keggOption" height="460px" @chart-click="onChartClick" /></div>
      </div>
      <div v-else><EChart :option="combinedOption" height="540px" @chart-click="onChartClick" /></div>
    </template>

    <div v-if="selected" class="mt-3">
      <h6 class="fw-semibold">{{ selected.source }} · {{ selected.termId }} · {{ selected.description }}（{{ selected.genes.length }} genes）</h6>
      <SimpleTable :columns="[
        { key: 'gene', label: 'Gene' },
        { key: 'symbol', label: 'Symbol' },
        { key: 'gene_name', label: 'Gene name' },
        { key: 'mean', label: 'Mean expr' },
      ]" :rows="geneRows" scroll-height="320px" :filename="selected.termId + '_genes'" />
      <EChart :option="heatOption" height="520px" />
    </div>
  </div>
</template>
