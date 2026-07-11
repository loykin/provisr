import { Pencil, Trash2 } from 'lucide-react'
import { useSidePanel } from '@loykin/side-panel'
import { IconAction } from '@/components/icon-action'
import { useAuth } from '@/features/auth/context'
import { UserEditPanel } from './UserFormPanel'
import { useDeleteUser } from './queries'
import type { User } from './types'

// Same admin-only gating rationale as ProcessActions/CronJobActions — the
// server has no permission entries for the user/client resources yet (see
// internal/auth/service.go HasPermission), so this is a UI-only guard.
export function UserActions({ user: target }: { user: User }) {
  const { user } = useAuth()
  const { open } = useSidePanel()
  const del = useDeleteUser()

  if (!user?.roles.includes('admin')) return null

  function handleDelete() {
    if (window.confirm(`Delete user "${target.username}"? This cannot be undone.`)) {
      del.mutate(target.id)
    }
  }

  return (
    <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
      <IconAction label="Edit user" onClick={() => open(<UserEditPanel user={target} />, { size: 480 })}>
        <Pencil className="h-3.5 w-3.5" />
      </IconAction>
      <IconAction label="Delete user" disabled={del.isPending} onClick={handleDelete}>
        <Trash2 className="h-3.5 w-3.5" />
      </IconAction>
    </div>
  )
}
