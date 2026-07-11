import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api'
import { JobFormFields, formToSpec, type JobFormState } from '@/features/jobs/JobFormPanel'
import { validateLifecycleHooks } from '@/components/lifecycle-hook-editor'
import { useCreateJob } from '@/features/jobs/queries'

const initialForm: JobFormState = {
  name: '',
  command: '',
  workDir: '',
  env: '',
  parallelism: '',
  completions: '',
  restartPolicy: 'Never',
  backoffLimit: '',
  activeDeadlineSeconds: '',
  lifecycle: {},
}

export default function JobRegisterPage() {
  const navigate = useNavigate()
  const [form, setForm] = useState(initialForm)
  const [error, setError] = useState<string | null>(null)
  const create = useCreateJob()

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
      await create.mutateAsync(formToSpec(form))
      await navigate({ to: '/jobs' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create job.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <DataBodyTemplate
        title="Create Job"
        contentClassName="flex-1"
        actions={
          <>
            <Button type="button" variant="ghost" onClick={() => void navigate({ to: '/jobs' })}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              Create
            </Button>
          </>
        }
      >
        <JobFormFields mode="create" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
      </DataBodyTemplate>
    </form>
  )
}
