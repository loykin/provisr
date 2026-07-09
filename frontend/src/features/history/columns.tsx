import type { DataGridColumnDef } from '@loykin/gridkit'
import { TruncateCell } from '@/components/truncate-cell'
import { ProcessStateBadge } from '@/features/processes/ProcessStateBadge'
import type { HistoryRecord } from './types'

export const columns: DataGridColumnDef<HistoryRecord>[] = [
  {
    accessorKey: 'timestamp',
    header: 'Time',
    cell: ({ row }) => <TruncateCell>{new Date(row.original.timestamp).toLocaleString()}</TruncateCell>,
    // Width is set via top-level `size` (TanStack's native sizing field) —
    // `meta.width` is not read by this gridkit version's column-sizing logic
    // and silently falls back to 150px.
    size: 250,
  },
  {
    accessorKey: 'name',
    header: 'Process',
    cell: ({ row }) => <TruncateCell>{row.original.name}</TruncateCell>,
    meta: { flex: 1, minWidth: 160 },
  },
  {
    accessorKey: 'status',
    header: 'Event',
    cell: ({ row }) => <ProcessStateBadge state={row.original.status} />,
    size: 110,
    meta: { cellOverflow: 'visible' },
  },
  { accessorKey: 'pid', header: 'PID', size: 90 },
  {
    accessorKey: 'error',
    header: 'Error',
    cell: ({ row }) => <TruncateCell>{row.original.error}</TruncateCell>,
    size: 200,
    meta: { minWidth: 120 },
  },
]
