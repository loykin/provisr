import { useState } from 'react'
import { X } from 'lucide-react'
import { DataBodyTemplate, PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { LifecycleHookEditor, hasLifecycleHooks, validateLifecycleHooks } from '@/components/lifecycle-hook-editor'
import type { LifecycleHooks } from '@/components/lifecycle-hooks'
import { ApiError } from '@/lib/api'
import { useJob, useUpdateJob } from './queries'
import type { JobSpec } from './types'

export interface JobFormState {
  name: string
  command: string
  workDir: string
  env: string
  parallelism: string
  completions: string
  restartPolicy: 'Never' | 'OnFailure'
  backoffLimit: string
  activeDeadlineSeconds: string
  lifecycle: LifecycleHooks
}

export function specToForm(spec: JobSpec): JobFormState {
  return {
    name: spec.name,
    command: spec.command ?? (spec.args ?? []).join(' '),
    workDir: spec.work_dir ?? '',
    env: (spec.env ?? []).join('\n'),
    parallelism: spec.parallelism ? String(spec.parallelism) : '',
    completions: spec.completions ? String(spec.completions) : '',
    restartPolicy: spec.restart_policy === 'OnFailure' ? 'OnFailure' : 'Never',
    backoffLimit: spec.backoff_limit ? String(spec.backoff_limit) : '',
    activeDeadlineSeconds: spec.active_deadline_seconds ? String(spec.active_deadline_seconds) : '',
    lifecycle: spec.lifecycle ?? {},
  }
}

function optionalPositiveInt(value: string): number | undefined {
  const n = Number(value)
  return Number.isFinite(n) && n > 0 ? n : undefined
}

export function formToSpec(form: JobFormState, base?: JobSpec): JobSpec {
  const env = form.env
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
  const baseArgsCommand = base?.args && base.args.length > 0 ? base.args.join(' ') : undefined
  const keepArgs = Boolean(baseArgsCommand && !base?.command && form.command === baseArgsCommand)
  return {
    ...base,
    name: form.name.trim(),
    command: keepArgs ? undefined : form.command,
    args: keepArgs ? base?.args : undefined,
    work_dir: form.workDir.trim() || undefined,
    env: env.length > 0 ? env : undefined,
    parallelism: optionalPositiveInt(form.parallelism),
    completions: optionalPositiveInt(form.completions),
    restart_policy: form.restartPolicy,
    backoff_limit: optionalPositiveInt(form.backoffLimit),
    active_deadline_seconds: optionalPositiveInt(form.activeDeadlineSeconds),
    lifecycle: hasLifecycleHooks(form.lifecycle) ? form.lifecycle : undefined,
  }
}

export function JobFormFields({
  mode,
  form,
  setForm,
}: {
  mode: 'create' | 'edit'
  form: JobFormState
  setForm: (updater: (f: JobFormState) => JobFormState) => void
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
      <DataBodyTemplate.Row label="Parallelism">
        <Input
          type="number"
          min={1}
          placeholder="1"
          value={form.parallelism}
          onChange={(e) => setForm((f) => ({ ...f, parallelism: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Completions">
        <Input
          type="number"
          min={1}
          placeholder="1"
          value={form.completions}
          onChange={(e) => setForm((f) => ({ ...f, completions: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Restart policy">
        <select
          className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
          value={form.restartPolicy}
          onChange={(e) =>
            setForm((f) => ({ ...f, restartPolicy: e.target.value as JobFormState['restartPolicy'] }))
          }
        >
          <option value="Never">Never</option>
          <option value="OnFailure">OnFailure</option>
        </select>
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Backoff limit">
        <Input
          type="number"
          min={0}
          placeholder="6"
          value={form.backoffLimit}
          onChange={(e) => setForm((f) => ({ ...f, backoffLimit: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Active deadline seconds">
        <Input
          type="number"
          min={1}
          value={form.activeDeadlineSeconds}
          onChange={(e) => setForm((f) => ({ ...f, activeDeadlineSeconds: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row
        label="Lifecycle hooks"
        description="Commands run before/after the job starts or stops"
      >
        <LifecycleHookEditor
          value={form.lifecycle}
          onChange={(lifecycle) => setForm((f) => ({ ...f, lifecycle }))}
        />
      </DataBodyTemplate.Row>
    </DataBodyTemplate.Group>
  )
}

function JobEditForm({ spec }: { spec: JobSpec }) {
  const { close } = useSidePanel()
  const [form, setForm] = useState(() => specToForm(spec))
  const [error, setError] = useState<string | null>(null)
  const update = useUpdateJob()

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
      await update.mutateAsync(formToSpec(form, spec))
      await close()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save job.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <div className="flex-1 overflow-y-auto">
        <JobFormFields mode="edit" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
      </div>
      <div className="flex justify-end gap-2 border-t border-border p-4">
        <Button type="button" variant="ghost" onClick={() => void close()}>
          Cancel
        </Button>
        <Button type="submit" disabled={update.isPending}>
          Save &amp; restart job
        </Button>
      </div>
    </form>
  )
}

export function JobEditPanel({ name }: { name: string }) {
  const { close } = useSidePanel()
  const { data: job, error, isLoading } = useJob(name, true)
  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )

  if (isLoading || !job) {
    return (
      <PanelTemplate eyebrow="Job" title={`Edit ${name}`} actions={closeBtn}>
        <p className="text-sm text-muted-foreground">
          {error ? 'Failed to load job spec.' : 'Loading…'}
        </p>
      </PanelTemplate>
    )
  }

  return (
    <PanelTemplate eyebrow="Job" title={`Edit ${name}`} actions={closeBtn}>
      <JobEditForm spec={job} />
    </PanelTemplate>
  )
}
