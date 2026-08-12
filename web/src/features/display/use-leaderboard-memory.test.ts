import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, renderHook } from '@testing-library/react'

import type { CurrentQuestion, LeaderboardEntry, LobbySnapshot } from '@/lib/types'
import { standingsOf, useLeaderboardMemory } from './use-leaderboard-memory'

// RTL's automatic cleanup only self-registers when a global afterEach
// exists, and vitest `globals` is off here.
afterEach(cleanup)

// Participant display names are DATA, not copy — Latin placeholders keep
// this file out of the frontend Hebrew grep (4.3's finding).
function entry(id: string, rank: number, score: number): LeaderboardEntry {
  return { participantId: id, displayName: `PLAYER-${id}`, score, rank }
}

function question(id: string): CurrentQuestion {
  return {
    id,
    position: 1,
    type: 'mcq',
    text: 'QUESTION-TEXT',
    options: ['OPTION-A', 'OPTION-B'],
    timeLimitSeconds: 20,
    answerCutoffAt: '2026-08-12T12:00:00Z',
    answeredCount: 4,
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
    currentQuestion: question('q-1'),
    leaderboard: [entry('a', 1, 300), entry('b', 2, 200), entry('c', 3, 100)],
    displaySettings: { reducedMotion: false },
    ...overrides,
  }
}

describe('standingsOf', () => {
  it('records rank AND slot for every entry, because shared ranks make them different numbers', () => {
    // 1,1,3 sits in slots 0,1,2. The ▲ count is a rank delta; the reshuffle
    // distance is a slot delta. Collapsing them is the bug this covers.
    const standings = standingsOf([entry('a', 1, 300), entry('b', 1, 300), entry('c', 3, 100)])

    expect(standings).toEqual({
      a: { rank: 1, index: 0 },
      b: { rank: 1, index: 1 },
      c: { rank: 3, index: 2 },
    })
  })

  it('covers the WHOLE board, not just the visible top ten', () => {
    // A climb from 14th to 8th only computes if 14th was remembered.
    const entries = Array.from({ length: 14 }, (_, i) => entry(`p${i}`, i + 1, 100 - i))

    const standings = standingsOf(entries)

    expect(Object.keys(standings)).toHaveLength(14)
    expect(standings['p13']).toEqual({ rank: 14, index: 13 })
  })
})

describe('useLeaderboardMemory', () => {
  it('yields undefined on the first showing — there is nothing to have moved from', () => {
    const { result } = renderHook(() => useLeaderboardMemory('game-1', snap()))

    expect(result.current).toBeUndefined()
  })

  it('yields the first showing as the second showing baseline', () => {
    const first = snap({
      leaderboard: [entry('a', 1, 300), entry('b', 2, 200), entry('c', 3, 100)],
    })
    const { result, rerender } = renderHook(
      ({ snapshot }: { snapshot: LobbySnapshot }) => useLeaderboardMemory('game-1', snapshot),
      { initialProps: { snapshot: first } },
    )
    expect(result.current).toBeUndefined()

    // A second question ran; c overtook b. The showing key is the question
    // id, so this is a genuinely new showing.
    rerender({
      snapshot: snap({
        currentQuestion: question('q-2'),
        leaderboard: [entry('a', 1, 500), entry('c', 2, 400), entry('b', 3, 200)],
      }),
    })

    expect(result.current).toEqual({
      a: { rank: 1, index: 0 },
      b: { rank: 2, index: 1 },
      c: { rank: 3, index: 2 },
    })
  })

  it('does NOT re-capture within one showing, so a repeated frame cannot erase the arrows', () => {
    const { result, rerender } = renderHook(
      ({ snapshot }: { snapshot: LobbySnapshot }) => useLeaderboardMemory('game-1', snapshot),
      { initialProps: { snapshot: snap({ currentQuestion: question('q-1') }) } },
    )
    rerender({
      snapshot: snap({
        currentQuestion: question('q-2'),
        leaderboard: [entry('c', 1, 400), entry('a', 2, 300), entry('b', 3, 200)],
      }),
    })
    const afterSecondShowing = result.current

    // The reachable repeat: the Organizer toggles the reduce-animations
    // setting mid-Leaderboard (strings.live is the copy; naming it in Hebrew
    // here put a literal in a .ts file, which the story's own *.tsx-scoped
    // scan cannot see — code review, 2026-08-12). Same question, same board,
    // a brand-new snapshot object.
    rerender({
      snapshot: snap({
        currentQuestion: question('q-2'),
        leaderboard: [entry('c', 1, 400), entry('a', 2, 300), entry('b', 3, 200)],
        displaySettings: { reducedMotion: true },
      }),
    })

    expect(result.current).toEqual(afterSecondShowing)
    expect(result.current).toEqual({
      a: { rank: 1, index: 0 },
      b: { rank: 2, index: 1 },
      c: { rank: 3, index: 2 },
    })
  })

  it('does not capture a baseline from a degraded frame with no current question', () => {
    const { result, rerender } = renderHook(
      ({ snapshot }: { snapshot: LobbySnapshot }) => useLeaderboardMemory('game-1', snapshot),
      { initialProps: { snapshot: snap({ currentQuestion: question('q-1') }) } },
    )

    // emptySnapshot: the real state, a nil currentQuestion, an empty board.
    // Recording that as "what the room saw" would poison the next showing.
    rerender({
      snapshot: snap({ currentQuestion: null, questionCount: 0, leaderboard: [] }),
    })
    expect(result.current).toBeUndefined()

    // The next real showing still sees the FIRST showing as its baseline.
    rerender({
      snapshot: snap({
        currentQuestion: question('q-2'),
        leaderboard: [entry('c', 1, 400), entry('a', 2, 300), entry('b', 3, 200)],
      }),
    })
    expect(result.current).toEqual({
      a: { rank: 1, index: 0 },
      b: { rank: 2, index: 1 },
      c: { rank: 3, index: 2 },
    })
  })

  it('does not capture a baseline from a degraded frame with a zero question count', () => {
    // The half of the correlation this hook did not check at all before the
    // code review of 2026-08-12: it tested only `currentQuestion !== null`,
    // while the stage refused to RENDER on either half. A frame the room was
    // never shown could therefore still be recorded as "what the room saw".
    // Both now go through isDegradedFrame, and this pins the disjunct the
    // hook used to miss.
    const { result, rerender } = renderHook(
      ({ snapshot }: { snapshot: LobbySnapshot }) => useLeaderboardMemory('game-1', snapshot),
      { initialProps: { snapshot: snap({ currentQuestion: question('q-1') }) } },
    )

    rerender({ snapshot: snap({ currentQuestion: question('q-2'), questionCount: 0, leaderboard: [] }) })
    expect(result.current).toBeUndefined()

    // The next healthy showing still baselines off the FIRST showing.
    rerender({
      snapshot: snap({
        currentQuestion: question('q-3'),
        leaderboard: [entry('c', 1, 400), entry('a', 2, 300), entry('b', 3, 200)],
      }),
    })
    expect(result.current).toEqual({
      a: { rank: 1, index: 0 },
      b: { rank: 2, index: 1 },
      c: { rank: 3, index: 2 },
    })
  })

  it('yields undefined at every state other than leaderboard', () => {
    for (const state of ['lobby', 'question_open', 'question_closed', 'revealed', 'finished'] as const) {
      const { result } = renderHook(() => useLeaderboardMemory('game-1', snap({ state })))
      expect(result.current).toBeUndefined()
    }
  })

  it('yields undefined for a different gameId — the shell does not remount on /display/A -> /display/B', () => {
    const { result, rerender } = renderHook(
      ({ gameId, snapshot }: { gameId: string; snapshot: LobbySnapshot }) =>
        useLeaderboardMemory(gameId, snapshot),
      { initialProps: { gameId: 'game-1', snapshot: snap({ currentQuestion: question('q-1') }) } },
    )
    rerender({
      gameId: 'game-1',
      snapshot: snap({ currentQuestion: question('q-2') }),
    })
    expect(result.current).toBeDefined()

    // Game B's leaderboard has nothing to do with game A's ranking.
    rerender({
      gameId: 'game-2',
      snapshot: snap({ gameId: 'game-2', currentQuestion: question('q-9') }),
    })
    expect(result.current).toBeUndefined()
  })

  it('yields undefined for a null snapshot', () => {
    const { result } = renderHook(() => useLeaderboardMemory('game-1', null))

    expect(result.current).toBeUndefined()
  })
})
