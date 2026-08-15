import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render } from '@testing-library/react'

import { strings } from '@/lib/strings.he'
import type { LeaderboardEntry, LobbySnapshot } from '@/lib/types'
// Imported by the TEST, deliberately, and never by winner-stage.tsx: derived
// requirement 5 is a claim about how this helper and this stage DISAGREE at
// `finished`, and a claim about two things cannot be asserted while only one
// of them is in scope.
import { isDegradedFrame } from './stage-props'
import { WinnerStage } from './winner-stage'

// RTL's automatic cleanup only self-registers when a global afterEach
// exists, and vitest `globals` is off here.
afterEach(cleanup)

// Expected copy is READ FROM strings.he.ts, never retyped (4.2's rule).
const copy = strings.display.winner

// Participant display names are Organizer-facing DATA, not copy. Latin
// placeholders on purpose: a Hebrew fixture would put this file in the
// frontend copy-centralization grep for a string carrying no meaning to any
// assertion here.
function entry(id: string, rank: number, score: number): LeaderboardEntry {
  return { participantId: id, displayName: `PLAYER-${id}`, score, rank }
}

// The one field on this snapshot that really does carry a phone number.
const platformNumber = '+972 50-000-0000'

/** A REAL finished frame, and every field of it matters.
 *
 *  `currentQuestion: null` is not a degraded value here — FinishGame resets
 *  current_question_position to 0 on every transition into `finished`, so
 *  buildSnapshot leaves the field nil on every healthy frame at this state.
 *  `questionCount: 5` is what actually distinguishes this from emptySnapshot
 *  (story 4.6, derived requirement 5). */
function snap(overrides: Partial<LobbySnapshot> = {}): LobbySnapshot {
  return {
    gameId: 'game-1',
    state: 'finished',
    joinCode: 'COHEN24',
    platformNumber,
    participantCount: 3,
    participants: [],
    questionCount: 5,
    currentQuestion: null,
    leaderboard: [entry('a', 1, 1240), entry('b', 2, 800), entry('c', 3, 300)],
    displaySettings: { reducedMotion: false },
    ...overrides,
  }
}

function renderStage(snapshot: LobbySnapshot, reducedMotion = false) {
  return render(<WinnerStage snapshot={snapshot} reducedMotion={reducedMotion} />)
}

// Structural reads rather than getByText: getByText joins only an element's
// DIRECT text-node children, so the stacked names inside the <h1> match on
// neither the heading nor the individual lines (4.4's harness finding).
//
// Both selectors are scoped by PARENT because the names and the score are
// now both <bdi> (code review, 2026-08-14 — the name carries the same
// free-form-participant-text isolation leaderboard-stage.tsx gives the
// identical field). A bare `bdi` selector would fold the two together and
// make every "score shown exactly once" assertion pass vacuously.
function namesOf(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll('h1 bdi')).map((node) => node.textContent ?? '')
}
function scoreLinesOf(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll('p bdi')).map((node) => node.textContent ?? '')
}
function confettiOf(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll('[aria-hidden="true"] span'))
}
function surfaceOf(container: HTMLElement): HTMLElement {
  return container.firstElementChild as HTMLElement
}

describe('WinnerStage', () => {
  it('names the single rank-1 winner and shows their score, and nobody else', () => {
    const { container } = renderStage(snap())

    expect(namesOf(container)).toEqual(['PLAYER-a'])
    expect(scoreLinesOf(container)).toEqual([copy.scoreSuffix(1240)])
    // The runners-up are on the same leaderboard this stage filtered — a
    // filter that renders the whole board would still pass the assertion
    // above.
    expect(container.textContent).not.toContain('PLAYER-b')
    expect(container.textContent).not.toContain('PLAYER-c')
    expect(container.textContent).toContain(copy.tagline)
  })

  it('stacks two tied winners and shows the score exactly ONCE', () => {
    // A16 and DESIGN.md both say "score shown once" — asserting only that
    // both names render would pass a stage that repeated the score per name.
    const { container } = renderStage(
      snap({ leaderboard: [entry('a', 1, 900), entry('b', 1, 900), entry('c', 3, 100)] }),
    )

    expect(namesOf(container)).toEqual(['PLAYER-a', 'PLAYER-b'])
    expect(scoreLinesOf(container)).toEqual([copy.scoreSuffix(900)])
  })

  it('caps a four-way tie at three names, in the order the server delivered them', () => {
    // Derived requirement 9: the fourth is CUT, with no "+N more". Reachable
    // in a small pilot game, and resolved by neither DESIGN.md nor
    // EXPERIENCE.md beyond "up to three".
    //
    // The delivered order is deliberately NOT alphabetical: RankLeaderboard
    // breaks ties stably by joined_at, and a stage that re-sorted would
    // produce a different three.
    const { container } = renderStage(
      snap({
        leaderboard: [
          entry('d', 1, 700),
          entry('a', 1, 700),
          entry('c', 1, 700),
          entry('b', 1, 700),
        ],
      }),
    )

    expect(namesOf(container)).toEqual(['PLAYER-d', 'PLAYER-a', 'PLAYER-c'])
    expect(container.textContent).not.toContain('PLAYER-b')
    expect(scoreLinesOf(container)).toEqual([copy.scoreSuffix(700)])
  })

  it('does NOT congratulate a rank-1 player whose score is zero', () => {
    // The case that proves the strict positivity half of game/final.go's
    // rule, not merely the rank half. StopGame from question_open finishes a
    // game before any Reveal: every player sits at 0 and RankLeaderboard
    // hands all of them rank 1. A rank-only filter would put the whole room
    // on the winner card.
    const { container } = renderStage(
      snap({ leaderboard: [entry('a', 1, 0), entry('b', 1, 0), entry('c', 1, 0)] }),
    )

    expect(namesOf(container)).toEqual([])
    expect(container.textContent).toContain(copy.noWinner)
    expect(container.textContent).not.toContain('PLAYER-a')
  })

  it('renders the no-winner copy and NO confetti on a healthy but empty board', () => {
    // The second route to zero winners: nobody registered at all, so
    // GetLeaderboard returns no rows. Same UI fact, same one sentence —
    // messages_he.go's msgFinalResultsNoWinner treats the two identically
    // for the same reason.
    const { container } = renderStage(snap({ leaderboard: [] }))

    expect(container.textContent).toContain(copy.noWinner)
    expect(container.textContent).not.toContain(strings.display.waiting)
    expect(confettiOf(container)).toHaveLength(0)
    expect(container.querySelector('h1')).toBeNull()
  })

  it('renders the WAITING copy — not the no-winner copy — on a degraded frame', () => {
    // emptySnapshot: the real State with QuestionCount at its Go zero value
    // and an empty leaderboard. Two different true things get two different
    // sentences: "we cannot read this frame" and "nobody scored".
    const { container } = renderStage(snap({ questionCount: 0, leaderboard: [] }))

    expect(container.textContent).toContain(strings.display.waiting)
    expect(container.textContent).not.toContain(copy.noWinner)
    expect(confettiOf(container)).toHaveLength(0)
  })

  it('splits two frames that isDegradedFrame would call identical', () => {
    // THE case that proves derived requirement 5, and the reason this stage
    // must not reuse stage-props.ts's isDegradedFrame(). That helper is
    // `questionCount === 0 || currentQuestion === null`, and the second
    // disjunct is unconditionally true at `finished` — FinishGame resets
    // current_question_position to 0, so buildSnapshot leaves currentQuestion
    // nil on every real frame at this state.
    //
    // Stated as an explicit CONTRAST rather than as a single positive case
    // (code review, 2026-08-14). snap()'s own defaults are already
    // `currentQuestion: null, questionCount: 5`, so a test passing those two
    // values as overrides was a no-op override — byte-identical input to the
    // first case in this file, carrying no discriminating power of its own
    // while being labelled as the case that proves the requirement. What
    // actually proves it is that these two frames DIVERGE: isDegradedFrame
    // returns true for both, and the stage must return true for only one.
    const healthy = renderStage(snap({ currentQuestion: null, questionCount: 5 }))
    expect(isDegradedFrame(snap({ currentQuestion: null, questionCount: 5 }))).toBe(true)
    expect(namesOf(healthy.container)).toEqual(['PLAYER-a'])
    expect(healthy.container.textContent).not.toContain(strings.display.waiting)

    cleanup()

    const degraded = renderStage(snap({ currentQuestion: null, questionCount: 0, leaderboard: [] }))
    expect(isDegradedFrame(snap({ currentQuestion: null, questionCount: 0, leaderboard: [] }))).toBe(
      true,
    )
    expect(namesOf(degraded.container)).toEqual([])
    expect(degraded.container.textContent).toContain(strings.display.waiting)
  })

  it('renders all 22 confetti pieces, decorative and animating, when motion is allowed', () => {
    const { container } = renderStage(snap())
    const pieces = confettiOf(container)

    // The mockup's exact count — a transcription that dropped or duplicated
    // a piece is a different picture from the one that was reviewed.
    expect(pieces).toHaveLength(22)
    for (const piece of pieces) {
      expect(piece.className).toContain('stage-confetti-piece')
      // `absolute` is unconditional, unlike the animation class: without it
      // the reduced-motion frame below would drop every piece into flow.
      expect(piece.className).toContain('absolute')
      expect(piece.style.animationDuration).not.toBe('')
      expect(piece.style.animationDelay).not.toBe('')
      expect(piece.style.left).not.toBe('')
      expect(piece.style.top).not.toBe('')
    }

    // The overlay itself is decoration, not a click target: it is inset-0
    // over the whole stage, so without this it would swallow pointer events
    // everywhere the content box does not cover (the mockup's fourth
    // declaration on .confetti, restored at code review 2026-08-14).
    const overlay = container.querySelector('[aria-hidden="true"]')
    expect(overlay?.className).toContain('pointer-events-none')
  })

  it('scatters the pieces the way the mockup does — distinct positions, all delays negative', () => {
    // Everything above this case is satisfied by 22 IDENTICAL pieces stacked
    // on one coordinate and moving in unison, which is precisely the picture
    // winner-stage.tsx's own comment says the verbatim transcription exists
    // to prevent (code review, 2026-08-14). `not.toBe('')` cannot see a
    // copy-paste error; only distinctness and sign can.
    const { container } = renderStage(snap())
    const pieces = confettiOf(container)

    const coordinates = pieces.map((piece) => `${piece.style.left}/${piece.style.top}`)
    expect(new Set(coordinates).size).toBe(22)

    for (const piece of pieces) {
      // Negative, always: a positive delay would queue all 22 pieces above
      // the screen, so the first painted frame — and the whole
      // reduced-motion frame, which never advances past it — would be an
      // empty green rectangle rather than a scattered celebration.
      expect(piece.style.animationDelay.startsWith('-')).toBe(true)
      expect(Number.parseFloat(piece.style.animationDuration)).toBeGreaterThan(0)
    }
  })

  it('paints every piece in one of the two permitted colours, with a shape', () => {
    // Deleting the colour and shape classes leaves 22 zero-size transparent
    // spans and passes every other assertion in this file — the celebration
    // becomes invisible while the suite stays green (code review,
    // 2026-08-14). UX-DR4 permits gold and green-600 and nothing else.
    const { container } = renderStage(snap())
    const pieces = confettiOf(container)

    const gold = pieces.filter((piece) => piece.className.includes('bg-gold'))
    const green = pieces.filter((piece) => piece.className.includes('bg-green-600'))
    // The mockup's own split, and the same 13/9 the browser pass measured.
    expect(gold).toHaveLength(13)
    expect(green).toHaveLength(9)

    for (const piece of pieces) {
      // Every piece has a measure. Without one it renders at 0x0 and the
      // colour above is unobservable.
      expect(piece.className).toMatch(/h-\[[\d.]+vw\]/)
      expect(piece.className).toMatch(/w-\[[\d.]+vw\]/)
    }
  })

  it('drops the confetti animation under reduced motion while keeping the pieces and the content', () => {
    // "Static celebratory frame", verbatim from EXPERIENCE.md. Asserting only
    // the absent class would pass a stage that removed the pieces entirely,
    // and asserting only their presence would pass one that kept animating.
    const { container } = renderStage(snap(), true)
    const pieces = confettiOf(container)

    expect(pieces).toHaveLength(22)
    for (const piece of pieces) {
      expect(piece.className).not.toContain('stage-confetti-piece')
      expect(piece.className).toContain('absolute')
      // The scatter survives — this is what makes it a frame rather than a
      // blank green screen.
      expect(piece.style.left).not.toBe('')
      expect(piece.style.top).not.toBe('')
    }
    expect(namesOf(container)).toEqual(['PLAYER-a'])
    expect(scoreLinesOf(container)).toEqual([copy.scoreSuffix(1240)])
  })

  it('renders the winner name as an <h1>', () => {
    // Closes deferred-work.md's heading-semantics entry (derived requirement
    // 8) — the ELEMENT is the deliverable, so the tag is asserted and not
    // just the text.
    const { container } = renderStage(snap())
    const heading = container.querySelector('h1')

    expect(heading).not.toBeNull()
    expect(heading!.textContent).toBe('PLAYER-a')
    // This component renders exactly one heading of its own. The shell keeps
    // one stage mounted at a time, so this is also the page's only heading —
    // but that half is display-page.test.tsx's to assert, not this file's,
    // which mounts the stage in isolation and could never observe a second.
    expect(container.querySelectorAll('h1, h2, h3')).toHaveLength(1)
  })

  it('isolates and wraps the winner name, which is free-form participant text', () => {
    // The same two guards leaderboard-stage.tsx gives the IDENTICAL field
    // (code review, 2026-08-14). display_name is uncapped TEXT and
    // parseRenameName accepts any length, so both hazards are participant-
    // reachable from WhatsApp.
    const { container } = renderStage(
      snap({ leaderboard: [{ participantId: 'a', displayName: '!David', score: 500, rank: 1 }] }),
    )
    const name = container.querySelector('h1 bdi')

    // <bdi>, so a leading neutral does not resolve to the RTL paragraph
    // level and jump to the far end — "!David" painting as "David!".
    expect(name).not.toBeNull()
    expect(name!.tagName).toBe('BDI')
    // break-words, so an unbreakable 200-character name wraps instead of
    // being clipped symmetrically by the surface's text-center +
    // overflow-hidden, which would show the room the MIDDLE of a name with
    // no ellipsis and no sign anything was cut.
    expect(name!.className).toContain('break-words')
    expect(namesOf(container)).toEqual(['!David'])
  })

  it('isolates the score without forcing its direction', () => {
    // <bdi> with no dir attribute, i.e. dir="auto". The composed value is a
    // Hebrew sentence whose first strong character is Hebrew, so dir="ltr"
    // would mirror it and put the digits on the wrong side of the word —
    // the exact defect winner-stage.tsx's comment says it is avoiding, and
    // one that reading textContent alone can never catch (code review,
    // 2026-08-14).
    const { container } = renderStage(snap())
    const score = container.querySelector('p bdi')

    expect(score).not.toBeNull()
    expect(score!.hasAttribute('dir')).toBe(false)
  })

  it('holds the stage surface contract: green-800 ground, gold name, safe margin, content above the confetti', () => {
    // None of these four had an assertion (code review, 2026-08-14), so each
    // could be dropped or changed with the suite staying green: a light
    // ground would put the gold name at ~1.5:1; losing the margin runs the
    // content into projector overscan on all three branches; losing z-10
    // paints 22 confetti pieces over the winner's own name.
    const { container } = renderStage(snap())
    const surface = surfaceOf(container)

    expect(surface.className).toContain('bg-green-800')
    expect(surface.className).toContain('p-[var(--stage-margin)]')
    expect(surface.className).toContain('overflow-hidden')

    // Gold's second and final permitted appearance (UX-DR2/UX-DR8) is the
    // name — and the confetti. Nothing else on this stage may carry it.
    const heading = container.querySelector('h1')
    expect(heading!.className).toContain('text-gold')
    for (const paragraph of Array.from(container.querySelectorAll('p'))) {
      expect(paragraph.className).not.toContain('gold')
    }

    const content = container.querySelector('.z-10')
    expect(content).not.toBeNull()
    expect(content!.contains(heading)).toBe(true)
  })

  it('keeps the green-800 surface and the safe margin on both message branches', () => {
    // The two fallbacks are full-bleed takeovers too, not bare text on the
    // shell's own ground — and neither had an assertion about it.
    for (const frame of [snap({ leaderboard: [] }), snap({ questionCount: 0, leaderboard: [] })]) {
      const { container } = renderStage(frame)
      const surface = surfaceOf(container)

      expect(surface.className).toContain('bg-green-800')
      expect(surface.className).toContain('p-[var(--stage-margin)]')
      cleanup()
    }
  })

  it('renders nothing from snapshot.participants — no roster name, no id, no phone number', () => {
    // Derived requirement 14, asserted rather than trusted. The winner IS
    // the content here, but everything identifying beyond a name and a score
    // still is not.
    const { container } = renderStage(
      snap({
        participants: [
          { id: 'ROSTER-ID-1', displayName: 'ROSTER-NAME-1' },
          { id: 'ROSTER-ID-2', displayName: 'ROSTER-NAME-2' },
        ],
        // The winner's id is a distinctive token that shares no substring
        // with their rendered name, so an id leak is genuinely observable.
        leaderboard: [
          { participantId: 'PID-LEAK-CANARY', displayName: 'WINNER-NAME', score: 1240, rank: 1 },
          entry('b', 2, 800),
        ],
      }),
    )

    // The name IS the content — the assertions below must be failing for the
    // right reason, not because nothing rendered at all.
    expect(namesOf(container)).toEqual(['WINNER-NAME'])
    expect(container.textContent).not.toContain('ROSTER-NAME-1')
    expect(container.textContent).not.toContain('ROSTER-ID-1')
    // Four of this case's assertions used to be vacuous (code review,
    // 2026-08-14): `+972` cannot come from the participants fixture, which
    // has no phone field at all; `game-1` guards a field no branch reads;
    // and `[data-participant-id]` is an attribute this component has never
    // emitted anywhere, so it was unconditionally true and would stay true
    // under a refactor that leaked the id as text. Replaced with checks that
    // can actually fail.
    //
    // platformNumber is the field that really does carry a phone number onto
    // every snapshot, so it is the one worth fencing.
    expect(container.innerHTML).not.toContain(platformNumber)
    // The winner's own participantId is a React key, never rendered content
    // — asserted against innerHTML, so an id leaked into an attribute is
    // caught as well as one leaked into text. The id is deliberately a
    // distinctive token: the old fixture used 'a', which appears inside
    // "PLAYER-a" and inside a dozen class names, so no assertion about it
    // could ever have failed.
    expect(container.innerHTML).not.toContain('PID-LEAK-CANARY')
    expect(container.innerHTML).not.toContain('game-1')
  })
})
