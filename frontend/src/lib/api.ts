const TOKEN_KEY = 'provisr_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

// AuthProvider subscribes to this so a 401 on any in-flight request (not
// just the token's own expiry check on mount) immediately drops the app
// back to the login screen instead of leaving queries to fail silently
// until the next full page reload.
const authExpiredListeners = new Set<() => void>()

export function onAuthExpired(listener: () => void): () => void {
  authExpiredListeners.add(listener)
  return () => authExpiredListeners.delete(listener)
}

function notifyAuthExpired(): void {
  authExpiredListeners.forEach((listener) => listener())
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    if (res.status === 401) {
      clearToken()
      notifyAuthExpired()
    }
    const body = await res.text().catch(() => '')
    let message = body || `${res.status} ${res.statusText}`
    try {
      const obj = JSON.parse(body) as { message?: string; error?: string }
      message = obj.message ?? obj.error ?? message
    } catch {
      // body wasn't JSON — keep the raw text
    }
    throw new ApiError(res.status, message)
  }
  const text = await res.text()
  return text ? (JSON.parse(text) as T) : (undefined as T)
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = getToken()
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`/api${path}`, { ...init, headers })
  return handleResponse<T>(res)
}
