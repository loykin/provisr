import { apiFetch } from '@/lib/api'
import type { AuthResult, AuthStatus, LoginRequest } from './types'

export async function login(username: string, password: string): Promise<AuthResult> {
  const req: LoginRequest = { method: 'basic', username, password }
  return apiFetch<AuthResult>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

export async function getAuthStatus(): Promise<AuthStatus> {
  return apiFetch<AuthStatus>('/auth/status')
}
