import { useEffect, useState, type ComponentType } from 'react'
import { useParams } from 'react-router'

import { strings } from '@/lib/strings.he'
import { useGameSocket } from '@/lib/use-game-socket'
import type { GameState, LobbySnapshot } from '@/lib/types'
import { LobbyStage } from './lobby-stage'
import { StagePlaceholder } from './stage-placeholder'

/** What every stage component receives. A stage gets the whole snapshot
 * (not narrowed props) because the snapshot is the store — narrowing would
 * mean editing this shell every time a stage needs one more field.
 * reducedMotion is already the OR of the viewer's OS setting and the
 * Organizer's room-level setting: no stage should ever call matchMedia. */
export interface StageProps {
  snapshot: LobbySnapshot
  reducedMotion: boolean
}

// One component per game state (UX-DR16). Stories 4.2–4.6 each replace
// exactly one entry; `draft` keeps the placeholder. A Record<GameState, …>
// and not a lookup with a fallback: the whole point is that adding a state
// to the union breaks the build here rather than blanking a projector.
const stageByState: Record<GameState, ComponentType<StageProps>> = {
  draft: StagePlaceholder,
  lobby: LobbyStage, // story 4.2 — lobby stage
  question_open: StagePlaceholder, // story 4.3 — question stage
  question_closed: StagePlaceholder, // story 4.3 — question stage
  revealed: StagePlaceholder, // story 4.4 — reveal stage
  leaderboard: StagePlaceholder, // story 4.5 — leaderboard stage
  finished: StagePlaceholder, // story 4.6 — winner takeover
}

const reducedMotionQuery = '(prefers-reduced-motion: reduce)'

/** The viewer's own OS setting, subscribed for changes. Only one of the two
 * reduced-motion sources — the room-level setting rides the snapshot,
 * because the audience cannot set this one on a projector. */
function usePrefersReducedMotion(): boolean {
  const [prefers, setPrefers] = useState(() => window.matchMedia(reducedMotionQuery).matches)
  useEffect(() => {
    const query = window.matchMedia(reducedMotionQuery)
    const onChange = (event: MediaQueryListEvent) => setPrefers(event.matches)
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [])
  return prefers
}

// Shared surface for every branch below: flat (no shadow — DESIGN.md says
// the Audience Display is flat), full-bleed, never scrolling, Festival
// Green ground, and dir="rtl" belt-and-braces over index.html's
// document-level dir since this window may be opened standalone.
const stageRootClass =
  'stage-root relative flex min-h-svh w-full flex-col items-center justify-center overflow-hidden bg-surface-base text-text-primary'
const stageMargin = { padding: 'var(--stage-margin)' }
const stageMessageClass = 'text-[length:var(--stage-heading)] font-heading text-text-secondary'

/** The last snapshot this window rendered, kept alive across a dropped
 * socket — and tagged with the game it belongs to, because navigating
 * /display/A -> /display/B does not remount this component and an untagged
 * retention would render (and announce) game A's stage while connecting to
 * game B. (Code review, 2026-08-09.) */
interface RetainedSnapshot {
  gameId: string
  snapshot: LobbySnapshot
}

/**
 * The Audience Display shell (FR-9): a full-screen, output-only surface
 * that renders one stage per game state, entirely from the WS snapshot.
 *
 * No TanStack Query and no api() call here, deliberately (architecture:
 * "display/* renders exclusively from the WS snapshot") — it must work
 * with nothing but a gameId and a socket. The one REST call in play is
 * use-game-socket's own probeThenDecide, which classifies a handshake that
 * never opened; that is error classification, not a data source.
 */
export function DisplayPage() {
  const { gameId = '' } = useParams()
  const { snapshot, notFound } = useGameSocket(gameId, 'display')
  const prefersReducedMotion = usePrefersReducedMotion()

  // useGameSocket nulls `snapshot` the moment the socket closes, which is
  // right for the dashboard (UX-DR12: never show stale data as live) and
  // exactly wrong for a projector — a blank screen mid-room is a worse
  // failure than a one-second-old one. Keep the last non-null snapshot and
  // render it under the reconnect overlay.
  //
  // State adjusted during render (React's documented "storing information
  // from previous renders" pattern), not a ref and not an effect: the
  // project's lint rules reject reading a ref during render
  // (react-hooks/refs) and calling setState from an effect body
  // (react-hooks/set-state-in-effect). React re-runs this component
  // immediately without committing the discarded render, so `rendered`
  // below is always current. The guard only ever stores a non-null
  // snapshot, so when `snapshot` flips to null this still holds the
  // previous one.
  const [retained, setRetained] = useState<RetainedSnapshot | null>(null)
  if (
    snapshot !== null &&
    (retained === null || retained.snapshot !== snapshot || retained.gameId !== gameId)
  ) {
    setRetained({ gameId, snapshot })
  }
  const rendered = snapshot ?? (retained?.gameId === gameId ? retained.snapshot : null)

  // Optional-chained even though the field is non-optional in LobbySnapshot:
  // store/migrate.go documents redeploys briefly running two instances, so a
  // new bundle whose socket lands on a still-draining old instance receives a
  // snapshot built before this field existed. use-game-socket validates only
  // the envelope, and there is no error boundary anywhere in the app, so an
  // unguarded deref here puts React Router's English crash page on the
  // projector. (Code review, 2026-08-09.)
  const reducedMotion = prefersReducedMotion || (rendered?.displaySettings?.reducedMotion ?? false)

  // Falls back at runtime for a state the client's union does not have —
  // the same server-newer-than-client skew as above. The Record annotation
  // still makes a missing key a build error, so the compile-time
  // exhaustiveness check that 4.2–4.6 rely on is untouched; this only stops
  // an unknown state from throwing "Element type is invalid" onto the
  // projector. (Code review, 2026-08-09.)
  const Stage = rendered ? (stageByState[rendered.state] ?? StagePlaceholder) : null

  return (
    <div
      dir="rtl"
      // Both channels, because 4.3–4.6 will each need one: the prop for JS
      // motion, the attribute so index.css can kill the cross-fade without
      // any stage having to opt in.
      data-reduced-motion={String(reducedMotion)}
      className={stageRootClass}
      style={stageMargin}
    >
      {/* UX-DR14 reserves assertive for game-state transitions, and the
          shell owns transitions. Counters (4.2) are polite; the timer
          numeral (4.3) is excluded from live regions entirely.
          Rendered in EVERY branch, and empty until the first snapshot
          lands: a live region inserted into the DOM already holding its
          text is not announced, so mounting it only alongside the first
          stage silently dropped the most important announcement of all.
          No role="status" — its implicit polite politeness contradicts the
          explicit assertive. (Code review, 2026-08-09.) */}
      <p aria-live="assertive" className="sr-only">
        {rendered ? strings.display.stateAnnouncement[rendered.state] : ''}
      </p>

      {/* A dead end, not a transient one: no overlay, no retry, no stage. */}
      {notFound ? (
        <p className={stageMessageClass}>{strings.display.notFound}</p>
      ) : !rendered || !Stage ? (
        // First connect, nothing ever rendered: the only moment the
        // "connecting" copy is allowed on screen (UX-DR12 — it must never
        // appear while the socket is up).
        <p className={stageMessageClass}>{strings.display.connecting}</p>
      ) : (
        // key remounts on every transition so the ≤300ms cross-fade
        // replays; under reduced motion the CSS makes it instant, so there
        // is no JS branch here.
        // The gameId is part of the key because 4.2 is the first stage with
        // local state (its high-water count, its announced value) and this
        // component does NOT remount when the URL's gameId changes — the
        // same trap the `retained` tagging above fixes. Without it,
        // /display/A -> /display/B carries game A's counter into game B.
        <div
          key={`${rendered.gameId}:${rendered.state}`}
          className="stage-fade flex w-full flex-1 flex-col items-center justify-center"
        >
          <Stage snapshot={rendered} reducedMotion={reducedMotion} />
        </div>
      )}

      {/* Reconnect: over the last rendered stage, never instead of it
          (EXPERIENCE.md's Resilience row). Absent entirely while the
          socket is up.
          Offset by --stage-margin rather than pinned to top-0: absolute
          offsets resolve against the padding box, i.e. the outer edge, and
          that margin exists as projector overscan tolerance — so the one
          element telling the room its screen has gone stale was the one
          element a projector could clip. (Code review, 2026-08-09.) */}
      {!notFound && rendered && !snapshot && (
        <p
          role="status"
          className="absolute top-[var(--stage-margin)] start-0 end-0 bg-green-900 py-2 text-center text-[length:var(--stage-body)] font-ui text-ink-on-dark"
        >
          {strings.display.reconnecting}
        </p>
      )}
    </div>
  )
}
