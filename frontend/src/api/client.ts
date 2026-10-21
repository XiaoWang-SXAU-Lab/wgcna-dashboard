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
  status: 'running' | 'done' | 'error'
  current_step: string
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
