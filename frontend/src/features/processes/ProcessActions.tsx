import { Pencil, Play, Square } from 'lucide-react'
import { useSidePanel } from '@loykin/side-panel'
import { IconAction } from '@/components/icon-action'
import { useAuth } from '@/features/auth/context'
import { ProcessEditPanel } from './ProcessFormPanel'
import { useStartProcess, useStopProcess } from './queries'
import type { ProcessStatus } from './types'

// Start/Stop/Edit are write actions, gated to the admin role. `HasPermission`
// on the server enforces this regardless (operator/viewer get 403), but
// hiding the buttons for roles that can't use them avoids a confusing error
// click.
export function ProcessActions({ status }: { status: ProcessStatus }) {
  const { user } = useAuth()
  const { open } = useSidePanel()
  const start = useStartProcess()
  const stop = useStopProcess()

  if (!user?.roles.includes('admin')) return null

  const pending = start.isPending || stop.isPending

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
        label="Edit process"
        onClick={() => open(<ProcessEditPanel name={status.name} />, { size: 480 })}
      >
        <Pencil className="h-3.5 w-3.5" />
      </IconAction>
    </div>
  )
}
