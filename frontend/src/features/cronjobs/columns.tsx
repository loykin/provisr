import type { DataGridColumnDef } from '@loykin/gridkit'
import { Badge } from '@/components/ui/badge'
import { CronJobActions } from './CronJobActions'
import type { CronJobInfo } from './types'

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : '-'
}

export const columns: DataGridColumnDef<CronJobInfo>[] = [
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'schedule', header: 'Schedule' },
  {
    id: 'status',
    header: 'Status',
    cell: ({ row }) =>
      row.original.suspend ? (
        <Badge variant="secondary">suspended</Badge>
      ) : (
        <Badge className="border-green-600/30 bg-green-600/15 text-green-700 dark:text-green-400" variant="outline">
          active
        </Badge>
      ),
  },
  {
    id: 'active',
    header: 'Running',
    cell: ({ row }) => row.original.status.active?.length ?? 0,
  },
  {
    id: 'last_schedule_time',
    header: 'Last run',
    cell: ({ row }) => formatTime(row.original.status.last_schedule_time),
  },
  {
    id: 'next_schedule',
    header: 'Next run',
    cell: ({ row }) => formatTime(row.original.next_schedule),
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => <CronJobActions job={row.original} />,
  },
]
