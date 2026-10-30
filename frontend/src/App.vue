<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useWgcnaStore } from './stores/wgcna'
import ChatWidget from './components/ChatWidget.vue'
import ProgressOverlay from './components/ProgressOverlay.vue'
import AnalysisNotice from './components/AnalysisNotice.vue'
import InputValidationDialog from './components/InputValidationDialog.vue'
import { registerAnalyzer } from './utils/aiBus'
import { locale, t, toggleLocale } from './utils/i18n'

const chatWidget = ref<{ analyzeError: (err: string) => void }>()

onMounted(() => {
  registerAnalyzer((err: string) => chatWidget.value?.analyzeError(err))
})

const store = useWgcnaStore()
const stepPaths: Record<string, string> = { filter: '/data-import', soft_threshold: '/sft', network: '/network', module_trait: '/module-trait', module_detail: '/module-explorer', hub: '/module-explorer' }

const nav = [
  { to: '/', key: 'nav.home', icon: 'bi-house-door' },
  { to: '/data-import', key: 'nav.dataImport', icon: 'bi-upload' },
  { to: '/sft', key: 'nav.sft', icon: 'bi-activity' },
  { to: '/network', key: 'nav.network', icon: 'bi-diagram-3' },
  { to: '/module-trait', key: 'nav.moduleTrait', icon: 'bi-table' },
  { to: '/module-explorer', key: 'nav.moduleExplorer', icon: 'bi-search' },
  { to: '/module-overview', key: 'nav.moduleOverview', icon: 'bi-grid-1x2' },
  { to: '/report', key: 'nav.report', icon: 'bi-file-earmark-text' },
]
</script>

<template>
  <nav class="navbar navbar-expand-lg bg-white border-bottom sticky-top py-2">
    <div class="container-fluid flex-nowrap">
      <router-link class="navbar-brand fw-bold d-flex align-items-center gap-2 me-3" to="/">
        <img src="/rsWGCNA_icon.svg" alt="rsWGCNA" width="30" height="30" />
        <span>rsWGCNA</span><small class="text-muted fw-normal ms-1">v0.6.0</small>
      </router-link>
      <ul class="navbar-nav flex-row flex-nowrap overflow-auto">
        <li v-for="n in nav" :key="n.to" class="nav-item">
          <router-link class="nav-link text-nowrap" :to="n.to">
            <i :class="['bi', n.icon, 'me-1']"></i>{{ t(n.key) }}
          </router-link>
        </li>
      </ul>
      <div class="d-flex align-items-center gap-2 ms-auto flex-shrink-0">
        <a class="navbar-contact" href="https://github.com/XiaoWang-SXAU-Lab/wgcna-dashboard" target="_blank" rel="noopener noreferrer" :title="locale === 'en' ? 'Project on GitHub' : '项目 GitHub 仓库'" :aria-label="locale === 'en' ? 'Project on GitHub (opens in a new tab)' : '项目 GitHub 仓库（新窗口打开）'">
          <i class="bi bi-github" aria-hidden="true"></i>
        </a>
        <a class="navbar-contact" href="mailto:shawnwang@sxau.edu.cn" :title="locale === 'en' ? 'Email: shawnwang@sxau.edu.cn' : '联系邮箱：shawnwang@sxau.edu.cn'" :aria-label="locale === 'en' ? 'Email shawnwang@sxau.edu.cn' : '发送邮件至 shawnwang@sxau.edu.cn'">
          <i class="bi bi-envelope" aria-hidden="true"></i>
        </a>
      <button class="btn btn-sm btn-outline-secondary text-nowrap" @click="toggleLocale()">
        <i class="bi bi-translate me-1"></i>{{ locale === 'en' ? '中文' : 'EN' }}
      </button>
      </div>
    </div>
  </nav>
  <main class="p-3 p-lg-4">
    <div v-if="store.dirtyStep && !store.stepRunning" class="alert alert-warning small" role="status">
      {{ locale === 'zh' ? '参数已更改，现有结果需要更新。请从相关步骤重新运行：' : 'Settings changed. Existing results need updating; rerun from:' }}
      <router-link :to="stepPaths[store.dirtyStep]">{{ store.dirtyStep }}</router-link>
    </div>
    <router-view />
  </main>
  <ChatWidget ref="chatWidget" />
  <ProgressOverlay />
  <InputValidationDialog />
  <AnalysisNotice />
</template>

<style scoped>
.navbar .nav-link { color: #4b5563; font-weight: 500; border-bottom: 3px solid transparent; }
.navbar .nav-link.router-link-active { color: #008080; border-bottom: 3px solid #008080; }
.navbar-contact { display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px; color: #607589; border-radius: 6px; font-size: 18px; text-decoration: none; }
.navbar-contact:hover { color: #067d89; background: #edf6f7; }
</style>
