import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render } from '@testing-library/react'

import { strings } from '@/lib/strings.he'
import type { CurrentQuestion, LeaderboardEntry, LobbySnapshot } from '@/lib/types'
import { LeaderboardStage } from './leaderboard-stage'
import type { PreviousStandings } from './stage-props'

// RTL's automatic cleanup only self-registers when a global afterEach
// exists, and vitest `globals` is off here.
afterEach(cleanup)

// Expected copy is READ FROM strings.he.ts, never retyped (4.2's rule).
const copy = strings.display.leaderboard

// Participant display names are Organizer-facing DATA, not copy. Latin
// placeholders on purpose: a Hebrew fixture would put this file in the
// frontend copy-centralization grep for a string carrying no meaning to any
// assertion here.
function entry(id: string, rank: number, score: number): LeaderboardEntry {
  return { participantId: id, displayName: `PLAYER-${id}`, score, rank }
}

function question(): CurrentQuestion {
  return {
    id: 'q-1',
    position: 3,
    type: 'mcq',
    text: 'QUESTION-TEXT',
    options: ['OPTION-A', 'OPTION-B'],
    timeLimitSeconds: 20,
    answerCutoffAt: '2026-08-12T12:00:00Z',
    answeredCount: 8,
    reveal: null,
  }
}

function snap(overrides: Partial<LobbySnapshot> = {}): LobbySnapshot {
  return {
    gameId: 'game-1',
    state: 'leaderboard',
    joinCode: 'COHEN24',
    platformNumber: '+972 50-000-0000',
    participantCount: 3,
    participants: [],
    questionCount: 5,
    currentQuestion: question(),
    leaderboard: [entry('a', 1, 300), entry('b', 2, 200), entry('c', 3, 100)],
    displaySettings: { reducedMotion: false },
    ...overrides,
  }
}

function renderStage(
  snapshot: LobbySnapshot,
  previousStandings?: PreviousStandings,
  reducedMotion = false,
) {
  return render(
    <LeaderboardStage
      snapshot={snapshot}
      reducedMotion={reducedMotion}
      previousStandings={previousStandings}
    />,
  )
}

function rowsOf(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll('li'))
}

// getByText joins only an element's DIRECT text-node children, so a row
// composed of several spans matches on neither half (4.4's harness finding).
// These read the columns structurally instead. Order inside a row is rank,
// mover slot, [sr-only label], name, score.
function rankOf(row: HTMLElement): string {
  return row.querySelector('span')?.textContent ?? ''
}
function moverOf(row: HTMLElement): string {
  return row.querySelector('[aria-hidden="true"]')?.textContent ?? ''
}
function srLabelOf(row: HTMLElement): string | null {
  return row.querySelector('.sr-only')?.textContent ?? null
}
function nameOf(row: HTMLElement): string {
  return row.querySelector('bdi.truncate')?.textContent ?? ''
}
function scoreOf(row: HTMLElement): string {
  const spans = row.querySelectorAll('span')
  return spans[spans.length - 1]?.textContent ?? ''
}
function shiftOf(row: HTMLElement): string {
  return row.style.getPropertyValue('--stage-row-shift')
}

describe('LeaderboardStage', () => {
  it('renders the top ten and no more, from a board of twelve', () => {
    // [A11] fixes the depth at ten. The eleventh and twelfth are real
    // players on a real board — they simply do not fit the room's screen.
    const board = Array.from({ length: 12 }, (_, i) => entry(`p${i + 1}`, i + 1, 1200 - i * 100))
    const { container } = renderStage(snap({ leaderboard: board }))

    const rows = rowsOf(container)
    expect(rows).toHaveLength(10)
    // Asserted on the parsed NAME column, not on container.textContent: the
    // rows concatenate to "…PLAYER-p1" + "1200" + …, which contains the
    // literal "PLAYER-p11" without any eleventh row existing.
    const names = rows.map(nameOf)
    expect(names[9]).toBe('PLAYER-p10')
    expect(names).not.toContain('PLAYER-p11')
    expect(names).not.toContain('PLAYER-p12')
  })

  it('renders shared ranks verbatim as 1,1,3 and never renumbers them', () => {
    // game.RankLeaderboard already assigns standard competition ranks. A
    // client-side re-rank would be a second implementation that can silently
    // disagree with the winner computation reading the same entries.
    const { container } = renderStage(
      snap({ leaderboard: [entry('a', 1, 300), entry('b', 1, 300), entry('c', 3, 100)] }),
    )

    expect(rowsOf(container).map(rankOf)).toEqual(['1', '1', '3'])
  })

  it('renders each row in the server order, with rank, name and raw score', () => {
    const { container } = renderStage(snap())
    const rows = rowsOf(container)

    expect(rows.map(rankOf)).toEqual(['1', '2', '3'])
    expect(rows.map(nameOf)).toEqual(['PLAYER-a', 'PLAYER-b', 'PLAYER-c'])
    // Raw digits, no thousands separator — results-summary.tsx renders the
    // same number the same way, so the two surfaces cannot disagree.
    expect(rows.map(scoreOf)).toEqual(['300', '200', '100'])
  })

  it('marks a climber with the glyph, the count, the accessible label AND the row fill', () => {
    // UX-DR14 forbids colour as the sole signal. Asserting only the fill —
    // or only the glyph — would pass the exact bug the rule exists to
    // forbid, so all three are asserted together.
    const previous: PreviousStandings = {
      a: { rank: 3, index: 2 },
      b: { rank: 1, index: 0 },
      c: { rank: 2, index: 1 },
    }
    const { container } = renderStage(snap(), previous)
    const climber = rowsOf(container)[0]!

    expect(moverOf(climber)).toBe(`${copy.moverMark}2`)
    expect(srLabelOf(climber)).toBe(copy.climbedLabel(2))
    expect(climber.className).toContain('bg-green-50')
  })

  it('gives a faller no indicator, no label and no row fill', () => {
    // EXPERIENCE.md marks rows that CLIMBED; AC-1 says "highlight climbers".
    // Falling is visible in the reshuffle itself, not in a second colour.
    const previous: PreviousStandings = {
      a: { rank: 3, index: 2 },
      b: { rank: 1, index: 0 },
      c: { rank: 2, index: 1 },
    }
    const { container } = renderStage(snap(), previous)
    const faller = rowsOf(container)[1]! // b went 1 -> 2

    expect(moverOf(faller)).toBe('')
    expect(srLabelOf(faller)).toBeNull()
    expect(faller.className).not.toContain('bg-green-50')
  })

  it('reserves the mover column on every row, so the names stay in one column', () => {
    // A conditionally-rendered mark pushes that one row's name out of the
    // column every other row shares — 4.4's review found exactly this on
    // the bar labels.
    const { container } = renderStage(snap(), { a: { rank: 3, index: 2 } })

    for (const row of rowsOf(container)) {
      expect(row.querySelector('[aria-hidden="true"]')).not.toBeNull()
    }
  })

  it('shows no indicators and animates nothing without a baseline', () => {
    // A freshly opened or reloaded display, and the first Leaderboard of
    // every game. The known limit: this looks exactly like "nobody climbed".
    const { container } = renderStage(snap(), undefined)

    for (const row of rowsOf(container)) {
      expect(moverOf(row)).toBe('')
      expect(row.className).not.toContain('stage-row-reshuffle')
      expect(shiftOf(row)).toBe('')
    }
  })

  it('animates nothing when the order is unchanged, even with a baseline', () => {
    // The case that separates "has a baseline" from "actually moved".
    const previous: PreviousStandings = {
      a: { rank: 1, index: 0 },
      b: { rank: 2, index: 1 },
      c: { rank: 3, index: 2 },
    }
    const { container } = renderStage(snap(), previous)

    for (const row of rowsOf(container)) {
      expect(row.className).not.toContain('stage-row-reshuffle')
      expect(shiftOf(row)).toBe('')
      expect(moverOf(row)).toBe('')
    }
  })

  it('sets --stage-row-shift to previousIndex - currentIndex, positive when the row climbed', () => {
    // The reshuffle is arithmetic, not measurement: every row is exactly one
    // row height, so the travel is a whole number of row heights. POSITIVE
    // means the row starts BELOW its final slot, i.e. it climbed.
    const previous: PreviousStandings = {
      a: { rank: 3, index: 2 },
      b: { rank: 1, index: 0 },
      c: { rank: 2, index: 1 },
    }
    const { container } = renderStage(snap(), previous)
    const rows = rowsOf(container)

    expect(shiftOf(rows[0]!)).toBe('2') // a: slot 2 -> 0
    expect(shiftOf(rows[1]!)).toBe('-1') // b: slot 0 -> 1
    expect(shiftOf(rows[2]!)).toBe('-1') // c: slot 1 -> 2
    expect(rows[0]!.className).toContain('stage-row-reshuffle')
  })

  it('clamps a row arriving from below the fold to just past the last visible slot', () => {
    // Math.min(previous.index, maxRows) caps where the row STARTS, before
    // the subtraction — so it slides in from just past slot 10 rather than
    // from forty rows away, off screen for most of the animation.
    //
    // The landing slot has to be non-zero to see it: into slot 0 the outer
    // clamp alone already yields 10 and the two forms agree. Into slot 5 they
    // do not — min(13,10)-5 = 5, but 13-5 = 8 survives the clamp untouched.
    const board = Array.from({ length: 12 }, (_, i) => entry(`p${i + 1}`, i + 1, 1200 - i * 100))
    const { container } = renderStage(snap({ leaderboard: board }), {
      p6: { rank: 14, index: 13 },
    })

    expect(shiftOf(rowsOf(container)[5]!)).toBe('5')
  })

  it('caps the start slot at the fold, so even an arrival into slot 0 starts just off the board', () => {
    // The companion to the case above, at the other landing slot. This used
    // to be titled "clamps the shift in both directions" and asserted
    // nothing: with the old outer clamp in place, min(13,10)-0, clamp(13-0)
    // and 13-0 all produced 10, so no mutation could fail it. The clamp was
    // removed at code review (2026-08-12) — it could never engage, because
    // the shift is bounded by construction at [-(maxRows - 1), maxRows] —
    // and with it gone this now discriminates: without Math.min the shift
    // would be 13, not 10. There is no "other direction" left to test.
    const board = Array.from({ length: 12 }, (_, i) => entry(`p${i + 1}`, i + 1, 1200 - i * 100))
    const { container } = renderStage(snap({ leaderboard: board }), {
      p1: { rank: 14, index: 13 },
    })

    expect(shiftOf(rowsOf(container)[0]!)).toBe('10')
  })

  it('gives a row that moved without changing rank no indicator — rank delta, not slot delta', () => {
    // The tie case Dev Notes item 2 predicted nobody would test. When every
    // player answers question 1 correctly the whole board ties at rank 1, so
    // at the next showing the new leader's RANK is unchanged while its SLOT
    // moves to the top: it flies up the board carrying no ▲, no label and no
    // fill. This pins the specified behaviour (derived requirement 6 makes
    // "places climbed" a rank delta) rather than changing it — the two
    // signals genuinely disagree here, and that is a recorded design call.
    const previous: PreviousStandings = {
      a: { rank: 1, index: 0 },
      b: { rank: 1, index: 1 },
      c: { rank: 1, index: 2 },
    }
    const { container } = renderStage(
      snap({ leaderboard: [entry('c', 1, 300), entry('a', 2, 200), entry('b', 3, 100)] }),
      previous,
    )
    const mover = rowsOf(container)[0]!

    expect(moverOf(mover)).toBe('')
    expect(srLabelOf(mover)).toBeNull()
    expect(mover.className).not.toContain('bg-green-50')
    // …and yet it is the row that visibly travels the furthest.
    expect(shiftOf(mover)).toBe('2')
    expect(mover.className).toContain('stage-row-reshuffle')
  })

  it('gives a row that gained rank into a tie an indicator but no movement', () => {
    // The mirror of the case above, and the other half of the divergence:
    // b catches a up exactly, so its rank improves 2 -> 1 while its slot does
    // not change at all. It carries the full climber treatment and stays
    // still. Same recorded design call, asserted from the other side.
    const previous: PreviousStandings = {
      a: { rank: 1, index: 0 },
      b: { rank: 2, index: 1 },
      c: { rank: 3, index: 2 },
    }
    const { container } = renderStage(
      snap({ leaderboard: [entry('a', 1, 300), entry('b', 1, 300), entry('c', 3, 100)] }),
      previous,
    )
    const tied = rowsOf(container)[1]!

    expect(moverOf(tied)).toBe(`${copy.moverMark}1`)
    expect(srLabelOf(tied)).toBe(copy.climbedLabel(1))
    expect(tied.className).toContain('bg-green-50')
    expect(shiftOf(tied)).toBe('')
    expect(tied.className).not.toContain('stage-row-reshuffle')
  })

  it('drops the animation under reduced motion while keeping the final order', () => {
    // "Static reordering when reduced", verbatim. The ORDER is what must
    // survive — asserting only the absent class would pass a stage that
    // rendered the rows in their old positions and never moved them.
    const previous: PreviousStandings = {
      a: { rank: 3, index: 2 },
      b: { rank: 1, index: 0 },
      c: { rank: 2, index: 1 },
    }
    const { container } = renderStage(snap(), previous, true)
    const rows = rowsOf(container)

    for (const row of rows) {
      expect(row.className).not.toContain('stage-row-reshuffle')
    }
    expect(rows.map(nameOf)).toEqual(['PLAYER-a', 'PLAYER-b', 'PLAYER-c'])
  })

  it('keeps the mover cues under reduced motion — accessibility is not part of the animation', () => {
    const { container } = renderStage(snap(), { a: { rank: 3, index: 2 } }, true)
    const climber = rowsOf(container)[0]!

    expect(moverOf(climber)).toBe(`${copy.moverMark}2`)
    expect(srLabelOf(climber)).toBe(copy.climbedLabel(2))
    expect(climber.className).toContain('bg-green-50')
  })

  it('renders the waiting copy and NO board on a degraded frame', () => {
    // emptySnapshot: the real state, a nil currentQuestion, a zero
    // questionCount and an empty leaderboard. An empty board is the one
    // degraded field that looks like a real value ("everyone scored 0").
    const { container } = renderStage(
      snap({ questionCount: 0, currentQuestion: null, leaderboard: [] }),
    )

    expect(container.textContent).toContain(strings.display.waiting)
    expect(rowsOf(container)).toHaveLength(0)
  })

  it('renders the waiting copy when the question is missing but the count is not', () => {
    // Either half of the correlation is enough — a post-commit buildSnapshot
    // failure can deliver one without the other.
    const { container } = renderStage(snap({ currentQuestion: null, leaderboard: [] }))

    expect(container.textContent).toContain(strings.display.waiting)
    expect(rowsOf(container)).toHaveLength(0)
  })

  it('renders the waiting copy when the count is zero but the question is not', () => {
    // The OTHER half, which nothing exercised until this review: deleting
    // `questionCount === 0` from isDegradedFrame left the entire suite green,
    // because the two existing cases both drive the currentQuestion half.
    // Unreachable from today's emptySnapshot, which sets both together — but
    // this correlation is what story 4.5 chose INSTEAD of a wire-contract
    // change, so both disjuncts have to be pinned or the choice is only half
    // defended. (Code review, 2026-08-12.)
    const { container } = renderStage(snap({ questionCount: 0, leaderboard: [] }))

    expect(container.textContent).toContain(strings.display.waiting)
    expect(rowsOf(container)).toHaveLength(0)
  })

  it('renders the no-players copy — a DIFFERENT sentence — on a healthy but empty board', () => {
    // The room is being told two different true things: "we cannot read the
    // board" and "nobody registered". Collapsing them is the trap
    // deferred-work.md's 3.7 entry is about.
    const { container } = renderStage(snap({ leaderboard: [] }))

    expect(container.textContent).toContain(copy.noPlayers)
    expect(container.textContent).not.toContain(strings.display.waiting)
    expect(rowsOf(container)).toHaveLength(0)
  })

  it('truncates a long display name to a single line rather than growing the row', () => {
    // A class assertion standing in for a behaviour jsdom cannot measure:
    // the reshuffle arithmetic is only exact while every row is the same
    // height, and a wrapping name makes one row taller and every shift below
    // it wrong by the difference. The browser pass measures the real thing.
    const long = 'PLAYER-' + 'X'.repeat(200)
    const { container } = renderStage(
      snap({ leaderboard: [{ participantId: 'a', displayName: long, score: 300, rank: 1 }] }),
    )
    const name = rowsOf(container)[0]!.querySelector('bdi.truncate')

    expect(name).not.toBeNull()
    expect(name!.textContent).toBe(long)
  })

  it('renders no participant id, phone number or roster name from snapshot.participants', () => {
    // Derived requirement 14, asserted rather than hoped. Names and scores
    // ARE this stage's content; everything identifying beyond them is not.
    const { container } = renderStage(
      snap({
        participants: [
          { id: 'ROSTER-ID-1', displayName: 'ROSTER-NAME-1' },
          { id: 'ROSTER-ID-2', displayName: 'ROSTER-NAME-2' },
        ],
      }),
    )

    expect(container.textContent).not.toContain('ROSTER-NAME-1')
    expect(container.textContent).not.toContain('ROSTER-ID-1')
    // The participantIds of the RENDERED rows must not leak either.
    expect(container.textContent).not.toContain('+972')
    for (const id of ['a', 'b', 'c']) {
      expect(container.querySelector(`[data-participant-id="${id}"]`)).toBeNull()
    }
    // ...but the names ARE present. Unlike 4.4, they are the point here.
    expect(container.textContent).toContain('PLAYER-a')
  })

  it('renders an ordered list, so the ranking carries real list semantics', () => {
    const { container } = renderStage(snap())

    expect(container.querySelector('ol')).not.toBeNull()
    // No heading: no vertical budget for one, none specified, and the shell
    // already announces the state assertively on entry.
    expect(container.querySelector('h1, h2, h3')).toBeNull()
  })
})
