<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { Dendrogram } from '../api/client'
import { t } from '../utils/i18n'
import { geneDendrogramOption } from '../utils/reportCharts'
import EChart from './EChart.vue'

const props = defineProps<{ genes: string[]; colors: string[]; dendrogram: Dendrogram }>()

const option = computed<EChartsOption>(() => geneDendrogramOption(props.genes, props.colors, props.dendrogram, t('comp.downloadPng'), 'module_dendrogram'))
</script>

<template>
  <div>
    <div
      class="border rounded bg-white p-1"
      style="resize: both; overflow: auto; height: 520px; min-height: 300px; min-width: 320px; max-width: 100%"
    >
      <EChart :option="option" height="100%" />
    </div>
    <div class="text-muted small mt-1">
      <i class="bi bi-arrows-angle-expand me-1"></i>{{ t('comp.resize') }}
    </div>
  </div>
</template>
