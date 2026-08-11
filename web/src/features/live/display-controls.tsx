import { useEffect, useId, useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import type { GameState } from '@/lib/types'
import { useSingleFlight } from '@/lib/use-single-flight'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'

interface DisplayControlsProps {
  gameId: string
  reducedMotion: boolean
  /** Only used to clear a stale error banner when the live state advances,
   * mirroring control-page.tsx's own reset effect. */
  gameState: GameState
}

/**
 * The two dashboard-side Audience Display controls (FR-9, story 4.1) in
 * one component, so the surfaces that show them (the lobby and the live
 * control panel, in both its branches) cannot diverge.
 *
 * reducedMotion is driven by the snapshot, never by local state: the PUT
 * broadcasts the new snapshot, the socket delivers it, and the checkbox
 * follows. That round trip is the AC's "rides the game snapshot" made
 * visible on the dashboard too — which is also why nothing here
 * invalidates a query.
 */
export function DisplayControls({ gameId, reducedMotion, gameState }: DisplayControlsProps) {
  const checkboxId = useId()
  const hintId = useId()
  // Stored as the state it was raised in rather than a bare boolean, so the
  // banner expires on its own when the live state advances — no effect, and
  // therefore no react-hooks/set-state-in-effect violation.
  const [popupBlockedIn, setPopupBlockedIn] = useState<GameState | null>(null)
  const popupBlocked = popupBlockedIn === gameState

  const setReducedMotion = useMutation({
    mutationFn: (next: boolean) =>
      api<unknown>(`/api/games/${gameId}/display-settings`, {
        method: 'PUT',
        body: JSON.stringify({ reducedMotion: next }),
      }),
  })

  // Single-flight, for the reason recorded at 2026-08-09's code review:
  // two concurrent PUTs race to decide both the persisted value and the
  // last broadcast, and they are easy to trigger here precisely because
  // the checkbox has no local echo — it snaps back for the whole round
  // trip, which reads as broken and invites a second click. Same shared
  // guard as control-page.tsx's fire() and lobby-page.tsx's fireStart.
  const submit = useSingleFlight(setReducedMotion)

  // Clears a stale failure once the live state advances — the same reason
  // control-page.tsx resets its own banners on [snapshot.state]: a banner
  // that outlives the moment it describes keeps telling the organizer the
  // panel needs attention through the rest of the game. Latest-ref pattern
  // so the reset depends only on gameState, not on the mutation's identity
  // (which changes on every settle and would wipe a freshly-set error
  // before it is ever seen). (Code review, 2026-08-09.)
  const mutationRef = useRef(setReducedMotion)
  useEffect(() => {
    mutationRef.current = setReducedMotion
  })
  useEffect(() => {
    mutationRef.current.reset()
  }, [gameState])

  // A new window, per the AC and EXPERIENCE.md Flow 1 (the Organizer drags
  // the new window to the projector) — not a <Link>, which would navigate
  // the dashboard away from the live panel mid-game.
  //
  // Named target, and no `noopener`, both deliberately (code review,
  // 2026-08-09). `noopener` forces the target to `_blank` per spec, so
  // every repeat click spawned another display window holding its own
  // socket — and it also makes window.open return null unconditionally,
  // which made a popup-blocked launch indistinguishable from a successful
  // one. The opened page is our own same-origin, output-only surface and
  // never touches window.opener, so the reference costs nothing while the
  // name buys window reuse and the null return buys a real error message.
  // Explicit popup features because a bare '_blank' is a tab in most
  // browsers, and AC-1 asks for a window.
  const openDisplay = () => {
    const opened = window.open(
      `/display/${gameId}`,
      `wc-display-${gameId}`,
      'popup=yes,width=1280,height=720',
    )
    setPopupBlockedIn(opened === null ? gameState : null)
    opened?.focus()
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-1">
        <Button variant="outline" className="h-10 border-host-border text-host-text" onClick={openDisplay}>
          {strings.live.openDisplayCta}
        </Button>
        {popupBlocked && (
          <p role="alert" className="text-sm text-error">
            {strings.live.openDisplayBlocked}
          </p>
        )}
      </div>

      <div className="flex flex-col gap-1">
        {/* A native checkbox, not a shadcn Switch: components/ui/ has no
            switch primitive, and a checkbox is semantically exact for a
            boolean with free native keyboard and AT support. */}
        <Label htmlFor={checkboxId} className="text-host-text">
          <input
            id={checkboxId}
            type="checkbox"
            checked={reducedMotion}
            aria-describedby={hintId}
            onChange={(event) => submit(event.target.checked)}
            className="size-5 accent-green-800"
          />
          {strings.live.reduceMotionLabel}
        </Label>
        {/* Programmatically associated, never floating (UX-DR12). */}
        <p id={hintId} className="text-sm text-host-text-secondary">
          {strings.live.reduceMotionHint}
        </p>
        {/* Kept local deliberately: control-page.tsx's error banner is
            already over-multiplexed with two mutations (deferred-work.md,
            3.10 entry) and a third would make that entry worse. */}
        {setReducedMotion.isError && (
          <p role="alert" className="text-sm text-error">
            {strings.live.reduceMotionError}
          </p>
        )}
      </div>
    </div>
  )
}
