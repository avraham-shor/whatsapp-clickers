package game

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/avraham-shor/whatsapp-clickers/internal/grading"
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

// Bounds on one answer's asynchronous AI grading (Story 3.6).
const (
	// DefaultMaxConcurrentAIGrades caps simultaneous AI Semantic stage
	// calls. Sized against the two resources a burst contends for — the
	// Anthropic account's rate limit and a pgxpool of max(4, numCPU) — not
	// against room size, which the queue absorbs instead. See
	// WithMaxConcurrentAIGrades.
	DefaultMaxConcurrentAIGrades = 8

	// gradeAsyncTimeout bounds one answer's whole grading attempt: waiting
	// for a slot, the AI call (which imposes its own ~5s deadline inside
	// this), and persisting the verdict. Generous enough that a full room
	// queued behind the concurrency cap still gets graded, finite so a
	// wedged dependency cannot park a goroutine and a pool waiter for the
	// rest of the process's life. Must stay well under OrphanAnswerAge, or
	// the sweep would fail-close answers still legitimately in flight.
	gradeAsyncTimeout = 90 * time.Second

	// gradePersistAttempts / gradePersistBackoff retry the verdict write.
	// This is the only writer that clears a pending row in-process, and
	// Reveal is gated on that column, so a single transient pool error must
	// not strand the question. Linear backoff: 100ms, then 200ms — short,
	// because this rides out pool contention rather than an outage, and the
	// orphan sweep is the backstop for anything longer-lived.
	gradePersistAttempts = 3
	gradePersistBackoff  = 100 * time.Millisecond

	// OrphanAnswerAge is how old a still-ungraded answer must be before the
	// orphan sweep may fail-close it. Comfortably beyond gradeAsyncTimeout
	// so the sweep can never touch a grade another process is still working
	// on — the property that makes the sweep safe to run on a ticker, and
	// safe while a second instance is live during a zero-downtime redeploy.
	OrphanAnswerAge = 5 * time.Minute
)

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

// AnswerResult is RecordAnswer's success return. GameID and Snapshot are
// only populated on AnswerAccepted (and only when the post-write snapshot
// build succeeds) — callers broadcast only when Snapshot.GameID != "", same
// convention as JoinResult.
type AnswerResult struct {
	Outcome  AnswerOutcome
	GameID   string
	Snapshot Snapshot
}

// hebrewOptionLetterBase is the Unicode code point of the Hebrew letter
// ALEF (U+05D0), the first of the four lettered MCQ options. ALEF, BET,
// GIMEL and DALET are four CONSECUTIVE code points (U+05D0-U+05D3) in the
// same order as messages_he.go's Question-MCQ template lists them, so the
// 1-based option number is just an offset from this base — expressed as a
// code-point range rather than a literal-keyed map (and named by letter
// name, never spelled with the actual glyph, even in a comment) so this
// file stays entirely free of raw Hebrew characters. That matters because
// game is imported BY wa (see wa/inbound.go), so game cannot import
// wa/messages_he.go back without an import cycle — the one file CI's
// copy-centralization check exempts (besides _test.go files) is not an
// option here.
const hebrewOptionLetterBase = 0x05D0

// parseMCQOption recognizes a TRIMMED single Hebrew option letter or digit
// (1-4) reply. Unlike parseJoinCode's multi-field tolerance, any extra
// content ("1 hi", trailing punctuation after the letter) fails to parse —
// FR-5 specifies a bare letter or digit, and a stricter parse here means
// such input correctly earns the format hint (AC-2) rather than a guessed,
// possibly-wrong option.
func parseMCQOption(body string) (option int, ok bool) {
	trimmed := strings.TrimSpace(body)
	if n, err := strconv.Atoi(trimmed); err == nil && n >= 1 && n <= 4 {
		return n, true
	}
	if utf8.RuneCountInString(trimmed) == 1 {
		r, _ := utf8.DecodeRuneInString(trimmed)
		if offset := r - hebrewOptionLetterBase; offset >= 0 && offset <= 3 {
			return int(offset) + 1, true
		}
	}
	return 0, false
}

// stripFormatMarks removes the invisible Unicode format characters
// (category Cf) that Hebrew mobile keyboards and copy-paste routinely
// inject — RIGHT-TO-LEFT MARK, LEFT-TO-RIGHT MARK, the BOM / zero-width
// no-break space, and the zero-width joiners. strings.TrimSpace cannot
// remove them: unicode.IsSpace covers Zs plus \t\n\v\f\r, U+0085 and
// U+00A0, none of which are Cf. Left in, they survive into
// grading.GradeExact's byte comparison and mark a visually identical
// Hebrew answer incorrect, while the participant still receives the
// normal ack and has no way to discover why; in the mcq branch they turn
// an otherwise valid "2" into an unparseable reply that earns the format
// hint. Distinct from the spelling variants (nikud, final-letter forms,
// punctuation) deliberately deferred to Story 3.5's Fuzzy stage: those
// are visible differences a human can recognize and correct, an invisible
// byte is not. Code review finding, story 3.4.
//
// Matched by Unicode category rather than an enumerated list of code
// points: the whole class is unwanted here, the literals are invisible in
// an editor, and a literal U+FEFF is rejected outright by the compiler.
func stripFormatMarks(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
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

	// Resolve closed/already-answered ahead of content-format validation
	// (review finding, story 3.3): otherwise a malformed or over-length
	// reply arriving after close, or from someone who already answered,
	// would short-circuit on AnswerFormatHint/AnswerTooLong below — never
	// reaching store.RecordAnswer's write-time guard, the only place that
	// distinguishes those two outcomes — and incorrectly imply the sender
	// can still answer. already_answered takes precedence: it is the more
	// specific, final fact for a sender who already has a recorded answer,
	// even if the question has also since closed. The write-time guard
	// below remains the authority for the read-then-write race window;
	// this is a best-effort pre-check off the same read, not a
	// replacement for it.
	if qc.AlreadyAnswered {
		return AnswerResult{Outcome: AnswerAlreadyAnswered}, nil
	}
	if !qc.IsOpen {
		return AnswerResult{Outcome: AnswerClosed}, nil
	}

	// Strip invisible format marks before any parsing, length check or
	// grading — see stripFormatMarks. Applied once here rather than per
	// branch: both the mcq parse and the free_text comparison are equally
	// defeated by a mark the sender cannot see.
	body := stripFormatMarks(rawText)

	var response string
	var isCorrectPtr *bool
	var stagePtr *string
	switch qc.QuestionType {
	case "mcq":
		option, ok := parseMCQOption(body)
		if !ok {
			return AnswerResult{Outcome: AnswerFormatHint}, nil
		}
		response = strconv.Itoa(option)
		ic := grading.GradeMCQ(response, int(qc.CorrectOption))
		st := grading.StageMCQ
		isCorrectPtr, stagePtr = &ic, &st
	case "free_text":
		trimmed := strings.TrimSpace(body)
		if utf8.RuneCountInString(trimmed) > maxFreeTextAnswerLength {
			return AnswerResult{Outcome: AnswerTooLong}, nil
		}
		response = trimmed
		switch {
		case grading.GradeExact(response, qc.AcceptedAnswers):
			ic := true
			st := grading.StageExact
			isCorrectPtr, stagePtr = &ic, &st
		case grading.GradeFuzzy(response, qc.AcceptedAnswers):
			ic := true
			st := grading.StageFuzzy
			isCorrectPtr, stagePtr = &ic, &st
		default:
			// Exact and Fuzzy both missed — leave both nil (pending AI).
			// Graded asynchronously below, once RecordAnswer persists the
			// pending row (Story 3.6, AC-5: never block the ack on the AI
			// call's ~5s worst case).
		}
	default:
		// questions_type_shape's CHECK constraint guarantees this never
		// happens for a real row — fail closed with an error (degrades to
		// Help + WARN in wa, matching every other "impossible" branch in
		// this codebase) rather than guessing a parse strategy.
		return AnswerResult{}, fmt.Errorf("game: question %s has unrecognized type %q", qc.QuestionID, qc.QuestionType)
	}

	answer, err := e.store.RecordAnswer(ctx, store.RecordAnswerParams{
		GameID:        qc.GameID,
		QuestionID:    qc.QuestionID,
		ParticipantID: qc.ParticipantID,
		Response:      response,
		ReceivedAt:    receivedAt,
		IsCorrect:     isCorrectPtr,
		Stage:         stagePtr,
	})
	switch {
	case err == nil:
		if isCorrectPtr == nil {
			e.launchAIGrading(ctx, answer.ID, qc.QuestionText, qc.AcceptedAnswers, response)
		}
		return AnswerResult{Outcome: AnswerAccepted, GameID: qc.GameID, Snapshot: e.snapshotAfterAnswer(ctx, qc.GameID)}, nil
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

// launchAIGrading starts the AI Semantic stage for one just-persisted
// pending answer (Exact and Fuzzy both missed) and returns immediately. The
// grading itself is NOT on RecordAnswer's synchronous request path (epic
// AC-5: FR-5 "immediately acknowledges", NFR-2 "never block the game loop")
// — that is why the pending row (is_correct=NULL, stage=NULL) exists at all.
func (e *Engine) launchAIGrading(ctx context.Context, answerID, questionText string, acceptedAnswers []string, response string) {
	if e.aiGrader == nil {
		// Not a supported degraded mode: nothing else will ever grade this
		// row, so Reveal for its question stays blocked until the orphan
		// sweep fail-closes it. Reachable only from a caller that built the
		// Engine without WithAIGrader (the 3-arg NewEngine form, kept
		// compiling for ~65 pre-3.6 tests), so say so rather than failing
		// silently. Code review finding, story 3.6.
		e.logger.Warn("no AI grader configured; answer left pending for the orphan sweep", "answer_id", answerID)
		return
	}

	// Detached from ctx's cancellation — the webhook request that triggered
	// this must not abort grading for this participant, same reasoning as
	// snapshotAfterAnswer — but bounded, for the same reason that one wraps
	// its detached context in a timeout: an unbounded context would let a
	// wedged DB park this goroutine and its pool waiter forever. GradeAI
	// derives its own ~5s per-call deadline inside this budget; the rest
	// covers waiting for a concurrency slot and persisting the verdict.
	gradeCtx := context.WithoutCancel(ctx)
	e.gradeWG.Add(1)
	e.runAsync(func() {
		defer e.gradeWG.Done()
		ctx, cancel := context.WithTimeout(gradeCtx, gradeAsyncTimeout)
		defer cancel()
		defer e.recoverGrading(ctx, answerID)
		e.gradeAIAsync(ctx, answerID, questionText, acceptedAnswers, response)
	})
}

// recoverGrading keeps a panic anywhere in the grading goroutine — the
// Anthropic SDK, its HTTP transport, JSON decoding — from taking the whole
// process down with it. This goroutine is spawned from the webhook's own
// goroutine, so neither wa/webhook.go's recover nor net/http's
// per-connection recover can see a panic here, and the repo has no
// panic-recovery middleware. Fail closed afterwards so the answer's
// question can still be revealed. Code review finding, story 3.6.
func (e *Engine) recoverGrading(ctx context.Context, answerID string) {
	r := recover()
	if r == nil {
		return
	}
	e.logger.Error("panic in AI grading goroutine, failing the answer closed",
		"answer_id", answerID, "panic", r, "stack", string(debug.Stack()))
	e.persistGrade(ctx, answerID, false, grading.StageFuzzy)
}

// gradeAIAsync runs the AI Semantic stage for one pending answer and
// persists the verdict once it resolves. ctx must already be detached from
// the triggering request and carry its own deadline (launchAIGrading does
// both).
func (e *Engine) gradeAIAsync(ctx context.Context, answerID, questionText string, acceptedAnswers []string, response string) {
	correct, err := e.gradeAI(ctx, questionText, acceptedAnswers, response)
	stage := grading.StageAI
	if err != nil {
		// Fail-closed (epic AC-2, NFR-8/SM-C1): AI unavailable — grade by
		// the first two stages only, i.e. exactly as a Fuzzy miss. AC-2
		// requires the recorded stage in this line, not just the identity
		// and the cause: it is the SM-C1 audit trail for how an answer that
		// never reached the AI stage came to be graded.
		correct = false
		stage = grading.StageFuzzy
		e.logger.Warn("AI grading degraded, falling back to two-stage grade",
			"answer_id", answerID, "stage", string(stage), "error", err)
	}
	e.persistGrade(ctx, answerID, correct, stage)
}

// gradeAI takes one of the bounded AI-grading slots, then runs the stage.
// Failing to get a slot before ctx expires is an "AI unavailable" error,
// which the caller fails closed exactly like a timeout — the room's
// Organizer waiting on Reveal is better served by a two-stage grade than by
// a queue that outlives the game.
func (e *Engine) gradeAI(ctx context.Context, questionText string, acceptedAnswers []string, response string) (bool, error) {
	select {
	case e.gradeSem <- struct{}{}:
		defer func() { <-e.gradeSem }()
	case <-ctx.Done():
		return false, fmt.Errorf("game: no AI grading slot available before the deadline: %w", ctx.Err())
	}
	return e.aiGrader.GradeAI(ctx, questionText, acceptedAnswers, response)
}

// persistGrade writes one resolved verdict, retrying a bounded number of
// times before giving up.
//
// The retry matters because this is the only writer that clears a pending
// row during a process's lifetime, and Reveal is gated on exactly that
// column: dropping a single transient pool error (the original behavior —
// log once and return) stranded one row and therefore one question for the
// rest of the process, contradicting epic AC-2's "never left ungraded". The
// orphan sweep is the backstop when even the retries fail, which is why the
// final log line says so. Code review finding, story 3.6.
func (e *Engine) persistGrade(ctx context.Context, answerID string, correct bool, stage grading.Stage) {
	var err error
	for attempt := 1; attempt <= gradePersistAttempts; attempt++ {
		if err = e.store.UpdateAnswerGrade(ctx, answerID, correct, string(stage)); err == nil {
			return
		}
		if attempt == gradePersistAttempts || !sleepCtx(ctx, gradePersistBackoff*time.Duration(attempt)) {
			break
		}
	}
	e.logger.Error("failed to persist AI grading verdict, leaving it to the orphan sweep",
		"answer_id", answerID, "attempts", gradePersistAttempts, "error", err)
}

// sleepCtx waits for d, reporting false if ctx finished first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// snapshotAfterAnswer builds the post-answer snapshot for gameID, used to
// broadcast the live answered-count (AC-5) after an accepted answer. Unlike
// snapshotAfterCommit, a failure here returns the zero Snapshot (GameID ==
// "") rather than a degraded placeholder: the answer itself already
// committed, so RecordAnswer's caller must still get AnswerAccepted, but
// there is no safe placeholder count to broadcast (it would undercount by
// at least this answer) — skip the broadcast entirely and let the next
// successful action or a dashboard reconnect self-heal it, same posture as
// joinLobby's post-join snapshot build failure.
//
// Detached from ctx's cancellation for the same reason as
// snapshotAfterCommit: this feeds the broadcast every other WS client
// receives, so the webhook request that triggered it being aborted must not
// degrade that broadcast for everyone else.
func (e *Engine) snapshotAfterAnswer(ctx context.Context, gameID string) Snapshot {
	buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	g, err := e.store.GetGameByID(buildCtx, gameID)
	if err != nil {
		e.logger.Warn("post-answer game lookup failed, skipping broadcast", "game_id", gameID, "error", err)
		return Snapshot{}
	}
	snap, err := e.buildSnapshot(buildCtx, g)
	if err != nil {
		e.logger.Warn("post-answer snapshot build failed, skipping broadcast", "game_id", gameID, "error", err)
		return Snapshot{}
	}
	return snap
}
