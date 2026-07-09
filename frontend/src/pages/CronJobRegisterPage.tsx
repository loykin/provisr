import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api'
import {
  CronJobFormFields,
  formToSpec,
  type CronJobFormState,
} from '@/features/cronjobs/CronJobFormPanel'
import { useCreateCronJob } from '@/features/cronjobs/queries'

const initialForm: CronJobFormState = {
  name: '',
  schedule: '',
  command: '',
  workDir: '',
  env: '',
  concurrencyPolicy: 'Allow',
}

export default function CronJobRegisterPage() {
  const navigate = useNavigate()
  const [form, setForm] = useState(initialForm)
  const [error, setError] = useState<string | null>(null)
  const create = useCreateCronJob()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (!form.name.trim() || !form.schedule.trim() || !form.command.trim()) {
      setError('Name, schedule, and command are required.')
      return
    }
    try {
      await create.mutateAsync(formToSpec(form))
      await navigate({ to: '/jobs' })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to create cronjob.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <DataBodyTemplate
        title="Create cron job"
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
        <CronJobFormFields mode="create" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
      </DataBodyTemplate>
    </form>
  )
}
