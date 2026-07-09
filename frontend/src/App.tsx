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
import { CalendarClock, History, Server } from 'lucide-react'
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
const ProcessDetailPage = lazy(() => import('@/pages/ProcessDetailPage'))
const HistoryPage = lazy(() => import('@/pages/HistoryPage'))
const JobsPage = lazy(() => import('@/pages/JobsPage'))

const navItems = [
  { id: 'processes', label: 'Processes', icon: Server, to: '/processes' },
  { id: 'jobs', label: 'Jobs', icon: CalendarClock, to: '/jobs' },
  { id: 'history', label: 'History', icon: History, to: '/history' },
]

function AppSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const navigate = useNavigate()
  const { user, logout } = useAuth()

  return (
    <>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>provisr</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {navItems.map((item) => (
                <SidebarMenuItem key={item.id}>
                  <SidebarMenuButton
                    isActive={pathname === item.to || pathname.startsWith(item.to + '/')}
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
            <span className="text-muted-foreground">{user?.username}</span>
            <Button variant="ghost" size="sm" onClick={logout}>
              Sign out
            </Button>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </>
  )
}

function RoutedContent() {
  const { user } = useAuth()
  const pathname = useRouterState({ select: (s) => s.location.pathname })

  if (!user) {
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

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  processesRoute,
  processDetailRoute,
  historyRoute,
  jobsRoute,
])

export const router = createRouter({ routeTree, basepath: '/ui' })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

export default function App() {
  return <RouterProvider router={router} />
}
