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
// non-blank phone. Called from httpapi/control.go's own goroutine, spawned
// right after a StartGame/NextQuestion transition commits, its WS snapshot
// broadcasts, and the REST response is already written; actual delivery
// happens asynchronously on the Dispatcher's own worker pool (AC-2/AC-3 are
// its pre-existing behavior, not this method's).
func (n *QuestionNotifier) DispatchQuestionOpened(gameID string, question game.CurrentQuestion, questionCount int, recipients []string) {
	var body string
	switch question.Type {
	case questionTypeMCQ:
		if len(question.Options) != 4 {
			// Same invariant-violation class as the unknown-type case below
			// (the questions table's CHECK constraint guarantees 4 options
			// for every mcq row) — guarded here, not inside
			// questionMCQMessage, which stays a pure template function that
			// trusts its caller (validate at the boundary, once).
			n.logger.Error("question dispatch: mcq question does not have exactly 4 options, sending nothing",
				"game_id", gameID, "question_id", question.ID, "option_count", len(question.Options))
			return
		}
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
		trimmed := strings.TrimSpace(phone)
		if trimmed == "" {
			n.logger.Warn("question dispatch: skipping recipient with a blank phone",
				"game_id", gameID, "question_id", question.ID)
			continue
		}
		n.replier.Enqueue(trimmed, body)
		dispatched++
	}
	n.logger.Info("question dispatch enqueued",
		"game_id", gameID, "question_id", question.ID, "question_type", question.Type,
		"recipient_count", dispatched)
}
