import { useEffect, useRef } from 'react'
import { Link } from 'react-router'
import { useMutation } from '@tanstack/react-query'

import { api, ApiError } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import { useSingleFlight } from '@/lib/use-single-flight'
import { useSpaceAction } from '@/lib/use-space-action'
import type { LobbySnapshot } from '@/lib/types'
import { Button } from '@/components/ui/button'
// Cross-feature import, the established exception rather than a new one:
// lobby-page.tsx already imports ControlPage from features/live/ for the
// same reason — one live surface composes the next as the game advances.
// Duplicating the summary or promoting it to lib/ would both be worse (it
// is a feature view, not shared logic).
import { ResultsSummary } from '@/features/results/results-summary'
import { ResponseStats } from './response-stats'
import { DisplayControls } from './display-controls'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

interface ControlPageProps {
  gameId: string
  snapshot: LobbySnapshot
}

interface PrimaryAction {
  label: string
  path: string
}

// The one CTA per state this story reaches — "revealed"'s "שאלה הבאה" is
// also the Leaderboard-skip path (UJ-4); this story never builds a control
// that enters the leaderboard state (see story Dev Notes). "finished" has
// no primary action — ControlPage renders a placeholder for it instead.
const primaryActionByState: Record<string, PrimaryAction> = {
  question_open: { label: strings.live.closeQuestionCta, path: 'close-question' },
  question_closed: { label: strings.live.revealCta, path: 'reveal' },
  revealed: { label: strings.live.nextQuestionCta, path: 'next-question' },
}

// States "עצור" is offered from — every state where a live round is
// actually running (see story Dev Notes: not from lobby, nothing running
// yet to abort).
const stoppableStates = new Set(['question_open', 'question_closed', 'revealed'])

// Renders the live-control surface for every game state past lobby. Reads
// snapshot as a prop only — no own data fetching, so it shares the single
// useGameSocket subscription LobbyPage already opened.
export function ControlPage({ gameId, snapshot }: ControlPageProps) {
  const primary = primaryActionByState[snapshot.state]

  const action = useMutation({
    mutationFn: (path: string) => api<void>(`/api/games/${gameId}/${path}`, { method: 'POST' }),
  })
  const stop = useMutation({
    mutationFn: () => api<void>(`/api/games/${gameId}/stop`, { method: 'POST' }),
  })
  const anyConflict =
    (action.error instanceof ApiError && action.error.status === 409) ||
    (stop.error instanceof ApiError && stop.error.status === 409)
  // GRADING_INCOMPLETE is itself a 409 (httpapi/errors.go), so it must be
  // checked before the generic anyConflict fallback below or it would
  // match that first and show the less specific message.
  const gradingIncomplete = action.error instanceof ApiError && action.error.code === 'GRADING_INCOMPLETE'

  const fire = useSingleFlight(action)

  useSpaceAction(() => {
    if (primary) fire(primary.path)
  }, Boolean(primary))

  // Clears a stale error banner once the live state actually advances (a
  // 409 from losing a race, or from a re-press that landed after the prior
  // call already succeeded, must not keep telling the organizer the panel
  // needs attention once the WS snapshot shows it self-healed). Latest-ref
  // pattern so the reset effect depends only on snapshot.state, not on
  // action/stop's identity (which changes on every settle and would
  // otherwise wipe a freshly-set error before it's ever seen) — the ref is
  // synced inside its own effect (not during render, which the lint rules
  // here forbid) and, since effects run in declaration order within one
  // commit, is always current by the time the reset effect below can fire
  // in that same commit.
  const actionRef = useRef(action)
  const stopRef = useRef(stop)
  useEffect(() => {
    actionRef.current = action
    stopRef.current = stop
  })
  useEffect(() => {
    actionRef.current.reset()
    stopRef.current.reset()
  }, [snapshot.state])

  // Game over: EXPERIENCE.md's Host-control-panel table specifies no
  // primary CTA here, the results summary on screen, and a secondary back
  // CTA. gameOverTitle stays the heading (the organizer just finished a
  // game); the routed page uses results.title instead. This branch returns
  // before the useSpaceAction/error-banner UI below, so the summary never
  // competes with the Space handler.
  if (snapshot.state === 'finished') {
    return (
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
        <h1 className="text-2xl font-heading text-host-text">{strings.live.gameOverTitle}</h1>
        {/* Game over is inside the launch CTA's window (EXPERIENCE.md's
            Host-control-panel table: Lobby through Game over). */}
        <DisplayControls
          gameId={gameId}
          reducedMotion={snapshot.displaySettings?.reducedMotion ?? false}
          gameState={snapshot.state}
        />
        <ResultsSummary gameId={gameId} />
        <Link to="/" className="text-green-800 underline">
          {strings.gameEditor.backToGames}
        </Link>
      </div>
    )
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      {/* Polite live region announcing the primary CTA's label — the same
          persistent button's text is the only visible signal a state
          change happened (EXPERIENCE.md's Host control panel pattern), so a
          screen-reader host needs this to notice the swap. */}
      <p aria-live="polite" role="status" className="sr-only">
        {primary?.label ?? strings.live.gameOverTitle}
      </p>

      {snapshot.currentQuestion && (
        <div className="flex flex-col gap-2">
          <p className="text-host-text-secondary">
            {strings.live.questionProgress(snapshot.currentQuestion.position, snapshot.questionCount)}
          </p>
          <p className="text-sm text-host-text-secondary">
            {snapshot.currentQuestion.type === 'mcq'
              ? strings.gameEditor.typeMcq
              : strings.gameEditor.typeFreeText}
          </p>
          <p className="text-lg text-host-text">{snapshot.currentQuestion.text}</p>
          {snapshot.state === 'question_open' && (
            <ResponseStats count={snapshot.currentQuestion.answeredCount} />
          )}
        </div>
      )}

      {(action.isError || stop.isError) && (
        <p role="alert" className="text-sm text-error">
          {gradingIncomplete
            ? strings.live.gradingIncomplete
            : anyConflict
              ? strings.live.actionConflict
              : strings.live.actionError}
        </p>
      )}

      <div className="flex items-center gap-3">
        {/* One persistent element across every state transition (never
            conditionally swapped) — a differently-keyed/conditional button
            would remount and drop keyboard focus (AC-4). Deliberately never
            disabled by action.isPending: a disabled button loses DOM focus
            in every browser the instant it's disabled, which would itself
            break AC-4's focus-retention requirement on every click.
            Double-fire is instead guarded inside fire()'s ref. */}
        <Button
          onClick={() => primary && fire(primary.path)}
          disabled={!primary}
          className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900"
        >
          {primary?.label ?? ''}
        </Button>

        {stoppableStates.has(snapshot.state) && (
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="outline" className="h-10 border-host-border text-error">
                {strings.live.stopCta}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{strings.live.stopConfirmTitle}</AlertDialogTitle>
                <AlertDialogDescription>{strings.live.stopConfirmBody}</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel className="h-10">{strings.common.cancel}</AlertDialogCancel>
                <AlertDialogAction
                  className="h-10 bg-error text-ink-on-dark hover:bg-error/90"
                  onClick={() => stop.mutate()}
                >
                  {strings.live.stopConfirmAction}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        )}
      </div>

      {/* The launch CTA and the room-level motion toggle stay available for
          the whole live run (EXPERIENCE.md's Host-control-panel table:
          Lobby through Game over), so they render below the primary CTA
          row rather than only in the finished branch above.
          Optional-chained despite the non-optional type: a redeploy briefly
          runs two instances (store/migrate.go), so this socket can be served
          a snapshot built before the field existed, and there is no error
          boundary — an unguarded deref would take down the organizer's live
          panel mid-game. (Code review, 2026-08-09.) */}
      <DisplayControls
        gameId={gameId}
        reducedMotion={snapshot.displaySettings?.reducedMotion ?? false}
        gameState={snapshot.state}
      />
    </div>
  )
}
