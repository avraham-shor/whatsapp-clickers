import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'

import { strings } from '@/lib/strings.he'
import type { CurrentQuestion, LobbySnapshot, QuestionReveal } from '@/lib/types'
import { RevealStage } from './reveal-stage'

// RTL's automatic cleanup only self-registers when a global afterEach
// exists, and vitest `globals` is off here — without this line the second
// test renders into the DOM the first one left behind.
afterEach(cleanup)

// Expected copy is READ FROM strings.he.ts, never retyped (4.2's rule).
const revealCopy = strings.display.reveal
const questionCopy = strings.display.question
const letters = strings.questionEditor.optionLetters

// The stage renders a TimerRing (via StageHeroBand), which reads Date.now()
// during render. A fixed clock keeps that deterministic; timer-ring.test.tsx
// owns the countdown itself.
const baseNow = Date.parse('2026-08-11T12:00:00Z')
const cutoff = new Date(baseNow - 1000).toISOString()

// Question text and options are Organizer-authored DATA, not copy. Latin
// placeholders on purpose: a Hebrew fixture would put this file in the
// frontend copy-centralization grep for a string that carries no meaning to
// the assertion. (Every expected STRING below is still read from
// strings.he.ts.)
function question(overrides: Partial<CurrentQuestion> = {}): CurrentQuestion {
  return {
    id: 'q-1',
    position: 3,
    type: 'mcq',
    text: 'QUESTION-TEXT',
    options: ['OPTION-A', 'OPTION-B', 'OPTION-C', 'OPTION-D'],
    timeLimitSeconds: 20,
    answerCutoffAt: cutoff,
    answeredCount: 10,
    reveal: null,
    ...overrides,
  }
}

function reveal(overrides: Partial<QuestionReveal> = {}): QuestionReveal {
  return { correctOption: 2, optionCounts: [2, 5, 0, 3], correctCount: 5, ...overrides }
}

function snap(currentQuestion: CurrentQuestion | null): LobbySnapshot {
  return {
    gameId: 'game-1',
    state: 'revealed',
    joinCode: 'COHEN24',
    platformNumber: '+972 50-000-0000',
    participantCount: 12,
    participants: [],
    questionCount: 10,
    currentQuestion,
    leaderboard: [],
    displaySettings: { reducedMotion: false },
  }
}

/** Renders the stage under a fixed clock and returns the container. */
function renderStage(current: CurrentQuestion | null, reducedMotion = false) {
  vi.useFakeTimers()
  vi.setSystemTime(baseNow)
  try {
    return render(<RevealStage snapshot={snap(current)} reducedMotion={reducedMotion} />)
  } finally {
    vi.useRealTimers()
  }
}

/** The pill elements, in option order — the div carrying the option text. */
function pills(container: HTMLElement): HTMLElement[] {
  return (strings.questionEditor.optionLetters as readonly string[]).map((_, index) => {
    const text = `OPTION-${String.fromCharCode(65 + index)}`
    const node = screen.getByText(text).parentElement
    if (!node) throw new Error(`no pill for ${text} in ${container.innerHTML}`)
    return node
  })
}

describe('RevealStage', () => {
  // AC-1 + UX-DR14: colour is never the sole signal. Asserting only the fill
  // would pass the exact bug the rule exists to forbid, so both the success
  // treatment AND the ✓'s accessible name are asserted here.
  it('marks the correct option with the success fill AND an aria-labeled ✓', () => {
    const { container } = renderStage(question({ reveal: reveal() }))

    const [a, b, c, d] = pills(container)
    expect(b?.className).toContain('bg-success')
    // The accessible name, not the glyph: a bare "✓" read aloud is not a
    // label. Exactly one, on the correct option.
    expect(screen.getAllByText(revealCopy.correctOptionLabel)).toHaveLength(1)
    for (const wrong of [a, c, d]) {
      expect(wrong?.className).not.toContain('bg-success')
    }

    // Derived req. 10's OTHER half, asserted directly rather than implied.
    // Counting the sr-only label above cannot see it: the bar label renders
    // the GLYPH, never the label text, so stripping aria-hidden from the bar
    // label left that assertion green while the row announced itself twice.
    // (Code review, 2026-08-12.)
    const marks = [...container.querySelectorAll('span')].filter(
      (node) => node.textContent === revealCopy.correctMark,
    )
    expect(marks).toHaveLength(2) // the pill's and the bar label's
    for (const mark of marks) {
      expect(mark.getAttribute('aria-hidden')).toBe('true')
    }
  })

  // AC-1: wrong options demote to stage-option-dimmed. "Dim the fill, never
  // the text" (DESIGN.md Don'ts) — an opacity-* would fail contrast on a
  // projector, so its absence is asserted rather than assumed.
  it('demotes the three wrong options to the dimmed fill and never to opacity', () => {
    const { container } = renderStage(question({ reveal: reveal() }))

    const [a, b, c, d] = pills(container)
    for (const wrong of [a, c, d]) {
      expect(wrong?.className).toContain('bg-green-100')
      expect(wrong?.className).toContain('text-text-primary')
      expect(wrong?.className).not.toMatch(/\bopacity-/)
    }
    expect(b?.className).not.toContain('bg-green-100')
  })

  // Derived req. 8, the load-bearing one: the denominator is the SUM OF THE
  // BARS, never answeredCount. Driven with the floored case the shell really
  // produces — answeredCount deliberately raised above the sum — so a
  // denominator swap moves every individual label.
  //
  // (The comment here used to say the swap "shows up as percentages that no
  // longer total 100". That was never the mechanism — before the apportionment
  // fix, rounding alone broke the 100 sum; after it, the sum is 100 whatever
  // the denominator. The four labels below are what actually catch a swap.
  // Code review, 2026-08-12.)
  it('computes percentages off the sum of the bars, not off answeredCount', () => {
    // 2 + 5 + 0 + 3 = 10 bars, but the shell's floor says 25 answered.
    renderStage(question({ answeredCount: 25, reveal: reveal() }))

    expect(screen.getByText(revealCopy.distributionLabel(2, 20))).toBeTruthy()
    expect(screen.getByText(revealCopy.distributionLabel(5, 50))).toBeTruthy()
    expect(screen.getByText(revealCopy.distributionLabel(0, 0))).toBeTruthy()
    expect(screen.getByText(revealCopy.distributionLabel(3, 30))).toBeTruthy()
  })

  // The four figures sit side by side on a projector, so a room can add them.
  // Four independent Math.round calls do not add up — these two splits are the
  // exact cases that produced 99% and 101% before largest-remainder
  // apportionment, and neither is exotic. (Code review, 2026-08-12.)
  it('apportions percentages so the four figures always total exactly 100', () => {
    const cases: [counts: number[], expected: number[]][] = [
      [
        [1, 1, 1, 0],
        [34, 33, 33, 0],
      ],
      [
        [2, 2, 2, 1],
        [29, 29, 28, 14],
      ],
    ]
    for (const [counts, expected] of cases) {
      cleanup()
      const total = counts.reduce((sum, n) => sum + n, 0)
      const { container } = renderStage(
        question({ answeredCount: total, reveal: reveal({ optionCounts: counts, correctCount: 1 }) }),
      )
      expect(expected.reduce((sum, n) => sum + n, 0)).toBe(100)
      // Read the four labels IN ORDER rather than querying by text: two rows
      // legitimately carry the same figure here, which is exactly the shape
      // getByText cannot express.
      const labels = [...container.querySelectorAll('bdi')].map((node) => node.textContent)
      expect(labels).toEqual(
        counts.map((count, index) => revealCopy.distributionLabel(count, expected[index])),
      )
    }
  })

  // A count above zero must never render an invisible bar: the label beside it
  // says somebody chose this option. The LABEL still carries the true
  // percentage — only the bar is floored. (Code review, 2026-08-12.)
  it('floors a nonzero bar to a visible width while the label keeps the true percent', () => {
    const { container } = renderStage(
      question({ answeredCount: 400, reveal: reveal({ optionCounts: [1, 0, 0, 399], correctCount: 1 }) }),
    )

    expect(screen.getByText(revealCopy.distributionLabel(1, 0))).toBeTruthy()
    const widths = [...container.querySelectorAll<HTMLElement>('[style*="width"]')].map(
      (node) => node.style.width,
    )
    expect(widths[0]).not.toBe('0%')
    expect(Number.parseFloat(widths[0] ?? '0')).toBeGreaterThan(0)
    // The option nobody chose stays at zero — the floor is for real answers.
    expect(widths[1]).toBe('0%')
    expect(widths[2]).toBe('0%')
  })

  // total === 0 is a real frame: a question nobody answered. A bare
  // count/total would put "NaN%" on a projector.
  it('renders 0 · 0% on every row and no NaN when nobody answered', () => {
    const { container } = renderStage(
      question({ answeredCount: 0, reveal: reveal({ optionCounts: [0, 0, 0, 0], correctCount: 0 }) }),
    )

    expect(screen.getAllByText(revealCopy.distributionLabel(0, 0))).toHaveLength(4)
    expect(container.textContent).not.toContain('NaN')
  })

  // Deploy skew / a malformed row can deliver fewer counts than options.
  // noUncheckedIndexedAccess is off, so the type says `number` and only this
  // test proves the ?? 0 guard is really there.
  it('fills the missing bars with 0 when optionCounts is shorter than options', () => {
    const { container } = renderStage(
      question({ reveal: reveal({ optionCounts: [4, 6], correctCount: 6 }) }),
    )

    expect(screen.getByText(revealCopy.distributionLabel(4, 40))).toBeTruthy()
    expect(screen.getByText(revealCopy.distributionLabel(6, 60))).toBeTruthy()
    expect(screen.getAllByText(revealCopy.distributionLabel(0, 0))).toHaveLength(2)
    expect(container.textContent).not.toContain('NaN')
  })

  // AC-2 / A15: the answer card replaces the bars, it does not join them.
  it('renders the free-text answer card and counts line with zero bars', () => {
    renderStage(
      question({
        type: 'free_text',
        options: undefined,
        answeredCount: 12,
        reveal: { acceptedAnswer: 'ACCEPTED-ANSWER', correctCount: 7 },
      }),
    )

    expect(screen.getByText('ACCEPTED-ANSWER')).toBeTruthy()
    // The whole composed line, in order — asserting the two halves
    // separately would pass on the hero band's own answeredStat, which
    // renders the same count above, and prove nothing about this stage. The
    // `·` is punctuation, not copy.
    expect(
      screen.getByText(`${strings.live.answeredStat(12)} · ${revealCopy.correctStat(7)}`),
    ).toBeTruthy()
    // Not one option row survives, and therefore not one bar label.
    for (const letter of letters) {
      expect(screen.queryByText(questionCopy.optionLetter(letter))).toBeNull()
    }
  })

  // Derived req. 9: a `revealed` frame whose reveal is null (emptySnapshot,
  // or deploy skew against an instance that predates the field) must render
  // the options UNDECORATED. A wrong mark in front of a room is worse than an
  // unmarked one.
  it('renders the options undecorated, without throwing, when reveal is null', () => {
    let container: HTMLElement | undefined
    expect(() => {
      container = renderStage(question({ reveal: null })).container
    }).not.toThrow()

    for (const pill of pills(container as HTMLElement)) {
      expect(pill.className).not.toContain('bg-success')
      expect(pill.className).not.toContain('bg-green-100')
    }
    // Nothing claims to be the correct answer.
    expect(screen.queryByText(revealCopy.correctOptionLabel)).toBeNull()
  })

  // Code review 2026-08-12. The branch order is
  // `options ? rows : answer ? card : hint`, and the middle arm used to be
  // keyed on `reveal !== null`. Every input that reached it without an answer
  // to show rendered a full-width success card whose entire content was the ✓
  // and the sr-only name — an empty green card asserting it holds the answer.
  // Both entry paths are asserted here because they are one bug.
  it.each([
    ['an mcq that arrives with no options', { type: 'mcq' as const, options: undefined }],
    ['an mcq that arrives with an empty options array', { type: 'mcq' as const, options: [] }],
  ])('falls back to the hint rather than an empty answer card for %s', (_label, shape) => {
    const { container } = renderStage(
      question({ ...shape, reveal: { correctOption: 2, correctCount: 5 } }),
    )

    expect(screen.getByText(questionCopy.freeTextHint)).toBeTruthy()
    expect(container.querySelector('.bg-success')).toBeNull()
    expect(screen.queryByText(revealCopy.correctOptionLabel)).toBeNull()
  })

  // The reachable half of the same bug: `questions_type_shape` guarantees at
  // least one accepted answer, but not that it carries a character. Bank
  // imports bypass the only trimming (deferred-work.md's 3.4 entry), so
  // whitespace is the case that really arrives.
  it.each([[''], ['   ']])(
    'falls back to the hint when the accepted answer is empty or whitespace (%j)',
    (acceptedAnswer) => {
      const { container } = renderStage(
        question({
          type: 'free_text',
          options: undefined,
          reveal: { acceptedAnswer, correctCount: 0 },
        }),
      )

      expect(screen.getByText(questionCopy.freeTextHint)).toBeTruthy()
      expect(container.querySelector('.bg-success')).toBeNull()
    },
  )

  // Code review 2026-08-12. The dimming used to be keyed on `reveal` merely
  // being non-null, so a reveal that identified no option dimmed all four and
  // marked none — the screen asserting every answer was wrong, which is worse
  // than the undecorated state derived req. 9 specifies.
  it.each([
    ['correctOption is absent', undefined],
    ['correctOption is 0', 0],
    ['correctOption is out of range', 9],
  ])('leaves every option undecorated when %s', (_label, correctOption) => {
    const { container } = renderStage(
      question({ reveal: { correctOption, optionCounts: [1, 2, 3, 4], correctCount: 0 } }),
    )

    for (const pill of pills(container)) {
      expect(pill.className).not.toContain('bg-success')
      expect(pill.className).not.toContain('bg-green-100')
    }
    expect(screen.queryByText(revealCopy.correctOptionLabel)).toBeNull()
  })

  // Code review 2026-08-12. The answer card is the one authored string on this
  // stage rendered at 64px in an RTL paragraph; without isolation an answer
  // like "(1948)" has its boundary neutrals mirrored to ")1948(" — a wrong
  // correct answer in front of a room.
  it('isolates the accepted answer for bidi', () => {
    const { container } = renderStage(
      question({
        type: 'free_text',
        options: undefined,
        reveal: { acceptedAnswer: '(1948)', correctCount: 1 },
      }),
    )

    const isolated = [...container.querySelectorAll('bdi')].map((node) => node.textContent)
    expect(isolated).toContain('(1948)')
  })

  // Code review 2026-08-12. The bar label is a flex row starting at the
  // inline-start edge, so rendering the ✓ only on the correct row pushed that
  // row's figure out of the column the fixed basis and tabular-nums exist to
  // create. The mark's slot is reserved on every row; equal child counts are
  // how that stays true.
  it('reserves the mark slot on every bar label so the figures stay in one column', () => {
    const { container } = renderStage(question({ reveal: reveal() }))

    const labels = [...container.querySelectorAll('p.tabular-nums')].filter((node) =>
      node.querySelector('bdi'),
    )
    expect(labels).toHaveLength(4)
    for (const label of labels) {
      expect(label.childElementCount).toBe(2) // reserved mark slot + the figure
    }
  })

  // The other half of derived req. 9, and the same guard question-stage.tsx
  // carries: emptySnapshot delivers `revealed` with no question at all.
  it('renders the waiting copy instead of crashing when currentQuestion is null', () => {
    expect(() => renderStage(null)).not.toThrow()
    expect(screen.getByText(strings.display.waiting)).toBeTruthy()
  })

  // Derived req. 11: killed from the PROP, not from a media query — a
  // CSS-only kill is invisible to jsdom and the Organizer's room-level toggle
  // is not a media query at all. The bars keep their widths: the
  // accessibility cues and the data are independent of the animation.
  it('drops stage-bar-grow under reduced motion while the bars keep their widths', () => {
    const widths = (root: HTMLElement) =>
      [...root.querySelectorAll<HTMLElement>('[style*="width"]')].map((n) => n.style.width)

    const reduced = renderStage(question({ reveal: reveal() }), true)
    expect(reduced.container.querySelectorAll('.stage-bar-grow')).toHaveLength(0)
    // Captured BEFORE cleanup: unmounting empties the container.
    const reducedWidths = widths(reduced.container)

    cleanup()
    const animated = renderStage(question({ reveal: reveal() }), false)
    expect(animated.container.querySelectorAll('.stage-bar-grow')).toHaveLength(4)

    expect(reducedWidths).toEqual(widths(animated.container))
    expect(reducedWidths).toEqual(['20%', '50%', '0%', '30%'])
  })

  // AC-3: no per-Participant private information on the shared screen.
  // Asserted against participants with distinctive names, so it can fail.
  it('renders no participant name or phone number anywhere', () => {
    const base = snap(question({ reveal: reveal() }))
    const withPeople: LobbySnapshot = {
      ...base,
      participantCount: 2,
      participants: [
        { id: 'p-1', displayName: 'ZZZ-PARTICIPANT-NAME' },
        { id: 'p-2', displayName: '+972-50-999-9999' },
      ],
      leaderboard: [
        { participantId: 'p-1', displayName: 'ZZZ-PARTICIPANT-NAME', score: 30, rank: 1 },
      ],
    }
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { container } = render(<RevealStage snapshot={withPeople} reducedMotion={false} />)
      expect(container.textContent).not.toContain('ZZZ-PARTICIPANT-NAME')
      expect(container.textContent).not.toContain('+972-50-999-9999')
    } finally {
      vi.useRealTimers()
    }
  })
})
