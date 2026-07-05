export interface JwtClaims {
  user_id: string
  username: string
  roles: string[]
  exp?: number
}

// Decodes the payload only — the server verifies the signature on every
// request, this is purely for reading claims to render the UI.
export function decodeJwt(token: string): JwtClaims | null {
  const parts = token.split('.')
  if (parts.length !== 3) return null
  try {
    const payload = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const json = decodeURIComponent(
      atob(payload)
        .split('')
        .map((c) => '%' + c.charCodeAt(0).toString(16).padStart(2, '0'))
        .join(''),
    )
    return JSON.parse(json) as JwtClaims
  } catch {
    return null
  }
}

export function isExpired(claims: JwtClaims): boolean {
  return typeof claims.exp === 'number' && claims.exp * 1000 < Date.now()
}
