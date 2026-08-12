import { useEffect, useState, type ComponentType } from 'react'
import { useParams } from 'react-router'

import { strings } from '@/lib/strings.he'
import { useGameSocket } from '@/lib/use-game-socket'
import type { GameState, LobbySnapshot } from '@/lib/types'
import { LobbyStage } from './lobby-stage'
import { QuestionStage } from './question-stage'
import { RevealStage } from './reveal-stage'
import { StagePlaceholder } from './stage-placeholder'
// The contract lives in its own module (story 4.3) and is deliberately NOT
// re-exported from here: a `export type { StageProps } from './stage-props'`
// would preserve the very import path whose value form recreates the cycle.
import type { StageProps } from './stage-props'

// One component per game state (UX-DR16). Stories 4.2–4.6 each replace
// exactly one entry; `draft` keeps the placeholder. A Record<GameState, …>
// and not a lookup with a fallback: the whole point is that adding a state
// to the union breaks the build here rather than blanking a projector.
const stageByState: Record<GameState, ComponentType<StageProps>> = {
  draft: StagePlaceholder,
  lobby: LobbyStage, // story 4.2 — lobby stage
  question_open: QuestionStage, // story 4.3 — question stage
  question_closed: QuestionStage, // story 4.3 — question stage (timer at 0)
  revealed: RevealStage, // story 4.4 — reveal stage
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

  // The live answered count must never go backwards in front of a room, and
  // the stage cannot hold that floor by itself: the key further down remounts
  // it on every state transition, so a hold kept inside the stage is discarded
  // at exactly the question_open -> question_closed moment where the closing
  // snapshot can carry a count LOWER than one already delivered. Measured at
  // 4.3's code review, not theorised: 8 -> 7 across that transition.
  //
  // The mechanism is deferred-work.md's 2.4 entry — `seq` is stamped at
  // Broadcast()-call time, so concurrent writers can deliver a lower count
  // under a higher seq and use-game-socket's `seq < lastSeq` guard accepts it.
  // Sound ONLY because within one Question the count cannot legitimately fall:
  // `answers` is append-once under UNIQUE (question_id, participant_id) and
  // nothing deletes a row. Keyed on the question id so question 2 never
  // inherits question 1's floor, and deliberately applied to this ONE field —
  // 4.5's leaderboard positions genuinely move both ways and freezing a
  // maximum there would be a real bug.
  //
  // The stage keeps its own within-mount hold as the near guard; this is the
  // half that survives the remount. Same render-phase state pattern as above.
  const [answeredFloor, setAnsweredFloor] = useState<{
    questionId: string
    count: number
  } | null>(null)
  const currentQuestion = rendered?.currentQuestion ?? null
  if (
    currentQuestion !== null &&
    (answeredFloor === null ||
      answeredFloor.questionId !== currentQuestion.id ||
      currentQuestion.answeredCount > answeredFloor.count)
  ) {
    setAnsweredFloor({ questionId: currentQuestion.id, count: currentQuestion.answeredCount })
  }
  const stageSnapshot: LobbySnapshot | null =
    rendered !== null &&
    currentQuestion !== null &&
    answeredFloor !== null &&
    answeredFloor.questionId === currentQuestion.id &&
    answeredFloor.count > currentQuestion.answeredCount
      ? { ...rendered, currentQuestion: { ...currentQuestion, answeredCount: answeredFloor.count } }
      : rendered

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
      ) : !rendered || !Stage || !stageSnapshot ? (
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
          <Stage snapshot={stageSnapshot} reducedMotion={reducedMotion} />
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
