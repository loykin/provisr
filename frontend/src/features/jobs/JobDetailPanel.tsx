import { X } from 'lucide-react'
import { PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { LifecycleHookList, LifecycleHookSummary } from '@/components/lifecycle-hooks'
import { DetailList, DetailRow, DetailSection, MonoValue } from '@/components/panel-detail'
import { JobActions } from './JobActions'
import { JobStateBadge } from './JobStateBadge'
import { useJob } from './queries'

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : '-'
}

function formatNumber(value?: number): string {
  return typeof value === 'number' ? String(value) : '-'
}

function formatList(values?: string[]): string {
  return values && values.length > 0 ? values.join(', ') : '-'
}

export function JobDetailPanel({ name }: { name: string }) {
  const { close } = useSidePanel()
  const { data: job, error } = useJob(name, true)

  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )

  if (!job) {
    return (
      <PanelTemplate title="Loading…" actions={closeBtn}>
        <p className="text-sm text-muted-foreground">
          {error ? 'Failed to load job.' : 'Loading…'}
        </p>
      </PanelTemplate>
    )
  }

  return (
    <PanelTemplate
      eyebrow="Job"
      title={name}
      status={<JobStateBadge phase={job.status.phase} />}
      actions={
        <div className="flex items-center gap-2">
          <JobActions job={job} />
          {closeBtn}
        </div>
      }
    >
      <DetailSection title="Details">
        <DetailList>
          <DetailRow label="Started">{formatTime(job.status.start_time)}</DetailRow>
          <DetailRow label="Completed">{formatTime(job.status.completion_time)}</DetailRow>
          <DetailRow label="Active">{job.status.active}</DetailRow>
          <DetailRow label="Succeeded">{job.status.succeeded}</DetailRow>
          <DetailRow label="Failed">{job.status.failed}</DetailRow>
          <DetailRow label="Command">
            <MonoValue>{job.command || formatList(job.args)}</MonoValue>
          </DetailRow>
          <DetailRow label="Working directory">
            <MonoValue>{job.work_dir || '-'}</MonoValue>
          </DetailRow>
          <DetailRow label="Parallelism">{formatNumber(job.parallelism)}</DetailRow>
          <DetailRow label="Completions">{formatNumber(job.completions)}</DetailRow>
          <DetailRow label="Completion mode">{job.completion_mode || 'NonIndexed'}</DetailRow>
          <DetailRow label="Restart policy">{job.restart_policy || 'Never'}</DetailRow>
          <DetailRow label="Backoff limit">{formatNumber(job.backoff_limit)}</DetailRow>
          <DetailRow label="Active deadline">{formatNumber(job.active_deadline_seconds)}</DetailRow>
          <DetailRow label="TTL after finished">{formatNumber(job.ttl_seconds_after_finished)}</DetailRow>
          <DetailRow label="Depends on">{formatList(job.depends_on)}</DetailRow>
          <DetailRow label="Hooks">
            <LifecycleHookSummary lifecycle={job.lifecycle} />
          </DetailRow>
        </DetailList>
      </DetailSection>

      <DetailSection title="Lifecycle hooks">
        <LifecycleHookList lifecycle={job.lifecycle} />
      </DetailSection>
    </PanelTemplate>
  )
}
