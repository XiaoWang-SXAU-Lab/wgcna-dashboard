import { createApp, h, nextTick, type Component } from 'vue'
import type { DetailResult, EnrichResult, HubResult, ModulesResult, MtaitResult, SftResult } from '../api/client'
import SampleTreeChart from '../components/SampleTreeChart.vue'
import SftPlot from '../components/SftPlot.vue'
import DendrogramChart from '../components/DendrogramChart.vue'
import TraitHeatmap from '../components/TraitHeatmap.vue'
import GsMmScatter from '../components/GsMmScatter.vue'
import ModuleGeneHeatmap from '../components/ModuleGeneHeatmap.vue'
import NetworkGraph from '../components/NetworkGraph.vue'
import ResizableChart from '../components/ResizableChart.vue'
import { makePoints, optionFromPoints } from './enrichmentChart'
import { saveFigure, flushReportFigures } from './reportSnapshots'
import { t } from './i18n'

interface Task { key: string; kind: string; trait: string; fingerprint: string; data: unknown }

function componentFor(task: Task): { component: Component; props: Record<string, unknown> } {
  const data = task.data
  switch (task.kind) {
    case 'sample-tree': {
      const d = data as { samples: string[]; tree: unknown; bands: unknown[] }
      return { component: SampleTreeChart, props: { samples: d.samples, tree: d.tree, categories: d.bands } }
    }
    case 'sft': return { component: SftPlot, props: { data: data as SftResult } }
    case 'dendrogram': { const d = data as ModulesResult; return { component: DendrogramChart, props: { genes: d.genes, colors: d.colors, dendrogram: d.dendrogram } } }
    case 'trait': return { component: TraitHeatmap, props: { data: data as MtaitResult } }
    case 'scatter': return { component: GsMmScatter, props: { data: data as DetailResult } }
    case 'expression': return { component: ModuleGeneHeatmap, props: { data: data as DetailResult } }
    case 'enrich-go': case 'enrich-kegg': {
      const source = task.kind === 'enrich-go' ? 'GO' : 'KEGG'
      return { component: ResizableChart, props: { option: optionFromPoints(makePoints(data as EnrichResult[], source), source + ' ' + t('enrich.title'), '460px') } }
    }
    case 'hub': return { component: NetworkGraph, props: {
      network: (data as HubResult).network, layout: 'fcose', sizeBasis: 'degree', nodeSizeMin: 5, nodeSizeMax: 30,
      edgeLowColor: '#cbd5e1', edgeHighColor: '#e11d48', edgeWidth: 6, labelSize: 10, labelMode: 'hub',
      geneMap: {}, colorMode: 'kme', nodeLowColor: '#dbeafe', nodeHighColor: '#1d4ed8',
      hubNodeColor: '#e11d48', hubBorderColor: '#0f172a', hubBorderWidth: 3,
    } }
    default: throw new Error(`Unsupported report figure: ${task.kind}`)
  }
}

// No second plotting implementation: mount the very same App components and
// photograph their DOM after ECharts/Cytoscape has finished rendering.
export async function prepareReportFigures(job: string, type: 'go' | 'kegg', progress: (text: string) => void) {
  await flushReportFigures()
  progress('正在检查图表快照并准备关键模块数据…')
  const response = await fetch(`/api/jobs/${encodeURIComponent(job)}/report/prepare?type=${type}`, { method: 'POST' })
  if (!response.ok) throw new Error(await response.text())
  const plan = await response.json() as { figures: Task[] }
  for (const [i, task] of plan.figures.entries()) {
    progress(`使用 App 原组件补绘 ${i + 1}/${plan.figures.length}：${task.key}`)
    const host = document.createElement('div')
    host.style.cssText = `position:fixed;left:-20000px;top:0;width:${['scatter','expression'].includes(task.kind) ? 720 : 1280}px;background:white;pointer-events:none;`
    document.body.append(host)
    const { component, props } = componentFor(task)
    const app = createApp({ render: () => h(component, props) })
    let renderError: unknown
    app.config.errorHandler = err => { renderError = err }
    let quiet: ReturnType<typeof setTimeout> | undefined
    let finish: (() => void) | undefined
    const rendered = new Promise<void>((resolve, reject) => {
      finish = resolve
      const timeout = setTimeout(() => reject(new Error(`图表绘制超时：${task.key}`)), 60000)
      finish = () => { clearTimeout(timeout); resolve() }
    })
    const ready = () => { clearTimeout(quiet); quiet = setTimeout(() => finish?.(), 500) }
    host.addEventListener('report-chart-rendered', ready)
    try {
      app.mount(host)
      await nextTick()
      await rendered
      if (renderError) throw renderError
      await saveFigure(job, task.key, host, task.trait, task.fingerprint)
    } finally {
      clearTimeout(quiet); finish?.(); app.unmount(); host.remove()
    }
  }
}
