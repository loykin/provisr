import { Pencil, Play, Square, Trash2 } from 'lucide-react'
import { useSidePanel } from '@loykin/side-panel'
import { IconAction } from '@/components/icon-action'
import { useAuth } from '@/features/auth/context'
import { canWriteWorkloads } from '@/features/auth/permissions'
import { ProcessEditPanel } from './ProcessFormPanel'
import { useStartProcess, useStopProcess, useUnregisterProcess } from './queries'
import type { ProcessStatus } from './types'

// Shown on Edit/Unregister when the process is declared in the main config
// file's [[processes]] array — the API refuses both (see
// internal/server/router.go's isInlineConfiguredProcess), so the buttons are
// disabled here rather than left to fail silently on click.
const PROVISIONED_HINT = 'Defined in the main config file — edit the config and restart the daemon to change this.'

// The UI mirrors the server's process:write role policy. The API remains the
// enforcement boundary; this gate only avoids controls that would return 403.
export function ProcessActions({ status, onUnregistered }: { status: ProcessStatus; onUnregistered?: () => void }) {
  const { user } = useAuth()
  const { open } = useSidePanel()
  const start = useStartProcess()
  const stop = useStopProcess()
  const unregister = useUnregisterProcess()

  if (!canWriteWorkloads(user)) return null

  const pending = start.isPending || stop.isPending || unregister.isPending
  const locked = status.provisioned === true

  function handleUnregister() {
    if (window.confirm(`Unregister process "${status.name}" and its instance set? Its persisted program file will also be removed.`)) {
      unregister.mutate(status.name, {
        onSuccess: onUnregistered,
        onError: (err) => window.alert(err instanceof Error ? err.message : 'Failed to unregister process.'),
      })
    }
  }

  return (
    <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
      {status.running ? (
        <IconAction label="Stop process" disabled={pending} onClick={() => stop.mutate(status.name)}>
          <Square className="h-3.5 w-3.5" />
        </IconAction>
      ) : (
        <IconAction label="Start process" disabled={pending} onClick={() => start.mutate(status.name)}>
          <Play className="h-3.5 w-3.5" />
        </IconAction>
      )}
      <IconAction
        label={locked ? PROVISIONED_HINT : 'Edit process'}
        disabled={locked}
        onClick={() => open(<ProcessEditPanel name={status.name} />, { size: 480 })}
      >
        <Pencil className="h-3.5 w-3.5" />
      </IconAction>
      <IconAction
        label={locked ? PROVISIONED_HINT : 'Unregister process'}
        disabled={pending || locked}
        onClick={handleUnregister}
      >
        <Trash2 className="h-3.5 w-3.5" />
      </IconAction>
    </div>
  )
}
