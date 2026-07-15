import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import type { Game, GameList } from '@/lib/types'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const dateFormat = new Intl.DateTimeFormat('he-IL', { dateStyle: 'medium' })

export function GamesListPage() {
  const games = useQuery({
    queryKey: ['games'],
    queryFn: () => api<GameList>('/api/games'),
  })

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <header className="flex items-center justify-between">
        <h1 className="text-2xl font-heading text-host-text">
          {strings.gamesList.title}
        </h1>
        <CreateGameDialog />
      </header>

      {games.isPending && (
        <p className="text-host-text-secondary">{strings.common.loading}</p>
      )}
      {games.isError && (
        <p role="alert" className="text-host-text">
          {strings.gamesList.loadError}
        </p>
      )}
      {games.isSuccess && games.data.items.length === 0 && (
        <div className="flex flex-col items-start gap-2 rounded-md border border-host-border bg-surface-raised p-6">
          <p className="text-host-text">{strings.gamesList.emptyStateTitle}</p>
          <p className="text-host-text-secondary">
            {strings.gamesList.emptyStateBody}
          </p>
        </div>
      )}
      {games.isSuccess && games.data.items.length > 0 && (
        <ul className="flex flex-col gap-4">
          {games.data.items.map((game) => (
            <li key={game.id}>
              <Link
                to={`/games/${game.id}`}
                className="flex flex-col gap-2 rounded-md border border-host-border bg-surface-raised p-6 hover:border-green-800"
              >
                <div className="flex items-center justify-between gap-4">
                  <span className="text-lg font-ui text-host-text">
                    {game.title}
                  </span>
                  <span className="text-host-text-secondary">
                    {strings.gamesList.joinCodeLabel}{' '}
                    {/* JOIN Code is a Latin/digit LTR token inside RTL —
                        isolate it or it visually scrambles (DESIGN.md bidi). */}
                    <bdi dir="ltr" className="font-ui text-host-text">
                      {game.joinCode}
                    </bdi>
                  </span>
                </div>
                <div className="flex items-center gap-4 text-sm text-host-text-secondary">
                  <span>{strings.gamesList.questionsCount(game.questionCount)}</span>
                  <span>{dateFormat.format(new Date(game.createdAt))}</span>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function CreateGameDialog() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [validationError, setValidationError] = useState(false)

  const createGame = useMutation({
    mutationFn: (gameTitle: string) =>
      api<Game>('/api/games', {
        method: 'POST',
        body: JSON.stringify({ title: gameTitle }),
      }),
    onSuccess: (game) => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      navigate(`/games/${game.id}`)
    },
  })

  const errorText = validationError
    ? strings.createGame.validationTitle
    : createGame.isError
      ? strings.createGame.errorServer
      : undefined

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (next) {
          setTitle('')
          setValidationError(false)
          createGame.reset()
        }
      }}
    >
      <DialogTrigger asChild>
        <Button className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900">
          {strings.gamesList.createGame}
        </Button>
      </DialogTrigger>
      <DialogContent className="rounded-xl">
        <DialogHeader>
          <DialogTitle className="font-heading text-host-text">
            {strings.createGame.dialogTitle}
          </DialogTitle>
        </DialogHeader>
        <form
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            const trimmed = title.trim()
            if (trimmed.length < 1 || trimmed.length > 120) {
              setValidationError(true)
              return
            }
            setValidationError(false)
            createGame.mutate(trimmed)
          }}
        >
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-game-title" className="text-host-text">
              {strings.createGame.titleLabel}
            </Label>
            <Input
              id="create-game-title"
              name="title"
              required
              className="h-10"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              aria-describedby={errorText ? 'create-game-error' : undefined}
            />
          </div>
          {errorText && (
            <p id="create-game-error" role="alert" className="text-sm text-error">
              {errorText}
            </p>
          )}
          <Button
            type="submit"
            disabled={createGame.isPending}
            className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900"
          >
            {strings.createGame.submit}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
