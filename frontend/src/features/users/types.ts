export interface User {
  id: string
  username: string
  email?: string
  roles: string[]
  metadata?: Record<string, string>
  created_at: string
  updated_at: string
  active: boolean
}

export interface CreateUserRequest {
  username: string
  password: string
  email?: string
  roles: string[]
}

export interface UpdateUserRequest {
  username?: string
  email?: string
  roles?: string[]
  active?: boolean
}

export interface UserListResponse {
  users: User[]
  total: number
  offset: number
  limit: number
}
