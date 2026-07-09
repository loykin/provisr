import { DataGrid } from '@loykin/gridkit'
import { PageTopBar } from '@loykin/designkit'
import { SidePanelProvider, useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/features/auth/context'
import { useProcesses } from '@/features/processes/queries'
import { columns } from '@/features/processes/columns'
import { ProcessDetailPanel } from '@/features/processes/ProcessDetailPanel'
import { ProcessRegisterPanel } from '@/features/processes/ProcessFormPanel'

function ProcessesGrid() {
  const { open } = useSidePanel()
  const { user } = useAuth()
  const { data: rows, error } = useProcesses()

  // react-query keeps the last successful `data` around even when a later
  // background refetch fails (e.g. the server restarting) — it doesn't
  // clear to undefined. So `error` alone isn't "nothing loaded"; only treat
  // it that way when there's no data to fall back on. Otherwise a single
  // dropped poll shows a scary "failed to load" banner above a list that's
  // still populated (with slightly stale data) right below it.
  const hasData = (rows?.length ?? 0) > 0

  return (
    <div className="flex h-full flex-col">
      <PageTopBar
        left="Processes"
        right={
          user?.roles.includes('admin') ? (
            <Button size="sm" onClick={() => open(<ProcessRegisterPanel />, { size: 480 })}>
              Register process
            </Button>
          ) : undefined
        }
      />
      <div className="flex-1 overflow-hidden p-4">
        {error && !hasData && (
          <p className="mb-2 text-sm text-destructive">Failed to load process list.</p>
        )}
        {error && hasData && (
          <p className="mb-2 text-sm text-muted-foreground">
            Connection lost — showing last known data.
          </p>
        )}
        <DataGrid
          data={rows ?? []}
          columns={columns}
          getRowId={(row) => row.name}
          initialSorting={[{ id: 'name', desc: false }]}
          onRowClick={(row) => open(<ProcessDetailPanel name={row.name} />, { size: 480 })}
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
