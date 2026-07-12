import { apiFetch } from '@/lib/api'
import type { CreateUserRequest, UpdateUserRequest, User, UserListResponse } from './types'

export async function listUsers(): Promise<UserListResponse> {
  return apiFetch<UserListResponse>('/auth/users?limit=100')
}

export async function getUser(id: string): Promise<User> {
  return apiFetch<User>(`/auth/users/${encodeURIComponent(id)}`)
}

export async function createUser(req: CreateUserRequest): Promise<User> {
  return apiFetch<User>('/auth/users', { method: 'POST', body: JSON.stringify(req) })
}

export async function updateUser(id: string, req: UpdateUserRequest): Promise<User> {
  return apiFetch<User>(`/auth/users/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  })
}

export async function deleteUser(id: string): Promise<void> {
  await apiFetch<void>(`/auth/users/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
