import { DataGrid } from '@loykin/gridkit'
import { PageTopBar } from '@loykin/designkit'
import { SidePanelProvider, useSidePanel } from '@loykin/side-panel'
import { useProcesses } from '@/features/processes/queries'
import { columns } from '@/features/processes/columns'
import { ProcessDetailPanel } from '@/features/processes/ProcessDetailPanel'

function ProcessesGrid() {
  const { open } = useSidePanel()
  const { data: rows, error } = useProcesses()

  return (
    <div className="flex h-full flex-col">
      <PageTopBar left="Processes" />
      <div className="flex-1 overflow-hidden p-4">
        {error && <p className="mb-2 text-sm text-destructive">Failed to load process list.</p>}
        <DataGrid
          data={rows ?? []}
          columns={columns}
          getRowId={(row) => row.name}
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
