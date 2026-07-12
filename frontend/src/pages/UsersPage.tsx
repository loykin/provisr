import { Navigate, useNavigate } from '@tanstack/react-router'
import { DataGrid } from '@loykin/gridkit'
import { DataPage, PageBreadcrumb } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/features/auth/context'
import { useUsers } from '@/features/users/queries'
import { userColumns } from '@/features/users/columns'

function UsersGrid() {
  const navigate = useNavigate()
  const { data } = useUsers()
  const rows = data?.users ?? []

  return (
    <DataGrid
      data={rows}
      columns={userColumns}
      getRowId={(row) => row.id}
      initialSorting={[{ id: 'username', desc: false }]}
      onRowClick={(row) => void navigate({ to: '/users/$id/edit', params: { id: row.id } })}
      classNames={{ root: 'shrink-0' }}
    />
  )
}

function UsersPageBody() {
  const navigate = useNavigate()
  const { user } = useAuth()
  const isAdmin = user?.roles.includes('admin') ?? false

  const { error: usersError } = useUsers()

  return (
    <div className="flex h-full flex-col">
      <DataPage.Header>
        <DataPage.TitleBlock
          title="Users"
          breadcrumb={<PageBreadcrumb items={['provisr', 'Users']} />}
        />
        <DataPage.Actions>
          {isAdmin && (
            <Button size="sm" onClick={() => void navigate({ to: '/users/new' })}>
              Create user
            </Button>
          )}
        </DataPage.Actions>
      </DataPage.Header>
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto overflow-x-hidden p-4">
        {usersError && <p className="mb-2 text-sm text-destructive">Failed to load users.</p>}
        <UsersGrid />
      </div>
    </div>
  )
}

export default function UsersPage() {
  const { authEnabled } = useAuth()

  // No auth service means no /auth/users routes on the backend either (see
  // router.go's authService != nil gate) — there's no user/role concept to
  // manage in that mode, so bounce back rather than rendering a page whose
  // every request 404s.
  if (!authEnabled) {
    return <Navigate to="/processes" replace />
  }

  return <UsersPageBody />
}
