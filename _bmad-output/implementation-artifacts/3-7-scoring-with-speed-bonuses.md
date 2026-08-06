---
baseline_commit: 827b83554cd1f36a77cda8f43bbe83b6e771038e
---

# Story 3.7: Scoring with Speed Bonuses

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a Participant,
I want correct and fast answers rewarded automatically,
so that the competition is real (FR-17).

## ⚠️ Prerequisite: verify 3.6 is actually on disk before writing code

At the time this story was created, **story 3.6 (AI Semantic Validation) is fully implemented on disk but uncommitted** — sprint-status.yaml and its own story doc both show `review`, and `git status` shows the diff still sitting in the working tree (baseline commit for this story, `827b835`, is 3.5's merge; 3.6 is layered on top, uncommitted). Verified by reading the actual files (not 3.6's story doc): `server/internal/game/engine.go` (`Store` interface, `Engine.aiGrader`/`runAsync`, `NewEngine`'s variadic `EngineOption`s, `Reveal`'s `CountUngradedAnswersForCurrentQuestion` gate), `server/internal/game/answers.go` (`RecordAnswer`'s free_text case: Exact → Fuzzy → pending-AI, `gradeAIAsync`), `server/internal/store/answers.go` (`RecordAnswerParams.IsCorrect`/`.Stage` as `*bool`/`*string`), `server/internal/store/games.go` (`RevealCurrentQuestion` wrapper, `FinishGame` resetting `current_question_position` to 0), `server/internal/store/gen/models.go` (`Answer.Seq pgtype.Int8`, `Game.PointsPerCorrect`/`SpeedBonusFirst/Second/Third int32`), and migrations 00010–00013. This story's tasks below describe that *current, on-disk* shape as the starting point.

**Re-verify these files still match before writing code** — if 3.6 has since been committed (or further changed), confirm the shapes quoted below are still accurate, especially `engine.go`'s `Reveal` function and `Store` interface, which this story rewrites.

## Acceptance Criteria

1. **Given** a revealed Question, **when** scoring runs, **then** each correct answer earns the Game's configured points, and Speed Bonuses go to at most the first, second, and third correct answers ordered by server receipt timestamp (`answers.received_at`) — ties broken by the monotonic sequence (`answers.seq`); fewer correct answers → fewer bonuses. *(epic AC-1)*
2. **Given** cumulative scores, **then** the Leaderboard ranks Participants with equal scores sharing a rank, **and** the leaderboard data is included in the game snapshot (consumed by result messages in Stories 3.8/3.9, the Audience Display in Epic 4). *(epic AC-2)*
3. **Given** `scoring.go`, **then** unit tests cover bonus ordering, timestamp ties, and fewer-than-three-correct cases. *(epic AC-3)*

## Tasks / Subtasks

- [x] **Task 1: Migration — `answers.points_awarded`** (AC: 1, 2)
  - [x] New `server/migrations/00014_answer_points.sql`:
    ```sql
    -- +goose Up
    -- Persists each answer's computed point award (FR-17, story 3.7) at
    -- Reveal time. NULL means "this answer's Question has not been revealed
    -- yet" (mirrors 00011's is_correct/stage NULL convention); a non-NULL
    -- value — including 0, for an incorrect answer or a correct one outside
    -- the top three — means "revealed and scored". Summing points_awarded
    -- over a participant's answers, filtered to IS NOT NULL, IS the
    -- cumulative leaderboard score: no separate "was this question
    -- revealed" tracking is needed, which matters because
    -- current_question_position resets to 0 on FinishGame (00009) and would
    -- otherwise make "which questions were revealed" undecidable once a
    -- game ends.
    ALTER TABLE answers ADD COLUMN points_awarded INTEGER CHECK (points_awarded IS NULL OR points_awarded >= 0);

    -- +goose Down
    ALTER TABLE answers DROP COLUMN points_awarded;
    ```
  - [x] No backfill needed (unlike 00011): every pre-existing answer row simply stays NULL ("not scored"), which is correct — nothing scored them under the old code either.

- [x] **Task 2: `game/scoring.go` — pure scoring + ranking (zero I/O)** (AC: 1, 2, 3)
  - [x] New file `server/internal/game/scoring.go`:
    ```go
    package game

    import (
        "sort"

        "github.com/avraham-shor/whatsapp-clickers/internal/store"
    )

    // ScoringConfig carries one game's points/bonus configuration
    // (games.points_per_correct/speed_bonus_first/second/third, story 1.4)
    // into AwardPoints.
    type ScoringConfig struct {
        PointsPerCorrect int32
        SpeedBonusFirst  int32
        SpeedBonusSecond int32
        SpeedBonusThird  int32
    }

    // AwardPoints computes each answer's point award for one just-revealed
    // Question (FR-17 epic AC-1). Every correct answer earns
    // cfg.PointsPerCorrect; the first three correct answers ordered by
    // ReceivedAt — ties broken by Seq, which is strictly increasing
    // (answers.seq is a BIGINT GENERATED ALWAYS AS IDENTITY, so no two
    // answers ever share one) — additionally earn SpeedBonusFirst/Second/Third.
    // Incorrect answers earn 0. Fewer than three correct answers means fewer
    // bonuses awarded, never a bonus reassigned to a lower place. Pure
    // function, no I/O: every input is already resolved by the caller
    // (game.Engine.Reveal) — mirrors grading.GradeMCQ/GradeExact's
    // "pure function first" split.
    func AwardPoints(answers []store.AnswerForScoring, cfg ScoringConfig) []store.AnswerPointsParams {
        correct := make([]store.AnswerForScoring, 0, len(answers))
        for _, a := range answers {
            if a.IsCorrect {
                correct = append(correct, a)
            }
        }
        sort.Slice(correct, func(i, j int) bool {
            if !correct[i].ReceivedAt.Equal(correct[j].ReceivedAt) {
                return correct[i].ReceivedAt.Before(correct[j].ReceivedAt)
            }
            return correct[i].Seq < correct[j].Seq
        })
        bonusByAnswerID := make(map[string]int32, 3)
        bonuses := [3]int32{cfg.SpeedBonusFirst, cfg.SpeedBonusSecond, cfg.SpeedBonusThird}
        for i, a := range correct {
            if i < len(bonuses) {
                bonusByAnswerID[a.AnswerID] = bonuses[i]
            }
        }
        points := make([]store.AnswerPointsParams, 0, len(answers))
        for _, a := range answers {
            p := int32(0)
            if a.IsCorrect {
                p = cfg.PointsPerCorrect + bonusByAnswerID[a.AnswerID]
            }
            points = append(points, store.AnswerPointsParams{AnswerID: a.AnswerID, Points: p})
        }
        return points
    }

    // LeaderboardEntry is one ranked row of the live leaderboard (FR-17/18,
    // Snapshot's wire shape — see snapshot.go).
    type LeaderboardEntry struct {
        ParticipantID string `json:"participantId"`
        DisplayName   string `json:"displayName"`
        Score         int32  `json:"score"`
        Rank          int    `json:"rank"`
    }

    // RankLeaderboard sorts scores descending and assigns standard
    // competition ranks: equal Score shares one Rank, and the next distinct
    // score's Rank skips ahead by the number of tied rows (1,1,3 — never
    // 1,1,2) — epic AC-2's "equal scores share a rank". sort.SliceStable
    // keeps tied participants in the order they arrived in scores; the
    // caller (game.Engine.buildSnapshot) passes them in ListParticipants'
    // joined_at-ASC order (store.GetLeaderboard mirrors that ORDER BY), so
    // ties break by whoever joined first — deterministic, though not itself
    // specified by the epic beyond "equal scores share a rank".
    func RankLeaderboard(scores []store.ParticipantScore) []LeaderboardEntry {
        sorted := make([]store.ParticipantScore, len(scores))
        copy(sorted, scores)
        sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })

        entries := make([]LeaderboardEntry, len(sorted))
        for i, s := range sorted {
            rank := i + 1
            if i > 0 && sorted[i-1].Score == s.Score {
                rank = entries[i-1].Rank
            }
            entries[i] = LeaderboardEntry{ParticipantID: s.ParticipantID, DisplayName: s.DisplayName, Score: s.Score, Rank: rank}
        }
        return entries
    }
    ```
  - [x] `AwardPoints`/`RankLeaderboard` take `store.AnswerForScoring`/`store.ParticipantScore`/`store.AnswerPointsParams` directly (Task 3 defines these) rather than duplicating parallel types in `game` — `game` already imports `store` for the `Store` interface's `gen.Game`/`gen.Participant` returns, so there is no new dependency-direction concern (still "game imports store, never the reverse"), and this avoids a pointless conversion layer at the `engine.go` call site.

- [x] **Task 3: `store` package — scoring types, queries, transactional Reveal** (AC: 1, 2)
  - [x] New queries in `server/internal/store/queries/answers.sql`:
    ```sql
    -- Answers to the Question at (game_id, position), already graded —
    -- read by game.Engine.Reveal AFTER its CountUngradedAnswersForCurrentQuestion
    -- pre-check confirms zero pending rows, and BEFORE the guarded
    -- RevealCurrentQuestionAndAwardPoints transaction (games.sql). Safe to
    -- read outside that transaction: RecordAnswer's own write-time guard
    -- (state = 'question_open') means no new answer can appear for this
    -- question once Reveal's caller has already observed
    -- state = 'question_closed', and nothing mutates is_correct/stage
    -- between the zero-pending check and this read (no other actor grades
    -- an already-graded row). is_correct is read via `.Bool` without a
    -- `.Valid` check downstream for the same reason — the zero-pending
    -- precondition guarantees every row here is graded.
    -- name: ListAnswersForScoring :many
    SELECT a.id, a.is_correct, a.received_at, a.seq
    FROM answers a
    JOIN questions q ON q.id = a.question_id
    WHERE q.game_id = sqlc.arg(game_id) AND q.position = sqlc.arg(position);

    -- Persists one answer's computed point award (story 3.7). Unconditional
    -- on id, same reasoning as UpdateAnswerGrade: the row was resolved by
    -- this same request's ListAnswersForScoring moments earlier, answers are
    -- never deleted (no delete-answer feature exists anywhere in this
    -- codebase), and nothing else concurrently writes points_awarded for a
    -- question only Reveal's guarded transaction can visit once.
    -- name: UpdateAnswerPoints :exec
    UPDATE answers SET points_awarded = sqlc.arg(points_awarded) WHERE id = sqlc.arg(id);

    -- Cumulative per-participant score (FR-17/18) — sums points_awarded
    -- across every revealed Question's answers; points_awarded IS NOT NULL
    -- is exactly "this answer's Question has been revealed" (see migration
    -- 00014). SUM(integer) is bigint in Postgres; ::int narrows back to
    -- int32 to match points_per_correct's own column type. LEFT JOIN so a
    -- participant with zero answers (hasn't played yet, or every answer
    -- they gave is still un-revealed) still gets a 0-score row instead of
    -- being absent from the leaderboard. role = 'player' excludes
    -- Spectators explicitly — they never answer, but this leaderboard is
    -- scoped intentionally rather than inheriting the role-blind roster gap
    -- tracked elsewhere (deferred-work.md, 2.5 review). ORDER BY mirrors
    -- ListParticipants' own tie-break exactly, so RankLeaderboard's stable
    -- sort breaks score ties by join order.
    -- name: GetLeaderboard :many
    SELECT p.id AS participant_id, p.display_name, COALESCE(SUM(a.points_awarded), 0)::int AS score
    FROM participants p
    LEFT JOIN answers a ON a.participant_id = p.id AND a.points_awarded IS NOT NULL
    WHERE p.game_id = sqlc.arg(game_id) AND p.role = 'player'
    GROUP BY p.id, p.display_name
    ORDER BY p.joined_at ASC, p.id ASC;
    ```
  - [x] New transactional method in `server/internal/store/games.go`, **replacing** the existing simple `RevealCurrentQuestion` wrapper (below it today, lines ~169-178) — same transaction pattern as `DeleteQuestion`/`ReorderQuestions` (`store/questions.go`: `s.pool.Begin` → `s.q.WithTx(tx)` → `defer tx.Rollback(ctx)` → `tx.Commit(ctx)`):
    ```go
    // RevealCurrentQuestionAndAwardPoints transitions gameID from
    // question_closed to revealed AND persists each answer's points_awarded
    // for the question just revealed, atomically (FR-17, story 3.7) — the
    // state transition and the scoring write happen together or not at all,
    // so points_awarded can never be non-NULL for a question the state
    // machine says isn't revealed, or vice versa. points is already
    // computed by game.AwardPoints (pure, no I/O) — this method only
    // persists what it's given, same posture as RecordAnswer's pre-graded
    // IsCorrect/Stage. A foreign/missing game, or one not question_closed
    // (including a lost race), is ErrNotFound.
    func (s *Store) RevealCurrentQuestionAndAwardPoints(ctx context.Context, gameID, organizerID string, points []AnswerPointsParams) (gen.Game, error) {
        tx, err := s.pool.Begin(ctx)
        if err != nil {
            return gen.Game{}, fmt.Errorf("begin reveal: %w", err)
        }
        defer tx.Rollback(ctx)
        q := s.q.WithTx(tx)

        g, err := q.RevealCurrentQuestion(ctx, gen.RevealCurrentQuestionParams{ID: gameID, OrganizerID: organizerID})
        if err != nil {
            if errors.Is(err, pgx.ErrNoRows) {
                return gen.Game{}, ErrNotFound
            }
            return gen.Game{}, err
        }
        for _, p := range points {
            if err := q.UpdateAnswerPoints(ctx, gen.UpdateAnswerPointsParams{
                ID:            p.AnswerID,
                PointsAwarded: pgtype.Int4{Int32: p.Points, Valid: true},
            }); err != nil {
                return gen.Game{}, err
            }
        }
        if err := tx.Commit(ctx); err != nil {
            return gen.Game{}, err
        }
        return g, nil
    }
    ```
    The underlying `gen.Queries.RevealCurrentQuestion` (sqlc-generated, unchanged SQL) is still used internally here — only the hand-written `*Store` wrapper around it changes shape and name. Delete the old `func (s *Store) RevealCurrentQuestion(...)` wrapper entirely; its only caller (`game.Engine.Reveal`) moves to the new method in Task 4. `games.go` does not currently import `github.com/jackc/pgx/v5/pgtype` (no existing method there needs it) — add it for the `pgtype.Int4{...}` literal above.
  - [x] New types + read/write wrappers in `server/internal/store/answers.go` (near `RecordAnswerParams`):
    ```go
    // AnswerForScoring is one graded answer's scoring inputs, read by
    // game.Engine.Reveal before it computes points via game.AwardPoints.
    type AnswerForScoring struct {
        AnswerID   string
        IsCorrect  bool
        ReceivedAt time.Time
        Seq        int64
    }

    // AnswerPointsParams carries one answer's already-computed point award
    // (game.AwardPoints, pure, no I/O) across the store boundary — same
    // "already computed, store just persists" posture as RecordAnswerParams'
    // pre-graded IsCorrect/Stage.
    type AnswerPointsParams struct {
        AnswerID string
        Points   int32
    }

    // ParticipantScore is one participant's cumulative score — the input to
    // game.RankLeaderboard.
    type ParticipantScore struct {
        ParticipantID string
        DisplayName   string
        Score         int32
    }

    // ListAnswersForScoring returns questionID's graded answers in no
    // particular order — game.AwardPoints does its own sort by
    // ReceivedAt/Seq.
    func (s *Store) ListAnswersForScoring(ctx context.Context, gameID string, position int32) ([]AnswerForScoring, error) {
        rows, err := s.q.ListAnswersForScoring(ctx, gen.ListAnswersForScoringParams{GameID: gameID, Position: position})
        if err != nil {
            return nil, err
        }
        out := make([]AnswerForScoring, 0, len(rows))
        for _, r := range rows {
            out = append(out, AnswerForScoring{AnswerID: r.ID, IsCorrect: r.IsCorrect.Bool, ReceivedAt: r.ReceivedAt, Seq: r.Seq.Int64})
        }
        return out, nil
    }

    // GetLeaderboard returns every player-role Participant's cumulative
    // score for gameID, in join order (game.RankLeaderboard sorts by score
    // and uses this order to break ties).
    func (s *Store) GetLeaderboard(ctx context.Context, gameID string) ([]ParticipantScore, error) {
        rows, err := s.q.GetLeaderboard(ctx, gameID)
        if err != nil {
            return nil, err
        }
        out := make([]ParticipantScore, 0, len(rows))
        for _, r := range rows {
            out = append(out, ParticipantScore{ParticipantID: r.ParticipantID, DisplayName: r.DisplayName, Score: r.Score})
        }
        return out, nil
    }
    ```
    Note `Answer.Seq`/`GetOpenQuestionForPlayerRow`-style nullable scanning: `r.IsCorrect`/`r.Seq` come back as `pgtype.Bool`/`pgtype.Int8` (no override configured for bool/int8 — same as the existing `Answer.Seq pgtype.Int8` in `gen/models.go`), hence `.Bool`/`.Int64`. `GetLeaderboard`'s `score` column has no nullable-type wrinkle: the `::int` cast plus `COALESCE` produces a plain non-null `int32` in the generated row.
  - [x] `sqlc generate` — expect a real diff (`answers.sql.go` gains `ListAnswersForScoring`/`UpdateAnswerPoints`/`GetLeaderboard` + their param/row types; `games.sql.go` loses nothing since `RevealCurrentQuestion`'s underlying SQL/generated function is unchanged, only its hand-written wrapper moved; `models.go` unaffected — `points_awarded` doesn't touch a table already mirrored by a `gen` struct field... actually it does: `Answer.PointsAwarded pgtype.Int4` is a new field on the existing `gen.Answer` struct. Expect that addition.).

- [x] **Task 4: `game/engine.go` — wire scoring into `Reveal`, extend `Store` interface, snapshot gains `Leaderboard`** (AC: 1, 2)
  - [x] `Store` interface changes: remove `RevealCurrentQuestion(ctx, gameID, organizerID string) (gen.Game, error)`; add:
    ```go
    ListAnswersForScoring(ctx context.Context, gameID string, position int32) ([]store.AnswerForScoring, error)
    RevealCurrentQuestionAndAwardPoints(ctx context.Context, gameID, organizerID string, points []store.AnswerPointsParams) (gen.Game, error)
    GetLeaderboard(ctx context.Context, gameID string) ([]store.ParticipantScore, error)
    ```
  - [x] Rewrite `Reveal` (replaces the current `g, err = e.store.RevealCurrentQuestion(ctx, gameID, organizerID)` call):
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

        answers, err := e.store.ListAnswersForScoring(ctx, gameID, g.CurrentQuestionPosition)
        if err != nil {
            return Snapshot{}, err
        }
        cfg := ScoringConfig{
            PointsPerCorrect: g.PointsPerCorrect,
            SpeedBonusFirst:  g.SpeedBonusFirst,
            SpeedBonusSecond: g.SpeedBonusSecond,
            SpeedBonusThird:  g.SpeedBonusThird,
        }
        points := AwardPoints(answers, cfg)

        g, err = e.store.RevealCurrentQuestionAndAwardPoints(ctx, gameID, organizerID, points)
        if err != nil {
            if errors.Is(err, store.ErrNotFound) {
                // Same race-loss reinterpretation as every other transition
                // here: the guard (state = 'question_closed') lost the race
                // between the read above and this write. Only the initial
                // RevealCurrentQuestion UPDATE inside the transaction can
                // plausibly return zero rows this way — the UpdateAnswerPoints
                // loop's targets were resolved moments ago by this same
                // request and nothing else deletes/reassigns answer rows, so
                // an error from that loop propagates here as a genuine,
                // un-reinterpreted failure instead.
                return Snapshot{}, ErrNotQuestionClosed
            }
            return Snapshot{}, err
        }
        return e.snapshotAfterCommit(ctx, g), nil
    }
    ```
    Keep the existing doc comment above `Reveal` (state-transition summary) — extend it with one sentence noting it now also computes and persists Speed Bonus scoring for the revealed question (FR-17).
  - [x] `buildSnapshot`: after the existing `questions`/`current` block, add:
    ```go
    scores, err := e.store.GetLeaderboard(ctx, g.ID)
    if err != nil {
        return Snapshot{}, err
    }
    leaderboard := RankLeaderboard(scores)
    ```
    and add `Leaderboard: leaderboard` to the returned `Snapshot{...}` literal.
  - [x] `emptySnapshot`: add `Leaderboard: []LeaderboardEntry{}` to its returned literal (same "never null on the wire" discipline as `Participants: []ParticipantSummary{}`).

- [x] **Task 5: `game/snapshot.go` — `Leaderboard` field** (AC: 2)
  - [x] Add to `Snapshot`: `Leaderboard []LeaderboardEntry `+"`"+`json:"leaderboard"`+"`"+`` (place it after `CurrentQuestion`, matching the struct's existing field order from outer game-shape to per-question detail).

- [x] **Task 6: `web/src/lib/types.ts` — mirror the wire shape (type-only, no rendering)** (AC: 2)
  - [x] Add `LeaderboardEntry` interface and a `leaderboard: LeaderboardEntry[]` field on `LobbySnapshot`, matching `game.Snapshot`'s new JSON shape exactly (camelCase keys already match `LeaderboardEntry`'s `json` tags). No component reads this field yet — Stories 3.8/3.9 (WhatsApp messages) and Epic 4 (Audience Display) are the consumers; this task only keeps the documented "mirrors the Go game.Snapshot" contract accurate, same discipline the file's own header comment states. **Explicitly not in scope**: any `control-page.tsx` rendering of the leaderboard — no AC in this story asks for it, and the architecture's file tree places `leaderboard-stage.tsx` under Epic 4's audience-display components, not Epic 3's control panel.

- [x] **Task 7: Update existing tests for the `Store` interface / `Reveal` changes** (AC: 1, 2)
  - [x] `server/internal/game/engine_test.go`'s `stubStore`:
    - Remove `revealCurrentQuestionResult`/`Err`/`Calls` and the `RevealCurrentQuestion` stub method.
    - Add `revealCurrentQuestionAndAwardPointsResult gen.Game`, `revealCurrentQuestionAndAwardPointsErr error`, `revealCurrentQuestionAndAwardPointsCalls int`, `revealCurrentQuestionAndAwardPointsArg []store.AnswerPointsParams`, and the matching method (records `arg` for new tests to assert on, same pattern as `recordAnswerArg`).
    - Add `listAnswersForScoringResult []store.AnswerForScoring`, `listAnswersForScoringErr error`, plus a matching `ListAnswersForScoring` method (zero-value default — nil slice, nil error — is safe: `AwardPoints(nil, cfg)` returns an empty, non-nil slice without panicking).
    - Add `getLeaderboardResult []store.ParticipantScore`, `getLeaderboardErr error`, plus a matching `GetLeaderboard` method (zero-value default is likewise safe: `RankLeaderboard(nil)` returns `[]LeaderboardEntry{}`).
    - These three additions are **required for `stubStore` to keep satisfying the `Store` interface at all** — every existing test in `engine_test.go`/`answers_test.go`/`participants_test.go` that reaches any successful `buildSnapshot` call (which is most of them) will keep compiling and passing unchanged once the zero-value stub methods exist, exactly the same low-blast-radius shape as 3.6's `WithAIGrader`/`WithAsyncRunner` addition — no other test in those files needs its fixture data touched.
  - [x] Update the 5 existing Reveal-path assertions that reference the old field/method name (`server/internal/game/engine_test.go`, "--- Reveal ---" section, currently lines ~565-656): `TestRevealFromQuestionClosedReturnsRevealedSnapshot`, `TestRevealFromNonQuestionClosedReturnsErrNotQuestionClosedWithoutWriting`, `TestRevealRaceLossReturnsErrNotQuestionClosed` (sets `st.revealCurrentQuestionErr` → rename to `st.revealCurrentQuestionAndAwardPointsErr`), `TestRevealSucceedsWhenNoOutstandingGrades`, `TestRevealReturnsErrGradingIncompleteWhenOutstandingGradesExist` — every `st.revealCurrentQuestionCalls` read becomes `st.revealCurrentQuestionAndAwardPointsCalls`. `questionClosedStub()` needs no other change (its `revealCurrentQuestionResult` field rename is covered above); it already returns a `gen.Game` with `PointsPerCorrect`/`SpeedBonusFirst/Second/Third` at their Go zero value (0) unless a test sets them — fine for these pre-existing tests, none of which assert on scoring.

- [x] **Task 8: New tests — scoring, ranking, and the Reveal integration path** (AC: 1, 2, 3)
  - [x] New `server/internal/game/scoring_test.go` (pure, zero I/O — mirrors `grading`'s test style):
    - `TestAwardPointsGivesTopThreeCorrectSpeedBonusesInReceiptOrder` — 5 correct answers at distinct `ReceivedAt` values plus 1 incorrect; assert the 1st/2nd/3rd-by-time get `PointsPerCorrect+SpeedBonusFirst/Second/Third`, the 4th/5th get bare `PointsPerCorrect`, the incorrect one gets `0`.
    - `TestAwardPointsBreaksReceiptTiesBySeq` — two correct answers sharing one `ReceivedAt`, distinguished only by `Seq`; assert the lower `Seq` wins the earlier bonus slot.
    - `TestAwardPointsWithFewerThanThreeCorrectAnswersOnlyBonusesThoseThatExist` — exactly 1 correct answer among several incorrect; assert it gets `PointsPerCorrect+SpeedBonusFirst` and nothing errors or panics over the "missing" 2nd/3rd slots.
    - `TestAwardPointsZeroCorrectAnswersReturnsAllZero` — every answer incorrect; assert every `Points == 0` and the function doesn't panic on an empty `correct` slice.
    - `TestRankLeaderboardSharesRankAcrossTiedScoresAndSkipsAhead` — scores `[100, 100, 50]` → ranks `[1, 1, 3]` (not `[1, 1, 2]`).
    - `TestRankLeaderboardBreaksTiesByInputOrder` — two equal-score entries in a known input order; assert the output preserves that relative order (proves the stable sort, since `store.GetLeaderboard`'s `ORDER BY joined_at ASC` is what actually establishes "who's first" among ties).
  - [x] New Reveal-integration test in `engine_test.go`, e.g. `TestRevealComputesAndPersistsSpeedBonusPoints` — `questionClosedStub()` + a populated `listAnswersForScoringResult` (3+ answers with distinct `ReceivedAt`, mixed correct/incorrect) + a `game.PointsPerCorrect`/`SpeedBonusFirst/Second/Third` set to distinguishable non-default values; call `Reveal`; assert `st.revealCurrentQuestionAndAwardPointsArg` contains exactly the points `AwardPoints` would compute for that input (either by calling `AwardPoints` directly in the test to build the expectation, or by asserting each specific `AnswerID`→`Points` pair) — this is what proves Task 4's wiring, not just Task 2's pure function in isolation.
  - [x] New snapshot test, e.g. `TestBuildSnapshotIncludesRankedLeaderboard` — a stub with `getLeaderboardResult` populated (including a tie) and any successful transition (e.g. `OpenLobby`); assert `snap.Leaderboard` is the `RankLeaderboard`-shaped, non-nil result (and separately, a test with `getLeaderboardResult` left at its zero value asserts `snap.Leaderboard` is `[]LeaderboardEntry{}`, not nil — the wire-shape discipline).

- [x] **Task 9: Quality gates + local E2E** (all ACs)
  - [x] Local gates: `gofmt -l .` (CRLF-checkout caveat from 3.4-3.6 still applies — verify only files this story touches) · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` diff — expect a real diff this time (Task 3's new queries + `Answer.PointsAwarded`). `npx tsc -b` / `npm run lint` for Task 6's `types.ts`-only change.
  - [x] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1-3.6): create a scratch game with **two** questions (so cumulative scoring across questions is actually exercised, per AC-2) — Q1 free_text (`accepted_answers: ["ירושלים"]`), Q2 mcq. Open lobby, join 4 players (A, B, C, D).
    - `POST /start`. On Q1: A, B, C answer correctly with staggered timing (send sequentially with a small real delay, or drive `game.Engine.RecordAnswer` in-process with explicit `receivedAt` values if the harness has direct engine access — either is fine as long as the three receipt times are genuinely ordered) so their `received_at` values differ; D answers incorrectly.
    - `close-question` → `reveal`. Query the DB directly: confirm A/B/C's `points_awarded` are `PointsPerCorrect+SpeedBonusFirst/Second/Third` respectively (in receipt order) and D's is `0`. Confirm the WS/REST snapshot's `leaderboard` field ranks A > B > C > D with `rank` 1,2,3,4 and `score` matching those `points_awarded` values (single-question cumulative == per-question here).
    - `next-question` to Q2. Configure it so exactly two players land on an equal *cumulative* score after Q2's reveal (e.g. both C and D answer Q2 correctly with no bonus room left, or set Q2's bonuses to 0 and have both answer correctly) — confirm the post-reveal leaderboard shows those two sharing one `rank` and the next distinct score's `rank` skips ahead (e.g. `1,1,3` not `1,1,2`), proving AC-2 end-to-end, not just in `scoring_test.go`.
    - Clean up the scratch game row afterward (cascades); delete the harness afterward — same convention as every prior story.

### Review Findings

**Resolved 2026-08-06** — the decision item was answered "accept and document" and all 13 patches were applied; see the Post-Review Changes subsection below for what landed and how each fix was verified.

Code review 2026-08-06 (Blind Hunter / Edge Case Hunter / Acceptance Auditor, all three layers completed). Verified against the working tree at review time: `go vet ./...` clean, `go test ./...` clean. **AC verdict: AC-1 MET, AC-2 MET, AC-3 PARTIALLY MET.** The delivered code matches the story's verbatim Task 1–8 code blocks essentially character-for-character — there are no substantive deviations from spec intent, and no finding below disputes `AwardPoints`/`RankLeaderboard`'s behavior as written. The high-severity findings are all *verification* gaps, four of them proven by mutation testing on a throwaway copy of the repo (each mutation left `go test ./internal/game/` green).

- [x] [Review][Decision] **Migration 00014 ships no backfill, so every answer to an already-revealed question is permanently invisible to the leaderboard** — the `ALTER TABLE` adds a nullable column with no `UPDATE`, so pre-existing answers keep `points_awarded = NULL` and `GetLeaderboard`'s `LEFT JOIN ... AND a.points_awarded IS NOT NULL` drops them; those participants read 0 forever. Nothing back-fills later: `Reveal` only ever writes points for `g.CurrentQuestionPosition`, and that position advances monotonically. The migration's own header asserts "`points_awarded IS NOT NULL` is exactly 'this answer's Question has been revealed'" — false for every pre-migration row, which quietly breaks the invariant the whole design rests on. Migration `00011_answer_grading.sql:19-33` added an explicit backfill for a structurally identical problem and its comment records why ("any game sitting on a question with pre-3.4 answers could never be revealed again… Code review finding, story 3.4"); that precedent is not followed here. Decision needed because the options genuinely differ and depend on deployment reality: (a) full recompute backfill in SQL — correct but must re-derive bonus ordering historically from `received_at`/`seq` and the `games` config, non-trivial; (b) flat backfill of `points_per_correct` with no bonuses for already-revealed correct answers — simple, slightly wrong; (c) accept and document, on the grounds that no deployed game has passed a reveal yet; (d) `points_awarded = 0` for all pre-existing rows, making them "scored, zero". [server/migrations/00014_answer_points.sql:13, server/internal/store/queries/answers.sql (GetLeaderboard), server/migrations/00011_answer_grading.sql:19-33]

- [x] [Review][Patch] AC-1's *primary* sort key is verified by nothing — a `seq`-only comparator passes the whole suite [server/internal/game/scoring_test.go] — every correct answer in every test has `Seq` order identical to `ReceivedAt` order, so the two keys are indistinguishable; the tie test holds `ReceivedAt` constant and therefore exercises only the *secondary* key. **Mutation-proven:** replacing the entire comparator in `scoring.go` with `return correct[i].Seq < correct[j].Seq` — deleting `ReceivedAt` from the ordering outright — leaves `go test ./internal/game/` green. This is exactly the regression AC-1 exists to prevent ("ordered by server receipt timestamp (`answers.received_at`) — ties broken by the monotonic sequence"), and `seq` (DB insert order) and `received_at` (webhook ingestion clock) genuinely diverge under concurrent webhook handling. Fix: one test case where `Seq` order is the *inverse* of `ReceivedAt` order, asserting `received_at` wins.
- [x] [Review][Patch] `RankLeaderboard`'s descending sort is verified by nothing — deleting it passes the whole suite [server/internal/game/scoring_test.go] — both ranking tests feed input that is *already* sorted descending. The rank loop is adjacent-only (`sorted[i-1].Score == s.Score`), so the sort is load-bearing for AC-2: on unsorted input, equal scores that are not adjacent do not share a rank. **Mutation-proven:** replacing the `sort.SliceStable` call with a no-op leaves `go test ./internal/game/` green, and `TestBuildSnapshotIncludesRankedLeaderboard` cannot catch it either because it builds its expectation from the mutated function. (Control: mutating the rank arithmetic to dense ranking `1,1,2` *is* caught — only the sort is unguarded.) Fix: feed the rank test input in ascending or shuffled order.
- [x] [Review][Patch] `Reveal`'s question-position wiring is untested — scoring the wrong question passes every test [server/internal/game/engine_test.go:145-147] — the `ListAnswersForScoring` stub discards both arguments and records nothing. **Mutation-proven:** changing `engine.go`'s call to `e.store.ListAnswersForScoring(ctx, gameID, 999)` leaves `go test ./internal/game/` green. Task 7 applied the arg-recording pattern (`revealCurrentQuestionAndAwardPointsArg`, "same pattern as `recordAnswerArg`") to the write method but not to this read, so the one thing tying scoring to the *revealed* question is unasserted. Fix: record `gameID`/`position` on the stub and assert `position == g.CurrentQuestionPosition` in `TestRevealComputesAndPersistsSpeedBonusPoints`.
- [x] [Review][Patch] `emptySnapshot`'s "never null on the wire" leaderboard default is untested, and both new error-propagation paths are dead [server/internal/game/engine.go:516, server/internal/game/engine_test.go:74,77] — **mutation-proven:** deleting `Leaderboard: []LeaderboardEntry{}` from `emptySnapshot` leaves `go test ./internal/game/` green. The one non-nil assertion added (in `TestOpenLobbyFromDraftReturnsLobbySnapshot`) covers the `buildSnapshot` success path only, never the degraded `snapshotAfterCommit` fallback — which is the only path where the field can actually be nil, and the TS contract (`leaderboard: LeaderboardEntry[]`, non-nullable) depends on it. Separately, `listAnswersForScoringErr` and `getLeaderboardErr` are declared and wired into the stub but never assigned by any test, so the new `return Snapshot{}, err` after `GetLeaderboard` is unexercised — a new failure mode for *every* transition, since a `GetLeaderboard` error now collapses the entire post-transition snapshot to `emptySnapshot`, discarding participants, question count and current question for every WS client. Fix: one degraded-path test plus one test per error field.
- [x] [Review][Patch] `answers.participant_id` is unindexed while `GetLeaderboard` now runs on every snapshot build — including once per inbound answer webhook [server/migrations/00014_answer_points.sql, server/internal/game/engine.go:486] — the only index on `answers` is `UNIQUE (question_id, participant_id)`, whose leading column is `question_id`, unusable for `LEFT JOIN answers a ON a.participant_id = p.id`. Every leaderboard read therefore sequentially scans the whole `answers` table and aggregates it. `buildSnapshot` is called by `snapshotAfterAnswer` once per accepted WhatsApp answer, so a question-open burst multiplies that by room size against a pool of `max(4, numCPU)` — the exact contention `WithMaxConcurrentAIGrades` was added in 3.6 to prevent, after "a hundred waiters starved a pgxpool sized max(4, numCPU), stalling the webhook handlers still recording answers." Also the standard un-indexed-FK case (`participant_id REFERENCES participants(id) ON DELETE CASCADE`). Fix: add `CREATE INDEX idx_answers_participant ON answers (participant_id);` to migration 00014 (with the matching `DROP INDEX` in its Down block).
- [x] [Review][Patch] The reveal transaction issues one round-trip per answer inside a fixed 5s request budget, with no batching and no bound [server/internal/store/games.go:195-202, server/internal/httpapi/control.go:163] — `ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)` now covers `GetGameForOrganizer` + `CountUngradedAnswers` + `ListAnswersForScoring` + `BEGIN` + 1 + N `UPDATE`s + `COMMIT`, where it previously covered a single guarded `UPDATE`. On expiry mid-loop `defer tx.Rollback(ctx)` aborts everything: no transition, no points, and `Reveal` returns `context.DeadlineExceeded`, which `writeStoreError`'s default arm reports as `503 DB_UNAVAILABLE`. Because the deadline is deterministic rather than transient, **every retry re-does the same O(N) work against the same 5s and fails identically** — and `Reveal` is the only route to `revealed` and therefore to `NextQuestion`, so the game wedges at `question_closed`. Note this also contradicts the story's own Dev Notes guardrail ("NFR-2 — no new concern here… No new latency source"), which is true of the *computation* but not of the persistence path it prescribes; `epics.md:67` requires the design not preclude the 500-participant target. Fix: one set-based statement — `UPDATE answers SET points_awarded = v.p FROM unnest($1::uuid[], $2::int[]) AS v(id, p) WHERE answers.id = v.id`.
- [x] [Review][Patch] Assertions loop over the *result* rather than the *expectation*, so short or missing output passes silently [server/internal/game/scoring_test.go, server/internal/game/engine_test.go] — `TestRankLeaderboardSharesRankAcrossTiedScoresAndSkipsAhead` does `for i, e := range entries` against `wantRanks := []int{1, 1, 3}` with no `len(entries)` assertion: if `RankLeaderboard` regressed to returning zero or one entry, the body never reaches the tie case and the test reports PASS — and this is the single test guarding AC-2. The map-based assertions have the same shape from the other direction: `pointsByAnswerID` returns a map and both `TestAwardPointsGivesTopThreeCorrectSpeedBonusesInReceiptOrder` and `TestRevealComputesAndPersistsSpeedBonusPoints` do `for id, wantPoints := range want { if got[id] != wantPoints`, so the `"a6": 0` expectation is satisfied equally by `a6` being *absent* — i.e. by `AwardPoints` silently dropping incorrect answers, which would leave their `points_awarded` NULL and break migration 00014's "non-NULL means revealed-and-scored" invariant. Only `TestAwardPointsZeroCorrectAnswersReturnsAllZero` checks a length, and only for the all-incorrect shape. Fix: `len()` assertions before every such loop.
- [x] [Review][Patch] The Reveal integration test builds its expected values by calling the unit under test [server/internal/game/engine_test.go:279] — `want := pointsByAnswerID(AwardPoints(answers, cfg))` moves in lockstep with any change to `AwardPoints`, so despite its name `TestRevealComputesAndPersistsSpeedBonusPoints` proves nothing about the awards themselves. Task 8 explicitly permitted this ("either by calling `AwardPoints` directly in the test to build the expectation, or by asserting each specific `AnswerID`→`Points` pair"), and mutation confirms it *does* catch config-plumbing errors (swapping `SpeedBonusSecond`/`SpeedBonusThird` in `Reveal`'s `cfg` fails it), so this is a cheap strengthening rather than a spec violation — but with the E2E harness deleted, hard-coded values are the only durable in-repo evidence of the actual award arithmetic. Fix: hard-code `a1: 150, a2: 130, a3: 0`.
- [x] [Review][Patch] `UpdateAnswerPoints` carries no write-time guard, diverging from the finality guard `UpdateAnswerGrade` added for the same reason [server/internal/store/queries/answers.sql:189-190] — `UPDATE answers SET points_awarded = sqlc.arg(points_awarded) WHERE id = sqlc.arg(id)` silently overwrites an already-published award instead of no-opping. `UpdateAnswerGrade`, two definitions above in the same file, carries `AND stage IS NULL` and its comment states the governing rule: "once a row is graded… that grade is final; a verdict landing afterwards is a no-op rather than a silent rewrite of a value the Organizer may already have revealed to the room (**and that story 3.7's scoring will read**)." The identical argument applies to a score already announced to the room. The new query's comment argues instead from "nothing else concurrently writes", which is a concurrency argument where the precedent's reasoning was about finality. Fix: `AND points_awarded IS NULL`.
- [x] [Review][Patch] `ListAnswersForScoring` is the one position-keyed query in the codebase without a `created_at` tie-break — it unions colliding questions instead of picking one [server/internal/store/queries/answers.sql:177-181] — `WHERE q.game_id = $1 AND q.position = $2` matches *every* question at that position, and `00003` deliberately declines `UNIQUE (game_id, position)` (this file's own comment at lines 36-43 records that `CreateQuestion`'s `max(position)+1` "has no backstop against concurrent inserts"). On a collision, `AwardPoints` would allocate the three speed bonuses across the union of two questions' answers and the `UpdateAnswerPoints` loop would stamp `points_awarded` on rows belonging to a question that was never revealed — breaking the "non-NULL ⟺ revealed" invariant `GetLeaderboard` depends on. Every other position-resolving query carries `q.created_at` in its ORDER BY for exactly this reason. Practically inert today (reads deterministically resolve to the earlier-created row, so the second question accumulates no answers) — the gap is that nothing enforces it. Fix: resolve to a single question id, mirroring `answers.sql:60`.
- [x] [Review][Patch] The replaced `Reveal` error comment drops a still-accurate known-issue marker at the exact moment it went live [server/internal/game/engine.go:294-303] — the deleted comment enumerated three causes of `ErrNotFound` (concurrent transition · no `questions` row matching `current_question_position`, the `UPDATE ... FROM` join being inner · an ungraded answer), said plainly "the reported cause can be wrong", and left a marker: "Revisit when Story 3.6 makes the grading condition genuinely reachable: at that point these deserve distinct errors rather than one message that can send an operator after the wrong problem." Story 3.6 has now landed (`7a4e55e`), and `queries/games.sql`'s `RevealCurrentQuestion` still carries both extra zero-row causes verbatim (`AND q.position = g.current_question_position` and `AND NOT EXISTS (SELECT 1 FROM answers a WHERE a.question_id = q.id AND a.stage IS NULL)`). The replacement text asserts "Only the initial `RevealCurrentQuestion` UPDATE inside the transaction can plausibly return zero rows this way" — correct about *which statement* fails, but it silently drops the *why*, so the tracked item now exists nowhere in the code. The dev followed Task 4's verbatim snippet, so this is a spec-authoring miss rather than a dev error. Fix: restore the three-cause enumeration alongside the new transaction-scoping sentence.
- [x] [Review][Patch] `ListAnswersForScoring`'s doc comment describes a parameter the function does not take [server/internal/store/answers.go:274] — "returns questionID's graded answers" while the signature is `(ctx, gameID string, position int32)`. A reader trusting it will miss that the lookup is keyed on `g.CurrentQuestionPosition`, read earlier from a game row that is not re-read inside the transaction. The comment also asserts the answers are "graded", which the SQL does not enforce (it is guaranteed upstream by the zero-ungraded pre-check, not by this query). Fix: correct the comment to `(gameID, position)` and attribute the graded-ness guarantee to the caller.
- [x] [Review][Patch] The E2E harness binary was not cleaned up [server/e2escratch.exe] — Task 9 requires "delete the harness afterward — same convention as every prior story". `server/cmd/e2escratch/` is correctly gone, but the compiled 23 MB `server/e2escratch.exe` is still sitting in the working tree. It is invisible to `git status` because `.gitignore:3` matches `*.exe`, which is exactly why it survived. Fix: delete the file.

- [x] [Review][Defer] Answers landing in the close→reveal race window get `points_awarded = NULL` forever [server/internal/game/engine.go:288-296, server/internal/store/games.go] — deferred, pre-existing race-window class
- [x] [Review][Defer] The degraded `emptySnapshot` broadcasts an empty leaderboard that is indistinguishable from "everyone scored 0" [server/internal/game/engine.go:436-445,508-518] — deferred, pre-existing degrade design
- [x] [Review][Defer] No automated coverage for the new SQL or for the transaction itself [server/internal/store/games.go, server/internal/store/queries/answers.sql] — deferred, pre-existing documented testing standard
- [x] [Review][Defer] Clock skew between instances defeats `received_at` ordering, and `seq` only breaks *exact* ties [server/internal/game/scoring.go:37-42, server/internal/wa/webhook.go:297] — deferred, inherent to the wall-clock design

#### Post-Review Changes (2026-08-06)

**Decision resolved — no backfill, documented.** Migration 00014 now carries an explicit block explaining why it declines the backfill that 00011 performed (a missing grade *blocks* reveal; a missing `points_awarded` only scores an already-revealed question 0, so nothing wedges), naming the decision, its owner, and the condition under which it would have to be revisited. This is the one place the "IS NOT NULL means revealed" invariant does not hold, and it now says so.

**Schema.** New migration `00015_answers_participant_index.sql` adds `CREATE INDEX idx_answers_participant ON answers (participant_id)`, so `GetLeaderboard`'s LEFT JOIN stops sequentially scanning `answers` on every snapshot build.

The index was **first** written into 00014 itself, and the E2E re-run below is what proved that wrong: 00014 had already been applied to the dev database by the story's original E2E, and goose keys applied migrations by version number alone — it never re-reads a file it has already run. The edited 00014 would therefore have created the index on no database that already had it, while its `Down` block failed immediately on `DROP INDEX` for something that did not exist (`SQLSTATE 42704`, the first thing the harness hit). 00014 is now byte-identical to its original DDL apart from the no-backfill comment, and the index lives in its own migration. Worth recording as the general rule this violated: never edit an applied migration, even an unmerged one — "unreleased" is not the same as "unapplied".

**SQL.** `ListAnswersForScoring` now resolves `(game_id, position)` to exactly one question via `ORDER BY q.created_at LIMIT 1`, matching `GetOpenQuestionForPlayer` and `buildSnapshot`. `UpdateAnswerPoints` was replaced by `UpdateAnswerPointsBatch` — one set-based `UPDATE ... FROM (SELECT unnest(...))` carrying `AND a.points_awarded IS NULL`, which both collapses the O(answers) round-trip loop to a single statement and gives the write the same finality guard `UpdateAnswerGrade` carries. (The two single-argument unnests instead of `unnest(a, b) AS v(id, points)` are a sqlc v1.31 catalog limitation, noted at the query.) `sqlc generate` re-run; `store/games.go` builds the two parallel slices and no longer needs `pgtype`.

**Comments.** `Reveal`'s error-collapse comment restores the three-cause enumeration (concurrent transition · inner `UPDATE ... FROM` join misses · `NOT EXISTS` ungraded clause) that the story's Task 4 snippet had dropped, and records that 3.6 made the grading cause reachable, so the item stays tracked in the code. `store.ListAnswersForScoring`'s doc comment now describes the parameters it actually takes and attributes the graded-ness guarantee to the caller's pre-check rather than to the query.

**Tests — verified by mutation, not by inspection.** Each of the five mutations below was applied to a throwaway copy of the tree and `go test ./internal/game/` re-run. All five were green before this round and **all five now fail**:

| Mutation | Before | After |
|---|---|---|
| Comparator drops `ReceivedAt`, sorts by `Seq` alone | 🟢 passed | 🔴 caught |
| `RankLeaderboard`'s `sort.SliceStable` becomes a no-op | 🟢 passed | 🔴 caught |
| `Reveal` scores position `999` instead of the current one | 🟢 passed | 🔴 caught |
| `emptySnapshot` drops `Leaderboard: []LeaderboardEntry{}` | 🟢 passed | 🔴 caught |
| `AwardPoints` drops incorrect answers from its result | 🟢 passed | 🔴 caught |

Concretely: new `TestAwardPointsOrdersByReceivedAtNotBySeq` inverts `Seq` order against receipt order so only a `received_at`-primary sort passes; `TestRankLeaderboardSharesRankAcrossTiedScoresAndSkipsAhead` now takes unsorted input and asserts whole entries against hard-coded values; `TestBuildSnapshotIncludesRankedLeaderboard` and `TestRevealComputesAndPersistsSpeedBonusPoints` no longer derive `want` by calling the unit under test; `len()` assertions precede every map/slice comparison; the stub records `ListAnswersForScoring`'s arguments and `TestRevealComputesAndPersistsSpeedBonusPoints` asserts the position; and three new tests cover the degraded `emptySnapshot` path (`TestSnapshotAfterCommitDegradesWithNonNilLeaderboard`) plus the two previously-dead error fields (`TestRevealPropagatesListAnswersForScoringError`, `TestSnapshotPropagatesGetLeaderboardError`).

**Housekeeping.** The leftover 23 MB `server/e2escratch.exe` was deleted (it survived because `.gitignore:3` hides `*.exe` from `git status`).

**Gates after the changes:** `sqlc generate` clean · `gofmt -l` clean on `scoring.go`/`scoring_test.go` (the repo-wide CRLF artifact on pre-existing files is unchanged) · `go vet ./...` clean · `go test ./...` fully green.

**Post-review E2E against live Postgres (2026-08-06).** The review's SQL changes were initially type-checked by `sqlc` only, so a fresh throwaway `cmd/e2escratch` harness was written and run against the real local database (`postgres://postgres:dev@localhost:5432/whatsapp_clickers`), then deleted. It found the 00014-edit defect above on its first statement, and after the fix all 22 checks passed:

- **Migration.** `goose up` applies 00015 to a database that already had 00014 (1 migration applied, index created); a `Down`/`Up` round trip drops and re-creates the index and leaves `answers.points_awarded` intact, confirming the two migrations are independently reversible.
- **`ListAnswersForScoring` + `AwardPoints` + `UpdateAnswerPointsBatch`.** Two-question game, four players, config 100/50/30/10. Answers were recorded **out of receipt order on purpose** — Carol's row was INSERTed first (lowest `seq`) carrying the *latest* `received_at` — and the persisted `points_awarded`, read straight out of the table, were Alice 150, Bob 130, Carol 110, Dave 0. Receipt time beat insert order against a real `BIGINT GENERATED ALWAYS AS IDENTITY`, which is AC-1's primary key verified end-to-end rather than in a stub.
- **The new `IS NULL` guard.** Re-running `UpdateAnswerPointsBatch` directly against an already-scored answer with `points = 9999` left it at 150 — the write is a genuine no-op, not a silent rewrite.
- **`GetLeaderboard`.** Cumulative across both questions with a real cross-question tie: Bob 260, Carol 260, Alice 150, Dave 0 → ranks **1, 1, 3, 4**. AC-2's shared rank *and* skip-ahead, from SQL rather than from `RankLeaderboard` in isolation. A late joiner (spectator, per 2.5) was absent from all four rows, confirming the `role = 'player'` filter.
- Scratch organizer/game deleted afterward (cascades); verified zero leftover rows and the harness removed.

**Dismissed as noise (7):** repeat submitter hogging all three bonus slots (blocked by `UNIQUE INDEX idx_answers_question_participant`, `00010_answers.sql:32`) · `int32` overflow in `AwardPoints` (`CHECK (... BETWEEN 0 AND 10000)` on all four config columns caps any award at 20,000, `00004_game_scoring.sql`) · `::int` overflow on `GetLeaderboard`'s `SUM` (needs >107,000 answers for one participant) · `received_at` carrying Meta's one-second granularity or being attacker-influenced (it is `time.Now().UTC()` from the Go webhook handler, `wa/webhook.go:297`) · an ungraded answer being silently scored 0 (blocked by the in-transaction `NOT EXISTS (... stage IS NULL)` guard inside `RevealCurrentQuestion`) · `sprint-status.yaml` missing from the reviewed diff (deliberate review scoping to `server/` + `web/`, not a missing change) · the Debug Log naming three `gofmt -l` files where seven are flagged (the substance holds — whole-file CRLF rewrites, with `scoring.go`/`scoring_test.go` genuinely clean).

## Dev Notes

### Architecture guardrails (violations = rework)

- **Dependency direction unaffected**: `scoring.go` lives in `game` (architecture's own file tree: `game/scoring.go # points, speed bonus, ties, leaderboard (FR-17, FR-18)`), imports only `store` (for its param/row types) and stdlib `sort` — zero I/O, exactly like `grading.GradeMCQ`/`GradeExact`/`Normalize`. `store` continues to own all DB access; `scoring.go` never touches `*pgxpool.Pool`/`pgx` directly.
- **Grade never disclosed before Reveal (FR-15/16) — scoring inherits the same discipline**: `points_awarded` is written only inside the guarded `RevealCurrentQuestionAndAwardPoints` transaction, which cannot run before `question_closed`→`revealed`, and `GetLeaderboard` only ever sums `points_awarded IS NOT NULL` rows — so a Participant's score cannot reflect a not-yet-revealed Question's outcome by construction, without needing a second "was this revealed" check anywhere in the read path.
- **NFR-2 ("never block the game loop") — no new concern here**: unlike 3.6's AI stage, scoring is pure in-process computation (no network I/O, no goroutines) added to an already-synchronous transition (`Reveal`). No new latency source.
- **Persist-before-ack (SM-4) is untouched** — this story adds nothing to `RecordAnswer`'s path at all; every change is downstream of Reveal, which already runs after every answer for the question is persisted and graded.
- **The `answers.seq` tie-break is not new plumbing** — migration 00010 (story 3.3) already added it with a comment naming this exact story ("Monotonic tie-break for Speed Bonus ordering (FR-17, story 3.7)"), and deferred-work.md's 3.3 entry closes the "two clocks" item by naming `answers.received_at` as "the load-bearing one (speed-bonus ordering, story 3.7)". Both fields arrive at this story already correct and already used elsewhere (`ORDER BY`s); this story is their first consumer, not their origin.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/game/engine.go](server/internal/game/engine.go)** — as of this story's baseline (3.6 fully on disk, uncommitted): `Store` interface has `RevealCurrentQuestion(ctx, gameID, organizerID) (gen.Game, error)`; `Reveal` reads `g`, checks `StateQuestionClosed`, checks `CountUngradedAnswersForCurrentQuestion`, calls `store.RevealCurrentQuestion`, reinterprets a lost race to `ErrNotQuestionClosed`. This story replaces only the final call + its error-reinterpretation comment, inserting the scoring read/compute step between the ungraded-check and the write. `buildSnapshot`/`emptySnapshot` gain the `Leaderboard` field; nothing else in either function changes. The `aiGrader`/`runAsync` fields and `EngineOption` machinery (3.6) are untouched — this story adds no new `NewEngine` parameter or option.
- **[server/internal/store/games.go](server/internal/store/games.go)** — `RevealCurrentQuestion` is currently a simple `pgx.ErrNoRows`→`ErrNotFound` wrapper around `s.q.RevealCurrentQuestion`, same shape as every other transition wrapper in the file. This story is the first to make a *transition* wrapper transactional (the transactional pattern itself — `DeleteQuestion`/`ReorderQuestions` — already exists in `store/questions.go`, just not yet applied to a `games` transition).
- **[server/internal/store/queries/games.sql](server/internal/store/queries/games.sql)** — `RevealCurrentQuestion`'s SQL (guarded UPDATE + `NOT EXISTS` ungraded check) is unchanged by this story; only its Go-level wrapper in `games.go` is replaced.
- **[server/migrations/00010_answers.sql](server/migrations/00010_answers.sql)** — its `seq` column comment ("Monotonic tie-break for Speed Bonus ordering (FR-17, story 3.7)... Not read by this story [3.3]") is this story's explicit, named cue that `seq` is ready to use exactly as designed.
- **[server/internal/game/snapshot.go](server/internal/game/snapshot.go)** — `Snapshot`/`CurrentQuestion`/`ParticipantSummary` structs, all their fields, and `CurrentQuestion`'s deliberate omission of `CorrectOption`/`AcceptedAnswers` (leak-prevention note) are unchanged; this story only adds the new `Leaderboard` field.
- **[web/src/lib/types.ts](web/src/lib/types.ts)** — `LobbySnapshot` mirrors `game.Snapshot` field-for-field today (missing only this story's new field, which doesn't exist yet at this baseline); Task 6 keeps that mirror accurate. No other type in the file changes.

### Design decisions worth flagging explicitly

- **Points are persisted per-answer at Reveal time, not recomputed on every snapshot read.** This was a real design choice, not the only option: the alternative (compute the leaderboard on the fly from `is_correct`+`received_at`+`seq`, re-deriving "was this question revealed" from `questions.position` vs. `games.current_question_position`/`state`) breaks once a game finishes, because `FinishGame` resets `current_question_position` to 0 (migration 00009) — at that point there is no way to reconstruct which questions were ever revealed without a second piece of state anyway. Persisting `points_awarded` (NULL = not revealed, a value = revealed-and-scored) sidesteps needing that second piece of state, gives Stories 3.8/3.9 a single column to read for "this participant's points on this specific answer, including any bonus" (3.8's AC needs exactly that), and keeps `GetLeaderboard` a single aggregate query. Do not "simplify" this to an on-the-fly recomputation — it would silently break post-game reads.
- **The transactional Reveal + scoring write is one atomic unit, not two sequential calls.** Computing points (`AwardPoints`, in-process, pure) happens *outside* the transaction — deliberately: it needs no DB access and doing it inside the transaction would hold a connection open for no reason. Only the persistence half (the guarded state UPDATE + the `points_awarded` UPDATEs) is transactional, so a crash or error partway through the write can never leave the game `revealed` with some answers unscored, or vice versa.
- **`GetLeaderboard` scopes to `role = 'player'` explicitly**, unlike `ListParticipants`/`ParticipantSummary` elsewhere in `snapshot.go`, which remain role-blind (a tracked, separate gap — deferred-work.md's 2.5-review entries). This story does not attempt to close that broader gap; it only makes sure *this new* surface doesn't inherit it, since a Spectator inflating the leaderboard would be a new, more visible instance of the same class of bug.
- **No new control-panel UI, no leaderboard state transition.** The epic's `leaderboard` game state remains unreachable by any control action (a pre-existing fact carried over from Story 3.1's Dev Notes — "this story never implements a control that enters the leaderboard state"); this story doesn't change that. Leaderboard *data* is available on every snapshot regardless of `game.State` (computed fresh every time, same as everything else in `buildSnapshot`), so whichever future surface renders it (a control-panel section, Epic 4's `leaderboard-stage.tsx`) can do so without requiring a dedicated state visit.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, same conventions as every prior story in this epic. `scoring.go`'s tests are pure/zero-I/O, mirroring `grading`'s established style (table-driven where natural). The `Reveal`-path integration test (Task 8) uses the existing `stubStore` pattern (`engine_test.go`) — no real DB in unit tests, consistent with this project's established standard; the SQL itself (new queries + the transactional wrapper) is verified only by the local E2E harness (Task 9), same posture explicitly accepted for 3.4's SQL (deferred-work.md's 3.4-review entry: "no automated coverage... this is the project's established and documented testing standard").

### Project Structure Notes

**New:**
- `server/migrations/00014_answer_points.sql`
- `server/internal/game/scoring.go`
- `server/internal/game/scoring_test.go`

**Modified:**
- `server/internal/store/queries/answers.sql` (+`ListAnswersForScoring`, `UpdateAnswerPoints`, `GetLeaderboard`)
- `server/internal/store/answers.go` (+`AnswerForScoring`, `AnswerPointsParams`, `ParticipantScore` types; +`ListAnswersForScoring`/`GetLeaderboard` wrappers)
- `server/internal/store/games.go` (`RevealCurrentQuestion` wrapper replaced by transactional `RevealCurrentQuestionAndAwardPoints`)
- `server/internal/store/gen/*` (sqlc-regenerated — real diff: new query functions/params/row types, `Answer.PointsAwarded pgtype.Int4`)
- `server/internal/game/engine.go` (`Store` interface swaps `RevealCurrentQuestion` for the three new methods; `Reveal` computes + persists points; `buildSnapshot`/`emptySnapshot` gain `Leaderboard`)
- `server/internal/game/snapshot.go` (`Snapshot` +`Leaderboard []LeaderboardEntry`)
- `server/internal/game/engine_test.go` (`stubStore` field/method renames + additions per Task 7)
- `web/src/lib/types.ts` (+`LeaderboardEntry`, `LobbySnapshot.leaderboard`)

**Untouched:** `server/internal/game/answers.go` (RecordAnswer's grading path is unaffected — scoring is a Reveal-time concern, not an intake-time one) · `server/internal/httpapi/*` (Reveal's external signature is unchanged, so `control.go`/`control_test.go`'s `ControlEngine`/`stubControlEngine` need no change) · `server/internal/ws/*` · `server/internal/wa/*` (no message changes — 3.8/3.9 send results, this story only makes the data available) · `web/src/features/live/control-page.tsx` (no AC calls for control-panel rendering; see Dev Notes) · `server/internal/config/config.go`/`cmd/server/main.go` (no new config — scoring configuration already lives on `games`, wired since story 1.4).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.7] — story + all 3 epic ACs verbatim, Epic 3 context, FR-17 scope
- [Source: _bmad-output/planning-artifacts/architecture.md] — "scoring with Speed Bonuses ordered by server receipt timestamp; live leaderboard computation" (Epic 3 summary); `game/scoring.go # points, speed bonus, ties, leaderboard (FR-17, FR-18)` (file tree); "§4.6 Scoring & Leaderboard (FR-17–18) | `game/scoring.go` | `leaderboard-stage`, `winner-stage`" (traceability table); "FR-17 Scoring | `game/scoring.go`; receipt-timestamp order, monotonic-seq ties, shared ranks" (requirements table)
- [Source: server/internal/game/engine.go] — current `Store` interface, `Reveal`, `buildSnapshot`/`emptySnapshot`, `snapshotAfterCommit` this story extends
- [Source: server/internal/game/snapshot.go] — current `Snapshot`/`CurrentQuestion`/`ParticipantSummary` shape this story extends; the leak-prevention rationale for `CurrentQuestion` omitting correct-answer fields (unaffected, cited for context only)
- [Source: server/internal/store/games.go, queries/games.sql] — current `RevealCurrentQuestion` wrapper/query this story replaces/reuses
- [Source: server/internal/store/questions.go] — `DeleteQuestion`/`ReorderQuestions`' transaction pattern (`s.pool.Begin`/`s.q.WithTx`/`defer tx.Rollback`/`tx.Commit`) this story's new transactional method follows
- [Source: server/internal/store/answers.go, queries/answers.sql] — current `RecordAnswerParams`/`RecordAnswer`/`UpdateAnswerGrade`/`CountUngradedAnswersForCurrentQuestion` this story sits alongside; `UpdateAnswerGrade`'s "no write-time guard to re-check" reasoning this story's `UpdateAnswerPoints` mirrors
- [Source: server/migrations/00010_answers.sql] — `answers.seq`'s comment explicitly naming this story as its consumer
- [Source: server/migrations/00009_game_question_progress.sql] — `current_question_position`/`answer_cutoff_at`, and (via `store/queries/games.sql`'s `FinishGame` comment) the "resets to 0 on finish" invariant that motivates persisting `points_awarded` instead of recomputing "was this revealed" post-game
- [Source: server/migrations/00004_game_scoring.sql] — `points_per_correct`/`speed_bonus_first/second/third` columns/defaults/CHECKs (story 1.4) this story's `ScoringConfig` consumes unchanged
- [Source: server/internal/store/gen/models.go] — current `Game`/`Answer`/`Participant` struct shapes (scoring fields, `Seq pgtype.Int8`) this story reads
- [Source: server/internal/store/queries/participants.sql] — `ListParticipants`' `ORDER BY joined_at ASC, id ASC` this story's `GetLeaderboard` mirrors for `RankLeaderboard`'s tie-break input order
- [Source: server/internal/httpapi/control.go, control_test.go] — confirms `Reveal`'s external signature/error mapping is unchanged, so no `httpapi` changes are needed
- [Source: web/src/lib/types.ts] — current `LobbySnapshot`/`CurrentQuestion` mirror this story extends with `LeaderboardEntry`
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#Deferred-from-code-review-of-2-5-late-join-becomes-spectator] — the role-blind-roster gap this story's `GetLeaderboard` deliberately does not inherit (scoped to `role='player'` instead)
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#Deferred-from-code-review-of-3-4-grading-pipeline-mcq-and-exact-match] — "no automated SQL coverage... established testing standard" precedent this story's Task 9 (E2E-only SQL verification) follows
- [Source: _bmad-output/implementation-artifacts/3-6-ai-semantic-validation-fail-closed.md] — previous story's Dev Notes/task structure and E2E harness conventions this story follows

## Dev Agent Record

### Agent Model Used

Claude Sonnet 5 (claude-sonnet-5)

### Debug Log References

- Re-verified the story's "prerequisite" section before writing code: story 3.6 is now fully committed (`7a4e55e`, branch `story/3-6-ai-semantic-validation-fail-closed`) but not yet merged to `main` (`main` is still at `827b835`). Branched `story/3-7-scoring-with-speed-bonuses` off `7a4e55e` rather than off `main` — `engine.go`, `store/answers.go`, `store/games.go`, `store/queries/answers.sql`, `snapshot.go`, and `web/src/lib/types.ts` were all read and confirmed to match the shapes the story quotes verbatim before any edit.
- `sqlc generate` produced the expected diff: `answers.sql.go` gained `ListAnswersForScoring`/`UpdateAnswerPoints`/`GetLeaderboard`, `models.go` gained `Answer.PointsAwarded pgtype.Int4`; `games.sql.go` unaffected (only the hand-written wrapper in `games.go` changed, not its underlying generated query).
- `gofmt -l` flags `engine.go`, `engine_test.go`, and `store/answers.go` — confirmed via `git stash`/`gofmt -l` on the pre-story tree that this is the pre-existing repo-wide CRLF-checkout artifact (documented in 3.4-3.6's Dev Notes/Debug Log), not a regression introduced here. `scoring.go`/`scoring_test.go` (new files) are clean.
- Local Go E2E (`server/cmd/e2escratch`, deleted after use) ran against the real local Postgres (`postgres://postgres:dev@localhost:5432/whatsapp_clickers`): drove `game.Engine` directly (`OpenLobby`/`Join`/`StartGame`/`RecordAnswer`/`CloseQuestion`/`Reveal`/`NextQuestion`) per the story's own explicit allowance ("drive `game.Engine.RecordAnswer` in-process with explicit `receivedAt` values ... is fine"), with a `fakeAIGrader` (always `false, nil`) plus a synchronous `WithAsyncRunner` standing in for the AI stage so Dave's Q1 double-miss free_text answer graded synchronously with no polling. Confirmed migration 00014 applies cleanly to the real DB, `points_awarded` persists correctly for a two-question game (`SELECT` directly against `answers`), and the snapshot `Leaderboard` matches `RankLeaderboard`'s output including a genuine cross-question cumulative tie (Bob 130 after Q1+incorrect-Q2, Dave 0 after Q1+correct-Q2, both landing on 130) with the rank-skip (`1,2,2,4`) AC-2 requires. One correction mid-run: the DB's default `speed_bonus_third` is 20 (migration 00004), not 10 as this story's Task 2/8 code comments/tests assume for illustration — the harness's expected values were corrected to match the real default; this does not affect `scoring.go`'s logic, which reads the value from `games` at runtime regardless of what the default is.
- `sqlc generate`/`gofmt`/`go vet`/`go test ./...`/`npx tsc -b`/`npm run lint` all clean; full regression suite green throughout (zero pre-existing test needed modification beyond the `Store`-interface-driven renames in Task 7).

### Completion Notes List

- All 3 ACs implemented and verified: AC-1 (points + top-3 Speed Bonuses by `received_at`, `seq` tie-break, fewer-than-three-correct → fewer bonuses) via `game.AwardPoints` + the E2E's direct-DB check; AC-2 (shared ranks + skip-ahead, leaderboard on every snapshot) via `game.RankLeaderboard` + the E2E's cross-question tie; AC-3 (unit test coverage for bonus ordering, timestamp ties, fewer-than-three-correct) via `scoring_test.go`'s 6 tests.
- Migration `00014_answer_points.sql` adds `answers.points_awarded` (nullable — NULL means "not yet revealed", matching the `is_correct`/`stage` NULL convention from 00011).
- `game/scoring.go` is new: `AwardPoints` (pure, zero I/O) and `RankLeaderboard` (pure, zero I/O), written exactly per the story's Task 2 code, mirroring `grading`'s pure-function style.
- `store` package gained `ListAnswersForScoring`/`GetLeaderboard` read wrappers, an `UpdateAnswerPoints` write, and replaced the old simple `RevealCurrentQuestion` wrapper with a transactional `RevealCurrentQuestionAndAwardPoints` (state transition + all `points_awarded` writes atomic, same `pool.Begin`/`WithTx`/`defer Rollback`/`Commit` pattern as `DeleteQuestion`/`ReorderQuestions`).
- `game.Engine`'s `Store` interface swapped `RevealCurrentQuestion` for `ListAnswersForScoring`/`RevealCurrentQuestionAndAwardPoints`/`GetLeaderboard`; `Reveal` now computes points between the ungraded-check and the write; `buildSnapshot`/`emptySnapshot` both gained `Leaderboard` (non-nil empty slice on the degraded path, matching the existing `Participants` wire-shape discipline).
- `game.Snapshot` gained `Leaderboard []LeaderboardEntry`; `web/src/lib/types.ts` mirrors it with a new `LeaderboardEntry` interface and `LobbySnapshot.leaderboard`. No `control-page.tsx` rendering added — out of scope per the story's own Dev Notes (Epic 4's job).
- Task 7: `engine_test.go`'s `stubStore` renamed/extended for the `Store` interface change; all 5 pre-existing Reveal-path assertions updated to the new method name; zero other test file needed changes (`answers_test.go`/`participants_test.go` compile and pass unchanged, as the story predicted).
- Task 8: 6 new pure unit tests in `scoring_test.go` (top-three-bonus ordering, seq tie-break, fewer-than-three-correct, zero-correct, shared-rank-skip-ahead, tie-order-preserved) plus 2 new tests in `engine_test.go` (`TestRevealComputesAndPersistsSpeedBonusPoints` proving the Reveal→AwardPoints wiring, `TestBuildSnapshotIncludesRankedLeaderboard` proving the leaderboard flows into every snapshot, plus the existing `TestOpenLobbyFromDraftReturnsLobbySnapshot` gained an assertion that a zero-value `getLeaderboardResult` yields `[]LeaderboardEntry{}`, not nil).
- No `httpapi`/`ws`/`wa` changes — `Reveal`'s external signature is unchanged, confirmed by reading `control.go`/`control_test.go` before starting (per the story's own "Untouched" list).

### File List

**New:**
- `server/migrations/00014_answer_points.sql`
- `server/migrations/00015_answers_participant_index.sql` (added at code review)
- `server/internal/game/scoring.go`
- `server/internal/game/scoring_test.go`

**Modified:**
- `server/internal/store/queries/answers.sql`
- `server/internal/store/answers.go`
- `server/internal/store/games.go`
- `server/internal/store/gen/answers.sql.go` (sqlc-regenerated)
- `server/internal/store/gen/models.go` (sqlc-regenerated)
- `server/internal/game/engine.go`
- `server/internal/game/snapshot.go`
- `server/internal/game/engine_test.go`
- `web/src/lib/types.ts`
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (status tracking)

## Change Log

- 2026-08-06: Story 3.7 implemented — Speed Bonus scoring + ranked leaderboard (migration 00014, `game/scoring.go`, transactional `RevealCurrentQuestionAndAwardPoints`, `Snapshot.Leaderboard`, `web/src/lib/types.ts` mirror). All tasks complete, full regression suite green, local Go E2E verified against real Postgres. Status → review.
- 2026-08-06: Code review (3 layers) — 1 decision resolved, 13 patches applied, 4 items deferred, 7 dismissed. Migration 00015 adds `idx_answers_participant`; `UpdateAnswerPointsBatch` replaces the per-answer UPDATE loop and gains a `points_awarded IS NULL` finality guard; `ListAnswersForScoring` resolves one question via `created_at`; `Reveal`'s three-cause error comment restored. Five test mutations that previously passed now fail. A second throwaway E2E against live Postgres (22 checks, 0 failures) verified the new SQL and caught an in-place edit to the already-applied 00014, which is why the index moved to its own migration. Status → done.
