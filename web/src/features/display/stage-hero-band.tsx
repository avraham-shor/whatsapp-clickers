import { useState } from 'react'

import { strings } from '@/lib/strings.he'
import type { CurrentQuestion } from '@/lib/types'
import { useThrottledAnnouncement } from '@/lib/use-throttled-announcement'
import { TimerRing } from './timer-ring'

interface StageHeroBandProps {
  question: CurrentQuestion
  questionCount: number
  /** True at question_closed AND at revealed: the numeral pins to 0, the
   *  ring is empty, the sweep does not run, and urgency is over. */
  closed: boolean
  reducedMotion: boolean
}

/**
 * The split-hero's top band (DESIGN.md split-hero.top): progress ->
 * timer -> live answer count, on green-800.
 *
 * Its own component rather than a copy in each stage that needs it (story
 * 4.4): the question stage and the reveal stage render it pixel-identically,
 * and the `leading-[1.2]` below is a FIT constraint measured to 0.4px — two
 * copies of that arithmetic is two chances to silently clip. It takes the
 * whole CurrentQuestion rather than six scalars because that is a wire type
 * both callers already hold, and six positional-ish props is where a caller
 * swaps two of them.
 */
export function StageHeroBand({
  question,
  questionCount,
  closed,
  reducedMotion,
}: StageHeroBandProps) {
  return (
    // The band: the game's control domain. 35% is the mockup's value and
    // its own comment explains why ("35% fits 40px labels") — do not tidy
    // it to DESIGN.md's 30% end of the range. rounded-b-md = 12px, per
    // DESIGN.md Shapes: no radius at the top (full bleed), rounded.md on
    // the bottom edge.
    <div className="flex flex-[0_0_35%] flex-col items-center justify-center gap-2 rounded-b-md bg-green-800 px-[var(--stage-margin)] py-4">
      {/* leading-[1.2], NOT leading-heading, and the 0.4px matters. The
          band is 378px at 1080p. With 1.2: py-4 32 + two gap-2 16 + labels
          2x48 + ring 220 = 364px, 14px of slack. With 1.38 the labels
          become 55.2px each and the stack is 378.4px — 0.4px over a 378px
          box, inside overflow-hidden, i.e. a silent clip. The mockup
          specifies 1.2 for both labels.
          ink-on-dark-muted (4.4:1 on green-800, cleared for large text
          only — these are 40px) is DESIGN.md split-hero.top verbatim.
          tabular-nums because it changes while the room watches. */}
      <p className="text-[length:var(--stage-body)] font-ui leading-[1.2] text-ink-on-dark-muted tabular-nums">
        {strings.live.questionProgress(question.position, questionCount)}
      </p>

      {/* The key is load-bearing: it is what re-captures the ring's sweep
          for a new question. TimerRing captures animationDelay once per
          mount on purpose, so without this a second question would resume
          the first one's arc. */}
      <TimerRing
        key={`${question.id}:${question.answerCutoffAt}`}
        deadlineIso={question.answerCutoffAt}
        totalSeconds={question.timeLimitSeconds}
        closed={closed}
        reducedMotion={reducedMotion}
      />

      <AnsweredCount questionId={question.id} count={question.answeredCount} />
    </div>
  )
}

/**
 * The live answer count and its polite announcement.
 *
 * Module-local and NOT exported: noUnusedLocals keeps it honest, and an
 * exported non-page component in a .tsx file is the shape
 * react-refresh/only-export-components exists to police. It is a child so
 * that its hooks sit below each stage's degraded-snapshot early return.
 */
function AnsweredCount({ questionId, count }: { questionId: string; count: number }) {
  // Same guard, same reason, same evidence as 4.2's lobby counter
  // (deferred-work.md 2.4: seq is stamped at Broadcast()-call time, and 3 of
  // 4 measured E2E runs delivered counts that go backwards). Answers arrive
  // in exactly that concurrent-writer shape. Sound HERE because within one
  // open Question the count only grows — `answers` is append-once under
  // UNIQUE (question_id, participant_id) and nothing deletes an answer row.
  //
  // Keyed on questionId, because the count must reset for a new question even
  // when nothing remounts: a reconnect delivering question_open for question 2
  // leaves the shell's `gameId:state` key unchanged, and question 2 inheriting
  // question 1's count would be a silent lie in front of a room.
  //
  // This hold is the WITHIN-MOUNT half only, and that limit is the point. It
  // does NOT survive the shell remounting the stage on a state transition, so
  // it cannot protect the question_open -> question_closed moment, which is
  // exactly where the closing snapshot can carry a count lower than one
  // already delivered (measured 8 -> 7, code review 4.3). The floor that
  // survives the remount lives in display-page.tsx, above this component;
  // this one stays as the near guard for frames arriving within one mount.
  //
  // State adjusted during render (React's documented "storing information
  // from previous renders" pattern), as display-page.tsx and lobby-stage.tsx
  // both do and both document — this project's lint rules reject reading a
  // ref during render and calling setState from an effect body.
  const [held, setHeld] = useState({ questionId, count })
  if (held.questionId !== questionId) setHeld({ questionId, count })
  else if (count > held.count) setHeld({ questionId, count })
  const shown = held.questionId === questionId ? Math.max(held.count, count) : count

  // The VISIBLE number is never throttled — only the announcement is (the
  // same split 4.2 measured: pill immediate, region silent until the 5s tick).
  // questionId is passed as the announcer's subject so the baseline moves with
  // the question: without it, a reconnect that swaps the question inside one
  // mount announced the new, lower count as an event (code review, 4.3).
  const announced = useThrottledAnnouncement(shown, questionId)

  return (
    <>
      <p className="text-[length:var(--stage-body)] font-ui leading-[1.2] text-ink-on-dark-muted tabular-nums">
        {strings.live.answeredStat(shown)}
      </p>
      {/* polite, never assertive — the shell owns the one assertive region
          (UX-DR14) and two would fight. sr-only is position:absolute, so this
          consumes no layout and no gap slot in the band's flex column. */}
      <p aria-live="polite" className="sr-only">
        {announced === null ? '' : strings.display.question.countAnnouncement(announced)}
      </p>
    </>
  )
}
