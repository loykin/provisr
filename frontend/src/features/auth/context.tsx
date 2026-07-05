import { createContext, useContext, useState, type ReactNode } from 'react'
import { login as apiLogin } from './api'
import { getToken, setToken, clearToken } from '@/lib/api'
import { decodeJwt, isExpired } from '@/lib/jwt'
import type { AuthUser } from './types'

function userFromToken(): AuthUser | null {
  const token = getToken()
  if (!token) return null
  const claims = decodeJwt(token)
  if (!claims || isExpired(claims)) {
    clearToken()
    return null
  }
  return { userId: claims.user_id, username: claims.username, roles: claims.roles ?? [] }
}

interface AuthContextValue {
  user: AuthUser | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  login: async () => {},
  logout: () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(() => userFromToken())

  async function login(username: string, password: string) {
    const result = await apiLogin(username, password)
    if (!result.success || !result.token) {
      throw new Error('Login failed')
    }
    setToken(result.token.value)
    setUser(userFromToken())
  }

  function logout() {
    clearToken()
    setUser(null)
  }

  return <AuthContext.Provider value={{ user, login, logout }}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  return useContext(AuthContext)
}
