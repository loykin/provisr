import { useEffect, useRef } from 'react'
import { DataBodyTemplate } from '@loykin/designkit'
import { useProcessLogs } from './queries'
import { renderAnsiLine } from '@/lib/ansi'
import type { ProcessStatus } from './types'

function LogTail({ name }: { name: string }) {
  const { data: lines, error } = useProcessLogs(name, true)
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [lines])

  return (
    <div className="mt-4">
      <div className="mb-1 text-sm font-medium text-muted-foreground">Live output</div>
      <div
        ref={scrollRef}
        className="h-64 overflow-y-auto rounded-(--radius) border border-border bg-black p-3 font-mono text-xs text-neutral-200"
      >
        {error && <p className="text-destructive">Failed to load logs.</p>}
        {!error && (!lines || lines.length === 0) && (
          <p className="text-neutral-500">No output yet.</p>
        )}
        {lines?.map((line) => (
          <div key={line.offset} className={line.stream === 'stderr' ? 'text-red-400' : undefined}>
            {renderAnsiLine(line.text)}
          </div>
        ))}
      </div>
    </div>
  )
}

export function ProcessDetailBody({ name, status }: { name: string; status: ProcessStatus }) {
  return (
    <>
      <DataBodyTemplate.Group layout="stacked">
        <DataBodyTemplate.Field label="PID">{status.pid}</DataBodyTemplate.Field>
        <DataBodyTemplate.Field label="Restarts">{status.restarts}</DataBodyTemplate.Field>
        <DataBodyTemplate.Field label="Started at">
          {status.running ? new Date(status.started_at).toLocaleString() : '-'}
        </DataBodyTemplate.Field>
        <DataBodyTemplate.Field label="Detected by">{status.detected_by || '-'}</DataBodyTemplate.Field>
      </DataBodyTemplate.Group>

      <LogTail name={name} />
    </>
  )
}
