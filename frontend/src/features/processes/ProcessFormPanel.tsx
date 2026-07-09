import { useState } from 'react'
import { X } from 'lucide-react'
import { DataBodyTemplate, PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { ApiError } from '@/lib/api'
import { useProcessSpec, useUpdateProcess } from './queries'
import type { ProcessSpec } from './types'

export interface ProcessFormState {
  name: string
  command: string
  workDir: string
  env: string
  autoRestart: boolean
  instances: string
}

export function specToForm(spec: ProcessSpec): ProcessFormState {
  return {
    name: spec.name,
    command: spec.command,
    workDir: spec.work_dir ?? '',
    env: (spec.env ?? []).join('\n'),
    autoRestart: spec.auto_restart ?? false,
    instances: spec.instances && spec.instances > 1 ? String(spec.instances) : '',
  }
}

export function formToSpec(form: ProcessFormState): ProcessSpec {
  const env = form.env
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
  const instances = Number(form.instances)
  return {
    name: form.name.trim(),
    command: form.command,
    work_dir: form.workDir.trim() || undefined,
    env: env.length > 0 ? env : undefined,
    auto_restart: form.autoRestart,
    instances: instances > 1 ? instances : undefined,
  }
}

// The field list shared by the register page and the edit panel — a
// DataBodyTemplate.Group/.Row pair renders a plain (no card) stacked
// key-value form, consistent with designkit's own form playground example,
// and works standalone whether the parent chrome is a PanelTemplate (side
// panel) or a full-page DataBodyTemplate.
export function ProcessFormFields({
  mode,
  form,
  setForm,
}: {
  mode: 'create' | 'edit'
  form: ProcessFormState
  setForm: (updater: (f: ProcessFormState) => ProcessFormState) => void
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
          placeholder="e.g. /usr/bin/myapp --flag"
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
      {mode === 'create' && (
        <DataBodyTemplate.Row label="Instances">
          <Input
            type="number"
            min={1}
            placeholder="1"
            value={form.instances}
            onChange={(e) => setForm((f) => ({ ...f, instances: e.target.value }))}
          />
        </DataBodyTemplate.Row>
      )}
      <DataBodyTemplate.Row label="Auto-restart">
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={form.autoRestart}
            onCheckedChange={(checked) => setForm((f) => ({ ...f, autoRestart: checked === true }))}
          />
          Restart automatically on exit
        </label>
      </DataBodyTemplate.Row>
    </DataBodyTemplate.Group>
  )
}

// Edit is intentionally an immediate stop+restart under the new spec (see
// POST /update on the backend) rather than a "changes apply on next
// restart" queue — simpler to reason about, and an operator editing a
// running process's command/env expects it to take effect right away.
// Register lives on its own page (ProcessRegisterPage) rather than a side
// panel — creating a new process is a primary navigation action, not a
// quick contextual edit.
function ProcessEditForm({ initial }: { initial: ProcessFormState }) {
  const { close } = useSidePanel()
  const [form, setForm] = useState(initial)
  const [error, setError] = useState<string | null>(null)
  const update = useUpdateProcess()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (!form.name.trim() || !form.command.trim()) {
      setError('Name and command are required.')
      return
    }
    try {
      await update.mutateAsync(formToSpec(form))
      await close()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save process.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <div className="flex-1 overflow-y-auto">
        <ProcessFormFields mode="edit" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
      </div>
      <div className="flex justify-end gap-2 border-t border-border p-4">
        <Button type="button" variant="ghost" onClick={() => void close()}>
          Cancel
        </Button>
        <Button type="submit" disabled={update.isPending}>
          Save &amp; restart
        </Button>
      </div>
    </form>
  )
}

export function ProcessEditPanel({ name }: { name: string }) {
  const { close } = useSidePanel()
  const { data: spec, error, isLoading } = useProcessSpec(name, true)
  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )

  if (isLoading || !spec) {
    return (
      <PanelTemplate eyebrow="Process" title={`Edit ${name}`} actions={closeBtn}>
        <p className="text-sm text-muted-foreground">
          {error ? 'Failed to load process spec.' : 'Loading…'}
        </p>
      </PanelTemplate>
    )
  }

  return (
    <PanelTemplate eyebrow="Process" title={`Edit ${name}`} actions={closeBtn}>
      <ProcessEditForm initial={specToForm(spec)} />
    </PanelTemplate>
  )
}
