// Builds the standalone report-charts.js bundle (IIFE exposing `reportCharts`).
// The App and the HTML report share the same pure option builders in
// src/utils/reportCharts.ts, so the report reproduces the App's figures exactly.
import { buildSync } from 'esbuild'
import { dirname, resolve } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const frontend = resolve(__dirname, '..')
const outfile = resolve(frontend, '../backend/internal/api/report_assets/report-charts.js')

buildSync({
  entryPoints: [resolve(frontend, 'src/utils/reportCharts.ts')],
  bundle: true,
  format: 'iife',
  globalName: 'reportCharts',
  outfile,
  minify: false,
  target: 'es2018',
})

console.log('built report-charts.js ->', outfile)
