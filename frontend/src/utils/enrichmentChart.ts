import type { EChartsOption } from 'echarts'
import type { EnrichResult } from '../api/client'
import { mixHex } from './colors'
export type Source = 'GO' | 'KEGG'

export interface BubblePoint {
  value: [number, string, number, number]
  termId: string
  description: string
  genes: string[]
  source: Source
}

export function ratioFrac(r: string): number {
  const [k, n] = r.split('/').map(Number)
  return n > 0 ? k / n : 0
}

function sigColor(q: number): string {
  // clusterProfiler dotplot: low p.adjust (significant) = red, high = blue.
  const t = q >= 0.1 ? 1 : q / 0.1
  return mixHex('#ff0000', '#0000ff', t)
}

function truncateLabel(s: string): string {
  return s.length > 30 ? s.slice(0, 30) + '…' : s
}

function makePoints(results: EnrichResult[], source: Source): BubblePoint[] {
  return results
    .map((r): BubblePoint => ({
      value: [ratioFrac(r.gene_ratio), r.description, r.count, r.qvalue],
      termId: r.id,
      description: r.description,
      genes: r.gene_id,
      source,
    }))
    .sort((a, b) => a.value[0] - b.value[0]) // GeneRatio ascending → highest at top (clusterProfiler)
}

function optionFromPoints(points: BubblePoint[], title: string, height: string): EChartsOption {
  const maxCount = Math.max(1, ...points.map((p) => p.value[2]))
  return {
    title: { text: title, left: 'center', textStyle: { fontSize: 14 } },
    tooltip: {
      trigger: 'item',
      formatter: (p: any) => {
        const d = p.data as BubblePoint
        return `<b>${d.termId}</b> (${d.source})<br/>${d.description}<br/>GeneRatio ${d.value[0].toFixed(3)} · Count ${d.value[2]}<br/>p.adjust ${d.value[3].toExponential(2)}`
      },
    },
    grid: { left: 330, right: 48, top: 42, bottom: 48 },
    xAxis: { type: 'value', name: 'GeneRatio', min: 0, nameLocation: 'middle', nameGap: 26, axisLine: { show: true, lineStyle: { color: '#9ca3af' } } },
    yAxis: {
      type: 'category',
      data: points.map((p) => p.description),
      axisLabel: { fontSize: 11, margin: 14, formatter: (v: string) => truncateLabel(v) },
      axisLine: { show: true, lineStyle: { color: '#9ca3af' } },
    },
    series: [
      {
        type: 'scatter',
        data: points,
        symbolSize: (val: number[]) => 8 + (val[2] / maxCount) * 26,
        itemStyle: { color: (p: any) => sigColor(p.value[3]), opacity: 0.85 },
      },
    ],
  }
}


export { makePoints, optionFromPoints }
