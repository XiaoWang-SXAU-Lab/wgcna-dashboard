import { computed, ref, watch } from 'vue'
import { useProjectStore } from '../stores/project'
import { getCandidateReport } from '../api/projects'
import type { CandidateReportEnvelope } from '../api/client'

// Production report-state logic for the candidate-priority evidence report.
// The report keeps its OWN provenance (project_id / execution_id / upstream /
// params / inputs), and is only "current evidence" while that identity matches the
// live project/execution and the store's unified selector agrees.
export function useCandidateReport() {
  const store = useProjectStore()
  const report = ref<CandidateReportEnvelope | null>(null)
  const loading = ref(false)
  const loadError = ref('')
  let reportReq = 0

  const reportCurrent = computed(() => {
    const r = report.value
    if (!r) return false
    if (r.project_id !== store.id) return false
    if (r.execution_id !== store.current?.state.steps.hub_priority?.execution_id) return false
    if (!store.currentResult('hub_priority')) return false
    return true
  })

  async function load() {
    if (!store.id) return
    const pid = store.id
    const execId = store.current?.state.steps.hub_priority?.execution_id
    const req = ++reportReq
    loading.value = true
    loadError.value = ''
    try {
      const data = await getCandidateReport(pid)
      // Late-response guard: only the newest request (same project + same
      // execution) may land; a superseded or cross-project response is dropped.
      if (req !== reportReq || pid !== store.id || store.current?.state.steps.hub_priority?.execution_id !== execId) return
      report.value = data
    } catch (e) {
      if (req !== reportReq) return // superseded; do not clear a newer report
      loadError.value = String(e)
      report.value = null
    } finally {
      if (req === reportReq) loading.value = false
    }
  }

  // Switching projects must not keep the previous project's report or leave the
  // new project stuck in a stale loading state (a hanging old request's finally
  // would otherwise never clear it, since its request id no longer matches).
  watch(() => store.id, () => {
    report.value = null
    loadError.value = ''
    loading.value = false
    reportReq++
  })

  return { report, loading, loadError, reportCurrent, load }
}
