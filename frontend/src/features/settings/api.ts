import { apiFetch } from '@/lib/api'
import type { RuntimeStatus } from './types'

export async function getRuntimeStatus(): Promise<RuntimeStatus> {
  return apiFetch<RuntimeStatus>('/settings/status')
}
