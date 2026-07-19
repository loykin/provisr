import { apiFetch } from '@/lib/api'
import type { GroupInfo, GroupStatus } from './types'

export async function listGroups(): Promise<GroupInfo[]> {
  return apiFetch<GroupInfo[]>('/groups')
}

export async function getGroupStatus(name: string): Promise<GroupStatus> {
  return apiFetch<GroupStatus>(`/group/status?group=${encodeURIComponent(name)}`)
}

export async function startGroup(name: string): Promise<void> {
  await apiFetch<void>(`/group/start?group=${encodeURIComponent(name)}`, { method: 'POST' })
}

export async function stopGroup(name: string): Promise<void> {
  await apiFetch<void>(`/group/stop?group=${encodeURIComponent(name)}`, { method: 'POST' })
}
