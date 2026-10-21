import { createRouter, createWebHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import DataImportView from './views/DataImportView.vue'
import SftView from './views/SftView.vue'
import NetworkView from './views/NetworkView.vue'
import ModuleTraitView from './views/ModuleTraitView.vue'
import InterestedModView from './views/InterestedModView.vue'
import HubGeneView from './views/HubGeneView.vue'
import OneStepView from './views/OneStepView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/data-import', name: 'data-import', component: DataImportView },
    { path: '/sft', name: 'sft', component: SftView },
    { path: '/network', name: 'network', component: NetworkView },
    { path: '/module-trait', name: 'module-trait', component: ModuleTraitView },
    { path: '/interested-mod', name: 'interested-mod', component: InterestedModView },
    { path: '/hub-gene', name: 'hub-gene', component: HubGeneView },
    { path: '/one-step', name: 'one-step', component: OneStepView },
  ],
})
