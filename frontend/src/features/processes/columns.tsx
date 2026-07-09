import type { DataGridColumnDef } from '@loykin/gridkit'
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
  { accessorKey: 'pid', header: 'PID', size: 90 },
  { id: 'uptime', header: 'Uptime', cell: ({ row }) => uptime(row.original), size: 90 },
  { accessorKey: 'restarts', header: 'Restarts', size: 90 },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => <ProcessActions status={row.original} />,
    size: 100,
    meta: { cellOverflow: 'visible' },
  },
]
