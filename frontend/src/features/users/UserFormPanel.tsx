import { useState } from 'react'
import { X } from 'lucide-react'
import { DataBodyTemplate, PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { ApiError } from '@/lib/api'
import { useUpdateUser } from './queries'
import type { User } from './types'

const ROLES = ['admin', 'operator', 'viewer'] as const

export interface UserFormState {
  username: string
  password: string
  email: string
  roles: string[]
}

export const initialUserForm: UserFormState = { username: '', password: '', email: '', roles: [] }

export function UserFormFields({
  mode,
  form,
  setForm,
}: {
  mode: 'create' | 'edit'
  form: UserFormState
  setForm: (updater: (f: UserFormState) => UserFormState) => void
}) {
  return (
    <DataBodyTemplate.Group layout="stacked">
      <DataBodyTemplate.Row label="Username" required>
        <Input
          value={form.username}
          disabled={mode === 'edit'}
          onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))}
          required
        />
      </DataBodyTemplate.Row>
      {mode === 'create' && (
        <DataBodyTemplate.Row label="Password" required>
          <Input
            type="password"
            value={form.password}
            onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
            required
          />
        </DataBodyTemplate.Row>
      )}
      <DataBodyTemplate.Row label="Email">
        <Input
          type="email"
          placeholder="(optional)"
          value={form.email}
          onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Roles">
        <div className="flex flex-col gap-2">
          {ROLES.map((role) => (
            <label key={role} className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={form.roles.includes(role)}
                onCheckedChange={(checked) =>
                  setForm((f) => ({
                    ...f,
                    roles:
                      checked === true ? [...f.roles, role] : f.roles.filter((r) => r !== role),
                  }))
                }
              />
              {role}
            </label>
          ))}
        </div>
      </DataBodyTemplate.Row>
    </DataBodyTemplate.Group>
  )
}

function UserEditForm({ user }: { user: User }) {
  const { close } = useSidePanel()
  const [form, setForm] = useState<UserFormState>({
    username: user.username,
    password: '',
    email: user.email ?? '',
    roles: user.roles,
  })
  const [error, setError] = useState<string | null>(null)
  const update = useUpdateUser()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await update.mutateAsync({
        id: user.id,
        req: { email: form.email.trim() || undefined, roles: form.roles },
      })
      await close()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save user.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <div className="flex-1 overflow-y-auto">
        <UserFormFields mode="edit" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
      </div>
      <div className="flex justify-end gap-2 border-t border-border p-4">
        <Button type="button" variant="ghost" onClick={() => void close()}>
          Cancel
        </Button>
        <Button type="submit" disabled={update.isPending}>
          Save
        </Button>
      </div>
    </form>
  )
}

export function UserEditPanel({ user }: { user: User }) {
  const { close } = useSidePanel()
  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )
  return (
    <PanelTemplate eyebrow="User" title={`Edit ${user.username}`} actions={closeBtn}>
      <UserEditForm user={user} />
    </PanelTemplate>
  )
}
