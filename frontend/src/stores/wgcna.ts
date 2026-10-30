import { defineStore } from 'pinia'
import { waitForRun } from '../utils/pollJob'
import { locale } from '../utils/i18n'
import { computed, ref, watch } from 'vue'
import {
  ApiError, cancelJob, validateInputs, type InputValidation, createJob, getJob, getResult, getSampleInfo, loadDemoApi, runStep as runStepApi, uploadMeta, uploadTrait,
  type DetailResult, type FilterResult, type HubResult, type JobStatus, type ModulesResult, type MtaitResult, type SampleInfo, type SftResult,
} from '../api/client'

export const useWgcnaStore = defineStore('wgcna', () => {
  const jobId = ref('')
  const job = ref<JobStatus | null>(null)
  const exprFile = ref<File | null>(null)
  const traitFile = ref<File | null>(null)
  const sampleInfoFile = ref<File | null>(null)
  const annotationFile = ref<File | null>(null)
  const gene2goFile = ref<File | null>(null)
  const gene2keggFile = ref<File | null>(null)
  const demoLoaded = ref(false)
  const threads = ref(Math.max(1, (navigator.hardwareConcurrency || 4) - 2))
  const nKeep = ref(8000)
  const dataType = ref<'count' | 'expected_count' | 'normalized_count' | 'peak_area' | 'protein_abundance'>('count')
  const normalizeMethod = ref('vst')
  const filterMethod = ref('mad')
  const samplePercentage = ref(0)
  const cutoff = ref(1)
  const excludedSamples = ref<string[]>([])

  // Normalization methods that make sense for the chosen data type (prevents
  // e.g. applying DESeq2 VST to FPKM / peak-area data).
  const normalizeOptions = computed<{ value: string; label: string }[]>(() => {
    switch (dataType.value) {
      case 'expected_count':
        return [
          { value: 'vst', label: 'vst (DESeq2) — Expected count' },
          { value: 'log2p1', label: 'log2(x+1)' },
          { value: 'raw', label: 'raw' },
        ]
      case 'normalized_count':
        return [
          { value: 'log2p1', label: 'log2(x+1) — FPKM/TPM' },
          { value: 'log10p1', label: 'log10(x+1)' },
          { value: 'log2', label: 'log2(x)' },
          { value: 'raw', label: 'raw' },
        ]
      case 'peak_area':
        return [
          { value: 'log2', label: 'log2(x) — Peak area' },
          { value: 'log10', label: 'log10(x)' },
          { value: 'log', label: 'ln(x)' },
          { value: 'raw', label: 'raw' },
        ]
      case 'protein_abundance':
        return [
          { value: 'log2', label: 'log2(x) — Protein abundance' },
          { value: 'log', label: 'ln(x)' },
          { value: 'raw', label: 'raw' },
        ]
      default: // count
        return [
          { value: 'vst', label: 'vst (DESeq2) — Raw count' },
          { value: 'log2p1', label: 'log2(x+1) — Raw count (simple)' },
          { value: 'raw', label: 'raw' },
        ]
    }
  })

  // Reset the normalization method when the data type changes and the current
  // choice no longer applies (e.g. vst → peak_area).
  watch(dataType, () => {
    if (!normalizeOptions.value.some((o) => o.value === normalizeMethod.value)) {
      normalizeMethod.value = normalizeOptions.value[0]?.value ?? 'raw'
    }
  })
  const rsquaredCut = ref(0.85)
  const sftNetworkType = ref('unsigned')
  const power = ref(6)
  const netNetworkType = ref('unsigned')
  // Network construction follows the soft-threshold network type (they describe
  // the same network); keep them in sync so a change in SFT carries over.
  watch(sftNetworkType, (v) => { netNetworkType.value = v })
  watch(netNetworkType, (v) => { sftNetworkType.value = v })
  const tomType = ref('signed')
  const tomDenom = ref('min')
  const minModuleSize = ref(30)
  const mergeCutHeight = ref(0.25)
  const deepSplit = ref(2)
  const detectCutHeight = ref(0.995)
  const traitUploaded = ref(false)
  const traitSource = ref<'uploaded' | 'sample_info'>('uploaded')
  const traitColumn = ref('')
  const sampleInfo = ref<SampleInfo[] | null>(null)
  const stepRunning = ref('')
  const error = ref('')
  const notice = ref<{ message: string; path?: string } | null>(null)
  const connectionLost = ref(false)
  const cancelling = ref(false)
  function notifyError(e: unknown, step = '') {
    const message = e instanceof Error ? e.message : String(e)
    error.value = friendlyError(message)
    const routes: Record<string, string> = { filter: '/data-import', soft_threshold: '/sft', network: '/network', module_trait: '/module-trait', module_detail: '/module-explorer', hub: '/module-explorer', cluster: '/module-overview', cluster_elbow: '/module-overview', overview_network: '/module-overview' }
    let relevant = step
    const changed = message.match(/(?:Parameters for |Complete |Rerun )(filter|soft_threshold|network|module_trait)/)?.[1]
    if (changed) relevant = changed
    if (e instanceof ApiError && ['normalization', 'filter_empty', 'inputs', 'samples', 'job_missing'].includes(e.code)) relevant = 'filter'
    notice.value = { message: error.value, path: routes[relevant] }
  }
  async function cancelCurrent() {
    if (!jobId.value || cancelling.value) return
    cancelling.value = true
    try { await cancelJob(jobId.value) } catch (e) { notifyError(e) } finally { cancelling.value = false }
  }

  const inputValidation = ref<InputValidation | null>(null)
  const validatingInputs = ref(false)
  const creatingJob = ref(false)
  const showInputValidation = ref(false)
  let validationVersion = 0
  const uploadFiles = () => ({ expr: exprFile.value, trait: traitFile.value, sample_info: sampleInfoFile.value,
    annotation: annotationFile.value, gene2go: gene2goFile.value, gene2kegg: gene2keggFile.value })

  async function checkInputs(): Promise<boolean> {
    const version = ++validationVersion
    validatingInputs.value = true
    try {
      const report = await validateInputs(uploadFiles(), dataType.value)
      if (version !== validationVersion) return false
      inputValidation.value = report
      if (report.issues.length) showInputValidation.value = true
      return report.valid
    } catch (e) {
      if (version !== validationVersion) return false
      inputValidation.value = { valid: false, error_count: 1, warning_count: 0, issues: [
        { severity: 'error', file: 'upload', message: e instanceof Error ? e.message : String(e) },
      ] }
      showInputValidation.value = true
      return false
    } finally {
      if (version === validationVersion) validatingInputs.value = false
    }
  }

  // A newly selected file belongs to a new input set. Never run an old job
  // with new filenames shown in the upload controls.
  function invalidateInputs() {
    ++validationVersion
    inputValidation.value = null
    validatingInputs.value = false
    showInputValidation.value = false
    jobId.value = ''
    job.value = null
    demoLoaded.value = false
    traitUploaded.value = false
    sampleInfo.value = null
    traitColumn.value = ''
    selectedModule.value = ''
    selectedTrait.value = ''
    excludedSamples.value = []
    clearDownstream('filter')
    appliedParams.value = {}
    error.value = ''
  }


  const appliedParams = ref<Record<string, string>>({})
  const sft = ref<SftResult | null>(null)
  const modules = ref<ModulesResult | null>(null)
  const mtait = ref<MtaitResult | null>(null)
  const detail = ref<DetailResult | null>(null)
const selectedModule = ref('')
const selectedTrait = ref('')
const kmeCutoff = ref(0.8)
const gsCutoff = ref(0)
const weightThreshold = ref(0.02)
const topNNeighbors = ref(20)
const maxHubs = ref(30)
  const hub = ref<HubResult | null>(null)
  const filterResult = ref<FilterResult | null>(null)

  const created = computed(() => !!jobId.value)

  // 当 trait/module 集合变化（如重跑 module_trait 换了 trait source）时，
  // 若已选值不在新集合里，重置为第一个合法值，避免 module_detail/hub 用旧值。
  watch([mtait, modules], () => {
    const traits = mtait.value?.traits ?? []
    if (!traits.includes(selectedTrait.value)) {
      selectedTrait.value = traits[0] ?? ''
    }
    const mods = Object.keys(modules.value?.module_sizes ?? {}).filter(m => m !== 'grey')
    if (!mods.includes(selectedModule.value)) {
      selectedModule.value = mods[0] ?? ''
    }
  })

  const sampleColumns = computed(() => Object.keys(sampleInfo.value?.[0]?.attrs ?? {}).filter(c => !['sample_id', 'sample_name', 'ID'].includes(c)))
  watch([sampleInfo, traitUploaded, traitSource], () => {
    if (!sampleColumns.value.includes(traitColumn.value)) traitColumn.value = sampleColumns.value[0] ?? ''
    if (!traitUploaded.value && sampleColumns.value.length) traitSource.value = 'sample_info'
    else if (traitUploaded.value && !sampleColumns.value.length) traitSource.value = 'uploaded'
  })

  async function loadSampleInfo() {
    if (!jobId.value) return
    const id = jobId.value
    try {
      const info = await getSampleInfo(id)
      if (jobId.value === id) sampleInfo.value = info
    } catch {
      if (jobId.value === id) sampleInfo.value = null
    }
  }

  function params() {
    return {
      filter: { data_type: dataType.value, normalize_method: normalizeMethod.value, n_keep: nKeep.value, filter_method: filterMethod.value, sample_percentage: samplePercentage.value / 100, cutoff: cutoff.value, exclude_samples: excludedSamples.value },
      soft_threshold: { network_type: sftNetworkType.value, rsquared_cut: rsquaredCut.value },
      network: { power: power.value, network_type: netNetworkType.value, tom_type: tomType.value, tom_denom: tomDenom.value, min_module_size: minModuleSize.value, merge_cut_height: mergeCutHeight.value, deep_split: deepSplit.value, detect_cut_height: detectCutHeight.value },
      module_trait: { trait_source: traitSource.value, trait_column: traitColumn.value },
      module_detail: { select_module: selectedModule.value, select_trait: selectedTrait.value },
      hub: { select_module: selectedModule.value, select_trait: selectedTrait.value, kme_cutoff: kmeCutoff.value, gs_cutoff: gsCutoff.value, weight_threshold: weightThreshold.value, power: power.value, top_n_neighbors: topNNeighbors.value, max_hubs: maxHubs.value },
    }
  }

  async function createJobAndUpload() {
    if (!exprFile.value || creatingJob.value || validatingInputs.value) return
    creatingJob.value = true
    error.value = ''
    try {
      if (!(await checkInputs())) return
      const { id } = await createJob(exprFile.value, params(), threads.value)
      demoLoaded.value = false
      if (traitFile.value) {
        await uploadTrait(id, traitFile.value)
        traitUploaded.value = true
      }
      await uploadMeta(id, {
        sample_info: sampleInfoFile.value,
        annotation: annotationFile.value,
        gene2go: gene2goFile.value,
        gene2kegg: gene2keggFile.value,
      })
      jobId.value = id
      await refresh()
    } catch (e) {
      notifyError(e)
    } finally {
      creatingJob.value = false
    }
  }

  async function loadDemo() {
    error.value = ''
    try {
      const { id } = await loadDemoApi(threads.value)
      ++validationVersion
      validatingInputs.value = false
      inputValidation.value = null
      showInputValidation.value = false
      excludedSamples.value = []
      clearDownstream('filter')
      appliedParams.value = {}
      jobId.value = id
      traitUploaded.value = true
      dataType.value = 'count'
      normalizeMethod.value = 'vst'
      demoLoaded.value = true
      await refresh()
    } catch (e) {
      notifyError(e)
    }
  }

  async function refresh() {
    if (!jobId.value) return
    try {
      job.value = await getJob(jobId.value)
    } catch {
      /* keep last known state */
    }
    loadSampleInfo()
  }

  const dependencySteps: Record<string, string[]> = {
    filter: [], soft_threshold: ['filter'], network: ['filter', 'soft_threshold'],
    module_trait: ['filter', 'soft_threshold', 'network'],
    module_detail: ['filter', 'soft_threshold', 'network', 'module_trait'],
    hub: ['filter', 'soft_threshold', 'network', 'module_trait'],
  }
  function isDirty(step: string) {
    const current = params() as Record<string, unknown>
    return [...(dependencySteps[step] ?? []), step].some(s =>
      job.value?.done_steps?.includes(s) && appliedParams.value[s] !== undefined && appliedParams.value[s] !== JSON.stringify(current[s]))
  }
  function isDone(step: string) {
    return (job.value?.done_steps?.includes(step) ?? false) && !isDirty(step)
  }
  const dirtyStep = computed(() => ['filter', 'soft_threshold', 'network', 'module_trait', 'module_detail', 'hub']
    .find(s => job.value?.done_steps?.includes(s) && isDirty(s)) ?? '')

  function stepReady(step: string) {
    if (!jobId.value) return false
    if (step === 'module_trait' || step === 'module_detail' || step === 'hub') {
      if (traitSource.value === 'uploaded' ? !traitUploaded.value : !sampleColumns.value.includes(traitColumn.value)) return false
    }
    const deps: Record<string, string[]> = {
      filter: [], soft_threshold: ['filter'], network: ['soft_threshold'],
      module_trait: ['network'], module_detail: ['module_trait'], hub: ['module_trait'],
    }
    return (deps[step] ?? []).every(isDone)
  }

  // 把底层 panic/错误翻译成人话
  function friendlyError(msg: string): string {
    if (!msg) return ''
    if (msg.includes('Option::unwrap')) return '计算中出现无效数值（可能含常数基因/零方差数据），请检查数据或减少保留基因数'
    if (msg.includes('trait.tsv') && (msg.includes('No such file') || msg.includes('cannot find the file'))) return '未找到性状表，请先上传性状文件再运行该步骤'
    if (msg.includes('R plot failed')) return (locale.value === 'zh' ? '绘图环境不可用或缺少 R 包。请检查 Rscript，或使用包含 R 环境的完整版。详细信息：\n' : 'The R plotting environment is unavailable or missing packages. Check Rscript or use the complete release with R. Details:\n') + msg.split('R plot failed:')[1]?.trim()
    return msg
  }

  async function waitForDone(id: string, runId: string) {
    const result = await waitForRun(id, runId,
      status => { if (jobId.value === id) job.value = status },
      lost => { connectionLost.value = lost })
    connectionLost.value = false
    if (result.status === 'cancelled') {
      error.value = locale.value === 'zh' ? '已取消；可调整参数后重新运行。' : 'Cancelled. Adjust settings and run again when ready.'
      return false
    }
    await loadSampleInfo()
    return true
  }

  // 每步状态：idle（未运行）| running（运行中）| done（完成）| error（失败）
  function stepStatus(step: string): 'idle' | 'running' | 'done' | 'error' | 'stale' {
    if (stepRunning.value === step) return 'running'
    if (isDirty(step)) return 'stale'
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
    const id = jobId.value
    stepRunning.value = step
    error.value = ''
    try {
      const submitted = params()
      const { run_id } = await runStepApi(id, step, submitted, threads.value)
      clearDownstream(step)
      if (await waitForDone(id, run_id)) {
        appliedParams.value[step] = JSON.stringify((submitted as Record<string, unknown>)[step])
        await loadStep(step)
      }
    } catch (e) {
      notifyError(e, step)
    } finally {
      connectionLost.value = false
      stepRunning.value = ''
    }
  }

  async function removeSamples(samples: string[], replace = false) {
    if (!jobId.value || stepRunning.value) return
    const previous = excludedSamples.value
    const next = [...new Set(replace ? samples : [...previous, ...samples])]
    if (filterResult.value && !replace && filterResult.value.samples.filter(s => !next.includes(s)).length < 3) {
      notifyError(new ApiError('At least 3 samples must remain. Select fewer outliers or restore samples.', 'samples', 422), 'filter')
      return
    }
    stepRunning.value = 'filter'
    error.value = ''
    excludedSamples.value = next
    let accepted = false
    try {
      const id = jobId.value
      const submitted = params()
      const { run_id } = await runStepApi(id, 'filter', submitted, threads.value)
      accepted = true
      clearDownstream('filter')
      if (await waitForDone(id, run_id)) { appliedParams.value.filter = JSON.stringify(submitted.filter); await loadStep('filter') }
    } catch (e) {
      if (!accepted) excludedSamples.value = previous
      notifyError(e, 'filter')
    } finally {
      connectionLost.value = false
      stepRunning.value = ''
    }
  }

  async function restoreSamples() { await removeSamples([], true) }

  async function loadStep(step: string) {
    if (!isDone(step)) return
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
    } catch (e) { notifyError(e, step) }
  }

  return {
    jobId, job, exprFile, traitFile, sampleInfoFile, annotationFile, gene2goFile, gene2keggFile, demoLoaded, threads, nKeep, dataType, normalizeMethod, normalizeOptions, filterMethod, samplePercentage, cutoff,
    rsquaredCut, sftNetworkType, power, netNetworkType, tomType, tomDenom, minModuleSize, mergeCutHeight, deepSplit, detectCutHeight,
    traitUploaded, traitSource, traitColumn, sampleInfo, sampleColumns, stepRunning, error, dirtyStep, notice, notifyError, connectionLost, cancelling, cancelCurrent,
    sft, modules, mtait, detail, hub, filterResult, created, selectedModule, selectedTrait, kmeCutoff, gsCutoff, weightThreshold, topNNeighbors, maxHubs,
    inputValidation, validatingInputs, creatingJob, showInputValidation, checkInputs, invalidateInputs,
    createJobAndUpload, loadDemo, runStep, removeSamples, restoreSamples, excludedSamples, refresh, loadSampleInfo, isDone, stepReady, stepStatus,
  }
})
