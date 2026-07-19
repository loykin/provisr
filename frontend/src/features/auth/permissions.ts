import type { AuthUser } from './types'

export function canWriteWorkloads(user: AuthUser | null | undefined): boolean {
  return user?.roles.some((role) => role === 'admin' || role === 'operator') ?? false
}
