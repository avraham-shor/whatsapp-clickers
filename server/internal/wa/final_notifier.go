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
