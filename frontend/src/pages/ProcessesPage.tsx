import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { DataGrid } from '@loykin/gridkit'
import { SidePanelProvider, useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { WorkloadSearch } from '@/components/workload-search'
import { WorkloadHeader } from '@/components/workload-tabs'
import { useAuth } from '@/features/auth/context'
import { canWriteWorkloads } from '@/features/auth/permissions'
import { columns } from '@/features/processes/columns'
import { ProcessDetailPanel } from '@/features/processes/ProcessDetailPanel'
import { useProcesses, useStartProcess, useStopProcess, useUnregisterProcess } from '@/features/processes/queries'
import { useGroups } from '@/features/groups/queries'

function ProcessesGrid() {
  const { open } = useSidePanel()
  const { user } = useAuth()
  const navigate = useNavigate()
  const { data: rows, error } = useProcesses()
  const { data: groups } = useGroups()
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set())
  const [globalFilter, setGlobalFilter] = useState('')
  const start = useStartProcess()
  const stop = useStopProcess()
  const unregister = useUnregisterProcess()
  const canWrite = canWriteWorkloads(user)

  const rowsWithGroups = useMemo(() => (rows ?? []).map((process) => ({
    ...process,
    groups: (groups ?? [])
      .filter((group) => group.members.some((member) => {
        if (member.instances <= 1) return process.name === member.name
        for (let i = 1; i <= member.instances; i += 1) {
          if (process.name === `${member.name}-${i}`) return true
        }
        return false
      }))
      .map((group) => group.name),
  })), [groups, rows])

  async function runBulk(action: 'start' | 'stop' | 'unregister') {
    const names = [...selectedIds]
    if (names.length === 0) return
    if (action === 'unregister' && !window.confirm(`Unregister ${names.length} selected processes? Persisted program files will also be removed.`)) return
    const mutation = action === 'start' ? start : action === 'stop' ? stop : unregister
    const results = await Promise.allSettled(names.map((name) => mutation.mutateAsync(name)))
    const failed = results.filter((result) => result.status === 'rejected').length
    setSelectedIds(new Set())
    if (failed > 0) window.alert(`${names.length - failed} succeeded; ${failed} failed.`)
  }

  // react-query keeps the last successful `data` around even when a later
  // background refetch fails (e.g. the server restarting) — it doesn't
  // clear to undefined. So `error` alone isn't "nothing loaded"; only treat
  // it that way when there's no data to fall back on. Otherwise a single
  // dropped poll shows a scary "failed to load" banner above a list that's
  // still populated (with slightly stale data) right below it.
  const hasData = (rows?.length ?? 0) > 0

  return (
    <div className="flex h-full flex-col">
      <WorkloadHeader active="processes" title="Processes" />
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto overflow-x-hidden p-4">
        {error && !hasData && (
          <p className="mb-2 text-sm text-destructive">Failed to load process list.</p>
        )}
        {error && hasData && (
          <p className="mb-2 text-sm text-muted-foreground">
            Connection lost — showing last known data.
          </p>
        )}
        <DataGrid
          data={rowsWithGroups}
          columns={columns}
          getRowId={(row) => row.name}
          initialSorting={[{ id: 'name', desc: false }]}
          globalFilter={globalFilter}
          onGlobalFilterChange={setGlobalFilter}
          searchableColumns={['name', 'state', 'groups', 'pid']}
          checkboxConfig={canWrite ? {
            getRowId: (row) => row.name,
            selectedIds,
            onSelectAll: (selectedRows, checked) => setSelectedIds(checked ? new Set(selectedRows.map((row) => row.original.name)) : new Set()),
            onSelectOne: (rowId, checked) => setSelectedIds((current) => {
              const next = new Set(current)
              if (checked) next.add(rowId)
              else next.delete(rowId)
              return next
            }),
          } : undefined}
          headerLeft={(
            <div className="flex flex-wrap items-center gap-2">
              <WorkloadSearch value={globalFilter} onChange={setGlobalFilter} placeholder="Search processes…" />
              {selectedIds.size > 0 && (
                <>
                  <span className="text-xs text-muted-foreground">{selectedIds.size} selected</span>
                  <Button variant="outline" onClick={() => void runBulk('start')}>Start</Button>
                  <Button variant="outline" onClick={() => void runBulk('stop')}>Stop</Button>
                  <Button variant="outline" onClick={() => void runBulk('unregister')}>Unregister</Button>
                </>
              )}
            </div>
          )}
          headerRight={canWrite ? (
            <Button onClick={() => void navigate({ to: '/processes/new' })}>Register process</Button>
          ) : undefined}
          onRowClick={(row) => open(<ProcessDetailPanel name={row.name} />, { size: 480 })}
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

export default function ProcessesPage() {
  return (
    <SidePanelProvider
      className="h-full"
      defaultSize={480}
      defaultMinSize={360}
      defaultMaxSize={800}
    >
      <ProcessesGrid />
    </SidePanelProvider>
  )
}
