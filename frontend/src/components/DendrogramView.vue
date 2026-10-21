<script setup lang="ts">
import { computed, ref } from 'vue'

const props = defineProps<{ jobId: string }>()

const interactive = ref(false)
const src = computed(
  () => '/api/jobs/' + props.jobId + '/plots/dendrogram?mode=' + (interactive.value ? 'interactive' : 'static'),
)
</script>

<template>
  <div>
    <div class="d-flex justify-content-end align-items-center gap-2 mb-2">
      <span class="text-muted small">静态（ggplot）</span>
      <div class="form-check form-switch mb-0">
        <input class="form-check-input" type="checkbox" role="switch" id="dendroInteractive" v-model="interactive" />
        <label class="form-check-label small" for="dendroInteractive">交互式（plotly）</label>
      </div>
    </div>
    <iframe
      v-if="interactive"
      :src="src"
      class="w-100 border-0 rounded bg-white"
      style="height: 560px"
      title="模块树状图"
    ></iframe>
    <img v-else :src="src" class="w-100 rounded" style="max-height: 560px; object-fit: contain" alt="模块树状图" />
  </div>
</template>
