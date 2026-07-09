import { useState } from 'react'
import { DataGrid, type DataGridColumnDef } from '@loykin/gridkit'
import { PageTopBar } from '@loykin/designkit'
import { Input } from '@/components/ui/input'
import { useHistory } from '@/features/history/queries'
import type { HistoryRecord } from '@/features/history/types'

const columns: DataGridColumnDef<HistoryRecord>[] = [
  {
    accessorKey: 'timestamp',
    header: 'Time',
    cell: ({ row }) => new Date(row.original.timestamp).toLocaleString(),
  },
  { accessorKey: 'name', header: 'Process' },
  { accessorKey: 'status', header: 'Event' },
  { accessorKey: 'pid', header: 'PID' },
  { accessorKey: 'error', header: 'Error' },
]

export default function HistoryPage() {
  const [nameFilter, setNameFilter] = useState('')
  const { data: rows, error } = useHistory(nameFilter || undefined)

  return (
    <div className="flex h-full flex-col">
      <PageTopBar left="History" />
      <div className="flex-1 overflow-hidden p-4">
        <div className="mb-3 max-w-xs">
          <Input
            placeholder="Filter by process name…"
            value={nameFilter}
            onChange={(e) => setNameFilter(e.target.value)}
          />
        </div>
        {error && <p className="mb-2 text-sm text-destructive">Failed to load history.</p>}
        <DataGrid
          data={rows ?? []}
          columns={columns}
          getRowId={(row, index) => `${row.name}-${row.timestamp}-${index}`}
          initialSorting={[{ id: 'timestamp', desc: true }]}
        />
      </div>
    </div>
  )
}
