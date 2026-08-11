import { useState } from 'react'

import { strings } from '@/lib/strings.he'
import { useThrottledAnnouncement } from '@/lib/use-throttled-announcement'
import type { StageProps } from './stage-props'
import { TimerRing } from './timer-ring'

// One alphabet, one source: the display and the authoring editor read the
// same array, so the two surfaces cannot drift.
const optionLetters = strings.questionEditor.optionLetters

/**
 * The question stage (FR-9, FR-10, UJ-5): DESIGN.md's split-hero, full-bleed.
 * A green-800 band on top carrying progress -> timer -> live answer count,
 * and surface-raised (WHITE) below carrying the question and its options.
 *
 * Owns BOTH question_open and question_closed (EXPERIENCE.md State Patterns:
 * "Question closed | ... | Timer at 0; options hold"). At question_closed the
 * numeral reads 0, the ring is not urgent — gold's moment has passed — and
 * the question, options and answered count hold unchanged.
 */
export function QuestionStage({ snapshot, reducedMotion }: StageProps) {
  const question = snapshot.currentQuestion
  // emptySnapshot carries the real State with a nil CurrentQuestion, so a
  // post-commit buildSnapshot failure delivers question_open with no
  // question. There is no error boundary in this app; an unguarded deref
  // here puts React Router's English crash page on the projector.
  //
  // This early return sits ABOVE every hook, which is why the high-water
  // state and the announcer live in AnsweredCount below and the countdown
  // lives in TimerRing: an early return between hooks is a rules-of-hooks
  // violation the linter catches.
  if (!question) {
    return (
      <p className="text-[length:var(--stage-heading)] font-heading text-text-secondary">
        {strings.display.waiting}
      </p>
    )
  }

  // Branch on the TYPE, not on `options` being present: a malformed MCQ that
  // arrives with no options renders the hint rather than an empty column.
  const options = question.type === 'mcq' ? question.options : undefined

  return (
    // Full bleed. The shell root is position:relative with
    // padding:var(--stage-margin), and absolute offsets resolve against the
    // PADDING box — so inset-0 covers the safe margin too, which is what
    // "full bleed with the 48px margin inside it" means (4.2's pattern).
    // Unlike 4.2 there is no separate backdrop element: this one wrapper IS
    // the layout, so the painting-order trap does not arise. Not -z-10: the
    // shell root establishes no stacking context, so a negative-z child would
    // slide behind its background.
    <div className="absolute inset-0 flex flex-col bg-surface-raised">
      {/* The band: the game's control domain. 35% is the mockup's value and
          its own comment explains why ("35% fits 40px labels") — do not tidy
          it to DESIGN.md's 30% end of the range. rounded-b-md = 12px, per
          DESIGN.md Shapes: no radius at the top (full bleed), rounded.md on
          the bottom edge. */}
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
          {strings.live.questionProgress(question.position, snapshot.questionCount)}
        </p>

        {/* The key is load-bearing: it is what re-captures the ring's sweep
            for a new question. TimerRing captures animationDelay once per
            mount on purpose, so without this a second question would resume
            the first one's arc. */}
        <TimerRing
          key={`${question.id}:${question.answerCutoffAt}`}
          deadlineIso={question.answerCutoffAt}
          totalSeconds={question.timeLimitSeconds}
          closed={snapshot.state === 'question_closed'}
          reducedMotion={reducedMotion}
        />

        <AnsweredCount questionId={question.id} count={question.answeredCount} />
      </div>

      {/* The body: the thinking domain. bg-surface-raised (WHITE) on the
          wrapper above, not bg-surface-base — DESIGN.md split-hero.bottom is
          surface-raised and the shell's own ground is surface-base; they are
          different colours and getting it wrong is a visible, easy-to-miss
          defect. min-h-0 so the children may shrink instead of overflowing
          the shell's overflow-hidden. */}
      <div className="flex min-h-0 flex-1 flex-col justify-center gap-4 px-[var(--stage-margin)] pt-4 pb-[var(--stage-margin)]">
        {/* <h1>, not <p>: nothing on the Audience Display had heading
            semantics (deferred-work.md, 4.2 entry, re-triggered by this
            story's StageProps revision). The question is unambiguously this
            stage's heading, Tailwind's preflight resets h1 font-size and
            font-weight to inherit so there is ZERO visual change, and only
            one stage is mounted at a time so there is never a second <h1>.
            text-primary on white is 10.6:1. leading-heading (1.38) here, per
            DESIGN.md Typography — the band's 1.2 deviation does not extend to
            the body. */}
        <h1 className="text-center text-[length:var(--stage-heading)] font-heading leading-heading text-text-primary">
          {question.text}
        </h1>

        {options ? (
          // Single column, deliberately: DESIGN.md offers "or 2x2 grid when
          // all four options are short" as an [ASSUMPTION], and
          // mockups/key-stage-question.html — the authoritative pixel
          // resolution — renders a single column.
          <div className="flex min-h-0 flex-col gap-4">
            {options.map((text, index) => (
              // key={index} deliberately: options have no id, and position IS
              // their identity (DESIGN.md: "position-as-identity").
              //
              // basis-[var(--stage-option-min)] with shrink and no grow: the
              // rows are 96px at 1080p (the spec figure) and shrink only when
              // a long question leaves no room, instead of clipping.
              // overflow-hidden keeps a shrunk row from spilling its text.
              // rounded-sm (8px) — "options are choices, not tags", never
              // rounded-full.
              <div
                key={index}
                className="flex min-h-0 shrink basis-[var(--stage-option-min)] items-center gap-4 overflow-hidden rounded-sm bg-green-800 px-6 text-[length:var(--stage-body)] leading-body text-ink-on-dark"
              >
                {/* Letter bold, text regular. DESIGN.md stage-option says
                    letterWeight 700 / textWeight 500, and the Don'ts row
                    calls same-weight "the letter becomes invisible". This
                    project's @theme has no 700 step; font-heading (800) is
                    the nearest existing one and preserves the CONTRAST the
                    spec is about — recorded rather than solved by inventing a
                    sixth weight token.
                    The letter sits on the inline-start side for free: flex +
                    gap-4 in a dir="rtl" document. No flex-row-reverse and no
                    physical margins — logical properties only.
                    ?? '' guards an options array longer than four:
                    CurrentQuestion.Options is a raw []string on the wire and
                    the four-option rule is enforced at authoring time, not
                    here. */}
                <span className="font-heading">
                  {strings.display.question.optionLetter(optionLetters[index] ?? '')}
                </span>
                <span className="font-body">{text}</span>
              </div>
            ))}
          </div>
        ) : (
          // In PLACE of the option rows, not in addition to them (A15).
          // Heading 800 per EXPERIENCE.md's Free-Text row.
          <p className="text-center text-[length:var(--stage-heading)] font-heading text-text-primary">
            {strings.display.question.freeTextHint}
          </p>
        )}
      </div>
    </div>
  )
}

/**
 * The live answer count and its polite announcement.
 *
 * Module-local and NOT exported: noUnusedLocals keeps it honest, and an
 * exported non-page component in a .tsx file is the shape
 * react-refresh/only-export-components exists to police. It is a child so
 * that its hooks sit below QuestionStage's degraded-snapshot early return.
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
