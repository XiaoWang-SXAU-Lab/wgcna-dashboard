// Convert an hclust dendrogram (merge / height / order) into a nested tree
// suitable for ECharts `tree` series, with module-colored leaves.
import type { Dendrogram } from '../api/client'
import { moduleColor } from './colors'

export interface TreeNode {
  name: string
  value?: number
  itemStyle?: { color?: string; borderColor?: string }
  children?: TreeNode[]
}

/**
 * Build the dendrogram tree iteratively. `merge` uses R's hclust convention:
 * negative = leaf -(i+1), positive = cluster (1-based merge step).
 *
 * hclust guarantees that merge[i] only references earlier clusters (verified),
 * so building nodes in order 0..n-2 needs no recursion — this stays robust for
 * deeply-unbalanced ("comb") dendrograms with tens of thousands of genes.
 */
export function buildDendrogramTree(
  dendro: Dendrogram,
  names: string[],
  colors: string[],
): TreeNode {
  const { merge, height } = dendro
  const n = merge.length
  const nodes: TreeNode[] = new Array(n)

  const leaf = (i: number): TreeNode => ({
    name: names[i] ?? `g${i}`,
    itemStyle: { color: moduleColor(colors[i] ?? 'grey') },
  })

  const child = (x: number): TreeNode => (x < 0 ? leaf(-x - 1) : nodes[x - 1])

  for (let i = 0; i < n; i++) {
    const [a, b] = merge[i]
    nodes[i] = {
      name: '',
      value: height[i],
      itemStyle: { borderColor: '#888' },
      children: [child(a), child(b)],
    }
  }
  return nodes[n - 1] // root = last merge
}
