// Module-aware, hub-centered network layout.
//
// Each WGCNA module is laid out independently as a hub-centered cluster, then
// the modules are packed into a coherent global coordinate system. Hub genes
// (highest |kME|) are pulled toward the module center; non-hub genes are
// arranged by a short weighted force-directed simulation using the real TOM
// edge weights. Positions are deterministic and computed once, then handed to
// the renderer as fixed coordinates (no per-frame force simulation).

export interface NetNode {
  id: string
  color: string
  degree: number
  kme: number
}

export interface NetEdge {
  source: string
  target: string
  weight: number
}

export interface LayoutResult {
  positions: Map<string, { x: number; y: number }>
  hubs: Set<string>
}

// Deterministic 32-bit hash (FNV-1a) for stable initial positions.
function hash(str: string, salt: number): number {
  let h = 2166136261 ^ salt
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return (h >>> 0) / 4294967295 // 0..1
}

function layoutOneModule(
  list: NetNode[],
  edges: NetEdge[],
  hubs: Set<string>,
  iterations: number,
): Map<string, { x: number; y: number }> {
  const n = list.length
  const idx = new Map<string, number>()
  list.forEach((node, i) => idx.set(node.id, i))

  // adjacency (undirected)
  const adj: Array<Array<{ j: number; w: number }>> = Array.from({ length: n }, () => [])
  for (const e of edges) {
    const i = idx.get(e.source)
    const j = idx.get(e.target)
    if (i == null || j == null) continue
    adj[i].push({ j, w: e.weight })
    adj[j].push({ j: i, w: e.weight })
  }

  // initial positions: hubs near center, others on a ring
  const pos = list.map((node, i) => {
    if (hubs.has(node.id)) {
      return { x: (hash(node.id, 1) - 0.5) * 0.05, y: (hash(node.id, 2) - 0.5) * 0.05 }
    }
    const ang = (i / n) * Math.PI * 2 + hash(node.id, 3) * 0.5
    const r = 1 + hash(node.id, 4) * 0.6
    return { x: Math.cos(ang) * r, y: Math.sin(ang) * r }
  })

  const k = Math.sqrt(4 / n) // optimal pairwise distance (FR)
  const repulse = (d: number) => (k * k) / Math.max(d, 0.02)
  const attract = (d: number) => (d * d) / k

  const disp = pos.map(() => ({ x: 0, y: 0 }))

  for (let it = 0; it < iterations; it++) {
    const temp = Math.max(0.02, 1 - it / iterations) // cooling
    for (let i = 0; i < n; i++) disp[i] = { x: 0, y: 0 }

    // repulsion (O(n^2))
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        const dx = pos[i].x - pos[j].x
        const dy = pos[i].y - pos[j].y
        const d = Math.sqrt(dx * dx + dy * dy) || 0.02
        const f = repulse(d) / d
        disp[i].x += dx * f
        disp[i].y += dy * f
        disp[j].x -= dx * f
        disp[j].y -= dy * f
      }
    }

    // attraction (springs, weighted)
    for (let i = 0; i < n; i++) {
      for (const { j, w } of adj[i]) {
        if (j <= i) continue
        const dx = pos[i].x - pos[j].x
        const dy = pos[i].y - pos[j].y
        const d = Math.sqrt(dx * dx + dy * dy) || 0.02
        const f = attract(d) * (0.3 + 0.7 * w) / d
        disp[i].x -= dx * f
        disp[i].y -= dy * f
        disp[j].x += dx * f
        disp[j].y += dy * f
      }
    }

    // hub-center soft constraint
    for (let i = 0; i < n; i++) {
      if (hubs.has(list[i].id)) {
        disp[i].x -= pos[i].x * 2.5
        disp[i].y -= pos[i].y * 2.5
      }
    }

    // integrate with displacement cap
    for (let i = 0; i < n; i++) {
      const len = Math.sqrt(disp[i].x * disp[i].x + disp[i].y * disp[i].y) || 1
      const cap = temp
      const scale = Math.min(len, cap) / len
      pos[i].x += disp[i].x * scale
      pos[i].y += disp[i].y * scale
    }
  }

  const out = new Map<string, { x: number; y: number }>()
  list.forEach((node, i) => out.set(node.id, pos[i]))
  return out
}

export function moduleAwareLayout(
  nodes: NetNode[],
  edges: NetEdge[],
  iterations = 70,
  hubPerModule = 10,
): LayoutResult {
  const byMod = new Map<string, NetNode[]>()
  for (const n of nodes) {
    if (!byMod.has(n.color)) byMod.set(n.color, [])
    byMod.get(n.color)!.push(n)
  }
  const modNames = [...byMod.keys()].sort()

  // hub genes: top |kME| per module (skip grey / kme<=0)
  const hubs = new Set<string>()
  for (const mod of modNames) {
    if (mod === 'grey') continue
    const list = byMod.get(mod)!.slice().sort((a, b) => b.kme - a.kme)
    const hubCount = Math.max(3, Math.min(hubPerModule, Math.round(Math.sqrt(list.length))))
    for (let i = 0; i < hubCount && i < list.length; i++) {
      if (list[i].kme > 0) hubs.add(list[i].id)
    }
  }

  // per-module local layout
  const local = new Map<string, { x: number; y: number }>()
  const localRadius = new Map<string, number>()
  for (const mod of modNames) {
    const list = byMod.get(mod)!
    const pos = layoutOneModule(list, edges, hubs, iterations)
    for (const [id, p] of pos) local.set(id, p)
    localRadius.set(mod, Math.sqrt(list.length))
  }

  // module packing: place module centers on a circle (deterministic order)
  const centers = new Map<string, { x: number; y: number }>()
  const totalSpread = Math.sqrt(nodes.length) * 1.6
  const nMods = modNames.length
  modNames.forEach((mod, i) => {
    const ang = (i / Math.max(1, nMods)) * Math.PI * 2
    const r = totalSpread * 0.9
    centers.set(mod, { x: Math.cos(ang) * r, y: Math.sin(ang) * r })
  })

  // scale + translate each module's local layout into global coords
  const positions = new Map<string, { x: number; y: number }>()
  for (const mod of modNames) {
    const c = centers.get(mod)!
    const list = byMod.get(mod)!
    const radius = localRadius.get(mod)!
    const scale = Math.max(radius * 0.55, 6)
    for (const node of list) {
      const p = local.get(node.id)!
      positions.set(node.id, { x: c.x + p.x * scale, y: c.y + p.y * scale })
    }
  }

  return { positions, hubs }
}
