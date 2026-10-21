<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
const route = useRoute()
const title = computed(() => (route.name === 'analysis' ? '分析工作台' : '主页'))
</script>

<template>
  <header class="topbar d-flex align-items-center justify-content-between px-4">
    <h4 class="mb-0 fw-semibold">{{ title }}</h4>
    <span class="badge text-bg-success d-flex align-items-center gap-1">
      <span class="spinner-grow spinner-grow-sm" role="status" aria-hidden="true"></span>
      服务运行中
    </span>
  </header>
</template>
