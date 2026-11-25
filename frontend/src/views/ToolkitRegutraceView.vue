<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { EChartsOption } from 'echarts'
import {
  cancelRegutraceJob,
  createRegutraceJob,
  getRegutraceInfo,
  getRegutraceJob,
  getRegutracePromoter,
  getRegutraceSiteEvidence,
  getRegutraceTFView,
  queryRegutrace,
  regutraceExportUrl,
  regutracePriorUrl,
  regutraceBackgroundUrl,
  startRegutraceJob,
  type RegutraceCountItem,
  type RegutraceDiagnostic,
  type RegutraceExportFormat,
  type RegutraceInfo,
  type RegutraceJob,
  type RegutraceParams,
  type RegutracePromoterHit,
  type RegutracePromoterView,
  type RegutraceQueryRow,
  type RegutraceTFInfo,
  type RegutraceTFView,
} from '../api/client'
import { locale } from '../utils/i18n'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import SimpleTable from '../components/SimpleTable.vue'
import EChart from '../components/EChart.vue'
import RegutraceTFEvidence from '../components/RegutraceTFEvidence.vue'
import RegutraceTFProfile from '../components/RegutraceTFProfile.vue'
import RegutraceMotifExplorer from '../components/RegutraceMotifExplorer.vue'
import RegutraceEnrichment from '../components/RegutraceEnrichment.vue'
import RegutraceProvenance from '../components/RegutraceProvenance.vue'
import RegutraceTargetRegulators from '../components/RegutraceTargetRegulators.vue'

const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const route = useRoute()
const router = useRouter()

const genomeFile = ref<File | null>(null)
const gffFile = ref<File | null>(null)
const motifsFile = ref<File | null>(null)
const tfMapFile = ref<File | null>(null)
const tfGenesFile = ref<File | null>(null)
const proteinsFile = ref<File | null>(null)
const sequenceType = ref<"pep" | "cds">("pep")
const hmmFile = ref<File | null>(null)
const familyMapFile = ref<File | null>(null)
const traceFile = ref<File | null>(null)
const annotationFile = ref<File | null>(null)
const referencePack = ref('')
const customPackFile = ref<File | null>(null)
type ScanMode = 'pack' | 'manual' | 'custom'
const scanMode = ref<ScanMode>('pack')
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
const backgroundMode = ref<'uniform' | 'empirical'>('uniform')
const threads = ref(0)
const includeFallback = ref(false)
const reuseExactPWM = ref(false)
const phase3Artifacts = ref(false)
const useScanCache = ref(false)
const sitesOnly = ref(false)
const mappingMode = ref<'strict' | 'balanced' | 'exploratory'>('strict')
const priorPolicy = ref<'strict' | 'strict_calibrated' | 'extended' | 'selected'>('strict')
const priorTfs = ref('')
const priorIncludeExploratory = ref(false)
const priorSelectedIDs = computed(() => priorTfs.value.split(/[\s,;]+/).map((value) => value.trim()).filter(Boolean))

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
const siteEvidence = ref<RegutraceQueryRow[]>([])
const siteEvidenceBusy = ref(false)
const siteEvidenceError = ref('')
let siteEvidenceSeq = 0
const siteLimit = ref(30)
const siteTf = ref('')
const hover = ref<{ x: number; y: number; hit: RegutracePromoterHit } | null>(null)
const trackWrap = ref<HTMLElement>()
const promoterSection = ref<HTMLElement>()
const tfSection = ref<HTMLElement>()
const selectedTfId = ref('')
const tfView = ref<RegutraceTFView | null>(null)
const tfBusy = ref(false)
const tfError = ref('')
let tfSeq = 0
let promoterSeq = 0

const running = computed(() => job.value?.status === 'validating' || job.value?.status === 'running')
const busy = computed(() => uploadBusy.value || actionBusy.value || running.value)
const summary = computed(() => job.value?.summary ?? null)
const diagnostics = computed(() => job.value?.diagnostics ?? null)
const priorReady = computed(() => {
  const inputs = job.value?.inputs
  if (!inputs) return false
  return !!inputs.reference_pack || (!!inputs.tf_genes && !!inputs.tf_map)
})
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
const packMode = computed(() => scanMode.value === 'pack')
const customMode = computed(() => scanMode.value === 'custom')
const pipelineMode = computed(() => scanMode.value !== 'manual')
const plantReferenceKey = "plant-jaspar-2024-rbh"
const selectedPack = computed(() => info.value?.packs?.find((pack) => pack.key === referencePack.value) ?? null)
const toolWarning = computed(() => (info.value?.tools ?? []).filter((tool) => tool.status !== 'ok'))
const motifReady = computed(() => pipelineMode.value || !!selectedLibrary.value || !!motifsFile.value)
const proteinsReady = computed(() => !pipelineMode.value || !!proteinsFile.value)
const packReady = computed(() => {
  if (packMode.value) return !!referencePack.value
  if (customMode.value) return !!customPackFile.value
  return true
})
const filesReady = computed(() => (referencePack.value !== plantReferenceKey || !!annotationFile.value) && !!genomeFile.value && !!gffFile.value && motifReady.value && proteinsReady.value && packReady.value)
const paramsDirty = computed(() => {
  if (!job.value) return false
  const p = job.value.params
  return p.promoter_len !== promoterLen.value || p.downstream !== downstream.value ||
    Math.abs(p.pvalue - pvalue.value) > 1e-15 || p.threads !== threads.value ||
    (p.background_mode ?? 'uniform') !== backgroundMode.value ||
    p.background.some((v, i) => Math.abs(v - background.value[i]) > 1e-12) ||
    !!p.include_single_domain_fallback !== includeFallback.value ||
    !!p.reuse_exact_pwm !== reuseExactPWM.value ||
    !!p.phase3_artifacts !== phase3Artifacts.value ||
    !!p.use_scan_cache !== useScanCache.value ||
    !!p.sites_only !== sitesOnly.value ||
    (p.mapping_mode ?? 'strict') !== mappingMode.value
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

const familyFilter = ref('')
const catalogueSearch = ref('')
const mappingFilter = ref('')
const testabilityFilter = ref('')
const cataloguePage = ref(1)
const cataloguePageSize = 25
const catalogueFamilies = computed(() => {
  const counts = new Map<string, { identified: number; mapped: number; other: number }>()
  for (const item of summary.value?.tf_catalogue ?? []) {
    const key = item.family || text('未分家族', 'Unassigned')
    const count = counts.get(key) ?? { identified: 0, mapped: 0, other: 0 }
    if (item.status === 'identified') {
      count.identified++
      if (item.motifs.length) count.mapped++
    } else count.other++
    counts.set(key, count)
  }
  return [...counts.entries()].sort((a, b) => b[1].identified - a[1].identified || a[0].localeCompare(b[0]))
})
const maxFamilyCount = computed(() => Math.max(1, ...catalogueFamilies.value.map(([, n]) => n.identified + n.other)))
const filteredCatalogue = computed(() => {
  const key = catalogueSearch.value.trim().toLowerCase()
  return (summary.value?.tf_catalogue ?? []).filter(item =>
    (!familyFilter.value || (item.family || text('未分家族', 'Unassigned')) === familyFilter.value)
    && (!mappingFilter.value || item.mapping_status === mappingFilter.value)
    && (!testabilityFilter.value
      || (testabilityFilter.value === 'testable' && (item.testable_motifs ?? 0) > 0)
      || (testabilityFilter.value === 'unattainable' && (item.unattainable_motifs ?? 0) > 0)
      || (testabilityFilter.value === 'none' && !item.motifs.length))
    && (!key || [item.id, item.name, item.description, ...item.motifs].some(v => v?.toLowerCase().includes(key))))
})
const cataloguePageCount = computed(() => Math.max(1, Math.ceil(filteredCatalogue.value.length / cataloguePageSize)))
const catalogueRows = computed(() => filteredCatalogue.value.slice(
  (cataloguePage.value - 1) * cataloguePageSize, cataloguePage.value * cataloguePageSize))
watch([familyFilter, catalogueSearch, mappingFilter, testabilityFilter, summary], () => { cataloguePage.value = 1 })
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
  for (const h of promoter.value.hits) for (const tf of h.tfs) names.add(tf.name || tf.symbol || tf.id)
  return [...names].sort()
})
const promoterHits = computed(() => {
  if (!promoter.value) return []
  const filter = siteTf.value.trim().toLowerCase()
  const list = promoter.value.hits.filter((h) => {
    if (!filter) return true
    return h.tfs.some((tf) => (tf.name || tf.symbol || tf.id).toLowerCase().includes(filter))
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
  return promoter.value.hits.filter((h) => !filter || h.tfs.some((tf) => (tf.name || tf.symbol || tf.id).toLowerCase().includes(filter))).length
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
  {
    key: 'tf_name',
    label: text('TF 基因名', 'TF gene'),
    fmt: (_v: any, row: Record<string, any>) => row.tf_name || row.tf_symbol || row.tf_id,
  },
  { key: 'tf_id', label: 'TF ID' },
  { key: 'tf_family', label: text('家族', 'Family') },
  { key: 'target_symbol', label: text('靶基因', 'Target') },
  { key: 'target_id', label: 'Target ID' },
  { key: 'tf_description', label: text('TF 注释', 'TF annotation') },
  { key: 'target_description', label: text('靶基因注释', 'Target annotation') },
  { key: 'motif_id', label: 'Motif' },
  { key: 'mapping_status', label: text('映射状态', 'Mapping'), fmt: (v: any) => mappingLabel(v) },
  { key: 'testability_status', label: text('可测试性', 'Testability'), fmt: (v: any) => v || '—' },
  { key: 'shared_motif_tfs', label: text('共享 PWM', 'Shared PWM'), fmt: (v: any) => v > 1 ? `×${v}` : '—' },
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

function mappingLabel(value?: string): string {
  switch (value) {
    case 'strict_rbh': return text('Strict RBH', 'Strict RBH')
    case 'dbd_calibrated': return text('Calibrated DBD', 'Calibrated DBD')
    case 'dbd_ambiguous': return text('Ambiguous DBD', 'Ambiguous DBD')
    case 'dbd_exploratory': return text('Exploratory DBD', 'Exploratory DBD')
    case 'no_supported_motif': return text('无支持 motif', 'No supported motif')
    case 'not_computed': return text('未计算', 'Not computed')
    default: return value || '—'
  }
}
function mappingClass(value?: string): string {
  switch (value) {
    case 'strict_rbh': return 'text-bg-success'
    case 'dbd_calibrated': return 'text-bg-primary'
    case 'dbd_ambiguous': return 'text-bg-warning'
    case 'dbd_exploratory': return 'text-bg-info'
    case 'no_supported_motif': return 'text-bg-secondary'
    default: return 'text-bg-light border'
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

function onFile(target: 'genome' | 'gff' | 'motifs' | 'tfMap' | 'tfGenes' | 'proteins' | 'hmm' | 'familyMap' | 'trace' | 'customPack' | 'annotation', event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0] ?? null
  if (target === 'genome') genomeFile.value = file
  if (target === 'gff') gffFile.value = file
  if (target === 'motifs') motifsFile.value = file
  if (target === 'tfMap') tfMapFile.value = file
  if (target === 'tfGenes') tfGenesFile.value = file
  if (target === 'proteins') proteinsFile.value = file
  if (target === 'hmm') hmmFile.value = file
  if (target === 'familyMap') familyMapFile.value = file
  if (target === 'trace') traceFile.value = file
  if (target === 'customPack') customPackFile.value = file
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
  siteEvidenceSeq++
  siteEvidence.value = []
  siteEvidenceError.value = ''
  lastResultJob = ''
  syncJobQuery(null)
}

function onKingdomChange() {
  if (selectedLibrary.value) motifsFile.value = null
  resetJob()
}

function setScanMode(mode: ScanMode) {
  if (mode === 'pack' && !(info.value?.packs?.length)) return
  scanMode.value = mode
  if (mode !== 'pack') referencePack.value = ''
  resetJob()
}

function onPackChange() {
  const pack = selectedPack.value
  if (pack) {
    scanMode.value = 'pack'
    if (pack.promoter_len) promoterLen.value = pack.promoter_len
    if (pack.pvalue) pvalue.value = pack.pvalue
  }
  resetJob()
}

function currentParams(): RegutraceParams {
  const requestedThreads = Number(threads.value)
  return {
    promoter_len: promoterLen.value,
    downstream: downstream.value,
    pvalue: pvalue.value,
    background: [...background.value] as [number, number, number, number],
    background_mode: backgroundMode.value,
    threads: Number.isFinite(requestedThreads) ? Math.max(0, Math.trunc(requestedThreads)) : 0,
    include_single_domain_fallback: includeFallback.value,
    reuse_exact_pwm: reuseExactPWM.value,
    phase3_artifacts: phase3Artifacts.value,
    use_scan_cache: useScanCache.value,
    sites_only: sitesOnly.value,
    mapping_mode: mappingMode.value,
  }
}

async function ensureJob(): Promise<string> {
  if (jobId.value) return jobId.value
  if (!genomeFile.value || !gffFile.value) throw new Error(text('请先选择 genome FASTA 和 GFF3。', 'Choose a genome FASTA and GFF3 first.'))
  if (packMode.value && !referencePack.value) throw new Error(text('请先选择参考 pack。', 'Choose a reference pack first.'))
  if (customMode.value && !customPackFile.value) throw new Error(text('请上传自定义 pack ZIP。', 'Choose a custom pack ZIP first.'))
  if (referencePack.value === plantReferenceKey && !annotationFile.value) throw new Error(text('请上传注释表。', 'Upload an annotation table.'))
  if (pipelineMode.value && !proteinsFile.value) throw new Error(text('reference/custom pack 模式需要代表性蛋白 FASTA。', 'Reference/custom pack mode requires a representative protein FASTA.'))
  const lib = selectedLibrary.value
  if (!pipelineMode.value && !lib && !motifsFile.value) throw new Error(text('该物种族没有内置 JASPAR 库，请上传自定义 JASPAR PFM。', 'No bundled JASPAR library for this taxon; upload a custom JASPAR PFM.'))
  uploadBusy.value = true
  uploadProgress.value = 0
  try {
    const j = await createRegutraceJob({
      genome: genomeFile.value,
      gff: gffFile.value,
      motifs: pipelineMode.value ? null : (lib ? null : motifsFile.value),
      motifLibrary: pipelineMode.value ? undefined : lib?.key,
      tfMap: pipelineMode.value ? null : tfMapFile.value,
      tfGenes: pipelineMode.value ? null : tfGenesFile.value,
      proteins: !pipelineMode.value || sequenceType.value === "pep" ? proteinsFile.value : null,
      cds: pipelineMode.value && sequenceType.value === "cds" ? proteinsFile.value : null,
      hmm: pipelineMode.value ? null : hmmFile.value,
      familyMap: pipelineMode.value ? null : familyMapFile.value,
      trace: pipelineMode.value ? null : traceFile.value,
      referencePack: packMode.value ? (referencePack.value || undefined) : undefined,
      customPack: customMode.value ? customPackFile.value : null,
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
      siteEvidenceSeq++
      siteEvidence.value = []
      siteEvidenceError.value = ''
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

async function loadTFView(tfId: string, targetId = '') {
  if (!job.value || !tfId) return
  const seq = ++tfSeq
  selectedTfId.value = tfId
  tfBusy.value = true
  tfError.value = ''
  tfView.value = null
  try {
    const view = await getRegutraceTFView(job.value.id, tfId, targetId || undefined)
    if (seq === tfSeq) tfView.value = view
  } catch (e) {
    if (seq === tfSeq) tfError.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === tfSeq) tfBusy.value = false
  }
}

async function openPromoter(targetId: string, tfId = '') {
  if (!job.value || !targetId) return
  const seq = ++promoterSeq
  promoterBusy.value = true
  promoterError.value = ''
  selectedHit.value = null
  siteEvidenceSeq++
  siteEvidence.value = []
  siteEvidenceError.value = ''
  siteTf.value = ''
  const tfLoad = tfId ? loadTFView(tfId, targetId) : Promise.resolve()
  try {
    const data = await getRegutracePromoter(job.value.id, targetId)
    if (seq !== promoterSeq) return
    promoter.value = data
    const query: Record<string, string> = {}
    for (const [key, value] of Object.entries(route.query)) {
      if (typeof value === 'string') query[key] = value
    }
    query.job = job.value.id
    query.target = targetId
    if (tfId) query.tf = tfId
    else delete query.tf
    void router.replace({ query })
    nextTick(() => {
      const target = tfId ? tfSection.value : promoterSection.value
      target?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
  } catch (e) {
    if (seq === promoterSeq) promoterError.value = e instanceof Error ? e.message : String(e)
  } finally {
    await tfLoad
    if (seq === promoterSeq) promoterBusy.value = false
  }
}

function onQueryRowClick(row: Record<string, any>) {
  void openPromoter(String(row.target_id ?? ''), String(row.tf_id ?? ''))
}

function selectTF(tfId: string) {
  if (!tfId) return
  void loadTFView(tfId, promoter.value?.target_id ?? '')
  nextTick(() => tfSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
}

function focusMotif(motifId: string) {
  const hit = promoter.value?.hits.find((item) => item.motif_id === motifId &&
    (!selectedTfId.value || item.tfs.some((tf) => tf.id === selectedTfId.value))) ??
    promoter.value?.hits.find((item) => item.motif_id === motifId)
  if (hit) {
    selectedHit.value = hit
    void loadSiteEvidence(hit)
  }
  promoterSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
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
  return hit.tfs.map((t) => t.name || t.symbol || t.id).join(', ')
}

function donorLabel(tf: RegutraceTFInfo): string {
  const donors = tf.donors ?? []
  if (!donors.length) return '—'
  return donors.map((d) => {
    const head = d.tf_symbol || d.tf_id
    const family = d.tf_family ? ` (${d.tf_family})` : ''
    const id = d.tf_symbol && d.tf_symbol !== d.tf_id ? ` · ${d.tf_id}` : ''
    return `${head}${family}${id}`
  }).join('; ')
}

function selectHit(hit: RegutracePromoterHit) {
  selectedHit.value = hit
  void loadSiteEvidence(hit)
}

async function selectSiteFromQuery() {
  if (!promoter.value) return
  const motif = typeof route.query.motif === 'string' ? route.query.motif : ''
  const hitStart = Number(route.query.hit_start)
  const hitEnd = Number(route.query.hit_end)
  const strand = typeof route.query.strand === 'string' ? route.query.strand : ''
  if (!motif || !Number.isFinite(hitStart) || !Number.isFinite(hitEnd)) return
  const hit = promoter.value.hits.find((h) =>
    h.motif_id === motif && h.start === hitStart && h.end === hitEnd && (!strand || h.strand === strand),
  )
  if (!hit) return
  selectedHit.value = hit
  await loadSiteEvidence(hit)
}

async function loadSiteEvidence(hit: RegutracePromoterHit) {
  if (!job.value || !promoter.value) return
  const seq = ++siteEvidenceSeq
  siteEvidenceBusy.value = true
  siteEvidenceError.value = ''
  siteEvidence.value = []
  try {
    const p = promoter.value
    const res = await getRegutraceSiteEvidence(job.value.id, {
      target: p.target_id,
      motif: hit.motif_id,
      promoter_start: hit.promoter_start ?? p.start,
      promoter_end: hit.promoter_end ?? p.end,
      hit_start: hit.start,
      hit_end: hit.end,
      strand: hit.strand,
    })
    if (seq !== siteEvidenceSeq) return
    siteEvidence.value = res.rows
  } catch (e) {
    if (seq !== siteEvidenceSeq) return
    siteEvidenceError.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === siteEvidenceSeq) siteEvidenceBusy.value = false
  }
}

function evidenceDonor(tfId: string): string {
  const tf = selectedHit.value?.tfs.find((t) => t.id === tfId)
  return tf ? donorLabel(tf) : '—'
}

function rowPromoter(row: RegutraceQueryRow): string {
  return row.promoter_start !== undefined && row.promoter_end !== undefined
    ? `${row.promoter_start}-${row.promoter_end}`
    : '—'
}

function rowSite(row: RegutraceQueryRow): string {
  return row.hit_start !== undefined && row.hit_end !== undefined
    ? `${row.hit_start}-${row.hit_end}(${row.strand ?? '?'})`
    : '—'
}

function baseSpans(seq: string) {
  return [...seq].map((b) => ({ b, cls: `base-${b.toLowerCase()}` }))
}

async function copyDiagnostics() {
  if (!job.value?.diagnostics) return
  try { await navigator.clipboard.writeText(JSON.stringify(job.value.diagnostics, null, 2)) } catch { /* clipboard unavailable */ }
}

function askAIAboutDiagnostic(d: RegutraceDiagnostic) {
  const lines = [
    `ReguTrace input diagnostic: ${d.code} (${d.severity})`,
    d.message,
    d.file ? `${text('文件', 'File')}: ${d.file}${d.line ? `:${d.line}` : ''}` : '',
    d.context && Object.keys(d.context).length ? `${text('上下文', 'Context')}: ${JSON.stringify(d.context)}` : '',
    d.suggestion ? `${text('建议', 'Suggested action')}: ${d.suggestion}` : '',
    text(
      '请解释这个 ID 不匹配的原因，并给出可执行的修正步骤；如果需要映射表，请生成 transcript/protein ID → gene ID 的映射方案，不要手工乱改 ID。',
      'Explain why the IDs do not match and give an actionable fix. If a mapping table is needed, propose a transcript/protein ID → gene ID mapping; do not rename IDs by hand.',
    ),
  ].filter(Boolean)
  analyzeError(lines.join('\n'), {
    regutraceJobId: job.value?.id,
    kind: d.code === 'ANNOTATION_ID_MISMATCH' ? 'id_mismatch' : 'diagnostic',
  })
}

function askAIAboutFailure() {
  if (!job.value?.error) return
  const error = job.value.error
  const hasIDMismatch = !!job.value.diagnostics?.diagnostics.some((d) => d.code === 'ANNOTATION_ID_MISMATCH') ||
    /ANNOTATION_ID_MISMATCH|do not match|missing from gene map/i.test(error)
  analyzeError(
    text(
      `ReguTrace 运行失败：\n${error}\n\n请判断是否由注释文件/eggNOG/CDS 的 ID 与 GFF ID 不匹配导致，并给出修正步骤或映射表方案。`,
      `ReguTrace failed:\n${error}\n\nDetermine whether annotation/eggNOG/CDS IDs do not match GFF IDs, and give a fix or mapping-table plan.`,
    ),
    {
      regutraceJobId: job.value?.id,
      kind: hasIDMismatch ? 'id_mismatch' : 'job_failure',
    },
  )
}

function onIDMapApplied(event: Event) {
  const detail = (event as CustomEvent<{ jobId?: string }>).detail
  if (detail?.jobId && detail.jobId === jobId.value) {
    void start('run')
  }
}

function copySequence(seq: string) {
  void navigator.clipboard?.writeText(seq)
}

function applyJobParams(j: RegutraceJob) {
  promoterLen.value = j.params.promoter_len
  downstream.value = j.params.downstream
  pvalue.value = j.params.pvalue
  background.value = [...j.params.background] as [number, number, number, number]
  backgroundMode.value = j.params.background_mode === 'empirical' ? 'empirical' : 'uniform'
  threads.value = j.params.threads ?? 0
  includeFallback.value = !!j.params.include_single_domain_fallback
  reuseExactPWM.value = !!j.params.reuse_exact_pwm
  phase3Artifacts.value = !!j.params.phase3_artifacts
  useScanCache.value = !!j.params.use_scan_cache
  sitesOnly.value = !!j.params.sites_only
  mappingMode.value = j.params.mapping_mode ?? 'strict'
}

onMounted(async () => {
  clockTimer = window.setInterval(() => { now.value = Date.now() }, 1000)
  window.addEventListener('regutrace-idmap-applied', onIDMapApplied)
  try { info.value = await getRegutraceInfo() } catch { /* optional */ }
  if (info.value && !(info.value.packs?.length)) scanMode.value = 'manual'
  if (typeof route.query.job !== "string" && info.value?.packs?.some((pack) => pack.key === plantReferenceKey)) {
    referencePack.value = plantReferenceKey
    onPackChange()
  }
  const q = route.query.job
  if (typeof q === 'string' && q) {
    jobId.value = q
    try {
      const j = await getRegutraceJob(q)
      job.value = j
      const key = j.inputs.motif_library_key
      if (j.inputs.reference_pack === 'custom') {
        scanMode.value = 'custom'
        referencePack.value = ''
      } else if (j.inputs.reference_pack) {
        scanMode.value = 'pack'
        referencePack.value = j.inputs.reference_pack
      } else {
        scanMode.value = 'manual'
        referencePack.value = ''
        if (key && kingdomOptions.value.some((o) => o.key === key)) kingdom.value = key
        else if (key === 'custom') kingdom.value = 'custom'
      }
      sequenceType.value = j.inputs.cds ? "cds" : "pep"
      applyJobParams(j)
      if (j.status === 'succeeded') {
        lastResultJob = j.id
        await initialQuery(j)
        const target = route.query.target
        const tf = route.query.tf
        if (typeof target === 'string' && target) {
          await openPromoter(target, typeof tf === 'string' ? tf : '')
          await selectSiteFromQuery()
        } else if (typeof tf === 'string' && tf) {
          await loadTFView(tf)
        }
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
  window.removeEventListener('regutrace-idmap-applied', onIDMapApplied)
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
        <label class="form-label small mb-1">{{ text('分析模式', 'Analysis mode') }}</label>
        <div class="btn-group w-100 flex-wrap" role="group" aria-label="ReguTrace analysis mode">
          <button type="button" class="btn btn-sm" :class="packMode ? 'btn-primary' : 'btn-outline-primary'"
            :disabled="!info?.packs?.length" @click="setScanMode('pack')">
            <i class="bi bi-box-seam me-1"></i>{{ text('自动预测', 'Automatic prediction') }}
          </button>
          <button type="button" class="btn btn-sm" :class="customMode ? 'btn-success' : 'btn-outline-success'"
            @click="setScanMode('custom')">
            <i class="bi bi-boxes me-1"></i>{{ text('自定义 pack', 'Custom pack') }}
          </button>
          <button type="button" class="btn btn-sm" :class="scanMode === 'manual' ? 'btn-secondary' : 'btn-outline-secondary'"
            @click="setScanMode('manual')">
            <i class="bi bi-sliders me-1"></i>{{ text('高级：手动扫描', 'Advanced: manual scan') }}
          </button>
        </div>
        <p v-if="packMode && referencePack === plantReferenceKey" class="small text-muted mt-1 mb-0">
          {{ text('上传基因组、GFF、CDS 或蛋白 FASTA，以及注释表。自动鉴定 TF，在同家族找参考 TF，转移 JASPAR 代表矩阵并扫描启动子。', 'Upload a genome, GFF, CDS or protein FASTA, and annotation table. ReguTrace identifies local TFs, finds reference TFs in the same family, transfers representative JASPAR matrices, and scans promoters.') }}
        </p>
        <p v-else-if="packMode" class="small text-muted mt-1 mb-0">
          {{ text('先用参考 pack 从蛋白鉴定本地 TF gene，再做 motif 映射和启动子扫描；结果可用于 WGCNA prior。', 'Identify local TF genes from proteins with a reference pack, then map motifs and scan promoters; results can be used as a WGCNA prior.') }}
        </p>
        <p v-else-if="customMode" class="small text-muted mt-1 mb-0">
          {{ text('上传一个完整自定义 pack ZIP（HMM、规则、参考 map、参考 DBD、DBD models、motifs），再提供 genome/GFF/protein。适合没有内置 pack 的非模式物种；pack 在本地解压、校验 SHA-256 后运行。', 'Upload a complete custom pack ZIP (HMM, rules, reference map, reference DBDs, DBD models, motifs), then provide genome/GFF/protein. This is the route for non-model species without a bundled pack; the pack is extracted locally and SHA-256-verified before running.') }}
        </p>
        <div v-else class="alert alert-warning small py-2 mt-2 mb-0">
          {{ text('手动扫描默认只用 motif 名作为 TF；没有 TF gene ID、gene name 或 domain。只有 motif 命中，通常不能直接作为 WGCNA prior。', 'Manual scan uses motif names as TF IDs by default: no TF gene ID, gene name or domain. It reports motif hits and is usually not usable as a WGCNA prior.') }}
        </div>
      </div>

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

      <div v-if="packMode" class="mb-3">
        <label class="form-label small mb-1">{{ text('参考类群', 'Reference taxon') }} <span class="text-danger">*</span></label>
        <select v-model="referencePack" class="form-select form-select-sm" @change="onPackChange">
          <option value="" disabled>{{ text('选择参考 pack', 'Choose a reference pack') }}</option>
          <option v-for="pack in info?.packs ?? []" :key="pack.key" :value="pack.key">
            {{ pack.key === plantReferenceKey ? text("植物 · JASPAR", "Plants · JASPAR") : pack.key }}<template v-if="pack.species"> · {{ pack.species }}</template>
          </option>
        </select>
        <p v-if="selectedPack" class="small text-muted mt-1 mb-0">
          {{ text('pack 提供 HMM、规则、参考 map、DBD 和 JASPAR motifs；只需给 genome、GFF3 和代表性蛋白。运行时自动完成 TF 鉴定、motif 映射、扫描和 trace 表。安装包已内置 HMMER/BLASTP，无需配置；高级用户仍可用 WGCNA_HMMERSEARCH/WGCNA_BLASTP 覆盖。', 'The pack supplies HMM, rules, reference map, DBD models and JASPAR motifs; provide only genome, GFF3 and representative proteins. TF identification, motif mapping, scanning and trace tables run automatically. The installer bundles HMMER/BLASTP, so no setup is needed; advanced users can override with WGCNA_HMMERSEARCH/WGCNA_BLASTP.') }}
        </p>
      </div>
      <div v-if="customMode" class="mb-3">
        <label class="form-label small mb-1">{{ text('自定义 pack ZIP（必需）', 'Custom pack ZIP (required)') }} <span class="text-danger">*</span></label>
        <label class="btn btn-outline-success w-100 text-truncate btn-sm">
          <i class="bi bi-upload me-1"></i>{{ customPackFile ? customPackFile.name : text('选择 custom-pack.zip', 'Choose custom-pack.zip') }}
          <input type="file" accept=".zip,application/zip" class="d-none" @change="onFile('customPack', $event)" />
        </label>
        <p class="small text-muted mt-1 mb-0">
          {{ text('ZIP 根目录需有 manifest.json，包含 hmm、rules、reference_map、reference_domains、dbd_models、motifs 六个文件及 checksums。可直接压缩 pack 文件夹；我们会安全解压到本机 job 目录并校验哈希。', 'The ZIP root needs manifest.json with files and checksums for hmm, rules, reference_map, reference_domains, dbd_models and motifs. Zipping the pack folder itself is also accepted; extraction is sandboxed to the job directory and hashes are verified.') }}
        </p>
      </div>
      <p v-if="info && !(info.packs?.length) && packMode" class="small text-warning mt-1 mb-2">
        {{ text('服务器没有发现参考 pack；设置 WGCNA_REGUTRACE_PACKS 或使用“自定义 pack”上传 ZIP。', 'No server-side reference packs found; set WGCNA_REGUTRACE_PACKS or upload a ZIP through Custom pack.') }}
      </p>
      <p v-if="info?.tools?.length" class="small mt-1 mb-2" :class="toolWarning.length ? 'text-danger' : 'text-muted'">
        {{ text('内置工具', 'Bundled tools') }}:
        <span v-for="(tool, i) in info.tools" :key="tool.name">
          {{ tool.name }} {{ tool.version || '?' }} ({{ tool.status }})<span v-if="i < info.tools.length - 1"> · </span>
        </span>
      </p>

      <div v-if="!pipelineMode" class="mb-3">
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

      <details class="mb-3" :open="pipelineMode">
        <summary class="small fw-semibold">{{ pipelineMode ? text('CDS / 蛋白序列、注释及参数', 'CDS / proteins, annotation, and parameters') : text('可选输入与参数', 'Optional inputs and parameters') }}</summary>
        <div class="mt-2">
          <label v-if="pipelineMode" for="regutrace-sequence-type" class="form-label small mb-1">{{ text('TF 序列格式', 'TF sequence type') }}</label>
          <select v-if="pipelineMode" id="regutrace-sequence-type" v-model="sequenceType" class="form-select form-select-sm mb-2"
            @change="proteinsFile = null; resetJob()">
            <option value="pep">{{ text('蛋白序列（PEP FASTA）', 'Protein (PEP FASTA)') }}</option>
            <option value="cds">{{ text('编码序列（CDS FASTA）', 'Coding sequences (CDS FASTA)') }}</option>
          </select>
          <label v-if="!pipelineMode" class="form-label small mb-1">{{ pipelineMode ? text('代表性蛋白 FASTA（reference/custom pack 必需）', 'Representative protein FASTA (required for reference/custom pack)') : text('代表性蛋白 FASTA + HMM（可选，自动鉴定 TF）', 'Representative proteins + HMM (optional, identifies TFs)') }}</label>
          <div class="row g-1 mb-1">
            <div :class="pipelineMode ? 'col-12' : 'col-6'">
              <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-0">
                <i class="bi bi-upload me-1"></i>{{ proteinsFile ? proteinsFile.name : (pipelineMode && sequenceType === 'cds' ? 'CDS FASTA' : 'PEP FASTA') }}
                <input type="file" :key="pipelineMode ? sequenceType : 'manual'" accept=".fa,.fasta,.faa,.pep,.fna,.cds,.gz" class="d-none" @change="onFile('proteins', $event)" />
              </label>
            </div>
            <div v-if="!pipelineMode" class="col-6">
              <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-0">
                <i class="bi bi-upload me-1"></i>{{ hmmFile ? hmmFile.name : text('TF HMM 文件', 'TF HMM file') }}
                <input type="file" accept=".hmm,.h3m" class="d-none" @change="onFile('hmm', $event)" />
              </label>
            </div>
          </div>
          <template v-if="!pipelineMode">
          <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-2">
            <i class="bi bi-upload me-1"></i>{{ familyMapFile ? familyMapFile.name : text('HMM→family 映射（可选）', 'HMM→family map (optional)') }}
            <input type="file" accept=".tsv,.txt,.gz" class="d-none" @change="onFile('familyMap', $event)" />
          </label>
          <p class="small text-muted mb-2">{{ text('蛋白一个基因一条代表序列；HMMER3 HMM 文件由用户提供。运行时先鉴定 TF，再生成 tf_genes.tsv；需要同时提供 tf_map。', 'One representative protein per gene; HMMER3 HMM file is supplied by the user. TF identification runs before scanning and produces tf_genes.tsv; tf_map is also required.') }}</p>
          </template>

          <template v-if="!pipelineMode">
          <label class="form-label small mb-1">{{ text('TF 鉴定表 tf_genes.tsv（可选）', 'TF catalogue tf_genes.tsv (optional)') }}</label>
          <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-2">
            <i class="bi bi-upload me-1"></i>{{ tfGenesFile ? tfGenesFile.name : text('上传 tf_genes.tsv', 'Upload tf_genes.tsv') }}
            <input type="file" accept=".tsv,.txt,.gz" class="d-none" @change="onFile('tfGenes', $event)" />
          </label>
          <p class="small text-muted mb-2">{{ text('提供本物种已鉴定的 TF-GeneID 表；必须和 tf_map 一起使用，未鉴定或无 motif 的 TF 不产生预测边。', 'Provide the species TF–GeneID catalogue; use it with tf_map. Unidentified or unmapped TFs produce no prediction edges.') }}</p>
          </template>

          <template v-if="!pipelineMode">
          <label class="form-label small mb-1">TF–motif map ({{ text('可选', 'optional') }})</label>
          <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-2">
            <i class="bi bi-upload me-1"></i>{{ tfMapFile ? tfMapFile.name : text('上传 tf_map.tsv', 'Upload tf_map.tsv') }}
            <input type="file" accept=".tsv,.txt,.gz" class="d-none" @change="onFile('tfMap', $event)" />
          </label>
          <p class="small text-muted mb-2">{{ text('不传时用 motif header name 作为 TF ID；上传 tf_genes 后，tf_map 的 tf_id 必须来自已鉴定基因。', 'Without it the motif header name is used as the TF ID. With tf_genes, every tf_map tf_id must be an identified catalogue gene.') }}</p>
          </template>

          <template v-if="!pipelineMode">
          <label class="form-label small mb-1">{{ text('TF–motif 追溯表 tf_motif_trace.tsv（可选）', 'TF–motif trace table tf_motif_trace.tsv (optional)') }}</label>
          <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-2">
            <i class="bi bi-upload me-1"></i>{{ traceFile ? traceFile.name : text('上传 tf_motif_trace.tsv', 'Upload tf_motif_trace.tsv') }}
            <input type="file" accept=".tsv,.txt,.gz" class="d-none" @change="onFile('trace', $event)" />
          </label>
          <p class="small text-muted mb-2">{{ text('从与本次 tf_map/evidence 同一运行的 CLI pipeline/trace 输出上传；详情页只用 reference_donors 给本次 tf_map 中已有的 donor 补充 symbol/family，不会新增或替换 donor ID。不上传时显示 tf_map 的 donor ID。', 'Upload the CLI pipeline/trace output from the same run as this tf_map/evidence. The detail page uses reference_donors only to add symbol/family to donor IDs already present in tf_map; it never adds or replaces a donor ID. Without it, the detail page shows tf_map donor IDs only.') }}</p>
          </template>

          <label class="form-label small mb-1">{{ text('Annotation（可选）', 'Annotation (optional)') }}</label>
          <label class="btn btn-outline-secondary w-100 text-truncate btn-sm mb-2">
            <i class="bi bi-upload me-1"></i>{{ annotationFile ? annotationFile.name : text('上传注释表（TSV 或 eggNOG）', 'Upload annotation (TSV or eggNOG)') }}
            <input type="file" accept=".tsv,.txt,.xls,.annotations,.gz" class="d-none" @change="onFile('annotation', $event)" />
          </label>
          <p class="small text-muted mb-2">{{ text('自动识别 eggNOG 或 gene_id / symbol / description TSV；没有注释时使用 GFF 的 Name。', 'Auto-detects eggNOG or a gene_id / symbol / description TSV. Without it, the GFF Name is used.') }}</p>

          <label class="form-label small mb-1">{{ text('Promoter 上游长度 (bp)', 'Promoter upstream length (bp)') }}</label>
          <input v-model.number="promoterLen" type="number" min="1" step="50" class="form-control form-control-sm mb-2" />
          <label class="form-label small mb-1">{{ text('TSS 下游长度 (bp)', 'Downstream of TSS (bp)') }}</label>
          <input v-model.number="downstream" type="number" min="0" step="10" class="form-control form-control-sm mb-2" />
          <label class="form-label small mb-1">{{ text('PWM p 值阈值', 'PWM p-value cutoff') }}</label>
          <input v-model.number="pvalue" type="number" min="0" max="1" step="0.00001" class="form-control form-control-sm mb-2" />
          <label class="form-label small mb-1">{{ text('背景模型', 'Background model') }}</label>
          <select v-model="backgroundMode" class="form-select form-select-sm mb-2">
            <option value="uniform">{{ text('Uniform（默认，0.25 A/C/G/T）', 'Uniform (default, 0.25 A/C/G/T)') }}</option>
            <option value="empirical">{{ text('Empirical promoter（推荐用于调查）', 'Empirical promoter (recommended for investigation)') }}</option>
          </select>
          <p v-if="backgroundMode === 'empirical'" class="small text-muted mb-2">
            {{ text('从本次启动子估计 A/C/G/T 频率；运行后显示实际频率。它是零阶模型，不建模二核苷酸；未声称能提高生物学 target 预测。', 'Estimates A/C/G/T from this run’s promoters; actual frequencies are shown after the run. It is a zero-order model, not a claim of better biological target prediction.') }}
          </p>
          <template v-else>
            <label class="form-label small mb-1">{{ text('背景碱基概率 A,C,G,T', 'Background probabilities A,C,G,T') }}</label>
            <div class="row g-1">
              <div v-for="(_, i) in background" :key="i" class="col-3">
                <input v-model.number="background[i]" type="number" min="0.000001" step="0.01" class="form-control form-control-sm" />
              </div>
            </div>
          </template>
          <label class="form-label small mb-1 mt-2">{{ text('结果准入模式', 'Result admission mode') }}</label>
          <select v-model="mappingMode" class="form-select form-select-sm mb-1">
            <option value="strict">{{ text('Strict：仅 full-length RBH', 'Strict: full-length RBH only') }}</option>
            <option value="balanced">{{ text('Balanced：strict + calibrated DBD', 'Balanced: strict + calibrated DBD') }}</option>
            <option value="exploratory">{{ text('Exploratory：含 DBD candidate motifs', 'Exploratory: include DBD candidate motifs') }}</option>
          </select>
          <p class="small text-muted mb-2">{{ text('模式只筛选展示/查询/导出准入，不改变 strict 语义，也不会把 exploratory 标为 calibrated。', 'The mode filters display/query/export admission; it does not change strict semantics and never labels exploratory as calibrated.') }}</p>
          <div class="form-check mt-2">
            <input id="rt-phase3-artifacts" v-model="phase3Artifacts" class="form-check-input" type="checkbox" />
            <label class="form-check-label small" for="rt-phase3-artifacts">{{ text('写出 site / testability / promoter summary / scan manifest', 'Write site / testability / promoter-summary / scan-manifest artifacts') }}</label>
            <div class="small text-muted">{{ text('Phase 4 概览、可测试性和 motif 诊断需要这些附加文件；不改变 evidence 语义。', 'Phase 4 overview, testability and motif diagnostics need these additive files; evidence semantics are unchanged.') }}</div>
          </div>
          <div class="form-check">
            <input id="rt-reuse-pwm" v-model="reuseExactPWM" class="form-check-input" type="checkbox" />
            <label class="form-check-label small" for="rt-reuse-pwm">{{ text('复用完全相同 PWM（exact reuse）', 'Reuse bit-identical PWMs (exact reuse)') }}</label>
          </div>
          <div class="form-check mb-2">
            <input id="rt-scan-cache" v-model="useScanCache" class="form-check-input" type="checkbox" />
            <label class="form-check-label small" for="rt-scan-cache">{{ text('使用 job 内 scan cache', 'Use a scan cache inside this job') }}</label>
          </div>
          <div v-if="!pipelineMode" class="form-check mb-2">
            <input id="rt-sites-only" v-model="sitesOnly" class="form-check-input" type="checkbox" />
            <label class="form-check-label small" for="rt-sites-only">{{ text('Exploratory site-only 扫描（不物化 TF–target–site 行）', 'Exploratory site-only scan (no materialized TF–target–site rows)') }}</label>
            <div class="small text-muted">{{ text('需要 Phase 3 artifacts 和 candidate tf_map；目标查询动态连接 site 与 mapping。', 'Requires Phase 3 artifacts and a candidate tf_map; target queries join sites to mappings on demand.') }}</div>
          </div>
          <label class="form-label small mb-1 mt-2">{{ text('扫描线程数', 'Scan threads') }}</label>
          <input v-model.number="threads" type="number" min="0" max="256" step="1" class="form-control form-control-sm" />
          <p class="small text-muted mt-1 mb-0">{{ text('0 = 自动（CPU 核数 − 1）；1 = 单线程。', '0 = auto (CPU cores − 1); 1 = serial.') }}</p>
          <div v-if="referencePack !== plantReferenceKey" class="form-check mt-2">
            <input id="rt-include-fallback" v-model="includeFallback" class="form-check-input" type="checkbox" />
            <label class="form-check-label small" for="rt-include-fallback">{{ text('纳入单域回退映射（single_domain_fallback）', 'Include single-domain fallback mappings') }}</label>
            <div class="small text-muted">{{ text('默认只扫描完整架构；纳入后会增加候选，也会增加歧义。', 'Default scans full architectures only; including fallback adds candidates and ambiguity.') }}</div>
          </div>
        </div>
      </details>

      <div class="d-grid gap-2">
        <button v-if="!pipelineMode" class="btn btn-outline-primary btn-sm" :disabled="!filesReady || busy" @click="start('validate')">
          <i class="bi bi-check2-circle me-1"></i>{{ text('检查输入', 'Validate inputs') }}
        </button>
        <button class="btn btn-primary btn-sm" :disabled="!filesReady || busy" @click="start('run')">
          <i class="bi bi-play-fill me-1"></i>{{ pipelineMode ? text('运行 ReguTrace', 'Run ReguTrace') : text('运行 ReguTrace', 'Run ReguTrace') }}
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
      <button v-if="job.error" class="btn btn-sm btn-outline-light mt-2" @click="askAIAboutFailure">
        <i class="bi bi-stars me-1"></i>{{ text('让 AI 助手分析 ID / 输入问题', 'Ask AI assistant about IDs / inputs') }}
      </button>
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
            <span v-if="summary.site_only" class="badge text-bg-warning ms-2">{{ text('Site-only 探索：导出 background，不物化 prior', 'Site-only exploration: export background; prior not materialized') }}</span>
            <span v-else-if="priorReady" class="badge text-bg-success ms-2">{{ text('可用于 WGCNA prior', 'Prior-ready') }}</span>
            <span v-else class="badge text-bg-warning ms-2">{{ text('仅 motif 候选，不可直接作 prior', 'Motif-only; not a prior') }}</span>
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
          <div v-if="priorReady && !summary.site_only" class="d-flex flex-wrap gap-2 align-items-center">
            <select v-model="priorPolicy" class="form-select form-select-sm" style="width: 210px" :aria-label="text('Prior 导出策略', 'Prior export policy')">
              <option value="strict">{{ text('Strict only', 'Strict only') }}</option>
              <option value="strict_calibrated">{{ text('Strict + calibrated DBD', 'Strict + calibrated DBD') }}</option>
              <option value="extended">{{ text('Extended (exploratory, explicit)', 'Extended (exploratory, explicit)') }}</option>
              <option value="selected">{{ text('Selected TF IDs', 'Selected TF IDs') }}</option>
            </select>
            <template v-if="priorPolicy === 'selected'">
              <input v-model="priorTfs" class="form-control form-control-sm" style="width: 240px"
                     :placeholder="text('TF ID，逗号分隔', 'TF IDs, comma separated')" />
              <label class="form-check form-check-inline small mb-0">
                <input v-model="priorIncludeExploratory" class="form-check-input" type="checkbox" />
                <span class="form-check-label">{{ text('包含 exploratory', 'include exploratory') }}</span>
              </label>
            </template>
            <a class="btn btn-success btn-sm" :class="{ disabled: priorPolicy === 'selected' && !priorSelectedIDs.length }"
               :href="priorSelectedIDs.length || priorPolicy !== 'selected' ? regutracePriorUrl(job!.id, priorPolicy, { tfs: priorSelectedIDs, includeExploratory: priorIncludeExploratory }) : undefined"
               :aria-disabled="priorPolicy === 'selected' && !priorSelectedIDs.length"
               download
               :title="text('导出过滤后的 regulatory background SQLite；保留 evidence level 与 mapping provenance', 'Export a filtered regulatory-background SQLite; evidence level and mapping provenance are preserved')">
              <i class="bi bi-database-down me-1"></i>Prior
            </a>
          </div>
          <span v-if="summary.site_only" class="badge text-bg-warning">{{ text('site-only：prior 需单独运行或连接', 'site-only: prior requires a separate run or join') }}</span>
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
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ (summary.candidate_sites ?? summary.motif_hits).toLocaleString() }}</div><div class="stat-label">{{ text('候选 PWM 位点', 'Candidate PWM sites') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ summary.tf_target_pairs.toLocaleString() }}</div><div class="stat-label">{{ text('TF–target 候选对（计算值）', 'Candidate TF–target pairs (computed)') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ (summary.tf_identified ?? summary.unique_tfs).toLocaleString() }}</div><div class="stat-label">{{ text('已鉴定 TF', 'Identified TFs') }}</div></div></div>
      </div>

      <div class="card mb-3">
        <div class="card-body">
          <div class="d-flex align-items-center flex-wrap gap-2 mb-2">
            <h6 class="mb-0">{{ text('覆盖层级（不要用一层替代另一层）', 'Coverage layers (do not substitute one for another)') }}</h6>
            <span class="small text-muted ms-auto">{{ summary.reference_pack || summary.annotation_source }}</span>
          </div>
          <div class="row g-2 small">
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 h-100">
                <div class="fw-semibold">{{ text('1. TF 鉴定', '1. TF identification') }}</div>
                <div>{{ (summary.tf_identified ?? summary.unique_tfs).toLocaleString() }} identified</div>
                <div class="text-muted">{{ summary.tf_ambiguous ?? '—' }} ambiguous · {{ summary.tf_families ?? '—' }} families</div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 h-100">
                <div class="fw-semibold">{{ text('2. Motif 映射', '2. Motif mapping') }}</div>
                <div>{{ summary.tf_mapped?.toLocaleString() ?? 'not computed' }} mapped</div>
                <div class="text-muted">{{ summary.unmapped_tfs ?? '—' }} unmapped · {{ summary.unique_pwms ?? '—' }} unique PWMs</div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 h-100">
                <div class="fw-semibold">{{ text('3. Motif 可测试性', '3. Motif testability') }}</div>
                <div>{{ summary.testability_available ? `${summary.testable_motifs ?? 0} testable` : 'not computed' }}</div>
                <div class="text-muted">{{ summary.testability_available ? `${summary.unattainable_motifs ?? 0} untestable at requested p` : 'enable Phase 3 artifacts' }}</div>
              </div>
            </div>
            <div class="col-6 col-lg-3">
              <div class="border rounded p-2 h-100">
                <div class="fw-semibold">{{ text('4. Target 预测', '4. Target prediction') }}</div>
                <div>{{ summary.tfs_with_targets?.toLocaleString() ?? '—' }} / {{ (summary.tf_identified ?? 0).toLocaleString() }} TFs with ≥1 target</div>
                <div class="text-muted">{{ summary.actual_target_coverage != null ? (summary.actual_target_coverage * 100).toFixed(1) + '% actual target coverage' : 'coverage not computed' }}</div>
                <div class="text-muted">{{ summary.tf_target_pairs.toLocaleString() }} TF–target pairs<template v-if="summary.site_only"> · site-only (not materialized)</template></div>
              </div>
            </div>
          </div>
          <div class="d-flex flex-wrap gap-2 mt-2">
            <span v-for="(count, status) in (summary.mapping_status_counts ?? {})" :key="status" class="badge" :class="mappingClass(String(status))">
              {{ mappingLabel(String(status)) }} {{ count }}
            </span>
            <span v-if="summary.background_mode" class="badge text-bg-light border">
              {{ text('背景', 'background') }}: {{ summary.background_mode }}
              <template v-if="summary.background_frequencies?.length"> · {{ summary.background_frequencies.map((v) => v.toFixed(3)).join(', ') }}</template>
            </span>
            <span v-if="summary.scan_runtime_seconds" class="badge text-bg-light border">{{ text('扫描', 'scan') }} {{ summary.scan_runtime_seconds.toFixed(1) }} s</span>
            <span v-if="summary.cache_hits != null" class="badge text-bg-light border">{{ text('cache', 'cache') }} {{ summary.cache_hits }} hit / {{ summary.cache_misses }} miss</span>
            <span class="badge text-bg-light border">{{ text('候选 site', 'candidate sites') }} {{ (summary.candidate_sites ?? summary.motif_hits).toLocaleString() }}</span>
          </div>
        </div>
      </div>

      <div class="card mb-3">
        <div class="card-body">
          <div class="d-flex align-items-start flex-wrap gap-2 mb-2">
            <div>
              <h6 class="mb-0">{{ text('Regulatory background 导出', 'Regulatory background export') }}</h6>
              <p class="small text-muted mb-0">{{ text('Build once, reuse across rsWGCNA projects. Manifest、TF catalogue、TF–motif mapping 和 site 表可独立下载；prior 保留现有 SQLite 导入契约。', 'Build once, reuse across rsWGCNA projects. Manifest, TF catalogue, TF–motif mapping and site tables are separate downloads; prior keeps the existing SQLite import contract.') }}</p>
            </div>
            <div class="ms-auto small text-end">
              <span class="badge text-bg-light border">{{ text('schema', 'schema') }} v1</span>
              <span v-if="summary.site_only" class="badge text-bg-warning ms-1">{{ text('site-only', 'site-only') }}</span>
            </div>
          </div>
          <div class="d-flex flex-wrap gap-2">
            <a class="btn btn-sm btn-outline-secondary" :href="regutraceBackgroundUrl(job!.id, 'manifest', mappingMode)" download><i class="bi bi-file-earmark-code me-1"></i>manifest.json</a>
            <a class="btn btn-sm btn-outline-secondary" :href="regutraceBackgroundUrl(job!.id, 'tf_catalogue', mappingMode)" download><i class="bi bi-table me-1"></i>TF catalogue</a>
            <a class="btn btn-sm btn-outline-secondary" :href="regutraceBackgroundUrl(job!.id, 'tf_motif', mappingMode)" download><i class="bi bi-diagram-2 me-1"></i>TF–motif ({{ mappingMode }})</a>
            <a v-if="summary.site_table_available" class="btn btn-sm btn-outline-secondary" :href="regutraceBackgroundUrl(job!.id, 'sites', mappingMode)" download><i class="bi bi-grid-3x3-gap me-1"></i>site table</a>
            <span v-if="summary.site_only" class="small text-muted align-self-center">{{ text('site-only exploratory job 不物化 25M TF–target prior；请使用 site table + TF–motif mapping 在 rsWGCNA 侧连接。', 'Site-only exploratory jobs do not materialize the 25M TF–target prior; join the site table and TF–motif mapping on the rsWGCNA side.') }}</span>
          </div>
          <p class="small text-muted mt-2 mb-0">{{ text('所有 target 仍是 evidence level D 候选；extended 导出必须显式选择，且不会把 exploratory 变成 calibrated。', 'All targets remain evidence level D candidates; extended export must be chosen explicitly and never promotes exploratory to calibrated.') }}</p>
        </div>
      </div>

      <div class="row g-2 mb-3 small text-muted">
        <div class="col-12">
          <template v-if="summary.tf_mapped != null">{{ text('已映射 TF', 'Mapped TFs') }} {{ summary.tf_mapped.toLocaleString() }} · {{ text('已扫描矩阵', 'Matrices scanned') }} {{ summary.motifs_scanned?.toLocaleString() }} · </template>
          {{ text('Motif 库', 'Motif library') }}: {{ summary.motif_library_size.toLocaleString() }}
          · {{ text('有命中 motif', 'motifs with hits') }}: {{ summary.motifs_with_hits.toLocaleString() }}
          · {{ text('唯一 target', 'unique targets') }}: {{ summary.unique_targets.toLocaleString() }}
          · {{ summary.site_only ? text('TF–target–site 行：未物化（site-only）', 'TF–target–site rows: not materialized (site-only)') : text('证据行', 'evidence rows') + ': ' + summary.tf_target_edges.toLocaleString() }}
          <span v-if="summary.threads">· {{ text('线程数', 'threads') }}: {{ summary.threads }}</span>
          · {{ text('注释来源', 'annotation source') }}: {{ summary.annotation_source }}
          <span v-if="summary.metadata_warnings?.length" class="text-warning-emphasis"> · {{ summary.metadata_warnings.length }} metadata warning(s)</span>
        </div>
      </div>

      <div v-if="summary.tf_identified != null" class="card mb-3">
        <div class="card-body">
          <h6>{{ text('转录因子家族分布（全量）', 'Transcription factor families') }}</h6>
          <p class="small text-muted">{{ text('深色为已获 motif，浅色为其余已鉴定 TF，灰色为待判定。点击家族筛选目录。', 'Dark bars show mapped TFs, light bars show other identified TFs, and gray shows unresolved candidates. Select a family to filter the catalogue.') }}</p>
          <div style="max-height: 25rem; overflow: auto" class="mb-3">
            <button v-for="[name, n] in catalogueFamilies" :key="name" type="button"
              class="btn btn-sm d-flex align-items-center gap-2 w-100 text-start"
              :class="familyFilter === name ? 'btn-outline-primary' : 'btn-light'"
              @click="familyFilter = familyFilter === name ? '' : name">
              <span style="width: 12rem">{{ name }}</span>
              <span class="d-flex flex-grow-1" style="height: .7rem">
                <span class="bg-primary rounded-start" :style="{width: (100 * n.mapped / maxFamilyCount) + '%'}"></span>
                <span class="bg-info" :style="{width: (100 * (n.identified - n.mapped) / maxFamilyCount) + '%'}"></span>
                <span class="bg-secondary rounded-end" :style="{width: (100 * n.other / maxFamilyCount) + '%'}"></span>
              </span>
              <span class="text-nowrap">{{ n.mapped }} / {{ n.identified }}<template v-if="n.other"> + {{ n.other }}</template></span>
            </button>
          </div>
          <div class="d-flex flex-wrap gap-2 mb-2">
            <input v-model="catalogueSearch" class="form-control form-control-sm flex-grow-1" style="min-width: 15rem" :aria-label="text('搜索转录因子', 'Search TFs')" :placeholder="text('搜索基因 ID、名称、注释或 motif', 'Search ID, name, annotation or motif')" />
            <select v-model="mappingFilter" class="form-select form-select-sm" style="width: 12rem">
              <option value="">{{ text('全部映射状态', 'All mapping statuses') }}</option>
              <option value="strict_rbh">{{ mappingLabel('strict_rbh') }}</option>
              <option value="dbd_calibrated">{{ mappingLabel('dbd_calibrated') }}</option>
              <option value="dbd_ambiguous">{{ mappingLabel('dbd_ambiguous') }}</option>
              <option value="dbd_exploratory">{{ mappingLabel('dbd_exploratory') }}</option>
              <option value="no_supported_motif">{{ mappingLabel('no_supported_motif') }}</option>
              <option value="not_computed">{{ mappingLabel('not_computed') }}</option>
            </select>
            <select v-model="testabilityFilter" class="form-select form-select-sm" style="width: 11rem">
              <option value="">{{ text('全部可测试性', 'All testability') }}</option>
              <option value="testable">{{ text('至少一个可测试 motif', 'Any testable motif') }}</option>
              <option value="unattainable">{{ text('有不可达 motif', 'Any untestable motif') }}</option>
              <option value="none">{{ text('无 motif', 'No motif') }}</option>
            </select>
            <button v-if="familyFilter" class="btn btn-sm btn-outline-secondary" @click="familyFilter = ''">{{ familyFilter }} ×</button>
          </div>
          <div class="table-responsive">
            <table class="table table-sm align-middle">
              <thead><tr><th>TF</th><th>{{ text('家族', 'Family') }}</th><th>{{ text('鉴定', 'Identified') }}</th><th>{{ text('映射状态', 'Mapping') }}</th><th>Motif</th><th>{{ text('靶标数', 'Targets') }}</th><th>{{ text('可测试性', 'Testability') }}</th><th>{{ text('注释 / 诊断', 'Annotation / diagnosis') }}</th></tr></thead>
              <tbody><tr v-for="item in catalogueRows" :key="item.id">
                <td><button class="btn btn-link btn-sm p-0" @click="loadTFView(item.id); nextTick(() => tfSection?.scrollIntoView({block:'start'}))">{{ item.name }}</button><small v-if="item.name !== item.id" class="d-block text-muted">{{ item.id }}</small></td>
                <td>{{ item.family }}</td><td><span class="small">{{ item.status }}</span></td>
                <td><span class="badge" :class="mappingClass(item.mapping_status)">{{ mappingLabel(item.mapping_status) }}</span></td>
                <td class="small">{{ (item.motifs ?? []).join(', ') || '—' }}<div v-if="item.motif_count" class="text-muted">{{ item.motif_count }} motif(s)</div></td>
                <td>{{ item.target_count?.toLocaleString() ?? '—' }}</td>
                <td class="small">
                  <span v-if="item.testable_motifs">{{ item.testable_motifs }} testable</span>
                  <span v-if="item.unattainable_motifs" class="text-warning-emphasis"> · {{ item.unattainable_motifs }} untestable</span>
                  <span v-if="!item.testable_motifs && !item.unattainable_motifs" class="text-muted">not computed</span>
                </td>
                <td class="small">{{ item.description || item.diagnostic_reason || '—' }}</td>
              </tr></tbody>
            </table>
          </div>
          <div class="d-flex gap-2 align-items-center">
            <button class="btn btn-sm btn-outline-secondary" :disabled="cataloguePage <= 1" @click="cataloguePage--">{{ text('上一页', 'Previous') }}</button>
            <span class="small">{{ cataloguePage }} / {{ cataloguePageCount }} · {{ filteredCatalogue.length }} TF</span>
            <button class="btn btn-sm btn-outline-secondary" :disabled="cataloguePage >= cataloguePageCount" @click="cataloguePage++">{{ text('下一页', 'Next') }}</button>
          </div>
        </div>
      </div>

      <div v-if="summary.tf_identified == null" class="card mb-3">
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
            <button v-for="item in (summary.top_tfs ?? []).slice(0, 8)" :key="item.id" class="btn btn-sm btn-outline-secondary py-0" @click="queryByTf(item.name || item.id)">{{ item.name || item.id }}</button>
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

    <RegutraceTFProfile
      v-if="tfView && job?.id"
      :view="tfView"
      :job-id="job.id"
      :selected-motif="selectedHit?.motif_id"
      @open-target="openPromoter"
    />

    <div v-if="tfView || tfBusy || tfError" ref="tfSection" class="card mb-3">
      <div class="card-body">
        <div class="d-flex align-items-center gap-2 mb-2">
          <h6 class="mb-0">{{ text('TF 基因结构与 domain 证据', 'TF gene structure and domain evidence') }}</h6>
          <span class="badge text-bg-light border">GFF + HMMER</span>
          <span class="small text-muted ms-auto">{{ text('点击 exon/CDS/UTR/domain 查看坐标；domain 投影到 CDS 为近似坐标。', 'Click exon/CDS/UTR/domain for coordinates; CDS projections are approximate.') }}</span>
        </div>
        <div v-if="tfBusy" class="text-muted small"><span class="spinner-border spinner-border-sm me-2"></span>{{ text('读取 GFF 结构和 domain…', 'Reading GFF structure and domains…') }}</div>
        <div v-else-if="tfError" class="alert alert-warning small mb-0">{{ tfError }}</div>
        <RegutraceTFEvidence
          v-else-if="tfView"
          :view="tfView"
          :promoter="promoter"
          :tf-id="selectedTfId"
          :selected-hit="selectedHit"
          @select-hit="selectHit"
          @focus-motif="focusMotif"
        />
      </div>
    </div>

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
                @click="selectHit(site.hit)"
                @keydown.enter="selectHit(site.hit)"
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
              <div class="small text-muted">
                {{ selectedHit.motif_id }} {{ selectedHit.motif_name }} · {{ selectedHit.strand }} · {{ selectedHit.start }}-{{ selectedHit.end }}
                · promoter {{ selectedHit.promoter_start ?? promoter?.start ?? '—' }}-{{ selectedHit.promoter_end ?? promoter?.end ?? '—' }}
              </div>
            </div>
            <button class="btn btn-sm btn-outline-secondary ms-auto" @click="copySequence(selectedHit.matched_site)"><i class="bi bi-clipboard me-1"></i>{{ text('复制序列', 'Copy sequence') }}</button>
          </div>
          <div class="row g-2 mt-1">
            <div class="col-12 col-md-6">
              <div class="small text-muted">{{ text('TF', 'TFs') }} <span class="badge text-bg-secondary">{{ selectedHit.tfs.length }}</span></div>
              <div v-for="tf in selectedHit.tfs" :key="tf.id" class="small">
                <span class="fw-semibold">{{ tf.name || tf.symbol || tf.id }}</span>
                <span v-if="tf.family" class="text-muted"> · {{ tf.family }}</span>
                <span class="text-muted"> · {{ tf.id }}</span>
                <button class="btn btn-sm btn-link p-0 ms-1" :title="text('查看 TF 基因结构和 domain', 'View TF gene structure and domains')" @click="selectTF(tf.id)">
                  <i class="bi bi-diagram-3"></i>
                </button>
                <div class="text-muted ms-3">{{ text('donor', 'donor') }}: {{ donorLabel(tf) }}</div>
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
          <div class="mt-3 border-top pt-2">
            <div class="d-flex align-items-center gap-2">
              <div class="fw-semibold small">{{ text('原始 evidence 行', 'Raw evidence rows') }}</div>
              <span class="badge text-bg-secondary">{{ siteEvidence.length }}</span>
            </div>
            <div v-if="siteEvidenceBusy" class="small text-muted mt-1"><span class="spinner-border spinner-border-sm me-1"></span>{{ text('查询原始位点证据…', 'Loading raw site evidence…') }}</div>
            <div v-else-if="siteEvidenceError" class="alert alert-warning small mt-2 mb-0">{{ siteEvidenceError }}</div>
            <div v-else-if="!siteEvidence.length" class="small text-muted mt-1">{{ text('没有匹配的原始 evidence 行。', 'No matching raw evidence rows.') }}</div>
            <div v-else class="table-responsive mt-2">
              <table class="table table-sm align-middle mb-0">
                <thead>
                  <tr>
                    <th>{{ text('TF', 'TF') }}</th>
                    <th>{{ text('donor', 'donor') }}</th>
                    <th>{{ text('级别', 'level') }}</th>
                    <th>{{ text('方法', 'method') }}</th>
                    <th>{{ text('promoter', 'promoter') }}</th>
                    <th>{{ text('位点', 'site') }}</th>
                    <th>score</th>
                    <th>p</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(row, i) in siteEvidence" :key="`${row.tf_id}-${row.promoter_start}-${row.promoter_end}-${row.hit_start}-${row.hit_end}-${i}`">
                    <td class="small"><span class="fw-semibold">{{ row.tf_symbol || row.tf_id }}</span><div class="text-muted">{{ row.tf_id }}</div></td>
                    <td class="small">{{ evidenceDonor(row.tf_id) }}</td>
                    <td class="small">{{ row.evidence_level || '—' }}</td>
                    <td class="small">{{ row.method || row.evidence_type || '—' }}<div v-if="row.source" class="text-muted">{{ row.source }}</div></td>
                    <td class="small font-monospace">{{ rowPromoter(row) }}</td>
                    <td class="small font-monospace">{{ rowSite(row) }}</td>
                    <td class="small">{{ row.score?.toFixed(2) ?? '—' }}</td>
                    <td class="small">{{ formatP(row.p_value) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <p v-else class="small text-muted mt-2 mb-0">{{ text('点击一个位点查看 TF 和序列。', 'Click a site to see TF and sequence.') }}</p>
      </div>
    </div>

    <RegutraceTargetRegulators
      v-if="promoter && job?.id && job.status === 'succeeded'"
      :job-id="job.id"
      :target-id="promoter.target_id"
    />

    <RegutraceMotifExplorer v-if="job?.id && job.status === 'succeeded'" :job-id="job.id" />
    <RegutraceEnrichment
      v-if="job?.id && job.status === 'succeeded'"
      :job-id="job.id"
      :motif-options="summary?.top_motifs?.map((item) => item.id) ?? []"
    />
    <RegutraceProvenance v-if="job?.id && job.status === 'succeeded'" :job-id="job.id" />

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
          <button v-if="d.severity === 'error'" class="btn btn-sm btn-outline-primary ms-2 py-0" @click="askAIAboutDiagnostic(d)">
            <i class="bi bi-stars me-1"></i>{{ text('让 AI 助手分析', 'Ask AI assistant') }}
          </button>
          <div v-if="d.suggestion" class="small text-muted ms-4">{{ d.suggestion }}</div>
          <div v-if="d.code === 'ANNOTATION_ID_MISMATCH'" class="small text-danger ms-4">
            {{ text('注释 ID 与 GFF 基因 ID 没有重合。常见原因是 eggNOG 使用 transcript/protein ID（如 Fta_1G0000020.1），而 GFF 使用 gene ID（Fta_1G0000020）。可以点“让 AI 助手分析”生成映射方案。', 'Annotation IDs have zero overlap with GFF gene IDs. eggNOG often uses transcript/protein IDs (e.g. Fta_1G0000020.1) while GFF uses gene IDs (Fta_1G0000020). Use “Ask AI assistant” to plan a mapping.') }}
          </div>
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
