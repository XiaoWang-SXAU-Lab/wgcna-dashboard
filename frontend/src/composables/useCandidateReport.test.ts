// Actual production report-state tests: import the REAL useCandidateReport + real
// useProjectStore, mocking only the getCandidateReport transport.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { CandidateReportEnvelope } from '../api/client'
import type { ProjectResponse } from '../api/projects'

const { getCandidateReportMock } = vi.hoisted(() => ({ getCandidateReportMock: vi.fn() }))

vi.mock('../api/projects', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/projects')>()
  return { ...actual, getCandidateReport: getCandidateReportMock }
})

import { useCandidateReport } from './useCandidateReport'
import { useProjectStore } from '../stores/project'

function envelope(projectId: string, execId: string): CandidateReportEnvelope {
  return {
    project_id: projectId, step: 'hub_priority', execution_id: execId, engine: 'e',
    params: {}, inputs: {}, upstream: { filter: 'f1', network: 'n1' }, validity: '',
    result: {
      module: 'blue', method_version: 'v1', scope: 'fixed_full_module', n_bootstraps: 200,
      attempted: 200, valid: 200, invalid: 0, invalid_reasons: [], seed: 42, top_k: 10,
      resampling: 'iid', tie_tol: 1e-9, design_confirmed: true, trait_source: 'none',
      power: 6, network_type: 'unsigned', gene_count: 4, sample_count: 6, limited: false,
      direction_undetermined: 0, strata_column: '', stratum_count: 0, strata_counts: [],
      strata_fingerprint: '', gene_fingerprint: '', sample_fingerprint: '', resource_note: '',
      genes: [{ gene: 'g1', kwithin: 1, kme_signed: 0.5, kme_abs: 0.5, rank_lower: 1, rank_upper: 1, midrank_median: 1, midrank_p05: 1, midrank_p95: 1, mean_top_k: 1 }],
    },
  }
}

function storeState(id: string, execId: string): ProjectResponse {
  return {
    state: {
      id, name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {},
      question_card: { question: '' }, steps: { hub_priority: { execution_id: execId, status: 'done', params: {} } },
    },
    validity: { hub_priority: '', filter: '', soft_threshold: '', network: '', module_trait: '', module_detail: '', hub: '', rean: '' } as ProjectResponse['validity'],
    path: '/p', busy: false, error: '', progress: { step: '', status: '', progress: 0, message: '' },
  }
}

describe('useCandidateReport', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getCandidateReportMock.mockReset()
  })

  it('is current only when its identity matches the live execution', async () => {
    const store = useProjectStore()
    store.current = storeState('p1', 'e1')
    store.resultMeta = { hub_priority: 'e1' }
    getCandidateReportMock.mockResolvedValue(envelope('p1', 'e1'))
    const { report, reportCurrent, load } = useCandidateReport()
    await load()
    expect(report.value?.execution_id).toBe('e1')
    expect(reportCurrent.value).toBe(true)
    // a later execution invalidates the report (it becomes history, not current).
    store.current = storeState('p1', 'e2')
    store.resultMeta = { hub_priority: 'e2' }
    expect(reportCurrent.value).toBe(false)
  })

  it('switching projects clears the report', async () => {
    const store = useProjectStore()
    store.current = storeState('p1', 'e1')
    store.resultMeta = { hub_priority: 'e1' }
    getCandidateReportMock.mockResolvedValue(envelope('p1', 'e1'))
    const { report, load } = useCandidateReport()
    await load()
    expect(report.value).not.toBeNull()
    store.current = storeState('p2', 'e1')
    await new Promise((r) => setTimeout(r, 0)) // let the id watch fire
    expect(report.value).toBeNull()
  })

  it('cross-project switch releases a hanging load (loading not stuck)', async () => {
    const store = useProjectStore()
    store.current = storeState('p1', 'e1')
    store.resultMeta = { hub_priority: 'e1' }
    let resolveA!: (v: CandidateReportEnvelope) => void
    getCandidateReportMock.mockImplementationOnce(() => new Promise<CandidateReportEnvelope>((r) => { resolveA = r }))
    const { loading, load } = useCandidateReport()
    const pa = load()
    expect(loading.value).toBe(true)
    store.current = storeState('p2', 'e1')
    await new Promise((r) => setTimeout(r, 0)) // let the id watch fire
    expect(loading.value).toBe(false) // watch released the new project's loading
    resolveA(envelope('p1', 'e-old'))
    await pa
    expect(loading.value).toBe(false) // the old finally must not re-lock it
  })

  it('out-of-order requests keep the newest', async () => {
    const store = useProjectStore()
    store.current = storeState('p1', 'e1')
    store.resultMeta = { hub_priority: 'e1' }
    let resolveA!: (v: CandidateReportEnvelope) => void
    let resolveB!: (v: CandidateReportEnvelope) => void
    getCandidateReportMock
      .mockImplementationOnce(() => new Promise<CandidateReportEnvelope>((r) => { resolveA = r }))
      .mockImplementationOnce(() => new Promise<CandidateReportEnvelope>((r) => { resolveB = r }))
    const { report, load } = useCandidateReport()
    const pa = load()
    const pb = load()
    resolveB(envelope('p1', 'e1'))
    await pb
    resolveA(envelope('p1', 'e-old'))
    await pa
    expect(report.value?.execution_id).toBe('e1') // A (older request) must not land
  })

  it('a superseded late success does not clear the newest report', async () => {
    const store = useProjectStore()
    store.current = storeState('p1', 'e1')
    store.resultMeta = { hub_priority: 'e1' }
    let resolveA!: (v: CandidateReportEnvelope) => void
    getCandidateReportMock
      .mockImplementationOnce(() => new Promise<CandidateReportEnvelope>((r) => { resolveA = r }))
      .mockResolvedValueOnce(envelope('p1', 'e1'))
    const { report, load } = useCandidateReport()
    const pa = load()
    const pb = load()
    await pb
    resolveA(envelope('p1', 'e-old'))
    await pa
    expect(report.value?.execution_id).toBe('e1')
  })

  it('a superseded late rejection does not clear the newest report', async () => {
    const store = useProjectStore()
    store.current = storeState('p1', 'e1')
    store.resultMeta = { hub_priority: 'e1' }
    let rejectA!: (e: Error) => void
    getCandidateReportMock
      .mockImplementationOnce(() => new Promise<CandidateReportEnvelope>((_, rej) => { rejectA = rej }))
      .mockResolvedValueOnce(envelope('p1', 'e1'))
    const { report, load } = useCandidateReport()
    const pa = load()
    const pb = load()
    await pb
    rejectA(new Error('late error'))
    await pa.catch(() => {})
    expect(report.value?.execution_id).toBe('e1') // the rejection must not wipe the newest report
  })
})
