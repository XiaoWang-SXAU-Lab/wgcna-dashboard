<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { SftResult } from '../api/client'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'
import { meanConnectivityOption, scaleIndependenceOption } from '../utils/reportCharts'

const props = defineProps<{ data: SftResult; rsquaredCut?: number }>()
const lineColor = ref('#5470c6')
const rsqCut = computed(() => props.rsquaredCut ?? 0.85)

const scaleIndependence = computed<EChartsOption>(() => scaleIndependenceOption(props.data.fit_indices, rsqCut.value, t('sft.scaleTitle'), lineColor.value, 'scale_independence', t('comp.downloadPng')))
const meanConnectivity = computed<EChartsOption>(() => meanConnectivityOption(props.data.fit_indices, t('sft.connTitle'), lineColor.value, 'mean_connectivity', t('comp.downloadPng')))
</script>

<template>
  <div>
    <div class="d-flex align-items-center gap-2 mb-2">
      <span class="text-muted small">{{ t('comp.lineColor') }}</span>
      <input type="color" v-model="lineColor" class="form-control form-control-color" />
    </div>
    <div class="row g-3">
      <div class="col-md-6"><ResizableChart :option="scaleIndependence" /></div>
      <div class="col-md-6"><ResizableChart :option="meanConnectivity" /></div>
    </div>
  </div>
</template>
