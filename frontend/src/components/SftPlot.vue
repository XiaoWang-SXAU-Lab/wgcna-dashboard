<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { SftResult } from '../api/client'
import ResizableChart from './ResizableChart.vue'
import { t } from '../utils/i18n'

const props = defineProps<{ data: SftResult; rsquaredCut?: number }>()
const lineColor = ref('#5470c6')
const rsqCut = computed(() => props.rsquaredCut ?? 0.85)
const powers = computed(() => props.data.fit_indices.map((f) => f.power))

const scaleIndependence = computed<EChartsOption>(() => ({
  title: { text: t('sft.scaleTitle'), left: 'center', textStyle: { fontSize: 14 } },
  tooltip: { trigger: 'axis' },
  toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: '下载 PNG', name: 'scale_independence', pixelRatio: 2, backgroundColor: '#ffffff' } } },
  xAxis: { type: 'category', name: 'Soft Threshold (power)', nameLocation: 'middle', data: powers.value },
  yAxis: { type: 'value', name: 'Scale Free Topology Model Fit, signed R²', nameLocation: 'middle', nameRotate: 90, nameGap: 55 },
  series: [{
    type: 'line',
    data: props.data.fit_indices.map((f) => +f.rsq.toFixed(3)),
    symbolSize: 8,
    lineStyle: { width: 2, color: lineColor.value },
    markLine: {
      data: [{ yAxis: rsqCut.value, label: { formatter: 'R² = ' + rsqCut.value } }],
      lineStyle: { color: '#e11d48', type: 'dashed' },
    },
  }],
}))

const meanConnectivity = computed<EChartsOption>(() => ({
  title: { text: t('sft.connTitle'), left: 'center', textStyle: { fontSize: 14 } },
  tooltip: { trigger: 'axis' },
  toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: '下载 PNG', name: 'mean_connectivity', pixelRatio: 2, backgroundColor: '#ffffff' } } },
  xAxis: { type: 'category', name: 'Soft Threshold (power)', nameLocation: 'middle', data: powers.value },
  yAxis: { type: 'value', name: 'Mean Connectivity', nameLocation: 'middle', nameRotate: 90, nameGap: 55 },
  series: [{
    type: 'line',
    data: props.data.fit_indices.map((f) => +f.mean_k.toFixed(3)),
    symbolSize: 8,
    lineStyle: { width: 2, color: lineColor.value },
    areaStyle: { opacity: 0.08 },
  }],
}))
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
