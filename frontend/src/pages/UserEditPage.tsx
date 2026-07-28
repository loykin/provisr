import { useEffect, useState } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageBreadcrumbTopBar } from '@/components/page-breadcrumb-topbar'
import { StackedFormField } from '@/components/stacked-form-field'
import { ApiError } from '@/lib/api'
import { UserFormFields } from '@/features/users/UserForm'
import { userToForm, type UserFormState } from '@/features/users/user-form-state'
import { useUpdateUser, useUser } from '@/features/users/queries'

export default function UserEditPage() {
  const { id } = useParams({ strict: false }) as { id: string }
  const navigate = useNavigate()
  const { data: user, error: loadError, isLoading } = useUser(id)
  const update = useUpdateUser()
  const [form, setForm] = useState<UserFormState | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')

  useEffect(() => {
    if (user) setForm(userToForm(user))
  }, [user])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form) return
    setError(null)
    if (newPassword && newPassword.length < 8) {
      setError('The new password must be at least 8 characters.')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('The new passwords do not match.')
      return
    }
    try {
      await update.mutateAsync({
        id,
        req: { email: form.email.trim() || undefined, roles: form.roles, password: newPassword || undefined },
      })
      await navigate({ to: '/users' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save user.')
    }
  }

  if (isLoading || !form) {
    return <div className="p-8 text-sm text-muted-foreground">{loadError ? 'Failed to load user.' : 'Loading…'}</div>
  }

  return (
    <DataBodyTemplate
      topBar={<PageBreadcrumbTopBar items={['provisr', 'Users', form.username, 'Edit']} />}
      title={`Edit ${form.username}`}
      description="Update account details, access roles, and credentials."
    >
      <form className="contents" onSubmit={(e) => void handleSubmit(e)}>
        <UserFormFields
          mode="edit"
          form={form}
          setForm={(updater) => setForm((current) => (current ? updater(current) : current))}
        />
        <DataBodyTemplate.Group layout="stacked" title="Credentials" description="Optionally replace the current password.">
          <StackedFormField label="New password" description="Leave blank to keep the current password">
            <Input type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} />
          </StackedFormField>
          <StackedFormField label="Confirm password">
            <Input type="password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} />
          </StackedFormField>
        </DataBodyTemplate.Group>
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
        <div className="flex justify-end gap-2 px-(--designkit-page-padding-x) pb-(--designkit-page-padding-y)">
          <Button type="button" variant="outline" onClick={() => void navigate({ to: '/users' })}>
            Cancel
          </Button>
          <Button type="submit" disabled={update.isPending}>Save</Button>
        </div>
      </form>
    </DataBodyTemplate>
  )
}
