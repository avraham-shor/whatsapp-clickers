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
