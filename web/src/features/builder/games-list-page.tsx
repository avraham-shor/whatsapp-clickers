import { useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import { Button } from '@/components/ui/button'

// Minimal authenticated landing — the real games CRUD arrives in Story 1.3.
export function GamesListPage() {
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
    <main className="min-h-svh bg-host-surface p-8">
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
        <header className="flex items-center justify-between">
          <h1 className="text-2xl font-heading text-host-text">
            {strings.gamesList.title}
          </h1>
          <Button
            variant="outline"
            disabled={logout.isPending}
            onClick={() => logout.mutate()}
            className="h-10 border-host-border text-host-text"
          >
            {strings.gamesList.logout}
          </Button>
        </header>
        {logout.isError && (
          <p role="alert" className="text-sm text-error">
            {strings.gamesList.logoutError}
          </p>
        )}
        <p className="text-host-text-secondary">{strings.gamesList.emptyState}</p>
      </div>
    </main>
  )
}
