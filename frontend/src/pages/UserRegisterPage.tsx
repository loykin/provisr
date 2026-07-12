import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api'
import { UserFormFields } from '@/features/users/UserForm'
import { initialUserForm, type UserFormState } from '@/features/users/user-form-state'
import { useCreateUser } from '@/features/users/queries'

export default function UserRegisterPage() {
  const navigate = useNavigate()
  const [form, setForm] = useState<UserFormState>(initialUserForm)
  const [error, setError] = useState<string | null>(null)
  const create = useCreateUser()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (!form.username.trim() || !form.password.trim()) {
      setError('Username and password are required.')
      return
    }
    try {
      await create.mutateAsync({
        username: form.username.trim(),
        password: form.password,
        email: form.email.trim() || undefined,
        roles: form.roles,
      })
      await navigate({ to: '/users' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create user.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <DataBodyTemplate
        title="Create user"
        contentClassName="flex-1"
        actions={
          <>
            <Button type="button" variant="ghost" onClick={() => void navigate({ to: '/users' })}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              Create
            </Button>
          </>
        }
      >
        <UserFormFields mode="create" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
      </DataBodyTemplate>
    </form>
  )
}
