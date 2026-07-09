import { X } from 'lucide-react'
import { PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { useCronJob, useCronJobHistory } from './queries'

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : '-'
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
    <PanelTemplate eyebrow="Cron job" title={name} actions={closeBtn}>
      <div className="grid grid-cols-2 gap-4 rounded-(--radius) border border-border bg-card p-4 text-sm">
        <div>
          <div className="text-muted-foreground">Schedule</div>
          <div className="font-mono">{job.schedule}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Status</div>
          <div>{job.suspend ? 'Suspended' : 'Active'}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Concurrency policy</div>
          <div>{job.concurrency_policy || 'Allow'}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Currently running</div>
          <div>{job.status.active?.length ?? 0}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Last scheduled</div>
          <div>{formatTime(job.status.last_schedule_time)}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Last successful</div>
          <div>{formatTime(job.status.last_successful_time)}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Next run</div>
          <div>{formatTime(job.next_schedule)}</div>
        </div>
        <div>
          <div className="text-muted-foreground">Command</div>
          <div className="font-mono">{job.job_template.command}</div>
        </div>
      </div>

      <div className="mt-4">
        <div className="mb-1 text-sm font-medium text-muted-foreground">Recent runs</div>
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
      </div>
    </PanelTemplate>
  )
}
