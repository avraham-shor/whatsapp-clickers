import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'

import { strings } from '@/lib/strings.he'
import type { CurrentQuestion, GameState, LobbySnapshot } from '@/lib/types'
import { QuestionStage } from './question-stage'

// RTL's automatic cleanup only self-registers when a global afterEach
// exists, and vitest `globals` is off here — without this line the second
// test renders into the DOM the first one left behind.
afterEach(cleanup)

// Expected copy is READ FROM strings.he.ts, never retyped (4.2's rule).
const questionCopy = strings.display.question
const letters = strings.questionEditor.optionLetters

// The stage renders a TimerRing, which reads Date.now() during render and
// schedules a tick. A fixed clock keeps that deterministic; these tests are
// about the stage's own logic, and timer-ring.test.tsx owns the countdown.
const baseNow = Date.parse('2026-08-11T12:00:00Z')
const cutoff = new Date(baseNow + 20000).toISOString()

// The question text and its options are Organizer-authored DATA, not copy —
// they arrive on the wire and this file never asserts their content, only
// that the stage renders what it was handed. Latin placeholders on purpose:
// a Hebrew fixture would put this file in the frontend copy-centralization
// grep alongside the two known pre-existing violations, for a string that
// carries no meaning to the assertion. (Every expected STRING below is still
// read from strings.he.ts.)
function question(overrides: Partial<CurrentQuestion> = {}): CurrentQuestion {
  return {
    id: 'q-1',
    position: 3,
    type: 'mcq',
    text: 'QUESTION-TEXT',
    options: ['OPTION-A', 'OPTION-B', 'OPTION-C', 'OPTION-D'],
    timeLimitSeconds: 20,
    answerCutoffAt: cutoff,
    answeredCount: 0,
    ...overrides,
  }
}

/** A question-stage snapshot with only the fields this stage reads varying. */
function snap(
  currentQuestion: CurrentQuestion | null,
  state: GameState = 'question_open',
): LobbySnapshot {
  return {
    gameId: 'game-1',
    state,
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

describe('QuestionStage', () => {
  // Derived req. 10. game/emptySnapshot sets CurrentQuestion: nil while
  // carrying the real State, so a post-commit buildSnapshot failure delivers
  // question_open with no question. There is no error boundary anywhere in
  // this app, so an unguarded deref puts React Router's English crash page on
  // the projector.
  it('renders the waiting copy instead of crashing when currentQuestion is null', () => {
    expect(() =>
      render(<QuestionStage snapshot={snap(null)} reducedMotion={false} />),
    ).not.toThrow()
    expect(screen.getByText(strings.display.waiting)).toBeTruthy()
  })

  // AC-1: the question text, the four lettered options, and the progress
  // line, all off the snapshot.
  it('renders the question, four lettered options and the progress line', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const current = question()
      render(<QuestionStage snapshot={snap(current)} reducedMotion={false} />)

      expect(screen.getByText(current.text)).toBeTruthy()
      for (const [index, option] of (current.options ?? []).entries()) {
        expect(screen.getByText(option)).toBeTruthy()
        // The letters come from questionEditor.optionLetters — one alphabet,
        // one source. Only the trailing period lives in display.question.
        expect(screen.getByText(questionCopy.optionLetter(letters[index]))).toBeTruthy()
      }
      // Reused from `live` on purpose: EXPERIENCE.md gives one sentence and
      // both surfaces show it.
      expect(screen.getByText(strings.live.questionProgress(3, 10))).toBeTruthy()
    } finally {
      vi.useRealTimers()
    }
  })

  // AC-1 / A15: the hint replaces the option rows, it does not join them.
  it('renders the free-text hint and no option rows for a free-text question', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const current = question({ type: 'free_text', options: undefined })
      render(<QuestionStage snapshot={snap(current)} reducedMotion={false} />)

      expect(screen.getByText(questionCopy.freeTextHint)).toBeTruthy()
      // Not one letter row survives.
      for (const letter of letters) {
        expect(screen.queryByText(questionCopy.optionLetter(letter))).toBeNull()
      }
    } finally {
      vi.useRealTimers()
    }
  })

  // Branching on the TYPE, not on `options` being present: a malformed MCQ
  // that arrives with no options must render the hint rather than an empty
  // column in front of a room.
  it('renders the hint rather than an empty column for an mcq with no options', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const current = question({ options: undefined })
      render(<QuestionStage snapshot={snap(current)} reducedMotion={false} />)
      expect(screen.getByText(questionCopy.freeTextHint)).toBeTruthy()
    } finally {
      vi.useRealTimers()
    }
  })

  // Derived req. 7. deferred-work.md's 2.4 entry: `seq` is stamped at
  // Broadcast()-call time, so concurrent writers can deliver a LOWER count
  // under a HIGHER seq and use-game-socket's `seq < lastSeq` guard accepts
  // it. Answers arrive in exactly that burst shape. 4.2 measured backwards
  // counts in 3 of 4 E2E runs on the lobby counter.
  it('never counts the answered total backwards', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { rerender } = render(
        <QuestionStage snapshot={snap(question({ answeredCount: 7 }))} reducedMotion={false} />,
      )
      expect(screen.getByText(strings.live.answeredStat(7))).toBeTruthy()

      rerender(
        <QuestionStage snapshot={snap(question({ answeredCount: 4 }))} reducedMotion={false} />,
      )
      expect(screen.getByText(strings.live.answeredStat(7))).toBeTruthy()
      expect(screen.queryByText(strings.live.answeredStat(4))).toBeNull()

      // A genuinely newer, higher count still moves it — the guard holds a
      // floor, it does not freeze the counter.
      rerender(
        <QuestionStage snapshot={snap(question({ answeredCount: 9 }))} reducedMotion={false} />,
      )
      expect(screen.getByText(strings.live.answeredStat(9))).toBeTruthy()
    } finally {
      vi.useRealTimers()
    }
  })

  // The other half of derived req. 7, and the one the shell's key does NOT
  // guarantee: question 2 inheriting question 1's count would be a silent lie
  // in front of a room. Keyed on questionId rather than trusting a remount,
  // because "two question_open stages are never adjacent" is the SHELL's
  // invariant, not this stage's.
  it('resets the answered count when the question id changes', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { rerender } = render(
        <QuestionStage snapshot={snap(question({ answeredCount: 30 }))} reducedMotion={false} />,
      )
      expect(screen.getByText(strings.live.answeredStat(30))).toBeTruthy()

      rerender(
        <QuestionStage
          snapshot={snap(question({ id: 'q-2', position: 4, answeredCount: 2 }))}
          reducedMotion={false}
        />,
      )
      expect(screen.getByText(strings.live.answeredStat(2))).toBeTruthy()
      expect(screen.queryByText(strings.live.answeredStat(30))).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  // Derived req. 9, from the stage's side: the prop must actually reach the
  // ring through StageProps. Together with timer-ring.test.tsx's own
  // assertion this is what closes deferred-work.md's 4.1 entry — silently
  // dropping the room-level half of the shell's OR now fails a test.
  it('passes reducedMotion through to the ring', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      // Two independent mounts rather than a rerender: the sweep is captured
      // once per mount by design (a new question remounts via the ring's
      // key), so flipping the prop on a live instance would not be the
      // scenario this asserts.
      const { container: moving } = render(
        <QuestionStage snapshot={snap(question())} reducedMotion={false} />,
      )
      expect(moving.querySelector('circle')?.classList.contains('stage-timer-sweep')).toBe(true)

      const { container: still } = render(
        <QuestionStage snapshot={snap(question())} reducedMotion />,
      )
      expect(still.querySelector('circle')?.classList.contains('stage-timer-sweep')).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })

  // Derived req. 5: this stage owns question_closed too. The numeral reads 0
  // while the question, the options and the count all hold unchanged.
  it('holds the question and options at question_closed with the numeral at 0', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const current = question({ answeredCount: 15 })
      render(
        <QuestionStage snapshot={snap(current, 'question_closed')} reducedMotion={false} />,
      )
      expect(screen.getByText(current.text)).toBeTruthy()
      expect(screen.getByText(strings.live.answeredStat(15))).toBeTruthy()
      // The deadline is still 20s away; `closed` pins the numeral anyway.
      expect(screen.getByText('0')).toBeTruthy()
    } finally {
      vi.useRealTimers()
    }
  })

  // Derived req. 8: the count's region is polite and starts EMPTY (assistive
  // tech does not announce content already present when a live region first
  // appears — the bug 4.1's review found on the shell's announcer).
  it('mounts the count announcement region empty and polite', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { container } = render(
        <QuestionStage snapshot={snap(question({ answeredCount: 5 }))} reducedMotion={false} />,
      )
      const regions = [...container.querySelectorAll('[aria-live]')]
      // Two: the ring's ≤5s region and the count's. Both polite — the shell
      // owns the one assertive region (UX-DR14) and two would fight.
      expect(regions).toHaveLength(2)
      for (const region of regions) {
        expect(region.getAttribute('aria-live')).toBe('polite')
        expect(region.textContent).toBe('')
      }
    } finally {
      vi.useRealTimers()
    }
  })
})
