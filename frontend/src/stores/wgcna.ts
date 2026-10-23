import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  createJob, getJob, getResult, runStep as runStepApi, uploadTrait,
  type DetailResult, type FilterResult, type HubResult, type JobStatus, type ModulesResult, type MtaitResult, type SftResult,
} from '../api/client'

export const useWgcnaStore = defineStore('wgcna', () => {
  const jobId = ref('')
  const job = ref<JobStatus | null>(null)
  const exprFile = ref<File | null>(null)
  const traitFile = ref<File | null>(null)
  const nKeep = ref(8000)
  const normalizeMethod = ref('log10p1')
  const filterMethod = ref('mad')
  const samplePercentage = ref(0)
  const cutoff = ref(1)
  const rsquaredCut = ref(0.85)
  const sftNetworkType = ref('unsigned')
  const power = ref(6)
  const netNetworkType = ref('unsigned')
  const tomType = ref('signed')
  const tomDenom = ref('min')
  const minModuleSize = ref(30)
  const mergeCutHeight = ref(0.25)
  const deepSplit = ref(2)
  const detectCutHeight = ref(0.995)
  const traitUploaded = ref(false)
  const stepRunning = ref('')
  const error = ref('')

  const sft = ref<SftResult | null>(null)
  const modules = ref<ModulesResult | null>(null)
  const mtait = ref<MtaitResult | null>(null)
  const detail = ref<DetailResult | null>(null)
const selectedModule = ref('')
const selectedTrait = ref('')
const kmeCutoff = ref(0.5)
const gsCutoff = ref(0.5)
const weightThreshold = ref(0.02)
const topNNeighbors = ref(20)
const maxHubs = ref(30)
  const hub = ref<HubResult | null>(null)
  const filterResult = ref<FilterResult | null>(null)

  const created = computed(() => !!jobId.value)

  function params() {
    return {
      filter: { normalize_method: normalizeMethod.value, n_keep: nKeep.value, filter_method: filterMethod.value, sample_percentage: samplePercentage.value, cutoff: cutoff.value },
      soft_threshold: { network_type: sftNetworkType.value, rsquared_cut: rsquaredCut.value },
      network: { power: power.value, network_type: netNetworkType.value, tom_type: tomType.value, tom_denom: tomDenom.value, min_module_size: minModuleSize.value, merge_cut_height: mergeCutHeight.value, deep_split: deepSplit.value, detect_cut_height: detectCutHeight.value },
      module_detail: { select_module: selectedModule.value, select_trait: selectedTrait.value },
      hub: { select_module: selectedModule.value, select_trait: selectedTrait.value, kme_cutoff: kmeCutoff.value, gs_cutoff: gsCutoff.value, weight_threshold: weightThreshold.value, power: power.value, top_n_neighbors: topNNeighbors.value, max_hubs: maxHubs.value },
    }
  }

  async function createJobAndUpload() {
    if (!exprFile.value) return
    error.value = ''
    try {
      const { id } = await createJob(exprFile.value, params())
      jobId.value = id
      if (traitFile.value) {
        await uploadTrait(id, traitFile.value)
        traitUploaded.value = true
      }
      await refresh()
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    }
  }

  async function refresh() {
    if (!jobId.value) return
    try {
      job.value = await getJob(jobId.value)
    } catch {
      /* keep last known state */
    }
  }

  function isDone(step: string) {
    return job.value?.done_steps?.includes(step) ?? false
  }

  function stepReady(step: string) {
    if (!jobId.value) return false
    if (step === 'module_trait' || step === 'module_detail' || step === 'hub') {
      if (!traitUploaded.value) return false
    }
    const deps: Record<string, string[]> = {
      filter: [], soft_threshold: ['filter'], network: ['soft_threshold'],
      module_trait: ['network'], module_detail: ['module_trait'], hub: ['module_trait'],
    }
    return (deps[step] ?? []).every(isDone)
  }

  function sleep(ms: number) { return new Promise((r) => setTimeout(r, ms)) }

  // 把底层 panic/错误翻译成人话
  function friendlyError(msg: string): string {
    if (!msg) return ''
    if (msg.includes('Option::unwrap')) return '计算中出现无效数值（可能含常数基因/零方差数据），请检查数据或减少保留基因数'
    if (msg.includes('read trait.tsv')) return '未上传性状表，请先上传性状文件再运行该步骤'
    if (msg.includes('R plot failed')) return '绘图失败：' + msg.split('R plot failed:')[1]?.split('\n')[0]?.trim()
    return msg
  }

  async function waitForDone() {
    await sleep(400)
    for (let i = 0; i < 600; i++) {
      await refresh()
      const st = job.value?.status
      if (st === 'error' || st === 'cancelled') break
      if (st === 'done') break
      await sleep(500)
    }
    // 把 job 的错误/取消状态传播到 store.error（友好化文案）
    if (job.value?.status === 'error') {
      error.value = friendlyError(job.value.error ?? '')
    } else if (job.value?.status === 'cancelled') {
      error.value = '已取消'
    } else {
      error.value = ''
    }
  }

  // 每步状态：idle（未运行）| running（运行中）| done（完成）| error（失败）
  function stepStatus(step: string): 'idle' | 'running' | 'done' | 'error' {
    if (stepRunning.value === step) return 'running'
    if (isDone(step)) return 'done'
    if (job.value?.status === 'error' && job.value?.current_step === step) return 'error'
    return 'idle'
  }

  // 重跑某步时，清空该步及下游的缓存结果（配合后端脏级联）。
  const STEP_ORDER = ['filter', 'soft_threshold', 'network', 'module_trait', 'module_detail', 'hub']
  function clearDownstream(step: string) {
    const idx = STEP_ORDER.indexOf(step)
    if (idx < 0) return
    const affected = STEP_ORDER.slice(idx)
    if (affected.includes('filter')) filterResult.value = null
    if (affected.includes('soft_threshold')) sft.value = null
    if (affected.includes('network')) modules.value = null
    if (affected.includes('module_trait')) mtait.value = null
    if (affected.includes('module_detail')) detail.value = null
    if (affected.includes('hub')) hub.value = null
  }

  async function runStep(step: string) {
    if (!jobId.value || stepRunning.value) return
    stepRunning.value = step
    error.value = ''
    clearDownstream(step)
    try {
      await runStepApi(jobId.value, step, params())
      await waitForDone()
      await loadStep(step)
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      stepRunning.value = ''
    }
  }

  async function removeSamples(samples: string[]) {
    if (!jobId.value || stepRunning.value) return
    stepRunning.value = 'filter'
    error.value = ''
    clearDownstream('filter')
    try {
      await runStepApi(jobId.value, 'filter', {
        filter: {
          normalize_method: normalizeMethod.value,
          n_keep: nKeep.value,
          filter_method: filterMethod.value,
          sample_percentage: samplePercentage.value,
          cutoff: cutoff.value,
          exclude_samples: samples,
        },
      })
      await waitForDone()
      await loadStep('filter')
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      stepRunning.value = ''
    }
  }

  async function loadStep(step: string) {
    try {
      if (step === 'filter') filterResult.value = await getResult<FilterResult>(jobId.value, step)
      else if (step === 'soft_threshold') {
        sft.value = await getResult<SftResult>(jobId.value, step)
        // 用 SFT 的推荐值自动回填第三步（network）的 power
        if (sft.value.power_estimate != null) power.value = sft.value.power_estimate
      }
      else if (step === 'network') modules.value = await getResult<ModulesResult>(jobId.value, step)
      else if (step === 'module_trait') mtait.value = await getResult<MtaitResult>(jobId.value, step)
      else if (step === 'module_detail') detail.value = await getResult<DetailResult>(jobId.value, step)
      else if (step === 'hub') hub.value = await getResult<HubResult>(jobId.value, step)
    } catch { /* not available */ }
  }

  return {
    jobId, job, exprFile, traitFile, nKeep, normalizeMethod, filterMethod, samplePercentage, cutoff,
    rsquaredCut, sftNetworkType, power, netNetworkType, tomType, tomDenom, minModuleSize, mergeCutHeight, deepSplit, detectCutHeight,
    traitUploaded, stepRunning, error,
    sft, modules, mtait, detail, hub, filterResult, created, selectedModule, selectedTrait, kmeCutoff, gsCutoff, weightThreshold, topNNeighbors, maxHubs,
    createJobAndUpload, runStep, removeSamples, refresh, isDone, stepReady, stepStatus,
  }
})
