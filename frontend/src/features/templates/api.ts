import { apiFetch } from '@/lib/api'
import type { ProcessSpec } from '@/features/processes/types'

export async function listTemplateTypes(): Promise<string[]> {
  return apiFetch<string[]>('/templates')
}

export async function previewTemplate(kind: string, name: string): Promise<ProcessSpec> {
  const query = name.trim() ? `?name=${encodeURIComponent(name.trim())}` : ''
  return apiFetch<ProcessSpec>(`/templates/${encodeURIComponent(kind)}${query}`)
}
