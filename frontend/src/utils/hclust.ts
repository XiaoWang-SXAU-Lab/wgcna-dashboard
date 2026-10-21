// Minimal average-linkage hierarchical clustering (for small matrices such as
// module eigengenes). Output matches R's hclust convention.

export interface Hclust {
  merge: [number, number][]
  height: number[]
  order: number[]
}

export function hclustAverage(dissim: number[][]): Hclust {
  const n = dissim.length
  if (n === 0) return { merge: [], height: [], order: [] }
  if (n === 1) return { merge: [], height: [], order: [0] }

  const idMembers = new Map<number, number[]>()
  for (let i = 0; i < n; i++) idMembers.set(i, [i])
  const active: number[] = Array.from({ length: n }, (_, i) => i)

  const avg = (a: number[], b: number[]): number => {
    let s = 0
    for (const i of a) for (const j of b) s += dissim[i][j]
    return s / (a.length * b.length)
  }

  const merge: [number, number][] = []
  const height: number[] = []

  for (let step = 0; step < n - 1; step++) {
    let best = Infinity
    let bi = 0
    let bj = 0
    for (let a = 0; a < active.length; a++) {
      for (let b = a + 1; b < active.length; b++) {
        const ma = idMembers.get(active[a]) as number[]
        const mb = idMembers.get(active[b]) as number[]
        const d = avg(ma, mb)
        if (d < best) { best = d; bi = a; bj = b }
      }
    }
    const ca = active[bi]
    const cb = active[bj]
    const newId = n + step
    const ma = idMembers.get(ca) as number[]
    const mb = idMembers.get(cb) as number[]
    idMembers.set(newId, ma.concat(mb))
    const enc = (id: number): number => (id < n ? -(id + 1) : id - n + 1)
    merge.push([enc(ca), enc(cb)])
    height.push(best)
    active[bi] = newId
    active.splice(bj, 1)
    idMembers.delete(ca)
    idMembers.delete(cb)
  }

  const clusterOrder: number[][] = new Array(n - 1)
  const childOrder = (x: number): number[] => (x < 0 ? [-x - 1] : clusterOrder[x - 1])
  for (let i = 0; i < n - 1; i++) {
    clusterOrder[i] = childOrder(merge[i][0]).concat(childOrder(merge[i][1]))
  }
  const order = clusterOrder.length ? clusterOrder[clusterOrder.length - 1] : [0]
  return { merge, height, order }
}
