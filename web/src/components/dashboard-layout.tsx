import { NavLink, Outlet, useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

// Dashboard shell (UX-DR9): 240px sidebar on the logical start side (right
// in RTL), white Level-1 panel, slate main region. Shared by the builder now
// and lobby/live/results later, so it lives above features/. Below 1024px
// (lg) the sidebar collapses into a top bar — the same code path 200% zoom
// hits (WCAG 1.4.4).
export function DashboardLayout() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const logout = useMutation({
    mutationFn: () => api<void>('/api/auth/logout', { method: 'POST' }),
    onSuccess: () => {
      queryClient.clear()
      navigate('/login')
    },
  })

  return (
    <div className="flex min-h-svh flex-col bg-host-surface lg:flex-row">
      <aside className="flex w-full shrink-0 flex-row items-center justify-between gap-4 border-b border-host-border bg-surface-raised p-4 lg:w-60 lg:flex-col lg:items-stretch lg:border-b-0 lg:border-e lg:p-6">
        <p className="text-lg font-heading text-host-text">{strings.appTitle}</p>
        <nav className="flex flex-row items-center gap-4 lg:flex-1 lg:flex-col lg:items-stretch lg:gap-2">
          <NavLink
            to="/"
            end
            className={({ isActive }) =>
              cn(
                'flex h-10 items-center rounded-sm px-3 text-host-text hover:bg-host-surface',
                isActive && 'bg-host-surface font-ui',
              )
            }
          >
            {strings.nav.myGames}
          </NavLink>
        </nav>
        <div className="flex flex-col gap-2">
          {logout.isError && (
            <p role="alert" className="text-sm text-error">
              {strings.nav.logoutError}
            </p>
          )}
          <Button
            variant="outline"
            disabled={logout.isPending}
            onClick={() => logout.mutate()}
            className="h-10 border-host-border text-host-text"
          >
            {strings.nav.logout}
          </Button>
        </div>
      </aside>
      <main className="flex-1 p-6">
        <Outlet />
      </main>
    </div>
  )
}
