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

export async function runStep(id: string, step: string): Promise<void> {
  const res = await fetch(`${BASE}/jobs/${id}/steps/${step}/run`, { method: 'POST' })
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
}

export async function getResult<T>(id: string, step: string): Promise<T> {
  const res = await fetch(`${BASE}/jobs/${id}/results/${step}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
