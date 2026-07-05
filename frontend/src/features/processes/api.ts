import { apiFetch } from '@/lib/api'
import type { ProcessStatus } from './types'

export async function listProcesses(): Promise<ProcessStatus[]> {
  return apiFetch<ProcessStatus[]>('/status?base=*')
}

export async function getProcess(name: string): Promise<ProcessStatus> {
  return apiFetch<ProcessStatus>(`/status?name=${encodeURIComponent(name)}`)
}
