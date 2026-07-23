import type { DataGridColumnDef } from '@loykin/gridkit'
import { lifecycleHookCount } from '@/components/lifecycle-hooks'
import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { TruncateCell } from '@/components/truncate-cell'
import { CronJobActions } from './CronJobActions'
import type { CronJobInfo } from './types'

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : '-'
}

export const columns: DataGridColumnDef<CronJobInfo>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    cell: ({ row }) => (
      <div className="flex min-w-0 items-center gap-1.5">
        <TruncateCell>{row.original.name}</TruncateCell>
        {row.original.provisioned && (
          <Tooltip>
            <TooltipTrigger render={<Badge variant="outline" className="shrink-0 text-muted-foreground">config</Badge>} />
            <TooltipContent>Defined in the main config file — update/delete/suspend/resume via the API is disabled.</TooltipContent>
          </Tooltip>
        )}
      </div>
    ),
    meta: { flex: 1, minWidth: 140, cellOverflow: 'visible' },
  },
  { accessorKey: 'schedule', header: 'Schedule', size: 140 },
  {
    id: 'status',
    accessorFn: (row) => (row.suspend ? 'suspended' : 'active'),
    header: 'Status',
    cell: ({ row }) => <StatusBadge status={row.original.suspend ? 'suspended' : 'active'} />,
    // Badge has rounded corners that the grid's default cell clipping cuts
    // into — cellOverflow: 'visible' is gridkit's documented fix for
    // Badge/Avatar/Chip cells. Width is set via top-level `size` (TanStack's
    // native sizing field) — `meta.width` is not read by this gridkit
    // version's column-sizing logic and silently falls back to 150px.
    size: 110,
    meta: { cellOverflow: 'visible' },
  },
  {
    id: 'active',
    header: 'Active runs',
    cell: ({ row }) => row.original.status.active?.length ?? 0,
    size: 110,
  },
  {
    id: 'hooks',
    header: 'Hooks',
    cell: ({ row }) =>
      lifecycleHookCount(row.original.lifecycle) + lifecycleHookCount(row.original.job_template.lifecycle),
    size: 90,
  },
  {
    id: 'last_schedule_time',
    header: 'Last run',
    cell: ({ row }) => <TruncateCell>{formatTime(row.original.status.last_schedule_time)}</TruncateCell>,
    size: 190,
  },
  {
    id: 'next_schedule',
    header: 'Next run',
    cell: ({ row }) => <TruncateCell>{formatTime(row.original.next_schedule)}</TruncateCell>,
    size: 190,
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => <CronJobActions job={row.original} />,
    size: 160,
    meta: { cellOverflow: 'visible' },
  },
]
