import assert from 'node:assert/strict'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { waitForRun } from '../src/utils/pollJob'
import { formatNumber } from '../src/utils/formatNumber'
import { useWgcnaStore } from '../src/stores/wgcna'
const state = (status: string, extra = {}) => ({ id: 'job', run_id: 'run', current_step: 'filter', status, done_steps: status === 'done' ? ['filter'] : [], ...extra }) as any
let reads = 0
await waitForRun('job', 'run', () => {}, () => {}, async () => state(++reads > 750 ? 'done' : 'running'), async () => {})
assert.equal(reads, 751)
reads = 0
const connections: boolean[] = []
await waitForRun('job', 'run', () => {}, lost => connections.push(lost), async () => { if (++reads <= 4) throw Error('offline'); return state('done') }, async () => {})
assert(connections.includes(true) && connections.at(-1) === false)
await assert.rejects(waitForRun('job', 'run', () => {}, () => {}, async () => state('done', { run_id: 'old' }), async () => {}), /replaced/)
await assert.rejects(waitForRun('job', 'run', () => {}, () => {}, async () => state('error', { error: 'actionable failure' }), async () => {}), /actionable failure/)
assert.equal((await waitForRun('job', 'run', () => {}, () => {}, async () => state('cancelled'), async () => {})).status, 'cancelled')
assert.equal(formatNumber(null, 3), '—')
assert.equal(formatNumber(NaN, 3), '—')
assert.equal(formatNumber(0, 3), '0.000')
setActivePinia(createPinia())
const store = useWgcnaStore()
store.jobId = 'job'
store.job = state('done', { done_steps: ['filter', 'soft_threshold', 'network'] })
store.sampleInfo = [{ sample_id: 'a', attrs: { group: 'A' } }, { sample_id: 'b', attrs: { group: 'B' } }] as any
await nextTick()
assert.equal(store.traitSource, 'sample_info')
assert(store.stepReady('module_trait'))
store.sampleInfo = null
await nextTick()
assert(!store.stepReady('module_trait'))
store.job = state('done')
let requestResult = 0, rejectStart = false, workerStatus = 'done'
globalThis.fetch = async (input: any) => {
  const url = String(input)
  const json = (body: any, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
  if (url.includes('/steps/')) return rejectStart ? json({ code: 'parameters', message: 'Correct the invalid parameter' }, 422) : json({ run_id: 'run' })
  if (url.includes('/sample-info')) return json([])
  if (url.includes('/results/')) { requestResult++; return json({ samples: ['a', 'b', 'c'], n_genes_final: 2 }) }
  return json(state(workerStatus, workerStatus === 'error' ? { error: 'worker failed' } : {}))
}
await store.runStep('filter')
assert(store.isDone('filter'))
const successful = store.filterResult
rejectStart = true
await store.runStep('filter')
assert.equal(store.filterResult, successful)
assert.match(store.notice?.message ?? '', /invalid parameter/)
store.nKeep++
assert.equal(store.dirtyStep, 'filter')
assert(!store.stepReady('soft_threshold'))
store.nKeep--
rejectStart = false
workerStatus = 'error'
const before = requestResult
await store.runStep('filter')
assert.equal(store.filterResult, null)
assert.equal(requestResult, before)
console.log('Pipeline UI guards passed: long runs, reconnect, run identity, cancellation, missing statistics, metadata-only traits, dirty parameters and failed reruns')
