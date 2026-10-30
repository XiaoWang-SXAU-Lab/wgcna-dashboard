import { build } from 'esbuild'
const result = await build({ entryPoints: ['scripts/test-pipeline-guards.ts'], bundle: true, write: false, platform: 'node', format: 'esm', define: { 'process.env.NODE_ENV': '"test"' } })
await import(`data:text/javascript;base64,${Buffer.from(result.outputFiles[0].text).toString('base64')}`)
