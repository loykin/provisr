import { useEffect, useState } from 'react'
import { DataGrid } from '@loykin/gridkit'
import { PageTopBar } from '@loykin/designkit'
import { SidePanelProvider, useSidePanel } from '@loykin/side-panel'
import { listProcesses } from '@/features/processes/api'
import { columns } from '@/features/processes/columns'
import { ProcessDetailPanel } from '@/features/processes/ProcessDetailPanel'
import type { ProcessStatus } from '@/features/processes/types'

const POLL_INTERVAL_MS = 3000

function ProcessesGrid() {
  const { open } = useSidePanel()
  const [rows, setRows] = useState<ProcessStatus[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const data = await listProcesses()
        if (!cancelled) {
          setRows(data)
          setError(null)
        }
      } catch {
        if (!cancelled) setError('Failed to load process list.')
      }
    }
    void load()
    const id = setInterval(load, POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [])

  return (
    <div className="flex h-full flex-col">
      <PageTopBar left="Processes" />
      <div className="flex-1 overflow-hidden p-4">
        {error && <p className="mb-2 text-sm text-destructive">{error}</p>}
        <DataGrid
          data={rows}
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
