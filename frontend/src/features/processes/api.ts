import { apiFetch } from '@/lib/api'
import type { DebugProcessInfo, LogsSinceResponse, ProcessMetrics, ProcessMetricsHistory, ProcessSpec, ProcessStatus } from './types'

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

export async function startProcess(name: string): Promise<void> {
  await apiFetch<void>(`/start?name=${encodeURIComponent(name)}`, { method: 'POST' })
}

export async function stopProcess(name: string): Promise<void> {
  await apiFetch<void>(`/stop?name=${encodeURIComponent(name)}`, { method: 'POST' })
}

export async function getProcessSpec(name: string): Promise<ProcessSpec> {
  return apiFetch<ProcessSpec>(`/processes/${encodeURIComponent(name)}/spec`)
}

export async function registerProcess(spec: ProcessSpec): Promise<void> {
  await apiFetch<void>('/register', { method: 'POST', body: JSON.stringify(spec) })
}

export async function updateProcess(spec: ProcessSpec): Promise<void> {
  await apiFetch<void>('/update', { method: 'POST', body: JSON.stringify(spec) })
}

export async function unregisterProcess(name: string): Promise<void> {
  await apiFetch<void>(`/unregister?name=${encodeURIComponent(name)}`, { method: 'POST' })
}

export async function runProcessBaseAction(base: string, action: 'start' | 'stop' | 'unregister'): Promise<void> {
  await apiFetch<void>(`/${action}?base=${encodeURIComponent(base)}`, { method: 'POST' })
}

export async function getProcessMetrics(name: string): Promise<ProcessMetrics> {
  return apiFetch<ProcessMetrics>(`/metrics?name=${encodeURIComponent(name)}`)
}

export async function getProcessMetricsHistory(name: string): Promise<ProcessMetricsHistory> {
  return apiFetch<ProcessMetricsHistory>(`/metrics/history?name=${encodeURIComponent(name)}`)
}

export async function getProcessDiagnostics(name: string): Promise<DebugProcessInfo | undefined> {
  const rows = await apiFetch<DebugProcessInfo[]>(`/debug/processes?pattern=${encodeURIComponent(name)}`)
  return rows.find((row) => row.status.name === name)
}
