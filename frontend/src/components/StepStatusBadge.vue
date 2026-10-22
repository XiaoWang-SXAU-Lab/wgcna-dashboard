<script setup lang="ts">
import { computed } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import { t } from '../utils/i18n'

const props = defineProps<{ step: string }>()
const store = useWgcnaStore()

const status = computed(() => store.stepStatus(props.step))

const badgeClass = computed(() => {
  switch (status.value) {
    case 'running':
      return 'text-bg-primary'
    case 'done':
      return 'text-bg-success'
    case 'error':
      return 'text-bg-danger'
    default:
      return 'text-bg-secondary'
  }
})

const label = computed(() => {
  switch (status.value) {
    case 'running':
      return t('common.running')
    case 'done':
      return t('common.done')
    case 'error':
      return t('common.error')
    default:
      return t('common.idle')
  }
})
</script>

<template>
  <span class="badge" :class="badgeClass">
    <span v-if="status === 'running'" class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span>
    <i v-else-if="status === 'done'" class="bi bi-check-circle me-1"></i>
    <i v-else-if="status === 'error'" class="bi bi-x-circle me-1"></i>
    {{ label }}
  </span>
</template>
