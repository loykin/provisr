import { useNavigate } from '@tanstack/react-router'
import { DataGrid } from '@loykin/gridkit'
import { SidePanelProvider, useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { WorkloadHeader } from '@/components/workload-tabs'
import { useAuth } from '@/features/auth/context'
import { columns } from '@/features/cronjobs/columns'
import { CronJobDetailPanel } from '@/features/cronjobs/CronJobDetailPanel'
import { useCronJobs } from '@/features/cronjobs/queries'

function CronJobsGrid() {
  const { open } = useSidePanel()
  const { user } = useAuth()
  const navigate = useNavigate()
  const { data: rows, error } = useCronJobs()

  const hasData = (rows?.length ?? 0) > 0

  return (
    <div className="flex h-full flex-col">
      <WorkloadHeader
        active="cronjobs"
        title="CronJobs"
        actions={
          user?.roles.includes('admin') ? (
            <Button size="sm" onClick={() => void navigate({ to: '/cronjobs/new' })}>
              Create CronJob
            </Button>
          ) : undefined
        }
      />
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto overflow-x-hidden p-4">
        {error && !hasData && (
          <p className="mb-2 text-sm text-destructive">Failed to load CronJobs.</p>
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
          onRowClick={(row) => open(<CronJobDetailPanel name={row.name} />, { size: 480 })}
          classNames={{ root: 'shrink-0' }}
        />
      </div>
    </div>
  )
}

export default function CronJobsPage() {
  return (
    <SidePanelProvider className="h-full" defaultSize={480} defaultMinSize={360} defaultMaxSize={800}>
      <CronJobsGrid />
    </SidePanelProvider>
  )
}
