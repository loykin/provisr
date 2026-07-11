import type { DataGridColumnDef } from '@loykin/gridkit'
import { lifecycleHookCount } from '@/components/lifecycle-hooks'
import { TruncateCell } from '@/components/truncate-cell'
import { JobActions } from './JobActions'
import { JobStateBadge } from './JobStateBadge'
import type { JobInfo } from './types'

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : '-'
}

export const columns: DataGridColumnDef<JobInfo>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    cell: ({ row }) => <TruncateCell>{row.original.name}</TruncateCell>,
    meta: { flex: 1, minWidth: 160 },
  },
  {
    id: 'phase',
    header: 'Status',
    cell: ({ row }) => <JobStateBadge phase={row.original.status.phase} />,
    size: 120,
    meta: { cellOverflow: 'visible' },
  },
  { id: 'active', header: 'Active', cell: ({ row }) => row.original.status.active, size: 90 },
  { id: 'succeeded', header: 'Succeeded', cell: ({ row }) => row.original.status.succeeded, size: 110 },
  { id: 'failed', header: 'Failed', cell: ({ row }) => row.original.status.failed, size: 90 },
  {
    id: 'started',
    header: 'Started',
    cell: ({ row }) => <TruncateCell>{formatTime(row.original.status.start_time)}</TruncateCell>,
    size: 190,
  },
  {
    id: 'hooks',
    header: 'Hooks',
    cell: ({ row }) => lifecycleHookCount(row.original.lifecycle),
    size: 90,
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => <JobActions job={row.original} />,
    size: 100,
    meta: { cellOverflow: 'visible' },
  },
]
