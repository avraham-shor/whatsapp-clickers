import { useEffect, useState, type CSSProperties } from 'react'

import { strings } from '@/lib/strings.he'

// viewBox 226 with r=106: the outer edge is 106+8/2 = 110 at rest (a 220px
// diameter, DESIGN.md components.timer-hero's figure exactly) and 106+14/2 =
// 113 when urgent - precisely the viewBox edge, so the thickening never
// clips. r is CONSTANT across the two widths on purpose: changing it would
// change the circumference and make the sweep jump at the 5s mark.
const ringRadius = 106
const ringCircumference = 2 * Math.PI * ringRadius

/** The inline half of the depletion animation: how long it runs, and how far
 *  into it to start. The NEGATIVE delay is what makes a display that connects
 *  (or reconnects) mid-question resume at the right point instead of starting
 *  the sweep over. Read once per sweep START, never per render — see `sweep`. */
function sweepStyle(totalSeconds: number, remainingMs: number): CSSProperties {
  const elapsed = Math.max(0, totalSeconds - remainingMs / 1000)
  return {
    animationDuration: `${totalSeconds}s`,
    animationDelay: `${-elapsed}s`,
    // consumed by @keyframes stage-timer-deplete, so the dasharray and
    // the final offset can never disagree
    '--stage-timer-circumference': String(ringCircumference),
  } as CSSProperties
}

interface TimerRingProps {
  /** RFC 3339 UTC, from the snapshot's currentQuestion.answerCutoffAt. */
  deadlineIso: string
  /** The question's configured limit — the upper clamp, never displayed. */
  totalSeconds: number
  /** True once the Organizer has closed the question: the numeral pins to
   *  0, the sweep does not run, and urgency is over. */
  closed: boolean
  /** Already the OR of the viewer's OS setting and the room-level toggle. */
  reducedMotion: boolean
}

/**
 * The authoritative countdown (FR-7, FR-9): a depleting ring with the
 * remaining seconds at its centre.
 *
 * Deliberately snapshot-free — it knows nothing about the game, the band or
 * the question, which is what makes its arithmetic unit-testable without
 * building a whole snapshot.
 *
 * The caller must mount it with key={`${question.id}:${question.answerCutoffAt}`}
 * so a new question gets a fresh sweep capture; see the comment on `sweep`.
 */
export function TimerRing({ deadlineIso, totalSeconds, closed, reducedMotion }: TimerRingProps) {
  const [now, setNow] = useState(() => Date.now())

  // Absolute deadline in, remaining seconds out. The server cutoff stays
  // authoritative (FR-7) - this only renders the room's view of it.
  //
  // Clamped BOTH ways against a projector laptop's untrusted clock. The
  // upper clamp is the load-bearing one: a client clock running behind the
  // server would otherwise display a number LARGER than the configured
  // limit, and AC-2 forbids ever showing the total. Math.ceil so the last
  // whole second reads "1" and 0 appears exactly at the deadline.
  //
  // What the clamps do NOT do is correct the skew - they mask the number
  // while the offset stays wrong, so a clock running AHEAD still floors at 0
  // for a whole live question. That half is a model gap rather than a coding
  // one and is recorded in deferred-work.md (code review, 4.3); the fix is a
  // server-stamped snapshot time, which this story's scope forbids.
  const deadlineMs = Date.parse(deadlineIso)
  const rawMs = Number.isFinite(deadlineMs) ? deadlineMs - now : 0
  const remainingMs = closed ? 0 : Math.min(Math.max(rawMs, 0), totalSeconds * 1000)
  const remainingSeconds = Math.ceil(remainingMs / 1000)

  // Scheduled to the NEXT integer-second boundary rather than a fixed
  // 1000ms interval: a fixed interval drifts against the deadline and makes
  // the room see a number repeat or skip. setState inside a timeout
  // callback is async, so it does not trip react-hooks/set-state-in-effect.
  //
  // `now` is in the dependency list and that is the load-bearing part: the
  // effect originally depended on `remainingMs` alone, which STOPS CHANGING
  // whenever the upper clamp is engaged (a projector clock >=1s behind the
  // server pins it at exactly totalSeconds*1000). React then saw no dep
  // change, never re-ran the effect, and never scheduled a successor
  // timeout - so the countdown froze on the total for the entire question
  // while the CSS sweep emptied the ring on schedule. Measured, not
  // theorised: numeral stuck at 20 for 12s with zero pending timers (code
  // review, 4.3). The delay is taken from the UNCLAMPED remainder so the
  // ticks stay aligned to the real deadline and land correctly the moment
  // the clamp releases.
  useEffect(() => {
    if (remainingMs <= 0) return
    const id = setTimeout(() => setNow(Date.now()), Math.max(rawMs, 0) % 1000 || 1000)
    return () => clearTimeout(id)
  }, [now, rawMs, remainingMs])

  // AC-3. Not `remainingSeconds <= 5` alone: at question_closed the numeral
  // is 0 and gold's moment has passed (UX-DR2 rations gold to two moments in
  // the whole product, and this is the first). Independent of reducedMotion
  // on purpose — this is the accessibility cue, not the animation.
  const urgent = !closed && remainingSeconds <= 5

  const sweeping = !reducedMotion && !closed && remainingMs > 0
  // Captured when the sweep STARTS, not on every render: animationDelay and
  // -Duration recomputed on a later render would restart the sweep from full,
  // and this component re-renders on every inbound answer frame, so a
  // per-render style object would reset the ring several times a second in a
  // busy room.
  //
  // Recaptured on a false->true transition, which is the case the original
  // capture-once-per-mount shape got wrong: an Organizer toggling the room's
  // reduced-motion setting off mid-question brought the class back with the
  // delay it had at MOUNT, so the ring needed another full totalSeconds to
  // deplete and finished long after the numeral reached 0 (code review, 4.3).
  // State adjusted during render, the pattern display-page.tsx and
  // question-stage.tsx both use: this project's lint rules reject writing a
  // ref during render.
  const [sweep, setSweep] = useState(() => ({
    running: sweeping,
    style: sweepStyle(totalSeconds, remainingMs),
  }))
  if (sweep.running !== sweeping) {
    setSweep({
      running: sweeping,
      style: sweeping ? sweepStyle(totalSeconds, remainingMs) : sweep.style,
    })
  }

  // How much of the ring is already consumed, 0..1. This drives
  // stroke-dashoffset whenever the CSS sweep is NOT running — which the
  // original left to the SVG default of 0, i.e. a FULL ring:
  //   - at the instant the countdown reached 0 with the question still open,
  //     removing the class removed animation-fill-mode: forwards with it, so
  //     the ring snapped from empty back to a full GOLD circle reading "0";
  //   - it stayed full through question_closed;
  //   - and under reduced motion it was full for the whole question, looking
  //     identical at 20s and at 1s, which is a static ring that misinforms
  //     rather than a static equivalent (Avraham's call at code review, 4.3:
  //     show the true remaining fraction statically — AC-3's "the ring is
  //     static while the numeral still counts" is satisfied either way).
  const totalMs = totalSeconds * 1000
  const depleted = totalMs > 0 ? Math.min(Math.max(1 - remainingMs / totalMs, 0), 1) : 1

  // The ≤5s region must be EMPTY on the commit that INSERTS it: assistive
  // tech does not announce content already present when a live region first
  // appears. That is the bug 4.1's review found on the shell announcer and
  // the reason use-throttled-announcement.ts starts at null — and rendering
  // this one straight from `urgent` reintroduced it for every ring that
  // MOUNTS urgent: a question whose limit is the DB minimum of 5s, a display
  // refreshed inside the last five seconds, or a second display opened late.
  // setState in a timeout callback is async, so it does not trip
  // react-hooks/set-state-in-effect. (Code review, 4.3.)
  const [urgentAnnounced, setUrgentAnnounced] = useState(false)
  useEffect(() => {
    if (!urgent) return
    const id = setTimeout(() => setUrgentAnnounced(true), 0)
    return () => clearTimeout(id)
  }, [urgent])

  return (
    <div className="relative grid size-[var(--stage-timer-ring)] place-items-center">
      <svg viewBox="0 0 226 226" aria-hidden className="absolute inset-0 block h-full w-full">
        <circle
          cx="113"
          cy="113"
          r={ringRadius}
          fill="none"
          // rotate so depletion starts at 12 o'clock; SVG's 0° is 3 o'clock.
          // Clockwise, the universal countdown convention - RTL mirrors text
          // and reading order, not a clock-derived graphic.
          transform="rotate(-90 113 113)"
          // rgba(...) is DESIGN.md timer-hero.border verbatim, not
          // ink-on-dark at full opacity — and never a green stroke ("white
          // ring on dark band — never green-on-green", Do's and Don'ts).
          // var(--color-gold) rather than a stroke-gold utility: Tailwind's
          // stroke-* on an SVG presentation attribute is a class this project
          // has never used, and the token is right there. Gold on green-800 is
          // 4.3:1 — DESIGN.md's contrast table clears it for NON-TEXT only,
          // which a ring is.
          stroke={urgent ? 'var(--color-gold)' : 'rgba(255,255,255,0.9)'}
          strokeWidth={urgent ? 14 : 8}
          strokeDasharray={ringCircumference}
          // Left to the CSS animation while it runs; set explicitly the rest
          // of the time so the arc always tells the truth. See `depleted`.
          // NEGATIVE, matching @keyframes stage-timer-deplete: the sign is
          // what makes the ring empty clockwise rather than counterclockwise
          // (measured in Edge, code review 4.3 — see the keyframes comment).
          strokeDashoffset={sweeping ? undefined : -ringCircumference * depleted}
          className={sweeping ? 'stage-timer-sweep' : undefined}
          style={sweep.style}
        />
      </svg>

      {/* No aria-live, and no role: EXPERIENCE.md excludes the timer numeral
          from live regions - a per-second announcement would bury every other
          announcement on the page. Question-open is already announced by the
          shell's assertive region.
          leading-none, not leading-heading: at 96px a 1.38 line-height adds
          ~36px the 220px ring cannot absorb, and the mockup sets line-height:1.
          Never render the total (AC-2): totalSeconds exists solely as the
          clamp and the animation duration. */}
      <span className="relative text-[length:var(--stage-display)] font-display leading-none tabular-nums text-ink-on-dark">
        {remainingSeconds}
      </span>

      {/* The ≤5s threshold, the one moment EXPERIENCE.md's Accessibility
          Floor allows the timer to speak. Filled one commit AFTER the region
          exists (see urgentAnnounced) and never cleared while the question is
          open, so it announces exactly once — no throttle needed and none
          wanted. Separate from the answered count's region, and polite: the
          shell owns the one assertive region (UX-DR14) and two would fight. */}
      <p aria-live="polite" className="sr-only">
        {urgent && urgentAnnounced ? strings.display.question.urgentAnnouncement : ''}
      </p>
    </div>
  )
}
