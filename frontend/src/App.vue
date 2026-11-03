<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { useProjectStore } from './stores/project'
import ChatWidget from './components/ChatWidget.vue'
import ProgressOverlay from './components/ProgressOverlay.vue'
import ProjectRunProgress from './components/ProjectRunProgress.vue'
import { registerAnalyzer } from './utils/aiBus'
import { locale, t, toggleLocale } from './utils/i18n'

const chatWidget = ref<{ analyzeError: (err: string) => void }>()
const project = useProjectStore()
const toolkitsOpen = ref(false)
const toolkitsBtn = ref<HTMLElement>()
const toolkitsPos = ref({ top: 0, left: 0 })
function toggleToolkits() {
  toolkitsOpen.value = !toolkitsOpen.value
  if (toolkitsOpen.value && toolkitsBtn.value) {
    const r = toolkitsBtn.value.getBoundingClientRect()
    toolkitsPos.value = { top: r.bottom + 4, left: Math.min(r.left, (window.innerWidth || 0) - 240) }
  }
}

onMounted(() => {
  registerAnalyzer((err: string) => chatWidget.value?.analyzeError(err))
  poll = setInterval(async () => {
    if (polling) return
    polling = true
    try {
      if (project.id && (project.current?.busy || !project.dirty)) await project.refresh()
      await project.loadResults()
    } catch { /* keep last known state */ } finally { polling = false }
  }, 800)
  window.addEventListener('beforeunload', beforeUnload)
})
onBeforeUnmount(() => {
  clearInterval(poll)
  window.removeEventListener('beforeunload', beforeUnload)
})
let poll: ReturnType<typeof setInterval>
let polling = false
function beforeUnload(event: BeforeUnloadEvent) {
  if (project.dirty || project.saving) { event.preventDefault(); event.returnValue = '' }
}

const nav = [
  { to: '/', key: 'nav.home', icon: 'bi-house-door' },
  { to: '/projects', key: 'nav.projects', icon: 'bi-folder2-open' },
  { to: '/data-import', key: 'nav.dataImport', icon: 'bi-upload' },
  { to: '/sft', key: 'nav.sft', icon: 'bi-activity' },
  { to: '/network', key: 'nav.network', icon: 'bi-diagram-3' },
  { to: '/module-trait', key: 'nav.moduleTrait', icon: 'bi-table' },
  { to: '/module-explorer', key: 'nav.moduleExplorer', icon: 'bi-search' },
  { to: '/module-overview', key: 'nav.moduleOverview', icon: 'bi-grid-1x2' },
  { to: '/report', key: 'nav.report', icon: 'bi-file-earmark-text' },
]
const isAnalysis = (to: string) => to !== '/' && to !== '/projects'
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const saveLabel = computed(() => project.saving ? text('保存中…', 'Saving…') : project.dirty ? text('有未保存修改', 'Unsaved changes') : text('已保存', 'Saved'))
const saveTone = computed(() => project.saving ? 'text-warning' : project.dirty ? 'text-warning' : 'text-success')
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
          <router-link v-if="!isAnalysis(n.to) || project.current" class="nav-link text-nowrap" :to="n.to">
            <i :class="['bi', n.icon, 'me-1']"></i>{{ t(n.key) }}
          </router-link>
          <span v-else class="nav-link text-nowrap disabled" :title="text('请先打开或新建项目', 'Open or create a project first')">
            <i :class="['bi', n.icon, 'me-1']"></i>{{ t(n.key) }}
          </span>
        </li>
        <li class="nav-item">
          <button ref="toolkitsBtn" class="nav-link text-nowrap border-0 bg-transparent toolkits-glow" @click="toggleToolkits">
            <i class="bi bi-tools me-1"></i>{{ text('工具箱', 'Toolkits') }}
          </button>
          <div v-if="toolkitsOpen" class="dropdown-menu show position-fixed" :style="{ top: toolkitsPos.top + 'px', left: toolkitsPos.left + 'px', zIndex: 1030 }">
            <router-link class="dropdown-item" to="/toolkits/eggnog" @click="toolkitsOpen = false"><i class="bi bi-file-earmark-arrow-down me-2"></i>{{ text('eggNOG 注释转换', 'eggNOG annotation converter') }}</router-link>
            <router-link class="dropdown-item" to="/toolkits/blast-merge" @click="toolkitsOpen = false"><i class="bi bi-shuffle me-2"></i>{{ text('BLAST 注释合并', 'BLAST annotation merge') }}</router-link>
            <hr class="dropdown-divider" />
            <router-link class="dropdown-item" to="/toolkits/guide" @click="toolkitsOpen = false"><i class="bi bi-book me-2"></i>{{ text('详细使用指引', 'Detailed guide') }}</router-link>
          </div>
        </li>
      </ul>
      <div class="d-flex align-items-center gap-2 ms-auto flex-shrink-0">
        <router-link v-if="project.current" to="/projects" class="navbar-project text-decoration-none me-1" :title="project.current.path">
          <span class="fw-semibold">{{ project.current.state.name }}</span>
          <span class="badge text-bg-light border ms-1" :class="saveTone">{{ saveLabel }}</span>
        </router-link>
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
    <router-view />
  </main>
  <ChatWidget v-if="!project.current" ref="chatWidget" />
  <ProgressOverlay />
  <ProjectRunProgress />
</template>

<style scoped>
.navbar .nav-link { color: #4b5563; font-weight: 500; border-bottom: 3px solid transparent; }
.navbar .nav-link.router-link-active { color: #008080; border-bottom: 3px solid #008080; }
.navbar .nav-link.disabled { color: #adb5bd; cursor: not-allowed; }
.navbar-contact { display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px; color: #607589; border-radius: 6px; font-size: 18px; text-decoration: none; }
.navbar-contact:hover { color: #067d89; background: #edf6f7; }
.navbar-project { display: inline-flex; align-items: center; max-width: 260px; color: #25475f; }
.navbar-project span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.toolkits-glow { border-radius: 8px; animation: toolkits-pulse 2.4s ease-in-out infinite; }
@keyframes toolkits-pulse {
  0%, 100% { box-shadow: 0 0 4px rgba(6, 125, 137, 0.25), 0 0 10px rgba(6, 125, 137, 0.15); text-shadow: 0 0 6px rgba(6, 125, 137, 0.35); }
  50% { box-shadow: 0 0 10px rgba(6, 125, 137, 0.55), 0 0 22px rgba(6, 125, 137, 0.3); text-shadow: 0 0 12px rgba(6, 125, 137, 0.7); }
}
</style>
