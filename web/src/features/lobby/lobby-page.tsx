import { useCallback, useRef } from 'react'
import { Link, useParams } from 'react-router'
import { useMutation } from '@tanstack/react-query'

import { api, ApiError } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import { useGameSocket } from '@/lib/use-game-socket'
import { useSpaceAction } from '@/lib/use-space-action'
import { Button } from '@/components/ui/button'
import { ControlPage } from '@/features/live/control-page'
// Same pre-existing cross-feature exception as ControlPage above, unchanged
// in kind: the display controls are host-side controls, so they live in
// features/live/ (features/display/ is output-only by the architecture's
// own boundary rule) and both host surfaces render the one component.
import { DisplayControls } from '@/features/live/display-controls'

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

  const startGame = useMutation({
    mutationFn: () => api<void>(`/api/games/${gameId}/start`, { method: 'POST' }),
  })
  const startGameNoQuestions =
    startGame.error instanceof ApiError && startGame.error.code === 'GAME_NO_QUESTIONS'
  const startGameConflict =
    startGame.error instanceof ApiError && startGame.error.status === 409 && !startGameNoQuestions

  // Same ref-guard shape as ControlPage's fire(): a plain ref instead of
  // startGame.isPending, which React Query doesn't flip synchronously
  // inside mutate() — a rapid double-press (or Space racing a click) could
  // otherwise read a stale, still-false isPending and fire twice.
  const startSubmittingRef = useRef(false)
  const fireStart = useCallback(() => {
    if (startSubmittingRef.current) return
    startSubmittingRef.current = true
    startGame.mutate(undefined, { onSettled: () => { startSubmittingRef.current = false } })
  }, [startGame])
  useSpaceAction(fireStart, snapshot?.state === 'lobby' && snapshot.questionCount > 0)

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

  // Anything past lobby has its own single-page live control surface,
  // sharing this component's one useGameSocket subscription rather than
  // opening a second one inside ControlPage.
  if (snapshot.state !== 'lobby') {
    return <ControlPage gameId={gameId} snapshot={snapshot} />
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

      {startGame.isError && (
        <p role="alert" className="text-sm text-error">
          {startGameNoQuestions
            ? strings.live.startGameNoQuestions
            : startGameConflict
              ? strings.live.actionConflict
              : strings.live.actionError}
        </p>
      )}
      {/* Not disabled while pending — a disabled button loses DOM focus in
          every browser, breaking the Space/click affordance the instant a
          click starts (same fix as ControlPage's persistent button);
          fireStart's ref guards the double-submit instead. Disabled only
          for a real, non-transient reason: no questions to start with. */}
      <Button
        onClick={fireStart}
        disabled={snapshot.questionCount === 0}
        className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900"
      >
        {strings.live.startGameCta}
      </Button>

      {/* Lobby branch only: EXPERIENCE.md's State Patterns puts the display
          at "— (not yet launched)" pre-lobby, and its Host-control-panel
          table makes the launch CTA available from Lobby through Game over.
          Optional-chained despite the non-optional type: a redeploy briefly
          runs two instances (store/migrate.go), so this socket can be served
          a snapshot built before the field existed, and there is no error
          boundary to catch the deref. (Code review, 2026-08-09.) */}
      <DisplayControls
        gameId={gameId}
        reducedMotion={snapshot.displaySettings?.reducedMotion ?? false}
        gameState={snapshot.state}
      />

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
