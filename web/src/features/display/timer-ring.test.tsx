import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render, screen } from '@testing-library/react'

import { strings } from '@/lib/strings.he'
import { TimerRing } from './timer-ring'

// RTL's automatic cleanup only self-registers when a global afterEach
// exists, and vitest `globals` is off here — without this line the second
// test renders into the DOM the first one left behind.
afterEach(cleanup)

// Expected copy is READ FROM strings.he.ts, never retyped (4.2's rule): a
// Hebrew literal here would put this file in the frontend copy-centralization
// grep, and asserting the wiring is the useful assertion.
const questionCopy = strings.display.question

// A fixed wall clock so "the deadline is absolute" is testable at all. Every
// test sets the system time first and then renders — the component reads
// Date.now() during its first render.
const baseNow = Date.parse('2026-08-11T12:00:00Z')

/** An ISO deadline `seconds` from baseNow. Negative means already past. */
function deadlineIn(seconds: number): string {
  return new Date(baseNow + seconds * 1000).toISOString()
}

/** Every test needs fake timers (the tick) AND a fixed clock (the deadline
 *  arithmetic), so they are always set up together and always torn down. */
function withClock(body: () => void): void {
  vi.useFakeTimers()
  vi.setSystemTime(baseNow)
  try {
    body()
  } finally {
    vi.useRealTimers()
  }
}

/** Advance the fake clock one second at a time, each inside its own act() so
 *  React commits the state update and the effect schedules the NEXT timeout
 *  before the clock moves again.
 *
 *  A single advanceTimersByTime(5000) only ever fires the one timer that was
 *  already pending, because the tick is self-scheduling: the next setTimeout
 *  is not created until React commits. That is an artifact of fake timers
 *  meeting React's commit boundary, not a defect in the component — real
 *  wall-clock time does not jump five seconds between commits, and check 3
 *  of the browser pass covers the real thing. */
function tickSeconds(count: number): void {
  for (let i = 0; i < count; i += 1) {
    act(() => void vi.advanceTimersByTime(1000))
  }
}

/** Flush the zero-delay timeout that fills the ≤5s live region one commit
 *  after it exists. The region must be EMPTY on the commit that inserts it or
 *  assistive tech never speaks it, so the fill is deliberately not synchronous
 *  with the urgency flip (code review, 4.3). */
function flushAnnouncement(): void {
  act(() => void vi.advanceTimersByTime(0))
}

/** The one SVG circle. Queried by tag rather than by test id: this stage is
 *  output-only and carries no test-only attributes. */
function ring(container: HTMLElement): SVGCircleElement {
  const circle = container.querySelector('circle')
  expect(circle).toBeTruthy()
  return circle as SVGCircleElement
}

describe('TimerRing', () => {
  // AC-2: the countdown is computed client-side from the server's absolute
  // deadline. The server cutoff stays authoritative; this only renders the
  // room's view of it.
  it('derives remaining seconds from the absolute deadline and ticks down', () => {
    withClock(() => {
      render(
        <TimerRing
          deadlineIso={deadlineIn(20)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      expect(screen.getByText('20')).toBeTruthy()

      tickSeconds(1)
      expect(screen.getByText('19')).toBeTruthy()

      // Five more seconds of wall clock, five numerals down: no repeat, no
      // skip. A naive 1000ms interval drifts against the deadline and makes
      // the room see exactly that.
      tickSeconds(5)
      expect(screen.getByText('14')).toBeTruthy()
    })
  })

  // AC-2 forbids ever showing the total, in as many words. A projector laptop
  // whose clock runs BEHIND the server would compute a remaining time larger
  // than the configured limit, so the upper clamp is the load-bearing one.
  it('never shows a number above totalSeconds when the client clock is behind', () => {
    withClock(() => {
      // The server said 20s; this client's clock is 60s behind, so the naive
      // subtraction yields 80.
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(80)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      expect(screen.getByText('20')).toBeTruthy()
      expect(container.textContent).not.toContain('80')
    })
  })

  // The regression for the defect that clamp shipped with (code review, 4.3).
  // The clamped remainder is CONSTANT while the clamp is engaged, so an effect
  // keyed on it alone never re-ran and never scheduled the next tick: the
  // numeral froze on the total for the whole question with zero pending
  // timers, while the CSS sweep emptied the ring on schedule. Any skew >=1s
  // reproduces it; 2s is the mildest realistic one.
  it('keeps counting once the clamp releases when the client clock is behind', () => {
    withClock(() => {
      render(
        <TimerRing
          deadlineIso={deadlineIn(22)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      // Held at the limit while the clamp is engaged — never above it.
      expect(screen.getByText('20')).toBeTruthy()
      tickSeconds(1)
      expect(screen.getByText('20')).toBeTruthy()
      // A pending tick must exist at every step, or the countdown is dead.
      expect(vi.getTimerCount()).toBeGreaterThan(0)

      // Two seconds of skew, so the clamp releases on the third tick and the
      // numeral must move from there on.
      tickSeconds(2)
      expect(screen.getByText('19')).toBeTruthy()
      tickSeconds(5)
      expect(screen.getByText('14')).toBeTruthy()
    })
  })

  // The lower clamp, and the fact that the tick stops scheduling: the display
  // runs for hours, so a timer that keeps rescheduling past zero leaks once
  // per question.
  it('floors at 0 and stops ticking once the deadline has passed', () => {
    withClock(() => {
      render(
        <TimerRing
          deadlineIso={deadlineIn(-5)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      expect(screen.getByText('0')).toBeTruthy()
      // A past deadline mounts already urgent, so the one-shot that fills the
      // ≤5s region is pending. Flush it first: the subject here is the
      // COUNTDOWN tick, and what must not survive is a timer that reschedules
      // itself once per second forever.
      flushAnnouncement()
      expect(vi.getTimerCount()).toBe(0)
    })
  })

  // Derived req. 5: this stage owns question_closed too, where the numeral
  // reads 0 regardless of what the deadline says.
  it('pins the numeral to 0 when the question is closed', () => {
    withClock(() => {
      render(
        <TimerRing
          deadlineIso={deadlineIn(12)}
          totalSeconds={20}
          closed
          reducedMotion={false}
        />,
      )
      expect(screen.getByText('0')).toBeTruthy()
      expect(screen.queryByText('12')).toBeNull()
    })
  })

  // AC-3, and the assertion that must check BOTH channels. UX-DR5 forbids a
  // hue-only cue, so a test asserting only the colour would pass the exact
  // bug the requirement exists to prevent.
  it('turns the ring gold AND thickens it to 14 at the 5s threshold', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(6)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      // At 6s: white, 8px, and no urgent announcement yet.
      expect(screen.getByText('6')).toBeTruthy()
      expect(ring(container).getAttribute('stroke')).toBe('rgba(255,255,255,0.9)')
      expect(ring(container).getAttribute('stroke-width')).toBe('8')
      expect(container.textContent).not.toContain(questionCopy.urgentAnnouncement)

      tickSeconds(1)
      flushAnnouncement()

      // At 5s: gold, 14px, announced once.
      expect(screen.getByText('5')).toBeTruthy()
      expect(ring(container).getAttribute('stroke')).toBe('var(--color-gold)')
      expect(ring(container).getAttribute('stroke-width')).toBe('14')
      expect(container.textContent).toContain(questionCopy.urgentAnnouncement)
    })
  })

  // Derived req. 5: gold is rationed to two moments in the whole product
  // (UX-DR2), and question_closed is not one of them — its moment has passed.
  it('is not urgent at question_closed even though the numeral reads 0', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(-1)}
          totalSeconds={20}
          closed
          reducedMotion={false}
        />,
      )
      expect(screen.getByText('0')).toBeTruthy()
      expect(ring(container).getAttribute('stroke')).toBe('rgba(255,255,255,0.9)')
      expect(ring(container).getAttribute('stroke-width')).toBe('8')
      expect(container.textContent).not.toContain(questionCopy.urgentAnnouncement)
    })
  })

  // UX-DR14 and derived req. 9. This is the assertion that closes
  // deferred-work.md's 4.1 StageProps entry: silently dropping the
  // room-level half of the shell's reducedMotion OR now fails a test.
  it('renders a static ring under reduced motion while the numeral still counts', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(20)}
          totalSeconds={20}
          closed={false}
          reducedMotion
        />,
      )
      expect(ring(container).classList.contains('stage-timer-sweep')).toBe(false)

      // The numeral is independent of the sweep: reduced motion removes the
      // animation, not the countdown.
      tickSeconds(3)
      expect(screen.getByText('17')).toBeTruthy()
      expect(ring(container).classList.contains('stage-timer-sweep')).toBe(false)
    })
  })

  // The same render with motion allowed, so the assertion above is about
  // reducedMotion and not about the class never being applied at all.
  it('sweeps when motion is allowed and the question is open', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(20)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      expect(ring(container).classList.contains('stage-timer-sweep')).toBe(true)
    })
  })

  // A malformed deadline must render a number, not "NaN", in front of a room.
  it('renders 0 rather than NaN for an unparseable deadline', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing deadlineIso="not-a-date" totalSeconds={20} closed={false} reducedMotion={false} />,
      )
      expect(screen.getByText('0')).toBeTruthy()
      expect(container.textContent).not.toContain('NaN')
    })
  })

  // Derived req. 8: EXPERIENCE.md excludes the timer numeral from live
  // regions entirely — a per-second announcement would bury every other
  // announcement on the page. The only live region here is the ≤5s one.
  it('keeps the numeral out of every live region', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(20)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      const numeral = screen.getByText('20')
      expect(numeral.closest('[aria-live]')).toBeNull()

      const regions = container.querySelectorAll('[aria-live]')
      expect(regions).toHaveLength(1)
      expect(regions[0]?.getAttribute('aria-live')).toBe('polite')
    })
  })

  // 2 * PI * 106, the one circumference the dasharray, the keyframes and the
  // static offset all read. Negative offsets deplete clockwise; see the
  // @keyframes comment in index.css.
  const circumference = 2 * Math.PI * 106

  /** The static offset, or null while the CSS animation owns it. */
  function offset(container: HTMLElement): number | null {
    const raw = ring(container).getAttribute('stroke-dashoffset')
    return raw === null ? null : Number.parseFloat(raw)
  }

  // Removing the sweep class removes animation-fill-mode: forwards with it,
  // so without an explicit offset the ring reverted to the SVG default of 0 —
  // a FULL circle. The room watched the ring refill to full, in gold, at the
  // exact moment the countdown hit 0 (code review, 4.3).
  it('empties the ring rather than refilling it when the countdown reaches 0', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(1)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      // While the sweep runs the CSS owns the offset, so no attribute.
      expect(ring(container).classList.contains('stage-timer-sweep')).toBe(true)
      expect(offset(container)).toBeNull()

      tickSeconds(1)

      expect(screen.getByText('0')).toBeTruthy()
      expect(ring(container).classList.contains('stage-timer-sweep')).toBe(false)
      expect(offset(container)).toBeCloseTo(-circumference, 1)
    })
  })

  // Derived req. 5's other half: the numeral pins to 0 at question_closed, so
  // the arc must read empty too rather than snapping back to a full circle.
  it('leaves the ring empty at question_closed', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing deadlineIso={deadlineIn(12)} totalSeconds={20} closed reducedMotion={false} />,
      )
      expect(screen.getByText('0')).toBeTruthy()
      expect(offset(container)).toBeCloseTo(-circumference, 1)
    })
  })

  // AC-3 and Avraham's call at 4.3's code review: "static" must mean a ring
  // frozen at the true remaining fraction, not a ring that is always full and
  // therefore identical at 20s and at 1s.
  it('shows the true remaining fraction statically under reduced motion', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing deadlineIso={deadlineIn(10)} totalSeconds={20} closed={false} reducedMotion />,
      )
      // Half the question gone, so half the ring gone.
      expect(offset(container)).toBeCloseTo(-circumference / 2, 1)

      tickSeconds(5)
      expect(screen.getByText('5')).toBeTruthy()
      // And it keeps tracking as the numeral counts — a quarter left.
      expect(offset(container)).toBeCloseTo(-circumference * 0.75, 1)
    })
  })

  // Assistive tech does not announce content already present when a live
  // region first appears, so rendering the text straight from `urgent` lost
  // the announcement for every ring that MOUNTED urgent — reachable at the
  // DB's 5s minimum limit, on a refresh inside the last five seconds, and on
  // a second display opened late (code review, 4.3).
  it('announces the 5s threshold even when it mounts already urgent', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(5)}
          totalSeconds={5}
          closed={false}
          reducedMotion={false}
        />,
      )
      // The region exists and is EMPTY on the commit that inserted it.
      const region = container.querySelector('[aria-live]')
      expect(region).toBeTruthy()
      expect(region?.textContent).toBe('')

      flushAnnouncement()
      expect(container.textContent).toContain(questionCopy.urgentAnnouncement)
    })
  })

  // The negative animation-delay is what makes a display that connects (or
  // reconnects) mid-question resume at the right arc instead of restarting the
  // sweep. Nothing asserted it before: every fixture started at a full
  // question, where elapsed is 0 and a sign error still renders "-0s" (code
  // review, 4.3).
  it('captures a negative animation-delay matching the time already elapsed', () => {
    withClock(() => {
      const { container } = render(
        <TimerRing
          deadlineIso={deadlineIn(8)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )
      const style = ring(container).style
      expect(style.animationDuration).toBe('20s')
      // 12 of the 20 seconds are already gone, so the sweep starts 12s in.
      expect(style.animationDelay).toBe('-12s')
    })
  })

  // The sweep is captured once so an inbound answer frame cannot restart it —
  // but it must be RE-captured when the sweep resumes, or turning the room's
  // reduced-motion toggle off mid-question replays the delay it had at mount
  // and the ring finishes long after the numeral reaches 0 (code review, 4.3).
  it('recaptures the sweep when reduced motion is turned off mid-question', () => {
    withClock(() => {
      const { container, rerender } = render(
        <TimerRing deadlineIso={deadlineIn(20)} totalSeconds={20} closed={false} reducedMotion />,
      )
      expect(ring(container).classList.contains('stage-timer-sweep')).toBe(false)

      tickSeconds(5)
      rerender(
        <TimerRing
          deadlineIso={deadlineIn(20)}
          totalSeconds={20}
          closed={false}
          reducedMotion={false}
        />,
      )

      expect(ring(container).classList.contains('stage-timer-sweep')).toBe(true)
      // Five seconds passed while the ring was static, so the resumed sweep
      // starts five seconds in — not at the mount-time -0s.
      expect(ring(container).style.animationDelay).toBe('-5s')
    })
  })
})
