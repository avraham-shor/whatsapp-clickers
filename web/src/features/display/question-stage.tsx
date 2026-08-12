import { strings } from '@/lib/strings.he'
import { StageHeroBand } from './stage-hero-band'
import { StageOption } from './stage-option'
import type { StageProps } from './stage-props'

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
      {/* Shared with the reveal stage (story 4.4) rather than copied: the
          band's `leading-[1.2]` is a fit constraint measured to 0.4px. */}
      <StageHeroBand
        question={question}
        questionCount={snapshot.questionCount}
        closed={snapshot.state === 'question_closed'}
        reducedMotion={reducedMotion}
      />

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
              // These live here rather than in StageOption because they are
              // this stage's layout: the reveal stage's row is a fixed 44%
              // column beside a bar.
              //
              // ?? '' guards an options array longer than four:
              // CurrentQuestion.Options is a raw []string on the wire and the
              // four-option rule is enforced at authoring time, not here.
              <StageOption
                key={index}
                letter={strings.display.question.optionLetter(optionLetters[index] ?? '')}
                text={text}
                className="min-h-0 shrink basis-[var(--stage-option-min)] overflow-hidden"
              />
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
