<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useProjectStore } from '../stores/project'
import { getHubPriorityEvidence, projectOutputUrl } from '../api/projects'
import type { HubPriorityEvidence } from '../api/client'
import { useCandidateReport } from '../composables/useCandidateReport'
import { locale } from '../utils/i18n'

const store = useProjectStore()
const text = (zh: string, en: string) => (locale.value === 'zh' ? zh : en)
const { report, loading, loadError, reportCurrent, load } = useCandidateReport()

// Deterministic offline evidence interpretation. The server rebuilds the bundle
// from the current atomic report envelope; this panel only displays the returned
// safe message/code and never calls an online model.
const evidence = ref<HubPriorityEvidence | null>(null)
const evidenceLoading = ref(false)
const evidenceError = ref('')
let evidenceReq = 0

async function loadEvidence() {
  if (!report.value || !reportCurrent.value) return
  const pid = store.id
  const execId = report.value.execution_id
  const req = ++evidenceReq
  evidenceLoading.value = true
  evidenceError.value = ''
  try {
    const data = await getHubPriorityEvidence(pid)
    if (req !== evidenceReq) return
    // The server may have moved to another execution between the report load
    // and this call; never display a mismatched interpretation.
    if (data.project_id !== store.id || data.execution_id !== execId) {
      evidence.value = null
      evidenceError.value = text('服务端当前执行已变化，请刷新报告后重试。', 'The server current execution changed; refresh the report and retry.')
      return
    }
    evidence.value = data
  } catch (e) {
    if (req !== evidenceReq) return
    const err = e as Error & { code?: string }
    evidence.value = null
    evidenceError.value = err.code ? `${err.code}: ${err.message}` : err.message
  } finally {
    if (req === evidenceReq) evidenceLoading.value = false
  }
}

watch(() => [store.id, report.value?.execution_id, reportCurrent.value], () => {
  evidence.value = null
  evidenceError.value = ''
  evidenceLoading.value = false
  evidenceReq++
})

const result = computed(() => report.value?.result)

// Upstream/inputs/params come from the REPORT's own envelope snapshot, never from
// the live store (which may already point at a different execution).
const upstream = computed(() => ({
  filter: report.value?.upstream.filter ?? '—',
  network: report.value?.upstream.network ?? '—',
}))

function downloadReportJson() {
  if (!report.value) return
  const blob = new Blob([JSON.stringify(report.value, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `hub_priority_report_${report.value.execution_id}.json`
  a.click()
  URL.revokeObjectURL(url)
}

const rows = computed(() => {
  const genes = result.value?.genes ?? []
  const valid = result.value?.valid ?? 0
  return [...genes]
    .sort((a, b) => {
      const am = a.midrank_median
      const bm = b.midrank_median
      if (am == null && bm == null) return a.gene.localeCompare(b.gene)
      if (am == null) return 1
      if (bm == null) return -1
      return am - bm
    })
    .map((g) => ({
      gene: g.gene,
      kwithin: g.kwithin.toFixed(2),
      kmeAbs: g.kme_abs.toFixed(2),
      rank: `${g.rank_lower}-${g.rank_upper}`,
      tie: g.rank_lower < g.rank_upper,
      midrank: g.midrank_median != null ? g.midrank_median.toFixed(1) : '—',
      p05: g.midrank_p05 != null ? g.midrank_p05.toFixed(1) : '—',
      p95: g.midrank_p95 != null ? g.midrank_p95.toFixed(1) : '—',
      freqPct: g.mean_top_k != null ? Math.round(g.mean_top_k * 100) : null,
      freqCount: g.mean_top_k != null && valid > 0 ? Math.round(g.mean_top_k * valid * 10) / 10 : null,
    }))
})

const showAll = ref(false)
const visibleRows = computed(() => (showAll.value ? rows.value : rows.value.slice(0, 10)))
const topGene = computed(() => rows.value[0] ?? null)

const summaryLine = computed(() => {
  const r = result.value
  const top = topGene.value
  if (!r) return ''
  if (r.valid === 0 || !top || top.midrank === '—') {
    return text('本次重抽样没有有效轮次，无法给出候选基因。', 'No valid resampling rounds; no candidate can be recommended.')
  }
  const freq = top.freqPct == null ? '—' : `${top.freqPct}%`
  const freqText = top.freqPct === 100
    ? text(`每一次都进入前 ${r.top_k} 名`, `entered the top ${r.top_k} in every round`)
    : text(`平均进入前 ${r.top_k} 名的比例是 ${freq}`, `entered the top ${r.top_k} in ${freq} of rounds on average`)
  const tied = top.tie
    ? text('它和部分基因的排名区间重叠，不能认为它明显优于这些基因。', 'Its rank interval overlaps other genes; do not treat it as clearly better.')
    : ''
  const invalid = r.invalid > 0
    ? text(`本次 ${r.attempted} 次抽样中有 ${r.invalid} 次无效，结果只基于 ${r.valid} 次有效抽样。`, `${r.invalid} of ${r.attempted} resamples were invalid; results use only the ${r.valid} valid rounds.`)
    : ''
  return text(
    `在这个模块的 ${r.gene_count} 个基因里，${top.gene} 的模块内连接排名最稳定：${r.valid} 次有效抽样中，它${freqText}。${tied}${invalid}`,
    `Among the ${r.gene_count} genes in this module, ${top.gene} has the most stable within-module connectivity rank: across ${r.valid} valid resamples it ${freqText}.${tied ? ' ' + tied : ''}${invalid ? ' ' + invalid : ''}`,
  )
})

const plainLimitation = computed(() => {
  const r = result.value
  if (!r) return ''
  return text(
    `这项分析只使用了网络连接信息，没有使用性状/表型数据（本次 trait_source = none）。它只能说明 ${r.module} 模块里哪些基因的连接排名比较稳定；不能说明这些基因与任何处理、表型、功能或因果有关。`,
    `This analysis used network connectivity only, with no trait/phenotype data (trait_source = none). It can only show which genes in module ${r.module} have stable connectivity ranks; it says nothing about association with any treatment, phenotype, function, or causation.`,
  )
})

const qualityNote = computed(() => {
  const r = result.value
  if (!r) return ''
  if (r.valid === 0) return text('本次重抽样全部无效，不能推荐候选。', 'All resampling rounds were invalid; no candidate can be recommended.')
  if (r.invalid > 0) return text(`本次 ${r.attempted} 次抽样中有 ${r.invalid} 次无效；结果只基于有效的 ${r.valid} 次。`, `${r.invalid} of ${r.attempted} resamples were invalid; results use only the ${r.valid} valid rounds.`)
  if (r.direction_undetermined > 0) return text(`另有 ${r.direction_undetermined} 轮模块特征向量方向无法确定；这只影响描述性指标，不影响连接排名。`, `${r.direction_undetermined} rounds had an undetermined module eigengene direction; this affects descriptive metrics only, not the connectivity ranking.`)
  return ''
})

const methodSteps = computed(() => {
  const r = result.value
  if (!r) return []
  return [
    text(`把表达模式相似的基因归为一个模块；本模块是 ${r.module}，共 ${r.gene_count} 个基因。`, `Genes with similar expression patterns were grouped into one module; this is ${r.module}, with ${r.gene_count} genes.`),
    text('计算每个基因与模块内其他基因的连接紧密程度（kWithin，越高表示连接越紧）。', 'Each gene’s connectivity to the other genes in the module was measured (kWithin; higher means more tightly connected).'),
    text(`为了看排名是否稳定，随机重复抽样 ${r.n_bootstraps} 次，每次重新计算排名并取前 ${r.top_k} 名（K = ${r.top_k}）。`, `To test whether the ranking is stable, the analysis resampled ${r.n_bootstraps} times, recomputing the ranking and taking the top ${r.top_k} each time (K = ${r.top_k}).`),
    text('统计每个基因在多次抽样中的典型排名，以及进入前 K 名的比例。', 'For each gene, the analysis summarizes its typical rank across resamples and how often it entered the top K.'),
    text('排名区间重叠的基因视为并列，不强行分出先后。', 'Genes with overlapping rank intervals are treated as tied; no forced ordering is applied.'),
  ]
})

const cannotConclude = computed(() => [
  text('不是“核心调控基因”或“真正的 hub”的证明。', 'It is not proof of a “core regulator” or a “true hub.”'),
  text('不能说明与性状、处理或表型相关（本次没有性状数据）。', 'It does not show association with any trait, treatment, or phenotype (no trait data were used).'),
  text('不是概率、p 值、FDR 或置信区间；进入前 K 名的比例只是抽样频率。', 'It is not a probability, p-value, FDR, or confidence interval; the top-K proportion is only a resampling frequency.'),
  text('没有实验验证，不能推断因果或功能。', 'There is no experimental validation; causation or function cannot be inferred.'),
  text('排名并列时，不能强行选唯一基因。', 'When ranks are tied, do not force a single gene choice.'),
])

const nextSteps = computed(() => [
  text('先查候选基因的注释、同源基因和已有文献。', 'Check the candidate’s annotation, homologs, and existing literature first.'),
  text('如果有表型/处理数据，补跑“模块-性状”分析，再看基因与性状的关系（GS/MM）。', 'If trait/treatment data exist, run the module–trait analysis and then examine gene–trait relationships (GS/MM).'),
  text('用 qPCR、突变体、过表达等实验验证候选。', 'Validate candidates experimentally (qPCR, mutants, overexpression, etc.).'),
  text('把候选表当作筛选清单，不是最终结论。', 'Treat the candidate table as a shortlist, not a final conclusion.'),
])
</script>

<template>
  <div v-if="!store.current" class="text-center py-5">
    <i class="bi bi-folder2-open fs-1 text-muted"></i>
    <h4 class="mt-3">{{ text('尚未打开项目', 'No project open') }}</h4>
    <router-link to="/projects" class="btn btn-primary mt-2">{{ text('前往项目', 'Go to projects') }}</router-link>
  </div>
  <div v-else>
    <div class="mb-3">
      <h4 class="mb-1">{{ text('候选基因优先级（网络稳定性）', 'Candidate gene priority (network stability)') }}</h4>
      <p class="text-muted small mb-0">{{ text('给生物学家的解读：这个模块里哪些基因的连接排名最稳定，以及这些结果能说明和不能说明什么。', 'A biologist-facing interpretation: which genes in this module have the most stable connectivity ranks, and what these results can and cannot tell you.') }}</p>
    </div>

    <div class="d-flex gap-2 mb-3">
      <button class="btn btn-sm btn-primary" :disabled="loading || store.saving || store.draftDirty" @click="load">
        <i class="bi bi-arrow-clockwise me-1"></i>{{ text('生成/刷新报告预览', 'Generate / refresh preview') }}
      </button>
      <button v-if="report" class="btn btn-sm btn-outline-primary" @click="downloadReportJson"><i class="bi bi-download me-1"></i>{{ text('完整报告 JSON（含来源/参数/全部候选）', 'Full report JSON (source/params/all candidates)') }}</button>
      <a v-if="report && reportCurrent" class="btn btn-sm btn-outline-secondary" :href="projectOutputUrl(store.id, 'hub_priority', 'step8_hub_priority.tsv')"><i class="bi bi-filetype-tsv me-1"></i>{{ text('候选 TSV（仅候选表，不含来源）', 'Candidate TSV (candidate table only)') }}</a>
    </div>
    <p v-if="store.draftDirty || store.saving" class="text-muted small mb-2">{{ text('存在未保存的参数修改，报告预览/导出已暂停；保存后再生成。', 'Unsaved parameter edits exist; report preview/export is paused until saved.') }}</p>

    <div v-if="loading" class="text-muted small">{{ text('加载中…', 'Loading…') }}</div>
    <div v-else-if="loadError" class="alert alert-warning py-2 small"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('报告不可用：', 'Report unavailable: ') }}{{ loadError }}</div>
    <div v-else-if="!report" class="text-muted">{{ text('尚未运行候选优先级，或结果已过期。请先在「模块解析 → 候选优先级」运行，再回到此处生成报告。', 'Candidate priority has not run, or its result is stale. Run it under "Module explorer → Candidate priority", then generate the report here.') }}</div>

    <template v-else>
      <div v-if="!reportCurrent" class="alert alert-warning py-2 small mb-3"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('这份结果属于较早的一次运行，以下内容只作为历史证据，不代表项目当前状态。', 'This result is from an earlier run; the content below is historical evidence, not the project’s current state.') }}</div>

      <div class="alert alert-primary">
        <h5 class="alert-heading mb-2">{{ text('一句话结论', 'In one sentence') }}</h5>
        <p class="mb-1">{{ summaryLine }}</p>
        <p class="mb-0 small">{{ plainLimitation }}</p>
        <p v-if="qualityNote" class="mb-0 small text-muted mt-1">{{ qualityNote }}</p>
      </div>

      <div class="card p-3 mb-3">
        <h6 class="mb-2">{{ text('这项分析做了什么', 'What this analysis did') }}</h6>
        <ol class="small mb-0">
          <li v-for="(step, i) in methodSteps" :key="i" class="mb-1">{{ step }}</li>
        </ol>
      </div>

      <div class="card p-3 mb-3">
        <div class="d-flex justify-content-between align-items-center mb-2">
          <h6 class="mb-0">{{ text('结果怎么读：优先关注哪些基因', 'How to read the result: which genes to prioritize') }}</h6>
          <button class="btn btn-sm btn-outline-secondary" @click="showAll = !showAll">
            {{ showAll ? text('只显示前 10 行', 'Show top 10 only') : text(`显示全部 ${rows.length} 行`, `Show all ${rows.length} rows`) }}
          </button>
        </div>
        <p class="text-muted small mb-2">{{ text('稳定性排名越低越稳定；它不是表达量高低，也不是差异倍数。排名波动是多次抽样中的典型范围，不是置信区间。', 'A lower stability rank means more stable; it is not expression level or fold change. The rank range is the typical spread across resamples, not a confidence interval.') }}</p>
        <div class="table-responsive">
          <table class="table table-sm table-striped small mb-0">
            <thead>
              <tr>
                <th>{{ text('基因', 'Gene') }}</th>
                <th>{{ text('稳定性排名（中位数）', 'Stability rank (median)') }}</th>
                <th>{{ text('进入前 K 名的比例', 'Top-K proportion') }}</th>
                <th>{{ text('排名波动（P05–P95）', 'Rank spread (P05–P95)') }}</th>
                <th>{{ text('是否并列', 'Tied?') }}</th>
                <th>{{ text('怎么理解', 'How to read it') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(r, i) in visibleRows" :key="r.gene">
                <td><b>{{ r.gene }}</b></td>
                <td>{{ r.midrank }}</td>
                <td>{{ r.freqPct != null ? r.freqPct + '%' : '—' }}<span v-if="r.freqCount != null" class="text-muted">（{{ result?.valid }} 次中约 {{ r.freqCount }} 次）</span></td>
                <td>{{ r.p05 }} – {{ r.p95 }}</td>
                <td>{{ r.tie ? text('是', 'Yes') : text('否', 'No') }}</td>
                <td>{{ i === 0 ? text('最稳定候选，优先关注', 'Most stable; prioritize') : r.tie ? text('并列，不能区分', 'Tied; indistinguishable') : text('候选，次于前者', 'Candidate; lower priority') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card p-3 mb-3 border-warning">
        <h6 class="mb-2">{{ text('不能从这个报告得出的结论', 'What this report does NOT show') }}</h6>
        <ul class="small mb-0">
          <li v-for="(line, i) in cannotConclude" :key="i" class="mb-1">{{ line }}</li>
        </ul>
      </div>

      <div class="card p-3 mb-3">
        <h6 class="mb-2">{{ text('下一步可以做什么', 'What to do next') }}</h6>
        <ul class="small mb-0">
          <li v-for="(line, i) in nextSteps" :key="i" class="mb-1">{{ line }}</li>
        </ul>
      </div>

      <details class="card p-3 mb-3">
        <summary class="fw-semibold">{{ text('技术明细（给需要复核的人）', 'Technical details (for review)') }}</summary>
        <div class="mt-3">
          <h6 class="mb-2">{{ text('方法与范围', 'Method & scope') }}</h6>
          <div class="row small">
            <div class="col-md-6">
              <div>{{ text('模块', 'Module') }}：<b>{{ result?.module }}</b>（{{ result?.gene_count }} {{ text('基因', 'genes') }} · {{ result?.sample_count }} {{ text('样本', 'samples') }}）</div>
              <div>{{ text('方法版本', 'Method version') }}：{{ result?.method_version }} · scope {{ result?.scope }}</div>
              <div>power {{ result?.power }} · {{ result?.network_type }} · K {{ result?.top_k }} · B {{ result?.n_bootstraps }} · seed {{ result?.seed }}</div>
              <div>{{ text('重采样', 'Resampling') }}：{{ result?.resampling }}（design_confirmed {{ result?.design_confirmed }}）</div>
              <div v-if="result?.strata_column">{{ text('分层列', 'Strata column') }}：{{ result?.strata_column }}（{{ result?.stratum_count }} {{ text('层', 'strata') }}：{{ result?.strata_counts.join(', ') }}）</div>
            </div>
            <div class="col-md-6">
              <div>{{ text('有效/尝试/无效', 'Valid / attempted / invalid') }}：{{ result?.valid }} / {{ result?.attempted }} / {{ result?.invalid }}</div>
              <div v-if="result?.invalid_reasons?.length">{{ text('无效原因', 'Invalid reasons') }}：{{ result?.invalid_reasons.map((r) => `${r.reason}×${r.count}`).join('、') }}</div>
              <div>{{ text('方向不确定轮数', 'Undetermined-direction rounds') }}：{{ result?.direction_undetermined }}</div>
              <div>{{ text('报告执行', 'Report execution') }}：{{ report.execution_id }}（engine {{ report.engine }}）</div>
              <div>{{ text('上游执行', 'Upstream execution') }}：filter {{ upstream.filter }} · network {{ upstream.network }}</div>
              <div class="text-muted">{{ result?.resource_note }}</div>
            </div>
          </div>

          <h6 class="mb-2 mt-3">{{ text('完整候选表（原始指标）', 'Full candidate table (raw metrics)') }}</h6>
          <div class="table-responsive" style="max-height: 420px; overflow: auto">
            <table class="table table-sm table-striped small mb-0">
              <thead class="sticky-top">
                <tr>
                  <th>Gene</th><th>kWithin</th><th>|kME|</th><th>{{ text('原始排名', 'Orig rank') }}</th><th>{{ text('中位排名', 'Median rank') }}</th><th>P05</th><th>P95</th><th>{{ text('前 K 比例', 'Top-K fraction') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in rows" :key="r.gene">
                  <td>{{ r.gene }}</td><td>{{ r.kwithin }}</td><td>{{ r.kmeAbs }}</td><td>{{ r.rank }}</td><td>{{ r.midrank }}</td><td>{{ r.p05 }}</td><td>{{ r.p95 }}</td><td>{{ r.freqPct != null ? (r.freqPct / 100).toFixed(3) : '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <h6 class="mb-2 mt-3">{{ text('服务端离线证据（技术文本）', 'Server-side offline evidence (technical text)') }}</h6>
          <div class="d-flex justify-content-between align-items-center mb-2">
            <p class="text-muted small mb-0">{{ text('由服务端根据当前原子报告快照重建事实包，只做确定性模板渲染；不调用在线模型、不执行建议。', 'The server rebuilds the fact bundle from the current atomic report snapshot and renders deterministic templates only; no online model, no suggestion execution.') }}</p>
            <button class="btn btn-sm btn-outline-primary" :disabled="evidenceLoading || !reportCurrent || store.draftDirty || store.saving" @click="loadEvidence">
              <i class="bi bi-shield-check me-1"></i>{{ evidenceLoading ? text('生成中…', 'Generating…') : text('生成服务端离线解读', 'Generate server-side offline interpretation') }}
            </button>
          </div>
          <div v-if="evidenceError" class="alert alert-warning py-2 small mb-0"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('解读不可用：', 'Interpretation unavailable: ') }}{{ evidenceError }}</div>
          <div v-else-if="evidence" class="small">
            <div class="text-muted mb-2">
              provenance {{ evidence.provenance.slice(0, 12) }}… · execution {{ evidence.execution_id }} · {{ evidence.valid }}/{{ evidence.invalid }} · {{ evidence.candidate_count }} {{ text('候选', 'candidates') }}
            </div>
            <pre class="mb-0" style="white-space: pre-wrap; font-family: inherit">{{ evidence.rendered }}</pre>
          </div>
          <div v-else class="text-muted small">{{ text('尚未生成；仅在当前执行上可用。', 'Not generated yet; available only for the current execution.') }}</div>
        </div>
      </details>
    </template>
  </div>
</template>
