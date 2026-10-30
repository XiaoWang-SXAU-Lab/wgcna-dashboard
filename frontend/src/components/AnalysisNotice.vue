<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useWgcnaStore } from '../stores/wgcna'
import { locale } from '../utils/i18n'
const store = useWgcnaStore()
const router = useRouter()
const dialog = ref<HTMLElement | null>(null)
let previous: HTMLElement | null = null
watch(() => store.notice, async notice => {
  if (notice) { previous = document.activeElement as HTMLElement; await nextTick(); dialog.value?.focus() }
  else previous?.focus()
})
function close() { store.notice = null }
function openStep() { const path = store.notice?.path; close(); if (path) void router.push(path) }
function keys(e: KeyboardEvent) {
  if (e.key === 'Escape') { close(); return }
  if (e.key !== 'Tab' || !dialog.value) return
  const buttons = Array.from(dialog.value.querySelectorAll<HTMLButtonElement>('button'))
  const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
  e.preventDefault()
  buttons[(index + (e.shiftKey ? -1 : 1) + buttons.length) % buttons.length]?.focus()
}
</script>
<template>
  <Teleport to="body">
    <div v-if="store.notice" class="analysis-notice-backdrop">
      <section ref="dialog" class="analysis-notice card shadow-lg" role="dialog" aria-modal="true" aria-labelledby="analysis-notice-title" tabindex="-1" @keydown="keys">
        <div class="card-body">
          <h5 id="analysis-notice-title">{{ locale === 'zh' ? '分析提示' : 'Analysis notice' }}</h5>
          <p class="notice-message">{{ store.notice.message }}</p>
          <p class="small text-muted">{{ locale === 'zh' ? '请根据提示检查相关输入或参数，再继续分析。' : 'Check the relevant inputs or parameters before continuing. The current analysis state is preserved.' }}</p>
          <div class="d-flex justify-content-end gap-2">
            <button v-if="store.notice.path" class="btn btn-primary" @click="openStep">{{ locale === 'zh' ? '查看相关步骤' : 'Open relevant step' }}</button>
            <button class="btn btn-outline-secondary" @click="close">{{ locale === 'zh' ? '关闭' : 'Close' }}</button>
          </div>
        </div>
      </section>
    </div>
  </Teleport>
</template>
<style scoped>
.analysis-notice-backdrop { position: fixed; inset: 0; background: #0007; z-index: 2200; display: grid; place-items: center; padding: 24px; }
.analysis-notice { width: min(680px, 100%); max-height: 85vh; overflow: auto; }
.notice-message { white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
