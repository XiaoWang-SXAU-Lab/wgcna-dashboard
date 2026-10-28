<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { liveFigures, saveFigure } from '../utils/reportSnapshots'

const props = defineProps<{ figureKey: string; trait?: string }>()
const store = useWgcnaStore()
const root = ref<HTMLElement>()
const error = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
let resize: ResizeObserver | undefined
let mutations: MutationObserver | undefined
let pending = Promise.resolve()
let disposed = false
function capture() {
  clearTimeout(timer)
  const element = root.value, job = store.jobId, key = props.figureKey, trait = props.trait
  if (!element || !job || !element.querySelector('canvas,svg') || !element.getBoundingClientRect().width) return pending
  pending = pending.catch(() => {}).then(async () => {
    if (disposed) return
    try { await saveFigure(job, key, element, trait); error.value = '' }
    catch (e) { error.value = `报告快照保存失败：${String(e)}`; throw e }
  })
  return pending
}
function schedule() { clearTimeout(timer); timer = setTimeout(() => { void capture().catch(() => {}) }, 800) }
watch(() => [props.figureKey, props.trait], schedule)
onMounted(() => {
  liveFigures.add(capture)
  resize = new ResizeObserver(schedule); resize.observe(root.value!)
  mutations = new MutationObserver(schedule)
  mutations.observe(root.value!, { childList: true, subtree: true, attributes: true, characterData: true })
  schedule()
})
onBeforeUnmount(() => { disposed = true; clearTimeout(timer); resize?.disconnect(); mutations?.disconnect(); liveFigures.delete(capture) })
</script>
<template>
  <div>
    <div ref="root" @report-chart-rendered="schedule" @change="schedule" @pointerup="schedule" @wheel.passive="schedule"><slot /></div>
    <div v-if="error" class="small text-danger" data-snapshot-ignore>{{ error }}</div>
  </div>
</template>
