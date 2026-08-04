---
baseline_commit: b21ed05bf3c0cd44882eeb68a552ed105f1ff221
---

# Story 3.3: Answer Intake with Immediate Acknowledgment

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a Participant,
I want my WhatsApp reply recorded instantly with a "התקבל ✓",
so that I know my answer counts (FR-5, FR-7, FR-8).

## Acceptance Criteria

1. **Given** an open Question (`answers` table created in this story: `UNIQUE (question_id, participant_id)`, server-receipt `timestamptz` + monotonic sequence), **when** I reply with a valid answer (MCQ: letter א–ד or digit 1–4; Free-Text: ≤200 chars), **then** the answer row is **persisted before** the "התקבל ✓" ack is sent (SM-4), stamped with server receipt time. *(epic AC-1)*

2. **Given** a Free-Text reply over 200 characters, **then** it is rejected with a Hebrew hint; **and** an unparseable reply during an open MCQ returns a short format hint, leaving me able to answer. *(epic AC-2)*

3. **Given** I already answered this Question, **when** I send a second answer, **then** it is not recorded and I am told my first answer counts — selection is final (FR-8). *(epic AC-3)*

4. **Given** a reply whose receipt timestamp is after the cutoff, **then** it is not counted and receives the polite "השאלה נסגרה" reply (FR-7, UJ-5). *(epic AC-4)*

5. **Given** the control panel during an open Question, **then** the live answered count updates via snapshots (FR-13 consequence, `host-stat-pill` UX-DR11 — canonical copy "87 ענו", not a separately rendered percentage number). *(epic AC-5)*

## Tasks / Subtasks

- [x] **Task 1: Schema + `store` package — the `answers` table and its guarded queries** (AC: 1, 3, 4)
  - [x] New `server/migrations/00010_answers.sql`:
    ```sql
    -- +goose Up
    CREATE TABLE answers (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
        participant_id UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
        -- Normalized: "1".."4" for MCQ (never the raw letter — story 3.4's
        -- grading compares this directly against questions.correct_option),
        -- trimmed free text for free_text. No CHECK tying this to the
        -- question's type — the FK alone can't express it, and the write path
        -- (game.RecordAnswer) is the only writer and already enforces it.
        response TEXT NOT NULL,
        -- No DEFAULT now(): the engine always supplies this explicitly, from
        -- the Go-process webhook-ingestion clock (wa.InboundMessage.ReceivedAt),
        -- not the DB transaction clock — FR-7's authoritative receipt time.
        -- Resolves the two-clocks item deferred from story 2.1's second-round
        -- review (see this story's Dev Notes).
        received_at timestamptz NOT NULL,
        -- Monotonic tie-break for Speed Bonus ordering (FR-17, story 3.7) when
        -- two answers share a received_at at timestamptz resolution. Not read
        -- by this story.
        seq BIGINT GENERATED ALWAYS AS IDENTITY
    );

    -- Named to match architecture's own Naming Patterns example verbatim
    -- ("Indexes: idx_<table>_<cols> (idx_answers_question_participant)").
    -- Enforces FR-8 (one recorded answer per participant per question) at the
    -- DB level; a duplicate INSERT raises a real 23505 unique_violation
    -- rather than silently matching zero rows — see RecordAnswer below, which
    -- depends on that distinction. No separate idx_answers_question index:
    -- this index already leads with question_id (participants table
    -- precedent, migration 00008).
    CREATE UNIQUE INDEX idx_answers_question_participant ON answers (question_id, participant_id);

    -- +goose Down
    DROP TABLE answers;
    ```
  - [x] New `server/internal/store/queries/answers.sql`:
    ```sql
    -- Resolves the (game, question, participant) context for phone's next
    -- reply — the one game where phone is a player-role Participant with a
    -- currently open question. ORDER BY + LIMIT 1 is defensive belt-and-braces
    -- for a phone somehow being an active player in more than one
    -- simultaneously-open-question game; NFR-3 assumes a single Game at a
    -- time, so this is not a real multi-game feature. [ASSUMPTION]
    -- name: GetOpenQuestionForPlayer :one
    SELECT g.id AS game_id, q.id AS question_id, q.type AS question_type, p.id AS participant_id
    FROM participants p
    JOIN games g ON g.id = p.game_id
    JOIN questions q ON q.game_id = g.id AND q.position = g.current_question_position
    WHERE p.phone = sqlc.arg(phone) AND p.role = 'player' AND g.state = 'question_open'
    ORDER BY g.updated_at DESC
    LIMIT 1;

    -- Persists an answer, re-validating at write time that question_id is
    -- STILL the game's current open question, the game is still
    -- question_open, and the cutoff has not passed — closes the
    -- read-then-write race between GetOpenQuestionForPlayer above and this
    -- INSERT (same "re-check the guard at write time" discipline as
    -- CreateParticipant's allowed_states / CreateQuestion's state='draft').
    -- Binding on the specific question_id (not re-derived via
    -- current_question_position) additionally closes a narrower race: if the
    -- question advances between the read and this write, current_question_position
    -- would point at a DIFFERENT question, and re-deriving it here could
    -- silently record the late answer against the wrong question.
    --
    -- Deliberately NOT `ON CONFLICT DO NOTHING`: a duplicate answer (participant
    -- already has a row for this question_id) must raise the real 23505
    -- unique_violation against idx_answers_question_participant, distinct from
    -- a zero-source-rows guard failure. Collapsing both into one "0 rows"
    -- result would make FR-7's "closed" reply and FR-8's "already answered"
    -- reply indistinguishable at the store layer.
    -- name: RecordAnswer :one
    INSERT INTO answers (question_id, participant_id, response, received_at)
    SELECT sqlc.arg(question_id), sqlc.arg(participant_id), sqlc.arg(response), sqlc.arg(received_at)
    FROM games g
    JOIN questions q ON q.id = sqlc.arg(question_id) AND q.game_id = g.id
    WHERE g.id = sqlc.arg(game_id)
      AND g.state = 'question_open'
      AND g.current_question_position = q.position
      AND now() <= g.answer_cutoff_at
    RETURNING *;

    -- Live answered-count for the control panel's host-stat-pill (FR-13,
    -- UX-DR11, epic AC-5). idx_answers_question_participant already leads
    -- with question_id, so this is an index-only count, same reasoning as
    -- the participants table's "no separate index" comment.
    -- name: CountAnswersByQuestion :one
    SELECT count(*) FROM answers WHERE question_id = $1;
    ```
  - [x] Run `sqlc generate` (from `server/`) to produce `gen.Answer`, `gen.GetOpenQuestionForPlayerRow`, `gen.RecordAnswerParams`, `gen.CountAnswersByQuestionRow`/`int64` return. **Unlike story 3.2, this story's `sqlc generate` diff will NOT be empty** — the new table and queries are real, expected output; commit the generated files.
  - [x] New `server/internal/store/answers.go`:
    ```go
    package store

    import (
        "context"
        "errors"
        "time"

        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgconn"

        "github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
    )

    // ErrAlreadyAnswered means the participant already has a recorded answer
    // for this question — RecordAnswer's duplicate path (a real 23505 unique
    // violation), distinct from ErrNotFound's "write-time guard failed /
    // question no longer open" path. See queries/answers.sql's RecordAnswer
    // comment for why these must not collapse into one outcome.
    var ErrAlreadyAnswered = errors.New("store: already answered")

    // GetOpenQuestionForPlayer resolves phone's currently-open-question
    // context; no such context anywhere is ErrNotFound.
    func (s *Store) GetOpenQuestionForPlayer(ctx context.Context, phone string) (gen.GetOpenQuestionForPlayerRow, error) {
        row, err := s.q.GetOpenQuestionForPlayer(ctx, phone)
        if errors.Is(err, pgx.ErrNoRows) {
            return gen.GetOpenQuestionForPlayerRow{}, ErrNotFound
        }
        return row, err
    }

    // RecordAnswerParams carries an already-validated answer ready to persist.
    type RecordAnswerParams struct {
        GameID        string
        QuestionID    string
        ParticipantID string
        Response      string
        ReceivedAt    time.Time
    }

    // RecordAnswer persists one answer. A write-time guard miss (the question
    // is no longer the game's current open one, the game left question_open,
    // or the cutoff passed) is ErrNotFound; a genuine duplicate is
    // ErrAlreadyAnswered — pgconn error-code knowledge stays inside store,
    // same posture as CreateGame's join-code collision handling.
    func (s *Store) RecordAnswer(ctx context.Context, arg RecordAnswerParams) (gen.Answer, error) {
        a, err := s.q.RecordAnswer(ctx, gen.RecordAnswerParams{
            QuestionID:    arg.QuestionID,
            ParticipantID: arg.ParticipantID,
            Response:      arg.Response,
            ReceivedAt:    arg.ReceivedAt,
            GameID:        arg.GameID,
        })
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "23505" {
            return gen.Answer{}, ErrAlreadyAnswered
        }
        if errors.Is(err, pgx.ErrNoRows) {
            return gen.Answer{}, ErrNotFound
        }
        return a, err
    }

    // CountAnswersByQuestion returns the number of recorded answers for a
    // question — the control panel's live answered-count (FR-13, epic AC-5).
    func (s *Store) CountAnswersByQuestion(ctx context.Context, questionID string) (int64, error) {
        return s.q.CountAnswersByQuestion(ctx, questionID)
    }
    ```
    Field order inside `gen.RecordAnswerParams{}` above must match whatever sqlc actually generates (alphabetical-by-Go-convention or `sqlc.arg` declaration order) — check the generated file rather than trusting this snippet's order.

- [x] **Task 2: `game` package — `RecordAnswer` (parse, validate, persist)** (AC: 1, 2, 3, 4)
  - [x] `server/internal/game/engine.go`: extend the `Store` interface with the three new methods:
    ```go
    GetOpenQuestionForPlayer(ctx context.Context, phone string) (gen.GetOpenQuestionForPlayerRow, error)
    RecordAnswer(ctx context.Context, arg store.RecordAnswerParams) (gen.Answer, error)
    CountAnswersByQuestion(ctx context.Context, questionID string) (int64, error)
    ```
    `*store.Store` already satisfies this after Task 1 — no other change to `engine.go` needed for this task (the snapshot wiring is Task 3).
  - [x] New `server/internal/game/answers.go`:
    ```go
    package game

    import (
        "context"
        "errors"
        "strconv"
        "strings"
        "time"
        "unicode/utf8"

        "github.com/avraham-shor/whatsapp-clickers/internal/store"
    )

    // ErrNoOpenQuestion means phone has no currently open question awaiting an
    // answer in any game right now. This is NOT a failure — wa's InboundRouter
    // treats it as "this message isn't an answer attempt" and falls through to
    // the existing Help/classify() routing (see wa/inbound.go Dev Notes on why
    // answer detection is not folded into classify()).
    var ErrNoOpenQuestion = errors.New("game: no open question for participant")

    // maxFreeTextAnswerLength is FR-5's 200-character cap, counted in runes
    // (Hebrew text) — never len(string), which counts UTF-8 bytes.
    const maxFreeTextAnswerLength = 200

    // AnswerOutcome distinguishes the reply RecordAnswer's caller must send.
    // Every value maps to exactly one messages_he.go template — see
    // wa/inbound.go's handleTextOrAnswer.
    type AnswerOutcome string

    const (
        AnswerAccepted        AnswerOutcome = "accepted"
        AnswerAlreadyAnswered AnswerOutcome = "already_answered"
        AnswerFormatHint      AnswerOutcome = "format_hint"
        AnswerTooLong         AnswerOutcome = "too_long"
        AnswerClosed          AnswerOutcome = "closed"
    )

    // AnswerResult is RecordAnswer's success return.
    type AnswerResult struct {
        Outcome AnswerOutcome
    }

    // mcqOptionLetters maps the four lettered options — same fixed ordering as
    // messages_he.go's Question-MCQ template (א/ב/ג/ד) — to their 1-based
    // option number, matching questions.correct_option's convention (1-4).
    var mcqOptionLetters = map[string]int{"א": 1, "ב": 2, "ג": 3, "ד": 4}

    // parseMCQOption recognizes a TRIMMED single letter (א-ד) or digit (1-4)
    // reply. Unlike parseJoinCode's multi-field tolerance, any extra content
    // ("1 hi", "א.") fails to parse — FR-5 specifies a bare letter or digit,
    // and a stricter parse here means "1." correctly earns the format hint
    // (AC-2) rather than a guessed, possibly-wrong option.
    func parseMCQOption(body string) (option int, ok bool) {
        trimmed := strings.TrimSpace(body)
        if n, err := strconv.Atoi(trimmed); err == nil && n >= 1 && n <= 4 {
            return n, true
        }
        if opt, ok := mcqOptionLetters[trimmed]; ok {
            return opt, true
        }
        return 0, false
    }

    // RecordAnswer processes phone's reply as an answer attempt.
    //
    // receivedAt is the Go-process webhook-ingestion clock
    // (wa.InboundMessage.ReceivedAt, captured in wa/webhook.go BEFORE the
    // dedupe DB round-trip) — FR-7's single authoritative receipt time. It is
    // used for the persisted answers.received_at, NOT Postgres now(); the
    // cutoff comparison itself (RecordAnswer query, store/queries/answers.sql)
    // separately and deliberately uses Postgres now() for symmetry with
    // CloseCurrentQuestion's LEAST(answer_cutoff_at, now()) — these are two
    // distinct, intentional clock choices, not an inconsistency. This
    // resolves the "two different clocks for one authoritative receipt time"
    // item deferred from story 2.1's second-round review
    // (deferred-work.md) — mark that entry closed as part of this story.
    func (e *Engine) RecordAnswer(ctx context.Context, phone, rawText string, receivedAt time.Time) (AnswerResult, error) {
        qc, err := e.store.GetOpenQuestionForPlayer(ctx, phone)
        if err != nil {
            if errors.Is(err, store.ErrNotFound) {
                return AnswerResult{}, ErrNoOpenQuestion
            }
            return AnswerResult{}, err
        }

        var response string
        switch qc.QuestionType {
        case "mcq":
            option, ok := parseMCQOption(rawText)
            if !ok {
                return AnswerResult{Outcome: AnswerFormatHint}, nil
            }
            response = strconv.Itoa(option)
        case "free_text":
            trimmed := strings.TrimSpace(rawText)
            if utf8.RuneCountInString(trimmed) > maxFreeTextAnswerLength {
                return AnswerResult{Outcome: AnswerTooLong}, nil
            }
            response = trimmed
        default:
            // questions_type_shape's CHECK constraint guarantees this never
            // happens for a real row — fail closed with an error (degrades to
            // Help + WARN in wa, matching every other "impossible" branch in
            // this codebase) rather than guessing a parse strategy.
            return AnswerResult{}, fmt.Errorf("game: question %s has unrecognized type %q", qc.QuestionID, qc.QuestionType)
        }

        _, err = e.store.RecordAnswer(ctx, store.RecordAnswerParams{
            GameID:        qc.GameID,
            QuestionID:    qc.QuestionID,
            ParticipantID: qc.ParticipantID,
            Response:      response,
            ReceivedAt:    receivedAt,
        })
        switch {
        case err == nil:
            return AnswerResult{Outcome: AnswerAccepted}, nil
        case errors.Is(err, store.ErrAlreadyAnswered):
            return AnswerResult{Outcome: AnswerAlreadyAnswered}, nil
        case errors.Is(err, store.ErrNotFound):
            // The write-time guard failed after a successful read: the
            // question closed (or moved on) in the window between
            // GetOpenQuestionForPlayer and this INSERT. Correct-by-construction
            // reinterpretation, same pattern as OpenLobby/StartGame's
            // race-loss-to-domain-error mapping in engine.go.
            return AnswerResult{Outcome: AnswerClosed}, nil
        default:
            return AnswerResult{}, err
        }
    }
    ```
    Add `"fmt"` to the import block (used by the default-branch error). Question type is compared against the raw `"mcq"`/`"free_text"` literals, not a shared constant — matches the codebase's existing, already-reviewed posture (story 3.2's code review explicitly dismissed centralizing these as noise, noting `game` itself already used raw literals; do not introduce a new shared constant that contradicts that accepted decision).
  - [x] `server/internal/game/engine_test.go`: extend `stubStore` with `getOpenQuestionForPlayerResult gen.GetOpenQuestionForPlayerRow`, `getOpenQuestionForPlayerErr error`, `recordAnswerResult gen.Answer`, `recordAnswerErr error`, `recordAnswerCalls int`, `recordAnswerArg store.RecordAnswerParams` (records the last call's argument, for assertion), `countAnswersByQuestionResult int64`, `countAnswersByQuestionErr error` + the three methods. **Required for the package to compile at all** (the `Store` interface gained three methods in this task) — every existing test in this file that builds a snapshot with a current question now has `stubStore.CountAnswersByQuestion` called on it too (Task 3); the zero-value defaults (`0, nil`) keep every pre-existing assertion valid, but re-run the full test suite to confirm none does exact `Snapshot{}` struct-literal equality that would now need `AnsweredCount: 0` added.
  - [x] New tests in `engine_test.go` (or a new `server/internal/game/answers_test.go` — either is fine, match this file's existing single-file-per-concept-or-not convention; `participants_test.go` exists alongside `participants.go`, so a sibling `answers_test.go` is the more consistent choice):
    - `TestRecordAnswerNoOpenQuestionReturnsErrNoOpenQuestion` — `getOpenQuestionForPlayerErr = store.ErrNotFound` → `ErrNoOpenQuestion`, `RecordAnswer` (store) never called.
    - `TestRecordAnswerMCQDigitAccepted` / `TestRecordAnswerMCQLetterAccepted` — for each of "1".."4" and "א".."ד": `getOpenQuestionForPlayerResult.QuestionType = "mcq"`, assert `recordAnswerArg.Response` is the matching normalized digit string ("1".."4") and outcome is `AnswerAccepted`.
    - `TestRecordAnswerMCQUnparseableReturnsFormatHint` — e.g. `"5"`, `"א."`, `"hello"` → `AnswerFormatHint`, store `RecordAnswer` never called.
    - `TestRecordAnswerFreeTextWithinLimitAccepted` — 200-rune Hebrew string (construct via `strings.Repeat` on a multi-byte rune, not an ASCII placeholder, to actually exercise `RuneCountInString` vs byte-length) → accepted, `recordAnswerArg.Response` is the trimmed text.
    - `TestRecordAnswerFreeTextOverLimitReturnsTooLong` — 201 runes → `AnswerTooLong`, store `RecordAnswer` never called.
    - `TestRecordAnswerFreeTextTrimsWhitespace` — leading/trailing spaces stored trimmed.
    - `TestRecordAnswerAlreadyAnsweredMapsToOutcome` — `recordAnswerErr = store.ErrAlreadyAnswered` → `AnswerAlreadyAnswered`.
    - `TestRecordAnswerClosedMapsToOutcome` — `recordAnswerErr = store.ErrNotFound` (the write-time-guard-miss path) → `AnswerClosed`.
    - `TestRecordAnswerStoreErrorPropagates` — an unrelated store error on either call propagates unwrapped (do not pin exact error-string equality — 3.2's review flagged that exact pattern as fragile; assert only that an error is returned, or via `errors.Is`/a sentinel).
    - `TestRecordAnswerPassesReceivedAtThrough` — assert `recordAnswerArg.ReceivedAt` equals the `receivedAt` argument exactly (proves the Go clock, not a DB default, is what reaches the store call — the deferred-item fix's only unit-testable surface).

- [x] **Task 3: `game` package — live answered-count on the snapshot** (AC: 5)
  - [x] `server/internal/game/snapshot.go`: add `AnsweredCount int` to `CurrentQuestion` (nested, not a root `Snapshot` field — meaningless outside an active question, and keeps every JSON consumer's null-check at the same level as `answerCutoffAt`):
    ```go
    type CurrentQuestion struct {
        ID               string   `json:"id"`
        Position         int      `json:"position"`
        Type             string   `json:"type"`
        Text             string   `json:"text"`
        Options          []string `json:"options,omitempty"`
        TimeLimitSeconds int      `json:"timeLimitSeconds"`
        AnswerCutoffAt   string   `json:"answerCutoffAt"`
        AnsweredCount    int      `json:"answeredCount"`
    }
    ```
  - [x] `server/internal/game/engine.go`'s `buildSnapshot`: inside the existing `if g.CurrentQuestionPosition > 0 { for _, q := range questions { if q.Position == ... { current = &CurrentQuestion{...} ... } } }` block, after resolving `current`, call `e.store.CountAnswersByQuestion(ctx, current.ID)` and set `current.AnsweredCount = int(count)`; a failure here propagates the same as the pre-existing `ListParticipants`/`ListQuestionsByGame` errors in this function (no special-case degradation — consistent with its two siblings in the same function). `emptySnapshot`'s degraded fallback needs no change (its `CurrentQuestion` is already nil).
  - [x] `engine_test.go`: extend the existing snapshot-producing tests (`TestStartGame*`, `TestNextQuestion*`, `TestSnapshot*` — grep for `current` / `CurrentQuestion` in this file for the authoritative list, do not trust a hardcoded count here) to set `countAnswersByQuestionResult` and assert `snapshot.CurrentQuestion.AnsweredCount` reflects it; add one case where `countAnswersByQuestionErr` is set and the snapshot build fails/degrades exactly like the pre-existing `listQuestionsByGameErr` case does.

- [x] **Task 4: `wa` package — five new templates + answer-intake routing** (AC: 1, 2, 3, 4)
  - [x] `server/internal/wa/messages_he.go`: add, copied **verbatim** from EXPERIENCE.md's "WhatsApp message templates" table (rows: Acknowledgment, Already answered, Format hint (MCQ), Too long (Free-Text), Question closed):
    ```go
    // msgAcknowledgment is the Acknowledgment row — the PRD-mandated "התקבל ✓"
    // (SM-4/FR-5), sent immediately after the answer row is committed. No
    // placeholders; the grade is never revealed here (FR-6).
    const msgAcknowledgment = "התקבל ✓ — בהצלחה!"

    func ackMessage() string {
        return msgAcknowledgment
    }

    // msgAlreadyAnswered is the Already-answered row (FR-8: first answer wins).
    const msgAlreadyAnswered = "כבר ענית ✓ התשובה הראשונה היא שקובעת."

    func alreadyAnsweredMessage() string {
        return msgAlreadyAnswered
    }

    // msgFormatHintMCQTemplate is the Format-hint (MCQ) row. Its "1–4" digit
    // range is an LTR run embedded in RTL text exactly like the identical
    // range in msgQuestionMCQTemplate — isolated the same way, per this
    // file's header rule.
    const msgFormatHintMCQTemplate = "כדי לענות שלחו אות (א–ד) או ספרה (%s) — עוד יש זמן!"

    func formatHintMessage() string {
        return fmt.Sprintf(msgFormatHintMCQTemplate, ltr("1–4"))
    }

    // msgTooLongFreeTextTemplate is the Too-long (Free-Text) row. "200" is a
    // digit run, isolated per this file's header rule (same as every other
    // digit token here).
    const msgTooLongFreeTextTemplate = "התשובה ארוכה מדי — עד %s תווים. שלחו שוב, בקצרה!"

    func tooLongMessage() string {
        return fmt.Sprintf(msgTooLongFreeTextTemplate, ltr("200"))
    }

    // msgQuestionClosed is the Question-closed row (FR-7: late answers).
    const msgQuestionClosed = "השאלה נסגרה — מתכוננים לשאלה הבאה!"

    func questionClosedMessage() string {
        return msgQuestionClosed
    }
    ```
    Update this file's header comment map: change the five `-> story 3.3` rows to `-> story 3.3 (below)`, matching every other implemented row's format.
  - [x] `server/internal/wa/messages_he_test.go`: add canonical-copy constants + tests mirroring the existing per-row pattern (`stripIsolates` for the two templates with an isolated token; a plain string-equality test for the three without): `TestAcknowledgmentMessageMatchesCanonicalCopy`, `TestAlreadyAnsweredMessageMatchesCanonicalCopy`, `TestFormatHintMessageMatchesCanonicalCopy` + `TestFormatHintMessageIsolatesDigitToken`, `TestTooLongMessageMatchesCanonicalCopy` + `TestTooLongMessageIsolatesDigitToken`, `TestQuestionClosedMessageMatchesCanonicalCopy`.
  - [x] `server/internal/wa/inbound.go`: extend the `Registrar` interface (no other consumer-defined interface — `AnswerRecorder`, a new one — is needed: `*game.Engine` already satisfies `Registrar` structurally and is already the concrete value wired into `NewInboundRouter` in main.go, so this is a **zero-wiring-change** addition, unlike story 3.2's genuinely new `QuestionDispatcher` dependency):
    ```go
    type Registrar interface {
        Join(ctx context.Context, joinCode, phone, profileName string) (game.JoinResult, error)
        Rename(ctx context.Context, phone, displayName string) (game.RenameResult, error)
        RecordAnswer(ctx context.Context, phone, rawText string, receivedAt time.Time) (game.AnswerResult, error)
    }
    ```
    Add `"time"` to this file's imports. **Do not** add a new field to `InboundRouter`, a new constructor parameter, or touch `main.go` — `r.registrar` already carries the concrete `*game.Engine`.

    Change `Handle`'s dispatch so `kindText` routes through a new pre-check before falling back to the generic reply:
    ```go
    kind := classify(msg)
    switch kind {
    case kindJoin:
        r.handleJoin(ctx, msg)
        return
    case kindRename:
        r.handleRename(ctx, msg)
        return
    case kindText:
        r.handleTextOrAnswer(ctx, msg)
        return
    }
    r.replier.Enqueue(msg.From, replyFor(kind))
    r.logger.Info("universal reply queued", ...) // unchanged
    ```
    New method:
    ```go
    // handleTextOrAnswer tries msg as an answer to phone's currently open
    // question before falling back to the generic Help reply. Unlike
    // handleJoin/handleRename, kindText's syntax alone cannot distinguish an
    // answer attempt from ordinary chat — "1", a bare word, or a full
    // sentence are all valid Free-Text answer content, and even a bare MCQ
    // digit is only an answer when the sender genuinely has an open MCQ
    // question right now. Rather than teaching classify() (a pure function
    // with no store access, by design) to recognize answers, this handler
    // always asks the engine first; ErrNoOpenQuestion degrades to exactly
    // today's kindText behavior. This is a deliberate departure from this
    // file's original registry comment ("Stories that add a kind... answers
    // in 3.3, add it here") — that comment is now stale and updated below,
    // since real answer detection is inherently game-state-dependent, unlike
    // JOIN/rename which are self-evidently commands regardless of state.
    func (r *InboundRouter) handleTextOrAnswer(ctx context.Context, msg InboundMessage) {
        result, err := r.registrar.RecordAnswer(ctx, msg.From, msg.TextBody, msg.ReceivedAt)
        if errors.Is(err, game.ErrNoOpenQuestion) {
            r.replier.Enqueue(msg.From, replyFor(kindText))
            r.logger.Info("universal reply queued",
                "kind", string(kindText),
                "phone_last4", PhoneLast4(msg.From),
                "wa_message_id", WaMessageIDDigest(msg.WaMessageID))
            return
        }
        if err != nil {
            r.replier.Enqueue(msg.From, helpMessage())
            r.logger.Warn("answer intake failed, degrading to help",
                "error", err.Error(),
                "phone_last4", PhoneLast4(msg.From),
                "wa_message_id", WaMessageIDDigest(msg.WaMessageID))
            return
        }
        var reply string
        switch result.Outcome {
        case game.AnswerAccepted:
            reply = ackMessage()
        case game.AnswerAlreadyAnswered:
            reply = alreadyAnsweredMessage()
        case game.AnswerFormatHint:
            reply = formatHintMessage()
        case game.AnswerTooLong:
            reply = tooLongMessage()
        case game.AnswerClosed:
            reply = questionClosedMessage()
        default:
            reply = helpMessage()
        }
        r.replier.Enqueue(msg.From, reply)
        r.logger.Info("answer processed",
            "outcome", string(result.Outcome),
            "phone_last4", PhoneLast4(msg.From),
            "wa_message_id", WaMessageIDDigest(msg.WaMessageID))
    }
    ```
    Update the `allInboundKinds`/`classify`/`replyFor` doc comments that currently say "answers in 3.3, add it here" — correct them to describe the actual design (a pre-check inside `Handle`'s `kindText` case, not a new `inboundKind`).
  - [x] `server/internal/wa/inbound_test.go`: extend `stubRegistrar` with `recordAnswerResult game.AnswerResult`, `recordAnswerErr error`, `recordAnswerCalls int`, and the recorded `(phone, rawText, receivedAt)` of the last call, + the `RecordAnswer` method (**required for the package to compile** — `Registrar` gained a method). New tests:
    - `TestHandleAnswerAcceptedSendsAck`, `...AlreadyAnswered...`, `...FormatHint...`, `...TooLong...`, `...Closed...` — one per `AnswerOutcome`, asserting the exact enqueued body matches the corresponding `messages_he.go` function's output.
    - `TestHandleTextFallsThroughToHelpWhenNoOpenQuestion` — `recordAnswerErr = game.ErrNoOpenQuestion` → the existing generic Help reply, proving kindText's pre-3.3 behavior is preserved for everyone not mid-question.
    - `TestHandleTextDegradesToHelpOnUnexpectedError` — an unrelated error → Help + WARN logged (mirror `handleJoin`'s default-error-path test).
    - `TestHandleTextPassesReceivedAtToRegistrar` — asserts `stubRegistrar`'s recorded `receivedAt` equals `msg.ReceivedAt` (not `time.Now()` captured inside the router — the value must originate from the webhook layer).
    - A quick regression case confirming `kindJoin`/`kindRename`/`kindEmpty`/`kindNonText` messages never reach `stubRegistrar.RecordAnswer` at all (only `kindText` does) — pins the routing-order decision (JOIN/rename keep priority; media/empty are not answer attempts) documented above.

- [x] **Task 5: `web` — live answered-count on the control panel** (AC: 5)
  - [x] `web/src/lib/types.ts`: add `answeredCount: number` to the `CurrentQuestion` interface.
  - [x] `web/src/lib/strings.he.ts`: add to the `live` section, next to `questionProgress`: `answeredStat: (count: number) => \`${count} ענו\`` — canonical copy per EXPERIENCE.md's Live-stat row ("87 ענו"), not `"X מתוך Y ענו"` or a percentage string.
  - [x] New `web/src/features/live/response-stats.tsx` (named file per architecture's planned structure — `live/response-stats.tsx: live answered count/%`):
    ```tsx
    import { strings } from '@/lib/strings.he'

    interface ResponseStatsProps {
      count: number
    }

    // host-stat-pill (DESIGN.md): green-50 background, green-800 text,
    // border-light border, pill shape. aria-live="polite" per UX-DR14 (score/
    // count updates) — no throttling here: EXPERIENCE.md's throttling note
    // (line 236) is written for the projected Audience Display (Epic 4), not
    // this internal host-facing panel.
    export function ResponseStats({ count }: ResponseStatsProps) {
      return (
        <span
          aria-live="polite"
          className="rounded-full border border-border-light bg-green-50 px-3 py-1 text-sm text-green-800"
        >
          {strings.live.answeredStat(count)}
        </span>
      )
    }
    ```
    Check `web/src/index.css` for the exact Tailwind utility names this codebase's Tailwind v4 setup maps `--color-border-light`/`--color-green-50`/`--color-green-800` to before trusting the class names above verbatim — confirm against an existing component's usage of a sibling token.
  - [x] `web/src/features/live/control-page.tsx`: render `<ResponseStats count={snapshot.currentQuestion.answeredCount} />` inside the existing `{snapshot.currentQuestion && (...)}` block, gated additionally on `snapshot.state === 'question_open'` (AC-5 scopes this to "during an open Question"; showing a frozen count during `question_closed`/`revealed` is out of this story's AC, not a bug to prevent, but the gate keeps the panel's information scoped to what the AC asks for).
  - [x] No test file: this codebase has no Vitest tests anywhere yet (`web/src/**/*.test.tsx` — none exist); do not introduce the first one as an incidental part of this story. Verify manually per Task 6.

- [x] **Task 6: Quality gates + local E2E** (all ACs)
  - [x] Local gates: `gofmt -l .` (LF-normalize first, documented Windows workaround) · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` diff — **expected non-empty this story** (new table/queries); confirm it matches exactly what Task 1 wrote, nothing else. `npm run lint` + `npx tsc -b --noEmit` under `web/`.
  - [x] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1–3.2): create a scratch game with one MCQ question (4 options, `time_limit_seconds` generous — e.g. 60s, so the "closed" scenario below can be driven by an explicit close rather than racing a real timer) and one Free-Text question (one Accepted Answer). Open lobby, join 2 players. `POST /start` (dispatches Q1). Drive every outcome via the real webhook path (HMAC-signed inbound `POST`s), asserting each reply against the fake Meta endpoint's captured sends:
    - Player A replies `"1"` → ack (`msgAcknowledgment`); Player B replies `"ג"` → ack.
    - Player A replies `"2"` again (second answer) → `msgAlreadyAnswered`; assert still exactly one `answers` row for (Q1, A) — query the scratch DB directly, or infer via `CountAnswersByQuestion` through a temporary snapshot read if the harness already opens a `/ws` connection (reuse 2.3's harness pattern if it does; otherwise a direct DB read is simplest for a throwaway harness).
    - A third phone (not yet joined) sends `"1"` → falls through to the generic Help reply (proves `ErrNoOpenQuestion` degrades correctly and Universal Reply (2.2) is unbroken).
    - Player A sends `"5"` on... wait, A already answered Q1 — use a fresh scenario: before A/B answer, send an unparseable reply first (`"5"` or `"שלום"`) from a third joined player C → `msgFormatHintMCQ`; then C sends `"4"` → ack (proves AC-2's "leaving me able to answer" — the format-hint reply must not consume C's one answer).
    - `POST /close-question` → `POST /reveal` → `POST /next-question` (opens Q2, Free-Text). Player A sends a 205-character string → `msgTooLongFreeText`; then a ≤200-character string → ack.
    - `POST /close-question` on Q2, then have Player B (who never answered Q2) send a reply → `msgQuestionClosed`.
    - Assert the control panel's snapshot (`GET` the open-lobby/start response, or a fresh `/ws?role=host` connect) shows `currentQuestion.answeredCount` matching the accepted-answer count at each step.
    - Clean up the scratch game row afterward; delete the harness afterward (never committed) — same convention as every prior story.
  - [x] No real-phone WhatsApp verification required — the E2E above exercises the real webhook → inbound-routing → engine → store path end-to-end against a fake Meta endpoint, matching 3.2's precedent (no real-phone pass called for by this story's Dev Notes).

### Review Findings

- [x] [Review][Patch] Live answered-count never broadcasts to the control panel after a successful answer — `handleTextOrAnswer` (`wa/inbound.go`) never calls `broadcaster.Broadcast`, unlike `handleJoin`. The frontend (`use-game-socket.ts`) updates purely via WS push with no polling fallback, so AC-5's "live answered count updates via snapshots" does not hold: the `host-stat-pill` stays stale after each WhatsApp answer until an unrelated event (a join, next-question, etc.) happens to push a fresh snapshot. Fix: have `game.RecordAnswer` build and return an updated `Snapshot` on `AnswerAccepted` (mirroring `JoinResult.Created`/`.Snapshot`), and have `handleTextOrAnswer` call `r.broadcaster.Broadcast(...)` the same way `handleJoin` already does. [server/internal/wa/inbound.go:208]
- [x] [Review][Patch] Malformed/over-length replies bypass the closed/already-answered guard — `game.RecordAnswer`'s format-hint (MCQ) and too-long (Free-Text) checks return immediately after `GetOpenQuestionForPlayer` (which, per this story's own fix, no longer checks `g.state`), without ever reaching `store.RecordAnswer`'s write-time guard — the only place that distinguishes "closed" (AC-4) from "already answered" (AC-3). A malformed or over-length reply sent after the question closed, or sent by a participant who already answered, incorrectly gets the Format-hint/Too-long reply (implying they can still answer) instead of "השאלה נסגרה" / "כבר ענית". Concrete case: a participant who already answered sends a natural follow-up like "wait, actually 2" — MCQ parsing fails, so they're told they can still answer rather than that their first answer is final. Fix: resolve closed/already-answered status ahead of (or independent from) content-format validation, e.g. widen `GetOpenQuestionForPlayer` to also report game state/cutoff/whether this participant already has a row, and branch on that before the parse/length checks. [server/internal/game/answers.go:92]
- [x] [Review][Defer] `handleTextOrAnswer`'s unexpected-error path logs the raw `err.Error()` at WARN — pre-existing pattern, not introduced by this story (`handleJoin`/`handleRename`'s default-error paths already do the same). Flagged here because an unclassified store/DB error could in principle embed parameter values, in tension with the rest of this file's phone-digest-only logging posture; revisit if/when error classification is generalized across the router. [server/internal/wa/inbound.go:221] — deferred, pre-existing

## Dev Notes

### Architecture guardrails (violations = rework)

- **Dependency direction unchanged**: this story adds no new cross-package import direction. `game` gains no import of `wa`; `wa` already imports `game` (via `Registrar`) since 2.4. `store` remains the only package importing pgx.
- **State mutation stays centralized**: no new `games.state` transition. `RecordAnswer` only reads game/question/participant state and writes to the new `answers` table — it never touches `games.state`.
- **Persist-before-ack (SM-4, non-negotiable)**: `game.RecordAnswer` returns `AnswerAccepted` only after `store.RecordAnswer`'s INSERT has committed; `wa.handleTextOrAnswer` enqueues the ack only after `RecordAnswer` returns. Do not enqueue optimistically.
- **Copy centralization**: all five new templates live in `messages_he.go` only; `game/answers.go` and `wa/inbound.go` must contain zero raw Hebrew (`inbound.go`'s existing exemption is only for the `שם:` prefix token, unaffected by this story).
- **Glossary**: `Participant`, `Question`, `Answer` verbatim; no `player-response`/`submission`/other synonyms as identifiers.
- **Logging (NFR-8)**: `slog.Info` on every answer outcome (`"answer processed"`, `outcome`, `phone_last4`, `wa_message_id`) and on the fallthrough-to-Help path; `slog.Warn` only for a genuine unexpected engine/store error degrading to Help. Nothing in this story's healthy paths logs ERROR (unlike 3.2's CHECK-constraint-violation branch, this story's "unknown question type" default branch returns an `error`, which surfaces as a WARN at the `wa` layer, not a raw ERROR inside `game` — mirror the existing WARN-at-the-boundary posture, don't invent a new ERROR site).
- **NFR-2 (never block the game loop)**: `RecordAnswer` runs synchronously inside the webhook HTTP request (same as `handleJoin`/`handleRename` already do) — this is the established posture for inbound-triggered writes (unlike the *outbound* Question-dispatch burst in 3.2, which was deliberately moved off the request path). Do not detach this story's DB calls into a goroutine; a participant's own answer confirming synchronously is the point.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/game/engine.go](server/internal/game/engine.go)** — `Store` interface currently has 11 methods (through `PlayerRecipients`, story 3.2); gains 3 more. `buildSnapshot` currently builds `current *CurrentQuestion` without any answer data — this story adds one more field to the struct literal already built there, not a new code path.
- **[server/internal/game/snapshot.go](server/internal/game/snapshot.go)** — `CurrentQuestion` currently has 6 fields (through `AnswerCutoffAt`, story 3.1); gains `AnsweredCount`. `Snapshot` itself is untouched (the count nests under `CurrentQuestion`, not the root).
- **[server/internal/wa/inbound.go](server/internal/wa/inbound.go)** — `Registrar` currently has `Join`/`Rename` (2.4); `Handle`'s switch currently has `kindJoin`/`kindRename` cases plus a fallthrough to `replyFor` for everything else, including `kindText`. This story adds `RecordAnswer` to `Registrar` and inserts one new `case kindText:` arm — `kindEmpty`/`kindNonText` keep falling through exactly as before (this story does not touch their behavior).
- **[server/internal/wa/messages_he.go](server/internal/wa/messages_he.go)** — currently ends at `questionFreeTextMessage` (story 3.2). The header comment's row-status map is the one place this story edits *existing* lines (five ` -> story 3.3` rows gain ` (below)`); every other edit is additive.
- **[server/cmd/server/main.go](server/cmd/server/main.go)** — **not modified by this story.** `engine := game.NewEngine(st, ...)` already satisfies the extended `Registrar` (structural typing — no code there references the interface by name), and `inboundRouter := wa.NewInboundRouter(dispatcher, engine, hub, logger)` already passes that same `engine` value. Verify this is actually true after Task 2/4 (it should compile with zero changes to this file) rather than assuming it — if it doesn't compile unchanged, something in Task 4's interface wiring deviated from this plan.
- **[web/src/features/live/control-page.tsx](web/src/features/live/control-page.tsx)** — renders `snapshot.currentQuestion` fields (`position`, `questionCount`, `type`, `text`) inside one `flex flex-col gap-2` block (lines ~125-137 as of story 3.1). This story adds one more child to that block, conditionally on state.

### Why answer detection is not a new `inboundKind`

`server/internal/wa/inbound.go`'s existing comments (`allInboundKinds`, `classify`, `replyFor`) all anticipate "answers in 3.3" joining the kind registry the same way JOIN and the name command joined it in 2.4. That doesn't hold up under this story's actual constraint: `classify` is a pure `func(InboundMessage) inboundKind` with no store access (by design — cheap, synchronous, no I/O before routing), but whether a given text message *is* an answer depends entirely on whether its sender currently has an open question — game state `classify` cannot see. JOIN and the name command are different in kind: `"JOIN COHEN24"` and `"שם: רחל"` are unambiguous commands regardless of what state the sender is in. `"1"` or `"כן תודה"` are not — they're an answer only in context. This story therefore keeps `classify`'s taxonomy exactly as-is and adds the context-aware check as a pre-step inside `Handle`'s existing `kindText` arm (see Task 4). Update the stale doc comments; do not force a `kindAnswer` enum value into a function whose whole contract is purity.

### A pre-existing ambiguity this story makes reachable (flag, do not silently fix)

`classify` checks `parseJoinCode` (needs `len(fields) >= 2` and `fields[0] == "JOIN"`) before falling through to `kindText`. A Free-Text answer that happens to start with the English word "join" followed by another word (e.g., a participant answering a "what verb..." question with "join the club") would misroute to `handleJoin` instead of being recorded as their answer — pre-existing ordering, unchanged by this story, but only now has a real consequence (pre-3.3, any mid-game free text got the same generic Help regardless of content, so nothing was lost). Pilot posture: Hebrew-speaking audience, an English word starting a reply is unlikely. Do not fix this as part of this story (out of scope, not one of the 5 ACs) — flag it in code review for a `deferred-work.md` entry if it survives triage, same as every other narrow edge case documented there.

### The two `deferred-work.md` items this story's design resolves or evaluates

- **Two clocks for one authoritative receipt time** (deferred from story 2.1's second-round review, revisit trigger "story 3.3"): resolved by Task 2 — `wa.InboundMessage.ReceivedAt` (Go clock, captured in `webhook.go` at ingestion) is threaded through `Registrar.RecordAnswer` → `store.RecordAnswerParams.ReceivedAt` → `answers.received_at`. The cutoff *comparison* in `RecordAnswer`'s SQL guard deliberately still uses Postgres `now()` (symmetry with `CloseCurrentQuestion`'s existing `LEAST(answer_cutoff_at, now())`) — these are two intentionally different clock uses, not an inconsistency to reconcile further. Mark that `deferred-work.md` entry closed (strike-through + "closed in 3.3", matching the file's existing convention) as part of this story.
- **`Enqueue` returning accepted/dropped for delivery-truthful logging** (deferred from 2-2's review, revisit trigger "story 3.3 (acknowledgments), the first caller that genuinely needs delivery feedback"): **evaluated, not taken up.** None of this story's 5 ACs require the ack's *enqueue* logging to distinguish "queued" from "dropped on a full outbound buffer" — AC-1 only requires persist-before-enqueue ordering, which this story already gets for free (the DB write happens before `Enqueue` is ever called). Widening `Replier.Enqueue`'s signature ripples across `dispatch.go`, `dispatch_test.go`, `question_notifier.go`, `inbound.go`, `inbound_test.go` for a truthfulness improvement no AC asks for. Leave `deferred-work.md`'s entry open; do not silently take this on as unstated scope.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in place (`stubStore`, `stubRegistrar`) — no mock framework, matching every prior story. No real-DB unit tests for query correctness (the guarded-INSERT race behavior and the 23505-vs-zero-rows distinction can only be proven against real Postgres) — Task 6's `cmd/e2escratch` harness is that verification, deleted after the run, per every prior story's convention. Free-text length tests must use actual multi-byte Hebrew runes, not ASCII placeholders — `len(string)` vs `utf8.RuneCountInString` only diverges on non-ASCII input, and a test built from ASCII would pass even if the implementation used the wrong one.

### Project Structure Notes

**New:**
- `server/migrations/00010_answers.sql`
- `server/internal/store/queries/answers.sql`
- `server/internal/store/answers.go`
- `server/internal/game/answers.go`
- `server/internal/game/answers_test.go` (or folded into `engine_test.go` — see Task 2)
- `web/src/features/live/response-stats.tsx`

**Modified:**
- `server/internal/store/gen/*` (sqlc-generated — `Answer`, `GetOpenQuestionForPlayerRow`, `RecordAnswerParams`, etc.)
- `server/internal/game/engine.go` (+3 `Store` interface methods, `buildSnapshot` +`AnsweredCount`)
- `server/internal/game/snapshot.go` (+`CurrentQuestion.AnsweredCount`)
- `server/internal/game/engine_test.go` (`stubStore` extension, snapshot-test updates)
- `server/internal/wa/messages_he.go` (+5 templates, header comment update)
- `server/internal/wa/messages_he_test.go`
- `server/internal/wa/inbound.go` (+`Registrar.RecordAnswer`, +`handleTextOrAnswer`, `Handle`'s switch, stale comment fixes)
- `server/internal/wa/inbound_test.go` (`stubRegistrar` extension, new routing tests)
- `web/src/lib/types.ts` (+`CurrentQuestion.answeredCount`)
- `web/src/lib/strings.he.ts` (+`live.answeredStat`)
- `web/src/features/live/control-page.tsx` (+`ResponseStats` render)
- `_bmad-output/implementation-artifacts/deferred-work.md` (close the two-clocks item)

**Untouched:** `server/internal/httpapi/*` (no new REST route or handler — this story is entirely WhatsApp-inbound-triggered; no `control.go`/`router.go` change, unlike 3.2) · `server/cmd/server/main.go` (verify, don't assume — see above) · `server/internal/ws/*` · `server/internal/auth/*` · `server/internal/grading/*` (does not exist yet — grading is story 3.4; this story stores raw normalized responses only, never compares them against `correct_option`/`accepted_answers`).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.3] — story + all 5 epic ACs verbatim, Epic 3 context, FR-5/FR-7/FR-8
- [Source: _bmad-output/planning-artifacts/architecture.md#Data-Architecture] — "an answer is persisted before its ack is sent (SM-4)"; `UNIQUE (question_id, participant_id)`; `idx_answers_question_participant` naming example (Naming Patterns section, verbatim)
- [Source: _bmad-output/planning-artifacts/architecture.md#Project-Structure] — `game/answers.go` planned file; `live/response-stats.tsx` planned file
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#WhatsApp-message-templates] (lines 104-125) — the canonical Acknowledgment / Already-answered / Format-hint / Too-long / Question-closed copy rows, verbatim, this story's only source of truth
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md] (line 139) — Live stat "87 ענו" (`host-stat-pill`) canonical copy
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/DESIGN.md] (lines 133-137) — `host-stat-pill` token spec (green-50/green-800/border-light/pill)
- [Source: server/internal/game/engine.go, snapshot.go, state.go] — `Store` interface, `CurrentQuestion`, `buildSnapshot`, `RolePlayer`/state constants this story extends
- [Source: server/internal/store/games.go, participants.go, questions.go] — the "guard at write time" pattern (`state = 'draft'`/`allowed_states`) and the 23505-collision-handling pattern (`CreateGame`) this story's `RecordAnswer` query combines
- [Source: server/internal/wa/inbound.go, messages_he.go, webhook.go] — existing `classify`/`Handle` routing, copy-centralization pattern, `InboundMessage.ReceivedAt`'s origin (`webhook.go:297`, `time.Now().UTC()` at ingestion)
- [Source: server/internal/httpapi/control.go] — `dispatchQuestionOpened`'s fire-and-forget-after-response pattern this story deliberately does NOT use (contrast noted in Dev Notes' NFR-2 guardrail — this story's writes stay synchronous, unlike 3.2's outbound dispatch)
- [Source: _bmad-output/implementation-artifacts/3-2-question-delivery-to-every-phone.md] — `wa.QuestionNotifier`/`CurrentQuestion` shape this story's dispatch already relies on being unchanged; the `sqlc generate`-diff-must-be-empty caveat this story explicitly does NOT inherit (schema does change here)
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — the two-clocks item (story 2.1, second round) this story closes, and the `Enqueue` delivery-feedback item (story 2.2) this story evaluates and leaves open; checked in full, no other entry names story 3.3 as a trigger

## Dev Agent Record

### Agent Model Used

Claude Sonnet 5 (claude-sonnet-5), via the bmad-dev-story workflow.

### Debug Log References

- `sqlc generate` diff after Task 1: non-empty as expected (new `answers` table + 3 queries) — `internal/store/gen/answers.sql.go` (new) and `internal/store/gen/models.go` (+`Answer` struct); re-ran after the Task 6 query fix and confirmed content-identical (idempotent).
- `gofmt -l .` (LF-normalized per the documented Windows/`core.autocrlf=true` workaround, 2.2 onward) — zero real violations on every changed file; the usual ~9 pre-existing files (`store/wa_messages.go`, six `wa/*.go` files, plus this run's touched test files) were CRLF false-positives only, confirmed by re-checking each with the LF-normalized comparison.
- `go1.26.5 vet ./...` and `go1.26.5 test ./...` clean on every run; `npm run lint` / `npx tsc -b --noEmit` under `web/` clean.
- Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1–3.2): built and booted the real `cmd/server` binary against the local dev Postgres (Docker) with `WHATSAPP_API_BASE_URL` pointed at a local fake Meta endpoint recording every outbound `(to, body)` pair; organizer provisioned in-process via `store.UpsertOrganizer` (no `cmd/provision` subprocess needed). Full sequence — create game + 1 MCQ (4 options) + 1 Free-Text question → open-lobby → JOIN 3 players (A, B, C; a 4th phone D never joins) via the real webhook path (HMAC-signed) → `POST /start` (asserted 3 sends matching `questionMCQMessage`'s exact output) → C sends an unparseable reply ("5", format-hint, answer not consumed) → C/A/B each answer once (ack) → A answers again (already-answered; verified via a direct DB query that exactly 1 `answers` row exists for (Q1, A)) → D (never joined) sends "1" (falls through to generic Help, proving `ErrNoOpenQuestion` degrades correctly) → asserted `currentQuestion.answeredCount == 3` via a fresh `/ws?role=host` snapshot → `close-question` → `reveal` (0 new sends) → `next-question` (3 sends matching the Free-Text template) → A sends a 205-char reply (too-long) then a valid one (ack) → asserted `answeredCount == 1` via `/ws` → `close-question` on Q2 → B (never answered Q2) sends a reply, asserting `msgQuestionClosed`. 18 total sends, all assertions passed on a clean re-run. Scratch game row deleted afterward (cascades to participants/questions/answers via `ON DELETE CASCADE`); harness deleted afterward (never committed).
- **Bug found and fixed by the local E2E's last scenario**: the first harness run showed B receiving the generic Help reply instead of `msgQuestionClosed` after `close-question`. Root cause: `GetOpenQuestionForPlayer`'s original query (copied verbatim from the story's Task 1 spec) filtered `AND g.state = 'question_open'` — once the organizer explicitly closes the question, that read returns zero rows, so `game.RecordAnswer` returns `ErrNoOpenQuestion` *before ever reaching* `store.RecordAnswer`'s write-time guard, which is the only place the "closed" outcome is derived. This contradicts both the story's own E2E plan (Task 6's "generous time_limit_seconds... so the closed scenario can be driven by an explicit close" note) and EXPERIENCE.md's UJ-5 scripted example (a reply after closing gets "השאלה נסגרה", not the generic Help copy). Fixed by dropping the `g.state = 'question_open'` predicate from `GetOpenQuestionForPlayer` (`server/internal/store/queries/answers.sql`) — the read now resolves by `q.position = g.current_question_position` alone (any state), leaving all open/cutoff enforcement to `RecordAnswer`'s existing write-time guard, unchanged. A finished game is still correctly excluded: `FinishGame` resets `current_question_position` to 0, which matches no question. Re-ran the full Go test suite (green, no unit test exercises the raw SQL) and the E2E (both scenarios now pass, twice in a row for reliability). No AC or task was skipped by this fix — it makes AC-4 actually reachable as specified.
- E2E harness pitfall (self-inflicted, fixed before the final run): the dedupe ledger (`wa_messages`) is not cleaned between harness runs (only the scratch game row is), so reusing literal `wamid.e2e-N` strings across two runs silently deduped every inbound message on the second run (0 sends, no error). Fixed by including a per-process timestamp in every synthetic wamid.

### Completion Notes List

- All 6 tasks and their subtasks complete; all 5 ACs satisfied and verified end-to-end.
- Task 1's `answers` table, guarded queries, and `store` wrapper match the story's spec verbatim, with one fix discovered during Task 6 (see Debug Log): `GetOpenQuestionForPlayer` no longer filters on `g.state = 'question_open'`, so a reply arriving after the organizer has explicitly closed the question still resolves to that question's context and correctly receives the "closed" outcome via `RecordAnswer`'s write-time guard, rather than degrading to the generic Help reply. `RecordAnswerParams`'s field order was verified against the generated code before use.
- Persist-before-ack (SM-4) holds by construction: `wa.handleTextOrAnswer` only enqueues a reply after `Registrar.RecordAnswer` returns, and `game.RecordAnswer` only returns `AnswerAccepted` after `store.RecordAnswer`'s INSERT has committed.
- All 5 new Hebrew templates live in `messages_he.go` only, copied verbatim from EXPERIENCE.md's templates table (confirmed by direct comparison against the source doc). `wa/inbound.go` contains zero raw Hebrew literals. `game/answers.go`'s `mcqOptionLetters` map (`א`/`ב`/`ג`/`ד`) does contain raw Hebrew — this is inbound-parsing vocabulary, not outbound copy, structurally identical to `inbound.go`'s pre-existing `שם:`-prefix parsing exemption (both recognize a fixed Hebrew token in an incoming reply); it was written into the story's own Task 2 code snippet verbatim, not an addition on top of it.
- Answer detection is deliberately NOT a new `inboundKind` — `handleTextOrAnswer` is a pre-check inside `Handle`'s existing `kindText` arm, per the story's Dev Notes. Updated the stale `allInboundKinds` doc comment accordingly (it previously said "answers in 3.3, add it here").
- `main.go` required zero changes — verified by a clean build after Tasks 2 and 4: `*game.Engine` already satisfies the extended `Registrar` structurally, and the same `engine`/`inboundRouter` wiring already in place picks up `RecordAnswer` for free.
- `TestInboundUniversalReplyMatrix` (pre-existing, story 2.2) needed one adjustment: its `kindText` cases now route through `handleTextOrAnswer`, so the stub registrar was given `recordAnswerErr: game.ErrNoOpenQuestion` to preserve the exact pre-3.3 assertions (Help reply, "universal reply queued" log line) for scenarios that never set up an open question — matches the new `TestHandleTextFallsThroughToHelpWhenNoOpenQuestion` test's semantics.
- `deferred-work.md`'s two-clocks item (story 2.1, second round) closed per the Dev Notes — `wa.InboundMessage.ReceivedAt` now reaches `answers.received_at` end-to-end. The `Enqueue` delivery-feedback item (story 2.2) was evaluated and deliberately left open — none of this story's ACs require the ack enqueue to distinguish queued from dropped.
- No Vitest test added for `response-stats.tsx` — this codebase has no frontend test runner yet (confirmed: zero `*.test.tsx` files exist); verified manually via the local E2E's `/ws` snapshot assertions plus `npm run lint` / `tsc -b --noEmit`, per the story's own Task 5 instruction not to introduce the first Vitest test incidentally.
- No real-phone WhatsApp verification performed or needed — the story's Dev Notes explicitly call for none, matching 3.2's precedent; the local E2E exercises the real webhook → inbound-routing → engine → store path end-to-end against a fake Meta endpoint.

### File List

**New:**
- `server/migrations/00010_answers.sql`
- `server/internal/store/queries/answers.sql`
- `server/internal/store/answers.go`
- `server/internal/store/gen/answers.sql.go` (sqlc-generated)
- `server/internal/game/answers.go`
- `server/internal/game/answers_test.go`
- `web/src/features/live/response-stats.tsx`

**Modified:**
- `server/internal/store/gen/models.go` (sqlc-generated — +`Answer` struct)
- `server/internal/game/engine.go` (+3 `Store` interface methods, `buildSnapshot` +`AnsweredCount` via `CountAnswersByQuestion`)
- `server/internal/game/engine_test.go` (`stubStore` extension, snapshot-test `AnsweredCount` assertions, new `TestSnapshotAnsweredCountErrorPropagates`)
- `server/internal/game/snapshot.go` (+`CurrentQuestion.AnsweredCount`)
- `server/internal/wa/messages_he.go` (+5 templates: Acknowledgment, Already-answered, Format-hint, Too-long, Question-closed; header comment update)
- `server/internal/wa/messages_he_test.go` (+5 canonical-copy/isolation tests)
- `server/internal/wa/inbound.go` (+`Registrar.RecordAnswer`, +`handleTextOrAnswer`, `Handle`'s switch gained `case kindText`, stale doc-comment fixes)
- `server/internal/wa/inbound_test.go` (`stubRegistrar` extension, new answer-intake routing tests, `TestInboundUniversalReplyMatrix` adjustment)
- `web/src/lib/types.ts` (+`CurrentQuestion.answeredCount`)
- `web/src/lib/strings.he.ts` (+`live.answeredStat`)
- `web/src/features/live/control-page.tsx` (+`ResponseStats` render, gated on `question_open`)
- `_bmad-output/implementation-artifacts/deferred-work.md` (closed the two-clocks item)

## Change Log

- 2026-08-04: Dev implementation complete (all 6 tasks, all 5 ACs) — `answers` table + guarded queries, `game.Engine.RecordAnswer` (MCQ/Free-Text parse+validate+persist), live `answeredCount` on the snapshot, 5 new Hebrew templates + `wa` answer-intake routing, `ResponseStats` on the control panel. All Go quality gates green (gofmt, vet, test, `sqlc generate` diff exactly the new table/queries), frontend gates green (eslint, tsc). Local Go E2E against a real webhook path + fake Meta endpoint passed clean (18/18 assertions) after one bug found and fixed during the E2E's own run: `GetOpenQuestionForPlayer` no longer gates on `g.state = 'question_open'`, so a reply arriving after the organizer explicitly closes a question now correctly receives the "השאלה נסגרה" reply instead of falling through to generic Help (see Debug Log for the full root-cause). Closed the two-clocks deferred-work item. Status: review.
