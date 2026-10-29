<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { DetailResult } from '../api/client'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'
import { gsMmScatterOption } from '../utils/reportCharts'

const props = defineProps<{ data: DetailResult }>()

const option = computed<EChartsOption>(() => gsMmScatterOption(props.data.mm, props.data.gs, props.data.genes, props.data.module, props.data.trait_name, t('gs.title'), t('comp.downloadPng')))
</script>

<template>
  <div>
    <ResizableChart :option="option" />
  </div>
</template>
