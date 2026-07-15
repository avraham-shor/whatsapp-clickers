import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useMutation } from '@tanstack/react-query'

import { api, ApiError } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface Organizer {
  id: string
  username: string
}

export function LoginPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  const login = useMutation({
    mutationFn: () =>
      api<Organizer>('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      }),
    onSuccess: () => navigate('/'),
  })

  const errorText = login.isError
    ? login.error instanceof ApiError && login.error.status === 401
      ? strings.login.errorInvalidCredentials
      : strings.login.errorServer
    : undefined

  return (
    <main className="flex min-h-svh items-center justify-center bg-host-surface p-4">
      {/* Sign-in card: 12px radius, 1px host border, no shadow (elevation
          rules: shadows are modal-only on the dashboard), 24px padding. */}
      <Card className="w-full max-w-sm rounded-md border border-host-border bg-surface-raised shadow-none ring-0 [--card-spacing:--spacing(6)]">
        <CardHeader>
          <CardTitle className="text-xl font-heading text-host-text">
            {strings.login.title}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form
            className="flex flex-col gap-4"
            aria-describedby={errorText ? 'login-error' : undefined}
            onSubmit={(event) => {
              event.preventDefault()
              login.mutate()
            }}
          >
            <div className="flex flex-col gap-2">
              <Label htmlFor="login-username" className="text-host-text">
                {strings.login.usernameLabel}
              </Label>
              <Input
                id="login-username"
                name="username"
                autoComplete="username"
                required
                className="h-10"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                aria-describedby={errorText ? 'login-error' : undefined}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="login-password" className="text-host-text">
                {strings.login.passwordLabel}
              </Label>
              <Input
                id="login-password"
                name="password"
                type="password"
                autoComplete="current-password"
                required
                className="h-10"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                aria-describedby={errorText ? 'login-error' : undefined}
              />
            </div>
            {errorText && (
              <p id="login-error" role="alert" className="text-sm text-error">
                {errorText}
              </p>
            )}
            <Button
              type="submit"
              disabled={login.isPending}
              className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900"
            >
              {strings.login.submit}
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  )
}
