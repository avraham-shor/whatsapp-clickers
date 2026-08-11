import { useEffect, useRef, useState } from 'react'

import { strings } from '@/lib/strings.he'
// import type, not a value import: display-page.tsx imports this component,
// so a value import would be a runtime cycle. A type-only import is erased
// at compile time and is not.
import type { StageProps } from './display-page'

// EXPERIENCE.md's Accessibility Floor: counters are polite AND throttled —
// "announce at most every 5s or at milestones". Module scope, not component
// scope, so it is a stable value the announcing effect can close over without
// becoming a dependency. The milestone half is deliberately not implemented:
// nothing in the spec defines what counts as one (code review, 4.2).
const announceIntervalMs = 5000

/**
 * The lobby stage (FR-10, UJ-1): how to join, in the largest type the
 * product has, plus the counter climbing as the room fills.
 *
 * `reducedMotion` is deliberately not destructured. This stage has no
 * animation of its own — the counter changes value, it does not move — so
 * there is nothing to disable. Do not invent one in order to have something
 * to turn off.
 */
export function LobbyStage({ snapshot }: StageProps) {
  // participantCount can arrive out of order: two concurrent joins each
  // do INSERT -> buildSnapshot -> Broadcast independently, and the hub
  // stamps `seq` at Broadcast()-call time, so the older (lower) count
  // can win the higher seq and the client's `seq < lastSeq` guard will
  // accept it (deferred-work.md, 2.4 entry — this story is its named
  // trigger). Holding the maximum is sound HERE and only here: in lobby
  // the count is monotonically non-decreasing by construction - nothing
  // deletes a participant row, and a spectator can only be created past
  // lobby. It is a symptom-level guard on the one surface a whole room
  // watches; the general seq-ordering fix stays deferred.
  //
  // State adjusted during render (React's documented "storing information
  // from previous renders" pattern), not a ref and not an effect:
  // display-page.tsx uses the identical shape and documents why — this
  // project's lint rules reject reading a ref during render
  // (react-hooks/refs) and calling setState from an effect body
  // (react-hooks/set-state-in-effect).
  const [highWater, setHighWater] = useState(snapshot.participantCount)
  if (snapshot.participantCount > highWater) setHighWater(snapshot.participantCount)
  // Belt-and-braces. React discards the output of a render that sets its own
  // state and re-runs the component before committing, so a committed render
  // can never observe highWater < participantCount; this Math.max is
  // therefore redundant, and is kept only because it makes the rendered value
  // self-evidently correct without the reader having to know that rule.
  // (Corrected at 4.2's code review: the original comment here claimed it
  // rescued a "one frame stale" render. React never commits that render —
  // display-page.tsx:94-96 states the same mechanism correctly.)
  const count = Math.max(highWater, snapshot.participantCount)

  // The region mounts EMPTY and announces a CHANGE, never the state it found
  // on arrival. Assistive tech does not announce content already present when
  // a live region first appears - 4.1's code review found exactly that bug on
  // the shell's state announcer, so seeding this with `count` would silently
  // swallow the first announcement.
  const [announced, setAnnounced] = useState<number | null>(null)
  // The count this stage arrived to. Nobody joined while the room was
  // watching, so it is the baseline rather than the first announcement.
  // Without it, the common case - a lobby opening at 0 - announced
  // countAnnouncement(0), a non-event announced as an event (code review,
  // 4.2).
  const mountedAt = useRef(count)
  // Wall clock of the last announcement, so the throttle window survives
  // re-renders instead of restarting with them. 0 means "never announced",
  // which is what makes the first change fire immediately.
  const lastAnnouncedAt = useRef(0)
  // Latest-ref via effect (the same pattern display-controls.tsx uses):
  // writing a ref in an effect body is allowed, reading one during
  // render is not.
  const countRef = useRef(count)
  useEffect(() => {
    countRef.current = count
  })
  // EXPERIENCE.md's Accessibility Floor: "announce at most every 5s". That is
  // an upper bound on FREQUENCY, which a leading edge respects - so the first
  // join is read out at once and later ones wait out the remainder of the
  // window, announcing whatever the count has reached by then.
  //
  // A bare setInterval was the original shape and it failed the scenario the
  // requirement exists for: its only tick was scheduled 5s after mount, so a
  // room that filled and started inside 5s announced nothing at all before
  // the state transition unmounted the stage (code review, 4.2).
  //
  // setState inside the timeout callback is async, so it does not trip
  // react-hooks/set-state-in-effect. Re-running on every `count` change is
  // deliberate and safe here: the cleanup clears the pending timer and the
  // recomputed `wait` preserves the original window rather than restarting
  // it, so a fast-filling lobby still announces on schedule.
  useEffect(() => {
    if (count === mountedAt.current || announced === count) return
    const wait = Math.max(0, announceIntervalMs - (Date.now() - lastAnnouncedAt.current))
    const id = setTimeout(() => {
      lastAnnouncedAt.current = Date.now()
      setAnnounced(countRef.current)
    }, wait)
    return () => clearTimeout(id)
  }, [count, announced])

  return (
    <>
      {/* The stage paints its own full-bleed dark ground: DESIGN.md's
          counter pill is green-50, which is the same colour as the shell's
          surface-base, so the pill would be invisible on the shell's own
          ground. mockups/key-stage-lobby.html puts the whole stage on
          green-800.
          The shell root is position:relative with padding:var(--stage-margin),
          and absolute offsets resolve against the PADDING box — so inset-0
          covers the safe margin too, which is exactly the mockup's
          full-bleed green with the 48px margin living inside it.
          Both siblings are positioned and this DOM order matters: painting
          order puts positioned elements above non-positioned in-flow
          content, so a static content sibling would end up underneath.
          Not -z-10 instead: the shell root establishes no stacking context
          (overflow-hidden does not), so a negative-z child would slide
          behind the root's own background and vanish. */}
      <div aria-hidden className="absolute inset-0 bg-green-800" />

      <div className="relative flex flex-col items-center gap-12 text-center text-ink-on-dark">
        {/* Heading 800. JOIN and the code are ONE <bdi> run, as in the
            mockup — splitting them lets the bidi algorithm reorder the
            pair. JOIN itself is not in strings.he.ts: it is the one
            product-defined Latin exception (DESIGN.md Brand & Style). */}
        <p className="text-[length:var(--stage-heading)] font-heading leading-heading">
          {strings.display.lobby.instructionPrefix}{' '}
          <bdi dir="ltr">JOIN {snapshot.joinCode}</bdi>{' '}
          {strings.display.lobby.instructionTo}{' '}
          <bdi dir="ltr" className="tabular-nums">
            {snapshot.platformNumber}
          </bdi>
        </p>

        {/* The two tokens the back row has to transcribe, grouped in their
            own tighter sub-stack so the outer 48px rhythm does not separate
            the pair. leading-[1.1], not leading-heading: at 172.8px a 1.38
            line-height burns ~66px per line for no legibility gain and
            risks clipping at 1280x720 under the shell's overflow-hidden.
            The number is NOT reformatted here — platformNumber comes from
            WHATSAPP_DISPLAY_NUMBER, which is already Meta's human-readable
            display_phone_number. */}
        {/* aria-hidden: these two tokens are a purely VISUAL affordance -
            they exist to be transcribed from the back row. Both strings are
            already spoken by the instruction line above, inside a sentence
            that gives them meaning; repeating them bare is pure redundancy
            for a screen reader. (Code review, 4.2.) */}
        <div aria-hidden className="flex flex-col items-center gap-2">
          {/* +0.08em tracking on uppercase Latin (DESIGN.md Typography). */}
          <p className="text-[length:var(--stage-lobby-code)] font-display leading-[1.1] tracking-[0.08em]">
            <bdi dir="ltr">{snapshot.joinCode}</bdi>
          </p>
          {/* Digits carry no uppercase tracking — the mockup zeroes it
              explicitly on .lobby-number. */}
          <p className="text-[length:var(--stage-lobby-code)] font-display leading-[1.1] tabular-nums">
            <bdi dir="ltr">{snapshot.platformNumber}</bdi>
          </p>
        </div>

        {/* The oversized pill, DESIGN.md components.stage-lobby.counter
            verbatim: green-50 background, green-800 tabular numeral
            (~6.9:1). rounded-full, not rounded-pill — every existing pill
            in the project (response-stats.tsx, games-list-page.tsx) uses
            the same token trio with rounded-full. The numeral steps up to
            --stage-heading (64px) rather than the mockup's off-ramp 60px:
            an existing step, still far above A19's 48px floor for
            display-only content, and it avoids inventing a sixth size.
            Padding stays on the px spacing scale; only the type ramp is
            viewport-relative. */}
        <p className="rounded-full bg-green-50 px-12 py-4 text-[length:var(--stage-ui)] font-ui text-green-800 tabular-nums">
          <span className="text-[length:var(--stage-heading)] font-heading">{count}</span>{' '}
          {strings.display.lobby.joinedLabel}
        </p>
      </div>

      {/* polite, never assertive — UX-DR14 reserves assertive for game-state
          transitions, which the SHELL owns; two assertive regions on one
          page would fight. sr-only is position:absolute, so this takes no
          layout space in the shell's flex wrapper. The visible pill updates
          immediately; only the announcement is throttled. */}
      <p aria-live="polite" className="sr-only">
        {announced === null ? '' : strings.display.lobby.countAnnouncement(announced)}
      </p>
    </>
  )
}
