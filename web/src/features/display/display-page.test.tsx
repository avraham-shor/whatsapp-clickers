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
})
