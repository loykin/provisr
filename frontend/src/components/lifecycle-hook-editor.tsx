// oxlint-disable react/only-export-components -- form panels reuse the editor's validation helpers to enforce the same limits
import { Plus, Trash2 } from 'lucide-react'
import { DetailBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { lifecyclePhases, type LifecycleHook, type LifecycleHooks } from '@/components/lifecycle-hooks'

const MAX_HOOKS_PER_PHASE = 50
const MAX_HOOKS_TOTAL = 100
const INVALID_NAME_CHARS = /[\s/\\<>:"|?*]/
const NANOS_PER_SECOND = 1_000_000_000

function emptyHook(): LifecycleHook {
  return { name: '', command: '', work_dir: '', env: [], timeout: undefined, failure_mode: '', run_mode: '' }
}

export function hasLifecycleHooks(lifecycle: LifecycleHooks): boolean {
  return lifecyclePhases.some(({ key }) => (lifecycle[key]?.length ?? 0) > 0)
}

export function validateLifecycleHooks(lifecycle: LifecycleHooks): string | null {
  const seenNames = new Set<string>()
  let total = 0

  for (const { key, label } of lifecyclePhases) {
    const hooks = lifecycle[key] ?? []
    if (hooks.length > MAX_HOOKS_PER_PHASE) {
      return `${label}: at most ${MAX_HOOKS_PER_PHASE} hooks allowed.`
    }
    total += hooks.length

    for (const hook of hooks) {
      const name = hook.name.trim()
      if (!name) return `${label}: every hook needs a name.`
      if (INVALID_NAME_CHARS.test(name)) {
        return `${label}: hook "${name}" has an invalid name (no spaces, path separators, or special characters).`
      }
      if (seenNames.has(name)) {
        return `Hook name "${name}" is used more than once — names must be unique across all phases.`
      }
      seenNames.add(name)

      if (!hook.command.trim()) return `${label}: hook "${name}" needs a command.`
      if (hook.command.length > 10000) return `${label}: hook "${name}" command is too long (max 10000 characters).`

      if (hook.work_dir && hook.work_dir.includes('..')) {
        return `${label}: hook "${name}" working directory cannot contain "..".`
      }

      for (const line of hook.env ?? []) {
        if (!line.includes('=')) {
          return `${label}: hook "${name}" env "${line}" must be in KEY=VALUE format.`
        }
        const key2 = line.slice(0, line.indexOf('=')).trim()
        if (!key2) return `${label}: hook "${name}" has an env entry with an empty key.`
        if (key2.startsWith('PROVISR_')) {
          return `${label}: hook "${name}" env key "${key2}" is reserved (PROVISR_ prefix).`
        }
      }

      const timeoutNs = typeof hook.timeout === 'string' ? Number(hook.timeout) : hook.timeout
      if (timeoutNs !== undefined && (timeoutNs < 0 || timeoutNs > 3600 * NANOS_PER_SECOND)) {
        return `${label}: hook "${name}" timeout must be between 0 and 3600 seconds.`
      }
    }
  }

  if (total > MAX_HOOKS_TOTAL) {
    return `At most ${MAX_HOOKS_TOTAL} lifecycle hooks allowed in total.`
  }
  return null
}

const selectClassName =
  'h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30'

function HookRow({
  hook,
  onChange,
  onRemove,
}: {
  hook: LifecycleHook
  onChange: (next: LifecycleHook) => void
  onRemove: () => void
}) {
  const timeoutSeconds =
    hook.timeout === undefined || hook.timeout === ''
      ? ''
      : String(Number(hook.timeout) / NANOS_PER_SECOND)

  return (
    <div className="space-y-2 rounded-(--radius) border border-border p-2.5">
      <div className="flex items-start gap-2">
        <Input
          placeholder="Hook name"
          value={hook.name}
          onChange={(e) => onChange({ ...hook, name: e.target.value })}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="shrink-0"
          onClick={onRemove}
          aria-label="Remove hook"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>
      <Textarea
        rows={2}
        className="font-mono text-xs"
        placeholder="Command"
        value={hook.command}
        onChange={(e) => onChange({ ...hook, command: e.target.value })}
      />
      <Input
        placeholder="Working directory (optional)"
        value={hook.work_dir ?? ''}
        onChange={(e) => onChange({ ...hook, work_dir: e.target.value })}
      />
      <Textarea
        rows={2}
        className="font-mono text-xs"
        placeholder="Env — one KEY=VALUE per line"
        value={(hook.env ?? []).join('\n')}
        onChange={(e) =>
          onChange({
            ...hook,
            env: e.target.value
              .split('\n')
              .map((line) => line.trim())
              .filter(Boolean),
          })
        }
      />
      <div className="grid grid-cols-3 gap-2">
        <Input
          type="number"
          min={0}
          max={3600}
          placeholder="Timeout (s)"
          value={timeoutSeconds}
          onChange={(e) =>
            onChange({
              ...hook,
              timeout: e.target.value ? Math.round(Number(e.target.value) * NANOS_PER_SECOND) : undefined,
            })
          }
        />
        <select
          className={selectClassName}
          value={hook.failure_mode || ''}
          onChange={(e) =>
            onChange({ ...hook, failure_mode: e.target.value as LifecycleHook['failure_mode'] })
          }
        >
          <option value="">Default (fail)</option>
          <option value="ignore">Ignore</option>
          <option value="fail">Fail</option>
          <option value="retry">Retry</option>
        </select>
        <select
          className={selectClassName}
          value={hook.run_mode || ''}
          onChange={(e) => onChange({ ...hook, run_mode: e.target.value as LifecycleHook['run_mode'] })}
        >
          <option value="">Default (blocking)</option>
          <option value="blocking">Blocking</option>
          <option value="async">Async</option>
        </select>
      </div>
    </div>
  )
}

export function LifecycleHookEditor({
  value,
  onChange,
}: {
  value: LifecycleHooks
  onChange: (next: LifecycleHooks) => void
}) {
  return (
    <div className="space-y-3">
      {lifecyclePhases.map(({ key, label }) => {
        const hooks = value[key] ?? []
        return (
          <DetailBodyTemplate.Section
            key={key}
            title={label}
            surface="bordered"
            actions={
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => onChange({ ...value, [key]: [...hooks, emptyHook()] })}
              >
                <Plus className="h-3.5 w-3.5" />
                Add hook
              </Button>
            }
          >
            {hooks.length === 0 ? (
              <p className="text-sm text-muted-foreground">No hooks configured.</p>
            ) : (
              <div className="space-y-2">
                {hooks.map((hook, index) => (
                  <HookRow
                    key={`${key}-${index}`}
                    hook={hook}
                    onChange={(next) => {
                      const nextHooks = hooks.slice()
                      nextHooks[index] = next
                      onChange({ ...value, [key]: nextHooks })
                    }}
                    onRemove={() => {
                      const nextHooks = hooks.slice()
                      nextHooks.splice(index, 1)
                      onChange({ ...value, [key]: nextHooks })
                    }}
                  />
                ))}
              </div>
            )}
          </DetailBodyTemplate.Section>
        )
      })}
    </div>
  )
}
