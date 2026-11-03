<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { locale } from '../utils/i18n'
import { useProjectStore } from '../stores/project'
const route = useRoute()
const store = useProjectStore()
const en = computed(() => locale.value === 'en')
const title = computed(() => (en.value ? (route.meta.titleEn as string) : (route.meta.title as string)) || '')
const projectName = computed(() => store.current?.state.name ?? '')
</script>

<template>
  <div class="text-center py-5">
    <i class="bi bi-tools fs-1 text-muted"></i>
    <h4 class="mt-3">{{ title }}</h4>
    <p class="text-muted mb-1">{{ en ? 'This page is not yet wired into the project workspace, so it cannot produce results here.' : '此分析页面尚未接入「项目」模式，暂不能在这里生成结果。' }}</p>
    <p v-if="projectName" class="text-muted small">{{ en ? 'Current project' : '当前项目' }}：<b>{{ projectName }}</b></p>
    <router-link to="/projects" class="btn btn-outline-primary mt-2">{{ en ? 'Back to project' : '前往项目' }}</router-link>
  </div>
</template>
