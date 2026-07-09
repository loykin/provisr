import { useState } from 'react'
import { X } from 'lucide-react'
import { PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ApiError } from '@/lib/api'
import { useProcessSpec, useRegisterProcess, useUpdateProcess } from './queries'
import type { ProcessSpec } from './types'

interface FormState {
  name: string
  command: string
  workDir: string
  env: string
  autoRestart: boolean
  instances: string
}

function specToForm(spec: ProcessSpec): FormState {
  return {
    name: spec.name,
    command: spec.command,
    workDir: spec.work_dir ?? '',
    env: (spec.env ?? []).join('\n'),
    autoRestart: spec.auto_restart ?? false,
    instances: spec.instances && spec.instances > 1 ? String(spec.instances) : '',
  }
}

function formToSpec(form: FormState): ProcessSpec {
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

// Edit is intentionally an immediate stop+restart under the new spec (see
// POST /update on the backend) rather than a "changes apply on next
// restart" queue — simpler to reason about, and an operator editing a
// running process's command/env expects it to take effect right away.
function ProcessForm({ mode, initial }: { mode: 'create' | 'edit'; initial: FormState }) {
  const { close } = useSidePanel()
  const [form, setForm] = useState(initial)
  const [error, setError] = useState<string | null>(null)
  const register = useRegisterProcess()
  const update = useUpdateProcess()
  const pending = register.isPending || update.isPending

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (!form.name.trim() || !form.command.trim()) {
      setError('Name and command are required.')
      return
    }
    try {
      const spec = formToSpec(form)
      if (mode === 'create') {
        await register.mutateAsync(spec)
      } else {
        await update.mutateAsync(spec)
      }
      await close()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save process.')
    }
  }

  return (
    <form className="flex h-full flex-col" onSubmit={(e) => void handleSubmit(e)}>
      <div className="flex-1 space-y-4 overflow-y-auto">
        <div className="space-y-1.5">
          <Label htmlFor="proc-name">Name</Label>
          <Input
            id="proc-name"
            value={form.name}
            disabled={mode === 'edit'}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="proc-command">Command</Label>
          <Input
            id="proc-command"
            placeholder="e.g. /usr/bin/myapp --flag"
            value={form.command}
            onChange={(e) => setForm((f) => ({ ...f, command: e.target.value }))}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="proc-workdir">Working directory</Label>
          <Input
            id="proc-workdir"
            placeholder="(optional) absolute path"
            value={form.workDir}
            onChange={(e) => setForm((f) => ({ ...f, workDir: e.target.value }))}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="proc-env">Environment (one KEY=VALUE per line)</Label>
          <Textarea
            id="proc-env"
            rows={4}
            className="font-mono text-xs"
            value={form.env}
            onChange={(e) => setForm((f) => ({ ...f, env: e.target.value }))}
          />
        </div>
        {mode === 'create' && (
          <div className="space-y-1.5">
            <Label htmlFor="proc-instances">Instances</Label>
            <Input
              id="proc-instances"
              type="number"
              min={1}
              placeholder="1"
              value={form.instances}
              onChange={(e) => setForm((f) => ({ ...f, instances: e.target.value }))}
            />
          </div>
        )}
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={form.autoRestart}
            onCheckedChange={(checked) => setForm((f) => ({ ...f, autoRestart: checked === true }))}
          />
          Auto-restart on exit
        </label>
        {error && <p className="text-sm text-destructive">{error}</p>}
      </div>
      <div className="flex justify-end gap-2 border-t border-border pt-4">
        <Button type="button" variant="ghost" onClick={() => void close()}>
          Cancel
        </Button>
        <Button type="submit" disabled={pending}>
          {mode === 'create' ? 'Register' : 'Save & restart'}
        </Button>
      </div>
    </form>
  )
}

export function ProcessRegisterPanel() {
  const { close } = useSidePanel()
  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )
  return (
    <PanelTemplate eyebrow="Process" title="Register process" actions={closeBtn}>
      <ProcessForm
        mode="create"
        initial={{ name: '', command: '', workDir: '', env: '', autoRestart: false, instances: '' }}
      />
    </PanelTemplate>
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
      <ProcessForm mode="edit" initial={specToForm(spec)} />
    </PanelTemplate>
  )
}
