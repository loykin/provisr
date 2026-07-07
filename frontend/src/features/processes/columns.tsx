import type { DataGridColumnDef } from '@loykin/gridkit'
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
  { accessorKey: 'name', header: 'Name' },
  {
    accessorKey: 'state',
    header: 'State',
    cell: ({ row }) => <ProcessStateBadge state={row.original.state} />,
  },
  { accessorKey: 'pid', header: 'PID' },
  { id: 'uptime', header: 'Uptime', cell: ({ row }) => uptime(row.original) },
  { accessorKey: 'restarts', header: 'Restarts' },
]
