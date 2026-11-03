<script setup lang="ts">
import { computed, ref } from 'vue'
import { useProjectStore } from '../stores/project'
import type { ModulesResult, SftResult } from '../api/client'
import { locale } from '../utils/i18n'
import DendrogramChart from '../components/DendrogramChart.vue'
import SimpleTable from '../components/SimpleTable.vue'
import SidebarDrawer from '../components/SidebarDrawer.vue'
import PlotDownloadWidget from '../components/PlotDownloadWidget.vue'
import { moduleColor } from '../utils/colors'

const store = useProjectStore()
const text = (zh: string, en: string) => locale.value === 'zh' ? zh : en
const network = computed(() => store.results.network as ModulesResult | undefined)
const showExport = ref(false)
const sft = computed(() => store.results.soft_threshold as SftResult | undefined)
const canRun = computed(() => store.current?.validity.filter === '' && store.current?.validity.soft_threshold === '' && !store.busy)
const moduleSizeRows = computed(() => {
  const sizes = network.value?.module_sizes ?? {}
  return Object.entries(sizes).map(([m, n]) => ({ module: m, count: n, color: moduleColor(m) })).sort((a, b) => b.count - a.count)
})
const geneRows = computed(() => (network.value?.genes ?? []).map((g, i) => ({ gene: g, color: network.value?.colors[i] ?? 'grey' })))
function useSft() {
  if (sft.value?.power_estimate != null) { store.draft.network.power = sft.value.power_estimate; store.draft.network.network_type = sft.value.network_type }
}
</script>

<template>
  <div v-if="!store.current" class="text-center py-5">
    <i class="bi bi-folder2-open fs-1 text-muted"></i>
    <h4 class="mt-3">{{ text('尚未打开项目', 'No project open') }}</h4>
    <p class="text-muted">{{ text('请先新建或打开项目并完成过滤与软阈值，再构建网络。', 'Open or create a project and finish filtering + soft threshold before building the network.') }}</p>
    <router-link to="/projects" class="btn btn-primary mt-2">{{ text('前往项目', 'Go to projects') }}</router-link>
  </div>
  <SidebarDrawer v-else>
    <template #sidebar>
      <fieldset :disabled="store.busy" class="vstack gap-3">
        <label>β<input v-model.number="store.draft.network.power" type="number" min="1" max="30" class="form-control" /></label>
        <button v-if="sft?.power_estimate" class="btn btn-outline-primary" @click="useSft">{{ text('使用软阈值推荐', 'Use SFT recommendation') }} ({{ sft.power_estimate }})</button>
        <label>{{ text('网络类型', 'Network type') }}<select v-model="store.draft.network.network_type" class="form-select"><option>unsigned</option><option>signed</option><option>signed hybrid</option></select></label>
        <label>TOM type<select v-model="store.draft.network.tom_type" class="form-select"><option value="signed">signed</option><option value="unsigned">unsigned</option></select></label>
        <label>TOM denom<select v-model="store.draft.network.tom_denom" class="form-select"><option value="min">min</option><option value="mean">mean</option></select></label>
        <label>{{ text('最小模块大小', 'Minimum module size') }}<input v-model.number="store.draft.network.min_module_size" type="number" min="2" class="form-control" /></label>
        <label>{{ text('模块合并阈值', 'Merge cut height') }}<input v-model.number="store.draft.network.merge_cut_height" type="number" min="0" max="1" step="0.05" class="form-control" /></label>
        <label>deepSplit<input v-model.number="store.draft.network.deep_split" type="number" min="0" max="4" class="form-control" /></label>
        <label>{{ text('检测阈值', 'Detection cut height') }}<input v-model.number="store.draft.network.detect_cut_height" type="number" min="0" max="1" step="0.05" class="form-control" /></label>
      </fieldset>
      <button class="btn btn-primary w-100 mt-3" :disabled="!canRun" @click="store.run('network')">{{ text('运行此步骤', 'Run step') }}</button>
    </template>
    <div class="mb-3 d-flex align-items-start gap-2">
      <div class="flex-grow-1"><h4 class="mb-1">{{ text('网络与模块', 'Network & modules') }}</h4><p class="text-muted small mb-0">{{ text('构建共表达网络并识别基因模块。', 'Build the co-expression network and detect gene modules.') }}</p></div>
      <button v-if="network" class="btn btn-sm btn-outline-primary" @click="showExport = true"><i class="bi bi-download me-1"></i>{{ text('导出高清图', 'Export') }}</button>
    </div>
    <div v-if="store.error" class="alert alert-danger">{{ store.error }}</div>
    <p v-if="store.busy" role="status" class="alert alert-info">{{ text('正在计算，完成后自动保存。', 'Running. Results will be saved on completion.') }}</p>
    <template v-else-if="network">
      <div class="d-flex flex-wrap gap-2 mb-3">
        <span v-for="r in moduleSizeRows" :key="r.module" class="badge text-bg-light border">
          <span class="d-inline-block rounded-circle me-1" :style="{ width: '10px', height: '10px', background: r.color }"></span>{{ r.module }}（{{ r.count }}）
        </span>
      </div>
      <DendrogramChart :genes="network.genes" :colors="network.colors" :dendrogram="network.dendrogram" />
      <SimpleTable class="mt-3" :columns="[{ key: 'gene', label: text('基因', 'Gene') }, { key: 'color', label: text('模块', 'Module') }]" :rows="geneRows" scroll-height="500px" filename="gene_module" />
    </template>
    <p v-else class="text-muted">{{ text('完成软阈值后运行，这里会显示模块识别结果。', 'Run after soft-threshold to see the module detection results.') }}</p>
    <PlotDownloadWidget kind="dendrogram" :show="showExport" @close="showExport = false" />
  </SidebarDrawer>
</template>

<style scoped>
fieldset { border: 0; min-width: 0; padding: 0; }
label { font-size: .875rem; }
</style>
