import { useEffect, useRef } from 'react'
import { LifecycleHookList, LifecycleHookSummary } from '@/components/lifecycle-hooks'
import { DetailList, DetailRow, DetailSection, MonoValue } from '@/components/panel-detail'
import { useProcessLogs, useProcessSpec } from './queries'
import { renderAnsiLine } from '@/lib/ansi'
import type { ProcessStatus } from './types'
import { ProcessObservability } from './ProcessObservability'

function LogTail({ name }: { name: string }) {
  const { data: lines, error } = useProcessLogs(name, true)
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [lines])

  return (
    <DetailSection title="Live output">
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
    </DetailSection>
  )
}

export function ProcessDetailBody({ name, status }: { name: string; status: ProcessStatus }) {
  const { data: spec } = useProcessSpec(name, true)

  return (
    <>
      <DetailSection title="Details">
        <DetailList>
          <DetailRow label="PID">{status.pid}</DetailRow>
          <DetailRow label="Restarts">{status.restarts}</DetailRow>
          <DetailRow label="Started at">
            {status.running ? new Date(status.started_at).toLocaleString() : '-'}
          </DetailRow>
          <DetailRow label="Detected by">{status.detected_by || '-'}</DetailRow>
          {spec && (
            <>
              <DetailRow label="Command">
                <MonoValue>{spec.command || (spec.args ?? []).join(' ') || '-'}</MonoValue>
              </DetailRow>
              <DetailRow label="Working directory">
                <MonoValue>{spec.work_dir || '-'}</MonoValue>
              </DetailRow>
              <DetailRow label="Instances">{spec.instances ?? 1}</DetailRow>
              <DetailRow label="Auto-restart">
                {spec.auto_restart ? 'Enabled' : 'Disabled'}
              </DetailRow>
              <DetailRow label="Priority">{spec.priority ?? 0}</DetailRow>
              <DetailRow label="PID file">
                <MonoValue>{spec.pid_file || '-'}</MonoValue>
              </DetailRow>
              <DetailRow label="Hooks">
                <LifecycleHookSummary lifecycle={spec.lifecycle} />
              </DetailRow>
            </>
          )}
        </DetailList>
      </DetailSection>

      {spec && (
        <DetailSection title="Lifecycle hooks">
          <LifecycleHookList lifecycle={spec.lifecycle} />
        </DetailSection>
      )}

      <ProcessObservability name={name} running={status.running} />

      <LogTail name={name} />
    </>
  )
}
