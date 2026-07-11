import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { bootstrapAdmin, getAuthStatus, login as apiLogin } from './api'
import { getToken, setToken, clearToken, onAuthExpired } from '@/lib/api'
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
  // true when auth is enabled and the store has no users yet — the login
  // page should show a "create the first admin account" form instead of
  // a login form (see AuthService.BootstrapFirstAdmin).
  needsSetup: boolean
  login: (username: string, password: string) => Promise<void>
  bootstrap: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue>({
  user: undefined,
  authEnabled: true,
  needsSetup: false,
  login: async () => {},
  bootstrap: async () => {},
  logout: () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authEnabled, setAuthEnabled] = useState(true)
  const [needsSetup, setNeedsSetup] = useState(false)
  const [user, setUser] = useState<AuthUser | null | undefined>(undefined)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const status = await getAuthStatus()
        if (cancelled) return
        setAuthEnabled(status.enabled)
        setNeedsSetup(status.enabled && status.needs_setup)
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

  // A 401 from any request (not just the token-expiry check on mount) means
  // the session is no longer valid server-side — drop back to the login
  // screen right away instead of leaving queries to fail silently.
  useEffect(() => {
    return onAuthExpired(() => {
      setUser(authEnabled ? null : NO_AUTH_USER)
    })
  }, [authEnabled])

  async function login(username: string, password: string) {
    const result = await apiLogin(username, password)
    if (!result.success || !result.token) {
      throw new Error('Login failed')
    }
    setToken(result.token.value)
    setUser(userFromToken())
  }

  async function bootstrap(username: string, password: string) {
    const result = await bootstrapAdmin(username, password)
    if (!result.success || !result.token) {
      throw new Error('Setup failed')
    }
    setToken(result.token.value)
    setUser(userFromToken())
    setNeedsSetup(false)
  }

  function logout() {
    clearToken()
    setUser(authEnabled ? null : NO_AUTH_USER)
  }

  return (
    <AuthContext.Provider value={{ user, authEnabled, needsSetup, login, bootstrap, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  return useContext(AuthContext)
}
