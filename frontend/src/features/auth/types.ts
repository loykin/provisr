export interface AuthToken {
  type: string
  value: string
  expires_at: string
}

export interface AuthResult {
  success: boolean
  user_id?: string
  username?: string
  roles?: string[]
  token?: AuthToken
}

export interface LoginRequest {
  method: 'basic'
  username: string
  password: string
}

export interface AuthUser {
  userId: string
  username: string
  roles: string[]
}

export interface AuthStatus {
  enabled: boolean
  needs_setup: boolean
}
