import { apiFetch } from '@/lib/api'
import type { CronJobHistoryEntry, CronJobInfo, CronJobSpec } from './types'

export async function listCronJobs(): Promise<CronJobInfo[]> {
  return apiFetch<CronJobInfo[]>('/cronjobs')
}

export async function getCronJob(name: string): Promise<CronJobInfo> {
  return apiFetch<CronJobInfo>(`/cronjobs/${encodeURIComponent(name)}`)
}

export async function getCronJobHistory(name: string): Promise<CronJobHistoryEntry[]> {
  return apiFetch<CronJobHistoryEntry[]>(`/cronjobs/${encodeURIComponent(name)}/history`)
}

export async function createCronJob(spec: CronJobSpec): Promise<void> {
  await apiFetch<void>('/cronjobs', { method: 'POST', body: JSON.stringify(spec) })
}

export async function updateCronJob(spec: CronJobSpec): Promise<void> {
  await apiFetch<void>(`/cronjobs/${encodeURIComponent(spec.name)}`, {
    method: 'POST',
    body: JSON.stringify(spec),
  })
}

export async function deleteCronJob(name: string): Promise<void> {
  await apiFetch<void>(`/cronjobs/${encodeURIComponent(name)}`, { method: 'DELETE' })
}

export async function suspendCronJob(name: string): Promise<void> {
  await apiFetch<void>(`/cronjobs/${encodeURIComponent(name)}/suspend`, { method: 'POST' })
}

export async function resumeCronJob(name: string): Promise<void> {
  await apiFetch<void>(`/cronjobs/${encodeURIComponent(name)}/resume`, { method: 'POST' })
}

export async function triggerCronJob(name: string): Promise<void> {
  await apiFetch<void>(`/cronjobs/${encodeURIComponent(name)}/trigger`, { method: 'POST' })
}
