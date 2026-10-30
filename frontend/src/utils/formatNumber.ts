// Missing/undefined statistics must not be presented as a measured zero.
export function formatNumber(value: unknown, digits: number, scientific = false): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '—'
  return scientific ? value.toExponential(digits) : value.toFixed(digits)
}
