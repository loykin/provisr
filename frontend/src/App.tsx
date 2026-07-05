import { lazy, Suspense } from 'react'
import {
  createRootRoute,
  createRoute,
  createRouter,
  Navigate,
  Outlet,
  RouterProvider,
  useRouterState,
} from '@tanstack/react-router'
import { AuthProvider, useAuth } from '@/features/auth/context'
import { Button } from '@/components/ui/button'

const LoginPage = lazy(() => import('@/pages/LoginPage'))
const ProcessesPage = lazy(() => import('@/pages/ProcessesPage'))
const ProcessDetailPage = lazy(() => import('@/pages/ProcessDetailPage'))

function RoutedContent() {
  const { user } = useAuth()
  const pathname = useRouterState({ select: (s) => s.location.pathname })

  if (!user) {
    if (pathname !== '/login') return <Navigate to="/login" replace />
  } else if (pathname === '/login' || pathname === '/') {
    return <Navigate to="/processes" replace />
  }
  return <Outlet />
}

function TopBar() {
  const { user, logout } = useAuth()
  if (!user) return null
  return (
    <div className="flex items-center justify-end gap-3 border-b border-border px-4 py-2 text-sm">
      <span className="text-muted-foreground">{user.username}</span>
      <Button variant="ghost" size="sm" onClick={logout}>
        Sign out
      </Button>
    </div>
  )
}

function AppLayout() {
  return (
    <AuthProvider>
      <div className="flex h-screen flex-col">
        <TopBar />
        <div className="flex-1 overflow-hidden">
          <Suspense fallback={<div className="p-8 text-center text-sm text-muted-foreground">Loading…</div>}>
            <RoutedContent />
          </Suspense>
        </div>
      </div>
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

const routeTree = rootRoute.addChildren([indexRoute, loginRoute, processesRoute, processDetailRoute])

export const router = createRouter({ routeTree, basepath: '/ui' })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

export default function App() {
  return <RouterProvider router={router} />
}
