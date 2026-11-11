import { flushReportFigures } from './utils/reportSnapshots'
import { createRouter, createWebHistory } from 'vue-router'
import { useProjectStore } from './stores/project'
import HomeView from './views/HomeView.vue'
import ProjectView from './views/ProjectView.vue'
import DataImportView from './views/DataImportView.vue'
import SftView from './views/SftView.vue'
import NetworkView from './views/NetworkView.vue'
import ProjectPlaceholderView from './views/ProjectPlaceholderView.vue'
import ProjectReportView from './views/ProjectReportView.vue'
import ModuleTraitView from './views/ModuleTraitView.vue'
import ModuleExplorerView from './views/ModuleExplorerView.vue'
import ToolkitEggnogView from './views/ToolkitEggnogView.vue'
import ToolkitBlastView from './views/ToolkitBlastView.vue'
import ToolkitGuideView from './views/ToolkitGuideView.vue'
import ToolkitRegutraceView from './views/ToolkitRegutraceView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/projects', name: 'projects', component: ProjectView },
    { path: '/data-import', name: 'data-import', component: DataImportView },
    { path: '/sft', name: 'sft', component: SftView },
    { path: '/network', name: 'network', component: NetworkView },
    { path: '/module-trait', name: 'module-trait', component: ModuleTraitView },
    { path: '/module-explorer', name: 'module-explorer', component: ModuleExplorerView },
    { path: '/toolkits/eggnog', name: 'toolkit-eggnog', component: ToolkitEggnogView },
    { path: '/toolkits/blast-merge', name: 'toolkit-blast', component: ToolkitBlastView },
    { path: '/toolkits/regutrace', name: 'toolkit-regutrace', component: ToolkitRegutraceView },
    { path: '/toolkits/guide', name: 'toolkit-guide', component: ToolkitGuideView },
    { path: '/module-overview', name: 'module-overview', component: ProjectPlaceholderView, meta: { title: '模块总览', titleEn: 'Module overview' } },
    { path: '/report', name: 'report', component: ProjectReportView, meta: { title: '分析报告', titleEn: 'Report' } },
    { path: '/interested-mod', redirect: '/module-explorer' },
    { path: '/hub-gene', redirect: '/module-explorer' },
  ],
})

router.beforeEach(async () => {
  await flushReportFigures()
  const project = useProjectStore()
  try { await project.flush() } catch { /* best-effort save on navigation; autosave retries */ }
})
export default router
