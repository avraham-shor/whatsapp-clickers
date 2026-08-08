---
baseline_commit: 65fafe96480f659dee6e769029709f37b8e770d0
---

# Story 3.9: Final Results for Everyone

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a Participant or Spectator,
I want the final results in my WhatsApp when the Game ends,
so that the event closes with my score and the winner's name (FR-6 game-end, FR-18 messaging half).

## ⚠️ Prerequisite: branch from 3.8, not from `main`

At the time this story was created, **story 3.8 (Personal Results After Reveal) is committed and pushed but NOT merged to `main`**. `main` is at `55ad5f6` (story 3.7's merge); 3.8's work sits on branch `story/3-8-personal-results-after-reveal` at commits `acbd781` (the story) + `65fafe9` (a CI copy-centralization fix on top of it). This story's `baseline_commit` is `65fafe9`.

**Branch from wherever 3.8's work actually sits** — the `story/3-8-personal-results-after-reveal` tip, or `main` once 3.8 has merged. Never from `main` alone while 3.8 is unmerged: this story extends `NewRouter`'s signature (which 3.8 grew to 10 args) and `messages_he.go`'s header map (which 3.8 populated), and branching off `main` produces a guaranteed conflict plus a wrong argument count.

Verify these 3.8/3.7 deliverables exist on disk exactly as described before writing any code:
- `server/internal/httpapi/router.go` — `NewRouter` takes **10** positional args, ending `dispatcher QuestionDispatcher, resultDispatcher ResultDispatcher`.
- `server/internal/httpapi/control.go` — `ResultDispatcher` interface; `dispatchAnswerRevealed`; `handleReveal(engine, hub, dispatcher)`; `handleNextQuestion(engine, hub, dispatcher QuestionDispatcher)`; `handleStopGame(engine, hub)` (two args — this story makes it three).
- `server/internal/wa/result_notifier.go` — `ResultNotifier`/`NewResultNotifier`/`DispatchAnswerRevealed` (the shape this story's `FinalNotifier` mirrors).
- `server/internal/wa/messages_he.go` — header "current map" comment ends with two `-> story 3.9` lines (no `(below)` suffix yet); four Result templates present.
- `server/internal/game/scoring.go` — `RankLeaderboard(scores []store.ParticipantScore) []LeaderboardEntry` with `LeaderboardEntry{ParticipantID, DisplayName, Score, Rank}`, standard competition ranks (1,1,3).
- `server/internal/game/engine.go` — `Store` interface already has `ListParticipants` and `GetLeaderboard`; `NextQuestion` and `StopGame` both transition to `finished` via `store.FinishGame`.
- `server/cmd/server/main.go` — `resultNotifier := wa.NewResultNotifier(dispatcher, logger)` immediately before the `NewRouter` call.

**Re-verify these shapes still match before writing code.** If 3.8 has since merged or changed further, confirm the quoted signatures below are still accurate.

## Acceptance Criteria

1. **Given** the `GameFinished` event, **when** final messages dispatch, **then** every Participant **and** every Spectator receives their score, rank, and the winner's name — copy per the EXPERIENCE.md templates table Final-results rows. *(epic AC-1)*
2. **Given** a tie for first place, **then** all tied names are included as winners — the `הזוכים` (plural) form, joined per A16. *(epic AC-2)*
3. **Given** the winner(s), **then** they **additionally** receive the personal winner variant — the `מזל טוב` row, addressed jointly on a tie per A16 (UJ-2's emotional climax). *(epic AC-3)*

**Scope boundaries for this story:**
- **No new migration, no new SQL, no new sqlc surface, no new `store` wrapper.** Every input already exists: `ListParticipants` (roles + phones) and `GetLeaderboard` (per-player cumulative score) are both already on the `game.Store` interface. If you find yourself writing SQL, stop — you have gone off-spec.
- **No `web/` changes.** Story 3.10 (dashboard summary) and Epic 4 (winner takeover) are the web-facing consumers of the same score data; nothing on the dashboard or Audience Display changes here.
- **No `Snapshot` wire-shape change.** `FinalResults`/`FinalRecipient` are internal Go values, never serialized to WS/REST.

## Tasks / Subtasks

- [x] **Task 1: EXPERIENCE.md — add two rows to the templates table BEFORE writing any Go** (AC: 1)

  `messages_he.go`'s own header states the rule: *"The canonical row list is the UX spec, not this file… A new message type is added to that table FIRST, then here."* This story needs two rows the table does not yet have. Add both to `_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md`'s **"WhatsApp message templates"** table, immediately after the existing `Final results` / `Winner's final message` rows, and add the matching entries to the **"WhatsApp — message types"** table near the top of the same file.

  - [x] **Row: `Final results — spectator`.** The table's existing `Final results` row's "When" column reads *"Game ends; all Participants + Spectators"*, but its copy carries a personal second line (`סיימת במקום [דירוג] עם [ניקוד] נקודות`) that a Spectator cannot have — Spectators never answer, and `GetLeaderboard` filters `role = 'player'`, so no Spectator has a score or a rank at all. The winner half of the message is what they were promised ("התוצאות יגיעו לכאן בסוף המשחק 🏆", the Spectator notice, story 2.5). Copy = the existing Final-results row's **first line only**:
    ```
    | Final results — spectator | "המשחק נגמר! 🏆 הזוכה: [שם] עם [ניקוד] נקודות." | Game ends; Spectators (no personal score/rank line — Spectators never answer); ties: "הזוכים: [שם] ו-[שם] עם [ניקוד] נקודות" `[A16]` |
    ```
  - [x] **Row: `Final results — no winner`.** Reachable and not exotic: `StopGame` ("עצור") can finish a game from `question_open` before any Reveal has run, so every `answers.points_awarded` is still NULL and `GetLeaderboard` returns every player at score 0. `RankLeaderboard` then gives **every** player rank 1 — naming all of them winners of 0 points, and sending 60 people "מזל טוב… ניצחת עם 0 נקודות". Fires whenever the top score is 0 (including a finished game with zero players). Suppresses the winner name, the personal rank line, and the entire winner-message pass. **No 🏆** — EXPERIENCE.md's "🏆 discipline" reserves the trophy for the winner moment, and there isn't one:
    ```
    | Final results — no winner | "המשחק נגמר! הפעם לא נצברו נקודות — נתראה במשחק הבא!" | Game ends with no positive score (e.g. stopped before the first Reveal); sent to Participants and Spectators alike; no winner message is sent |
    ```
  - [x] Do **not** add a row for the multi-winner name join — `[A16]`'s "up to three names" is an Audience-Display *layout* constraint (a projector has finite space; see the "Winner takeover" stage row), not a copy rule. WhatsApp has no such constraint, so this story joins **all** tied names with no cap. Record that reading in the `Final results` row's cell as a parenthetical so the next reader does not re-litigate it.

- [x] **Task 2: `wa/messages_he.go` — seven new templates + a name-join helper** (AC: 1, 2, 3)
  - [x] Update the header "current map" comment: the two `-> story 3.9` lines become `-> story 3.9 (below)`, and add the two new rows, matching every landed row's convention:
    ```
    //	Final results            -> story 3.9 (below)
    //	Final results - spectator -> story 3.9 (below)
    //	Final results - no winner -> story 3.9 (below)
    //	Winner's final message   -> story 3.9 (below)
    ```
    (Keep the map's existing alignment style; the exact column padding is cosmetic, `gofmt` does not touch comment interiors.)
  - [x] Add `strings` to the import block (needed by `joinNames`).
  - [x] Append after `resultWrongLastMessage`:
    ```go
    // joinNames renders one or more display names as a Hebrew list: a single
    // name alone, two joined by the vav conjunction, three or more
    // comma-separated with the conjunction before the last. Every name is
    // ltr()-isolated individually, same reasoning as welcomeMessage: a
    // WhatsApp profile name can legitimately be Latin-script (mixed-language
    // families), and isolating a Hebrew one is harmless.
    //
    // No cap on the number of names. EXPERIENCE.md's A16 "up to three names"
    // is an Audience-Display layout constraint (finite projector space), not
    // a copy rule; a WhatsApp message has no such limit, and truncating the
    // list would drop a real winner's name from the one message that names
    // them.
    //
    // Callers guarantee len(names) > 0 — the no-winner case is a different
    // template entirely (msgFinalResultsNoWinner), selected upstream in
    // FinalNotifier.DispatchGameFinished.
    func joinNames(names []string) string {
        isolated := make([]string, 0, len(names))
        for _, n := range names {
            isolated = append(isolated, ltr(n))
        }
        if len(isolated) == 1 {
            return isolated[0]
        }
        last := len(isolated) - 1
        return strings.Join(isolated[:last], ", ") + nameConjunction + isolated[last]
    }

    // nameConjunction is the vav-prefix separator before the final name in a
    // multi-name list, per the templates table's tie forms. It lives here,
    // not inline in joinNames, so every Hebrew fragment in this package stays
    // a named constant in this file.
    const nameConjunction = " ו-"

    // msgFinalResultsTemplate is the Final results row for a player when
    // exactly one participant holds first place: the winner line, then the
    // recipient's own placing. Two emoji are permitted on this row
    // (EXPERIENCE.md A20's winner/final-results exception); it uses one.
    const msgFinalResultsTemplate = "המשחק נגמר! 🏆 הזוכה: %s עם %s נקודות.\nסיימת במקום %s עם %s נקודות — כל הכבוד!"

    // finalResultsMessage returns the finished Final-results copy for a
    // player. winnerNames must hold exactly one name; winnerScore is the
    // winning cumulative score; rank/score are this recipient's own final
    // placing. A winner receives this too, then the winner variant on top
    // (epic AC-3's "additionally").
    func finalResultsMessage(winnerNames []string, winnerScore int32, rank int, score int32) string {
        return fmt.Sprintf(msgFinalResultsTemplate,
            joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))),
            ltr(strconv.Itoa(rank)), ltr(strconv.Itoa(int(score))))
    }

    // msgFinalResultsTieTemplate is the Final results row's tie form: the
    // plural winner noun, all tied names joined (EXPERIENCE.md A16). The
    // score appears once — it is by definition the same for every tied
    // winner.
    const msgFinalResultsTieTemplate = "המשחק נגמר! 🏆 הזוכים: %s עם %s נקודות.\nסיימת במקום %s עם %s נקודות — כל הכבוד!"

    func finalResultsTieMessage(winnerNames []string, winnerScore int32, rank int, score int32) string {
        return fmt.Sprintf(msgFinalResultsTieTemplate,
            joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))),
            ltr(strconv.Itoa(rank)), ltr(strconv.Itoa(int(score))))
    }

    // msgFinalResultsSpectatorTemplate is the Final results - spectator row:
    // the winner line alone. A Spectator has no score and no rank
    // (GetLeaderboard filters role = 'player'), so the personal second line
    // of msgFinalResultsTemplate has nothing to render.
    const msgFinalResultsSpectatorTemplate = "המשחק נגמר! 🏆 הזוכה: %s עם %s נקודות."

    func finalResultsSpectatorMessage(winnerNames []string, winnerScore int32) string {
        return fmt.Sprintf(msgFinalResultsSpectatorTemplate, joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))))
    }

    // msgFinalResultsSpectatorTieTemplate is the spectator row's tie form.
    const msgFinalResultsSpectatorTieTemplate = "המשחק נגמר! 🏆 הזוכים: %s עם %s נקודות."

    func finalResultsSpectatorTieMessage(winnerNames []string, winnerScore int32) string {
        return fmt.Sprintf(msgFinalResultsSpectatorTieTemplate, joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))))
    }

    // msgFinalResultsNoWinner is the Final results - no winner row, sent to
    // players and spectators alike when no participant finished with a
    // positive score (a game stopped before the first Reveal, or one with no
    // players at all). No trophy: EXPERIENCE.md reserves it for the winner
    // moment, and there is none. No placeholders — a rank line would read
    // "place 1" for every single recipient, which is exactly the outcome
    // this row exists to avoid.
    const msgFinalResultsNoWinner = "המשחק נגמר! הפעם לא נצברו נקודות — נתראה במשחק הבא!"

    func finalResultsNoWinnerMessage() string {
        return msgFinalResultsNoWinner
    }

    // msgWinnerFinalTemplate is the Winner's final message row, sent only to
    // the winner and only on top of their Final-results message — UJ-2's
    // emotional climax. Singular verb form for a sole winner.
    const msgWinnerFinalTemplate = "מזל טוב, %s! 🏆 ניצחת עם %s נקודות!"

    func winnerFinalMessage(winnerNames []string, winnerScore int32) string {
        return fmt.Sprintf(msgWinnerFinalTemplate, joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))))
    }

    // msgWinnerFinalTieTemplate is the Winner's final message tie form:
    // every tied winner receives the same jointly-addressed message naming
    // all of them, with the plural verb (EXPERIENCE.md A16).
    const msgWinnerFinalTieTemplate = "מזל טוב, %s! 🏆 ניצחתם עם %s נקודות!"

    func winnerFinalTieMessage(winnerNames []string, winnerScore int32) string {
        return fmt.Sprintf(msgWinnerFinalTieTemplate, joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))))
    }
    ```

- [x] **Task 3: `game/final.go` (new file) — resolve the whole game-end dispatch data set** (AC: 1, 2, 3)

  **🚨 ZERO Hebrew characters in this file, comments included.** CI's `Hebrew copy centralization` gate greps the Hebrew Unicode block across every non-`_test.go` `.go` file and exempts only `internal/wa/messages_he.go`. It does not distinguish a comment from a literal — commit `65fafe9` exists solely because story 3.8's `results.go` doc comments quoted three Hebrew fragments and turned CI red. Name the template identifier in English instead (`msgFinalResultsNoWinner`, `msgWinnerFinalTieTemplate`, …).

  - [x] New file `server/internal/game/final.go`:
    ```go
    package game

    import (
        "context"
        "fmt"
    )

    // FinalRecipient is one game-end message recipient's resolved inputs to
    // the Final-results templates (wa/messages_he.go's Final-results rows) —
    // the WhatsApp game-end dispatch's data source (FR-6 game-end, FR-18,
    // story 3.9).
    //
    // Every Participant is a recipient, players and spectators alike: the
    // game-end message is the one message a Spectator was explicitly
    // promised at join time (the Spectator-notice row, story 2.5). Rank and
    // Score are meaningless for a spectator and are left at 0 —
    // IsSpectator, not a zero check, is what selects the spectator template.
    type FinalRecipient struct {
        Phone       string
        IsSpectator bool
        // IsWinner marks a player who shares the top positive score. Winners
        // receive the ordinary final-results message AND, on top of it, the
        // personal winner message (epic AC-3's "additionally").
        IsWinner bool
        Rank     int
        Score    int32
    }

    // FinalResults is ResultsForFinishedGame's return value — everything
    // the WhatsApp game-end dispatch needs, resolved in one call.
    type FinalResults struct {
        // Recipients holds one entry per Participant in the game, in
        // ListParticipants' joined_at order. Non-nil, possibly empty, same
        // "never null" discipline as PlayerRecipients.
        Recipients []FinalRecipient
        // WinnerNames holds the display names of every player sharing the
        // top positive score, in leaderboard order. EMPTY means there is no
        // winner (see the WinnerScore doc below), which selects
        // msgFinalResultsNoWinner for everyone and suppresses the winner
        // message pass entirely. Non-nil, possibly empty.
        WinnerNames []string
        // WinnerScore is the shared top score. Zero exactly when
        // WinnerNames is empty.
        WinnerScore int32
    }

    // ResultsForFinishedGame resolves the WhatsApp game-end dispatch's full
    // data source for gameID (FR-6 game-end, FR-18 messaging half). Named
    // to sit beside ResultsForRevealedQuestion (story 3.8), its per-question
    // sibling - and deliberately NOT named FinalResults, which is the
    // struct it returns.
    //
    // Called only after a transition into StateFinished has committed —
    // from httpapi/control.go's own goroutine, spawned after the REST
    // response is written, mirroring dispatchQuestionOpened/
    // dispatchAnswerRevealed's placement (stories 3.2, 3.8) exactly. Both
    // transitions into finished (NextQuestion past the last question, and
    // StopGame) route here.
    //
    // Unlike ResultsForRevealedQuestion this takes no position and no
    // organizerID: neither store call it makes is question-scoped or
    // organizer-scoped, and finished is terminal — nothing can advance the
    // game underneath this read, so there is no equivalent of story 3.8's
    // wrong-question race to close. The organizerID omission matches
    // PlayerRecipients' own posture: every call site has already validated
    // ownership via the state transition that immediately preceded it.
    //
    // Winner definition: every player whose Rank is 1 AND whose Score is
    // strictly positive. The positivity condition is load-bearing, not
    // defensive. StopGame can finish a game from question_open, before any
    // Reveal has awarded a single point, at which moment GetLeaderboard
    // returns every player at 0 and RankLeaderboard hands all of them
    // rank 1 - without this condition, an aborted game would congratulate
    // the entire room on winning with zero points. The empty-WinnerNames
    // result is that case's signal to the caller.
    func (e *Engine) ResultsForFinishedGame(ctx context.Context, gameID string) (FinalResults, error) {
        participants, err := e.store.ListParticipants(ctx, gameID)
        if err != nil {
            return FinalResults{}, err
        }
        scores, err := e.store.GetLeaderboard(ctx, gameID)
        if err != nil {
            return FinalResults{}, err
        }
        entries := RankLeaderboard(scores)

        byParticipant := make(map[string]LeaderboardEntry, len(entries))
        winnerNames := make([]string, 0, 1)
        var winnerScore int32
        for _, entry := range entries {
            byParticipant[entry.ParticipantID] = entry
            if entry.Rank == 1 && entry.Score > 0 {
                winnerNames = append(winnerNames, entry.DisplayName)
                winnerScore = entry.Score
            }
        }
        hasWinner := len(winnerNames) > 0

        recipients := make([]FinalRecipient, 0, len(participants))
        for _, p := range participants {
            switch p.Role {
            case RoleSpectator:
                recipients = append(recipients, FinalRecipient{Phone: p.Phone, IsSpectator: true})
            case RolePlayer:
                entry, ok := byParticipant[p.ID]
                if !ok {
                    // GetLeaderboard LEFT JOINs every role='player' row in
                    // the game, so a player missing from it is an invariant
                    // violation, not a scoreless player (a scoreless player
                    // is present with Score 0). Every other anomaly in this
                    // function fails loudly rather than dispatching
                    // something wrong, and a zero rank would reach a real
                    // phone - so this does too. Same posture as
                    // ResultsForRevealedQuestion's own missing-rank guard
                    // (code review, story 3.8).
                    return FinalResults{}, fmt.Errorf("game: player %s of game %s is absent from the leaderboard", p.ID, gameID)
                }
                recipients = append(recipients, FinalRecipient{
                    Phone:    p.Phone,
                    IsWinner: hasWinner && entry.Rank == 1 && entry.Score > 0,
                    Rank:     entry.Rank,
                    Score:    entry.Score,
                })
            default:
                // participants_role's CHECK constraint (migration 00008)
                // guarantees this never happens for a real row - fail
                // closed rather than guess which template applies.
                return FinalResults{}, fmt.Errorf("game: participant %s of game %s has unrecognized role %q", p.ID, gameID, p.Role)
            }
        }

        return FinalResults{Recipients: recipients, WinnerNames: winnerNames, WinnerScore: winnerScore}, nil
    }
    ```
  - [x] **No change to `engine.go`.** The `Store` interface already declares both `ListParticipants` and `GetLeaderboard`; `final.go` is purely additive and adds no store method. Confirm this before touching `engine.go` for any reason — if you are editing it, you have added an unnecessary store call.

- [x] **Task 4: `wa/final_notifier.go` (new file) — one (or two) messages per recipient** (AC: 1, 2, 3)

  **🚨 Same zero-Hebrew rule as Task 3.**

  - [x] New file, mirroring `result_notifier.go`'s shape exactly:
    ```go
    package wa

    import (
        "log/slog"
        "strings"

        "github.com/avraham-shor/whatsapp-clickers/internal/game"
    )

    // FinalNotifier turns a GameFinished transition into the game-end
    // WhatsApp burst (FR-6 game-end, FR-18, story 3.9): one final-results
    // message to every Participant and every Spectator, plus a second,
    // personal winner message to each winner. Wraps a Replier - the same
    // non-blocking Enqueue every other outbound path in this package already
    // uses - so DispatchGameFinished never itself blocks its caller (NFR-2).
    type FinalNotifier struct {
        replier Replier
        logger  *slog.Logger
    }

    // NewFinalNotifier builds a FinalNotifier. logger may be nil, in which
    // case slog.Default() is used (matches NewResultNotifier/
    // NewQuestionNotifier/NewDispatcher).
    func NewFinalNotifier(replier Replier, logger *slog.Logger) *FinalNotifier {
        if logger == nil {
            logger = slog.Default()
        }
        return &FinalNotifier{replier: replier, logger: logger}
    }

    // DispatchGameFinished composes and enqueues the game-end burst.
    //
    // Two passes, deliberately: every recipient's final-results message is
    // enqueued before any winner message. The queue is FIFO and this is the
    // only ordering this layer can express - the 8-worker Dispatcher
    // provides no per-recipient delivery ordering (a known, accepted gap
    // logged in deferred-work.md at story 3.8), so a winner can still see
    // the winner message land first. Enqueueing in the intended order costs
    // nothing and is what makes the common case read correctly.
    //
    // An empty results.WinnerNames means no participant finished with a
    // positive score (see game.FinalResults): everyone gets
    // msgFinalResultsNoWinner and the winner pass does not run at all -
    // results.Recipients cannot contain an IsWinner entry in that case, but
    // the explicit hasWinner guard keeps the two facts from having to agree
    // by accident.
    func (n *FinalNotifier) DispatchGameFinished(gameID string, results game.FinalResults) {
        hasWinner := len(results.WinnerNames) > 0
        tie := len(results.WinnerNames) > 1

        dispatched := 0
        for _, r := range results.Recipients {
            trimmed := strings.TrimSpace(r.Phone)
            if trimmed == "" {
                n.logger.Warn("final results dispatch: skipping recipient with a blank phone", "game_id", gameID)
                continue
            }
            var body string
            switch {
            case !hasWinner:
                body = finalResultsNoWinnerMessage()
            case r.IsSpectator && tie:
                body = finalResultsSpectatorTieMessage(results.WinnerNames, results.WinnerScore)
            case r.IsSpectator:
                body = finalResultsSpectatorMessage(results.WinnerNames, results.WinnerScore)
            case tie:
                body = finalResultsTieMessage(results.WinnerNames, results.WinnerScore, r.Rank, r.Score)
            default:
                body = finalResultsMessage(results.WinnerNames, results.WinnerScore, r.Rank, r.Score)
            }
            n.replier.Enqueue(trimmed, body)
            dispatched++
        }

        winners := 0
        if hasWinner {
            var body string
            if tie {
                body = winnerFinalTieMessage(results.WinnerNames, results.WinnerScore)
            } else {
                body = winnerFinalMessage(results.WinnerNames, results.WinnerScore)
            }
            for _, r := range results.Recipients {
                if !r.IsWinner {
                    continue
                }
                trimmed := strings.TrimSpace(r.Phone)
                if trimmed == "" {
                    // Already warned about in the pass above; skip silently
                    // rather than log the same recipient twice.
                    continue
                }
                n.replier.Enqueue(trimmed, body)
                winners++
            }
        }

        n.logger.Info("final results dispatch enqueued",
            "game_id", gameID, "recipient_count", dispatched, "winner_count", winners, "has_winner", hasWinner)
    }
    ```

- [x] **Task 5: `httpapi/control.go` — dispatch on BOTH transitions into `finished`** (AC: 1, 2, 3)

  **🚨 Same zero-Hebrew rule.**

  - [x] New interface (after `ResultDispatcher`):
    ```go
    // FinalDispatcher is the WhatsApp fan-out surface handleNextQuestion and
    // handleStopGame need at game end; *wa.FinalNotifier satisfies it.
    // Consumer-defined here, referencing only game types, so httpapi never
    // imports wa - same posture as QuestionDispatcher/ResultDispatcher.
    type FinalDispatcher interface {
        DispatchGameFinished(gameID string, results game.FinalResults)
    }
    ```
  - [x] `ControlEngine` interface gains one line (after `ResultsForRevealedQuestion`):
    ```go
    ResultsForFinishedGame(ctx context.Context, gameID string) (game.FinalResults, error)
    ```
  - [x] New function, mirroring `dispatchAnswerRevealed`'s shape/timing:
    ```go
    // dispatchGameFinished hands the WhatsApp game-end burst to dispatcher
    // when snapshot reflects a game that just finished. Spawned as a
    // goroutine after the HTTP response is written (see handleNextQuestion/
    // handleStopGame) - ResultsForFinishedGame is a real DB round trip
    // (roster + leaderboard), and gating the organizer-facing response on it would
    // mean the last "next question" or a "stop" click could visibly stall
    // for up to this function's own timeout (same reasoning as
    // dispatchQuestionOpened, story 3.2).
    //
    // Guards on snapshot.State == game.StateFinished and nothing else.
    // Unlike story 3.8's dispatchAnswerRevealed, this needs no defence
    // against a degraded snapshot: emptySnapshot preserves State = g.State,
    // and State is the ONLY snapshot field this path reads - the whole data
    // set is re-resolved from the DB by ResultsForFinishedGame(gameID). A
    // degraded post-commit snapshot therefore costs this dispatch nothing.
    //
    // Reached from both transitions into finished: NextQuestion past the
    // last question, and StopGame. Neither can fire twice for one game -
    // NextQuestion requires state revealed and StopGame requires
    // question_open/question_closed/revealed, so once a game is finished
    // every route into finished is closed, and finished is terminal. There
    // is no double-dispatch to guard against and no idempotency key needed.
    //
    // Detached from ctx's cancellation with its own bounded timeout - same
    // reasoning as dispatchQuestionOpened/dispatchAnswerRevealed: the
    // transition already committed and its WS snapshot already broadcast by
    // the time this runs, so the organizer's connection closing must not
    // skip delivering the closing message to the whole room.
    func dispatchGameFinished(ctx context.Context, engine ControlEngine, dispatcher FinalDispatcher, gameID string, snapshot game.Snapshot) {
        if dispatcher == nil || snapshot.State != game.StateFinished {
            return
        }
        dispatchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
        defer cancel()
        results, err := engine.ResultsForFinishedGame(dispatchCtx, gameID)
        if err != nil {
            slog.Error("final results dispatch skipped, could not resolve final results", "game_id", gameID, "error", err)
            return
        }
        dispatcher.DispatchGameFinished(gameID, results)
    }
    ```
  - [x] `handleNextQuestion` gains a fourth parameter and a second dispatch goroutine. The two guards are mutually exclusive (`question_open` vs `finished`), so exactly one of them ever does work — that is the point, not a redundancy to collapse:
    ```go
    func handleNextQuestion(engine ControlEngine, hub SnapshotBroadcaster, dispatcher QuestionDispatcher, finalDispatcher FinalDispatcher) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            // ... unchanged through writeJSON ...
            writeJSON(w, http.StatusOK, snapshot)
            go dispatchQuestionOpened(ctx, engine, dispatcher, gameID, snapshot)
            go dispatchGameFinished(ctx, engine, finalDispatcher, gameID, snapshot)
        }
    }
    ```
  - [x] `handleStopGame` gains a third parameter and the dispatch goroutine, bringing it to the same shape as every other dispatching handler:
    ```go
    func handleStopGame(engine ControlEngine, hub SnapshotBroadcaster, finalDispatcher FinalDispatcher) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            // ... unchanged through writeJSON ...
            writeJSON(w, http.StatusOK, snapshot)
            go dispatchGameFinished(ctx, engine, finalDispatcher, gameID, snapshot)
        }
    }
    ```

- [x] **Task 6: wire `finalDispatcher` through `router.go` and `main.go`** (AC: 1, 2, 3)
  - [x] `router.go`'s `NewRouter` gains an 11th, trailing parameter (the precedent story 3.2 set with the 9th and 3.8 repeated with the 10th):
    ```go
    func NewRouter(db Pinger, authSvc AuthService, games GameStore, static fs.FS, webhook http.Handler, engine ControlEngine, hub SnapshotBroadcaster, wsHandler http.Handler, dispatcher QuestionDispatcher, resultDispatcher ResultDispatcher, finalDispatcher FinalDispatcher) http.Handler {
    ```
    Extend the doc comment's dispatcher paragraph: "finalDispatcher is independently nilable the same way — it only affects whether /next-question and /stop also dispatch WhatsApp game-end messages, never whether any route exists." Update two mount lines inside the existing `if engine != nil && hub != nil` block:
    ```go
    gr.Post("/next-question", handleNextQuestion(engine, hub, dispatcher, finalDispatcher))
    gr.Post("/stop", handleStopGame(engine, hub, finalDispatcher))
    ```
  - [x] `main.go`: add right after `resultNotifier := wa.NewResultNotifier(dispatcher, logger)`:
    ```go
    finalNotifier := wa.NewFinalNotifier(dispatcher, logger)
    ```
    Update the `NewRouter` call:
    ```go
    router := httpapi.NewRouter(st, authSvc, st, webdist.FS(), webhookHandler, engine, hub, wsHandler, questionNotifier, resultNotifier, finalNotifier)
    ```
    The "st satisfies game.Store (...)" comment needs **no** change — this story adds no store method.

- [x] **Task 7: update every existing call site for the signature changes** (AC: 1, 2, 3)
  - [x] `server/internal/httpapi/control_test.go`'s `stubControlEngine`: add `ResultsForFinishedGame` (required to keep satisfying the widened `ControlEngine` interface), following the exact `resultsForRevealedQuestion*` mutex-guarded pattern — this method also runs from a goroutine spawned post-response, and this file's own convention (`PlayerRecipientsRequestedFor`, and 3.8's review-added accessors) is that tests read these fields **through a copy-under-lock accessor**, never directly:
    ```go
    resultsForFinishedGameMu           sync.Mutex
    resultsForFinishedGameResult       game.FinalResults
    resultsForFinishedGameErr          error
    resultsForFinishedGameRequestedFor []string
    resultsForFinishedGameDone         chan struct{}
    ```
    ```go
    func (s *stubControlEngine) ResultsForFinishedGame(ctx context.Context, gameID string) (game.FinalResults, error) {
        s.resultsForFinishedGameMu.Lock()
        s.resultsForFinishedGameRequestedFor = append(s.resultsForFinishedGameRequestedFor, gameID)
        result, err := s.resultsForFinishedGameResult, s.resultsForFinishedGameErr
        s.resultsForFinishedGameMu.Unlock()
        if s.resultsForFinishedGameDone != nil {
            s.resultsForFinishedGameDone <- struct{}{}
        }
        return result, err
    }

    // ResultsForFinishedGameRequestedFor returns a thread-safe snapshot of
    // every gameID ResultsForFinishedGame was called with.
    func (s *stubControlEngine) ResultsForFinishedGameRequestedFor() []string {
        s.resultsForFinishedGameMu.Lock()
        defer s.resultsForFinishedGameMu.Unlock()
        return append([]string(nil), s.resultsForFinishedGameRequestedFor...)
    }
    ```
  - [x] New `stubFinalDispatcher` alongside `stubQuestionDispatcher`/`stubResultDispatcher` (same file they already live in), same shape:
    ```go
    type stubFinalDispatcher struct {
        mu    sync.Mutex
        calls []finalDispatchCall
        done  chan struct{}
    }

    type finalDispatchCall struct {
        gameID  string
        results game.FinalResults
    }

    func (s *stubFinalDispatcher) DispatchGameFinished(gameID string, results game.FinalResults) {
        s.mu.Lock()
        s.calls = append(s.calls, finalDispatchCall{gameID, results})
        s.mu.Unlock()
        if s.done != nil {
            s.done <- struct{}{}
        }
    }

    func (s *stubFinalDispatcher) Calls() []finalDispatchCall {
        s.mu.Lock()
        defer s.mu.Unlock()
        return append([]finalDispatchCall(nil), s.calls...)
    }
    ```
  - [x] Add a `controlRouterWithFinalDispatcher(engine, hub, finalDispatcher)` helper mirroring the existing `controlRouterWithResultDispatcher` (`control_test.go:267`).
  - [x] **Every existing `NewRouter(...)` call gains an 11th trailing argument.** Purely mechanical: every 10-argument call becomes 11 by appending `nil` (or this story's stub, only where a test exercises final dispatch). Current call sites, all of which pass 10 today:
    - `server/cmd/server/main.go:209` — becomes `..., resultNotifier, finalNotifier)` (Task 6, not a mechanical `nil`).
    - `server/internal/httpapi/control_test.go` — **5** calls: lines 254, 261, 269 (the three `controlRouter*` helpers) and 340, 516 (inline, both inside "without session" tests). Helpers 254/261/269 gain a trailing `nil`; the new helper from the bullet above passes the stub.
    - `server/internal/httpapi/games_test.go` lines 142, 331 — trailing `nil`.
    - `server/internal/httpapi/packages_test.go` line 81 — trailing `nil`.
    - `server/internal/httpapi/router_test.go` — **25** calls, all `nil, nil, nil, nil, nil` today — trailing `nil`.
    That is 33 test call sites plus `main.go`. No other production call site exists.
  - [x] **No `game/engine_test.go` change.** This story adds no `Store` method, so `stubStore` still satisfies the interface unchanged — a diff there means you added a store call you did not need.

- [x] **Task 8: new tests** (AC: 1, 2, 3)
  - [x] New `server/internal/game/final_test.go` (co-located `game` package test — reuses `stubStore`/fixtures already in `engine_test.go`, same package):
    - `TestResultsForFinishedGameSingleWinnerNamesOnlyTheTopScorer` — 4 players, distinct scores → `WinnerNames` has exactly one name, `WinnerScore` is the top score, and exactly one recipient has `IsWinner: true`.
    - `TestResultsForFinishedGameTiedWinnersNamesAllTiedAtRankOne` — 2 players tied at the top plus a third below → `WinnerNames` has both names in leaderboard order, both recipients `IsWinner: true`, the third not (epic AC-2).
    - `TestResultsForFinishedGameEveryPlayerGetsOwnRankAndScore` — each recipient's `Rank`/`Score` match `RankLeaderboard`'s output for that participant, including a shared rank on a tie (1,1,3 — never 1,1,2).
    - `TestResultsForFinishedGameSpectatorsAreRecipientsWithNoRankOrScore` — a `role='spectator'` participant present in `ListParticipants` but absent from `GetLeaderboard` → present in `Recipients` with `IsSpectator: true, Rank: 0, Score: 0, IsWinner: false`, and **no error** (this is the case a naive missing-from-leaderboard guard would wrongly reject).
    - `TestResultsForFinishedGameAllScoresZeroYieldsNoWinner` — every player at score 0 (the stopped-before-Reveal case) → `WinnerNames` empty, `WinnerScore` 0, **no** recipient has `IsWinner: true`, and `Recipients` is still fully populated.
    - `TestResultsForFinishedGameNoParticipantsReturnsEmptyNonNilSlices` — empty roster → `Recipients` is `[]FinalRecipient{}` and `WinnerNames` is `[]string{}`, neither nil, no panic.
    - `TestResultsForFinishedGamePlayerAbsentFromLeaderboardReturnsError` — a `role='player'` participant with no `GetLeaderboard` row → non-nil error (the invariant guard), distinguishing it from the spectator case above.
    - `TestResultsForFinishedGameUnknownRoleReturnsError` — `Role: "organizer"` → non-nil error, not a silent spectator.
    - `TestResultsForFinishedGamePropagatesStoreErrors` — table-driven over `ListParticipants` and `GetLeaderboard` each erroring → the error surfaces unwrapped.
  - [x] New tests in `server/internal/wa/messages_he_test.go` — extend the canonical-copy `const` block with **seven** new rows (`canonicalFinalResultsCopy`, `…TieCopy`, `…SpectatorCopy`, `…SpectatorTieCopy`, `canonicalFinalResultsNoWinnerCopy`, `canonicalWinnerFinalCopy`, `canonicalWinnerFinalTieCopy`) and add `MatchesCanonicalCopy` + `IsolatesDigitTokens` pairs for each, following the file's own pattern exactly (see `TestResultCorrectBonusMessageMatchesCanonicalCopy`/`…IsolatesDigitTokens`). Plus:
    - `TestJoinNamesSingleNameHasNoConjunction`
    - `TestJoinNamesTwoNamesUseTheConjunction`
    - `TestJoinNamesThreeNamesCommaSeparateAllButTheLast`
    - `TestJoinNamesIsolatesEveryName` — **use Latin-script fixtures** (e.g. `"David Cohen"`, `"Rachel Levi"`), mirroring `TestWelcomeMessageIsolatesLTRTokens`'s stated reasoning: a pure-Hebrew fixture "looks right" whether or not the isolate is there, so it cannot catch a missing one. Assert each name appears wrapped in `lriMark`/`pdiMark` and that the conjunction sits **outside** the isolates.
    - `TestFinalResultsNoWinnerMessageCarriesNoTrophy` — asserts the no-winner copy contains no 🏆, pinning EXPERIENCE.md's trophy discipline against a well-meaning future edit.
  - [x] New `server/internal/wa/final_notifier_test.go`, mirroring `result_notifier_test.go`'s structure (`stubReplier`-based):
    - `TestDispatchGameFinishedPlayerGetsFinalResultsWithOwnRankAndScore`
    - `TestDispatchGameFinishedSpectatorGetsWinnerLineOnly` — asserts the spectator body equals `finalResultsSpectatorMessage(...)` and, specifically, does **not** contain the personal-placing line.
    - `TestDispatchGameFinishedWinnerGetsBothMessagesFinalResultsFirst` — a sole winner receives **exactly two** enqueues, and the recorded order is final-results then winner message (epic AC-3's "additionally").
    - `TestDispatchGameFinishedTieUsesPluralFormsForEveryoneAndBothWinners` — two tied winners: every recipient's final-results body uses the tie template, and both winners get the jointly-addressed tie winner message (epic AC-2 + AC-3).
    - `TestDispatchGameFinishedNoWinnerSendsNoWinnerCopyAndZeroWinnerMessages` — empty `WinnerNames` → every recipient (player and spectator alike) gets `finalResultsNoWinnerMessage()`, and the total enqueue count equals the recipient count exactly (proves the winner pass did not run).
    - `TestDispatchGameFinishedSkipsBlankPhoneRecipientAndTrimsWhitespace` — mirrors `TestDispatchAnswerRevealedSkipsBlankPhoneRecipientAndTrimsWhitespace`; also asserts a blank-phone **winner** is skipped in both passes and does not double-log.
    - `TestDispatchGameFinishedEmptyRecipientsEnqueuesNothing`
  - [x] New tests in `server/internal/httpapi/control_test.go`, in a new section "Final results dispatch (story 3.9)":
    - `TestNextQuestionPastLastQuestionDispatchesFinalResults` — a `nextQuestionSnapshot` with `State: "finished"` → exactly one `stubFinalDispatcher` call carrying the engine's `resultsForFinishedGameResult`, and `ResultsForFinishedGameRequestedFor()` is `[testGameID]`.
    - `TestNextQuestionToAnotherQuestionDoesNotDispatchFinalResults` — `State: "question_open"` → the final dispatcher is never called (and the question dispatcher still is, unchanged).
    - `TestStopGameDispatchesFinalResults` — `stopGameSnapshot` with `State: "finished"` → exactly one final-dispatcher call. **This is the AC-1 case the natural end-of-game path does not cover.**
    - `TestFinalResultsDispatchSurvivesDegradedSnapshot` — a snapshot with `State: "finished"` and a nil `CurrentQuestion` / empty `Participants` (i.e. `emptySnapshot`'s shape) still dispatches — pins the design note that `State` is the only snapshot field this path reads, so story 3.8's degraded-snapshot failure mode cannot recur here.
    - `TestFinalResultsDispatchSkippedOnEngineError` — `resultsForFinishedGameErr` set → `POST /stop` still returns 200 (already-committed transition), the dispatcher is never called.
    - `TestOtherControlActionsNeverDispatchFinalResults` — `StartGame`/`CloseQuestion`/`Reveal` never call `ResultsForFinishedGame` (read the stub via `ResultsForFinishedGameRequestedFor()`, **not** the raw field).
  - [x] Read `control_test.go`'s existing `TestOtherControlActionsNeverDispatch` doc comment and correct it if it still claims `handleStopGame` accepts no dispatcher — it now accepts a `FinalDispatcher`. (Story 3.8's review already reworded this once for `handleReveal`; the same sentence needs the same treatment.)

- [x] **Task 9: Quality gates + local E2E** (all ACs)
  - [x] **Copy-centralization gate first, and run it locally before pushing** — this is the gate that turned CI red on story 3.8 (commit `65fafe9`). Reproduce it verbatim:
    ```bash
    cd server && LC_ALL=C.UTF-8 grep -rlP '[\x{0590}-\x{05FF}\x{FB1D}-\x{FB4F}]' --include='*.go' . \
      | grep -v '_test\.go$' | grep -v '^\./internal/wa/messages_he\.go$'
    ```
    Any output is a build failure. Expected output: **nothing**. Comments count — describe copy in English and name the template identifier instead of quoting it.
  - [x] Other local gates: `gofmt -l .` (the CRLF-checkout caveat from 3.4–3.8 still applies — verify only files this story touches, and confirm any flag is byte-identical once line endings are normalized) · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` diff — **must be empty**, this story adds no query. No `web/` changes: no `tsc`/`eslint` run needed.
  - [x] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1–3.8 — including the fake-provider `WHATSAPP_API_BASE_URL` override `main.go` documents, so sent bodies are inspectable). **Two scenarios, both required** — reset the fake provider's captured-message store between them (3.8's Debug Log records losing an hour to exactly that omission):

    **Scenario A — natural end, tie for first, spectator present.** One 2-question game. Join **4 players** (A, B, C, D) in the lobby, `POST /start`. On Q1: A and B answer correctly, C and D wrong. `close-question` → `reveal`. **Now have a 5th phone (E) send `JOIN` mid-game** so E registers as a real `role='spectator'` row (story 2.5's path — do not fabricate a spectator by direct insert; the point is to prove the roster path works end to end). `next-question` → Q2. Arrange Q2 so **A and B finish exactly tied at the top** (e.g. both correct with the same bonus tier is not achievable — instead give A the Q1 first-bonus and B the Q2 first-bonus, and verify the DB totals are equal before revealing). `close-question` → `reveal` → `next-question` (past the last question → `finished`). Then assert against the captured bodies:
    - A and B each received **two** messages: the tie final-results body (plural `הזוכים`, both names, joined score) and the tie winner body (plural `ניצחתם`).
    - C and D each received **one** message: the tie final-results body with **their own** rank and score in the personal line — cross-check both numbers against a direct `GetLeaderboard` + `RankLeaderboard` call, not against a plausible-looking guess (same discipline 3.7/3.8's E2Es used).
    - **E (spectator) received exactly one** message: the spectator tie body — winner line only, **no** personal-placing line. This is the only assertion in the whole suite that proves AC-1's "and every Spectator" against the real roster.
    - Nobody received a no-winner body.

    **Scenario B — stopped before any Reveal (the no-winner path).** A fresh scratch game, 3 players joined, `POST /start`, **no answers, no close, no reveal** → `POST /stop`. Assert: all 3 players received exactly one message each, all three bodies equal `msgFinalResultsNoWinner`, and **zero** winner messages were sent. This is the only assertion that proves an aborted game does not congratulate the whole room on winning with zero points — a unit test proves the engine's branch, this proves the wiring reaches it from the real `/stop` route.
    - Clean up both scratch game rows afterward (cascades); delete the harness — same convention as every prior story.

## Dev Notes

### Architecture guardrails (violations = rework)

- **🚨 The Hebrew-in-comments gate is the single most likely way this story breaks CI.** `.github/workflows/*.yml`'s "Hebrew copy centralization" step greps the Hebrew Unicode block (U+0590–05FF and U+FB1D–FB4F) across every `.go` file, excludes only `*_test.go` and exactly `./internal/wa/messages_he.go`, and **cannot tell a comment from a string literal**. Story 3.8 shipped three Hebrew doc-comment quotes in `game/results.go` and needed a follow-up commit (`65fafe9`) to unbreak the build. This story writes two brand-new non-test files (`game/final.go`, `wa/final_notifier.go`) whose entire subject matter is Hebrew copy — the temptation to quote it in a doc comment is maximal. Name the identifier (`msgFinalResultsNoWinner`) and describe the copy in English. Run the grep locally before pushing (Task 9).
- **Dependency direction unaffected**: `final.go` lives in `game` and imports only `context` + `fmt` — it does not even need `store/gen` (unlike `results.go`), because `ListParticipants`/`GetLeaderboard` return types it already handles. `wa/final_notifier.go` imports `game` for `game.FinalResults`, exactly as `result_notifier.go` already imports it for `game.PersonalResult`. `game` still never imports `wa`/`httpapi`/`ws`.
- **No new persistence surface at all.** No migration, no `queries/*.sql` edit, no `store/*.go` edit, no `sqlc generate` diff, no `game.Store` interface line. This story is pure composition over data stories 1.4, 2.4, 2.5, 3.7 and 3.8 already persist. `sqlc generate` producing a diff means you went off-spec.
- **NFR-2 ("never block the game loop") — no new concern**: one more DB-read-then-dispatch step, structurally identical to `dispatchQuestionOpened` (3.2) and `dispatchAnswerRevealed` (3.8): goroutine spawned after the HTTP response is written, own bounded timeout, detached from request cancellation.
- **`finished` is terminal and single-entry — no idempotency machinery needed.** `NextQuestion` requires `revealed`; `StopGame` requires `question_open`/`question_closed`/`revealed`. Once a game is `finished`, both refuse, so the transition into `finished` happens at most once per game and `dispatchGameFinished` fires at most once. Do not add a "already dispatched" guard, a sent-marker column, or a mutex — there is nothing to guard.

### Existing code this story modifies — current state, and what must survive

- **[server/internal/httpapi/control.go](server/internal/httpapi/control.go)** — `handleStopGame` currently takes `(engine, hub)`, two args, and spawns **no** goroutine at all; this story makes it three args with one goroutine. `handleNextQuestion` currently takes three args and spawns one goroutine (`dispatchQuestionOpened`); this story makes it four args with two goroutines. `handleReveal`/`handleStartGame`/`handleCloseQuestion`/`handleOpenLobby` are untouched. `dispatchQuestionOpened`'s existing `snapshot.State != game.StateQuestionOpen` guard is what keeps the two `handleNextQuestion` goroutines from both acting — do not "optimize" it into an if/else in the handler; the guards belong with the dispatch functions, where every other one in this file lives.
- **[server/internal/httpapi/router.go](server/internal/httpapi/router.go)** — `NewRouter` currently takes 10 positional args (3.2 added the 9th, 3.8 the 10th, both in exactly this way). Repeat the precedent: one more trailing param. Both `/next-question` and `/stop` mount lines change; the other four control routes do not.
- **[server/internal/wa/messages_he.go](server/internal/wa/messages_he.go)** — purely additive except the header "current map" comment (two existing `-> story 3.9` lines gain `(below)`, two new rows appended). Every existing template/function is untouched. This story adds `strings` to the imports — the first non-`fmt`/`strconv` import in the file.
- **[server/cmd/server/main.go](server/cmd/server/main.go)** — one new line after `resultNotifier`, one changed `NewRouter` call. The `dispatcher` passed to `NewFinalNotifier` is the **same** `*wa.Dispatcher` instance every other notifier already wraps — one queue, one worker pool, one rate limiter for the whole process. Do not construct a second one.
- **[server/internal/game/engine.go](server/internal/game/engine.go)** — **untouched.** `Store` already declares both methods `final.go` calls.
- **[EXPERIENCE.md](_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md)** — two new template rows plus two new message-type rows (Task 1). This is the first story in the epic to edit the UX spec rather than only read it; it does so because `messages_he.go`'s own header makes the table the canonical source and mandates "table first, then here".

### Design decisions worth flagging explicitly

- **Both paths into `finished` dispatch — the natural end AND `StopGame`.** The epic's trigger is "the `GameFinished` event", which is a state fact, not a route fact. A Participant whose organizer pressed "עצור" is owed closure exactly as much as one who reached the last question; leaving them on an unanswered "התקבל ✓" is the "chat goes silent" failure Epic 2 was built to eliminate. This is also why the no-winner template (Task 1) is not an edge case bolted on — it is what the stop-early path *usually* produces.
- **Winner = rank 1 AND score > 0.** Dropping the positivity condition is the single highest-consequence mistake available in this story: `StopGame` from `question_open` leaves every `points_awarded` NULL, `GetLeaderboard` returns every player at 0, and `RankLeaderboard` assigns all of them rank 1 — so a bare `Rank == 1` test congratulates an entire 60-person room on winning with zero points, in the message that is supposed to be the event's emotional close. The empty-`WinnerNames` return is that case's explicit signal.
- **Spectators are recipients, not a filtered-out role.** `PlayerRecipients` (story 3.2) exists precisely to *exclude* spectators from question delivery; this story is the one place that must not. Do not reach for `PlayerRecipients` here — `ResultsForFinishedGame` calls `ListParticipants` directly and branches on `p.Role`, which is also why it needs no new store method.
- **A spectator's absence from `GetLeaderboard` is correct, not a bug.** `GetLeaderboard`'s SQL filters `p.role = 'player'` ([answers.sql:291](server/internal/store/queries/answers.sql)). A player absent from it *is* an invariant violation (the query LEFT JOINs, so a scoreless player appears with score 0). `ResultsForFinishedGame` must therefore treat "missing from leaderboard" as an error for players and as normal for spectators — one map lookup, two different meanings, decided by `p.Role`. Getting this backwards fails every game with a spectator in it.
- **Winners get two messages, final-results first.** Epic AC-3 says "additionally", and EXPERIENCE.md lists them as two distinct table rows with different audiences. Do not merge them into one body. `DispatchGameFinished` enqueues in two full passes so the intended order is expressed at the queue; actual delivery order across the 8-worker dispatcher is not guaranteed (a known gap deferred at story 3.8's review — see deferred-work.md's "next-question burst can overtake the reveal-result burst" entry, which is the same class). Do not attempt to solve per-recipient ordering here; it is a dispatcher-level concern and a story of its own.
- **`dispatchGameFinished` reads only `snapshot.State`.** That is deliberate and worth preserving: story 3.8's original design read `snapshot.CurrentQuestion.Position` and silently lost the entire fan-out whenever `snapshotAfterCommit` degraded to `emptySnapshot` — unrecoverable, because the transition could not be re-run. `emptySnapshot` preserves `State`, and everything else this path needs is re-resolved from the DB by `ResultsForFinishedGame(gameID)`, so the same failure mode is structurally impossible here. Do not "enrich" this function by pulling `snapshot.Leaderboard` or `snapshot.Participants` in to save a query — that reintroduces exactly the bug 3.8's review spent a decision on.
- **`ResultsForFinishedGame` takes no `organizerID`.** `ListParticipants` and `GetLeaderboard` are both organizer-unscoped, `finished` is terminal so no concurrent transition can move the game underneath the read, and every call site has already validated ownership via the transition that immediately preceded it — the same trust posture `PlayerRecipients` documents. Adding an `organizerID` parameter would require `GetGameForOrganizer` for no gain.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, same conventions as every prior story in this epic. `ResultsForFinishedGame`'s tests live in `game` (new `final_test.go`, reusing `engine_test.go`'s `stubStore`/fixtures in the same package — no new test-only package). `wa` tests follow `messages_he_test.go` (canonical-copy constants + isolate-stripping) and `result_notifier_test.go` (`stubReplier`-based dispatch) exactly. `httpapi` tests extend the existing `stubControlEngine` + mutex-guarded-goroutine-signal pattern (`resultsForRevealedQuestionDone` → `resultsForFinishedGameDone`), and **read stub fields only through copy-under-lock accessors** — story 3.8's review flagged a direct field read as a finding precisely because it was benign only by accident of current wiring. No real DB in any unit test. `go test -race` may be unavailable (`CGO_ENABLED=0` in this environment, per 3.8's Change Log) — if so, say so in the Dev Agent Record rather than claiming the goroutine paths are race-verified.

### Project Structure Notes

**New:**
- `server/internal/game/final.go`
- `server/internal/game/final_test.go`
- `server/internal/wa/final_notifier.go`
- `server/internal/wa/final_notifier_test.go`

**Modified:**
- `_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md` (+2 template rows, +2 message-type rows)
- `server/internal/wa/messages_he.go` (+7 templates/functions, +`joinNames`/`nameConjunction`, +`strings` import; header map updated)
- `server/internal/wa/messages_he_test.go` (+7 canonical-copy test pairs, +`joinNames` tests, +no-trophy test)
- `server/internal/httpapi/control.go` (+`FinalDispatcher`, +`dispatchGameFinished`, `handleNextQuestion` gains a 4th param, `handleStopGame` gains a 3rd)
- `server/internal/httpapi/router.go` (`NewRouter` gains an 11th param `finalDispatcher`; `/next-question` and `/stop` mounts updated)
- `server/internal/httpapi/control_test.go` (`stubControlEngine` gains `ResultsForFinishedGame` + accessor; new `stubFinalDispatcher`; new helper; new "Final results dispatch" block; 5 `NewRouter` calls)
- `server/internal/httpapi/router_test.go` (25 `NewRouter` calls), `games_test.go` (2), `packages_test.go` (1) — trailing `nil` only
- `server/cmd/server/main.go` (+`finalNotifier`; `NewRouter` call updated)

**Untouched (a diff here means you went off-spec):** `server/migrations/*` · `server/internal/store/**` including `queries/*.sql` and `gen/*` · `server/internal/game/engine.go` · `server/internal/game/engine_test.go` · `server/internal/game/scoring.go` (this story consumes `RankLeaderboard`, does not change it) · `server/internal/game/results.go` · `server/internal/game/snapshot.go` · `server/internal/ws/*` · `web/*` · `server/internal/wa/question_notifier.go`, `result_notifier.go`, `dispatch.go`, `client.go`, `inbound.go`.

### Latest technical information

No new third-party dependency, no version bump, and no external API surface: this story is stdlib-only Go (`context`, `fmt`, `strings`, `strconv`, `log/slog`) composed over code already on disk. Toolchain pinned at Go **1.26.5** ([server/go.mod](server/go.mod)); `sqlc` at **v1.31.1** (CI-pinned) is invoked only to prove an empty diff. No web research was warranted — nothing here touches the Anthropic SDK (`anthropic-sdk-go v1.61.0`, story 3.6's surface), the Meta Graph API, or any library whose behavior could have drifted.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.9] — story + all 3 epic ACs verbatim, Epic 3 context, FR-6 game-end / FR-18 scope
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#WhatsApp-message-templates] — the Final-results and Winner's-final-message canonical rows, their tie forms, and the A16 joint-address rule
- [Source: …/EXPERIENCE.md#Voice-and-Tone] — emoji discipline (≤1 per message except winner/final-results/speed-bonus, A20); the "🏆 discipline" (trophy only at the winner moment) that governs the no-winner row; gender-neutral Hebrew
- [Source: …/EXPERIENCE.md#WhatsApp-message-types] — "Final results | Game ends; all Participants + Spectators"; "Winner's final message | Game ends; winner(s) only"; "Late joiner | Spectator notice; final results at game end"
- [Source: …/EXPERIENCE.md#Audience-Display-stages] — A16's "ties stack up to three names" in the **Winner takeover** row, the layout constraint this story reads as display-only
- [Source: _bmad-output/planning-artifacts/prds/prd-whatsapp-clickers-2026-07-08/prd.md#FR-18] — "names the winner at game end everywhere… the final results message in every Participant's WhatsApp"; the out-of-scope note (Participants get their own rank, never the full list)
- [Source: _bmad-output/planning-artifacts/architecture.md] — `GameFinished` in the canonical event list; `game/scoring.go` as FR-17/18's home; dependency direction; centralized-Hebrew-copy rule
- [Source: .github/workflows] — the "Hebrew copy centralization" gate's exact grep, exemptions, and why comments are not exempt
- [Source: server/internal/game/engine.go] — `NextQuestion`/`StopGame` both routing to `store.FinishGame`; `PlayerRecipients`' organizer-unscoped trust posture; `snapshotAfterCommit`/`emptySnapshot`'s degradation behavior
- [Source: server/internal/game/scoring.go] — `RankLeaderboard`'s standard-competition ranks (1,1,3) and join-order tiebreak this story's winner set depends on
- [Source: server/internal/game/results.go] — story 3.8's `ResultsForRevealedQuestion`: the fail-loudly-on-missing-rank posture, the `default:` unknown-value branch, and the doc-comment discipline `final.go` mirrors
- [Source: server/internal/store/queries/answers.sql#GetLeaderboard] — the `p.role = 'player'` filter that makes a spectator's absence correct and a player's absence an invariant violation
- [Source: server/migrations/00008_participants.sql] — `role` CHECK `IN ('player','spectator')`, the invariant `final.go`'s `default:` branch trusts
- [Source: server/internal/wa/result_notifier.go, question_notifier.go, dispatch.go] — `Replier`/notifier shape (nil-logger default, blank-phone skip + WARN, count INFO) `FinalNotifier` mirrors; the 8-worker no-ordering dispatcher
- [Source: server/internal/wa/messages_he.go] — the bidi-isolation rule, the "rows land with the story that sends them" convention, and the two `-> story 3.9` placeholder lines this story fills in
- [Source: server/internal/httpapi/control.go, router.go] — `dispatchAnswerRevealed`'s goroutine/detached-context pattern; `NewRouter`'s 10-arg shape and the 3.2/3.8 precedent for growing it
- [Source: server/internal/httpapi/control_test.go] — `stubControlEngine`/`stubResultDispatcher`/`waitForSignal`/`PlayerRecipientsRequestedFor` patterns this story's stubs extend
- [Source: _bmad-output/implementation-artifacts/3-8-personal-results-after-reveal.md] — previous story's Review Findings (the degraded-snapshot decision, the missing-rank guard, the direct-stub-field-read finding), E2E harness conventions, and the fake-provider reset gotcha
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — the no-per-recipient-ordering dispatcher gap and the untracked-dispatch-goroutine-at-shutdown gap, both of which this story extends rather than introduces

## Dev Agent Record

### Agent Model Used

claude-sonnet-5 (Claude Code, dev-story workflow)

### Debug Log References

- **Branched from `main`, not from 3.8's branch.** By the time this story ran, 3.8 had merged (PR #6, `efe4fc9`). Verified `git merge-base --is-ancestor 65fafe9 origin/main` and that `origin/main`'s tree was byte-identical to the 3.8 branch tip before branching, so the story's "branch from wherever 3.8's work sits" prerequisite resolved to `main`. All seven prerequisite shapes (`NewRouter` at 10 args, `handleStopGame` at 2, `ResultDispatcher`, `ResultNotifier`, the two `-> story 3.9` header lines, `RankLeaderboard`, `Store` already declaring both methods) were re-verified on disk before any code was written.
- **`gofmt` flagged one real issue, not just the known CRLF noise.** The whole-tree `gofmt -l .` flags nearly every file because of the CRLF checkout (the caveat carried since 3.4). To separate signal from noise, each touched file was copied to LF-normalized temporaries and re-checked: 12 of 13 were CRLF-only, but `messages_he_test.go` had genuine drift — my new `const` block widened the longest identifier, so the two pre-existing alignment columns needed re-padding. Fixed; all 13 then clean under LF normalization.
- **The Hebrew copy-centralization gate silently false-passed once.** Running it as `LC_ALL=C.UTF-8 out=$(grep -rlP ...)` applies the prefix to the assignment, not to `grep`, which then errored with "-P supports only unibyte and UTF-8 locales" and produced empty output that *looks* like a pass — exactly the failure mode CI's own inline comment warns about and guards with its `status -gt 1` check. Re-ran with `export LC_ALL=C.UTF-8` and printed the full match list as a positive control: grep really ran, found Hebrew in 18 files, and all 18 are `_test.go` or `messages_he.go`. `game/final.go` and `wa/final_notifier.go` are correctly absent — the zero-Hebrew-comment rule that cost 3.8 a follow-up commit (`65fafe9`) held.
- **E2E harness race, twice — the same class 3.8 logged.** Scenario B initially reported 6 messages instead of 3. Not a product bug: `POST /start` fans its question-delivery burst out from an untracked goroutine, which enqueued *after* the harness cleared its capture, so each player showed one question message plus one final-results message. The `Reset()` between phases is necessary but not sufficient — it must be preceded by a settle. Added a settle before both resets (Scenario A's pre-`next-question` clear had the same latent race against the Q2 reveal burst, and passed only by luck of DB-query timing). Both scenarios then passed clean.
- **`go test -race` was NOT run — the goroutine paths are not race-verified.** `CGO_ENABLED=0` in this environment and `-race` requires cgo; forcing `CGO_ENABLED=1` fails with `cgo: C compiler "gcc" not found`. Recording this rather than implying race coverage, per the story's testing-standards note. The new concurrent surface is `dispatchGameFinished`'s goroutine and `stubControlEngine.ResultsForFinishedGame`; the latter follows the file's mutex-guarded, copy-under-lock-accessor convention exactly, and no test reads a stub field directly.

### Completion Notes List

- **All 3 epic ACs implemented and verified end to end.** AC-1 (every Participant *and* Spectator receives score, rank, winner name) is proven against a real roster by Scenario A's spectator assertion — the only check in the suite that exercises AC-1's "and every Spectator" through story 2.5's real mid-game JOIN path rather than a fabricated row. AC-2 (tie names all included, plural form) and AC-3 (winners *additionally* get the personal winner message) are covered at unit and E2E level.
- **Both transitions into `finished` dispatch.** `handleNextQuestion` (past the last question) and `handleStopGame` both spawn `dispatchGameFinished`. `handleStopGame` previously spawned no goroutine at all and took 2 args; it now takes 3. The two goroutines `handleNextQuestion` spawns are mutually exclusive by their own state guards (`question_open` vs `finished`) — left as two guarded dispatch functions rather than collapsed into an if/else, per the story's explicit instruction that guards live with the dispatch functions.
- **The winner = `Rank == 1 AND Score > 0` condition is load-bearing and directly tested.** Scenario B proves the whole path: a game stopped from `question_open` before any Reveal leaves every `points_awarded` NULL, `GetLeaderboard` returns all players at 0, and `RankLeaderboard` gives all of them rank 1 — a bare `Rank == 1` test would have congratulated the entire room on winning with zero points. Verified: 3 players, 3 messages total, all `msgFinalResultsNoWinner`, zero winner messages, no trophy.
- **A spectator's absence from `GetLeaderboard` is correct; a player's is an error.** One map lookup, two meanings, decided by `p.Role`. Both directions are pinned by dedicated tests (`…SpectatorsAreRecipientsWithNoRankOrScore` expects *no* error; `…PlayerAbsentFromLeaderboardReturnsError` expects one).
- **No new persistence surface, exactly as scoped.** No migration, no `queries/*.sql` edit, no `store/*.go` edit, no `game.Store` interface line, no `web/` change. `sqlc generate` was run with the CI-pinned v1.31.1 and produced an empty diff. `engine.go`, `scoring.go`, `results.go`, `snapshot.go`, `engine_test.go` are untouched.
- **Quality gates, all green:** Hebrew copy-centralization clean (re-verified with a positive control after catching a false pass) · `gofmt` clean on all 13 touched files under LF normalization · `go vet ./...` clean · `go test ./... -count=1` fully green, zero regressions · `sqlc generate` empty diff.
- **Tests added: 37 new test functions.** `game/final_test.go` 9 (single winner, tie, per-player rank/score cross-checked against `RankLeaderboard` for the 1,1,3 shape, spectators, all-zero no-winner, empty roster non-nil slices, missing-player error, unknown-role error, table-driven store-error propagation) · `wa/messages_he_test.go` +21 (7 canonical-copy/isolate pairs, 4 `joinNames` tests with Latin-script fixtures, no-trophy) · `wa/final_notifier_test.go` 7 · `httpapi/control_test.go` 6.
- **Local E2E: 39/39 checks passed** against real local Postgres, both required scenarios, harness deleted and scratch rows cleaned afterward. Scenario A (2-question game, 4 players, real mid-game spectator JOIN, engineered tie via staggered first-place Speed Bonuses): the DB tie was verified before finishing (A=250, B=250, both rank 1); A and B each got exactly 2 messages (plural `הזוכים` final-results + plural `ניצחתם` winner message naming both); C and D each got exactly 1 carrying **their own** rank and score, cross-checked against an independent `GetLeaderboard` + `RankLeaderboard` call rather than a guess; spectator E got exactly 1 winner-line-only message with no personal-placing line; 7 messages total. Scenario B as described above.
- **Deliberately not done, per the story's scope boundaries:** no per-recipient delivery ordering fix (a dispatcher-level concern already deferred in `deferred-work.md`; `DispatchGameFinished` expresses intent by enqueueing in two full passes, which is all this layer can do) · no idempotency guard on `dispatchGameFinished` (`finished` is terminal and single-entry, so there is nothing to guard) · no `organizerID` parameter on `ResultsForFinishedGame`.

### File List

**New:**
- `server/internal/game/final.go`
- `server/internal/game/final_test.go`
- `server/internal/wa/final_notifier.go`
- `server/internal/wa/final_notifier_test.go`

**Modified:**
- `_bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md`
- `server/internal/wa/messages_he.go`
- `server/internal/wa/messages_he_test.go`
- `server/internal/httpapi/control.go`
- `server/internal/httpapi/control_test.go`
- `server/internal/httpapi/router.go`
- `server/internal/httpapi/router_test.go`
- `server/internal/httpapi/games_test.go`
- `server/internal/httpapi/packages_test.go`
- `server/cmd/server/main.go`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

## Change Log

- 2026-08-07: Story created (create-story workflow). Status: ready-for-dev.
- 2026-08-07: Story 3.9 implemented — final results to every Participant and Spectator at game end, tie-aware winner naming, personal winner message, and a no-winner path for games that end without a positive score. Dispatch fires on both transitions into `finished` (`/next-question` past the last question and `/stop`). 4 new files, 11 modified, 37 new tests, 39/39 local E2E checks. All quality gates green; `go test -race` unavailable in this environment (no cgo/gcc). Status: review.
