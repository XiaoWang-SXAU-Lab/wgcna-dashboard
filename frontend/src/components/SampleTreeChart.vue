<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { Dendrogram } from '../api/client'
import EChart from './EChart.vue'
import { t } from '../utils/i18n'
import { sampleTreeOption, type CategoryBand } from '../utils/reportCharts'

export type { CategoryBand }

const props = defineProps<{
  samples: string[]
  tree: Dendrogram
  categories?: CategoryBand[]
  selected?: string[]
}>()
const emit = defineEmits<{ select: [name: string] }>()

const PALETTE = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b', '#e377c2', '#7f7f7f', '#bcbd22', '#17becf', '#aec7e8', '#ffbb78']

const palettes = computed(() => (props.categories ?? []).map((c) => {
  const vals = Object.values(c.values)
  const uniq = [...new Set(vals)].sort()
  const m: Record<string, string> = {}
  uniq.forEach((v, i) => { m[v] = PALETTE[i % PALETTE.length] })
  return { name: c.name, values: c.values, palette: m }
}))

const option = computed<EChartsOption>(() => sampleTreeOption(props.samples, props.tree, props.categories ?? [], props.selected ?? [], 'sample_cluster', t('comp.downloadPng')))

const echartRef = ref<any>(null)

onMounted(() => {
  const c = echartRef.value?.getChart?.()
  if (!c) return
  c.on('click', (params: any) => {
    if (params.seriesType === 'scatter' && params.name) {
      emit('select', params.name)
    }
  })
})
</script>

<template>
  <div>
    <div class="border rounded bg-white p-1" style="resize: both; overflow: auto; height: 560px; min-height: 300px; min-width: 320px; max-width: 100%">
      <EChart ref="echartRef" :option="option" height="100%" />
    </div>
    <div v-if="palettes.length" class="mt-2">
      <div v-for="b in palettes" :key="b.name" class="d-flex flex-wrap gap-2 align-items-center mb-1">
        <span class="small text-muted fw-semibold" style="min-width:90px">{{ b.name }}</span>
        <span v-for="(color, value) in b.palette" :key="value" class="small d-flex align-items-center gap-1">
          <span class="d-inline-block" :style="{ width: '10px', height: '10px', background: color }"></span>{{ value || '—' }}
        </span>
      </div>
    </div>
    <div class="text-muted small mt-1"><i class="bi bi-arrows-angle-expand me-1"></i>{{ t('comp.resize') }} · {{ t('di.sampleTreeHint') }}</div>
  </div>
</template>
