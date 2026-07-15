import { createBrowserRouter, Link, Navigate, Outlet, RouterProvider } from 'react-router'
import { useQuery } from '@tanstack/react-query'

import { api, ApiError } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import { DashboardLayout } from '@/components/dashboard-layout'
import { LoginPage } from '@/features/auth/login-page'
import { GamesListPage } from '@/features/builder/games-list-page'
import { GameEditorPage } from '@/features/builder/game-editor-page'

interface Organizer {
  id: string
  username: string
}

// RequireAuth gates the authenticated branch on the session probe. The
// probe opts out of the api() hard-redirect and navigates in-router instead.
function RequireAuth() {
  const me = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: () => api<Organizer>('/api/auth/me', undefined, { on401: 'throw' }),
  })

  if (me.isPending) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-host-surface">
        <p className="text-host-text-secondary">{strings.common.loading}</p>
      </main>
    )
  }
  if (me.isError) {
    if (me.error instanceof ApiError && me.error.status === 401) {
      return <Navigate to="/login" replace />
    }
    return (
      <main className="flex min-h-svh items-center justify-center bg-host-surface">
        <p role="alert" className="text-host-text">
          {strings.common.connectionError}
        </p>
      </main>
    )
  }
  return <Outlet />
}

// Catch-all: an unmatched path must render Hebrew copy, not React Router's
// default English error boundary (the UI is Hebrew-only).
function NotFoundPage() {
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 bg-host-surface">
      <p className="text-host-text">{strings.notFound.message}</p>
      <Link to="/" className="text-green-800 underline">
        {strings.notFound.backHome}
      </Link>
    </main>
  )
}

// Data-mode router, but TanStack Query owns data fetching (architecture) —
// plain route elements, no loaders.
const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireAuth />,
    children: [
      {
        path: '/',
        element: <DashboardLayout />,
        children: [
          { index: true, element: <GamesListPage /> },
          { path: 'games/:gameId', element: <GameEditorPage /> },
        ],
      },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
])

export function App() {
  return <RouterProvider router={router} />
}
