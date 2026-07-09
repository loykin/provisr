import { useState } from 'react'
import { X } from 'lucide-react'
import { PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ApiError } from '@/lib/api'
import { useCronJob, useCreateCronJob, useUpdateCronJob } from './queries'
import type { CronJobSpec } from './types'

interface FormState {
  name: string
  schedule: string
  command: string
  workDir: string
  env: string
  concurrencyPolicy: 'Allow' | 'Forbid' | 'Replace'
}

function specToForm(spec: CronJobSpec): FormState {
  return {
    name: spec.name,
    schedule: spec.schedule,
    command: spec.job_template.command,
    workDir: spec.job_template.work_dir ?? '',
    env: (spec.job_template.env ?? []).join('\n'),
    concurrencyPolicy: (spec.concurrency_policy || 'Allow') as FormState['concurrencyPolicy'],
  }
}

function formToSpec(form: FormState): CronJobSpec {
  const env = form.env
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
  return {
    name: form.name.trim(),
    schedule: form.schedule.trim(),
    job_template: {
      command: form.command,
      work_dir: form.workDir.trim() || undefined,
      env: env.length > 0 ? env : undefined,
    },
    concurrency_policy: form.concurrencyPolicy,
  }
}

// Update stops the old schedule and starts the new one immediately (see
// POST /cronjobs/:name on the backend) — consistent with how process spec
// edits also apply immediately rather than queuing for a later restart.
function CronJobForm({ mode, initial }: { mode: 'create' | 'edit'; initial: FormState }) {
  const { close } = useSidePanel()
  const [form, setForm] = useState(initial)
  const [error, setError] = useState<string | null>(null)
  const create = useCreateCronJob()
  const update = useUpdateCronJob()
  const pending = create.isPending || update.isPending

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (!form.name.trim() || !form.schedule.trim() || !form.command.trim()) {
      setError('Name, schedule, and command are required.')
      return
    }
    try {
      const spec = formToSpec(form)
      if (mode === 'create') {
        await create.mutateAsync(spec)
      } else {
        await update.mutateAsync(spec)
      }
      await close()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save cronjob.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <div className="flex-1 space-y-4 overflow-y-auto">
        <div className="space-y-1.5">
          <Label htmlFor="cron-name">Name</Label>
          <Input
            id="cron-name"
            value={form.name}
            disabled={mode === 'edit'}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="cron-schedule">Schedule</Label>
          <Input
            id="cron-schedule"
            placeholder="e.g. 0 */6 * * * or @every 1h"
            value={form.schedule}
            onChange={(e) => setForm((f) => ({ ...f, schedule: e.target.value }))}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="cron-command">Command</Label>
          <Input
            id="cron-command"
            placeholder="e.g. /usr/bin/backup.sh"
            value={form.command}
            onChange={(e) => setForm((f) => ({ ...f, command: e.target.value }))}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="cron-workdir">Working directory</Label>
          <Input
            id="cron-workdir"
            placeholder="(optional) absolute path"
            value={form.workDir}
            onChange={(e) => setForm((f) => ({ ...f, workDir: e.target.value }))}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="cron-env">Environment (one KEY=VALUE per line)</Label>
          <Textarea
            id="cron-env"
            rows={4}
            className="font-mono text-xs"
            value={form.env}
            onChange={(e) => setForm((f) => ({ ...f, env: e.target.value }))}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="cron-concurrency">Concurrency policy</Label>
          <select
            id="cron-concurrency"
            className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
            value={form.concurrencyPolicy}
            onChange={(e) =>
              setForm((f) => ({ ...f, concurrencyPolicy: e.target.value as FormState['concurrencyPolicy'] }))
            }
          >
            <option value="Allow">Allow — run concurrently</option>
            <option value="Forbid">Forbid — skip if previous still running</option>
            <option value="Replace">Replace — cancel previous, start new</option>
          </select>
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
      </div>
      <div className="flex justify-end gap-2 border-t border-border pt-4">
        <Button type="button" variant="ghost" onClick={() => void close()}>
          Cancel
        </Button>
        <Button type="submit" disabled={pending}>
          {mode === 'create' ? 'Create' : 'Save & restart schedule'}
        </Button>
      </div>
    </form>
  )
}

export function CronJobRegisterPanel() {
  const { close } = useSidePanel()
  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )
  return (
    <PanelTemplate eyebrow="Cron job" title="Create cron job" actions={closeBtn}>
      <CronJobForm
        mode="create"
        initial={{ name: '', schedule: '', command: '', workDir: '', env: '', concurrencyPolicy: 'Allow' }}
      />
    </PanelTemplate>
  )
}

export function CronJobEditPanel({ name }: { name: string }) {
  const { close } = useSidePanel()
  const { data: spec, error, isLoading } = useCronJob(name, true)
  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )

  if (isLoading || !spec) {
    return (
      <PanelTemplate eyebrow="Cron job" title={`Edit ${name}`} actions={closeBtn}>
        <p className="text-sm text-muted-foreground">
          {error ? 'Failed to load cronjob spec.' : 'Loading…'}
        </p>
      </PanelTemplate>
    )
  }

  return (
    <PanelTemplate eyebrow="Cron job" title={`Edit ${name}`} actions={closeBtn}>
      <CronJobForm mode="edit" initial={specToForm(spec)} />
    </PanelTemplate>
  )
}
