<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import EChart from './EChart.vue'
import { t } from '../utils/i18n'

defineProps<{ option: EChartsOption }>()
</script>

<template>
  <div>
    <div
      class="border rounded bg-white p-1"
      style="resize: both; overflow: auto; height: 460px; min-height: 300px; min-width: 320px; max-width: 100%"
    >
      <EChart :option="option" height="100%" />
    </div>
    <div class="text-muted small mt-1">
      <i class="bi bi-arrows-angle-expand me-1"></i>{{ t('comp.resize') }}
    </div>
  </div>
</template>
