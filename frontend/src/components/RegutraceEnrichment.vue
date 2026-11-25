<script setup lang="ts">
import { ref } from 'vue'
import { enrichRegutrace, type RegutraceEnrichmentResult } from '../api/client'
import { locale } from '../utils/i18n'

const props = defineProps<{ jobId: string; motifOptions?: string[] }>()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const geneSet = ref('')
const background = ref('')
const motifs = ref('')
const maxP = ref('')
const busy = ref(false)
const error = ref('')
const result = ref<RegutraceEnrichmentResult | null>(null)
let runSeq = 0

function parseGenes(value: string): string[] {
  return value.split(/[\s,;]+/).map((item) => item.trim()).filter(Boolean)
}
async function run() {
  if (!props.jobId) return
  const seq = ++runSeq
  busy.value = true
  error.value = ''
  result.value = null
  try {
    const maxPValue = maxP.value.trim() === '' ? undefined : Number(maxP.value)
    const res = await enrichRegutrace(props.jobId, {
      gene_set: parseGenes(geneSet.value),
      background: background.value.trim() ? parseGenes(background.value) : undefined,
      motif_ids: parseGenes(motifs.value),
      max_p_value: Number.isFinite(maxPValue as number) ? maxPValue : undefined,
    })
    if (seq === runSeq) result.value = res
  } catch (e) {
    if (seq === runSeq) error.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (seq === runSeq) busy.value = false
  }
}
function formatP(value: number): string {
  if (!Number.isFinite(value)) return '—'
  return value < 1e-3 || value >= 1e4 ? value.toExponential(2) : value.toPrecision(3)
}
function fillTopMotifs() {
  motifs.value = (props.motifOptions ?? []).slice(0, 10).join(', ')
}
</script>

<template>
  <div class="card mb-3">
    <div class="card-body">
      <div class="d-flex align-items-start flex-wrap gap-2 mb-2">
        <div>
          <h6 class="mb-0">{{ text('Motif 富集（探索性）', 'Motif enrichment (exploratory)') }}</h6>
          <p class="small text-muted mb-0">
            {{ text('对基因集的二元 promoter 命中做单侧 Fisher 检验，并在所选 motif 内做 BH。不是实验验证的调控富集。', 'One-sided Fisher test on binary promoter hits, with BH across selected motifs. Not experimentally validated regulatory enrichment.') }}
          </p>
        </div>
        <button v-if="motifOptions?.length" class="btn btn-sm btn-outline-secondary ms-auto" @click="fillTopMotifs">{{ text('填入 top motifs', 'Fill top motifs') }}</button>
      </div>
      <div class="row g-2">
        <div class="col-12 col-md-6">
          <label class="form-label small mb-1">{{ text('基因集（每行/逗号分隔）', 'Gene set (newline/comma separated)') }}</label>
          <textarea v-model="geneSet" class="form-control form-control-sm font-monospace" rows="4" :placeholder="text('GENE1\\nGENE2', 'GENE1\\nGENE2')"></textarea>
        </div>
        <div class="col-12 col-md-6">
          <label class="form-label small mb-1">{{ text('背景基因（留空 = 全部已扫描 promoter）', 'Background genes (empty = all scanned promoters)') }}</label>
          <textarea v-model="background" class="form-control form-control-sm font-monospace" rows="4" :placeholder="text('显式 universe；建议与基因集同一 ID 命名空间', 'Explicit universe; use the same gene-ID namespace')"></textarea>
        </div>
        <div class="col-12 col-md-8">
          <label class="form-label small mb-1">{{ text('Motif IDs（逗号分隔，最多 200）', 'Motif IDs (comma separated, max 200)') }}</label>
          <input v-model="motifs" class="form-control form-control-sm font-monospace" placeholder="MA0001.1, MA0574.1" />
        </div>
        <div class="col-6 col-md-2">
          <label class="form-label small mb-1">max p</label>
          <input v-model="maxP" class="form-control form-control-sm" placeholder="optional" />
        </div>
        <div class="col-6 col-md-2">
          <button class="btn btn-primary btn-sm w-100" :disabled="busy" @click="run">{{ busy ? text('计算中…', 'Running…') : text('运行', 'Run') }}</button>
        </div>
      </div>
      <div v-if="error" class="alert alert-danger small py-2 mt-2">{{ error }}</div>
      <div v-if="result" class="mt-3">
        <div class="small text-muted mb-2">
          {{ text('基因集', 'Gene set') }} {{ result.gene_set_size.toLocaleString() }} ·
          {{ text('背景', 'Background') }} {{ result.background_size.toLocaleString() }} ·
          {{ result.background_source }} · {{ result.multiple_testing_scope }}
        </div>
        <div class="table-responsive">
          <table class="table table-sm">
            <thead><tr><th>Motif</th><th>{{ text('集合命中', 'Set hits') }}</th><th>{{ text('背景命中', 'Background hits') }}</th><th>fold</th><th>OR</th><th>p</th><th>BH q</th><th>{{ text('贡献基因', 'Contributing genes') }}</th></tr></thead>
            <tbody><tr v-for="row in result.rows" :key="row.motif_id">
              <td><span class="font-monospace">{{ row.motif_id }}</span><div class="small text-muted">{{ row.motif_name }}</div></td>
              <td>{{ row.observed_motif_genes }} / {{ row.gene_set_size }}</td>
              <td>{{ row.background_motif_genes }} / {{ row.background_size }}</td>
              <td>{{ row.enrichment_ratio.toFixed(2) }}</td>
              <td>{{ row.odds_ratio.toFixed(2) }}</td>
              <td>{{ formatP(row.p_value) }}</td>
              <td>{{ formatP(row.bh_q_value) }}</td>
              <td class="small">{{ row.contributing_genes.slice(0, 8).join(', ') }}<span v-if="row.contributing_genes_truncated || row.contributing_genes.length > 8"> …</span></td>
            </tr></tbody>
          </table>
        </div>
        <ul class="small text-muted mb-0 ps-3">
          <li v-for="caveat in result.caveats" :key="caveat">{{ caveat }}</li>
        </ul>
      </div>
    </div>
  </div>
</template>
