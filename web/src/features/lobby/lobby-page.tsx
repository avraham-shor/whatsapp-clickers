import { Link, useParams } from 'react-router'
import { useMutation } from '@tanstack/react-query'

import { api, ApiError } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import { useGameSocket } from '@/lib/use-game-socket'
import { Button } from '@/components/ui/button'

// Single data source: connect immediately on mount regardless of state
// (AC-3 doesn't gate the connection on being past draft) and render off the
// live snapshot alone — no separate REST game-fetch on this page.
export function LobbyPage() {
  const { gameId = '' } = useParams()
  const { snapshot, notFound: socketNotFound } = useGameSocket(gameId, 'host')

  const openLobby = useMutation({
    mutationFn: () => api<void>(`/api/games/${gameId}/open-lobby`, { method: 'POST' }),
  })
  const openLobbyNotFound = openLobby.error instanceof ApiError && openLobby.error.status === 404
  const openLobbyConflict = openLobby.error instanceof ApiError && openLobby.error.status === 409

  // socketNotFound covers the common case (a foreign/mistyped/deleted
  // gameId, classified via a REST probe once the WS handshake never opens
  // — see use-game-socket.ts). openLobbyNotFound covers the narrower race
  // where the game was deleted between page load and the click itself.
  if (socketNotFound || openLobbyNotFound) {
    return (
      <div className="flex flex-col items-start gap-4">
        <p className="text-host-text">{strings.gameEditor.notFound}</p>
        <Link to="/" className="text-green-800 underline">
          {strings.gameEditor.backToGames}
        </Link>
      </div>
    )
  }

  if (!snapshot) {
    return <p className="text-host-text-secondary">{strings.common.connecting}</p>
  }

  if (snapshot.state === 'draft') {
    return (
      <div className="flex flex-col items-start gap-4">
        {openLobby.isError && (
          <p role="alert" className="text-sm text-error">
            {/* 409 means the lobby is already open elsewhere — retrying
                the same action can never succeed, so it gets its own,
                accurate copy rather than the generic "try again". */}
            {openLobbyConflict ? strings.lobby.openLobbyAlreadyOpen : strings.lobby.openLobbyError}
          </p>
        )}
        <Button
          onClick={() => openLobby.mutate()}
          disabled={openLobby.isPending}
          className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900"
        >
          {strings.lobby.openLobbyCta}
        </Button>
      </div>
    )
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <header className="flex flex-col gap-2">
        <p className="text-host-text-secondary">
          {strings.gameEditor.joinCodeLabel}{' '}
          {/* Latin/digit LTR token inside RTL — bidi-isolate (DESIGN.md). */}
          <bdi dir="ltr" className="text-lg font-heading tracking-wide text-host-text">
            {snapshot.joinCode}
          </bdi>
        </p>
        <p className="text-host-text-secondary">
          {strings.lobby.platformNumberLabel}{' '}
          <bdi dir="ltr" className="text-lg font-heading tracking-wide text-host-text">
            {snapshot.platformNumber}
          </bdi>
        </p>
      </header>

      <p className="text-host-text">
        {strings.lobby.participantCountLabel(snapshot.participantCount)}
      </p>

      <ul className="flex flex-col gap-2">
        {snapshot.participants.map((participant) => (
          <li
            key={participant.id}
            className="rounded-md border border-host-border bg-surface-raised p-3 text-host-text"
          >
            {participant.displayName}
          </li>
        ))}
      </ul>
    </div>
  )
}
