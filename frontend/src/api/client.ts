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

export async function runAll(id: string): Promise<void> {
  const res = await fetch(`${BASE}/jobs/${id}/run-all`, { method: 'POST' })
  if (!res.ok) throw new Error(await res.text())
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
}

export interface NetNode {
  id: string
  color: string
  kme: number
  gs: number
  degree: number
  is_hub: boolean
}

export interface NetEdge {
  source: string
  target: string
  weight: number
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
  top = 20,
): Promise<EnrichResult[]> {
  const res = await fetch(
    `${BASE}/jobs/${id}/enrich?module=${encodeURIComponent(module)}&type=${type}&fdr=${fdr}&top=${top}`,
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
): Promise<void> {
  const res = await fetch(`${BASE}/ai/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ question, error: errorCtx }),
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
