import type { DataGridColumnDef } from '@loykin/gridkit'
import { Badge } from '@/components/ui/badge'
import { StatusBadge } from '@/components/status-badge'
import { TruncateCell } from '@/components/truncate-cell'
import { GroupActions } from './GroupActions'
import type { GroupInfo } from './types'

function MemberBadges({ group }: { group: GroupInfo }) {
  const visible = group.members.slice(0, 3)
  return (
    <div className="flex items-center gap-1 overflow-hidden">
      {visible.map((member) => (
        <Badge key={member.name} variant="outline" className="max-w-32 truncate">
          {member.name}{member.instances > 1 ? ` ×${member.instances}` : ''}
        </Badge>
      ))}
      {group.members.length > visible.length && <Badge variant="outline">+{group.members.length - visible.length}</Badge>}
    </div>
  )
}

export const groupColumns: DataGridColumnDef<GroupInfo>[] = [
  {
    accessorKey: 'name',
    header: 'Name',
    cell: ({ row }) => <TruncateCell>{row.original.name}</TruncateCell>,
    meta: { flex: 1, minWidth: 160 },
  },
  {
    accessorKey: 'state',
    header: 'State',
    cell: ({ row }) => <StatusBadge status={row.original.state} />,
    size: 120,
    meta: { cellOverflow: 'visible' },
  },
  { accessorKey: 'running', header: 'Running', size: 90 },
  { accessorKey: 'total', header: 'Total', size: 80 },
  {
    id: 'members',
    accessorFn: (row) => row.members.map((member) => member.name).join(' '),
    header: 'Members',
    cell: ({ row }) => <MemberBadges group={row.original} />,
    size: 360,
    meta: { cellOverflow: 'visible' },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => <GroupActions group={row.original} />,
    size: 90,
    meta: { cellOverflow: 'visible' },
  },
]
