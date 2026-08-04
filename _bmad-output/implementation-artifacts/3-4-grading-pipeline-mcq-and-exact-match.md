---
baseline_commit: b21ed05bf3c0cd44882eeb68a552ed105f1ff221
---

# Story 3.4: Grading Pipeline — MCQ and Exact Match

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an Organizer,
I want answers graded automatically as they arrive, without leaking grades early,
so that Reveal is instant and fair (FR-15, FR-16 exact stage).

## ⚠️ Prerequisite: Story 3.3 is not finished

At the time this story was created, **Story 3.3 (Answer Intake) is only partially implemented and uncommitted** — its status is `in-progress`, not `done`. What exists on disk today (verified by reading the actual files, not 3.3's story doc): `server/internal/game/answers.go` (`Engine.RecordAnswer`, MCQ/Free-Text parsing, `AnswerOutcome`), `server/internal/store/answers.go` + `queries/answers.sql` (`GetOpenQuestionForPlayer`, `RecordAnswer`, `CountAnswersByQuestion`), `server/migrations/00010_answers.sql`, and `engine.go`'s `Store` interface extension — this is 3.3's Tasks 1–2. **Not yet done**: 3.3's Task 3 (`CurrentQuestion.AnsweredCount` on the snapshot), Task 4 (the five `wa` templates + `handleTextOrAnswer` routing so replies actually reach `RecordAnswer`), Task 5 (the web `ResponseStats` pill), Task 6 (tests — no `answers_test.go` exists yet), and 3.3's own quality gates/E2E have not run.

**Do not start this story's implementation until 3.3 reaches `done`** (or at minimum, until Task 4 lands — this story's E2E needs a real WhatsApp-reply path to exercise grading). This story's tasks below describe the *current, already-on-disk* shape of `game/answers.go`, `store/answers.go`, and `queries/answers.sql` as the starting point — re-verify they still match before writing code, in case 3.3's remaining tasks changed them.

## Acceptance Criteria

1. **Given** an MCQ answer, **when** it is recorded (`Engine.RecordAnswer`), **then** it is graded mechanically against the question's `correct_option` and the verdict is persisted in the same INSERT that records the answer (`answers.is_correct`, `answers.stage = 'mcq'`) — never disclosed to the Participant (the ack stays "התקבל ✓" regardless of correctness) or on any live surface before Reveal. *(epic AC-1, FR-15)*
2. **Given** a Free-Text answer, **when** it is recorded, **then** the Exact stage compares the trimmed response against every one of the question's `accepted_answers` by literal string equality; a match persists `stage = 'exact'`, `is_correct = true`; a miss persists `stage = 'exact'`, `is_correct = false` (this story implements Exact only — Fuzzy/AI, Stories 3.5/3.6, are the "later stages" an exact match must never invoke, and are also what a miss would fall through to once they exist). *(epic AC-2, FR-16)*
3. **Given** a Question in `question_closed`, **when** the Organizer presses "גלה תשובה" (`POST /reveal`), **then** the transition to `revealed` succeeds only if every recorded answer for that Question has been graded (`stage IS NOT NULL`); an outstanding-grade game returns a distinct `GRADING_INCOMPLETE` 409, not a generic `GAME_NOT_QUESTION_CLOSED`. In this story grading is fully synchronous, so this gate is never observably reachable end-to-end — it is the structural mechanism Story 3.6's async AI stage will actually stress. *(epic AC-3, FR-16)*

## Tasks / Subtasks

- [ ] **Task 1: Migration — grading columns on `answers`** (AC: 1, 2, 3)
  - [ ] New `server/migrations/00011_answer_grading.sql`:
    ```sql
    -- +goose Up
    -- Nullable now even though this story's write path (game.RecordAnswer)
    -- always supplies both together, synchronously — forward-compatible
    -- with Story 3.6, where an AI verdict may not be known at INSERT time
    -- and a row could (in that story's design) be inserted ungraded and
    -- updated once the async call resolves. The shape CHECK keeps the two
    -- columns from ever drifting into a half-graded state in the meantime.
    ALTER TABLE answers
        ADD COLUMN is_correct BOOLEAN,
        ADD COLUMN stage TEXT CHECK (stage IN ('mcq','exact')),
        ADD CONSTRAINT answers_grading_shape CHECK ((is_correct IS NULL) = (stage IS NULL));

    -- +goose Down
    ALTER TABLE answers
        DROP CONSTRAINT answers_grading_shape,
        DROP COLUMN stage,
        DROP COLUMN is_correct;
    ```
    Story 3.5/3.6 will each widen the `stage` CHECK (`'fuzzy'`, `'ai'`) in their own migrations — do not pre-add those values now.

- [ ] **Task 2: New `grading` package — MCQ mechanical + Exact stage** (AC: 1, 2)
  - [ ] New `server/internal/grading/pipeline.go`:
    ```go
    // Package grading implements FR-15/FR-16's answer-correctness pipeline —
    // MCQ mechanical grading and Free-Text's Exact → Fuzzy → AI stages
    // (Fuzzy lands in Story 3.5, AI in Story 3.6; this story is Exact only).
    // Pure functions, no store/DB dependency — game calls in, store persists
    // the verdict game hands back (architecture: game imports grading).
    package grading

    import "strconv"

    // Stage identifies which grading stage produced an answer's verdict —
    // persisted verbatim as answers.stage (FR-16: "recording which stage
    // matched"; NFR-9 audit trail). Plain string alias, not a distinct
    // type — same reasoning as game.State: the DB column is TEXT + CHECK,
    // and a distinct type would force casts at the sqlc.arg boundary for
    // no safety gain.
    type Stage = string

    const (
        StageMCQ   Stage = "mcq"
        StageExact Stage = "exact"
        // StageFuzzy and StageAI join this list in Stories 3.5/3.6.
    )

    // GradeMCQ reports whether response (a normalized "1".."4" digit
    // string — see game.parseMCQOption) matches correctOption (1-4,
    // questions.correct_option).
    func GradeMCQ(response string, correctOption int) bool {
        return response == strconv.Itoa(correctOption)
    }

    // GradeExact reports whether response (already trimmed by the caller)
    // exactly matches any of acceptedAnswers by literal string equality —
    // no normalization (trim beyond the caller's, final-letter forms,
    // nikud, punctuation): that is the Fuzzy stage, Story 3.5. An empty
    // acceptedAnswers never reaches this function for a real question
    // (questions_type_shape requires cardinality >= 1 for free_text) — no
    // special-case guard needed.
    func GradeExact(response string, acceptedAnswers []string) bool {
        for _, accepted := range acceptedAnswers {
            if response == accepted {
                return true
            }
        }
        return false
    }
    ```
  - [ ] New `server/internal/grading/pipeline_test.go`: table-driven tests —
    - `TestGradeMCQ`: cases for each `correctOption` 1-4 with a matching response, a mismatching response, and an out-of-range response (e.g. `"5"` — defensive; `game.parseMCQOption` already excludes this, but the function itself must not panic).
    - `TestGradeExact`: single accepted answer match; multi-value `acceptedAnswers` matching the first/middle/last entry; no match; response differing only by trailing whitespace the caller didn't trim (must NOT match — proves this stage does no normalization of its own); empty `response` against a non-empty accepted answer (no match).

- [ ] **Task 3: `store` package — thread grading into `RecordAnswer`, add the outstanding-grade count** (AC: 1, 2, 3)
  - [ ] `server/internal/store/queries/answers.sql` — extend the existing `GetOpenQuestionForPlayer` SELECT to also carry the grading inputs (currently only selects `game_id, question_id, question_type, participant_id`):
    ```sql
    -- name: GetOpenQuestionForPlayer :one
    SELECT g.id AS game_id, q.id AS question_id, q.type AS question_type,
           q.correct_option, q.accepted_answers, p.id AS participant_id
    FROM participants p
    JOIN games g ON g.id = p.game_id
    JOIN questions q ON q.game_id = g.id AND q.position = g.current_question_position
    WHERE p.phone = sqlc.arg(phone) AND p.role = 'player' AND g.state = 'question_open'
    ORDER BY g.updated_at DESC
    LIMIT 1;
    ```
    (Comment above this query is unchanged — only the SELECT list grows.) Extend `RecordAnswer`'s INSERT to persist the verdict in the same statement (no separate UPDATE — keeps the answer row's grade write atomic with its creation, same as every other column):
    ```sql
    -- name: RecordAnswer :one
    INSERT INTO answers (question_id, participant_id, response, received_at, is_correct, stage)
    SELECT sqlc.arg(question_id), sqlc.arg(participant_id), sqlc.arg(response), sqlc.arg(received_at), sqlc.arg(is_correct), sqlc.arg(stage)
    FROM games g
    JOIN questions q ON q.id = sqlc.arg(question_id) AND q.game_id = g.id
    WHERE g.id = sqlc.arg(game_id)
      AND g.state = 'question_open'
      AND g.current_question_position = q.position
      AND now() <= g.answer_cutoff_at
    RETURNING *;
    ```
    (The rest of this query's existing comment block — the race-closing rationale — is unchanged; only the column list grows.) `sqlc.arg(is_correct)`/`sqlc.arg(stage)` generate required (non-pointer) `bool`/`string` Go parameters regardless of the columns' own nullability — sqlc infers parameter nullability from `arg` vs `narg`, not from the target column — so `RecordAnswerParams` gains plain `IsCorrect bool` and `Stage string` fields, matching this story's fully-synchronous callers, which always have a definite verdict before calling.

    Add a new query for the Reveal-gate pre-check:
    ```sql
    -- Powers the engine's pre-check before attempting RevealCurrentQuestion
    -- (game/engine.go Reveal) — read-check for an accurate
    -- ErrGradingIncomplete vs. ErrNotQuestionClosed distinction, mirroring
    -- every other transition's "read for message accuracy, write-guard for
    -- the race" discipline in this codebase. RevealCurrentQuestion's own
    -- NOT EXISTS guard (games.sql) is the write-time race-safety net; this
    -- is the friendlier, non-transactional read.
    -- name: CountUngradedAnswersForCurrentQuestion :one
    SELECT count(*) FROM answers a
    JOIN questions q ON q.id = a.question_id
    JOIN games g ON g.id = q.game_id
    WHERE g.id = sqlc.arg(game_id) AND q.position = g.current_question_position AND a.stage IS NULL;
    ```
  - [ ] `server/internal/store/queries/games.sql` — `RevealCurrentQuestion` currently reads (no FROM clause):
    ```sql
    -- name: RevealCurrentQuestion :one
    UPDATE games
    SET state = 'revealed', updated_at = now()
    WHERE id = $1 AND organizer_id = $2 AND state = 'question_closed'
    RETURNING *;
    ```
    It is preceded by a comment: `-- No grading-completion gate here: the answers/grading tables don't exist until Stories 3.3-3.6; Story 3.4 is where "activates only once every received answer is graded" gets added.` — **replace that comment and the query** with the guarded version (same `UPDATE g ... FROM questions q` shape as `StartGameFirstQuestion`/`OpenNextQuestion` above it in this file; no new bind params needed — `g.current_question_position` is already a column):
    ```sql
    -- Reveal additionally guards on every recorded answer for the current
    -- question being graded (stage IS NOT NULL) — FR-16's "Reveal control
    -- activates only once every received answer is graded" (epic 3.4
    -- AC-3). Grading is fully synchronous through Story 3.5, so this NOT
    -- EXISTS is always vacuously true today; it becomes load-bearing once
    -- Story 3.6 makes AI grading genuinely async. Write-time race-safety
    -- net for the engine's own CountUngradedAnswersForCurrentQuestion
    -- pre-check (game/engine.go) — same "read for message accuracy,
    -- write-guard for the race" discipline as every other transition here.
    -- name: RevealCurrentQuestion :one
    UPDATE games g
    SET state = 'revealed',
        updated_at = now()
    FROM questions q
    WHERE g.id = $1 AND g.organizer_id = $2 AND g.state = 'question_closed'
      AND q.game_id = g.id AND q.position = g.current_question_position
      AND NOT EXISTS (SELECT 1 FROM answers a WHERE a.question_id = q.id AND a.stage IS NULL)
    RETURNING g.*;
    ```
  - [ ] Run `sqlc generate` (from `server/`) — expected non-empty diff (new `Answer.IsCorrect`/`Answer.Stage` as `pgtype.Bool`/`pgtype.Text` in `gen/models.go`, extended `GetOpenQuestionForPlayerRow`/`RecordAnswerParams`, new `CountUngradedAnswersForCurrentQuestion`); commit the generated files.
  - [ ] `server/internal/store/answers.go`: extend `RecordAnswerParams` with `IsCorrect bool` and `Stage string`, thread both into the `gen.RecordAnswerParams{}` literal inside `RecordAnswer`; add:
    ```go
    // CountUngradedAnswersForCurrentQuestion returns how many recorded
    // answers for gameID's current question have not yet been graded
    // (stage IS NULL) — the Reveal-gate pre-check (FR-16 epic AC-3).
    func (s *Store) CountUngradedAnswersForCurrentQuestion(ctx context.Context, gameID string) (int64, error) {
        return s.q.CountUngradedAnswersForCurrentQuestion(ctx, gameID)
    }
    ```
    (Field order inside the `gen.RecordAnswerParams{}` literal must match whatever sqlc actually generates — check the generated file, same caveat 3.3 already carries for this same struct literal.)

- [ ] **Task 4: `game` package — grade inline in `RecordAnswer`, gate `Reveal`** (AC: 1, 2, 3)
  - [ ] `server/internal/game/answers.go`: add `"github.com/avraham-shor/whatsapp-clickers/internal/grading"` to imports. Inside `RecordAnswer`, after the existing `switch qc.QuestionType` block resolves `response`, compute the verdict per branch and thread it into the store call:
    ```go
    var response string
    var isCorrect bool
    var stage grading.Stage
    switch qc.QuestionType {
    case "mcq":
        option, ok := parseMCQOption(rawText)
        if !ok {
            return AnswerResult{Outcome: AnswerFormatHint}, nil
        }
        response = strconv.Itoa(option)
        isCorrect = grading.GradeMCQ(response, int(qc.CorrectOption))
        stage = grading.StageMCQ
    case "free_text":
        trimmed := strings.TrimSpace(rawText)
        if utf8.RuneCountInString(trimmed) > maxFreeTextAnswerLength {
            return AnswerResult{Outcome: AnswerTooLong}, nil
        }
        response = trimmed
        isCorrect = grading.GradeExact(response, qc.AcceptedAnswers)
        stage = grading.StageExact
    default:
        return AnswerResult{}, fmt.Errorf("game: question %s has unrecognized type %q", qc.QuestionID, qc.QuestionType)
    }

    _, err = e.store.RecordAnswer(ctx, store.RecordAnswerParams{
        GameID:        qc.GameID,
        QuestionID:    qc.QuestionID,
        ParticipantID: qc.ParticipantID,
        Response:      response,
        ReceivedAt:    receivedAt,
        IsCorrect:     isCorrect,
        Stage:         stage,
    })
    ```
    `AnswerResult`/`AnswerOutcome` are **unchanged** — no `IsCorrect` field is added to either; the grade never needs to reach `wa`'s ack path (FR-15's "never disclosed before Reveal" — the ack is always "התקבל ✓" regardless of correctness, already true since 3.3).
  - [ ] `server/internal/game/engine.go`: add to the `Store` interface: `CountUngradedAnswersForCurrentQuestion(ctx context.Context, gameID string) (int64, error)`. Add a new sentinel error near the other `ErrNot*` vars:
    ```go
    // ErrGradingIncomplete means the current question's answers aren't all
    // graded yet — the rejection for Reveal when grading is still in
    // flight (FR-16 epic AC-3). A no-op condition in this story (MCQ/Exact
    // grading is synchronous); load-bearing from Story 3.6's async AI
    // stage onward.
    var ErrGradingIncomplete = errors.New("game: grading not yet complete for current question")
    ```
    Extend `Reveal` with the pre-check, inserted between the existing state check and the guarded write:
    ```go
    func (e *Engine) Reveal(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
        g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
        if err != nil {
            return Snapshot{}, err
        }
        if g.State != StateQuestionClosed {
            return Snapshot{}, ErrNotQuestionClosed
        }
        outstanding, err := e.store.CountUngradedAnswersForCurrentQuestion(ctx, gameID)
        if err != nil {
            return Snapshot{}, err
        }
        if outstanding > 0 {
            return Snapshot{}, ErrGradingIncomplete
        }
        g, err = e.store.RevealCurrentQuestion(ctx, gameID, organizerID)
        if err != nil {
            if errors.Is(err, store.ErrNotFound) {
                // The write-time guard failed after a successful read:
                // either a concurrent transition moved the game off
                // question_closed, or (vanishingly unlikely while grading
                // stays fully synchronous) a fresh ungraded answer landed
                // in the race window between the count above and this
                // write. Both reinterpret to ErrNotQuestionClosed — an
                // accepted approximation of the single-condition
                // race-loss mapping every other transition in this file
                // uses; see this story's Dev Notes.
                return Snapshot{}, ErrNotQuestionClosed
            }
            return Snapshot{}, err
        }
        return e.snapshotAfterCommit(ctx, g), nil
    }
    ```
  - [ ] `server/internal/httpapi/errors.go`: add a case to `writeStoreError`, immediately after the `game.ErrNotQuestionClosed` case (both concern `/reveal`):
    ```go
    case errors.Is(err, game.ErrGradingIncomplete):
        writeError(w, http.StatusConflict, "GRADING_INCOMPLETE", "not every received answer is graded yet")
    ```
  - [ ] `server/internal/game/engine_test.go`: extend `stubStore` with `countUngradedAnswersForCurrentQuestionResult int64`, `countUngradedAnswersForCurrentQuestionErr error`, `countUngradedAnswersForCurrentQuestionCalls int` + the method (zero-value default `0, nil` keeps every pre-existing `Reveal` test passing unchanged — 0 outstanding is the common case). Also extend `getOpenQuestionForPlayerResult`'s construction in existing tests where relevant — `gen.GetOpenQuestionForPlayerRow` gains `CorrectOption`/`AcceptedAnswers` fields; any test building this struct literal for `mcq`/`free_text` question types must set them for the new grading assertions to be meaningful. New tests (in `answers_test.go` if 3.3 created one by the time this story is implemented, else appended to `engine_test.go` — check which exists first):
    - `TestRecordAnswerMCQCorrectSetsIsCorrectTrue` / `TestRecordAnswerMCQIncorrectSetsIsCorrectFalse` — assert `recordAnswerArg.IsCorrect` and `recordAnswerArg.Stage == grading.StageMCQ`.
    - `TestRecordAnswerFreeTextExactMatchSetsIsCorrectTrue` / `TestRecordAnswerFreeTextNoMatchSetsIsCorrectFalse` — assert `recordAnswerArg.Stage == grading.StageExact` in both cases (a miss is still "graded exact, false" — not left ungraded).
    - `TestRevealSucceedsWhenNoOutstandingGrades` — extend/confirm the existing happy-path `Reveal` test still passes with the new pre-check in place (0 outstanding, `RevealCurrentQuestion` called once).
    - `TestRevealReturnsErrGradingIncompleteWhenOutstandingGradesExist` — `countUngradedAnswersForCurrentQuestionResult = 1` → `ErrGradingIncomplete`; assert `revealCurrentQuestionCalls == 0` (the guarded write must never be attempted).
    - `TestRevealPropagatesCountUngradedAnswersError` — an unrelated store error from the count call propagates unwrapped (`errors.Is`, not string equality — 3.2's review precedent).
  - [ ] `server/internal/httpapi/control_test.go`: `controlActionCases`' existing `"Reveal"` row already covers `ErrNotQuestionClosed` → 409 `GAME_NOT_QUESTION_CLOSED` via the shared per-endpoint suite — do not add a second row there (that table verifies one representative error per endpoint, not every possible error a given endpoint can return). Add one focused test mirroring `TestControlActionDomainErrorReturns409WithoutBroadcasting`'s body, scoped to Reveal only:
    ```go
    func TestHandleRevealGradingIncompleteReturns409WithoutBroadcasting(t *testing.T) {
        engine := &stubControlEngine{revealErr: game.ErrGradingIncomplete}
        hub := &stubBroadcaster{}
        rec := httptest.NewRecorder()
        controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/reveal", ""))

        if rec.Code != http.StatusConflict {
            t.Fatalf("POST /reveal on grading-incomplete = %d, want 409", rec.Code)
        }
        if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GRADING_INCOMPLETE" {
            t.Errorf("error code = %q, want GRADING_INCOMPLETE", code)
        }
        if len(hub.calls) != 0 {
            t.Error("Broadcast called on a rejected transition")
        }
    }
    ```

- [ ] **Task 5: Quality gates + local E2E** (all ACs)
  - [ ] Local gates: `gofmt -l .` · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` diff — expected non-empty (Task 3), confirm it matches exactly what Task 3 wrote. No `web/` changes in this story — skip `npm run lint`/`tsc -b` only if genuinely nothing under `web/` changed; run them anyway if unsure, they should no-op.
  - [ ] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1–3.3 — **requires 3.3's Task 4 (wa routing) to be done first**, since this is the only path that exercises `RecordAnswer` end-to-end): create a scratch game with one MCQ question (correct option known, e.g. option 2) and one Free-Text question (accepted answers `["ירושלים", "ירושלים עיר הקודש"]`). Open lobby, join 3 players (A, B, C). `POST /start` (opens Q1, MCQ). Drive via signed inbound webhook `POST`s:
    - A replies `"2"` (correct) — after the send, query the scratch DB directly (`SELECT is_correct, stage FROM answers WHERE ...`) and assert `is_correct = true, stage = 'mcq'`.
    - B replies `"1"` (incorrect) — assert `is_correct = false, stage = 'mcq'`.
    - `POST /close-question` → `POST /reveal`: must succeed (200), since both recorded answers are already graded — this is the "gate is vacuously satisfied" case AC-3 describes.
    - `POST /next-question` (opens Q2, Free-Text). A replies `"ירושלים"` (exact match) — assert `is_correct = true, stage = 'exact'`. B replies `"ירושלים עיר הקודש "` with a trailing space (still trimmed-exact match per 3.3's intake trim) — assert `is_correct = true`. C replies `"ירו"` (no match) — assert `is_correct = false, stage = 'exact'` (graded, not left NULL).
    - `POST /close-question` → `POST /reveal`: must again succeed 200 (all three graded, including C's miss).
    - No way to directly exercise the `GRADING_INCOMPLETE` 409 path in this E2E — nothing in this story's implemented pipeline is asynchronous, so no answer is ever left ungraded at the point Reveal is attempted. That branch is verified only by the unit tests in Task 4 (mocking `countUngradedAnswersForCurrentQuestionResult > 0`). Note this explicitly in the Completion Notes rather than contriving a fake DB row to force it.
    - Clean up the scratch game row afterward; delete the harness afterward — same convention as every prior story.

## Dev Notes

### Architecture guardrails (violations = rework)

- **Dependency direction**: this story is the first to populate `server/internal/grading/` — confirms the architecture's `game` → `grading` edge (`grading` imports nothing from `game`/`store`; it is pure functions taking primitives). `game` now imports both `store` and `grading`, matching architecture's "`game` owns all state transitions... it imports `store` and `grading`."
- **Persist-before-ack (SM-4) unaffected**: the grade is written in the *same* INSERT as the answer (Task 3), so there is no new "persist then separately grade" window — SM-4's guarantee (answer persisted before the ack) is untouched; the grade simply rides along atomically.
- **Grade never disclosed before Reveal (FR-15/FR-16, non-negotiable)**: `AnswerResult`/`AnswerOutcome` (wa's only view into `RecordAnswer`) gain no correctness field. `Snapshot`/`CurrentQuestion` (game/snapshot.go) already omit `CorrectOption`/`AcceptedAnswers` "at every state, including revealed" per its own doc comment — this story does not touch `snapshot.go` and must not add `IsCorrect`/grade data to any snapshot type. Personal-result disclosure is Story 3.8's job, gated on Reveal already having happened.
- **Glossary**: `Answer`, `Question`, `Grade`/`grading` are PRD Glossary-adjacent terms; the new package is named `grading` (matches architecture's planned `server/internal/grading/` directory exactly) — do not call it `validation`, `scoring` (that is a different, later package, Story 3.7), or `checker`.
- **Logging (NFR-8)**: no new log lines are strictly required by this story's ACs (grading success/failure isn't itself a degradation — mechanical/exact grading cannot fail short of a programming error). If a defensive log is added for the `default` branch in `RecordAnswer`'s type switch, it already exists (unchanged from 3.3) — do not add INFO-per-answer-graded logging; that would spam at pilot scale (up to ~2,000 answers/game) for no operational value, inconsistent with this codebase's existing restraint (3.2/3.3 log per *burst*/*outcome*, not per silent internal computation).
- **NFR-2 (never block the game loop)**: `GradeMCQ`/`GradeExact` are pure, allocation-light, sub-microsecond functions — no risk of blocking `RecordAnswer`'s synchronous webhook-request path. This remains true through Story 3.5 (Levenshtein is also pure Go); Story 3.6 (AI, network I/O) is where this posture must change, which is exactly why the grading-completeness *gate* is being built now rather than then.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/game/answers.go](server/internal/game/answers.go)** — as of the current (uncommitted, 3.3-in-progress) state on disk: `RecordAnswer` parses MCQ/Free-Text, returns one of five `AnswerOutcome` values, calls `store.RecordAnswer` once. This story adds grading computation between response-parsing and the store call — no new outcome values, no change to the function's error-mapping `switch` at the bottom (already-correct: `AnswerAccepted`/`AnswerAlreadyAnswered`/`AnswerClosed`).
- **[server/internal/game/engine.go](server/internal/game/engine.go)** — `Store` interface currently has 12 methods (through `CountAnswersByQuestion`, story 3.3's in-progress work); gains 1 more (`CountUngradedAnswersForCurrentQuestion`). `Reveal` currently has zero grading awareness (calls `RevealCurrentQuestion` immediately after the state check) — this story is what first makes it grading-aware; every other transition method (`OpenLobby`, `StartGame`, `CloseQuestion`, `NextQuestion`, `StopGame`) is untouched.
- **[server/internal/store/queries/answers.sql](server/internal/store/queries/answers.sql)** and **[server/internal/store/answers.go](server/internal/store/answers.go)** — both currently exist (3.3's in-progress Task 1) with exactly the shape quoted in Task 3 above; re-read them before editing in case 3.3's remaining tasks changed them by the time this story starts.
- **[server/internal/store/queries/games.sql](server/internal/store/queries/games.sql)** — `RevealCurrentQuestion` carries a comment literally naming this story as where the grading gate arrives (`"Story 3.4 is where ... gets added"`) — confirms this design was anticipated at architecture time, not improvised here. `CloseCurrentQuestion`/`StartGameFirstQuestion`/`OpenNextQuestion` in the same file are untouched.
- **[server/internal/httpapi/errors.go](server/internal/httpapi/errors.go)** — `writeStoreError`'s `switch` currently has 8 cases (7 domain errors + `ErrNotFound`/`ErrReorderMismatch`); gains one more. Ordering in the switch doesn't matter functionally (each `case` is a distinct `errors.Is` check) but group `ErrGradingIncomplete` next to `ErrNotQuestionClosed` for readability (both are `/reveal`'s rejections).
- **[server/internal/httpapi/control.go](server/internal/httpapi/control.go)** — `handleReveal` calls `engine.Reveal` and maps any error via `writeStoreError(w, err, "GAME_NOT_FOUND")` already — **no change needed**; the new `ErrGradingIncomplete` case is entirely inside `errors.go`, reached through the same call site. Verify this after Task 4 rather than assuming it (same "verify, don't assume" discipline 3.3's Dev Notes applied to `main.go`).

### Why no web/`strings.he.ts`/`messages_he.go` changes

Every one of this story's 3 ACs is server-internal (grade computation + storage + a REST error code). The organizer-facing "גלה תשובה" button has no new visible state in this story — EXPERIENCE.md's own control-panel table entry for it ("activates once all received answers are graded — FR-16") describes the *server-enforced precondition*, not a new UI affordance; since grading is 100% synchronous through this story, an organizer can never actually observe the button being blocked (there is nothing for a "waiting for grading" indicator to ever show). Adding disabled/waiting UI now would be speculative work for a state this story cannot produce — correctly deferred to Story 3.6, the first story where the AI stage's ~5s latency makes `GRADING_INCOMPLETE` genuinely reachable by a real organizer clicking quickly. Flag this scoping decision in this story's code review for a `deferred-work.md` entry (revisit trigger: Story 3.6) if it survives triage.

### A design decision worth flagging explicitly: what happens to a Free-Text miss

FR-16 describes a pipeline (Exact → Fuzzy → AI) where a miss falls through to the next stage. This story implements only the first stage, so a question arises: does an Exact miss stay "pending" (ungraded, waiting for a future stage) or "graded, false"? **This story marks it graded-false** (`stage = 'exact'`, `is_correct = false`) — not pending — because leaving it NULL would permanently block Reveal for any game with an incorrect Free-Text answer until Stories 3.5/3.6 ship, which cannot be the intent (the epic's own AC-3 language is about *async latency*, not "pipeline stages that don't exist yet"). When Story 3.5 adds Fuzzy, its design must decide how a Free-Text answer's grading extends past this story's single-pass Exact-then-done — most likely, the whole synchronous pipeline (Exact → Fuzzy) simply runs to completion inside one `RecordAnswer` call, the same way this story runs MCQ/Exact to completion today, rather than revisiting already-inserted rows. This story does not attempt to solve that; it only establishes that "graded" means "the currently-implemented pipeline ran to completion for this answer," whatever that pipeline currently contains.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in place (`stubStore`, `stubControlEngine`) — no mock framework, matching every prior story. `grading`'s tests are pure unit tests with no store/DB dependency — the first package in this codebase with zero I/O in its own tests. No real-DB unit tests for the `RevealCurrentQuestion` guard's race behavior (same reasoning as 3.3: only provable against real Postgres) — Task 5's `cmd/e2escratch` harness is that verification for the happy path; the `GRADING_INCOMPLETE` path is unit-tested only, per Task 5's explicit note.

### Project Structure Notes

**New:**
- `server/migrations/00011_answer_grading.sql`
- `server/internal/grading/pipeline.go`
- `server/internal/grading/pipeline_test.go`

**Modified:**
- `server/internal/store/queries/answers.sql` (`GetOpenQuestionForPlayer` +2 columns, `RecordAnswer` +2 columns, +`CountUngradedAnswersForCurrentQuestion`)
- `server/internal/store/queries/games.sql` (`RevealCurrentQuestion` gains the grading-completeness guard)
- `server/internal/store/gen/*` (sqlc-generated)
- `server/internal/store/answers.go` (`RecordAnswerParams` +2 fields, +`CountUngradedAnswersForCurrentQuestion`)
- `server/internal/game/answers.go` (`RecordAnswer` grades inline before persisting)
- `server/internal/game/engine.go` (+1 `Store` interface method, +`ErrGradingIncomplete`, `Reveal` gains the pre-check)
- `server/internal/game/engine_test.go` (`stubStore` extension, new grading/Reveal tests)
- `server/internal/httpapi/errors.go` (+1 `writeStoreError` case)
- `server/internal/httpapi/control_test.go` (+1 focused test)

**Untouched:** `server/internal/game/snapshot.go` (no grade data added to the wire — see Dev Notes) · `server/internal/wa/*` (no new templates; the ack is already grade-blind since 3.3) · `web/*` (no UI change — see Dev Notes) · `server/internal/httpapi/control.go` (verify, don't assume — see Dev Notes).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.4] — story + all 3 epic ACs verbatim, Epic 3 context, FR-15/FR-16 exact-stage scope
- [Source: _bmad-output/planning-artifacts/architecture.md#Data-Architecture] — `grading/` package structure (`pipeline.go`, `normalize.go`, `fuzzy.go`, `ai.go` — this story delivers only the first), "AI invoked only on Exact/Fuzzy miss", "all grading for a Question completes before Reveal is published — the Reveal control activates only once every received answer is graded"
- [Source: _bmad-output/planning-artifacts/architecture.md#Project-Structure] — dependency direction `game` → `grading`/`store`; `grading/pipeline.go` planned file
- [Source: server/internal/store/queries/games.sql] — `RevealCurrentQuestion`'s pre-existing comment literally naming this story as the trigger for the grading gate; `StartGameFirstQuestion`/`OpenNextQuestion`'s `UPDATE g ... FROM questions q` pattern this story's `RevealCurrentQuestion` now follows
- [Source: server/internal/game/answers.go, engine.go, snapshot.go] — current (3.3-in-progress) `RecordAnswer`/`Store` interface/`Reveal`/`CurrentQuestion` shapes this story extends; `CurrentQuestion`'s own doc comment on deliberately omitting grade data "at every state, including revealed"
- [Source: server/internal/httpapi/control.go, errors.go, control_test.go] — `handleReveal`'s existing error-mapping call site (unchanged); `writeStoreError`'s switch pattern; `controlActionCases`' one-representative-error-per-endpoint test design
- [Source: server/internal/store/queries/questions.sql, gen/models.go] — `questions.correct_option INTEGER`/`accepted_answers TEXT[]` (`Question.CorrectOption int32`/`AcceptedAnswers []string` in Go, both NOT NULL — no pgtype wrapper needed when reading them)
- [Source: _bmad-output/implementation-artifacts/3-3-answer-intake-with-immediate-acknowledgment.md] — previous story's plan for `game/answers.go`/`store/answers.go`/`queries/answers.sql`, cross-checked in this story's Dev Notes against what actually exists on disk (they matched, as of this story's baseline commit)

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
