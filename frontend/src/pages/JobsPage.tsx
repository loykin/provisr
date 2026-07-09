import { DataGrid } from '@loykin/gridkit'
import { PageTopBar } from '@loykin/designkit'
import { SidePanelProvider, useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/features/auth/context'
import { columns } from '@/features/cronjobs/columns'
import { CronJobDetailPanel } from '@/features/cronjobs/CronJobDetailPanel'
import { CronJobRegisterPanel } from '@/features/cronjobs/CronJobFormPanel'
import { useCronJobs } from '@/features/cronjobs/queries'

function JobsGrid() {
  const { open } = useSidePanel()
  const { user } = useAuth()
  const { data: rows, error } = useCronJobs()

  const hasData = (rows?.length ?? 0) > 0

  return (
    <div className="flex h-full flex-col">
      <PageTopBar
        left="Jobs"
        right={
          user?.roles.includes('admin') ? (
            <Button size="sm" onClick={() => open(<CronJobRegisterPanel />, { size: 480 })}>
              Create cron job
            </Button>
          ) : undefined
        }
      />
      <div className="flex-1 overflow-hidden p-4">
        {error && !hasData && (
          <p className="mb-2 text-sm text-destructive">Failed to load cron jobs.</p>
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
