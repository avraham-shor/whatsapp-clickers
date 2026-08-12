import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'

import { strings } from '@/lib/strings.he'
import type { CurrentQuestion, GameState, LobbySnapshot } from '@/lib/types'

// Hoisted so the module factory below can close over it: vi.mock is lifted
// above the imports, and a plain const would be in its TDZ when it runs.
const mocks = vi.hoisted(() => ({ useGameSocket: vi.fn() }))
vi.mock('@/lib/use-game-socket', () => ({ useGameSocket: mocks.useGameSocket }))

const { DisplayPage } = await import('./display-page')

afterEach(cleanup)

// jsdom implements no matchMedia, and the shell reads it for the viewer's OS
// reduced-motion setting on its first render.
beforeEach(() => {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
})

const baseNow = Date.parse('2026-08-11T12:00:00Z')

function question(overrides: Partial<CurrentQuestion> = {}): CurrentQuestion {
  return {
    id: 'q-1',
    position: 3,
    type: 'mcq',
    text: 'QUESTION-TEXT',
    options: ['OPTION-A', 'OPTION-B', 'OPTION-C', 'OPTION-D'],
    timeLimitSeconds: 20,
    answerCutoffAt: new Date(baseNow + 20000).toISOString(),
    answeredCount: 0,
    ...overrides,
  }
}

function snap(currentQuestion: CurrentQuestion, state: GameState): LobbySnapshot {
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

function renderWith(snapshot: LobbySnapshot) {
  mocks.useGameSocket.mockReturnValue({ snapshot, notFound: false })
  return render(
    <MemoryRouter initialEntries={['/display/game-1']}>
      <DisplayPage />
    </MemoryRouter>,
  )
}

describe('DisplayPage', () => {
  // The defect this floor exists for (code review, 4.3, measured 8 -> 7).
  // The stage's own high-water cannot cover it: the shell's
  // `${gameId}:${state}` key REMOUNTS the stage on the transition, so a hold
  // kept inside the stage is discarded at exactly the moment the closing
  // snapshot can carry a count lower than one already delivered.
  // deferred-work.md 2.4: `seq` is stamped at Broadcast()-call time, so
  // concurrent writers can deliver a lower count under a higher seq and
  // use-game-socket's `seq < lastSeq` guard accepts it.
  it('holds the answered count across the question_open -> question_closed remount', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { rerender } = renderWith(snap(question({ answeredCount: 8 }), 'question_open'))
      expect(screen.getByText(strings.live.answeredStat(8))).toBeTruthy()

      mocks.useGameSocket.mockReturnValue({
        snapshot: snap(question({ answeredCount: 7 }), 'question_closed'),
        notFound: false,
      })
      rerender(
        <MemoryRouter initialEntries={['/display/game-1']}>
          <DisplayPage />
        </MemoryRouter>,
      )

      expect(screen.getByText(strings.live.answeredStat(8))).toBeTruthy()
      expect(screen.queryByText(strings.live.answeredStat(7))).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  // The floor is a floor, not a freeze — a genuinely newer, higher count still
  // moves the number.
  it('still lets the answered count rise', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { rerender } = renderWith(snap(question({ answeredCount: 8 }), 'question_open'))

      mocks.useGameSocket.mockReturnValue({
        snapshot: snap(question({ answeredCount: 11 }), 'question_open'),
        notFound: false,
      })
      rerender(
        <MemoryRouter initialEntries={['/display/game-1']}>
          <DisplayPage />
        </MemoryRouter>,
      )

      expect(screen.getByText(strings.live.answeredStat(11))).toBeTruthy()
    } finally {
      vi.useRealTimers()
    }
  })

  // Keyed on the question id so question 2 never inherits question 1's floor —
  // that would be a silent lie in front of a room.
  it('resets the floor when the question changes', () => {
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { rerender } = renderWith(snap(question({ answeredCount: 30 }), 'question_open'))
      expect(screen.getByText(strings.live.answeredStat(30))).toBeTruthy()

      mocks.useGameSocket.mockReturnValue({
        snapshot: snap(question({ id: 'q-2', position: 4, answeredCount: 2 }), 'question_open'),
        notFound: false,
      })
      rerender(
        <MemoryRouter initialEntries={['/display/game-1']}>
          <DisplayPage />
        </MemoryRouter>,
      )

      expect(screen.getByText(strings.live.answeredStat(2))).toBeTruthy()
      expect(screen.queryByText(strings.live.answeredStat(30))).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  // --- The Leaderboard stage and its cross-stage memory (story 4.5) ---

  /** Re-renders the shell with a new snapshot, the way a WS frame arrives. */
  function deliver(rerender: (ui: React.ReactElement) => void, snapshot: LobbySnapshot) {
    mocks.useGameSocket.mockReturnValue({ snapshot, notFound: false })
    rerender(
      <MemoryRouter initialEntries={['/display/game-1']}>
        <DisplayPage />
      </MemoryRouter>,
    )
  }

  function board(ids: string[]): LobbySnapshot['leaderboard'] {
    return ids.map((id, index) => ({
      participantId: id,
      displayName: `PLAYER-${id}`,
      score: 500 - index * 100,
      rank: index + 1,
    }))
  }

  function leaderboardSnap(questionId: string, ids: string[]): LobbySnapshot {
    return {
      ...snap(question({ id: questionId }), 'leaderboard'),
      leaderboard: board(ids),
    }
  }

  it('mounts the leaderboard stage at `leaderboard` and at no other state', () => {
    // AC-2's "renders only when entered", from the other direction: the map
    // entry is what makes the stage reachable at all.
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      // Asserted on the <ol>, which only LeaderboardStage renders — an `li`
      // count is really an assertion about whatever ELSE happens to be
      // mounted, and would move the day another stage renders a list for an
      // unrelated reason. And every other state is delivered, not three of
      // them, because the title says "at no other state". (Code review,
      // 2026-08-12.)
      const { container, rerender } = renderWith(leaderboardSnap('q-1', ['a', 'b']))
      expect(container.querySelectorAll('ol')).toHaveLength(1)
      expect(container.querySelectorAll('li')).toHaveLength(2)

      for (const state of [
        'draft',
        'lobby',
        'question_open',
        'question_closed',
        'revealed',
        'finished',
      ] as const) {
        deliver(rerender, { ...snap(question(), state), leaderboard: board(['a', 'b']) })
        expect(container.querySelectorAll('ol')).toHaveLength(0)
      }
    } finally {
      vi.useRealTimers()
    }
  })

  it('never mounts the leaderboard stage on the UJ-4 skip (AC-2)', () => {
    // revealed -> question_open directly. The board data is on every frame
    // (buildSnapshot always reads it), so "no leaderboard frame arrived" is
    // the only thing that keeps the stage off the screen.
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { container, rerender } = renderWith({
        ...snap(question(), 'revealed'),
        leaderboard: board(['a', 'b']),
      })
      expect(container.querySelectorAll('ol')).toHaveLength(0)

      deliver(rerender, {
        ...snap(question({ id: 'q-2', position: 4 }), 'question_open'),
        leaderboard: board(['a', 'b']),
      })
      expect(container.querySelectorAll('ol')).toHaveLength(0)
    } finally {
      vi.useRealTimers()
    }
  })

  it('delivers the first showing standings as the second showing baseline', () => {
    // The whole point of the shell owning this: the stage is remounted on
    // the `${gameId}:${state}` key at every transition, so nothing it
    // remembered would survive to the next Leaderboard.
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { container, rerender } = renderWith(leaderboardSnap('q-1', ['a', 'b', 'c']))
      // First showing: no baseline, so no indicator anywhere. Scoped to
      // `li .sr-only` — the shell's own assertive announcer is sr-only too.
      expect(container.querySelector('li .sr-only')).toBeNull()

      // A question runs, then the second Leaderboard — c climbed 3 -> 1.
      deliver(rerender, { ...snap(question({ id: 'q-2', position: 4 }), 'revealed') })
      deliver(rerender, leaderboardSnap('q-2', ['c', 'a', 'b']))

      const climber = container.querySelectorAll('li')[0]!
      expect(climber.querySelector('.sr-only')?.textContent).toBe(
        strings.display.leaderboard.climbedLabel(2),
      )
      expect(climber.style.getPropertyValue('--stage-row-shift')).toBe('2')
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not re-capture the baseline when a frame repeats within one showing', () => {
    // The reachable repeat is the Organizer's display-settings toggle.
    // Re-capturing would overwrite the baseline with the CURRENT standings
    // and silently erase every arrow one frame after they appeared.
    vi.useFakeTimers()
    vi.setSystemTime(baseNow)
    try {
      const { container, rerender } = renderWith(leaderboardSnap('q-1', ['a', 'b', 'c']))
      deliver(rerender, { ...snap(question({ id: 'q-2', position: 4 }), 'revealed') })
      deliver(rerender, leaderboardSnap('q-2', ['c', 'a', 'b']))

      const before = container.querySelector('li .sr-only')?.textContent
      expect(before).toBe(strings.display.leaderboard.climbedLabel(2))

      // Same question, same board, a brand-new frame.
      const repeat = leaderboardSnap('q-2', ['c', 'a', 'b'])
      deliver(rerender, { ...repeat, displaySettings: { reducedMotion: false } })

      expect(container.querySelector('li .sr-only')?.textContent).toBe(before)
    } finally {
      vi.useRealTimers()
    }
  })
})
