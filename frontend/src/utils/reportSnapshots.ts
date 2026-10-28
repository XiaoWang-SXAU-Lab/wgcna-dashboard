import { toPng } from 'html-to-image'

export const liveFigures = new Set<() => Promise<void>>()
export async function flushReportFigures() {
  await Promise.all([...liveFigures].map(capture => capture()))
}

export async function saveFigure(job: string, key: string, element: HTMLElement, trait = '', fingerprint = '') {
  const png = await toPng(element, { backgroundColor: '#ffffff', pixelRatio: 2, cacheBust: false,
    filter: node => !(node instanceof HTMLElement && node.hasAttribute('data-snapshot-ignore')) })
  const response = await fetch(`/api/jobs/${encodeURIComponent(job)}/report/snapshot`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, trait, fingerprint, png, width: element.clientWidth, height: element.clientHeight }),
  })
  if (!response.ok) throw new Error(await response.text())
}
