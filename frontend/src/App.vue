<script setup lang="ts">
import { onMounted, ref } from 'vue'
import ChatWidget from './components/ChatWidget.vue'
import ProgressOverlay from './components/ProgressOverlay.vue'
import { registerAnalyzer } from './utils/aiBus'
import { locale, t, toggleLocale } from './utils/i18n'

const chatWidget = ref<{ analyzeError: (err: string) => void }>()

onMounted(() => {
  registerAnalyzer((err: string) => chatWidget.value?.analyzeError(err))
})

const nav = [
  { to: '/', key: 'nav.home', icon: 'bi-house-door' },
  { to: '/data-import', key: 'nav.dataImport', icon: 'bi-upload' },
  { to: '/sft', key: 'nav.sft', icon: 'bi-activity' },
  { to: '/network', key: 'nav.network', icon: 'bi-diagram-3' },
  { to: '/module-trait', key: 'nav.moduleTrait', icon: 'bi-table' },
  { to: '/interested-mod', key: 'nav.interestedMod', icon: 'bi-search' },
  { to: '/hub-gene', key: 'nav.hubGene', icon: 'bi-stars' },
  { to: '/one-step', key: 'nav.oneStep', icon: 'bi-lightning-charge' },
]
</script>

<template>
  <nav class="navbar navbar-expand-lg bg-white border-bottom sticky-top py-2">
    <div class="container-fluid flex-nowrap">
      <router-link class="navbar-brand fw-bold d-flex align-items-center gap-2 me-3" to="/">
        <img src="/rsWGCNA_icon.svg" alt="rsWGCNA" width="30" height="30" />
        <span>rsWGCNA</span><small class="text-muted fw-normal ms-1">v0.1.0</small>
      </router-link>
      <ul class="navbar-nav flex-row flex-nowrap overflow-auto">
        <li v-for="n in nav" :key="n.to" class="nav-item">
          <router-link class="nav-link text-nowrap" :to="n.to">
            <i :class="['bi', n.icon, 'me-1']"></i>{{ t(n.key) }}
          </router-link>
        </li>
      </ul>
      <button class="btn btn-sm btn-outline-secondary ms-auto text-nowrap" @click="toggleLocale()">
        <i class="bi bi-translate me-1"></i>{{ locale === 'en' ? '中文' : 'EN' }}
      </button>
    </div>
  </nav>
  <main class="p-3 p-lg-4">
    <router-view />
  </main>
  <ChatWidget ref="chatWidget" />
  <ProgressOverlay />
</template>

<style scoped>
.navbar .nav-link { color: #4b5563; font-weight: 500; border-bottom: 3px solid transparent; }
.navbar .nav-link.router-link-active { color: #008080; border-bottom: 3px solid #008080; }
</style>
