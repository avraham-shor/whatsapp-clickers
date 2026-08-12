---
baseline_commit: 3998281
---

# Story 4.4: Reveal Stage — the Answer, Marked

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As the room,
we want the correct answer marked and the answer distribution shown,
so that the reveal lands as one shared moment (FR-10 reveal).

## Baseline: branch from `main`

**Story 4.3 merged to `main` on 2026-08-11** (PR #11, merge commit `3998281`, containing `5a4b2e0` + the review-fix commit `1019251`). Unlike 4.2→4.3 there is no branch gymnastics here: branch from `main` and confirm `1019251` is in its history.

This story consumes 4.3's shapes directly and **refactors two of them**. Verify on disk before writing any code:

- `web/src/features/display/question-stage.tsx` — the split-hero. Its hero band (progress label → `TimerRing` → `AnsweredCount`) and its option pill are **what Tasks 6 and 7 extract**.
- `web/src/features/display/timer-ring.tsx` — `TimerRingProps { deadlineIso, totalSeconds, closed, reducedMotion }`. This story reuses it unchanged, with `closed={true}`.
- `web/src/features/display/display-page.tsx` — holds the cross-stage `answeredFloor` (added at 4.3's code review) and passes a `stageSnapshot` whose `currentQuestion.answeredCount` is floored. `stageByState.revealed` is still `StagePlaceholder`.
- `web/src/features/display/stage-props.ts` — `StageProps { snapshot: LobbySnapshot; reducedMotion: boolean }`.
- `web/src/lib/use-throttled-announcement.ts` — `useThrottledAnnouncement(value: number, subject?: string)`.
- `web/src/index.css` — `.stage-root` (unlayered) holds `--stage-display/-heading/-ui/-body/-margin/-lobby-code/-timer-ring/-option-min`; `@layer components` holds `.stage-fade` and `.stage-timer-sweep`; two `@keyframes` sit outside the layer.
- `web/src/lib/strings.he.ts` — `display` block with `connecting`, `reconnecting`, `notFound`, `waiting`, `stateAnnouncement`, `lobby`, `question`.
- **Four display test files are the regression gate for Tasks 6–7**: `lobby-stage.test.tsx` (5 cases), `question-stage.test.tsx` (9), `timer-ring.test.tsx` (17), `display-page.test.tsx` (3) — **34 cases**. None of them may be edited. (Re-count them at `3998281` before relying on the number; a review-fix commit can add one.)

## Acceptance Criteria

1. **Given** the Organizer reveals, **then** the display marks the correct option with the success treatment **and** a ✓ icon — colour is never the sole signal (UX-DR14) — and shows the answer distribution across options per the distribution-bar spec: labels outside the fills, ✓ on the correct bar's label; wrong options demote to `stage-option-dimmed`. *(epic AC-1)*
2. **Given** a Free-Text Question, **then** the reveal shows the `stage-answer-card` with the Accepted Answer's primary form and the counts line "X ענו · Y צדקו" — no distribution bars (A15). *(epic AC-2)*
3. **Given** the shared screen, **then** no per-Participant private information appears (grades live only in each Participant's WhatsApp — FR-10 out-of-scope rule). *(epic AC-3)*

### Derived requirements — binding, and each has a source

These are not extra scope; they are what the epic ACs resolve to against the code that exists. Each is cited so you can check it yourself.

4. **This story is the first Epic-4 story that changes `.go` files, and it has to be.** `game/snapshot.go`'s comment on `CurrentQuestion` says the correct answer is omitted *"always, at every state, including revealed"*, and 4.3's Dev Notes recorded the consequence in as many words: *"4.4 will need a decision about how the reveal gets it."* The decision is made here (derived req. 5). The distribution counts are likewise nowhere on the wire — `CountAnswersByQuestion` returns a bare total. A REST fetch is not an option: epic 4.1 AC-2 and `architecture.md#Component-Boundaries-(Web)` both bind the display to *"the snapshot is the only data source… no TanStack Query"*.

5. **The reveal payload is a nested `reveal` object on `CurrentQuestion`, populated only when `games.state = 'revealed'`.** One nil check gates the whole secret, instead of five independently-forgettable optional fields. The old comment's fear — *"leaking the correct answer here would hand it out with no separate reveal gate to add later"* — is answered by the gate now existing: the state itself. Note also that no Participant ever receives a snapshot (they are on WhatsApp); both `role=host` and `role=display` sockets are authenticated with the Organizer's own session, so at `revealed` the answer is going only to surfaces the Organizer is already showing the room. **The gate is on `g.State`, never on the requesting role** — one payload serves both roles, and a role-conditional payload would be a new wire contract this story does not have.

6. **The hero band persists, and the ring stays empty with the numeral at 0 — it must not refill.** `mockups/key-stage-reveal.html` renders `.hero-ring` as a *full* white circle reading `0`, and that is an artifact of a static HTML mockup drawing a CSS **border**, not a specification: the same mockup family draws the ring full at 12s of a 20s question, which is definitely wrong. 4.3's review settled the behaviour deliberately (`timer-ring.test.tsx` → *"leaves the ring empty at question_closed"*): the arc shows the true remaining fraction, and at 0 it is empty. Reusing `TimerRing` with `closed={true}` therefore gives an empty ring, a white 8px stroke, no gold, no sweep — **and the room sees no change to the ring at the moment of reveal**, which is the only defensible behaviour. A ring that visibly refilled on reveal would be a bug that the mockup, read literally, tells you to write.

7. **The question `<h1>` must not move between `question_closed` and `revealed`.** The reveal is a change of the options, not a re-layout. The vertical budget is identical by construction — band 378px, body 702px, `pt-4` 16 + `pb` 48 → 638px of content; `h1` 88px (one line) / 177px (two, measured at 4.3) + `gap-4` 16 + four 96px rows with three 16px gaps = 432 → 536px / 625px. The reveal rows are the same 96px because the bar track (24px) and the label (48px × 1.2 = 57.6px) are both shorter than the option pill. **Keep the body's `gap-4` / `pt-4` / `pb-[var(--stage-margin)]` exactly as the question stage has them** — including the fact that they deviate from the reveal mockup's own `1.6667cqw` (32px), because matching the mockup here would make the question text jump on reveal, and consistency with the stage the room is already looking at wins.

8. **The distribution's denominator is the sum of `optionCounts`, never `answeredCount`.** They are normally equal — every MCQ answer is stored normalized to `"1".."4"` (migration `00010_answers.sql`'s comment on `response`). They can diverge in two reachable ways, and both break the display if `answeredCount` is the denominator: (a) `display-page.tsx`'s `answeredFloor` deliberately raises `answeredCount` above the value in the frame, so every bar would silently understate; (b) an `answers.response` outside `1..len(options)` contributes to the total but to no bar. Percentages are `Math.round(count / total * 100)` with `total === 0` short-circuiting to `0`, and the bar width is clamped to `0..100`. The band's "X ענו" label keeps using `answeredCount` — that number is the *answered* count and is correct as-is.

9. **A `revealed` snapshot whose `reveal` is `null` must render, not crash and not lie.** Two reachable sources: `game/emptySnapshot` (a post-commit `buildSnapshot` failure delivers `state: "revealed"` with `currentQuestion: null` — the 4.3 guard), and deploy skew (`store/migrate.go` documents redeploys briefly running two instances, so a new bundle's socket can land on an old one that has never heard of this field — the same reasoning `display-page.tsx` optional-chains `displaySettings` for). There is no error boundary anywhere in the app. So: `currentQuestion === null` → `strings.display.waiting`; `currentQuestion` present but `reveal === null` → render the question and its options **undecorated** — no ✓, no dimming, no bars — rather than guessing an answer. A wrong mark in front of a room is worse than an unmarked one.

10. **Both the ✓ and the dimming carry non-colour signals, and the ✓ is accessible.** UX-DR14, verbatim: *"colour is never the sole signal (✓/✗ icons alongside success/error fills, **aria-labeled**)"*. So the correct option's ✓ gets an accessible name; the **bar label's** ✓ is `aria-hidden`, because it restates the same fact on the same row and two announcements per row is noise. The bar track and fill are `aria-hidden` decoration — the label text beside them is the real content.

11. **The bar entry animation is killed from the `reducedMotion` PROP, not from a `prefers-reduced-motion` media query.** Exactly the reasoning 4.3 recorded as its derived requirement 9 and that closed 4.1's `StageProps` entry: a CSS-only kill is invisible to jsdom, so it cannot be asserted, and the room-level toggle (which is not a media query at all) has to work. EXPERIENCE.md's Motion row names the distribution bars explicitly among the animations that must have a static equivalent.

12. **The two extractions are behaviour- and DOM-preserving refactors, and the four existing display test files are the proof.** `deferred-work.md`'s 3.11 entries record this project's most expensive mistake: the same pattern copied into three files shipped the same bug twice, and the fix was one `lib` hook rather than three patches. The hero band's `leading-[1.2]` is a **fit** requirement whose arithmetic overflows a 378px box by 0.4px at 1.38 — two copies of that is two chances to silently clip. `stage-option` / `-correct` / `-dimmed` are three named components in DESIGN.md's frontmatter that differ only by fill and text colour. Both extractions are gated: `git diff 3998281 -- web/src/features/display/*.test.tsx` must be **empty** at the end of the story.

## Scope boundaries for this story

- **Backend changes are in scope, and are exactly these files**: `game/snapshot.go`, `game/engine.go` (the `Store` interface + `buildSnapshot`), `store/answers.go`, `store/queries/answers.sql`, the regenerated `store/gen/*`, and the Go tests for them. **A diff anywhere else under `server/` means you went off-spec.**
- **No migration.** Every column this story reads already exists (`questions.correct_option`, `questions.accepted_answers`, `answers.response`, `answers.is_correct`). `server/migrations/` must show no diff.
- **`sqlc generate` must be run and its output committed** — CI has a diff-check step that fails otherwise.
- **No change to `server/internal/ws/**`, `server/internal/wa/**`, `httpapi/control.go`, `grading/**`, `game/scoring.go`, `game/answers.go`, `game/results.go`, `game/final.go`.** The reveal transition, its broadcast and its WhatsApp burst are all story 3.x's and all already work.
- **`CountAnswersByQuestion` is not modified.** It runs on every `buildSnapshot`, i.e. on every inbound answer frame; the two new reads run **only at `revealed`**. Do not fold them together to save a round trip on a path that is not hot.
- **One entry in `stageByState` changes: `revealed`.** `draft`, `leaderboard`, `finished` keep `StagePlaceholder` — they belong to 4.5–4.6. Do not "while we're here" any of them.
- **No gold anywhere on this stage.** UX-DR2 rations gold to two moments: 4.3's ≤5s ring and 4.6's winner. The reveal is neither. The ring here is white.
- **No leaderboard.** The snapshot carries one, and 4.5 renders it. `deferred-work.md`'s 3.7 entry (the degraded empty leaderboard indistinguishable from all-zeros) is explicitly triggered by `leaderboard-stage.tsx`, not by this story — do not render a leaderboard and do not resolve that entry.
- **No per-Participant data of any kind** (AC-3): no names, no phone numbers, no who-answered-what. The distribution is aggregate by construction, and `snapshot.participants` must not be read by this stage.
- **No new npm package, no `shadcn add`, no `components/ui/*` edit.**
- **The four display test files are not edited** (derived req. 12).

## Tasks / Subtasks

- [x] **Task 1: `server/internal/game/snapshot.go` — the reveal payload** (AC: 1, 2; derived reqs. 4, 5)

  - [x] Add the type, and replace the "always, at every state, including revealed" clause in `CurrentQuestion`'s doc comment — that sentence is about to become false and leaving it would mislead the next reader:
    ```go
    // QuestionReveal carries what the Audience Display needs to mark the
    // answer, and it is present ONLY while games.state = 'revealed' (story
    // 4.4). That state IS the gate: before the Organizer reveals, the field
    // is nil and the correct answer is nowhere on the wire, which is the
    // property CurrentQuestion's doc comment used to hold absolutely.
    //
    // Not role-conditional: one Snapshot serves both role=host and
    // role=display over the same WS envelope, and both are authenticated
    // with the Organizer's own session. Participants never receive a
    // snapshot at all — they are on WhatsApp.
    type QuestionReveal struct {
        // 1-based index into Options; mcq only. omitempty drops the
        // free_text sentinel 0 (questions_type_shape guarantees 1..4 for
        // mcq, so a real value is never dropped).
        CorrectOption int `json:"correctOption,omitempty"`
        // The first Accepted Answer — "the primary form shown at Reveal"
        // (EXPERIENCE.md Content Rules, A15); free_text only.
        AcceptedAnswer string `json:"acceptedAnswer,omitempty"`
        // Answers per option, index-aligned with Options; mcq only.
        OptionCounts []int `json:"optionCounts,omitempty"`
        // Answers graded correct. NO omitempty: 0 correct is a real and
        // interesting number, and dropping it would make the free-text
        // counts line read "undefined צדקו".
        CorrectCount int `json:"correctCount"`
    }
    ```
  - [x] Add `Reveal *QuestionReveal \`json:"reveal"\`` to `CurrentQuestion`, last field. **Pointer with no `omitempty`**, so the wire always carries `"reveal": null` before the reveal rather than omitting the key — an absent key and a null both read as `null` in TS, but an explicit null is self-documenting in a captured frame and is what the E2E harness asserts on.
  - [x] **Nothing else in this file changes.** Not `Snapshot`, not `DisplaySettings`, not `ParticipantSummary`, and not `CurrentQuestion`'s existing eight fields.

- [x] **Task 2: `server/internal/store/queries/answers.sql` + `store/answers.go` — two reads, both reveal-only** (AC: 1, 2; derived req. 8)

  - [x] Append to `answers.sql`, after `CountAnswersByQuestion`:
    ```sql
    -- The Reveal's answer distribution (FR-10, story 4.4). MCQ only, and
    -- called only when games.state = 'revealed' — never on the hot path
    -- CountAnswersByQuestion above serves.
    --
    -- GROUP BY response is exact rather than a heuristic: migration
    -- 00010's comment on answers.response fixes MCQ storage as the
    -- normalized "1".."4" (never the raw Hebrew letter), and
    -- game.RecordAnswer is the only writer. Rows whose response falls
    -- outside 1..cardinality(options) are returned like any other and
    -- dropped by the caller — unreachable today, but the count must not
    -- silently land on the wrong bar if it ever is.
    --
    -- idx_answers_question_participant leads with question_id, so this is
    -- an index scan plus a small group, same reasoning as the count above.
    -- name: ListAnswerCountsByResponse :many
    SELECT a.response, count(*)::int AS answer_count
    FROM answers a
    WHERE a.question_id = sqlc.arg(question_id)
    GROUP BY a.response;

    -- The Reveal's "Y צדקו" (FR-10, story 4.4), for both question types.
    -- FILTER rather than a WHERE, so a question nobody got right still
    -- returns one row reading 0 instead of no rows.
    --
    -- is_correct is NOT NULL by the time this runs: Reveal is gated on
    -- CountUngradedAnswersForCurrentQuestion == 0 plus
    -- RevealCurrentQuestion's own NOT EXISTS write-guard, so every answer
    -- for this question is graded before the state can be 'revealed'.
    -- A NULL would count as not-correct, which is also the fail-closed
    -- direction (FR-16).
    -- name: CountCorrectAnswersByQuestion :one
    SELECT (count(*) FILTER (WHERE a.is_correct))::int AS correct_count
    FROM answers a WHERE a.question_id = sqlc.arg(question_id);
    ```
    (`::int` on both, so sqlc generates `int32` rather than the `bigint`/`int64` a bare `count(*)` yields — `ListQuestionResponseStats` already does exactly this; copy its shape. The alias on a single-column `:one` does not change the generated return, which is a bare scalar either way, but it names the column in `EXPLAIN` output and in the generated doc comment.)
  - [x] Add the two `store` wrappers in `answers.go` in the file's established style — thin pass-throughs, no `pgx.ErrNoRows` remap (a `:many` with no rows is not `ErrNoRows`, and the `:one` aggregate always returns a row; `store/answers.go`'s existing comments say both).
  - [x] Run `sqlc generate` (v1.31.1, matching CI) and commit `server/internal/store/gen/`. **Do not hand-edit anything under `gen/`.**
  - [x] **`CountAnswersByQuestion` is untouched** (scope boundary).

- [x] **Task 3: `server/internal/game/engine.go` — populate `Reveal` at `revealed` and nowhere else** (AC: 1, 2, 3; derived reqs. 5, 8)

  - [x] Add both methods to the `Store` interface, beside `CountAnswersByQuestion`:
    ```go
    ListAnswerCountsByResponse(ctx context.Context, questionID string) ([]gen.ListAnswerCountsByResponseRow, error)
    CountCorrectAnswersByQuestion(ctx context.Context, questionID string) (int32, error)
    ```
    (Match the generated signatures exactly — read `gen/answers.sql.go` after Task 2 rather than trusting these.)
  - [x] In `buildSnapshot`, inside the existing `if q.Position == g.CurrentQuestionPosition` block and **after** the `AnsweredCount` assignment:
    ```go
    // The answer becomes wire-visible at exactly one state (story 4.4's
    // derived requirement 5). Two extra round trips, taken only here:
    // every other state, including the question_open frame that fires on
    // every inbound answer, skips this entirely.
    if g.State == StateRevealed {
        rev, err := e.buildQuestionReveal(ctx, q)
        if err != nil {
            return Snapshot{}, err
        }
        current.Reveal = rev
    }
    ```
    - [x] Propagate the error rather than degrading. Every `buildSnapshot` caller that must not fail already routes through `snapshotAfterCommit`, which degrades to `emptySnapshot` — that machinery exists and this must not duplicate it. A reveal stage that renders `strings.display.waiting` for a beat is the documented degraded path (derived req. 9).
  - [x] Add `buildQuestionReveal(ctx, q gen.Question) (*QuestionReveal, error)` in the same file, directly below `buildSnapshot`:
    - correct count from `CountCorrectAnswersByQuestion` for **both** types;
    - `free_text` → `AcceptedAnswer = q.AcceptedAnswers[0]`, guarded on `len(...) > 0`. `questions_type_shape` enforces `cardinality(accepted_answers) >= 1`, so the guard is belt-and-braces against a row that predates the constraint — an empty string then renders an empty card rather than panicking a live game;
    - `mcq` → `CorrectOption = int(q.CorrectOption)`, and `OptionCounts` a **zero-filled `make([]int, len(q.Options))`** (non-nil, so the wire carries `[0,0,0,0]` and not `null` for a question nobody answered — the same "never null on the wire" discipline as `Participants`), filled by `strconv.Atoi` on each row's `Response` with `if err != nil || n < 1 || n > len(counts) { continue }`;
    - `free_text` sets **no** `OptionCounts` — the field stays nil and `omitempty` drops it, matching how `Options` itself is already absent on free-text.
  - [x] `StateRevealed` already exists in `game/state.go`. Do not add a new constant and do not compare against the string literal.
  - [x] **Go tests** in `engine_test.go`, extending `stubStore` with the two methods (default: zero rows / 0, so every existing test compiles and passes unchanged):
    - a `question_open` snapshot has `CurrentQuestion.Reveal == nil` **and neither new store method was called** — this is the security property, and a test that only checks the nil misses a call that leaks into logs or costs a round trip;
    - a `revealed` MCQ snapshot has `CorrectOption`, `CorrectCount`, and `OptionCounts` index-aligned with `Options`;
    - a response outside `1..len(options)` (`"0"`, `"5"`, `"ג"`) is dropped and lands on no bar;
    - a `revealed` free-text snapshot has `AcceptedAnswer` = the first accepted answer, `CorrectCount` set, and `OptionCounts` nil;
    - an error from either new read fails the whole build (mirroring `TestSnapshotAnsweredCountErrorPropagates`, which is the template — copy its shape).
  - [x] **Nothing else in `engine.go` changes.** Not `Reveal()`, not `snapshotAfterCommit`, not `detachedSnapshot`, not `emptySnapshot` (its `CurrentQuestion` is nil, so it carries no reveal by construction — say so in a test rather than adding a field).

- [x] **Task 4: `web/src/lib/types.ts` — the TS mirror** (AC: 1, 2)

  - [x] Add `QuestionReveal` mirroring the Go struct, with the same doc comment about the state gate, and `reveal: QuestionReveal | null` on `CurrentQuestion` — **non-optional and nullable**, matching the Go pointer with no `omitempty`, so the type forces the null check derived req. 9 requires. `correctOption?`, `acceptedAnswer?` and `optionCounts?` are optional (they mirror `omitempty`, exactly as `options?: string[]` already does); `correctCount: number` is not.
  - [x] Replace the corresponding clause in `CurrentQuestion`'s existing comment, which currently says the correct answer is deliberately omitted full stop.
  - [x] **Nothing else in `types.ts` changes.**

- [x] **Task 5: `web/src/index.css` — two ramp values, one animation** (AC: 1; derived req. 11)

  - [x] Append **inside the existing `.stage-root` block**, after `--stage-option-min`:
    ```css
      /* mockups/key-stage-reveal.html .bar-track height 1.25cqw = 24px at
         1920. */
      --stage-bar-track: 1.25vw;
      /* .bar-label flex-basis 13cqw = 249.6px at 1920. A FIXED basis, not
         auto: four labels of different widths would give four tracks of
         different lengths, and bars you cannot compare are not a
         distribution. Unlike --stage-lobby-code / --stage-option-min there
         is deliberately NO vh fit guard - this measure competes for
         horizontal space only, and a vh term here would shrink it on a tall
         window for no reason. Sized for the widest realistic label,
         "✓ 80 · 100%" at 48px tabular; Task 12's browser check 6 measures
         it, and raising THIS value is the sanctioned fix if it overflows. */
      --stage-bar-label: 13vw;
    ```
  - [x] Append **inside the existing `@layer components` block**, after `.stage-timer-sweep`:
    ```css
      /* The distribution bar's entry (DESIGN.md components.distribution-bar:
         "~400ms scaleX on entry"). The bar's WIDTH is inline, from the
         counts; this only animates it in.

         transform-origin is PHYSICAL and that is not an oversight: there is
         no logical transform-origin, and the Audience Display is dir="rtl"
         on the document AND again on the shell root, so 100% is always the
         inline-start edge here. The bar must grow FROM the side it is
         anchored to; growing from the far edge would slide the whole fill
         across the track.

         Layered for the same reason .stage-fade and .stage-timer-sweep are
         (code review, 2026-08-09): an unlayered rule would outrank any
         animation-* utility a later stage puts on the element.

         Deliberately NO prefers-reduced-motion rule: this stage kills the
         animation from the reducedMotion PROP (story 4.4's derived
         requirement 11) so a unit test can see it and so the Organizer's
         room-level toggle - which is not a media query at all - works. */
      .stage-bar-grow {
        transform-origin: 100% 50%;
        animation: stage-bar-grow 400ms ease-out both;
      }
    ```
  - [x] Append after `@keyframes stage-timer-deplete`, outside the layer (matching its placement):
    ```css
    /* Only a `from`: the implicit `to` is the element's own transform, so the
       bar settles at its inline width with nothing to keep in sync. */
    @keyframes stage-bar-grow {
      from { transform: scaleX(0); }
    }
    ```
  - [x] **Nothing else in `index.css` changes.** Not the `@theme` blocks, not the shadcn `:root`/`.dark` variables, not `@layer base`, not the eight existing ramp values, not `.stage-fade` / `.stage-timer-sweep` or their rules, not either existing `@keyframes`, and not the long `vw`-vs-`rem` deviation comment.
  - [x] **No new `@theme` colour token.** `success`, `green-100`, `green-600`, `green-800`, `text-primary`, `text-secondary`, `ink-on-dark`, `surface-base` all already exist. (`--color-success` is already referenced by `scoring-editor.tsx`'s `text-success`, so unlike 4.3's gold it is not tree-shaken — but `bg-success` is a different utility, so verify it in the built CSS per Task 11.)

- [x] **Task 6: `web/src/features/display/stage-hero-band.tsx` (new) — extract the band before it is copied** (no AC; derived req. 12)

  The reveal stage's hero band is pixel-identical to the question stage's. Copying it would duplicate the one measurement in this feature that is known to be 0.4px from a silent clip.

  - [x] Move, **verbatim in behaviour and DOM**, out of `question-stage.tsx`: the band `<div>` with its `flex-[0_0_35%] … rounded-b-md bg-green-800` classes, both label `<p>`s with their `leading-[1.2]` comment, the `<TimerRing>` element with its `key`, and the whole `AnsweredCount` child component. Signature:
    ```tsx
    interface StageHeroBandProps {
      question: CurrentQuestion
      questionCount: number
      /** True at question_closed AND at revealed: the numeral pins to 0, the
       *  ring is empty, the sweep does not run, and urgency is over. */
      closed: boolean
      reducedMotion: boolean
    }
    export function StageHeroBand({ question, questionCount, closed, reducedMotion }: StageHeroBandProps)
    ```
    - [x] Takes the whole `CurrentQuestion` rather than six scalars: it is a wire type both stages already hold, and six positional-ish props is where a caller swaps two of them.
    - [x] `AnsweredCount` moves with it and stays **module-local and unexported** — the same `noUnusedLocals` / `react-refresh/only-export-components` reasoning that kept it unexported in `question-stage.tsx`.
    - [x] Carry every existing comment across unchanged. The `leading-[1.2]` block, the `ink-on-dark-muted` contrast note, the within-mount-hold note and its cross-reference to `display-page.tsx`'s floor, and the `TimerRing` `key` note each record a decision that was argued once.
  - [x] `question-stage.tsx` renders `<StageHeroBand question={question} questionCount={snapshot.questionCount} closed={snapshot.state === 'question_closed'} reducedMotion={reducedMotion} />` in place of the band, and drops the now-unused `TimerRing` / `useThrottledAnnouncement` / `useState` imports.
  - [x] **Gate: `npm test` passes with all four display test files untouched.** `question-stage.test.tsx`'s answered-count cases (the backwards-count case demonstrated red at 4.3, and the question-id reset case) and `display-page.test.tsx`'s three floor cases all run through this component now. If any needs editing, the extraction changed behaviour and is wrong.

- [x] **Task 7: `web/src/features/display/stage-option.tsx` (new) — one pill, three variants** (AC: 1; derived req. 12)

  DESIGN.md's frontmatter names three components — `stage-option`, `stage-option-correct`, `stage-option-dimmed` — that differ only in fill and text colour. One component with a variant maps onto that 1:1; two copies of the class list would let the letter/text weight rule, the 8px radius and the 40px size drift.

  - [x] Signature and the variant map:
    ```tsx
    type StageOptionVariant = 'default' | 'correct' | 'dimmed'

    // COMPLETE class strings, never composed - Tailwind v4's scanner reads
    // source text, so `bg-${x}` generates nothing and the pill would render
    // unstyled on a projector with a green build.
    const variantClass: Record<StageOptionVariant, string> = {
      default: 'bg-green-800 text-ink-on-dark',
      correct: 'bg-success text-ink-on-dark',
      dimmed: 'bg-green-100 text-text-primary',
    }

    interface StageOptionProps {
      /** Already formatted by strings.display.question.optionLetter. */
      letter: string
      text: string
      variant?: StageOptionVariant
      /** Flex sizing, supplied by the caller: the question stage's
       *  full-width shrinking row vs the reveal row's fixed 44% column. */
      className?: string
    }
    ```
    - [x] Base class list, carried over from `question-stage.tsx` **minus the sizing**, which moves to the caller: `flex items-center gap-4 rounded-sm px-6 text-[length:var(--stage-body)] leading-body`.
    - [x] Join with a template string, **not `cn()`**. `cn` is `twMerge(clsx(...))` and is used nowhere outside `components/ui`; twMerge silently dropping one of two classes it thinks conflict is a failure whose only symptom is a wrong-looking projector. The variant classes and the callers' sizing classes cannot collide, so there is nothing to merge.
    - [x] `correct` additionally renders the ✓ at the inline-end with an accessible name (derived req. 10):
      ```tsx
      <span aria-hidden className="ms-auto font-heading">{strings.display.reveal.correctMark}</span>
      <span className="sr-only">{strings.display.reveal.correctOptionLabel}</span>
      ```
      `ms-auto` = `margin-inline-start: auto`, DESIGN.md `stage-option-correct.icon`'s "✓ inline-end" verbatim. No `mr-*`/`ml-*`.
    - [x] `dimmed` keeps `letterWeight: 700` (DESIGN.md `stage-option-dimmed` says so explicitly) — the letter stays `font-heading` in every variant. **"Dim the fill, never the text"**: no `opacity-*`, ever. `text-primary` on `green-100` is 9.7:1.
  - [x] `question-stage.tsx`'s option `<div>` becomes `<StageOption letter={…} text={text} className="min-h-0 shrink basis-[var(--stage-option-min)] overflow-hidden" />` with no `variant`. Its long comment block splits: the weight/RTL/radius half moves onto `StageOption`, the `basis`/`shrink`/`overflow-hidden` half and the `key={index}` note stay at the call site where those decisions live.
  - [x] **Do not add the mockup's `.opt-letter{min-width:2.2cqw}`.** 4.3 shipped without it and measured clean; adding it here would change the question stage's rendering and break the point of the gate. Record it in the Dev Agent Record as a known, pre-existing deviation for whoever next touches these rows.
  - [x] **Same gate as Task 6**: all four display test files untouched and green. `question-stage.test.tsx`'s "renders the question, four lettered options and the progress line" is the one that proves this.

- [x] **Task 8: `web/src/lib/strings.he.ts` — the reveal copy** (AC: 1, 2, 3)

  Append a `reveal` sub-block **inside the existing `display` block**, after `question`:

  ```ts
    // Reveal stage (story 4.4).
    //
    // As with `question`, live.answeredStat is NOT redefined here - the
    // free-text counts line composes it with correctStat at the call site
    // rather than restating "ענו" a third time.
    reveal: {
      // DESIGN.md stage-option-correct / stage-answer-card / distribution-bar
      // all specify "✓" as the icon. One constant, so the option, the card
      // and the bar label cannot drift onto three different glyphs.
      correctMark: '✓',
      // [ASSUMPTION]: UX-DR14 requires the ✓ to be aria-labeled but gives no
      // wording. Gender-neutral (A2), states the fact rather than the glyph.
      correctOptionLabel: 'התשובה הנכונה',
      // "54 · 71%" - mockups/key-stage-reveal.html's bar label verbatim.
      // Rendered inside <bdi dir="ltr"> by the stage: it is an all-digit run
      // in an RTL paragraph and the bidi algorithm has no strong character
      // to anchor it (DESIGN.md Typography: bidi isolation is mandatory).
      distributionLabel: (count: number, percent: number) => `${count} · ${percent}%`,
      // The "Y צדקו" half of EXPERIENCE.md's Free-Text reveal counts line
      // "X ענו · Y צדקו"; the "X ענו" half is strings.live.answeredStat.
      // Near-duplicate of results.correctColumn ('צדקו'), deliberately not
      // reused: that one is a table COLUMN HEADER on the dashboard and a
      // later copy change to either surface must not silently move the other.
      correctStat: (count: number) => `${count} צדקו`,
    },
  ```

  - [x] **Nothing else in `strings.he.ts` changes.** In particular leave `display.lobby`'s and `display.question`'s `[ASSUMPTION]` markers alone — those are 4.2's and 4.3's records to correct.
  - [x] Flag the two new `[ASSUMPTION]` items in the Dev Agent Record for the same confirm-at-review treatment 4.1–4.3's copy got.
  - [x] **Zero Hebrew literals outside this file.** The `·` separator in the counts line is punctuation, not copy, and may sit in the component — the same call 4.2 made composing the lobby instruction from fragments.

- [x] **Task 9: `web/src/features/display/reveal-stage.tsx` (new) — the answer, marked** (AC: 1, 2, 3; derived reqs. 6, 7, 8, 9, 10, 11)

  The architecture names this exact file: `reveal-stage.tsx  # correct answer + distribution`.

  - [x] **Signature**: `export function RevealStage({ snapshot, reducedMotion }: StageProps)`, `import type { StageProps } from './stage-props'`.
  - [x] **The two guards, first thing in the body, above every hook** (derived req. 9):
    ```tsx
    const question = snapshot.currentQuestion
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
    const reveal = question.reveal
    ```
  - [x] **Reuse the shell, not a copy of it.** Same full-bleed wrapper, same band, same body container as `question-stage.tsx`, and the body's spacing values are load-bearing (derived req. 7 — the `<h1>` must not move):
    ```tsx
    <div className="absolute inset-0 flex flex-col bg-surface-raised">
      <StageHeroBand
        question={question}
        questionCount={snapshot.questionCount}
        // Always true here: `revealed` is past the cutoff by definition, so
        // the numeral reads 0, the ring is empty and white, and gold's moment
        // has passed. The room sees no change to the ring on reveal, which is
        // the point - see the story's derived requirement 6 before "fixing"
        // this to match the mockup's full circle.
        closed
        reducedMotion={reducedMotion}
      />
      <div className="flex min-h-0 flex-1 flex-col justify-center gap-4 px-[var(--stage-margin)] pt-4 pb-[var(--stage-margin)]">
        <h1 className="text-center text-[length:var(--stage-heading)] font-heading leading-heading text-text-primary">
          {question.text}
        </h1>
        …distribution rows / answer card…
      </div>
    </div>
    ```
    - [x] The `<h1>` is duplicated markup rather than a third extraction, deliberately: it is one element with one class list, 4.5's leaderboard and 4.6's winner will not have a question heading, and Task 10's `deferred-work.md` entry is where the shell-wide heading decision actually belongs.
  - [x] **MCQ branch — the distribution rows** (AC-1). Branch on the **type**, matching 4.3's "branch on the TYPE, not on `options` being present" — `const options = question.type === 'mcq' ? question.options : undefined`, then render the rows when `options` is truthy:
    ```tsx
    // Denominator is the sum of the BARS, never answeredCount: the shell's
    // answeredFloor deliberately raises answeredCount above the frame's value,
    // and a response outside 1..options.length counts toward the total but
    // toward no bar. Either divergence would make the percentages lie or a bar
    // exceed its track. (Story 4.4, derived requirement 8.)
    const counts = options.map((_, i) => reveal?.optionCounts?.[i] ?? 0)
    const total = counts.reduce((sum, n) => sum + n, 0)
    const percent = (n: number) => (total === 0 ? 0 : Math.round((n / total) * 100))
    // Derived requirement 11: the PROP, not a media query.
    const animated = !reducedMotion
    ```
    The rows container is `<div className="flex min-h-0 flex-col gap-4">` — the same 16px stack the question stage uses (`.rev-rows{gap:.8333cqw}`), so the four rows land where the four option pills were. Each row:
    ```tsx
    <div className="flex min-h-0 shrink basis-[var(--stage-option-min)] items-center gap-6">
      <StageOption
        letter={strings.display.question.optionLetter(optionLetters[index] ?? '')}
        text={text}
        variant={isCorrect ? 'correct' : reveal ? 'dimmed' : 'default'}
        className="h-full min-h-0 shrink-0 basis-[44%] overflow-hidden"
      />
      {/* aria-hidden: pure decoration. The number beside it is the content. */}
      <div aria-hidden className="h-[var(--stage-bar-track)] min-w-0 flex-1 overflow-hidden rounded-full bg-surface-base">
        <div
          className={`h-full rounded-full ${isCorrect ? 'bg-success' : 'bg-green-600'} ${animated ? 'stage-bar-grow' : ''}`}
          style={{ width: `${percent(counts[index] ?? 0)}%` }}
        />
      </div>
      <p className="flex shrink-0 basis-[var(--stage-bar-label)] items-center gap-2 whitespace-nowrap text-[length:var(--stage-ui)] font-ui leading-[1.2] text-text-secondary tabular-nums">
        {isCorrect && <span aria-hidden>{strings.display.reveal.correctMark}</span>}
        <bdi dir="ltr">{strings.display.reveal.distributionLabel(counts[index] ?? 0, percent(counts[index] ?? 0))}</bdi>
      </p>
    </div>
    ```
    - [x] `isCorrect` is `reveal !== null && reveal.correctOption === index + 1` — **1-based**, matching `questions.correct_option`. When `reveal` is null nothing is correct and nothing is dimmed (derived req. 9), which is why the variant expression has three arms.
    - [x] The **row** owns the vertical basis and the shrink, and `StageOption` gets `h-full` — the same fit behaviour 4.3 measured at 1280×720 (rows shrank 64→58px rather than clipping), moved up one level because the row now has three children.
    - [x] `basis-[44%]` on the pill and `gap-6` (24px) between the three children are `mockups/key-stage-reveal.html`'s `.opt{flex:0 0 44%}` and `.rev-row{gap:1.25cqw}`. At 1920 that resolves to pill 802.6px, label 249.6px, track 723.8px.
    - [x] The track's ground is `bg-surface-base` (`#F0FDF4`) — the mockup's light track, and the reason the label can sit outside the fill on the empty side. Not white; the body is already white and the track would vanish.
    - [x] The bar label's ✓ is `aria-hidden` (derived req. 10) — the pill's ✓ on the same row already carries the accessible name.
    - [x] `text-[length:var(--stage-ui)]` is 48px at 1080p, DESIGN.md's `distribution-bar.label` figure and A19's "display-only content ≥48px". **Not `--stage-body`** — the bar labels are display-only content and sit one step above the option text.
    - [x] `animated` is `!reducedMotion` (derived req. 11). Under reduced motion the class is absent, the fill renders at its final inline width immediately, and nothing else changes.
  - [x] **Free-Text branch — the answer card** (AC-2, A15), in place of the rows, not in addition:
    ```tsx
    <div className="flex flex-col items-center gap-4">
      <div className="flex w-full items-center gap-4 rounded-md bg-success px-6 py-4 text-[length:var(--stage-heading)] font-heading text-ink-on-dark">
        <span className="min-w-0 break-words">{reveal?.acceptedAnswer ?? ''}</span>
        <span aria-hidden className="ms-auto">{strings.display.reveal.correctMark}</span>
        <span className="sr-only">{strings.display.reveal.correctOptionLabel}</span>
      </div>
      <p className="text-[length:var(--stage-ui)] font-ui text-text-secondary tabular-nums">
        {strings.live.answeredStat(question.answeredCount)} · {strings.display.reveal.correctStat(reveal?.correctCount ?? 0)}
      </p>
    </div>
    ```
    - [x] DESIGN.md `stage-answer-card`: `success` fill, `ink-on-dark`, ✓ inline-end, `rounded.md` (12px), 64px — which is `--stage-heading`, the same step as the question. Caption `text-secondary` at 48px (`--stage-ui`).
    - [x] The counts line uses `answeredCount` (the *answered* total, correctly floored by the shell) and `reveal.correctCount`. This is the one place the two numbers legitimately come from different sources — derived req. 8's denominator rule is about the MCQ **bars**, not this line.
    - [x] With `reveal === null` the card would be an empty success bar, which is a lie of a different kind. **Render the free-text branch only when `reveal !== null`; otherwise fall back to `strings.display.question.freeTextHint`**, which is what the room was already reading a moment ago and is true at any state.
  - [x] **`letters` comes from `strings.questionEditor.optionLetters`** — one alphabet, one source, exactly as `question-stage.tsx` reads it.
  - [x] **`key={index}` on the rows**, deliberately: options have no id and position is their identity (DESIGN.md "position-as-identity").
  - [x] **AC-3, and it is an assertion not a hope**: this file must not reference `snapshot.participants`, `snapshot.participantCount`, or `snapshot.leaderboard`. Nothing on this stage identifies a person.
  - [x] **Zero Hebrew literals in this file. Nothing focusable. No `shadow-*`.** Comments in English.

- [x] **Task 10: `web/src/features/display/display-page.tsx` — one map entry** (AC: 1)

  - [x] `revealed: RevealStage, // story 4.4 — reveal stage` and the import. The three other placeholder entries keep their `// story 4.x` comments unchanged.
  - [x] **Nothing else in this file changes.** The `answeredFloor` block, the `stageSnapshot` derivation, the `retained` render-phase pattern, the `?? StagePlaceholder` runtime fallback, `usePrefersReducedMotion`, the OR producing `reducedMotion`, the always-mounted assertive announcer, the three overlay branches, the `${gameId}:${state}` key and the reconnect band's offset are all 4.1–4.3 review decisions with recorded reasoning.
    - [x] In particular: the floor is keyed on the question id, not on the state, so it carries correctly from `question_closed` into `revealed` and needs no reveal-specific handling. Confirm that in the Dev Agent Record rather than changing it.

- [x] **Task 11: re-triage the `deferred-work.md` entries this story triggers** (no product code)

  Three entries name this story or fire on it, and three more need a written not-triggered. Each outcome is appended under the existing entry in the file's established style — sub-bullet, dated, trigger re-pointed off story numbers. **Do not rewrite the original text**; the file's convention is to append.

  - [x] **"The reveal-result burst and the next-question burst have no ordering guarantee"** (3.8 entry; *"Checked at 4.1's code review: NOT triggered — **story 4.4 triggers it**"*). This is the trigger, and it fires. Record: the reveal now has a *visible scripted beat* on the projector, so the window in which a Participant can receive "שאלה N+1…" before "לא נכון הפעם. התשובה: …" is now a window in which the room is watching the answer being marked. Decide and record whether that changes the disposition — the honest answer is likely still re-defer (the fix is a dispatcher-ordering design choice, and the Organizer's own click rhythm is seconds apart), but it must be **decided in writing here**, not inherited. Re-point the trigger off story numbers.
  - [x] **"Nothing on the Audience Display has heading semantics"** (4.2 entry, partially closed by 4.3, trigger re-pointed to *"the next stage story that renders a dominant text element — 4.4's answer card or 4.6's winner name"*). This is that story. Record what this story did (the reveal stage's question is an `<h1>`, same as the question stage's, because it is the same element in the same place; the free-text answer card is **not** a heading — it is the answer, and the question above it is the heading) and re-point the trigger to 4.6's winner name or the first real screen-reader evaluation. Note that the lobby stage's join code is still a `<p>` and the shell-level question is still open.
  - [x] **The SQL-coverage family** (head entry under 3.4's review; family trigger includes *"the first `sqlc`/migration change made by someone who did not write the original query"*). This story adds **two hand-written queries** whose only real exercise is a throwaway harness that gets deleted. State plainly what is and is not covered: the Go tests drive `stubStore`, so deleting `GROUP BY a.response`, inverting the `FILTER (WHERE a.is_correct)`, or pointing either query at the wrong column would leave the whole suite green — the E2E in Task 12 is the only thing that ever runs the real SQL. Re-defer with the family, and say why this story does not build the tier.
  - [x] **Confirm in writing, with reasons, that these are NOT triggered:**
    - the **2.4 out-of-order broadcast** entry (trigger: *"the first snapshot field rendered on the Audience Display that can legitimately decrease"*). The distribution counts and the correct count cannot: `answers` is append-once under `UNIQUE (question_id, participant_id)`, nothing deletes a row, and by `revealed` the cutoff has passed so no new rows arrive either. 4.5's leaderboard remains the likely trigger. Record also that this stage adds **no** new high-water guard — 4.2's own note says copying it where a value can legitimately move both ways would be a real bug, and the reason it is unnecessary here is stronger than at 4.3.
    - the **2.5 spectator-roster** entry. This stage renders no roster; a spectator never answers, so `answers`-derived counts cannot see one.
    - the **3.7 degraded-empty-leaderboard** entry (trigger: `leaderboard-stage.tsx`, story 4.5). This story renders no leaderboard.
  - [x] **Re-measure and record the reconnect-band geometry** (4.1 entry, re-deferred at 4.3 with *"the first stage whose content sits within ~20px of y=124px, or any change to the band's height or offset"*). This stage's band is the same component at the same 35%, so the numbers should reproduce 4.3's exactly (progress label 23–71px covered; ring top arc covered; numeral 141–237px, 17px clear; `<h1>` 445–533px). **Measure rather than assume** — that is what makes it a second data point on a structurally different body — and re-defer unless the numeral or the question text is covered, in which case escalate to Avraham rather than patching the stage.
  - [x] **One note, not an entry**: `deferred-work.md`'s 3.4 "bank-imported `accepted_answers` bypass the only trimming" item is partially closed and its remaining half is now *visible* — this story is the first code to render an accepted answer on a projector, so a leading/trailing space in an imported answer shows up as a misaligned card. Append that observation under the existing entry; do not open a new one.
  - [x] Do not re-triage the other Epic-4-adjacent entries; 4.1–4.3's reviews already dispositioned them.

- [x] **Task 12: quality gates, E2E, and the manual browser pass** (all ACs)

  - [x] **Backend gates**: `go build ./...` · `go vet ./...` · `go test ./...` · `sqlc generate` leaves **no diff** (CI's own check) · `git status` shows **no `migrations/` diff**.
  - [x] **Hebrew centralization**: CI's Go gate covers `*.go` — confirm the two new SQL comment blocks are English, since sqlc copies `queries/*.sql` comments verbatim into `gen/*.sql.go`, which the gate scans (`deferred-work.md`'s 3.10 entry — this is the exact trap it names). For the frontend, `rg -l '[\p{Hebrew}]' -g '*.tsx' web/src` must list only the two known pre-existing violations (`features/builder/scoring-editor.tsx`, `features/live/control-page.tsx`). **No new `.tsx` may appear** — including the test files, so use Latin placeholders for fixture question/option text as 4.3 did, and read every expected *string* from `strings.he.ts`.
  - [x] **Frontend gates**: `npm run lint` · `npx tsc -b --noEmit` · `npm test` · `npm run build` · both filter-safety scans (source and built bundle — no external URL, no font `@import`, no `url(//…)`).
  - [x] **The Task 6/7 regression gate, called out separately because it is the point of the refactors**: `git diff 3998281 -- web/src/features/display/lobby-stage.test.tsx web/src/features/display/question-stage.test.tsx web/src/features/display/timer-ring.test.tsx web/src/features/display/display-page.test.tsx` is **empty**, and all 34 of those cases pass.
  - [x] **Built-CSS verification** (4.3's rule — verify, do not assume): `--stage-bar-track`, `--stage-bar-label`, `.stage-bar-grow`, `@keyframes stage-bar-grow`, and the utilities this story is the project's first caller of — `bg-success`, `bg-green-100`, `bg-green-600`, `rounded-full` on the track, `ms-auto`, `basis-[44%]`, `basis-[var(--stage-bar-label)]`, `h-[var(--stage-bar-track)]`, `whitespace-nowrap`, `break-words`, `shrink-0`, `min-w-0`, `gap-6` — all appear in the built CSS. If one fails to resolve, fall back to `style={{…}}`, never to a hardcoded px.
  - [x] **New Vitest coverage** — co-located, `describe`/`it` imported explicitly (`globals` is off), `afterEach(cleanup)` declared per file, expected copy read from `strings.he.ts` and never retyped.
    - `reveal-stage.test.tsx`: the correct option renders the success variant **and** the ✓ with its accessible name (assert **both** — colour-alone is the specific thing UX-DR14 forbids, so a test that checks only the fill would pass the exact bug); the other three render dimmed and **not** `opacity-*`; percentages are computed off the **sum of the bars** and not off `answeredCount` (drive it with an `answeredCount` deliberately higher than the sum — the floored case — and assert the percentages still total ~100); `total === 0` renders `0 · 0%` on every row and no `NaN`; a response count array shorter than `options` fills the missing bars with 0; free-text renders the answer card, the counts line, and **zero** bars; `reveal === null` at `question_closed`-shaped input renders the options **undecorated** and does not throw; `currentQuestion === null` renders the waiting copy; `reducedMotion` removes `stage-bar-grow` while the bars keep their widths.
    - **Demonstrate at least two guards red first** (swap the denominator to `answeredCount`; drop the `?? 0` on `optionCounts[i]`) and record the failure messages, as 3.11, 4.2 and 4.3 all did. A test that has never been red has proved nothing.
    - Go side: the five `engine_test.go` cases from Task 3, with the "`question_open` calls neither new store method" one demonstrated red against a build that populates `Reveal` unconditionally.
  - [x] **Local Go E2E** (`server/cmd/e2escratch`, deleted after use — the 2.1–4.3 convention). Reuse 4.3's harness shape verbatim rather than reinventing: `websocket.Dial` from `github.com/coder/websocket` with the session cookie on the dial request header, signed-webhook POSTs (`X-Hub-Signature-256: sha256=<hmac over the RAW body>`, `webhook_test.go`'s `sign()` helper, a **unique `wamid` per message** or the dedupe ledger silently drops it), port 8099, and `WHATSAPP_API_BASE_URL` + `ANTHROPIC_API_BASE_URL` pointed at dead local addresses so nothing leaves the machine. Build the binary once, exec it directly, and grep the log for exactly one `server listening` and no `bind:` error — **a green E2E is not evidence unless you confirm which process answered** (3.10's Debug Log records a run silently served by a leftover server).

    Four things only a real server proves:
    - **Scenario A — the answer is not on the wire before the reveal.** This is the security property derived req. 5 rests on, and it is the one an ordinary test cannot see. On an open `role=display` socket, capture the `question_open` and `question_closed` frames for an MCQ question and assert `currentQuestion.reveal` is **null** in both, and that the raw JSON contains neither the correct option's index nor any accepted answer.
    - **Scenario B — the MCQ reveal payload is right and its counts add up.** Answer from ~12 phones spread deliberately across all four options (record the intended split), close, reveal, and assert on the `revealed` frame: `reveal.correctOption` matches what the question was authored with; `reveal.optionCounts` matches the intended split element for element; `sum(optionCounts) === currentQuestion.answeredCount`; `reveal.correctCount` equals the number sent to the correct option. Report the measured split, not the intended one.
    - **Scenario C — free-text.** Same shape on a free-text question with two Accepted Answers: `reveal.acceptedAnswer` is the **first** one, `reveal.optionCounts` is **absent** from the JSON, and `reveal.correctCount` counts the graded-correct answers (send at least one that matches and one that does not, so the number is not trivially equal to the total).
    - **Scenario D — one payload, both roles.** Open a second socket with `role=host` and confirm it receives the identical `reveal` object at `revealed` and `null` before. The gate is the state, not the role, and a future reader must be able to see that was verified rather than assumed.
    - Clean up the scratch rows afterward (cascades) and delete the harness.
  - [x] **Manual browser pass** (real browser — `playwright-core` + `channel: 'msedge'`, installed into the scratchpad and never into `web/package.json`; see 4.3's Debug Log for the working shape, including the fact that **the display's WS handshake is authenticated**, so the Playwright context needs `addCookies()` with the login cookie or the shell renders its connecting branch and looks like a broken stage). The unit tests cover the arithmetic; **only the browser covers the layout**, and this stage's failure modes are a silent clip and a bar that points the wrong way.

    The checks:
    1. **AC-1** — reveal an MCQ. The correct option is `rgb(21, 128, 61)` (success) with a ✓ at the **inline-end**; the other three are `rgb(209, 250, 229)` (green-100) with `rgb(20, 83, 45)` text at **full opacity**; four bars with labels outside the fills; the correct bar's fill is success and its label carries a ✓; radius 8px on the pills, pill 12px on the tracks.
    2. **AC-1, RTL direction — measure, do not eyeball.** Each bar's fill must be anchored to the **inline-start (right)** edge of its track: compare the fill's `getBoundingClientRect().right` against the track's. A fill anchored left would look plausible in a screenshot and be exactly backwards for the room.
    3. **AC-2** — reveal a free-text question: the answer card is success-filled with the accepted answer at 64px, ✓ at the inline-end, 12px radius, and the counts line beneath in `rgb(75, 85, 99)` at 48px reading "X ענו · Y צדקו" in the right order. **Zero** bars and zero option pills.
    4. **Derived req. 7 — the question must not move.** Record the `<h1>`'s `getBoundingClientRect()` at `question_closed`, press "גלה תשובה", and record it again. **Identical**, to the pixel. This is the check that catches a copied-but-drifted body container.
    5. **Derived req. 6 — the ring must not refill.** At `question_closed` and again at `revealed`: numeral 0, stroke `rgba(255, 255, 255, 0.9)` at 8px, `stroke-dashoffset` unchanged (empty), `stage-timer-sweep` absent, **no gold anywhere on the page**.
    6. **The label fit — the one number this story is least sure of.** Construct the widest realistic label: a question where one option took every answer, so the correct row reads "✓ 80 · 100%". Measure the label's `scrollWidth` against its `clientWidth` at **both** 1920×1080 and 1280×720. If it overflows, the fix is `--stage-bar-label` in `index.css`, **not** a layout rewrite — record the new value and the numbers.
    7. **Projection scale / fit.** At 1920×1080 expect: question 64px, option text 40px, bar labels 48px, band 378px, rows 96px, track 24px tall, pill 802.6px / label 249.6px / track 723.8px wide. Then the two worst realistic cases at **both** 1920×1080 and 1280×720: (a) a one-line question with four short options, (b) a **two-line** question with four long wrapping options. The shell is `overflow-hidden`, so a clipped bottom row is silent. Predicted budget is identical to 4.3's (638px of content against 536px / 625px), and 4.3 measured the 720p two-line case shrinking rows 64→58px rather than clipping — confirm that still happens now that the row has three children.
    8. **Reduced motion**, both channels separately (the dashboard's "הפחת אנימציות" toggle, and the OS setting alone): `stage-bar-grow` is absent from every fill, the bars are at their final widths immediately, and the ✓ / success / dimmed treatments are unchanged — the accessibility cues are independent of the animation.
    9. **Motion, when allowed**: the bars grow from the inline-start edge over ~400ms and settle at their widths. Confirm they do not overshoot or slide.
    10. **Reconnect at `revealed`** — kill the Go server with the reveal on screen (`ctx.setOffline` does **not** close an established socket and passes vacuously; assert `ws.on('close')` fired). The stage holds with its marks and bars, the band appears over it, and after restart it re-renders with no interaction. **Measure what the band covers** and feed the numbers to Task 11.
    11. **AC-3** — no participant name, phone number or per-person datum anywhere in the rendered DOM. Assert against a game whose participants have distinctive names, so the check can actually fail.
    12. **Palette / filter-safety** — Festival Green plus `success` only; zero gold; zero requests to any external host; no `Fetch/XHR` on the display route.
    13. Zero console errors outside a deliberate outage.
  - [x] Stories 3.10's, 4.1's and 4.2's manual passes may still be outstanding. If so, do them in the same session — this pass already puts you in front of those surfaces.

### Review Findings

Code review 2026-08-12 (bmad-code-review; three parallel adversarial layers — Blind Hunter, Edge Case Hunter, Acceptance Auditor — all three completed, no failed layer). All three epic ACs and all nine derived requirements verified implemented. **Every gate Task 12 claims was independently re-run in the triage session and reproduced green**: `go build` / `go vet` / `go test ./...` (8 packages), `sqlc generate` at v1.31.1 (the only *content* diff under `gen/` is this story's 68 additions — the two other files that appear modified after a local run are LF-vs-CRLF churn with zero content change), `tsc -b --noEmit`, `eslint`, `vitest` **47 passed / 6 files**, `npm run build`, the built-CSS check (all items present, including the three Tailwind-escaped arbitrary utilities and `@keyframes stage-bar-grow`), both Hebrew scans (`.tsx` lists exactly the two known pre-existing violations; every Hebrew-carrying `.go` file is a `_test.go` or `wa/messages_he.go`, so `gen/answers.sql.go` is clean and the sqlc-copies-comments trap was genuinely avoided), the empty-diff gate on the four display test files (**0 lines**, 34 cases green), and `server/migrations/` (**0 lines**). Scope is clean: 13 tracked files, all inside the story's allowed set. 8 findings dismissed as noise.

**The one flagged deviation — `reveal?: QuestionReveal | null` instead of Task 4's non-optional form — was audited independently and stands.** The `TS2322` was reproduced with the project's own `tsc` against the two fixture builders' actual shape: spreading `Partial<CurrentQuestion>` into a literal that omits `reveal` yields an *optional* property, which is not assignable to a required one, and both builders are in the program under `strict`. Three alternatives the Dev Agent Record did not consider were examined and rejected — excluding the tests from `tsc` (forfeits type coverage of the whole suite), a `CurrentQuestion & { reveal: QuestionReveal | null }` intersection for `RevealStage` alone (needs a cast and splits the wire contract across two declarations), and escalating rather than resolving unilaterally. The safety claim holds: `question.reveal.correctOption` is still a compile error, so derived requirement 9's guard remains compiler-forced. **Not a finding.**

**Four findings below were demonstrated by execution, not argued.** A throwaway characterisation test was rendered against the real component and deleted afterwards (working tree left byte-identical); its output is quoted inline where it applies.

**Decision needed — all three resolved 2026-08-12 by Avraham (all three → patch):**

- [x] [Review][Decision] `--stage-bar-label: 13vw` has no overflow guard and was only ever measured at two digits — Task 12's browser check 6 measured the worst case as `"✓ 80 · 100%"` (`scrollWidth 256 / clientWidth 256` at 1920×1080) and concluded the token needs no change. **That reading was correct but its margin was unknowable**: `scrollWidth` clamps to the padding box, so `scrollWidth === clientWidth` means "the content fits" and can never report headroom — a fact worth stating because the equal numbers look like a tight fit and are not evidence either way. (This review's first write-up of this finding called it "a zero-pixel margin"; that was wrong, and the browser re-measurement below is what corrected it.) What is true is that it was never measured at three digits. A room of 100+ produces `"✓ 100 · 100%"`, and because the label is `shrink-0` + `whitespace-nowrap` with **no** `overflow-hidden` on the label, the row or the body, the failure mode is a **visible spill across the bar track**, not a truncation — then a hard clip at the screen edge by `.stage-root`'s `overflow-hidden`. The story sanctions "raising THIS value is the sanctioned fix if it overflows", but raising it takes horizontal space from every track, so the call is Avraham's: (a) raise the token and re-measure at three digits, (b) add `overflow-hidden` so the failure degrades to a clip instead of a spill, or (c) accept it on the grounds that 100+ answering participants is out of the product's range. **Resolved: (a) + (b) — raise the token AND add `overflow-hidden`.** The raise buys the three-digit case; the `overflow-hidden` is the belt-and-braces that turns any future overflow into a clip instead of a spill across the track, so the next label the ramp does not anticipate degrades quietly rather than wrecking the row. Re-measure at three digits at both 1920×1080 and 1280×720 and record the numbers, as browser check 6 did for two digits. → **patch P10**. [web/src/index.css:251, web/src/features/display/reveal-stage.tsx:145]
- [x] [Review][Decision] The four percentages do not sum to 100, and a real answer can render a zero-width bar — `Math.round` with no largest-remainder correction and no minimum bar width. **Demonstrated:** counts `[1,1,1,0]` render `33 · 33 · 33 · 0` = **99%**; counts `[2,2,2,1]` render `29 · 29 · 29 · 14` = **101%**. A room can add four numbers on a projector. Separately, counts `[1,0,0,399]` render `1 · 0%` beside `style={{width:'0%'}}` — an invisible bar whose own label says somebody chose it. `reveal-stage.test.tsx` only ever drives counts that divide evenly into 10, and its own comment claims a denominator swap "shows up as percentages that no longer total 100" — which rounding alone already breaks. Options: (a) largest-remainder rounding so the four always total 100, (b) a `min-width` floor on a nonzero bar, (c) both, (d) accept and delete the misleading test comment. **Resolved: (c) — both.** Largest-remainder so the four figures always total exactly 100 (a room can and will add them up), plus a minimum width on any bar whose count is nonzero, so a real answer is never represented by an invisible bar beside a label saying somebody chose it. Add the two cases the current tests cannot see — `[1,1,1,0]` and `[2,2,2,1]` — as regression coverage, and fix the test comment that claims the percentages "still total ~100". → **patch P11**. [web/src/features/display/reveal-stage.tsx:59,135]
- [x] [Review][Decision] The two `[ASSUMPTION]` copy items need Avraham's sign-off, and only one of them carries the marker — `display.reveal.correctOptionLabel` = `'התשובה הנכונה'` is marked; `display.reveal.correctStat` = `` `${count} צדקו` `` is not, while the Dev Agent Record lists two. **The story is self-contradictory here** and the code took the correct side: Task 8's own supplied literal marks only `correctOptionLabel`, and EXPERIENCE.md gives the counts line `"X ענו · Y צדקו"` verbatim — so `צדקו` is *specified*, not assumed. Both strings are impersonal and A2-compliant, and `צדקו` mirrors the existing `${count} ענו`. Decide: approve both strings as-is (and correct the Record's "two assumptions" to one), or reword. **Resolved: approved, both strings ship as written.** `'התשובה הנכונה'` and `` `${count} צדקו` `` are signed off. The `[ASSUMPTION]` marker on `correctOptionLabel` stays (UX-DR14 genuinely gives no wording); `correctStat` correctly carries none, because EXPERIENCE.md specifies the counts line verbatim. The Dev Agent Record's "Two `[ASSUMPTION]` items need Avraham's confirmation" is the thing that is wrong and gets corrected to one — now confirmed. → **patch P12** (documentation only; no copy change). [web/src/lib/strings.he.ts:297,310]

**Patch:**

- [x] [Review][Patch] The answer-card branch fires on `reveal !== null` rather than on there being an answer to show, and the comment above it states the opposite of what the code does — the branch order is `options ? rows : reveal !== null ? card : hint`, so any input that reaches the middle arm without an `acceptedAnswer` renders `{reveal.acceptedAnswer ?? ''}` = **empty** inside a full-width `bg-success` card carrying the ✓ and the `sr-only` name `התשובה הנכונה`. **Demonstrated, twice:** an mcq with absent `options` → card whose entire text content is `"✓התשובה הנכונה"`, and `freeTextHint` rendered **0** times; a free-text whose `acceptedAnswer` is the empty string → byte-identical result. This is precisely the "lie of a different kind" the third arm's own comment says it exists to prevent, and the source comment at `reveal-stage.tsx:42-44` claims the first case "renders the free-text branch's fallback", which it does not. Both entry paths are `questions_type_shape`-guarded today, but a whitespace-only **bank-imported** accepted answer is not — and this story's own Task 11 note records that import path as bypassing the only trimming. Fix: guard the middle arm on the answer being present (`reveal?.acceptedAnswer`) so both cases fall through to `freeTextHint`, and correct the comment. [web/src/features/display/reveal-stage.tsx:42-45,155-166]
- [x] [Review][Patch] A `reveal` with no valid `correctOption` dims all four options and marks none — strictly worse than the undecorated state derived requirement 9 specifies. `isCorrect` is `reveal !== null && reveal.correctOption === index + 1`, but the variant fallback is keyed on `reveal` being non-null rather than on the reveal actually identifying an option, so `correctOption` absent (Go's `omitempty` drops the value 0) or out of range yields **4 dimmed pills, 0 marked correct, 0 accessible claim — demonstrated**. The screen actively asserts every answer was wrong. DB-unreachable today (`correct_option BETWEEN 1 AND 4`), but the guard asymmetry is free to close: key the fallback on the same validity test as the mark. [web/src/features/display/reveal-stage.tsx:103,121]
- [x] [Review][Patch] The accepted answer is the only authored string on this stage with no bidi isolation, while its sibling on the same component got one — the distribution label is wrapped in `<bdi dir="ltr">` with a comment citing DESIGN.md's "bidi isolation is mandatory", and the answer card's 64px string is a bare `<span>` inside the inherited `dir="rtl"`. An accepted answer such as `"(1948)"`, `"1948-1949"` or `"Ben-Gurion."` has its boundary neutrals resolved to R and mirrored — `(1948)` renders as `)1948(`, i.e. a **wrong** correct answer at 64px in front of a room. Fully legal input: `validateQuestion` only trims and bounds to 1–200 runes. Fix: wrap in `<bdi>`. [web/src/features/display/reveal-stage.tsx:162]
- [x] [Review][Patch] The free-text branch's container lacks the `min-h-0` its MCQ sibling has, so the counts line is the element that gets clipped — the MCQ container is `flex min-h-0 flex-col gap-4` and the free-text one is `flex flex-col items-center gap-4`. Without `min-h-0` a column flex item keeps `min-height:auto` and cannot shrink, `justify-center` splits the overflow top and bottom, and `.stage-root`'s `overflow-hidden` cuts it. The clipped element is the last one: `X ענו · Y צדקו` — the free-text reveal's only quantitative content and the **only consumer of the new `correctCount` field anywhere in the UI**. Reachable with a long-but-legal accepted answer (200 runes at 64px) under a long question (500 runes). Fix: add `min-h-0` to match the sibling. [web/src/features/display/reveal-stage.tsx:157]
- [x] [Review][Patch] The correct row's figure is pushed out of line by the ✓, defeating the `tabular-nums` and the fixed `basis` that exist to make the four comparable — the label is `flex items-center gap-2` with default `justify-content: flex-start`, so the correct row renders `[✓][gap-2][number]` while the other three render `[number]`. **Demonstrated:** the label rows carry child counts `[1, 2, 1, 1]` — only the correct row has two. Browser check 1 measured the label *box* position on every row (`label.right=298`) but never the number's offset inside it, so the manual pass could not have caught this. Fix: `ms-auto` on the `<bdi>`, or a reserved-width slot for the mark. [web/src/features/display/reveal-stage.tsx:145-150]
- [x] [Review][Patch] A required comment was silently dropped in the Task 7 extraction, and the Completion Notes claim otherwise — Task 7 says the pill's comment block "splits: the weight/RTL/**radius** half moves onto `StageOption`", and the Completion Notes state "Every comment in the extracted code was carried across unchanged". The baseline's `// rounded-sm (8px) — "options are choices, not tags", never rounded-full.` is deleted from `question-stage.tsx` and appears nowhere in `stage-option.tsx`; `rounded-sm` survives as a bare token. Every *other* extracted comment did survive verbatim. It bites specifically here: this same story puts `rounded-full` on the bar track **immediately adjacent to the pill**, which is exactly the drift the deleted rationale guarded against. Fix: restore the comment onto `StageOption`, and correct the Completion Note. [web/src/features/display/stage-option.tsx:18]
- [x] [Review][Patch] `reveal-stage.test.tsx` does not assert derived requirement 10's bar-label `aria-hidden`, though its comment says it does — the comment reads "Exactly one, on the correct option — the bar label's ✓ is aria-hidden (derived req. 10)", but the assertion is `getAllByText(revealCopy.correctOptionLabel)).toHaveLength(1)`, which counts the `sr-only` **label text** that the bar label never renders under any `aria-hidden` setting. Removing `aria-hidden` from the bar label's ✓ leaves this test green. (The production code is currently **correct** — both check-mark spans carry `aria-hidden="true"`, verified — so this is a coverage gap, not a defect.) Fix: assert on the `aria-hidden` attribute of the check-mark spans, or count `correctMark` occurrences. [web/src/features/display/reveal-stage.test.tsx:88-100, web/src/features/display/reveal-stage.tsx:146]
- [x] [Review][Patch] No Go test marshals a revealed snapshot, so the entire `omitempty` wire contract — the thing the design rests on — is unasserted on the server side. Every reveal assertion is on the Go struct. The three `omitempty` decisions (`CorrectOption`, `AcceptedAnswer`, `OptionCounts`) and the deliberate **absence** of `omitempty` on `CorrectCount` are the contract the TS types, the "nothing leaks before the reveal" property and the explicit `"reveal": null` on the wire all depend on, and only the now-deleted E2E ever checked them. Two cheap gaps alongside it: `TestSnapshotBeforeRevealCarriesNoRevealAndReadsNothing` has no `leaderboard` or `finished` case (both keep `CurrentQuestionPosition > 0`, so a `CurrentQuestion` is still built and `Reveal` silently vanishes with nothing asserting that is intended), and `engine.go`'s `len(q.AcceptedAnswers) > 0` guard is never exercised. Fix: one `json.Marshal` test over a revealed snapshot and a nil-reveal snapshot, plus the two table rows. [server/internal/game/engine_test.go]
- [x] [Review][Patch] **(P10, from decision 1)** Raise `--stage-bar-label` to fit a three-digit count and add `overflow-hidden` to the label. **Applied: `13vw → 16vw`, measured in real Edge against the built stylesheet, not reasoned.** `"✓ 100 · 100%"` at 48px tabular `system-ui` needs **288px at 1920×1080 and 193px at 1280×720**; the candidates measure 13vw → 249.6 / 166.4 (overflows both), 15vw → 288 / 192 (1080p fits exactly, **720p is 1px short**), 16vw → 307.2 / 204.8 (**19px and 12px of real slack — chosen**). **720p is the binding constraint, not 1080p**, which is the non-obvious part: the label and the type both scale with the viewport, so it is the ratio that is tight, and checking only 1080p would have shipped a 1px overflow. Four digits (1000+ answers) still overflows by 6px at both and is now **contained by the new `overflow-hidden` with zero page overflow** — which is exactly the belt-and-braces behaviour the decision asked for. [web/src/index.css:251, web/src/features/display/reveal-stage.tsx:145]
- [x] [Review][Patch] **(P11, from decision 2)** Replace the bare `Math.round` with largest-remainder rounding so the four percentages always total exactly 100, and give any bar with a nonzero count a minimum width so a real answer is never an invisible bar. Add `[1,1,1,0]` and `[2,2,2,1]` as regression cases, and fix the test comment claiming the percentages "still total ~100" — rounding alone already broke that claim. [web/src/features/display/reveal-stage.tsx:59,135, web/src/features/display/reveal-stage.test.tsx:117-129]
- [x] [Review][Patch] **(P12, from decision 3)** Correct the Dev Agent Record and Completion Notes: **one** `[ASSUMPTION]` item, not two, and it is now confirmed. Both Hebrew strings are approved as written and neither changes. [_bmad-output/implementation-artifacts/4-4-reveal-stage-the-answer-marked.md]
- [x] [Review][Patch] The 2.5 not-triggered outcome in `deferred-work.md` misquotes its own evidence — it cites the Vitest fixture as `ZZQ-PARTICIPANT-NAME`; the actual fixture is `ZZZ-PARTICIPANT-NAME`. One character, but the entry's whole value is that a future reader can go and check it. [_bmad-output/implementation-artifacts/deferred-work.md, web/src/features/display/reveal-stage.test.tsx:142]

**Deferred (appended to `deferred-work.md`):**

- [x] [Review][Defer] `SetDisplaySettings` at `revealed` now inherits two new non-degrading failure points [server/internal/game/engine.go:238-244,565-575] — deferred, widens an existing deliberately-chosen surface
- [x] [Review][Defer] `correctCount` is read on the MCQ path and rendered nowhere, and the MCQ wire now carries two independent answers to the same question [server/internal/game/engine.go:55-59] — deferred, the read is spec-mandated for both types
- [x] [Review][Defer] A dropped `strconv.Atoi` has no observability at all [server/internal/game/engine.go:81-85] — deferred, correct behaviour on an unreachable path, but silent
- [x] [Review][Defer] `.stage-bar-grow`'s `both` fill mode holds a `transform` on the fill indefinitely [web/src/index.css:290-321] — deferred, latent, no current consumer

**Dismissed as noise (8), with the reason each was rejected:** a stale `revealed` frame re-showing a previous answer (`use-game-socket.ts:130` already drops `seq < lastSeq`; the residual `seq`-stamping defect is the 2.4/3.1 family, whose trigger this story correctly re-pointed at 4.5) · the three reveal reads not being in one transaction (the answer set is closed at `revealed` — cutoff passed, `answers` append-once under `UNIQUE (question_id, participant_id)`) · the SQL comment's index claim (it says "index scan plus a small group", which is accurate; it never claims index-*only*) · `omitempty` dropping a zero-length `OptionCounts` (needs an mcq with 0 options, forbidden by `questions_type_shape`; the client half is covered by patch 1) · `strconv.Atoi` folding `"02"`/`"+2"` into one bar (folding is the desirable behaviour, and the only writer is `strconv.Itoa` of a 1–4 parse) · toggling reduced motion mid-reveal replaying the bars (that is the toggle working) · derived req. 8's "clamped to `0..100`" not being a literal `Math.min` (spec-internal contradiction — Task 9's own literal has no clamp — and every share is ≤100 by construction; the substantive rounding issue is decision D2) · `StageOptionVariant` being exported where the spec's literal is not (type-only export, lint-clean, unused elsewhere).

#### Patches applied — 2026-08-12

All 12 (the 9 raised plus the 3 the decisions converted). Gates re-run after: `go build` / `go vet` / `go test ./...` green (8 packages), `sqlc generate` still reproduces `gen/` with only the story's 68 additions, `tsc -b --noEmit` clean, `eslint` clean, **`npm test` 58 passed / 6 files** (up from 47 — 11 new cases, all of them guards these patches introduced), `npm run build` clean, both Hebrew scans still list exactly the two known pre-existing violations, the four-display-test-file diff still **0 lines**, `server/migrations/` still **0 lines**.

**Nine guards demonstrated RED before green** — the project's own standard, applied to the review's own work rather than only asked of the story's:

| Reverted | Test that went red |
|---|---|
| `apportion` → per-share `Math.round` | `apportions percentages so the four figures always total exactly 100` |
| card guard → `reveal !== null` | all **4** hint-fallback cases (2 mcq shapes × 2 whitespace shapes) |
| dim guard → `reveal` | all **3** `leaves every option undecorated when …` cases |
| `<bdi>` → `<span>` | `isolates the accepted answer for bidi` |
| reserved mark slot → conditional ✓ | `reserves the mark slot on every bar label…` |
| `aria-hidden` off the bar ✓ | `marks the correct option with the success fill AND an aria-labeled ✓` |
| `omitempty` **added** to `CorrectCount` | `TestRevealWireContract/mcq…` — frame shipped with no `correctCount` at all |
| `omitempty` **dropped** from `OptionCounts` | `TestRevealWireContract/free_text…` — frame shipped `"optionCounts":null` |
| state gate widened past `revealed` | `TestSnapshotAfterRevealCarriesNoReveal/leaderboard` — payload **and** both reads leaked to `leaderboard` |

**Two review-only tools, both removed:** a throwaway characterisation test under `web/src/features/display/` (deleted; it is what turned P1, P2, P5 and P11 from argued into demonstrated) and a `playwright-core` measurement driver installed **into the scratchpad, never into `web/package.json`** (P10's browser figures). The working tree holds exactly the file set it held before the review.

**One correction to this review's own first write-up**, recorded rather than quietly edited: the finding text originally called browser check 6's `256/256` reading "a zero-pixel margin". That is not what `scrollWidth` reports — it clamps to the padding box, so equal values mean "fits" and carry no information about headroom. The re-measurement is what established the real numbers, and the token comment in `index.css` now says so explicitly for the next reader.

**One claim softened rather than reported:** "the extractions are DOM-preserving" is true in effect, not literally — the `class` **attribute string** order differs from the baseline and the `className = ''` default leaves a trailing space. The class *set* is identical and Tailwind's cascade order comes from the stylesheet, so rendering is unaffected and the 34-case gate proves it.

## Dev Notes

### Why this story has a backend half, and how far it goes

Every Epic-4 story so far has been frontend-only because everything it needed was already on the snapshot. This one is not, and the gap is deliberate rather than accidental: `game/snapshot.go` withholds the correct answer *by design*, and `CountAnswersByQuestion` was written for a host-stat pill that only ever needed a total.

The backend half is therefore three small things — a nested payload type, two reveal-only reads, and a `g.State == StateRevealed` branch in `buildSnapshot`. It is not a new endpoint, not a migration, not a change to how the reveal transition works, and not a change to the hot path. `httpapi/control.go`'s `handleReveal` already commits the transition, awards points, builds the post-commit snapshot and broadcasts it; this story only makes that snapshot carry more.

### Where every rendered value comes from

| Rendered | Source | Notes |
|---|---|---|
| "שאלה 3 מתוך 10" | `currentQuestion.position`, `questionCount` | unchanged from 4.3 |
| Ring numeral `0` | `TimerRing` with `closed` | derived req. 6 — empty ring, not full |
| "76 ענו" | `currentQuestion.answeredCount` | floored by the shell; **not** the bar denominator |
| Question text | `currentQuestion.text` | same `<h1>`, same position (derived req. 7) |
| Option text, letters | `currentQuestion.options`, `strings.questionEditor.optionLetters` | unchanged from 4.3 |
| Which option is correct | `currentQuestion.reveal.correctOption` (1-based) | **new**; `questions.correct_option` |
| "54 · 71%" per option | `currentQuestion.reveal.optionCounts` | **new**; `GROUP BY answers.response` |
| Free-text answer card | `currentQuestion.reveal.acceptedAnswer` | **new**; `questions.accepted_answers[1]` |
| "Y צדקו" | `currentQuestion.reveal.correctCount` | **new**; `count(*) FILTER (WHERE is_correct)` |

`answers.response` is normalized to `"1".."4"` for MCQ at write time (`00010_answers.sql`'s own comment), which is what makes a `GROUP BY` an exact distribution rather than a heuristic. `questions_type_shape` guarantees `cardinality(options) = 4` and `correct_option BETWEEN 1 AND 4` for MCQ, and `cardinality(accepted_answers) >= 1` for free-text — so the shapes above cannot be malformed by a legal row.

### What 4.1–4.3 built that this story consumes

- **The socket, the retention and the reconnect band.** `useGameSocket`, the `retained` snapshot, the `stage-fade` cross-fade and the `${gameId}:${state}` key. **Do not touch `use-game-socket.ts`.**
- **The answered-count floor** now lives in `display-page.tsx` (4.3's code review moved the surviving half up out of the stage, after measuring an 8→7 regression across the `question_open → question_closed` remount). This stage inherits a floored `answeredCount` for free and must not add a second guard — see derived req. 8 for why that floor is exactly why the bars use their own denominator.
- **`reducedMotion`** is already the OR of the viewer's `prefers-reduced-motion` and the Organizer's room-level setting, and is also mirrored as `data-reduced-motion` on `.stage-root`. **No stage ever calls `matchMedia`.** This stage uses the prop, for the reasons in derived req. 11.
- **The projection ramp.** Use the variables; do not hardcode a px. This stage adds two.
- **The full-bleed split-hero.** 4.3 established it; this stage is its second user, and Task 6 makes the band a single shared component rather than a second copy.
- **`TimerRing`.** Reused **unchanged**. Its `closed` prop already does everything this stage needs — a `revealed`-specific variant would be reinventing a component that was reviewed twice.

### Three things about the reveal path that this stage must not be surprised by

1. **`answerCutoffAt` is already in the past at `revealed`.** `CloseCurrentQuestion` sets `answer_cutoff_at = LEAST(answer_cutoff_at, now())` (measured at 4.3: pulled back by 0.4s), so `TimerRing` reads 0 from the deadline alone. `closed={true}` is belt-and-braces on top of that, and it is what kills urgency.
2. **`current_question_position` does not move until `NextQuestion`.** So `currentQuestion` at `revealed` is still the question that was just answered — there is no separate "revealed question" to resolve.
3. **A degraded post-commit snapshot at reveal is a documented path, not a hypothetical.** `snapshotAfterCommit` falls back to `emptySnapshot`, which carries `State = "revealed"` with `CurrentQuestion = nil`, and `handleReveal` broadcasts exactly that. Derived req. 9 is the guard; 4.3 already proved the pattern works.

### The distribution bar: the four details that are easy to get subtly wrong

- **Direction.** The track is a plain block with the fill as its first child; in a `dir="rtl"` document the fill sits at the right (inline-start) edge and grows leftward. The mockup gets this from `display:flex` on the track; a block child with `width:%` gets it from normal flow. Either works — **check 2 measures it** rather than trusting either.
- **`transform-origin: 100% 50%` is physical**, and there is no logical equivalent in CSS. It is correct here only because the Audience Display is unconditionally RTL (`dir="rtl"` on `index.html` *and* on the shell root). Written down in `index.css` so nobody "fixes" it to `0% 50%` for an LTR surface that does not exist.
- **The label is outside the fill, on the empty side, always** — DESIGN.md `distribution-bar.label`, in as many words: *"always outside the fill on white"*. A label overlaid on a 4% fill is unreadable, which is the whole reason for the rule.
- **Two greens are not a distinguishable signal.** The correct bar is `success` (#15803D) and the others are `green-600` (#16A34A) — DESIGN.md's A18 note records that these two were *the same colour* before the accessibility review re-pointed `success`. They are still close. The ✓ on the correct label is what actually carries the information; the fill difference is reinforcement. Do not drop the ✓ to "reduce clutter".

### Design decisions worth flagging explicitly

- **The mockup's full ring at reveal is wrong, and the story says so on purpose** (derived req. 6). This is the single most likely thing for a careful developer to "fix" in the wrong direction, because the mockup is otherwise the authoritative pixel resolution.
- **The bar-label width is the one measurement this story is not confident about.** 13vw = 249.6px is the mockup's value, and the mockup's widest label is "54 · 71%" (8 characters, no ✓). The realistic worst case — "✓ 80 · 100%" on the correct row — estimates to ~278px at 48px tabular. It may well overflow. Check 6 exists to find out, and the fix is a ramp value.
- **The reveal body reuses the question stage's spacing rather than the reveal mockup's** (derived req. 7). The two mockups disagree with each other (`1.6667cqw` vs what 4.3 shipped), and "the question does not jump" is the stronger constraint.
- **The option letter's `min-width` is still missing**, in both stages. The mockup specifies `2.2cqw`; 4.3 shipped without it and measured clean. Task 7 deliberately does not add it, because doing so would change the question stage and defeat the extraction gate. It is now a two-caller deviation and should be recorded as one.
- **`strings.live.answeredStat` is reused by a third surface.** 4.3 flagged the cross-block import for Avraham; this story adds the free-text counts line as another caller. If the answer is "duplicate them" or "promote to a shared block", it is now a three-site rename rather than two.
- **No gold on this stage, at all.** Worth stating because the reveal *feels* like a peak moment and gold is right there in the palette. UX-DR2 rations it to two, and this is neither.

### Existing code this story modifies — current state, and what must survive

- **[web/src/features/display/question-stage.tsx](web/src/features/display/question-stage.tsx)** — loses its band and its option markup to Tasks 6 and 7 and gains two imports. Everything else is a 4.3 review decision: the degraded-snapshot early return above every hook, the branch on `question.type` rather than on `options`, the single-column comment, `key={index}`, and the `pt-4`/`gap-4`/`pb-[var(--stage-margin)]` body values.
- **[web/src/features/display/display-page.tsx](web/src/features/display/display-page.tsx)** — one map entry and one import. The `answeredFloor` / `stageSnapshot` block is the newest and most easily "simplified" thing in the file; it was added to fix a measured 8→7 regression.
- **[web/src/index.css](web/src/index.css)** — two custom properties inside `.stage-root`, one rule inside `@layer components`, one `@keyframes` beside the two existing ones.
- **[web/src/lib/strings.he.ts](web/src/lib/strings.he.ts)** — one `reveal` sub-block inside `display`.
- **[web/src/lib/types.ts](web/src/lib/types.ts)** — one interface, one field, one comment correction.
- **[server/internal/game/snapshot.go](server/internal/game/snapshot.go)** — one struct, one field, one comment correction. The correction matters: the existing comment states an absolute that this story ends.
- **[server/internal/game/engine.go](server/internal/game/engine.go)** — two interface methods, one branch in `buildSnapshot`, one new helper. `Reveal()`, `snapshotAfterCommit`, `detachedSnapshot` and `emptySnapshot` are untouched.
- **[server/internal/store/queries/answers.sql](server/internal/store/queries/answers.sql)** + **[server/internal/store/answers.go](server/internal/store/answers.go)** — two appended queries and two thin wrappers. Nothing existing is modified.
- **[_bmad-output/implementation-artifacts/deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md)** — Task 11's written outcomes, appended under the existing entries.

### Testing standards

Vitest (since 3.11): co-located `*.test.tsx`, one `test` block in `web/vite.config.ts` (`environment: 'jsdom'`, `include: ['src/**/*.test.{ts,tsx}']`), `globals` **off** so every test imports `describe`/`it`/`expect`/`vi` explicitly, `afterEach(cleanup)` per file, and a CI step in the existing `frontend` job. Expected Hebrew is read from `strings.he.ts`, never retyped; fixture question/option **data** uses Latin placeholders so the frontend Hebrew grep stays clean (4.3's finding).

**Test the arithmetic and the guards, not the layout.** Asserting Tailwind class strings in jsdom proves nothing about a projector — the browser pass is what covers layout. The exceptions are the class assertions that stand in for a *behaviour* with no other observable: `stage-bar-grow` present/absent is how derived req. 11 is proved, and the success/dimmed variant is how AC-1's non-colour-alone requirement is proved alongside the ✓.

Go: stdlib `testing`, co-located `_test.go`, stubs grown in place — no mock framework. **No real-DB unit tests**: the documented standard since 2.1, and the reason Task 11 has a SQL-coverage entry to re-triage rather than a tier to build. `go test -race` may be unavailable (`CGO_ENABLED=0` in this environment, carried since 3.8) — if so, say so in the Dev Agent Record rather than implying race coverage.

### Project Structure Notes

**New:**
- `server/internal/game/` — no new file; `QuestionReveal` belongs in `snapshot.go` beside the type it hangs off
- `web/src/features/display/reveal-stage.tsx` — named by the architecture (`reveal-stage.tsx  # correct answer + distribution`)
- `web/src/features/display/reveal-stage.test.tsx`
- `web/src/features/display/stage-hero-band.tsx` — not in the architecture's tree; it exists to stop a measured fit constraint being duplicated, and Task 11 records why (the `stage-props.ts` precedent from 4.3)
- `web/src/features/display/stage-option.tsx` — likewise; it maps 1:1 onto three named DESIGN.md components

**Modified:**
- `server/internal/game/snapshot.go`, `server/internal/game/engine.go`, `server/internal/game/engine_test.go`
- `server/internal/store/queries/answers.sql`, `server/internal/store/answers.go`, `server/internal/store/gen/*` (regenerated)
- `web/src/features/display/question-stage.tsx` (two extractions), `display-page.tsx` (one map entry)
- `web/src/index.css` (+2 custom properties, +1 component rule, +1 keyframes)
- `web/src/lib/strings.he.ts` (+`display.reveal` block), `web/src/lib/types.ts` (+`QuestionReveal`)
- `_bmad-output/implementation-artifacts/deferred-work.md`

**Untouched (a diff here means you went off-spec):** `server/migrations/**` · `server/internal/ws/**` · `server/internal/wa/**` · `server/internal/httpapi/**` · `server/internal/grading/**` · `server/internal/game/{answers,scoring,results,final,state,participants}.go` · `web/src/lib/{use-game-socket,api,text,use-space-action,use-single-flight,use-throttled-announcement}.ts` · `web/src/features/display/timer-ring.tsx` · `web/src/features/display/{lobby-stage,stage-placeholder,stage-props}.*` · `web/src/app.tsx` · `web/src/components/**` · `web/src/features/{lobby,live,builder,results,auth}/**` · `web/index.html` · `web/package.json` · `web/vite.config.ts` · `.github/workflows/ci.yml` · all four existing display test files.

### References

- Epic + ACs: [epics.md](_bmad-output/planning-artifacts/epics.md#L716-L731) (Story 4.4), [#L652-L654](_bmad-output/planning-artifacts/epics.md#L652-L654) (Epic 4 framing)
- Visual spec: DESIGN.md frontmatter `components.stage-option`, `stage-option-correct`, `stage-option-dimmed`, `stage-answer-card`, `distribution-bar`, `split-hero`, `timer-hero`; `Colors` contrast table + the A18 note re-pointing `success`; `Typography` → A19 projection ramp ("display-only content ≥48px"); `Layout & Spacing`; `Shapes` (options `rounded.sm`, answer card `rounded.md`); `Elevation & Depth` ("Audience Display: flat"); `Components` (the `stage-option` / `distribution-bar` / `stage-answer-card` paragraphs); `Do's and Don'ts` → success + ✓ never colour alone, dim the fill never the text
- Behaviour: EXPERIENCE.md → IA `Audience Display — stages` (Reveal — MCQ and Free-Text rows); `Component Patterns` → Reveal MCQ / Reveal Free-Text; `State Patterns` → Reveal row; `Content Rules` → "The first Accepted Answer is the primary form shown at Reveal `[A15]`"; `Accessibility Floor` → Motion (distribution bars named), live regions, distance legibility
- Mockup: `_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/mockups/key-stage-reveal.html` — the authoritative pixel resolution for the MCQ reveal, **except its static full ring** (derived req. 6) and its body spacing (derived req. 7). There is no free-text reveal mockup; DESIGN.md's `stage-answer-card` frontmatter is the spec for that half.
- FR-10 (per-state content, no per-Participant data), FR-15/FR-16 (grading, published only at Reveal): [prd.md](_bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md)
- Architecture: the `features/display/` tree naming `reveal-stage.tsx`; `Component Boundaries (Web)` ("display/* renders exclusively from the WS snapshot"); `Dependency direction` (`game` imports `store`, never the reverse); `Requirements to Structure Mapping` (§4.5 → "reveal-stage shows distribution"); `Structure Patterns` (co-located Vitest)
- Deferred entries this story must resolve or explicitly clear: [deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md) — the 3.8 reveal/next-question ordering entry (*"story 4.4 triggers it"*), the 4.2 heading-semantics entry (trigger: *"4.4's answer card or 4.6's winner name"*), the 3.4/3.7/3.10 SQL-coverage family (this story adds two queries), and the 4.1 reconnect-band entry (re-measure). Not triggered, but confirm in writing: the 2.4 out-of-order entry, the 2.5 spectator-roster entry, and the 3.7 degraded-empty-leaderboard entry.
- Prior stories: [4-3-question-stage-with-the-authoritative-timer.md](_bmad-output/implementation-artifacts/4-3-question-stage-with-the-authoritative-timer.md) (the split-hero, the ring, the extraction-with-a-gate pattern, and the measured fit budget this story reuses) · [4-2-lobby-stage-the-room-fills-the-screen.md](_bmad-output/implementation-artifacts/4-2-lobby-stage-the-room-fills-the-screen.md) (the full-bleed pattern and the out-of-order measurement) · [3-4](_bmad-output/implementation-artifacts/3-4-grading-pipeline-mcq-and-exact-match.md) / [3-7](_bmad-output/implementation-artifacts/3-7-scoring-with-speed-bonuses.md) (what "graded" and "revealed" mean at the DB level)

### Latest technical information

No dependency changes and no new libraries. Versions as pinned: React 19.2, React Router 8.2, Tailwind CSS 4.3, TypeScript 6.0, Vite 8.1, Vitest 4.1, `@testing-library/react` 16.3, `eslint-plugin-react-hooks` 7.1; Go 1.26, pgx v5, sqlc **1.31.1** (CI installs exactly this and diff-checks the output — a different local version will fail the build even if the SQL is right).

Version-specific details that will bite if missed:

- **Tailwind v4 reads source text, not runtime values.** Every class must appear as a complete literal — `bg-${variant}` or `` `bg-${isCorrect ? 'success' : 'green-600'}` `` inside a template generates *nothing* and the bars render transparent. Task 7's variant map and Task 9's ternary both keep whole class strings on both arms for this reason.
- **`text-*` with a CSS variable needs the `length:` hint**: `text-[length:var(--stage-ui)]`, not `text-[var(--stage-ui)]` (ambiguous between font-size and colour). Established at 4.1, verified in the built CSS at 4.2 and 4.3.
- **`verbatimModuleSyntax: true`** (`tsconfig.app.json`): `StageProps`, `CurrentQuestion` and `QuestionReveal` must all be imported with `import type`. `noUncheckedIndexedAccess` is **off**, so `optionCounts[i]` types as `number` rather than `number | undefined` — the `?? 0` in Task 9 is a genuine runtime guard against a short array, not a type-level formality, and no lint rule will flag it as redundant (the config is `tseslint.configs.recommended`, not the type-checked variant).
- **`eslint-plugin-react-hooks` v7** enforces `react-hooks/refs` (no ref reads during render) and `react-hooks/set-state-in-effect`, and the rules-of-hooks ordering that makes Task 9's early returns require any hooks to live in children. `RevealStage` as specified needs **no hooks at all** — if you find yourself adding one, put it in a child rather than moving the guards.
- **sqlc `:one` with a single unnamed aggregate column** generates a bare scalar return (`int32`), not a row struct — name the column (`AS correct_count`) if you want a named field, and read the generated signature before writing the `Store` interface line rather than guessing it.
- **`count(*) FILTER (…)` returns `bigint`**; the `::int` casts in Task 2 are what keep sqlc from generating `int64` and forcing conversions at three call sites. The existing `ListQuestionResponseStats` does exactly this — copy its shape.

## Dev Agent Record

### Agent Model Used

claude-sonnet-5 (Claude Code, `/bmad-dev-story`).

### Debug Log References

**Branch.** `main` was at `3998281` exactly — the story's baseline — with `1019251` in its history, so no branch gymnastics were needed. Work branched from there as `story/4-4-reveal-stage`.

**Two binding requirements in this story collide, and the collision is real rather than a misreading.** Task 4 mandates `reveal: QuestionReveal | null` — **non-optional** — on the TS `CurrentQuestion`. Derived requirement 12 and Task 12 mandate that the four display test files show an **empty** `git diff` against `3998281`. Non-optional is a compile error in two of those four files (`question-stage.test.tsx:31`, `display-page.test.tsx:33`), whose fixture builders predate the field:

```
error TS2322: Types of property 'reveal' are incompatible.
  Type 'QuestionReveal | null | undefined' is not assignable to type 'QuestionReveal | null'.
```

**Resolved in favour of the gate**, and the field is `reveal?: QuestionReveal | null`. The reasoning, recorded in full in `types.ts` beside the field: the optional form loses **nothing the non-optional form was for** — reading `reveal.correctOption` without a check is still a type error, so derived requirement 9's guard is still compiler-forced — whereas *any* edit to those four files destroys the extraction gate outright, and the gate is stated as an absolute twice, with reasoning about this project's most expensive mistake. What the optional form does lose is documentation fidelity to the wire; that is recovered by the comment and by the E2E, which asserts on the raw JSON's explicit `"reveal":null`. **The Go side is exactly as Task 1 specifies** — pointer, no `omitempty`, the key is always on the wire. This is the one place the implementation deviates from the story's letter, and it is flagged here for the reviewer.

**Two Hebrew-gate violations were introduced and caught by the gate, both in my own comments.** (1) `snapshot.go` — Task 1's specified comment text contains `"undefined צדקו"`, and CI's Go gate scans `*.go` exempting only `_test.go` and `wa/messages_he.go`, so shipping it verbatim would have failed the build. Reworded to "…render its count as `undefined`". (2) Task 2's specified SQL comment says `The Reveal's "Y צדקו"` — and Task 12 names this exact trap (`deferred-work.md`'s 3.10 entry: sqlc copies `queries/*.sql` comments verbatim into `gen/*.sql.go`, which the gate scans). Written as `"Y correct"`. Both are deviations from the story's literal text made *because* the story's own Task 12 forbids the literal text.

**No question-type constant exists in `package game`** — `answers.go:199` switches on the bare literal `"mcq"`. `buildQuestionReveal` matches that convention rather than introducing a constant, which would have been a rename touching files this story must not.

**Quality gates.** Backend: `go build ./...` · `go vet ./...` · `go test ./...` — all 8 packages `ok`. `sqlc generate` (v1.31.1, matching CI) run and committed; re-running leaves **no diff** beyond the intended 68 added lines in `gen/answers.sql.go`. `server/migrations/` shows **no diff**. `gofmt`: local `gofmt -l .` flags ~50 files including ones this story never touched — it is **CRLF churn from the Windows checkout, not formatting**; verified by normalizing to LF and re-running, which reports every touched file clean (control: an untouched file behaves identically). CI runs on Linux with LF. Frontend: `npm run lint` clean · `npx tsc -b --noEmit` clean · `npm test` **47 passed / 6 files** · `npm run build` clean. `go test -race` **remains unavailable** (`CGO_ENABLED=0`, carried since 3.8) — **no race coverage is claimed.**

**Hebrew centralization.** `.tsx` scan lists **exactly the two known pre-existing violations** (`features/live/control-page.tsx`, `features/builder/scoring-editor.tsx`). *Caught during the gate:* `reveal-stage.test.tsx` initially failed — a comment quoted `"12 ענו"` to explain why the counts-line assertion must read the whole composed line. Reworded; the file now carries zero Hebrew, and every expected string is read from `strings.he.ts`. The Go SQL comments are English (above), so `gen/answers.sql.go` is clean.

**Filter-safety.** Source and built bundle both scanned: zero external hosts, zero font `@import`, zero `url(//…)`.

**Built-CSS verification** (verify, do not assume). All 17 items present: `--stage-bar-track`, `--stage-bar-label`, `.stage-bar-grow`, `@keyframes stage-bar-grow`, `bg-success`, `bg-green-100`, `bg-green-600`, `rounded-full`, `ms-auto`, `whitespace-nowrap`, `break-words`, `shrink-0`, `min-w-0`, `gap-6`, plus the three arbitrary-value utilities that a naive literal grep reports as MISSING because Tailwind **escapes them in the selector** — `.basis-\[44\%\]{flex-basis:44%}`, `.basis-\[var\(--stage-bar-label\)\]`, `.h-\[var\(--stage-bar-track\)\]`. **No fallback to inline `style` was needed.** (Recording the escaping explicitly: it is a false-fail waiting for 4.5/4.6.)

**Red-first demonstrations — four, two per side.**

- *Frontend, denominator swapped to `answeredCount`* → `computes percentages off the sum of the bars, not off answeredCount` FAILED: `Unable to find an element with the text: 2 · 20%`. Restored, green.
- *Frontend, `?? 0` dropped from `optionCounts[i]`* → `fills the missing bars with 0 when optionCounts is shorter than options` FAILED: `Unable to find an element with the text: 4 · 40%`. Restored, green.
- *Backend, the whole state gate removed* (`if g.State == StateRevealed` → `if true`) → `TestSnapshotBeforeRevealCarriesNoRevealAndReadsNothing` FAILED on **both** subtests and on **both** assertions: `Reveal = &{CorrectOption:0 … OptionCounts:[0 7 0 0] CorrectCount:7}, want nil before the reveal` and `reveal reads ran before the reveal: distribution=1 correct=1, want 0/0`. This is the security property; the call-counter half is what proves no round trip and no log line. Restored, green.
- *Backend, before `buildQuestionReveal` existed*, the four reveal cases failed as written (`Reveal = nil at revealed, want the payload`; both error-propagation subtests `err = nil, want a propagated error`).

**Two test-harness facts worth carrying forward.** (1) `getByText` joins only an element's **direct text-node children**, so a composed line like `{answeredStat} · {correctStat}` matches on the whole string and on neither half — and asserting the half `"12 ענו"` **passed vacuously** against the hero band's own answered count, which renders the same number one element up. Assert the composed line. (2) RTL's `cleanup()` empties the container, so any measurement taken from a render must be **captured before** cleanup; the reduced-motion width comparison initially compared `[]` against the real widths and failed for that reason, not for a product reason.

**Local Go E2E — `server/cmd/e2escratch`, created, run 4×, deleted.** Port **8099**, `WHATSAPP_API_BASE_URL` and `ANTHROPIC_API_BASE_URL` at dead local addresses. Log confirms exactly **1 `server listening`, 0 `bind:` errors, 2 base-URL overrides, 0 `graph.facebook.com`, 0 `api.anthropic.com`** — the 3.10 "which process answered?" trap closed by construction, and nothing left the machine. Unique `wamid` per message. **17 assertions, 0 failures, on each of 3 clean runs.**

*The first run failed 3 assertions and the cause was the harness, not the product* — worth recording because it looks exactly like a product bug: every count came back 0 because the harness sent `phone_number_id: "PNID"` instead of the configured id, and `webhook.go` drops a foreign change **silently with a 200** (`"webhook change for another phone number, dropped"`). A harness that gets this wrong reports a completely dead reveal payload.

- *Scenario A — nothing leaks before the reveal.* On a live `role=display` socket, the `question_open` and `question_closed` frames both carry `currentQuestion.reveal === null`, the raw JSON contains the explicit `"reveal":null`, and **no `correctOption`, `acceptedAnswer`, `optionCounts` or accepted-answer string appears anywhere in either frame** (2489 / 2491 bytes scanned whole). 6/6.
- *Scenario B — the MCQ payload.* 12 phones, intended split `[3 4 2 3]` with the correct option (2) deliberately **not** the largest. **Measured split `[3 4 2 3]`, element for element.** `correctOption = 2`; `sum(optionCounts) = 12 = answeredCount`; `correctCount = 4` = the number sent to option 2; `acceptedAnswer` empty. 5/5, identical across all three runs.
- *Scenario C — free-text.* Two accepted answers, 3 matching and 2 not. `acceptedAnswer = "PRIMARY-FORM"` (the **first**), `optionCounts` **absent from the JSON** entirely, `correctCount = 3` against `answeredCount = 5` — so the number is not trivially the total. 3/3.
- *Scenario D — one payload, both roles.* A second `role=host` socket received a **byte-identical** `reveal` object at `revealed` (`{"correctOption":2,"acceptedAnswer":"","optionCounts":[3,4,2,3],"correctCount":4}`) and `"reveal":null` before it. The gate is the state, not the role — verified, not assumed. 3/3.
- Scratch rows cleaned (13 games + the scratch organizer, cascaded; 0 scratch participants remain), harness deleted.

**Manual browser pass — 77 checks, all green, in real Edge** (`playwright-core` + `channel: 'msedge'`, installed into the scratchpad, never into `web/package.json`; driver deleted afterwards). Driven against the **real production bundle** embedded into the server binary, not a Vite dev server, so every measurement is of the built CSS. Seeded through the real API and real HMAC-signed webhooks. Three phases; phase 3 uses a bash orchestrator to kill and restart the Go server, because Node's `child_process.spawn` cannot launch anything under this machine's Hebrew home directory.

*Phase 1 — 35/35, at 1920×1080. Checks 1, 2, 4, 5, 7(a), 9, 11, 12, 13.*

- **Check 1 / AC-1.** Correct pill `rgb(21, 128, 61)` (success); the other three `rgb(209, 250, 229)` (green-100) with `rgb(20, 83, 45)` text at **opacity 1** — dim the fill, never the text. Radius **8px** on all four. The ✓ measured at **x=1093** against the letter at **x=1817**: inline-end in RTL, i.e. leftmost, achieved with `ms-auto` and no physical margin. The glyph is `aria-hidden` with an `sr-only` label beside it. Four bars; correct fill success, others `rgb(22, 163, 74)` (green-600); the correct bar's label carries a ✓; **labels sit outside the fills** (`label.right=298` vs `track.x=322` on every row); track **24.0px** tall at pill radius.
- **Check 2 / RTL direction, measured not eyeballed.** Every fill's `right` equals its track's `right` to within 0.05px (`1045.5` vs `1045.5`, four for four) — anchored to the inline-start edge, growing leftward. A fill anchored left would look plausible in a screenshot and be exactly backwards.
- **Check 4 / derived req. 7 — the question does not move.** `<h1>` `boundingBox` at `question_closed` and at `revealed`: **`{x:48, y:444.84375, w:1824, h:88.3125}` both times — identical to the pixel.**
- **Check 5 / derived req. 6 — the ring does not refill.** Numeral **0**; stroke `rgba(255, 255, 255, 0.9)` at **8px** (white, not gold, not thickened); `stroke-dashoffset` **`-666.018px` at `question_closed` and `-666.018px` at `revealed` — unchanged**, i.e. the ring is empty and the room sees no change at the moment of reveal; `stage-timer-sweep` absent; **zero gold elements on the page** (scanned `color`/`background`/`stroke`/`fill` on every node for `rgb(251, 191, 36)`).
- **Check 7 / projection scale.** Question **64.0013px**, option text **40.0013px**, bar labels **48px**, band **378px** (exactly 35%), rows **96px**, track **24px**. Widths: pill **802.5px** / label **249.6px** / track **723.9px** against the predicted 802.6 / 249.6 / 723.8. No horizontal overflow.
- **Check 9 / motion.** `stage-bar-grow` on all four fills, `animation-duration: 0.4s`, `animation-name: stage-bar-grow`, `animation-fill-mode: both`. **`transform-origin` x resolves to 542.891px against a fill width of 542.891px** — the origin *is* the anchored edge, which is what `100% 50%` has to mean here. Fills settled at their inline widths with no overshoot.
- **Checks 11–13.** Zero participant names and zero phone numbers in the DOM, asserted against seeded participants with distinctive names. **Zero requests to any external host, zero `fetch`/`XHR` on the display route, zero console errors.**

*One honest note on phase 1's data.* The intended MCQ split was `[3 4 2 3]`, but the **measured** split was `[3 1 0 0]`: the driver joined 8 extra phones *after* `start`, and story 2.5 correctly made them **Spectators**, who cannot answer. That is correct product behaviour and a driver-ordering mistake, corrected in phases 2–3 (all joins before `start`). It does not weaken phase 1, whose 35 checks are about layout, colour, geometry and direction — the arithmetic is the E2E's job and it measured the full split exactly.

*Phase 2 — 25/25. Checks 3, 6, 7(b), 8.*

- **Check 3 / AC-2.** Card `rgb(21, 128, 61)` at **12px** radius with white text; the accepted answer is the **first** form (`PRIMARY-FORM-ANSWER`) at **64.0013px**; ✓ at the inline-end (`x=72` vs the answer at `x=1018`); counts line reads **`"6 ענו · 4 צדקו"`** — right order — in `rgb(75, 85, 99)` at **48px**; **zero bars and zero option pills**, replaced not appended.
- **Check 6 / the label fit, the one number this story was least sure of.** Constructed the real worst case: 80 phones all on one option, so the correct row reads **`"✓ 80 · 100%"`**. **It fits at both resolutions** — `scrollWidth 256 / clientWidth 256` at 1920×1080 and `173 / 173` at 1280×720, with no page overflow either time. **`--stage-bar-label: 13vw` needs no change.** The story predicted it "may well overflow"; measured, it does not.
- **Check 7b / the fit budget, two worst cases × two resolutions.** *Re-measured properly in phase 3* — phase 2's first metric took the max over all elements and so caught the **absolute full-bleed wrapper**, which is viewport-height by construction and proves nothing. The honest measurement is the last row's bottom against the body container's content box: **(a) one-line question** — 1920: h1 88px/1 line, rows 96/96/96/96, last row ends **981.2 of 1032**; 720p: h1 59px, rows 64×4, ends **670.8 of 688**. **(b) a question that wrapped to *three* lines with four long wrapping options** (harsher than the story's predicted two-line case) — 1920: h1 265px/3 lines, **rows shrank 96 → 77.3**, ends **1032.0 of 1032**; 720p: h1 177px/3 lines, **rows shrank 64 → 43.2**, ends **688.0 of 688**. **Four rows present in every case; the rows shrink rather than clip**, exactly as 4.3 measured, and it still holds now that each row has three children.
- **Check 8 / reduced motion, both channels separately.** Room-level toggle (a real `PUT /display-settings`): `stage-bar-grow` **absent from every fill**, `animation-name: none`, `data-reduced-motion="true"`, bars still at their final widths (`25% 38% 13% 25%` → 181/275/94/181px), and the ✓, the success fill and the dimmed fills **all unchanged at opacity 1**. OS-level `prefers-reduced-motion: reduce` **alone**, on a separate context with the room toggle back off: **identical result**. The accessibility cues are independent of the animation.

*Phase 3 — 17/17. Check 10, plus the honest re-measure of 7b above.*

- **Check 10 / reconnect at `revealed`.** The Go server was **really killed** (orchestrator + `Stop-Process`, not `ctx.setOffline`, which does not close an established socket and would pass vacuously) and **`ws.on('close')` was asserted `true` before anything else was trusted**. The stage **held its marks and bars unchanged** — identical pill fills and identical inline widths before, during and after — the band appeared over it reading `מתחבר מחדש…`, and after restart it **re-rendered with no interaction** and the band disappeared.
- **Band geometry, measured and fed to Task 11:** band **48–124px** (height 76px), timer numeral **141–237px** (**17px clear**), question `<h1>` **445–533px** (321px clear). **Reproduces 4.3's figures to the pixel** on a structurally different body, so it is a genuine second data point. Neither must-fix element is covered → recorded and re-deferred per 4.3's own rule, not escalated.

**Stories 3.10 / 4.1 / 4.2 manual passes.** Not attempted here. This pass was scoped to the reveal stage and to the shell behaviour it exercises (the reconnect band, the assertive announcer, the `revealed` map entry); it did not put me in front of 3.10's dashboard summary or 4.2's lobby stage, so claiming them would be false. They remain outstanding.

**One self-inflicted incident, recorded because the fix is worth knowing.** Marking the task checkboxes with a PowerShell `Get-Content -Raw` / `Set-Content -Encoding utf8` round-trip **corrupted every non-ASCII character in this story file** — PS 5.1's `Get-Content` decodes with the ANSI codepage (CP1255 here), so the UTF-8 bytes were read as Hebrew mojibake and rewritten as UTF-8 (`—` → `ג€”`). It was fully reversible at the byte level, because CP1255 maps every byte and the round-trip is lossless: read as UTF-8 → re-encode as CP1255 → those bytes *are* the original UTF-8. Verified lossless before writing (0 unmappable characters), then verified after (0 mojibake, BOM stripped, all 90 checkboxes intact). **Use the Edit tool, or `[System.IO.File]::ReadAllText($p, [Text.Encoding]::UTF8)`, for any bulk rewrite of a UTF-8 file on this machine.**

### Completion Notes List

- **All 12 tasks and every subtask are complete**; all three epic ACs and all nine derived requirements are implemented and verified. Every acceptance claim above is backed by a measurement recorded in the Debug Log.
- **The backend half is exactly the three things the story scoped**: a nested `QuestionReveal` on `CurrentQuestion` populated only at `g.State == StateRevealed`, two reveal-only reads, and one branch in `buildSnapshot`. `CountAnswersByQuestion` is untouched, no migration, no change to the reveal transition, its broadcast or its WhatsApp burst. The untouched-file check against `3998281` is empty for `migrations/`, `ws/`, `wa/`, `httpapi/`, `grading/`, `use-game-socket.ts`, `timer-ring.tsx`, `lobby-stage.*`, `stage-placeholder.*`, `stage-props.ts`, `app.tsx`, `components/`, `package.json`, `vite.config.ts` and `.github/`.
- **The extraction gate passed exactly as specified**: `git diff 3998281 -- web/src/features/display/{lobby-stage,question-stage,timer-ring,display-page}.test.tsx` is **empty**, and all **34** of those cases pass. `StageHeroBand` and `StageOption` are therefore behaviour-preserving. ~~Every comment in the extracted code was carried across unchanged.~~ **Corrected at code review (2026-08-12): one comment was dropped** — the pill's `// rounded-sm (8px) — "options are choices, not tags", never rounded-full.` did not survive into `stage-option.tsx`, and Task 7 names the radius half explicitly among what must move. Restored at review. Two further precisions on the same claim: "DOM-preserving" is true in *effect*, not literally — the `class` **attribute string** order differs from the baseline and the `className = ''` default leaves a trailing space; the class *set* is identical and Tailwind's cascade order comes from the stylesheet, so rendering is unaffected and the 34-case gate is what proves it.
- **One deviation from the story's letter, flagged for review**: `reveal` is optional-and-nullable in TS rather than non-optional, because non-optional and the extraction gate cannot both hold. Full reasoning in the Debug Log and in `types.ts`. **The Go wire contract is exactly as specified.**
- **Two comment texts the story supplies verbatim were reworded** because they contain Hebrew and would fail the Hebrew-centralization gates the same story mandates (`snapshot.go`'s `QuestionReveal.CorrectCount` comment, and the SQL comment on `CountCorrectAnswersByQuestion` which sqlc copies into scanned Go).
- ~~**Two `[ASSUMPTION]` items need Avraham's confirmation at review**~~ — **corrected at code review (2026-08-12): there is ONE, and it is now confirmed.** `display.reveal.correctOptionLabel` = `'התשובה הנכונה'` is the assumption (UX-DR14 requires the ✓ to be aria-labeled but gives no wording; gender-neutral per A2, states the fact rather than the glyph) and it carries the `[ASSUMPTION]` marker in `strings.he.ts`. `display.reveal.correctStat` = `` `${count} צדקו` `` is **not** an assumption and correctly carries no marker: EXPERIENCE.md gives the counts line "X ענו · Y צדקו" verbatim, so `צדקו` is specified rather than invented — it is only the decision *not* to reuse `results.correctColumn` (a dashboard table header) that is a judgement call, and that is a code comment, not an open question. **Avraham approved both strings as written on 2026-08-12; neither changes.**
- **`strings.live.answeredStat` now has a third caller**, as the story predicted. 4.3 flagged the cross-block import; the free-text counts line adds one more. If the answer is "duplicate them" or "promote to a shared block", it is now a **three-site** change.
- **The option letter's `min-width` (`2.2cqw` in the mockup) is still missing, and is now a two-caller deviation.** Adding it would have changed the question stage's rendering and broken the extraction gate, so Task 7 deliberately declined it. Recorded here for whoever next touches these rows. Both stages measured clean at both resolutions.
- **`display-page.tsx`'s `answeredFloor` needed no reveal-specific handling, confirmed rather than assumed**: it is keyed on the question id, not on the state, and `current_question_position` does not move until `NextQuestion` — so it carries correctly from `question_closed` into `revealed`. The block is unchanged. Derived requirement 8 is precisely *why* the distribution uses its own denominator: the floor deliberately raises `answeredCount` above the frame's value.
- **Task 11 recorded eight outcomes in `deferred-work.md`**, each appended under the existing entry in the file's convention: the 3.8 reveal/next ordering entry **triggered and decided in writing** (re-deferred — the consequence worsened, the mechanism and the evidence did not; trigger re-pointed off story numbers to "the first report… or any change that shortens the reveal→next gap"); the 4.2 heading-semantics entry **triggered and partially addressed** (the reveal question is an `<h1>`; the answer card deliberately is **not** a heading, with reasons; the shell-level decision is still open and re-pointed to 4.6); the SQL-coverage family **triggered on its "someone who did not write the original query" clause**, re-deferred with the gap stated plainly (deleting `GROUP BY a.response` or inverting the `FILTER` leaves the whole Go suite green — only the now-deleted E2E ever ran the real SQL); **three written not-triggered confirmations** (2.4 out-of-order, with the reason *stronger* here than at 4.3 because the answer set is closed before the first revealed frame exists, and an explicit note that this stage adds **no** high-water guard and why; 2.5 spectator roster; 3.7 degraded empty leaderboard); the 4.1 reconnect-band entry **re-measured and re-deferred**; and a **note, not a new entry**, under 3.4's bank-import item recording that its remaining half is now visible on a projector.
- **Verification totals: 47 frontend tests / 6 files · 8 Go packages green (6 new reveal cases) · 17 E2E assertions × 3 clean runs · 77 real-browser checks.** Four guards demonstrated red before green.
- **`go test -race` is unavailable in this environment** (`CGO_ENABLED=0`, carried since 3.8) — no race coverage is claimed.
- **Left outstanding, deliberately and stated rather than quietly skipped**: the manual browser passes for stories 3.10, 4.1 and 4.2. The story asks for them "if outstanding, in the same session"; this session's browser work was scoped to the reveal stage and did not put me in front of those surfaces, so they are reported as still open rather than claimed.

### File List

**New:**

- `web/src/features/display/reveal-stage.tsx`
- `web/src/features/display/reveal-stage.test.tsx`
- `web/src/features/display/stage-hero-band.tsx`
- `web/src/features/display/stage-option.tsx`

**Modified:**

- `server/internal/game/snapshot.go`
- `server/internal/game/engine.go`
- `server/internal/game/engine_test.go`
- `server/internal/store/queries/answers.sql`
- `server/internal/store/answers.go`
- `server/internal/store/gen/answers.sql.go` (regenerated by `sqlc generate`)
- `web/src/features/display/question-stage.tsx`
- `web/src/features/display/display-page.tsx`
- `web/src/index.css`
- `web/src/lib/strings.he.ts`
- `web/src/lib/types.ts`
- `_bmad-output/implementation-artifacts/deferred-work.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/4-4-reveal-stage-the-answer-marked.md` (this file)

**Created and deleted during the run** (not in the diff): `server/cmd/e2escratch/main.go` (the Go E2E harness) and the scratchpad Playwright driver.

## Change Log

| Date | Change |
|---|---|
| 2026-08-11 | Story 4.4 created — reveal stage, MCQ distribution bars, free-text answer card, and the first Epic-4 backend change (a state-gated `reveal` payload on the snapshot plus two reveal-only reads). Baseline set to `3998281` (story 4.3 merged to `main`). |
| 2026-08-12 | Story 4.4 code review (three parallel adversarial layers). All three epic ACs and all nine derived requirements verified; every Task 12 gate independently re-run green; the flagged `reveal?:` TS deviation audited and upheld. 3 decisions resolved, **12 patches applied**, 4 items deferred, 8 dismissed. The substantive fixes: the answer-card branch now guards on there being an answer to show rather than on a reveal existing (it rendered an empty green card asserting "התשובה הנכונה"); the dimming shares one validity test with the mark (a reveal identifying no option dimmed all four and marked none); largest-remainder apportionment so the four percentages total exactly 100 and a nonzero bar is never invisible; `<bdi>` on the accepted answer; `min-h-0` on the free-text column; a reserved ✓ slot so the four figures stay in one column; `--stage-bar-label` 13vw → **16vw** with `overflow-hidden`, measured in real Edge at both resolutions (720p was the binding constraint); the dropped `rounded-full` comment restored onto `StageOption`; and Go tests that assert the **marshalled** reveal wire contract, which nothing did. Frontend tests 47 → **58**; nine guards demonstrated red first. |
| 2026-08-12 | Story 4.4 implemented. Backend: `QuestionReveal` on `CurrentQuestion`, populated only at `games.state = 'revealed'`; two reveal-only reads (`ListAnswerCountsByResponse`, `CountCorrectAnswersByQuestion`) with `sqlc` output regenerated; six new `engine_test.go` cases. Frontend: `reveal-stage.tsx` with MCQ distribution rows and the free-text answer card; `stage-hero-band.tsx` and `stage-option.tsx` extracted from `question-stage.tsx` as DOM-preserving refactors gated on an empty diff across the four existing display test files; two ramp values and one animation in `index.css`; a `display.reveal` copy block; `revealed` wired into `stageByState`. Verified by 47 frontend tests, 8 green Go packages, a 17-assertion local E2E (3 clean runs) and a 77-check real-browser pass. Eight outcomes recorded in `deferred-work.md`. One flagged deviation: `CurrentQuestion.reveal` is optional-and-nullable in TS rather than non-optional, because Task 4's non-optional form and derived requirement 12's empty-diff gate cannot both hold. |
