import { useEffect, useRef, useState } from 'react'

// EXPERIENCE.md's Accessibility Floor: counters are polite AND throttled —
// "announce at most every 5s or at milestones". Module scope, not component
// scope, so it is a stable value the announcing effect can close over without
// becoming a dependency. The milestone half is deliberately not implemented:
// nothing in the spec defines what counts as one (code review, 4.2).
//
// A module constant and NOT a parameter: no caller has a reason to differ,
// and a parameter would let one surface drift off the Accessibility Floor's
// number silently.
const announceIntervalMs = 5000

/**
 * A live-region value for a counter that changes while a room watches:
 * politely announced, leading-edge, at most once per 5s.
 *
 * Extracted from lobby-stage.tsx at story 4.3, when the question stage
 * became the second caller. Not copied — deferred-work.md's 3.11 entries
 * record the same single-flight pattern being duplicated into three files
 * and shipping the same bug twice, and that the fix was one lib hook rather
 * than three patches. The architecture's Web boundary rule ("shared logic is
 * promoted to lib") makes this the same call, and use-single-flight.ts /
 * use-space-action.ts set the location.
 *
 * @param value The counter to announce.
 * @param subject What the counter is counting, when that can change without
 *   the component remounting — pass the question id, the round id, whatever
 *   identifies the thing being counted. Changing it re-baselines the hook so
 *   the new subject's starting value is treated as the state it ARRIVED to
 *   rather than as an event. Omit it when the counter has one subject for the
 *   lifetime of the mount (the lobby's participant count).
 * @returns The value most recently announced, or null while nothing has been
 *   announced yet. Render it inside an aria-live="polite" region; render ''
 *   when it is null.
 */
export function useThrottledAnnouncement(value: number, subject?: string): number | null {
  // The region mounts EMPTY and announces a CHANGE, never the state it found
  // on arrival. Assistive tech does not announce content already present when
  // a live region first appears - 4.1's code review found exactly that bug on
  // the shell's state announcer, so seeding this with `value` would silently
  // swallow the first announcement.
  const [announced, setAnnounced] = useState<number | null>(null)
  // The value this hook is measuring changes FROM. Nothing changed while the
  // room was watching, so it is the baseline rather than the first
  // announcement. Without it, the common case - a lobby opening at 0 -
  // announced countAnnouncement(0), a non-event announced as an event (code
  // review, 4.2).
  //
  // State keyed on `subject`, not a bare ref, because the baseline has to be
  // able to MOVE. The question stage can outlive the question it is counting:
  // a reconnect delivering question_open for a new question leaves the shell's
  // `gameId:state` key unchanged, so nothing remounts, and a mount-time
  // baseline then belonged to the previous question - which made the new
  // question's lower count read as an event and made its count passing the
  // old baseline permanently unannounceable (code review, 4.3). Callers that
  // pass no subject get exactly the old behaviour: `undefined` never changes.
  //
  // State adjusted during render (React's documented "storing information
  // from previous renders"), as display-page.tsx and question-stage.tsx both
  // do: this project's lint rules reject writing a ref during render.
  const [baseline, setBaseline] = useState({ subject, value })
  if (baseline.subject !== subject) {
    setBaseline({ subject, value })
    // The previous subject's number must not survive into the new one's
    // region — it would be read out as though it described the new subject.
    setAnnounced(null)
  }
  // Wall clock of the last announcement, so the throttle window survives
  // re-renders instead of restarting with them. 0 means "never announced",
  // which is what makes the first change fire immediately. Deliberately NOT
  // reset with the baseline: "at most every 5s" is a bound on how often this
  // page speaks, not on how often it speaks about one subject.
  const lastAnnouncedAt = useRef(0)
  // Latest-ref via effect (the same pattern display-controls.tsx uses):
  // writing a ref in an effect body is allowed, reading one during
  // render is not.
  const valueRef = useRef(value)
  useEffect(() => {
    valueRef.current = value
  })
  // EXPERIENCE.md's Accessibility Floor: "announce at most every 5s". That is
  // an upper bound on FREQUENCY, which a leading edge respects - so the first
  // change is read out at once and later ones wait out the remainder of the
  // window, announcing whatever the value has reached by then.
  //
  // A bare setInterval was the original shape and it failed the scenario the
  // requirement exists for: its only tick was scheduled 5s after mount, so a
  // room that filled and started inside 5s announced nothing at all before
  // the state transition unmounted the stage (code review, 4.2).
  //
  // setState inside the timeout callback is async, so it does not trip
  // react-hooks/set-state-in-effect. Re-running on every `value` change is
  // deliberate and safe here: the cleanup clears the pending timer and the
  // recomputed `wait` preserves the original window rather than restarting
  // it, so a fast-changing counter still announces on schedule.
  useEffect(() => {
    if (value === baseline.value || announced === value) return
    const wait = Math.max(0, announceIntervalMs - (Date.now() - lastAnnouncedAt.current))
    const id = setTimeout(() => {
      lastAnnouncedAt.current = Date.now()
      setAnnounced(valueRef.current)
    }, wait)
    return () => clearTimeout(id)
  }, [value, announced, baseline.value])

  return announced
}
