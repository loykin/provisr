import type { DataGridColumnDef } from '@loykin/gridkit'
import { Badge } from '@/components/ui/badge'
import { TruncateCell } from '@/components/truncate-cell'
import { ProcessActions } from './ProcessActions'
import { ProcessStateBadge } from './ProcessStateBadge'
import type { ProcessStatus } from './types'

function uptime(status: ProcessStatus): string {
  if (!status.running) return '-'
  const ms = Date.now() - new Date(status.started_at).getTime()
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ${minutes % 60}m`
}

function GroupBadges({ groups = [] }: { groups?: string[] }) {
  if (groups.length === 0) return <span className="text-muted-foreground">-</span>
  const visible = groups.slice(0, 2)
  return (
    <div className="flex items-center gap-1 overflow-hidden">
      {visible.map((group) => <Badge key={group} variant="outline" className="max-w-28 truncate">{group}</Badge>)}
      {groups.length > visible.length && <Badge variant="outline">+{groups.length - visible.length}</Badge>}
    </div>
  )
}

export const columns: DataGridColumnDef<ProcessStatus>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    cell: ({ row }) => <TruncateCell>{row.original.name}</TruncateCell>,
    meta: { flex: 1, minWidth: 140 },
  },
  {
    accessorKey: 'state',
    header: 'State',
    cell: ({ row }) => <ProcessStateBadge state={row.original.state} />,
    // Badge has rounded corners that the grid's default cell clipping cuts
    // into — cellOverflow: 'visible' is gridkit's documented fix for
    // Badge/Avatar/Chip cells. Width is set via top-level `size` (TanStack's
    // native sizing field) — `meta.width` is not read by this gridkit
    // version's column-sizing logic and silently falls back to 150px.
    size: 110,
    meta: { cellOverflow: 'visible' },
  },
  {
    id: 'groups',
    accessorFn: (row) => (row.groups ?? []).join(' '),
    header: 'Groups',
    cell: ({ row }) => <GroupBadges groups={row.original.groups} />,
    size: 220,
    meta: { cellOverflow: 'visible' },
  },
  { accessorKey: 'pid', header: 'PID', size: 90 },
  { id: 'uptime', header: 'Uptime', cell: ({ row }) => uptime(row.original), size: 90 },
  { accessorKey: 'restarts', header: 'Restarts', size: 90 },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => <ProcessActions status={row.original} />,
    size: 130,
    meta: { cellOverflow: 'visible' },
  },
]
