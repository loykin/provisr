// oxlint-disable react/only-export-components -- create pages and edit panels intentionally share one form state conversion boundary
import { useState } from 'react'
import { X } from 'lucide-react'
import { DataBodyTemplate, PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { LifecycleHookEditor, hasLifecycleHooks, validateLifecycleHooks } from '@/components/lifecycle-hook-editor'
import type { LifecycleHooks } from '@/components/lifecycle-hooks'
import { useProcessSpec, useUpdateProcess } from './queries'
import type { ProcessSpec } from './types'

export interface ProcessFormState {
  name: string
  command: string
  workDir: string
  env: string
  autoRestart: boolean
  instances: string
  pidFile: string
  priority: string
  retryCount: string
  retryInterval: string
  startDuration: string
  restartInterval: string
  detached: boolean
  detectors: string
  logDir: string
  stdoutPath: string
  stderrPath: string
  logMaxSize: string
  logMaxBackups: string
  logMaxAge: string
  logCompress: boolean
  lifecycle: LifecycleHooks
}

function durationToForm(value?: string | number): string {
  if (value === undefined || value === '' || value === 0) return ''
  if (typeof value === 'string') return value
  if (value % 1_000_000_000 === 0) return `${value / 1_000_000_000}s`
  if (value % 1_000_000 === 0) return `${value / 1_000_000}ms`
  return `${value}ns`
}

function durationToNanos(value: string): number | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  const match = trimmed.match(/^(\d+(?:\.\d+)?)\s*(ns|us|µs|ms|s|m|h)$/)
  if (!match) throw new Error(`Invalid duration "${value}". Use values such as 500ms, 2s, or 1m.`)
  const multipliers: Record<string, number> = { ns: 1, us: 1_000, 'µs': 1_000, ms: 1_000_000, s: 1_000_000_000, m: 60_000_000_000, h: 3_600_000_000_000 }
  return Number(match[1]) * multipliers[match[2]]
}

function optionalNumber(value: string): number | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  return Number(trimmed)
}

export function specToForm(spec: ProcessSpec): ProcessFormState {
  return {
    name: spec.name,
    command: spec.command ?? (spec.args ?? []).join(' '),
    workDir: spec.work_dir ?? '',
    env: (spec.env ?? []).join('\n'),
    autoRestart: spec.auto_restart ?? false,
    instances: spec.instances && spec.instances > 1 ? String(spec.instances) : '',
    pidFile: spec.pid_file ?? '',
    priority: spec.priority === undefined ? '' : String(spec.priority),
    retryCount: spec.retry_count === undefined ? '' : String(spec.retry_count),
    retryInterval: durationToForm(spec.retry_interval),
    startDuration: durationToForm(spec.start_duration),
    restartInterval: durationToForm(spec.restart_interval),
    detached: spec.detached ?? false,
    detectors: spec.detectors?.length ? JSON.stringify(spec.detectors, null, 2) : '',
    logDir: spec.log?.file?.dir ?? '',
    stdoutPath: spec.log?.file?.stdoutPath ?? '',
    stderrPath: spec.log?.file?.stderrPath ?? '',
    logMaxSize: spec.log?.file?.maxSizeMB ? String(spec.log.file.maxSizeMB) : '',
    logMaxBackups: spec.log?.file?.maxBackups ? String(spec.log.file.maxBackups) : '',
    logMaxAge: spec.log?.file?.maxAgeDays ? String(spec.log.file.maxAgeDays) : '',
    logCompress: spec.log?.file?.compress ?? false,
    lifecycle: spec.lifecycle ?? {},
  }
}

export function formToSpec(form: ProcessFormState, base?: ProcessSpec): ProcessSpec {
  const env = form.env
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
  const instances = Number(form.instances)
  const baseArgsCommand = base?.args && base.args.length > 0 ? base.args.join(' ') : undefined
  const keepArgs = Boolean(baseArgsCommand && !base?.command && form.command === baseArgsCommand)
  let detectors: Array<Record<string, unknown>> | undefined
  if (form.detectors.trim()) {
    const parsed: unknown = JSON.parse(form.detectors)
    if (!Array.isArray(parsed)) throw new Error('Detectors must be a JSON array.')
    detectors = parsed as Array<Record<string, unknown>>
  }
  const hasLog = Boolean(base?.log || form.logDir.trim() || form.stdoutPath.trim() || form.stderrPath.trim() || form.logMaxSize || form.logMaxBackups || form.logMaxAge || form.logCompress)
  return {
    ...base,
    name: form.name.trim(),
    command: keepArgs ? undefined : form.command,
    args: keepArgs ? base?.args : undefined,
    work_dir: form.workDir.trim() || undefined,
    env: env.length > 0 ? env : undefined,
    auto_restart: form.autoRestart,
    instances: instances > 1 ? instances : undefined,
    pid_file: form.pidFile.trim() || undefined,
    priority: optionalNumber(form.priority),
    retry_count: optionalNumber(form.retryCount),
    retry_interval: durationToNanos(form.retryInterval),
    start_duration: durationToNanos(form.startDuration),
    restart_interval: durationToNanos(form.restartInterval),
    detached: form.detached,
    detectors,
    log: hasLog ? {
      ...base?.log,
      file: {
        ...base?.log?.file,
        dir: form.logDir.trim() || undefined,
        stdoutPath: form.stdoutPath.trim() || undefined,
        stderrPath: form.stderrPath.trim() || undefined,
        maxSizeMB: optionalNumber(form.logMaxSize),
        maxBackups: optionalNumber(form.logMaxBackups),
        maxAgeDays: optionalNumber(form.logMaxAge),
        compress: form.logCompress,
      },
    } : undefined,
    lifecycle: hasLifecycleHooks(form.lifecycle) ? form.lifecycle : undefined,
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
		  aria-label="Name"
          value={form.name}
          disabled={mode === 'edit'}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          required
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Command" required>
        <Input
		  aria-label="Command"
          placeholder="e.g. /usr/bin/myapp --flag"
          value={form.command}
          onChange={(e) => setForm((f) => ({ ...f, command: e.target.value }))}
          required
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Working directory">
        <Input
		  aria-label="Working directory"
          placeholder="(optional) absolute path"
          value={form.workDir}
          onChange={(e) => setForm((f) => ({ ...f, workDir: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Environment" description="One KEY=VALUE per line">
        <Textarea
		  aria-label="Environment"
          rows={4}
          className="font-mono text-xs"
          value={form.env}
          onChange={(e) => setForm((f) => ({ ...f, env: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Instances" description={mode === 'edit' ? 'Changing this restarts the process set' : undefined}>
        <Input
          aria-label="Instances"
          type="number"
          min={1}
          placeholder="1"
          value={form.instances}
          onChange={(e) => setForm((f) => ({ ...f, instances: e.target.value }))}
        />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Auto-restart">
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={form.autoRestart}
            onCheckedChange={(checked) => setForm((f) => ({ ...f, autoRestart: checked === true }))}
          />
          Restart automatically on exit
        </label>
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Detached">
        <label className="flex items-center gap-2 text-sm">
          <Checkbox checked={form.detached} onCheckedChange={(checked) => setForm((f) => ({ ...f, detached: checked === true }))} />
          Run independently from provisr log capture
        </label>
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="PID file">
        <Input aria-label="PID file" placeholder="(optional) absolute path" value={form.pidFile} onChange={(e) => setForm((f) => ({ ...f, pidFile: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Priority" description="Lower values start first">
        <Input aria-label="Priority" type="number" placeholder="0" value={form.priority} onChange={(e) => setForm((f) => ({ ...f, priority: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Retry count">
        <Input aria-label="Retry count" type="number" min={0} placeholder="0" value={form.retryCount} onChange={(e) => setForm((f) => ({ ...f, retryCount: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Retry interval" description="Examples: 500ms, 2s, 1m">
        <Input aria-label="Retry interval" placeholder="(optional)" value={form.retryInterval} onChange={(e) => setForm((f) => ({ ...f, retryInterval: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Start duration" description="Minimum healthy runtime before start succeeds">
        <Input aria-label="Start duration" placeholder="(optional) e.g. 2s" value={form.startDuration} onChange={(e) => setForm((f) => ({ ...f, startDuration: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Restart interval">
        <Input aria-label="Restart interval" placeholder="(optional) e.g. 5s" value={form.restartInterval} onChange={(e) => setForm((f) => ({ ...f, restartInterval: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Detectors" description='JSON array, e.g. [{"type":"pidfile","path":"/tmp/app.pid"}]'>
        <Textarea aria-label="Detectors" rows={4} className="font-mono text-xs" value={form.detectors} onChange={(e) => setForm((f) => ({ ...f, detectors: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Log directory" description="Cannot be combined with detached mode">
        <Input aria-label="Log directory" placeholder="(optional) absolute path" value={form.logDir} onChange={(e) => setForm((f) => ({ ...f, logDir: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Stdout path">
        <Input aria-label="Stdout path" placeholder="(optional) absolute path" value={form.stdoutPath} onChange={(e) => setForm((f) => ({ ...f, stdoutPath: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Stderr path">
        <Input aria-label="Stderr path" placeholder="(optional) absolute path" value={form.stderrPath} onChange={(e) => setForm((f) => ({ ...f, stderrPath: e.target.value }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Log rotation" description="Size MB / backups / age days">
        <div className="grid grid-cols-3 gap-2">
          <Input aria-label="Max log size MB" type="number" min={1} placeholder="10" value={form.logMaxSize} onChange={(e) => setForm((f) => ({ ...f, logMaxSize: e.target.value }))} />
          <Input aria-label="Max log backups" type="number" min={0} placeholder="3" value={form.logMaxBackups} onChange={(e) => setForm((f) => ({ ...f, logMaxBackups: e.target.value }))} />
          <Input aria-label="Max log age days" type="number" min={0} placeholder="7" value={form.logMaxAge} onChange={(e) => setForm((f) => ({ ...f, logMaxAge: e.target.value }))} />
        </div>
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row label="Compress rotated logs">
        <Checkbox checked={form.logCompress} onCheckedChange={(checked) => setForm((f) => ({ ...f, logCompress: checked === true }))} />
      </DataBodyTemplate.Row>
      <DataBodyTemplate.Row
        label="Lifecycle hooks"
        description="Commands run before/after the process starts or stops"
      >
        <LifecycleHookEditor
          value={form.lifecycle}
          onChange={(lifecycle) => setForm((f) => ({ ...f, lifecycle }))}
        />
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
function ProcessEditForm({ spec }: { spec: ProcessSpec }) {
  const { close } = useSidePanel()
  const [form, setForm] = useState(() => specToForm(spec))
  const [error, setError] = useState<string | null>(null)
  const update = useUpdateProcess()

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
      setError(err instanceof Error ? err.message : 'Failed to save process.')
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
      <ProcessEditForm spec={spec} />
    </PanelTemplate>
  )
}
