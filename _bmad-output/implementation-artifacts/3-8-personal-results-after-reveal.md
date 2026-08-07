---
baseline_commit: 7a4e55e016cd38ac6151234528feeb3a689e3643
---

# Story 3.8: Personal Results After Reveal

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a Participant,
I want my grade, points, and rank in my WhatsApp right after the Reveal,
so that I feel the game beat-by-beat (FR-6).

## ⚠️ Prerequisite: verify 3.7 is actually on disk before writing code

At the time this story was created, **story 3.7 (Scoring with Speed Bonuses) is fully implemented on disk but uncommitted** — sprint-status.yaml and its own story doc both show `review`, `git status` shows the diff still sitting in the working tree on branch `story/3-7-scoring-with-speed-bonuses`, and nothing has been pushed. This story's baseline_commit (`7a4e55e`, story 3.6's commit) is the last **committed** ancestor; 3.7 is layered on top of it, uncommitted. **This story depends entirely on 3.7's output** — branch from wherever 3.7's actual work sits (the `story/3-7-scoring-with-speed-bonuses` branch tip, or `main` if it has merged by the time you start), never from `main` alone if 3.7 hasn't landed there yet.

Verify these 3.7 deliverables exist on disk exactly as described before writing any code:
- `server/internal/game/engine.go` — `Store` interface includes `ListAnswersForScoring`, `RevealCurrentQuestionAndAwardPoints`, `GetLeaderboard`; `buildSnapshot`/`emptySnapshot` populate `Leaderboard`.
- `server/internal/game/scoring.go` — `AwardPoints`, `RankLeaderboard`, `LeaderboardEntry{ParticipantID, DisplayName, Score, Rank}`.
- `server/internal/game/snapshot.go` — `Snapshot.Leaderboard []LeaderboardEntry`.
- `server/internal/store/answers.go` — `AnswerForScoring`, `AnswerPointsParams`, `ParticipantScore` types; `ListAnswersForScoring`/`GetLeaderboard` wrappers.
- `server/migrations/00014_answer_points.sql` — `answers.points_awarded INTEGER` (NULL until revealed; non-NULL — including 0 — once revealed).

**Re-verify these shapes still match before writing code** — if 3.7 has since been committed, merged, or further changed, confirm the quoted signatures below are still accurate.

## Acceptance Criteria

1. **Given** the `AnswerRevealed` event, **when** result messages dispatch, **then** each Participant who answered receives their grade, points earned including any Speed Bonus, and current rank — copy per the templates table result rows: "נכון! 🎉 +[ניקוד] נקודות / ⚡ בונוס מהירות +[בונוס] / מקום [דירוג] בטבלה" (two emoji allowed on the bonus message, A20), wrong-answer variants included — the last-question form drops "עוד הכול פתוח" (A5) (UJ-2). *(epic AC-1)*
2. **Given** a Question not yet revealed, **then** no grade is ever sent (FR-6: no leakage into the room). *(epic AC-2)*
3. **Given** a Participant who did not answer, **then** they receive no per-question message (silence by design). *(epic AC-3)*

**No new migration this story.** Every input already exists on disk courtesy of story 3.7: `answers.points_awarded`/`is_correct` (NULL until revealed, then final), `games.points_per_correct`/`speed_bonus_first/second/third`, and `game.RankLeaderboard`'s post-reveal ranking.

## Tasks / Subtasks

- [x] **Task 1: `store` package — one new query + wrapper** (AC: 1, 2, 3)
  - [x] New query in `server/internal/store/queries/answers.sql`:
    ```sql
    -- Per-answer results for gameID's just-revealed question (story 3.8) —
    -- the WhatsApp personal-result dispatch's data source (FR-6). Read only
    -- after Reveal's transaction has committed
    -- (game.Engine.ResultsForRevealedQuestion, called from
    -- httpapi/control.go's post-response goroutine, mirroring
    -- dispatchQuestionOpened's placement, story 3.2): is_correct is
    -- guaranteed non-NULL by Reveal's own pre-check
    -- (CountUngradedAnswersForCurrentQuestion == 0 before the transaction
    -- runs), and points_awarded is guaranteed non-NULL because
    -- RevealCurrentQuestionAndAwardPoints (story 3.7) writes it inside the
    -- same transaction that flips the game to revealed (migration 00014's
    -- NULL convention) — so both are read via `.Bool`/`.Int32` without a
    -- `.Valid` check downstream, same posture as ListAnswersForScoring's
    -- is_correct.
    -- name: ListAnswerResultsForQuestion :many
    SELECT a.participant_id, p.phone, a.is_correct, a.points_awarded
    FROM answers a
    JOIN questions q ON q.id = a.question_id
    JOIN participants p ON p.id = a.participant_id
    WHERE q.game_id = sqlc.arg(game_id) AND q.position = sqlc.arg(position);
    ```
  - [x] New type + wrapper in `server/internal/store/answers.go` (near `AnswerForScoring`):
    ```go
    // AnswerResultRow is one answering Participant's phone/correctness/points
    // for a just-revealed question — game.Engine.ResultsForRevealedQuestion's
    // raw input (story 3.8, FR-6).
    type AnswerResultRow struct {
        ParticipantID string
        Phone         string
        IsCorrect     bool
        Points        int32
    }

    // ListAnswerResultsForQuestion returns gameID's question-at-position
    // answers, one row per answering Participant, in no particular order —
    // game.Engine.ResultsForRevealedQuestion does its own leaderboard-based
    // rank lookup per participant.
    func (s *Store) ListAnswerResultsForQuestion(ctx context.Context, gameID string, position int32) ([]AnswerResultRow, error) {
        rows, err := s.q.ListAnswerResultsForQuestion(ctx, gen.ListAnswerResultsForQuestionParams{GameID: gameID, Position: position})
        if err != nil {
            return nil, err
        }
        out := make([]AnswerResultRow, 0, len(rows))
        for _, r := range rows {
            out = append(out, AnswerResultRow{ParticipantID: r.ParticipantID, Phone: r.Phone, IsCorrect: r.IsCorrect.Bool, Points: r.PointsAwarded.Int32})
        }
        return out, nil
    }
    ```
  - [x] `sqlc generate` — expect a diff limited to `answers.sql.go` gaining `ListAnswerResultsForQuestion` + its param/row types. No `models.go` change (no new columns).

- [x] **Task 2: `game/results.go` (new file) — resolve one revealed question's dispatch data, zero-I/O composition inline** (AC: 1, 2, 3)
  - [x] New file `server/internal/game/results.go`:
    ```go
    package game

    import (
        "context"
        "fmt"

        "github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
    )

    // PersonalResult is one answering Participant's resolved inputs to the
    // Result message templates (wa/messages_he.go's four Result rows) — the
    // WhatsApp personal-result dispatch's data source (FR-6, story 3.8).
    // BasePoints is the Game's configured PointsPerCorrect when IsCorrect (0
    // otherwise); BonusPoints is the Speed Bonus component only (0 when no
    // bonus applied). The templates show these as two separate numbers,
    // never their sum (EXPERIENCE.md's "Result — correct + bonus" row) —
    // that is why this struct splits them instead of carrying one Points
    // total.
    type PersonalResult struct {
        Phone       string
        IsCorrect   bool
        BasePoints  int32
        BonusPoints int32
        Rank        int
    }

    // RevealedQuestionResults is ResultsForRevealedQuestion's return value.
    type RevealedQuestionResults struct {
        // Results holds one entry per Participant who answered the revealed
        // question. Non-answerers are absent by construction —
        // ListAnswerResultsForQuestion only returns rows with a recorded
        // answer — which IS epic AC-3's "no per-question message" silence
        // rule; the caller never filters anything out.
        Results []PersonalResult
        // CorrectAnswer is the human-readable correct answer shown in a
        // wrong-answer message: the correct MCQ option's text, or the
        // Free-Text question's primary (first) Accepted Answer
        // (EXPERIENCE.md A15 — the same "primary form" already used at
        // Reveal on the Audience Display stage, Epic 4).
        CorrectAnswer string
        // IsLastQuestion is true when the revealed question was the game's
        // final one — the wrong-answer message drops "עוד הכול פתוח" in
        // that case (EXPERIENCE.md A5).
        IsLastQuestion bool
    }

    // ResultsForRevealedQuestion resolves the WhatsApp personal-result
    // dispatch's full data source (FR-6) for gameID's question at position.
    //
    // position is the caller's own already-committed Reveal snapshot's
    // CurrentQuestion.Position — NOT re-derived from a fresh
    // GetGameForOrganizer read of g.CurrentQuestionPosition. This matters:
    // this method runs from httpapi/control.go's own goroutine, spawned
    // AFTER the REST response is written, so an organizer could in
    // principle click "next question" before this goroutine runs, which
    // would advance g.CurrentQuestionPosition to a DIFFERENT question and —
    // if this method re-read it fresh — silently compute results for the
    // wrong question. Binding on the specific position the caller already
    // knows was just revealed closes that race entirely, the same
    // discipline RecordAnswer's own SQL comment describes ("binding on the
    // specific question_id... closes a narrower race" — queries/answers.sql).
    // g.PointsPerCorrect is safe to re-read fresh regardless: it is a
    // game-level config, not per-question, and nothing in this codebase
    // edits it mid-game.
    //
    // Called only after Reveal has committed — from httpapi/control.go's
    // own goroutine, mirroring dispatchQuestionOpened's placement (story
    // 3.2) exactly. organizerID scopes the question lookup; like
    // PlayerRecipients, this trusts the caller's own guard
    // (snapshot.State == StateRevealed) rather than re-checking state here.
    func (e *Engine) ResultsForRevealedQuestion(ctx context.Context, gameID, organizerID string, position int32) (RevealedQuestionResults, error) {
        g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
        if err != nil {
            return RevealedQuestionResults{}, err
        }
        questions, err := e.store.ListQuestionsByGame(ctx, gameID, organizerID)
        if err != nil {
            return RevealedQuestionResults{}, err
        }

        var current gen.Question
        found := false
        for _, q := range questions {
            if q.Position == position {
                current = q
                found = true
                break
            }
        }
        if !found {
            // Positions are a dense 1..N sequence (story 3.1 / DeleteQuestion's
            // gap-closing) and the caller only ever passes a position it just
            // saw revealed — unreachable in practice; fail loudly rather than
            // dispatch with a blank correct answer.
            return RevealedQuestionResults{}, fmt.Errorf("game: no question at position %d for game %s", position, gameID)
        }

        var correctAnswer string
        switch current.Type {
        case "mcq":
            if current.CorrectOption < 1 || int(current.CorrectOption) > len(current.Options) {
                return RevealedQuestionResults{}, fmt.Errorf("game: question %s has out-of-range correct_option %d", current.ID, current.CorrectOption)
            }
            correctAnswer = current.Options[current.CorrectOption-1]
        case "free_text":
            if len(current.AcceptedAnswers) == 0 {
                return RevealedQuestionResults{}, fmt.Errorf("game: free_text question %s has no accepted answers", current.ID)
            }
            correctAnswer = current.AcceptedAnswers[0]
        default:
            // questions_type_shape's CHECK constraint guarantees this never
            // happens for a real row (same invariant RecordAnswer already
            // trusts) — fail closed with an error rather than guess.
            return RevealedQuestionResults{}, fmt.Errorf("game: question %s has unrecognized type %q", current.ID, current.Type)
        }
        isLastQuestion := int(current.Position) == len(questions)

        scores, err := e.store.GetLeaderboard(ctx, gameID)
        if err != nil {
            return RevealedQuestionResults{}, err
        }
        rankByParticipant := make(map[string]int, len(scores))
        for _, entry := range RankLeaderboard(scores) {
            rankByParticipant[entry.ParticipantID] = entry.Rank
        }

        rows, err := e.store.ListAnswerResultsForQuestion(ctx, gameID, position)
        if err != nil {
            return RevealedQuestionResults{}, err
        }
        results := make([]PersonalResult, 0, len(rows))
        for _, r := range rows {
            var base, bonus int32
            if r.IsCorrect {
                base = g.PointsPerCorrect
                if r.Points > base {
                    bonus = r.Points - base
                }
            }
            results = append(results, PersonalResult{
                Phone:       r.Phone,
                IsCorrect:   r.IsCorrect,
                BasePoints:  base,
                BonusPoints: bonus,
                Rank:        rankByParticipant[r.ParticipantID],
            })
        }

        return RevealedQuestionResults{Results: results, CorrectAnswer: correctAnswer, IsLastQuestion: isLastQuestion}, nil
    }
    ```
  - [x] `engine.go`'s `Store` interface gains one line (near `GetLeaderboard`):
    ```go
    ListAnswerResultsForQuestion(ctx context.Context, gameID string, position int32) ([]store.AnswerResultRow, error)
    ```
    No other change to `engine.go` — `Reveal` itself is untouched by this story; `results.go` is purely additive.

- [x] **Task 3: `wa/messages_he.go` — four new Result templates** (AC: 1)
  - [x] Update the file's header "current map" comment: change the four `-> story 3.8` lines to `-> story 3.8 (below)`, matching every other landed row's convention.
  - [x] Append (after `questionClosedMessage`):
    ```go
    // msgResultCorrectTemplate is the Result — correct row (no Speed Bonus).
    const msgResultCorrectTemplate = "נכון! 🎉 +%s נקודות\nמקום %s בטבלה"

    // resultCorrectMessage returns the finished Result-correct copy. points
    // is the Game's configured points-per-correct-answer (no bonus); rank is
    // the Participant's post-reveal cumulative Leaderboard rank.
    func resultCorrectMessage(points int32, rank int) string {
        return fmt.Sprintf(msgResultCorrectTemplate, ltr(strconv.Itoa(int(points))), ltr(strconv.Itoa(rank)))
    }

    // msgResultCorrectBonusTemplate is the Result — correct + bonus row. Two
    // emoji (🎉 and ⚡) are allowed here — the templates table's stated
    // exception for winner/final-results/speed-bonus messages (EXPERIENCE.md
    // A20); every other row in this file stays at one.
    const msgResultCorrectBonusTemplate = "נכון! 🎉 +%s נקודות\n⚡ בונוס מהירות +%s\nמקום %s בטבלה"

    // resultCorrectBonusMessage returns the finished Result-correct-with-bonus
    // copy. basePoints is points-per-correct-answer; bonusPoints is the
    // Speed Bonus component alone — shown as two separate numbers, never
    // their sum.
    func resultCorrectBonusMessage(basePoints, bonusPoints int32, rank int) string {
        return fmt.Sprintf(msgResultCorrectBonusTemplate, ltr(strconv.Itoa(int(basePoints))), ltr(strconv.Itoa(int(bonusPoints))), ltr(strconv.Itoa(rank)))
    }

    // msgResultWrongTemplate is the Result — wrong row for every question
    // except the game's last (EXPERIENCE.md A5's "עוד הכול פתוח" closing
    // line). correctAnswer is inserted raw, NOT isolated: it is Hebrew
    // content (an Accepted Answer or MCQ option text), the same treatment
    // questionMCQMessage/questionFreeTextMessage give the question text
    // itself — content, not an LTR token like a code or digit.
    const msgResultWrongTemplate = "לא נכון הפעם. התשובה: %s\nמקום %s בטבלה — עוד הכול פתוח!"

    func resultWrongMessage(correctAnswer string, rank int) string {
        return fmt.Sprintf(msgResultWrongTemplate, correctAnswer, ltr(strconv.Itoa(rank)))
    }

    // msgResultWrongLastTemplate is the Result — wrong, last-question row:
    // the same content as msgResultWrongTemplate minus "עוד הכול פתוח"
    // (EXPERIENCE.md A5 — there is no more game left to stay open about).
    const msgResultWrongLastTemplate = "לא נכון הפעם. התשובה: %s\nמקום %s בטבלה"

    func resultWrongLastMessage(correctAnswer string, rank int) string {
        return fmt.Sprintf(msgResultWrongLastTemplate, correctAnswer, ltr(strconv.Itoa(rank)))
    }
    ```

- [x] **Task 4: `wa/result_notifier.go` (new file) — one message per answering Participant** (AC: 1, 2, 3)
  - [x] New file, mirroring `question_notifier.go`'s shape exactly:
    ```go
    package wa

    import (
        "log/slog"
        "strings"

        "github.com/avraham-shor/whatsapp-clickers/internal/game"
    )

    // ResultNotifier turns an AnswerRevealed transition into one outbound
    // WhatsApp personal-result message per answering Participant (FR-6,
    // story 3.8). Wraps a Replier — the same non-blocking Enqueue every
    // other outbound path in this package already uses — so
    // DispatchAnswerRevealed never itself blocks its caller (NFR-2).
    //
    // Non-answerers get no call at all: game.Engine.ResultsForRevealedQuestion
    // only returns an entry per Participant who actually has a recorded
    // answer for the revealed question — silence by design
    // (EXPERIENCE.md's Rejected "scolding non-answerers"; epic AC-3).
    type ResultNotifier struct {
        replier Replier
        logger  *slog.Logger
    }

    // NewResultNotifier builds a ResultNotifier. logger may be nil, in which
    // case slog.Default() is used (matches NewQuestionNotifier/NewDispatcher).
    func NewResultNotifier(replier Replier, logger *slog.Logger) *ResultNotifier {
        if logger == nil {
            logger = slog.Default()
        }
        return &ResultNotifier{replier: replier, logger: logger}
    }

    // DispatchAnswerRevealed composes and enqueues one message per entry in
    // results — correctAnswerText/isLastQuestion select the wrong-answer
    // template variant (EXPERIENCE.md A5). Called from httpapi/control.go's
    // own goroutine, spawned after handleReveal's snapshot has already
    // broadcast and its REST response is written — by the time this runs,
    // the grade is already public knowledge on the wire (epic AC-2 is
    // satisfied upstream, by Reveal's own gating, not by anything in this
    // method).
    func (n *ResultNotifier) DispatchAnswerRevealed(gameID string, results []game.PersonalResult, correctAnswerText string, isLastQuestion bool) {
        dispatched := 0
        for _, r := range results {
            trimmed := strings.TrimSpace(r.Phone)
            if trimmed == "" {
                n.logger.Warn("result dispatch: skipping recipient with a blank phone", "game_id", gameID)
                continue
            }
            var body string
            switch {
            case r.IsCorrect && r.BonusPoints > 0:
                body = resultCorrectBonusMessage(r.BasePoints, r.BonusPoints, r.Rank)
            case r.IsCorrect:
                body = resultCorrectMessage(r.BasePoints, r.Rank)
            case isLastQuestion:
                body = resultWrongLastMessage(correctAnswerText, r.Rank)
            default:
                body = resultWrongMessage(correctAnswerText, r.Rank)
            }
            n.replier.Enqueue(trimmed, body)
            dispatched++
        }
        n.logger.Info("result dispatch enqueued", "game_id", gameID, "recipient_count", dispatched)
    }
    ```

- [x] **Task 5: `httpapi/control.go` — dispatch on Reveal** (AC: 1, 2, 3)
  - [x] New interface (near `QuestionDispatcher`):
    ```go
    // ResultDispatcher is the WhatsApp fan-out surface handleReveal needs;
    // *wa.ResultNotifier satisfies it. Consumer-defined here, referencing
    // only game types, so httpapi never imports wa — same posture as
    // QuestionDispatcher.
    type ResultDispatcher interface {
        DispatchAnswerRevealed(gameID string, results []game.PersonalResult, correctAnswerText string, isLastQuestion bool)
    }
    ```
  - [x] `ControlEngine` interface gains one line (near `PlayerRecipients`):
    ```go
    ResultsForRevealedQuestion(ctx context.Context, gameID, organizerID string, position int32) (game.RevealedQuestionResults, error)
    ```
  - [x] New function, mirroring `dispatchQuestionOpened` exactly in shape/timing:
    ```go
    // dispatchAnswerRevealed hands the WhatsApp personal-result burst to
    // dispatcher when snapshot reflects a freshly revealed question. Spawned
    // as a goroutine after the HTTP response is written (see handleReveal)
    // — ResultsForRevealedQuestion is a real DB round trip (question lookup
    // + leaderboard + per-answer read), and gating the organizer-facing
    // response on it would mean an organizer's "reveal" click could
    // visibly stall for up to this function's own timeout (same reasoning
    // as dispatchQuestionOpened, story 3.2).
    //
    // Guards on snapshot.State == game.StateRevealed and a non-nil
    // CurrentQuestion — Reveal never clears CurrentQuestion (it stays
    // populated at the just-revealed position; NextQuestion is the only
    // thing that later moves it), so together these are the correct,
    // unambiguous "a question was just revealed" signal.
    //
    // Detached from ctx's cancellation with its own bounded timeout — same
    // reasoning as dispatchQuestionOpened/engine.snapshotAfterCommit: the
    // transition already committed and its WS snapshot already broadcast
    // by the time this runs, so the organizer's connection closing must
    // not skip delivering results to everyone who answered.
    func dispatchAnswerRevealed(ctx context.Context, engine ControlEngine, dispatcher ResultDispatcher, gameID, organizerID string, snapshot game.Snapshot) {
        if dispatcher == nil || snapshot.State != game.StateRevealed {
            return
        }
        if snapshot.CurrentQuestion == nil {
            slog.Error("result dispatch skipped, snapshot missing current question for a revealed state", "game_id", gameID)
            return
        }
        dispatchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
        defer cancel()
        results, err := engine.ResultsForRevealedQuestion(dispatchCtx, gameID, organizerID, int32(snapshot.CurrentQuestion.Position))
        if err != nil {
            slog.Error("result dispatch skipped, could not resolve revealed question results", "game_id", gameID, "error", err)
            return
        }
        dispatcher.DispatchAnswerRevealed(gameID, results.Results, results.CorrectAnswer, results.IsLastQuestion)
    }
    ```
  - [x] `handleReveal` gains a third parameter and spawns the dispatch goroutine, same shape as `handleStartGame`:
    ```go
    func handleReveal(engine ControlEngine, hub SnapshotBroadcaster, dispatcher ResultDispatcher) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            organizerID, ok := requireOrganizer(w, r)
            if !ok {
                return
            }
            gameID, ok := gameIDParam(w, r)
            if !ok {
                return
            }
            ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
            defer cancel()
            snapshot, err := engine.Reveal(ctx, gameID, organizerID)
            if err != nil {
                writeStoreError(w, err, "GAME_NOT_FOUND")
                return
            }
            hub.Broadcast(gameID, snapshot)
            slog.Info("question revealed", "game_id", gameID, "organizer_id", organizerID)
            writeJSON(w, http.StatusOK, snapshot)
            go dispatchAnswerRevealed(ctx, engine, dispatcher, gameID, organizerID, snapshot)
        }
    }
    ```

- [x] **Task 6: wire `resultDispatcher` through `router.go` and `main.go`** (AC: 1, 2, 3)
  - [x] `router.go`'s `NewRouter` gains a 10th, trailing parameter (matches the precedent story 3.2 set adding `dispatcher QuestionDispatcher` as the 9th):
    ```go
    func NewRouter(db Pinger, authSvc AuthService, games GameStore, static fs.FS, webhook http.Handler, engine ControlEngine, hub SnapshotBroadcaster, wsHandler http.Handler, dispatcher QuestionDispatcher, resultDispatcher ResultDispatcher) http.Handler {
    ```
    Update the doc comment's dispatcher sentence to add: "resultDispatcher is independently nilable the same way — it only affects whether /reveal also dispatches WhatsApp personal-result messages, never whether any route exists." Update the `/reveal` mount line inside the existing `if engine != nil && hub != nil` block:
    ```go
    gr.Post("/reveal", handleReveal(engine, hub, resultDispatcher))
    ```
  - [x] `main.go`: add right after `questionNotifier := wa.NewQuestionNotifier(dispatcher, logger)`:
    ```go
    resultNotifier := wa.NewResultNotifier(dispatcher, logger)
    ```
    Update the `NewRouter` call:
    ```go
    router := httpapi.NewRouter(st, authSvc, st, webdist.FS(), webhookHandler, engine, hub, wsHandler, questionNotifier, resultNotifier)
    ```
    Update the nearby "st satisfies game.Store (...)" comment to add `ListAnswerResultsForQuestion` to its parenthetical list.

- [x] **Task 7: update every existing call site for the two signature changes** (AC: 1, 2, 3)
  - [x] `server/internal/game/engine_test.go`'s `stubStore` (required just to keep satisfying the widened `Store` interface — every existing test that reaches `buildSnapshot` keeps compiling and passing unchanged, same low-blast-radius shape as 3.7's own `Store`-interface addition): add
    ```go
    listAnswerResultsForQuestionResult []store.AnswerResultRow
    listAnswerResultsForQuestionErr    error
    ```
    and
    ```go
    func (s *stubStore) ListAnswerResultsForQuestion(ctx context.Context, gameID string, position int32) ([]store.AnswerResultRow, error) {
        return s.listAnswerResultsForQuestionResult, s.listAnswerResultsForQuestionErr
    }
    ```
  - [x] `server/internal/httpapi/control_test.go`'s `stubControlEngine`: add `ResultsForRevealedQuestion` (required to keep satisfying the widened `ControlEngine` interface), following the exact `playerRecipients*` mutex-guarded pattern (this method also now runs from a goroutine spawned post-response):
    ```go
    resultsForRevealedQuestionMu           sync.Mutex
    resultsForRevealedQuestionResult       game.RevealedQuestionResults
    resultsForRevealedQuestionErr          error
    resultsForRevealedQuestionRequestedFor [][2]string // {gameID, organizerID}
    resultsForRevealedQuestionPosition     int32
    resultsForRevealedQuestionDone         chan struct{}
    ```
    ```go
    func (s *stubControlEngine) ResultsForRevealedQuestion(ctx context.Context, gameID, organizerID string, position int32) (game.RevealedQuestionResults, error) {
        s.resultsForRevealedQuestionMu.Lock()
        s.resultsForRevealedQuestionRequestedFor = append(s.resultsForRevealedQuestionRequestedFor, [2]string{gameID, organizerID})
        s.resultsForRevealedQuestionPosition = position
        result, err := s.resultsForRevealedQuestionResult, s.resultsForRevealedQuestionErr
        s.resultsForRevealedQuestionMu.Unlock()
        if s.resultsForRevealedQuestionDone != nil {
            s.resultsForRevealedQuestionDone <- struct{}{}
        }
        return result, err
    }
    ```
  - [x] New `stubResultDispatcher` in `control_test.go` (or `router_test.go`, alongside `stubQuestionDispatcher` — either file is fine, match wherever `stubQuestionDispatcher` already lives), same shape:
    ```go
    type stubResultDispatcher struct {
        mu    sync.Mutex
        calls []resultDispatchCall
        done  chan struct{}
    }

    type resultDispatchCall struct {
        gameID            string
        results           []game.PersonalResult
        correctAnswerText string
        isLastQuestion    bool
    }

    func (s *stubResultDispatcher) DispatchAnswerRevealed(gameID string, results []game.PersonalResult, correctAnswerText string, isLastQuestion bool) {
        s.mu.Lock()
        s.calls = append(s.calls, resultDispatchCall{gameID, results, correctAnswerText, isLastQuestion})
        s.mu.Unlock()
        if s.done != nil {
            s.done <- struct{}{}
        }
    }

    func (s *stubResultDispatcher) Calls() []resultDispatchCall {
        s.mu.Lock()
        defer s.mu.Unlock()
        return append([]resultDispatchCall(nil), s.calls...)
    }
    ```
  - [x] **Every existing `NewRouter(...)` call gains a 10th trailing argument.** This is purely mechanical — every 9-argument call becomes 10-argument by appending `nil` (or the story's new stub, only where a test specifically exercises result dispatch). Current call sites (each already passes `nil` or `dispatcher` as its 9th argument today):
    - `server/cmd/server/main.go:209` — becomes `..., questionNotifier, resultNotifier)` (Task 6, not a mechanical `nil`).
    - `server/internal/httpapi/control_test.go` lines 175, 182, 253, 429 — `controlRouter`/`controlRouterWithDispatcher` helpers plus two inline calls. Update `controlRouter` to accept and pass through a `resultDispatcher ResultDispatcher` param (or add a sibling `controlRouterWithResultDispatcher(engine, hub, resultDispatcher)` mirroring `controlRouterWithDispatcher`, whichever keeps the existing helpers' call sites smallest — this file's own tests decide which is less churn); the two inline `NewRouter(...)` calls (lines 253, 429, both inside "without session" tests) just gain a trailing `nil`.
    - `server/internal/httpapi/games_test.go` lines 142, 331 — trailing `nil`.
    - `server/internal/httpapi/packages_test.go` line 81 — trailing `nil`.
    - `server/internal/httpapi/router_test.go` — every call in this file (roughly two dozen, all `nil, nil, nil, nil` today) — trailing `nil`.
    No other production call site exists — `main.go`'s is the only one.

- [x] **Task 8: new tests** (AC: 1, 2, 3)
  - [x] New `server/internal/game/results_test.go` (co-located `game` package test file — reuses `stubStore`/fixtures already defined in `engine_test.go`, same package):
    - `TestResultsForRevealedQuestionMCQSplitsBaseAndBonusPointsByRank` — a stub with 4 `ListAnswerResultsForQuestion` rows for one mcq question (`CorrectOption` set to a real value, e.g. 2 — **do not reuse `oneQuestion()`/`twoQuestions()` as-is, their `CorrectOption` defaults to the Go zero value 0, which this method now rejects as out-of-range**): 3 correct rows with `Points` = `PointsPerCorrect+bonus1/2/3`, 1 incorrect row with `Points` = 0; a `GetLeaderboard` result giving each participant a distinct rank. Assert each `PersonalResult`'s `BasePoints`/`BonusPoints`/`Rank` match, and the incorrect row gets `BasePoints=0, BonusPoints=0`.
    - `TestResultsForRevealedQuestionCorrectWithoutBonusHasZeroBonusPoints` — a correct row whose `Points == PointsPerCorrect` exactly (no bonus room left, or a 4th+ correct answer) → `BonusPoints == 0`.
    - `TestResultsForRevealedQuestionMCQWrongUsesCorrectOptionText` — `CorrectAnswer` equals `Options[CorrectOption-1]`.
    - `TestResultsForRevealedQuestionFreeTextWrongUsesPrimaryAcceptedAnswer` — a free_text question stub; `CorrectAnswer` equals `AcceptedAnswers[0]`, not any other accepted value.
    - `TestResultsForRevealedQuestionIsLastQuestionTrueWhenPositionEqualsQuestionCount` and `...FalseOtherwise` — reuse `revealedLastQuestionStub()`/`revealedStub()`'s question-list shape (one question vs. two), calling with the matching `position`.
    - `TestResultsForRevealedQuestionNoAnswersReturnsEmptyResultsSlice` — `listAnswerResultsForQuestionResult` left at its zero value (nil) → `Results` is `[]PersonalResult{}`, not nil, and no panic (epic AC-3's silence case: the caller ranges over an empty slice and enqueues nothing).
    - `TestResultsForRevealedQuestionPropagatesStoreErrors` — table-driven over each of the four store calls erroring (`GetGameForOrganizer`, `ListQuestionsByGame`, `GetLeaderboard`, `ListAnswerResultsForQuestion`) → the error surfaces unwrapped.
    - `TestResultsForRevealedQuestionOutOfRangeCorrectOptionReturnsError` — a stub mcq question with `CorrectOption: 0` (or 5) → a non-nil error, not a panic (guards the exact gotcha the other tests above are warned about).
  - [x] New tests in `server/internal/wa/messages_he_test.go` — extend the canonical-copy `const` block with the four new rows and add `MatchesCanonicalCopy`/`IsolatesDigitTokens` test pairs for each, following the file's own established pattern exactly (see e.g. `TestFormatHintMessageMatchesCanonicalCopy`/`TestFormatHintMessageIsolatesDigitToken`). `resultWrongMessage`/`resultWrongLastMessage`'s `correctAnswer` argument is NOT isolated (per Task 3's comment) — assert this explicitly with a Latin-script correct-answer fixture (mirrors `TestWelcomeMessageIsolatesLTRTokens`'s reasoning for why a pure-Hebrew fixture wouldn't catch a missing/wrongly-present isolate either way), confirming it passes through unisolated.
  - [x] New `server/internal/wa/result_notifier_test.go`, mirroring `question_notifier_test.go`'s structure:
    - `TestDispatchAnswerRevealedCorrectNoBonusEnqueuesCorrectMessage`
    - `TestDispatchAnswerRevealedCorrectWithBonusEnqueuesBonusMessage`
    - `TestDispatchAnswerRevealedWrongMidGameEnqueuesWrongMessageWithContinuationLine`
    - `TestDispatchAnswerRevealedWrongLastQuestionEnqueuesWrongMessageWithoutContinuationLine`
    - `TestDispatchAnswerRevealedSkipsBlankPhoneRecipientAndTrimsWhitespace` (mirrors `TestDispatchQuestionOpenedSkipsBlankPhoneRecipientAndTrimsWhitespace`)
    - `TestDispatchAnswerRevealedEmptyResultsLogsZeroCountAndEnqueuesNothing`
  - [x] New tests in `server/internal/httpapi/control_test.go` (mirrors the existing "Question dispatch (story 3.2)" test block, renamed section "Result dispatch (story 3.8)"):
    - `TestRevealDispatchesResultsToResultDispatcher` — a `revealSnapshot` with `State: "revealed"` and a populated `CurrentQuestion`; assert `stubResultDispatcher.Calls()` has exactly one call with the engine's `resultsForRevealedQuestionResult` fields threaded through, and `ResultsForRevealedQuestion` was called with the snapshot's `CurrentQuestion.Position`.
    - `TestRevealDispatchSkippedOnResultsError` — mirrors `TestStartGameDispatchSkippedOnRecipientsError`: `resultsForRevealedQuestionErr` set → `POST /reveal` still returns 200 (already-committed transition), dispatcher never called.
    - `TestOtherControlActionsNeverDispatchResults` — `CloseQuestion`/`StartGame`/`NextQuestion`/`StopGame` never call `ResultsForRevealedQuestion` or the result dispatcher (true by construction: only `handleReveal` accepts a `ResultDispatcher` at all).
    - Update `TestOtherControlActionsNeverDispatch`'s doc comment: it no longer accurately says "CloseQuestion/Reveal/StopGame don't accept a dispatcher param at all" — Reveal now accepts a *different* dispatcher (`ResultDispatcher`, not `QuestionDispatcher`). Reword to name `QuestionDispatcher` specifically; the test's actual assertions (against `stubQuestionDispatcher`, on the `/close-question` route) still hold unchanged.

- [x] **Task 9: Quality gates + local E2E** (all ACs)
  - [x] Local gates: `gofmt -l .` (CRLF-checkout caveat from 3.4-3.7 still applies — verify only files this story touches) · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` diff (Task 1's new query only). No `web/` changes this story — no `tsc`/`eslint` run needed.
  - [x] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1-3.7 — including the fake-provider `WHATSAPP_API_BASE_URL` override main.go documents, so sent message bodies are actually inspectable): create a scratch game with **two** questions — Q1 mcq (correct option text distinguishable, e.g. option 2 = "ירושלים"), Q2 free_text (`accepted_answers: ["ירושלים", "יְרוּשָׁלַיִם"]`, so `AcceptedAnswers[0]` = "ירושלים" is the primary form asserted below). Open lobby, join **5** players: A, B, C, D, E.
    - `POST /start`. On Q1: A, B, C answer correctly with staggered receipt times (real small delay or explicit `receivedAt` via direct engine calls, same allowance 3.7's E2E used); D answers incorrectly; **E answers nothing** (silence case).
    - `close-question` → `reveal`. Inspect the fake provider's captured outbound bodies: A/B/C each got the correct+bonus template (distinct bonus amounts, ranks 1/2/3) with the two allowed emoji; D got the wrong-mid-game template (mentions the Q1 correct option's text, ends with "עוד הכול פתוח"); **E received nothing at all** for this question (AC-3).
    - `next-question` → Q2 opens. This time have **A, B, C, D all answer correctly** (4 correct answers on this single question, so the 4th — D — earns no Speed Bonus even though D is correct: proves the "correct without bonus" branch against a real DB round trip, not just a unit test); **E answers incorrectly**.
    - `close-question` → `reveal` (Q2 is the last question). Confirm: A/B/C got correct+bonus again (bonuses reset per-question); D got the plain correct template (`BonusPoints == 0`); E got the wrong-**last**-question template (mentions "ירושלים", no "עוד הכול פתוח" line) — the only assertion this E2E makes that a unit test cannot: the real `IsLastQuestion` computation against the real `questions` table.
    - Cross-check ranks against the DB's post-Q2 leaderboard directly (`SELECT` against `answers`/`participants`, same style as 3.7's E2E) — confirms `ResultsForRevealedQuestion`'s rank lookup matches `GetLeaderboard`/`RankLeaderboard` exactly, not just a plausible-looking number.
    - Clean up the scratch game row afterward (cascades); delete the harness afterward — same convention as every prior story.

### Review Findings

Code review 2026-08-06. Three adversarial layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor) plus independent quality-gate re-runs. Copy fidelity: all four Result templates verified byte-identical to EXPERIENCE.md's canonical rows, placeholder order correct in every row. AC-1/AC-2/AC-3 all assessed **Met**. Gates re-run independently and confirmed clean (`go vet`, `go test -count=1 ./...`, `sqlc generate` idempotent, `gofmt` flags are CRLF-only artifacts with byte-identical content).

- [x] [Review][Patch] **(was Decision — resolved 2026-08-06 by Avraham: carry `position` out of `Reveal`'s own committed game row rather than depending on the snapshot being fully built. Explicitly NOT the fresh-re-read fallback, which would reintroduce the wrong-question race the spec closed on purpose.)** A degraded post-commit snapshot permanently and silently loses the entire result fan-out — `snapshotAfterCommit` falls back to `emptySnapshot` on any `buildSnapshot` error, and `emptySnapshot` sets `State: g.State` (= `"revealed"`) while leaving `CurrentQuestion` nil. `dispatchAnswerRevealed` therefore passes its `State == StateRevealed` guard, hits the nil check, logs one `slog.Error`, and returns — nobody who answered receives a result. It is not recoverable: `Reveal` refuses any state but `StateQuestionClosed` (`engine.go:278-280`), so re-clicking Reveal returns a conflict and never re-dispatches. The same total-loss outcome applies to any error or 5s-timeout out of `ResultsForRevealedQuestion`, which makes four sequential DB round trips inside one shared budget. Fixing this means changing where `position` comes from (e.g. carrying it out of `Reveal`'s own committed game row instead of depending on the snapshot being fully built), which touches a design decision the story states explicitly — hence a decision, not a mechanical patch. [server/internal/game/engine.go:456-465, 528-538; server/internal/httpapi/control.go:111-117]
- [x] [Review][Patch] New query drops the single-question disambiguation that story 3.7's own code review installed on its sibling — `ListAnswerResultsForQuestion` uses a plain `JOIN questions q ... WHERE q.game_id = $1 AND q.position = $2`, exactly the shape `ListAnswersForScoring`'s 13-line comment warns against; that sibling was changed to `WHERE a.question_id = (SELECT q.id ... ORDER BY q.created_at LIMIT 1)` for this reason one story ago, in the same file. `00003` deliberately declines `UNIQUE (game_id, position)` and `CreateQuestion`'s `max(position)+1` has no backstop against concurrent inserts, so two questions can share a position; the union then messages participants with a grade for a question that was never revealed. The defective SQL came from this story's own Task 1 code block — the dev implemented the spec faithfully; the spec is what was wrong. [server/internal/store/queries/answers.sql:249-254]
- [x] [Review][Patch] `isLastQuestion` uses `position == len(questions)`, which disagrees with `NextQuestion`'s own end-of-game test (existence of `position+1`). With any non-dense position set the two diverge, and a wrong answerer on the real final question is told "עוד הכול פתוח" immediately before the game ends. [server/internal/game/results.go:116 vs server/internal/game/engine.go:358-370]
- [x] [Review][Patch] A missing rank silently renders "מקום 0 בטבלה" — `rankByParticipant[r.ParticipantID]` is a plain map read whose miss yields Go's zero value, with no comma-ok guard. Not reachable today (verified: `GetLeaderboard` LEFT JOINs so every `role='player'` participant gets a row, and `answers.sql:59` restricts answering to `role='player'`, so no answerer can lack a leaderboard entry), but every other anomaly in this same function fails loudly with an error — this one path emits a rank that cannot exist straight to a participant's phone. [server/internal/game/results.go:145]
- [x] [Review][Patch] `ResultsForRevealedQuestion`'s doc comment claims binding on `position` "closes that race entirely" — it does not. Only `ListAnswerResultsForQuestion` is position-bound; `GetLeaderboard(ctx, gameID)` reads current cumulative standings with no position or snapshot bound, and the four store calls share no transaction. Per-answer data for position N can be stitched to ranks that already include position N+1's points. The code is defensible (AC-1 asks for "current rank"); the comment asserts an invariant the code does not provide. [server/internal/game/results.go:58-61, 118]
- [x] [Review][Patch] Test reads a mutex-guarded stub field directly instead of through the copy-under-lock accessor the same file established for exactly this purpose (`PlayerRecipientsRequestedFor()`). Benign only because `/close-question` happens to spawn no goroutine today — an accident of current wiring, not a property the test asserts. [server/internal/httpapi/control_test.go]
- [x] [Review][Defer] `.Valid` never checked on `pgtype.Bool`/`pgtype.Int4` — `r.IsCorrect.Bool` / `r.PointsAwarded.Int32` read through unconditionally, so a NULL is indistinguishable from a genuine `false`/`0`. [server/internal/store/answers.go:228] — deferred, pre-existing: identical posture to the sibling `ListAnswersForScoring`, and the one documented case where the "IS NOT NULL means revealed" invariant fails is already an accepted risk logged in migration `00014` (accepted by Avraham, 3.7 code review).
- [x] [Review][Defer] The `r.Points < base` branch is unwritten, so a correct answer awarded less than `points_per_correct` would display the config value rather than the persisted award. [server/internal/game/results.go:133-139] — deferred, pre-existing: unreachable while `handleUpdateScoring` stays `requireDraftGame`-guarded (`httpapi/games.go:261`) and `AwardPoints` remains the only writer.
- [x] [Review][Defer] Error-path dispatch tests signal from inside the stub before the code under test evaluates its error guard, so they would pass even with the guard deleted. [server/internal/httpapi/control_test.go] — deferred, pre-existing: the new test faithfully copies story 3.2's `TestStartGameDispatchSkippedOnRecipientsError`, which has the identical weakness; fix both together.
- [x] [Review][Defer] Post-response dispatch goroutines are untracked by `srv.Shutdown`/`drainAndStop`, so a SIGTERM landing mid-dispatch drops every result message. [server/internal/httpapi/control.go, server/cmd/server/main.go:293-295] — deferred, pre-existing: true of `dispatchQuestionOpened` since story 3.2; this story adds a second instance and makes main.go's "this drain is the complete one" comment demonstrably false for both.
- [x] [Review][Defer] The next-question burst can overtake the reveal-result burst for the same recipient — a participant can see "שאלה N+1" before "לא נכון הפעם". [server/internal/httpapi/control.go, server/internal/wa/dispatch.go:19] — deferred, pre-existing: the 8-worker dispatcher provides no per-recipient ordering at all; this story adds a second burst type that can interleave with the first.

**Dismissed as noise (2):** duplicate messages on a repeated Reveal (disproved — `Reveal` requires `state == question_closed`, so a second call errors and never re-dispatches); bidi reordering of the raw-interpolated correct answer (the placeholder is line-final in both wrong-answer templates, `התשובה: %s\n`, so the mid-line reordering case does not arise — and the raw treatment is spec-mandated, test-pinned, and consistent with the question-text precedent).

## Dev Notes

### Architecture guardrails (violations = rework)

- **Dependency direction unaffected**: `results.go` lives in `game`, imports only stdlib (`context`, `fmt`) and `store/gen` for the `gen.Question` value — no new dependency, and `game` still never imports `wa`/`httpapi`/`ws`. `wa/result_notifier.go` imports `game` (for `game.PersonalResult`) exactly like `question_notifier.go` already imports `game` for `game.CurrentQuestion` — an existing, established direction, not a new one.
- **Grade never disclosed before Reveal (FR-15/16/AC-2) — satisfied by construction, not by a runtime check in this story's code**: `dispatchAnswerRevealed` only ever runs after `Engine.Reveal` has already committed (guarded on `snapshot.State == StateRevealed`), and `ResultsForRevealedQuestion` only reads `points_awarded`/`is_correct`, both of which stay NULL until `RevealCurrentQuestionAndAwardPoints` (story 3.7) writes them inside Reveal's own transaction. There is no code path in this story that can read a grade before Reveal — nothing to additionally guard here.
- **Silence for non-answerers (AC-3) — also satisfied by construction**: `ListAnswerResultsForQuestion`'s SQL only returns rows present in `answers`; a Participant who never answered has no row and therefore never appears in `RevealedQuestionResults.Results`, so `ResultNotifier.DispatchAnswerRevealed` never enqueues anything for them. No explicit "skip non-answerers" branch exists anywhere — there is nothing to skip.
- **NFR-2 ("never block the game loop") — no new concern**: this story adds one more DB-read-then-dispatch step, structurally identical to `dispatchQuestionOpened` (story 3.2): spawned as a goroutine after the HTTP response is written, with its own bounded, detached-from-request-cancellation timeout, so an organizer's connection closing (or a slow read) never blocks the `/reveal` response or corrupts state.
- **No new migration, no new `gen.*` struct fields**: every column this story reads (`answers.points_awarded`, `answers.is_correct`, `questions.correct_option`/`accepted_answers`, `participants.phone`) already exists. The only new sqlc surface is one query's generated function + param/row types.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/game/engine.go](server/internal/game/engine.go)** — as of this story's baseline (3.7 fully on disk, uncommitted): `Store` interface already has `ListAnswersForScoring`/`RevealCurrentQuestionAndAwardPoints`/`GetLeaderboard` (story 3.7). This story adds exactly one more line (`ListAnswerResultsForQuestion`) and nothing else — `Reveal`, `buildSnapshot`, `emptySnapshot` are untouched.
- **[server/internal/game/snapshot.go](server/internal/game/snapshot.go)** — `Snapshot.Leaderboard` (story 3.7) is read by this story's caller (`dispatchAnswerRevealed` reads `snapshot.CurrentQuestion.Position`) but `Snapshot` itself gains no new field this story — `RevealedQuestionResults`/`PersonalResult` are internal Go values, never serialized to the WS/REST wire.
- **[server/internal/httpapi/control.go](server/internal/httpapi/control.go)** — `handleReveal` currently takes `(engine ControlEngine, hub SnapshotBroadcaster)`, two args, unlike `handleStartGame`/`handleNextQuestion`'s three (`..., dispatcher QuestionDispatcher`). This story brings `handleReveal` to the same three-arg shape, with its own dispatcher type (`ResultDispatcher`, not `QuestionDispatcher` — a Reveal never dispatches a *question*, so reusing that interface would be a type-safety lie even though the underlying wiring pattern is identical).
- **[server/internal/httpapi/router.go](server/internal/httpapi/router.go)** — `NewRouter` currently takes 9 positional args, ending in `dispatcher QuestionDispatcher` (added by story 3.2 in exactly this same way, over story 3.1's simpler signature). This story repeats that precedent: one more trailing param, `resultDispatcher ResultDispatcher`.
- **[server/cmd/server/main.go](server/cmd/server/main.go)** — `questionNotifier := wa.NewQuestionNotifier(dispatcher, logger)` is the last relevant line before `NewRouter`'s call; this story inserts `resultNotifier := wa.NewResultNotifier(dispatcher, logger)` right after it (same `dispatcher` — the one `*wa.Dispatcher`/`Replier` instance backs every outbound notifier in this codebase, not a new one per notifier).
- **[server/internal/wa/messages_he.go](server/internal/wa/messages_he.go)** — purely additive; every existing template/function is untouched. The file's header "current map" comment is the one piece of existing text this story edits (four `-> story 3.8` lines gain `(below)`, matching the file's own established convention for every prior landed row).

### Design decisions worth flagging explicitly

- **`ResultsForRevealedQuestion` takes `position` as an explicit parameter — it does NOT re-derive "the revealed question" from a fresh `g.CurrentQuestionPosition` read.** This was a deliberate choice to close a real (if narrow) race: this method runs from a goroutine spawned *after* `/reveal`'s HTTP response is already written, so an organizer could click "next question" before it runs. `NextQuestion` is the only thing that ever changes `current_question_position`; if this method re-read it fresh, that race would silently compute results for the *new* question instead of the one that was actually just revealed. Binding on the caller's own already-observed `snapshot.CurrentQuestion.Position` instead removes the race by construction — the same "bind to the specific identity, not the current pointer" discipline `RecordAnswer`'s own SQL already documents for a structurally identical race. Do not "simplify" this back to reading `g.CurrentQuestionPosition` inside the method.
- **`PersonalResult` splits `BasePoints`/`BonusPoints` rather than carrying one `Points` total.** The templates table shows them as two separate numbers on two separate lines ("+100 נקודות" then "⚡ בונוס מהירות +50") — never their sum — so the split is what the message layer actually needs; computing `BasePoints + BonusPoints` back in `wa` to get `answers.points_awarded` would be pointless indirection.
- **No AI/grading-stage involvement.** This story only reads already-graded, already-scored data (`is_correct`, `points_awarded`) — Story 3.6's AI stage and Story 3.7's scoring are both strictly upstream and already complete by the time `Reveal` has committed. Nothing here waits on or re-triggers grading.
- **Rank is looked up per-participant from the same `RankLeaderboard` output Story 3.7 already produces for the WS snapshot** (`GetLeaderboard` + `RankLeaderboard`), not recomputed with different logic — a second, subtly different ranking algorithm living in `wa` or `httpapi` would be a real disaster-class bug (two "current rank"s disagreeing between the Audience Display, later, and the participant's own WhatsApp).

### Testing standards

Go stdlib `testing`, co-located `_test.go`, same conventions as every prior story in this epic. `results.go`'s `ResultsForRevealedQuestion` tests live in `game` (new `results_test.go`, reusing `engine_test.go`'s `stubStore`/fixtures in the same package — no new test-only package needed). `wa` package tests follow `question_notifier_test.go`/`messages_he_test.go`'s existing patterns exactly (canonical-copy constants + isolate-stripping for template tests, `stubReplier`-based dispatch tests). `httpapi` tests extend the existing `stubControlEngine`/mutex-guarded-goroutine-signal pattern (`playerRecipientsDone` → `resultsForRevealedQuestionDone`) established by story 3.2's dispatch tests. No real DB in any unit test — the SQL itself (Task 1's new query) is verified only by the local E2E harness (Task 9), the project's established and repeatedly-documented testing standard for hand-written SQL.

### Project Structure Notes

**New:**
- `server/internal/game/results.go`
- `server/internal/game/results_test.go`
- `server/internal/wa/result_notifier.go`
- `server/internal/wa/result_notifier_test.go`

**Modified:**
- `server/internal/store/queries/answers.sql` (+`ListAnswerResultsForQuestion`)
- `server/internal/store/answers.go` (+`AnswerResultRow` type; +`ListAnswerResultsForQuestion` wrapper)
- `server/internal/store/gen/*` (sqlc-regenerated — new query function/param/row types only)
- `server/internal/game/engine.go` (`Store` interface +`ListAnswerResultsForQuestion`)
- `server/internal/wa/messages_he.go` (+4 Result templates/functions; header comment updated)
- `server/internal/httpapi/control.go` (+`ResultDispatcher` interface, +`dispatchAnswerRevealed`, `handleReveal` gains a 3rd param)
- `server/internal/httpapi/router.go` (`NewRouter` gains a 10th param `resultDispatcher ResultDispatcher`; `/reveal` mount updated)
- `server/cmd/server/main.go` (+`resultNotifier`; `NewRouter` call updated; Store-satisfaction comment updated)
- `server/internal/game/engine_test.go` (`stubStore` gains `ListAnswerResultsForQuestion`)
- `server/internal/httpapi/control_test.go` (`stubControlEngine` gains `ResultsForRevealedQuestion`; new `stubResultDispatcher`; router-building helpers updated; new "Result dispatch" test block)
- `server/internal/httpapi/router_test.go`, `games_test.go`, `packages_test.go` (every `NewRouter(...)` call gains a trailing `nil` — Task 7)
- `server/internal/wa/messages_he_test.go` (+4 new canonical-copy test pairs)

**Untouched:** `server/migrations/*` (no new migration) · `server/internal/game/scoring.go`/`scoring_test.go` (this story consumes `RankLeaderboard`, doesn't change it) · `server/internal/game/answers.go` (`RecordAnswer`'s path is unaffected — this is a Reveal-time concern) · `server/internal/game/snapshot.go` (no new field) · `server/internal/ws/*` (no wire-shape change) · `web/*` (no dashboard/Audience-Display surface for this story — WhatsApp-only, per the epic's own FR-6 scope; Epic 4 and story 3.10 are the web-facing consumers of the same underlying score data, unaffected here) · `server/internal/wa/question_notifier.go`, `dispatch.go`, `client.go` (unrelated to this story's dispatch path beyond sharing the same `Replier`/`Dispatcher`).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.8] — story + all 3 epic ACs verbatim, Epic 3 context, FR-6 scope
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#WhatsApp-message-templates] — the four canonical Result rows' exact copy, emoji-count exception (A20), last-question wrong-answer variant (A5), and the "Free-Text results never mention which Validation Stage matched" rule (A7, already satisfied — this story's messages never reference `stage`)
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Voice-and-Tone] — emoji discipline (symbolic set, ≤1 per message except the stated exceptions), warm-playful register, gender-neutral Hebrew
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#Key-Flows] — Flow 2 (UJ-2, "מקום 1 בטבלה"/bonus example), Flow 3 (UJ-3, free_text wrong-answer example), Flow 4 (UJ-4, non-answerer silence: "מי שלא — כלום. אף אחד לא ננזף")
- [Source: _bmad-output/planning-artifacts/architecture.md] — "AnswerRevealed event → per-participant grade/points/rank messages" (FR-6 architectural home); dependency direction (`wa`/`httpapi` → `game` → `store`); centralized-Hebrew-copy rule (`messages_he.go`)
- [Source: server/internal/game/engine.go, scoring.go, snapshot.go] — story 3.7's `Store` interface, `AwardPoints`/`RankLeaderboard`/`LeaderboardEntry`, `Snapshot.Leaderboard` this story consumes unchanged
- [Source: server/internal/store/queries/answers.sql, store/answers.go] — `ListAnswersForScoring`'s NULL-safety reasoning (post-Reveal, `.Bool`/`.Int32` without `.Valid` checks) this story's new query mirrors exactly
- [Source: server/internal/store/queries/answers.sql#RecordAnswer] — the "bind to the specific question_id, don't re-derive from current_question_position" race-avoidance discipline `ResultsForRevealedQuestion`'s explicit `position` parameter follows
- [Source: server/internal/wa/question_notifier.go, dispatch.go] — `Replier`/`QuestionNotifier`'s shape (goroutine-spawned, non-blocking `Enqueue`, per-recipient blank-phone skip+WARN) `ResultNotifier` mirrors exactly
- [Source: server/internal/wa/messages_he.go] — bidi-isolation rule (LTR tokens/digits isolated via `ltr()`, content fields like question text left raw) this story's four new templates follow; the file's "rows land with the story that sends them" convention
- [Source: server/internal/httpapi/control.go, router.go] — `dispatchQuestionOpened`'s exact goroutine-timing/detached-context pattern (story 3.2) this story's `dispatchAnswerRevealed` mirrors; `NewRouter`'s existing 9-arg shape (`dispatcher QuestionDispatcher` as story 3.2's own precedent for growing this signature)
- [Source: server/cmd/server/main.go] — the `WHATSAPP_API_BASE_URL` fake-provider override this story's E2E task relies on to inspect sent message bodies
- [Source: server/internal/httpapi/control_test.go, router_test.go] — `stubControlEngine`/`stubQuestionDispatcher`/`waitForSignal`'s existing goroutine-testing patterns this story's new stubs and tests extend
- [Source: server/migrations/00003_games_questions.sql] — `correct_option`'s 1-based, `BETWEEN 1 AND 4` convention (`Options[CorrectOption-1]` indexing)
- [Source: server/migrations/00014_answer_points.sql] — `points_awarded`'s NULL-until-revealed convention (story 3.7) this story's SQL comment cites
- [Source: _bmad-output/implementation-artifacts/3-7-scoring-with-speed-bonuses.md] — previous story's Dev Notes/task structure, E2E harness conventions, and its own "verify the prerequisite is actually on disk" pattern this story's Prerequisite section follows

## Dev Agent Record

### Agent Model Used

Claude Sonnet 5

### Debug Log References

- Verified all five story-3.7 prerequisites (Store interface additions, `scoring.go`, `Snapshot.Leaderboard`, `store/answers.go` types, `00014_answer_points.sql`) against disk before writing any code — all matched exactly.
- 3.7 was still unmerged to `main` at story start (branch `story/3-7-scoring-with-speed-bonuses` pushed but no PR merged). Paused and asked the user to merge first, per project convention of not stacking a new story on unmerged, potentially-diverging prior work; user merged PR #5 (commit `55ad5f6`) during the session, then this story branched from updated `main`.
- `sqlc generate` diff confirmed limited to `answers.sql.go` gaining `ListAnswerResultsForQuestion` + param/row types, as the story predicted.
- Two self-inflicted `gofmt` misalignments (manual edits to `messages_he_test.go`'s const block and `control_test.go`'s struct literal) — fixed with `gofmt -w`; every other `gofmt -l` flag confirmed as the known CRLF-checkout artifact (byte-identical diff once line endings are normalized), same caveat stories 3.4-3.7 documented.
- Local E2E harness (`cmd/e2escratch`, deleted after use): hit two bugs before it passed clean — (1) `CreateQuestionParams.Options`/`AcceptedAnswers` must be explicit `[]string{}`, not the Go nil-slice zero value, or pgx sends SQL NULL against the `NOT NULL DEFAULT '{}'` column; (2) the fake WhatsApp provider's captured-message store needed an explicit reset between Q1's and Q2's reveal-dispatch assertions, since it does not reset itself between questions — without it, the Q2 assertions were reading Q1's stale captured bodies. Both fixed; the full 53-check run (2-question, 5-player game) then passed clean against real local Postgres.

### Completion Notes List

- Implemented all 9 tasks exactly as specified in the story's Dev Notes/task code blocks — no deviations from the planned shapes (`store.AnswerResultRow`, `game.PersonalResult`/`RevealedQuestionResults`/`ResultsForRevealedQuestion`, the four `wa` Result templates, `wa.ResultNotifier`, `httpapi.ResultDispatcher`/`dispatchAnswerRevealed`, the `NewRouter`/`main.go` wiring).
- All three epic ACs verified: AC-1 (grade/points/rank copy, including the two-emoji bonus exception and the last-question "עוד הכול פתוח" drop) via unit tests (`messages_he_test.go`, `result_notifier_test.go`) and the E2E's captured-body assertions; AC-2 (no grade before Reveal) is satisfied by construction — `dispatchAnswerRevealed` only ever runs after `Engine.Reveal` has committed — and has no dedicated runtime guard to test; AC-3 (silence for non-answerers) verified by both a unit test (`TestResultsForRevealedQuestionNoAnswersReturnsEmptyResultsSlice`) and the E2E (Eve gets zero messages for the question she never answered).
- Zero regressions: full existing suite (`go test ./...`) passed unchanged both before and after this story's changes.
- Quality gates all clean: `gofmt -l .` (no real issues, only the known CRLF-checkout noise), `go vet ./...`, `go test ./...`, and a `sqlc generate` diff limited to the one new query.
- Local E2E (`cmd/e2escratch`, written, run against real local Postgres, then deleted): a 2-question/5-player game exercising the exact scenario the story's Task 9 specifies — MCQ with staggered correct answers (distinct Speed Bonuses + ranks), a wrong MCQ answer, a silent non-answerer, then a free_text final question with a 4th correct answer (proving the no-bonus branch against real DB rank data) and a wrong answer on the last question (proving the real `IsLastQuestion` computation). 53/53 checks passed, including an independent leaderboard-rank cross-check via a fresh `GetLeaderboard` + `RankLeaderboard` call.
- No new migration; no new `gen.*` struct fields — confirmed unchanged per the story's own "Untouched" list.

### File List

**New:**
- `server/internal/game/results.go`
- `server/internal/game/results_test.go`
- `server/internal/wa/result_notifier.go`
- `server/internal/wa/result_notifier_test.go`

**Modified:**
- `server/internal/store/queries/answers.sql`
- `server/internal/store/answers.go`
- `server/internal/store/gen/answers.sql.go` (sqlc-regenerated)
- `server/internal/game/engine.go`
- `server/internal/game/engine_test.go`
- `server/internal/wa/messages_he.go`
- `server/internal/wa/messages_he_test.go`
- `server/internal/httpapi/control.go`
- `server/internal/httpapi/control_test.go`
- `server/internal/httpapi/router.go`
- `server/internal/httpapi/router_test.go`
- `server/internal/httpapi/games_test.go`
- `server/internal/httpapi/packages_test.go`
- `server/cmd/server/main.go`
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (status tracking)

## Change Log

- 2026-08-06: Story implemented end-to-end (Tasks 1-9), all ACs met, zero regressions, local E2E against real Postgres passed 53/53 checks. Status moved to review.
- 2026-08-07: Code review (3 adversarial layers + independent gate re-runs). Copy fidelity clean — all four Result templates byte-identical to EXPERIENCE.md, AC-1/2/3 all Met. 1 decision + 6 patches applied, 5 deferred, 2 dismissed. Status moved to done.
  - **`Engine.Reveal` now returns `(Snapshot, int32, error)`** — the just-revealed position, taken from the committed game row. `dispatchAnswerRevealed` binds to it instead of `snapshot.CurrentQuestion.Position`, which silently and permanently dropped the entire result fan-out whenever `snapshotAfterCommit` degraded to `emptySnapshot` (State "revealed", nil CurrentQuestion) — unrecoverable, since Reveal refuses to re-run from `revealed`. Two new httpapi tests cover the degraded-snapshot and unset-position paths.
  - **`ListAnswerResultsForQuestion` gained the `ORDER BY q.created_at LIMIT 1` subquery** its sibling `ListAnswersForScoring` received at story 3.7's review. The shipped version used the plain `q.position` join that query's comment explicitly warns against; with two questions sharing a position it would union their answers and message participants a grade for a question that was never revealed. **The defective SQL came from this story's own Task 1 code block — the spec was wrong, not the implementation.**
  - **`isLastQuestion` now mirrors `NextQuestion`** (no question at `position+1`) instead of `position == len(questions)`, which disagreed with the state machine on any non-dense position set.
  - **A participant absent from the leaderboard is now an error**, not a silent `מקום 0 בטבלה` on a real phone.
  - **`ResultsForRevealedQuestion`'s doc comment corrected** — it claimed the position binding "closes that race entirely"; `GetLeaderboard` is deliberately unbound, and the comment now says so and explains why that is acceptable under AC-1's "current rank".
  - **Stub accessors added** (`ResultsForRevealedQuestionRequestedFor`/`Position`) so tests copy mutex-guarded fields under the lock, per the file's own `PlayerRecipientsRequestedFor` convention.
  - Gates after patching: `go vet ./...` clean · `go test -count=1 ./...` all packages pass · `sqlc generate` idempotent · `gofmt` flags are CRLF-only artifacts (content byte-identical). `go test -race` could not run — `CGO_ENABLED=0` in this environment — so the new goroutine path is not race-verified.
