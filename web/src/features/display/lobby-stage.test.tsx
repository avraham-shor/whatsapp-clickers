import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render, screen } from '@testing-library/react'

import { strings } from '@/lib/strings.he'
import type { LobbySnapshot } from '@/lib/types'
import { LobbyStage } from './lobby-stage'

// RTL's automatic cleanup only self-registers when a global afterEach
// exists, and vitest `globals` is off here — without this line the second
// test renders into the DOM the first one left behind.
afterEach(cleanup)

// Expected copy is READ FROM strings.he.ts, never retyped. Two reasons, and
// both matter: a Hebrew literal here would put this file in the frontend
// copy-centralization grep alongside the two known pre-existing violations,
// and asserting the wiring (does the stage render the string the copy module
// holds?) is the useful assertion — re-typing the sentence would only assert
// that two files agree on a string a reviewer changed in one of them.
const lobbyCopy = strings.display.lobby

/** A lobby snapshot with only the count varying — the one field the guard
 *  under test cares about. Values mirror mockups/key-stage-lobby.html. */
function snap(participantCount: number): LobbySnapshot {
  return {
    gameId: 'game-1',
    state: 'lobby',
    joinCode: 'COHEN24',
    platformNumber: '+972 50-000-0000',
    participantCount,
    participants: [],
    questionCount: 3,
    currentQuestion: null,
    leaderboard: [],
    displaySettings: { reducedMotion: false },
  }
}

describe('LobbyStage', () => {
  // The StageProps wiring itself (deferred-work.md, 4.1 entry): until this
  // story every entry in stageByState was a placeholder that read neither
  // prop, so passing the wrong object was invisible to the type checker,
  // to the tests and to a running browser.
  it('renders the join code, the phone number and the count off the snapshot', () => {
    render(<LobbyStage snapshot={snap(57)} reducedMotion={false} />)

    // The instruction line carries JOIN + code as ONE bidi-isolated run;
    // the giant token below it carries the code alone. Both are expected —
    // the sentence says what to do, the token exists to be transcribed.
    expect(screen.getByText('JOIN COHEN24')).toBeTruthy()
    expect(screen.getByText('COHEN24')).toBeTruthy()
    // The phone number appears exactly twice, for the same reason.
    expect(screen.getAllByText('+972 50-000-0000')).toHaveLength(2)
    expect(screen.getByText('57')).toBeTruthy()
    expect(screen.getByText(lobbyCopy.joinedLabel)).toBeTruthy()
  })

  // The resolution of deferred-work.md's 2.4 entry on this surface. Two
  // concurrent joins each do INSERT -> buildSnapshot -> Broadcast, and the
  // hub stamps `seq` at Broadcast()-call time — so the older, LOWER count
  // can carry the higher seq, and use-game-socket's `seq < lastSeq` guard
  // accepts it as newer. Without the high-water hold the projector visibly
  // counts backwards in front of the whole room and sticks there until the
  // next join.
  it('never counts backwards when a stale lower count arrives', () => {
    const { rerender } = render(<LobbyStage snapshot={snap(2)} reducedMotion={false} />)
    expect(screen.getByText('2')).toBeTruthy()

    rerender(<LobbyStage snapshot={snap(1)} reducedMotion={false} />)
    expect(screen.getByText('2')).toBeTruthy()
    expect(screen.queryByText('1')).toBeNull()

    // A genuinely newer, higher count still moves it — the guard holds a
    // floor, it does not freeze the counter.
    rerender(<LobbyStage snapshot={snap(3)} reducedMotion={false} />)
    expect(screen.getByText('3')).toBeTruthy()
  })

  // EXPERIENCE.md's Accessibility Floor: counters announce politely and at
  // most every 5s. Three halves, all load-bearing, and each one is a bug the
  // pre-review implementation actually had:
  //   - the region must not be seeded at mount (assistive tech does not read
  //     a live region that already holds its text when it first appears, the
  //     bug 4.1's review found on the shell's announcer);
  //   - it must not announce the state it ARRIVED to, or every game opens by
  //     reading out countAnnouncement(0) — a non-event announced as an event;
  //   - the first real change must be read out AT ONCE, or a room that fills
  //     and starts inside 5s announces nothing at all before the state
  //     transition unmounts the stage.
  // "At most every 5s" is an upper bound on frequency, which a leading edge
  // respects.
  it('stays silent until the count actually changes, then announces at once', () => {
    vi.useFakeTimers()
    try {
      const { container, rerender } = render(
        <LobbyStage snapshot={snap(0)} reducedMotion={false} />,
      )
      const region = container.querySelector('[aria-live="polite"]')
      expect(region).toBeTruthy()
      expect(region?.textContent).toBe('')

      // Six times the throttle window with an empty room: still nothing.
      act(() => void vi.advanceTimersByTime(30000))
      expect(region?.textContent).toBe('')

      // The first join is read out immediately, not 5s later.
      rerender(<LobbyStage snapshot={snap(1)} reducedMotion={false} />)
      act(() => void vi.advanceTimersByTime(0))
      expect(region?.textContent).toBe(lobbyCopy.countAnnouncement(1))
    } finally {
      vi.useRealTimers()
    }
  })

  // The negative boundary is the whole test. Asserting only that the text
  // appears AFTER 5s would pass with announceIntervalMs set to 5000, 100 or
  // 1 — the 4998ms assertion is the one that pins the constant.
  it('throttles later announcements to one per 5s without throttling the pill', () => {
    vi.useFakeTimers()
    try {
      const { container, rerender } = render(
        <LobbyStage snapshot={snap(1)} reducedMotion={false} />,
      )
      const region = container.querySelector('[aria-live="polite"]')

      rerender(<LobbyStage snapshot={snap(2)} reducedMotion={false} />)
      act(() => void vi.advanceTimersByTime(0))
      expect(region?.textContent).toBe(lobbyCopy.countAnnouncement(2))

      // A join 1s into the window. The VISIBLE pill moves immediately; the
      // announcement must wait out the remaining 4s.
      act(() => void vi.advanceTimersByTime(1000))
      rerender(<LobbyStage snapshot={snap(3)} reducedMotion={false} />)
      expect(screen.getByText('3')).toBeTruthy()
      expect(region?.textContent).toBe(lobbyCopy.countAnnouncement(2))

      // 4998ms after the last announcement — still inside the window.
      act(() => void vi.advanceTimersByTime(3998))
      expect(region?.textContent).toBe(lobbyCopy.countAnnouncement(2))

      // 5000ms. It announces the count the room has reached BY NOW, not the
      // one that was pending when the window opened.
      rerender(<LobbyStage snapshot={snap(8)} reducedMotion={false} />)
      act(() => void vi.advanceTimersByTime(2))
      expect(region?.textContent).toBe(lobbyCopy.countAnnouncement(8))
    } finally {
      vi.useRealTimers()
    }
  })

  // Without this the effect's cleanup could be deleted and every other test
  // in this file would still pass. The display is designed to run for hours
  // across many state transitions, so a per-transition timer leak compounds.
  it('cancels a pending announcement when the stage unmounts', () => {
    vi.useFakeTimers()
    try {
      const { rerender, unmount } = render(<LobbyStage snapshot={snap(1)} reducedMotion={false} />)
      // Arriving at a count schedules nothing; a change schedules one timer.
      expect(vi.getTimerCount()).toBe(0)
      rerender(<LobbyStage snapshot={snap(2)} reducedMotion={false} />)
      expect(vi.getTimerCount()).toBe(1)

      unmount()
      expect(vi.getTimerCount()).toBe(0)
    } finally {
      vi.useRealTimers()
    }
  })
})
