import { apiFetch } from '@/lib/api'
import type { LogsSinceResponse, ProcessStatus } from './types'

export async function listProcesses(): Promise<ProcessStatus[]> {
  return apiFetch<ProcessStatus[]>('/status?base=*')
}

export async function getProcess(name: string): Promise<ProcessStatus> {
  return apiFetch<ProcessStatus>(`/status?name=${encodeURIComponent(name)}`)
}

export async function getProcessLogsSince(name: string, since: number, limit = 200): Promise<LogsSinceResponse> {
  return apiFetch<LogsSinceResponse>(
    `/processes/${encodeURIComponent(name)}/logs?since=${since}&limit=${limit}`,
  )
}
