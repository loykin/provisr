import type { ProcessStatus } from './types'

export function ProcessDetailBody({ status }: { status: ProcessStatus }) {
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

      <div className="mt-4 rounded-(--radius) border border-dashed border-border p-4 text-sm text-muted-foreground">
        Live stdout/stderr tailing isn&apos;t available yet — the backend SSE
        endpoint hasn&apos;t been built (see the &quot;live tail&quot; task in test.md).
      </div>
    </>
  )
}
