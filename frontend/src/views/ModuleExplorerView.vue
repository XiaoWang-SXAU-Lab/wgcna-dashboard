<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { EChartsOption } from 'echarts'
import { useProjectStore } from '../stores/project'
import type { AnnotationRow, DetailResult, EnrichResult, ExpressionResult, HubPriorityResult, HubResult, MtaitResult, ModulesResult, NetEdge, NetworkResult, ReanResult, SampleInfo, TfTargetUpstreamRow } from '../api/client'
import { getTfTargetUpstream } from '../api/client'
import { getAnnotation, getEnrich, getExpression, getSampleInfo, projectOutputUrl } from '../api/projects'
import { locale } from '../utils/i18n'
import GsMmScatter from '../components/GsMmScatter.vue'
import ModuleGeneHeatmap from '../components/ModuleGeneHeatmap.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
import SimpleTable from '../components/SimpleTable.vue'
import HubPriorityPlainSummary from '../components/HubPriorityPlainSummary.vue'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'
import ResizableChart from '../components/ResizableChart.vue'
import type { NetLayout, SizeBasis, LabelMode, NodeColorMode } from '../components/NetworkGraph.vue'
import { makePoints, optionFromPoints } from '../utils/enrichmentChart'
import { hclustAverage } from '../utils/hclust'

type Tab = 'detail' | 'network' | 'priority' | 'annotation' | 'go' | 'kegg' | 'prior'

const store = useProjectStore()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const tab = ref<Tab>('detail')
const showExportGsmm = ref(false)
const showExportHeatmap = ref(false)

const detail = computed(() => store.results.module_detail as DetailResult | undefined)
const hub = computed(() => store.results.hub as HubResult | undefined)
const hasTomStrength = computed(() => (hub.value?.network.nodes ?? []).some((n) => n.extracted_tom_strength !== undefined || n.kwithin !== undefined))
const hasKwithin = computed(() => (hub.value?.network.nodes ?? []).some((n) => n.module_kwithin !== undefined))
const mtrait = computed(() => store.results.module_trait as MtaitResult | undefined)
const network = computed(() => store.results.network as ModulesResult | undefined)
const rean = computed(() => store.results.rean as ReanResult | undefined)
const canRunRean = computed(() => store.currentResult('hub') && !!hub.value && !store.busy)
const reanStale = computed(() => !!rean.value && !store.currentResult('rean'))
const showEdgeStability = ref(false)
watch([tab, () => store.id], () => { showEdgeStability.value = false })

const traitOptions = computed(() => mtrait.value?.traits ?? [])
const moduleOptions = computed(() => {
  const sizes = network.value?.module_sizes ?? {}
  return Object.keys(sizes).sort((a, b) => (sizes[b] ?? 0) - (sizes[a] ?? 0))
})

const canRunDetail = computed(() =>
  store.current?.validity.filter === '' && store.current?.validity.network === '' && store.current?.validity.module_trait === ''
  && !!store.draft.module_detail.select_module && !!store.draft.module_detail.select_trait && !store.busy)
const invalidReasons = computed(() => {
  const v = (store.current?.validity ?? {}) as Record<string, string>
  // Only the steps that actually gate "module detail" / "hub" here. hub / rean
  // are downstream and are surfaced separately next to their own buttons.
  return ['filter', 'network', 'module_trait'].filter((s) => v[s]).map((s) => ({ step: s, reason: v[s] }))
})
// The hub step itself may be stale (engine/params/draft changed) even when its
// upstream is valid. Use the unified currentResult selector, which also covers
// local (unflushed) draft changes, not just backend validity.
const hubStale = computed(() => !!hub.value && !store.currentResult('hub'))

// --- network visualization params (local, no backend) ---
const layout = ref<NetLayout>('fcose')
const ringTopN = ref(25)
const ringInnerN = ref(10)
const sizeBasis = ref<SizeBasis>('degree')
const nodeSizeMin = ref(5)
const nodeSizeMax = ref(30)
const edgeLowColor = ref('#cbd5e1')
const edgeHighColor = ref('#e11d48')
const edgeWidth = ref(6)
const labelSize = ref(10)
const labelMode = ref<LabelMode>('hub')
const colorMode = ref<NodeColorMode>('kme')
const nodeLowColor = ref('#dbeafe')
const nodeHighColor = ref('#1d4ed8')
const hubNodeColor = ref('#e11d48')
const hubBorderColor = ref('#0f172a')
const nodeBorderColor = ref('#ffffff')
const nodeBorderWidth = ref(1.5)

// If the loaded result lacks a metric (old result), fall the selected size/color
// basis back to kME instead of mapping unknown to zero.
watch([hasTomStrength, hasKwithin], () => {
  if (!hasTomStrength.value) {
    if (sizeBasis.value === 'tomStrength') sizeBasis.value = 'kme'
    if (colorMode.value === 'tomStrength') colorMode.value = 'kme'
  }
  if (!hasKwithin.value) {
    if (sizeBasis.value === 'kwithin') sizeBasis.value = 'kme'
    if (colorMode.value === 'kwithin') colorMode.value = 'kme'
  }
})

const gradientColor = computed(() => ['kme', 'gs', 'degree'].includes(colorMode.value))
const netNodeCount = computed(() => hub.value?.network.nodes.length ?? 0)
const netEdgeCount = computed(() => hub.value?.network.edges.length ?? 0)

// Display-only network sparsification: the worker always returns the full induced
// subgraph; these controls only decide what is drawn (no re-run).
const displayMode = ref<'sparse' | 'hub' | 'full'>('hub')
const maxEdges = ref(500)
const minEdgeWeight = ref(0)
const hideIsolated = ref(true)
const selectedElement = ref<{ kind: 'node' | 'edge'; data: Record<string, any> } | null>(null)
function onSelect(payload: { kind: 'node' | 'edge'; data: Record<string, any> }) {
  selectedElement.value = payload
}
function onClear() {
  selectedElement.value = null
}
watch([() => store.current?.state.steps.hub?.execution_id, () => store.currentResult('hub')], onClear)

// Sparse mode: keep hub-incident edges first (they carry the hub structure),
// then spend the remaining budget on the strongest non-hub edges so
// neighbor--neighbor topology still shows. Never invent edges, and never exceed
// the weight floor or the edge budget.
function pickSparseEdges(edges: NetEdge[], hubIds: Set<string>, budget: number, minW: number): NetEdge[] {
  const elig = edges.filter((e) => e.weight > minW)
  const hubE: NetEdge[] = []
  const otherE: NetEdge[] = []
  for (const e of elig) (hubIds.has(e.source) || hubIds.has(e.target) ? hubE : otherE).push(e)
  hubE.sort((a, b) => b.weight - a.weight)
  otherE.sort((a, b) => b.weight - a.weight)
  // Reserve up to 30% of the budget for non-hub edges (when any exist) so the
  // graph does not collapse back into a hub star when hub edges are numerous.
  const otherReserve = Math.min(otherE.length, Math.max(0, Math.floor(budget * 0.3)))
  const hubTake = Math.min(hubE.length, Math.max(0, budget - otherReserve))
  const otherTake = Math.min(otherE.length, budget - hubTake)
  const picked: NetEdge[] = []
  const seen = new Set<string>()
  const push = (e: NetEdge) => {
    const key = e.source < e.target ? e.source + '|' + e.target : e.target + '|' + e.source
    if (seen.has(key)) return
    seen.add(key)
    picked.push(e)
  }
  hubE.slice(0, hubTake).forEach(push)
  otherE.slice(0, otherTake).forEach(push)
  return picked
}

const reanRows = computed(() =>
  [...(rean.value?.edges ?? [])]
    .sort((a, b) => (a.stability ?? Infinity) - (b.stability ?? Infinity))
    .map((e) => ({ source: e.source, target: e.target, stability: e.stability?.toFixed(3) ?? '—' })))

const displayNetwork = computed<NetworkResult | undefined>(() => {
  const net = hub.value?.network
  if (!net) return undefined
  const hubIds = new Set(net.nodes.filter((n) => n.is_seed).map((n) => n.id))
  let edges = net.edges
  if (displayMode.value === 'hub') {
    // Hub-centric = a clean hub-and-spoke: keep each hub's top-K edges (by weight)
    // instead of every hub-incident edge, which in a dense module would connect
    // every hub to every node (degree ≈ N-1 for hubs, ≈ #hubs for non-hubs).
    const k = Math.max(1, Number(store.draft.hub.top_n_neighbors) || 20)
    const perHub = new Map<string, NetEdge[]>()
    for (const e of net.edges) {
      if (e.weight <= minEdgeWeight.value) continue
      if (hubIds.has(e.source)) { const a = perHub.get(e.source) ?? []; a.push(e); perHub.set(e.source, a) }
      if (hubIds.has(e.target)) { const a = perHub.get(e.target) ?? []; a.push(e); perHub.set(e.target, a) }
    }
    const seen = new Set<string>()
    edges = []
    for (const es of perHub.values()) {
      es.sort((a, b) => b.weight - a.weight)
      for (const e of es.slice(0, k)) {
        const key = e.source < e.target ? e.source + '::' + e.target : e.target + '::' + e.source
        if (seen.has(key)) continue
        seen.add(key)
        edges.push(e)
      }
    }
  } else if (displayMode.value === 'sparse') {
    edges = pickSparseEdges(edges, hubIds, Math.max(1, maxEdges.value), minEdgeWeight.value)
  } else {
    edges = edges.filter((e) => e.weight > minEdgeWeight.value)
  }
  const used = new Set<string>()
  edges.forEach((e) => { used.add(e.source); used.add(e.target) })
  const nodes = hideIsolated.value ? net.nodes.filter((n) => used.has(n.id)) : net.nodes
  return { ...net, nodes, edges }
})

const retention = computed(() => {
  const full = hub.value?.network
  const shown = displayNetwork.value
  if (!full || !shown) return { nodePct: 0, edgePct: 0 }
  return {
    nodePct: full.nodes.length ? Math.round((shown.nodes.length / full.nodes.length) * 100) : 0,
    edgePct: full.edges.length ? Math.round((shown.edges.length / full.edges.length) * 100) : 0,
  }
})

const largeNetwork = computed(() => (displayNetwork.value?.edges.length ?? 0) > 3000)

const hubRows = computed(() => (hub.value?.hub_genes ?? []).map((h) => ({
  gene: h.gene, kme: h.kme.toFixed(4), gs: h.gs.toFixed(4),
})))

function syncHub() {
  store.draft.hub.select_module = store.draft.module_detail.select_module
  store.draft.hub.select_trait = store.draft.module_detail.select_trait
}

// --- candidate priority (hub_priority) ---
const hubPriority = computed(() => store.results.hub_priority as HubPriorityResult | undefined)
const hubPriorityCurrent = computed(() => store.currentResult('hub_priority'))
const hubPriorityStale = computed(() => !!hubPriority.value && !hubPriorityCurrent.value)
const hubPriorityModuleSize = computed(() => network.value?.module_sizes?.[store.draft.hub_priority.select_module as string] ?? 0)
// Engineering budget g²×B (not a scientific threshold); shown as guidance only.
const hubPriorityCost = computed(() => hubPriorityModuleSize.value * hubPriorityModuleSize.value * (store.draft.hub_priority.n_bootstraps as number))
const hubPriorityOverBudget = computed(() => hubPriorityCost.value > 1_000_000_000)
const hubPriorityMaxBootstraps = computed(() => {
  const g = hubPriorityModuleSize.value
  return g > 0 ? Math.floor(1_000_000_000 / (g * g)) : 0
})
const hubPrioritySuggestedB = computed(() => Math.min(100, hubPriorityMaxBootstraps.value))
// Gate: only filter/network valid + module selected + design explicitly confirmed
// + the worker's g²×B engineering budget. module_trait / hub / trait are NOT required.
const canRunHubPriority = computed(() =>
  store.current?.validity.filter === '' && store.current?.validity.network === ''
  && !!store.draft.hub_priority.select_module
  && !!store.draft.hub_priority.design_confirmed
  && !hubPriorityOverBudget.value
  && !store.busy)
const priorityAnnotationRows = ref<AnnotationRow[]>([])
const priorityAnnotationLoading = ref(false)
const priorityAnnotationMap = computed(() => new Map(priorityAnnotationRows.value.map((a) => [a.gene_id, a])))
const hubPriorityRows = computed(() => {
  const genes = hubPriority.value?.genes ?? []
  return [...genes]
    .sort((a, b) => {
      const am = a.midrank_median, bm = b.midrank_median
      if (am == null && bm == null) return a.gene.localeCompare(b.gene)
      if (am == null) return 1
      if (bm == null) return -1
      return am - bm
    })
    .map((g) => {
      const a = priorityAnnotationMap.value.get(g.gene)
      return {
        gene: g.gene,
        symbol: a?.symbol || a?.gene_name || '—',
        description: a?.description || '—',
        kwithin: g.kwithin.toFixed(4),
        kme_signed: g.kme_signed.toFixed(4),
        kme_abs: g.kme_abs.toFixed(4),
        rank: `${g.rank_lower}-${g.rank_upper}`,
        midrank_median: g.midrank_median != null ? g.midrank_median.toFixed(2) : '—',
        midrank_p05: g.midrank_p05 != null ? g.midrank_p05.toFixed(2) : '—',
        midrank_p95: g.midrank_p95 != null ? g.midrank_p95.toFixed(2) : '—',
        mean_top_k: g.mean_top_k != null ? g.mean_top_k.toFixed(3) : '—',
      }
    })
})
const priorityPage = ref(0)
const priorityPageSize = ref(50)
const priorityPagedRows = computed(() => {
  const start = priorityPage.value * priorityPageSize.value
  return hubPriorityRows.value.slice(start, start + priorityPageSize.value)
})
const priorityPageCount = computed(() => Math.max(1, Math.ceil(hubPriorityRows.value.length / priorityPageSize.value)))
const priorityAllInvalid = computed(() => !!hubPriority.value && (hubPriority.value.valid ?? 0) === 0)
watch(() => store.draft.hub_priority.select_module, () => { priorityPage.value = 0 })

const annotationRows = ref<AnnotationRow[]>([])
const annotationLoading = ref(false)
const enrichResults = ref<EnrichResult[]>([])
const enrichLoading = ref(false)
const enrichFdr = ref(0.05)
const enrichTop = ref(0)
const maxCountOn = ref(true)
const goOntology = ref<'ALL' | 'MF' | 'BP' | 'CC'>('ALL')
const showExportEnrich = ref(false)
const enrichType = computed<'go' | 'kegg'>(() => (tab.value === 'kegg' ? 'kegg' : 'go'))
const selectedModuleId = computed(() => store.draft.module_detail.select_module as string)

const priorRows = ref<TfTargetUpstreamRow[]>([])
const priorGeneCount = ref(0)
const priorLoading = ref(false)
const priorError = ref('')
const priorModuleGenes = computed(() => {
  const net = network.value
  const moduleId = selectedModuleId.value
  if (!net || !moduleId) return []
  const genes: string[] = []
  net.genes.forEach((gene, i) => { if (net.colors[i] === moduleId) genes.push(gene) })
  return genes
})
const priorChecksum = computed(() => store.current?.state.inputs?.tf_target?.sha256 ?? '')
let priorRequest = 0
const priorDisplayRows = computed(() => priorRows.value.map((r) => ({
  ...r,
  best_p: r.best_p == null ? '—' : r.best_p.toExponential(2),
  evidence_levels: r.evidence_levels || '—',
})))
async function loadPriorData() {
  const request = ++priorRequest
  priorError.value = ''
  priorRows.value = []
  priorGeneCount.value = 0
  if (!store.id || !selectedModuleId.value) {
    priorLoading.value = false
    return
  }
  if (!priorModuleGenes.value.length) {
    priorLoading.value = false
    priorError.value = text('当前模块没有可查询的基因列表；请先运行网络分析。', 'No gene list for this module; run the network step first.')
    return
  }
  const projectId = store.id
  const genes = [...priorModuleGenes.value]
  priorLoading.value = true
  try {
    const result = await getTfTargetUpstream(projectId, genes)
    if (request !== priorRequest) return
    priorRows.value = result.rows
    priorGeneCount.value = result.gene_count
  } catch (e) {
    if (request === priorRequest) priorError.value = String(e)
  } finally {
    if (request === priorRequest) priorLoading.value = false
  }
}
watch([tab, priorModuleGenes, () => store.id, priorChecksum], ([t]) => {
  if (t === 'prior') void loadPriorData()
  else {
    priorRequest++
    priorLoading.value = false
  }
})

// User-editable per-node labels (overrides the auto symbol/gene_name/id).
const labelOverrides = ref<Record<string, string>>({})
const nodeInfoOpen = ref(false)

// gene_id -> label: user override > symbol > gene_name > gene_id.
const geneMap = computed<Record<string, string>>(() => {
  const m: Record<string, string> = {}
  const anno = new Map(annotationRows.value.map((r) => [r.gene_id, r]))
  for (const n of hub.value?.network.nodes ?? []) {
    const a = anno.get(n.id)
    m[n.id] = labelOverrides.value[n.id] || a?.symbol || a?.gene_name || n.id
  }
  return m
})

const nodeInfoRows = computed(() => {
  const anno = new Map(annotationRows.value.map((r) => [r.gene_id, r]))
  return (displayNetwork.value?.nodes ?? []).map((n) => {
    const a = anno.get(n.id)
    return { gene_id: n.id, symbol: a?.symbol || '', is_hub: n.is_hub, autoLabel: a?.symbol || a?.gene_name || n.id }
  })
})
function onLabelInput(geneId: string, e: Event) {
  labelOverrides.value[geneId] = (e.target as HTMLInputElement).value
}
function clearLabels() { labelOverrides.value = {} }

const filteredEnrich = computed(() => {
  if (enrichType.value !== 'go' || goOntology.value === 'ALL') return enrichResults.value
  return enrichResults.value.filter((r) => r.ontology === goOntology.value)
})
const enrichOption = computed(() => optionFromPoints(makePoints(filteredEnrich.value, enrichType.value.toUpperCase() as 'GO' | 'KEGG'), enrichType.value.toUpperCase() + ' ' + text('富集', 'Enrichment'), '460px'))
const enrichRows = computed(() => filteredEnrich.value.map((r) => ({
  id: r.id, description: r.description, gene_ratio: r.gene_ratio, bg_ratio: r.bg_ratio,
  p_adjust: r['p.adjust'].toExponential(2), count: r.count,
})))

let annotationRequest = 0
let priorityAnnotationRequest = 0
let enrichRequest = 0
let termRequest = 0

async function loadAnnotation() {
  const request = ++annotationRequest
  const projectId = store.id
  const m = selectedModuleId.value
  if (!projectId || !m) {
    annotationRows.value = []
    annotationLoading.value = false
    return
  }
  annotationLoading.value = true
  try {
    const rows = await getAnnotation(projectId, m)
    if (request === annotationRequest && store.id === projectId && selectedModuleId.value === m) annotationRows.value = rows
  } catch {
    if (request === annotationRequest && store.id === projectId && selectedModuleId.value === m) annotationRows.value = []
  } finally {
    if (request === annotationRequest) annotationLoading.value = false
  }
}
async function loadPriorityAnnotation(module?: string) {
  const request = ++priorityAnnotationRequest
  const projectId = store.id
  const m = module || (store.draft.hub_priority.select_module as string)
  if (!projectId || !m) {
    priorityAnnotationRows.value = []
    priorityAnnotationLoading.value = false
    return
  }
  priorityAnnotationLoading.value = true
  try {
    const rows = await getAnnotation(projectId, m)
    if (request === priorityAnnotationRequest && store.id === projectId && (store.draft.hub_priority.select_module as string) === m) {
      priorityAnnotationRows.value = rows
    }
  } catch {
    if (request === priorityAnnotationRequest && store.id === projectId && (store.draft.hub_priority.select_module as string) === m) {
      priorityAnnotationRows.value = []
    }
  } finally {
    if (request === priorityAnnotationRequest) priorityAnnotationLoading.value = false
  }
}
function enrichSelectionCurrent(request: number, projectId: string, m: string, type: 'go' | 'kegg', fdr: number, top: number, maxCount: number) {
  return request === enrichRequest && store.id === projectId && selectedModuleId.value === m &&
    enrichType.value === type && enrichFdr.value === fdr && enrichTop.value === top &&
    (maxCountOn.value ? 500 : 0) === maxCount
}
async function loadEnrich() {
  const request = ++enrichRequest
  const projectId = store.id
  const m = selectedModuleId.value
  const type = enrichType.value
  const fdr = enrichFdr.value
  const top = enrichTop.value
  const maxCount = maxCountOn.value ? 500 : 0
  if (!projectId || !m) {
    enrichLoading.value = false
    return
  }
  enrichLoading.value = true
  try {
    const rows = await getEnrich(projectId, m, type, fdr, top, maxCount)
    if (enrichSelectionCurrent(request, projectId, m, type, fdr, top, maxCount)) enrichResults.value = rows
  } catch (e) {
    if (enrichSelectionCurrent(request, projectId, m, type, fdr, top, maxCount)) {
      store.error = String(e)
      enrichResults.value = []
    }
  } finally {
    if (request === enrichRequest) enrichLoading.value = false
  }
}
watch(selectedModuleId, (m) => { if (m) { void loadAnnotation(); enrichResults.value = [] } }, { immediate: true })
watch(() => tab.value, (t) => {
  if (t === 'annotation') void loadAnnotation()
  if (t === 'go' || t === 'kegg') { enrichResults.value = []; void loadEnrich() }
})
// Candidate priority may target a different module than Module Explorer's
// annotation tab; load the annotation for the priority module explicitly so
// the candidate table can show gene function without a second click.
watch(
  [() => tab.value, () => store.draft.hub_priority.select_module, () => store.id],
  ([t, m]) => {
    if (t === 'priority' && m) void loadPriorityAnnotation(m as string)
    else if (!m) priorityAnnotationRows.value = []
  },
  { immediate: true },
)

const selectedTerm = ref<{ termId: string; description: string; source: string; genes: string[] } | null>(null)
const termExpr = ref<ExpressionResult | null>(null)
const termExprLoading = ref(false)
watch([selectedModuleId, () => store.id], () => {
  termRequest++
  selectedTerm.value = null
  termExpr.value = null
  termExprLoading.value = false
})

async function selectTerm(termId: string, description: string, genes: string[], source: string) {
  if (!genes?.length || !store.id) return
  const request = ++termRequest
  const projectId = store.id
  selectedTerm.value = { termId, description, source, genes }
  termExprLoading.value = true
  try {
    const result = await getExpression(projectId, genes)
    if (request === termRequest && store.id === projectId) termExpr.value = result
  } catch (e) {
    if (request === termRequest && store.id === projectId) {
      store.error = String(e)
      termExpr.value = null
    }
  } finally {
    if (request === termRequest) termExprLoading.value = false
  }
}
function onChartClick(params: any) {
  const d = params.data
  if (d && d.genes?.length) selectTerm(d.termId, d.description, d.genes, d.source)
}
function onEnrichRowClick(row: Record<string, any>) {
  const r = filteredEnrich.value.find((x) => x.id === row.id)
  if (!r) return
  selectTerm(r.id, r.description, r.gene_id, enrichType.value.toUpperCase())
}

const termGeneRows = computed(() => (termExpr.value?.rows ?? []).map((r) => ({
  gene: r.gene, symbol: r.symbol || '', gene_name: r.gene_name || '', mean: r.mean.toFixed(1),
})))

const sampleInfos = ref<SampleInfo[]>([])
let sampleInfoRequest = 0
watch([() => store.current?.state.inputs?.sample_info?.sha256, () => store.session], async () => {
  const request = ++sampleInfoRequest
  const session = store.session
  const projectId = store.id
  const checksum = store.current?.state.inputs?.sample_info?.sha256 ?? ''
  if (!projectId || !checksum) { sampleInfos.value = []; return }
  try {
    const rows = await getSampleInfo(projectId)
    if (request === sampleInfoRequest && store.session === session && store.id === projectId && (store.current?.state.inputs?.sample_info?.sha256 ?? '') === checksum) sampleInfos.value = rows
  } catch {
    if (request === sampleInfoRequest && store.session === session && store.id === projectId && (store.current?.state.inputs?.sample_info?.sha256 ?? '') === checksum) sampleInfos.value = []
  }
}, { immediate: true })
const sampleColumns = computed(() => {
  if (!sampleInfos.value.length) return []
  return Object.keys(sampleInfos.value[0].attrs).filter((c) => !['sample_id', 'sample_name', 'species'].includes(c))
})
const ANNOT_PALETTE = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b', '#e377c2', '#7f7f7f', '#bcbd22', '#17becf', '#aec7e8', '#ffbb78']
function colorFor(v: string): string {
  if (!v) return '#e5e7eb'
  let h = 0
  for (let i = 0; i < v.length; i++) h = (h * 31 + v.charCodeAt(i)) >>> 0
  return ANNOT_PALETTE[h % ANNOT_PALETTE.length]
}

const termHeatOption = computed<EChartsOption>(() => {
  if (!termExpr.value?.rows.length) return {}
  const rows = termExpr.value.rows
  const samples = termExpr.value.samples
  const nSamples = samples.length
  const z = rows.map((r) => {
    const n = r.values.length || 1
    const mean = r.values.reduce((a, b) => a + b, 0) / n
    const ss = r.values.reduce((s, v) => s + (v - mean) * (v - mean), 0)
    const sd = Math.sqrt(ss / (n - 1 || 1))
    return r.values.map((v) => (sd === 0 ? 0 : (v - mean) / sd))
  })
  // cluster columns (samples) via average-linkage hclust on the z-scored matrix
  let sampleOrder = samples.map((_, i) => i)
  if (nSamples > 1) {
    const dissim = Array.from({ length: nSamples }, (_, i) => Array.from({ length: nSamples }, (_, j) => {
      if (i === j) return 0
      let s = 0
      for (let g = 0; g < z.length; g++) s += (z[g][i] - z[g][j]) ** 2
      return Math.sqrt(s)
    }))
    sampleOrder = hclustAverage(dissim).order
  }
  const orderedSamples = sampleOrder.map((i) => samples[i])
  const geneOrder = rows.map((_, i) => i).sort((a, b) => rows[b].mean - rows[a].mean)
  const labels = geneOrder.map((i) => rows[i].symbol || rows[i].gene)
  const heat: [number, number, number][] = []
  geneOrder.forEach((gi, newGi) => sampleOrder.forEach((si, newSi) => heat.push([newSi, newGi, +z[gi][si].toFixed(3)])))
  let maxAbs = 1
  z.forEach((row) => row.forEach((v) => (maxAbs = Math.max(maxAbs, Math.abs(v)))))

  const sampleMap = new Map(sampleInfos.value.map((s) => [s.sample_id, s.attrs]))
  const bands = sampleColumns.value
  const nBands = bands.length
  const bandHeight = 18
  const grids: any[] = [
    { left: 160, right: 40, top: 48 + nBands * bandHeight, bottom: 90 },
    ...bands.map((_, i) => ({ left: 160, right: 40, top: 48 + i * bandHeight, height: bandHeight })),
  ]
  const xAxes: any[] = [
    { type: 'category', data: orderedSamples, gridIndex: 0, axisLabel: { rotate: 90, fontSize: 9 } },
    ...bands.map((_, i) => ({ type: 'category', data: orderedSamples, gridIndex: i + 1, axisLabel: { show: false }, axisTick: { show: false } })),
  ]
  const yAxes: any[] = [
    { type: 'category', data: labels, gridIndex: 0, axisLabel: { fontSize: 9 } },
    ...bands.map((b, i) => ({ type: 'category', data: [b], gridIndex: i + 1, axisLabel: { fontSize: 9 } })),
  ]
  const series: any[] = [
    { type: 'heatmap', xAxisIndex: 0, yAxisIndex: 0, data: heat, itemStyle: { borderColor: 'rgba(255,255,255,0.25)', borderWidth: 0.4 } },
  ]
  bands.forEach((b, bi) => {
    series.push({
      type: 'custom', xAxisIndex: bi + 1, yAxisIndex: bi + 1, data: [0], z: 5,
      renderItem: (_params: any, api: any) => {
        const children: any[] = []
        for (let si = 0; si < nSamples; si++) {
          const val = sampleMap.get(orderedSamples[si])?.[b] ?? ''
          const color = colorFor(val)
          const x0 = api.coord([si - 0.5, 0])[0]
          const x1 = api.coord([si + 0.5, 0])[0]
          const yTop = api.coord([si, 0.5])[1]
          const yBot = api.coord([si, -0.5])[1]
          children.push({ type: 'rect', shape: { x: x0, y: yTop, width: x1 - x0, height: yBot - yTop }, style: { fill: color } })
        }
        return { type: 'group', children }
      },
    })
  })

  return {
    title: { text: (selectedTerm.value?.description ?? '') + ' (' + rows.length + ' genes)', left: 'center', top: 2, textStyle: { fontSize: 13 } },
    tooltip: { formatter: (p: any) => p.seriesType === 'heatmap' ? labels[p.value[1]] + ' × ' + orderedSamples[p.value[0]] + '<br/>z = ' + p.value[2] : '' },
    grid: grids,
    xAxis: xAxes,
    yAxis: yAxes,
    visualMap: { min: -maxAbs, max: maxAbs, calculable: true, orient: 'horizontal', left: 'center', bottom: 8, inRange: { color: ['#2166ac', '#ffffff', '#b2182b'] }, seriesIndex: 0 },
    series,
  }
})
</script>

<template>
  <div v-if="!store.current" class="text-center py-5">
    <i class="bi bi-folder2-open fs-1 text-muted"></i>
    <h4 class="mt-3">{{ text('尚未打开项目', 'No project open') }}</h4>
    <p class="text-muted">{{ text('请先新建或打开项目并完成前面的步骤，再深入单个模块。', 'Open or create a project and finish earlier steps before exploring a module.') }}</p>
    <router-link to="/projects" class="btn btn-primary mt-2">{{ text('前往项目', 'Go to projects') }}</router-link>
  </div>
  <SidebarDrawer v-else>
    <template #sidebar>
      <div class="d-flex flex-column gap-2">
        <div class="nav nav-pills flex-column gap-1 mb-1">
          <button v-for="tk in (['detail', 'network', 'priority', 'annotation', 'go', 'kegg', 'prior'] as Tab[])" :key="tk"
            class="nav-link text-start" :class="{ active: tab === tk }" @click="tab = tk">
            <i class="bi me-1" :class="{ detail: 'bi-grid-1x2', network: 'bi-diagram-3', priority: 'bi-trophy', annotation: 'bi-tags', go: 'bi-circle', kegg: 'bi-diagram-2', prior: 'bi-arrow-up-right-circle' }[tk]"></i>
            {{ text({ detail: '模块详情', network: '模块内网络（Hub）', priority: '候选优先级', annotation: '基因注释', go: 'GO 富集', kegg: 'KEGG 富集', prior: '上游 TF（ReguTrace）' }[tk]!, { detail: 'Module detail', network: 'Intra-module (hub)', priority: 'Candidate priority', annotation: 'Annotation', go: 'GO', kegg: 'KEGG', prior: 'Upstream TFs (ReguTrace)' }[tk]!) }}
          </button>
        </div>
        <hr class="my-1" />
        <label class="form-label small mb-0">{{ text('性状', 'Trait') }}</label>
        <select v-model="store.draft.module_detail.select_trait" class="form-select form-select-sm">
          <option value="">{{ text('选择性状', 'Select trait') }}</option>
          <option v-for="tr in traitOptions" :key="tr" :value="tr">{{ tr }}</option>
        </select>
        <label class="form-label small mb-0">{{ text('模块', 'Module') }}</label>
        <select v-model="store.draft.module_detail.select_module" class="form-select form-select-sm">
          <option value="">{{ text('选择模块', 'Select module') }}</option>
          <option v-for="m in moduleOptions" :key="m" :value="m">{{ m }}</option>
        </select>
        <button class="btn btn-primary w-100" :disabled="!canRunDetail" @click="store.run('module_detail')">{{ text('运行模块详情', 'Run module detail') }}</button>
        <div v-if="invalidReasons.length" class="alert alert-warning py-1 px-2 small mb-0 mt-1">
          <i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('上游步骤需重跑：', 'Upstream steps need a re-run:') }}
          <span v-for="r in invalidReasons" :key="r.step" class="me-1">{{ r.step }}（{{ r.reason }}）</span>
        </div>

        <template v-if="tab === 'network'">
          <hr class="my-1" />
          <span class="text-muted small fw-semibold">{{ text('候选基因筛选', 'Hub gene filter') }}</span>
          <label class="form-label small mb-0">|kME| ≥ <b>{{ store.draft.hub.kme_cutoff }}</b></label>
          <input type="range" class="form-range" min="0" max="1" step="0.01" v-model.number="store.draft.hub.kme_cutoff" />
          <label class="form-label small mb-0">|GS| ≥ <b>{{ store.draft.hub.gs_cutoff }}</b></label>
          <input type="range" class="form-range" min="0" max="1" step="0.01" v-model.number="store.draft.hub.gs_cutoff" />
          <label class="form-label small mb-0">{{ text('边权重阈值', 'Weight threshold') }}: <b>{{ store.draft.hub.weight_threshold }}</b></label>
          <input type="range" class="form-range" min="0" max="0.5" step="0.005" v-model.number="store.draft.hub.weight_threshold" />
          <label class="form-label small mb-0">{{ text('最大候选基因数', 'Max hubs') }}<input type="number" class="form-control form-control-sm" min="1" max="200" v-model.number="store.draft.hub.max_hubs" /></label>
          <label class="form-label small mb-0">{{ text('布局', 'Layout') }}
            <select v-model="layout" class="form-select form-select-sm">
              <option value="fcose">ForceAtlas2 (Gephi)</option>
              <option value="cose">Fruchterman-Reingold</option>
              <option value="radial">Hub-and-spoke radial</option>
              <option value="circle">Circular</option>
              <option value="concentric">Concentric</option>
              <option value="breadthfirst">Hierarchy</option>
              <option value="ring">{{ text('环状（Hub 内圈）', 'Ring (hub inner)') }}</option>
              <option value="ringTopN">{{ text('环状 top-N（hdWGCNA）', 'Ring top-N (hdWGCNA)') }}</option>
            </select>
          </label>
          <template v-if="layout === 'ringTopN'">
            <label class="form-label small mb-0">{{ text('top-N 数量', 'Top-N genes') }}<input type="number" class="form-control form-control-sm" min="1" max="500" v-model.number="ringTopN" /></label>
            <label class="form-label small mb-0">{{ text('内圈数量', 'Inner count') }}<input type="number" class="form-control form-control-sm" min="1" max="100" v-model.number="ringInnerN" /></label>
            <div class="small text-muted mt-1">
              {{ text('灵感来自', 'Inspired by') }}
              <a href="https://smorabit.github.io/hdWGCNA/articles/network_visualizations.html" target="_blank" rel="noopener noreferrer">hdWGCNA</a>
              {{ text('（Morabito et al., Cell Reports Methods 2023）', '(Morabito et al., Cell Reports Methods 2023)') }}
            </div>
          </template>
          <label class="form-label small mb-0">{{ text('节点大小依据', 'Size by') }}
            <select v-model="sizeBasis" class="form-select form-select-sm">
              <option value="kme">kME</option><option value="degree">degree</option><option value="tomStrength" :disabled="!hasTomStrength">TOM strength</option><option value="kwithin" :disabled="!hasKwithin">kWithin</option><option value="gs">GS</option><option value="fixed">fixed</option>
            </select>
          </label>
          <label class="form-label small mb-0">{{ text('标签', 'Labels') }}
            <select v-model="labelMode" class="form-select form-select-sm">
              <option value="hub">{{ text('仅 hub', 'Hub only') }}</option><option value="all">{{ text('全部', 'All') }}</option><option value="none">{{ text('无', 'None') }}</option>
            </select>
          </label>
          <label class="form-label small mb-0">{{ text('节点着色', 'Node color') }}
            <select v-model="colorMode" class="form-select form-select-sm">
              <option value="kme">kME</option><option value="gs">GS</option><option value="degree">degree</option><option value="tomStrength" :disabled="!hasTomStrength">TOM strength</option><option value="kwithin" :disabled="!hasKwithin">kWithin</option><option value="hub">hub</option><option value="module">module</option>
            </select>
          </label>
          <details class="mt-1" open>
            <summary class="small fw-semibold">{{ text('节点样式（Node）', 'Node style') }}</summary>
            <div class="d-flex flex-column gap-1 mt-2">
              <div class="d-flex gap-2">
                <label class="small mb-0 flex-fill d-flex align-items-center gap-1">{{ text('节点低色', 'Node low') }}<input type="color" v-model="nodeLowColor" class="form-control form-control-color form-control-sm" /></label>
                <label class="small mb-0 flex-fill d-flex align-items-center gap-1">{{ text('节点高色', 'Node high') }}<input type="color" v-model="nodeHighColor" class="form-control form-control-color form-control-sm" /></label>
              </div>
              <div class="d-flex gap-2">
                <label class="small mb-0 flex-fill d-flex align-items-center gap-1">{{ text('Hub 填充色', 'Hub fill') }}<input type="color" v-model="hubNodeColor" class="form-control form-control-color form-control-sm" /></label>
                <label class="small mb-0 flex-fill d-flex align-items-center gap-1">{{ text('Hub 边框色', 'Hub border color') }}<input type="color" v-model="hubBorderColor" class="form-control form-control-color form-control-sm" /></label>
              </div>
              <label class="small mb-0">{{ text('节点大小范围', 'Node size range') }}: <b>{{ nodeSizeMin }}–{{ nodeSizeMax }}</b>
                <input type="range" class="form-range" min="1" max="40" v-model.number="nodeSizeMin" />
                <input type="range" class="form-range" min="4" max="80" v-model.number="nodeSizeMax" />
              </label>
              <label class="small mb-0 flex-fill d-flex align-items-center gap-1">{{ text('节点边框色', 'Node border color') }}<input type="color" v-model="nodeBorderColor" class="form-control form-control-color form-control-sm" /></label>
              <label class="small mb-0">{{ text('节点边框粗细', 'Node border width') }}: <b>{{ nodeBorderWidth }}</b>
                <input type="range" class="form-range" min="0" max="10" step="0.5" v-model.number="nodeBorderWidth" />
              </label>
              <label class="small mb-0">{{ text('标签字号', 'Label size') }}: <b>{{ labelSize }}</b>
                <input type="range" class="form-range" min="6" max="18" v-model.number="labelSize" />
              </label>
            </div>
          </details>

          <details class="mt-1" open>
            <summary class="small fw-semibold">{{ text('边样式（Edge）', 'Edge style') }}</summary>
            <div class="d-flex flex-column gap-1 mt-2">
              <div class="d-flex gap-2">
                <label class="small mb-0 flex-fill d-flex align-items-center gap-1">{{ text('边低色', 'Edge low') }}<input type="color" v-model="edgeLowColor" class="form-control form-control-color form-control-sm" /></label>
                <label class="small mb-0 flex-fill d-flex align-items-center gap-1">{{ text('边高色', 'Edge high') }}<input type="color" v-model="edgeHighColor" class="form-control form-control-color form-control-sm" /></label>
              </div>
              <label class="small mb-0">{{ text('边宽（最大）', 'Edge width (max)') }}: <b>{{ edgeWidth }}</b>
                <input type="range" class="form-range" min="0.5" max="12" step="0.5" v-model.number="edgeWidth" />
              </label>
            </div>
          </details>

          <button class="btn btn-primary w-100" :disabled="!canRunDetail" @click="syncHub(); store.run('hub')">{{ text('运行候选基因', 'Run hub') }}</button>
          <div v-if="hubStale" class="small text-warning mt-1"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('候选基因（hub）需重新运行（引擎已更新）', 'Hub needs a re-run (engine changed)') }}</div>
        </template>
      </div>
    </template>

    <div class="mb-3"><h4 class="mb-1">{{ text('模块解析', 'Module explorer') }}</h4><p class="text-muted small mb-0">{{ text('深入单个模块：GS/MM 散点、候选基因网络、注释与富集。', 'Explore one module: GS/MM scatter, hub network, annotation and enrichment.') }}</p></div>
    <div v-if="store.error" class="alert alert-danger">{{ store.error }}</div>

    <template v-if="tab === 'detail'">
      <div v-if="detail">
        <div class="d-flex gap-2 mb-2">
          <button class="btn btn-sm btn-outline-primary" @click="showExportGsmm = true"><i class="bi bi-download me-1"></i>{{ text('导出 GS/MM 图', 'Export GS/MM') }}</button>
          <button class="btn btn-sm btn-outline-primary" @click="showExportHeatmap = true"><i class="bi bi-download me-1"></i>{{ text('导出表达热图', 'Export heatmap') }}</button>
        </div>
        <div class="row g-3">
          <div class="col-lg-6"><GsMmScatter :data="detail" /></div>
          <div class="col-lg-6"><ModuleGeneHeatmap :data="detail" /></div>
        </div>
      </div>
      <p v-else class="text-muted">{{ text('选择模块与性状并运行模块详情，这里会显示 GS/MM 散点与表达热图。', 'Select a module and trait, then run module detail to see the GS/MM scatter and expression heatmap.') }}</p>
    </template>

    <template v-else-if="tab === 'network'">
      <div v-if="hub">
        <p class="text-muted small mb-1">{{ hub.module }} × {{ hub.trait_name }} · {{ hub.hub_genes.length }} {{ text('个候选基因', 'hub genes') }} · {{ text('模块内网络', 'intra-module') }}</p>
        <p class="text-muted small mb-1">
          {{ text('显示', 'Shown') }}：{{ displayNetwork?.nodes.length ?? 0 }} / {{ hub.network.nodes.length }} {{ text('节点', 'nodes') }}（{{ retention.nodePct }}%）
          · {{ displayNetwork?.edges.length ?? 0 }} / {{ hub.network.edges.length }} {{ text('边', 'edges') }}（{{ retention.edgePct }}%）
        </p>
        <p v-if="hub.raw_edge_count !== undefined" class="text-muted small mb-2">
          {{ text('Worker', 'Worker') }}：{{ hub.raw_edge_count }} {{ text('边', 'edges') }} / {{ hub.node_count ?? hub.network.nodes.length }} {{ text('节点', 'nodes') }}
          · {{ text('权重阈值', 'weight threshold') }} {{ hub.used_threshold ?? 0 }}
          · {{ hub.weight_type ?? 'TOM' }} ({{ hub.network_type ?? 'unsigned' }} · power {{ hub.power ?? '?' }})
          · TOM {{ hub.tom_scope ?? 'candidate_pool' }}（{{ hub.tom_pool_size ?? '?' }} {{ text('基因', 'genes') }}）
          · {{ hub.edge_set_completeness ?? 'thresholded' }}
        </p>
        <div class="d-flex align-items-center gap-2 flex-wrap mb-2">
          <label class="small mb-0 d-flex align-items-center gap-1">{{ text('显示模式', 'Display mode') }}
            <select v-model="displayMode" class="form-select form-select-sm" style="width:auto">
              <option value="sparse">{{ text('稀疏（默认）', 'Sparse (default)') }}</option>
              <option value="hub">{{ text('仅 Hub 边', 'Hub-centric') }}</option>
              <option value="full">{{ text('完整诱导子图', 'Full induced') }}</option>
            </select>
          </label>
          <label v-if="displayMode !== 'full'" class="small mb-0 d-flex align-items-center gap-1">{{ text('最多边数', 'Max edges') }}
            <input type="number" class="form-control form-control-sm" style="width:96px" v-model.number="maxEdges" min="10" max="20000" step="10" />
          </label>
          <label class="small mb-0 d-flex align-items-center gap-1">{{ text('最低边权', 'Min weight') }}
            <input type="number" class="form-control form-control-sm" style="width:96px" v-model.number="minEdgeWeight" min="0" max="1" step="0.01" />
          </label>
          <label class="small mb-0 d-flex align-items-center gap-1"><input type="checkbox" class="form-check-input" v-model="hideIsolated" />{{ text('隐藏孤立节点', 'Hide isolated') }}</label>
          <span class="text-muted small">{{ text('仅影响显示，不重算网络', 'Display only, no re-run') }}</span>
          <button class="btn btn-sm btn-outline-secondary ms-auto" @click="nodeInfoOpen = true"><i class="bi bi-table me-1"></i>{{ text('节点信息 / 标签', 'Node info & labels') }}</button>
        </div>
        <div v-if="displayMode === 'full' && (displayNetwork?.edges.length ?? 0) > 2000" class="alert alert-warning py-2 small"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('完整诱导子图边数较多，渲染可能较慢（不会静默截断）。', 'Full induced graph has many edges and may render slowly (no silent truncation).') }}</div>
        <div v-else-if="largeNetwork" class="alert alert-warning py-2 small"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('网络较大，可能较慢', 'Large network, may be slow') }}（{{ netNodeCount }} / {{ netEdgeCount }}）</div>
        <NetworkGraph
          :network="displayNetwork ?? hub.network"
          :layout="layout"
          :size-basis="sizeBasis"
          :node-size-min="nodeSizeMin"
          :node-size-max="nodeSizeMax"
          :edge-low-color="edgeLowColor"
          :edge-high-color="edgeHighColor"
          :edge-width="edgeWidth"
          @select="onSelect"
          @clear="onClear"
          :label-size="labelSize"
          :label-mode="labelMode"
          :gene-map="geneMap"
          :color-mode="colorMode"
          :node-low-color="nodeLowColor"
          :node-high-color="nodeHighColor"
          :hub-node-color="hubNodeColor"
          :hub-border-color="hubBorderColor"
          :node-border-color="nodeBorderColor"
          :node-border-width="nodeBorderWidth"
          :ring-top-n="ringTopN"
          :ring-inner-n="ringInnerN"
        />
        <div v-if="selectedElement" class="card p-3 mt-3 border-start border-4" :class="selectedElement.kind === 'edge' ? 'border-primary' : 'border-success'">
          <div class="d-flex align-items-center justify-content-between mb-1">
            <h6 class="mb-0">{{ selectedElement.kind === 'edge' ? text('连接信息', 'Edge details') : text('基因证据', 'Gene evidence') }}</h6>
            <button class="btn-close" @click="onClear"></button>
          </div>
          <template v-if="selectedElement.kind === 'node'">
            <div class="small">{{ text('基因', 'Gene') }}：<b>{{ selectedElement.data.label }}</b>（{{ selectedElement.data.id }}）</div>
            <div class="small">{{ text('模块', 'Module') }}：{{ selectedElement.data.color }}</div>
            <table class="table table-sm small mb-0 mt-1">
              <tbody>
                <tr><td>kME</td><td>{{ selectedElement.data.kme.toFixed(3) }}</td></tr>
                <tr><td>GS</td><td>{{ selectedElement.data.gs.toFixed(3) }}</td></tr>
                <tr><td>degree</td><td>{{ selectedElement.data.degree }}</td></tr>
                <tr v-if="selectedElement.data.tomStrength != null"><td>{{ text('TOM strength', 'TOM strength') }}</td><td>{{ selectedElement.data.tomStrength.toFixed(2) }}</td></tr>
                <tr v-if="selectedElement.data.kwithin != null"><td>{{ text('kWithin', 'kWithin') }}</td><td>{{ selectedElement.data.kwithin.toFixed(2) }}</td></tr>
                <tr><td>{{ text('科学 hub / 网络种子', 'hub / seed') }}</td><td>{{ selectedElement.data.isHubSci ? 'hub' : '—' }} / {{ selectedElement.data.isHub ? 'seed' : '—' }}</td></tr>
              </tbody>
            </table>
          </template>
          <template v-else>
            <div class="small">{{ text('连接', 'Edge') }}：<b>{{ selectedElement.data.source }}</b> ↔ <b>{{ selectedElement.data.target }}</b></div>
            <table class="table table-sm small mb-0 mt-1">
              <tbody>
                <tr><td>{{ text('连接强度（weight）', 'strength (weight)') }}</td><td>{{ selectedElement.data.weight.toFixed(4) }}</td></tr>
              </tbody>
            </table>
          </template>
        </div>
        <SimpleTable class="mt-3" :columns="[{ key: 'gene', label: text('基因', 'Gene') }, { key: 'kme', label: 'kME' }, { key: 'gs', label: 'GS' }]" :rows="hubRows" filename="hub_gene" />

      </div>
      <p v-else class="text-muted">{{ text('运行候选基因后，这里会显示该模块的候选基因网络。', 'Run hub to see the candidate gene network.') }}</p>
      <section v-if="hub || rean" class="mt-3">
        <button type="button" class="btn btn-link btn-sm text-secondary p-0" :aria-expanded="showEdgeStability" aria-controls="edge-stability" @click="showEdgeStability = !showEdgeStability">
          {{ text('边稳定性（可选）', 'Edge stability (optional)') }}
        </button>
        <div v-if="showEdgeStability" id="edge-stability" class="card p-3 mt-2">
          <p class="small text-muted mb-2">{{ text('对样本重采样，查看已选连接在有效重复中超过当前网络阈值的比例。', 'Resample the samples to see how often each selected edge exceeds the network threshold in valid runs.') }}</p>
          <div class="d-flex gap-2 align-items-end flex-wrap mb-2">
            <label class="form-label small mb-0">{{ text('重采样次数', 'Resamples') }}<input type="number" class="form-control form-control-sm" min="10" max="1000" v-model.number="store.draft.rean.n_bootstraps" /></label>
            <label class="form-label small mb-0">{{ text('随机种子', 'Seed') }}<input type="number" class="form-control form-control-sm" min="0" v-model.number="store.draft.rean.seed" /></label>
            <button type="button" class="btn btn-outline-secondary btn-sm" :disabled="!canRunRean" @click="store.run('rean')">{{ text('计算稳定性', 'Check stability') }}</button>
          </div>
          <p v-if="!canRunRean && !store.busy" class="small text-muted mb-2">{{ text('请先运行当前模块的候选基因网络。', 'Run the candidate gene network for this module first.') }}</p>
          <template v-if="rean">
            <p v-if="reanStale" class="small text-muted mb-1">{{ text('历史结果 · 参数或网络已变更。', 'Historical result · settings or network have changed.') }}</p>
            <p class="small text-muted mb-2">{{ rean.module }} · {{ text('TOM 阈值', 'TOM threshold') }} {{ rean.threshold }} · {{ text('有效重复', 'Valid runs') }} {{ rean.valid ?? '—' }} / {{ rean.attempted ?? '—' }}</p>
            <SimpleTable :columns="[{ key: 'source', label: text('基因 A', 'Gene A') }, { key: 'target', label: text('基因 B', 'Gene B') }, { key: 'stability', label: text('稳定性', 'Stability') }]" :rows="reanRows" filename="rean_edge_stability" scroll-height="360px" />
          </template>
        </div>
      </section>
    </template>

    <template v-else-if="tab === 'priority'">
      <p class="text-muted small mb-2">{{ text('对固定完整模块做 bootstrap 重采样，按 kWithin 排名稳定性给候选基因排序；不依赖性状或 hub。', 'Rank genes by kWithin ranking stability across bootstrap resamples of the fixed full module; does not require a trait or hub.') }}</p>
      <div class="card p-3 mb-3">
        <div class="row g-2">
          <div class="col-md-4">
            <label class="form-label small mb-0">{{ text('模块', 'Module') }}
              <select v-model="store.draft.hub_priority.select_module" class="form-select form-select-sm">
                <option value="">{{ text('选择模块', 'Select module') }}</option>
                <option v-for="m in moduleOptions" :key="m" :value="m">{{ m }}</option>
              </select>
            </label>
          </div>
          <div class="col-md-4">
            <label class="form-label small mb-0">{{ text('Bootstrap 次数', 'Bootstraps') }}<input type="number" class="form-control form-control-sm" min="10" max="1000" v-model.number="store.draft.hub_priority.n_bootstraps" /></label>
          </div>
          <div class="col-md-4">
            <label class="form-label small mb-0">{{ text('前 K 名', 'Top K') }}<input type="number" class="form-control form-control-sm" min="1" v-model.number="store.draft.hub_priority.top_k" /></label>
          </div>
          <div class="col-md-4">
            <label class="form-label small mb-0">{{ text('随机种子', 'Seed') }}<input type="number" class="form-control form-control-sm" min="0" v-model.number="store.draft.hub_priority.seed" /></label>
          </div>
          <div class="col-md-4">
            <label class="form-label small mb-0">{{ text('重采样方式', 'Resampling') }}
              <select v-model="store.draft.hub_priority.resampling" class="form-select form-select-sm">
                <option value="iid">iid</option>
                <option value="stratified">{{ text('分层', 'stratified') }}</option>
              </select>
            </label>
          </div>
          <div class="col-md-4" v-if="store.draft.hub_priority.resampling === 'stratified'">
            <label class="form-label small mb-0">{{ text('分层列', 'Strata column') }}
              <select v-model="store.draft.hub_priority.strata_column" class="form-select form-select-sm">
                <option value="">{{ text('选择样本信息列', 'Select sample-info column') }}</option>
                <option v-for="c in sampleColumns" :key="c" :value="c">{{ c }}</option>
              </select>
            </label>
          </div>
        </div>
        <div class="form-check mt-2">
          <input type="checkbox" class="form-check-input" id="hp-design-confirmed" v-model="store.draft.hub_priority.design_confirmed" />
          <label class="form-check-label small" for="hp-design-confirmed">{{ text('我确认样本相互独立（重复测量/配对设计尚未支持）', 'I confirm samples are independent (repeated-measures/paired designs not yet supported)') }}</label>
        </div>
        <div class="small text-muted mt-1">
          {{ text('工程预算', 'Engineering budget') }}：g²×B ≈ {{ hubPriorityCost.toLocaleString() }}（{{ text('模块', 'module') }} {{ hubPriorityModuleSize }} × B {{ store.draft.hub_priority.n_bootstraps }}）
          <span v-if="hubPriorityOverBudget" class="text-warning"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text(`超过 1e9 预算，worker 会拒绝；请把 B 降到 ≤ ${hubPriorityMaxBootstraps}（推荐 ≤100）或选择更小模块（不会自动截断基因）`, `Exceeds the 1e9 budget and will be rejected; reduce B to ≤ ${hubPriorityMaxBootstraps} (100 or less recommended) or choose a smaller module (genes are never silently truncated)`) }}</span>
          <button v-if="hubPriorityOverBudget && hubPrioritySuggestedB >= 10" class="btn btn-sm btn-outline-warning mt-1" @click="store.draft.hub_priority.n_bootstraps = hubPrioritySuggestedB">{{ text(`设为 B=${hubPrioritySuggestedB}`, `Set B=${hubPrioritySuggestedB}`) }}</button>
        </div>
        <button class="btn btn-primary mt-2" :disabled="!canRunHubPriority" @click="store.run('hub_priority')">{{ hubPriority ? text('重新计算优先级', 'Re-run priority') : text('计算候选优先级', 'Compute priority') }}</button>
        <div v-if="!store.draft.hub_priority.design_confirmed" class="small text-muted mt-1">{{ text('需先勾选独立性确认才能运行。', 'Tick the independence confirmation to run.') }}</div>
      </div>

      <div v-if="hubPriorityStale" class="alert alert-warning py-2 small"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('候选优先级结果已过期（参数/上游/引擎变化），请重新计算。', 'Priority result is stale (params/upstream/engine changed); re-run.') }}</div>
      <template v-else-if="hubPriority">
        <HubPriorityPlainSummary :result="hubPriority" :annotations="priorityAnnotationRows" />
        <details class="card p-3 mt-3">
          <summary class="fw-semibold">{{ text('技术明细（原始指标）', 'Technical details (raw metrics)') }}</summary>
          <div class="mt-3">
            <div v-if="priorityAllInvalid" class="alert alert-warning py-2 small"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('所有 bootstrap 轮均无效（网络退化），无可用排名；原始 kWithin/kME 仍可查看。', 'All bootstrap rounds were invalid (degenerate network); no usable ranking. Original kWithin/kME are still shown.') }}</div>
            <div v-else-if="(hubPriority.invalid ?? 0) > 0" class="alert alert-warning py-2 small"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ hubPriority.invalid }}/{{ hubPriority.attempted }} {{ text('轮无效，结果条件于可计算重复，不宣称可靠候选。', 'rounds invalid; results conditioned on computable replicates, not reliable candidates.') }}</div>
            <div v-if="(hubPriority.direction_undetermined ?? 0) > 0" class="alert alert-info py-2 small"><i class="bi bi-info-circle me-1"></i>{{ hubPriority.direction_undetermined }} {{ text('轮 eigengene 方向无法确定（仅描述性，不影响 kWithin 排名）。', 'rounds had undetermined eigengene direction (descriptive only; does not affect kWithin ranking).') }}</div>
            <p class="text-muted small mb-1">{{ hubPriority.module }} · {{ hubPriority.scope }} · {{ hubPriority.resampling }} · seed {{ hubPriority.seed }} · K {{ hubPriority.top_k }} · power {{ hubPriority.power }}（{{ hubPriority.network_type }}）</p>
            <p class="text-muted small mb-1">
              {{ text('有效', 'valid') }} {{ hubPriority.valid }}/{{ hubPriority.attempted }} · {{ text('无效', 'invalid') }} {{ hubPriority.invalid }}
              <template v-if="hubPriority.invalid_reasons?.length"> · {{ hubPriority.invalid_reasons.map((r) => `${r.reason}×${r.count}`).join('、') }}</template>
            </p>
            <p class="text-muted small mb-2">{{ text('定义：midrank = 并列组平均排名；top-K 频率 = 跨前 K 边界的并列按分数计入。是 bootstrap 采样频率，不是概率或显著性。', 'Definition: midrank = tie-group average rank; top-K frequency = fractional top-K membership across resamples. A bootstrap frequency, not a probability or significance.') }}</p>
            <div class="d-flex align-items-center gap-2 flex-wrap mb-2">
              <span class="small text-muted">{{ hubPriorityRows.length }} {{ text('个基因', 'genes') }}</span>
              <button class="btn btn-sm btn-outline-secondary" :disabled="priorityPage <= 0" @click="priorityPage--"><i class="bi bi-chevron-left"></i></button>
              <span class="small">{{ priorityPage + 1 }} / {{ priorityPageCount }}</span>
              <button class="btn btn-sm btn-outline-secondary" :disabled="priorityPage + 1 >= priorityPageCount" @click="priorityPage++"><i class="bi bi-chevron-right"></i></button>
              <label class="small mb-0 ms-auto d-flex align-items-center gap-1">{{ text('每页', 'Per page') }}
                <select v-model.number="priorityPageSize" class="form-select form-select-sm" style="width:auto"><option :value="25">25</option><option :value="50">50</option><option :value="100">100</option><option :value="500">500</option></select>
              </label>
              <a class="btn btn-sm btn-outline-primary" :href="projectOutputUrl(store.id, 'hub_priority', 'step8_hub_priority.tsv')" :title="text('原始 worker 指标，不含基因注释；带注释请用表格上方的下载按钮', 'Raw worker metrics without gene annotation; use the table download button for annotated CSV')"><i class="bi bi-download me-1"></i>{{ text('原始指标 TSV（无注释）', 'Raw metrics TSV (no annotation)') }}</a>
            </div>
            <p v-if="priorityAnnotationLoading" class="small text-muted mb-1">{{ text('正在加载基因注释…', 'Loading gene annotation…') }}</p>
            <p v-else-if="priorityAnnotationRows.length === 0" class="small text-muted mb-1">{{ text('未找到该模块的注释；Symbol/Description 显示为 —。', 'No annotation found for this module; Symbol/Description show —.') }}</p>
            <SimpleTable :columns="[
              { key: 'gene', label: 'Gene' },
              { key: 'symbol', label: text('符号/名称', 'Symbol / name') },
              { key: 'description', label: text('功能描述', 'Description') },
              { key: 'kwithin', label: 'kWithin' },
              { key: 'kme_signed', label: 'kME (signed)' },
              { key: 'kme_abs', label: '|kME|' },
              { key: 'rank', label: text('原始排名', 'Orig rank') },
              { key: 'midrank_median', label: 'midrank 中位数' },
              { key: 'midrank_p05', label: 'P05' },
              { key: 'midrank_p95', label: 'P95' },
              { key: 'mean_top_k', label: 'top-K 频率' },
            ]" :rows="priorityPagedRows" scroll-height="520px" />
          </div>
        </details>
      </template>
      <p v-else class="text-muted">{{ text('尚未运行：选择模块、勾选独立性确认并运行后，这里显示候选优先级表。', 'Not run: select a module, confirm independence, and run to see the candidate priority table.') }}</p>
    </template>

    <template v-else-if="tab === 'annotation'">
      <div v-if="annotationLoading" class="text-muted small">{{ text('加载中…', 'Loading…') }}</div>
      <SimpleTable v-else :columns="[
        { key: 'gene_id', label: 'Gene ID' },
        { key: 'symbol', label: 'Symbol' },
        { key: 'gene_name', label: 'Gene name' },
        { key: 'description', label: 'Description' },
      ]" :rows="annotationRows" scroll-height="560px" filename="module_annotation" />
    </template>

    <template v-else-if="tab === 'prior'">
      <div class="d-flex align-items-center flex-wrap gap-2 mb-2">
        <div>
          <h6 class="mb-1">{{ text('模块上游 TF（ReguTrace prior）', 'Upstream TFs for this module (ReguTrace prior)') }}</h6>
          <div class="small text-muted">
            {{ text('只查询当前模块基因；prior 来自 Data Import，独立于 WGCNA worker。', 'Queries only the current module genes; the prior comes from Data Import and is independent of the WGCNA worker.') }}
            <span v-if="selectedModuleId">· {{ selectedModuleId }} · {{ priorGeneCount.toLocaleString() }} {{ text('个基因', 'genes') }}</span>
          </div>
        </div>
        <span v-if="priorLoading" class="spinner-border spinner-border-sm ms-auto"></span>
      </div>
      <div v-if="priorError" class="alert alert-warning small">{{ priorError }}</div>
      <SimpleTable v-else-if="priorDisplayRows.length" :columns="[
        { key: 'tf_id', label: 'TF ID' },
        { key: 'tf_symbol', label: 'TF symbol' },
        { key: 'module_targets', label: text('模块内 target 数', 'Module targets') },
        { key: 'n_sites', label: text('位点数', 'Sites') },
        { key: 'best_p', label: 'best p' },
        { key: 'evidence_levels', label: text('证据级别', 'Evidence levels') },
      ]" :rows="priorDisplayRows" scroll-height="560px" :filename="'tf_target_upstream_' + (selectedModuleId || 'module')" />
      <p v-else class="text-muted small">
        {{ text('没有匹配结果。请确认 Data Import 已导入 TF–target prior，且模块基因 ID 与 prior 的 target_id 一致。', 'No matching rows. Confirm a TF–target prior was imported in Data Import and module gene IDs match prior target_id values.') }}
      </p>
    </template>

    <template v-else>
      <div class="d-flex align-items-center gap-2 flex-wrap mb-2">
        <label class="small mb-0">fdr <input type="number" class="form-control form-control-sm" style="width:80px" v-model.number="enrichFdr" step="0.01" min="0.01" max="1" /></label>
        <label class="small mb-0">top <input type="number" class="form-control form-control-sm" style="width:72px" v-model.number="enrichTop" min="0" max="1000" title="0 = all" /></label>
        <label class="small mb-0 d-flex align-items-center gap-1"><input type="checkbox" v-model="maxCountOn" class="form-check-input me-1" />{{ text('去掉大条目', 'Drop broad terms') }}</label>
        <button class="btn btn-sm btn-primary" :disabled="enrichLoading" @click="loadEnrich"><i class="bi bi-arrow-repeat me-1"></i>{{ text('运行富集', 'Run') }}</button>
        <button v-if="enrichResults.length" class="btn btn-sm btn-outline-primary ms-auto" @click="showExportEnrich = true"><i class="bi bi-download me-1"></i>{{ text('导出富集图', 'Export') }}</button>
        <div v-if="enrichType === 'go'" class="btn-group btn-group-sm">
          <button class="btn" :class="goOntology === 'ALL' ? 'btn-primary' : 'btn-outline-primary'" @click="goOntology = 'ALL'">ALL</button>
          <button class="btn" :class="goOntology === 'MF' ? 'btn-primary' : 'btn-outline-primary'" @click="goOntology = 'MF'">MF</button>
          <button class="btn" :class="goOntology === 'BP' ? 'btn-primary' : 'btn-outline-primary'" @click="goOntology = 'BP'">BP</button>
          <button class="btn" :class="goOntology === 'CC' ? 'btn-primary' : 'btn-outline-primary'" @click="goOntology = 'CC'">CC</button>
        </div>
      </div>
      <div v-if="enrichLoading" class="text-muted small">{{ text('运行中…', 'Running…') }}</div>
      <template v-else-if="enrichResults.length">
        <div class="row g-3">
          <div class="col-lg-7"><ResizableChart :option="enrichOption" @chart-click="onChartClick" /></div>
          <div class="col-lg-5"><SimpleTable :columns="[
            { key: 'id', label: 'ID' },
            { key: 'description', label: 'Description' },
            { key: 'gene_ratio', label: 'GeneRatio' },
            { key: 'bg_ratio', label: 'BgRatio' },
            { key: 'p_adjust', label: 'p.adjust' },
            { key: 'count', label: 'Count' },
          ]" :rows="enrichRows" filename="enrich" scroll-height="460px" @row-click="onEnrichRowClick" /></div>
        </div>
        <div v-if="selectedTerm" class="mt-3">
          <h6 class="fw-semibold">{{ selectedTerm.source }} · {{ selectedTerm.termId }} · {{ selectedTerm.description }} · {{ selectedTerm.genes.length }} {{ text('个基因', 'genes') }}</h6>
          <SimpleTable :columns="[
            { key: 'gene', label: text('基因', 'Gene') },
            { key: 'symbol', label: text('符号', 'Symbol') },
            { key: 'gene_name', label: text('基因名', 'Gene name') },
            { key: 'mean', label: text('平均表达', 'Mean expr') },
          ]" :rows="termGeneRows" scroll-height="320px" :filename="selectedTerm.termId + '_genes'" />
          <div v-if="termExprLoading" class="text-muted small my-2">{{ text('加载中…', 'Loading…') }}</div>
          <ResizableChart v-else-if="termExpr" class="mt-3" :option="termHeatOption" />
        </div>
      </template>
      <p v-else class="text-muted">{{ text('选择模块后点击运行富集。', 'Select a module and run enrichment.') }}</p>
      <PlotDownloadWidget kind="enrich" :type="enrichType" :module="selectedModuleId" :fdr="enrichFdr" :top="enrichTop" :ontology="goOntology" :show="showExportEnrich" @close="showExportEnrich = false" />
    </template>
    <PlotDownloadWidget kind="gsmm" :show="showExportGsmm" @close="showExportGsmm = false" />
    <PlotDownloadWidget kind="geneheatmap" :show="showExportHeatmap" @close="showExportHeatmap = false" />

    <div v-if="nodeInfoOpen" class="modal d-block" tabindex="-1" style="background: rgba(15,23,42,0.5)">
      <div class="modal-dialog modal-lg modal-dialog-scrollable">
        <div class="modal-content">
          <div class="modal-header py-2">
            <h6 class="modal-title">{{ text('节点信息与标签', 'Node info & labels') }}</h6>
            <button class="btn-close" @click="nodeInfoOpen = false"></button>
          </div>
          <div class="modal-body py-2">
            <p class="small text-muted mb-2">{{ text('第三列标注 hub；第四列可自定义显示标签（留空则回退 symbol/gene_name/gene_id）。', 'Column 3 marks hubs; column 4 sets a custom label (empty falls back to symbol/gene_name/gene_id).') }}</p>
            <table class="table table-sm table-hover align-middle mb-0">
              <thead><tr><th style="width:24%">GeneID</th><th style="width:20%">Symbol</th><th style="width:12%">{{ text('Hub', 'Hub') }}</th><th>{{ text('标签（可编辑）', 'Label (editable)') }}</th></tr></thead>
              <tbody>
                <tr v-for="r in nodeInfoRows" :key="r.gene_id">
                  <td class="small text-nowrap">{{ r.gene_id }}</td>
                  <td class="small text-nowrap">{{ r.symbol }}</td>
                  <td><span v-if="r.is_hub" class="badge text-bg-danger">hub</span></td>
                  <td><input class="form-control form-control-sm" :value="labelOverrides[r.gene_id] ?? ''" :placeholder="r.autoLabel" @input="onLabelInput(r.gene_id, $event)" /></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="modal-footer py-2">
            <button class="btn btn-sm btn-outline-secondary" @click="clearLabels">{{ text('清空自定义标签', 'Clear custom labels') }}</button>
            <button class="btn btn-sm btn-primary" @click="nodeInfoOpen = false">{{ text('完成', 'Done') }}</button>
          </div>
        </div>
      </div>
    </div>
  </SidebarDrawer>
</template>
