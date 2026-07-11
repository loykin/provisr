import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api'
import { ProcessFormFields, formToSpec, type ProcessFormState } from '@/features/processes/ProcessFormPanel'
import { validateLifecycleHooks } from '@/components/lifecycle-hook-editor'
import { useRegisterProcess } from '@/features/processes/queries'

const initialForm: ProcessFormState = {
  name: '',
  command: '',
  workDir: '',
  env: '',
  autoRestart: false,
  instances: '',
  lifecycle: {},
}

export default function ProcessRegisterPage() {
  const navigate = useNavigate()
  const [form, setForm] = useState(initialForm)
  const [error, setError] = useState<string | null>(null)
  const register = useRegisterProcess()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (!form.name.trim() || !form.command.trim()) {
      setError('Name and command are required.')
      return
    }
    const lifecycleError = validateLifecycleHooks(form.lifecycle)
    if (lifecycleError) {
      setError(lifecycleError)
      return
    }
    try {
      await register.mutateAsync(formToSpec(form))
      await navigate({ to: '/processes' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to register process.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <DataBodyTemplate
        title="Register process"
        contentClassName="flex-1"
        actions={
          <>
            <Button type="button" variant="ghost" onClick={() => void navigate({ to: '/processes' })}>
              Cancel
            </Button>
            <Button type="submit" disabled={register.isPending}>
              Register
            </Button>
          </>
        }
      >
        <ProcessFormFields mode="create" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
      </DataBodyTemplate>
    </form>
  )
}
