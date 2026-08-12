import { strings } from '@/lib/strings.he'
import { StageHeroBand } from './stage-hero-band'
import { StageOption } from './stage-option'
import type { StageProps } from './stage-props'

// One alphabet, one source: the display and the authoring editor read the
// same array, so the two surfaces cannot drift.
const optionLetters = strings.questionEditor.optionLetters

/** The smallest width a bar with a real answer behind it may render at, as a
 *  percentage of the track. ~11px at 1920 — visible from the back of a room,
 *  and small enough not to distort the comparison the bars exist to make. */
const minBarPercent = 1.5

/**
 * Largest-remainder apportionment of `counts` into whole percentages.
 *
 * NOT four independent Math.round calls, which is what this was and which does
 * not add up: [1,1,1,0] rounds to 33/33/33/0 = 99 and [2,2,2,1] to
 * 29/29/29/14 = 101. Four figures side by side on a projector are four figures
 * a room can add, so they have to total exactly 100. Floor every share, then
 * hand the leftover points to the largest fractional remainders; ties break on
 * the lower index, which is stable because option order is fixed
 * (position-as-identity). total === 0 short-circuits to all zeroes rather than
 * dividing — a question nobody answered is a real frame, and a bare n/total
 * would put "NaN%" on a projector. (Code review, 2026-08-12.)
 */
function apportion(counts: number[], total: number): number[] {
  if (total === 0) return counts.map(() => 0)
  const exact = counts.map((n) => (n / total) * 100)
  const shares = exact.map((value) => Math.floor(value))
  const order = exact
    .map((value, index) => ({ index, remainder: value - Math.floor(value) }))
    .sort((a, b) => b.remainder - a.remainder || a.index - b.index)
  let left = 100 - shares.reduce((sum, n) => sum + n, 0)
  for (const { index } of order) {
    if (left <= 0) break
    shares[index] += 1
    left -= 1
  }
  return shares
}

/**
 * The reveal stage (FR-10): the correct answer marked, and the room's
 * answers shown back to it.
 *
 * MCQ gets one distribution row per option — the pill, a bar, and a count
 * label outside the fill. Free-Text gets the answer card with the Accepted
 * Answer's primary form and a counts line, and NO bars (A15).
 *
 * Nothing here identifies a person (FR-10's out-of-scope rule / epic AC-3):
 * grades live only in each Participant's WhatsApp, the distribution is
 * aggregate by construction, and this file never reads snapshot.participants.
 */
export function RevealStage({ snapshot, reducedMotion }: StageProps) {
  const question = snapshot.currentQuestion
  // emptySnapshot carries the real State with a nil CurrentQuestion, so a
  // post-commit buildSnapshot failure delivers `revealed` with no question.
  // There is no error boundary in this app; an unguarded deref here puts
  // React Router's English crash page on the projector.
  if (!question) {
    return (
      <p className="text-[length:var(--stage-heading)] font-heading text-text-secondary">
        {strings.display.waiting}
      </p>
    )
  }
  // reveal may be null at state=revealed from two directions: emptySnapshot
  // (a post-commit buildSnapshot failure) and deploy skew against an old
  // instance that never had the field. Render the question and its options
  // UNDECORATED rather than guessing - a wrong mark in front of a room is
  // worse than an unmarked one.
  const reveal = question.reveal ?? null

  // Branch on the TYPE, not on `options` being present (4.3's rule). The
  // length test is part of the guard, not decoration: `[]` is truthy, so
  // testing the array alone would render an empty column for a malformed MCQ
  // instead of falling through. Undefined here means "no rows to draw", and
  // every branch below reads it that way. (Code review, 2026-08-12.)
  const options = question.type === 'mcq' && question.options?.length ? question.options : undefined

  // Denominator is the sum of the BARS, never answeredCount: the shell's
  // answeredFloor deliberately raises answeredCount above the frame's value,
  // and a response outside 1..options.length counts toward the total but
  // toward no bar. Either divergence would make the percentages lie or a bar
  // exceed its track. (Story 4.4, derived requirement 8.)
  //
  // The ?? 0 is a genuine runtime guard, not a type-level formality:
  // noUncheckedIndexedAccess is off, so optionCounts[i] types as `number`
  // while an array shorter than options (deploy skew, a malformed row) really
  // does yield undefined - which would render "NaN%" on a projector.
  const counts = (options ?? []).map((_, i) => reveal?.optionCounts?.[i] ?? 0)
  const total = counts.reduce((sum, n) => sum + n, 0)
  const percents = apportion(counts, total)

  // The mark and the dimming share ONE validity test. Keying the dimming on
  // `reveal` merely being non-null instead would, for a reveal that carries no
  // usable correctOption, dim all four and mark none — the screen actively
  // asserting that every answer was wrong, which is strictly worse than the
  // undecorated state derived requirement 9 specifies. correctOption is
  // 1-based, matching questions.correct_option; null here means "this reveal
  // identifies no option", and every pill then stays undecorated.
  // (Code review, 2026-08-12.)
  const correctOption = reveal?.correctOption ?? 0
  const correctIndex =
    correctOption >= 1 && correctOption <= (options?.length ?? 0) ? correctOption - 1 : null

  // Derived requirement 11: the PROP, not a media query. A CSS-only kill is
  // invisible to jsdom, and the Organizer's room-level toggle is not a media
  // query at all.
  const animated = !reducedMotion

  return (
    // Full bleed, same wrapper as the question stage: the reveal is a change
    // of the OPTIONS, not a re-layout.
    <div className="absolute inset-0 flex flex-col bg-surface-raised">
      <StageHeroBand
        question={question}
        questionCount={snapshot.questionCount}
        // Always true here: `revealed` is past the cutoff by definition, so
        // the numeral reads 0, the ring is empty and white, and gold's moment
        // has passed. The room sees no change to the ring on reveal, which is
        // the point - see story 4.4's derived requirement 6 before "fixing"
        // this to match the mockup's full circle.
        closed
        reducedMotion={reducedMotion}
      />

      {/* gap-4 / pt-4 / pb-[var(--stage-margin)] are copied from the question
          stage EXACTLY, including the fact that they deviate from the reveal
          mockup's own 1.6667cqw (32px). Matching the mockup here would make
          the question text jump at the moment of reveal, and consistency with
          the stage the room is already looking at is the stronger constraint.
          (Derived requirement 7.) */}
      <div className="flex min-h-0 flex-1 flex-col justify-center gap-4 px-[var(--stage-margin)] pt-4 pb-[var(--stage-margin)]">
        {/* Duplicated markup rather than a third extraction, deliberately: it
            is one element with one class list, and 4.5's leaderboard and
            4.6's winner will have no question heading at all. */}
        <h1 className="text-center text-[length:var(--stage-heading)] font-heading leading-heading text-text-primary">
          {question.text}
        </h1>

        {options ? (
          // The same 16px stack the question stage uses, so the four rows
          // land exactly where the four option pills were.
          <div className="flex min-h-0 flex-col gap-4">
            {options.map((text, index) => {
              // correctIndex is null when this reveal identifies no option, so
              // nothing is correct AND nothing is dimmed — the same undecorated
              // state a null reveal produces. That is why the variant
              // expression below still has three arms.
              const isCorrect = correctIndex === index
              const count = counts[index] ?? 0
              const share = percents[index] ?? 0
              return (
                // key={index} deliberately: options have no id, and position
                // IS their identity (DESIGN.md "position-as-identity").
                //
                // The ROW owns the vertical basis and the shrink — the same
                // fit behaviour 4.3 measured at 1280x720 (rows shrank 64->58px
                // rather than clipping), moved up one level because the row
                // now has three children rather than being the pill itself.
                <div
                  key={index}
                  className="flex min-h-0 shrink basis-[var(--stage-option-min)] items-center gap-6"
                >
                  <StageOption
                    letter={strings.display.question.optionLetter(optionLetters[index] ?? '')}
                    text={text}
                    variant={isCorrect ? 'correct' : correctIndex !== null ? 'dimmed' : 'default'}
                    className="h-full min-h-0 shrink-0 basis-[44%] overflow-hidden"
                  />
                  {/* aria-hidden: pure decoration. The number beside it is
                      the content. bg-surface-base (#F0FDF4) is the mockup's
                      light track and the reason a label can sit outside the
                      fill on the empty side — not white, which is already the
                      body and would make the track vanish. */}
                  <div
                    aria-hidden
                    className="h-[var(--stage-bar-track)] min-w-0 flex-1 overflow-hidden rounded-full bg-surface-base"
                  >
                    <div
                      className={`h-full rounded-full ${isCorrect ? 'bg-success' : 'bg-green-600'} ${animated ? 'stage-bar-grow' : ''}`}
                      // A count above zero never renders an invisible bar: the
                      // label beside it says somebody chose this option, and a
                      // 0%-wide fill next to "1 · 0%" reads as a rendering bug
                      // rather than as a small share. The LABEL keeps the true
                      // percentage — only the bar is floored.
                      style={{ width: `${count > 0 ? Math.max(share, minBarPercent) : share}%` }}
                    />
                  </div>
                  {/* text-[length:var(--stage-ui)] is 48px at 1080p —
                      DESIGN.md distribution-bar.label and A19's "display-only
                      content >= 48px". NOT --stage-body: the bar labels sit
                      one step above the option text.
                      overflow-hidden is the fit guard of last resort: the label
                      is shrink-0 + nowrap on a FIXED basis, so without it an
                      over-wide label spills across the bar track instead of
                      being clipped. Raising --stage-bar-label is the real fix
                      when it happens; this makes the failure quiet either way.
                      (Code review, 2026-08-12.) */}
                  <p className="flex shrink-0 basis-[var(--stage-bar-label)] items-center gap-2 overflow-hidden whitespace-nowrap text-[length:var(--stage-ui)] font-ui leading-[1.2] text-text-secondary tabular-nums">
                    {/* The mark's slot is reserved on EVERY row, not only the
                        correct one. The label is a flex row starting at the
                        inline-start edge, so rendering the ✓ conditionally
                        pushed that one row's figure out of the column that the
                        fixed basis and tabular-nums exist to create — the four
                        numbers stopped lining up. aria-hidden throughout
                        (derived requirement 10): the pill's ✓ on the same row
                        already carries the accessible name, and two
                        announcements per row is noise.
                        (Code review, 2026-08-12.) */}
                    <span aria-hidden className="w-[1.5ch] shrink-0">
                      {isCorrect ? strings.display.reveal.correctMark : ''}
                    </span>
                    <bdi dir="ltr">
                      {strings.display.reveal.distributionLabel(count, share)}
                    </bdi>
                  </p>
                </div>
              )
            })}
          </div>
        ) : reveal !== null && reveal.acceptedAnswer?.trim() ? (
          // The guard is "there is an answer to SHOW", not "a reveal exists".
          // .trim() and not a bare truthiness test, because whitespace is the
          // reachable half: a card holding one space is indistinguishable from
          // an empty one at ten metres.
          // Keyed on reveal alone, every input that reached this arm without an
          // acceptedAnswer — a free-text row whose first accepted answer is
          // empty or whitespace (bank imports bypass the only trimming, see
          // deferred-work.md's 3.4 entry), or a malformed MCQ that fell through
          // above — rendered a full-width success card whose entire content was
          // the ✓ and the screen-reader name "the correct answer". An empty
          // green card asserting it holds the answer is the same class of lie
          // the third arm below exists to prevent. (Code review, 2026-08-12.)
          //
          // In PLACE of the rows, not in addition to them (A15).
          <div className="flex min-h-0 flex-col items-center gap-4">
            {/* DESIGN.md stage-answer-card: success fill, ink-on-dark, ✓
                inline-end, rounded.md (12px), 64px — which is
                --stage-heading, the same step as the question above it. */}
            <div className="flex w-full items-center gap-4 rounded-md bg-success px-6 py-4 text-[length:var(--stage-heading)] font-heading text-ink-on-dark">
              {/* <bdi>, for the same reason the distribution label has one and
                  with more at stake: this is the ANSWER, at 64px, in a
                  dir="rtl" paragraph. An accepted answer like "(1948)" or
                  "1948-1949" has its boundary neutrals resolved to R and
                  mirrored — ")1948(" — i.e. a wrong correct answer in front of
                  a room. DESIGN.md Typography makes bidi isolation mandatory
                  and this was the one authored string on the stage without it.
                  (Code review, 2026-08-12.) */}
              <bdi className="min-w-0 break-words">{reveal.acceptedAnswer}</bdi>
              <span aria-hidden className="ms-auto">
                {strings.display.reveal.correctMark}
              </span>
              <span className="sr-only">{strings.display.reveal.correctOptionLabel}</span>
            </div>
            {/* answeredCount is the *answered* total (correctly floored by
                the shell) and correctCount comes from the reveal. This is the
                one place the two numbers legitimately come from different
                sources — derived requirement 8's denominator rule is about
                the MCQ BARS, not this line. The `·` is punctuation, not
                copy, so it may sit here rather than in strings.he.ts. */}
            <p className="text-[length:var(--stage-ui)] font-ui text-text-secondary tabular-nums">
              {strings.live.answeredStat(question.answeredCount)} ·{' '}
              {strings.display.reveal.correctStat(reveal.correctCount)}
            </p>
          </div>
        ) : (
          // Nothing to draw: no options AND no answer to show. Reached by a
          // null reveal on a free-text question (emptySnapshot, deploy skew),
          // by a reveal whose accepted answer is empty or whitespace, and by a
          // malformed MCQ that carried no options — the case the comment above
          // `options` describes, and which now really does land here.
          //
          // The card would be an empty success bar, which is a lie of a
          // different kind. Fall back to what the room was already reading a
          // moment ago, and which is true at any state.
          <p className="text-center text-[length:var(--stage-heading)] font-heading text-text-primary">
            {strings.display.question.freeTextHint}
          </p>
        )}
      </div>
    </div>
  )
}
