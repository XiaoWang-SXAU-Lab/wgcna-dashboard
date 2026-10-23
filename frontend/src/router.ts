import { createRouter, createWebHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import DataImportView from './views/DataImportView.vue'
import SftView from './views/SftView.vue'
import NetworkView from './views/NetworkView.vue'
import ModuleTraitView from './views/ModuleTraitView.vue'
import ModuleExplorerView from './views/ModuleExplorerView.vue'
import OneStepView from './views/OneStepView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/data-import', name: 'data-import', component: DataImportView },
    { path: '/sft', name: 'sft', component: SftView },
    { path: '/network', name: 'network', component: NetworkView },
    { path: '/module-trait', name: 'module-trait', component: ModuleTraitView },
    { path: '/module-explorer', name: 'module-explorer', component: ModuleExplorerView },
    { path: '/interested-mod', redirect: '/module-explorer' },
    { path: '/hub-gene', redirect: '/module-explorer' },
    { path: '/one-step', name: 'one-step', component: OneStepView },
  ],
})
