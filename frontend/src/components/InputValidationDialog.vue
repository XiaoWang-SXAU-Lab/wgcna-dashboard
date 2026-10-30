<script setup lang="ts">
import { useWgcnaStore } from '../stores/wgcna'
import { t } from '../utils/i18n'
import { nextTick, ref, watch } from 'vue'
const store = useWgcnaStore()
const dialog = ref<HTMLElement | null>(null)
let previousFocus: HTMLElement | null = null
watch(() => store.showInputValidation, async (open) => {
  if (open) {
    previousFocus = document.activeElement as HTMLElement | null
    await nextTick()
    dialog.value?.focus()
  } else previousFocus?.focus()
})
function trapFocus(event: KeyboardEvent) {
  if (event.key !== 'Tab' || !dialog.value) return
  const buttons = Array.from(dialog.value.querySelectorAll<HTMLButtonElement>('button'))
  const first = buttons[0], last = buttons[buttons.length - 1]
  if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.value)) {
    event.preventDefault(); last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault(); first?.focus()
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="store.showInputValidation && store.inputValidation" class="input-check-backdrop" @keydown.esc="store.showInputValidation = false">
      <section ref="dialog" @keydown="trapFocus" class="input-check-dialog" role="dialog" aria-modal="true" aria-labelledby="input-check-title" tabindex="-1">
        <header class="d-flex align-items-center justify-content-between border-bottom p-3">
          <h5 id="input-check-title" class="mb-0">{{ t('di.validationTitle') }}</h5>
          <button class="btn-close" :aria-label="t('di.validationClose')" @click="store.showInputValidation = false"></button>
        </header>
        <div class="p-3 input-check-body">
          <p :class="store.inputValidation.valid ? 'text-success' : 'text-danger'">
            {{ store.inputValidation.valid ? t('di.validationWarnings') : t('di.validationBlocked') }}
            ({{ store.inputValidation.error_count }} {{ t('di.validationErrors') }}, {{ store.inputValidation.warning_count }} {{ t('di.validationNotes') }})
          </p>
          <p class="small text-muted">{{ t('di.validationPolicy') }}</p>
          <ul class="list-unstyled mb-0">
            <li v-for="(issue, index) in store.inputValidation.issues" :key="index" class="border rounded p-2 mb-2" :class="issue.severity === 'error' ? 'border-danger' : 'border-warning'">
              <strong>{{ issue.file }}</strong>
              <span v-if="issue.row"> · {{ t('di.validationRow') }} {{ issue.row }}</span>
              <span v-if="issue.column"> · {{ t('di.validationColumn') }} {{ issue.column }}</span>
              <div class="small input-check-message">{{ issue.message }}</div>
            </li>
          </ul>
          <p v-if="store.inputValidation.error_count + store.inputValidation.warning_count > store.inputValidation.issues.length" class="small text-muted">{{ t('di.validationTruncated') }}</p>
        </div>
        <footer class="border-top p-3 text-end">
          <button class="btn btn-primary" autofocus @click="store.showInputValidation = false">{{ t('di.validationClose') }}</button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.input-check-backdrop { position: fixed; inset: 0; z-index: 2100; background: #0007; display: grid; place-items: center; padding: 24px; }
.input-check-dialog { background: white; color: #243746; border-radius: 12px; width: min(760px, 100%); max-height: 85vh; display: flex; flex-direction: column; box-shadow: 0 18px 60px #0004; }
.input-check-body { overflow: auto; }
.input-check-message { white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
