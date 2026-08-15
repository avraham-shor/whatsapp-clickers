---
baseline_commit: bf0b93a
---

# Story 4.5: Leaderboard Stage — the Room Reshuffles

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As the room,
we want the leaderboard between questions with the movers highlighted,
so that the competition stays alive all game (FR-18, FR-10).

## Baseline: branch on top of story 4.4

**At story-creation time (2026-08-12) story 4.4 is `review` and its work is UNCOMMITTED** on branch `story/4-4-reveal-stage`, whose tip is still `3998281` (= `main`). This story therefore has no fixed baseline SHA yet. Resolve it before writing any code:

- **If 4.4 has merged to `main`** — branch from `main`, and confirm `web/src/features/display/reveal-stage.tsx` exists in the checkout.
- **If it has not** — branch from the tip of `story/4-4-reveal-stage` after 4.4's work is committed. Do **not** branch from `3998281`: `display-page.tsx`, `index.css`, `strings.he.ts` and `types.ts` are all touched by both stories, and a `main`-based branch would collide on all four (this is the same situation 4.2→4.3 hit, and it was resolved the same way).
- Either way, **record the actual baseline SHA in the Dev Agent Record and correct this file's frontmatter.**

This story consumes 4.1–4.4's shapes. Verify each on disk before writing code:

- `web/src/features/display/display-page.tsx` — the shell. `stageByState.leaderboard` is still `StagePlaceholder`; the `answeredFloor` render-phase-state block is the pattern Task 10 follows; `retained`, `stageSnapshot`, `usePrefersReducedMotion`, the `${gameId}:${state}` key, the assertive announcer and the reconnect band are all 4.1–4.3 review decisions.
- `web/src/features/display/stage-props.ts` — `StageProps { snapshot: LobbySnapshot; reducedMotion: boolean }`. Task 8 adds **one optional field**.
- `web/src/features/display/reveal-stage.tsx` and `lobby-stage.tsx` — the two full-bleed stage shapes. The reveal stage's guard-above-every-hook, `<bdi>` discipline and prop-driven motion kill are the patterns this stage repeats.
- `web/src/index.css` — `.stage-root` (unlayered) holds ten `--stage-*` ramp values; `@layer components` holds `.stage-fade`, `.stage-timer-sweep`, `.stage-bar-grow`; three `@keyframes` sit outside the layer.
- `web/src/lib/strings.he.ts` — `live` (CTA copy) and `display` (with `lobby`, `question`, `reveal` sub-blocks).
- `server/internal/game/state.go`, `engine.go`, `httpapi/control.go`, `router.go`, `store/queries/games.sql` — the five existing state transitions. This story adds the sixth.
- **The existing display test files are a regression surface, not a frozen gate.** 4.4's empty-diff gate was specific to 4.4's two extractions. This story changes shell behaviour, so `display-page.test.tsx` **will** grow new cases; `lobby-stage.test.tsx`, `question-stage.test.tsx`, `timer-ring.test.tsx` and `reveal-stage.test.tsx` must all still pass **unedited** — if one needs editing, you changed something you should not have.

## Acceptance Criteria

1. **Given** the Organizer advances to `leaderboard`, **then** the display shows the top 10 (or fewer) as rank / name / score rows (tabular-nums, `leaderboard-row` spec — UX-DR7) with shared ranks rendered correctly, **and** movement indicators highlight climbers since the previous Leaderboard — ▲ + places climbed beside the rank, indicator ≥40px (A11), static reordering under reduced motion. *(epic AC-1)*
2. **Given** the Organizer skips the Leaderboard (UJ-4), **then** the display transitions directly from reveal to the next question — the stage renders only when entered. *(epic AC-2)*

### Derived requirements — binding, and each has a source

These are not extra scope; they are what the epic ACs resolve to against the code that exists. Each is cited so you can check it yourself.

3. **A game cannot currently enter `leaderboard` at all, and building that path is this story's job.** `game/state.go` defines `StateLeaderboard`, the DB `CHECK` allows it, `participants.go` lets a late joiner become a Spectator during it, `display-page.tsx` maps it and `strings.display.stateAnnouncement` names it — but **nothing writes it**: `grep -rn "leaderboard" server/internal --include=*.sql` matches only `GetLeaderboard`'s `SELECT`, and `router.go:86-91` exposes exactly six transition routes — `open-lobby`, `start`, `close-question`, `reveal`, `next-question`, `stop` — with no seventh. Story 3.1 did this deliberately and said so in as many words: *"This story therefore implements `revealed → question_open` (or `→ finished`) as the only path out of `revealed`, and never implements a control that enters `leaderboard`… has zero reachable code path until Epic 4 Story 4.5"* — and flagged it: *"better to confirm before 4.5, not after."* Epic AC-1's "**Given** the Organizer advances to `leaderboard`" has nowhere to hook without it. **This story is therefore backend + dashboard + display, not display-only.**

4. **The transition is `revealed → leaderboard`, guarded in SQL, and it moves nothing else.** A new `ShowLeaderboard` query in the established shape of every other transition in `games.sql`: guarded `WHERE … AND state = 'revealed'`, zero rows → `store.ErrNotFound` → the engine's existing `ErrNotRevealed` → the existing 409 `GAME_NOT_REVEALED` (`httpapi/errors.go:61`). **`current_question_position` must NOT move**: the Leaderboard is a pause on the question just revealed, `buildSnapshot` still has to resolve a `CurrentQuestion`, and derived requirement 7 keys the display's movement baseline on that question's id. No new error type — `ErrNotRevealed` is exactly this precondition.

5. **`leaderboard` must be an exit, not a trap — four guards widen, and missing any one of them bricks a live game.** Today `OpenNextQuestion` is `WHERE … g.state = 'revealed'` (`games.sql:125`), `FinishGame` is `WHERE … state IN ('question_open','question_closed','revealed')` (`games.sql:140`), `Engine.NextQuestion` rejects anything but `StateRevealed` (`engine.go:403`) and `Engine.StopGame`'s switch lists three states (`engine.go:446`). An Organizer who reaches the Leaderboard with only the entry built would find **every** exit returning 409 in front of a room, with no recovery short of a DB edit. All four widen to include `leaderboard`, and Task 3's tests assert each one individually — a single "it works" happy-path test would pass with three of the four still broken.

6. **The previous ranking is client-side memory owned by the shell, not a new wire field.** Four reasons, in order of weight: (a) the epic says "since the previous **Leaderboard**" — the leaderboard *as shown to this room*, which only the display knows; a server-side "previous" could only mean "before the current question", which diverges the moment the Organizer skips a Leaderboard (UJ-4, and AC-2 says they will). (b) The reshuffle animation this story is named for has to start from the order the room last saw; nothing on the server knows that. (c) The player set is **fixed** for the whole game — `GetLeaderboard` filters `role = 'player'` and `participants.go:146` only ever creates Spectators past `lobby` — so a stored ranking never goes stale against a changed roster. (d) The precedent is already in this file: `display-page.tsx`'s `answeredFloor` is cross-stage memory living in the shell **because the stage remounts** on the `${gameId}:${state}` key. **The limit, stated rather than hidden:** a display reloaded or opened mid-game has no baseline and shows no ▲ on the first Leaderboard it sees, which looks exactly like "nobody climbed". That is a real weakness of the cheap option and is flagged for Avraham under "Open questions" — do not silently fix it by inventing a wire field.

7. **The baseline is captured once per showing, keyed on `currentQuestion.id`.** `current_question_position` does not move until `NextQuestion` (derived req. 4), so the question id uniquely identifies one Leaderboard showing and is stable across every frame within it — a display-settings toggle mid-Leaderboard must not re-capture the baseline and erase the arrows. The baseline holds **rank and slot for every entry, not just the visible ten**, so a climb from 14th to 8th is computed correctly and a row entering the top ten knows where it came from.

8. **A degraded `leaderboard` frame must be distinguishable from a real one — and it already is, without a wire change.** This is `deferred-work.md`'s 3.7 entry, whose named trigger is *this file*: *"`leaderboard-stage.tsx` is the first surface that actually renders this, and is where 'no data' vs 'all zeros' first becomes visible to a room"*, and 4.1's follow-up: *"Story 4.5 must resolve this before rendering a leaderboard to a room."* The entry concluded a fix would be a wire-contract change (a `null`, or a `degraded: true` flag) spanning Go, TS and Epic 4. **It does not have to be**, and that is this story's resolution: `emptySnapshot` (`engine.go:662`) sets `Leaderboard: []` **together with** `ParticipantCount: 0`, `QuestionCount: 0` and `CurrentQuestion: nil`, while a legitimate `leaderboard` frame always has `questionCount ≥ 1` and a non-nil `currentQuestion` (the state is only reachable from `revealed`, which requires an open question). The discriminator is a **correlation across fields already on the wire**. So: `questionCount === 0 || currentQuestion === null` at `leaderboard` → render `strings.display.waiting`, never a board. `leaderboard.length === 0` on an otherwise-healthy frame is the real "no players registered" case and gets its own honest copy. Both are Vitest-pinned, and Task 11 records the entry as **resolved for the display half** with the remaining wire ambiguity re-deferred.

9. **No high-water guard — and `deferred-work.md`'s 2.4 entry fires here, by name.** Its trigger is *"the first snapshot field rendered on the Audience Display that can legitimately decrease — 4.3's answered count cannot; **4.5's leaderboard can**, and is the likely one"*, and 4.2's own note says copying its guard *"into any later stage where a snapshot field can legitimately decrease… would be a real bug"*. So: **this stage applies no monotonic guard of any kind**, and Task 11 records that as a decision. Record the analysis too, because it is what makes the disposition honest: the concurrent-writer burst that produced 4.2's measurement (20 simultaneous inbound joins) **has no analogue at `leaderboard`** — the answer cutoff has passed, no inbound message writes anything, and the only writers are the Organizer's own serialized clicks plus the display-settings toggle. The reachable race is therefore the same press-then-tick window the 3.1 `seq` entry already owns, whose fix is a `ws`/`control.go` redesign. Re-defer; do not re-derive 4.2's measurement.

10. **Ranks come from the server; the client never sorts and never re-ranks.** `game.RankLeaderboard` (`scoring.go:79`) already sorts descending and assigns standard competition ranks — *"equal Score shares one Rank, and the next distinct score's Rank skips ahead by the number of tied rows (1,1,3 — never 1,1,2)"* — with ties broken stably by `joined_at`. AC-1's "shared ranks rendered correctly" is satisfied by rendering `entry.rank` verbatim in the order received and slicing the first ten. A client-side sort would be a second ranking implementation that can silently disagree with `final.go`'s winner computation, which reads the same entries. **A tie straddling the cut is cut**: a plain `slice(0, 10)` can show one member of a tied pair and not the other. Accepted rather than solved — A11 fixes the depth at ten, extending the list for a boundary tie would blow the vertical budget the row height is sized against, and the alternative (dropping both) hides a real top-ten finisher. Record it; do not invent a rule.

11. **The reshuffle is arithmetic, not measurement.** Every row is exactly `--stage-leaderboard-row` tall, so a row's travel is a whole number of row heights, and `translateY(calc(var(--stage-row-shift) * 100%))` expresses it exactly — **no FLIP, no `getBoundingClientRect`, no ref, no layout read**, and nothing a jsdom test cannot observe (`react-hooks/refs` forbids reading a ref during render anyway). This is why the row height is fixed and the display name is single-line and truncated: a wrapping name would make one row taller and every shift below it wrong by the difference.

12. **The animation is killed from the `reducedMotion` PROP, not from a media query.** Third story in a row with the same requirement and the same reasoning (4.3's derived req. 9, 4.4's derived req. 11): a CSS-only kill is invisible to jsdom so it cannot be asserted, and the Organizer's room-level toggle is not a media query at all. EXPERIENCE.md names the reshuffle explicitly in the reduced-motion list: *"The reshuffle animation is explicitly in the reduced-motion list — static reordering when reduced."*

13. **Mover rows are `green-50`, which is the shell's own ground colour — so this stage must paint a white full-bleed body.** `DESIGN.md components.leaderboard-row-mover.background` is `{colors.green-50}` = `#F0FDF4` = `--color-surface-base`, and `display-page.tsx`'s `stageRootClass` is `bg-surface-base`. On the shell's ground the one highlight the spec asks for would be **invisible**. Same class of finding as 4.2's counter pill (green-50 on a green-50 shell, fixed by painting the lobby's own green-800 ground). The body is `surface-raised` (white), full-bleed, matching split-hero's "white below = the thinking domain". **This stage has no hero band** — `split-hero` is specified "game stages only" and there is no timer, no question and no progress here.

14. **This is the one stage where per-Participant data is specified, and that is not a contradiction of 4.4's AC-3.** 4.4's epic AC-3 forbade individual data on the *reveal*; the leaderboard's entire content is names and scores (`DESIGN.md leaderboard-row`, EXPERIENCE.md's IA table). What stays forbidden here: **grades** (which live only in each Participant's WhatsApp — FR-10), phone numbers, who answered what, and *"No current-user highlighting — it's a shared screen"* (EXPERIENCE.md). The stage reads `snapshot.leaderboard` and must not read `snapshot.participants`.

15. **Both non-colour signals are accessible, and the ▲ is not colour-alone.** UX-DR14, verbatim: *"colour is never the sole signal (✓/✗ icons alongside success/error fills, **aria-labeled**)"*. The glyph is the non-colour signal; it is `aria-hidden` and carries an `sr-only` name beside it, exactly as 4.4's ✓ does. Two contrast figures this story is the first to rely on, computed because DESIGN.md's table does not list them: **green-600 `#16A34A` on white = 3.30:1** and **on green-50 = 3.15:1** — both pass the ≥3:1 large-text/non-text floor at 48px and **neither would pass 4.5:1**, so the indicator must never drop below the large-text size. `--stage-ui` (48px at 1920) satisfies A11's ≥40px.

## Scope boundaries for this story

- **Backend changes are in scope, and are exactly these files**: `store/queries/games.sql`, `store/games.go`, the regenerated `store/gen/*`, `game/engine.go`, `httpapi/control.go`, `httpapi/router.go`, and the Go tests for them. **A diff anywhere else under `server/` means you went off-spec.**
- **No migration.** `games.state`'s CHECK already allows `'leaderboard'` (migration 00003/00008). `server/migrations/` must show no diff.
- **`sqlc generate` must be run and its output committed** — CI has a diff-check step that fails otherwise. Version **1.31.1**, matching CI.
- **No change to `server/internal/ws/**`, `server/internal/wa/**`, `grading/**`, `game/{scoring,answers,results,final,state,participants,snapshot}.go`.** In particular: **`Snapshot` gains no field** (derived req. 6), `RankLeaderboard` is not touched (derived req. 10), and there is **no WhatsApp traffic at `leaderboard`** — EXPERIENCE.md's State Patterns table gives that row "— (quiet)", and its inbound grammar already covers the state under "Between questions²". Verify that rather than changing it (Task 4).
- **`GetLeaderboard` is not modified and no second leaderboard read is added.** It already runs on every `buildSnapshot`; the previous ranking is client-side (derived req. 6).
- **One entry in `stageByState` changes: `leaderboard`.** `draft` and `finished` keep `StagePlaceholder` — `finished` is 4.6's. Do not "while we're here" either.
- **No gold anywhere on this stage.** UX-DR2 rations gold to 4.3's ≤5s ring and 4.6's winner. A leaderboard is neither, and this one is especially tempting.
- **No winner treatment.** Rank 1 gets no crown, no gold, no larger type — that is 4.6's moment and spending it here would flatten it. Rank 1 is a row like the others.
- **No current-participant highlighting, no per-row grade, no phone number, no `snapshot.participants` read** (derived req. 14).
- **No new npm package, no `shadcn add`, no `components/ui/*` edit.**
- **No error-banner redesign on the control panel.** `deferred-work.md`'s 3.7-round entry about the multiplexed banner is explicitly *not* triggered by adding a second path to the same mutation — say so in Task 11 rather than fixing it.

## Tasks / Subtasks

- [x] **Task 1: `server/internal/store/queries/games.sql` — one new transition, two widened guards** (AC: 1; derived reqs. 4, 5)

  - [x] Append after `RevealCurrentQuestion` (so the file reads in state order):
    ```sql
    -- The Leaderboard pause (FR-13, FR-18, story 4.5) — the last of the
    -- live-game transitions, and the one story 3.1 deliberately left out:
    -- nothing could render a leaderboard until the Audience Display
    -- existed, and a control with nothing to show it on is speculative
    -- work.
    --
    -- current_question_position deliberately does NOT move. The Leaderboard
    -- is a pause on the question just revealed, not a step past it:
    -- buildSnapshot still has to resolve a CurrentQuestion here, and the
    -- display keys its movement baseline on that question's id (story 4.5,
    -- derived requirement 7).
    --
    -- Guarded on state = 'revealed' like every other transition in this
    -- file, so a lost race returns zero rows -> store.ErrNotFound -> the
    -- engine's existing ErrNotRevealed -> the 409 the organizer already
    -- gets from next-question. No new error type: this IS that precondition.
    -- name: ShowLeaderboard :one
    UPDATE games
    SET state = 'leaderboard',
        updated_at = now()
    WHERE id = $1 AND organizer_id = $2 AND state = 'revealed'
    RETURNING *;
    ```
  - [x] **Widen `OpenNextQuestion`**: `AND g.state = 'revealed'` → `AND g.state IN ('revealed', 'leaderboard')`, and extend its header comment (currently *"guarded from 'revealed' instead of 'lobby'"*) to say both, with the reason: the Leaderboard is skippable, so the next question opens from either side of it.
  - [x] **Widen `FinishGame`**: add `'leaderboard'` to the `state IN (…)` list, and extend its comment — it serves `NextQuestion`'s no-more-questions path **and** `StopGame`, so both need the Leaderboard to be a stoppable state.
  - [x] **Nothing else in this file changes.** Not `CloseCurrentQuestion`, not `RevealCurrentQuestion` (its ungraded-answers `NOT EXISTS` guard is 3.4/3.6's and stays), not `StartGameFirstQuestion`.
  - [x] Run `sqlc generate` (v1.31.1) and commit `server/internal/store/gen/`. **Do not hand-edit anything under `gen/`.**

- [x] **Task 2: `server/internal/store/games.go` — one wrapper** (AC: 1; derived req. 4)

  - [x] Add `ShowLeaderboard(ctx, gameID, organizerID string) (gen.Game, error)` in the file's established style, next to the other transition wrappers. **Copy the neighbouring transition's exact shape**, including its `pgx.ErrNoRows → store.ErrNotFound` remap — a guarded `:one` UPDATE returns no rows when the guard fails, and that remap is what turns a lost race into a 409 instead of a 500.
  - [x] **Nothing else in this file changes.**

- [x] **Task 3: `server/internal/game/engine.go` — the missing transition and two widened guards** (AC: 1, 2; derived reqs. 4, 5)

  - [x] `Store` interface: add `ShowLeaderboard(ctx context.Context, gameID, organizerID string) (gen.Game, error)`, beside the other transition methods. Match the generated signature exactly — read `gen/games.sql.go` after Task 1 rather than trusting this line.
  - [x] Add `func (e *Engine) ShowLeaderboard(ctx context.Context, gameID, organizerID string) (Snapshot, error)`. **Mirror `CloseQuestion`/`NextQuestion` exactly** — read them and copy the shape rather than inventing one: `GetGameForOrganizer` → `if g.State != StateRevealed { return Snapshot{}, ErrNotRevealed }` → the guarded write → `errors.Is(err, store.ErrNotFound)` reinterpreted as `ErrNotRevealed` (the race-loss path every transition here has) → `return e.snapshotAfterCommit(ctx, g), nil`. The read-then-write is not redundant: it is this file's documented "read for message accuracy, write-guard for the race" discipline.
  - [x] **Widen `NextQuestion`** (`engine.go:403`): `if g.State != StateRevealed` → reject anything that is neither `StateRevealed` nor `StateLeaderboard`, keeping `ErrNotRevealed` as the error. Update the comment on the race-loss reinterpretation, which currently names `state = 'revealed'` as *the* guard.
  - [x] **Widen `StopGame`** (`engine.go:446`): add `StateLeaderboard` to the `switch`, and update the doc comment, which enumerates the three stoppable states in prose.
  - [x] **Nothing else in `engine.go` changes.** Not `buildSnapshot`, not `buildQuestionReveal`, not `snapshotAfterCommit`, not `emptySnapshot`, not `Reveal`.
  - [x] **Go tests** in `engine_test.go`, extending `stubStore` with `ShowLeaderboard` (default: returns the game with `State` set, so existing tests compile unchanged):
    - `ShowLeaderboard` from `revealed` returns a snapshot whose `State == "leaderboard"` and whose `CurrentQuestion` is **still the revealed question, at the same position** (derived req. 4 — this is what the display's baseline key depends on);
    - `ShowLeaderboard` from each of `question_open`, `question_closed`, `leaderboard`, `finished` returns `ErrNotRevealed` **without calling the store write** — a table test, and assert the call counter, not only the error;
    - a `store.ErrNotFound` from the write is reinterpreted as `ErrNotRevealed`;
    - **`NextQuestion` from `leaderboard` opens the next question**, and **from `leaderboard` on the last question finishes the game** — two cases, because they take different store writes;
    - **`StopGame` from `leaderboard` finishes the game**;
    - the negative control that makes the above meaningful: `NextQuestion` and `StopGame` still reject `question_open`.
    - **Demonstrate at least one red first** — remove `StateLeaderboard` from `NextQuestion`'s guard and record the failure message, as 3.11, 4.2, 4.3 and 4.4 all did.

- [x] **Task 4: `server/internal/httpapi/control.go` + `router.go` — the route** (AC: 1; derived reqs. 4, 5)

  - [x] `ControlEngine` interface: add `ShowLeaderboard(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)`, after `Reveal`.
  - [x] Add `handleShowLeaderboard(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc`. **Mirror `handleCloseQuestion`** — organizer + gameID guards, 5s context, engine call, `writeStoreError(w, err, "GAME_NOT_FOUND")`, `hub.Broadcast`, `slog.Info`, `writeJSON`. **No dispatcher argument and no `go dispatch…` call**: EXPERIENCE.md's State Patterns table gives the Leaderboard row "— (quiet)" for WhatsApp. Say so in a comment, because every other transition in this file has one and its absence reads like an omission.
  - [x] `router.go`: `gr.Post("/show-leaderboard", handleShowLeaderboard(engine, hub))`, inside the existing `if engine != nil && hub != nil` guard beside the other five transition routes (`router.go:86-91`). Path is `show-leaderboard` — verb-phrase, matching `close-question`/`next-question`.
  - [x] **Verify, do not change, the WhatsApp inbound path.** EXPERIENCE.md's conversation grammar covers `leaderboard` under "Between questions²" (footnote 2 names the state), and `game/participants.go:146` already lists it in `spectatorAllowedStates`. Confirm by reading `wa/inbound.go` that no branch keys on the specific state strings, so an answer or a JOIN arriving during the Leaderboard already gets the right reply. **Record the result in the Dev Agent Record.** If it turns out a branch does need `leaderboard`, that is a scope surprise — stop and flag it rather than widening this story quietly.
  - [x] **Tests** in `control_test.go` and `router_test.go`, following the existing cases for `reveal`/`next-question`: the route exists and is organizer-scoped; a successful call broadcasts once and returns the snapshot; an engine `ErrNotRevealed` becomes **409 `GAME_NOT_REVEALED`**; an unauthenticated call is rejected by the same middleware as its neighbours.
  - [x] **Nothing else in these two files changes.**

- [x] **Task 5: `web/src/lib/strings.he.ts` — two CTA strings and the leaderboard copy** (AC: 1)

  - [x] Append to the existing `live` block, after `stopCta`:
    ```ts
    // [ASSUMPTION]: EXPERIENCE.md's State Patterns table gives the Reveal
    // row's host options as "Next / Leaderboard", but the Host-microcopy
    // table has no row for a Leaderboard CTA, so the wording is authored
    // here. 'טבלת התוצאות' matches display.stateAnnouncement.leaderboard
    // and results.leaderboardTitle character for character, so all three
    // surfaces name one thing one way.
    showLeaderboardCta: 'טבלת התוצאות',
    // EXPERIENCE.md's Host-control-panel table, "Between questions" row:
    // "פתח שאלה [N]" — the only CTA in that table carrying a number, and
    // the reason the leaderboard state cannot reuse the static
    // primaryActionByState map (see control-page.tsx).
    openQuestionNumberCta: (n: number) => `פתח שאלה ${n}`,
    ```
  - [x] Append a `leaderboard` sub-block **inside the existing `display` block**, after `reveal`:
    ```ts
    // Leaderboard stage (story 4.5).
    leaderboard: {
      // DESIGN.md leaderboard-row-mover: "▲ + places climbed". One
      // constant so the glyph cannot drift, the same discipline as
      // reveal.correctMark.
      moverMark: '▲',
      // [ASSUMPTION]: UX-DR14 requires the indicator to be aria-labeled
      // but gives no wording. Noun form so there is no gendered verb
      // (A2: 'עלה' is masculine); the singular is spelled out for the
      // same reason results.playerCountLabel spells out 'משתתף אחד'.
      climbedLabel: (places: number) =>
        places === 1 ? 'עלייה של מקום אחד' : `עלייה של ${places} מקומות`,
      // [ASSUMPTION]: reachable — a game can be started and played with
      // no registered players (nothing gates start on a roster), and
      // GetLeaderboard then returns zero rows. Distinct from the degraded
      // frame, which renders display.waiting instead (story 4.5, derived
      // requirement 8). Impersonal, matching results.noPlayers rather
      // than restating it: that one is a dashboard empty state and a
      // later copy change to either must not move the other.
      noPlayers: 'אף אחד לא נרשם למשחק הזה.',
    },
    ```
  - [x] **`display.stateAnnouncement.leaderboard` already exists** (`'טבלת התוצאות'`, added at 4.1) and the shell already announces it assertively on entry. **Do not add a second announcement string and do not change this one** — the new `live.showLeaderboardCta` is deliberately the same words, so the Organizer's button and the room's announcement name one thing one way.
  - [x] **Nothing else in `strings.he.ts` changes.** Leave every existing `[ASSUMPTION]` marker alone — they are 4.1–4.4's records to correct.
  - [x] Flag the three new `[ASSUMPTION]` items in the Dev Agent Record for the same confirm-at-review treatment 4.1–4.4's copy got.
  - [x] **Zero Hebrew literals outside this file.** The `▲` glyph is a symbol, not copy, and lives here anyway so the two places that could render it cannot drift.

- [x] **Task 6: `web/src/features/live/control-page.tsx` — the entry CTA and the Between-questions state** (AC: 1, 2)

  - [x] **A secondary "טבלת התוצאות" CTA, rendered only at `revealed`**, in the existing `flex items-center gap-3` row between the primary button and "עצור". It fires through the **same `action` mutation and the same `fire()`** as the primary (`fire('show-leaderboard')`), so it inherits 3.11's single-flight guard and the existing error banner with no new state. `variant="outline" className="h-10 border-host-border text-green-800"` — the "עצור" button's shape without its error colour.
    - [x] **Do not** add it to `useSpaceAction`. Space stays bound to the primary CTA only (UX-DR10: exactly one primary action per state), and binding a second key would be new interaction design.
    - [x] The primary CTA at `revealed` is **unchanged** — "שאלה הבאה ←" is the UJ-4 skip, exactly as story 3.1 resolved it. AC-2 depends on that staying true.
  - [x] **The primary CTA at `leaderboard`.** `primaryActionByState` is a static `Record` and cannot interpolate the question number, so compute the primary once:
    ```tsx
    // EXPERIENCE.md's Host-control-panel table calls this state "Between
    // questions" and gives it "פתח שאלה [N]" — the one CTA in that table
    // carrying a number, which is why this state cannot live in the
    // static map above. Same POST as `revealed`'s skip: next-question
    // opens position+1, or finishes the game when there is no such
    // question (game.Engine.NextQuestion owns that fork). On the LAST
    // question there is no [N] to name, so the label falls back to
    // "שאלה הבאה ←" — the identical button, doing the identical thing it
    // already does at `revealed`.
    ```
    with `nextPosition = (snapshot.currentQuestion?.position ?? 0) + 1` and the numbered label used only while `nextPosition <= snapshot.questionCount`.
  - [x] Add `'leaderboard'` to `stoppableStates` — EXPERIENCE.md's "Between questions" row gives "עצור" as its secondary CTA, and derived req. 5 makes the backend accept it.
  - [x] Update the stale comment above `primaryActionByState`, which says *"this story never builds a control that enters the leaderboard state"*. That sentence is about to become false and leaving it would mislead the next reader.
  - [x] **Nothing else in this file changes.** Not the `finished` branch, not the `ResultsSummary` composition, not the polite live region, not the latest-ref error-reset effect, not `DisplayControls`, and not the never-disabled primary button (3.11 and 4.1's review decisions, each with a measured reason).

- [x] **Task 7: `web/src/index.css` — one ramp value, one animation** (AC: 1; derived reqs. 11, 12)

  - [x] Append **inside the existing `.stage-root` block**, after `--stage-bar-label`:
    ```css
      /* DESIGN.md components.leaderboard-row minHeight: 72px. 3.75vw = 72px
         at 1920; the 6.6667vh term is the same fit guard --stage-option-min
         and --stage-timer-ring carry - equal at 16:9, so every measured
         value at both verified resolutions is unchanged and min() only
         engages on a viewport shorter than 16:9.

         Used as an EXACT height, not a minimum, and that is load bearing
         rather than stylistic: the reshuffle below translates a row by a
         whole number of row heights (story 4.5, derived requirement 11),
         which is only exact while every row is the same height. It is also
         why the display name is single-line and truncated.

         Fit: ten rows = 720px inside a 984px content box at 1920x1080
         (1080 minus two --stage-margin), and 480px inside 656px at
         1280x720. If the board looks lost on the screen, raising this
         toward 5vw (96px, the --stage-option-min step) is the sanctioned
         adjustment - the 1080p headroom allows up to ~98px rows - and it
         must be re-measured at BOTH resolutions, because 720p is the
         binding one (see the --stage-bar-label note above). */
      --stage-leaderboard-row: min(3.75vw, 6.6667vh);
    ```
  - [x] Append **inside the existing `@layer components` block**, after `.stage-bar-grow`:
    ```css
      /* The Leaderboard reshuffle (EXPERIENCE.md Audience Display stages:
         "The reshuffle animation is explicitly in the reduced-motion list -
         static reordering when reduced").

         Every row is exactly --stage-leaderboard-row tall, so a row's travel
         is a whole number of ROW HEIGHTS and translateY(N * 100%) expresses
         it exactly - no FLIP measurement, no getBoundingClientRect, no ref
         (react-hooks/refs forbids reading one during render anyway), and
         nothing a jsdom test cannot see. --stage-row-shift is set inline per
         row as a unitless row count; POSITIVE means the row starts BELOW its
         final slot, i.e. it climbed.

         No transform-origin: this is a translate, not a scale. Unlike
         .stage-bar-grow above, where the origin IS the direction.

         The 300ms DELAY is the point of the stage, not padding: the board
         arrives holding the order the room last saw, the shell's 300ms
         cross-fade finishes, and only THEN does it reshuffle. Without it
         the movement happens underneath the fade and the room never sees
         where anyone came from - which is the whole spectacle EXPERIENCE.md
         calls "the reshuffle". 300ms + 600ms = 900ms, inside NFR-1's 1s
         transition budget.

         fill-mode `backwards`, NOT `both`, and here it is load bearing
         rather than tidy: `backwards` is what holds the `from` transform
         through the delay. Without it the rows would paint in their FINAL
         order for 300ms and then jump backwards to start - the animation
         playing in reverse, visibly. `both` would additionally pin a
         transform on every row forever, making each one a containing block
         for anything positioned inside it later (deferred-work.md carries
         an open note about .stage-bar-grow's `both` for exactly that - do
         not "make them consistent" by copying the older one).

         Layered for the same reason .stage-fade / .stage-timer-sweep /
         .stage-bar-grow are (code review, 2026-08-09).

         Deliberately NO prefers-reduced-motion rule: killed from the
         reducedMotion PROP (story 4.5's derived requirement 12) so a unit
         test can see it and so the Organizer's room-level toggle - which is
         not a media query at all - works. */
      .stage-row-reshuffle {
        animation: stage-row-reshuffle 600ms ease-out 300ms backwards;
      }
    ```
  - [x] Append after `@keyframes stage-bar-grow`, outside the layer:
    ```css
    /* Only a `from`: the implicit `to` is the row's own position, so it
       settles in its real slot with nothing to keep in sync - the same shape
       as stage-bar-grow. The fallback in var() matters: a row with no shift
       renders the class not at all, but a future caller that forgets the
       inline property gets a no-op instead of an invalid transform that
       drops the whole animation. */
    @keyframes stage-row-reshuffle {
      from { transform: translateY(calc(var(--stage-row-shift, 0) * 100%)); }
    }
    ```
  - [x] **[ASSUMPTION] the 300ms delay and the 600ms duration.** EXPERIENCE.md and DESIGN.md give no figures for the reshuffle (unlike the bar's "~400ms" and the cross-fade's "≤300ms"). The delay is derived — it matches `.stage-fade`'s duration exactly, so the two never overlap — and 600ms is chosen so the movement is legible from the back of a room while 300 + 600 stays inside NFR-1's 1s budget. Flag both in the Dev Agent Record, and confirm in the browser pass that the board really is readable in its old order before it moves (check 5).
  - [x] **Nothing else in `index.css` changes.** Not the `@theme` blocks, not the shadcn variables, not `@layer base`, not the ten existing ramp values, not `.stage-fade` / `.stage-timer-sweep` / `.stage-bar-grow`, not the three existing `@keyframes`, and not the long `vw`-vs-`rem` deviation comment.
  - [x] **No new `@theme` colour token.** `green-50`, `green-600`, `green-800`, `border-light`, `text-primary`, `text-secondary`, `surface-raised` all already exist. `bg-green-50` and `text-green-800` already ship (lobby stage, control page); `text-green-600` and `border-border-light` are new *utilities* over existing tokens, so verify them in the built CSS per Task 12.

- [x] **Task 8: `web/src/features/display/stage-props.ts` — one optional field** (AC: 1; derived reqs. 6, 7)

  - [x] Add the standings type and the field:
    ```ts
    /** One row of the previous Leaderboard: the rank it held, and the slot
     *  it occupied in the order the room actually saw. Rank drives the ▲
     *  count (AC-1's "places climbed"); index drives the reshuffle, because
     *  with shared ranks the two are different numbers — ranks 1,1,3 sit in
     *  slots 0,1,2. */
    export interface PreviousStanding {
      rank: number
      index: number
    }

    /** Keyed by participantId, covering the WHOLE previous leaderboard and
     *  not only its visible top ten (story 4.5, derived requirement 7). */
    export type PreviousStandings = Readonly<Record<string, PreviousStanding>>
    ```
    and on `StageProps`:
    ```ts
      /** The ranking this window showed at the PREVIOUS Leaderboard for this
       *  game, or undefined when it has shown none — a freshly opened or
       *  reloaded display, and the first Leaderboard of every game.
       *
       *  Cross-stage memory, so it cannot live in the stage: the shell keys
       *  each stage on `${gameId}:${state}` and remounts on every
       *  transition. The same reason display-page.tsx owns answeredFloor.
       *  Optional because exactly one stage consumes it; the four others
       *  ignore it. */
      previousStandings?: PreviousStandings
    ```
  - [x] **Nothing else in this file changes**, and in particular its header comment about the module cycle stays — that comment is why the file exists.

- [x] **Task 9: `web/src/features/display/use-leaderboard-memory.ts` (new) — the shell's memory** (AC: 1; derived reqs. 6, 7, 8)

  Its own module, not inline in `display-page.tsx` and not exported from the stage: `react-refresh/only-export-components` rejects a non-component value export from a component module (which is why `StageOptionVariant` is a *type*-only export), and a plain `.ts` hook module is also directly unit-testable.

  - [x] Signature: `export function useLeaderboardMemory(gameId: string, snapshot: LobbySnapshot | null): PreviousStandings | undefined`.
  - [x] The showing is identified by `snapshot.state === 'leaderboard' && snapshot.currentQuestion !== null` → `currentQuestion.id`. **A frame with a null `currentQuestion` never captures a baseline** — that is the degraded `emptySnapshot` (derived req. 8), and capturing an empty board as "what the room saw" would poison the next showing's arrows.
  - [x] One piece of render-phase state, in the shape `display-page.tsx` already uses and documents (`{ gameId, questionId, current, previous }`). On a frame whose showing key differs from the stored one: `previous` ← the stored `current` (only when `gameId` matches), `current` ← the standings of this frame, `questionId` ← the new key. On every other frame: **no write at all**, so a display-settings toggle mid-Leaderboard cannot erase the arrows.
    - [x] Not a ref and not an effect — `react-hooks/refs` and `react-hooks/set-state-in-effect` both forbid it here. React re-runs the component without committing the render that set the state, so the value read below is always current; `display-page.tsx:86-94` states the mechanism and is the reference.
    - [x] Not module-level state either, however tempting: `StrictMode` is on (`main.tsx`), double-invoked initializers would consume the baseline on the first pass and see their own write on the second, and tests would become order-dependent.
  - [x] Returns `previous` only while a showing is current; `undefined` at every other state, and `undefined` for a different `gameId` (the shell does not remount on a `/display/A → /display/B` navigation — the same trap `retained` and the stage key already guard).
  - [x] A small pure helper in the same file, `standingsOf(entries: LeaderboardEntry[]): PreviousStandings`, mapping each entry to `{ rank, index }`. Covers the **whole** array, not a slice.
  - [x] **Zero Hebrew, no JSX, comments in English.**

- [x] **Task 10: `web/src/features/display/leaderboard-stage.tsx` (new) — the room reshuffles** (AC: 1, 2; derived reqs. 8, 9, 10, 11, 12, 13, 14, 15)

  The architecture names this exact file: `leaderboard-stage.tsx`.

  - [x] **Signature**: `export function LeaderboardStage({ snapshot, reducedMotion, previousStandings }: StageProps)`, `import type { StageProps } from './stage-props'`.
  - [x] **The degraded guard, first thing in the body, above everything** (derived req. 8):
    ```tsx
    // emptySnapshot carries the real State with a nil CurrentQuestion, a
    // zero QuestionCount and an EMPTY leaderboard - and an empty leaderboard
    // is the one degraded field that looks like a real value ("everyone
    // scored 0"). deferred-work.md's 3.7 entry is about exactly this frame
    // and names this file as its trigger. The discriminator is already on
    // the wire: a real `leaderboard` frame always has questions and a
    // current question, because the state is only reachable from `revealed`.
    if (snapshot.questionCount === 0 || snapshot.currentQuestion === null) { …waiting… }
    ```
    rendering `strings.display.waiting` in the same `text-[length:var(--stage-heading)] font-heading text-text-secondary` treatment the reveal stage's guard uses.
  - [x] **The empty-but-healthy case**: `snapshot.leaderboard.length === 0` on a frame that passed the guard above → `strings.display.leaderboard.noPlayers`, in the same treatment. This is a *different sentence* from the degraded one on purpose: the room is being told two different true things.
  - [x] **The surface** (derived req. 13): full-bleed white, centred, with the safe margin restored inside — `absolute inset-0` resolves against the shell's padding box.
    ```tsx
    <div className="absolute inset-0 flex flex-col justify-center bg-surface-raised p-[var(--stage-margin)]">
    ```
    - [x] **No hero band, no question text, no progress line, no title.** The vertical budget has no room for one (ten rows leave ~264px at 1080p and the list is centred in it), the spec lists none, and the shell's assertive announcer already says "טבלת התוצאות" on entry. Do not add a heading to satisfy the 4.2 heading-semantics entry — Task 12 records why it stays deferred here.
  - [x] **The list**: `<ol>` (implicit list semantics and position, and safe to transform, unlike `<tr>`), rows as `<li>`. `const maxRows = 10` with the A11 citation; `snapshot.leaderboard.slice(0, maxRows)` in the server's order — **never sorted, never re-ranked** (derived req. 10).
  - [x] **Per row**, computed from `previousStandings?.[entry.participantId]`:
    - `climbed = previous ? previous.rank - entry.rank : 0` — positive only. **Falls are not shown** (EXPERIENCE.md: "rows that climbed carry ▲"; AC-1: "highlight climbers"), and neither is a fall's row background.
    - `shift = previous ? clamp(Math.min(previous.index, maxRows) - index, -maxRows, maxRows) : 0` — `Math.min(previous.index, maxRows)` makes a row arriving from below the fold slide up from just past the last visible slot instead of from forty rows away; the outer clamp bounds the other direction the same way.
    - `key={entry.participantId}` — unlike the option rows, these have a real id, and identity is exactly what a reshuffle is about.
  - [x] **Row markup** — the four columns, with fixed measures on three of them so the numbers form columns (which is the whole point of `tabular-nums`):
    ```tsx
    <li
      // Inline custom property, not a class: the shift is data. Cast is
      // required — CSSProperties has no index signature for custom props.
      style={shift !== 0 ? ({ '--stage-row-shift': shift } as CSSProperties) : undefined}
      className={`flex h-[var(--stage-leaderboard-row)] items-center gap-6 border-b border-border-light px-6 last:border-b-0 ${climbed > 0 ? 'bg-green-50' : ''} ${animated && shift !== 0 ? 'stage-row-reshuffle' : ''}`}
    >
    ```
    - rank — `w-[3ch] shrink-0 … text-text-secondary tabular-nums`, `<bdi dir="ltr">` (DESIGN.md `leaderboard-row.rank`: text-secondary 600 = `font-ui`);
    - the mover slot — **reserved on every row**, `w-[5ch] shrink-0 … text-green-600 tabular-nums`, holding `moverMark` + `<bdi dir="ltr">{climbed}</bdi>` when `climbed > 0` and nothing otherwise. Reserved rather than conditional for exactly the reason 4.4's review found on the bar labels: a conditionally-rendered mark pushes that one row's name out of the column every other row shares. `aria-hidden` on the slot, with a sibling `<span className="sr-only">{climbedLabel(climbed)}</span>` when it is populated (derived req. 15);
    - name — `<bdi className="min-w-0 flex-1 truncate … text-text-primary">`. `<bdi>` because a display name is free-form participant-supplied text that can be Latin, mixed or start with a digit inside an RTL row; `truncate` because derived req. 11 needs a single-line row of known height;
    - score — `w-[6ch] shrink-0 text-end … text-green-800 tabular-nums`, `<bdi dir="ltr">`. **Rendered raw, no `toLocaleString` and no thousands separator**, matching `results-summary.tsx`'s `{entry.score}` so the dashboard table and the projector cannot disagree about the same number.
    - Everything sits at `text-[length:var(--stage-ui)]` (48px at 1920 — DESIGN.md `leaderboard-row.fontSize` and A19's "display-only content ≥48px", and the size that keeps the ▲'s 3.15:1 inside the large-text floor).
    - `font-ui` throughout, **including the score**: DESIGN.md asks for weight 700 there and this project's `@theme` has no 700 step. `font-heading` (800) is the nearest, but the leaderboard's contrast is rank-vs-name-vs-score by *colour*, not by weight, and bumping only the score to 800 would make it shout next to a 600 name. Record the deviation exactly as `stage-option.tsx` records its own (letter at 800 for a specified 700), rather than inventing a sixth weight token.
  - [x] `const animated = !reducedMotion` (derived req. 12). Under reduced motion the class is simply absent and the rows render in their final order immediately — "static reordering", verbatim.
  - [x] **No live region.** UX-DR14 reserves assertive for stage transitions and the shell owns them; the rows are static content, and a second announcer would fight the first. (`lobby-stage.tsx` has a polite one because its number *changes* while mounted; nothing here does.)
  - [x] **AC-3-style privacy, asserted not hoped** (derived req. 14): this file must not reference `snapshot.participants` or `snapshot.participantCount`, must not highlight any individual, and must render no grade, phone number or per-answer datum.
  - [x] **Zero Hebrew literals. Nothing focusable. No `shadow-*`. No gold.** Comments in English.

- [x] **Task 11: `web/src/features/display/display-page.tsx` — one map entry, one hook, one prop** (AC: 1, 2)

  - [x] `leaderboard: LeaderboardStage, // story 4.5 — leaderboard stage` and the import. `draft` and `finished` keep their `// story 4.x` comments unchanged.
  - [x] Call `useLeaderboardMemory(gameId, rendered)` and pass the result as `previousStandings` to `<Stage …>`. One extra prop on the existing element; every other stage ignores it.
  - [x] **Nothing else in this file changes.** The `answeredFloor` block, the `stageSnapshot` derivation, the `retained` render-phase pattern, the `?? StagePlaceholder` runtime fallback, `usePrefersReducedMotion`, the OR producing `reducedMotion`, the always-mounted assertive announcer, the three overlay branches, the `${gameId}:${state}` key and the reconnect band's offset are all 4.1–4.4 review decisions with recorded reasoning.
    - [x] Pass `rendered`, **not** `stageSnapshot`, to the memory hook — they differ only in the `answeredCount` floor, which is nothing to do with the leaderboard, and reading the un-rewritten frame keeps the two mechanisms independent. Confirm the choice in the Dev Agent Record.
  - [x] `display-page.test.tsx` **gains** cases (it is not frozen — see Baseline): the leaderboard stage mounts for `leaderboard` and not for any other state (AC-2's "renders only when entered"); a `revealed → question_open` sequence never mounts it; two successive Leaderboard showings deliver the first showing's standings as the second's baseline; a `leaderboard` frame arriving twice for the same question does **not** re-capture the baseline. The other four display test files must pass **unedited**.

- [x] **Task 12: re-triage the `deferred-work.md` entries this story triggers** (no product code)

  Four entries name this story or fire on it, and three more need a written not-triggered. Append each outcome under the existing entry in the file's established style — sub-bullet, dated, trigger re-pointed off story numbers. **Do not rewrite the original text.**

  - [x] **The 3.7 degraded-empty-leaderboard entry** (*"`leaderboard-stage.tsx` is the first surface that actually renders this"*; 4.1's follow-up: *"Story 4.5 must resolve this before rendering a leaderboard to a room"*). This is the trigger and it fires. Record the resolution and its limit precisely: **the display half is closed** by derived req. 8 — the degraded frame is distinguished by a *correlation of fields already on the wire* (`questionCount === 0` / `currentQuestion === null`), not by the wire-contract change the entry assumed was necessary, and two Vitest cases pin it. **The wire ambiguity itself is not closed**: `Snapshot.Leaderboard` is still an empty non-nil slice in both the degraded and the genuinely-empty case, so any *future* consumer that reads the field alone inherits the same trap. Re-point the trigger to "the next consumer of `Snapshot.Leaderboard` outside `leaderboard-stage.tsx`" and note that 4.6's winner takeover reads the same field.
  - [x] **The 2.4 out-of-order broadcast entry** (trigger: *"the first snapshot field rendered on the Audience Display that can legitimately decrease… 4.5's leaderboard can, and is the likely one"*). **Triggered.** Record derived req. 9's analysis in writing: the field genuinely can decrease, **no high-water guard is applied and that is the decision**, and the burst shape that produced 4.2's measurement has no analogue at `leaderboard` (cutoff passed, no inbound writer, only the Organizer's serialized clicks plus the display-settings toggle). Re-defer with the 3.1 `seq` entry, which owns the real fix, and re-point the trigger off story numbers.
  - [x] **The 3.1 "control mutations discard the REST response snapshot" entry.** This story adds a **seventh** broadcasting write path and an **eighth** `api<void>` call site. State that plainly, re-defer (the fix is one architectural change for all of them and does not belong here), and note the one thing that is new: the Leaderboard is the first state a dropped frame can strand the *room* on while the panel has already advanced.
  - [x] **The 4.2 heading-semantics entry** (re-pointed by 4.4 to *"4.6's winner name or the first real screen-reader evaluation"*). Not this story's trigger, but record what it did anyway: the leaderboard stage adds an `<ol>`, which gives the room's ranking real list semantics, and deliberately adds **no** heading (no vertical budget, none specified, and the shell already announces the state). Trigger unchanged.
  - [x] **Confirm in writing, with reasons, that these are NOT triggered:**
    - the **2.5 spectator-roster** entry (trigger: *"the first surface rendering a live roster past lobby"*). 4.3 predicted this story might fire it and predicted correctly that it would not: the leaderboard comes from `GetLeaderboard`, which filters `role = 'player'` in SQL, not from the role-blind `ListParticipants` the entry is about. **This is the closest call of the three — say why it is still a no**, and note that the stage never reads `snapshot.participants`.
    - the **3.7 close→reveal scoring-race** entry (an answer stuck at `points_awarded = NULL`). This story renders `GetLeaderboard`'s output on a projector, which makes a lost point *visible* for the first time — worth recording as a change in **consequence** with no change in mechanism or evidence. Re-defer, do not re-derive.
    - the **SQL-coverage family** (head entry under 3.4's review; family trigger includes *"the first `sqlc`/migration change made by someone who did not write the original query"*). This story adds one query and **modifies two existing guarded writes** — a strictly sharper case than 4.4's, because widening a `WHERE state IN (…)` in the wrong direction would let a transition fire from a state it must not. State plainly what the Go tests do and do not cover (they drive `stubStore`, so the SQL text itself is exercised only by the E2E in Task 13), re-defer with the family, and say why this story does not build the tier.
  - [x] **One note, not an entry**, under 3.1's multiplexed-error-banner item: this story adds a second path to the same `action` mutation, which is *not* a fourth error branch and does not move that entry. Say so, so a reviewer does not read the new CTA as triggering it.

- [x] **Task 13: quality gates, E2E, and the manual browser pass** (all ACs)

  - [x] **Backend gates**: `go build ./...` · `go vet ./...` · `go test ./...` · `sqlc generate` leaves **no diff** (CI's own check) · `git status` shows **no `migrations/` diff**. `go test -race` may be unavailable (`CGO_ENABLED=0`) — if so, say so rather than implying race coverage.
  - [x] **Hebrew centralization**: CI's Go gate scans `*.go`, exempting only `_test.go` and `wa/messages_he.go` — and **sqlc copies `queries/*.sql` comments verbatim into `gen/*.sql.go`**, so the new SQL comment block must be English (`deferred-work.md`'s 3.10 entry; 4.4 hit this exact trap twice). For the frontend, `rg -l '[\p{Hebrew}]' -g '*.tsx' web/src` must list only the two known pre-existing violations (`features/builder/scoring-editor.tsx`, `features/live/control-page.tsx` — note the second is a file **this story edits**, so confirm you did not add to it). **No new `.tsx` may appear**, test files included: use Latin placeholders for fixture participant names and read every expected string from `strings.he.ts`.
  - [x] **Frontend gates**: `npm run lint` · `npx tsc -b --noEmit` · `npm test` · `npm run build` · both filter-safety scans (source and built bundle — no external URL, no font `@import`, no `url(//…)`).
  - [x] **The four untouched display test files**: `git diff <baseline> -- web/src/features/display/{lobby-stage,question-stage,timer-ring,reveal-stage}.test.tsx` is **empty** and all their cases pass. (`display-page.test.tsx` is expected to change — see Task 11.)
  - [x] **Built-CSS verification** (4.3's rule — verify, do not assume): `--stage-leaderboard-row`, `.stage-row-reshuffle`, `@keyframes stage-row-reshuffle`, and the utilities this story is the project's first caller of — `text-green-600`, `border-border-light`, `truncate`, `last:border-b-0`, `text-end`, `h-[var(--stage-leaderboard-row)]`, `w-[3ch]`, `w-[5ch]`, `w-[6ch]`. **Tailwind escapes arbitrary-value selectors** (`.h-\[var\(--stage-leaderboard-row\)\]`), so a naive literal grep reports those as MISSING — 4.4's Debug Log records this false-fail; search for the escaped form. If one genuinely fails to resolve, fall back to `style={{…}}`, never to a hardcoded px.
  - [x] **New Vitest coverage** — co-located, `describe`/`it` imported explicitly (`globals` is off), `afterEach(cleanup)` per file, expected copy read from `strings.he.ts` and never retyped.
    - `leaderboard-stage.test.tsx`: ten rows render from a twelve-entry board and the eleventh and twelfth do not; **shared ranks render as 1,1,3** and are not renumbered; a climber shows ▲ with the right count **and** the accessible label **and** the `bg-green-50` row (assert all three — colour-alone is what UX-DR14 forbids); a faller shows **no** indicator and no row background; `previousStandings === undefined` renders no indicators at all and no `stage-row-reshuffle`; **standings whose order is unchanged animate nothing** (every shift 0, the class absent on every row) — the case that separates "has a baseline" from "actually moved"; `--stage-row-shift` equals `previousIndex - currentIndex` for a climber and is clamped for a row arriving from outside the visible ten; `reducedMotion` removes `stage-row-reshuffle` while the rows keep their final **order** (assert the order, not just the class); a `questionCount: 0` frame renders the waiting copy and **no rows**; a healthy frame with an empty leaderboard renders the no-players copy and **no rows**; a long display name is truncated rather than wrapping the row; no participant id, phone number or roster name from `snapshot.participants` appears in the container.
    - `use-leaderboard-memory.test.ts`: the first showing yields `undefined`; the second yields the first's standings; a repeated frame within one showing does not re-capture; a frame with `currentQuestion === null` does not capture; a different `gameId` yields `undefined`.
    - **Demonstrate at least two guards red first** (drop `Math.min(previous.index, maxRows)`; key the memory on `state` instead of the question id) and record the failure messages, as 3.11, 4.2, 4.3 and 4.4 all did. A test that has never been red has proved nothing.
    - Go side: the Task 3 cases, with the widened-`NextQuestion` one demonstrated red.
  - [x] **Local Go E2E** (`server/cmd/e2escratch`, deleted after use — the 2.1–4.4 convention). Reuse 4.4's harness shape verbatim rather than reinventing: `websocket.Dial` from `github.com/coder/websocket` with the session cookie on the dial request header, signed-webhook POSTs (`X-Hub-Signature-256: sha256=<hmac over the RAW body>`, `webhook_test.go`'s `sign()` helper, a **unique `wamid` per message**, and **the configured `phone_number_id`** — 4.4 lost a whole run to `"PNID"`, which `webhook.go` drops silently with a 200), port 8099, and `WHATSAPP_API_BASE_URL` + `ANTHROPIC_API_BASE_URL` pointed at dead local addresses. Build the binary once, exec it directly, and grep the log for exactly one `server listening` and no `bind:` error — **a green E2E is not evidence unless you confirm which process answered** (3.10's Debug Log records a run silently served by a leftover server).

    Six things only a real server proves:
    - **Scenario A — the transition exists and moves nothing else.** `POST /show-leaderboard` from `revealed` returns 200; the `role=display` socket receives a frame with `state: "leaderboard"`, a populated `leaderboard`, and a `currentQuestion` whose `id` and `position` are **unchanged from the revealed frame** (derived req. 4 — the display's baseline key).
    - **Scenario B — the guard.** `POST /show-leaderboard` from `question_open`, `question_closed`, `leaderboard` and `finished` each returns **409 `GAME_NOT_REVEALED`**, and the state does not move.
    - **Scenario C — the Leaderboard is not a trap.** From `leaderboard`, `next-question` opens the next question **and the WhatsApp question burst still fires** (assert the dispatch, not only the state). Then, on the last question, `next-question` from `leaderboard` finishes the game **and the final-results dispatch fires**.
    - **Scenario D — stop.** `POST /stop` from `leaderboard` finishes the game.
    - **Scenario E — shared ranks on the wire.** Two participants with identical scores come back with the **same `rank`**, and the next distinct score skips (1,1,3). Drive it with real answers, not a fixture.
    - **Scenario F — the skip path still works** (AC-2). `revealed → next-question` goes straight to `question_open` with no `leaderboard` frame in between. Assert on the **frame sequence**, which is the only place this is observable.
    - Clean up the scratch rows afterward (cascades) and delete the harness.
  - [x] **Manual browser pass** (real browser — `playwright-core` + `channel: 'msedge'`, installed into the scratchpad and never into `web/package.json`; see 4.3's and 4.4's Debug Logs for the working shape, including that **the display's WS handshake is authenticated**, so the Playwright context needs `addCookies()` with the login cookie, and that Node's `child_process.spawn` cannot launch anything under this machine's Hebrew home directory — 4.4 used a bash orchestrator for the kill/restart phase). The unit tests cover the arithmetic; **only the browser covers the layout and the motion**, and this stage's failure modes are a silent clip and a reshuffle that moves the wrong way.

    The checks:
    1. **AC-1 rows.** Ten rows; each `72px` tall at 1920×1080; text `48px`; rank in `rgb(75, 85, 99)`, name in `rgb(20, 83, 45)`, score in `rgb(22, 101, 52)`; `1px` `rgb(209, 250, 229)` bottom borders with none on the last row; the score column's digits aligned across rows (measure the score spans' `right`, they must be equal).
    2. **RTL, measured not eyeballed.** Rank is the **inline-start (rightmost)** element of its row and the score the inline-end (leftmost): compare `getBoundingClientRect()` on both against the row. A row laid out LTR would look plausible in a screenshot and be backwards for the room.
    3. **AC-1 movers.** A climber's row background is `rgb(240, 253, 244)` (green-50) **against a white body** — assert the body is `rgb(255, 255, 255)` in the same check, because that contrast is the whole point of derived req. 13. The ▲ and its count are `rgb(22, 163, 74)` at ≥40px, positioned beside the rank, with the `sr-only` label present. A faller has neither.
    4. **Shared ranks.** Seed a genuine tie and confirm the projector renders 1, 1, 3.
    5. **The reshuffle, measured — and its delay.** Show a Leaderboard, run a question that changes the order, show it again. Sample a moved row's computed `transform` at ~100ms (during the delay, held by `backwards`) and at ~250ms: both must be `translateY` of **exactly** `(previousIndex − currentIndex) × rowHeight`, i.e. **the board is still holding the old order while the cross-fade runs**. Sample again at ~1000ms: `none` or the identity. Confirm a row that climbed starts **below** its final slot — this is the check that catches a sign error, which a screenshot cannot — and that no row paints in its final position before the delay elapses (the `backwards` failure mode: a visible reverse-play).
    6. **Reduced motion**, both channels separately (the dashboard's "הפחת אנימציות" toggle, and the OS setting alone): `stage-row-reshuffle` absent from every row, rows in their final order at first paint, and the ▲, the label and the green-50 background **unchanged** — the accessibility cues are independent of the animation.
    7. **Fit, at both 1920×1080 and 1280×720.** Ten rows with the longest realistic names (truncation active) and five-digit scores: the last row's bottom against the body's content box, no page overflow, no clipped row. The shell is `overflow-hidden`, so a clipped tenth row is silent. Predicted: 720px of rows in a 984px box at 1080p, 480px in 656px at 720p.
    8. **AC-2, the skip.** From `revealed`, press "שאלה הבאה ←": the display goes straight to the question stage and the leaderboard stage never mounts.
    9. **The control panel** (this story's dashboard half): at `revealed` both CTAs are present and the secondary POSTs `show-leaderboard`; at `leaderboard` the primary reads "פתח שאלה N" with the right N, "עצור" is offered, and **keyboard focus is retained on the primary button across the `revealed → leaderboard → question_open` transitions** (3.1 AC-4, the property 3.11 exists to protect). On the last question, confirm the label falls back to "שאלה הבאה ←" and the button finishes the game.
    10. **Reconnect at `leaderboard`** — kill the Go server with the board on screen (`ctx.setOffline` does **not** close an established socket and passes vacuously; assert `ws.on('close')` fired). The board holds with its rows and indicators, the band appears over it, and after restart it re-renders with no interaction. **Measure what the band covers** and feed the numbers to Task 12 — this stage's top row sits far higher than 4.3/4.4's content did, so the 4.1 reconnect-band entry may genuinely fire here for the first time. If the band covers a row, **escalate to Avraham rather than patching the stage**.
    11. **Privacy** (derived req. 14) — no phone number, no grade, no per-answer datum anywhere in the rendered DOM, and no individual highlighted. Assert against a game whose participants have distinctive names, and confirm the *names* ARE present (they are the content here, unlike 4.4).
    12. **Palette / filter-safety** — Festival Green only; **zero gold** (scan `color`/`background`/`stroke`/`fill` on every node for `rgb(251, 191, 36)`); zero requests to any external host; no `Fetch/XHR` on the display route.
    13. Zero console errors outside a deliberate outage.
  - [x] Stories 3.10's, 4.1's and 4.2's manual passes are still outstanding (4.4 reported them so, honestly, rather than claiming them). This pass puts you in front of the control panel and the shell again — if it also puts you in front of those surfaces, do them; if it does not, say so rather than claiming them.

### Review Findings

Code review 2026-08-12 (bmad-code-review; three parallel adversarial layers — Blind Hunter, Edge Case Hunter, Acceptance Auditor — all three completed, no failed layer). **Both epic ACs and all thirteen derived requirements verified implemented**, including the four widened guards checked individually (derived req. 5), the untouched `current_question_position` (req. 4), the no-wire-field decision (req. 6), the correlation discriminator (req. 8), the absent high-water guard (req. 9), the no-client-sort rule (req. 10), the measurement-free shift arithmetic (req. 11), the prop-driven motion kill (req. 12), the white body under the green-50 movers (req. 13), the never-read `snapshot.participants` (req. 14) and the not-colour-alone indicator (req. 15).

**Every gate the Dev Agent Record claims was independently re-run in the triage session and reproduced green**: `go build ./...` · `go vet ./...` · `go test ./...` (all packages) · `tsc -b --noEmit` · `eslint .` · **Vitest 90 passed / 8 files — matching the recorded "+19 stage, +9 hook, +4 shell" exactly** · `npm run build` with every Task 13 built-CSS item present including the Tailwind-escaped arbitrary utilities · the `.tsx` Hebrew scan listing exactly the two known pre-existing violations · `git status server/migrations` empty · zero diff on the four frozen display test files. **Scope is clean**: the changed set is exactly the story's in-scope list, and every file on the "a diff here means you went off-spec" list shows zero diff.

Three claims from the Blind Hunter were falsified by direct inspection and dismissed rather than reported: that `.stage-row-reshuffle` inherits a `prefers-reduced-motion` guard by placement (it sits at `index.css:404` inside `@layer components`, and the media block closes at `:318` — the comment is accurate); that the new secondary CTA has no single-flight guard (`use-single-flight.ts` locks on a synchronous ref, not on `disabled`); and that game A's ranking can become game B's baseline (`use-game-socket.ts:190` keys its store on `gameId:role`, so a route change hands the hook a `null` snapshot in the same render — the Edge Case Hunter reached the same conclusion independently). 8 findings dismissed as noise in total.

**Decision needed — all three resolved 2026-08-12 by Avraham (two → patch, one → defer):**

- [x] [Review][Decision] **The dropped focus arms Space, and the next press skips the Leaderboard** — the Dev Agent Record recorded that activating the new "טבלת התוצאות" CTA drops focus to `<body>`, and classified it as interaction design outside this story's scope. That classification rests on a premise the review falsified: `use-space-action.ts:20` fires **only** when `document.activeElement === document.body`, so the focus drop does not merely lose the organizer's place — it **arms** the bare-Space handler. At `leaderboard` the primary action is `next-question`. One Space press after clicking the CTA — the habitual advance key this panel teaches all game long — skips straight out of the Leaderboard that was just opened, in front of the room. Before this story the same click left focus on the persistent primary button, where the `activeElement === body` check keeps Space inert; the conditionally-rendered CTA is what changes that, and `control-page.tsx:191-197` states the persistent-element rule it departs from. Options: **(a)** move focus to the persistent primary button inside the CTA's `onClick` — closes the recorded focus-drop item at the same time; **(b)** accept as recorded and note the Space consequence in the ledger; **(c)** suppress Space for one transition after a secondary action. [web/src/features/live/control-page.tsx:214-222, web/src/lib/use-space-action.ts:20] — **Resolved: option (a).** Move focus to the persistent primary button inside the CTA's `onClick`; this also closes the focus-drop item the Dev Agent Record recorded. Carried below as a patch.
- [x] [Review][Decision] **▲ is a rank delta while the movement is a slot delta, and a dramatic reshuffle can produce zero highlights** — Dev Notes item 2 anticipated the divergence ("subtly out of step with the arrows in exactly the tie cases nobody tests") and derived req. 6 chose rank deltas deliberately. Both hunters found it independently, and it is more reachable than the story assumed: when every player answers Q1 correctly the whole board ties at rank 1, so at the next showing the new leader's rank stays **1 → `climbed === 0`** — no ▲, no `bg-green-50`, no `sr-only` label — while `shift` is up to 10 and the row visibly flies to the top of the board. The mirror case holds too: a row whose rank improves into a tie renders "▲1" and a green fill while `shift === 0` and it never moves a pixel. AC-1 asks that "movement indicators highlight climbers"; in the tie shapes the two signals contradict each other, and nothing in `leaderboard-stage.test.tsx` renders a tie delta (the tie tests cover `standingsOf`'s *recording*, never the *rendering*). Options: **(a)** keep rank deltas as specified and add the missing tie-rendering cases; **(b)** show the indicator when either rank or slot improved; **(c)** escalate as a spec change to derived req. 6. [web/src/features/display/leaderboard-stage.tsx:92 vs :99-101] — **Resolved: option (a).** Rank deltas stand exactly as derived req. 6 specifies; what was missing was the coverage the story itself named. The tie-rendering cases are carried below as a patch.
- [x] [Review][Decision] **Two rows arriving from below the fold stack on one start slot, and departed rows are not rendered — the 300ms hold is not the board the room just saw** — `Math.min(previous.index, maxRows)` clamps every below-the-fold arrival to start slot 10, so with more than ten players and two or more climbing into the top ten in one question, both rows paint on top of each other for the full 300ms delay. Simultaneously the slots vacated by rows that dropped out of the ten sit empty, because a departing row is not rendered at all. There is vertical room for this to be visible (720px of rows in an 888px box at 1080p). The `index.css` comment sells the delay as "the board arrives holding the order the room last saw"; in this shape it demonstrably does not. Options: **(a)** accept and record as a known limit of the measurement-free approach, beside the reloaded-display limit; **(b)** stagger clamped start slots (10, 11, …) so arrivals do not overlap; **(c)** render exiting rows for the animation's duration. [web/src/features/display/leaderboard-stage.tsx:99-101, web/src/index.css:404-405] — **Resolved: option (a) — accepted and deferred.** Reason: cosmetic, reachable only with more than ten players *and* two simultaneous below-the-fold climbers in one question, and both alternatives add machinery to a stage deliberately built without measurement (derived req. 11). Recorded in `deferred-work.md` beside the reloaded-display limit, which is the same family of accepted cost.

**Patch — all nine applied 2026-08-12, in the same session.** Four product/test changes were demonstrated **red first**, per this project's standard, with the mutation applied to the shipped code and reverted afterwards:

1. `isDegradedFrame` with the `questionCount === 0` disjunct removed → `renders the waiting copy when the count is zero but the question is not` fails with `expected 'אף אחד לא נרשם למשחק הזה.' to contain 'המסך מוכן — ממתינים למארגן.'`, and `does not capture a baseline from a degraded frame with a zero question count` fails with `expected { a: { rank: 1, index: +0 }, …(2) } to be undefined`. **Two surfaces, one predicate — which is the point of the patch.**
2. `Math.min(previous.index, maxRows)` dropped → the retitled cap test fails with `expected '13' to be '10'`. Under the *old* code this same assertion could not fail at all, because the outer clamp turned 13 into 10; removing the dead clamp is what made it discriminating.
3. A dispatch fired 10ms after the response (`go func(){ time.Sleep(10ms); DispatchQuestionOpened(…) }()`) → `TestShowLeaderboardDispatchesNoWhatsAppTrafficAtAll` now fails with `DispatchQuestionOpened fired — EXPERIENCE.md gives the Leaderboard row "— (quiet)" on WhatsApp`. The previous form read the counters synchronously and passed over exactly this.

**Gates after the patches**: `go build` · `go vet` · `go test -count=1 ./...` all green · **Vitest 94 passed / 8 files** (90 → 94: two tie-rendering cases, one stage disjunct, one hook disjunct) · `tsc -b --noEmit` · `eslint` · `npm run build` · `.tsx` Hebrew scan back to exactly the two known pre-existing violations and `use-leaderboard-memory.test.ts` no longer matching a `.ts` scan · `server/migrations/` still zero diff · the changed file set now matches the story's File List exactly (`gen/models.go` restored, so the tree carries only the one generated file that genuinely changed).

- [x] [Review][Patch] **Move focus to the persistent primary button when the Leaderboard CTA fires** (from decision 1) — the conditionally-rendered secondary CTA unmounts on the transition it triggers, dropping focus to `<body>`, which is the one condition under which `use-space-action.ts:20` fires. **Precision on what the fix does and does not do**, checked while applying it: handing focus to the primary button does *not* stop Space from advancing — a focused `<button>` activates on Space natively, so Space still fires `next-question` at `leaderboard`. That is correct and intended: `next-question` **is** the documented primary action for the "Between questions" state, and Space firing the primary is the panel's design (UX-DR10). What the drop to `<body>` actually breaks is the correspondence between the two: the advance key stays live while **nothing on screen has focus to show what it will do**, and a keyboard or screen-reader Organizer loses their position outright. Handing focus back to the persistent primary restores 3.1's AC-4 invariant and makes the Space target visible again. [web/src/features/live/control-page.tsx:214-222]
- [x] [Review][Patch] **Add the tie-delta rendering cases the story named and never wrote** (from decision 2) — `leaderboard-stage.test.tsx` covers ties only through `standingsOf`'s *recording*. Two rendering cases are missing, and they are the ones Dev Notes item 2 predicted nobody would write: a previous board tied at rank 1 where the new leader's rank is unchanged (`climbed === 0`, no indicator) while `shift` is non-zero, and a row whose rank improves into a tie (`climbed === 1`, indicator present) while `shift === 0`. Both assert the current, specified behaviour — they pin the divergence rather than change it. [web/src/features/display/leaderboard-stage.test.tsx]
- [x] [Review][Patch] The hook's capture guard and the stage's degraded guard are two hand-maintained predicates that disagree, and the divergent branch is untested — the stage refuses to render when `questionCount === 0 || currentQuestion === null`, the hook refuses to capture only when `currentQuestion === null`. A frame with `questionCount === 0` and a non-null `currentQuestion` shows the waiting copy while the hook records that board as "what the room saw", poisoning the next showing's arrows — exactly what the hook's own comment says it prevents. Unreachable from today's `emptySnapshot` (which sets both together), but this correlation is the discriminator the story chose *instead of* a wire-contract change, so it should be one shared predicate, not two. Deleting `snapshot.questionCount === 0` from the stage leaves the whole suite green: `leaderboard-stage.test.tsx:285` drives both halves together and `:295` drives `currentQuestion` alone, so the first disjunct is never exercised on its own. Add that case, and correct the `deferred-work.md:136` sentence claiming "either half of the correlation alone is enough" is pinned. [web/src/features/display/use-leaderboard-memory.ts:71-74, web/src/features/display/leaderboard-stage.tsx:41]
- [x] [Review][Patch] The outer `clamp` is unreachable dead code, and the test that names it exercises neither the clamp nor the "other direction" it claims — `Math.min(previous.index, 10) ∈ [0,10]` and `index ∈ [0,9]`, so the expression's range is `[-9,10]` and the `[-10,10]` bound can never engage. The test "clamps the shift to the visible depth in both directions" seeds one climber landing at index 0 and expects `'10'`, which every variant produces: with both guards, with `Math.min` deleted, and with `clamp` deleted. It tests no fall and no clamp. The Debug Log diagnoses this precise defect in the *sibling* test and fixed that one (landing slot 5, which does discriminate) without applying the lesson here. [web/src/features/display/leaderboard-stage.tsx:13-15,100; web/src/features/display/leaderboard-stage.test.tsx:242-251]
- [x] [Review][Patch] `TestShowLeaderboardDispatchesNoWhatsAppTrafficAtAll` cannot catch the regression it exists to catch — its own comment names the risk ("'by construction' is exactly what a later refactor breaks quietly"), but every sibling handler dispatches via `go dispatch…` (`control.go:239,286,345,346,370`) and the test reads `questionDispatcher.Calls()` synchronously after `ServeHTTP` returns. A refactor that added a goroutine dispatch would almost never have scheduled it by then: the test reads 0 and passes while the room's phones buzz. It currently proves only the synchronous-dispatch variant, which is not how this codebase dispatches. [server/internal/httpapi/control_test.go:947-985]
- [x] [Review][Patch] Four statements in the new `deferred-work.md` records are contradicted by the code they describe — (1) `handleShowLeaderboard` is the **eighth** broadcasting write path, not the seventh (`control.go:213,236,260,283,314,337,367` + `display.go:52`; the same file at `:66` already numbers `handleUpdateDisplaySettings` seventh); (2) there is **no new `api<void>` call site at all** — `control-page.tsx` still has exactly two (`:87`, `:90`) and the CTA reuses the existing `action` mutation, as the entry's own parenthetical says; (3) `ShowLeaderboard`'s rejection table iterates **six** states, not five (`engine_test.go:239`); (4) the `<ol>` list-semantics claim is weaker than stated — Tailwind preflight's `ol { list-style: none }` plus `display:flex` rows strip `display: list-item`, under which WebKit/VoiceOver stops exposing list semantics. Two of the numbers were seeded by the story's own Task 12 text and reproduced unchecked. [_bmad-output/implementation-artifacts/deferred-work.md:81,110,136,204]
- [x] [Review][Patch] A Hebrew literal landed in a new frontend file, in a blind spot of the story's own centralization gate — Task 9 requires "zero Hebrew, comments in English" for this module family, and Task 13's scan is `*.tsx`-scoped, so it passes vacuously over a `.ts` file. Comment only, no runtime effect; the gate should widen to `.ts` as well. [web/src/features/display/use-leaderboard-memory.test.ts:117]
- [x] [Review][Patch] The shell's mount test says "at no other state" but iterates three of six, and asserts `li` counts rather than stage identity — `display-page.test.tsx:190` delivers only `question_open`, `question_closed` and `revealed`; `draft`, `lobby` and `finished` are never delivered. Both leaderboard cases assert `container.querySelectorAll('li').length`, which couples them to an implementation detail of unrelated sibling stages instead of to `stageByState.leaderboard`. Residual risk is low (the `Record<GameState, …>` map is compile-checked), but the titles over-claim. [web/src/features/display/display-page.test.tsx:181-218]
- [x] [Review][Patch] `gen/models.go` is staged with an EOL-only change and no content diff — `git diff --numstat` produces nothing for it (LF→CRLF churn). It is in scope and is not in the story's File List; committing it adds noise to a generated file. [server/internal/store/gen/models.go]

**Deferred (pre-existing or out of scope):**

- [x] [Review][Defer] Two rows arriving from below the fold stack on one start slot, and departed rows are not rendered — deferred by Avraham's decision 2026-08-12; reason: cosmetic, needs >10 players with two simultaneous below-the-fold climbers, and both fixes add machinery to a deliberately measurement-free stage
- [x] [Review][Defer] A degraded post-commit frame at `leaderboard` costs the room the whole board with no self-heal — deferred, pre-existing
- [x] [Review][Defer] Fixed-width numeric columns have no overflow handling (`w-[6ch]` score, `w-[3ch]` rank, `w-[5ch]` mover slot) — deferred, pre-existing
- [x] [Review][Defer] On the last question the Leaderboard's primary CTA reads "שאלה הבאה ←" and ends the game — deferred, pre-existing
- [x] [Review][Defer] `live.openQuestionNumberCta` interpolates a digit run into a Hebrew template with no bidi isolation — deferred, pre-existing

## Dev Notes

### Why this story is bigger than 4.2–4.4, and where the extra size comes from

Every Epic-4 story so far has been display-only or nearly so. This one is a three-layer story, and the reason is recorded rather than discovered: **story 3.1 built five of the six state transitions and deliberately left the sixth out**, because a "show Leaderboard" control with nothing to show it on would have been speculative work. It flagged the debt at the time — *"better to confirm before 4.5, not after"* — and this is 4.5.

So the backend half is small but non-negotiable: one guarded `UPDATE`, one store wrapper, one engine method, two widened guards, one route. The dashboard half is smaller still: one secondary CTA, one numbered primary label, one entry in `stoppableStates`. The display half is the story's actual subject.

**If this needs to be split, the only sound line is between Task 4 and Task 5** — backend + route first, display + dashboard second. It is not recommended: the halves are untestable apart (a transition nothing can reach, or a stage nothing can enter), and 4.4 was a comparable size and shipped whole.

### Where every rendered value comes from

| Rendered | Source | Notes |
|---|---|---|
| Rank "3" | `leaderboard[i].rank` | server-assigned, standard competition ranks (1,1,3) — never recomputed |
| Name | `leaderboard[i].displayName` | free-form participant text → `<bdi>` + `truncate` |
| Score | `leaderboard[i].score` | raw digits, no separator, matching `results-summary.tsx` |
| "▲3" | `previousStandings[id].rank − entry.rank` | **client-side memory**, not on the wire (derived req. 6) |
| The row's travel | `previousStandings[id].index − i` | whole row-heights, as `--stage-row-shift` (derived req. 11) |
| Row highlight | `climbed > 0` | green-50, on a white body (derived req. 13) |
| Waiting copy | `questionCount === 0 \|\| currentQuestion === null` | the degraded frame (derived req. 8) |
| No-players copy | healthy frame, `leaderboard.length === 0` | a real, different fact |

`GetLeaderboard` (`answers.sql:321`) LEFT JOINs **every `role='player'` participant**, so a player who answered nothing still appears at 0 — the board is the whole room, not just the scorers. That is also why the player set is fixed for the game (Spectators are created only past `lobby` and are filtered out), which is what makes a remembered ranking safe to compare against a later one.

### Three things about this stage that are easy to get subtly wrong

1. **The mover highlight is the same colour as the shell's ground.** `green-50` = `surface-base` = `#F0FDF4`. On the shell's own background the highlight is literally invisible. The stage paints white. 4.2 hit the identical trap with the counter pill and fixed it the same way.
2. **Rank and slot are different numbers.** With shared ranks, ranks 1,1,3 sit in slots 0,1,2. "Places climbed" is a **rank** delta (what the room understands); the reshuffle distance is a **slot** delta (what the eye sees). `PreviousStanding` carries both on purpose, and using one for the other produces animations that are subtly out of step with the arrows in exactly the tie cases nobody tests.
3. **The baseline must not be re-captured mid-showing.** Every frame that arrives during the Leaderboard — a display-settings toggle is the reachable one — carries the same board. Capturing on every frame would overwrite the baseline with the current standings and silently erase every arrow one frame after they appeared. The showing key exists for this.

### Design decisions worth flagging explicitly

- **There is no leaderboard mockup.** `mockups/` holds `key-stage-lobby.html`, `key-stage-question.html`, `key-stage-reveal.html`, `key-stage-winner.html` and `key-screens-1.html` — and none of them contains a leaderboard (`grep -i "leaderboard\|rank\|▲"` over the directory returns nothing). Unlike 4.2–4.4, **there is no authoritative pixel resolution to defer to**: DESIGN.md's `leaderboard-row` / `leaderboard-row-mover` frontmatter plus EXPERIENCE.md's one-line behaviour spec are the whole specification, and every measurement in this story is derived from them rather than transcribed. That raises the value of Task 13's browser pass and is the reason the row height carries its own "sanctioned adjustment" note.
- **Row height is DESIGN.md's 72px *minimum* used as an exact height.** Justified by derived req. 11 (the reshuffle arithmetic) and by uniformity. The alternative — rows flexing to fill the screen — was rejected because a three-player game would render three 300px rows.
- **No winner treatment on rank 1.** Gold is rationed to two moments and this is neither; a crown or a larger first row would also pre-spend 4.6's takeover.
- **The score's weight is a recorded deviation, not an oversight.** DESIGN.md asks for 700; the `@theme` has 500/600/800/900. `stage-option.tsx` already records the same gap for the option letter and resolved it upward; this stage resolves it *downward* to `font-ui` (600) and says why — the leaderboard's hierarchy is carried by colour, and an 800 score beside a 600 name reads as a different kind of information.
- **`text-green-600` at 3.30:1 on white is legal only because the indicator is large text.** Recorded here because it is the tightest contrast in the story and the number is not in DESIGN.md's table. Anything that shrinks the indicator below the large-text threshold breaks AA.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/store/queries/games.sql](server/internal/store/queries/games.sql)** — one appended query; two `WHERE state …` lists widened. Every other guard, and `RevealCurrentQuestion`'s ungraded-answers `NOT EXISTS`, stay exactly as they are.
- **[server/internal/game/engine.go](server/internal/game/engine.go)** — one interface method, one new transition, two widened guards. `buildSnapshot` and its reveal branch are 4.4's and are untouched.
- **[server/internal/httpapi/control.go](server/internal/httpapi/control.go)** + **[router.go](server/internal/httpapi/router.go)** — one interface method, one handler, one route. The dispatch helpers, the 5s contexts and the error mapping are 3.1–3.9's.
- **[web/src/features/live/control-page.tsx](web/src/features/live/control-page.tsx)** — one secondary CTA, one computed primary label, one entry in `stoppableStates`, one stale comment corrected. **The never-disabled persistent primary button, the single-flight `fire()`, the latest-ref error reset and the polite live region are 3.1/3.11/4.1 review decisions**, each fixing a measured defect. This file is also one of the two known Hebrew-scan violations — do not add to it.
- **[web/src/features/display/display-page.tsx](web/src/features/display/display-page.tsx)** — one map entry, one hook call, one prop. The `answeredFloor` / `stageSnapshot` block is the newest and most easily "simplified" thing in the file; it was added to fix a measured 8→7 regression.
- **[web/src/features/display/stage-props.ts](web/src/features/display/stage-props.ts)** — one type, one optional field. Its header comment about the module cycle is why the file exists.
- **[web/src/index.css](web/src/index.css)** — one custom property inside `.stage-root`, one rule inside `@layer components`, one `@keyframes` beside the three existing ones.
- **[web/src/lib/strings.he.ts](web/src/lib/strings.he.ts)** — two entries in `live`, one `leaderboard` sub-block inside `display`.
- **[_bmad-output/implementation-artifacts/deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md)** — Task 12's written outcomes, appended under the existing entries.

### Testing standards

Vitest (since 3.11): co-located `*.test.tsx` / `*.test.ts`, one `test` block in `web/vite.config.ts` (`environment: 'jsdom'`, `include: ['src/**/*.test.{ts,tsx}']`), `globals` **off** so every test imports `describe`/`it`/`expect`/`vi` explicitly, `afterEach(cleanup)` per file, and a CI step in the existing `frontend` job. Expected Hebrew is read from `strings.he.ts`, never retyped; fixture participant **names** use Latin placeholders so the frontend Hebrew grep stays clean (4.3's finding).

**Test the arithmetic, the guards and the ordering — not the layout.** Asserting Tailwind class strings in jsdom proves nothing about a projector; the browser pass covers layout. The exceptions are the class and inline-property assertions that stand in for a *behaviour* with no other observable: `stage-row-reshuffle` present/absent is how derived req. 12 is proved, `--stage-row-shift`'s value is how derived req. 11 is proved, and the green-50 row background alongside the ▲ and its label is how AC-1's not-colour-alone requirement is proved.

Two harness facts carried forward from 4.4, both of which cost real time there: `getByText` joins only an element's **direct text-node children**, so a row composed of several spans matches on neither half — assert the composed line or query the spans; and RTL's `cleanup()` empties the container, so any measurement taken from a render must be **captured before** cleanup.

Go: stdlib `testing`, co-located `_test.go`, stubs grown in place — no mock framework. **No real-DB unit tests**: the documented standard since 2.1, and the reason Task 12 has a SQL-coverage entry to re-triage rather than a tier to build. `go test -race` may be unavailable (`CGO_ENABLED=0`, carried since 3.8) — if so, say so rather than implying race coverage.

### Project Structure Notes

**New:**
- `web/src/features/display/leaderboard-stage.tsx` — named by the architecture (`leaderboard-stage.tsx` in the `features/display/` tree)
- `web/src/features/display/leaderboard-stage.test.tsx`
- `web/src/features/display/use-leaderboard-memory.ts` + `.test.ts` — not in the architecture's tree; feature-local rather than `lib/` because it has exactly one consumer, following `stage-props.ts`'s precedent (`use-throttled-announcement.ts` went to `lib/` only when it acquired a second caller)

**Modified:**
- `server/internal/store/queries/games.sql`, `server/internal/store/games.go`, `server/internal/store/gen/*` (regenerated)
- `server/internal/game/engine.go`, `server/internal/game/engine_test.go`
- `server/internal/httpapi/control.go`, `router.go`, `control_test.go`, `router_test.go`
- `web/src/features/live/control-page.tsx`
- `web/src/features/display/display-page.tsx`, `display-page.test.tsx`, `stage-props.ts`
- `web/src/index.css`, `web/src/lib/strings.he.ts`
- `_bmad-output/implementation-artifacts/deferred-work.md`

**Untouched (a diff here means you went off-spec):** `server/migrations/**` · `server/internal/ws/**` · `server/internal/wa/**` · `server/internal/grading/**` · `server/internal/game/{snapshot,scoring,answers,results,final,state,participants}.go` · `server/internal/httpapi/{display,results,games,questions,packages}.go` · `web/src/lib/{use-game-socket,api,text,use-space-action,use-single-flight,use-throttled-announcement,types}.ts` · `web/src/features/display/{lobby-stage,question-stage,reveal-stage,timer-ring,stage-hero-band,stage-option,stage-placeholder}.*` · `web/src/features/{lobby,builder,results,auth}/**` · `web/src/features/live/{display-controls,response-stats}.tsx` · `web/src/app.tsx` · `web/src/components/**` · `web/index.html` · `web/package.json` · `web/vite.config.ts` · `.github/workflows/ci.yml` · the four display test files named in Task 13.

Note `web/src/lib/types.ts` is in the untouched list: `LeaderboardEntry` already carries everything the wire needs, and the previous ranking is deliberately not a wire type (derived req. 6).

### References

- Epic + ACs: [epics.md](_bmad-output/planning-artifacts/epics.md#L733-L746) (Story 4.5), [#L652-L654](_bmad-output/planning-artifacts/epics.md#L652-L654) (Epic 4 framing)
- Visual spec: DESIGN.md frontmatter `components.leaderboard-row`, `components.leaderboard-row-mover`; `Colors` contrast table; `Typography` → A19 projection ramp ("display-only content ≥48px"); `Layout & Spacing`; `Elevation & Depth` ("Audience Display: flat"); `Do's and Don'ts` → bidi isolation, colour never the sole signal
- Behaviour: EXPERIENCE.md → IA `Audience Display — stages` (Leaderboard row, `[A11]` top-10 depth); `Component Patterns` → Audience Display stages, Leaderboard bullet ("No current-user highlighting — it's a shared screen"); `Host control panel` table (the **"Between questions"** row: "פתח שאלה [N]" / "עצור"); `State Patterns` → Leaderboard row ("— (quiet)" on WhatsApp, "Next / עצור" on the dashboard) and Reveal row ("Next / Leaderboard"); `Accessibility Floor` → Motion (the reshuffle named explicitly), live regions, distance legibility; Flow 4 / UJ-4 (the skip)
- **No mockup exists for this stage** — see Dev Notes. `key-stage-lobby/question/reveal/winner.html` and `key-screens-1.html` contain no leaderboard.
- FR-10 (per-state content), FR-13 (skippable Leaderboard), FR-17/FR-18 (scoring, leaderboard, winner): [prd.md](_bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md)
- Architecture: the `features/display/` tree naming `leaderboard-stage.tsx`; `Component Boundaries (Web)` ("display/* renders exclusively from the WS snapshot"); `Dependency direction` (`game` imports `store`, never the reverse); `Requirements to Structure Mapping` (§4.6 Scoring & Leaderboard → `leaderboard-stage`); `Structure Patterns` (co-located Vitest)
- **The debt this story pays**: [3-1-run-the-game-live-control-state-machine.md](_bmad-output/implementation-artifacts/3-1-run-the-game-live-control-state-machine.md) — "The central judgment call" section, which built five transitions, declined the sixth, and named story 4.5 as its owner
- Deferred entries this story must resolve or explicitly clear: [deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md) — the 3.7 degraded-empty-leaderboard entry (*"Story 4.5 must resolve this"*), the 2.4 out-of-order entry (*"4.5's leaderboard can, and is the likely one"*), the 3.1 discarded-REST-snapshot entry, and the SQL-coverage family. Not triggered, but confirm in writing: the 2.5 spectator-roster entry, the 3.7 scoring-race entry, and the 4.2 heading-semantics entry.
- Prior stories: [4-4-reveal-stage-the-answer-marked.md](_bmad-output/implementation-artifacts/4-4-reveal-stage-the-answer-marked.md) (the newest stage patterns, the prop-driven motion kill, the reserved-slot fix, the built-CSS escaping false-fail, and the E2E/browser harness shapes) · [4-3-question-stage-with-the-authoritative-timer.md](_bmad-output/implementation-artifacts/4-3-question-stage-with-the-authoritative-timer.md) (the measured fit budget and the shell-owns-cross-stage-memory decision) · [3-11-the-control-panel-that-never-goes-dead.md](_bmad-output/implementation-artifacts/3-11-the-control-panel-that-never-goes-dead.md) (`useSingleFlight` and why the primary button is never disabled) · [3-7-scoring-with-speed-bonuses.md](_bmad-output/implementation-artifacts/3-7-scoring-with-speed-bonuses.md) (`RankLeaderboard`, ties, and the degraded-empty-leaderboard entry's origin)

### Latest technical information

No dependency changes and no new libraries. Versions as pinned: React 19.2, React Router 8.2, Tailwind CSS 4.3, TypeScript 6.0, Vite 8.1, Vitest 4.1, `@testing-library/react` 16.3, `eslint-plugin-react-hooks` 7.1; Go 1.26, pgx v5, sqlc **1.31.1** (CI installs exactly this and diff-checks the output — a different local version fails the build even if the SQL is right).

Version-specific details that will bite if missed:

- **Tailwind v4 reads source text, not runtime values.** Every class must appear as a complete literal. The row's conditional classes (`bg-green-50`, `stage-row-reshuffle`) must both be whole strings on both arms of their ternaries — a template like `` `bg-${x}` `` generates nothing and the mover highlight silently disappears.
- **Tailwind escapes arbitrary-value selectors in the built CSS** (`.h-\[var\(--stage-leaderboard-row\)\]`, `.w-\[3ch\]`). A literal grep reports them MISSING; 4.4's Debug Log records this as a false-fail waiting for 4.5.
- **`text-*` with a CSS variable needs the `length:` hint**: `text-[length:var(--stage-ui)]`, not `text-[var(--stage-ui)]`.
- **Inline CSS custom properties need a cast**: `style={{ '--stage-row-shift': shift } as CSSProperties}` — `CSSProperties` has no index signature for custom properties. `import type { CSSProperties } from 'react'` (`verbatimModuleSyntax: true`).
- **`verbatimModuleSyntax: true`**: `StageProps`, `PreviousStandings`, `LobbySnapshot`, `LeaderboardEntry` and `CSSProperties` must all be imported with `import type`. `noUncheckedIndexedAccess` is **off**, so `previousStandings[id]` types as `PreviousStanding` rather than `PreviousStanding | undefined` — **the runtime guard is real and no lint rule will call it redundant**, exactly as 4.4's `?? 0` on `optionCounts[i]` was.
- **`eslint-plugin-react-hooks` v7** enforces `react-hooks/refs` (no ref reads during render) and `react-hooks/set-state-in-effect`. The memory hook's render-phase `setState` is the documented React pattern and is what those two rules leave available; `display-page.tsx` uses it twice already.
- **`react-refresh/only-export-components`** rejects a non-component value export from a `.tsx` module — the reason `useLeaderboardMemory` and `standingsOf` get their own `.ts` file rather than riding along in the stage.
- **sqlc `:one` on a guarded `UPDATE … RETURNING *`** generates `(gen.Game, error)` and returns `pgx.ErrNoRows` when the guard fails — the store wrapper's remap to `store.ErrNotFound` is what turns that into a 409 rather than a 500. Read the generated signature after Task 1 rather than guessing it.

## Dev Agent Record

### Agent Model Used

claude-sonnet-5 (Claude Code, `bmad-dev-story`)

### Baseline

**`bf0b93a`** — `main` at the merge of PR #12 (story 4.4). The story file was written while 4.4 was uncommitted and left the baseline open with two branch options. Resolution as it actually happened: work started from 4.4's own commit `3fd277a` (4.4 was then committed but unmerged), and mid-implementation Avraham reported 4.4 had merged. `bf0b93a`'s tree is **byte-identical** to `3fd277a`'s (verified: both `e427db3…`, `git diff` empty — PR #12 was a `--no-ff` merge of a fast-forwardable branch), so the branch was advanced with `git merge --ff-only` and no file changed. Frontmatter corrected to `bf0b93a`.

### Debug Log References

**Reds demonstrated before green** (each mutation applied to shipped code, failure recorded, then reverted):

1. **`NextQuestion`'s widened guard** — narrowed `if g.State != StateRevealed && g.State != StateLeaderboard` back to `!= StateRevealed`:
   ```
   --- FAIL: TestNextQuestionFromLeaderboardOpensTheNextQuestion
       engine_test.go:1396: NextQuestion() from leaderboard err = game: not in revealed state, want nil
   --- FAIL: TestNextQuestionFromLeaderboardOnLastQuestionFinishesTheGame
       engine_test.go:1418: NextQuestion() from leaderboard err = game: not in revealed state, want nil
   ```
2. **The memory hook's showing key** — keyed on `state` instead of `currentQuestion.id`: 4 of 9 cases failed, `AssertionError: expected undefined to deeply equal { a: { rank: 1, index: +0 }, …(2) }`. Proves the baseline is genuinely per-showing.
3. **The below-the-fold clamp** — dropped `Math.min(previous.index, maxRows)`: `AssertionError: expected '8' to be '5'`.

**Red #3 initially passed, and that was a test defect worth recording.** The first version of the clamp test put the arriving row in slot **0**, where `min(13,10)-0` and `13-0` both clamp to 10 — the two forms agree and the mutation survived. The `Math.min` only bites when the arriving row lands in a **non-zero** slot; the test now lands it in slot 5 (`min(13,10)-5 = 5` vs `13-5 = 8`). A mutation that does not fail is a test that proves nothing, not a redundant guard.

**Harness false-fails and fixture defects hit during verification** (all mine, none product bugs):

- **Built-CSS `last:border-b-0` reported MISSING.** Tailwind emits `.last\:border-b-0:last-child`; the literal grep cannot see it. This is exactly the escaped-selector false-fail 4.4's Debug Log predicted for this story. Confirmed present with the escaped form: `.last\:border-b-0:last-child{border-bottom-width:0}`.
- **`grep -P` is unavailable here** (`-P supports only unibyte and UTF-8 locales`) — the Hebrew scans must run through ripgrep, not `grep -P`.
- **A `--`-prefixed grep pattern is parsed as a flag.** `grep -qF "--stage-leaderboard-row"` reports MISSING; needs `grep -qF -e "…"`.
- **E2E: the WS snapshot rides under `"state"`, not `"snapshot"`** (`ws/hub.go:45-47`). An envelope guessing `"snapshot"` unmarshals cleanly into a zero value, so the harness reported "no frames" while the server was working perfectly.
- **E2E: MCQ answers are numeric (`1..4`), not letters.** Sending `"B"` grades as an out-of-range response worth zero, and every score came back 0.
- **E2E: a leftover server held port 8099** from an earlier attempt — caught by the "exactly one `server listening`, zero `bind:`" check, which is the check 3.10's Debug Log added for precisely this. The green run was re-done after killing it.
- **Browser: `wa_id` must equal `from` EXACTLY.** `profileNameFor` matches `c.WaID == m.From` (`wa/webhook.go:371`); stripping the `+` from `wa_id` silently drops the pushname and every participant rendered as a phone-derived label (`1003`, `1004`) instead of a name.
- **Browser: the first reshuffle fixture produced zero movement.** Q1-correct/Q2-correct sets that leave everyone tied re-order nothing (ties break on `joined_at`, which preserves the previous order), so ranks changed while slots did not — the arrows appeared but every `--stage-row-shift` was 0 and the timing assertions passed **vacuously** (`want=0`, `actual=0`). Fixed by inverting which half of the room wins each question, and both timing checks are now gated on `moved.length > 0` so they can never pass over a board that did not move.
- **Browser: `document.body.innerText.includes('PLAYER-p11')` false-failed** — row text concatenates `PLAYER-p1` + score `1200`. Assert on the parsed name column, not raw `textContent`.
- **Browser: a bare `.sr-only` selector matches the shell's own assertive announcer.** Scope to `li .sr-only`.

**WhatsApp inbound path — verified, not changed (Task 4).** `rg '\p{Hebrew}'`-style scan of `server/internal/wa/*.go` for state strings returns **only prose comments** (`messages_he.go:137` even reads "question_open through leaderboard"); `inbound.go` classifies purely on message *syntax* (`parseJoinCode`, `parseRenameName`, else text/answer) and branches on **no** game state anywhere. State-dependence lives entirely in `game.Engine` — `Join`'s switch and `RecordAnswer` → `GetOpenQuestionForPlayer`'s SQL guard — and `participants.go:146` already lists `StateLeaderboard` in `spectatorAllowedStates`. So a JOIN arriving during the Leaderboard already becomes a Spectator and an answer already gets the between-questions reply. **No scope surprise; zero `wa/**` diff.**

**`go test -race` is unavailable on this machine** — `cgo: C compiler "gcc" not found`. Race coverage is therefore **not** claimed for this story, stated rather than implied (the standard carried since 3.8).

### Completion Notes List

**What shipped.** The sixth and last live-game transition (`revealed → leaderboard`), its two exit guards, the dashboard controls that reach it, and the Audience Display stage that renders it. Backend + dashboard + display, as derived requirement 3 predicted.

**Verification, in numbers.** 90 frontend tests (8 files, +19 leaderboard-stage, +9 memory-hook, +4 shell cases); full Go suite green; `sqlc generate` idempotent with zero `migrations/` diff; **19 local E2E assertions across scenarios A–F against a real Postgres**; **74 automated browser assertions** across three drivers at 1920×1080 and 1280×720.

**Answers to the questions the story asked to be recorded:**

- **`rendered` vs `stageSnapshot` for the memory hook (Task 11).** Passed `rendered`, as specified. The two differ only in the `answeredCount` floor, which is a question-stage concern; reading the un-rewritten frame keeps the two cross-stage mechanisms independent, and the leaderboard never reads `answeredCount` at all.
- **The three new `[ASSUMPTION]` copy items** need Avraham's confirm-at-review, same treatment as 4.1–4.4's: `live.showLeaderboardCta` ('טבלת התוצאות' — authored here; EXPERIENCE.md's Host-microcopy table has no Leaderboard row), `display.leaderboard.climbedLabel` (noun form, gender-neutral per A2), and `display.leaderboard.noPlayers`. Also the **300ms delay and 600ms duration** of the reshuffle: both derived, neither specified. The delay matches `.stage-fade` exactly so the two never overlap, and 300+600 = 900ms sits inside NFR-1's 1s budget. The browser pass confirms the board is readable in its old order before it moves.
- **The reconnect band, measured (Task 13 check 10, feeding Task 12).** Band occupies **y = 48…124**; the first leaderboard row starts at **y = 180**. A **56px gap** — the band covers **no** row, so the 4.1 reconnect-band entry does **not** fire here despite this stage's content sitting higher than 4.3/4.4's. Measured with the server genuinely killed and `ws.on('close')` asserted to have fired, so the result is not vacuous.
- **A discrepancy in the story's own spec, resolved the sensible way.** Task 3 asks for a negative control that "`NextQuestion` **and `StopGame`** still reject `question_open`". `StopGame` *accepts* `question_open` — it is one of the stoppable states, and always has been. Implemented the meaningful control instead: `NextQuestion` rejects a table of five states (`draft`, `lobby`, `question_open`, `question_closed`, `finished`) asserting the call counter as well as the error, and `StopGame`'s existing disallowed-state table (`draft`, `lobby`, `finished`) is untouched.

**One finding for Avraham's decision, from the browser pass.** The primary CTA retains keyboard focus across `revealed → leaderboard → question_open`, so 3.1's AC-4 holds (measured). But **activating the new secondary "טבלת התוצאות" CTA drops focus to `<body>`**: the button is rendered only at `revealed`, so the transition it triggers unmounts the element the activation just focused. This is inherent to a state-scoped secondary action rather than a regression in the persistent primary, and fixing it would be new interaction design (where should focus go — the primary?), which is outside this story's scope. Recorded rather than patched.

**Known limit, stated not hidden (derived req. 6).** A display reloaded or opened mid-game has no baseline and shows **no ▲** on the first Leaderboard it sees — indistinguishable from "nobody climbed". This is the accepted cost of client-side memory and is flagged for Avraham; it was deliberately not "fixed" by inventing a wire field.

**Manual browser pass.** The 13 specified checks were run as automated measurements (74 assertions) — layout, RTL, colours, contrast, motion timing and sign, reduced motion on both channels, fit at both resolutions, privacy, palette and reconnect. Avraham additionally ran his own pass on a live game. **Stories 3.10's, 4.1's and 4.2's manual passes remain outstanding** — this pass put me in front of the leaderboard stage, the control panel and the shell, but not in front of those stories' surfaces, so they are not claimed.

### File List

**New:**
- `web/src/features/display/leaderboard-stage.tsx`
- `web/src/features/display/leaderboard-stage.test.tsx`
- `web/src/features/display/use-leaderboard-memory.ts`
- `web/src/features/display/use-leaderboard-memory.test.ts`

**Modified:**
- `server/internal/store/queries/games.sql`
- `server/internal/store/gen/games.sql.go` (regenerated, sqlc 1.31.1)
- `server/internal/store/games.go`
- `server/internal/game/engine.go`
- `server/internal/game/engine_test.go`
- `server/internal/httpapi/control.go`
- `server/internal/httpapi/control_test.go`
- `server/internal/httpapi/router.go`
- `server/internal/httpapi/router_test.go`
- `web/src/features/display/display-page.tsx`
- `web/src/features/display/display-page.test.tsx`
- `web/src/features/display/stage-props.ts`
- `web/src/features/live/control-page.tsx`
- `web/src/index.css`
- `web/src/lib/strings.he.ts`
- `_bmad-output/implementation-artifacts/deferred-work.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/4-5-leaderboard-stage-the-room-reshuffles.md`

**Created and deleted within the story** (the 2.1–4.4 convention): `server/cmd/e2escratch/` (harness + cleanup tool).

## Change Log

| Date | Change |
|---|---|
| 2026-08-12 | Story 4.5 created — leaderboard stage, movement indicators and the reshuffle, plus the sixth state transition (`revealed → leaderboard`) that story 3.1 deliberately deferred to this story and the two exit guards it requires. Baseline left open pending story 4.4's commit/merge. |
| 2026-08-12 | Baseline resolved to `bf0b93a` (4.4 merged via PR #12; tree byte-identical to `3fd277a`, branch advanced with `--ff-only`, zero file changes). |
| 2026-08-12 | Story 4.5 implemented — Tasks 1–13. Backend: `ShowLeaderboard` query/wrapper/engine method/route, plus `OpenNextQuestion`, `FinishGame`, `NextQuestion` and `StopGame` widened to admit `leaderboard`. Dashboard: secondary Leaderboard CTA at `revealed`, numbered primary at `leaderboard`, `leaderboard` added to `stoppableStates`. Display: `leaderboard-stage.tsx`, `use-leaderboard-memory.ts`, one `stageByState` entry, one optional `StageProps` field, one CSS ramp value and one animation. 32 new tests (19 stage, 9 hook, 4 shell) plus new Go engine/httpapi cases; three reds demonstrated first. |
| 2026-08-12 | Code review (bmad-code-review, three parallel adversarial layers). Both ACs and all 13 derived requirements verified; every gate re-run independently and reproduced green. 3 decisions resolved by Avraham (two → patch, one → defer), 9 patches applied, 5 items deferred, 8 findings dismissed. Product changes: focus handed to the persistent primary CTA when the Leaderboard button fires; the degraded-frame correlation moved into one shared `isDegradedFrame` in `stage-props.ts` after the stage's and the hook's copies were found to disagree; the unreachable outer `clamp` removed from the shift arithmetic. Test changes: two tie-delta rendering cases, both degraded disjuncts pinned, the vacuous no-dispatch assertion given a signal to wait on, and the shell's mount test widened to every state and re-anchored on the `<ol>`. Ledger: four statements in Task 12's records corrected against the code. 90 → 94 frontend tests, four reds demonstrated first. |
| 2026-08-12 | Task 12: eight `deferred-work.md` entries re-triaged — the 3.7 degraded-empty-leaderboard entry **resolved for the display half** (discriminated by a correlation of fields already on the wire, no wire-contract change) with the wire ambiguity re-pointed at 4.6; the 2.4 out-of-order entry **triggered, no high-water guard applied by decision**; the 3.1 discarded-REST-snapshot entry triggered and re-deferred; and written not-triggered records for the 2.5 spectator-roster, 3.7 scoring-race, SQL-coverage-family and 4.2 heading-semantics entries. |
