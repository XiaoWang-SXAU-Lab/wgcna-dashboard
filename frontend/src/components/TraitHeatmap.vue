<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EChartsOption } from 'echarts'
import type { MtaitResult } from '../api/client'
import EChart from './EChart.vue'
import { t } from '../utils/i18n'
import { moduleTraitHeatmapOption } from '../utils/reportCharts'

const props = defineProps<{ data: MtaitResult }>()

const lowColor = ref('#2166ac')
const midColor = ref('#ffffff')
const highColor = ref('#b2182b')

const option = computed<EChartsOption>(() => moduleTraitHeatmapOption(props.data.modules, props.data.traits, props.data.cor, props.data.p, lowColor.value, midColor.value, highColor.value, t('comp.downloadPng'), 'module_trait'))
</script>

<template>
  <div>
    <div class="d-flex align-items-center gap-3 mb-2 flex-wrap">
      <span class="text-muted small">{{ t('comp.color') }}</span>
      <label class="d-flex align-items-center gap-1 small mb-0">
        {{ t('comp.low') }} <input type="color" v-model="lowColor" class="form-control form-control-color" :title="t('comp.lowColor')" />
      </label>
      <label class="d-flex align-items-center gap-1 small mb-0">
        {{ t('comp.mid') }} <input type="color" v-model="midColor" class="form-control form-control-color" :title="t('comp.midColor')" />
      </label>
      <label class="d-flex align-items-center gap-1 small mb-0">
        {{ t('comp.high') }} <input type="color" v-model="highColor" class="form-control form-control-color" :title="t('comp.highColor')" />
      </label>
    </div>
    <div class="border rounded bg-white p-1" style="resize: both; overflow: auto; height: 460px; min-height: 300px; min-width: 320px; max-width: 100%">
      <EChart :option="option" height="100%" />
    </div>
    <div class="text-muted small mt-1"><i class="bi bi-arrows-angle-expand me-1"></i>{{ t('comp.resize') }}</div>
  </div>
</template>
