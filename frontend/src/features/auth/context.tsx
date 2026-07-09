import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { getAuthStatus, login as apiLogin } from './api'
import { getToken, setToken, clearToken } from '@/lib/api'
import { decodeJwt, isExpired } from '@/lib/jwt'
import type { AuthUser } from './types'

// Used when the server has [server.auth] disabled: every API is already
// wide open in that mode (see internal/server/router.go's noopMiddleware),
// so the UI treats itself as a full-access "local" user rather than
// blocking on a login screen with no /auth/login route to submit to.
const NO_AUTH_USER: AuthUser = { userId: 'local', username: 'local', roles: ['admin'] }

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
  // undefined while the initial /auth/status check is in flight.
  user: AuthUser | null | undefined
  authEnabled: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue>({
  user: undefined,
  authEnabled: true,
  login: async () => {},
  logout: () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authEnabled, setAuthEnabled] = useState(true)
  const [user, setUser] = useState<AuthUser | null | undefined>(undefined)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const status = await getAuthStatus()
        if (cancelled) return
        setAuthEnabled(status.enabled)
        setUser(status.enabled ? userFromToken() : NO_AUTH_USER)
      } catch {
        // Network/parse failure: fail closed (require login) rather than
        // silently granting access if the status check itself is broken.
        if (cancelled) return
        setAuthEnabled(true)
        setUser(userFromToken())
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

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
    setUser(authEnabled ? null : NO_AUTH_USER)
  }

  return (
    <AuthContext.Provider value={{ user, authEnabled, login, logout }}>{children}</AuthContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  return useContext(AuthContext)
}
