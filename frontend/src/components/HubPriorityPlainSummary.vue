<script setup lang="ts">
import { computed, ref } from 'vue'
import type { AnnotationRow, HubPriorityResult } from '../api/client'
import { locale } from '../utils/i18n'
import SimpleTable from './SimpleTable.vue'

const props = defineProps<{ result?: HubPriorityResult; annotations?: AnnotationRow[] }>()
const text = (zh: string, en: string) => (locale.value === 'zh' ? zh : en)

const result = computed(() => props.result)
const annotationMap = computed(() => new Map((props.annotations ?? []).map((a) => [a.gene_id, a])))

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
    .map((g, i) => {
      const a = annotationMap.value.get(g.gene)
      const tie = g.rank_lower < g.rank_upper
      return {
        gene: g.gene,
        symbol: a?.symbol || a?.gene_name || '—',
        description: a?.description || '—',
        tie,
        midrank: g.midrank_median != null ? g.midrank_median.toFixed(1) : '—',
        p05: g.midrank_p05 != null ? g.midrank_p05.toFixed(1) : '—',
        p95: g.midrank_p95 != null ? g.midrank_p95.toFixed(1) : '—',
        freqPct: g.mean_top_k != null ? Math.round(g.mean_top_k * 100) : null,
        freqCount: g.mean_top_k != null && valid > 0 ? Math.round(g.mean_top_k * valid * 10) / 10 : null,
        meaning: i === 0
          ? text('最稳定候选，优先关注', 'Most stable; prioritize')
          : tie
            ? text('并列，不能区分', 'Tied; indistinguishable')
            : text('候选，次于前者', 'Candidate; lower priority'),
      }
    })
})

const showAll = ref(false)
const visibleRows = computed(() => (showAll.value ? rows.value : rows.value.slice(0, 10)))
const topGene = computed(() => rows.value[0] ?? null)

const candidateColumns = computed(() => [
  { key: 'gene', label: text('基因', 'Gene') },
  { key: 'symbol', label: text('符号/名称', 'Symbol / name') },
  { key: 'description', label: text('功能描述', 'Description') },
  { key: 'midrank', label: text('稳定性排名（中位数）', 'Stability rank (median)') },
  {
    key: 'freqPct',
    label: text('进入前 K 名的比例', 'Top-K proportion'),
    fmt: (v: any, r: Record<string, any>) => {
      if (v == null) return '—'
      const suffix = r.freqCount != null
        ? text(`（${result.value?.valid ?? '—'} 次中约 ${r.freqCount} 次）`, ` (about ${r.freqCount} of ${result.value?.valid ?? '—'} rounds)`)
        : ''
      return `${v}%${suffix}`
    },
  },
  {
    key: 'p05',
    label: text('排名波动（P05–P95）', 'Rank spread (P05–P95)'),
    fmt: (_v: any, r: Record<string, any>) => `${r.p05} – ${r.p95}`,
  },
  { key: 'tie', label: text('是否并列', 'Tied?'), fmt: (v: any) => (v ? text('是', 'Yes') : text('否', 'No')) },
  { key: 'meaning', label: text('怎么理解', 'How to read it') },
])

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
    `这项分析只使用了网络连接信息，没有使用性状/表型数据。它只能说明 ${r.module} 模块里哪些基因的连接排名比较稳定；不能说明这些基因与任何处理、表型、功能或因果有关。`,
    `This analysis used network connectivity only, with no trait/phenotype data. It can only show which genes in module ${r.module} have stable connectivity ranks; it says nothing about association with any treatment, phenotype, function, or causation.`,
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
  <div v-if="result">
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
      <p class="text-muted small mb-2">{{ text('稳定性排名越低越稳定；它不是表达量高低，也不是差异倍数。排名波动是多次抽样中的典型范围，不是置信区间。Symbol / Description 来自上传的 annotation.tsv；— 表示未注释。', 'A lower stability rank means more stable; it is not expression level or fold change. The rank range is the typical spread across resamples, not a confidence interval. Symbol / Description come from the uploaded annotation.tsv; — means not annotated.') }}</p>
      <SimpleTable :columns="candidateColumns" :rows="visibleRows" scroll-height="420px" filename="candidate_priority" />
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
  </div>
</template>
