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
