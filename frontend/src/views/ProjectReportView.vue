<script setup lang="ts">
import { computed } from 'vue'
import { useProjectStore } from '../stores/project'
import { projectOutputUrl } from '../api/projects'
import { useCandidateReport } from '../composables/useCandidateReport'
import { locale } from '../utils/i18n'

const store = useProjectStore()
const text = (zh: string, en: string) => (locale.value === 'zh' ? zh : en)
const { report, loading, loadError, reportCurrent, load } = useCandidateReport()

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
      kwithin: g.kwithin.toFixed(4),
      kme_signed: g.kme_signed.toFixed(4),
      kme_abs: g.kme_abs.toFixed(4),
      rank: `${g.rank_lower}-${g.rank_upper}`,
      midrank_median: g.midrank_median != null ? g.midrank_median.toFixed(2) : '—',
      midrank_p05: g.midrank_p05 != null ? g.midrank_p05.toFixed(2) : '—',
      midrank_p95: g.midrank_p95 != null ? g.midrank_p95.toFixed(2) : '—',
      mean_top_k: g.mean_top_k != null ? g.mean_top_k.toFixed(3) : '—',
    }))
})

const interpretation = computed(() => {
  const r = result.value
  if (!r) return []
  const out: string[] = []
  out.push(text('无表型证据：本报告仅基于网络 kWithin 排名稳定性，属「网络候选优先级」，不含基因-性状（GS）证据。', 'No trait evidence: this report is network-only candidate priority by kWithin ranking stability; no gene-trait (GS) evidence.'))
  if (r.valid === 0) {
    out.push(text('所有 bootstrap 轮均无效：仅保留原始 kWithin/kME 与质量说明，禁止给出候选推荐。', 'All bootstrap rounds were invalid: only original kWithin/kME and a quality note are kept; no candidate recommendation.'))
  } else if (r.invalid > 0) {
    out.push(text(`部分重复无效（${r.invalid}/${r.attempted}）：排名条件于可计算重复，不宣称可靠候选。`, `Some replicates were invalid (${r.invalid}/${r.attempted}): ranking is conditioned on computable replicates, not reliable candidates.`))
  }
  out.push(text('midrank 中位数 / P05 / P95 是 bootstrap 采样分布的经验区间，不是置信区间，也不是显著性。', 'midrank median / P05 / P95 are empirical bootstrap intervals, not confidence intervals or significance.'))
  out.push(text('并列跨前 K 边界按分数计入；同频率或重叠排名区间视为不可区分，不强制唯一推荐。', 'Ties across the top-K boundary are counted fractionally; equal frequency / overlapping rank intervals are indistinguishable, with no forced unique recommendation.'))
  out.push(text('本报告不合成总分，也不把候选称为「真正 hub」。', 'This report composes no composite score and does not label candidates as "true hubs".'))
  if (r.direction_undetermined > 0) {
    out.push(text(`有 ${r.direction_undetermined} 轮 eigengene 方向无法确定（仅描述性，不影响 kWithin 排名）。`, `${r.direction_undetermined} rounds had an undetermined eigengene direction (descriptive only; does not affect kWithin ranking).`))
  }
  return out
})
</script>

<template>
  <div v-if="!store.current" class="text-center py-5">
    <i class="bi bi-folder2-open fs-1 text-muted"></i>
    <h4 class="mt-3">{{ text('尚未打开项目', 'No project open') }}</h4>
    <router-link to="/projects" class="btn btn-primary mt-2">{{ text('前往项目', 'Go to projects') }}</router-link>
  </div>
  <div v-else>
    <div class="mb-3">
      <h4 class="mb-1">{{ text('候选优先级证据报告', 'Candidate priority evidence report') }}</h4>
      <p class="text-muted small mb-0">{{ text('当前项目 · 候选优先级（hub_priority）· 固定完整模块 · 网络候选优先级（无表型证据）', 'Current project · candidate priority (hub_priority) · fixed full module · network candidate priority (no trait evidence)') }}</p>
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
      <div v-if="!reportCurrent" class="alert alert-warning py-2 small mb-3"><i class="bi bi-exclamation-triangle-fill me-1"></i>{{ text('本报告属于较早执行（execution ', 'This report is from an earlier execution (') }}{{ report.execution_id }}{{ text('），以下为历史证据，不视为当前事实。', '); shown as history, not current fact.') }}</div>

      <div class="card p-3 mb-3">
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
      </div>

      <div class="card p-3 mb-3">
        <h6 class="mb-2">{{ text('确定性解释与局限', 'Deterministic interpretation & limitations') }}</h6>
        <ul class="small mb-0">
          <li v-for="(line, i) in interpretation" :key="i">{{ line }}</li>
        </ul>
      </div>

      <div class="card p-3">
        <h6 class="mb-2">{{ text('候选表（完整，按 midrank 中位数升序）', 'Candidates (full, by midrank median ascending)') }} · {{ rows.length }} {{ text('行', 'rows') }}</h6>
        <div class="table-responsive" style="max-height: 520px; overflow: auto">
          <table class="table table-sm table-striped small mb-0">
            <thead class="sticky-top">
              <tr>
                <th>Gene</th><th>kWithin</th><th>kME (signed)</th><th>|kME|</th><th>{{ text('原始排名', 'Orig rank') }}</th><th>midrank 中位数</th><th>P05</th><th>P95</th><th>top-K 频率</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in rows" :key="r.gene">
                <td>{{ r.gene }}</td><td>{{ r.kwithin }}</td><td>{{ r.kme_signed }}</td><td>{{ r.kme_abs }}</td><td>{{ r.rank }}</td><td>{{ r.midrank_median }}</td><td>{{ r.midrank_p05 }}</td><td>{{ r.midrank_p95 }}</td><td>{{ r.mean_top_k }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>
