import { useState } from 'react'
import { X } from 'lucide-react'
import { DataBodyTemplate, PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { ApiError } from '@/lib/api'
import { useCronJob, useUpdateCronJob } from './queries'
import type { CronJobSpec } from './types'

export interface CronJobFormState {
  name: string
  schedule: string
  command: string
  workDir: string
  env: string
  concurrencyPolicy: 'Allow' | 'Forbid' | 'Replace'
}

export function specToForm(spec: CronJobSpec): CronJobFormState {
  return {
    name: spec.name,
    schedule: spec.schedule,
    command: spec.job_template.command,
    workDir: spec.job_template.work_dir ?? '',
    env: (spec.job_template.env ?? []).join('\n'),
    concurrencyPolicy: (spec.concurrency_policy || 'Allow') as CronJobFormState['concurrencyPolicy'],
  }
}

export function formToSpec(form: CronJobFormState): CronJobSpec {
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

// The field list shared by the register page and the edit panel — see
// ProcessFormFields for why this uses DataBodyTemplate.Group/.Row directly
// rather than hand-rolled label+input divs.
export function CronJobFormFields({
  mode,
  form,
  setForm,
}: {
  mode: 'create' | 'edit'
  form: CronJobFormState
  setForm: (updater: (f: CronJobFormState) => CronJobFormState) => void
}) {
  return (
    <DataBodyTemplate.Group layout="stacked">
      <DataBodyTemplate.Row label="Name" required>
        <Input
          value={form.name}
          disabled={mode === 'edit'}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          required
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Schedule" required>
        <Input
          placeholder="e.g. 0 */6 * * * or @every 1h"
          value={form.schedule}
          onChange={(e) => setForm((f) => ({ ...f, schedule: e.target.value }))}
          required
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Command" required>
        <Input
          placeholder="e.g. /usr/bin/backup.sh"
          value={form.command}
          onChange={(e) => setForm((f) => ({ ...f, command: e.target.value }))}
          required
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Working directory">
        <Input
          placeholder="(optional) absolute path"
          value={form.workDir}
          onChange={(e) => setForm((f) => ({ ...f, workDir: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Environment" description="One KEY=VALUE per line">
        <Textarea
          rows={4}
          className="font-mono text-xs"
          value={form.env}
          onChange={(e) => setForm((f) => ({ ...f, env: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Concurrency policy">
        <select
          className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
          value={form.concurrencyPolicy}
          onChange={(e) =>
            setForm((f) => ({
              ...f,
              concurrencyPolicy: e.target.value as CronJobFormState['concurrencyPolicy'],
            }))
          }
        >
          <option value="Allow">Allow — run concurrently</option>
          <option value="Forbid">Forbid — skip if previous still running</option>
          <option value="Replace">Replace — cancel previous, start new</option>
        </select>
      </DataBodyTemplate.Row>
    </DataBodyTemplate.Group>
  )
}

// Update stops the old schedule and starts the new one immediately (see
// POST /cronjobs/:name on the backend) — consistent with how process spec
// edits also apply immediately rather than queuing for a later restart.
// Create lives on its own page (CronJobRegisterPage) rather than a side
// panel — same rationale as ProcessRegisterPage.
function CronJobEditForm({ initial }: { initial: CronJobFormState }) {
  const { close } = useSidePanel()
  const [form, setForm] = useState(initial)
  const [error, setError] = useState<string | null>(null)
  const update = useUpdateCronJob()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (!form.name.trim() || !form.schedule.trim() || !form.command.trim()) {
      setError('Name, schedule, and command are required.')
      return
    }
    try {
      await update.mutateAsync(formToSpec(form))
      await close()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save cronjob.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <div className="flex-1 overflow-y-auto">
        <CronJobFormFields mode="edit" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
      </div>
      <div className="flex justify-end gap-2 border-t border-border p-4">
        <Button type="button" variant="ghost" onClick={() => void close()}>
          Cancel
        </Button>
        <Button type="submit" disabled={update.isPending}>
          Save &amp; restart schedule
        </Button>
      </div>
    </form>
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
      <CronJobEditForm initial={specToForm(spec)} />
    </PanelTemplate>
  )
}
