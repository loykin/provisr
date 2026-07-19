import { Play, Square } from 'lucide-react'
import { IconAction } from '@/components/icon-action'
import { useAuth } from '@/features/auth/context'
import { canWriteWorkloads } from '@/features/auth/permissions'
import { useStartGroup, useStopGroup } from './queries'
import type { GroupInfo } from './types'

export function GroupActions({ group }: { group: GroupInfo }) {
  const { user } = useAuth()
  const start = useStartGroup()
  const stop = useStopGroup()
  if (!canWriteWorkloads(user)) return null

  const pending = start.isPending || stop.isPending
  return (
    <div className="flex items-center gap-1" onClick={(event) => event.stopPropagation()}>
      {group.state === 'stopped' || group.state === 'degraded' ? (
        <IconAction label="Start group" disabled={pending} onClick={() => start.mutate(group.name)}>
          <Play className="h-3.5 w-3.5" />
        </IconAction>
      ) : null}
      {group.state === 'running' || group.state === 'degraded' ? (
        <IconAction label="Stop group" disabled={pending} onClick={() => stop.mutate(group.name)}>
          <Square className="h-3.5 w-3.5" />
        </IconAction>
      ) : null}
    </div>
  )
}
