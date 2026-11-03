import type { AnnotationRow, CandidateReportEnvelope, EnrichResult, ExpressionResult, HubPriorityResult, SampleInfo } from './client'

export type StepName = 'filter' | 'soft_threshold' | 'network' | 'module_trait' | 'module_detail' | 'hub' | 'rean' | 'hub_priority'
export const steps: StepName[] = ['filter', 'soft_threshold', 'network', 'module_trait', 'module_detail', 'hub', 'rean', 'hub_priority']
export type Params = Record<StepName, Record<string, string | number | boolean | string[]>>
export interface QuestionCard { question: string; hypothesis?: string; organism?: string; design?: string; limitations?: string }
export interface ProjectState {
  id: string; name: string; revision: number; updated_at: string
  inputs: Record<string, { path: string; sha256: string }>
  draft_params: Partial<Params>; ui_state: { step?: StepName; threads?: number }; question_card: QuestionCard
  steps: Partial<Record<StepName, { execution_id: string; status: string; params: Record<string, unknown>; error?: string }>>
}
export interface RunProgress { step: string; status: string; progress: number; message: string }
export interface ProjectResponse { state: ProjectState; validity: Record<StepName, string>; path: string; busy: boolean; error: string; progress: RunProgress }
export interface RecentProject { id: string; name: string; path: string }

export async function projectRequest<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const multipart = body instanceof FormData
  const res = await fetch('/api/projects' + path, { method, headers: body && !multipart ? { 'Content-Type': 'application/json' } : undefined, body: body ? multipart ? body : JSON.stringify(body) : undefined })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export function defaultParams(): Params {
  return {
    filter: { data_type: 'count', normalize_method: 'vst', n_keep: 8000, filter_method: 'mad', sample_percentage: 0, cutoff: 1, exclude_samples: [] },
    soft_threshold: { network_type: 'unsigned', rsquared_cut: 0.85 },
    network: { power: 6, network_type: 'unsigned', tom_type: 'signed', tom_denom: 'min', min_module_size: 30, merge_cut_height: 0.25, deep_split: 2, detect_cut_height: 0.995 },
    module_trait: { trait_source: 'uploaded', trait_column: '' },
    module_detail: { select_module: '', select_trait: '' },
    hub: { select_module: '', select_trait: '', kme_cutoff: 0.8, gs_cutoff: 0, weight_threshold: 0.02, power: 6, top_n_neighbors: 20, max_hubs: 30 },
    rean: { select_module: '', n_bootstraps: 100, seed: 42 },
    hub_priority: { select_module: '', n_bootstraps: 200, seed: 42, top_k: 10, resampling: 'iid', strata_column: '', design_confirmed: false, trait_source: 'none' },
  }
}

export async function getSampleInfo(id: string): Promise<SampleInfo[]> {
  return projectRequest<SampleInfo[]>(`/${id}/sample-info`)
}

export function projectPlotUrl(id: string, kind: string, params: Record<string, string | number> = {}): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) q.set(k, String(v))
  return `/api/projects/${id}/plots/${kind}?${q}`
}

// Read-only committed output download (exact (step, basename) whitelist on the backend).
export function projectOutputUrl(id: string, step: StepName, basename: string): string {
  return `/api/projects/${id}/outputs/${step}/${encodeURIComponent(basename)}`
}

// Candidate-priority evidence report: server re-validates the current execution
// (409 with a reason when stale/not-run), then returns the provenance envelope.
export async function getCandidateReport(id: string): Promise<CandidateReportEnvelope> {
  return projectRequest<CandidateReportEnvelope>(`/${id}/report/hub-priority`)
}

export async function getAnnotation(id: string, module: string): Promise<AnnotationRow[]> {
  return projectRequest<AnnotationRow[]>(`/${id}/annotation?module=${encodeURIComponent(module)}`)
}

export async function getEnrich(id: string, module: string, type: 'go' | 'kegg', fdr = 0.05, top = 0, maxCount = 500): Promise<EnrichResult[]> {
  return projectRequest<EnrichResult[]>(`/${id}/enrich?module=${encodeURIComponent(module)}&type=${type}&fdr=${fdr}&top=${top}&max_count=${maxCount}`)
}

export async function getExpression(id: string, genes: string[]): Promise<ExpressionResult> {
  return projectRequest<ExpressionResult>(`/${id}/expression?genes=${encodeURIComponent(genes.join(','))}`)
}
