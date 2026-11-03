import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { defaultParams, projectRequest, steps, type Params, type ProjectResponse, type QuestionCard, type RecentProject, type StepName } from '../api/projects'

export const useProjectStore = defineStore('project', () => {
  const current = ref<ProjectResponse | null>(null)
  const draft = ref<Params>(defaultParams())
  const card = ref<QuestionCard>({ question: '' })
  const ui = ref<{ step: StepName; threads: number }>({ step: 'filter', threads: 2 })
  const recent = ref<RecentProject[]>([])
  const error = ref('')
  const saving = ref(false)
  const dirty = ref(false)
  const draftDirty = ref(false)
  const working = ref(false)
  const results = ref<Partial<Record<StepName, unknown>>>({})
  // Step → execution_id of the loaded result, so `currentResult` can verify the
  // cached numbers match the CURRENT execution (not a late same-project response).
  const resultMeta = ref<Partial<Record<StepName, string>>>({})
  let hydrating = false
  let generation = 0
  let draftGeneration = 0
  let sessionGen = 0   // project session (hydrate/close) boundary
  let resultsGen = 0   // loadResults batch boundary
  let stateGen = 0     // every state-write/op boundary (run/import/demo/save/refresh/hydrate/close)
  let timer: ReturnType<typeof setTimeout> | undefined
  let pendingSave: Promise<void> | undefined
  let resultsKey = ''
  const id = computed(() => current.value?.state.id ?? '')
  const busy = computed(() => working.value || !!current.value?.busy)

  watch(() => draft.value.filter.data_type, () => {
    if (!hydrating && draft.value.filter.data_type !== 'count' && draft.value.filter.normalize_method === 'vst') draft.value.filter.normalize_method = 'log2p1'
  }, { flush: 'sync' })
  watch(() => draft.value.soft_threshold.network_type, value => {
    if (!hydrating) draft.value.network.network_type = value
  }, { flush: 'sync' })

  watch(draft, () => {
    if (hydrating || !id.value) return
    generation++; draftGeneration++; dirty.value = true; draftDirty.value = true
    clearTimeout(timer)
    timer = setTimeout(() => { void flush().catch(() => {}) }, 600)
  }, { deep: true, flush: 'sync' })
  watch([card, ui], () => {
    if (hydrating || !id.value) return
    generation++; dirty.value = true
    clearTimeout(timer)
    timer = setTimeout(() => { void flush().catch(() => {}) }, 600)
  }, { deep: true, flush: 'sync' })

  async function flush(): Promise<void> {
    clearTimeout(timer)
    if (pendingSave) { await pendingSave; if (dirty.value) return flush(); return }
    if (!dirty.value || !current.value) return
    if (current.value.busy) throw new Error('等待当前步骤完成后再保存 / Wait for the running step')
    saving.value = true
    pendingSave = (async () => {
      while (dirty.value && current.value) {
        const version = generation
        const dVersion = draftGeneration
        const body = JSON.parse(JSON.stringify({ revision: current.value.state.revision, draft_params: draft.value, ui_state: ui.value, question_card: card.value }))
        current.value = await projectRequest<ProjectResponse>(`/${id.value}/draft`, 'PUT', body)
        dirty.value = generation !== version
        draftDirty.value = draftGeneration !== dVersion
        stateGen++
      }
    })()
    try { await pendingSave; error.value = '' }
    catch (e) { error.value = String(e); throw e }
    finally { saving.value = false; pendingSave = undefined }
  }

  function hydrate(response: ProjectResponse) {
    hydrating = true
    sessionGen++; stateGen++
    results.value = {}; resultsKey = ''; resultMeta.value = {}
    current.value = response
    const defaults = defaultParams()
    for (const key of Object.keys(defaults) as StepName[]) Object.assign(defaults[key], response.state.draft_params[key] ?? {})
    draft.value = defaults
    card.value = { ...response.state.question_card }
    const savedStep = response.state.ui_state?.step
    const savedThreads = response.state.ui_state?.threads
    ui.value = { step: savedStep && steps.includes(savedStep) ? savedStep : 'filter', threads: savedThreads && Number.isInteger(savedThreads) && savedThreads > 0 ? savedThreads : 2 }
    dirty.value = false; draftDirty.value = false; error.value = ''; hydrating = false
  }

  async function loadRecent() { recent.value = await projectRequest<RecentProject[]>('/recent') }
  async function close() {
    if (!current.value) return
    await flush()
    await projectRequest(`/${id.value}/close`, 'POST')
    sessionGen++; stateGen++
    current.value = null; dirty.value = false; draftDirty.value = false
    results.value = {}; resultsKey = ''; resultMeta.value = {}
  }
  async function open(path: string) {
    if (busy.value) throw new Error('请先等待或取消当前分析 / Finish or cancel the current analysis')
    await close()
    hydrate(await projectRequest<ProjectResponse>('/open', 'POST', { path }))
    await loadRecent()
  }
  async function create(parent: string, name: string) {
    if (busy.value) throw new Error('请先等待或取消当前分析 / Finish or cancel the current analysis')
    await close()
    hydrate(await projectRequest<ProjectResponse>('', 'POST', { parent, name }))
    dirty.value = true; await flush(); await loadRecent()
  }
  async function importFile(role: string, file: File) {
    await flush()
    stateGen++
    const fd = new FormData(); fd.append('file', file)
    current.value = await projectRequest<ProjectResponse>(`/${id.value}/inputs/${role}`, 'POST', fd)
    stateGen++ // landing: a read issued during this mutation must not overwrite it
  }
  async function loadDemo() {
    await flush()
    stateGen++
    current.value = await projectRequest<ProjectResponse>(`/${id.value}/demo`, 'POST')
    stateGen++ // landing
  }
  async function refresh() {
    if (!id.value || dirty.value || saving.value) return
    const sess = sessionGen
    const ver = ++stateGen
    const pid = id.value
    const resp = await projectRequest<ProjectResponse>(`/${pid}`)
    // A late refresh must never overwrite a newer session / state write (run,
    // import, demo, save) or a newer refresh.
    if (sess !== sessionGen || ver !== stateGen || pid !== id.value) return
    current.value = resp
    if (current.value.error) error.value = current.value.error
  }
  async function run(step: StepName) {
    working.value = true
    stateGen++
    try {
      if (step === 'hub') draft.value.hub.power = draft.value.network.power
      if (step === 'rean') draft.value.rean.select_module = draft.value.hub.select_module
      await flush()
      current.value = await projectRequest<ProjectResponse>(`/${id.value}/steps/${step}/run`, 'POST', { threads: ui.value.threads, revision: current.value!.state.revision })
      stateGen++ // landing: invalidate any read issued during this run's POST
    } finally { working.value = false }
  }
  async function cancel() { stateGen++; await projectRequest(`/${id.value}/cancel`, 'POST'); stateGen++ }

  // Load committed results for every done step. The key guard skips reloading an
  // unchanged step table, so the three analysis pages share one in-memory cache.
  // Every response is bound to the project session generation + a batch counter +
  // the step execution_id, so a late same-project response can never overwrite a
  // newer execution's numbers.
  async function loadResults() {
    const c = current.value
    if (!c || c.busy) return
    const key = c.state.id + ':' + JSON.stringify(c.state.steps)
    if (resultsKey === key) return
    resultsKey = key
    results.value = {}
    resultMeta.value = {}
    const sess = sessionGen
    const batch = ++resultsGen
    for (const step of steps) {
      const exec = c.state.steps[step]
      if (exec?.status !== 'done') continue
      const execId = exec.execution_id
      try {
        const result = await projectRequest<unknown>(`/${c.state.id}/results/${step}`)
        if (sess !== sessionGen || batch !== resultsGen) return
        if (current.value?.state.steps[step]?.execution_id !== execId) continue
        results.value[step] = result
        resultMeta.value[step] = execId
      } catch (e) {
        if (sess === sessionGen && batch === resultsGen) { error.value = String(e); resultsKey = '' }
      }
    }
  }
  function stepStatus(step: StepName): 'idle' | 'running' | 'done' | 'error' {
    const s = current.value?.state.steps[step]
    if (s?.status === 'done') return 'done'
    if (s?.status === 'error') return 'error'
    if (s?.status === 'running' || (current.value?.busy && !!s)) return 'running'
    return 'idle'
  }

  // Whether the committed result for `step` is still CURRENT evidence (not stale).
  // Requires: step done, backend validity EXPLICITLY empty (missing validity is not
  // valid), the cached numbers were loaded for the CURRENT execution_id, and no
  // unsaved scientific-param change (draftDirty) or in-flight save. Plot/UI changes
  // do NOT revoke current evidence.
  function currentResult(step: StepName): boolean {
    const c = current.value
    if (!c) return false
    const exec = c.state.steps[step]
    if (exec?.status !== 'done') return false
    if (c.validity[step] !== '') return false
    if (resultMeta.value[step] !== exec.execution_id) return false
    if (draftDirty.value || saving.value) return false
    return true
  }

  return { current, draft, card, ui, recent, error, saving, dirty, draftDirty, busy, results, resultMeta, id, flush, open, create, close, loadRecent, importFile, loadDemo, refresh, run, cancel, loadResults, stepStatus, currentResult }
})
