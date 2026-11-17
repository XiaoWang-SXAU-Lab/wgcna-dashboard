export interface AIErrorContext {
  regutraceJobId?: string
  kind?: 'id_mismatch' | 'job_failure' | 'diagnostic'
}

let analyzer: ((err: string, context?: AIErrorContext) => void) | null = null

export function registerAnalyzer(fn: (err: string, context?: AIErrorContext) => void) {
  analyzer = fn
}

export function analyzeError(err: string, context?: AIErrorContext) {
  if (analyzer) analyzer(err, context)
}
