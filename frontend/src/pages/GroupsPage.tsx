import { useState } from 'react'
import { DataGrid } from '@loykin/gridkit'
import { SidePanelProvider, useSidePanel } from '@loykin/side-panel'
import { WorkloadHeader } from '@/components/workload-tabs'
import { WorkloadSearch } from '@/components/workload-search'
import { GroupDetailPanel } from '@/features/groups/GroupDetailPanel'
import { groupColumns } from '@/features/groups/columns'
import { useGroups } from '@/features/groups/queries'

function GroupsGrid() {
  const { open } = useSidePanel()
  const { data: rows, error } = useGroups()
  const [globalFilter, setGlobalFilter] = useState('')
  const hasData = (rows?.length ?? 0) > 0
  return (
    <div className="flex h-full flex-col">
      <WorkloadHeader active="groups" title="Groups" />
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto overflow-x-hidden p-4">
        {error && !hasData && <p className="mb-2 text-sm text-destructive">Failed to load Groups.</p>}
        {error && hasData && <p className="mb-2 text-sm text-muted-foreground">Connection lost — showing last known data.</p>}
        {!error && !hasData && <p className="text-sm text-muted-foreground">No groups are configured.</p>}
        {hasData && (
          <DataGrid
            data={rows ?? []}
            columns={groupColumns}
            getRowId={(row) => row.name}
            initialSorting={[{ id: 'name', desc: false }]}
            globalFilter={globalFilter}
            onGlobalFilterChange={setGlobalFilter}
            searchableColumns={['name', 'state', 'members']}
            headerLeft={(
              <div className="flex flex-wrap items-center gap-3">
                <WorkloadSearch value={globalFilter} onChange={setGlobalFilter} placeholder="Search groups…" />
                <span className="text-xs text-muted-foreground">
                  Running: all configured instances · Degraded: some running · Stopped: none running
                </span>
              </div>
            )}
            onRowClick={(group) => open(<GroupDetailPanel group={group} />, { size: 480 })}
            classNames={{ root: 'shrink-0' }}
          />
        )}
      </div>
    </div>
  )
}

export default function GroupsPage() {
  return <SidePanelProvider className="h-full" defaultSize={480} defaultMinSize={360} defaultMaxSize={800}><GroupsGrid /></SidePanelProvider>
}
