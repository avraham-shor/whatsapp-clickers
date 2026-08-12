import type { CSSProperties } from 'react'

import { strings } from '@/lib/strings.he'
import { isDegradedFrame } from './stage-props'
import type { StageProps } from './stage-props'

/** [A11] fixes the board's depth at ten. A tie that straddles the cut IS
 *  cut — a plain slice can show one member of a tied pair and not the other.
 *  Accepted rather than solved: extending the list for a boundary tie blows
 *  the vertical budget the row height is sized against, and dropping both
 *  hides a real top-ten finisher. */
const maxRows = 10

/**
 * The Leaderboard stage (FR-18, FR-10): the standings between questions,
 * with the climbers marked and the rows reshuffling into their new order.
 *
 * Ranks come from the server and are rendered verbatim, in the order
 * received. This file never sorts and never re-ranks: game.RankLeaderboard
 * already assigns standard competition ranks (1,1,3 — never 1,1,2) with ties
 * broken stably by joined_at, and a second client-side ranking could
 * silently disagree with the winner computation that reads the same entries.
 *
 * The one stage where per-Participant data is specified — names and scores
 * ARE the content here. What stays forbidden: grades (they live only in each
 * Participant's WhatsApp, FR-10), phone numbers, who answered what, and any
 * current-user highlighting. It is a shared screen. This file never reads
 * snapshot.participants.
 */
export function LeaderboardStage({ snapshot, reducedMotion, previousStandings }: StageProps) {
  // deferred-work.md's 3.7 entry is about exactly this frame and names this
  // file as its trigger. The predicate itself lives in stage-props.ts because
  // use-leaderboard-memory.ts has to refuse the SAME frame, and two
  // hand-maintained copies of one correlation had already drifted apart
  // (code review, 2026-08-12).
  if (isDegradedFrame(snapshot)) {
    return (
      <p className="text-[length:var(--stage-heading)] font-heading text-text-secondary">
        {strings.display.waiting}
      </p>
    )
  }

  // A DIFFERENT sentence from the one above, on purpose: the room is being
  // told two different true things. This frame is healthy — nobody
  // registered for this game, which nothing gates a start on.
  if (snapshot.leaderboard.length === 0) {
    return (
      <p className="text-[length:var(--stage-heading)] font-heading text-text-secondary">
        {strings.display.leaderboard.noPlayers}
      </p>
    )
  }

  // Killed from the PROP, not from a media query: a CSS-only kill is
  // invisible to jsdom so it could not be asserted, and the Organizer's
  // room-level toggle is not a media query at all. Third stage in a row with
  // the same requirement (4.3, 4.4, and now this one).
  const animated = !reducedMotion
  const rows = snapshot.leaderboard.slice(0, maxRows)

  return (
    // Full-bleed WHITE, and that is load bearing rather than stylistic:
    // DESIGN.md's mover highlight is green-50, which is the shell's own
    // ground colour (--color-surface-base), so on the shell's background the
    // one highlight the spec asks for would be literally invisible. Same
    // class of finding as 4.2's counter pill. absolute inset-0 resolves
    // against the shell's padding box, so the safe margin is restored
    // inside. No hero band: split-hero is specified "game stages only" and
    // there is no timer, no question and no progress here.
    <div className="absolute inset-0 flex flex-col justify-center bg-surface-raised p-[var(--stage-margin)]">
      {/* <ol> and not a table: implicit list semantics carry the ranking,
          and an <li> is safe to transform, unlike a <tr>. No heading —
          there is no vertical budget for one, none is specified, and the
          shell's assertive announcer already names this state on entry
          (strings.display.stateAnnouncement.leaderboard), so a visible
          title would be a third naming of the same thing.
          No live region either: UX-DR14 reserves assertive for stage
          transitions and the shell owns those; these rows are static
          content and a second announcer would fight the first. */}
      <ol className="w-full text-[length:var(--stage-ui)] font-ui">
        {rows.map((entry, index) => {
          const previous = previousStandings?.[entry.participantId]
          // Positive only. Falls are not shown — EXPERIENCE.md says "rows
          // that climbed carry ▲" and AC-1 says "highlight climbers"; a
          // faller gets neither an indicator nor a row background.
          const climbed = previous ? previous.rank - entry.rank : 0
          // Math.min(previous.index, maxRows) makes a row arriving from
          // below the fold slide up from just past the last visible slot
          // instead of from forty rows away. There is deliberately NO second,
          // outer clamp: the result is bounded by construction, because the
          // capped start is in [0, maxRows] and `index` is in
          // [0, maxRows - 1], so the shift can only land in
          // [-(maxRows - 1), maxRows]. The clamp that used to sit here could
          // not engage in either direction, and the test that claimed to
          // prove it could not fail (code review, 2026-08-12).
          // The runtime guard on `previous` is real, not a formality:
          // noUncheckedIndexedAccess is off, so the index type says
          // `PreviousStanding` and lies.
          const shift = previous ? Math.min(previous.index, maxRows) - index : 0

          return (
            <li
              // Real identity, unlike the option rows — and identity is
              // exactly what a reshuffle is about.
              key={entry.participantId}
              // Inline custom property, not a class: the shift is data. Cast
              // is required — CSSProperties has no index signature for
              // custom properties.
              style={shift !== 0 ? ({ '--stage-row-shift': shift } as CSSProperties) : undefined}
              className={`flex h-[var(--stage-leaderboard-row)] items-center gap-6 border-b border-border-light px-6 last:border-b-0 ${climbed > 0 ? 'bg-green-50' : ''} ${animated && shift !== 0 ? 'stage-row-reshuffle' : ''}`}
            >
              {/* Fixed measures on three of the four columns, so the
                  numbers form columns — which is the whole point of
                  tabular-nums. <bdi dir="ltr"> on every digit run: they are
                  all-digit runs in an RTL row with no strong character to
                  anchor them (DESIGN.md Typography: bidi isolation is
                  mandatory). */}
              <span className="w-[3ch] shrink-0 text-text-secondary tabular-nums">
                <bdi dir="ltr">{entry.rank}</bdi>
              </span>

              {/* RESERVED on every row, not conditionally rendered: a
                  conditional mark pushes that one row's name out of the
                  column every other row shares (4.4's review found exactly
                  this on the bar labels). aria-hidden because the glyph is
                  decorative; the sr-only sibling carries the meaning, which
                  is what keeps this from being colour-alone (UX-DR14). */}
              <span
                aria-hidden="true"
                className="w-[5ch] shrink-0 text-green-600 tabular-nums"
              >
                {climbed > 0 ? (
                  <>
                    {strings.display.leaderboard.moverMark}
                    <bdi dir="ltr">{climbed}</bdi>
                  </>
                ) : null}
              </span>
              {climbed > 0 && (
                <span className="sr-only">
                  {strings.display.leaderboard.climbedLabel(climbed)}
                </span>
              )}

              {/* <bdi> because a display name is free-form participant-
                  supplied text that can be Latin, mixed, or start with a
                  digit inside an RTL row. `truncate` because the reshuffle
                  arithmetic needs every row to be exactly one row height —
                  a wrapping name would make one row taller and every shift
                  below it wrong by the difference. */}
              <bdi className="min-w-0 flex-1 truncate text-text-primary">{entry.displayName}</bdi>

              {/* Raw digits, no toLocaleString and no thousands separator,
                  matching results-summary.tsx's {entry.score} so the
                  dashboard table and the projector cannot disagree about
                  the same number.
                  font-ui (600) where DESIGN.md asks for 700, which this
                  @theme has no step for. The nearest is font-heading (800),
                  and 800 beside a 600 name would read as a different kind
                  of information — this stage's hierarchy is carried by
                  COLOUR, not by weight. Recorded as a deviation, the same
                  way stage-option.tsx records its own. */}
              <span className="w-[6ch] shrink-0 text-end text-green-800 tabular-nums">
                <bdi dir="ltr">{entry.score}</bdi>
              </span>
            </li>
          )
        })}
      </ol>
    </div>
  )
}
