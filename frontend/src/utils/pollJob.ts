import { getJob, type JobStatus } from '../api/client'

export async function waitForRun(
  id: string, runId: string,
  onStatus: (status: JobStatus) => void,
  onConnection: (lost: boolean) => void = () => {},
  read = getJob,
  pause = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms)),
): Promise<JobStatus> {
  let failures = 0
  for (;;) {
    let status: JobStatus
    try {
      status = await read(id)
      failures = 0
      onConnection(false)
    } catch {
      if (++failures >= 3) onConnection(true)
      await pause(1500)
      continue
    }
    if (status.run_id !== runId) throw new Error('This run was replaced by another analysis. Refresh the job before continuing.')
    onStatus(status)
    if (status.status === 'error') throw new Error(status.error || 'Analysis failed. Check the inputs and parameters before retrying.')
    if (status.status === 'cancelled') return status
    if (status.status === 'done') return status
    await pause(700)
  }
}
