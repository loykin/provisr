import { apiFetch } from '@/lib/api'
import type { AuthResult, LoginRequest } from './types'

export async function login(username: string, password: string): Promise<AuthResult> {
  const req: LoginRequest = { method: 'basic', username, password }
  return apiFetch<AuthResult>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}
