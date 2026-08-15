---
baseline_commit: 7168de8
---

# Story 4.6: Winner Takeover — מזל טוב!

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As the room,
we want a full-screen festive winner moment,
so that the game ends on its emotional peak (FR-18, UJ-2).

## Baseline

**At story-creation time (2026-08-14) story 4.5 is `done` and committed at `7168de8`, on branch `story/4-5-leaderboard-stage`, but that branch is not yet merged to `main`** (`origin/main` still sits at `bf0b93a`, the 4.4 merge). Resolve before writing any code, exactly as 4.5 resolved the identical situation for 4.4:

- **If 4.5 has merged to `main`** by the time you start — branch from `main`, and confirm `web/src/features/display/leaderboard-stage.tsx` and `web/src/features/display/stage-props.ts`'s `isDegradedFrame` export exist in the checkout.
- **If it has not** — branch from the tip of `story/4-5-leaderboard-stage` (`7168de8`). Do **not** branch from `bf0b93a`: `display-page.tsx`, `index.css`, `strings.he.ts` and `deferred-work.md` are all touched by both stories, and a `main`-based branch would collide on all four.
- Either way, **record the actual baseline SHA in the Dev Agent Record and correct this file's frontmatter.**

This story consumes 4.1–4.5's shapes. Verify each on disk before writing code:

- `web/src/features/display/display-page.tsx` — the shell. `stageByState.finished` is still `StagePlaceholder` (comment: `// story 4.6 — winner takeover`); `draft` is the only other placeholder left after this story. `retained`, `stageSnapshot`, `usePrefersReducedMotion`, the `${gameId}:${state}` key and the reconnect band are 4.1–4.5 review decisions — none of them change here.
- `web/src/features/display/stage-props.ts` — `StageProps { snapshot, reducedMotion, previousStandings? }` and the exported `isDegradedFrame(snapshot)` helper. **This story does NOT import or reuse `isDegradedFrame` — see derived requirement 2 for exactly why it would be wrong at `finished`.**
- `web/src/features/display/leaderboard-stage.tsx` — the closest sibling pattern: a full-bleed stage that reads `snapshot.leaderboard` verbatim (never re-sorts, never re-ranks), guards on a degraded frame first, and kills its one animation from the `reducedMotion` prop rather than a media query. This story repeats all three disciplines.
- `web/src/index.css` — `.stage-root` (unlayered) holds eleven `--stage-*` ramp values; `@layer components` holds `.stage-fade`, `.stage-timer-sweep`, `.stage-bar-grow`, `.stage-row-reshuffle`; four `@keyframes` sit outside the layer. Gold (`--color-gold`) is defined but, per the file's own header comment, "never applied in this story" — true of every story before this one.
- `web/src/lib/strings.he.ts` — `live`, `results` and `display` (with `lobby`, `question`, `reveal`, `leaderboard` sub-blocks).
- `server/internal/game/final.go` — `ResultsForFinishedGame`'s winner rule, which this story's frontend must mirror exactly (derived requirement 1). **Read this file before writing the filter** — do not reconstruct the rule from the epic text alone.
- `server/internal/game/engine.go` (`buildSnapshot`, `emptySnapshot`) and `server/internal/store/queries/games.sql` (`FinishGame`) — read both before writing the degraded-frame guard. Derived requirement 2 depends on exactly what they do to `current_question_position` and `QuestionCount` at `finished`, and copying leaderboard-stage's guard verbatim is the wrong answer here.
- **The existing display test files are a regression surface.** `git diff <baseline> -- web/src/features/display/{lobby-stage,question-stage,timer-ring,reveal-stage,leaderboard-stage,use-leaderboard-memory}.test.tsx` must be **empty**, and all their cases must still pass. `display-page.test.tsx` **will** grow one new case (the `finished` mount) — see Task 3.

## Acceptance Criteria

1. **Given** the Game finishes, **Then** the display takes over full-screen with the `winner-card`: green-800 background, winner name in gold Display weight, score below ("מזל טוב, משה כהן! 1,240 נקודות" — per the winner-card spec and templates table) — gold's second and final permitted appearance (UX-DR2, UX-DR8). *(epic AC-1)*
2. **Given** the celebration, **Then** it is CSS-animated geometric shapes in gold and green-600 only — no images, no GIFs, no external assets (UX-DR4), honoring `prefers-reduced-motion`. *(epic AC-2)*
3. **Given** a tie for first, **Then** all tied winners are named — up to three names stacked, score shown once (A16, consistent with Story 3.9), **and** game-state transitions announce via `aria-live="assertive"` (UX-DR14). *(epic AC-3)*

### Derived requirements — binding, and each has a source

4. **The winner rule is not this story's to invent — it already exists, server-side, and this story's only job is to read the same field the same way.** `game/final.go`'s `ResultsForFinishedGame` defines it precisely, in its own doc comment: *"every player whose Rank is 1 AND whose Score is strictly positive. The positivity condition is load-bearing, not defensive… without this condition, an aborted game would congratulate the entire room on winning with zero points."* That function feeds the WhatsApp winner message (story 3.9); it is **not** wired to the Snapshot and is **not** what this story calls. The display's only data source is `snapshot.leaderboard` (`game.RankLeaderboard`'s output, already on the wire, already sorted, already rank-assigned with standard-competition ties). This story's job is to **filter it client-side by the identical predicate** — `entry.rank === 1 && entry.score > 0` — never to sort, never to re-derive rank. `leaderboard-stage.tsx`'s own doc comment predicted this exact move: *"a second client-side ranking could silently disagree with the winner computation that reads the same entries."* Filtering by an already-assigned field is not that second ranking; inventing any other test (e.g. `entry === leaderboard[0]`, or a score comparison against other rows) would be.

5. **🚨 The `finished` degraded-frame discriminator is DIFFERENT from `leaderboard`'s, and reusing `isDegradedFrame` verbatim silently breaks this story.** This is the single most important thing to get right, so it is stated as plainly as possible:

   - `server/internal/store/queries/games.sql`'s `FinishGame` query sets `current_question_position = 0` **on every transition into `finished`**, deliberately — its own comment: *"the migration's own invariant is that 0 means 'no question open', mirroring draft/lobby/finished — leaving a stale position here would make buildSnapshot keep reporting a currentQuestion for a game that already ended."*
   - `engine.go`'s `buildSnapshot` only populates `current` (→ wire field `currentQuestion`) when `g.CurrentQuestionPosition > 0`. Position 0 at `finished` means **`currentQuestion` is `null` on every real, healthy `finished` frame** — not only on a degraded one.
   - `QuestionCount` (`len(questions)`), by contrast, is **never reset** — it is `ListQuestionsByGame`'s length regardless of state or position, so a genuinely finished game (which cannot have started without `startGameNoQuestions`' guard being satisfied — a game cannot start with zero questions) always carries `questionCount ≥ 1`.
   - `leaderboard-stage.tsx`'s guard is `isDegradedFrame(snapshot)`, defined in `stage-props.ts` as `questionCount === 0 || currentQuestion === null`. **Applying that OR at `finished` would be true on every single frame**, real or degraded alike, because the `currentQuestion === null` half is unconditionally true here. The winner stage would show `strings.display.waiting` forever and never render a winner, in every game, always — a defect a naive pattern-copy from the closest sibling file would introduce silently, with every existing test still green (nothing exercises `finished` yet).
   - **The correct guard at `finished` is `snapshot.questionCount === 0` alone.** `emptySnapshot` sets `QuestionCount: 0` (Go zero value, never populated) together with `Leaderboard: []{}` on the degraded path; a genuine `finished` frame never has `questionCount === 0` because the game could not have started without a question. Do **not** import or call `isDegradedFrame` from this file — write the one-line check locally, with a comment citing this derived requirement, so a future reader does not "simplify" it back to the shared helper.
   - This resolves `deferred-work.md`'s re-pointed 3.7 entry (*"story 4.6's winner takeover reads the same [`Snapshot.Leaderboard`] field and is the likely one"*) — Task 4 records the resolution and explicitly notes it is **not** the same discriminator as `leaderboard`'s.

6. **A healthy `finished` frame can still have zero winners, and that is a distinct, real case from the degraded one.** Two ways there: (a) the Organizer stops the game before any Reveal (`FinishGame`'s guard admits `question_open`), so every registered player sits at score 0 and `RankLeaderboard` assigns them all rank 1 — the positivity condition in derived req. 4 correctly excludes all of them; (b) nobody registered at all (nothing gates game start on a roster — `leaderboard-stage.tsx` already documents this as reachable), so `snapshot.leaderboard` is empty and the filter yields nothing regardless. Both collapse to the same UI fact — "no winner to announce" — and get **one** honest fallback message, not three-way copy: WhatsApp's own no-winner template (`messages_he.go`'s `msgFinalResultsNoWinner`, *"המשחק נגמר! הפעם לא נצברו נקודות — נתראה במשחק הבא!"*) already treats these identically for the same reason. This story's display copy should say the analogous thing for the same reason, not invent a three-way split leaderboard-stage's roster-vs-degraded distinction does not apply to here (this stage never renders a roster — only a winner or the absence of one).

7. **Score renders as raw digits, with no thousands separator — the mockup's comma is not the convention to follow.** `mockups/key-stage-winner.html` shows "1,240 נקודות", but **every existing source of this exact number disagrees**: `messages_he.go`'s `winnerFinalMessage`/`finalResultsMessage` format the score via bare `strconv.Itoa` (no separator), and `leaderboard-stage.tsx`'s own Dev Notes record the identical decision for the identical reason — *"Rendered raw, no `toLocaleString` and no thousands separator, matching `results-summary.tsx`'s `{entry.score}` so the dashboard table and the projector cannot disagree about the same number."* Follow the codebase, not the mockup's illustrative comma. (Flagged for Avraham per this project's [ASSUMPTION] convention, same as every other mockup-vs-code discrepancy this epic has recorded — but the derivation here is unambiguous, unlike a case with no precedent.)

8. **The winner name gets `<h1>` semantics, closing the shell's heading-semantics entry rather than deferring it a fourth time.** `deferred-work.md`'s running entry (opened at 4.2, touched at 4.3 for the question text, at 4.4 for the reveal's shared question `<h1>`, checked-not-triggered at 4.5) was last re-pointed explicitly to *"4.6's winner name (the last dominant text element Epic 4 will produce)… If 4.6 also decides stage-locally, the shell decision should be taken as its own item rather than deferred a fourth time."* Epic 4 has no story after this one to inherit a fifth deferral. Decide locally: the winner name is this stage's one dominant text element (exactly the reasoning 4.3 used for the question and 4.4 used for the reveal's shared question), so it renders as `<h1>`. Task 5 updates `deferred-work.md` to close the entry — the lobby join code and the leaderboard rows remaining plain elements is the accepted final shape, not an open item, because no further Epic 4 story will touch them.

9. **Ties cap at three names, stacked, with no overflow indicator — the same accepted-cut posture as 4.5's top-10 boundary tie.** DESIGN.md's `winner-card.name` frontmatter: *"ties: up to 3 names stacked, score shown once"*; EXPERIENCE.md repeats it (A16) and explicitly distinguishes the *display's* finite-space cap from WhatsApp's uncapped `joinNames` join, which lists every tied name with no limit. A fourth-plus simultaneous winner is reachable (a small pilot game's last question can produce a 4-way tie at rank 1) and is not addressed by either spec beyond "up to three" — render `winners.slice(0, 3)` with no "+N more" affordance, matching 4.5's own accepted-cost precedent for its top-10 tie cut rather than inventing new overflow UI for a case neither DESIGN.md nor EXPERIENCE.md resolves.

10. **The confetti is transcribed from the mockup's exact 22-piece layout, converted from `cqw` to `vw`, and killed from the `reducedMotion` PROP — never a media query.** `mockups/key-stage-winner.html` is pixel-authoritative here (unlike 4.5, which had no mockup at all) — its 22 `<div class="cf …">` elements, each with a fixed `left`/`top` percentage and an `animation-duration`/`animation-delay` pair, are the full specification; port them verbatim rather than generating positions programmatically, so the result matches what was actually reviewed. The mockup's own CSS uses `@media (prefers-reduced-motion: reduce) { .cf { animation: none } }` — that is **the wrong mechanism for this codebase** and must not be copied literally: every prior stage (4.3's ring sweep, 4.4's bar grow, 4.5's row reshuffle) kills its one animation from the `reducedMotion` **prop** instead, for the two reasons those stories already recorded — a CSS-only kill is invisible to jsdom (so AC-2's "honoring `prefers-reduced-motion`" clause could not be asserted by a unit test), and the Organizer's room-level "הפחת אנימציות" toggle is not a media query at all. Apply the class conditionally in JSX exactly as `leaderboard-stage.tsx`'s `animated && shift !== 0 ? 'stage-row-reshuffle' : ''` does. Under reduced motion the pieces render at their bare `left`/`top` coordinates with no transform — the mockup's own "static celebratory frame" — which is honest simply because the animation is a translate, not a reveal.

11. **The `cqw` unit itself needs no real conversion, only a rename.** The mockup's own comment states its scaling assumption: *"container-query units — 1cqw = 19.2px at the 1920px design width"* — i.e. numerically identical to `1vw` at this project's design width, which every existing ramp value in `index.css` already assumes (`--stage-display: 5vw` = 96px at 1920, etc., with no `container-type` ever declared anywhere in this codebase). Rewrite every `cqw` in the mockup's CSS as `vw` unchanged in magnitude; do not recompute anything.

12. **One new ramp value is needed — the winner name is the single largest text this product ever renders, larger than any existing step.** The mockup's `.w-name{font-size:6.5cqw}` is ≈125px at 1920, above even `--stage-display` (96px, the timer numeral's size) — DESIGN.md's own note calls it "projection-large", a size reserved for this one moment. Add `--stage-winner-name: min(6.5vw, 11.5741vh);` to `.stage-root`, following the established equal-at-16:9 fit-guard shape every other large ramp step already uses (`--stage-lobby-code`, `--stage-timer-ring`, `--stage-leaderboard-row`): 11.5741vh = 124.8px at 1080p, exactly equal to 6.5vw at 1920p, so the guard only ever engages below 16:9. The tagline ("מזל טוב!") and the score line need **no new ramp value** — they land, by direct computation, on the two ramp steps that already exist: 2.0833cqw ≈ 40px = the existing `--stage-body` (2.0834vw = 40px exactly), and 2.5cqw = 48px = the existing `--stage-ui` (2.5vw = 48px exactly). Reuse both; do not invent new custom properties for either.

13. **Gold's palette-scan gate inverts at this one stage, and that inversion must be asserted, not just permitted.** Every prior stage's browser pass (4.3–4.5) asserts **zero** gold anywhere on screen — this stage is the one place in the entire product where that assertion is supposed to fail. The corresponding check here is the opposite shape: gold **must** appear, and **only** on the winner name's text color and the confetti pieces carrying `cf-gold`-equivalent styling — the tagline, the score line, and the card's own background must stay off gold entirely (DESIGN.md: tagline and score are `ink-on-dark-muted`; background is `green-800`). Task 7's browser check states both halves explicitly, because a browser check that only confirms gold's presence would pass even if gold leaked onto the score line by mistake.

14. **The winner takeover is the one stage epic 4 gates on `snapshot.leaderboard` alone, with no per-participant privacy fence to build — the winner IS the content, not a leak.** Unlike `reveal-stage.tsx` (epic AC-3 forbids per-participant data entirely) and `leaderboard-stage.tsx` (names and scores are content, grades and phone numbers are not), this stage's entire purpose is to publish one or more names and one score — nothing here reads `snapshot.participants`, and nothing needs to. No new privacy assertion is required beyond what `snapshot.leaderboard` already excludes (no phone number, no per-question grade — `LeaderboardEntry` never carried either).

## Scope boundaries for this story

- **No backend changes of any kind.** `snapshot.leaderboard` already carries `participantId`, `displayName`, `score` and `rank` — everything this story needs. **A diff anywhere under `server/` means you went off-spec.** In particular: no new `Snapshot` field, no touch to `final.go`, `engine.go`, `snapshot.go`, `games.sql`, or any `httpapi`/`ws` file. This story is read-only against a wire contract Epic 3/3.9 and Epic 4/4.1 already finished.
- **No new hook module.** Unlike 4.5's `use-leaderboard-memory.ts`, this stage needs no cross-stage memory — the winner set is derived fresh from the current snapshot on every render, with nothing to remember across a stage remount. `stage-props.ts`'s `previousStandings` field is untouched and unread by this stage.
- **One entry in `stageByState` changes: `finished`.** `draft` keeps `StagePlaceholder` — it is the only placeholder left after this story, and no story replaces it (a display opened before the lobby exists has nothing to show).
- **`web/src/features/display/stage-props.ts` is not modified.** No new field on `StageProps`; `isDegradedFrame` is read (to understand why NOT to use it — derived req. 5) but not imported into the new stage file.
- **No change to `web/src/features/live/control-page.tsx` or any other dashboard file.** The `finished` branch, `ResultsSummary` and the game-over screen are 3.9/3.10's and are complete; this story is the Audience Display's half of FR-18 only, exactly as the epic's implementation note says: *"The final WhatsApp winner message (FR-18's messaging half) is already delivered by Epic 3's FR-6 results story."*
- **No gold anywhere except the winner name text and the confetti's gold-colored pieces.** UX-DR2 rations gold to exactly two moments total across the whole product — the timer's ≤5s ring and this one. Nothing else on this stage, and nothing on any other stage, may use it.
- **No current-participant highlighting beyond naming the winner(s)** — there is no "current user" concept on a shared projector (EXPERIENCE.md, repeated at every stage).
- **No new npm package, no `shadcn add`, no `components/ui/*` edit.** The confetti is pure CSS/DOM, exactly as the mockup builds it.
- **Do not touch `leaderboard-stage.tsx`, `use-leaderboard-memory.ts`, or `stage-props.ts`'s `isDegradedFrame`.** This story reads their patterns for reference; it does not modify or extend them, per derived requirement 5's explicit "write it locally" instruction.

## Tasks / Subtasks

- [x] **Task 1: `web/src/lib/strings.he.ts` — the winner copy** (AC: 1, 3; derived reqs. 6, 7)

  - [x] Add a `winner` sub-block inside the existing `display` block, after `leaderboard`:
    ```ts
    // Winner takeover (story 4.6). Mirrors messages_he.go's Winner's-final-
    // message tone ("מזל טוב, [שם]! 🏆 ניצחת עם [ניקוד] נקודות!") without
    // repeating its exact wording — the projector composes name and score as
    // separate visual elements (mockups/key-stage-winner.html), not one
    // interpolated sentence.
    winner: {
      // mockups/key-stage-winner.html's w-tagline, verbatim. Gender/number
      // neutral, so it needs no tie/no-tie variant.
      tagline: 'מזל טוב!',
      // [ASSUMPTION]: no score-suffix string exists yet for the display
      // surface (messages_he.go's templates interpolate "X נקודות" inline
      // in Go, not as a reusable fragment). One function so the digit-plus-
      // suffix cannot drift from results.playerCountLabel's spelled-out-
      // singular discipline elsewhere in this file — though a score has no
      // natural "one point" singular concern the way a participant count
      // does, so this is a plain suffix, not a pluralizing function.
      scoreSuffix: (score: number) => `${score} נקודות`,
      // Distinct from display.leaderboard.noPlayers on purpose: that one
      // describes an EMPTY roster; this one describes a roster where
      // nobody's score cleared game.final.go's strict positivity condition
      // (a stopped-before-first-reveal game, or genuinely nobody registered).
      // Both collapse to "no winner to announce" and get one honest
      // sentence, mirroring messages_he.go's msgFinalResultsNoWinner rather
      // than inventing new wording for a case WhatsApp already names.
      noWinner: 'המשחק נגמר! הפעם לא נצברו נקודות.',
    },
    ```
  - [x] **Nothing else in `strings.he.ts` changes.** `display.stateAnnouncement.finished` already exists (`'המשחק הסתיים'`) and the shell already announces it assertively on entry (AC-3's second clause) — do not add a second announcement string.
  - [x] Flag `winner.scoreSuffix` and `winner.noWinner` as `[ASSUMPTION]` in the Dev Agent Record for Avraham's confirm-at-review, the same treatment every prior stage's new copy got. `winner.tagline` is a direct mockup transcription and is not an assumption.
  - [x] **Zero Hebrew literals outside this file**, `.tsx` and `.ts` alike (the 4.5 review found a `.ts`-file violation that a `.tsx`-only CI scan missed — check both).

- [x] **Task 2: `web/src/index.css` — one ramp value, the confetti animation** (AC: 1, 2; derived reqs. 10, 11, 12)

  - [x] Append **inside the existing `.stage-root` block**, after `--stage-leaderboard-row`:
    ```css
      /* DESIGN.md components.winner-card.name: "{typography.display},
         projection-large" — larger than --stage-display (96px, the timer
         numeral), which is the biggest text ANY other stage renders.
         mockups/key-stage-winner.html's .w-name is 6.5cqw ≈ 125px at 1920;
         cqw needs only a unit rename to vw at this project's 1920 design
         width (the mockup's own comment: "1cqw = 19.2px at the 1920px
         design width" — no container-type is declared anywhere in this
         codebase, so vw is the direct equivalent). 11.5741vh = 124.8px at
         1080p, equal to 6.5vw at 1920x1080 - the same fit-guard shape every
         other large ramp step in this file already uses. */
      --stage-winner-name: min(6.5vw, 11.5741vh);
    ```
    Note the tagline and score need **no** new custom property — they land on the existing `--stage-body` (40px) and `--stage-ui` (48px) steps exactly (derived req. 12).
  - [x] Append **inside the existing `@layer components` block**, after `.stage-row-reshuffle`:
    ```css
      /* The winner-card celebration (DESIGN.md components.winner-card.
         celebration: "CSS geometric confetti, gold + green-600, no images").
         Transcribed from mockups/key-stage-winner.html's .cf/@keyframes
         drift, cqw renamed to vw (derived requirement 11 - no scaling
         change, a straight rename).

         Deliberately NO prefers-reduced-motion rule here, unlike the
         mockup's own @media block: every animated stage before this one
         (4.3's ring sweep, 4.4's bar grow, 4.5's row reshuffle) kills its
         animation from the reducedMotion PROP instead, for the same two
         reasons recorded at each of them - a CSS-only kill is invisible to
         jsdom, and the Organizer's room-level toggle is not a media query
         at all. The class below is applied conditionally in JSX exactly as
         .stage-row-reshuffle is. */
      .stage-confetti-piece {
        position: absolute;
        animation-name: stage-confetti-drift;
        animation-timing-function: linear;
        animation-iteration-count: infinite;
        /* duration and delay are per-piece (mockup's own scattered start
           offsets, via a negative delay, are what makes the frame read as
           already in motion / already scattered under reduced motion) and
           are set inline, the same shape --stage-row-shift uses for
           per-row data. */
      }
    ```
  - [x] Append after `@keyframes stage-row-reshuffle`, outside the layer:
    ```css
    /* mockups/key-stage-winner.html's @keyframes drift, renamed. from/to
       only - both ends are needed here (unlike the from-only keyframes
       above) because this is an infinite loop with no implicit settle
       state to fall back to. */
    @keyframes stage-confetti-drift {
      from { transform: translateY(-60vw) rotate(0deg); }
      to   { transform: translateY(60vw) rotate(300deg); }
    }
    ```
  - [x] **Nothing else in `index.css` changes.** Not the `@theme` blocks (`--color-gold` already exists), not `.stage-fade`/`.stage-timer-sweep`/`.stage-bar-grow`/`.stage-row-reshuffle`, not their three existing `@keyframes`, not any other ramp value.
  - [x] **No new `@theme` color token.** `green-800`, `green-600`, `gold`, `ink-on-dark-muted` all already exist. This is the first story to use `bg-green-800` as a full-bleed stage background (previously only a hero-band accent) and the first to use `text-gold`/`bg-gold` as Tailwind utility classes at all (`timer-ring.tsx` applies gold via an inline SVG `stroke` attribute, not a utility class, for a documented and unrelated reason — read its comment before assuming the same approach applies here; a plain-DOM text color has no such constraint, and `text-gold` is the direct, idiomatic form). Verify both resolve in the built CSS per Task 7.

- [x] **Task 3: `web/src/features/display/display-page.tsx` — one map entry** (AC: 1)

  - [x] `finished: WinnerStage, // story 4.6 — winner takeover` and the import. `draft` keeps its placeholder comment unchanged — it is the last one standing.
  - [x] **Nothing else in this file changes.** No new hook call (unlike 4.5's `useLeaderboardMemory`), no new prop passed to `<Stage>` — `WinnerStage` reads only `snapshot` and `reducedMotion`, exactly like `reveal-stage.tsx`.
  - [x] `display-page.test.tsx` **gains** one case (it is not frozen — see Baseline): the winner stage mounts for `finished` and not for any other state. The other display test files (`lobby-stage`, `question-stage`, `timer-ring`, `reveal-stage`, `leaderboard-stage`, `use-leaderboard-memory`) must all still pass **unedited**.

- [x] **Task 4: `web/src/features/display/winner-stage.tsx` (new) — the takeover** (AC: 1, 2, 3; derived reqs. 4, 5, 6, 7, 8, 9, 10, 13, 14)

  The architecture names this exact file: `winner-stage.tsx`.

  - [x] **Signature**: `export function WinnerStage({ snapshot, reducedMotion }: StageProps)`, `import type { StageProps } from './stage-props'`. **Do not destructure or import `previousStandings`** — this stage does not read it (unlike `leaderboard-stage.tsx`, it consumes no cross-stage memory).
  - [x] **The degraded guard, first thing in the body, above everything — and it is `questionCount === 0` ALONE, not `isDegradedFrame`** (derived req. 5):
    ```tsx
    // NOT stage-props.ts's isDegradedFrame(): that helper's second disjunct
    // (currentQuestion === null) is unconditionally TRUE at `finished` on
    // every real frame, because FinishGame resets current_question_position
    // to 0 (games.sql, deliberately - see this story's derived requirement
    // 5). Reusing the shared helper here would show the waiting copy on
    // every real finished game and never render a winner. QuestionCount is
    // never reset, so it alone distinguishes emptySnapshot's degraded
    // fallback (QuestionCount: 0) from a real finished frame (>= 1 - no
    // game can start without at least one question).
    if (snapshot.questionCount === 0) { …waiting… }
    ```
    rendering `strings.display.waiting` in the same `stageMessageClass`-equivalent treatment (`text-[length:var(--stage-heading)] font-heading …`) every other stage's degraded guard uses — but note this stage's own body is `green-800`, so the text color must be `ink-on-dark`/`ink-on-dark-muted`, not `text-text-secondary` (which is the light-surface color every prior degraded guard used against a light or `surface-base` ground).
  - [x] **The winner filter** (derived req. 4): `const winners = snapshot.leaderboard.filter((e) => e.rank === 1 && e.score > 0)`. Never sort, never slice-then-filter — filter the whole array, matching `final.go`'s rule field-for-field. `const score = winners[0]?.score` (all tied entries share one score by construction).
  - [x] **The no-winner case** (derived req. 6): `winners.length === 0` on a frame that passed the guard above → full-bleed green-800 body with `strings.display.winner.noWinner`, in `ink-on-dark` — no confetti, no name, no score. This is a **healthy** frame (unlike the guard above), so it gets the celebration surface's own background, not the generic waiting treatment.
  - [x] **The surface** (matching `winner-card.background` and the mockup's `.winner` class): full-bleed, centered, green-800.
    ```tsx
    <div className="absolute inset-0 flex flex-col items-center justify-center overflow-hidden bg-green-800 p-[var(--stage-margin)] text-center">
    ```
  - [x] **The confetti**, `aria-hidden="true"`, rendered before the content so it sits behind it in paint order (mockup: confetti div precedes `.w-content`):
    - [x] Transcribe the mockup's 22 pieces verbatim as a `const confettiPieces` array of `{ left, top, duration, delay, color: 'gold' | 'green', shape: 'circle' | 'bar' | 'square', size: 'sm' | 'md' | 'lg' }` (bars/squares have no size variant in the mockup beyond their fixed class — keep that shape). Position (`left`/`top`) is already percentage-based in the mockup and needs no conversion; only the size classes (`cf-sm`/`cf-md`/`cf-lg`/`cf-bar`/`cf-sq`) use `cqw`, renamed to `vw` per derived requirement 11 — add them as Tailwind arbitrary-value utilities (`w-[.7vw] h-[.7vw]` etc.) or a small set of local classes; either is acceptable, but the values must be whole literals per Tailwind v4's source-text scanning (leaderboard-stage's Dev Notes: a template string generates nothing).
    - [x] Each piece: `style={{ left: piece.left, top: piece.top, animationDuration: piece.duration, animationDelay: piece.delay }}` (inline, per-piece — the same shape `--stage-row-shift` uses) and `className` composed from a static literal per color/shape/size combination plus, conditionally, `stage-confetti-piece` when `!reducedMotion` (derived req. 10 — never a media query). Under reduced motion the class is absent and every piece renders at its bare position with no transform: the mockup's own "static celebratory frame".
  - [x] **The content**, centered above the confetti:
    ```tsx
    <div className="relative z-10">
      <p className="text-[length:var(--stage-body)] font-ui text-ink-on-dark-muted">
        {strings.display.winner.tagline}
      </p>
      {/* <h1>, closing the shell's heading-semantics entry (derived
          requirement 8) - the winner name is this stage's one dominant
          text element, the same reasoning 4.3 applied to the question and
          4.4 to the reveal's shared question <h1>. font-heading is used
          for the WEIGHT token name only per this @theme's four-role
          system; the visual weight at this size is carried by
          --stage-winner-name's own scale, matching how stage-option.tsx
          and leaderboard-stage.tsx already record their own weight-token
          deviations rather than inventing a fifth role. */}
      <h1 className="text-[length:var(--stage-winner-name)] font-display text-gold">
        {winners.slice(0, 3).map((w) => (
          // Real identity, matching leaderboard-stage's precedent - a
          // winner's own participantId, not the array index.
          <div key={w.participantId}>{w.displayName}</div>
        ))}
      </h1>
      <p className="text-[length:var(--stage-ui)] font-ui text-ink-on-dark-muted tabular-nums">
        <bdi dir="ltr">{strings.display.winner.scoreSuffix(score)}</bdi>
      </p>
    </div>
    ```
    - [x] **Score shown once**, per A16 and DESIGN.md, regardless of how many names are stacked above it — it is outside the name loop, not repeated per winner.
    - [x] `<bdi dir="ltr">` wraps the whole score line (which is a Hebrew sentence with an embedded digit run), following the project's bidi-isolation discipline for any all-digit token inside RTL text — or, if `scoreSuffix`'s Hebrew suffix breaks a single `<bdi>` around the composed string, isolate only the digits as every other stage's score/count rendering does (`leaderboard-stage.tsx`'s `<bdi dir="ltr">{entry.score}</bdi>`) and compose the Hebrew suffix outside it. Verify the DOM order reads correctly in the browser pass (Task 7).
    - [x] Names render as **plain stacked block elements**, one per line (`<div>`, not a joined/comma sentence) — "stacked" per DESIGN.md is a layout instruction, not a text-join instruction; `messages_he.go`'s `joinNames` (used for the WhatsApp tie sentence) is not the pattern to copy here.
  - [x] **AC-3's aria-live clause is already satisfied and needs no new code**: `display-page.tsx`'s existing assertive announcer fires `strings.display.stateAnnouncement.finished` on entry to this stage, exactly as it does for every other state. Confirm this in the Dev Agent Record rather than adding a second announcer (leaderboard-stage's precedent: a second announcer would fight the first).
  - [x] **Zero Hebrew literals. No `shadow-*` (flat elevation, DESIGN.md). No gold anywhere in this file except the winner name's `text-gold`.** Comments in English.

- [x] **Task 5: re-triage the `deferred-work.md` entries this story triggers** (no product code)

  Two entries name this story directly. Append each outcome under the existing entry in the file's established style — sub-bullet, dated, trigger re-pointed. **Do not rewrite the original text.**

  - [x] **The 3.7 degraded-`Snapshot.Leaderboard`-consumer entry** (re-pointed by 4.5: *"story 4.6's winner takeover reads the same field and is the likely one"*). **Triggered, and resolved DIFFERENTLY from 4.5.** Record derived requirement 5 in full: `finished`'s discriminator is `questionCount === 0` alone, not `isDegradedFrame`'s two-part OR, because `current_question_position` is unconditionally reset to 0 on entry to `finished` (unlike `leaderboard`, which never moves it) — so `currentQuestion === null` is true on every real `finished` frame and cannot be part of this stage's guard. State explicitly that this is now the **second** state-specific discriminator on `Snapshot.Leaderboard`'s degraded ambiguity, and re-point the trigger to *"the next consumer of `Snapshot.Leaderboard` outside `leaderboard-stage.tsx` and `winner-stage.tsx`"* — there is no third consumer in Epic 4, so note this is likely dormant going forward unless a future epic adds one.
  - [x] **The heading-semantics entry** (re-pointed by 4.4 to *"4.6's winner name… or the first real screen-reader evaluation of the display, whichever comes first"*, with 4.4's explicit warning that a fourth deferral should not happen). **Triggered and CLOSED, not re-deferred.** Record that the winner name is now an `<h1>`, and that the entry's final accepted shape is: two of five built stages have `<h1>`s (question/reveal's shared one, and this one), the lobby join code and the leaderboard rows remain plain elements, and this is the shape the epic ships with — no further Epic 4 story exists to revisit it. If a real screen-reader evaluation ever happens, it is a fresh entry, not a reopening of this one.
  - [x] **Confirm in writing, with reasons, that these are NOT triggered:**
    - the **2.5 spectator-roster entry** (role-blind `ListParticipants`). Not triggered for the same reason 4.5 was not: this stage reads only `snapshot.leaderboard`, which `GetLeaderboard` already filters to `role = 'player'` in SQL, and never reads `snapshot.participants`.
    - the **timer clock-skew entry**'s parenthetical mention of *"4.6's winner"* possibly needing server time. Does not apply: this stage renders no countdown, no deadline and no timer of any kind — it is a static (or CSS-looping) full-screen takeover with nothing time-relative to compute. State this plainly so a future reader does not go looking for a server-time dependency that was never real.
    - the **4.5 leaderboard-reshuffle enter/exit-animation entry** (*"4.6's winner takeover is the next candidate"* for rows entering/leaving mid-animation). Does not apply: this stage's confetti pieces are a static set that loops in place — there is no row set that grows, shrinks or reorders the way the leaderboard's does, so there is no enter/exit case to build.
    - the **reconnect-band legibility-on-green-800 note** (4.2/4.3 recorded it as a two-data-point pattern already established, and explicitly said a third from this story "is no longer needed to establish the pattern"). Not a re-triage — just confirm in the Dev Agent Record whether the browser pass (Task 7, check 8) happened to observe it, without treating it as a required measurement.

- [x] **Task 6: quality gates and new Vitest coverage** (all ACs)

  - [x] **Frontend gates**: `npm run lint` · `npx tsc -b --noEmit` · `npm test` · `npm run build` · both filter-safety scans (source and built bundle — no external URL, no font `@import`, no `url(//…)`).
  - [x] **Hebrew centralization**: `rg -l '[\p{Hebrew}]' -g '*.tsx' web/src` and the same over `-g '*.ts'` must list only the two known pre-existing violations (`scoring-editor.tsx`, `control-page.tsx`) — the 4.5 review's finding that a `.ts`-only scan misses a `.ts` file applies equally in the other direction; check both globs. **No new file may appear in either.**
  - [x] **The six untouched display test files**: `git diff <baseline> -- web/src/features/display/{lobby-stage,question-stage,timer-ring,reveal-stage,leaderboard-stage}.test.tsx web/src/features/display/use-leaderboard-memory.test.ts` is **empty** and all their cases pass. (`display-page.test.tsx` is expected to change — see Task 3.)
  - [x] **Built-CSS verification** (4.3's rule, repeated at every stage since — verify, do not assume): `--stage-winner-name`, `.stage-confetti-piece`, `@keyframes stage-confetti-drift`, and `text-gold`/`bg-gold`/`bg-green-800` (the two gold utilities are this story's first use of either as a Tailwind class rather than an inline SVG attribute). Remember Tailwind escapes arbitrary-value selectors (`.h-\[var\(--stage-winner-name\)\]` etc.) — a naive literal grep reports them MISSING (4.4's and 4.5's Debug Logs both hit this; search the escaped form).
  - [x] **New Vitest coverage** — co-located `winner-stage.test.tsx`, `describe`/`it` imported explicitly (`globals` is off), `afterEach(cleanup)`, expected copy read from `strings.he.ts` and never retyped:
    - a single positive-score leader with `rank: 1` renders their name and score, and no other participant's name appears;
    - **two entries tied at `rank: 1` with equal positive scores** both render, stacked, with the score shown exactly **once**;
    - **four entries tied at `rank: 1`** render only the first three (order as delivered by the server — never re-sorted by this file) and the fourth does not appear;
    - an entry at `rank: 1` with `score: 0` (the stopped-before-first-Reveal case) does **not** render as a winner — this is the case that proves the strict positivity filter, not merely the rank filter;
    - a healthy frame whose filtered winners array is empty (either an empty `leaderboard` or an all-zero one) renders `strings.display.winner.noWinner` and **no** confetti;
    - a `questionCount: 0` frame renders `strings.display.waiting`, **not** the no-winner copy — the case that specifically distinguishes derived requirement 5's guard from derived requirement 6's, and the one most likely to be silently wrong if the wrong discriminator were used; demonstrate this red first by swapping in `isDegradedFrame` and recording the failure, exactly as 4.5's Debug Log demonstrates each of its guards red before green;
    - `reducedMotion={true}` removes `stage-confetti-piece` from every confetti node while the name and score are unaffected;
    - `reducedMotion={false}` applies `stage-confetti-piece` to every confetti node;
    - the winner name renders inside an `<h1>` element (closes derived requirement 8 — assert the tag, not just the text);
    - no participant id, phone number, or any field from `snapshot.participants` appears anywhere in the rendered container (mirrors 4.4's and 4.5's privacy assertions, even though this stage never reads that field — assert it rather than trusting the omission).
  - [x] `display-page.test.tsx`'s new case: the winner stage mounts for `finished` and for no other state (mirrors 4.5's Task 11 assertion shape, anchored on `stageByState.finished` rather than an incidental DOM count).

- [x] **Task 7: the manual browser pass** (all ACs)

  - [x] **Reuse the 4.3–4.5 harness shape** (`playwright-core` + `channel: 'msedge'`, installed into the scratchpad, never into `web/package.json`; a bash orchestrator for any kill/restart phase, per the Hebrew-home-directory `child_process.spawn` limitation 4.4 first hit). No new Go E2E harness is needed — this story ships no backend code, so there is nothing server-side to exercise that Task 13-style scenarios in prior stories covered; drive `finished` by playing a real game to its end (or via `POST /stop`) and observe the display.
  - [x] **AC-1, the card.** Green-800 background (`rgb(22, 101, 52)`); winner name in gold (`rgb(251, 191, 36)`) at the `--stage-winner-name` size, computed and compared at both 1920×1080 and 1280×720; tagline and score in `ink-on-dark-muted` (`rgba(255,255,255,0.70)`); score shown exactly once even with a tie seeded.
  - [x] **Derived req. 13, both halves.** Scan every node's `color`/`background`/`fill`/`stroke` for `rgb(251, 191, 36)` (gold): it must appear on the winner name **and only there** among text/background — confirm the tagline and score are NOT gold, and the confetti pieces carrying the gold variant are the only other gold pixels on screen. This is the one stage where the standard "assert zero gold" check from every prior stage's pass must be inverted, and both directions need asserting.
  - [x] **AC-2, the celebration.** 22 confetti pieces present, `aria-hidden`, positioned per the transcribed mockup coordinates, animating (sample a piece's computed `transform` at two points in time and confirm it changed) under normal motion.
  - [x] **AC-2's reduced-motion clause, both channels** (the dashboard toggle and the OS setting, independently, per this project's established two-channel check): confetti pieces present at their static positions with no `stage-confetti-piece` class and no changing `transform` over a sampled interval; name, score and tagline unaffected.
  - [x] **AC-3, the tie.** Seed a genuine tie for first (at least two participants, identical top score) and confirm both names stack, in the received order, with one score line.
  - [x] **AC-3, the announcement.** Confirm the assertive live region's text reads `stateAnnouncement.finished` at the moment this stage mounts (reuses the shell's existing mechanism — this check is verifying, not building).
  - [x] **The no-winner case.** Stop a game from `question_open` (before any Reveal) and confirm the fallback message renders with no confetti and no name.
  - [x] **Fit, at both 1920×1080 and 1280×720.** Three stacked names at realistic length, plus the tagline and score, all within the safe margin with no overflow and no scrollbar (the shell is `overflow-hidden`, so an overflowing name would be silently clipped).
  - [x] **Reconnect.** Kill the server with the winner card on screen (real kill, `ws.on('close')` asserted, not `ctx.setOffline`); the card holds through the outage and re-renders unchanged after restart with no interaction. Note whether the reconnect band covers any content, per Task 5's confirm-not-measure note.
  - [x] **Privacy.** No phone number, no per-question grade, no non-winning participant's name anywhere in the DOM.
  - [x] **Filter-safety.** Zero external host requests; no `Fetch/XHR` on the display route; only geometric CSS shapes, no images.
  - [x] Zero console errors outside a deliberate outage.
  - [x] Stories 3.10's, 4.1's and 4.2's manual passes are still outstanding per 4.5's own honest accounting — this pass does not put you in front of those surfaces either, so continue not claiming them.

### Review Findings

Code review 2026-08-14 (bmad-code-review; three parallel adversarial layers — Blind Hunter, Edge Case Hunter, Acceptance Auditor — all three completed, no failed layer).

**The story's own headline risk is clean.** Derived requirement 5 is implemented exactly as specified and was verified independently against the Go source rather than against the diff's comments: `FinishGame` sets `current_question_position = 0` (`queries/games.sql`), `buildSnapshot` only populates `current` when the position is `> 0` (`engine.go:591`), `emptySnapshot` leaves `QuestionCount` at the Go zero (`engine.go:706-720`), and `StartGame` refuses `len(questions) == 0` (`engine.go:264`). `DeleteQuestion` is `state = 'draft'`-guarded, so no finished game can fall back to zero questions afterwards. `questionCount === 0` alone is therefore the correct discriminator, `isDegradedFrame` is genuinely not imported, and reusing it really would have shown the waiting copy on every finished game forever. The winner filter mirrors `final.go:92` field-for-field, and rank-1 rows are provably a contiguous max-score prefix (`scoring.go:79-93`), so the `[topWinner]` destructure is sound.

**All 22 confetti pieces were compared against the mockup one by one** — `left`, `top`, `animation-duration`, `animation-delay`, colour and shape/size — and every value matches, with the 13-gold/9-green split as claimed and the five size classes exact `cqw→vw` renames. **Scope is clean**: zero diff under `server/`, zero diff on the six frozen display test files, `stage-props.ts` untouched, and every file on the "a diff here means you went off-spec" list shows zero diff. Gates re-run independently and reproduced green (`tsc -b --noEmit`, `eslint`, Vitest 106 passed, `go build`/`vet`/`test`), and all five `deferred-work.md` dispositions are present with the right outcomes.

**All three declared deviations were judged independently and all three hold.** `<span className="block">` is required because `<h1>`'s content model is phrasing content, so the story's own `<div>` sketch was invalid markup. The plain `<bdi>` is correct because the composed value's first strong character is Hebrew, so `dir="ltr"` would mirror the sentence. And moving `position: absolute` out of `.stage-confetti-piece` **fixes a real defect in the story's own CSS sketch** — the class is removed under reduced motion, so the bundled form would have dropped all 22 pieces into normal flow at exactly the moment derived requirement 10 asks for a static scattered frame.

Six findings were falsified by direct inspection and dismissed rather than reported: that `snapshot.leaderboard` can arrive `null` and throw in `.filter` (`RankLeaderboard` returns `make(...)` and `emptySnapshot` uses `[]LeaderboardEntry{}` — both non-nil, so Go marshals `[]`); that a finished game's questions can later be deleted and strand the guard (`DeleteQuestion` is draft-only); that `tagline`/`noWinner` need bidi isolation for their trailing `!`/`.` (base direction is RTL at both `index.html:2` and `display-page.tsx:174`, so neutral punctuation resolves correctly); that the missing `deferred-work.md` edit is absent (it exists — it was outside the patch handed to the context-free layer); that the degraded and no-winner branches emitting no heading is a defect (a `<p>` fallback matches every other stage); and the `ConfettiPiece` type dropping the spec's `size` field (declared, and the spec's own type was internally contradictory — a non-optional `size` would force a meaningless value on bars and squares).

**Decision needed — both resolved 2026-08-14 by Avraham (one → patch, one → confirmed with a sub-item deferred):**

- [x] [Review][Decision] **The confetti's vertical travel lost the mockup's 16:9 container invariant, and this is the one large measure in the file with no fit guard** — the mockup measures `60cqw` against `.frame-shell` (`container-type: inline-size`) wrapping a `.stage` with `aspect-ratio: 16/9`, so the drift distance was *always* 1.067 × stage height. Ported to `vw` against the raw viewport, that invariant is gone. For the top-most piece (`top: 8%`) to clear the bottom at the `to` keyframe the viewport must satisfy `W/H > 1.533`; 16:9 is 1.778 so 1920×1080 and 1280×720 are both clean, which is why Task 7's pass at exactly those two resolutions saw nothing. Below ~1.53:1 (4:3, 5:4, portrait, or a merely un-maximized window) at least one piece is still on screen at both ends of an `infinite` cycle with an explicit `from`/`to`, so it teleports and its rotation jumps 300° in a single frame every 10–14s. This is the only large viewport-relative measure in `index.css` without the `min(Xvw, Yvh)` guard that `--stage-lobby-code`, `--stage-timer-ring`, `--stage-option-min`, `--stage-leaderboard-row` **and `--stage-winner-name` in this very diff** all carry — and `--stage-winner-name`'s own new comment cites the sub-16:9 case as reachable. The call is genuinely ambiguous because derived requirements 10 and 11 bind a verbatim transcription, while the file's convention says guard it. Options: **(a)** clamp the travel — `translateY(min(60vw, 33.75vh))` at both ends, equal at 16:9, same shape as every ramp guard; **(b)** accept 16:9 as the display's contract and record it, since a projector is 16:9 and the story never scoped anything else; **(c)** treat it as a spec question and re-point derived requirement 11 to say the fit-guard convention applies on top of the rename. [web/src/index.css:506-515] — **Resolved: option (a).** Clamp the travel with the same fit-guard shape every large measure in the file already carries. The two terms are equal at 16:9, so nothing Task 7's browser pass actually observed changes; what changes is only the behaviour below 16:9, which was never measured. Carried below as a patch.
- [x] [Review][Decision] **The three `[ASSUMPTION]`s the Dev Agent Record raised for your confirm-at-review, plus one grammar consequence nobody flagged** — (i) `display.winner.scoreSuffix` → `"1240 נקודות"`; (ii) `display.winner.noWinner` → `"המשחק נגמר! הפעם לא נצברו נקודות."`, which the comment says "mirrors" `messages_he.go`'s `msgFinalResultsNoWinner` while in fact inventing new wording, with no shared constant and no test pinning the two together — they can now drift silently, which is the exact failure the neighbouring `noPlayers` comment says this file is trying to avoid; (iii) the score renders as raw digits with no thousands separator, against the mockup's illustrative `1,240` but with `messages_he.go`'s bare `strconv.Itoa`, `results-summary.tsx` and `leaderboard-stage.tsx` — three code sources agreeing and only the mockup disagreeing. **And the consequence:** `scoreSuffix(1)` renders `"1 נקודות"` ("1 points"), reachable whenever a scoring rule can award a single point. Six other strings in this same file spell out the Hebrew singular (`שאלה אחת`, `משתתף אחד שיחק`, `תשובה נכונה אחת`, `עלייה של מקום אחד`), and `scoreSuffix`'s own comment invokes `results.playerCountLabel`'s "spelled-out-singular discipline" before declining to apply it. Mitigating: `messages_he.go` has the identical gap, so the two surfaces currently agree — fixing only the display would create the divergence (ii) warns about. Options per item: confirm as-is · reword · add the singular form here only · add it on both surfaces. [web/src/lib/strings.he.ts:345-378] — **Resolved: all three `[ASSUMPTION]`s confirmed as-is.** `scoreSuffix`'s wording, `noWinner`'s wording and the raw-digit score all stand as written; the `[ASSUMPTION]` flags in the Dev Agent Record are closed as approved, not outstanding. **The singular consequence was taken separately and deferred**, reason recorded: `messages_he.go` has the identical gap, so the two surfaces agree today, and fixing only the display would manufacture exactly the drift item (ii) warns about — while fixing both means a change under `server/`, which this story's scope boundary forbids outright. Carried to `deferred-work.md` as one item covering both surfaces.

**Patch — all eight applied 2026-08-14, in the same session.** Five were demonstrated **red first**, per this project's standard, with the mutation applied to the shipped code and reverted afterwards:

1. Colour and shape classes deleted from the piece (`colorClass`/`shapeClass` — the mutant that makes the whole celebration invisible while leaving 22 spans in the DOM) → `paints every piece in one of the two permitted colours, with a shape` fails with `expected [] to have a length of 13 but got +0`. Under the original suite this mutation was completely silent.
2. `<bdi>` → `<bdi dir="ltr">` on the score — the exact defect the ten-line comment above it exists to prevent → `isolates the score without forcing its direction` fails. `scoreLinesOf` reads only `textContent`, so nothing in the original suite could see the attribute.
3. `z-10` dropped from the content wrapper (confetti paints over the winner's name) → `holds the stage surface contract…` fails with `expected null not to be null`.
4. The name's `<bdi className="… break-words">` → `<span className="block">` → **seven** cases fail, including the tie, cap, privacy and reduced-motion cases, because `namesOf` is now structurally anchored on the isolation element rather than on an incidental `span`.
5. `pointer-events-none` removed **and** every piece's `left`/`top` pinned to one coordinate → three cases fail across two files: the overlay assertion, `scatters the pieces the way the mockup does` (`expected 1 to be 22` distinct coordinates), and `display-page.test.tsx`'s mount anchor. The original suite passed under both.

**Gates after the patches**: `tsc -b --noEmit` · `eslint` · **Vitest 112 passed / 9 files** (106 → 112) · `npm run build` · built-CSS verification of every new artifact in the escaped form (`.gap-\[1\.25vw\]`, `.leading-heading`, `.break-words`, `.pointer-events-none`, `.max-w-full`, `.text-\[length\:var\(--stage-winner-name\)\]`, `--stage-winner-name:min(6.5vw, 11.5556vh)`, and `@keyframes stage-confetti-drift{0%{transform:translateY(calc(-1*min(60vw,33.75vh)))rotate(0)}to{…}}`) · the `.tsx` Hebrew scan back to exactly the two known pre-existing violations and the `.ts` scan to `strings.he.ts` plus the pre-existing `types.ts` doc-comment fragment, **no new file in either** · `git status server/` still empty · the six frozen display test files still byte-identical to `7168de8`.

- [x] [Review][Patch] **Clamp the confetti's vertical travel to restore the mockup's 16:9 invariant** [web/src/index.css:506-515] — from the decision above. `translateY(min(60vw, 33.75vh))` at both ends of `@keyframes stage-confetti-drift` (33.75vh = 60vw at 16:9, the same equal-at-16:9 shape as `--stage-lobby-code`, `--stage-timer-ring`, `--stage-option-min`, `--stage-leaderboard-row` and `--stage-winner-name`). Nothing changes at 1920×1080 or 1280×720, so Task 7's measurements stand unaltered; below 16:9 the pieces stop teleporting. Update the keyframe's comment to say the guard is there and why, since the rule's current comment presents the `cqw→vw` rename as complete.

- [x] [Review][Patch] **The winner's `displayName` is neither bidi-isolated nor overflow-guarded, while the identical field is both in every sibling stage** [web/src/features/display/winner-stage.tsx:236-252] — `leaderboard-stage.tsx:152` renders the same field as `<bdi className="min-w-0 flex-1 truncate">` with the comment *"a display name is free-form participant-supplied text that can be Latin, mixed, or start with a digit inside an RTL row"*, and `reveal-stage.tsx:253` uses `<bdi className="min-w-0 break-words">`. This file wrote ten lines justifying `<bdi>` for the **score** and left the free-form field bare. Consequence 1 (bidi): inside the RTL document, a name with leading neutrals — `!David`, `(Dana)`, `#7 Moshe` — has them reordered to the opposite end, so `!David` paints as `David!`. Consequence 2 (overflow): `display_name` is uncapped `TEXT` (`migrations/00008_participants.sql:6`) and `parseRenameName` accepts any length, so a participant can set a 200-character name from WhatsApp; with no `break-words`/`min-w-0` an unbreakable word blows past the 1824px content box at `--stage-winner-name` = 124.8px, and `surfaceClass`'s `text-center` + `overflow-hidden` clips it **symmetrically** — the room sees the middle of the name with no ellipsis and no indication anything was cut. This is the largest text in the product and the only one with no guard.
- [x] [Review][Patch] **`--stage-winner-name`'s vh term is arithmetically wrong and its comment states a false measurement** [web/src/index.css:296-313] — the comment claims *"11.5741vh = 124.8px at 1080p, equal to 6.5vw at 1920x1080"*. It is 125.00px, not 124.8px; the exact coefficient is **11.5556vh** (`6.5 × 16/9`). Every other large step in the file is exact to the same rule — `min(9vw, 16vh)` → 172.8/172.8, `min(11.4583vw, 20.3704vh)` → 220.0/220.0, `min(5vw, 8.8889vh)` → 96/96, `min(3.75vw, 6.6667vh)` → 72/72 — so this is the only inexact member of the pattern its own comment claims membership in, and the guard engages marginally below 16:9 rather than at it. The error originates in the story's own Task 2 literal, which was transcribed faithfully; the fix is the number and the sentence.
- [x] [Review][Patch] **`pointer-events: none` was dropped from the confetti overlay** [web/src/features/display/winner-stage.tsx:190] — the mockup's `.confetti{position:absolute;inset:0;overflow:hidden;pointer-events:none}` has four declarations and the port carries three. The overlay is `inset-0` across the whole stage and paints under the `relative z-10` content, so it swallows pointer events everywhere the content box does not cover. Latent today (nothing on this stage is interactive) but it is a straight omission from a transcription derived requirement 10 calls "the full specification".
- [x] [Review][Patch] **The tagline and score line-heights were not transcribed, while the name's was** [web/src/features/display/winner-stage.tsx:220-222, 269-271] — the mockup gives `.w-tagline{line-height:1.38}` and `.w-score{line-height:1.38}`; the `<h1>` correctly received `leading-[1.1]` from `.w-name{line-height:1.1}`, but neither `<p>` carries a leading utility, so both inherit preflight's `1.5`. The project already has the exact token — `--leading-heading: 1.38` (`index.css:63`), used as `leading-heading` by `reveal-stage.tsx:140`. At 1080p the tagline's line box is 60px instead of 55.2px and the score's 72px instead of 66.2px.
- [x] [Review][Patch] **`gap-6` recomputes the mockup's `1.25cqw` instead of renaming it** [web/src/features/display/winner-stage.tsx:219] — derived requirement 11: *"Rewrite every `cqw` in the mockup's CSS as `vw` unchanged in magnitude; do not recompute anything."* The mockup's `.w-name` and `.w-score` both carry `margin-top:1.25cqw`. The code uses a fixed 24px stop citing `reveal-stage.tsx`'s precedent, but that precedent does not transfer: `reveal-stage.tsx:130-135` deviates from *its* mockup **because its values are copied from the question stage** — *"matching the mockup here would make the question text jump at the moment of reveal"*. The winner stage shares no element with any other stage, so no continuity constraint exists. Effect: correct at 1920, but 24px at 1280 where the mockup specifies 16px — the only spacing on the stage that stops scaling with the projector. Not listed among the three declared deviations. `gap-[1.25vw]`.
- [x] [Review][Patch] **The unit suite leaves the entire visual contract unasserted — eight surviving mutants, several of them silent catastrophes** [web/src/features/display/winner-stage.test.tsx] — the browser pass covered these manually, but nothing in CI would catch a regression. Named mutants that pass all 12 new assertions: deleting `${colorClass[piece.color]} ${shapeClass[piece.shape]}` (22 zero-size transparent spans — `colorClass`, `shapeClass`, `PieceColor` and `PieceShape` have **zero** assertions anywhere); changing `bg-green-800` or `text-gold` to anything at all (the stage's two mandated colours, including "the product's second and last gold moment", are unasserted — a light ground would put gold text at ~1.5:1); replacing all 22 `confettiPieces` rows with 22 identical ones (only the count and `not.toBe('')` are checked, so the "transcribed verbatim, the scatter is what was actually reviewed" claim is untested, as is the negative-delay sign the source comment says the static frame depends on); `<bdi>` → `<bdi dir="ltr">` (the exact defect the ten-line comment above it exists to prevent — `scoreLinesOf` reads only `textContent`); deleting `z-10` (confetti paints over the winner's name); deleting `p-[var(--stage-margin)]` (content runs into projector overscan on all three branches); and `topWinner.score` → `snapshot.leaderboard[0].score`. Separately, four of the six assertions in the privacy case are vacuous: `[data-participant-id="…"]` is an attribute the component never emits, `+972` cannot come from the `participants` fixture (which has no phone field), and `game-1` guards a field no branch reads — only the two `ROSTER-*` checks discriminate. And `renders the winner on a healthy frame whose currentQuestion is null` passes `snap({ currentQuestion: null, questionCount: 5 })`, which is a **no-op override of `snap()`'s own defaults** — its input is byte-identical to the first test's, so the case billed as "THE case that proves derived requirement 5" adds no discriminating power over a case that already exists.
- [x] [Review][Patch] **`.stage-confetti-piece`'s layering rationale is carried over from a rule where it was true** [web/src/index.css:439-441] — the comment justifies the layer because *"an unlayered rule would outrank the `w-`/`h-`/`bg-` utilities the pieces themselves carry"*. This rule declares only `animation-name`, `animation-timing-function` and `animation-iteration-count` — three properties no width, height or background utility declares. There is no cascade conflict to resolve here. The layer placement is right (consistency), but a reader deciding whether it is *required* has been given a reason that is not true of this rule.

**Deferred:**

- [x] [Review][Defer] **`scoreSuffix(1)` renders `"1 נקודות"` — no Hebrew singular form, on either surface** [web/src/lib/strings.he.ts:367] — deferred at Avraham's call at this code review (2026-08-14), reason recorded: `messages_he.go`'s own templates have the identical gap, so the display and WhatsApp agree with each other today; patching only `strings.he.ts` would create precisely the silent divergence between the two surfaces that `noWinner`'s neighbouring comment says this file is trying to avoid, and patching both requires a change under `server/`, which this story's scope boundary forbids outright. Reachable whenever a scoring rule can award exactly one point. Note that six other strings in this same file *do* spell out the Hebrew singular, and `scoreSuffix`'s own comment invokes that discipline before declining to apply it.
- [x] [Review][Defer] **A degraded frame at `finished` is permanent, not "for a beat" — the winner takeover never appears at all** [web/src/features/display/winner-stage.tsx:144-150] — deferred, pre-existing and out of scope. If `buildSnapshot` fails on the transition into `finished`, `snapshotAfterCommit` degrades to `emptySnapshot` (`engine.go:548-555`) with `QuestionCount: 0`, and the stage correctly renders `strings.display.waiting`. But `finished` is **terminal**: `'finished'` appears in `games.sql` only as `FinishGame`'s target and never in a transition's `WHERE`, and `RecordAnswer`/`joinLobby` are gated to `question_open`/`lobby`. That degraded frame is therefore the last snapshot ever broadcast for the game — the projector shows the waiting copy for the rest of the event, and the only recovery is a socket drop (`ws/handler.go:82` reads a fresh `engine.Snapshot` per connection). The shell's `retained` does not help: a degraded frame is a real snapshot and replaces it. The same degradation at `leaderboard` lasts "for a beat" (`stage-props.ts:47-64`); at this one state there is no next frame. Not this story's to fix — the scope boundary forbids any change under `server/`, and the fix belongs in `snapshotAfterCommit` or in a client-side refetch, not in the stage.

## Dev Notes

### Why this story is smaller than 4.5, and where its real difficulty is

4.5 was a three-layer story (backend transition + dashboard CTA + display stage) because story 3.1 had deliberately left the sixth state transition unbuilt. Every transition and every dashboard control this story needs already exists — `finished` has been reachable, and fully handled by the dashboard's results summary, since stories 3.9/3.10. This story is **display-only**, the same shape as 4.2–4.4: one new component, one `stageByState` entry, some copy, some CSS.

The size is small; the risk is not. This story's actual difficulty is entirely in derived requirement 5 — the temptation to copy `leaderboard-stage.tsx`'s `isDegradedFrame` guard verbatim, because it is the freshest, most obviously analogous pattern in the codebase, and because `deferred-work.md` itself says this story "reads the same field" as the leaderboard did. That framing is correct about the field and wrong about the guard. Get the guard right and the rest of this story is straightforward transcription from a mockup that, unlike 4.5's, actually exists.

### Where every rendered value comes from

| Rendered | Source | Notes |
|---|---|---|
| Winner name(s) | `snapshot.leaderboard.filter(e => e.rank === 1 && e.score > 0)` | mirrors `game/final.go`'s `ResultsForFinishedGame` rule exactly (derived req. 4); never sorted, never re-ranked |
| Score | `winners[0].score` | raw digits, no separator (derived req. 7); shown once regardless of tie count |
| Waiting copy | `snapshot.questionCount === 0` | the degraded frame — **not** `isDegradedFrame` (derived req. 5) |
| No-winner copy | guard passed, `winners.length === 0` | a healthy frame with nobody clearing the positivity bar (derived req. 6) |
| Tagline | `strings.display.winner.tagline` | static, mockup-transcribed, no tie/no-tie variant needed |

### Existing code this story modifies — current state, and what must survive

- **[web/src/features/display/display-page.tsx](web/src/features/display/display-page.tsx)** — one map entry, one import. The `answeredFloor`/`stageSnapshot` block, `retained`, `usePrefersReducedMotion`, `useLeaderboardMemory` and the reconnect band are all 4.1–4.5 review decisions and are untouched.
- **[web/src/index.css](web/src/index.css)** — one custom property inside `.stage-root`, one rule inside `@layer components`, one `@keyframes` beside the four existing ones.
- **[web/src/lib/strings.he.ts](web/src/lib/strings.he.ts)** — one `winner` sub-block inside `display`.
- **[_bmad-output/implementation-artifacts/deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md)** — Task 5's written outcomes, appended under the existing entries.

### Testing standards

Vitest, unchanged since 3.11: co-located `*.test.tsx`, `globals` off (import `describe`/`it`/`expect` explicitly), `afterEach(cleanup)`. Expected Hebrew read from `strings.he.ts`, never retyped; fixture participant names use Latin placeholders so the frontend Hebrew grep stays clean.

**The one case worth demonstrating red first, per this project's standard** (3.11, 4.2, 4.3, 4.4 and 4.5 all did this at least once): the `questionCount: 0` case, swapping in `isDegradedFrame` in place of the local check and recording that it does not change the result for THAT case (it still shows waiting) but **does** change the result for a normal finished frame with `currentQuestion: null` and `questionCount: 5` — which `isDegradedFrame` would incorrectly also call degraded. That second, contrasting assertion is the one that actually proves derived requirement 5; the first alone would not.

Go: no changes this story, so no new Go tests. `go build ./... && go vet ./... && go test ./...` should still be run and confirmed green as a matter of course (unchanged, so this is a smoke check, not new coverage), and `git status` should show zero diff under `server/`.

### Project Structure Notes

**New:**
- `web/src/features/display/winner-stage.tsx` — named by the architecture (`winner-stage.tsx` in the `features/display/` tree, the last file that tree names)
- `web/src/features/display/winner-stage.test.tsx`

**Modified:**
- `web/src/features/display/display-page.tsx`, `display-page.test.tsx`
- `web/src/index.css`
- `web/src/lib/strings.he.ts`
- `_bmad-output/implementation-artifacts/deferred-work.md`

**Untouched (a diff here means you went off-spec):** everything under `server/**` · `web/src/features/display/{stage-props,leaderboard-stage,use-leaderboard-memory,reveal-stage,question-stage,timer-ring,lobby-stage,stage-hero-band,stage-option,stage-placeholder}.*` · `web/src/lib/{use-game-socket,api,text,use-space-action,use-single-flight,use-throttled-announcement,types}.ts` · `web/src/features/{lobby,builder,results,auth,live}/**` · `web/src/app.tsx` · `web/src/components/**` · `web/index.html` · `web/package.json` · `web/vite.config.ts` · `.github/workflows/ci.yml` · the five display test files plus `use-leaderboard-memory.test.ts` named in Task 6.

### References

- Epic + ACs: [epics.md](_bmad-output/planning-artifacts/epics.md#L748-L764) (Story 4.6), [#L191-L194](_bmad-output/planning-artifacts/epics.md#L191-L194) (Epic 4 framing, incl. "the final WhatsApp winner message… is already delivered by Epic 3's FR-6 results story")
- Visual spec: DESIGN.md frontmatter `components.winner-card`; `Colors` contrast table (gold on green-800, 4.3:1, large-text/non-text only); `Brand & Style` (gold's two moments); `Typography` (Display 900, "projection-large")
- **Mockup, pixel-authoritative** (unlike 4.5, which had none): [mockups/key-stage-winner.html](_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/mockups/key-stage-winner.html) — the confetti layout, the tagline/name/score composition, and the `cqw`-scaling note this story's derived requirement 11 relies on
- Behaviour: EXPERIENCE.md → IA `Audience Display — stages`, Winner takeover row (`[A16]`); `Component Patterns` → Audience Display stages, Winner takeover bullet; `State Patterns` → Game over row; `Voice and Tone` → the מזל טוב discipline; templates table → Winner's final message row and Final results — no winner row; Flow 2 (UJ-2) step 7
- Backend winner rule this story mirrors, does not reimplement: [server/internal/game/final.go](server/internal/game/final.go) — `ResultsForFinishedGame`'s strict-positivity winner definition
- FR-18 (leaderboard + winner takeover), UJ-2: [prd.md](_bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md)
- Architecture: the `features/display/` tree naming `winner-stage.tsx` as the last file in that tree; `Component Boundaries (Web)` ("display/* renders exclusively from the WS snapshot")
- Deferred entries this story resolves or confirms not-triggered: [deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md) — the re-pointed 3.7 `Snapshot.Leaderboard`-consumer entry (line ~136, *"4.6's winner takeover reads the same field"*), the heading-semantics entry (re-pointed through 4.2→4.3→4.4, closing here), and confirmations for the 2.5 spectator-roster, timer clock-skew and 4.5 enter/exit-animation entries
- Prior stories: [4-5-leaderboard-stage-the-room-reshuffles.md](_bmad-output/implementation-artifacts/4-5-leaderboard-stage-the-room-reshuffles.md) (the `isDegradedFrame` mechanism this story deliberately does NOT reuse, and why — read its Dev Notes and the `stage-props.ts` doc comment before writing Task 4) · [4-4-reveal-stage-the-answer-marked.md](_bmad-output/implementation-artifacts/4-4-reveal-stage-the-answer-marked.md) and [4-3-question-stage-with-the-authoritative-timer.md](_bmad-output/implementation-artifacts/4-3-question-stage-with-the-authoritative-timer.md) (the `<h1>` heading-semantics precedent derived req. 8 closes) · [3-9-final-results-for-everyone.md](_bmad-output/implementation-artifacts/3-9-final-results-for-everyone.md) if present, else `server/internal/game/final.go` and `server/internal/wa/messages_he.go` directly (the winner rule and copy tone this story mirrors)

### Latest technical information

No dependency changes and no new libraries — this story is pure CSS/TSX over an existing wire contract. Versions unchanged since 4.5: React 19.2, React Router 8.2, Tailwind CSS 4.3, TypeScript 6.0, Vite 8.1, Vitest 4.1, `@testing-library/react` 16.3, `eslint-plugin-react-hooks` 7.1.

Carried-forward version-specific traps, all still live:

- **Tailwind v4 reads source text, not runtime values** — every confetti class combination must be a complete literal; no `` `bg-${x}` `` template.
- **Tailwind escapes arbitrary-value selectors** in the built CSS (`.h-\[var\(--stage-winner-name\)\]`) — a literal grep reports them MISSING; search the escaped form (4.4's and 4.5's Debug Logs both record this).
- **`text-*` with a CSS variable needs the `length:` hint**: `text-[length:var(--stage-winner-name)]`, not `text-[var(--stage-winner-name)]`.
- **`verbatimModuleSyntax: true`**: `StageProps` and `CSSProperties` (if used for the per-piece inline style) must be imported with `import type`.

## Dev Agent Record

### Agent Model Used

claude-opus-5 (Claude Code, `bmad-dev-story` workflow), 2026-08-14.

### Debug Log References

**1. Baseline resolution — 4.5 had NOT merged, so the second branch of the Baseline section applied.** At start, `origin/main` sat at `bf0b93a` (the 4.4 merge) and `story/4-5-leaderboard-stage` was at `7168de8`, **local only** (no upstream tracking ref — it has not been pushed, let alone merged). Branched `story/4-6-winner-takeover` from `7168de8` exactly as the Baseline section's second bullet instructs. **The actual baseline SHA is `7168de8`**, which is what this file's frontmatter already carried, so no correction was needed. Verified `leaderboard-stage.tsx` and `stage-props.ts`'s `isDegradedFrame` export are both present in the checkout.

**2. The red-first demonstration of derived requirement 5, and it is more emphatic than predicted.** Before writing the guard, `isDegradedFrame(snapshot)` was swapped in for the local `snapshot.questionCount === 0` check and `winner-stage.test.tsx` was run:

```
Tests  9 failed | 2 passed (11)
```

The **only two** cases that stayed green were `renders the WAITING copy … on a degraded frame` (which is why it cannot stand alone as coverage — it passes under both implementations) and the privacy case (which asserts absence). Everything requiring a winner to exist failed, including `renders the winner on a healthy frame whose currentQuestion is null` — the contrasting case the story's Testing Standards identify as the one that actually proves the requirement. In production this would have shown `display.waiting` on **every finished game, forever**. Guard reverted to `questionCount === 0`; suite green.

**3. `npm run build` caught a comment-nesting error the linter and tsc could not.** The first `index.css` edit placed the new `.stage-confetti-piece` rationale **after** the preceding comment's closing `*/`, leaving prose in bare CSS and a stray `*/`. Tailwind's parser reported `CssSyntaxError: Unterminated string: 's .cf, which bundles both…'` — the apostrophe in "mockup's" opening a CSS string. Fixed by folding the paragraph into the preceding comment block. Worth recording because `npm run lint` and `npx tsc -b` were both green with the file in that state: **only the build gate sees CSS syntax.**

**4. Built-CSS verification (4.3's verify-don't-assume rule), all confirmed in `dist/assets/index-*.css`:**

| artifact | found as |
|---|---|
| `--stage-winner-name` | `--stage-winner-name:min(6.5vw, 11.5741vh)` inside `.stage-root` |
| `.stage-confetti-piece` | `{animation-name:stage-confetti-drift;animation-timing-function:linear;animation-iteration-count:infinite}` |
| `@keyframes stage-confetti-drift` | `0%{transform:translateY(-60vw)rotate(0)}to{transform:translateY(60vw)rotate(300deg)}` |
| `text-gold` / `bg-gold` / `bg-green-800` | all present (first use of the two gold utilities as Tailwind classes anywhere) |
| `font-display` | present |
| size utilities | `.h-\[0\.7vw\]`, `.w-\[0\.7vw\]`, `.h-\[0\.9vw\]`, `.h-\[1\.05vw\]`, `.h-\[1\.25vw\]`, `.w-\[0\.55vw\]`, `.h-\[0\.8vw\]`, `.rounded-\[0\.15vw\]`, `.leading-\[1\.1\]` |
| the ramp utility | `.text-\[length\:var\(--stage-winner-name\)\]{font-size:var(--stage-winner-name)}` |

The escaping trap 4.4 and 4.5 both hit bit again: a first pass grepping for `.text-\[length:var(--stage-winner-name)\]` and `.p-\[var(--stage-margin)\]` reported **both** missing, including the one every stage since 4.2 has used. Tailwind also escapes `:` and `(`/`)` — the real selectors are `.text-\[length\:var\(--stage-winner-name\)\]` and `.p-\[var\(--stage-margin\)\]`. **Note for the next story: the escaped form includes the colon and the parens, not just the brackets and dots.**

Also noted: Lightning CSS minifies the keyframe's `rotate(0deg)` to `rotate(0)`. Both ends stay plain interpolable values, so this is not the discrete-degradation trap `@keyframes stage-timer-deplete` documents — and the browser pass confirmed real movement rather than trusting the reading (below).

**5. The manual browser pass — 64 checks, 0 failures** (`playwright-core` + `channel: 'msedge'`, installed into the scratchpad, `web/package.json` untouched; `WHATSAPP_API_BASE_URL` overridden to `http://localhost:9099`, and the server log confirmed **zero** `graph.facebook.com` requests afterwards).

Three games were seeded through the **real** API and **real HMAC-signed webhooks** (never row inserts), so the winner rule, the rank assignment and the `role='player'` filter all mean something:

- **W — one winner:** `WINNER-ALPHA` 150 (rank 1), `RUNNERUP-BETA` 130 (rank 2), `LOSER-GAMMA` 0 (rank 3).
- **T — a genuine FOUR-way tie at rank 1**, all at 100 (speed bonuses zeroed while draft so the scores land identical), with long realistic names — this one game covers AC-3's tie, derived requirement 9's three-name cap, **and** the fit worst case.
- **N — no winner:** stopped from `question_open` before any Reveal, so both players sit at 0 and `RankLeaderboard` gives them **both rank 1** — precisely the input the strict positivity condition exists for.

Results by AC:

- **AC-1** — background `rgb(22, 101, 52)` (green-800); name `rgb(251, 191, 36)` (gold) at weight `900`; name font-size **124.8px at 1920×1080** and **83.2px at 1280×720**, both exactly `6.5vw`, token resolving `min(6.5vw, 11.5741vh)`; tagline `40.0013px` `rgba(255,255,255,0.7)`; score `48px`, same muted colour; safe margin `48px`. Score text `150 נקודות` — **raw digits, no separator**, and exactly one `<bdi>` in the document even with three names stacked.
- **Derived req. 13, both halves** — every gold **text** node is the `<h1>` or a descendant inheriting from it (two nodes, **one** authored declaration); the tagline, the score and the card background are all confirmed **not** gold; every gold **background** is a confetti piece with no text; the pieces resolve to exactly two colours, `rgb(251,191,36)` and `rgb(22,163,74)` — **13 gold and 9 green, matching the mockup's own split.**
- **AC-2** — 22 pieces, `aria-hidden`, all 22 carrying `stage-confetti-piece`, and a sampled piece's computed transform genuinely changed over 450ms (`matrix(-0.928…, 62.79)` → `matrix(-0.987…, 160.54)`), so the animation is really running and really interpolating.
- **AC-2 reduced motion, both channels independently** — the viewer's OS setting (`emulateMedia({reducedMotion:'reduce'})`) and the Organizer's room-level toggle (a real `PUT /display-settings`, OS setting left normal). Identical result on both: 22 pieces still present, **zero** carrying the animation class, transform unchanged across 450ms, and 22 **distinct** `left/top` coordinates — the mockup's static celebratory frame, not a blank screen. Name, score and tagline unaffected.
- **AC-3 tie** — exactly three names stacked in the order the server delivered (`TIE-Menachem Rosenberg`, `TIE-Yehoshua Friedman`, `TIE-Shlomo Zalman Katz`), the fourth tied winner absent, one score line.
- **AC-3 announcement** — the shell's assertive live region reads `המשחק הסתיים` at this stage. Verified, not built.
- **No-winner** — the fallback copy renders on the green-800 takeover with **zero** confetti, no `<h1>`, and neither zero-scorer named.
- **Fit** — no horizontal or vertical overflow at either resolution, with three long names. Content box `244–836px` inside 1080 (`592px` tall), and `155–565px` inside 720.
- **Privacy** — no runner-up name, no zero-scorer name, no phone number, and no UUID anywhere in the DOM.
- **Filter safety** — zero external-host requests, **zero image requests**, zero Fetch/XHR on the display route, zero console errors.

**6. Reconnect — 16 checks, 0 failures, with a REAL kill.** `ctx.setOffline` was not used (it does not close an established socket and the check would pass vacuously). The Go server was killed via `Stop-Process -Force` on the PID holding :8080, driven by a **bash orchestrator** reading a flag file, because Node's `child_process.spawn` cannot launch anything under this machine's Hebrew home directory. **`ws.on('close')` was asserted to have fired on the app's own `/ws?gameId=…` socket — filtered from Vite's HMR socket — before anything else was trusted.** Through the outage the card held completely: same name, same score, same tagline, same gold at the same size, all 22 pieces at byte-identical inline coordinates. After restart the band cleared and the card re-rendered **identical to before the outage with no interaction at all**.

**7. Reconnect-band note (Task 5's confirm-don't-measure item).** Measured incidentally: band box **48–124px**, this stage's content box starts at **381px** → **257px of clearance, nothing covered.** The band does land on green-800 again here, so this stage *would* have been the third data point for the legibility note — which 4.3 explicitly said is no longer needed to establish the pattern, so the contrast was **not** re-measured and nothing is claimed about it.

**8. Harness and data cleanup.** Scratch organizer `winner46org`, its three games and all their questions/participants/answers/sessions deleted from the dev DB (verified: a second cleanup run reports the organizer already gone). The `playwright-core` harness and the throwaway `cmd/cleanup46` program were both deleted; `git status` confirms **zero diff anywhere under `server/`**.

### Completion Notes List

**All three ACs and all eleven derived requirements are satisfied; every task and subtask is genuinely complete.**

- **The whole story turned on derived requirement 5, exactly as the Dev Notes predicted.** `finished` needs `questionCount === 0` **alone**, not `stage-props.ts`'s `isDegradedFrame`, because `FinishGame` resets `current_question_position` to 0 on every transition into the state, which makes `currentQuestion === null` unconditionally true on every real frame. The guard is written locally with a comment citing the reasoning, `isDegradedFrame` is **not imported**, and the wrong version was demonstrated red first (9 of 11 cases) rather than argued.
- **`[ASSUMPTION] — for Avraham's confirm-at-review:** `display.winner.scoreSuffix` (`"150 נקודות"`) and `display.winner.noWinner` (`"המשחק נגמר! הפעם לא נצברו נקודות."`). `display.winner.tagline` (`"מזל טוב!"`) is a direct transcription of the mockup's `w-tagline` and is **not** an assumption.
- **[ASSUMPTION] confirmed by derivation, flagged per this project's mockup-vs-code convention:** the score renders as **raw digits with no thousands separator**, following `messages_he.go`'s bare `strconv.Itoa`, `results-summary.tsx` and `leaderboard-stage.tsx` — **not** the mockup's illustrative `1,240`. Three existing sources agree with each other and only the mockup disagrees.
- **Three small, deliberate deviations from the story's own code sketches, each for a stated reason:**
  1. **`<span className="block">` for the stacked names, not `<div>`.** `<h1>`'s content model is phrasing content, so a `<div>` inside it is invalid markup. `block` gives the identical stacked layout.
  2. **A plain `<bdi>` around the score, not `<bdi dir="ltr">`.** The story's own subtask anticipated this ("if `scoreSuffix`'s Hebrew suffix breaks a single `<bdi>`…"). It does: the composed value is a Hebrew sentence, so forcing LTR would mirror it and put the digits on the wrong side of the word. `dir="auto"` resolves RTL from the first strong character while still isolating the run, and the digit run keeps its own LTR order. **Verified in the browser: the score reads `150 נקודות` correctly.**
  3. **`position: absolute` lives on the pieces as a Tailwind utility, not inside `.stage-confetti-piece`.** The story's CSS sketch bundled both — but that class is removed under reduced motion, so a bundled `position` would drop all 22 pieces back into normal flow at exactly the moment the spec asks for a static scattered frame. The CSS rule now carries the animation only, matching `.stage-row-reshuffle`'s shape, and both reduced-motion channels were verified to keep the scatter.
- **The 22 confetti pieces are transcribed verbatim** from `mockups/key-stage-winner.html`, including the negative delays that make the first painted frame read as already-scattered. `cqw` → `vw` is a pure rename at this project's 1920 design width, with no magnitude recomputed. The mockup's two class slots (`cf-circle` + `cf-sm/md/lg`, or the single-measure `cf-bar`/`cf-sq`) are collapsed into one exhaustive `shape` key, because only circles have a size variant and a separate optional `size` would need a fallback the type system could never discharge.
- **AC-3's `aria-live` clause needed no new code** — `display-page.tsx`'s existing assertive announcer fires `stateAnnouncement.finished` on entry, verified in the browser. A second announcer would fight the first (leaderboard-stage's precedent).
- **`deferred-work.md`: two entries triggered, three confirmed not triggered**, all recorded in the file itself. The 3.7 degraded-`Snapshot.Leaderboard` entry is **triggered and resolved with a *different* discriminator from 4.5's**, and re-pointed to a trigger that is likely dormant (no third consumer exists in Epic 4). The heading-semantics entry is **triggered and CLOSED** rather than deferred a fourth time — the winner name is an `<h1>`, and the epic's final accepted shape is recorded. Not triggered: the 2.5 spectator-roster entry, the timer clock-skew entry (its parenthetical guess that "4.6's winner" needs server time is **wrong**, and is now closed by inspection — this stage reads no clock at all), and the 4.5 enter/exit-animation entry (the confetti is a fixed set that loops in place; there is no membership change to animate).
- **Hebrew centralization:** the `.tsx` scan lists only the two known pre-existing violations (`scoring-editor.tsx`, `control-page.tsx`). The `.ts` scan lists `strings.he.ts` (the sanctioned home) and — worth flagging — **`web/src/lib/types.ts:73`, a pre-existing Hebrew fragment inside a doc comment** (`"undefined צדקו"`), confirmed untouched by this story (`git diff` against the baseline is empty). Not a regression here, but the story's Task 6 wording did not anticipate it. **No new file appears in either scan.**
- **Zero backend change**, as the scope boundary requires: `git status` shows no diff anywhere under `server/`. `go build ./... && go vet ./... && go test ./...` all green as a smoke check.
- **The six frozen display test files are byte-identical to the baseline** (`git diff 7168de8 --` on all six returns empty) and all their cases still pass. `display-page.test.tsx` gained exactly one case, as Task 3 permits.
- **Not claimed, per 4.5's honest accounting:** stories 3.10's, 4.1's and 4.2's manual browser passes remain outstanding. This pass did not put me in front of those surfaces either.

### File List

**New:**

- `web/src/features/display/winner-stage.tsx`
- `web/src/features/display/winner-stage.test.tsx`

**Modified:**

- `web/src/features/display/display-page.tsx`
- `web/src/features/display/display-page.test.tsx`
- `web/src/index.css`
- `web/src/lib/strings.he.ts`
- `_bmad-output/implementation-artifacts/deferred-work.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/4-6-winner-takeover.md` (this file)

### Change Log

| Date | Change |
|---|---|
| 2026-08-14 | Code review (bmad-code-review, three parallel adversarial layers). Both `decision-needed` items resolved by Avraham — the confetti's lost 16:9 invariant becomes a fit guard, the three `[ASSUMPTION]`s are confirmed as written, and the Hebrew singular at score 1 is deferred as a both-surfaces change. All eight patches applied in-session, five demonstrated red first: the confetti travel gains `min(60vw, 33.75vh)`; `--stage-winner-name`'s vh term corrected 11.5741 → 11.5556 (`6.5 × 16/9`, exact like every sibling step); the winner name gains `<bdi>` and `break-words`, matching the guards `leaderboard-stage.tsx` gives the identical field; `pointer-events-none` restored on the confetti overlay from the mockup; `leading-heading` (1.38) transcribed onto the tagline and score; `gap-6` → `gap-[1.25vw]` per derived requirement 11's straight rename; the `.stage-confetti-piece` layer comment corrected; and the unit suite closed eight surviving mutants and four vacuous assertions (106 → 112 tests). Two items deferred to `deferred-work.md`: the permanent degraded frame at terminal `finished`, and the missing Hebrew singular on both surfaces. Status → `done`. |
| 2026-08-14 | Story 4.6 implemented on `story/4-6-winner-takeover`, branched from `7168de8` (the tip of the unmerged `story/4-5-leaderboard-stage`, per the Baseline section's second branch). New `winner-stage.tsx` renders the full-screen green-800 takeover with the gold winner name, the score shown once, and 22 CSS-only confetti pieces killed from the `reducedMotion` prop. `finished` now maps to it in `stageByState`; `draft` is the last placeholder standing. One new ramp value (`--stage-winner-name`), one new component class, one new `@keyframes`, one new `winner` copy block. Eleven new Vitest cases plus one in `display-page.test.tsx`; the guard's wrong form was demonstrated red first. Two `deferred-work.md` entries triggered (one resolved with a new discriminator, one closed), three confirmed not triggered. Zero backend change. |
