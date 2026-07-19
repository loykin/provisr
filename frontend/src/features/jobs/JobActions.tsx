import { Pencil, Trash2 } from 'lucide-react'
import { useSidePanel } from '@loykin/side-panel'
import { IconAction } from '@/components/icon-action'
import { useAuth } from '@/features/auth/context'
import { canWriteWorkloads } from '@/features/auth/permissions'
import { JobEditPanel } from './JobFormPanel'
import { useDeleteJob } from './queries'
import type { JobInfo } from './types'

export function JobActions({ job }: { job: JobInfo }) {
  const { user } = useAuth()
  const { open } = useSidePanel()
  const del = useDeleteJob()

  if (!canWriteWorkloads(user)) return null

  function handleDelete() {
    if (window.confirm(`Delete job "${job.name}"? This will stop active processes.`)) {
      del.mutate(job.name)
    }
  }

  return (
    <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
      <IconAction label="Edit job" onClick={() => open(<JobEditPanel name={job.name} />, { size: 480 })}>
        <Pencil className="h-3.5 w-3.5" />
      </IconAction>
      <IconAction label="Delete job" onClick={handleDelete} disabled={del.isPending}>
        <Trash2 className="h-3.5 w-3.5" />
      </IconAction>
    </div>
  )
}
