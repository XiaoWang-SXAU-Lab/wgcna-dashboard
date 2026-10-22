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

export async function createJob(expr: File, params: unknown): Promise<{ id: string }> {
  const fd = new FormData()
  fd.append('expr', expr)
  fd.append('params', JSON.stringify(params))
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

export async function runStep(id: string, step: string, params?: unknown): Promise<void> {
  const fd = new FormData()
  if (params !== undefined) fd.append('params', JSON.stringify(params))
  const res = await fetch(`${BASE}/jobs/${id}/steps/${step}/run`, { method: 'POST', body: fd })
  if (!res.ok) throw new Error(await res.text())
}

export async function uploadTrait(id: string, trait: File): Promise<void> {
  const fd = new FormData()
  fd.append('trait', trait)
  const res = await fetch(`${BASE}/jobs/${id}/trait`, { method: 'POST', body: fd })
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
}

export interface DetailResult {
  module: string
  trait_name: string
  genes: string[]
  gs: number[]
  mm: number[]
  expression: number[][]
  eigengene: number[]
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
  network_type: string
  fit_indices: FitIndex[]
}

export interface HubGene {
  gene: string
  kme: number
  gs: number
}

export interface HubResult {
  module: string
  trait_name: string
  hub_genes: HubGene[]
}

export async function getResult<T>(id: string, step: string): Promise<T> {
  const res = await fetch(`${BASE}/jobs/${id}/results/${step}`)
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

// plotUrl builds a download URL for a backend-rendered plot.
// kind: 'sft' | 'dendrogram'; mode: 'pdf' | 'svg' | 'png' | 'interactive'.
export function plotUrl(id: string, kind: string, mode: string, rsqCut?: number): string {
  const params = new URLSearchParams({ mode })
  if (rsqCut != null) params.set('rsq_cut', String(rsqCut))
  return `${BASE}/jobs/${id}/plots/${kind}?${params}`
}
