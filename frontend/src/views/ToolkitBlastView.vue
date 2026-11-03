<script setup lang="ts">
import { computed, ref } from 'vue'
import { blastMerge, type BlastMergeResult } from '../api/client'
import { locale } from '../utils/i18n'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import SimpleTable from '../components/SimpleTable.vue'

const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const blast = ref<File | null>(null)
const reference = ref<File | null>(null)
const base = ref<File | null>(null)
const maxEvalue = ref(1e-5)
const minPident = ref(0)
const busy = ref(false)
const error = ref('')
const result = ref<BlastMergeResult | null>(null)

function onBlast(e: Event) { blast.value = (e.target as HTMLInputElement).files?.[0] ?? null; result.value = null }
function onRef(e: Event) { reference.value = (e.target as HTMLInputElement).files?.[0] ?? null; result.value = null }
function onBase(e: Event) { base.value = (e.target as HTMLInputElement).files?.[0] ?? null; result.value = null }

async function merge() {
  if (!blast.value || !reference.value) return
  busy.value = true; error.value = ''
  try { result.value = await blastMerge(blast.value, reference.value, base.value, maxEvalue.value, minPident.value) }
  catch (e) { error.value = e instanceof Error ? e.message : String(e) }
  finally { busy.value = false }
}

function download(name: string, content: string) {
  const blob = new Blob([content], { type: 'text/tab-separated-values;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = name; a.click()
  URL.revokeObjectURL(url)
}

function preview(content: string, limit = 200) {
  const lines = content.split(/\r?\n/).filter((l) => l.length > 0)
  if (!lines.length) return { columns: [] as { key: string; label: string }[], rows: [] as Record<string, string>[] }
  const header = lines[0].split('\t').map((h) => h.trim())
  const rows = lines.slice(1, 1 + limit).map((line) => {
    const cells = line.split('\t')
    const row: Record<string, string> = {}
    header.forEach((h, i) => { row[h] = (cells[i] ?? '').trim() })
    return row
  })
  return { columns: header.map((h) => ({ key: h, label: h })), rows }
}
const previewData = computed(() => preview(result.value?.annotation ?? ''))
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <h6 class="fw-semibold mb-1">{{ text('BLAST 注释合并', 'BLAST annotation merge') }}</h6>
      <p class="text-muted small mb-3">{{ text('把 BLAST 结果和参考注释表合并成 annotation.tsv，补充或替换 symbol / gene_name。', 'Merge a BLAST result and reference annotation into annotation.tsv to supplement symbol / gene_name.') }}</p>

      <label class="form-label small mb-1">{{ text('BLAST 结果（outfmt 6）', 'BLAST result (outfmt 6)') }} *</label>
      <label class="btn btn-outline-secondary w-100 text-truncate mb-3"><i class="bi bi-upload me-1"></i>{{ blast ? blast.name : text('选择文件', 'Choose file') }}<input type="file" accept=".tsv,.txt" class="d-none" @change="onBlast" /></label>

      <label class="form-label small mb-1">{{ text('参考注释表（id → symbol/name/description）', 'Reference annotation') }} *</label>
      <label class="btn btn-outline-secondary w-100 text-truncate mb-3"><i class="bi bi-upload me-1"></i>{{ reference ? reference.name : text('选择文件', 'Choose file') }}<input type="file" accept=".tsv,.txt" class="d-none" @change="onRef" /></label>

      <label class="form-label small mb-1">{{ text('（可选）基础注释表', 'Optional base annotation') }}</label>
      <label class="btn btn-outline-secondary w-100 text-truncate mb-3"><i class="bi bi-upload me-1"></i>{{ base ? base.name : text('选择文件', 'Choose file') }}<input type="file" accept=".tsv,.txt" class="d-none" @change="onBase" /></label>

      <div class="row g-2 mb-3">
        <div class="col-6"><label class="form-label small mb-1">Max E-value</label><input type="number" class="form-control" v-model.number="maxEvalue" step="0.00001" min="0" /></div>
        <div class="col-6"><label class="form-label small mb-1">Min % identity</label><input type="number" class="form-control" v-model.number="minPident" min="0" max="100" /></div>
      </div>

      <button class="btn btn-primary w-100" :disabled="!blast || !reference || busy" @click="merge">{{ busy ? text('合并中…', 'Merging…') : text('合并', 'Merge') }}</button>
      <hr class="my-3" />
      <router-link to="/toolkits/guide" class="small"><i class="bi bi-book me-1"></i>{{ text('查看详细使用指引', 'View the detailed guide') }}</router-link>
    </template>

    <div class="mb-3"><h4 class="mb-1">{{ text('BLAST 注释合并结果', 'BLAST merge result') }}</h4><p class="text-muted small mb-0">{{ text('左侧上传 BLAST 结果与参考注释表，这里显示统计与合并后的 annotation.tsv 预览。', 'Upload the BLAST result and reference annotation on the left; stats and the merged annotation.tsv preview appear here.') }}</p></div>
    <div class="alert alert-info small">
      <i class="bi bi-info-circle me-1"></i>{{ text('推荐 BLAST 到同 kingdom 的模式物种：植物用 TAIR / RAP-DB，动物用 HGNC / MGI，跨物种用 UniProt Swiss-Prot。', 'BLAST to a model organism of the same kingdom: TAIR / RAP-DB for plants, HGNC / MGI for animals, UniProt Swiss-Prot cross-species.') }}
    </div>

    <div v-if="error" class="alert alert-danger">{{ error }}</div>
    <p v-if="!result && !busy" class="text-muted">{{ text('尚未合并。请在左侧上传文件后点击“合并”。', 'Nothing merged yet. Upload files on the left and click Merge.') }}</p>

    <template v-if="result">
      <div class="row g-2 mb-3">
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ result.stats.genes }}</div><div class="stat-label">{{ text('基因', 'Genes') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value text-success">{{ result.stats.from_blast }}</div><div class="stat-label">{{ text('来自 BLAST', 'From BLAST') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ result.stats.from_base }}</div><div class="stat-label">{{ text('来自基础表', 'From base') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ result.stats.blast_hits }}</div><div class="stat-label">{{ text('BLAST 命中数', 'BLAST hits') }}</div></div></div>
      </div>

      <div class="d-flex align-items-center gap-2 mb-2">
        <span class="text-muted small">{{ text('预览前 200 行', 'First 200 rows') }}</span>
        <button class="btn btn-sm btn-outline-primary ms-auto" @click="download('annotation.tsv', result.annotation)"><i class="bi bi-download me-1"></i>{{ text('下载完整文件', 'Download full file') }}</button>
      </div>
      <SimpleTable v-if="previewData.columns.length" :columns="previewData.columns" :rows="previewData.rows" filename="annotation" scroll-height="480px" />
      <p v-else class="text-muted">{{ text('没有可合并的结果。', 'No mergeable result.') }}</p>
    </template>
  </SidebarDrawer>
</template>

<style scoped>
.stat-tile { background: #fff; border: 1px solid #e2e9f0; border-radius: 10px; padding: 12px 14px; }
.stat-value { font-size: 22px; font-weight: 700; color: #0f766e; line-height: 1.1; }
.stat-label { font-size: 12px; color: #64748b; margin-top: 4px; }
</style>
