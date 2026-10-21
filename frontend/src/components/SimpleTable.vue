<script setup lang="ts">
import { computed, ref } from 'vue'
import { t } from '../utils/i18n'

type Column = { key: string; label: string; fmt?: (v: any) => string }

const props = defineProps<{ columns: Column[]; rows: Record<string, any>[]; scrollHeight?: string; filename?: string }>()

const search = ref('')
const sortKey = ref('')
const sortAsc = ref(true)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return props.rows
  return props.rows.filter((r) => props.columns.some((c) => String(r[c.key] ?? '').toLowerCase().includes(q)))
})

const sorted = computed(() => {
  const arr = [...filtered.value]
  if (!sortKey.value) return arr
  const k = sortKey.value
  arr.sort((a, b) => {
    const av = a[k]
    const bv = b[k]
    if (av == null) return 1
    if (bv == null) return -1
    const cmp = typeof av === 'number' && typeof bv === 'number' ? av - bv : String(av).localeCompare(String(bv))
    return sortAsc.value ? cmp : -cmp
  })
  return arr
})

function toggleSort(key: string) {
  if (sortKey.value === key) {
    sortAsc.value = !sortAsc.value
  } else {
    sortKey.value = key
    sortAsc.value = true
  }
}

const QUOTE = String.fromCharCode(34)
const COMMA = String.fromCharCode(44)

function csvEscape(v: any): string {
  const s = String(v ?? '')
  if (s.includes(COMMA) || s.includes(QUOTE)) {
    return QUOTE + s.split(QUOTE).join(QUOTE + QUOTE) + QUOTE
  }
  return s
}

function download() {
  const cols = props.columns
  const lines = [cols.map((c) => csvEscape(c.label)).join(COMMA)]
  for (const r of sorted.value) {
    lines.push(cols.map((c) => csvEscape(r[c.key])).join(COMMA))
  }
  const csv = lines.join(String.fromCharCode(10))
  const blob = new Blob([String.fromCharCode(65279) + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = (props.filename ?? 'table') + '.csv'
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div>
    <div class="d-flex gap-2 mb-2 align-items-center flex-wrap">
      <div class="input-group input-group-sm" style="max-width: 280px">
        <span class="input-group-text"><i class="bi bi-search"></i></span>
        <input v-model="search" type="text" class="form-control" :placeholder="t('comp.search')" />
      </div>
      <button class="btn btn-sm btn-outline-secondary" @click="download">
        <i class="bi bi-download me-1"></i>{{ t('comp.download') }}
      </button>
      <span class="text-muted small ms-auto">{{ sorted.length }} / {{ rows.length }} {{ t('comp.rows') }}</span>
    </div>
    <div :style="{ maxHeight: scrollHeight ?? '420px', overflow: 'auto' }">
      <table class="table table-sm table-hover align-middle mb-0">
        <thead class="table-light" style="position: sticky; top: 0; z-index: 1">
          <tr>
            <th
              v-for="c in columns"
              :key="c.key"
              class="text-nowrap user-select-none"
              style="cursor: pointer"
              @click="toggleSort(c.key)"
            >
              {{ c.label }}
              <i v-if="sortKey === c.key" :class="['bi', sortAsc ? 'bi-caret-up-fill' : 'bi-caret-down-fill']"></i>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, i) in sorted" :key="i">
            <td v-for="c in columns" :key="c.key" class="text-nowrap">{{ c.fmt ? c.fmt(r[c.key]) : r[c.key] }}</td>
          </tr>
          <tr v-if="sorted.length === 0">
            <td :colspan="columns.length" class="text-center text-muted py-3">{{ t('comp.noMatch') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
