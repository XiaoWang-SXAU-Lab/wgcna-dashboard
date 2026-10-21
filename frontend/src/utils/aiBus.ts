let analyzer: ((err: string) => void) | null = null

export function registerAnalyzer(fn: (err: string) => void) {
  analyzer = fn
}

export function analyzeError(err: string) {
  if (analyzer) analyzer(err)
}
