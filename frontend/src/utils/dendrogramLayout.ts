// Compute node positions and line segments for a vertical rectangle dendrogram
// using R's plot.hclust 'hang' convention: leaves hang BELOW their parent merge
// by hang.scaled = hang * (maxH - minH), not aligned at y=0.

export interface DendrogramLayout {
  n: number
  leafX: number[]
  nodeX: number[]
  nodeY: number[]
  hSegments: Array<[number, number, number]>
  vSegments: Array<[number, number, number]>
  minH: number
  maxH: number
  hangScaled: number
}

export function layoutDendrogram(
  merge: [number, number][],
  height: number[],
  order: number[],
  hang = 0.1,
): DendrogramLayout {
  const n = merge.length + 1
  const leafX = new Array(n).fill(0)
  order.forEach((leaf, pos) => { leafX[leaf] = pos })

  const nodeX = new Array(2 * n - 1).fill(0)
  const nodeY = new Array(2 * n - 1).fill(0)
  for (let i = 0; i < n; i++) { nodeX[i] = leafX[i]; nodeY[i] = 0 }

  const minH = height.length ? Math.min(...height) : 0
  const maxH = height.length ? Math.max(...height) : 1
  const hangScaled = hang * (maxH - minH)

  const hSegments: Array<[number, number, number]> = []
  const vSegments: Array<[number, number, number]> = []
  const childIdx = (x: number): number => (x < 0 ? -x - 1 : n + x - 1)

  for (let i = 0; i < merge.length; i++) {
    const a = merge[i][0]
    const b = merge[i][1]
    const ca = childIdx(a)
    const cb = childIdx(b)
    const xa = nodeX[ca]
    const xb = nodeX[cb]
    const h = height[i]
    // leaf child hangs below the parent by hangScaled; internal child at its own height.
    const ya = a < 0 ? h - hangScaled : nodeY[ca]
    const yb = b < 0 ? h - hangScaled : nodeY[cb]
    const k = n + i
    nodeX[k] = (xa + xb) / 2
    nodeY[k] = h
    hSegments.push([xa, xb, h])
    vSegments.push([xa, ya, h], [xb, yb, h])
  }

  return { n, leafX, nodeX, nodeY, hSegments, vSegments, minH, maxH, hangScaled }
}
