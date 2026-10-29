// Shared, framework-agnostic ECharts option builders.
//
// These pure functions (data + display params -> EChartsOption) are the single
// source of truth for every chart: the App's Vue components call them, and the
// standalone report bundle (report-charts.js) exposes the exact same functions
// so the report reproduces the App's figures bit-for-bit.
import type { EChartsOption } from 'echarts'
import type { Dendrogram, FitIndex, ScaleFreeCheck } from '../api/client'
import { layoutDendrogram } from './dendrogramLayout'
import { moduleColor } from './colors'
import { pearson, linearRegression, corPvalueStudent, tCritical } from './stats'

export interface CategoryBand {
  name: string
  values: Record<string, string>
}

const PALETTE = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b', '#e377c2', '#7f7f7f', '#bcbd22', '#17becf', '#aec7e8', '#ffbb78']

// ---- Soft threshold: scale independence (R² vs power) ----
export function scaleIndependenceOption(fit: FitIndex[], rsqCut: number, title: string, color = '#5470c6', saveName = 'scale_independence', downloadTitle = 'Download PNG'): EChartsOption {
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis' },
    toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: '#ffffff' } } },
    xAxis: { type: 'category', name: 'Soft Threshold (power)', nameLocation: 'middle', nameGap: 32, data: fit.map((f) => f.power) },
    yAxis: { type: 'value', name: 'Scale Free Topology Model Fit, signed R²', nameLocation: 'middle', nameRotate: 90, nameGap: 55 },
    series: [{
      type: 'line',
      data: fit.map((f) => +f.rsq.toFixed(3)),
      symbolSize: 8,
      lineStyle: { width: 2, color },
      markLine: {
        data: [{ yAxis: rsqCut, label: { formatter: 'R² = ' + rsqCut } }],
        lineStyle: { color: '#e11d48', type: 'dashed' },
      },
    }],
  }
}

// ---- Soft threshold: mean connectivity vs power ----
export function meanConnectivityOption(fit: FitIndex[], title: string, color = '#5470c6', saveName = 'mean_connectivity', downloadTitle = 'Download PNG'): EChartsOption {
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis' },
    toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: '#ffffff' } } },
    xAxis: { type: 'category', name: 'Soft Threshold (power)', nameLocation: 'middle', nameGap: 32, data: fit.map((f) => f.power) },
    yAxis: { type: 'value', name: 'Mean Connectivity', nameLocation: 'middle', nameRotate: 90, nameGap: 55 },
    series: [{
      type: 'line',
      data: fit.map((f) => +f.mean_k.toFixed(3)),
      symbolSize: 8,
      lineStyle: { width: 2, color },
      areaStyle: { opacity: 0.08 },
    }],
  }
}

// ---- Soft threshold: connectivity histogram (k vs frequency) ----
export function sftHistogramOption(sc: ScaleFreeCheck, title: string, freqName = 'Frequency'): EChartsOption {
  return {
    title: { text: `${title} (power = ${sc.power})`, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 60, right: 20, top: 50, bottom: 40 },
    xAxis: { type: 'category', name: 'k', nameLocation: 'middle', nameGap: 32, data: sc.hist_mids.map((m) => m.toFixed(1)) },
    yAxis: { type: 'value', name: freqName, nameLocation: 'middle', nameRotate: 90, nameGap: 55 },
    series: [{
      type: 'bar',
      data: sc.hist_counts,
      itemStyle: { color: '#94a3b8', borderColor: '#334155', borderWidth: 0.5 },
      barCategoryGap: '5%',
    }],
  }
}

// ---- Soft threshold: log-log scale-free check ----
export function sftLogLogOption(sc: ScaleFreeCheck, title: string): EChartsOption {
  const { log_k: x, log_p: y, intercept, slope, adj_r2 } = sc
  const x0 = Math.min(...x)
  const x1 = Math.max(...x)
  const line = [[x0, intercept + slope * x0], [x1, intercept + slope * x1]]
  return {
    title: {
      text: `${title}\nscale R²=${adj_r2.toFixed(2)}, slope=${slope.toFixed(2)}`,
      left: 'center',
      textStyle: { fontSize: 13 },
    },
    tooltip: { trigger: 'item' },
    grid: { left: 60, right: 20, top: 60, bottom: 40 },
    xAxis: { type: 'value', name: 'log10(k)', nameLocation: 'middle', nameGap: 32 },
    yAxis: { type: 'value', name: 'log10(p(k))', nameLocation: 'middle', nameRotate: 90, nameGap: 55 },
    series: [
      {
        type: 'scatter',
        data: x.map((v, i) => [v, y[i]]),
        symbolSize: 9,
        itemStyle: { color: '#5470c6', borderColor: '#1e40af', borderWidth: 1 },
      },
      {
        type: 'line',
        data: line,
        symbol: 'none',
        lineStyle: { color: '#e11d48', width: 2 },
      },
    ],
  }
}

// ---- Sample clustering tree (dendrogram + annotation bands) ----
export function sampleTreeOption(samples: string[], tree: Dendrogram, bands: CategoryBand[], selected: string[] = [], saveName = 'sample_cluster', downloadTitle = 'Download PNG'): EChartsOption {
  const order = tree.order.map((i) => i - 1)
  const sampleNames = order.map((i) => samples[i] ?? '')
  const selectedSet = new Set(selected)
  const layout = layoutDendrogram(tree.merge, tree.height, order, 0.03)
  const n = layout.n
  const minH = layout.minH
  const maxH = layout.maxH
  const span = Math.max(maxH - minH, 1e-6)
  const lowestLeafY = minH - layout.hangScaled

  const leafTipY = new Array(n).fill(lowestLeafY)
  for (let i = 0; i < tree.merge.length; i++) {
    const tip = tree.height[i] - layout.hangScaled
    const a = tree.merge[i][0]
    const b = tree.merge[i][1]
    if (a < 0) leafTipY[-a - 1] = tip
    if (b < 0) leafTipY[-b - 1] = tip
  }

  const renderItem = (params: any, api: any): any => {
    const children: any[] = []
    for (const [x1, x2, y] of layout.hSegments) {
      const a = api.coord([x1, y])
      const b = api.coord([x2, y])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    }
    for (const [x, y1, y2] of layout.vSegments) {
      const a = api.coord([x, y1])
      const b = api.coord([x, y2])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    }
    return { type: 'group', children }
  }

  const cats = bands.map((c) => {
    const vals = Object.values(c.values)
    const uniq = [...new Set(vals)].sort()
    const m: Record<string, string> = {}
    uniq.forEach((v, i) => { m[v] = PALETTE[i % PALETTE.length] })
    return { name: c.name, values: c.values, palette: m }
  })
  const K = cats.length
  const grids: any[] = [{ left: 110, right: 16, top: '4%', bottom: K > 0 ? 48 + K * 30 : 44 }]
  const xAxes: any[] = [{ type: 'value', min: -0.5, max: n - 0.5, show: false, gridIndex: 0 }]
  const yAxes: any[] = [{
    type: 'value', min: lowestLeafY - 0.08 * span, max: maxH * 1.03,
    name: 'Height', nameLocation: 'middle', nameRotate: 90, nameGap: 55, gridIndex: 0,
    axisLabel: { formatter: (v: number) => (v < minH ? '' : v.toFixed(2)) },
    splitLine: { show: true, lineStyle: { color: '#eef1f4', type: 'dashed' } },
  }]
  const series: any[] = [
    { type: 'custom', renderItem, data: [0], xAxisIndex: 0, yAxisIndex: 0, z: 1 },
    {
      type: 'scatter', xAxisIndex: 0, yAxisIndex: 0, z: 10,
      data: Array.from({ length: n }, (_, p) => ({ value: [p, leafTipY[order[p]]], name: sampleNames[p] })),
      symbolSize: 9,
      itemStyle: {
        color: (p: any) => (selectedSet.has(p.name) ? '#dc2626' : '#0d9488'),
        borderColor: '#ffffff',
        borderWidth: 1.5,
      },
      emphasis: { scale: 1.6 },
    },
  ]

  for (let i = 0; i < K; i++) {
    const gi = i + 1
    const band = cats[i]
    grids.push({ left: 110, right: 16, bottom: 10 + (K - 1 - i) * 30, height: 26 })
    xAxes.push({ type: 'value', min: -0.5, max: n - 0.5, show: false, gridIndex: gi })
    yAxes.push({
      type: 'value', min: 0, max: 1, gridIndex: gi,
      name: band.name, nameLocation: 'middle', nameGap: 12,
      nameRotate: 0,
      nameTextStyle: { fontSize: 11, color: '#4b5563', align: 'right' },
      axisLine: { show: false }, axisTick: { show: false }, axisLabel: { show: false }, splitLine: { show: false },
    })
    series.push({
      type: 'custom', data: [0], xAxisIndex: gi, yAxisIndex: gi, z: 5,
      renderItem: (params: any, api: any): any => {
        const children: any[] = []
        for (let p = 0; p < n; p++) {
          const name = sampleNames[p]
          const color = band.palette[band.values[name]] ?? '#d1d5db'
          const x0 = api.coord([p - 0.5, 0])[0]
          const x1 = api.coord([p + 0.5, 0])[0]
          const yTop = api.coord([p, 1])[1]
          const yBot = api.coord([p, 0])[1]
          children.push({ type: 'rect', shape: { x: x0, y: yTop, width: x1 - x0, height: yBot - yTop }, style: { fill: color } })
        }
        return { type: 'group', children }
      },
    })
  }

  return {
    grid: grids,
    xAxis: xAxes,
    yAxis: yAxes,
    tooltip: {
      trigger: 'item', confine: true, backgroundColor: '#0f172a', borderWidth: 0,
      textStyle: { color: '#fff', fontSize: 12 },
      formatter: (params: any) => {
        if (params.seriesType === 'scatter') {
          const name = params.name
          const parts = cats.map((c) => `${c.name}: ${c.values[name] ?? ''}`)
          return name + (parts.length ? '<br/>' + parts.join('<br/>') : '')
        }
        return ''
      },
    },
    toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: '#ffffff' } } },
    series,
  }
}

// ---- Gene clustering dendrogram with module color band ----
export function geneDendrogramOption(genes: string[], colors: string[], dendrogram: Dendrogram, downloadTitle = 'Download PNG', saveName = 'module_dendrogram'): EChartsOption {
  const { merge, height } = dendrogram
  const order = dendrogram.order.map((i) => i - 1)
  const layout = layoutDendrogram(merge, height, order, 0.03)
  const n = layout.n
  const minH = layout.minH
  const maxH = layout.maxH
  const lowestLeafY = minH - layout.hangScaled
  const barH = (maxH - minH) * 0.3

  const renderItem = (params: any, api: any): any => {
    const children: any[] = []
    for (let p = 0; p < n; p++) {
      const geneIdx = order[p]
      const color = moduleColor(colors[geneIdx])
      const tl = api.coord([p - 0.5, lowestLeafY])
      const br = api.coord([p + 0.5, lowestLeafY - barH])
      children.push({
        type: 'rect',
        shape: { x: tl[0], y: tl[1], width: br[0] - tl[0], height: br[1] - tl[1] },
        style: { fill: color },
      })
    }
    const bTop1 = api.coord([-0.5, lowestLeafY])
    const bTop2 = api.coord([n - 0.5, lowestLeafY])
    children.push({ type: 'line', shape: { x1: bTop1[0], y1: bTop1[1], x2: bTop2[0], y2: bTop2[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    const bBot1 = api.coord([-0.5, lowestLeafY - barH])
    const bBot2 = api.coord([n - 0.5, lowestLeafY - barH])
    children.push({ type: 'line', shape: { x1: bBot1[0], y1: bBot1[1], x2: bBot2[0], y2: bBot2[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    for (const [x1, x2, y] of layout.hSegments) {
      const a = api.coord([x1, y])
      const b = api.coord([x2, y])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    }
    for (const [x, y1, y2] of layout.vSegments) {
      const a = api.coord([x, y1])
      const b = api.coord([x, y2])
      children.push({ type: 'line', shape: { x1: a[0], y1: a[1], x2: b[0], y2: b[1] }, style: { stroke: '#111827', lineWidth: 1 } })
    }
    const lp = api.coord([0, lowestLeafY - barH / 2])
    children.push({ type: 'text', style: { text: 'Module color', x: 10, y: lp[1], fill: '#6b7280', fontSize: 11, textAlign: 'left', textVerticalAlign: 'middle' } })
    return { type: 'group', children }
  }

  return {
    grid: { left: 80, right: 16, top: 20, bottom: 20 },
    xAxis: { type: 'value', min: -0.5, max: n - 0.5, show: false },
    yAxis: {
      type: 'value',
      min: lowestLeafY - barH,
      max: maxH * 1.03,
      name: 'Height',
      nameLocation: 'middle',
      nameRotate: 90,
      nameGap: 55,
      axisLabel: { formatter: (v: number) => (v < minH ? '' : v.toFixed(2)) },
      splitLine: { show: true, lineStyle: { color: '#eef1f4', type: 'dashed' } },
    },
    toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: '#ffffff' } } },
    series: [{ type: 'custom', renderItem, data: [0] }],
  }
}

function sigMark(p: number): string {
  if (p < 0.001) return '***'
  if (p < 0.01) return '**'
  if (p < 0.05) return '*'
  return ''
}

// ---- Module–trait correlation heatmap (cor matrix + p-value stars) ----
export function moduleTraitHeatmapOption(
  modules: string[],
  traits: string[],
  cor: number[][],
  p: number[][],
  lowColor = '#2166ac',
  midColor = '#ffffff',
  highColor = '#b2182b',
  downloadTitle = 'Download PNG',
  saveName = 'module_trait',
): EChartsOption {
  const heat: [number, number, number][] = []
  cor.forEach((row, i) => row.forEach((v, j) => heat.push([j, i, +v.toFixed(4)])))
  return {
    tooltip: {
      formatter: (params: any) => {
        if (params.seriesType === 'bar') return ''
        const xj = params.value[0] as number
        const yi = params.value[1] as number
        const v = params.value[2] as number
        return modules[yi] + ' × ' + traits[xj] + '<br/>cor = ' + v + '<br/>p = ' + (p[yi]?.[xj] ?? 0)
      },
    },
    grid: [
      { left: 125, width: 22, bottom: 100, top: 20 },
      { left: 147, right: 30, bottom: 100, top: 20 },
    ],
    xAxis: [
      { type: 'value', min: 0, max: 1, show: false, gridIndex: 0 },
      { type: 'category', data: traits, gridIndex: 1, axisLabel: { rotate: 45 }, axisLine: { show: false }, axisTick: { show: false } },
    ],
    yAxis: [
      { type: 'category', data: modules, gridIndex: 0, axisLine: { show: false }, axisTick: { show: false } },
      { type: 'category', data: modules, gridIndex: 1, show: false },
    ],
    visualMap: {
      min: -1, max: 1, calculable: true, orient: 'horizontal', left: 'center', bottom: 0,
      inRange: { color: [lowColor, midColor, highColor] },
      seriesIndex: 1,
    },
    toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: saveName, pixelRatio: 2, backgroundColor: '#ffffff' } } },
    series: [
      {
        type: 'bar', xAxisIndex: 0, yAxisIndex: 0,
        data: modules.map((m) => ({ value: 1, itemStyle: { color: moduleColor(m), borderColor: '#333', borderWidth: 0.8 } })),
        barWidth: '100%', barCategoryGap: '0%', silent: true, z: 2,
      },
      {
        type: 'heatmap', xAxisIndex: 1, yAxisIndex: 1, data: heat,
        label: {
          show: true,
          formatter: (params: any) => {
            const xj = params.value[0] as number
            const yi = params.value[1] as number
            const c = cor[yi]?.[xj] ?? 0
            const pv = p[yi]?.[xj] ?? 1
            return c.toFixed(2) + String.fromCharCode(10) + sigMark(pv)
          },
          fontSize: 11, lineHeight: 14,
        },
        itemStyle: { borderColor: '#333', borderWidth: 0.6 },
        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.4)' } },
      },
    ],
  }
}

// ---- GS vs MM scatter (dynamic) ----
export function gsMmScatterOption(mm: number[], gs: number[], genes: string[], moduleName: string, trait: string, title: string, downloadTitle = 'Download PNG'): EChartsOption {
  const xs = mm.map((m) => Math.abs(m))
  const ys = gs.map((g) => Math.abs(g))
  const points = xs.map((x, i) => [x, ys[i]])

  const cor = pearson(xs, ys)
  const p = corPvalueStudent(cor, xs.length)
  const corText = Number.isFinite(cor) ? cor.toFixed(3) : 'NA'
  const pText = Number.isFinite(p) ? (p < 0.001 ? p.toExponential(1) : p.toFixed(4)) : 'NA'

  const { slope, intercept } = linearRegression(xs, ys)
  const xmin = Math.min(...xs)
  const xmax = Math.max(...xs)
  const line = [[xmin, slope * xmin + intercept], [xmax, slope * xmax + intercept]]

  const n = xs.length
  const xMean = xs.reduce((a, b) => a + b, 0) / n
  const sxx = xs.reduce((s, x) => s + (x - xMean) ** 2, 0) || 1
  const s2 = xs.reduce((s, x, i) => s + (ys[i] - (slope * x + intercept)) ** 2, 0) / Math.max(n - 2, 1)
  const tCrit = tCritical(0.05, n - 2)
  const se = (x: number) => Math.sqrt(s2 * (1 / n + (x - xMean) ** 2 / sxx))
  const N = 50
  const ciX = Array.from({ length: N }, (_, i) => xmin + ((xmax - xmin) * i) / (N - 1))
  const ciBand: [number, number][] = []
  for (let i = 0; i < N; i++) {
    const x = ciX[i]
    ciBand.push([x, slope * x + intercept - tCrit * se(x)])
  }
  for (let i = N - 1; i >= 0; i--) {
    const x = ciX[i]
    ciBand.push([x, slope * x + intercept + tCrit * se(x)])
  }

  return {
    title: {
      text: title + '（' + moduleName + ' / ' + trait + '）',
      subtext: 'cor = ' + corText + '　p = ' + pText,
      left: 'center', textStyle: { fontSize: 14 }, subtextStyle: { fontSize: 12, color: '#6b7280' },
    },
    tooltip: {
      trigger: 'item',
      formatter: (params: any) => {
        const i = params.dataIndex as number
        const gene = genes[i] ?? ''
        const mmv = Math.abs(mm[i] ?? 0)
        const gsv = Math.abs(gs[i] ?? 0)
        return gene + '<br/>|MM| = ' + mmv.toFixed(4) + '<br/>|GS| = ' + gsv.toFixed(4)
      },
    },
    xAxis: { type: 'value', name: 'Module Membership (|MM|)', nameLocation: 'middle', nameGap: 32, min: 0 },
    yAxis: { type: 'value', name: 'Gene Significance (|GS|)', nameLocation: 'middle', nameRotate: 90, nameGap: 55 },
    toolbox: { right: 10, top: 10, feature: { saveAsImage: { title: downloadTitle, name: 'gs_mm_scatter', pixelRatio: 2, backgroundColor: '#ffffff' } } },
    series: [
      {
        type: 'scatter', data: points, symbolSize: 8,
        itemStyle: { color: moduleColor(moduleName), opacity: 0.6 },
      },
      {
        type: 'custom', z: 1, silent: true, data: [0],
        renderItem: (params: any, api: any) => {
          const pts = ciBand.map(([x, y]) => api.coord([x, y]))
          return {
            type: 'polygon',
            shape: { points: pts },
            style: { fill: 'rgba(128,128,128,0.28)', stroke: 'rgba(128,128,128,0.5)', lineWidth: 1 },
          }
        },
      },
      { type: 'line', data: line, lineStyle: { color: '#e11d48', width: 2 }, symbol: 'none', silent: true, z: 10 },
    ],
  }
}

// ---- Hub network graph (force layout) ----
export function hubNetworkOption(nodes: any[], edges: any[], nodeScale = 1, edgeWidth = 1, edgeColor = '#9aa5b1'): EChartsOption {
  const data = nodes.map((n) => ({
    id: n.id,
    name: n.id,
    symbolSize: (n.is_hub ? 20 : 9) * nodeScale,
    itemStyle: {
      color: n.color,
      borderColor: n.is_hub ? '#111111' : n.color,
      borderWidth: n.is_hub ? 2 : 0.5,
      opacity: 0.95,
    },
    value: n.kme,
    gs: n.gs,
    module: n.color,
    isHub: !!n.is_hub,
  }))
  const links = edges.map((e) => ({
    source: e.source,
    target: e.target,
    lineStyle: { width: edgeWidth, color: edgeColor, opacity: 0.5 },
  }))
  return {
    tooltip: {
      formatter: (p: any) => {
        const d = p.data as any
        if (!d || p.dataType === 'edge') return ''
        const kme = d.value != null ? d.value.toFixed(3) : '—'
        const gs = d.gs != null ? d.gs.toFixed(3) : '—'
        return `<b>${d.id}</b><br/>module: ${d.module}<br/>|kME|: ${kme}<br/>|GS|: ${gs}${d.isHub ? '<br/>hub' : ''}`
      },
    },
    series: [{
      type: 'graph',
      layout: 'force',
      roam: true,
      data,
      links,
      categories: Array.from(new Set(nodes.map((n: any) => n.color))).map((c) => ({ name: c as string, itemStyle: { color: c as string } })),
      force: { repulsion: 260, gravity: 0.06, edgeLength: 70, layoutAnimation: true },
      label: { show: false },
      emphasis: { focus: 'adjacency', label: { show: true, fontSize: 8, color: '#334' } },
      lineStyle: { color: edgeColor, curveness: 0 },
    }],
  }
}

// ---- Enrichment bubble chart (GO/KEGG dotplot) ----
export function enrichBubbleOption(results: any[]): EChartsOption {
  const ratioFrac = (r: string) => {
    const [k, n] = String(r).split('/').map(Number)
    return n > 0 ? k / n : 0
  }
  const sigColor = (q: number) => {
    const t = q >= 0.1 ? 1 : q / 0.1
    return `rgb(${Math.round(255 * (1 - t))},0,${Math.round(255 * t)})`
  }
  const truncate = (s: string) => (s.length > 40 ? s.slice(0, 40) + '…' : s)
  const points = results
    .map((r) => ({
      value: [ratioFrac(r.gene_ratio), r.description, r.count, r['p.adjust']] as [number, string, number, number],
      termId: r.id,
      description: r.description,
      genes: r.gene_id || [],
      ontology: r.ontology || '',
    }))
    .sort((a, b) => a.value[0] - b.value[0])
  const maxCount = Math.max(1, ...points.map((p) => p.value[2]))
  return {
    tooltip: {
      trigger: 'item',
      formatter: (p: any) => {
        const d = p.data as any
        return `<b>${d.termId}</b><br/>${d.description}<br/>GeneRatio ${d.value[0].toFixed(3)} · Count ${d.value[2]}<br/>p.adjust ${d.value[3].toExponential(2)}`
      },
    },
    grid: { left: 220, right: 48, top: 16, bottom: 48 },
    xAxis: { type: 'value', name: 'GeneRatio', min: 0, nameLocation: 'middle', nameGap: 26, axisLine: { show: true, lineStyle: { color: '#9ca3af' } } },
    yAxis: {
      type: 'category',
      data: points.map((p) => p.description),
      axisLabel: { fontSize: 11, margin: 14, formatter: (v: string) => truncate(v) },
      axisLine: { show: true, lineStyle: { color: '#9ca3af' } },
    },
    series: [{
      type: 'scatter',
      data: points,
      symbolSize: (val: number[]) => 8 + (val[2] / maxCount) * 26,
      itemStyle: { color: (p: any) => sigColor(p.value[3]), opacity: 0.85 },
    }],
  }
}
