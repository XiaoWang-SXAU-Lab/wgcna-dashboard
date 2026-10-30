<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { plotUrl } from '../api/client'
import { t, locale } from '../utils/i18n'

const props = defineProps<{ kind: string; rsqCut?: number; show: boolean; attrs?: string; type?: string; module?: string; fdr?: number; top?: number; ontology?: string }>()
const emit = defineEmits<{ close: [] }>()
const store = useWgcnaStore()

const isSimple = computed(() => props.kind === 'dendrogram')
const isHeatmap = computed(() => props.kind === 'mtrait' || props.kind === 'eigengene' || props.kind === 'geneheatmap')
const isGeneHeatmap = computed(() => props.kind === 'geneheatmap')
const isSampleTree = computed(() => props.kind === 'sample-tree')
const isEnrich = computed(() => props.kind === 'enrich')

// sample_info annotation columns (excluding id/name) for the geneheatmap export
const selectedAnns = ref<string[]>([])
const annColumns = computed(() => store.sampleColumns.filter((c) => c !== 'sample_id' && c !== 'sample_name'))

const size = ref(2.4)
const color = ref('#5470c6')
const lineColor = ref('#5470c6')
const baseSize = ref(13)
const axisSize = ref(11)
const axisColor = ref('#333333')
const lowColor = ref('#2166ac')
const midColor = ref('#ffffff')
const highColor = ref('#b2182b')
const legend = ref<'right' | 'bottom' | 'none'>('right')
const clusterRows = ref(true)
const clusterCols = ref(true)
const showRowNames = ref(true)
const showColNames = ref(true)
const cellFontSize = ref(12)
const border = ref(true)
const heatTitle = ref('')
const width = ref(10)
const height = ref(8)
const dpi = ref(300)
const format = ref<'pdf' | 'svg' | 'png'>('pdf')

const enrichX = ref<'GeneRatio' | 'Count'>('GeneRatio')
const enrichColor = ref<'p.adjust' | 'pvalue' | 'qvalue'>('p.adjust')
const showCategory = ref(10)
const sizeMin = ref(3)
const sizeMax = ref(8)
const enrichLow = ref('red')
const enrichHigh = ref('blue')
const enrichTitle = ref('')
const wrap = ref(30)

const preview = ref({
  size: size.value, color: color.value, lineColor: lineColor.value,
  baseSize: baseSize.value, axisSize: axisSize.value, axisColor: axisColor.value,
  lowColor: lowColor.value, midColor: midColor.value, highColor: highColor.value,
  legend: legend.value, width: width.value, height: height.value, dpi: dpi.value,
  enrichX: enrichX.value, enrichColor: enrichColor.value, showCategory: showCategory.value,
  sizeMin: sizeMin.value, sizeMax: sizeMax.value, enrichLow: enrichLow.value,
  enrichHigh: enrichHigh.value, enrichTitle: enrichTitle.value, wrap: wrap.value,
})
const loading = ref(false)
const previewFailed = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
watch([size, color, lineColor, baseSize, axisSize, axisColor, lowColor, midColor, highColor, legend, width, height, dpi, enrichX, enrichColor, showCategory, sizeMin, sizeMax, enrichLow, enrichHigh, enrichTitle, wrap], () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    preview.value = {
      size: size.value, color: color.value, lineColor: lineColor.value,
      baseSize: baseSize.value, axisSize: axisSize.value, axisColor: axisColor.value,
      lowColor: lowColor.value, midColor: midColor.value, highColor: highColor.value,
      legend: legend.value, width: width.value, height: height.value, dpi: dpi.value,
      enrichX: enrichX.value, enrichColor: enrichColor.value, showCategory: showCategory.value,
      sizeMin: sizeMin.value, sizeMax: sizeMax.value, enrichLow: enrichLow.value,
      enrichHigh: enrichHigh.value, enrichTitle: enrichTitle.value, wrap: wrap.value,
    }
    loading.value = true
  }, 350)
})
watch(() => props.show, (v) => { if (v) loading.value = true })

function baseParams(mode: string): Record<string, string | number> {
  const p: Record<string, string | number> = {
    mode, width: preview.value.width, height: preview.value.height, dpi: preview.value.dpi,
  }
  if (isHeatmap.value) {
    p.low = preview.value.lowColor
    p.mid = preview.value.midColor
    p.high = preview.value.highColor
    p.legend = preview.value.legend
    p.cluster_rows = clusterRows.value ? '1' : '0'
    p.cluster_cols = clusterCols.value ? '1' : '0'
    p.show_rownames = showRowNames.value ? '1' : '0'
    p.show_colnames = showColNames.value ? '1' : '0'
    p.cell_fontsize = cellFontSize.value
    p.border = border.value ? '1' : '0'
    if (heatTitle.value) p.title = heatTitle.value
    if (isGeneHeatmap.value && selectedAnns.value.length) p.ann = selectedAnns.value.join(',')
  } else if (isSampleTree.value) {
    p.base_size = preview.value.baseSize
    p.line_color = preview.value.lineColor
    if (props.attrs) p.attrs = props.attrs
  } else if (isEnrich.value) {
    if (props.type) p.type = props.type
    if (props.module) p.module = props.module
    if (props.fdr != null) p.fdr = props.fdr
    if (props.top != null) p.top = props.top
    p.x = preview.value.enrichX
    p.color_var = preview.value.enrichColor
    p.show_category = preview.value.showCategory
    p.size_min = preview.value.sizeMin
    p.size_max = preview.value.sizeMax
    p.low = preview.value.enrichLow
    p.high = preview.value.enrichHigh
    p.base_size = preview.value.baseSize
    p.wrap = preview.value.wrap
    if (props.ontology) p.ontology = props.ontology
    if (preview.value.enrichTitle) p.title = preview.value.enrichTitle
  } else if (!isSimple.value) {
    p.size = preview.value.size
    p.color = preview.value.color
    p.line_color = preview.value.lineColor
    p.base_size = preview.value.baseSize
    p.axis_size = preview.value.axisSize
    p.axis_color = preview.value.axisColor
  }
  if (props.rsqCut != null) p.rsq_cut = props.rsqCut
  return p
}

const previewUrl = computed(() => (store.jobId && props.show ? plotUrl(store.jobId, props.kind, baseParams('png')) : ''))
watch(previewUrl, url => { loading.value = !!url; previewFailed.value = false })
onBeforeUnmount(() => clearTimeout(timer))

function download() {
  if (!store.jobId) return
  const a = document.createElement('a')
  a.href = plotUrl(store.jobId, props.kind, baseParams(format.value))
  a.download = `${props.kind}.${format.value}`
  a.click()
}
</script>

<template>
  <div v-if="show" class="plot-overlay" @click.self="emit('close')">
    <div class="plot-modal">
      <div class="d-flex justify-content-between align-items-center px-3 py-2 border-bottom bg-white">
        <span class="fw-semibold"><i class="bi bi-sliders me-2"></i>{{ t('plot.title') }}</span>
        <button class="btn btn-sm btn-outline-secondary" @click="emit('close')"><i class="bi bi-x-lg"></i></button>
      </div>

      <div class="d-flex" style="height: 620px">
        <div class="border-end overflow-auto flex-shrink-0" style="width: 300px">
          <div class="p-3 d-flex flex-column gap-2">
            <template v-if="isHeatmap">
              <div class="row g-2">
                <div class="col-4">
                  <label class="form-label small mb-1">{{ t('comp.low') }}</label>
                  <input type="color" class="form-control form-control-color" v-model="lowColor" />
                </div>
                <div class="col-4">
                  <label class="form-label small mb-1">{{ t('comp.mid') }}</label>
                  <input type="color" class="form-control form-control-color" v-model="midColor" />
                </div>
                <div class="col-4">
                  <label class="form-label small mb-1">{{ t('comp.high') }}</label>
                  <input type="color" class="form-control form-control-color" v-model="highColor" />
                </div>
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.legend') }}</label>
                <select class="form-select form-select-sm" v-model="legend">
                  <option value="right">right</option>
                  <option value="bottom">bottom</option>
                  <option value="none">none</option>
                </select>
              </div>
              <div class="d-flex flex-wrap gap-3 small">
                <label class="form-check form-check-inline mb-0">
                  <input class="form-check-input" type="checkbox" v-model="clusterRows" />{{ t('plot.clusterRows') }}
                </label>
                <label class="form-check form-check-inline mb-0">
                  <input class="form-check-input" type="checkbox" v-model="clusterCols" />{{ t('plot.clusterCols') }}
                </label>
                <label class="form-check form-check-inline mb-0">
                  <input class="form-check-input" type="checkbox" v-model="showRowNames" />{{ t('plot.showRowNames') }}
                </label>
                <label class="form-check form-check-inline mb-0">
                  <input class="form-check-input" type="checkbox" v-model="showColNames" />{{ t('plot.showColNames') }}
                </label>
                <label class="form-check form-check-inline mb-0">
                  <input class="form-check-input" type="checkbox" v-model="border" />{{ t('plot.border') }}
                </label>
              </div>
              <div class="row g-2">
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.cellFontSize') }}</label>
                  <input type="number" class="form-control form-control-sm" min="6" max="20" step="1" v-model.number="cellFontSize" />
                </div>
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.heatTitle') }}</label>
                  <input type="text" class="form-control form-control-sm" v-model="heatTitle" />
                </div>
              </div>
              <template v-if="isGeneHeatmap">
                <div>
                  <label class="form-label small mb-1">{{ t('plot.annotateColumns') }}</label>
                  <select class="form-select form-select-sm" multiple size="4" v-model="selectedAnns" :disabled="annColumns.length === 0">
                    <option v-for="c in annColumns" :key="c" :value="c">{{ c }}</option>
                  </select>
                  <div class="small text-muted mt-1">{{ annColumns.length === 0 ? t('plot.noSampleInfo') : t('plot.annotateHint') }}</div>
                </div>
              </template>
            </template>
            <template v-if="isSampleTree">
              <div>
                <label class="form-label small mb-1">{{ t('plot.lineColor') }}</label>
                <div class="d-flex align-items-center gap-1">
                  <input type="color" class="form-control form-control-color" v-model="lineColor" />
                  <code class="small">{{ lineColor }}</code>
                </div>
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.fontSize') }}: <b>{{ baseSize }}</b></label>
                <input type="range" class="form-range" min="8" max="20" step="1" v-model.number="baseSize" />
              </div>
            </template>
            <template v-if="isEnrich">
              <div class="row g-2">
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.xAxis') }}</label>
                  <select v-model="enrichX" class="form-select form-select-sm">
                    <option value="GeneRatio">GeneRatio</option>
                    <option value="Count">Count</option>
                  </select>
                </div>
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.colorBy') }}</label>
                  <select v-model="enrichColor" class="form-select form-select-sm">
                    <option value="p.adjust">p.adjust</option>
                    <option value="pvalue">pvalue</option>
                    <option value="qvalue">qvalue</option>
                  </select>
                </div>
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.showCategory') }}: <b>{{ showCategory }}</b></label>
                <input type="number" class="form-control form-control-sm" min="5" max="100" v-model.number="showCategory" />
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.wrap') }}: <b>{{ wrap }}</b></label>
                <input type="number" class="form-control form-control-sm" min="0" max="120" v-model.number="wrap" />
              </div>
              <div class="row g-2">
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.sizeMin') }}: <b>{{ sizeMin }}</b></label>
                  <input type="range" class="form-range" min="1" max="12" step="1" v-model.number="sizeMin" />
                </div>
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.sizeMax') }}: <b>{{ sizeMax }}</b></label>
                  <input type="range" class="form-range" min="3" max="16" step="1" v-model.number="sizeMax" />
                </div>
              </div>
              <div class="row g-2">
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.lowColor') }}</label>
                  <input type="color" class="form-control form-control-color" v-model="enrichLow" />
                </div>
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.highColor') }}</label>
                  <input type="color" class="form-control form-control-color" v-model="enrichHigh" />
                </div>
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.fontSize') }}: <b>{{ baseSize }}</b></label>
                <input type="range" class="form-range" min="8" max="20" step="1" v-model.number="baseSize" />
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.title') }}</label>
                <input type="text" class="form-control form-control-sm" v-model="enrichTitle" />
              </div>
            </template>
            <template v-if="!isSimple && !isHeatmap && !isSampleTree && !isEnrich">
              <div>
                <label class="form-label small mb-1">{{ t('plot.size') }}: <b>{{ size.toFixed(1) }}</b></label>
                <input type="range" class="form-range" min="1" max="6" step="0.1" v-model.number="size" />
              </div>
              <div class="row g-2">
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.pointColor') }}</label>
                  <div class="d-flex align-items-center gap-1">
                    <input type="color" class="form-control form-control-color" v-model="color" />
                    <code class="small">{{ color }}</code>
                  </div>
                </div>
                <div class="col-6">
                  <label class="form-label small mb-1">{{ t('plot.lineColor') }}</label>
                  <div class="d-flex align-items-center gap-1">
                    <input type="color" class="form-control form-control-color" v-model="lineColor" />
                    <code class="small">{{ lineColor }}</code>
                  </div>
                </div>
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.fontSize') }}: <b>{{ baseSize }}</b></label>
                <input type="range" class="form-range" min="8" max="20" step="1" v-model.number="baseSize" />
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.axisSize') }}: <b>{{ axisSize }}</b></label>
                <input type="range" class="form-range" min="6" max="20" step="1" v-model.number="axisSize" />
              </div>
              <div>
                <label class="form-label small mb-1">{{ t('plot.axisColor') }}</label>
                <div class="d-flex align-items-center gap-1">
                  <input type="color" class="form-control form-control-color" v-model="axisColor" />
                  <code class="small">{{ axisColor }}</code>
                </div>
              </div>
            </template>

            <div class="mb-1">
              <label class="form-label small mb-1">{{ t('plot.width') }}: <b>{{ width }}</b></label>
              <input type="range" class="form-range" min="3" max="24" step="0.5" v-model.number="width" />
            </div>
            <div class="mb-1">
              <label class="form-label small mb-1">{{ t('plot.height') }}: <b>{{ height }}</b></label>
              <input type="range" class="form-range" min="2" max="24" step="0.5" v-model.number="height" />
            </div>
            <div>
              <label class="form-label small mb-1">{{ t('plot.dpi') }}</label>
              <input type="number" class="form-control form-control-sm" min="72" max="600" step="1" v-model.number="dpi" />
            </div>

            <hr class="my-1" />

            <div>
              <label class="form-label small mb-1">{{ t('plot.format') }}</label>
              <select class="form-select form-select-sm" v-model="format">
                <option value="pdf">PDF</option>
                <option value="svg">SVG</option>
                <option value="png">PNG</option>
              </select>
            </div>
            <button class="btn btn-primary w-100" @click="download">
              <i class="bi bi-download me-1"></i>{{ t('plot.download') }}
            </button>
          </div>
        </div>

        <div class="flex-grow-1 p-3 bg-light position-relative d-flex align-items-center justify-content-center overflow-auto">
          <img v-if="previewUrl && !previewFailed" :key="previewUrl" :src="previewUrl" :class="{ 'opacity-50': loading }" @load="loading = false" @error="loading = false; previewFailed = true" alt="preview" style="max-width: 100%; max-height: 100%; object-fit: contain" />
          <div v-if="previewFailed" class="alert alert-warning" role="alert">{{ locale === 'zh' ? '预览生成失败。请检查绘图参数和 R 环境；调整设置后可重试。' : 'Preview failed. Check plot settings and the R environment, then adjust settings to retry.' }}</div>
          <div v-if="loading" class="position-absolute top-50 start-50 translate-middle">
            <span class="spinner-border spinner-border-sm text-primary"></span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.plot-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.5); display: flex; align-items: center; justify-content: center; z-index: 1050; }
.plot-modal { background: #fff; border-radius: 10px; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3); width: min(980px, 96vw); max-height: 92vh; overflow: hidden; }
</style>
