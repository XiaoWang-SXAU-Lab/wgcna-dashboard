<script setup lang="ts">
import { computed, ref } from 'vue'
import { convertEggnog, type EggnogConvertResult } from '../api/client'
import { locale } from '../utils/i18n'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import SimpleTable from '../components/SimpleTable.vue'

const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const file = ref<File | null>(null)
const kingdom = ref('plants')
const busy = ref(false)
const error = ref('')
const result = ref<EggnogConvertResult | null>(null)
const tab = ref<'annotation' | 'gene2go' | 'gene2kegg'>('annotation')

const kingdoms = computed(() => [
  ['plants', text('植物', 'Plants')],
  ['animals', text('动物', 'Animals')],
  ['fungi', text('真菌', 'Fungi')],
  ['protists', text('原生生物', 'Protists')],
  ['bacteria', text('细菌', 'Bacteria')],
  ['archaea', text('古菌', 'Archaea')],
  ['viruses', text('病毒', 'Viruses')],
  ['other', text('全部 / 不过滤', 'All / no filter')],
])

function onFile(e: Event) {
  file.value = (e.target as HTMLInputElement).files?.[0] ?? null
  result.value = null
}

async function convert() {
  if (!file.value) return
  busy.value = true; error.value = ''
  try { result.value = await convertEggnog(file.value, kingdom.value) }
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

const annPreview = computed(() => preview(result.value?.annotation ?? ''))
const goPreview = computed(() => preview(result.value?.gene2go ?? ''))
const koPreview = computed(() => preview(result.value?.gene2kegg ?? ''))
const activePreview = computed(() => tab.value === 'annotation' ? annPreview.value : tab.value === 'gene2go' ? goPreview.value : koPreview.value)

const tabs = [
  { key: 'annotation' as const, label: 'annotation.tsv', count: () => annPreview.value.rows.length },
  { key: 'gene2go' as const, label: 'gene2go.tsv', count: () => goPreview.value.rows.length },
  { key: 'gene2kegg' as const, label: 'gene2kegg.tsv', count: () => koPreview.value.rows.length },
]
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <h6 class="fw-semibold mb-1">{{ text('eggNOG 注释转换', 'eggNOG converter') }}</h6>
      <p class="text-muted small mb-3">{{ text('把 eggNOG-mapper 的 .annotations 结果转换成注释表、Gene2GO 和 Gene2KEGG。', 'Convert an eggNOG-mapper .annotations result into annotation / Gene2GO / Gene2KEGG.') }}</p>
      <label class="form-label small mb-1">{{ text('eggNOG .annotations 文件', 'eggNOG .annotations file') }}</label>
      <label class="btn btn-outline-secondary w-100 text-truncate mb-3">
        <i class="bi bi-upload me-1"></i>{{ file ? file.name : text('选择文件', 'Choose file') }}
        <input type="file" accept=".tsv,.txt,.annotations" class="d-none" @change="onFile" />
      </label>
      <label class="form-label small mb-1">{{ text('Kingdom（过滤 KEGG）', 'Kingdom (filters KEGG)') }}</label>
      <select v-model="kingdom" class="form-select mb-3">
        <option v-for="[v, l] in kingdoms" :key="v" :value="v">{{ l }}</option>
      </select>
      <button class="btn btn-primary w-100" :disabled="!file || busy" @click="convert">{{ busy ? text('转换中…', 'Converting…') : text('转换', 'Convert') }}</button>
      <p class="small text-muted mt-2 mb-0">{{ text('仅 Plants 内置 KEGG 背景；其它 kingdom 保留全部 KO。', 'Only Plants has a bundled KEGG background; other kingdoms keep all KO.') }}</p>
      <hr class="my-3" />
      <router-link to="/toolkits/guide" class="small"><i class="bi bi-book me-1"></i>{{ text('查看详细使用指引', 'View the detailed guide') }}</router-link>
    </template>

    <div class="mb-3"><h4 class="mb-1">{{ text('eggNOG 注释转换结果', 'eggNOG conversion result') }}</h4><p class="text-muted small mb-0">{{ text('左侧上传 .annotations 并选择 kingdom，这里显示统计与三张表的预览。', 'Upload the .annotations and pick a kingdom on the left; stats and table previews appear here.') }}</p></div>
    <div class="alert alert-info small">
      <i class="bi bi-info-circle me-1"></i>{{ text('需要 .annotations 文件？打开', 'Need the .annotations file? Open') }}
      <a href="http://eggnog-mapper.embl.de/" target="_blank" rel="noopener noreferrer">eggNOG-mapper</a>
      {{ text('在线版，上传蛋白序列后下载 annotations 结果。完整步骤见', 'online, upload proteins and download the annotations result. Full steps in') }}
      <router-link to="/toolkits/guide">{{ text('详细指引', 'the guide') }}</router-link>{{ text('。', '.') }}
    </div>

    <div v-if="error" class="alert alert-danger">{{ error }}</div>
    <p v-if="!result && !busy" class="text-muted">{{ text('尚未转换。请在左侧上传文件后点击“转换”。', 'Nothing converted yet. Upload a file on the left and click Convert.') }}</p>

    <template v-if="result">
      <div class="row g-2 mb-3">
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ result.stats.genes }}</div><div class="stat-label">{{ text('基因', 'Genes') }}</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ result.stats.go_count }}</div><div class="stat-label">GO ({{ result.stats.go_terms }} terms)</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value">{{ result.stats.ko_count }}</div><div class="stat-label">KO ({{ result.stats.ko_terms }} terms)</div></div></div>
        <div class="col-6 col-md-3"><div class="stat-tile"><div class="stat-value text-warning">{{ result.stats.ko_filtered }}</div><div class="stat-label">{{ text('被过滤 KO', 'KO filtered') }}</div></div></div>
      </div>

      <ul class="nav nav-tabs mb-2">
        <li v-for="tb in tabs" :key="tb.key" class="nav-item">
          <button class="nav-link" :class="{ active: tab === tb.key }" @click="tab = tb.key">{{ tb.label }} <span class="badge text-bg-light border ms-1">{{ tb.count() }}</span></button>
        </li>
      </ul>

      <div v-if="activePreview.columns.length" class="d-flex align-items-center gap-2 mb-2">
        <span class="text-muted small">{{ text('预览前 200 行', 'First 200 rows') }}</span>
        <button class="btn btn-sm btn-outline-primary ms-auto" @click="download(tab + '.tsv', result[tab])"><i class="bi bi-download me-1"></i>{{ text('下载完整文件', 'Download full file') }}</button>
      </div>
      <SimpleTable v-if="activePreview.columns.length" :columns="activePreview.columns" :rows="activePreview.rows" :filename="tab" scroll-height="480px" />
      <p v-else class="text-muted">{{ text('这张表没有内容。', 'This table is empty.') }}</p>
    </template>
  </SidebarDrawer>
</template>

<style scoped>
.stat-tile { background: #fff; border: 1px solid #e2e9f0; border-radius: 10px; padding: 12px 14px; }
.stat-value { font-size: 22px; font-weight: 700; color: #0f766e; line-height: 1.1; }
.stat-label { font-size: 12px; color: #64748b; margin-top: 4px; }
</style>
