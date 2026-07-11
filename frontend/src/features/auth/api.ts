import { apiFetch } from '@/lib/api'
import type { AuthResult, AuthStatus, LoginRequest } from './types'

export async function login(username: string, password: string): Promise<AuthResult> {
  const req: LoginRequest = { method: 'basic', username, password }
  return apiFetch<AuthResult>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

// Only succeeds once — creates the first admin user when the store has
// none yet, and logs them in. The backend refuses this after the first
// admin exists (see AuthService.BootstrapFirstAdmin).
export async function bootstrapAdmin(username: string, password: string): Promise<AuthResult> {
  return apiFetch<AuthResult>('/auth/bootstrap', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export async function getAuthStatus(): Promise<AuthStatus> {
  return apiFetch<AuthStatus>('/auth/status')
}
