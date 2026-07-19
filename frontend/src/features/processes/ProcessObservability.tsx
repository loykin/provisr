import { DetailList, DetailRow, DetailSection } from '@/components/panel-detail'
import { useProcessDiagnostics, useProcessMetrics, useProcessMetricsHistory } from './queries'

function Sparkline({ values }: { values: number[] }) {
  if (values.length < 2) return <span className="text-xs text-muted-foreground">Collecting history…</span>
  const width = 300
  const height = 64
  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  const points = values.map((value, index) => {
    const x = (index / (values.length - 1)) * width
    const y = height - ((value - min) / range) * (height - 8) - 4
    return `${x},${y}`
  }).join(' ')
  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="h-16 w-full" role="img" aria-label="Recent metric history">
      <polyline points={points} fill="none" stroke="currentColor" strokeWidth="2" className="text-primary" />
    </svg>
  )
}

export function ProcessObservability({ name, running }: { name: string; running: boolean }) {
  const metrics = useProcessMetrics(name, running)
  const history = useProcessMetricsHistory(name, running)
  const diagnostics = useProcessDiagnostics(name, true)
  const metricHistory = history.data?.history ?? []

  return (
    <>
      <DetailSection title="Metrics">
        {!running && <p className="text-sm text-muted-foreground">Metrics are available while the process is running.</p>}
        {running && metrics.error && <p className="text-sm text-muted-foreground">Process metrics are disabled or not available yet.</p>}
        {metrics.data && (
          <>
            <DetailList>
              <DetailRow label="CPU">{metrics.data.cpu_percent.toFixed(1)}%</DetailRow>
              <DetailRow label="Memory">{metrics.data.memory_mb.toFixed(1)} MB</DetailRow>
              <DetailRow label="Threads">{metrics.data.num_threads}</DetailRow>
              <DetailRow label="File descriptors">{metrics.data.num_fds ?? '-'}</DetailRow>
            </DetailList>
            <div className="rounded-(--radius) border border-border p-3">
              <div className="mb-1 flex justify-between text-xs text-muted-foreground"><span>Recent CPU</span><span>{metricHistory.length} samples</span></div>
              <Sparkline values={metricHistory.map((item) => item.cpu_percent)} />
            </div>
          </>
        )}
      </DetailSection>
      <DetailSection title="Diagnostics">
        {diagnostics.error && <p className="text-sm text-destructive">Failed to load diagnostics.</p>}
        {diagnostics.data ? (
          <DetailList>
            <DetailRow label="Internal state">{diagnostics.data.internal_state || '-'}</DetailRow>
            <DetailRow label="Health">{diagnostics.data.health_check || '-'}</DetailRow>
            <DetailRow label="Detected by">{diagnostics.data.status.detected_by || '-'}</DetailRow>
          </DetailList>
        ) : !diagnostics.error ? <p className="text-sm text-muted-foreground">Loading…</p> : null}
      </DetailSection>
    </>
  )
}
