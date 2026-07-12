import type { User } from './types'

export interface UserFormState {
  username: string
  password: string
  email: string
  roles: string[]
}

export const initialUserForm: UserFormState = { username: '', password: '', email: '', roles: [] }

export function userToForm(user: User): UserFormState {
  return {
    username: user.username,
    password: '',
    email: user.email ?? '',
    roles: user.roles,
  }
}
