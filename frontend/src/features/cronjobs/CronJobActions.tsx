import { Pause, Pencil, Play, Trash2, Zap } from 'lucide-react'
import { useSidePanel } from '@loykin/side-panel'
import { IconAction } from '@/components/icon-action'
import { useAuth } from '@/features/auth/context'
import { canWriteWorkloads } from '@/features/auth/permissions'
import { CronJobEditPanel } from './CronJobFormPanel'
import { useDeleteCronJob, useResumeCronJob, useSuspendCronJob, useTriggerCronJob } from './queries'
import type { CronJobInfo } from './types'

// Mirror the server's cronjob:write policy; the server remains authoritative.
export function CronJobActions({ job }: { job: CronJobInfo }) {
  const { user } = useAuth()
  const { open } = useSidePanel()
  const suspend = useSuspendCronJob()
  const resume = useResumeCronJob()
  const trigger = useTriggerCronJob()
  const del = useDeleteCronJob()

  if (!canWriteWorkloads(user)) return null

  const pending = suspend.isPending || resume.isPending || trigger.isPending || del.isPending
  const isSuspended = job.suspend === true

  function handleDelete() {
    if (window.confirm(`Delete cronjob "${job.name}"? This cannot be undone.`)) {
      del.mutate(job.name)
    }
  }

  return (
    <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
      {isSuspended ? (
        <IconAction label="Resume schedule" disabled={pending} onClick={() => resume.mutate(job.name)}>
          <Play className="h-3.5 w-3.5" />
        </IconAction>
      ) : (
        <IconAction label="Suspend schedule" disabled={pending} onClick={() => suspend.mutate(job.name)}>
          <Pause className="h-3.5 w-3.5" />
        </IconAction>
      )}
      <IconAction label="Run now" disabled={pending} onClick={() => trigger.mutate(job.name)}>
        <Zap className="h-3.5 w-3.5" />
      </IconAction>
      <IconAction
        label="Edit cronjob"
        onClick={() => open(<CronJobEditPanel name={job.name} />, { size: 480 })}
      >
        <Pencil className="h-3.5 w-3.5" />
      </IconAction>
      <IconAction label="Delete cronjob" onClick={handleDelete}>
        <Trash2 className="h-3.5 w-3.5" />
      </IconAction>
    </div>
  )
}
