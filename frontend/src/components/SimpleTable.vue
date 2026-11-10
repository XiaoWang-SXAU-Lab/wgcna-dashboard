<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { t } from '../utils/i18n'

type Column = { key: string; label: string; fmt?: (v: any, row: Record<string, any>) => string }

const props = defineProps<{
  columns: Column[]
  rows: Record<string, any>[]
  scrollHeight?: string
  filename?: string
  selectedKey?: string
  rowKey?: string
}>()

const emit = defineEmits<{ (e: 'row-click', row: Record<string, any>): void }>()

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

// Windowed rendering: large tables (e.g. 8,000 genes × 7 modules) otherwise put
// tens of thousands of cells in the DOM and make scrolling/layout janky.
const ROW_HEIGHT_FALLBACK = 32
const BUFFER = 8
const rowHeight = ref(ROW_HEIGHT_FALLBACK)
const scrollTop = ref(0)
const viewport = ref<HTMLElement>()
const measuredHeight = ref(0)
let viewportObserver: ResizeObserver | null = null

const viewportHeight = computed(() => {
  if (measuredHeight.value > 0) return measuredHeight.value
  const parsed = Number.parseInt(props.scrollHeight ?? '', 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 420
})

const start = computed(() => {
  const maxStart = Math.max(0, sorted.value.length - 1)
  return Math.min(maxStart, Math.max(0, Math.floor(scrollTop.value / rowHeight.value) - BUFFER))
})

const visibleCount = computed(() => Math.ceil(viewportHeight.value / rowHeight.value) + BUFFER * 2)
const end = computed(() => Math.min(sorted.value.length, start.value + visibleCount.value))
const windowRows = computed(() => sorted.value.slice(start.value, end.value))
const topPad = computed(() => start.value * rowHeight.value)
const bottomPad = computed(() => Math.max(0, (sorted.value.length - end.value) * rowHeight.value))

function onScroll(event: Event) {
  scrollTop.value = (event.target as HTMLElement).scrollTop
}

function resetScroll() {
  scrollTop.value = 0
  if (viewport.value) viewport.value.scrollTop = 0
}

// Row height is font/DPI dependent; measure the first rendered row so spacer
// heights match the browser exactly.
function measureRow() {
  const el = viewport.value?.querySelector<HTMLTableRowElement>('tr.simple-row')
  const h = el?.getBoundingClientRect().height
  if (h && h > 0 && Math.abs(h - rowHeight.value) > 0.25) rowHeight.value = h
}

onMounted(() => {
  nextTick(measureRow)
  if (viewport.value) {
    measuredHeight.value = viewport.value.clientHeight
    viewportObserver = new ResizeObserver(() => {
      measuredHeight.value = viewport.value?.clientHeight ?? 0
    })
    viewportObserver.observe(viewport.value)
  }
})
onBeforeUnmount(() => {
  viewportObserver?.disconnect()
  viewportObserver = null
})
watch([start, end], () => nextTick(measureRow), { flush: 'post' })
watch([search, sortKey, sortAsc, () => props.rows], () => {
  resetScroll()
  nextTick(measureRow)
})

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
    <div ref="viewport" :style="{ maxHeight: scrollHeight ?? '420px', overflow: 'auto' }" @scroll.passive="onScroll">
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
          <tr v-if="topPad > 0" aria-hidden="true">
            <td :colspan="columns.length" :style="{ height: topPad + 'px', padding: 0, border: 0 }">&nbsp;</td>
          </tr>
          <tr
            v-for="(r, i) in windowRows"
            :key="start + i"
            class="simple-row"
            :style="{ height: rowHeight + 'px', cursor: 'pointer' }"
            :class="{ 'table-active': !!selectedKey && String(r[rowKey ?? 'id']) === selectedKey }"
            @click="emit('row-click', r)"
          >
            <td v-for="c in columns" :key="c.key" class="text-nowrap">{{ c.fmt ? c.fmt(r[c.key], r) : r[c.key] }}</td>
          </tr>
          <tr v-if="bottomPad > 0" aria-hidden="true">
            <td :colspan="columns.length" :style="{ height: bottomPad + 'px', padding: 0, border: 0 }">&nbsp;</td>
          </tr>
          <tr v-if="sorted.length === 0">
            <td :colspan="columns.length" class="text-center text-muted py-3">{{ t('comp.noMatch') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
