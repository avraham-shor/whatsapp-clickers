package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

// questionStatsPayload is one Question's row of the post-game summary
// (camelCase wire format). No omitempty anywhere: 0 answered is a real,
// meaningful value on this page - it is exactly what a Question the game
// never reached looks like - and must serialize.
type questionStatsPayload struct {
	ID            string `json:"id"`
	Position      int32  `json:"position"`
	Type          string `json:"type"`
	Text          string `json:"text"`
	AnsweredCount int32  `json:"answeredCount"`
	CorrectCount  int32  `json:"correctCount"`
}

// resultsPayload is the whole post-game summary in one direct payload
// (no {"items":...} wrapper - that convention is for plain lists; this
// is a composite resource, same posture as gameDetailPayload).
//
// Leaderboard reuses game.LeaderboardEntry rather than redeclaring a
// near-identical struct: it already carries the camelCase JSON tags and
// is already the wire shape the Snapshot serializes, so the dashboard
// sees one leaderboard shape whether it arrives over WS or over REST.
//
// PlayerCount is the response-rate denominator, and it is len(leaderboard)
// by construction, not a separate count: GetLeaderboard LEFT JOINs every
// role='player' Participant, so it returns exactly one row each - a
// player with no answers included, a Spectator excluded. It is sent
// explicitly anyway so the client renders "X out of Y" without having to
// know that identity.
type resultsPayload struct {
	GameID      string                  `json:"gameId"`
	Title       string                  `json:"title"`
	State       string                  `json:"state"`
	PlayerCount int                     `json:"playerCount"`
	Leaderboard []game.LeaderboardEntry `json:"leaderboard"`
	Questions   []questionStatsPayload  `json:"questions"`
}

// handleGameResults serves the Organizer's post-game summary (FR-14):
// the final ranked Leaderboard plus per-question response counts.
//
// Guarded on state = finished, per the epic's "Given a finished Game".
// A game still in play answers 409 GAME_NOT_FINISHED rather than a
// partial summary - the surface is a review artifact, not a second live
// panel, and the live panel is where an in-progress game belongs.
//
// Reads only; no engine call, no transition, no broadcast. That is what
// makes the page durable across sessions (epic AC-2): every number comes
// from Postgres on each request, so a reload, a different browser, or a
// fresh sign-in renders identically, with no dependence on the WS
// connection that ran the game.
//
// GetLeaderboard is organizer-unscoped by design (queries/answers.sql) -
// ownership is established one line earlier by GetGameForOrganizer,
// whose failure is indistinguishable from a missing game (404, never
// 403). Same trust posture as PlayerRecipients' own unscoped roster read.
func handleGameResults(games GameStore) http.HandlerFunc {
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

		g, err := games.GetGameForOrganizer(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		if g.State != game.StateFinished {
			writeError(w, http.StatusConflict, "GAME_NOT_FINISHED", "results are available only after the game has finished")
			return
		}

		scores, err := games.GetLeaderboard(ctx, gameID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		stats, err := games.ListQuestionResponseStats(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}

		// Non-nil so the wire always carries [], never null - the TS
		// mirror types both as non-nullable arrays (gameDetailPayload's
		// own discipline).
		questions := make([]questionStatsPayload, 0, len(stats))
		for _, s := range stats {
			questions = append(questions, questionStatsPayload{
				ID:            s.QuestionID,
				Position:      s.Position,
				Type:          s.Type,
				Text:          s.Text,
				AnsweredCount: s.AnsweredCount,
				CorrectCount:  s.CorrectCount,
			})
		}
		// RankLeaderboard returns make([]LeaderboardEntry, len(sorted)) -
		// empty but non-nil for a game nobody joined, so this needs no
		// extra guard of its own (game/scoring.go).
		entries := game.RankLeaderboard(scores)

		writeJSON(w, http.StatusOK, resultsPayload{
			GameID:      g.ID,
			Title:       g.Title,
			State:       g.State,
			PlayerCount: len(entries),
			Leaderboard: entries,
			Questions:   questions,
		})
	}
}
