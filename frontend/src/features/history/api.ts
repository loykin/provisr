import { apiFetch } from '@/lib/api'
import type { HistoryRecord } from './types'

export async function getHistory(name?: string, limit = 200): Promise<HistoryRecord[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (name) params.set('name', name)
  return apiFetch<HistoryRecord[]>(`/history?${params.toString()}`)
}
