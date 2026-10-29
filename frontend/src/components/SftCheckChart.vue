<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsOption } from 'echarts'
import type { ScaleFreeCheck } from '../api/client'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'
import { sftHistogramOption, sftLogLogOption } from '../utils/reportCharts'

const props = defineProps<{ data: ScaleFreeCheck }>()

const hist = computed<EChartsOption>(() => sftHistogramOption(props.data, t('sft.connHist'), t('sft.freq')))
const loglog = computed<EChartsOption>(() => sftLogLogOption(props.data, t('sft.scaleCheckTitle')))
</script>

<template>
  <div class="row g-3">
    <div class="col-md-6"><ResizableChart :option="hist" /></div>
    <div class="col-md-6"><ResizableChart :option="loglog" /></div>
  </div>
</template>
