import { Pause, Pencil, Play, Trash2, Zap } from 'lucide-react'
import { useSidePanel } from '@loykin/side-panel'
import { IconAction } from '@/components/icon-action'
import { useAuth } from '@/features/auth/context'
import { canWriteWorkloads } from '@/features/auth/permissions'
import { CronJobEditPanel } from './CronJobFormPanel'
import { useDeleteCronJob, useResumeCronJob, useSuspendCronJob, useTriggerCronJob } from './queries'
import type { CronJobInfo } from './types'

// Shown on Suspend/Resume/Edit/Delete when the cronjob is declared in the
// main config file's [[processes]] array — the API refuses all four (see
// internal/server/router.go's isInlineConfiguredCronJob), so the buttons are
// disabled here rather than left to fail silently on click. "Run now" stays
// enabled: triggering doesn't rewrite the persisted definition.
const PROVISIONED_HINT = 'Defined in the main config file — edit the config and restart the daemon to change this.'

function showError(err: unknown, fallback: string) {
  window.alert(err instanceof Error ? err.message : fallback)
}

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
  const locked = job.provisioned === true

  function handleDelete() {
    if (window.confirm(`Delete cronjob "${job.name}"? This cannot be undone.`)) {
      del.mutate(job.name, { onError: (err) => showError(err, 'Failed to delete cronjob.') })
    }
  }

  return (
    <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
      {isSuspended ? (
        <IconAction
          label={locked ? PROVISIONED_HINT : 'Resume schedule'}
          disabled={pending || locked}
          onClick={() => resume.mutate(job.name, { onError: (err) => showError(err, 'Failed to resume cronjob.') })}
        >
          <Play className="h-3.5 w-3.5" />
        </IconAction>
      ) : (
        <IconAction
          label={locked ? PROVISIONED_HINT : 'Suspend schedule'}
          disabled={pending || locked}
          onClick={() => suspend.mutate(job.name, { onError: (err) => showError(err, 'Failed to suspend cronjob.') })}
        >
          <Pause className="h-3.5 w-3.5" />
        </IconAction>
      )}
      <IconAction
        label="Run now"
        disabled={pending}
        onClick={() => trigger.mutate(job.name, { onError: (err) => showError(err, 'Failed to trigger cronjob.') })}
      >
        <Zap className="h-3.5 w-3.5" />
      </IconAction>
      <IconAction
        label={locked ? PROVISIONED_HINT : 'Edit cronjob'}
        disabled={locked}
        onClick={() => open(<CronJobEditPanel name={job.name} />, { size: 480 })}
      >
        <Pencil className="h-3.5 w-3.5" />
      </IconAction>
      <IconAction
        label={locked ? PROVISIONED_HINT : 'Delete cronjob'}
        disabled={pending || locked}
        onClick={handleDelete}
      >
        <Trash2 className="h-3.5 w-3.5" />
      </IconAction>
    </div>
  )
}
