import { apiFetch } from '@/lib/api'
import type { JobInfo, JobSpec } from './types'

export async function listJobs(): Promise<JobInfo[]> {
  return apiFetch<JobInfo[]>('/jobs')
}

export async function getJob(name: string): Promise<JobInfo> {
  return apiFetch<JobInfo>(`/jobs/${encodeURIComponent(name)}`)
}

export async function createJob(spec: JobSpec): Promise<void> {
  await apiFetch<void>('/jobs', { method: 'POST', body: JSON.stringify(spec) })
}

export async function updateJob(spec: JobSpec): Promise<void> {
  await apiFetch<void>(`/jobs/${encodeURIComponent(spec.name)}`, {
    method: 'POST',
    body: JSON.stringify(spec),
  })
}

export async function deleteJob(name: string): Promise<void> {
  await apiFetch<void>(`/jobs/${encodeURIComponent(name)}`, { method: 'DELETE' })
}
