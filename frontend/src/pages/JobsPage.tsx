import { useNavigate } from '@tanstack/react-router'
import { DataGrid } from '@loykin/gridkit'
import { SidePanelProvider, useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { WorkloadHeader } from '@/components/workload-tabs'
import { useAuth } from '@/features/auth/context'
import { columns } from '@/features/jobs/columns'
import { JobDetailPanel } from '@/features/jobs/JobDetailPanel'
import { useJobs } from '@/features/jobs/queries'

function JobsGrid() {
  const { open } = useSidePanel()
  const { user } = useAuth()
  const navigate = useNavigate()
  const { data: rows, error } = useJobs()

  const hasData = (rows?.length ?? 0) > 0

  return (
    <div className="flex h-full flex-col">
      <WorkloadHeader
        active="jobs"
        title="Jobs"
        actions={
          user?.roles.includes('admin') ? (
            <Button size="sm" onClick={() => void navigate({ to: '/jobs/new' })}>
              Create Job
            </Button>
          ) : undefined
        }
      />
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto overflow-x-hidden p-4">
        {error && !hasData && (
          <p className="mb-2 text-sm text-destructive">Failed to load Jobs.</p>
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
          onRowClick={(row) => open(<JobDetailPanel name={row.name} />, { size: 480 })}
          classNames={{ root: 'shrink-0' }}
        />
      </div>
    </div>
  )
}

export default function JobsPage() {
  return (
    <SidePanelProvider className="h-full" defaultSize={480} defaultMinSize={360} defaultMaxSize={800}>
      <JobsGrid />
    </SidePanelProvider>
  )
}
