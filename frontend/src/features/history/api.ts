import { apiFetch } from '@/lib/api'
import type { HistoryPage } from './types'

export async function getHistory(name?: string, limit = 20, offset = 0): Promise<HistoryPage> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  if (name) params.set('name', name)
  return apiFetch<HistoryPage>(`/history?${params.toString()}`)
}
