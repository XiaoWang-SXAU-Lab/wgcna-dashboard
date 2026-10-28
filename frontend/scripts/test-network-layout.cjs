// Run with: node scripts/test-network-layout.cjs [saved-network.json]
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const ts = require('typescript')
const context = { exports: {} }
vm.createContext(context)
vm.runInContext(ts.transpileModule(
  fs.readFileSync(path.join(__dirname, '../src/utils/networkLayout.ts'), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } },
).outputText, context)
const { moduleAwareLayout } = context.exports

// Sparse pink module: dense core, several isolates, and two bridge-only nodes.
const nodes = Array.from({ length: 59 }, (_, i) => ({ id: `p${i}`, color: 'pink', degree: 1, kme: 1 - i / 100 }))
nodes.push({ id: 'b0', color: 'blue', degree: 2, kme: 1 })
const edges = []
for (let i = 1; i < 46; i++) edges.push({ source: 'p0', target: `p${i}`, weight: 0.5 })
edges.push({ source: 'p57', target: 'b0', weight: 0.3 }, { source: 'p58', target: 'b0', weight: 0.2 })

function verify(network) {
  const before = JSON.stringify(network)
  const a = moduleAwareLayout(network.nodes, network.edges)
  const b = moduleAwareLayout(network.nodes, network.edges)
  assert.equal(a.positions.size, network.nodes.length)
  assert.deepEqual([...a.positions], [...b.positions], 'layout must be deterministic')
  for (const p of a.positions.values()) assert(Number.isFinite(p.x) && Number.isFinite(p.y))
  for (const color of new Set(network.nodes.map((n) => n.color))) {
    const group = network.nodes.filter((n) => n.color === color)
    // A module must stay inside its membership boundary, including bridge-only
    // nodes. The previous repulsion-only layout fails this for sparse modules.
    const maxDiameter = Math.max(2, Math.sqrt(group.length)) * 4
    for (const n of group) for (const m of group) {
      const p = a.positions.get(n.id), q = a.positions.get(m.id)
      assert(Math.hypot(p.x - q.x, p.y - q.y) <= maxDiameter + 1e-8, `${color}: runaway node ${n.id}`)
    }
  }
  assert.equal(JSON.stringify(network), before, 'layout must not modify nodes or biological edges')
}
verify({ nodes, edges })
verify({ nodes: [], edges: [] })
verify({ nodes: [nodes[0]], edges: [] })
if (process.argv[2]) verify(JSON.parse(fs.readFileSync(process.argv[2], 'utf8')))
console.log('PASS: bounded sparse/bridge-only nodes, deterministic finite positions, unchanged network')
