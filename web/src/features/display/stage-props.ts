import type { LobbySnapshot } from '@/lib/types'

/** What every stage component receives. A stage gets the whole snapshot
 * (not narrowed props) because the snapshot is the store — narrowing would
 * mean editing this shell every time a stage needs one more field.
 * reducedMotion is already the OR of the viewer's OS setting and the
 * Organizer's room-level setting: no stage should ever call matchMedia.
 *
 * Its own module, and not `@/lib/types` (whose header is "Wire types
 * mirroring the Go payloads" — this is a component contract, not a wire
 * type) and no longer display-page.tsx: that file value-imports every
 * stage, so the display module graph held a real cycle apart with one
 * `type` keyword that no type checker enforces. Writing `import { StageProps }`
 * in any stage file reintroduced it, and the symptom is an undefined
 * component at module-init time — a blank projector.
 * (deferred-work.md, 4.2 entry; closed by story 4.3.) */
export interface StageProps {
  snapshot: LobbySnapshot
  reducedMotion: boolean
  /** The ranking this window showed at the PREVIOUS Leaderboard for this
   *  game, or undefined when it has shown none — a freshly opened or
   *  reloaded display, and the first Leaderboard of every game.
   *
   *  Cross-stage memory, so it cannot live in the stage: the shell keys
   *  each stage on `${gameId}:${state}` and remounts on every
   *  transition. The same reason display-page.tsx owns answeredFloor.
   *  Optional because exactly one stage consumes it; the four others
   *  ignore it. */
  previousStandings?: PreviousStandings
}

/** One row of the previous Leaderboard: the rank it held, and the slot
 *  it occupied in the order the room actually saw. Rank drives the ▲
 *  count (AC-1's "places climbed"); index drives the reshuffle, because
 *  with shared ranks the two are different numbers — ranks 1,1,3 sit in
 *  slots 0,1,2. */
export interface PreviousStanding {
  rank: number
  index: number
}

/** Keyed by participantId, covering the WHOLE previous leaderboard and
 *  not only its visible top ten (story 4.5, derived requirement 7). */
export type PreviousStandings = Readonly<Record<string, PreviousStanding>>

/** True for a frame that carries a real State but no readable content —
 *  `emptySnapshot`, which `snapshotAfterCommit` falls back to when
 *  `buildSnapshot` fails after the write already committed. It sets an EMPTY
 *  leaderboard together with a zero questionCount and a nil currentQuestion,
 *  and an empty board is the one degraded field that looks like a real value
 *  ("everyone scored 0").
 *
 *  Story 4.5's derived requirement 8 resolved that without a wire-contract
 *  change, by reading a CORRELATION across fields already on the wire: a
 *  legitimate `leaderboard` frame always has questions and a current question,
 *  because the state is only reachable from `revealed`.
 *
 *  It lives here, in one place, because two call sites depend on it and they
 *  must never drift apart: leaderboard-stage.tsx refuses to render such a
 *  frame, and use-leaderboard-memory.ts refuses to capture a baseline from
 *  one. They were two hand-maintained predicates that already disagreed —
 *  the hook tested only the currentQuestion half, so a frame the stage
 *  refused to show could still be recorded as "what the room saw" and poison
 *  the next showing's arrows. (Code review, 2026-08-12.) */
export function isDegradedFrame(snapshot: LobbySnapshot): boolean {
  return snapshot.questionCount === 0 || snapshot.currentQuestion === null
}
