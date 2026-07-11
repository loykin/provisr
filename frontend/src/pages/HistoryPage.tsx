import { useState } from 'react'
import { DataGrid, DataGridPaginationBar } from '@loykin/gridkit'
import { Search, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageHeader } from '@/components/page-header'
import { columns } from '@/features/history/columns'
import { useHistory } from '@/features/history/queries'

const PAGE_SIZE = 20

export default function HistoryPage() {
  const [draftNameFilter, setDraftNameFilter] = useState('')
  const [nameFilter, setNameFilter] = useState('')
  const [pageIndex, setPageIndex] = useState(0)
  const { data, error } = useHistory(nameFilter || undefined, pageIndex, PAGE_SIZE)

  const rows = data?.rows ?? []
  const total = data?.total ?? 0
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))

  function applyFilter() {
    setNameFilter(draftNameFilter.trim())
    setPageIndex(0)
  }

  function clearFilter() {
    setDraftNameFilter('')
    setNameFilter('')
    setPageIndex(0)
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader title="History" />
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto overflow-x-hidden p-4">
        <form
          className="mb-3 flex max-w-xl items-center gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            applyFilter()
          }}
        >
          <Input
            placeholder="Filter by process name…"
            value={draftNameFilter}
            onChange={(e) => setDraftNameFilter(e.target.value)}
          />
          <Button type="submit" size="sm">
            <Search className="h-3.5 w-3.5" />
            Search
          </Button>
          {nameFilter ? (
            <Button type="button" variant="outline" size="sm" onClick={clearFilter}>
              <X className="h-3.5 w-3.5" />
              Clear
            </Button>
          ) : null}
        </form>
        {error && <p className="mb-2 text-sm text-destructive">Failed to load history.</p>}
        <DataGrid
          data={rows}
          columns={columns}
          getRowId={(row, index) => `${row.name}-${row.timestamp}-${index}`}
          initialSorting={[{ id: 'timestamp', desc: true }]}
          pagination={{
            pageSize: PAGE_SIZE,
            pageIndex,
            pageCount,
            onPageChange: (nextIndex) => setPageIndex(nextIndex),
          }}
          footer={(table) => <DataGridPaginationBar table={table} totalCount={total} className="pt-2" />}
          // gridkit's shell has `overflow: hidden`, which per the flexbox spec
          // makes its automatic min-height resolve to 0 — so as a flex child
          // it would otherwise get silently squashed (and its excess content
          // invisibly clipped) by this page's own scroll container. shrink-0
          // keeps it at natural content height so our own overflow-y-auto
          // wrapper is what scrolls, not gridkit's internal (hidden) overflow.
          classNames={{ root: 'shrink-0' }}
        />
      </div>
    </div>
  )
}
