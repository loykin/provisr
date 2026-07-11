import type { DataGridColumnDef } from '@loykin/gridkit'
import { Badge } from '@/components/ui/badge'
import { TruncateCell } from '@/components/truncate-cell'
import { UserActions } from './UserActions'
import type { User } from './types'

export const userColumns: DataGridColumnDef<User>[] = [
  {
    accessorKey: 'username',
    header: 'Username',
    cell: ({ row }) => <TruncateCell>{row.original.username}</TruncateCell>,
    meta: { flex: 1, minWidth: 140 },
  },
  {
    accessorKey: 'email',
    header: 'Email',
    cell: ({ row }) => <TruncateCell>{row.original.email || '-'}</TruncateCell>,
    meta: { flex: 1, minWidth: 140 },
  },
  {
    id: 'roles',
    header: 'Roles',
    cell: ({ row }) => (
      <div className="flex flex-wrap gap-1">
        {row.original.roles.length === 0 ? (
          <span className="text-muted-foreground">-</span>
        ) : (
          row.original.roles.map((role) => (
            <Badge key={role} variant="secondary">
              {role}
            </Badge>
          ))
        )}
      </div>
    ),
    size: 200,
    meta: { cellOverflow: 'visible' },
  },
  {
    id: 'active',
    header: 'Active',
    cell: ({ row }) => (row.original.active ? 'Yes' : 'No'),
    size: 90,
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => <UserActions user={row.original} />,
    size: 90,
    meta: { cellOverflow: 'visible' },
  },
]
