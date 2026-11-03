// Actual-store deferred-request tests: import the REAL useProjectStore and mock
// only the `projectRequest` transport. Guards under test are NOT re-implemented
// here — the assertions exercise the production store's session/state/execution
// generation logic.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { ProjectResponse } from '../api/projects'

const { projectRequestMock } = vi.hoisted(() => ({ projectRequestMock: vi.fn() }))

vi.mock('../api/projects', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/projects')>()
  return { ...actual, projectRequest: projectRequestMock }
})

import { useProjectStore } from './project'

function baseResponse(overrides: Partial<ProjectResponse> = {}): ProjectResponse {
  return {
    state: {
      id: 'p1', name: 'p', revision: 1, updated_at: 't',
      inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: {},
    },
    validity: {} as ProjectResponse['validity'],
    path: '/p1', busy: false, error: '', progress: { step: '', status: '', progress: 0, message: '' },
    ...overrides,
  }
}

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => { resolve = r })
  return { promise, resolve }
}

describe('project store async guards', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    projectRequestMock.mockReset()
  })

  async function openStore(resp: ProjectResponse) {
    const store = useProjectStore()
    projectRequestMock.mockImplementation(async (path: string) => {
      if (path === '/open') return resp
      if (path === '/recent') return []
      if (path === '/close') return {}
      throw new Error('unexpected ' + path)
    })
    await store.open('/p1')
    projectRequestMock.mockReset()
    return store
  }

  it('late refresh does not roll back a param-less run', async () => {
    const store = await openStore(baseResponse({
      state: { id: 'p1', name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { filter: { execution_id: 'f0', status: 'done', params: {} } } },
      validity: { filter: '', soft_threshold: '', network: '', hub_priority: '', module_trait: '', module_detail: '', hub: '', rean: '' } as ProjectResponse['validity'],
    }))

    const refreshD = deferred<ProjectResponse>()
    const runD = deferred<ProjectResponse>()
    // refresh is issued FIRST (slow), run is issued SECOND (fast) — exactly the
    // reviewer's counterexample (param-less run does not bump `generation`).
    projectRequestMock
      .mockImplementationOnce(() => refreshD.promise) // 1st = refresh GET
      .mockImplementationOnce(() => runD.promise)     // 2nd = run POST
    const refreshP = store.refresh()
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(1))
    const runP = store.run('filter')
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(2))

    // run commits first (newer state: execution f1 running).
    runD.resolve(baseResponse({
      state: { id: 'p1', name: 'p', revision: 2, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { filter: { execution_id: 'f1', status: 'running', params: {} } } },
      validity: { filter: '', soft_threshold: '', network: '', hub_priority: '', module_trait: '', module_detail: '', hub: '', rean: '' } as ProjectResponse['validity'],
      busy: true,
    }))
    await runP
    // late refresh returns the OLD snapshot (execution f0); it must be discarded.
    refreshD.resolve(baseResponse({
      state: { id: 'p1', name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { filter: { execution_id: 'f0', status: 'done', params: {} } } },
      validity: { filter: '', soft_threshold: '', network: '', hub_priority: '', module_trait: '', module_detail: '', hub: '', rean: '' } as ProjectResponse['validity'],
    }))
    await refreshP
    expect(store.current?.state.steps.filter?.execution_id).toBe('f1')
    expect(store.current?.state.steps.filter?.status).toBe('running')
  })

  it('two refreshes out of order keep the newest', async () => {
    const store = await openStore(baseResponse())
    const a = deferred<ProjectResponse>()
    const b = deferred<ProjectResponse>()
    projectRequestMock.mockImplementationOnce(() => a.promise).mockImplementationOnce(() => b.promise)
    const pa = store.refresh()
    const pb = store.refresh()
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(2))
    b.resolve(baseResponse({ state: { ...baseResponse().state, revision: 2 } }))
    await pb
    a.resolve(baseResponse({ state: { ...baseResponse().state, revision: 1 } }))
    await pa
    expect(store.current?.state.revision).toBe(2)
  })

  it('A/B results out of order keep the current execution', async () => {
    const store = await openStore(baseResponse({
      state: { id: 'p1', name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { rean: { execution_id: 'exec-A', status: 'done', params: {} } } },
    }))
    const a = deferred<unknown>()
    const b = deferred<unknown>()
    // loadResults issues one request per done step (rean). First batch = A.
    projectRequestMock.mockImplementationOnce(() => a.promise)
    const loadA = store.loadResults()
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(1))
    // execution changes to B; new loadResults starts (B), B resolves first.
    store.current = { ...store.current!, state: { ...store.current!.state, steps: { rean: { execution_id: 'exec-B', status: 'done', params: {} } } } }
    projectRequestMock.mockImplementationOnce(() => b.promise)
    const loadB = store.loadResults()
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(2))
    b.resolve({ from: 'B' })
    await loadB
    a.resolve({ from: 'A' })
    await loadA
    expect(store.results.rean).toEqual({ from: 'B' })
    expect(store.resultMeta.rean).toBe('exec-B')
  })

  it('same-project reopen discards old in-flight results', async () => {
    const store = await openStore(baseResponse({
      state: { id: 'p1', name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { rean: { execution_id: 'exec-A', status: 'done', params: {} } } },
    }))
    const a = deferred<unknown>()
    projectRequestMock.mockImplementationOnce(() => a.promise)
    const loadA = store.loadResults()
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(1))
    // close + reopen the SAME project; sessionGen bumps, invalidating loadA.
    projectRequestMock.mockImplementation(async (path: string) => {
      if (path.endsWith('/close')) return {}
      if (path === '/open') return baseResponse({ state: { id: 'p1', name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { rean: { execution_id: 'exec-A', status: 'done', params: {} } } } })
      if (path === '/recent') return []
      throw new Error('unexpected ' + path)
    })
    await store.close()
    await store.open('/p1')
    a.resolve({ from: 'A' })
    await loadA
    expect(store.results.rean).toBeUndefined()
  })

  it('draft save interleave revokes a late refresh', async () => {
    const store = await openStore(baseResponse())
    const refreshD = deferred<ProjectResponse>()
    projectRequestMock.mockImplementationOnce(() => refreshD.promise)
    const refreshP = store.refresh()
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(1))
    // a draft edit + flush (save) happens during the refresh.
    store.draft.filter.n_keep = 50
    projectRequestMock.mockImplementation(async () => baseResponse({ state: { ...baseResponse().state, revision: 2 } }))
    await store.flush()
    refreshD.resolve(baseResponse({ state: { ...baseResponse().state, revision: 1 } }))
    await refreshP
    expect(store.current?.state.revision).toBe(2)
  })

  it('missing validity is not treated as current', async () => {
    const store = await openStore(baseResponse({
      state: { id: 'p1', name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { rean: { execution_id: 'e', status: 'done', params: {} } } },
      // validity has NO 'rean' key at all.
      validity: {} as ProjectResponse['validity'],
    }))
    // load the result so resultMeta matches; validity is still missing.
    projectRequestMock.mockImplementation(async (path: string) => (path.includes('/results/rean') ? { ok: 1 } : (() => { throw new Error('unexpected') })()))
    await store.loadResults()
    expect(store.currentResult('rean')).toBe(false)
  })

  it('refresh issued during an in-flight run must not roll back the landed run', async () => {
    const store = await openStore(baseResponse({
      state: { id: 'p1', name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { filter: { execution_id: 'f0', status: 'done', params: {} } } },
      validity: { filter: '', soft_threshold: '', network: '', hub_priority: '', module_trait: '', module_detail: '', hub: '', rean: '' } as ProjectResponse['validity'],
    }))
    const runD = deferred<ProjectResponse>()
    const refreshD = deferred<ProjectResponse>()
    projectRequestMock
      .mockImplementationOnce(() => runD.promise)     // 1st = run POST
      .mockImplementationOnce(() => refreshD.promise)  // 2nd = refresh GET
    const runP = store.run('filter')
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(1))
    const refreshP = store.refresh()
    await vi.waitFor(() => expect(projectRequestMock).toHaveBeenCalledTimes(2))
    // run lands first (revision 2, new running execution f1).
    runD.resolve(baseResponse({
      state: { id: 'p1', name: 'p', revision: 2, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { filter: { execution_id: 'f1', status: 'running', params: {} } } },
      validity: { filter: '', soft_threshold: '', network: '', hub_priority: '', module_trait: '', module_detail: '', hub: '', rean: '' } as ProjectResponse['validity'],
      busy: true,
    }))
    await runP
    // late refresh returns the old snapshot; must be discarded.
    refreshD.resolve(baseResponse({
      state: { id: 'p1', name: 'p', revision: 1, updated_at: 't', inputs: {}, draft_params: {}, ui_state: {}, question_card: { question: '' }, steps: { filter: { execution_id: 'f0', status: 'done', params: {} } } },
      validity: { filter: '', soft_threshold: '', network: '', hub_priority: '', module_trait: '', module_detail: '', hub: '', rean: '' } as ProjectResponse['validity'],
    }))
    await refreshP
    expect(store.current?.state.revision).toBe(2)
    expect(store.current?.state.steps.filter?.execution_id).toBe('f1')
    expect(store.current?.state.steps.filter?.status).toBe('running')
  })
})
