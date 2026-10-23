<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { MtaitResult } from '../api/client'
import EChart from './EChart.vue'
import { moduleColor } from '../utils/colors'
import { t } from '../utils/i18n'

const props = defineProps<{ data: MtaitResult }>()

const lowColor = ref('#2166ac')
const midColor = ref('#ffffff')
const highColor = ref('#b2182b')

function sig(p: number): string {
  if (p < 0.001) return '***'
  if (p < 0.01) return '**'
  if (p < 0.05) return '*'
  return ''
}

const option = computed<EChartsOption>(() => {
  const { modules, traits, cor, p } = props.data
  const heat: [number, number, number][] = []
  cor.forEach((row, i) => row.forEach((v, j) => heat.push([j, i, +v.toFixed(4)])))
  return {
    tooltip: {
      formatter: (params: any) => {
        if (params.seriesType === 'bar') return ''
        const xj = params.value[0] as number
        const yi = params.value[1] as number
        const v = params.value[2] as number
        return modules[yi] + ' × ' + traits[xj] + '<br/>cor = ' + v + '<br/>p = ' + (p[yi]?.[xj] ?? 0)
      },
    },
    grid: [
      { left: 125, width: 22, bottom: 100, top: 20 },
      { left: 147, right: 30, bottom: 100, top: 20 },
    ],
    xAxis: [
      { type: 'value', min: 0, max: 1, show: false, gridIndex: 0 },
      { type: 'category', data: traits, gridIndex: 1, axisLabel: { rotate: 45 }, axisLine: { show: false }, axisTick: { show: false } },
    ],
    yAxis: [
      { type: 'category', data: modules, gridIndex: 0, axisLine: { show: false }, axisTick: { show: false } },
      { type: 'category', data: modules, gridIndex: 1, show: false },
    ],
    visualMap: {
      min: -1, max: 1, calculable: true, orient: 'horizontal', left: 'center', bottom: 0,
      inRange: { color: [lowColor.value, midColor.value, highColor.value] },
      seriesIndex: 1,
    },
    toolbox: {
      right: 10, top: 10,
      feature: {
        saveAsImage: { title: t('comp.downloadPng'), name: 'module_trait', pixelRatio: 2, backgroundColor: '#ffffff' },
      },
    },
    series: [
      {
        type: 'bar', xAxisIndex: 0, yAxisIndex: 0,
        data: modules.map((m) => ({ value: 1, itemStyle: { color: moduleColor(m), borderColor: '#333', borderWidth: 0.8 } })),
        barWidth: '100%', barCategoryGap: '0%', silent: true, z: 2,
      },
      {
        type: 'heatmap', xAxisIndex: 1, yAxisIndex: 1, data: heat,
        label: {
          show: true,
          formatter: (params: any) => {
            const xj = params.value[0] as number
            const yi = params.value[1] as number
            const c = cor[yi]?.[xj] ?? 0
            const pv = p[yi]?.[xj] ?? 1
            return c.toFixed(2) + String.fromCharCode(10) + sig(pv)
          },
          fontSize: 11, lineHeight: 14,
        },
        itemStyle: { borderColor: '#333', borderWidth: 0.6 },
        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.4)' } },
      },
    ],
  }
})
</script>

<template>
  <div>
    <div class="d-flex align-items-center gap-3 mb-2 flex-wrap">
      <span class="text-muted small">{{ t('comp.color') }}</span>
      <label class="d-flex align-items-center gap-1 small mb-0">
        {{ t('comp.low') }} <input type="color" v-model="lowColor" class="form-control form-control-color" title="低值颜色" />
      </label>
      <label class="d-flex align-items-center gap-1 small mb-0">
        {{ t('comp.mid') }} <input type="color" v-model="midColor" class="form-control form-control-color" title="中值颜色" />
      </label>
      <label class="d-flex align-items-center gap-1 small mb-0">
        {{ t('comp.high') }} <input type="color" v-model="highColor" class="form-control form-control-color" title="高值颜色" />
      </label>
    </div>
    <div class="border rounded bg-white p-1" style="resize: both; overflow: auto; height: 460px; min-height: 300px; min-width: 320px; max-width: 100%">
      <EChart :option="option" height="100%" />
    </div>
    <div class="text-muted small mt-1"><i class="bi bi-arrows-angle-expand me-1"></i>{{ t('comp.resize') }}</div>
  </div>
</template>
