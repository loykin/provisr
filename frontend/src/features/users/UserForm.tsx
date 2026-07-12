import { DataBodyTemplate } from '@loykin/designkit'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import type { UserFormState } from './user-form-state'

const ROLES = ['admin', 'operator', 'viewer'] as const

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
