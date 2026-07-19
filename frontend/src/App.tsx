import { lazy, Suspense } from 'react'
import {
  createRootRoute,
  createRoute,
  createRouter,
  Navigate,
  Outlet,
  RouterProvider,
  useNavigate,
  useRouterState,
} from '@tanstack/react-router'
import { Boxes, History, Settings, Users } from 'lucide-react'
import { AuthProvider, useAuth } from '@/features/auth/context'
import { Button } from '@/components/ui/button'
import {
  SidebarProvider,
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarInset,
  SidebarRail,
  SidebarTrigger,
} from '@/components/ui/sidebar'
import { TooltipProvider } from '@/components/ui/tooltip'

const LoginPage = lazy(() => import('@/pages/LoginPage'))
const ProcessesPage = lazy(() => import('@/pages/ProcessesPage'))
const ProcessRegisterPage = lazy(() => import('@/pages/ProcessRegisterPage'))
const ProcessDetailPage = lazy(() => import('@/pages/ProcessDetailPage'))
const HistoryPage = lazy(() => import('@/pages/HistoryPage'))
const JobsPage = lazy(() => import('@/pages/JobsPage'))
const JobRegisterPage = lazy(() => import('@/pages/JobRegisterPage'))
const CronJobsPage = lazy(() => import('@/pages/CronJobsPage'))
const CronJobRegisterPage = lazy(() => import('@/pages/CronJobRegisterPage'))
const GroupsPage = lazy(() => import('@/pages/GroupsPage'))
const UsersPage = lazy(() => import('@/pages/UsersPage'))
const UserRegisterPage = lazy(() => import('@/pages/UserRegisterPage'))
const UserEditPage = lazy(() => import('@/pages/UserEditPage'))
const SettingsPage = lazy(() => import('@/pages/SettingsPage'))

const navItems = [
  { id: 'workloads', label: 'Workloads', icon: Boxes, to: '/processes', match: ['/processes', '/jobs', '/cronjobs', '/groups'] },
  { id: 'history', label: 'History', icon: History, to: '/history' },
  { id: 'users', label: 'Users', icon: Users, to: '/users', adminOnly: true, requiresAuth: true },
  { id: 'settings', label: 'Settings', icon: Settings, to: '/settings', adminOnly: true },
]

function AppSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const navigate = useNavigate()
  const { user, authEnabled, logout } = useAuth()
  const isAdmin = user?.roles.includes('admin') ?? false

  return (
    <>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>provisr</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {navItems
                .filter((item) => (!item.requiresAuth || authEnabled) && (!item.adminOnly || isAdmin))
                .map((item) => (
                  <SidebarMenuItem key={item.id}>
                    <SidebarMenuButton
                      isActive={
                        pathname === item.to ||
                        pathname.startsWith(item.to + '/') ||
                        item.match?.some((prefix) => pathname === prefix || pathname.startsWith(prefix + '/'))
                      }
                      onClick={() => void navigate({ to: item.to })}
                    >
                      <item.icon />
                      <span>{item.label}</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem className="flex items-center justify-between px-2 py-1 text-sm">
            <span className="text-muted-foreground">
              {authEnabled ? user?.username : 'auth disabled'}
            </span>
            {authEnabled && (
              <Button variant="ghost" size="sm" onClick={logout}>
                Sign out
              </Button>
            )}
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </>
  )
}

function RoutedContent() {
  const { user } = useAuth()
  const pathname = useRouterState({ select: (s) => s.location.pathname })

  // undefined: the initial /auth/status check hasn't resolved yet — avoid
  // flashing the login screen for users who won't need one.
  if (user === undefined) {
    return <div className="p-8 text-center text-sm text-muted-foreground">Loading…</div>
  }
  if (user === null) {
    if (pathname !== '/login') return <Navigate to="/login" replace />
    return <Outlet />
  }
  if (pathname === '/login' || pathname === '/') {
    return <Navigate to="/processes" replace />
  }

  return (
    <SidebarProvider className="h-screen">
      <Sidebar>
        <AppSidebar />
        <SidebarRail />
      </Sidebar>
      <SidebarInset>
        <div className="flex h-full flex-col overflow-hidden">
          <div className="shrink-0 border-b p-2 md:hidden">
            <SidebarTrigger />
          </div>
          <div className="min-h-0 flex-1">
            <Outlet />
          </div>
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}

function AppLayout() {
  return (
    <AuthProvider>
      <TooltipProvider>
        <Suspense fallback={<div className="p-8 text-center text-sm text-muted-foreground">Loading…</div>}>
          <RoutedContent />
        </Suspense>
      </TooltipProvider>
    </AuthProvider>
  )
}

const rootRoute = createRootRoute({ component: AppLayout })

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: () => null,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'login',
  component: LoginPage,
})

const processesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'processes',
  component: ProcessesPage,
})

const processRegisterRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'processes/new',
  component: ProcessRegisterPage,
})

const processDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'processes/$name',
  component: ProcessDetailPage,
})

const historyRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'history',
  component: HistoryPage,
})

const jobsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'jobs',
  component: JobsPage,
})

const jobsRegisterRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'jobs/new',
  component: JobRegisterPage,
})

const cronJobsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'cronjobs',
  component: CronJobsPage,
})

const cronJobsRegisterRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'cronjobs/new',
  component: CronJobRegisterPage,
})

const groupsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'groups',
  component: GroupsPage,
})

const usersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'users',
  component: UsersPage,
})

const userRegisterRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'users/new',
  component: UserRegisterPage,
})

const userEditRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'users/$id/edit',
  component: UserEditPage,
})

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'settings',
  component: SettingsPage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  processesRoute,
  processRegisterRoute,
  processDetailRoute,
  historyRoute,
  jobsRoute,
  jobsRegisterRoute,
  cronJobsRoute,
  cronJobsRegisterRoute,
  groupsRoute,
  usersRoute,
  userRegisterRoute,
  userEditRoute,
  settingsRoute,
])

const router = createRouter({ routeTree, basepath: '/ui' })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

export default function App() {
  return <RouterProvider router={router} />
}
