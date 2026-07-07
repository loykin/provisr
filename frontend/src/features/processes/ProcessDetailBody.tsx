import { useEffect, useRef } from 'react'
import { useProcessLogs } from './queries'
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
            {line.text}
          </div>
        ))}
      </div>
    </div>
  )
}

export function ProcessDetailBody({ name, status }: { name: string; status: ProcessStatus }) {
  return (
    <>
      <div className="grid grid-cols-2 gap-4 rounded-(--radius) border border-border bg-card p-4 text-sm">
        <div>
          <div className="text-muted-foreground">PID</div>
          <div>{status.pid}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Restarts</div>
          <div>{status.restarts}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Started at</div>
          <div>{status.running ? new Date(status.started_at).toLocaleString() : '-'}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Detected by</div>
          <div>{status.detected_by || '-'}</div>
        </div>
      </div>

      <LogTail name={name} />
    </>
  )
}
