<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { plotUrl } from '../api/client'
import { t } from '../utils/i18n'

// Publication-quality plot widget: point size / color / font size / dimensions
// with a live preview. The preview and the download share the same parameters,
// so what you see is what you get (WYSIWYG).
const props = defineProps<{ kind: string; rsqCut?: number }>()
const store = useWgcnaStore()

const size = ref(2.4)
const color = ref('#5470c6')
const baseSize = ref(13)
const width = ref(10)
const height = ref(4.6)

// Debounced params drive the preview (so rapid slider drags don't spam R).
const preview = ref({ size: size.value, color: color.value, baseSize: baseSize.value, width: width.value, height: height.value })
const loading = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
watch([size, color, baseSize, width, height], () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    preview.value = { size: size.value, color: color.value, baseSize: baseSize.value, width: width.value, height: height.value }
    loading.value = true
  }, 350)
})

function baseParams(mode: string): Record<string, string | number> {
  const p: Record<string, string | number> = {
    mode,
    size: preview.value.size,
    color: preview.value.color,
    base_size: preview.value.baseSize,
    width: preview.value.width,
    height: preview.value.height,
  }
  if (props.rsqCut != null) p.rsq_cut = props.rsqCut
  return p
}

const previewUrl = computed(() => (store.jobId ? plotUrl(store.jobId, props.kind, baseParams('png')) : ''))

function download(mode: 'pdf' | 'svg') {
  if (!store.jobId) return
  const a = document.createElement('a')
  a.href = plotUrl(store.jobId, props.kind, baseParams(mode))
  a.download = `${props.kind}.${mode}`
  a.click()
}
</script>

<template>
  <div class="card">
    <div class="card-header bg-white fw-semibold d-flex align-items-center gap-2">
      <i class="bi bi-sliders"></i>{{ t('plot.title') }}
    </div>
    <div class="card-body">
      <div class="row g-3">
        <div class="col-6">
          <label class="form-label small mb-1">{{ t('plot.size') }}: <b>{{ size.toFixed(1) }}</b></label>
          <input type="range" class="form-range" min="1" max="6" step="0.1" v-model.number="size" />
        </div>
        <div class="col-6">
          <label class="form-label small mb-1">{{ t('comp.color') }}</label>
          <div class="d-flex align-items-center gap-2">
            <input type="color" class="form-control form-control-color" v-model="color" />
            <code class="small">{{ color }}</code>
          </div>
        </div>
        <div class="col-6">
          <label class="form-label small mb-1">{{ t('plot.fontSize') }}: <b>{{ baseSize }}</b></label>
          <input type="range" class="form-range" min="8" max="20" step="1" v-model.number="baseSize" />
        </div>
        <div class="col-3">
          <label class="form-label small mb-1">{{ t('plot.width') }}</label>
          <input type="number" class="form-control form-control-sm" min="3" max="24" step="0.1" v-model.number="width" />
        </div>
        <div class="col-3">
          <label class="form-label small mb-1">{{ t('plot.height') }}</label>
          <input type="number" class="form-control form-control-sm" min="2" max="24" step="0.1" v-model.number="height" />
        </div>
      </div>

      <div class="mt-3 border rounded p-2 bg-white text-center position-relative" style="min-height: 120px">
        <img v-if="previewUrl" :key="previewUrl" :src="previewUrl" class="img-fluid" :class="{ 'opacity-50': loading }" @load="loading = false" alt="preview" />
        <div v-if="loading" class="position-absolute top-50 start-50 translate-middle">
          <span class="spinner-border spinner-border-sm text-primary"></span>
        </div>
      </div>

      <div class="mt-2 d-flex gap-2 justify-content-end">
        <button class="btn btn-sm btn-outline-secondary" @click="download('pdf')"><i class="bi bi-file-earmark-pdf me-1"></i>{{ t('comp.downloadPdf') }}</button>
        <button class="btn btn-sm btn-outline-secondary" @click="download('svg')"><i class="bi bi-file-earmark-image me-1"></i>{{ t('comp.downloadSvg') }}</button>
      </div>
    </div>
  </div>
</template>
