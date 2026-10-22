<script setup lang="ts">
import { computed, ref } from 'vue'
import { useWgcnaStore } from '../stores/wgcna'
import DendrogramChart from '../components/DendrogramChart.vue'
import SimpleTable from '../components/SimpleTable.vue'
import { moduleColor } from '../utils/colors'
import { analyzeError } from '../utils/aiBus'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import { t } from '../utils/i18n'
import StepStatusBadge from '../components/StepStatusBadge.vue'

const store = useWgcnaStore()

const moduleSizeRows = computed(() => {
  const sizes = store.modules?.module_sizes ?? {}
  return Object.entries(sizes).map(([m, n]) => ({ module: m, count: n, color: moduleColor(m) })).sort((a, b) => b.count - a.count)
})

const geneRows = computed(() => (store.modules?.genes ?? []).map((g, i) => ({ gene: g, color: store.modules?.colors[i] ?? 'grey' })))
</script>

<template>
  <SidebarDrawer>
    <template #sidebar>
      <div>
        <h6 class="fw-bold mb-3">{{ t('param.parameters') }}</h6>
        <div class="vstack gap-2">
          <div>
            <label class="form-label small mb-1">{{ t('param.power') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.power" min="1" max="30" />
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.networkType') }}</label>
            <select class="form-select form-select-sm" v-model="store.netNetworkType">
              <option value="unsigned">unsigned</option>
              <option value="signed">signed</option>
            </select>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.tomType') }}</label>
            <select class="form-select form-select-sm" v-model="store.tomType">
              <option value="signed">signed</option>
              <option value="unsigned">unsigned</option>
            </select>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.tomDenom') }}</label>
            <select class="form-select form-select-sm" v-model="store.tomDenom">
              <option value="min">min</option>
              <option value="mean">mean</option>
            </select>
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.minModuleSize') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.minModuleSize" min="10" />
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.mergeCutHeight') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.mergeCutHeight" step="0.05" min="0" max="1" />
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.deepSplit') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.deepSplit" min="0" max="4" />
          </div>
          <div>
            <label class="form-label small mb-1">{{ t('param.detectCutHeight') }}</label>
            <input class="form-control form-control-sm" type="number" v-model.number="store.detectCutHeight" step="0.05" min="0" max="1" />
          </div>
          <button class="btn btn-primary w-100 mt-2" :disabled="!store.stepReady('network') || store.stepRunning !== ''" @click="store.runStep('network')">
            {{ store.stepRunning === 'network' ? t('common.running') : store.isDone('network') ? t('common.rerun') : t('common.run') }}
          </button>
        </div>
      </div>
    </template>
      <div class="mb-3">
        <h4 class="mb-1 d-flex align-items-center gap-2">{{ t('net.title') }} <StepStatusBadge step="network" /></h4>
        <p class="text-muted small mb-0">{{ t('net.desc') }}</p>
      </div>
      <div v-if="store.error" class="alert alert-danger d-flex align-items-center gap-2 py-2">
        <i class="bi bi-exclamation-triangle-fill"></i>
        <span class="flex-grow-1">{{ store.error }}</span>
        <button class="btn btn-sm btn-outline-danger" @click="analyzeError(store.error)">🤖 {{ t('common.ai') }}</button>
      </div>
      <div v-if="store.modules">
        <div class="d-flex flex-wrap gap-2 mb-3">
          <span v-for="r in moduleSizeRows" :key="r.module" class="badge text-bg-light border">
            <span class="d-inline-block rounded-circle me-1" :style="{ width: '10px', height: '10px', background: r.color }"></span>
            {{ r.module }}（{{ r.count }}）
          </span>
        </div>
        <DendrogramChart :genes="store.modules.genes" :colors="store.modules.colors" :dendrogram="store.modules.dendrogram" />
        <SimpleTable class="mt-3" :columns="[{ key: 'gene', label: t('net.col.gene') }, { key: 'color', label: t('net.col.module') }]" :rows="geneRows" scroll-height="500px" filename="gene_module" />
      </div>
      <p v-else class="text-muted">{{ t('net.empty') }}</p>
  </SidebarDrawer>
</template>
