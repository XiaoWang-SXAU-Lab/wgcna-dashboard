<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { plotUrl } from '../api/client'
import { t } from '../utils/i18n'

// Publication-quality export dialog: left = parameters (point size / point
// color / line color / font size / axis text size / axis color / dimensions
// + format select + download), right = live preview. Preview and download
// share the same parameters → WYSIWYG.
const props = defineProps<{ kind: string; rsqCut?: number; show: boolean }>()
const emit = defineEmits<{ close: [] }>()
const store = useWgcnaStore()

const size = ref(2.4)
const color = ref('#5470c6')
const lineColor = ref('#5470c6')
const baseSize = ref(13)
const axisSize = ref(11)
const axisColor = ref('#333333')
const width = ref(10)
const height = ref(4.6)
const format = ref<'pdf' | 'svg' | 'png'>('pdf')

// Debounced params drive the preview (so rapid slider drags don't spam R).
const preview = ref({
  size: size.value, color: color.value, lineColor: lineColor.value,
  baseSize: baseSize.value, axisSize: axisSize.value, axisColor: axisColor.value,
  width: width.value, height: height.value,
})
const loading = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
watch([size, color, lineColor, baseSize, axisSize, axisColor, width, height], () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    preview.value = {
      size: size.value, color: color.value, lineColor: lineColor.value,
      baseSize: baseSize.value, axisSize: axisSize.value, axisColor: axisColor.value,
      width: width.value, height: height.value,
    }
    loading.value = true
  }, 350)
})
watch(() => props.show, (v) => { if (v) loading.value = true })

function baseParams(mode: string): Record<string, string | number> {
  const p: Record<string, string | number> = {
    mode,
    size: preview.value.size,
    color: preview.value.color,
    line_color: preview.value.lineColor,
    base_size: preview.value.baseSize,
    axis_size: preview.value.axisSize,
    axis_color: preview.value.axisColor,
    width: preview.value.width,
    height: preview.value.height,
  }
  if (props.rsqCut != null) p.rsq_cut = props.rsqCut
  return p
}

const previewUrl = computed(() => (store.jobId && props.show ? plotUrl(store.jobId, props.kind, baseParams('png')) : ''))

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
        <!-- left sidebar: parameters (scrollable as a whole) -->
        <div class="border-end overflow-auto flex-shrink-0" style="width: 300px">
          <div class="p-3 d-flex flex-column gap-2">
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
            <div class="row g-2">
              <div class="col-6">
                <label class="form-label small mb-1">{{ t('plot.width') }}</label>
                <input type="number" class="form-control form-control-sm" min="3" max="24" step="0.1" v-model.number="width" />
              </div>
              <div class="col-6">
                <label class="form-label small mb-1">{{ t('plot.height') }}</label>
                <input type="number" class="form-control form-control-sm" min="2" max="24" step="0.1" v-model.number="height" />
              </div>
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

        <!-- right: live preview -->
        <div class="flex-grow-1 p-3 bg-light position-relative d-flex align-items-center justify-content-center overflow-auto">
          <img v-if="previewUrl" :key="previewUrl" :src="previewUrl" :class="{ 'opacity-50': loading }" @load="loading = false" alt="preview" style="max-width: 100%; max-height: 100%; object-fit: contain" />
          <div v-if="loading" class="position-absolute top-50 start-50 translate-middle">
            <span class="spinner-border spinner-border-sm text-primary"></span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.plot-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1050;
}
.plot-modal {
  background: #fff;
  border-radius: 10px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  width: min(980px, 96vw);
  max-height: 92vh;
  overflow: hidden;
}
</style>
