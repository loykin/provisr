import { X } from 'lucide-react'
import { PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { LifecycleHookList, LifecycleHookSummary } from '@/components/lifecycle-hooks'
import { DetailList, DetailRow, DetailSection, MonoValue } from '@/components/panel-detail'
import { StatusBadge } from '@/components/status-badge'
import { CronJobActions } from './CronJobActions'
import { useCronJob, useCronJobHistory } from './queries'

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : '-'
}

function formatNumber(value?: number): string {
  return typeof value === 'number' ? String(value) : '-'
}

function formatList(values?: string[]): string {
  return values && values.length > 0 ? values.join(', ') : '-'
}

export function CronJobDetailPanel({ name }: { name: string }) {
  const { close } = useSidePanel()
  const { data: job, error } = useCronJob(name, true)
  const { data: history } = useCronJobHistory(name, true)

  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )

  if (!job) {
    return (
      <PanelTemplate title="Loading…" actions={closeBtn}>
        <p className="text-sm text-muted-foreground">
          {error ? 'Failed to load cronjob.' : 'Loading…'}
        </p>
      </PanelTemplate>
    )
  }

  return (
    <PanelTemplate
      eyebrow="Cron job"
      title={name}
      status={<StatusBadge status={job.suspend ? 'suspended' : 'active'} />}
      actions={
        <div className="flex items-center gap-2">
          <CronJobActions job={job} />
          {closeBtn}
        </div>
      }
    >
      <DetailSection title="Details">
        <DetailList>
          <DetailRow label="Schedule">
            <MonoValue>{job.schedule}</MonoValue>
          </DetailRow>
          <DetailRow label="Concurrency policy">{job.concurrency_policy || 'Allow'}</DetailRow>
          <DetailRow label="Time zone">{job.time_zone || '-'}</DetailRow>
          <DetailRow label="Starting deadline">{formatNumber(job.starting_deadline_seconds)}</DetailRow>
          <DetailRow label="History limits">
            success {formatNumber(job.successful_jobs_history_limit)} / failed{' '}
            {formatNumber(job.failed_jobs_history_limit)}
          </DetailRow>
          <DetailRow label="Currently running">{job.status.active?.length ?? 0}</DetailRow>
          <DetailRow label="Last scheduled">{formatTime(job.status.last_schedule_time)}</DetailRow>
          <DetailRow label="Last successful">{formatTime(job.status.last_successful_time)}</DetailRow>
          <DetailRow label="Next run">{formatTime(job.next_schedule)}</DetailRow>
          <DetailRow label="Command">
            <MonoValue>{job.job_template.command || formatList(job.job_template.args)}</MonoValue>
          </DetailRow>
          <DetailRow label="Working directory">
            <MonoValue>{job.job_template.work_dir || '-'}</MonoValue>
          </DetailRow>
          <DetailRow label="Parallelism">{formatNumber(job.job_template.parallelism)}</DetailRow>
          <DetailRow label="Completions">{formatNumber(job.job_template.completions)}</DetailRow>
          <DetailRow label="Completion mode">{job.job_template.completion_mode || 'NonIndexed'}</DetailRow>
          <DetailRow label="Restart policy">{job.job_template.restart_policy || 'Never'}</DetailRow>
          <DetailRow label="Backoff limit">{formatNumber(job.job_template.backoff_limit)}</DetailRow>
          <DetailRow label="Active deadline">
            {formatNumber(job.job_template.active_deadline_seconds)}
          </DetailRow>
          <DetailRow label="TTL after finished">
            {formatNumber(job.job_template.ttl_seconds_after_finished)}
          </DetailRow>
          <DetailRow label="Depends on">{formatList(job.job_template.depends_on)}</DetailRow>
          <DetailRow label="CronJob hooks">
            <LifecycleHookSummary lifecycle={job.lifecycle} />
          </DetailRow>
          <DetailRow label="Job template hooks">
            <LifecycleHookSummary lifecycle={job.job_template.lifecycle} />
          </DetailRow>
        </DetailList>
      </DetailSection>

      <DetailSection title="CronJob lifecycle hooks">
        <LifecycleHookList lifecycle={job.lifecycle} />
      </DetailSection>

      <DetailSection title="Job template lifecycle hooks">
        <LifecycleHookList lifecycle={job.job_template.lifecycle} />
      </DetailSection>

      <DetailSection title="Job runs">
        <div className="rounded-(--radius) border border-border">
          {!history || history.length === 0 ? (
            <p className="p-3 text-sm text-muted-foreground">No runs yet.</p>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-left text-muted-foreground">
                <tr>
                  <th className="p-2 font-medium">Started</th>
                  <th className="p-2 font-medium">Status</th>
                  <th className="p-2 font-medium">Reason</th>
                </tr>
              </thead>
              <tbody>
                {[...history].reverse().map((entry) => (
                  <tr key={`${entry.Name}-${entry.StartTime}`} className="border-t border-border">
                    <td className="p-2">{formatTime(entry.StartTime)}</td>
                    <td className="p-2">{entry.Status}</td>
                    <td className="p-2 text-muted-foreground">{entry.Reason}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </DetailSection>
    </PanelTemplate>
  )
}
