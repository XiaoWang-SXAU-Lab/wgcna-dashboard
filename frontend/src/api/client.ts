// API client for the wgcna-dashboard backend.
export interface Progress {
  step: string
  status: string
  progress: number
  message: string
}

export interface Dendrogram {
  merge: [number, number][]
  height: number[]
  order: number[]
}

export interface FilterResult {
  n_genes_before: number
  n_genes_after_low: number
  n_genes_final: number
  n_samples_before: number
  n_samples: number
  samples: string[]
  sample_tree: Dendrogram
}

export interface ModulesResult {
  genes: string[]
  colors: string[]
  module_sizes: Record<string, number>
  dendrogram: Dendrogram
}

export interface JobStatus {
  id: string
  status: 'idle' | 'running' | 'done' | 'error' | 'cancelled'
  current_step: string
  done_steps: string[]
  progress?: Progress
  error?: string
  result?: ModulesResult
}

const BASE = '/api'

export async function createJob(expr: File, params: unknown, threads?: number): Promise<{ id: string }> {
  const fd = new FormData()
  fd.append('expr', expr)
  fd.append('params', JSON.stringify(params))
  if (threads) fd.append('threads', String(threads))
  const res = await fetch(`${BASE}/jobs`, { method: 'POST', body: fd })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getJob(id: string): Promise<JobStatus> {
  const res = await fetch(`${BASE}/jobs/${id}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function runStep(id: string, step: string, params?: unknown, threads?: number): Promise<void> {
  const fd = new FormData()
  if (params !== undefined) fd.append('params', JSON.stringify(params))
  if (threads) fd.append('threads', String(threads))
  const res = await fetch(`${BASE}/jobs/${id}/steps/${step}/run`, { method: 'POST', body: fd })
  if (!res.ok) throw new Error(await res.text())
}

export async function uploadTrait(id: string, trait: File): Promise<void> {
  const fd = new FormData()
  fd.append('trait', trait)
  const res = await fetch(`${BASE}/jobs/${id}/trait`, { method: 'POST', body: fd })
  if (!res.ok) throw new Error(await res.text())
}

export async function uploadMeta(id: string, files: Record<string, File | null>): Promise<void> {
  const fd = new FormData()
  for (const [name, file] of Object.entries(files)) {
    if (file) fd.append(name, file)
  }
  if ([...fd.entries()].length === 0) return
  const res = await fetch(`${BASE}/jobs/${id}/meta`, { method: 'POST', body: fd })
  if (!res.ok) throw new Error(await res.text())
}

export async function cancelJob(id: string): Promise<void> {
  await fetch(`${BASE}/jobs/${id}/cancel`, { method: 'POST' })
}

export interface MtaitResult {
  modules: string[]
  traits: string[]
  cor: number[][]
  p: number[][]
  eigengenes: number[][]
  genes: string[]
  kme: number[][]
}

export interface DetailResult {
  module: string
  trait_name: string
  genes: string[]
  gs: number[]
  mm: number[]
  expression: number[][]
  eigengene: number[]
  samples: string[]
}

export interface FitIndex {
  power: number
  rsq: number
  slope: number
  truncated_rsq: number
  mean_k: number
  median_k: number
  max_k: number
}

export interface SftResult {
  power_estimate: number | null
  power_source?: string // "estimate"（R² 达标）| "experience"（经验兜底）
  network_type: string
  fit_indices: FitIndex[]
  scale_free_check?: ScaleFreeCheck
}

export interface ScaleFreeCheck {
  power: number
  hist_mids: number[]
  hist_counts: number[]
  log_k: number[]
  log_p: number[]
  intercept: number
  slope: number
  adj_r2: number
}

export interface HubGene {
  gene: string
  kme: number
  gs: number
  // Standard kWithin (sum of adjacency over the full module); absent on older results.
  module_kwithin?: number
}

export interface NetNode {
  id: string
  color: string
  kme: number
  gs: number
  degree: number
  // Sum of non-diagonal TOM weights in the extracted node set (absent on older
  // results). NOT the standard full-module kWithin.
  extracted_tom_strength?: number
  // Legacy alias (pre-rename); prefer extracted_tom_strength.
  kwithin?: number
  // Standard kWithin (sum of adjacency over the full module); absent on older results.
  module_kwithin?: number
  // Scientific hub (passes the |kME| / conditional |GS| 判定).
  is_hub: boolean
  // Network seed (top max_hubs); the drawn network is built around these.
  is_seed?: boolean
}

export interface NetEdge {
  source: string
  target: string
  weight: number
  // R-EAN edge_stability (absent when R-EAN has not run).
  stability?: number
}

export interface NetworkResult {
  nodes: NetNode[]
  edges: NetEdge[]
}

export interface HubResult {
  module: string
  trait_name: string
  hub_genes: HubGene[]
  network: NetworkResult
  // Worker-emitted metadata (absent on older result files).
  raw_edge_count?: number
  node_count?: number
  used_threshold?: number
  tom_scope?: string
  tom_pool_size?: number
  weight_type?: string
  network_type?: string
  power?: number
  edge_set_completeness?: string
}

export interface ReanEdge {
  source: string
  target: string
  pass_count?: number
  // null when there are no valid network replicates (denominator = valid).
  stability: number | null
}

export interface ReanResult {
  module: string
  n_bootstraps: number
  attempted?: number
  valid?: number
  invalid?: number
  seed: number
  threshold: number
  power: number
  network_type?: string
  tom_type?: string
  tom_denom?: string
  scope: string
  edges: ReanEdge[]
}

export interface HubPriorityGene {
  gene: string
  kwithin: number
  kme_signed: number
  kme_abs: number
  rank_lower: number
  rank_upper: number
  midrank_median: number | null
  midrank_p05: number | null
  midrank_p95: number | null
  mean_top_k: number | null
}

export interface HubPriorityInvalidReason {
  reason: string
  count: number
}

export interface HubPriorityResult {
  module: string
  method_version: string
  scope: string
  n_bootstraps: number
  attempted: number
  valid: number
  invalid: number
  invalid_reasons: HubPriorityInvalidReason[]
  seed: number
  top_k: number
  resampling: string
  tie_tol: number
  design_confirmed: boolean
  trait_source: string
  power: number
  network_type: string
  gene_count: number
  sample_count: number
  limited: boolean
  direction_undetermined: number
  strata_column: string
  stratum_count: number
  strata_counts: number[]
  strata_fingerprint: string
  gene_fingerprint: string
  sample_fingerprint: string
  resource_note: string
  genes: HubPriorityGene[]
}

// Report envelope returned by the candidate-priority report endpoint. It carries the
// report's OWN provenance (project/execution/upstream/inputs/params) so the frontend
// can verify it belongs to the captured execution instead of guessing from live state.
export interface CandidateReportEnvelope {
  project_id: string
  step: string
  execution_id: string
  engine: string
  params: Record<string, unknown>
  inputs: Record<string, string>
  upstream: Record<string, string>
  validity: string
  result: HubPriorityResult
}

export interface EvidenceSuggestion {
  project_id: string
  execution_id: string
  provenance: string
  gene_ids: string[]
  fact_ids: string[]
  explanations: string[]
  limitations: string[]
  plan_preview: string[]
  claims?: Record<string, unknown>
}

// Server-rebuilt evidence bundle summary + deterministic offline suggestion.
// The backend rebuilds from the current atomic report envelope; this response
// never contains a client-supplied bundle or a Diagnostic string.
export interface HubPriorityEvidence {
  project_id: string
  execution_id: string
  engine: string
  provenance: string
  valid: number
  invalid: number
  gene_count: number
  full_coverage: boolean
  candidate_count: number
  fact_count: number
  summary: string
  limitations: string[]
  offline: boolean
  suggestion: EvidenceSuggestion
  rendered: string
}

export async function getResult<T>(id: string, step: string): Promise<T> {
  const res = await fetch(`${BASE}/jobs/${id}/results/${step}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export interface AnnotationRow {
  gene_id: string
  gene_name: string
  symbol: string
  description: string
}

export interface EnrichResult {
  id: string
  description: string
  ontology: string
  gene_ratio: string
  bg_ratio: string
  pvalue: number
  'p.adjust': number
  qvalue: number
  gene_id: string[]
  count: number
}

export async function getAnnotation(id: string, module: string): Promise<AnnotationRow[]> {
  const res = await fetch(`${BASE}/jobs/${id}/annotation?module=${encodeURIComponent(module)}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getEnrich(
  id: string,
  module: string,
  type: 'go' | 'kegg',
  fdr = 0.05,
  top = 0,
  maxCount = 500,
): Promise<EnrichResult[]> {
  const res = await fetch(
    `${BASE}/jobs/${id}/enrich?module=${encodeURIComponent(module)}&type=${type}&fdr=${fdr}&top=${top}&max_count=${maxCount}`,
  )
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export interface ExpressionRow {
  gene: string
  values: number[]
  symbol: string
  gene_name: string
  description: string
  mean: number
}

export interface ExpressionResult {
  samples: string[]
  rows: ExpressionRow[]
}

export async function getExpression(id: string, genes: string[]): Promise<ExpressionResult> {
  const res = await fetch(`${BASE}/jobs/${id}/expression?genes=${encodeURIComponent(genes.join(','))}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export interface SampleInfo {
  sample_id: string
  attrs: Record<string, string>
}

export async function getSampleInfo(id: string): Promise<SampleInfo[]> {
  const res = await fetch(`${BASE}/jobs/${id}/sample-info`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function loadDemoApi(threads?: number): Promise<{ id: string }> {
  const q = threads ? `?threads=${threads}` : ''
  const res = await fetch(`${BASE}/demo/load${q}`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export interface GOVersionInfo {
  current: string
  latest: string
  update_available: boolean
  error?: string
}

export async function getGOVersion(): Promise<GOVersionInfo> {
  const res = await fetch(`${BASE}/meta/go-version`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function updateGO(): Promise<{ version: string }> {
  const res = await fetch(`${BASE}/meta/go-update`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

// ---- AI navigator ----

export interface AIConfig {
  provider: string
  base_url: string
  model: string
  configured: boolean
}

export async function getAIConfig(): Promise<AIConfig> {
  const res = await fetch(`${BASE}/ai/config`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function saveAIConfig(cfg: {
  provider: string
  base_url?: string
  model?: string
  api_key?: string
}): Promise<void> {
  const res = await fetch(`${BASE}/ai/config`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(cfg),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function chatStream(
  question: string,
  errorCtx: string,
  onDelta: (delta: string) => void,
  ctx?: { jobId?: string; module?: string },
): Promise<void> {
  const res = await fetch(`${BASE}/ai/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ question, error: errorCtx, job_id: ctx?.jobId ?? '', module: ctx?.module ?? '' }),
  })
  if (!res.ok) throw new Error(await res.text())
  const reader = res.body!.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    const parts = buf.split('\n\n')
    buf = parts.pop() ?? ''
    for (const part of parts) {
      if (!part.startsWith('data: ')) continue
      const msg = JSON.parse(part.slice(6))
      if (msg.delta) onDelta(msg.delta)
      if (msg.error) throw new Error(msg.error)
    }
  }
}

// plotUrl builds a backend-rendered plot URL.
// kind: 'sft' | 'dendrogram'; params: e.g. { mode:'pdf', size:2.4, ... }.
export function plotUrl(id: string, kind: string, params: Record<string, string | number> = {}): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) q.set(k, String(v))
  return `${BASE}/jobs/${id}/plots/${kind}?${q}`
}

// ---- module overview ----

export interface OverviewData {
  genes: string[]
  samples: string[]
  z: number[][]
  modules: string[]
  module_order: string[]
  eigengenes: Record<string, number[]>
  kme: Record<string, number>
  enrich: Record<string, { term: string; padj: number }[]>
  module_sizes: Record<string, number>
  enrich_type: string
  enrich_ok: boolean
}

export interface OverviewNetwork {
  nodes: { id: string; symbol: string; color: string; degree: number; kme: number; k_within: number; k_cross: number; cross_degree: number }[]
  edges: { source: string; target: string; weight: number; type: 'intra' | 'inter' }[]
  mode: string
  weight_type: string
}

export interface ClusterResult {
  genes: string[]
  samples: string[]
  clusters: string[]
  cluster_order: string[]
  centers: Record<string, number[]>
  score: number[]
  method: string
  k: number
  degenerate?: boolean
  metrics?: { ks: number[]; wss: number[]; xb: number[]; pc: number[] }
}

export async function getOverview(id: string, type: 'go' | 'kegg' = 'go', onto = '', fdr = 0.05, top = 5): Promise<OverviewData> {
  const res = await fetch(`${BASE}/jobs/${id}/overview?type=${type}&onto=${onto}&fdr=${fdr}&top=${top}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getOverviewNetwork(
  id: string,
  mode: 'intra' | 'inter' | 'combined' = 'intra',
  maxNodes = 2000,
  minPerModule = 20,
  intraPerGene = 8,
  interPerPair = 20,
  interBudget = 300,
  hideGrey = true,
): Promise<OverviewNetwork> {
  const res = await fetch(`${BASE}/jobs/${id}/overview/network?mode=${mode}&max_nodes=${maxNodes}&min_per_module=${minPerModule}&intra_per_gene=${intraPerGene}&inter_per_pair=${interPerPair}&inter_budget=${interBudget}&hide_grey=${hideGrey}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function runOverviewNetwork(id: string, percentile = 10): Promise<void> {
  const res = await fetch(`${BASE}/jobs/${id}/overview/network?percentile=${percentile}`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
}

export async function runCluster(id: string, method: 'kmeans' | 'cmeans', k = 0, seed = 42, kmax = 12): Promise<void> {
  const res = await fetch(`${BASE}/jobs/${id}/overview/cluster?method=${method}&k=${k}&seed=${seed}&kmax=${kmax}`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
}

export async function getCluster(id: string): Promise<ClusterResult> {
  const res = await fetch(`${BASE}/jobs/${id}/overview/cluster`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export interface ClusterElbow {
  method: string
  ks: number[]
  wss: number[]
  xb: number[]
  pc: number[]
  suggested_k: number
}

export async function runClusterElbow(id: string, method: 'kmeans' | 'cmeans', seed = 42, kmax = 12): Promise<void> {
  const res = await fetch(`${BASE}/jobs/${id}/overview/elbow?method=${method}&seed=${seed}&kmax=${kmax}`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
}

export async function getClusterElbow(id: string): Promise<ClusterElbow> {
  const res = await fetch(`${BASE}/jobs/${id}/overview/elbow`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export function overviewExportUrl(id: string, mode: 'modules' | 'clusters'): string {
  return `${BASE}/jobs/${id}/overview/export.xlsx?mode=${mode}`
}

export interface ReportStatus {
  state: 'running' | 'done' | 'error'
  message: string
  dir: string
  done: string[]
  warnings?: string[]
  total?: number
  preview_url?: string
}

export async function exportReport(id: string, dir: string, name: string, type: 'go' | 'kegg' = 'go', lang = 'zh'): Promise<{ dir: string }> {
  const fd = new FormData()
  fd.append('dir', dir)
  if (name) fd.append('name', name)
  fd.append('type', type)
  fd.append('lang', lang)
  const res = await fetch(`${BASE}/jobs/${id}/report/export`, { method: 'POST', body: fd })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getReportStatus(id: string): Promise<ReportStatus> {
  const res = await fetch(`${BASE}/jobs/${id}/report/status`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function pickFolder(): Promise<string> {
  const res = await fetch(`${BASE}/folder-picker`)
  if (!res.ok) throw new Error(await res.text())
  const d = await res.json()
  return d.dir
}

// ---- toolkits ----

export interface EggnogConvertResult {
  kingdom: string
  stats: { genes: number; go_count: number; ko_count: number; ko_filtered: number; go_terms: number; ko_terms: number }
  annotation: string
  gene2go: string
  gene2kegg: string
}

export async function convertEggnog(file: File, kingdom: string): Promise<EggnogConvertResult> {
  const fd = new FormData()
  fd.append('file', file)
  fd.append('kingdom', kingdom)
  const res = await fetch(`${BASE}/toolkits/eggnog`, { method: 'POST', body: fd })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export interface BlastMergeResult {
  stats: { genes: number; from_blast: number; from_base: number; blast_hits: number }
  annotation: string
}

export async function blastMerge(blast: File, reference: File, base: File | null, maxEvalue: number, minPident: number): Promise<BlastMergeResult> {
  const fd = new FormData()
  fd.append('blast', blast)
  fd.append('reference', reference)
  if (base) fd.append('base', base)
  fd.append('max_evalue', String(maxEvalue))
  fd.append('min_pident', String(minPident))
  const res = await fetch(`${BASE}/toolkits/blast-merge`, { method: 'POST', body: fd })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

// ---- ReguTrace toolkit ----

export interface RegutraceLibraryInfo {
  key: string
  name: string
  count: number
}

export interface RegutraceInfo {
  motif_library: string
  motif_count: number
  libraries: RegutraceLibraryInfo[]
}

export interface RegutraceParams {
  promoter_len: number
  downstream: number
  pvalue: number
  background: [number, number, number, number]
  threads: number
}

export interface RegutraceInputs {
  genome: string
  gff: string
  motifs: string
  motif_library_key?: string
  tf_map?: string
  annotation?: string
  annotation_source: string
  genome_bytes: number
  gff_bytes: number
}

export interface RegutraceDiagnostic {
  severity: 'error' | 'warning' | 'info'
  code: string
  message: string
  file?: string
  line?: number
  context?: Record<string, string>
  suggestion?: string
}

export interface RegutraceReport {
  tool: string
  version: string
  command: string
  inputs: Record<string, string>
  summary: { errors: number; warnings: number; infos: number }
  diagnostics: RegutraceDiagnostic[]
}

export interface RegutraceCountItem {
  id: string
  name: string
  targets: number
  hits: number
  edges: number
}

export interface RegutraceSummary {
  genes: number
  promoters: number
  motif_library_size: number
  motifs_with_hits: number
  motif_hits: number
  tf_target_pairs: number
  tf_target_edges: number
  unique_tfs: number
  unique_targets: number
  unique_motifs: number
  evidence_level_d: number
  threads?: number
  annotation_source: string
  top_tfs: RegutraceCountItem[]
  top_motifs: RegutraceCountItem[]
}

export interface RegutraceJob {
  id: string
  status: 'uploaded' | 'validating' | 'validated' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  stage: string
  message: string
  progress: number
  error?: string
  created_at: string
  updated_at: string
  params: RegutraceParams
  inputs: RegutraceInputs
  diagnostics?: RegutraceReport
  summary?: RegutraceSummary
}

export interface RegutraceQueryRow {
  tf_id: string
  tf_symbol: string
  tf_family?: string
  target_id: string
  target_symbol: string
  target_description?: string
  annotation_source: string
  evidence_type?: string
  evidence_level?: string
  method?: string
  source?: string
  pmid?: string
  condition?: string
  promoter_start?: number
  promoter_end?: number
  motif_id?: string
  motif_name?: string
  consensus?: string
  hit_start?: number
  hit_end?: number
  strand?: string
  score?: number
  p_value?: number
  notes?: string
}

export interface RegutraceQueryResult {
  total: number
  limit: number
  offset: number
  rows: RegutraceQueryRow[]
}

export interface RegutraceTFInfo {
  id: string
  symbol: string
  family?: string
}

export interface RegutracePromoterHit {
  motif_id: string
  motif_name?: string
  consensus?: string
  start: number
  end: number
  genomic_start?: number
  genomic_end?: number
  strand: string
  score?: number
  p_value?: number
  site: string
  matched_site: string
  tfs: RegutraceTFInfo[]
}

export interface RegutracePromoterView {
  target_id: string
  target_symbol: string
  target_description?: string
  annotation_source: string
  chrom?: string
  start?: number
  end?: number
  strand?: string
  length: number
  sequence: string
  tss_position: number
  hits: RegutracePromoterHit[]
}

export async function getRegutraceInfo(): Promise<RegutraceInfo> {
  const res = await fetch(`${BASE}/toolkits/regutrace/info`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export function createRegutraceJob(
  files: { genome: File; gff: File; motifs?: File | null; motifLibrary?: string; tfMap?: File | null; annotation?: File | null },
  onProgress?: (fraction: number) => void,
): Promise<RegutraceJob> {
  return new Promise((resolve, reject) => {
    const fd = new FormData()
    fd.append('genome', files.genome)
    fd.append('gff', files.gff)
    if (files.motifs) fd.append('motifs', files.motifs)
    if (files.motifLibrary) fd.append('motif_library', files.motifLibrary)
    if (files.tfMap) fd.append('tf_map', files.tfMap)
    if (files.annotation) fd.append('annotation', files.annotation)
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${BASE}/toolkits/regutrace/jobs`)
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) onProgress(e.loaded / e.total)
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try { resolve(JSON.parse(xhr.responseText)) } catch (e) { reject(e) }
      } else {
        reject(new Error(xhr.responseText || `HTTP ${xhr.status}`))
      }
    }
    xhr.onerror = () => reject(new Error('upload failed'))
    xhr.send(fd)
  })
}

export async function startRegutraceJob(id: string, mode: 'validate' | 'run', params: RegutraceParams): Promise<RegutraceJob> {
  const res = await fetch(`${BASE}/toolkits/regutrace/jobs/${id}/${mode}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getRegutraceJob(id: string): Promise<RegutraceJob> {
  const res = await fetch(`${BASE}/toolkits/regutrace/jobs/${id}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function cancelRegutraceJob(id: string): Promise<void> {
  const res = await fetch(`${BASE}/toolkits/regutrace/jobs/${id}/cancel`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
}

export async function queryRegutrace(
  id: string,
  opts: { direction: 'tf' | 'target'; q: string; motif?: string; maxP?: number; limit?: number; offset?: number },
): Promise<RegutraceQueryResult> {
  const p = new URLSearchParams()
  p.set('direction', opts.direction)
  p.set('q', opts.q)
  if (opts.motif) p.set('motif', opts.motif)
  if (opts.maxP !== undefined) p.set('max_p', String(opts.maxP))
  if (opts.limit !== undefined) p.set('limit', String(opts.limit))
  if (opts.offset !== undefined) p.set('offset', String(opts.offset))
  const res = await fetch(`${BASE}/toolkits/regutrace/jobs/${id}/query?${p}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function getRegutracePromoter(id: string, target: string): Promise<RegutracePromoterView> {
  const res = await fetch(`${BASE}/toolkits/regutrace/jobs/${id}/promoter?target=${encodeURIComponent(target)}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export function regutraceExportUrl(id: string, kind: 'evidence' | 'report' | 'promoters' | 'log'): string {
  return `${BASE}/toolkits/regutrace/jobs/${id}/export?kind=${kind}`
}
