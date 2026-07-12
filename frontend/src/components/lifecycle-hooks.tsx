// oxlint-disable react/only-export-components -- lifecycle display components share their small wire-type and counting helpers
import { Badge } from '@/components/ui/badge'

export interface LifecycleHook {
  name: string
  command: string
  work_dir?: string
  env?: string[]
  timeout?: number | string
  failure_mode?: 'ignore' | 'fail' | 'retry' | ''
  run_mode?: 'blocking' | 'async' | ''
}

export interface LifecycleHooks {
  pre_start?: LifecycleHook[]
  post_start?: LifecycleHook[]
  pre_stop?: LifecycleHook[]
  post_stop?: LifecycleHook[]
}

export const lifecyclePhases: Array<{ key: keyof LifecycleHooks; label: string }> = [
  { key: 'pre_start', label: 'Pre start' },
  { key: 'post_start', label: 'Post start' },
  { key: 'pre_stop', label: 'Pre stop' },
  { key: 'post_stop', label: 'Post stop' },
]

export function lifecycleHookCount(lifecycle?: LifecycleHooks): number {
  if (!lifecycle) return 0
  return lifecyclePhases.reduce((count, phase) => count + (lifecycle[phase.key]?.length ?? 0), 0)
}

export function LifecycleHookSummary({ lifecycle }: { lifecycle?: LifecycleHooks }) {
  const count = lifecycleHookCount(lifecycle)
  if (count === 0) return <span className="text-muted-foreground">-</span>

  return (
    <div className="flex flex-wrap gap-1">
      {lifecyclePhases.map(({ key, label }) => {
        const hooks = lifecycle?.[key] ?? []
        if (hooks.length === 0) return null
        return (
          <Badge key={key} variant="secondary">
            {label}: {hooks.length}
          </Badge>
        )
      })}
    </div>
  )
}

export function LifecycleHookList({ lifecycle }: { lifecycle?: LifecycleHooks }) {
  const count = lifecycleHookCount(lifecycle)
  if (count === 0) {
    return <p className="text-sm text-muted-foreground">No lifecycle hooks configured.</p>
  }

  return (
    <div className="space-y-3">
      {lifecyclePhases.map(({ key, label }) => {
        const hooks = lifecycle?.[key] ?? []
        if (hooks.length === 0) return null
        return (
          <div key={key}>
            <div className="mb-1 text-sm font-medium text-muted-foreground">{label}</div>
            <div className="space-y-2">
              {hooks.map((hook) => (
                <div key={`${key}-${hook.name}`} className="rounded-(--radius) border border-border p-2">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-medium">{hook.name}</span>
                    <span className="text-xs text-muted-foreground">
                      {hook.run_mode || 'blocking'} / {hook.failure_mode || 'fail'}
                    </span>
                  </div>
                  <div className="mt-1 break-all font-mono text-xs text-muted-foreground">
                    {hook.command}
                  </div>
                  {hook.work_dir && (
                    <div className="mt-1 break-all text-xs text-muted-foreground">
                      cwd: <span className="font-mono">{hook.work_dir}</span>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}
