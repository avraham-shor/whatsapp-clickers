---
baseline_commit: fd637d2
---

# Story 3.2: Question Delivery to Every Phone

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a Participant,
I want each Question to arrive as a WhatsApp message the moment it opens,
so that I can play from any device with WhatsApp (FR-4).

## Acceptance Criteria

1. **Given** the `QuestionOpened` event, **when** dispatch runs, **then** every `player`-role Participant receives the question text — MCQ with lettered options א–ד — including how to answer and the time limit — copy per the templates table Question rows (MCQ / Free-Text); Spectators receive nothing. *(epic AC-1)*

2. **Given** 80 registered Participants, **when** a Question opens, **then** dispatch begins immediately and the burst completes within the 5s/95% target (token-bucket math per architecture: ~1–2s at 80 mps). *(epic AC-2 — the timing math is a property of the pre-existing `wa.Dispatcher` worker pool/rate-limiter from story 2.1, unchanged by this story; this story's job is only to enqueue every recipient's message synchronously and immediately when the transition succeeds, not to re-prove the Dispatcher's throughput.)*

3. **Given** an individual send failure, **then** it retries up to 3 times with backoff, final failure logs WARN, and the game loop is never blocked (NFR-2). *(epic AC-3 — entirely pre-existing `wa.Dispatcher.send` behavior from story 2.1; this story adds no new retry/backoff logic — it only needs to call the existing `Enqueue`.)*

## Tasks / Subtasks

- [x] **Task 1: `game` package — the player-only recipient list** (AC: 1)
  - [x] `server/internal/game/engine.go`: add `PlayerRecipients(ctx context.Context, gameID string) ([]string, error)` to `*Engine`. Calls the already-present `Store.ListParticipants(ctx, gameID)` (no `Store` interface change needed — this method already exists and is already used by `buildSnapshot`), filters to `Role == RolePlayer`, returns phone numbers only (non-nil, possibly-empty slice — same "never null on the wire" discipline as `buildSnapshot`'s participant summaries, even though this return value is never serialized). Unscoped by `organizerID`: every call site (Task 3) has already validated ownership via the state transition that immediately preceded it — identical trust posture to `buildSnapshot`'s own unscoped `ListParticipants` call.
    - Deliberately returns `[]string`, not a new `Recipient{Phone, DisplayName}` struct: nothing in this story's WhatsApp copy is personalized (the Question message is identical for every recipient — see the templates table), so a display-name field would be dead weight. Do not invent it.
  - [x] `server/internal/game/engine_test.go`: no `stubStore` extension needed — `participants`/`participantsErr` already exist and already back `ListParticipants`. Add `TestPlayerRecipients*` cases: a mixed player/spectator roster returns only player phones (order as returned by the store); an all-spectator roster returns an empty non-nil slice; a store error propagates unwrapped.

- [x] **Task 2: `wa` package — Question message copy + dispatch** (AC: 1, 2, 3)
  - [x] `server/internal/wa/messages_he.go`: add the two Question rows, copied **verbatim** from EXPERIENCE.md's "WhatsApp message templates" table (the canonical source — see References):
    ```go
    // msgQuestionMCQTemplate is the Question — MCQ row: question number/total,
    // question text, the four lettered options in fixed order, then the
    // answer-format + time-limit line. Every Western-digit run is isolated via
    // ltr() at the call site — this file's header rule ("mandatory for every
    // template here") is not placeholder-only: the template's own fixed
    // "1–4" digit range is an LTR run embedded in RTL text exactly like a
    // placeholder digit, so it is isolated the same way.
    // [ASSUMPTION — EXPERIENCE.md's table shows the row as plain text with no
    // isolation marks (the rule lives in this file's header, not the table);
    // flag for confirmation if a native-speaker WhatsApp render check reveals
    // this over- or under-isolates.]
    const msgQuestionMCQTemplate = "שאלה %s מתוך %s:\n%s\nא. %s\nב. %s\nג. %s\nד. %s\nהשיבו באות (א–ד) או בספרה (%s) — יש לכם %s שניות!"

    // questionMCQMessage returns the finished MCQ Question copy. number/total
    // are 1-based (number is the question's Position; total is the game's
    // question count). options must have exactly 4 elements — the questions
    // table's own CHECK constraint (questions_type_shape, migration 00003)
    // already guarantees this for every mcq row; not re-validated here
    // (validate at boundaries, trust internal invariants).
    func questionMCQMessage(number, total int, text string, options []string, timeLimitSeconds int) string {
        return fmt.Sprintf(msgQuestionMCQTemplate,
            ltr(strconv.Itoa(number)), ltr(strconv.Itoa(total)), text,
            options[0], options[1], options[2], options[3],
            ltr("1–4"), ltr(strconv.Itoa(timeLimitSeconds)))
    }

    // msgQuestionFreeTextTemplate is the Question — Free-Text row.
    const msgQuestionFreeTextTemplate = "שאלה %s מתוך %s:\n%s\nכתבו את התשובה בהודעה — יש לכם %s שניות!"

    func questionFreeTextMessage(number, total int, text string, timeLimitSeconds int) string {
        return fmt.Sprintf(msgQuestionFreeTextTemplate,
            ltr(strconv.Itoa(number)), ltr(strconv.Itoa(total)), text, ltr(strconv.Itoa(timeLimitSeconds)))
    }
    ```
    Requires adding `"strconv"` to this file's imports (alongside the existing `"fmt"`). Question `text` and MCQ `options` are organizer-authored prose, not short tokens (codes/digits/names) — left un-isolated, consistent with every other template's scope (only JOIN codes, digits, and display names get `ltr()`; free-form sentence content never has).
  - [x] Update this file's header comment map: change `Question - MCQ           -> story 3.2` and `Question - Free-Text     -> story 3.2` to append ` (below)`, matching every other implemented row's format.
  - [x] New `server/internal/wa/question_notifier.go`:
    ```go
    package wa

    import (
        "log/slog"
        "strings"

        "github.com/avraham-shor/whatsapp-clickers/internal/game"
    )

    const (
        questionTypeMCQ      = "mcq"
        questionTypeFreeText = "free_text"
    )

    // QuestionNotifier turns a QuestionOpened transition into one outbound
    // WhatsApp message per player-role Participant (FR-4). Wraps a Replier —
    // the same non-blocking Enqueue every other outbound path in this package
    // already uses — so DispatchQuestionOpened never itself blocks its caller
    // (NFR-2: the game loop must never wait on delivery).
    type QuestionNotifier struct {
        replier Replier
        logger  *slog.Logger
    }

    // NewQuestionNotifier builds a QuestionNotifier. logger may be nil, in
    // which case slog.Default() is used (matches NewDispatcher/NewInboundRouter).
    func NewQuestionNotifier(replier Replier, logger *slog.Logger) *QuestionNotifier {
        if logger == nil {
            logger = slog.Default()
        }
        return &QuestionNotifier{replier: replier, logger: logger}
    }

    // DispatchQuestionOpened composes the question message once — it is
    // identical for every recipient, no per-participant placeholder exists in
    // either template row — then enqueues it for every recipient with a
    // non-blank phone. Called synchronously from httpapi/control.go right
    // after a StartGame/NextQuestion transition commits and its WS snapshot
    // broadcasts; actual delivery happens asynchronously on the Dispatcher's
    // own worker pool (AC-2/AC-3 are its pre-existing behavior, not this
    // method's).
    func (n *QuestionNotifier) DispatchQuestionOpened(gameID string, question game.CurrentQuestion, questionCount int, recipients []string) {
        var body string
        switch question.Type {
        case questionTypeMCQ:
            body = questionMCQMessage(question.Position, questionCount, question.Text, question.Options, question.TimeLimitSeconds)
        case questionTypeFreeText:
            body = questionFreeTextMessage(question.Position, questionCount, question.Text, question.TimeLimitSeconds)
        default:
            // The questions table's CHECK constraint (questions_type_shape)
            // guarantees this never happens for a real row — an invariant
            // violation, so ERROR is correct here (NFR-8: a healthy run
            // produces zero ERRORs).
            n.logger.Error("question dispatch: unknown question type, sending nothing",
                "game_id", gameID, "question_id", question.ID, "type", question.Type)
            return
        }
        dispatched := 0
        for _, phone := range recipients {
            if strings.TrimSpace(phone) == "" {
                continue
            }
            n.replier.Enqueue(phone, body)
            dispatched++
        }
        n.logger.Info("question dispatch enqueued",
            "game_id", gameID, "question_id", question.ID, "question_type", question.Type,
            "recipient_count", dispatched)
    }
    ```
  - [x] `server/internal/wa/messages_he_test.go`: add `canonicalQuestionMCQCopy`/`canonicalQuestionFreeTextCopy` constants (verbatim from EXPERIENCE.md, same `%s`-placeholder style as the existing constants in this file) and `TestQuestionMCQMessageMatchesCanonicalCopy`/`TestQuestionFreeTextMessageMatchesCanonicalCopy` (using `stripIsolates`, the existing helper) plus `TestQuestionMCQMessageIsolatesDigitTokens` asserting the number/total/time-limit/`"1–4"` substrings are each wrapped in `lriMark`/`pdiMark` — mirrors this file's existing per-row test pairs exactly.
  - [x] New `server/internal/wa/question_notifier_test.go`: a `stubReplier` recording `Enqueue` calls (or reuse one if `dispatch_test.go`/`inbound_test.go` already defines an equivalent — check before adding a duplicate). Tests: MCQ and Free-Text questions each enqueue the same composed body to every recipient; a blank-phone recipient in the input is skipped and not counted; an unrecognized `question.Type` enqueues nothing and logs at ERROR (captured `slog.NewTextHandler`, the house pattern per 3.1's Dev Notes); an empty `recipients` slice enqueues nothing and still logs the INFO line with `recipient_count=0`.

- [x] **Task 3: `httpapi` package — wire dispatch into `/start` and `/next-question`** (AC: 1, 2)
  - [x] `server/internal/httpapi/control.go`:
    - Extend `ControlEngine` with `PlayerRecipients(ctx context.Context, gameID string) ([]string, error)` — `*game.Engine` already satisfies it after Task 1.
    - Add `QuestionDispatcher` interface (consumer-defined, same convention as `SnapshotBroadcaster`):
      ```go
      type QuestionDispatcher interface {
          DispatchQuestionOpened(gameID string, question game.CurrentQuestion, questionCount int, recipients []string)
      }
      ```
      `*wa.QuestionNotifier` satisfies it; `httpapi` does not import `wa` to get this — the interface only references `game` types, preserving the existing zero-cross-import boundary between `httpapi` and `wa` (both remain siblings that only depend on `game`; main.go is the sole place that wires the concrete `*wa.QuestionNotifier` into this slot).
    - Add a shared helper (both `handleStartGame` and `handleNextQuestion` need identical logic — don't duplicate it twice):
      ```go
      // dispatchQuestionOpened hands the WhatsApp question-opened burst to
      // dispatcher when snapshot carries a freshly opened question —
      // StartGame/NextQuestion's only success shape that populates
      // CurrentQuestion (CloseQuestion/Reveal/StopGame never do, so this is a
      // correct, sufficient guard with no state-name check needed). A nil
      // dispatcher skips dispatch without touching route-mounting (that gate
      // stays engine != nil && hub != nil, unchanged).
      //
      // Detached from ctx's cancellation with its own bounded timeout — same
      // reasoning as engine.snapshotAfterCommit (story 3.1): the transition
      // already committed and its WS snapshot already broadcast by the time
      // this runs, so an aborted REST request (closed organizer tab, client
      // timeout) must not silently skip delivering the question to everyone
      // else. A recipient-lookup failure degrades to a WARN and skips
      // dispatch — the transition must not be reported as failed for a
      // WhatsApp-side problem after it already committed.
      func dispatchQuestionOpened(ctx context.Context, engine ControlEngine, dispatcher QuestionDispatcher, gameID string, snapshot game.Snapshot) {
          if dispatcher == nil || snapshot.CurrentQuestion == nil {
              return
          }
          dispatchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
          defer cancel()
          recipients, err := engine.PlayerRecipients(dispatchCtx, gameID)
          if err != nil {
              slog.Warn("question dispatch skipped, could not list player recipients", "game_id", gameID, "error", err)
              return
          }
          dispatcher.DispatchQuestionOpened(gameID, *snapshot.CurrentQuestion, snapshot.QuestionCount, recipients)
      }
      ```
    - `handleStartGame` and `handleNextQuestion` each gain a third parameter, `dispatcher QuestionDispatcher`; call `dispatchQuestionOpened(ctx, engine, dispatcher, gameID, snapshot)` immediately after `hub.Broadcast(...)`, before the `slog.Info` line. `handleOpenLobby`, `handleCloseQuestion`, `handleReveal`, `handleStopGame` are untouched (none of their transitions populate `CurrentQuestion` on success).
  - [x] `server/internal/httpapi/router.go`: `NewRouter` gains one new trailing parameter, `dispatcher QuestionDispatcher` (9th positional arg — after `wsHandler`). The route-mounting condition stays exactly `if engine != nil && hub != nil` (unchanged) — `dispatcher` is independently nilable and only affects whether `/start` and `/next-question` also dispatch WhatsApp messages, never whether any route exists. Update the two call sites inside that block:
    ```go
    gr.Post("/start", handleStartGame(engine, hub, dispatcher))
    gr.Post("/next-question", handleNextQuestion(engine, hub, dispatcher))
    ```
    Update `NewRouter`'s doc comment to name the new parameter and its independent nilability.
  - [x] **Mechanical, codebase-wide**: every existing call to `httpapi.NewRouter(...)` gains one new trailing argument. Re-run `grep -rn "NewRouter(" server` to get the current, authoritative line list (do not trust a story-file snapshot of line numbers — they drift). At the time this story was written that was: `server/cmd/server/main.go` (1 call, real wiring — see Task 4), `server/internal/httpapi/control_test.go` (3 calls), `server/internal/httpapi/router_test.go` (~25 calls), `server/internal/httpapi/games_test.go` (2 calls), `server/internal/httpapi/packages_test.go` (1 call). Every one of these except `control_test.go`'s `controlRouter` helper and main.go gets a plain trailing `nil` — none of them exercise control routes, so a nil dispatcher (dispatch skipped) is correct and changes no existing assertion.
  - [x] `server/internal/httpapi/control_test.go`:
    - `controlRouter(engine, hub)` helper: keep its signature unchanged (its 7+ call sites throughout this file must not all need editing); internally pass `nil` for the new `dispatcher` param in its own `NewRouter(...)` call.
    - Add a second helper, `controlRouterWithDispatcher(engine ControlEngine, hub SnapshotBroadcaster, dispatcher QuestionDispatcher) http.Handler`, identical to `controlRouter` but threading a real dispatcher — used only by the new dispatch tests below.
    - Extend `stubControlEngine` with `playerRecipients []string`, `playerRecipientsErr error`, `playerRecipientsRequestedFor []string` (records every `gameID` asked for) + the `PlayerRecipients` method.
    - Add `stubQuestionDispatcher` recording each call's `(gameID, question, questionCount, recipients)`.
    - New tests:
      - `TestStartGameDispatchesQuestionToPlayers` — `startGameSnapshot` carries a non-nil `CurrentQuestion` and `QuestionCount`; `playerRecipients` set to a few phones; assert exactly one `DispatchQuestionOpened` call with the matching `gameID`/question/questionCount/recipients, and exactly one `PlayerRecipients` call for the game.
      - `TestNextQuestionDispatchesQuestionToPlayers` — same shape for `/next-question` when it opens a next question.
      - `TestNextQuestionFinishingGameDoesNotDispatch` — `nextQuestionSnapshot` has `CurrentQuestion == nil` (the finishing branch) → dispatcher never called, `PlayerRecipients` never called.
      - `TestStartGameDispatchSkippedOnRecipientsError` — `playerRecipientsErr` set → handler still returns 200 with the transition's snapshot (already-committed transition is not reported as failed) but `DispatchQuestionOpened` is never called.
      - A quick sanity case that `CloseQuestion`/`Reveal`/`StopGame` never call the dispatcher even when routed through `controlRouterWithDispatcher` (they don't accept a dispatcher param at all, so this is true by construction — one short test is enough to pin it, not a full table).

- [x] **Task 4: `main.go` wiring** (AC: 1)
  - [x] `server/cmd/server/main.go`: after the existing `dispatcher := wa.NewDispatcher(waClient, logger)` line, add `questionNotifier := wa.NewQuestionNotifier(dispatcher, logger)` (same `dispatcher` — it already satisfies `wa.Replier`, exactly like `inboundRouter`'s construction two lines below reuses it). Pass `questionNotifier` as `NewRouter`'s new trailing argument. Update the existing `// st satisfies game.Store (...)` comment: no new `Store` methods this story (leave as-is) — but the `NewRouter` call's own line now has one more argument; no comment update needed there beyond what Task 3 already covers in router.go.

- [x] **Task 5: Quality gates + local E2E** (all ACs)
  - [x] Local gates: `gofmt -l .` (LF-normalize first, documented Windows/CRLF workaround) · `go1.26.5 vet ./...` · `go1.26.5 test ./...` (zero regressions) · `sqlc generate` diff must be **empty** — this story makes no schema change; a non-empty diff means something unrelated leaked in, stop and investigate · `npm run lint` + `npx tsc -b --noEmit` under `web/` (no frontend files change this story, but the gate still runs per every prior story's convention).
  - [x] Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.3–3.1): boot the real binary with `WHATSAPP_API_BASE_URL` pointed at a local fake Meta endpoint that records every outbound `(to, body)` pair (reuse the harness `main.go`'s `clientOpts`/`wa.WithBaseURL` comment references — 2.1/2.2 already built and used one; do not build a second). Create a scratch game with one MCQ question (4 options) and one Free-Text question; join 2 players via the real `JOIN` webhook path; `POST /start`; assert the fake endpoint received exactly 2 sends, both bodies exactly matching `questionMCQMessage`'s expected output for that question ("שאלה 1 מתוך 2..."); `POST /close-question` → `POST /reveal` → assert no new sends; `POST /next-question`; assert exactly 2 more sends matching the Free-Text template ("שאלה 2 מתוך 2..."). Then join a third phone via `JOIN` (game is now past lobby, so it registers as a Spectator) and confirm no message beyond its own JOIN-triggered Spectator-notice reply was ever sent to it. Clean up the scratch game row afterward. Not testing 80-participant timing (AC-2's math is the pre-existing Dispatcher's, not new work) — 2 participants is enough to prove correctness of who gets what.

### Review Findings

Code review run 2026-08-04 (Blind Hunter + Edge Case Hunter + Acceptance Auditor, parallel adversarial layers). 2 decision-needed (both resolved to patch, see below), 8 patch, 4 defer, 7 dismissed as noise.

**Decision needed (resolved 2026-08-04):**

- [x] [Review][Decision] Post-commit DB failures can silently skip WhatsApp dispatch to every player, while the organizer still gets 200 OK — two distinct trigger paths converge on this: (a) `snapshotAfterCommit` degrades to `emptySnapshot` on a transient `buildSnapshot` failure after `StartGame`/`NextQuestion` already committed → `CurrentQuestion` is nil → `dispatchQuestionOpened`'s guard returns with **zero log**, indistinguishable from the legitimate "NextQuestion finished the game" case; (b) `PlayerRecipients` itself errors inside `dispatchQuestionOpened` → a WARN is logged but dispatch is skipped, with no retry and nothing surfaced to the organizer. Either way, the question opened in the DB and on the dashboard, but no phone ever receives it, and nothing distinguishes this from a healthy run except a log line an operator would have to already be watching for. **Decision: log ERROR (not WARN) on every silent-skip path** — small, targeted fix; no retry/typed-error API surfaced now. Moved to Patch below. [server/internal/httpapi/control.go (dispatchQuestionOpened), server/internal/game/engine.go (snapshotAfterCommit, ~L299-308)]
- [x] [Review][Decision] Synchronous `PlayerRecipients` DB lookup runs before the HTTP response is written in `handleStartGame`/`handleNextQuestion`, with its own fresh 5s timeout budget on top of whatever the request context already spent — worst case adds up to 5s of visible latency to the organizer's "start"/"next question" click, in tension with the `QuestionNotifier` doc comment's framing that dispatch "never itself blocks its caller." The `Enqueue` call is genuinely non-blocking; the DB read gating it is not. **Decision: write the HTTP response first, then run `dispatchQuestionOpened` fire-and-forget in a goroutine after `writeJSON`** — the organizer's response is no longer gated on the recipient lookup. Moved to Patch below. [server/internal/httpapi/control.go (dispatchQuestionOpened call sites, handleStartGame/handleNextQuestion)]

**Patch (all applied 2026-08-04):**

- [x] [Review][Patch] (from Decision 1) Log ERROR on every silent-skip path in `dispatchQuestionOpened` — both when `snapshot.CurrentQuestion` is nil in a context where a question should have opened (cannot be distinguished from the legitimate finishing branch without a signal from `snapshotAfterCommit`; at minimum, upgrade the existing recipient-lookup-failure WARN to ERROR) and when `PlayerRecipients` errors. [server/internal/httpapi/control.go]
- [x] [Review][Patch] (from Decision 2) Move `dispatchQuestionOpened` to run after `writeJSON`, fire-and-forget in its own goroutine, in both `handleStartGame` and `handleNextQuestion` — the HTTP response must not wait on the recipient-lookup DB read. [server/internal/httpapi/control.go]

- [x] [Review][Patch] `questionMCQMessage` panics (index out of range) on an MCQ question with fewer than 4 options — no length guard, unlike the sibling unknown-question-type branch in `question_notifier.go` which degrades gracefully with an ERROR log for the identical class of CHECK-constraint-should-guarantee-this violation. [server/internal/wa/messages_he.go (questionMCQMessage)]
- [x] [Review][Patch] `TestOtherControlActionsNeverDispatch` is a vacuous regression test, and the guard it's meant to validate rests on a false invariant — `buildSnapshot` populates `CurrentQuestion` whenever `g.CurrentQuestionPosition > 0`, regardless of `g.State` (confirmed by reading `engine.go` ~L326-342), so a real `CloseQuestion`/`Reveal` snapshot *does* carry a non-nil `CurrentQuestion` today, contrary to this story's own Dev Notes claim that "CloseQuestion/Reveal/StopGame never do [populate CurrentQuestion]." The test's hand-built stub snapshot sets `CurrentQuestion` to nil, so it cannot catch a future accidental dispatcher-wiring regression on those handlers. Fix: harden `dispatchQuestionOpened`'s guard to also check `snapshot.State == game.StateQuestionOpen` (correct by construction instead of by accident-of-wiring), and fix the test stub to reflect real `buildSnapshot` output. [server/internal/httpapi/control.go (dispatchQuestionOpened guard), server/internal/httpapi/control_test.go (TestOtherControlActionsNeverDispatch)]
- [x] [Review][Patch] Blank-phone recipients are skipped with zero logging, and non-blank phones are enqueued untrimmed — a `" +972... "` phone with surrounding whitespace passes the blank check and is enqueued raw. [server/internal/wa/question_notifier.go (DispatchQuestionOpened)]
- [x] [Review][Patch] The story's own Task 2 code snippet specifies an `[ASSUMPTION — EXPERIENCE.md's table shows the row as plain text with no isolation marks...]` flag-for-confirmation comment on `msgQuestionMCQTemplate`; it is missing from the shipped code, breaking traceability for a flagged-but-unconfirmed design assumption the task checkbox claims shipped verbatim. [server/internal/wa/messages_he.go (msgQuestionMCQTemplate)]
- [x] [Review][Patch] `TestPlayerRecipientsStoreErrorPropagates` pins exact error-string equality (`err.Error() != "boom"`), which will break the moment call-site error wrapping (`fmt.Errorf("...: %w", err)`) is added — freezing a bare, context-free store error into the WARN log from finding 1 above. [server/internal/game/engine_test.go (TestPlayerRecipientsStoreErrorPropagates)]
- [x] [Review][Patch] `TestStartGameDispatchesQuestionToPlayers`/`TestNextQuestionDispatchesQuestionToPlayers` assert `len(call.recipients)` only, never the recipient values — a handler that passed the wrong recipient slice (or another game's list) would still pass. [server/internal/httpapi/control_test.go]

**Patch application notes:** `dispatchQuestionOpened` now guards on `snapshot.State == game.StateQuestionOpen` (not a bare nil-`CurrentQuestion` check) and logs ERROR on both the degraded-snapshot and recipient-lookup-failure skip paths. `handleStartGame`/`handleNextQuestion` now call it via `go dispatchQuestionOpened(...)` *after* `writeJSON`, so the organizer's response is never gated on the `PlayerRecipients` DB read. This made `dispatchQuestionOpened` genuinely fire-and-forget from the handler's perspective, so `control_test.go`'s dispatch tests needed real synchronization to stay deterministic (not sleep-based): `stubControlEngine`/`stubQuestionDispatcher` gained a mutex plus an optional `chan struct{}` signal that tests wait on before asserting — the two negative-path tests that trip the guard before any stub call (`TestNextQuestionFinishingGameDoesNotDispatch`, `TestOtherControlActionsNeverDispatch`) need no synchronization since nothing is ever written concurrently in those paths. The MCQ options-length guard was added in `question_notifier.go` (the actual dispatch boundary, which has a logger and `gameID` context), not inside `questionMCQMessage` itself, keeping `messages_he.go` a pure template file per the story's copy-centralization guardrail. All Go quality gates re-verified after the patches: `gofmt -l` clean, `go1.26.5 vet ./...` clean, `go1.26.5 build ./...` clean, `go1.26.5 test ./...` clean, dispatch tests additionally run 30x (`-count=30`) with no flakiness (`-race` unavailable on this machine — no C compiler).

**Deferred:**

- [x] [Review][Defer] `PlayerRecipients` is unscoped by `organizerID` on `ControlEngine` — authorization-by-convention, relying on the caller having already validated ownership via the preceding state transition. Mirrors the pre-existing `buildSnapshot`/`ListParticipants` pattern; not a new risk surface introduced by this diff, but the pattern is now used twice. [server/internal/httpapi/control.go (ControlEngine), server/internal/game/engine.go (PlayerRecipients)] — deferred, pre-existing pattern extended (not introduced) by this story; revisit if `PlayerRecipients` or a sibling unscoped method ever gains a direct HTTP-reachable call site.
- [x] [Review][Defer] Question text and MCQ options are not bidi-isolated, unlike every digit token in the same templates — organizer-authored Latin/mixed content (e.g. "C++", English question text) can render direction-scrambled in RTL. This is a deliberate, documented tradeoff per this story's own Dev Notes ("free-form sentence content never has [isolation]"), not an oversight. [server/internal/wa/messages_he.go (questionMCQMessage, questionFreeTextMessage)] — deferred, intentional scope decision; revisit if organizers author non-Hebrew question/option content in practice.
- [x] [Review][Defer] The displayed time limit doesn't account for dispatch/delivery latency — a slow burst under retry/backoff could show "20 seconds" with meaningfully less time actually remaining once story 3.3's answer-cutoff enforcement lands. Inherent to the pre-existing async `Dispatcher` model, unchanged by this story per its own AC-2 note. [server/internal/wa/messages_he.go, server/internal/wa/dispatch.go] — deferred, pre-existing dispatch-latency model; revisit at story 3.3 (answer-cutoff enforcement) if delivery-lag complaints surface.
- [x] [Review][Defer] `dispatchQuestionOpened`'s recipient-lookup-failure WARN uses the package-level `slog` instead of an injected/testable logger — matches every other `slog.Info`/`slog.Warn` call already in `control.go`; not a new departure introduced by this diff. [server/internal/httpapi/control.go (dispatchQuestionOpened)] — deferred, pre-existing file-wide convention.

**Dismissed as noise (7):** typed-nil `QuestionDispatcher` panic risk (no live call site — `main.go` always wires a concrete dispatcher); INFO log at `recipient_count=0` (legitimate all-spectator-game case, fully greppable); magic `5*time.Second` with no named constant (matches the identical pre-existing pattern in `engine.snapshotAfterCommit`); `NewQuestionNotifier` not nil-guarding its `Replier` param (matches `NewDispatcher`/`NewInboundRouter`'s existing posture, no production call site risk); `questionTypeMCQ`/`questionTypeFreeText` duplicated as local `wa` constants (matches a codebase-wide absence of centralized question-type constants — confirmed `game` package itself uses raw string literals); an extra `TestQuestionFreeTextMessageIsolatesDigitTokens` beyond the story's test list (net positive, mirrors the file's existing per-row pattern more faithfully than the spec); Dev Agent Record's E2E claims being unverifiable since the harness is deleted after use (inherent to this story's own documented convention, shared by every prior story).

## Dev Notes

### Architecture guardrails (violations = rework)

- **Dependency direction unchanged**: `httpapi` and `wa` still never import each other — `QuestionDispatcher`/`ControlEngine` in `httpapi` and `Registrar`/`SnapshotBroadcaster` in `wa` are both consumer-defined interfaces referencing only `game` types; `main.go` remains the sole place a concrete `wa.QuestionNotifier` is handed to `httpapi.NewRouter`. Do not add `"github.com/.../internal/wa"` to any `httpapi/*.go` import block, or vice versa.
- **State mutation stays centralized**: this story adds no new engine transition — it only reads (`PlayerRecipients`) and reacts to snapshots that story 3.1's transitions already produce. No handler or component sets `games.state` directly.
- **Copy centralization**: the two new Hebrew templates live in `messages_he.go` only — CI's Hebrew-literal grep enforces this on every other `.go` file; `question_notifier.go` must contain zero raw Hebrew.
- **Glossary**: `Game`, `Question`, `Participant` verbatim; no `quiz`/`session`/`host` identifiers.
- **Logging (NFR-8)**: `slog.Info` on every dispatch (game_id, question_id, question_type, recipient_count); `slog.Warn` only for the recipient-lookup degrade path; `slog.Error` reserved for the type-CHECK-constraint-violation case, which a healthy run never hits.
- **NFR-2 (never block the game loop)**: `Enqueue` is the only I/O-adjacent call in the hot path and it is non-blocking by construction (2.1); `DispatchQuestionOpened` itself does no I/O — the one real I/O call (`PlayerRecipients`'s DB read) happens in `httpapi`, bounded to 5s, and detached from request cancellation (see Task 3's helper).

### Existing code this story modifies — current state, and what must survive

- **[server/internal/game/engine.go](server/internal/game/engine.go)** — currently has `OpenLobby`, `StartGame`, `CloseQuestion`, `Reveal`, `NextQuestion`, `StopGame`, `Snapshot`, `snapshotAfterCommit`, `buildSnapshot`, `emptySnapshot` (all from 3.1). `PlayerRecipients` is purely additive — it does not touch any transition method, `Store` interface, or `buildSnapshot`.
- **[server/internal/wa/messages_he.go](server/internal/wa/messages_he.go)** — currently ends at `spectatorNoticeMessage` (story 2.5). Its header comment's row-status map is the one place this story edits an *existing* line (the two `-> story 3.2` rows gain ` (below)`) — every other edit is additive.
- **[server/internal/httpapi/control.go](server/internal/httpapi/control.go)** — currently: `ControlEngine` (6 methods), `SnapshotBroadcaster`, `handleOpenLobby`, `handleStartGame`, `handleCloseQuestion`, `handleReveal`, `handleNextQuestion`, `handleStopGame` (all from 3.1, all 2-arg `(engine, hub)`). Only `handleStartGame` and `handleNextQuestion` change shape (gain a 3rd param); the other three stay exactly as 3.1 left them.
- **[server/internal/httpapi/router.go](server/internal/httpapi/router.go)** — `NewRouter`'s signature and its `if engine != nil && hub != nil` block (6 routes) are from 3.1. This story adds one parameter and rewires 2 of the 6 route registrations; the other 4 lines in that block are untouched.
- **[server/cmd/server/main.go](server/cmd/server/main.go)** — the `dispatcher := wa.NewDispatcher(...)` / `inboundRouter := wa.NewInboundRouter(dispatcher, ...)` wiring from 2.1/2.2 is untouched; this story inserts one new line between them and adds one argument to the existing `httpapi.NewRouter(...)` call.
- **Every `httpapi` test file that calls `NewRouter(...)`** — purely mechanical trailing-nil additions (Task 3's mechanical step); no existing assertion in `router_test.go`, `games_test.go`, or `packages_test.go` changes meaning.

### Why this story pushes dispatch orchestration into `httpapi`, not `game`

Architecture's Integration Points section describes the intended flow explicitly: *"Organizer action (REST) → httpapi/control.go → game.Engine transition → persists via store → emits event → ws.Hub broadcasts new snapshot AND wa.Dispatcher sends the corresponding messages. One action, one transition, two transports."* Story 3.1 already put the WS-broadcast half of that "two transports" fan-out in `httpapi` (`hub.Broadcast(...)`, called from each handler, not from inside `game.Engine`). Putting the WhatsApp-dispatch half in the same place — a second fan-out call from the same handlers, right after the first — keeps both halves of one conceptual event symmetric; otherwise a future reader would find it surprising that one fan-out lives in `httpapi` and the other lives inside `Engine`. `game.Engine` stays free of any WhatsApp-specific concern, consistent with `game` never importing `wa`.

### The NewRouter signature change — deliberate, not an oversight

Story 3.1's Task 3 explicitly avoided changing `NewRouter`'s argument count when it only needed to extend an existing interface's methods. This story cannot do the same: dispatch is a genuinely new capability with no existing parameter to extend through. The trailing-`nil`-append across ~30 test call sites (Task 3's mechanical step) is real but bounded and low-risk — every one of those call sites already passes `nil` for `engine`/`hub`/`wsHandler` today and doesn't exercise control routes, so a fourth trailing `nil` changes nothing about what they test.

### Detached-context dispatch — the one non-obvious correctness detail

`hub.Broadcast` takes no `context.Context` at all (it's synchronous, in-process, from 3.1) — request cancellation can never affect it. `PlayerRecipients`, by contrast, is a real DB round-trip bound to whatever `ctx` it receives. If it inherited the REST handler's request-scoped `ctx` directly, an organizer's browser closing or timing out mid-click — right as `/start` or `/next-question` is processed — could cancel that context and silently skip dispatching the question to every other Participant, even though the state transition and WS broadcast already committed. This is the exact same failure shape `engine.snapshotAfterCommit` was built in story 3.1 to prevent for the broadcast path; Task 3's `dispatchQuestionOpened` helper applies the identical `context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)` pattern for the same reason. Do not simplify this to a plain `ctx` pass-through.

### Testing standards

Go stdlib `testing`, co-located `_test.go`, stubs grown in place (`stubStore`, `stubControlEngine`, new `stubQuestionDispatcher`) — no mock framework, matching every prior story. Captured-`slog` (`slog.New(slog.NewTextHandler(&buf, nil))`) is the house pattern for asserting the ERROR/WARN/INFO log lines this story adds. No real-DB unit tests; Task 5's throwaway `cmd/e2escratch` binary is the DB-and-WhatsApp-touching verification, deleted after the run.

### Project Structure Notes

**New:**
- `server/internal/wa/question_notifier.go`
- `server/internal/wa/question_notifier_test.go`

**Modified:**
- `server/internal/game/engine.go` (+`PlayerRecipients`)
- `server/internal/game/engine_test.go`
- `server/internal/wa/messages_he.go` (+2 templates, header comment update)
- `server/internal/wa/messages_he_test.go`
- `server/internal/httpapi/control.go` (+`QuestionDispatcher`, extended `ControlEngine`, +`dispatchQuestionOpened` helper, `handleStartGame`/`handleNextQuestion` signatures)
- `server/internal/httpapi/control_test.go`
- `server/internal/httpapi/router.go` (`NewRouter` +1 param, 2 route registrations updated)
- `server/internal/httpapi/router_test.go`, `games_test.go`, `packages_test.go` (mechanical trailing-`nil` only)
- `server/cmd/server/main.go` (+1 line, +1 argument)

**Untouched:** `server/migrations/*` (no schema change) · `server/internal/store/*` (no new query — `ListParticipants` already exists and already covers this need) · `server/internal/ws/*` · `server/internal/auth/*` · `server/internal/httpapi/errors.go` (no new error code — dispatch failures degrade silently, never surface as an HTTP error) · everything under `web/*` (no frontend AC in this story — the live control panel already renders `currentQuestion` from 3.1; this story only adds the WhatsApp side-channel to the same transition).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.2] — story + all 3 epic ACs verbatim, Epic 3 context, FR-4
- [Source: _bmad-output/planning-artifacts/architecture.md#API-Communication-Patterns] — `wa/dispatch.go` worker pool/rate-limiter description (pre-existing, unchanged by this story); Integration Points' "ws.Hub broadcasts... AND wa.Dispatcher sends... one action, one transition, two transports" (the design this story's httpapi-level orchestration follows)
- [Source: _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/EXPERIENCE.md#WhatsApp-message-templates] (lines 104-125) — the canonical Question — MCQ / Question — Free-Text copy rows, verbatim, this story's only source of truth for the templates (not the earlier "Information Architecture" inventory table, which only names the rows)
- [Source: server/internal/game/engine.go, snapshot.go, state.go] — `CurrentQuestion`/`Snapshot`/`RolePlayer` this story reads from and extends
- [Source: server/internal/wa/messages_he.go, dispatch.go, inbound.go] — existing copy-centralization pattern, the pre-existing `Dispatcher`/`Replier` this story reuses unchanged, `ltr()`/`lri`/`pdi` isolation helpers
- [Source: server/internal/wa/messages_he_test.go, inbound_test.go] — the `stripIsolates`/`lriMark`/`pdiMark` canonical-copy test pattern this story's new tests extend
- [Source: server/internal/httpapi/control.go, control_test.go, router.go] — the `ControlEngine`/`SnapshotBroadcaster` consumer-defined-interface convention and the `if engine != nil && hub != nil` route-mounting gate this story extends
- [Source: server/cmd/server/main.go] — existing `wa.NewDispatcher`/`wa.NewInboundRouter`/`httpapi.NewRouter` wiring order this story inserts into
- [Source: _bmad-output/implementation-artifacts/3-1-run-the-game-live-control-state-machine.md] — `snapshotAfterCommit`'s detached-context pattern (reused here for `PlayerRecipients`), the established Windows/encoding/go1.26.5/E2E-scratch-binary house patterns, `CurrentQuestion`/`QuestionCount`'s exact shape and the guarantee that only `StartGame`/`NextQuestion` populate `CurrentQuestion` on success
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — checked in full; no entry names story 3.2 as a trigger, and this story introduces none (recipient-fetch latency is a single indexed query, not a scale concern at pilot size)

## Dev Agent Record

### Agent Model Used

Claude Sonnet 5 (claude-sonnet-5), via the bmad-dev-story workflow.

### Debug Log References

- `sqlc generate` diff after every task: empty, as expected (no schema change this story) — confirmed via `git status --porcelain internal/store/gen/`.
- `gofmt -l .` on the raw (CRLF-checked-out) tree flagged this story's own new/modified files (`control_test.go`, `router_test.go`, `games_test.go`, `packages_test.go`) in addition to the ~9 pre-existing files 2.2–3.1 already documented; LF-normalizing each changed file first (per the documented Windows/`core.autocrlf=true` workaround) and re-running `gofmt -l` on the normalized copies showed zero real violations — confirmed the flags were the known CRLF false-positive, not new formatting drift. `messages_he_test.go`'s new `const` block genuinely needed `gofmt -w` (alignment), applied.
- `go1.26.5 vet ./...` and `go1.26.5 test ./...` clean on every run; `npm run lint` / `npx tsc -b --noEmit` under `web/` clean (no frontend files touched this story).
- The mechanical `NewRouter(...)` trailing-`nil` pass across `router_test.go`/`games_test.go`/`packages_test.go`/`control_test.go`: a first regex-based pass (`NewRouter\([^)]*\)`) matched the *first* `)` on the line instead of the last (broke on nested calls like `noAuth()`/`testStatic()`), corrupting all four files. Caught immediately via `go1.26.5 vet` errors, reverted with `git checkout --`, and redone with a line-anchored approach (append `, nil` before the final `)` on any line containing `NewRouter(` and ending in `)`) — 25/2/1/3 replacements respectively, matching the story's documented call-site counts exactly.
- Local Go E2E (`cmd/e2escratch`, deleted after use, same pattern as 2.1–3.1): confirmed ports 8080/18080/19099 were free before starting (learned from 3.1's stale-process pitfall). Built and booted the real `cmd/server` binary with `WHATSAPP_API_BASE_URL` pointed at a local fake Meta endpoint recording every outbound `(to, body)` pair; `e2e-org-a` re-provisioned with a known scratch password for this run (`cmd/provision`). Full sequence — create game + 1 MCQ (4 options) + 1 Free-Text question → open-lobby → JOIN 2 players (real webhook path, HMAC-signed) → `POST /start` (asserted exactly 2 sends, both bodies exactly matching `questionMCQMessage`'s output, "שאלה 1 מתוך 2...") → `POST /close-question` → `POST /reveal` (asserted 0 new sends) → `POST /next-question` (asserted exactly 2 new sends matching the Free-Text template, "שאלה 2 מתוך 2...") → JOIN a 3rd phone (game past lobby, registered as Spectator; asserted its only-ever message was its own JOIN spectator-notice reply, never a Question) — passed clean on the first run, no bugs found. Scratch game row deleted afterward; harness deleted afterward (never committed).

### Completion Notes List

- All 5 tasks and their subtasks complete; all 3 ACs satisfied.
- `game.Engine.PlayerRecipients` is purely additive (Task 1) — no existing transition, `Store` interface, or `buildSnapshot` behavior changed.
- Two new Hebrew Question templates added to `messages_he.go` (the sole home for outbound copy), copied verbatim from EXPERIENCE.md and confirmed by direct comparison against the source table during implementation. `question_notifier.go` contains zero raw Hebrew.
- `httpapi`/`wa` dependency direction preserved: `QuestionDispatcher` is a consumer-defined interface in `httpapi` referencing only `game` types; `main.go` remains the sole place a concrete `*wa.QuestionNotifier` is wired into `httpapi.NewRouter`.
- `NewRouter` gained one trailing `dispatcher QuestionDispatcher` parameter; every non-dispatch-exercising call site (~31 across 4 test files) got a mechanical trailing `nil`, verified individually against the story's documented per-file counts before moving on.
- No real-phone WhatsApp verification performed or needed — the local E2E already proves delivery end-to-end through the real webhook/dispatch code paths against a fake Meta endpoint; this story's Dev Notes do not call for a real-phone pass (unlike 2.1's Task 0).
- `deferred-work.md` was read per the story's Dev Notes; no entry names story 3.2, and none of its items are reachable by this story's files.

### File List

**New:**
- `server/internal/wa/question_notifier.go`
- `server/internal/wa/question_notifier_test.go`

**Modified:**
- `server/internal/game/engine.go` (+`PlayerRecipients`)
- `server/internal/game/engine_test.go` (+`TestPlayerRecipients*`)
- `server/internal/wa/messages_he.go` (+2 Question templates, header comment update, `+"strconv"` import)
- `server/internal/wa/messages_he_test.go` (+Question-MCQ/Free-Text canonical-copy + isolation tests)
- `server/internal/httpapi/control.go` (+`QuestionDispatcher`, extended `ControlEngine`, +`dispatchQuestionOpened` helper, `handleStartGame`/`handleNextQuestion` signatures)
- `server/internal/httpapi/control_test.go` (+`PlayerRecipients` stub method, +`stubQuestionDispatcher`, +`controlRouterWithDispatcher`, +dispatch tests, mechanical trailing-`nil`)
- `server/internal/httpapi/router.go` (`NewRouter` +1 param, 2 route registrations updated, doc comment updated)
- `server/internal/httpapi/router_test.go`, `games_test.go`, `packages_test.go` (mechanical trailing-`nil` only)
- `server/cmd/server/main.go` (+`questionNotifier` wiring, +1 `NewRouter` argument)

## Change Log

- 2026-08-04: Dev implementation complete (all 5 tasks, all 3 ACs) — game.Engine.PlayerRecipients, two new Hebrew Question templates, wa.QuestionNotifier, httpapi dispatch wiring on /start and /next-question, main.go wiring. All Go quality gates green (gofmt, vet, test), sqlc generate diff empty, frontend gates green (no frontend files touched), local Go E2E against a fake Meta endpoint passed clean on the first run. Status: review.
- 2026-08-04: Code review complete (Blind Hunter + Edge Case Hunter + Acceptance Auditor). 2 decision-needed findings resolved by the user and applied as patches (dispatch moved off the HTTP response path into a goroutine after `writeJSON`; both post-commit silent-skip paths now log ERROR instead of vanishing or logging WARN); 6 further patches applied (MCQ options-length guard, hardened `question_open`-state dispatch guard, blank-phone trim+WARN, restored the `[ASSUMPTION]` template comment, de-pinned an error-string test, strengthened recipient-value assertions); 4 findings deferred (documented in `deferred-work.md`); 7 dismissed as noise. All Go quality gates re-verified green after the patches (gofmt, vet, build, test; dispatch tests additionally run 30x with no flakiness — `-race` unavailable, no C compiler on this machine). Status: done.
