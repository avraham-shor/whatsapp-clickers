import { useState } from 'react'

import type { LeaderboardEntry, LobbySnapshot } from '@/lib/types'
import { isDegradedFrame } from './stage-props'
import type { PreviousStandings } from './stage-props'

/** The whole board's ranking as one lookup, covering EVERY entry and not
 * just the visible top ten (story 4.5, derived requirement 7): a climb from
 * 14th to 8th only computes correctly if 14th was remembered, and a row
 * entering the top ten has to know where it came from.
 *
 * Rank and index are both kept because with shared ranks they are different
 * numbers — ranks 1,1,3 sit in slots 0,1,2. Rank drives the ▲ count (what
 * the room understands); index drives the reshuffle distance (what the eye
 * sees). */
export function standingsOf(entries: LeaderboardEntry[]): PreviousStandings {
  const standings: Record<string, { rank: number; index: number }> = {}
  entries.forEach((entry, index) => {
    standings[entry.participantId] = { rank: entry.rank, index }
  })
  return standings
}

/** One Leaderboard showing, identified by the question it is paused on. */
interface LeaderboardMemory {
  gameId: string
  questionId: string
  current: PreviousStandings
  previous: PreviousStandings | undefined
}

/**
 * The shell's memory of the ranking this window last showed the room.
 *
 * Client-side and not a wire field, deliberately (story 4.5, derived
 * requirement 6): the epic asks for movement "since the previous
 * Leaderboard", which means the leaderboard *as shown to this room* —
 * something only this window knows, and something a server-side "previous"
 * would get wrong the moment the Organizer skips a Leaderboard (UJ-4).
 *
 * Owned by the shell rather than the stage because the shell keys each stage
 * on `${gameId}:${state}` and remounts it on every transition, so anything
 * the stage remembered would be discarded exactly when it is needed. The
 * same reason display-page.tsx owns answeredFloor.
 *
 * The known limit, stated rather than hidden: a display reloaded or opened
 * mid-game has no baseline and shows no ▲ on the first Leaderboard it sees,
 * which looks exactly like "nobody climbed".
 */
export function useLeaderboardMemory(
  gameId: string,
  snapshot: LobbySnapshot | null,
): PreviousStandings | undefined {
  // State adjusted during render — React's documented "storing information
  // from previous renders" pattern, and the only option the project's lint
  // rules leave: react-hooks/refs forbids reading a ref during render and
  // react-hooks/set-state-in-effect forbids setState from an effect body.
  // React re-runs the component without committing the render that set this,
  // so the value read below is always current. display-page.tsx:86-94 states
  // the mechanism and is the reference.
  //
  // Not module-level state either, however tempting: StrictMode is on
  // (main.tsx), a double-invoked initializer would consume the baseline on
  // the first pass and see its own write on the second, and the tests would
  // become order-dependent.
  const [memory, setMemory] = useState<LeaderboardMemory | null>(null)

  // A degraded frame NEVER captures a baseline: recording its empty board as
  // "what the room saw" would poison the next showing's arrows with a board
  // of nobody. The predicate is shared with leaderboard-stage.tsx rather than
  // restated here, because the stage's refusal to RENDER such a frame and
  // this hook's refusal to REMEMBER it are the same decision (derived
  // requirement 8) and had already drifted: this test used to check only the
  // currentQuestion half, so a frame the stage refused to show could still be
  // captured here. (Code review, 2026-08-12.)
  const showing =
    snapshot !== null && snapshot.state === 'leaderboard' && !isDegradedFrame(snapshot)
      ? snapshot
      : null
  const questionId = showing?.currentQuestion?.id ?? null

  if (
    showing !== null &&
    questionId !== null &&
    (memory === null || memory.gameId !== gameId || memory.questionId !== questionId)
  ) {
    // A new showing. The order the room last saw becomes the baseline —
    // but only within one game: /display/A -> /display/B does not remount
    // the shell, and game A's ranking is meaningless against game B's.
    setMemory({
      gameId,
      questionId,
      current: standingsOf(showing.leaderboard),
      previous: memory !== null && memory.gameId === gameId ? memory.current : undefined,
    })
  }
  // Every OTHER frame writes nothing at all. That is the point of keying on
  // the question id: current_question_position does not move until
  // NextQuestion, so every frame arriving during one showing — a
  // display-settings toggle is the reachable one — carries the same board,
  // and re-capturing would overwrite the baseline with the current standings
  // and silently erase every arrow one frame after they appeared.

  if (showing === null || memory === null || memory.gameId !== gameId) {
    return undefined
  }
  return memory.previous
}
