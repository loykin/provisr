import { useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Button, Input, Label, LoginBodyTemplate } from '@loykin/designkit'
import { useAuth } from '@/features/auth/context'

function SignInForm() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login(username, password)
      void navigate({ to: '/processes' })
    } catch {
      setError('Invalid username or password.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <div className="space-y-1 text-center">
        <h1 className="text-lg font-semibold">provisr</h1>
        <p className="text-sm text-muted-foreground">Sign in to your account</p>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="username">Username</Label>
        <Input
          id="username"
          autoComplete="username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          required
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Button type="submit" className="w-full" disabled={submitting}>
        {submitting ? 'Signing in…' : 'Sign in'}
      </Button>
    </form>
  )
}

// Shown instead of the login form when the auth store has no users yet
// (AuthContext.needsSetup) — creates the first admin account and logs
// straight in, Gitea/Nextcloud-style, so there's no bootstrap credential to
// generate, print, or lose.
function SetupForm() {
  const { bootstrap } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    if (password.length < 8) {
      setError('Password must be at least 8 characters.')
      return
    }
    if (password !== confirmPassword) {
      setError('Passwords do not match.')
      return
    }
    setSubmitting(true)
    try {
      await bootstrap(username, password)
      void navigate({ to: '/processes' })
    } catch {
      setError('Failed to create the admin account.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <div className="space-y-1 text-center">
        <h1 className="text-lg font-semibold">provisr</h1>
        <p className="text-sm text-muted-foreground">Create the first admin account to get started</p>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="setup-username">Username</Label>
        <Input
          id="setup-username"
          autoComplete="username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          required
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="setup-password">Password</Label>
        <Input
          id="setup-password"
          type="password"
          autoComplete="new-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="setup-confirm-password">Confirm password</Label>
        <Input
          id="setup-confirm-password"
          type="password"
          autoComplete="new-password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          required
        />
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Button type="submit" className="w-full" disabled={submitting}>
        {submitting ? 'Creating account…' : 'Create admin account'}
      </Button>
    </form>
  )
}

export default function LoginPage() {
  const { needsSetup } = useAuth()

  return (
    <LoginBodyTemplate layout="centered" card="card" cardWidth="sm">
      {needsSetup ? <SetupForm /> : <SignInForm />}
    </LoginBodyTemplate>
  )
}
