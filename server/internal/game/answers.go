package game

import (
	"context"
	"errors"
	"fmt"
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

// AnswerResult is RecordAnswer's success return. GameID and Snapshot are
// only populated on AnswerAccepted (and only when the post-write snapshot
// build succeeds) — callers broadcast only when Snapshot.GameID != "", same
// convention as JoinResult.
type AnswerResult struct {
	Outcome  AnswerOutcome
	GameID   string
	Snapshot Snapshot
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
