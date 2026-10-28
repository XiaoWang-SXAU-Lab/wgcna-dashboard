<script setup lang="ts">
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { EChartsOption } from 'echarts'

const props = defineProps<{
  option: EChartsOption
  height?: string
}>()
const emit = defineEmits<{ (e: 'chart-click', params: unknown): void }>()

const el = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let ro: ResizeObserver | null = null

onMounted(() => {
  if (!el.value) return
  chart = echarts.init(el.value)
  chart.on('finished', () => el.value?.dispatchEvent(new CustomEvent('report-chart-rendered', { bubbles: true })))
  chart.setOption(props.option)
  chart.on('click', (params: unknown) => emit('chart-click', params))
  window.addEventListener('resize', onResize)
  ro = new ResizeObserver(() => chart?.resize())
  ro.observe(el.value)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  ro?.disconnect()
  chart?.dispose()
  chart = null
})

watch(
  () => props.option,
  (opt) => chart?.setOption(opt, true),
  { deep: true },
)

function onResize() {
  chart?.resize()
}

defineExpose({ getChart: () => chart })
</script>

<template>
  <div ref="el" :style="{ width: '100%', height: height ?? '420px' }" />
</template>
