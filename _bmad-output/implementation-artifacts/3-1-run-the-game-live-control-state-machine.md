---
baseline_commit: 5be8425
---

# Story 3.1: Run the Game — Live Control State Machine

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an Organizer,
I want to start my Game and drive it state-by-state from the control panel,
so that pacing is always my explicit decision (FR-13).

## Acceptance Criteria

1. **Given** a Game in `lobby` with at least one Question, **when** I press "התחל משחק", **then** the engine transitions through the canonical state machine (`lobby → question_open → question_closed → revealed → [leaderboard] → … → finished`), each advance an explicit REST action, persisted before the snapshot broadcast — no auto-advance anywhere. *(epic AC-1)*

2. **Given** any game state, **then** the control panel shows exactly one primary CTA per UX-DR10 ("התחל משחק" / "פתח שאלה [N]" / "סגור שאלה" / "גלה תשובה" / "שאלה הבאה"), with "עצור" as the secondary stop control (renamed from "סיים משחק" per the UX revision 2026-07-09) always behind the confirm-stop dialog, so no state can be skipped accidentally, **and** from `revealed` I can skip the Leaderboard and open the next Question directly (UJ-4). *(epic AC-2 — scoped precisely in Dev Notes: "פתח שאלה [N]" is the Leaderboard-entry CTA, and this story does not build a path that reaches it — see "The central judgment call" below.)*

3. **Given** a Question opens, **then** the engine records the single server-side cutoff — the earlier of open-time + time limit, or my explicit "סגור שאלה" (FR-7 rule; enforced on intake in Story 3.3). *(epic AC-3 — this story persists the cutoff value; rejecting late answers against it is Story 3.3's job, since the `answers` table doesn't exist yet.)*

4. **Given** the keyboard, **then** Space fires the primary CTA only when focus is on `body` or the main region — never inside inputs or dialogs — focused buttons use native activation (no global handler racing them), and Escape has exactly one meaning: close the open dialog, never open one (UX-DR13); stopping the game is the visible "עצור" control, whose confirm dialog guards destructive advances on every path, keyboard included, with all copy from `strings.he.ts` (UX-DR11). *(epic AC-4)*

5. **Given** a server restart mid-Game, **when** the control panel reconnects, **then** the full state is recovered from Postgres with no corruption (NFR-2). *(epic AC-5 — satisfied by construction if Task 2 always rebuilds the snapshot from Postgres; verify, don't special-case.)*

## Tasks / Subtasks

- [x] **Task 1: Migration + store layer — question-progress tracking** (AC: 1, 3, 5)
  - [x] `server/migrations/00009_game_question_progress.sql`: add two columns to `games`, **NOT NULL with sentinel defaults** — this codebase deliberately avoids nullable columns (see `questions.correct_option DEFAULT 0` — comment: "every column NOT NULL with a neutral default so sqlc models stay pgtype-free"). Follow the same convention here, not `*time.Time`/`pgtype.Timestamptz`:
    ```sql
    -- +goose Up
    ALTER TABLE games
        ADD COLUMN current_question_position INTEGER NOT NULL DEFAULT 0,
        ADD COLUMN answer_cutoff_at timestamptz NOT NULL DEFAULT '1970-01-01T00:00:00Z';

    -- +goose Down
    ALTER TABLE games
        DROP COLUMN answer_cutoff_at,
        DROP COLUMN current_question_position;
    ```
    `current_question_position = 0` means "no question open" (mirrors `lobby`/`draft`/`finished`); `answer_cutoff_at` at its epoch sentinel is meaningless whenever `current_question_position = 0` — never read it without checking that guard first.
  - [x] `server/internal/store/queries/games.sql`: append five queries, each following `OpenGameLobby`'s established race-guard shape (`WHERE ... AND state = 'expected'`, comment explaining the guard):
    - `StartGameFirstQuestion` — `question_open` on question `position = 1`, guarded `state = 'lobby'`, sets `current_question_position = 1` and `answer_cutoff_at = now() + (q.time_limit_seconds || ' seconds')::interval` via `UPDATE games g ... FROM questions q WHERE q.game_id = g.id AND q.position = 1`. Zero rows covers **both** "not in lobby" and "no questions" in one statement — the engine (Task 2) disambiguates which one happened via a prior read, exactly like `OpenLobby` already disambiguates a lost race from a missing game.
    - `CloseCurrentQuestion` — `state = 'question_closed'`, `answer_cutoff_at = LEAST(answer_cutoff_at, now())` (an early explicit close tightens the cutoff; a close arriving after the timer already elapsed must not push it later), guarded `state = 'question_open'`.
    - `RevealCurrentQuestion` — `state = 'revealed'`, guarded `state = 'question_closed'`. No grading-completion gate — the `answers`/grading tables don't exist until Stories 3.3–3.6; Story 3.4 is where "activates only once every received answer is graded" gets added (its own AC says so). Don't invent a gate here with nothing to gate on.
    - `OpenNextQuestion` — same shape as `StartGameFirstQuestion` but guarded `state = 'revealed'` and parameterized on the target `position` (current + 1) instead of hardcoding `1`.
    - `FinishGame` — `state = 'finished'`, guarded `state IN ('question_open','question_closed','revealed')`. This single query serves **two** callers: `NextQuestion` when there's no question at `position + 1`, and the `StopGame` engine method (see Task 2) — don't write a second near-identical query.
  - [x] `sqlc generate` (via `make generate` or direct) — regenerates `internal/store/gen/games.sql.go` (five new methods) and `models.go` (`gen.Game` gains `CurrentQuestionPosition int32`, `AnswerCutoffAt time.Time`). Commit the generated diff; empty-diff-check in CI will otherwise fail.
  - [x] `server/internal/store/games.go`: add five thin wrapper methods (`StartGameFirstQuestion`, `CloseCurrentQuestion`, `RevealCurrentQuestion`, `OpenNextQuestion`, `FinishGame`), each mapping `pgx.ErrNoRows` → `store.ErrNotFound`, identical shape to `OpenGameLobby`.

- [x] **Task 2: `game` package — engine transitions, snapshot, errors** (AC: 1, 2, 3, 5)
  - [x] `server/internal/game/engine.go`: extend the `Store` interface with `ListQuestionsByGame(ctx, gameID, organizerID) ([]gen.Question, error)` plus the five Task-1 methods; `*store.Store` already satisfies both once Task 1 lands.
  - [x] New sentinel errors, same pattern as `ErrNotDraft`: `ErrNotLobby`, `ErrNoQuestions`, `ErrNotQuestionOpen`, `ErrNotQuestionClosed`, `ErrNotRevealed`, `ErrNotStoppable`.
  - [x] `StartGame(ctx, gameID, organizerID) (Snapshot, error)`: read game → `ErrNotLobby` if not `StateLobby` → `ListQuestionsByGame` → `ErrNoQuestions` if empty → `StartGameFirstQuestion` (race-guarded write) → `store.ErrNotFound` from that write means a concurrent transition won the race, reinterpret as `ErrNotLobby` (exactly like `OpenLobby` reinterprets its own race loss) → build and return the snapshot.
  - [x] `CloseQuestion(ctx, gameID, organizerID) (Snapshot, error)`: same shape, guard `StateQuestionOpen`, error `ErrNotQuestionOpen`.
  - [x] `Reveal(ctx, gameID, organizerID) (Snapshot, error)`: same shape, guard `StateQuestionClosed`, error `ErrNotQuestionClosed`.
  - [x] `NextQuestion(ctx, gameID, organizerID) (Snapshot, error)`: read game → `ErrNotRevealed` if not `StateRevealed` → `ListQuestionsByGame` → find the question at `position == g.CurrentQuestionPosition + 1` → if found, `OpenNextQuestion(..., position)`; if not found (this was the last question), `FinishGame(...)` — **this is where AC-2's "skip the Leaderboard and open the next Question directly" lives**: there is no intermediate leaderboard step in this story (see Dev Notes). Reinterpret a race-lost `store.ErrNotFound` from either write as `ErrNotRevealed`.
  - [x] `StopGame(ctx, gameID, organizerID) (Snapshot, error)`: read game → if `g.State` not in `{StateQuestionOpen, StateQuestionClosed, StateRevealed}`, `ErrNotStoppable` → `FinishGame(...)` (reusing Task 1's shared query) → build and return the snapshot. Not callable from `draft`, `lobby`, or `finished` — see Dev Notes for why lobby-abandonment is out of scope.
  - [x] Refactor the post-commit "snapshot build failed, degrade rather than fail" logic (currently inline in `OpenLobby`, lines ~78-83) into a small shared helper, e.g. `func (e *Engine) snapshotAfterCommit(ctx context.Context, g gen.Game) Snapshot` that logs the WARN and returns a best-effort snapshot on `buildSnapshot` failure — six call sites now need this, not one; don't duplicate the inline pattern five more times.
  - [x] `server/internal/game/snapshot.go`: add `QuestionCount int` and `CurrentQuestion *CurrentQuestion` to `Snapshot`; add:
    ```go
    type CurrentQuestion struct {
        ID               string   `json:"id"`
        Position         int      `json:"position"`
        Type             string   `json:"type"`
        Text             string   `json:"text"`
        Options          []string `json:"options,omitempty"`
        TimeLimitSeconds int      `json:"timeLimitSeconds"`
        AnswerCutoffAt   string   `json:"answerCutoffAt"` // RFC 3339 UTC, per architecture's Format Patterns
    }
    ```
    **Deliberately omits `CorrectOption`/`AcceptedAnswers` — always, at every state, including `revealed`.** `Snapshot` is the one payload both `role=host` and (from Epic 4) `role=display` receive over the same WS envelope; leaking the correct answer into it before/at reveal would hand it to a future Audience Display with no separate gate to add later. The reveal payload (marking which option was correct) is Story 3.4/4.4's job, once grading exists — don't build it now with nothing to grade.
  - [x] `buildSnapshot`: also call `ListQuestionsByGame`, set `QuestionCount = len(questions)`; when `g.CurrentQuestionPosition > 0`, find the question at that position and populate `CurrentQuestion` (nil otherwise — `draft`/`lobby`/`finished`).
  - [x] `server/internal/game/engine_test.go`: extend `stubStore` with the new `Store` methods (list-questions result/err, and one result/err/calls triple per new write method — same shape as `openGameLobbyResult/Err/Calls`). Add tests per method: success (correct state → correct snapshot fields, correct store calls), wrong-state rejection (write never attempted), race-loss reinterpretation (write's `store.ErrNotFound` → the specific `ErrNot*`), and for `StartGame` specifically the empty-questions case. For `NextQuestion`: both branches (opens position+1; finishes when no next question exists).

- [x] **Task 3: `httpapi` package — control endpoints, router wiring, error mapping** (AC: 1, 2)
  - [x] `server/internal/httpapi/control.go`: rename `LobbyEngine` → `ControlEngine` and add the five new methods to it (`StartGame`, `CloseQuestion`, `Reveal`, `NextQuestion`, `StopGame` — same signatures as Task 2's engine methods). This is a **same-file, same-parameter-count** change — `NewRouter`'s `engine` parameter already threads through to every call site; no new positional argument, no other file's `NewRouter(...)` call needs touching (verified: `router_test.go`/`games_test.go`/`packages_test.go` all pass `nil` for this slot when they don't exercise control routes).
  - [x] Five new handlers, each following `handleOpenLobby`'s exact shape (extract organizer → extract gameID → 5s-bounded context → call engine method → `writeStoreError` on failure → broadcast **before** writing the REST response → `slog.Info` → `writeJSON(200, snapshot)`):
    - `handleStartGame`
    - `handleCloseQuestion`
    - `handleReveal`
    - `handleNextQuestion`
    - `handleStopGame`
  - [x] `server/internal/httpapi/router.go`: inside the existing `if engine != nil && hub != nil { ... }` block (which already mounts `open-lobby`), add:
    ```go
    gr.Post("/start", handleStartGame(engine, hub))
    gr.Post("/close-question", handleCloseQuestion(engine, hub))
    gr.Post("/reveal", handleReveal(engine, hub))
    gr.Post("/next-question", handleNextQuestion(engine, hub))
    gr.Post("/stop", handleStopGame(engine, hub))
    ```
    Kebab-case action nouns under the game resource, matching `open-lobby`'s existing convention (architecture's API naming pattern).
  - [x] `server/internal/httpapi/errors.go`: extend `writeStoreError`'s switch with six new cases, each 409 (same status class as the existing `game.ErrNotDraft` → `GAME_NOT_EDITABLE` case — a valid game in the wrong state is a conflict, not a 404 or 400):
    | error | code | message |
    |---|---|---|
    | `game.ErrNotLobby` | `GAME_NOT_LOBBY` | "game must be in lobby state to start" |
    | `game.ErrNoQuestions` | `GAME_NO_QUESTIONS` | "game must have at least one question to start" |
    | `game.ErrNotQuestionOpen` | `GAME_NOT_QUESTION_OPEN` | "game must have an open question to close" |
    | `game.ErrNotQuestionClosed` | `GAME_NOT_QUESTION_CLOSED` | "question must be closed before it can be revealed" |
    | `game.ErrNotRevealed` | `GAME_NOT_REVEALED` | "question must be revealed before advancing" |
    | `game.ErrNotStoppable` | `GAME_NOT_STOPPABLE` | "game cannot be stopped from its current state" |
  - [x] `server/internal/httpapi/control_test.go`: rename `stubLobbyEngine` → `stubControlEngine`, add the five new methods (result/err/calls per method, same shape as the existing `OpenLobby` field triple). Add per-action tests mirroring the existing `TestOpenLobby*` suite: success (200, correct broadcast, correct engine args), each domain-error → its mapped status/code with **zero broadcast**, missing/foreign game → 404, no session → 401 with the engine never reached.
  - [x] `server/cmd/server/main.go`: no signature change. Update the existing wiring comment (currently: "st satisfies game.Store (GetGameForOrganizer, OpenGameLobby, ListParticipants, GetGameByJoinCode, CreateParticipant, UpdateParticipantNameByPhone)") to list the newly required methods too.

- [x] **Task 4: `web` — live control panel** (AC: 1, 2, 4)
  - [x] `web/src/lib/types.ts`: add `CurrentQuestion` (mirrors Task 2's Go struct, `answerCutoffAt: string`) and extend `LobbySnapshot` with `questionCount: number` and `currentQuestion: CurrentQuestion | null`.
  - [x] `web/src/lib/strings.he.ts`: new `live` section. Copy sourced verbatim from EXPERIENCE.md's "Host microcopy" table (lines 133-136) and the "Host control panel" state table (line 185) — do not paraphrase:
    - `startGameCta: 'התחל משחק'`, `closeQuestionCta: 'סגור שאלה'`, `revealCta: 'גלה תשובה'`, `nextQuestionCta: 'שאלה הבאה ←'`, `stopCta: 'עצור'`.
    - `stopConfirmTitle`/`stopConfirmBody`/`stopConfirmAction` — **`[ASSUMPTION]`**: EXPERIENCE.md specifies the confirm-stop dialog exists (line 138: "opens confirm dialog") but not its copy. Write reasonable Hebrew per UX-DR12 (say what happens, no vague "בטוח?"), e.g. title "לעצור את המשחק?", body "המשחק יסתיים ולא ניתן יהיה להמשיך אותו. משתתפים לא יקבלו הודעה על העצירה.", action "עצור את המשחק". Flag for Avraham to confirm/replace, same as Story 1.4's scoring-default assumption.
    - `questionProgress: (n: number, total: number) => \`שאלה ${n} מתוך ${total}\`` (reuses the WhatsApp template's "מתוך" phrasing from EXPERIENCE.md's Question message rows, for consistency).
    - `gameOverTitle`, `actionError` (generic retry copy, matching `strings.lobby.openLobbyError`'s pattern), `actionConflict` (generic "game state changed" copy — see Dev Notes on why per-action distinct conflict copy isn't needed).
  - [x] `web/src/features/lobby/lobby-page.tsx`: when `snapshot.state === 'lobby'`, add the "התחל משחק" button (mutation → `POST /api/games/{gameId}/start`, `disabled={isPending}`, error line on failure following the existing `openLobbyConflict`/generic-error branch pattern). When `snapshot.state` is anything past `lobby` (i.e. not `draft`, not `lobby`), render `<ControlPage gameId={gameId} snapshot={snapshot} />` instead of the roster view — **reuse the single `useGameSocket` subscription already open in `LobbyPage`; do not call `useGameSocket` again inside `ControlPage`** (a second call with the same key returns the same cached store per `use-game-socket.ts`'s module-level `stores` map, so it's harmless but redundant — pass `snapshot` down as a prop instead, it's simpler to reason about and matches how every other piece of this codebase avoids duplicate subscriptions).
  - [x] New `web/src/features/live/control-page.tsx` (per architecture's directory structure). Renders from the `snapshot` prop only — no own data fetching:
    - One persistent `<Button>` element (not conditionally-swapped different elements) whose label/onClick/disabled are computed from `snapshot.state` — this is what makes "focus is retained across state changes" (AC-4) work for free; a differently-keyed/conditionally-rendered button would remount and drop focus.
    - State → primary action mapping (leaderboard/"Between questions" is intentionally absent — see Dev Notes):
      | `snapshot.state` | primary CTA | POST path |
      |---|---|---|
      | `question_open` | `closeQuestionCta` | `close-question` |
      | `question_closed` | `revealCta` | `reveal` |
      | `revealed` | `nextQuestionCta` | `next-question` |
      | `finished` | — (render a minimal "game over" placeholder + link back to the games list; the full results summary is Story 3.10's job, not this one's) | — |
    - Secondary "עצור" button (states `question_open`, `question_closed`, `revealed` only — see Dev Notes on why not `lobby`), wrapped in `AlertDialog`/`AlertDialogTrigger`/`AlertDialogContent` (reuse `web/src/components/ui/alert-dialog.tsx` — already built, unused until now) with `AlertDialogAction` firing the `POST /stop` mutation. **Do not write custom Escape handling** — Radix's `AlertDialog` already closes on Escape and traps focus while open, which already satisfies AC-4's "Escape closes the open dialog, never opens one" for free; adding a second handler risks a double-fire race the AC explicitly warns against ("no global handler racing them").
    - Space-key handling: a `useEffect`-mounted `window` `keydown` listener, active only while a primary action exists, that fires the current primary mutation when `event.code === 'Space'` **and** `document.activeElement === document.body` (no dialog open, no input/button focused — a focused button already handles its own native Space activation, so the global handler must not also fire in that case, per AC-4's "no global handler racing them"). Clean up the listener on unmount/dependency change.
    - Question display: while `snapshot.currentQuestion` is set, show its `text`, `type`, and `questionProgress(position, questionCount)` — plain text, no timer ring, no MCQ option highlighting, no live answer count. **Scope note**: DESIGN.md's `timer-inline` component and `response-stats.tsx` (live answered count/%) are explicitly out of this story — both need data (a running countdown UI, the `answers` table) that either doesn't exist yet or wasn't required by this story's ACs. `snapshot.currentQuestion.answerCutoffAt` is persisted and broadcast so a later story can render the timer without a schema change.
  - [x] No changes to `web/src/app.tsx` — no new route. See Dev Notes ("one page, not two routes") for why.

- [x] **Task 5: Quality gates + local E2E + manual browser verification** (all ACs)
  - [x] Local gates: `gofmt -l .` (LF-normalize first, per 2.2/2.3/2.4/2.5's documented Windows/CRLF workaround) · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` (diff should be exactly the new methods/models from Task 1, nothing else) · `npm run lint` + `npx tsc -b --noEmit` under `web/` (no Vitest exists yet — see Dev Notes on why this story doesn't add it).
  - [x] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.3/2.4/2.5): boot the real binary against the Docker dev Postgres, log in as `e2e-org-a`, create a scratch game with ≥2 questions, walk the full REST sequence — `open-lobby` → `start` → `close-question` → `reveal` → `next-question` (opens question 2) → `close-question` → `reveal` → `next-question` (now finishes — no question 3) — asserting the returned/broadcast state and `currentQuestion` at each step, plus one negative case (e.g. `POST /reveal` while `question_open` → 409 `GAME_NOT_QUESTION_CLOSED`, game state unchanged). Clean up the scratch game row afterward.
  - [x] **Restart recovery (AC-5)**: as part of (or right after) the E2E above, with the game mid-sequence (e.g. `question_open`), restart the server process and confirm a fresh `GET /api/games/{id}` (or a new WS connection) returns the same state and `currentQuestion` — no in-memory state to lose, since `buildSnapshot` always reads Postgres fresh, but verify it rather than assume it.
  - [x] **Manual browser pass** (`make dev`, real browser — this story's frontend behavior can't be verified by the Go E2E): walk a full game lobby→start→close→reveal→next×N→stop through the actual UI. Confirm: the primary button's focus is visually retained across each state transition (tab to it once, then click through — focus ring should never jump away); Space triggers the current primary action only when nothing else is focused and no dialog is open; Space does nothing while a text input or the stop-confirm dialog has focus; Escape closes the stop-confirm dialog and does nothing elsewhere; the stop-confirm dialog's action actually ends the game and the page settles on the "game over" placeholder. No real-phone WhatsApp pass needed this story — nothing in `wa` changes.

### Review Findings

Code review 2026-08-04 (three-layer adversarial: Blind Hunter / Edge Case Hunter / Acceptance Auditor; 28 raw findings → 17 after dedup, 5 dismissed as noise, 12 actionable). Four decision-needed findings were resolved with the user same-session (see inline "Decision (2026-08-04)" notes below) and are now filed as patch/defer.

- [x] [Review][Patch] Question-position gaps break start/next — `DeleteQuestion` never renumbers and the editor reorders only on explicit move, so deleting a question leaves non-contiguous positions. A roster with no position-1 question makes `StartGame` permanently fail with the *wrong* 409 (`GAME_NOT_LOBBY` while the game is in lobby — a dead end, since no lobby→draft path exists); a gap after the current position makes `NextQuestion` silently `FinishGame` early, skipping remaining questions. **Decision (2026-08-04): renumber on delete** — `DeleteQuestion` closes the gap in the same transaction, restoring the 1..N invariant at its source (matches `ReorderQuestions`' existing pattern). **Fixed**: `DeleteQuestion` now runs in a transaction (`RETURNING position` + a new `CloseQuestionPositionGap` shifting every later position down by one). No backfill was needed — no gapped rosters exist in the dev DB today. [server/internal/store/queries/questions.sql, server/internal/store/questions.go]
- [x] [Review][Patch] Pre-existing unguarded writes are now reachable mid-game — the four 2.4/2.5 deferred items (join state-TOCTOU, spectator-into-finished flavor, cross-game `שם:` rename hijack, re-JOIN player told they're a spectator) plus question CRUD carrying no `state` predicate at all. **Decision (2026-08-04): add the guards now.** **Fixed, with a correction to this finding's original scope**: `CreateParticipant`'s INSERT now sources from `games g WHERE g.state = ANY(allowed_states)` (`joinLobby` passes `{lobby}`, `joinSpectator` passes the live states), and `CreateQuestion`/`UpdateQuestion`/`DeleteQuestion`/`UpdateQuestionPosition` all gained `AND state = 'draft'`, `ReorderQuestions`' transaction also re-checks state up front. This closes exactly **two** of the four 2.4/2.5 items (Join's state-TOCTOU; the spectator-into-finished flavor) — the other two (cross-game `שם:` rename hijack; re-JOIN player told they're a spectator) are a different code path (`UpdateParticipantNameByPhone`, `joinSpectator`'s role-blind branch) untouched by this patch and remain open in `deferred-work.md`, re-pointed since "unreachable until 3.1" no longer holds. [server/internal/store/queries/questions.sql, server/internal/store/queries/participants.sql, server/internal/game/participants.go, server/internal/store/questions.go, server/internal/store/participants.go]
- [x] [Review][Patch] Lobby-state keyboard/focus gap — Space never fires "התחל משחק" (the global handler lives only in ControlPage), the start CTA is a separate element that unmounts at lobby→question_open (focus drops by construction), and its `disabled={isPending}` reintroduces the exact disabled-drops-focus bug the Dev Record fixed on ControlPage's persistent button. **Decision (2026-08-04): minimal fix** — add the same Space-key effect (with the `event.repeat` fix below) to the lobby start button, and drop `disabled={isPending}` in favor of an in-handler pending guard, matching ControlPage's own fix. **Fixed**: new shared `web/src/lib/use-space-action.ts` hook used by both the lobby start button and ControlPage; the start button is now disabled only when `questionCount === 0` (a real, non-transient reason), never on pending. The lobby→question_open focus-continuity gap itself (the button unmounts into `ControlPage` on that transition) is unchanged by design — the user explicitly chose the minimal fix over unifying the two buttons. [web/src/lib/use-space-action.ts, web/src/features/lobby/lobby-page.tsx]
- [x] [Review][Patch] Space handler multi-fire: no `event.repeat` guard and the double-fire check reads `action.isPending` from a stale render-closure — held/rapid Space issues duplicate POSTs and, as each snapshot re-binds the effect to the new state's action, can chain close→reveal→next all the way to `finished`. **Fixed**: `useSpaceAction` ignores `event.repeat`; both ControlPage and the lobby start button now guard double-submits with a plain `useRef` set/cleared synchronously around `mutate()` (via `onSettled`), not `isPending` (which React Query doesn't flip synchronously inside `mutate()`). [web/src/lib/use-space-action.ts, web/src/features/live/control-page.tsx, web/src/features/lobby/lobby-page.tsx]
- [x] [Review][Patch] `actionConflict` copy instructs a manual page refresh ("רעננו את הדף"), contradicting EXPERIENCE.md's "no refresh dependency on live web surfaces" — the WS broadcast already self-heals the panel. **Fixed**: copy changed to "מצב המשחק השתנה בינתיים. הלוח יתעדכן אוטומטית." (no refresh instruction). [web/src/lib/strings.he.ts]
- [x] [Review][Patch] `GAME_NO_QUESTIONS` renders as the generic "game state changed — refresh" (permanently misleading; refreshing can never help) and the start button is not disabled when `questionCount === 0` even though the snapshot carries it. **Fixed**: new `strings.live.startGameNoQuestions` copy, selected by checking `ApiError.code === 'GAME_NO_QUESTIONS'`; start button disabled when `questionCount === 0`. [web/src/features/lobby/lobby-page.tsx, web/src/lib/strings.he.ts]
- [x] [Review][Patch] Mutation error banner never clears when a fresh WS snapshot advances the state — a transient 409 leaves "רעננו את הדף" rendered over an already-correct live panel. **Fixed**: a `useEffect` keyed on `snapshot.state` calls `action.reset()`/`stop.reset()` via a latest-ref pair (refs synced in their own no-deps effect, not during render — required by this repo's `react-hooks/refs` lint rule). [web/src/features/live/control-page.tsx]
- [x] [Review][Patch] Post-commit snapshot build runs on the request context — a client abort right after the guarded UPDATE commits degrades the broadcast *every* WS viewer receives to an empty snapshot. **Fixed**: `snapshotAfterCommit` now builds on `context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)` and the WARN wording was widened to "empty/questionless snapshot". [server/internal/game/engine.go]
- [x] [Review][Patch] `NewRouter` doc comment still says nil engine/hub omits "the open-lobby route" — it now silently omits six control routes. **Fixed**: comment now names all six routes. [server/internal/httpapi/router.go]
- [x] [Review][Patch] No polite live region announces state changes on the control panel — EXPERIENCE.md's persistent-button pattern (line 192) pairs the label swap with an `aria-live` announcement; only `role="alert"` error lines exist. **Fixed**: added a `sr-only` `aria-live="polite" role="status"` element carrying the primary CTA's current label. [web/src/features/live/control-page.tsx]
- [x] [Review][Defer] Broadcast `seq` is assigned at Broadcast-call time, not commit order — deferred, pre-existing pattern (mirrors 2.4's out-of-order lobby snapshots), sharpened trigger recorded in deferred-work.md [server/internal/ws/hub.go:153-161, server/internal/httpapi/control.go]
- [x] [Review][Defer] Control mutations discard the REST response snapshot (`api<void>`); the panel depends on a single WS frame that the hub drops-on-full (8-slot queue) with no redelivery — a dropped `finished` frame is terminal until manual reconnect. Deferred: established single-source-WS posture from 2.3; changing it is an architecture call, and the drop window is small at pilot scale [web/src/features/live/control-page.tsx:55-59, server/internal/ws/hub.go:172-178] — deferred, pre-existing

Dismissed as noise (5): draft-state games routed to ControlPage (false positive — the draft branch returns earlier); `FinishGame` leaves a future `answer_cutoff_at` (dismissed by the documented invariant: cutoff is meaningless when `current_question_position = 0`, which FinishGame sets); `time_limit_seconds = 0` opens an expired question (unreachable — httpapi validates 5–300); RFC 3339 sub-second truncation (spec-mandated format, display-only); `[ASSUMPTION]` stop-dialog copy shipped unconfirmed (already tracked in this story's Completion Notes for Avraham's confirmation).

## Dev Notes

### The central judgment call — "שאלה הבאה" always skips the Leaderboard in this story

EXPERIENCE.md's Host control panel table (line 183-190) and its State Patterns table (line 204-216) read, at face value, as if they disagree: the control-panel table gives `revealed` exactly **one** primary CTA ("שאלה הבאה", secondary "עצור") — no separate "show Leaderboard" button anywhere in the canonical Host-microcopy list (lines 133-136, which names only Open/Close/Reveal/Next) — while the State-Patterns table's Reveal row lists the Host Dashboard's options as "Next / Leaderboard", implying two distinct choices.

Flow 4 (UJ-4, lines 315-321) resolves this concretely, and is the most literal, unambiguous source available: *"שלמה לוחץ 'גלה תשובה' ... שלמה מדלג על הלוח: לוחץ ישר 'שאלה הבאה'. הכפתור הראשי הוא אותו כפתור — רק התווית מתחלפת"* ("Shlomo skips the board: clicks straight 'next question'. The primary button is the same button — only the label changes"). Clicking the one documented `revealed`-state CTA **is** the skip action — the flow narrates it as remarkable, but it's still the plain, undecorated "שאלה הבאה" click. Epic AC-2 says the identical thing in its own words: "...with 'שאלה הבאה' [as one of the primary CTAs]... And from revealed I can skip the Leaderboard and open the next Question directly (UJ-4)" — the AC does not describe a second button; it equates the one listed CTA with the skip.

**This story therefore implements `revealed → question_open` (or `→ finished`) as the only path out of `revealed`, and never implements a control that enters `leaderboard`.** The `leaderboard` state stays a valid, already-migrated enum value (untouched CHECK constraint) but has zero reachable code path until Epic 4 Story 4.5 ("Leaderboard Stage") adds it — which makes sense on its own terms: a Leaderboard pause is a spectacle for the projected room, and the Audience Display doesn't exist until Epic 4 (Story 3.1's own architecture note confirms this reading: "a full game is playable end-to-end via WhatsApp **without the projected screen**"). Building a "show Leaderboard" host control now, with nothing to show it *on*, would be exactly the kind of speculative work the project's own conventions reject. `[ASSUMPTION — flag for Avraham/reviewer: if this reading is wrong and a Leaderboard-entry action is actually wanted in Epic 3, it changes this story's engine surface (a sixth transition) and epic Story 4.5's AC-1 "Given the Organizer advances to leaderboard" then has nowhere new to hook — better to confirm before 4.5, not after.]`

One consequence: "פתח שאלה [N]" (the "Between questions" row's CTA) never renders in this story — AC-2's text is quoted in full above because it's the epic's authoritative wording, but only four of its five named CTAs ("התחל משחק" / "סגור שאלה" / "גלה תשובה" / "שאלה הבאה") have an exercised path.

### Second judgment call — "עצור" scope: not from `lobby`

Epic AC-2 says "עצור" is the secondary control for "any game state." EXPERIENCE.md's own control-panel table is narrower: it lists "עצור" only for "Between questions" (unreachable, see above) and "Answer revealed," and gives `lobby` a **different** secondary ("פתח מסך קהל" — launch display). This story reconciles the two by implementing `StopGame` for `{question_open, question_closed, revealed}` only — every state where a live round is actually running and an organizer might need to abort mid-event — and explicitly **not** from `lobby`: nothing is running yet for anyone to need rescuing from, EXPERIENCE.md's own table doesn't put "עצור" there, and there is no existing "abandon a lobby" affordance anywhere in the system to extend. `[ASSUMPTION]`. "פתח מסך קהל" (launch display) itself is out of scope for this story regardless — `/display/:gameId` doesn't exist until Epic 4 Story 4.1.

### Why one page, not the two routes architecture.md sketched

architecture.md's directory listing shows `features/lobby/lobby-page.tsx` and `features/live/control-page.tsx` as if they were reached by different routes. Story 2.3 already built `lobby-page.tsx` as a single component that branches internally on `snapshot.state` (`draft` vs. everything else) rather than routing between two pages — and that pattern is the better fit here too: the WS snapshot already re-renders the *same mounted component* on every state change with no navigation, which is simpler and strictly safer for AC-4's focus-retention requirement than introducing a route transition (which would unmount/remount and lose focus by construction). This story keeps `control-page.tsx` as its own **file** (matching architecture's intent to separate lobby concerns from live-control concerns) but has `LobbyPage` render `<ControlPage>` directly once `state` moves past `lobby`, sharing the one `useGameSocket` subscription already open, rather than adding a `/live` route. No route/link elsewhere in the app needs to change.

### What FR-13's "live response count and percentage" and the timer do NOT get built here

Both need data this story doesn't have: live answer counts need the `answers` table (Story 3.3), and a decorated `timer-inline` ring is meaningful UI polish once there's something to watch tick down toward an enforced cutoff (also 3.3). `snapshot.currentQuestion.answerCutoffAt` is computed and broadcast correctly by this story (AC-3) specifically so a later story can add the countdown display and the intake rejection without a schema migration — but building the ring now, unenforced, would be UI theater with no backing correctness. `response-stats.tsx`, named in architecture's directory listing, is not created by this story.

### Deferred-work.md items this story's arrival makes newly reachable — and what this story does and doesn't do about them

Several `deferred-work.md` entries were explicitly deferred *because* no code path moved a game past `lobby` — that stops being true the moment `StartGame` ships. Read all of these before touching `participants.go` or `wa/inbound.go` — **this story does not modify either file**, so none of them are fixed here, but "unreachable" is no longer an accurate status for any of them after this story merges:

- **Join's state-check→INSERT TOCTOU** (deferred from 2.4, explicit trigger "story 3.1... decide the concurrency posture for ALL state-guarded mutations together"): this story's own five new transitions **do** establish that posture — every one uses the same guarded-`UPDATE ... WHERE state = 'expected'` pattern `OpenGameLobby` already used, so the "decide the posture" question is answered for all *new* code this story adds. The **pre-existing** gap is specifically in `participants.go`'s `Join`/`CreateParticipant` path (a different package, no ACs in this story touch it) and remains open. Flag to the code-review triage session whether it should be pulled into this story's scope or opened as its own fast-follow — do not silently leave it looking "resolved by 3.1" in `deferred-work.md`.
- **Player mis-labeled as spectator on re-JOIN**, **spectator flavor of the TOCTOU**, **cross-game `שם:` rename hijack** (all deferred from 2.5, all triggered on "story 3.1"): same disposition — genuinely reachable for the first time after this story, none touched by this story's files, all still open. Same triage flag applies.
- **Role-blind snapshot roster count** (deferred from 2.5, trigger: *"story 3.1's control panel or Epic 4's audience displays — the first surface rendering a live roster decides"*): this story's `control-page.tsx` does **not** render a participant roster (only the current question + CTA — see Task 4), so this item is not resolved by this story either. Correct the trigger's assumption when updating `deferred-work.md`: point it forward past 3.1, to whichever surface first renders a live roster past lobby (plausibly never needed, or Epic 4).

### Existing code this story modifies — current state, and what must survive

- **[server/internal/game/engine.go](server/internal/game/engine.go)** — currently only `OpenLobby` + `Snapshot` (read path). `Store` interface currently: `GetGameForOrganizer`, `OpenGameLobby`, `ListParticipants`, `GetGameByJoinCode`, `CreateParticipant`, `UpdateParticipantNameByPhone`. Must survive unchanged: `OpenLobby`'s full behavior (including its own race-loss reinterpretation and post-commit degrade), `buildSnapshot`'s participant-building logic (only extend it, don't restructure it).
- **[server/internal/game/snapshot.go](server/internal/game/snapshot.go)** — `Snapshot`/`ParticipantSummary` currently carry no question data at all. Adding `QuestionCount`/`CurrentQuestion` must not change the wire shape of any existing field (camelCase names, JSON tags already established).
- **[server/internal/httpapi/control.go](server/internal/httpapi/control.go)** — currently exactly `LobbyEngine` (one method), `SnapshotBroadcaster`, `handleOpenLobby`. The rename to `ControlEngine` is additive to the interface, not a behavior change to `handleOpenLobby` itself.
- **[server/internal/httpapi/router.go](server/internal/httpapi/router.go)** — the `if engine != nil && hub != nil` block currently mounts one route (`open-lobby`); must keep that route working unchanged while adding five siblings inside the same guard.
- **[server/internal/httpapi/errors.go](server/internal/httpapi/errors.go)** — `writeStoreError`'s switch currently has three cases (`store.ErrNotFound`, `store.ErrReorderMismatch`, `game.ErrNotDraft`) plus a default that logs `slog.Error` and returns 503. The six new cases are additive; the default arm (infrastructure failures) must stay last and unchanged.
- **[web/src/features/lobby/lobby-page.tsx](web/src/features/lobby/lobby-page.tsx)** — currently branches only on `state === 'draft'` (open-lobby button) vs. else (roster view, unconditionally, for every other state). This story adds a **third** branch (state past lobby → `ControlPage`) and a start button on the `state === 'lobby'` case of the *existing* else-branch. Must survive unchanged: the `socketNotFound`/`openLobbyNotFound` dead-end handling, the `!snapshot` connecting state, the join-code/platform-number header, the single `useGameSocket` call.
- **[web/src/lib/use-game-socket.ts](web/src/lib/use-game-socket.ts)**, **[web/src/lib/api.ts](web/src/lib/api.ts)** — no changes. `ControlPage` consumes the snapshot as a prop and calls `api()` directly for its mutations, same as `LobbyPage` already does for `open-lobby`.
- **[server/cmd/server/main.go](server/cmd/server/main.go)** — `NewRouter(...)`'s call already passes `engine` positionally where the new routes get mounted; **no argument-count change**. Only the explanatory comment about which `game.Store` methods `st` must satisfy needs updating.

### Architecture guardrails (violations = rework)

- **Dependency direction unchanged**: `httpapi`/`ws` → `game` → `store`. This story adds no new import edges and touches no `wa` code at all (question *delivery* is Story 3.2; this story only flips state and records the cutoff).
- **State mutation is centralized**: every new transition writes through `game.Engine` → `store`, exactly like `OpenLobby`. No handler or component ever sets `games.state` directly.
- **Glossary**: `Game`, `Question`, `Organizer` verbatim; no `quiz`/`session`/`host` identifiers.
- **Race-guard convention**: every new SQL write's `WHERE` clause names its expected source state(s) explicitly (Task 1) — this is now the established, repeated pattern for state-guarded mutations in this codebase, not a one-off.
- **Copy centralization**: every new Hebrew string lives in `strings.he.ts` only (`live` section) — CI's Hebrew-literal grep enforces this on `.go` files already; the same discipline applies to `.tsx`.
- **Dates**: `answerCutoffAt` is an RFC 3339 UTC string, never "seconds remaining" (architecture's Format Patterns) — the client computes the countdown, when a later story adds one; this story only needs to carry the value correctly.
- **Logging (NFR-8)**: `slog.Info` on every successful transition (state, gameID, organizerID — matches `handleOpenLobby`'s existing "game lobby opened" line); no new WARN paths this story introduces beyond the existing post-commit-degrade one, now shared across six methods.

### Previous story intelligence (2.3/2.4/2.5 — established patterns, reused verbatim)

- **Encoding discipline (Windows tax)**: write new/edited files with the Write tool only, never PowerShell redirection (emits UTF-16 + BOM) — relevant to both the new migration file and the new Hebrew strings.
- **`gofmt -l .` false-positives on Windows** (`core.autocrlf=true`) — LF-normalize before piping through `gofmt`; the same ~9 pre-existing files will still show up and can be ignored (documented in 2.2/2.3/2.4/2.5).
- **Go 1.26.5 via the `go1.26.5` wrapper** (system `go` is older); `go test -race` is not available in this environment.
- **Captured-slog testing** (`slog.New(slog.NewTextHandler(&buf, nil))`) is the house pattern for asserting log level/absence — reuse for the new transition-success INFO lines if a test needs to assert one.
- **Dev DB carries `e2e-org-a`/`e2e-org-b`** test organizers, reusable for Task 5's E2E; clean up scratch rows afterward (direct `DELETE FROM games WHERE id = $1`, cascades to `questions`).
- **No frontend test framework exists yet** (`web/package.json` confirmed — no Vitest, no `test` script). This story is the first to add genuinely tricky, stateful frontend behavior (persistent-button focus retention, a global keydown listener with focus-target gating) that a unit test would meaningfully protect — flag this as worth reconsidering for the *next* frontend-heavy story if manual verification proves fragile in practice, but this story follows every predecessor's precedent and does not introduce Vitest itself (a testing-framework bootstrap is a separate initiative, not a rider on this one).
- **2.3's `deferred-work.md` entry on `role=display` reusing the full organizer session** — irrelevant to this story (no display code touched), but worth knowing before Epic 4 reuses `Snapshot`/`ws` further.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in-test (extend `stubStore` and `stubControlEngine`/`stubLobbyEngine` in place — no mock framework). No real-DB unit tests; Task 5's throwaway `cmd/e2escratch` binary is the DB-touching verification, deleted after the run (2.3/2.4/2.5 pattern). No frontend test framework exists and this story does not add one — Task 5's manual browser pass is the frontend verification method, same posture as every prior frontend story.

### Project Structure Notes

**New:**
- `server/migrations/00009_game_question_progress.sql`
- `web/src/features/live/control-page.tsx`

**Modified (backend):** `server/internal/store/queries/games.sql` (+5 queries) · `server/internal/store/gen/games.sql.go` + `models.go` (sqlc-regenerated) · `server/internal/store/games.go` (+5 wrapper methods) · `server/internal/game/engine.go` (+5 transition methods, extended `Store` interface, shared degrade helper) · `server/internal/game/engine_test.go` · `server/internal/game/snapshot.go` (+`QuestionCount`, +`CurrentQuestion`) · `server/internal/httpapi/control.go` (`LobbyEngine`→`ControlEngine`, +5 handlers) · `server/internal/httpapi/control_test.go` · `server/internal/httpapi/router.go` (+5 routes) · `server/internal/httpapi/errors.go` (+6 error mappings) · `server/cmd/server/main.go` (comment only).

**Modified (frontend):** `web/src/lib/types.ts` (+`CurrentQuestion`, extended `LobbySnapshot`) · `web/src/lib/strings.he.ts` (+`live` section) · `web/src/features/lobby/lobby-page.tsx` (start button + delegate to `ControlPage`).

**Untouched:** everything under `server/internal/wa/*` (no dispatch, no message copy — that's 3.2) · `server/internal/ws/*` (reuses `Broadcast` unchanged) · `server/internal/auth/*` · `server/internal/store/participants.go` + `queries/participants.sql` (the deferred-work TOCTOU items above are read-only context for this story, not a to-do) · `web/src/app.tsx` (no new route) · `web/src/lib/use-game-socket.ts` · Epic 1 authoring surfaces · Epic 4 packages (don't exist yet).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.1] — story + all 5 epic ACs verbatim, Epic 3 context, FR-13
- [Source: _bmad-output/planning-artifacts/architecture.md#Project-Structure-Structure-Boundaries] — `game`/`httpapi`/`ws` directory listing and dependency-direction rule; `games.state` enum; `answers`/grading tables not yet built (confirms Story 3.3/3.4 own the cutoff-enforcement and reveal-gate work this story only lays groundwork for)
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Host-control-panel] (lines 179-192) — the primary/secondary CTA table this story's state→action mapping is built from
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Host-microcopy] (lines 129-142) — canonical CTA copy strings, verbatim
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Flow-4] (lines 315-321, UJ-4) — the walkthrough resolving the Leaderboard-skip ambiguity (this story's central judgment call)
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Interaction-Primitives] (lines 218-227) — Space/Escape keyboard-shortcut rules, verbatim (AC-4)
- [Source: server/internal/game/engine.go, snapshot.go, state.go] — existing `OpenLobby`/`Snapshot`/state-enum machinery this story extends
- [Source: server/internal/httpapi/control.go, control_test.go, router.go, errors.go, games.go] — existing control-route, error-mapping, and GameStore patterns this story's new endpoints must match
- [Source: server/internal/store/games.go, queries/games.sql, gen/models.go] — `OpenGameLobby`'s race-guard SQL shape, the `questions` table's NOT-NULL-sentinel convention this story's migration follows
- [Source: web/src/features/lobby/lobby-page.tsx, lib/use-game-socket.ts, lib/types.ts, lib/strings.he.ts, components/ui/alert-dialog.tsx, components/ui/button.tsx] — existing frontend surfaces and primitives this story extends/reuses
- [Source: _bmad-output/implementation-artifacts/2-5-late-join-becomes-spectator.md] — established Windows/encoding/testing house patterns; the four `deferred-work.md` items this story's arrival makes newly reachable
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#2-4, #2-5] — the TOCTOU/role items explicitly naming "story 3.1" as their revisit trigger; this story's disposition on each is recorded in Dev Notes above

## Dev Agent Record

### Agent Model Used

Claude Sonnet 5 (claude-sonnet-5), via the `bmad-dev-story` workflow.

### Debug Log References

- `sqlc generate` diff after Task 1 was exactly the five new query methods + `Game.CurrentQuestionPosition`/`Game.AnswerCutoffAt` in `models.go` — confirmed via `git diff --stat` before touching any other file.
- `gofmt -l .` (LF-normalized per the documented Windows/CRLF workaround) flagged exactly the same 9 pre-existing files 2.2–2.5 already documented (`store/wa_messages.go`, six `internal/wa/*.go` files) — no new violations from this story's files.
- `go1.26.5 vet ./...` and `go1.26.5 test ./...` clean on every run; `npm run lint` / `npx tsc -b --noEmit` under `web/` clean (one `react-hooks/exhaustive-deps` warning on the Space-key effect, resolved by depending on the whole `action` mutation object instead of two of its members).
- Local Go E2E (`cmd/e2escratch`, deleted after use) against the Docker dev Postgres, logged in as `e2e-org-a` (re-provisioned with a known scratch password for this run — the account's prior password was unknown). First run hit a **stale leftover `go run ./cmd/server` process already bound to port 8080** from an earlier session — every request silently exercised pre-story routes/behavior instead of the new binary; diagnosed via `netstat`/`tasklist`, confirmed via the process's start time and temp-dir path, killed, re-verified the new binary actually bound the port before re-running. Full sequence (`open-lobby → start → close-question → reveal → next-question(→q2) → close-question → reveal → next-question(→finished)` + one negative case) passed after that; scratch game row deleted afterward.
- **Restart recovery (AC-5)**: mid-sequence at `question_open`, killed the server process and started a fresh one against the same Postgres; a subsequent `GET /api/games/{id}` returned `question_open` unchanged, and the next real action's response snapshot confirmed `currentQuestion.position` also survived — no in-memory state lost, as `buildSnapshot` reads Postgres fresh on every call.
- **Real bug found by the E2E, fixed before it, not after**: `FinishGame`'s SQL only set `state = 'finished'`, leaving `current_question_position` at its last live value — so a finished game's snapshot kept reporting a stale `currentQuestion`, contradicting the migration's own documented invariant ("0 means no question open, mirrors draft/lobby/finished"). Fixed by adding `current_question_position = 0` to `FinishGame`'s `SET` clause (`server/internal/store/queries/games.sql`), regenerated, and updated the two engine tests that had encoded the old (buggy) expectation.
- Manual browser pass: `make dev` (Go API :8080 + Vite :5173, proxied), driven with a throwaway Playwright script (not committed) since no `chromium-cli` was available in this environment — logged in as `e2e-org-a` via the real login form, created a game + 2 questions via the authenticated session, then drove the actual `LobbyPage`/`ControlPage` UI through the full lobby→start→close→reveal→Space-triggered-reveal→stop-confirm→finished sequence. Found and fixed a real bug: the primary button was `disabled` while its mutation was pending, and a `disabled` button loses DOM focus in every browser the instant it's disabled — silently breaking AC-4's "focus retained across state transitions" on every single click. Fixed by never disabling the persistent primary button on `isPending`; double-submit is guarded inside the click handler instead. Re-ran to green (14/14 checks): focus retention across transitions, Space firing the primary action from `document.body`, focus correctly leaving `document.body` the instant the stop-confirm dialog opens (so the global Space handler's own guard would not fire), Escape closing the dialog and doing nothing when none is open, the stop flow actually ending the game and landing on the "game over" placeholder with a working back-link, and zero console errors throughout. Scratch games and dev processes cleaned up afterward.

### Completion Notes List

- All 5 tasks and their subtasks complete; all 5 ACs satisfied.
- Two real bugs surfaced and fixed during Task 5 verification (not left for code review): (1) `FinishGame` wasn't resetting `current_question_position`, leaking a stale `currentQuestion` into finished-game snapshots; (2) the live control panel's primary button was disabled during its own mutation, which drops browser focus and broke the AC-4 focus-retention requirement it exists to satisfy. Both are covered by adjusted/new assertions (engine unit tests for the first; the manual Playwright pass for the second, not committed — no frontend test framework exists yet, consistent with every prior frontend story).
- `[ASSUMPTION]` carried from the story's Dev Notes, unchanged: the stop-confirm dialog copy (`stopConfirmTitle`/`stopConfirmBody`/`stopConfirmAction`) is original Hebrew per UX-DR12, not sourced from EXPERIENCE.md (which specifies the dialog exists but not its copy) — flagged for Avraham to confirm/replace.
- No real-phone WhatsApp verification needed or performed — this story adds no code under `server/internal/wa/*`.
- `deferred-work.md` was read per the story's Dev Notes but not edited: the four 2.4/2.5 items it names as "reachable after story 3.1" are genuinely reachable now, but none are touched by this story's files (confirmed by File List below) — updating their status is left to the code-review triage session as the story's Dev Notes direct.

### File List

**New:**
- `server/migrations/00009_game_question_progress.sql`
- `web/src/features/live/control-page.tsx`

**Modified (backend):**
- `server/internal/store/queries/games.sql`
- `server/internal/store/gen/games.sql.go` (sqlc-regenerated)
- `server/internal/store/gen/models.go` (sqlc-regenerated)
- `server/internal/store/games.go`
- `server/internal/game/engine.go`
- `server/internal/game/engine_test.go`
- `server/internal/game/snapshot.go`
- `server/internal/httpapi/control.go`
- `server/internal/httpapi/control_test.go`
- `server/internal/httpapi/router.go`
- `server/internal/httpapi/errors.go`
- `server/cmd/server/main.go` (comment only)

**Modified (frontend):**
- `web/src/lib/types.ts`
- `web/src/lib/strings.he.ts`
- `web/src/features/lobby/lobby-page.tsx`

**Not committed (throwaway verification tooling, deleted before finishing):**
- `server/cmd/e2escratch/main.go` (Task 5 local Go E2E)
- a scratch Playwright script under the session scratchpad (Task 5 manual browser pass)

## Change Log

- 2026-08-04: Code review findings applied (three-layer adversarial review; see Review Findings above for the full list). All 10 `patch` items fixed, 4 `decision-needed` items resolved with the user and filed as patch/defer, 2 `defer` items recorded in `deferred-work.md` with sharpened triggers. Notably: `DeleteQuestion` now renumbers positions transactionally; `CreateParticipant` and question CRUD gained state guards closing two of the four 2.4/2.5 TOCTOU deferrals (the other two remain open, re-pointed); a shared `useSpaceAction` hook fixes the Space-key multi-fire bug and extends keyboard support to the lobby's start button; `snapshotAfterCommit` survives a client-aborted request; several UX-constraint deviations (refresh-instructing copy, a non-clearing error banner, a missing aria-live region) were corrected. All Go quality gates green (`gofmt`, `vet`, `test`) and all frontend gates green (`eslint`, `tsc`) after the patch. Status: done.
- 2026-08-04: Dev implementation complete (all 5 tasks, all 5 ACs) — see Dev Agent Record for the two real bugs found and fixed during Task 5 verification (`FinishGame` stale `current_question_position`; live control panel's primary button losing focus when disabled mid-mutation). Status: review.
- 2026-08-04: Story created by create-story workflow — full-context analysis: epics Story 3.1 + Epic 3 overview, architecture's project structure/dependency-direction/naming patterns, EXPERIENCE.md's Host control panel table, Host microcopy table, Flow 4 (UJ-4) walkthrough, and State Patterns table (surfacing and resolving a real discrepancy between the control-panel table and the state-patterns table over how/whether the Leaderboard is entered — see Dev Notes), a live read of `engine.go`, `snapshot.go`, `state.go`, `engine_test.go`, `control.go`, `control_test.go`, `router.go`, `router_test.go`, `errors.go`, `games.go` (store + httpapi), `queries/games.sql`, `queries/questions.sql`, `gen/models.go`, `main.go`, migrations 00003/00008, `lobby-page.tsx`, `use-game-socket.ts`, `types.ts`, `strings.he.ts`, `alert-dialog.tsx`, `app.tsx`, and `deferred-work.md` in full; confirmed via grep that no `NewRouter(...)` call site outside `control_test.go` constructs a real engine stub (all others pass `nil`), which grounded the decision to extend the existing `LobbyEngine`/`ControlEngine` interface in place rather than add a new `NewRouter` parameter. Marked epic-3 in-progress in sprint-status.yaml (first story of the epic). Status: ready-for-dev.
