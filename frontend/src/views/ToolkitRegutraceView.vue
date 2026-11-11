<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { EChartsOption } from 'echarts'
import {
  cancelRegutraceJob,
  createRegutraceJob,
  getRegutraceInfo,
  getRegutraceJob,
  getRegutracePromoter,
  queryRegutrace,
  regutraceExportUrl,
  startRegutraceJob,
  type RegutraceCountItem,
  type RegutraceExportFormat,
  type RegutraceInfo,
  type RegutraceJob,
  type RegutraceParams,
  type RegutracePromoterHit,
  type RegutracePromoterView,
  type RegutraceQueryRow,
} from '../api/client'
import { locale } from '../utils/i18n'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import SimpleTable from '../components/SimpleTable.vue'
import EChart from '../components/EChart.vue'

const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const route = useRoute()
const router = useRouter()

const genomeFile = ref<File | null>(null)
const gffFile = ref<File | null>(null)
const motifsFile = ref<File | null>(null)
const tfMapFile = ref<File | null>(null)
const annotationFile = ref<File | null>(null)
const kingdom = ref('plants')
const info = ref<RegutraceInfo | null>(null)

const fallbackLibraries: Record<string, { name: string; count: number }> = {
  plants: { name: 'JASPAR 2024 CORE plants (non-redundant)', count: 805 },
  vertebrates: { name: 'JASPAR 2024 CORE vertebrates (non-redundant)', count: 879 },
  insects: { name: 'JASPAR 2024 CORE insects (non-redundant)', count: 286 },
  nematodes: { name: 'JASPAR 2024 CORE nematodes (non-redundant)', count: 103 },
  fungi: { name: 'JASPAR 2024 CORE fungi (non-redundant)', count: 178 },
  urochordates: { name: 'JASPAR 2024 CORE urochordates (non-redundant)', count: 94 },
  diatoms: { name: 'JASPAR 2024 CORE diatoms (non-redundant)', count: 1 },
}

const promoterLen = ref(1000)
const downstream = ref(0)
const pvalue = ref(1e-4)
const background = ref<[number, number, number, number]>([0.25, 0.25, 0.25, 0.25])
const threads = ref(0)

const job = ref<RegutraceJob | null>(null)
const jobId = ref('')
const uploadBusy = ref(false)
const uploadProgress = ref(0)
const actionBusy = ref(false)
const progressOpen = ref(false)
const error = ref('')
const now = ref(Date.now())
let pollTimer: number | undefined
let clockTimer: number | undefined
let lastResultJob = ''

const queryDirection = ref<'tf' | 'target'>('tf')
const queryText = ref('')
const queryMotif = ref('')
const queryMaxP = ref('')
const queryLimit = ref(200)
const queryRows = ref<RegutraceQueryRow[]>([])
const queryTotal = ref(0)
const queryBusy = ref(false)
const queryError = ref('')
const queryLabel = ref('')
const querySection = ref<HTMLElement>()

const chartMode = ref<'tf' | 'motif'>('tf')
const exportFormat = ref<RegutraceExportFormat>('sqlite')

const promoter = ref<RegutracePromoterView | null>(null)
const promoterBusy = ref(false)
const promoterError = ref('')
const selectedHit = ref<RegutracePromoterHit | null>(null)
const siteLimit = ref(30)
const siteTf = ref('')
const hover = ref<{ x: number; y: number; hit: RegutracePromoterHit } | null>(null)
const trackWrap = ref<HTMLElement>()
const promoterSection = ref<HTMLElement>()

const running = computed(() => job.value?.status === 'validating' || job.value?.status === 'running')
const busy = computed(() => uploadBusy.value || actionBusy.value || running.value)
const summary = computed(() => job.value?.summary ?? null)
const diagnostics = computed(() => job.value?.diagnostics ?? null)
const selectedLibrary = computed(() => {
  const found = info.value?.libraries?.find((lib) => lib.key === kingdom.value)
  if (found) return found
  const fallback = fallbackLibraries[kingdom.value]
  return fallback ? { key: kingdom.value, name: fallback.name, count: fallback.count } : null
})
const kingdomOptions = computed(() => [
  { key: 'plants', label: text('植物 / Plants', 'Plants') },
  { key: 'vertebrates', label: text('脊椎动物 / Vertebrates', 'Vertebrates') },
  { key: 'insects', label: text('昆虫 / Insects', 'Insects') },
  { key: 'nematodes', label: text('线虫 / Nematodes', 'Nematodes') },
  { key: 'fungi', label: text('真菌 / Fungi', 'Fungi') },
  { key: 'urochordates', label: text('尾索动物 / Urochordates', 'Urochordates') },
  { key: 'diatoms', label: text('硅藻 / Diatoms', 'Diatoms') },
  { key: 'bacteria', label: text('细菌 / Bacteria（需上传 motif）', 'Bacteria (upload motifs)') },
  { key: 'archaea', label: text('古菌 / Archaea（需上传 motif）', 'Archaea (upload motifs)') },
  { key: 'viruses', label: text('病毒 / Viruses（需上传 motif）', 'Viruses (upload motifs)') },
  { key: 'protists', label: text('原生生物 / Protists（需上传 motif）', 'Protists (upload motifs)') },
  { key: 'custom', label: text('其他 / 自定义 JASPAR PFM', 'Other / custom JASPAR PFM') },
])
const motifReady = computed(() => !!selectedLibrary.value || !!motifsFile.value)
const filesReady = computed(() => !!genomeFile.value && !!gffFile.value && motifReady.value)
const paramsDirty = computed(() => {
  if (!job.value) return false
  const p = job.value.params
  return p.promoter_len !== promoterLen.value || p.downstream !== downstream.value ||
    Math.abs(p.pvalue - pvalue.value) > 1e-15 || p.threads !== threads.value ||
    p.background.some((v, i) => Math.abs(v - background.value[i]) > 1e-12)
})

const statusLabel = computed(() => statusText(job.value?.status))
const stageLabel = computed(() => stageText(job.value?.stage))
const statusBadge = computed(() => {
  switch (job.value?.status) {
    case 'succeeded': return 'text-bg-success'
    case 'failed': return 'text-bg-danger'
    case 'cancelled': return 'text-bg-secondary'
    case 'validating':
    case 'running': return 'text-bg-warning'
    case 'validated': return 'text-bg-info'
    default: return 'text-bg-light border'
  }
})
const progressPct = computed(() => {
  const p = Math.max(0, Math.min(1, job.value?.progress ?? 0))
  if (job.value?.status === 'validating' && p < 0.05) return 6
  return Math.round(p * 100)
})
const elapsedText = computed(() => {
  if (!job.value) return ''
  const start = new Date(job.value.created_at).getTime()
  const end = running.value ? now.value : new Date(job.value.updated_at).getTime()
  if (!Number.isFinite(start) || !Number.isFinite(end)) return ''
  return formatDuration(Math.max(0, end - start))
})
const progressTitle = computed(() => {
  if (uploadBusy.value) return text('上传输入', 'Uploading inputs')
  if (job.value?.status === 'validating') return text('校验输入', 'Validating inputs')
  return text('运行 ReguTrace', 'Running ReguTrace')
})
const overlayPct = computed(() => uploadBusy.value ? Math.round(uploadProgress.value * 100) : progressPct.value)
const overlayMessage = computed(() => uploadBusy.value
  ? text('正在上传 genome / GFF3 / motif 文件…', 'Uploading genome / GFF3 / motif files…')
  : (job.value?.message || '…'))

const chartItems = computed<RegutraceCountItem[]>(() => {
  if (!summary.value) return []
  return chartMode.value === 'tf' ? summary.value.top_tfs : summary.value.top_motifs
})
const chartDisplayItems = computed(() => [...chartItems.value].reverse())
const chartOption = computed<EChartsOption>(() => {
  const items = chartDisplayItems.value
  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        const item = items[p?.dataIndex ?? -1]
        if (!item) return ''
        return `<b>${escapeHtml(item.name || item.id)}</b><br/>targets: ${item.targets}<br/>evidence rows: ${item.edges}`
      },
    },
    grid: { left: 150, right: 34, top: 8, bottom: 42 },
    xAxis: {
      type: 'value',
      name: text('靶基因数', 'targets'),
      nameLocation: 'middle',
      nameGap: 22,
      nameTextStyle: { color: '#64748b' },
      splitLine: { lineStyle: { color: '#e2e8f0' } },
    },
    yAxis: {
      type: 'category',
      data: items.map((i) => chartMode.value === 'tf' ? (i.name || i.id) : `${i.id} ${i.name}`.trim()),
      axisLabel: { width: 132, overflow: 'truncate', color: '#334155' },
      axisTick: { show: false },
    },
    series: [{
      type: 'bar',
      data: items.map((i) => i.targets),
      barMaxWidth: 16,
      itemStyle: { color: chartMode.value === 'tf' ? '#0f766e' : '#d97706', borderRadius: [0, 4, 4, 0] },
    }],
  }
})
const chartHeight = computed(() => `${Math.max(230, chartDisplayItems.value.length * 24 + 58)}px`)

const tfOptions = computed(() => {
  if (!promoter.value) return []
  const names = new Set<string>()
  for (const h of promoter.value.hits) for (const tf of h.tfs) names.add(tf.symbol || tf.id)
  return [...names].sort()
})
const promoterHits = computed(() => {
  if (!promoter.value) return []
  const filter = siteTf.value.trim().toLowerCase()
  const list = promoter.value.hits.filter((h) => {
    if (!filter) return true
    return h.tfs.some((tf) => (tf.symbol || tf.id).toLowerCase().includes(filter))
  })
  list.sort((a, b) => {
    const ap = a.p_value ?? 2
    const bp = b.p_value ?? 2
    if (ap !== bp) return ap - bp
    return (b.score ?? -Infinity) - (a.score ?? -Infinity)
  })
  return siteLimit.value > 0 ? list.slice(0, siteLimit.value) : list
})
const totalSites = computed(() => {
  if (!promoter.value) return 0
  const filter = siteTf.value.trim().toLowerCase()
  return promoter.value.hits.filter((h) => !filter || h.tfs.some((tf) => (tf.symbol || tf.id).toLowerCase().includes(filter))).length
})
const siteOptions = [
  { value: 20, label: '20' },
  { value: 30, label: '30' },
  { value: 50, label: '50' },
  { value: 100, label: '100' },
  { value: 0, label: text('全部', 'All') },
]

interface SiteLayout {
  hit: RegutracePromoterHit
  x: number
  y: number
  width: number
  lane: number
  plus: boolean
}
const track = computed(() => {
  const p = promoter.value
  if (!p) return { sites: [] as SiteLayout[], height: 180, baseline: 100, left: 40, width: 920 }
  const LEFT = 40
  const WIDTH = 920
  const plusLanes: number[] = []
  const minusLanes: number[] = []
  const placed: { hit: RegutracePromoterHit; lane: number; plus: boolean; x: number; width: number }[] = []
  const sorted = [...promoterHits.value].sort((a, b) => a.start - b.start || a.end - b.end)
  for (const hit of sorted) {
    const plus = hit.strand !== '-'
    const lanes = plus ? plusLanes : minusLanes
    let lane = lanes.findIndex((end) => end < hit.start)
    if (lane < 0) {
      lane = lanes.length
      lanes.push(0)
    }
    lanes[lane] = hit.end
    placed.push({
      hit,
      lane,
      plus,
      x: LEFT + ((hit.start - 1) / p.length) * WIDTH,
      width: Math.max(3, ((hit.end - hit.start + 1) / p.length) * WIDTH),
    })
  }
  const baseline = 28 + plusLanes.length * 13
  const height = Math.max(180, baseline + 30 + minusLanes.length * 13)
  const sites = placed.map((s) => ({
    ...s,
    y: s.plus ? baseline - 13 - s.lane * 13 : baseline + 7 + s.lane * 13,
  }))
  return { sites, height, baseline, left: LEFT, width: WIDTH }
})
const tssX = computed(() => {
  const p = promoter.value
  if (!p) return 40
  const pos = Math.max(1, Math.min(p.length, p.tss_position || p.length))
  return 40 + ((pos - 1) / p.length) * 920
})
const rulerTicks = computed(() => {
  const p = promoter.value
  if (!p) return []
  return [0, 0.25, 0.5, 0.75, 1].map((f) => ({
    x: 40 + f * 920,
    label: `${Math.round(f * p.length)} bp`,
  }))
})

const queryColumns = [
  { key: 'tf_symbol', label: 'TF' },
  { key: 'tf_family', label: text('家族', 'Family') },
  { key: 'target_symbol', label: text('靶基因', 'Target') },
  { key: 'target_id', label: 'Target ID' },
  { key: 'target_description', label: text('注释', 'Annotation') },
  { key: 'motif_id', label: 'Motif' },
  { key: 'score', label: 'Score', fmt: (v: any) => v == null ? '—' : Number(v).toFixed(2) },
  { key: 'p_value', label: 'p', fmt: (v: any) => formatP(v) },
  { key: 'hit_start', label: text('位点', 'Site'), fmt: (_v: any, row: Record<string, any>) => row.hit_start == null ? '—' : `${row.hit_start}-${row.hit_end}${row.strand ?? ''}` },
  { key: 'annotation_source', label: text('注释来源', 'Annotation source') },
]

function statusText(status?: string): string {
  switch (status) {
    case 'uploaded': return text('已上传', 'Uploaded')
    case 'validating': return text('校验中', 'Validating')
    case 'validated': return text('校验通过', 'Validated')
    case 'running': return text('运行中', 'Running')
    case 'succeeded': return text('已完成', 'Succeeded')
    case 'failed': return text('失败', 'Failed')
    case 'cancelled': return text('已取消', 'Cancelled')
    default: return text('未开始', 'Not started')
  }
}

function stageText(stage?: string): string {
  switch (stage) {
    case 'starting': return text('启动', 'Starting')
    case 'reading_motifs': return text('读取 motif 库', 'Reading motifs')
    case 'reading_promoters': return text('读取 promoter FASTA', 'Reading promoter FASTA')
    case 'reading_genome': return text('读取基因组', 'Reading genome')
    case 'reading_gff': return text('读取 GFF3', 'Reading GFF3')
    case 'extracting_promoters': return text('提取 promoter', 'Extracting promoters')
    case 'writing_promoters': return text('写出 promoter', 'Writing promoters')
    case 'scanning': return text('扫描 motif', 'Scanning motifs')
    case 'writing_evidence': return text('写出证据表', 'Writing evidence')
    case 'validating': return text('校验输入', 'Validating inputs')
    case 'validated': return text('校验完成', 'Validated')
    case 'done':
    case 'succeeded': return text('完成', 'Done')
    case 'failed': return text('失败', 'Failed')
    case 'cancelled': return text('已取消', 'Cancelled')
    case 'uploaded': return text('已上传', 'Uploaded')
    default: return stage ?? ''
  }
}

function formatP(v: any): string {
  if (v == null || v === '') return '—'
  const n = Number(v)
  if (!Number.isFinite(n)) return String(v)
  if (n === 0) return '0'
  if (n < 1e-3) return n.toExponential(2)
  return n.toPrecision(3)
}

function formatDuration(ms: number): string {
  const sec = Math.floor(ms / 1000)
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  const rem = sec % 60
  if (min < 60) return `${min}m ${rem}s`
  return `${Math.floor(min / 60)}h ${min % 60}m`
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c] ?? c))
}

function onFile(target: 'genome' | 'gff' | 'motifs' | 'tfMap' | 'annotation', event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0] ?? null
  if (target === 'genome') genomeFile.value = file
  if (target === 'gff') gffFile.value = file
  if (target === 'motifs') motifsFile.value = file
  if (target === 'tfMap') tfMapFile.value = file
  if (target === 'annotation') annotationFile.value = file
  resetJob()
}

function syncJobQuery(id: string | null) {
  const q = { ...route.query }
  if (id) {
    q.job = id
    delete q.target
  } else {
    delete q.job
    delete q.target
  }
  void router.replace({ query: q })
}

function resetJob() {
  stopPoll()
  job.value = null
  jobId.value = ''
  error.value = ''
  queryRows.value = []
  queryTotal.value = 0
  promoter.value = null
  selectedHit.value = null
  lastResultJob = ''
  syncJobQuery(null)
}

function onKingdomChange() {
  if (selectedLibrary.value) motifsFile.value = null
  resetJob()
}

function currentParams(): RegutraceParams {
  const requestedThreads = Number(threads.value)
  return {
    promoter_len: promoterLen.value,
    downstream: downstream.value,
    pvalue: pvalue.value,
    background: [...background.value] as [number, number, number, number],
    threads: Number.isFinite(requestedThreads) ? Math.max(0, Math.trunc(requestedThreads)) : 0,
  }
}

async function ensureJob(): Promise<string> {
  if (jobId.value) return jobId.value
  if (!genomeFile.value || !gffFile.value) throw new Error(text('请先选择 genome FASTA 和 GFF3。', 'Choose a genome FASTA and GFF3 first.'))
  const lib = selectedLibrary.value
  if (!lib && !motifsFile.value) throw new Error(text('该物种族没有内置 JASPAR 库，请上传自定义 JASPAR PFM。', 'No bundled JASPAR library for this taxon; upload a custom JASPAR PFM.'))
  uploadBusy.value = true
  uploadProgress.value = 0
  try {
    const j = await createRegutraceJob({
      genome: genomeFile.value,
      gff: gffFile.value,
      motifs: lib ? null : motifsFile.value,
      motifLibrary: lib?.key,
      tfMap: tfMapFile.value,
      annotation: annotationFile.value,
    }, (f) => { uploadProgress.value = f })
    job.value = j
    jobId.value = j.id
    syncJobQuery(j.id)
    return j.id
  } finally {
    uploadBusy.value = false
  }
}

async function start(mode: 'validate' | 'run') {
  error.value = ''
  actionBusy.value = true
  progressOpen.value = true
  try {
    const id = await ensureJob()
    const j = await startRegutraceJob(id, mode, currentParams())
    job.value = j
    if (mode === 'run') {
      queryRows.value = []
      queryTotal.value = 0
      promoter.value = null
      selectedHit.value = null
    }
    startPoll()
  } catch (e) {
    progressOpen.value = false
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    actionBusy.value = false
  }
}

function startPoll() {
  stopPoll()
  pollTimer = window.setInterval(pollOnce, 800)
  void pollOnce()
}

function stopPoll() {
  if (pollTimer !== undefined) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
}

async function pollOnce() {
  if (!jobId.value) return
  try {
    const j = await getRegutraceJob(jobId.value)
    job.value = j
    if (j.status === 'succeeded') {
      stopPoll()
      progressOpen.value = false
      if (lastResultJob !== j.id) {
        lastResultJob = j.id
        await initialQuery(j)
      }
    } else if (j.status === 'failed' || j.status === 'cancelled' || j.status === 'validated') {
      stopPoll()
      progressOpen.value = false
    }
  } catch (e) {
    stopPoll()
    progressOpen.value = false
    error.value = e instanceof Error ? e.message : String(e)
  }
}

async function cancel() {
  if (!jobId.value) return
  try { await cancelRegutraceJob(jobId.value) } catch (e) { error.value = e instanceof Error ? e.message : String(e) }
}

async function initialQuery(j: RegutraceJob) {
  if ((j.summary?.tf_target_edges ?? 0) > 200000) {
    queryRows.value = []
    queryTotal.value = j.summary?.tf_target_edges ?? 0
    queryLabel.value = text('结果较大，请输入 TF 或 target 后查询（或直接下载完整证据表）', 'Large result: enter a TF or target to query, or download the full evidence table')
    return
  }
  await runQuery()
}

async function runQuery() {
  if (!job.value || job.value.status !== 'succeeded') return
  if (!queryText.value.trim() && !queryMotif.value.trim() && (job.value.summary?.tf_target_edges ?? 0) > 200000) {
    queryError.value = text('结果较大，请输入 TF、target 或 motif 后再查询。', 'This result is large; enter a TF, target or motif before searching.')
    return
  }
  queryBusy.value = true
  queryError.value = ''
  try {
    const maxP = queryMaxP.value.trim() === '' ? undefined : Number(queryMaxP.value)
    const res = await queryRegutrace(job.value.id, {
      direction: queryDirection.value,
      q: queryText.value,
      motif: queryMotif.value,
      maxP: Number.isFinite(maxP as number) ? maxP : undefined,
      limit: queryLimit.value,
      offset: 0,
    })
    queryRows.value = res.rows
    queryTotal.value = res.total
    queryLabel.value = queryText.value || queryMotif.value
      ? text('匹配结果', 'Matching evidence')
      : text('按 p 值排序的预览（前 ' + res.limit + ' 条）', 'Preview by p-value (top ' + res.limit + ')')
  } catch (e) {
    queryError.value = e instanceof Error ? e.message : String(e)
  } finally {
    queryBusy.value = false
  }
}

async function queryByTf(name: string) {
  queryDirection.value = 'tf'
  queryText.value = name
  queryMotif.value = ''
  await runQuery()
  nextTick(() => querySection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
}

function onChartClick(params: unknown) {
  const idx = (params as { dataIndex?: number }).dataIndex
  if (idx == null) return
  const item = chartDisplayItems.value[idx]
  if (!item) return
  if (chartMode.value === 'tf') {
    queryDirection.value = 'tf'
    queryText.value = item.name || item.id
    queryMotif.value = ''
  } else {
    queryDirection.value = 'tf'
    queryText.value = ''
    queryMotif.value = item.id
  }
  void runQuery()
  nextTick(() => querySection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
}

async function openPromoter(targetId: string) {
  if (!job.value || !targetId) return
  promoterBusy.value = true
  promoterError.value = ''
  selectedHit.value = null
  siteTf.value = ''
  try {
    promoter.value = await getRegutracePromoter(job.value.id, targetId)
    void router.replace({ query: { ...route.query, job: job.value.id, target: targetId } })
    nextTick(() => promoterSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
  } catch (e) {
    promoterError.value = e instanceof Error ? e.message : String(e)
  } finally {
    promoterBusy.value = false
  }
}

function onQueryRowClick(row: Record<string, any>) {
  void openPromoter(String(row.target_id ?? ''))
}

function showHover(event: Event, hit: RegutracePromoterHit) {
  const rect = trackWrap.value?.getBoundingClientRect()
  const mouse = event as MouseEvent
  const x = typeof mouse.clientX === 'number' ? mouse.clientX : (rect?.left ?? 0) + 20
  const y = typeof mouse.clientY === 'number' ? mouse.clientY : (rect?.top ?? 0) + 20
  hover.value = {
    hit,
    x: x - (rect?.left ?? 0) + 12,
    y: y - (rect?.top ?? 0) + 12,
  }
}

function hoverTfs(hit: RegutracePromoterHit): string {
  return hit.tfs.map((t) => t.symbol || t.id).join(', ')
}

function baseSpans(seq: string) {
  return [...seq].map((b) => ({ b, cls: `base-${b.toLowerCase()}` }))
}

async function copyDiagnostics() {
  if (!job.value?.diagnostics) return
  try { await navigator.clipboard.writeText(JSON.stringify(job.value.diagnostics, null, 2)) } catch { /* clipboard unavailable */ }
}

function copySequence(seq: string) {
  void navigator.clipboard?.writeText(seq)
}

function applyJobParams(j: RegutraceJob) {
  promoterLen.value = j.params.promoter_len
  downstream.value = j.params.downstream
  pvalue.value = j.params.pvalue
  background.value = [...j.params.background] as [number, number, number, number]
  threads.value = j.params.threads ?? 0
}

onMounted(async () => {
  clockTimer = window.setInterval(() => { now.value = Date.now() }, 1000)
  try { info.value = await getRegutraceInfo() } catch { /* optional */ }
  const q = route.query.job
  if (typeof q === 'string' && q) {
    jobId.value = q
    try {
      const j = await getRegutraceJob(q)
      job.value = j
      const key = j.inputs.motif_library_key
      if (key && kingdomOptions.value.some((o) => o.key === key)) kingdom.value = key
      else if (key === 'custom') kingdom.value = 'custom'
      applyJobParams(j)
      if (j.status === 'succeeded') {
        lastResultJob = j.id
        await initialQuery(j)
        const target = route.query.target
        if (typeof target === 'string' && target) await openPromoter(target)
      } else if (j.status === 'validating' || j.status === 'running') {
        progressOpen.value = true
        startPoll()
      }
    } catch (e) {
      jobId.value = ''
      error.value = e instanceof Error ? e.message : String(e)
    }
  }
})
onBeforeUnmount(() => {
  stopPoll()
  if (clockTimer !== undefined) window.clearInterval(clockTimer)
})
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <h6 class="fw-semibold mb-1">{{ text('TF–Target 预测', 'TF–target prediction') }}</h6>
      <p class="text-muted small mb-3">
        {{ text('本地扫描 promoter 上的 PWM 结合位点，输出带坐标的 TF–target 候选证据。结果级别为 D（计算预测），不是体内 binding 证据。', 'Scan promoter PWM sites locally and produce coordinate-aware TF–target candidates. Results are level D (computational prediction), not in vivo binding evidence.') }}
      </p>

      <div class="mb-3">
        <label class="form-label small mb-1">Genome FASTA <span class="text-danger">*</span></label>
        <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-2">
          <i class="bi bi-upload me-1"></i>{{ genomeFile ? genomeFile.name : text('选择 FASTA / .gz', 'Choose FASTA / .gz') }}
          <input type="file" accept=".fa,.fasta,.fna,.gz" class="d-none" @change="onFile('genome', $event)" />
        </label>
        <label class="form-label small mb-1">GFF3 <span class="text-danger">*</span></label>
        <label class="btn btn-outline-secondary w-100 text-truncate btn-sm">
          <i class="bi bi-upload me-1"></i>{{ gffFile ? gffFile.name : text('选择 GFF3 / .gz', 'Choose GFF3 / .gz') }}
          <input type="file" accept=".gff,.gff3,.gz" class="d-none" @change="onFile('gff', $event)" />
        </label>
      </div>

      <div class="mb-3">
        <label class="form-label small mb-1">{{ text('Kingdom / 类群（选择 motif 库）', 'Kingdom / taxon (selects motif library)') }}</label>
        <select v-model="kingdom" class="form-select form-select-sm mb-1" @change="onKingdomChange">
          <option v-for="opt in kingdomOptions" :key="opt.key" :value="opt.key">{{ opt.label }}</option>
        </select>
        <p v-if="selectedLibrary" class="small text-muted mb-0">
          {{ selectedLibrary.name }} · {{ selectedLibrary.count }} motifs
          · <a href="https://jaspar.elixir.no/" target="_blank" rel="noopener noreferrer">JASPAR</a> CC BY 4.0
        </p>
        <template v-else>
          <p class="small text-warning mb-1">{{ text('该物种族没有内置 JASPAR CORE 库；请上传 JASPAR PFM（来源见下方指南）。', 'No bundled JASPAR CORE library for this taxon; upload a JASPAR PFM (sources below).') }}</p>
          <label class="btn btn-outline-secondary w-100 text-truncate btn-sm">
            <i class="bi bi-upload me-1"></i>{{ motifsFile ? motifsFile.name : text('选择 JASPAR PFM 文本', 'Choose JASPAR PFM text') }}
            <input type="file" accept=".txt,.pfm,.jaspar,.gz" class="d-none" @change="onFile('motifs', $event)" />
          </label>
        </template>
        <p class="small text-muted mt-1 mb-0">{{ text('默认全量扫描；不会跳过 motif。内置 JASPAR taxa：植物、脊椎动物、昆虫、线虫、真菌、尾索动物、硅藻；细菌/古菌/病毒/原生生物需自定义上传。全基因组 + 数百 motifs 可能需要数分钟到数十分钟。', 'All motifs are scanned; nothing is skipped. Bundled JASPAR taxa: plants, vertebrates, insects, nematodes, fungi, urochordates, diatoms; bacteria/archaea/viruses/protists need a custom upload. A whole genome × hundreds of motifs can take minutes to tens of minutes.') }}</p>
        <details class="rt-motif-help mt-2">
          <summary class="small">{{ text('类群与 motif 库来源', 'Taxa and motif library sources') }}</summary>
          <div class="small text-muted mt-1">
            <p class="mb-1">{{ text('ReguTrace 只读 JASPAR PFM 文本（>ID name + A/C/G/T 矩阵）。', 'ReguTrace reads JASPAR PFM text only (>ID name + A/C/G/T rows).') }}</p>
            <ul class="mb-1 ps-3">
              <li><b>{{ text('通用真核', 'Eukaryotes') }}</b>: <a href="https://jaspar.elixir.no/downloads/" target="_blank" rel="noopener noreferrer">JASPAR downloads</a>, <a href="https://cisbp.ccbr.utoronto.ca/" target="_blank" rel="noopener noreferrer">CIS-BP</a></li>
              <li><b>{{ text('动物', 'Animals') }}</b>: {{ text('内置 JASPAR vertebrates / insects / nematodes / urochordates', 'Built-in JASPAR vertebrates / insects / nematodes / urochordates') }}; <a href="https://hocomoco14.autosome.org/downloads_v14" target="_blank" rel="noopener noreferrer">HOCOMOCO</a> (human/mouse); <a href="http://uniprobe.org/" target="_blank" rel="noopener noreferrer">UniPROBE</a></li>
              <li><b>{{ text('植物', 'Plants') }}</b>: {{ text('内置 JASPAR plants；另有 CIS-BP plants、PlantTFDB（站点可能不稳定）', 'Built-in JASPAR plants; also CIS-BP plants and PlantTFDB (availability varies)') }}</li>
              <li><b>{{ text('真菌', 'Fungi') }}</b>: {{ text('内置 JASPAR fungi', 'Built-in JASPAR fungi') }}; <a href="http://yetfasco.ccbr.utoronto.ca/" target="_blank" rel="noopener noreferrer">YeTFaSCo</a> (S. cerevisiae); CIS-BP fungi</li>
              <li><b>{{ text('细菌', 'Bacteria') }}</b>: <a href="https://regulondb.ccg.unam.mx/" target="_blank" rel="noopener noreferrer">RegulonDB</a> (E. coli), <a href="https://www.prodoric.de/" target="_blank" rel="noopener noreferrer">PRODORIC</a>, <a href="http://www.collectf.org/" target="_blank" rel="noopener noreferrer">CollecTF</a></li>
              <li><b>{{ text('古菌', 'Archaea') }}</b>: PRODORIC, <a href="http://bioinformatics.zj.cn/archaeatf/" target="_blank" rel="noopener noreferrer">ArchaeaTF</a>, <a href="https://entraf.iimas.unam.mx/" target="_blank" rel="noopener noreferrer">ENTRAF</a>（覆盖较稀疏）</li>
              <li><b>{{ text('病毒 / 原生生物', 'Viruses / protists') }}</b>: {{ text('没有统一的病毒 TF motif 库；通常用宿主 JASPAR/CIS-BP。原生生物可查 CIS-BP 和 VEuPathDB/PlasmoDB，覆盖有限。', 'No unified viral TF motif library; use host JASPAR/CIS-BP. For protists try CIS-BP and VEuPathDB/PlasmoDB; coverage is limited.') }}</li>
            </ul>
            <p class="mb-0">{{ text('MEME / TRANSFAC / HOCOMOCO 等格式需要先转成 JASPAR PFM 再上传；本页暂未内置转换器。', 'MEME / TRANSFAC / HOCOMOCO matrices must be converted to JASPAR PFM before upload; no converter is built into this page yet.') }}</p>
          </div>
        </details>
      </div>

      <details class="mb-3">
        <summary class="small fw-semibold">{{ text('可选输入与参数', 'Optional inputs and parameters') }}</summary>
        <div class="mt-2">
          <label class="form-label small mb-1">TF–motif map ({{ text('可选', 'optional') }})</label>
          <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-2">
            <i class="bi bi-upload me-1"></i>{{ tfMapFile ? tfMapFile.name : text('上传 tf_map.tsv', 'Upload tf_map.tsv') }}
            <input type="file" accept=".tsv,.txt,.gz" class="d-none" @change="onFile('tfMap', $event)" />
          </label>
          <p class="small text-muted mb-2">{{ text('不传时用 motif header name 作为 TF ID；上传后可得到 tf_id / symbol / family。', 'Without it, the motif header name is used as the TF ID. Upload it to get tf_id / symbol / family.') }}</p>

          <label class="form-label small mb-1">{{ text('Annotation（可选）', 'Annotation (optional)') }}</label>
          <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-2">
            <i class="bi bi-upload me-1"></i>{{ annotationFile ? annotationFile.name : text('eggNOG .annotations 或注释 TSV', 'eggNOG .annotations or annotation TSV') }}
            <input type="file" accept=".tsv,.txt,.annotations,.gz" class="d-none" @change="onFile('annotation', $event)" />
          </label>
          <p class="small text-muted mb-2">{{ text('自动识别 eggNOG 或 gene_id / symbol / description TSV；没有注释时使用 GFF 的 Name。', 'Auto-detects eggNOG or a gene_id / symbol / description TSV. Without it, the GFF Name is used.') }}</p>

          <label class="form-label small mb-1">{{ text('Promoter 上游长度 (bp)', 'Promoter upstream length (bp)') }}</label>
          <input v-model.number="promoterLen" type="number" min="1" step="50" class="form-control form-control-sm mb-2" />
          <label class="form-label small mb-1">{{ text('TSS 下游长度 (bp)', 'Downstream of TSS (bp)') }}</label>
          <input v-model.number="downstream" type="number" min="0" step="10" class="form-control form-control-sm mb-2" />
          <label class="form-label small mb-1">{{ text('PWM p 值阈值', 'PWM p-value cutoff') }}</label>
          <input v-model.number="pvalue" type="number" min="0" max="1" step="0.00001" class="form-control form-control-sm mb-2" />
          <label class="form-label small mb-1">{{ text('背景碱基概率 A,C,G,T', 'Background probabilities A,C,G,T') }}</label>
          <div class="row g-1">
            <div v-for="(_, i) in background" :key="i" class="col-3">
              <input v-model.number="background[i]" type="number" min="0.000001" step="0.01" class="form-control form-control-sm" />
            </div>
          </div>
          <label class="form-label small mb-1 mt-2">{{ text('扫描线程数', 'Scan threads') }}</label>
          <input v-model.number="threads" type="number" min="0" max="256" step="1" class="form-control form-control-sm" />
          <p class="small text-muted mt-1 mb-0">{{ text('0 = 自动（CPU 核数 − 1）；1 = 单线程。', '0 = auto (CPU cores − 1); 1 = serial.') }}</p>
        </div>
      </details>

      <div class="d-grid gap-2">
        <button class="btn btn-outline-primary btn-sm" :disabled="!filesReady || busy" @click="start('validate')">
          <i class="bi bi-check2-circle me-1"></i>{{ text('检查输入', 'Validate inputs') }}
        </button>
        <button class="btn btn-primary btn-sm" :disabled="!filesReady || busy" @click="start('run')">
          <i class="bi bi-play-fill me-1"></i>{{ text('运行 ReguTrace', 'Run ReguTrace') }}
        </button>
      </div>

      <div v-if="error" class="alert alert-danger small mt-3 mb-0">{{ error }}</div>
      <p v-if="paramsDirty && job?.status === 'succeeded'" class="small text-warning mt-2 mb-0">
        <i class="bi bi-exclamation-triangle me-1"></i>{{ text('参数已修改，当前结果仍来自旧参数；重新运行后更新。', 'Parameters changed; the current results are from the previous run. Re-run to update.') }}
      </p>
    </template>

    <div class="mb-3">
      <h4 class="mb-1">{{ text('ReguTrace：TF–Target 候选', 'ReguTrace: TF–target candidates') }}</h4>
      <p class="text-muted small mb-0">
        {{ text('左侧上传 genome / GFF3，选择 Kingdom / motif 库和参数。运行后可查询 TF 或 target，点击结果行查看 promoter 上的结合位点。', 'Upload genome / GFF3 on the left, choose a Kingdom / motif library and parameters. Then query a TF or target and click a result row to inspect binding sites on the promoter.') }}
      </p>
    </div>

    <div v-if="!job && !uploadBusy" class="rt-empty">
      <i class="bi bi-diagram-3"></i>
      <h5>{{ text('从左侧开始', 'Start from the sidebar') }}</h5>
      <p class="text-muted small mb-0">
        {{ text('1. 选择 genome FASTA 和 GFF3；2. 选择 Kingdom / 类群（或上传自定义 JASPAR PFM）；3. 点击“运行 ReguTrace”。', '1. Choose genome FASTA and GFF3; 2. Choose a Kingdom / taxon (or upload a custom JASPAR PFM); 3. Click Run ReguTrace.') }}
      </p>
    </div>

    <div v-if="job?.status === 'failed'" class="alert alert-danger">
      <div class="fw-semibold mb-1"><i class="bi bi-x-circle me-1"></i>{{ text('运行失败', 'Run failed') }}</div>
      <div class="small" style="white-space: pre-wrap">{{ job.error }}</div>
    </div>
    <div v-if="job?.status === 'validated'" class="alert alert-info small">
      <i class="bi bi-check2-circle me-1"></i>{{ text('输入校验通过。可以点击左侧“运行 ReguTrace”开始扫描。', 'Inputs validated. Click Run ReguTrace on the left to start the scan.') }}
    </div>

    <template v-if="summary">
      <div class="d-flex align-items-start flex-wrap gap-2 mb-3">
        <div>
          <h5 class="mb-1">{{ text('本次 ReguTrace 结果', 'ReguTrace results') }}</h5>
          <div class="small text-muted">
            {{ text('独立 toolkit，不依赖 WGCNA 项目，也不要求先执行项目流程。', 'Standalone toolkit; it does not depend on a WGCNA project or a project run.') }}
            <span v-if="job?.id">· job <code>{{ job.id }}</code></span>
          </div>
        </div>
        <div class="ms-auto d-flex flex-wrap gap-2 align-items-center">
          <div class="input-group input-group-sm" style="width: 330px">
            <span class="input-group-text">{{ text('下载全量结果', 'Full export') }}</span>
            <select v-model="exportFormat" class="form-select" :aria-label="text('导出格式', 'Export format')">
              <option value="sqlite">SQLite (.db)</option>
              <option value="tsv">TSV (.tsv)</option>
              <option value="csv">CSV (.csv)</option>
              <option value="tsv.gz">TSV gzip (.tsv.gz)</option>
              <option value="csv.gz">CSV gzip (.csv.gz)</option>
              <option value="jsonl.gz">JSONL gzip (.jsonl.gz)</option>
            </select>
            <a class="btn btn-primary" :href="regutraceExportUrl(job!.id, 'evidence', exportFormat)" download
               :title="text('下载全量证据表；SQLite 可查询，CSV/TSV 可能超过 1 GB', 'Download the full evidence table; SQLite is queryable, CSV/TSV may exceed 1 GB')">
              <i class="bi bi-download me-1"></i>{{ text('下载', 'Download') }}
            </a>
          </div>
          <a class="btn btn-sm btn-outline-secondary" :href="regutraceExportUrl(job!.id, 'promoters')" download
             :title="text('下载本次扫描的启动子 FASTA', 'Download promoter FASTA for this scan')">
            <i class="bi bi-download me-1"></i>{{ text('启动子 FASTA', 'Promoter FASTA') }}
          </a>
          <a class="btn btn-sm btn-outline-secondary" :href="regutraceExportUrl(job!.id, 'report')" download
             :title="text('下载参数、诊断和运行摘要 JSON', 'Download parameters, diagnostics and run summary as JSON')">
            <i class="bi bi-download me-1"></i>{{ text('报告 JSON', 'Report JSON') }}
          </a>
        </div>
      </div>
      <p class="small text-muted mb-3">
        {{ text('SQLite 推荐用于查询和 Data Import；CSV/TSV 是全量 hit 表，Excel 最多只能打开 1,048,576 行。', 'SQLite is recommended for querying and Data Import; CSV/TSV are full hit tables and Excel is limited to 1,048,576 rows.') }}
      </p>

      <div class="row g-2 mb-3">
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ summary.genes.toLocaleString() }}</div><div class="stat-label">{{ text('基因 / promoters', 'Genes / promoters') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ summary.motif_hits.toLocaleString() }}</div><div class="stat-label">{{ text('motif 结合位点', 'Motif hits') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ summary.tf_target_pairs.toLocaleString() }}</div><div class="stat-label">{{ text('TF–target 关系（去重）', 'TF–target pairs') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ summary.unique_tfs.toLocaleString() }}</div><div class="stat-label">{{ text('TF 数', 'TFs') }}</div></div></div>
      </div>

      <div class="row g-2 mb-3 small text-muted">
        <div class="col-12">
          {{ text('Motif 库', 'Motif library') }}: {{ summary.motif_library_size.toLocaleString() }}
          · {{ text('有命中 motif', 'motifs with hits') }}: {{ summary.motifs_with_hits.toLocaleString() }}
          · {{ text('唯一 target', 'unique targets') }}: {{ summary.unique_targets.toLocaleString() }}
          · {{ text('证据行', 'evidence rows') }}: {{ summary.tf_target_edges.toLocaleString() }}
          <span v-if="summary.threads">· {{ text('线程数', 'threads') }}: {{ summary.threads }}</span>
          · {{ text('注释来源', 'annotation source') }}: {{ summary.annotation_source }}
        </div>
      </div>

      <div class="card mb-3">
        <div class="card-body">
          <div class="d-flex align-items-center flex-wrap gap-2 mb-2">
            <h6 class="mb-0">{{ text('TF / motif 的 target 数', 'Targets per TF / motif') }}</h6>
            <div class="btn-group btn-group-sm ms-auto">
              <button class="btn" :class="chartMode === 'tf' ? 'btn-primary' : 'btn-outline-primary'" @click="chartMode = 'tf'">TF</button>
              <button class="btn" :class="chartMode === 'motif' ? 'btn-primary' : 'btn-outline-primary'" @click="chartMode = 'motif'">Motif</button>
            </div>
          </div>
          <p class="small text-muted mb-2">{{ text('按唯一 target 数排序；点击柱子可直接查询。', 'Sorted by unique targets; click a bar to query it.') }}</p>
          <EChart v-if="chartItems.length" :option="chartOption" :height="chartHeight" @chart-click="onChartClick" />
          <p v-else class="text-muted small mb-0">{{ text('没有命中。', 'No hits.') }}</p>
        </div>
      </div>

      <div ref="querySection" class="card mb-3">
        <div class="card-body">
          <div class="d-flex align-items-center flex-wrap gap-2 mb-2">
            <h6 class="mb-0">{{ text('自由查询', 'Free query') }}</h6>
            <span class="small text-muted">{{ text('不打印全量关系表；完整结果用结果顶部的下载按钮。', 'No full table here; use the download buttons at the top of the results.') }}</span>
          </div>
          <div class="row g-2 align-items-end mb-2">
            <div class="col-12 col-md-3">
              <label class="form-label small mb-1">{{ text('查询方向', 'Direction') }}</label>
              <select v-model="queryDirection" class="form-select form-select-sm">
                <option value="tf">{{ text('TF → targets', 'TF → targets') }}</option>
                <option value="target">{{ text('target → regulators', 'target → regulators') }}</option>
              </select>
            </div>
            <div class="col-12 col-md-4">
              <label class="form-label small mb-1">{{ text('TF 名称 / gene ID / 注释', 'TF name / gene ID / annotation') }}</label>
              <input v-model="queryText" class="form-control form-control-sm" :placeholder="queryDirection === 'tf' ? 'MYB5 / MA…' : 'GENE1 / gene ID'" @keyup.enter="runQuery" />
            </div>
            <div class="col-6 col-md-2">
              <label class="form-label small mb-1">Motif ID</label>
              <input v-model="queryMotif" class="form-control form-control-sm" placeholder="MA0001.1" @keyup.enter="runQuery" />
            </div>
            <div class="col-6 col-md-1">
              <label class="form-label small mb-1">max p</label>
              <input v-model="queryMaxP" class="form-control form-control-sm" placeholder="any" @keyup.enter="runQuery" />
            </div>
            <div class="col-12 col-md-2">
              <button class="btn btn-primary btn-sm w-100" :disabled="queryBusy" @click="runQuery">
                <i class="bi bi-search me-1"></i>{{ queryBusy ? text('查询中…', 'Searching…') : text('查询', 'Search') }}
              </button>
            </div>
          </div>
          <div class="d-flex flex-wrap gap-1 mb-2">
            <span class="small text-muted me-1">{{ text('快速查询', 'Quick query') }}:</span>
            <button v-for="item in summary.top_tfs.slice(0, 8)" :key="item.id" class="btn btn-sm btn-outline-secondary py-0" @click="queryByTf(item.name || item.id)">{{ item.name || item.id }}</button>
          </div>
          <div v-if="queryError" class="alert alert-danger small py-2">{{ queryError }}</div>
          <div class="d-flex align-items-center gap-2 mb-1">
            <span class="small text-muted">{{ queryLabel }} · {{ text('显示', 'showing') }} {{ queryRows.length }} / {{ queryTotal.toLocaleString() }}</span>
            <select v-model.number="queryLimit" class="form-select form-select-sm ms-auto" style="width: 100px" @change="runQuery">
              <option :value="50">50</option>
              <option :value="200">200</option>
              <option :value="500">500</option>
            </select>
          </div>
          <SimpleTable v-if="queryRows.length" :columns="queryColumns" :rows="queryRows as any" scroll-height="440px" @row-click="onQueryRowClick" />
          <p v-else class="text-muted small mb-0">{{ text('没有匹配结果。', 'No matching rows.') }}</p>
          <p class="small text-muted mt-2 mb-0">{{ text('点击任意结果行，查看该 target 的 promoter 和结合位点。', 'Click any row to view that target’s promoter and binding sites.') }}</p>
        </div>
      </div>
    </template>

    <div v-if="promoterBusy" class="text-muted small mb-2"><span class="spinner-border spinner-border-sm me-2"></span>{{ text('加载 promoter…', 'Loading promoter…') }}</div>
    <div v-if="promoterError" class="alert alert-warning small">{{ promoterError }}</div>

    <div v-if="promoter" ref="promoterSection" class="card mb-3">
      <div class="card-body">
        <div class="d-flex align-items-start flex-wrap gap-2 mb-2">
          <div>
            <h6 class="mb-0">{{ promoter.target_symbol }} <span class="text-muted fw-normal small">{{ promoter.target_id }}</span></h6>
            <div class="small text-muted">
              {{ promoter.chrom || '—' }}:{{ promoter.start || '—' }}-{{ promoter.end || '—' }} ({{ promoter.strand || '?' }})
              · {{ promoter.length }} bp · {{ text('注释', 'annotation') }}: {{ promoter.annotation_source }}
            </div>
            <div v-if="promoter.target_description" class="small">{{ promoter.target_description }}</div>
          </div>
          <div class="ms-auto d-flex align-items-center gap-2">
            <label class="small text-muted">{{ text('显示', 'Show') }}</label>
            <select v-model.number="siteLimit" class="form-select form-select-sm" style="width: 90px">
              <option v-for="o in siteOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
            </select>
            <select v-model="siteTf" class="form-select form-select-sm" style="width: 150px">
              <option value="">{{ text('全部 TF', 'All TFs') }}</option>
              <option v-for="name in tfOptions" :key="name" :value="name">{{ name }}</option>
            </select>
          </div>
        </div>
        <p class="small text-muted mb-2">
          {{ text('启动子按基因转录方向 5′→3′ 展示；上方为 + 链位点，下方为 − 链位点。显示', 'Promoter is shown 5′→3′ in the gene direction; sites above are + strand, below are − strand. Showing') }}
          {{ promoterHits.length }} / {{ totalSites }} {{ text('个位点', 'sites') }}
          <span v-if="siteLimit > 0 && totalSites > siteLimit">（{{ text('按 p 值取前 N 个', 'top N by p-value') }}）</span>。
          {{ text('悬停查看 TF，点击查看序列。', 'Hover for TF, click for sequence.') }}
        </p>
        <div ref="trackWrap" class="rt-track-wrap" @mouseleave="hover = null">
          <svg :viewBox="`0 0 1000 ${track.height}`" class="rt-track" role="img" :aria-label="text('promoter 结合位点图', 'promoter binding-site track')">
            <line :x1="track.left" :x2="track.left + track.width" :y1="track.baseline" :y2="track.baseline" class="rt-axis" />
            <g v-for="tick in rulerTicks" :key="tick.x">
              <line :x1="tick.x" :x2="tick.x" :y1="track.baseline - 3" :y2="track.baseline + 3" class="rt-tick" />
              <text :x="tick.x" :y="track.height - 6" text-anchor="middle" class="rt-ruler">{{ tick.label }}</text>
            </g>
            <line :x1="tssX" :x2="tssX" :y1="track.baseline - 12" :y2="track.baseline + 12" class="rt-tss" />
            <text :x="tssX + 4" :y="track.baseline - 14" class="rt-tss-label">TSS</text>
            <g v-for="(site, i) in track.sites" :key="`${site.hit.motif_id}-${site.hit.start}-${site.hit.strand}-${i}`">
              <rect
                :x="site.x" :y="site.y" :width="site.width" height="10" rx="2"
                :class="['rt-site', site.plus ? 'rt-plus' : 'rt-minus', { 'rt-selected': selectedHit === site.hit }]"
                tabindex="0"
                @mouseenter="showHover($event, site.hit)"
                @focus="showHover($event, site.hit)"
                @click="selectedHit = site.hit"
                @keydown.enter="selectedHit = site.hit"
              />
            </g>
            <text x="40" :y="track.baseline - 10" class="rt-strand-label">+</text>
            <text x="40" :y="track.baseline + 18" class="rt-strand-label">−</text>
          </svg>
          <div v-if="hover" class="rt-tooltip" :style="{ left: hover.x + 'px', top: hover.y + 'px' }">
            <div class="fw-semibold">{{ hoverTfs(hover.hit) || '—' }}</div>
            <div class="small">{{ hover.hit.motif_id }} {{ hover.hit.motif_name }}</div>
            <div class="small">{{ text('位置', 'position') }} {{ hover.hit.start }}-{{ hover.hit.end }} ({{ hover.hit.strand }})</div>
            <div class="small">p = {{ formatP(hover.hit.p_value) }} · score = {{ hover.hit.score?.toFixed(2) ?? '—' }}</div>
          </div>
        </div>

        <div v-if="selectedHit" class="rt-detail mt-3">
          <div class="d-flex align-items-start flex-wrap gap-2">
            <div>
              <div class="fw-semibold">{{ text('位点详情', 'Site details') }}</div>
              <div class="small text-muted">{{ selectedHit.motif_id }} {{ selectedHit.motif_name }} · {{ selectedHit.strand }} · {{ selectedHit.start }}-{{ selectedHit.end }}</div>
            </div>
            <button class="btn btn-sm btn-outline-secondary ms-auto" @click="copySequence(selectedHit.matched_site)"><i class="bi bi-clipboard me-1"></i>{{ text('复制序列', 'Copy sequence') }}</button>
          </div>
          <div class="row g-2 mt-1">
            <div class="col-12 col-md-6">
              <div class="small text-muted">{{ text('TF', 'TFs') }}</div>
              <div v-for="tf in selectedHit.tfs" :key="tf.id" class="small">
                <span class="fw-semibold">{{ tf.symbol || tf.id }}</span>
                <span v-if="tf.family" class="text-muted"> · {{ tf.family }}</span>
                <span class="text-muted"> · {{ tf.id }}</span>
              </div>
            </div>
            <div class="col-12 col-md-6">
              <div class="small text-muted">{{ text('Motif consensus', 'Motif consensus') }}</div>
              <div class="font-monospace">{{ selectedHit.consensus || '—' }}</div>
              <div class="small text-muted mt-1">{{ text('基因组坐标', 'Genomic coordinates') }}: {{ selectedHit.genomic_start || '—' }}-{{ selectedHit.genomic_end || '—' }}</div>
            </div>
          </div>
          <div class="row g-3 mt-1">
            <div class="col-12 col-md-6">
              <div class="small text-muted">{{ text('Promoter 上的序列（5′→3′）', 'Site sequence on promoter (5′→3′)') }}</div>
              <div class="rt-seq font-monospace">
                <span v-for="(b, i) in baseSpans(selectedHit.site)" :key="i" :class="b.cls">{{ b.b }}</span>
              </div>
            </div>
            <div class="col-12 col-md-6">
              <div class="small text-muted">{{ text('与 motif 匹配的序列', 'Motif-matching sequence') }}</div>
              <div class="rt-seq font-monospace">
                <span v-for="(b, i) in baseSpans(selectedHit.matched_site)" :key="i" :class="b.cls">{{ b.b }}</span>
              </div>
            </div>
          </div>
          <div class="small text-muted mt-2">p = {{ formatP(selectedHit.p_value) }} · score = {{ selectedHit.score?.toFixed(2) ?? '—' }}</div>
        </div>
        <p v-else class="small text-muted mt-2 mb-0">{{ text('点击一个位点查看 TF 和序列。', 'Click a site to see TF and sequence.') }}</p>
      </div>
    </div>

    <div v-if="diagnostics" class="card mb-3">
      <div class="card-body">
        <div class="d-flex align-items-center gap-2 mb-2">
          <h6 class="mb-0">{{ text('输入检查', 'Input diagnostics') }}</h6>
          <span class="badge text-bg-danger">{{ diagnostics.summary.errors }}</span>
          <span class="badge text-bg-warning">{{ diagnostics.summary.warnings }}</span>
          <span class="badge text-bg-info">{{ diagnostics.summary.infos }}</span>
          <div class="ms-auto d-flex gap-2">
            <button class="btn btn-sm btn-outline-secondary" @click="copyDiagnostics"><i class="bi bi-clipboard me-1"></i>JSON</button>
            <a class="btn btn-sm btn-outline-secondary" :href="regutraceExportUrl(job!.id, 'report')" download><i class="bi bi-download me-1"></i>{{ text('报告', 'Report') }}</a>
          </div>
        </div>
        <div v-if="!diagnostics.diagnostics.length" class="small text-muted">{{ text('没有诊断信息。', 'No diagnostics.') }}</div>
        <div v-for="d in diagnostics.diagnostics.slice(0, 40)" :key="d.code + d.message" class="rt-diag">
          <span class="badge" :class="d.severity === 'error' ? 'text-bg-danger' : d.severity === 'warning' ? 'text-bg-warning' : 'text-bg-info'">{{ d.severity }}</span>
          <span class="fw-semibold small ms-2">{{ d.code }}</span>
          <span class="small ms-2">{{ d.message }}</span>
          <div v-if="d.suggestion" class="small text-muted ms-4">{{ d.suggestion }}</div>
        </div>
        <p v-if="diagnostics.diagnostics.length > 40" class="small text-muted mb-0">{{ text('仅显示前 40 条；完整 JSON 可下载。', 'Showing the first 40; download the JSON for all.') }}</p>
      </div>
    </div>

    <div v-if="progressOpen" class="rt-progress-overlay">
      <div class="card shadow-lg rt-progress-modal">
        <div class="card-body p-3">
          <div class="d-flex align-items-center gap-2 mb-2">
            <div class="spinner-border spinner-border-sm text-primary"></div>
            <span class="fw-semibold">{{ progressTitle }}</span>
            <span class="badge ms-auto" :class="uploadBusy ? 'text-bg-info' : statusBadge">{{ uploadBusy ? text('上传中', 'Uploading') : statusLabel }}</span>
          </div>
          <div class="progress" style="height: 8px">
            <div class="progress-bar progress-bar-striped progress-bar-animated" :style="{ width: overlayPct + '%' }"></div>
          </div>
          <div class="text-muted small mt-2 text-truncate" :title="overlayMessage">{{ stageLabel }} · {{ overlayMessage }}</div>
          <div class="d-flex align-items-center mt-1">
            <span class="text-muted small">{{ elapsedText }}</span>
            <button v-if="running" class="btn btn-sm btn-outline-danger ms-auto" @click="cancel">
              <i class="bi bi-stop-circle me-1"></i>{{ text('取消', 'Cancel') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </SidebarDrawer>
</template>

<style scoped>
.stat-tile { background: #fff; border: 1px solid #e2e9f0; border-radius: 10px; padding: 12px 14px; }
.stat-value { font-size: 22px; font-weight: 700; color: #0f766e; line-height: 1.1; }
.stat-label { font-size: 12px; color: #64748b; margin-top: 4px; }
.rt-empty { border: 1px dashed #cbd5e1; border-radius: 12px; padding: 32px; text-align: center; color: #64748b; background: #fff; }
.rt-empty > i { font-size: 32px; color: #0f766e; }
.rt-progress-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; z-index: 2000; }
.rt-progress-modal { width: 380px; max-width: 90vw; }
.rt-motif-help summary { cursor: pointer; color: #0f766e; }
.rt-motif-help a { color: #0f766e; }
.rt-track-wrap { position: relative; overflow-x: auto; border: 1px solid #e2e9f0; border-radius: 10px; background: #fbfdff; }
.rt-track { width: 100%; min-width: 620px; display: block; }
.rt-axis { stroke: #94a3b8; stroke-width: 1.2; }
.rt-tick { stroke: #94a3b8; stroke-width: 1; }
.rt-ruler { fill: #64748b; font-size: 10px; }
.rt-tss { stroke: #dc2626; stroke-width: 1.5; stroke-dasharray: 4 3; }
.rt-tss-label { fill: #dc2626; font-size: 10px; font-weight: 600; }
.rt-strand-label { fill: #94a3b8; font-size: 10px; }
.rt-site { cursor: pointer; opacity: 0.85; transition: opacity 0.12s ease; }
.rt-site:hover, .rt-site:focus { opacity: 1; stroke: #0f172a; stroke-width: 1; outline: none; }
.rt-plus { fill: #0f766e; }
.rt-minus { fill: #d97706; }
.rt-selected { fill: #dc2626; opacity: 1; stroke: #7f1d1d; stroke-width: 1; }
.rt-tooltip { position: absolute; z-index: 20; pointer-events: none; background: rgba(15, 23, 42, 0.94); color: #fff; border-radius: 8px; padding: 7px 9px; font-size: 12px; max-width: 280px; box-shadow: 0 6px 20px rgba(15, 23, 42, 0.24); }
.rt-detail { border-top: 1px solid #e2e9f0; padding-top: 12px; }
.rt-seq { font-size: 18px; letter-spacing: 2px; padding: 6px 8px; background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; overflow-x: auto; white-space: nowrap; }
.base-a { color: #15803d; }
.base-c { color: #1d4ed8; }
.base-g { color: #b45309; }
.base-t { color: #b91c1c; }
.base-n { color: #64748b; }
.rt-diag { padding: 6px 0; border-bottom: 1px solid #f1f5f9; }
.rt-diag:last-child { border-bottom: none; }
</style>
