package game

import (
	"context"
	"errors"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// finalPlayer/finalSpectator build roster rows in ListParticipants' shape.
// Phones follow this package's existing +9725000000NN fixture convention.
func finalPlayer(id, phone string) gen.Participant {
	return gen.Participant{ID: id, GameID: testGameID, Phone: phone, Role: RolePlayer}
}

func finalSpectator(id, phone string) gen.Participant {
	return gen.Participant{ID: id, GameID: testGameID, Phone: phone, Role: RoleSpectator}
}

func finalScore(participantID, displayName string, score int32) store.ParticipantScore {
	return store.ParticipantScore{ParticipantID: participantID, DisplayName: displayName, Score: score}
}

func recipientByPhone(t *testing.T, recipients []FinalRecipient, phone string) FinalRecipient {
	t.Helper()
	for _, r := range recipients {
		if r.Phone == phone {
			return r
		}
	}
	t.Fatalf("no FinalRecipient for phone %q in %+v", phone, recipients)
	return FinalRecipient{}
}

func TestResultsForFinishedGameSingleWinnerNamesOnlyTheTopScorer(t *testing.T) {
	st := &stubStore{
		participants: []gen.Participant{
			finalPlayer("p1", "+972500000001"),
			finalPlayer("p2", "+972500000002"),
			finalPlayer("p3", "+972500000003"),
			finalPlayer("p4", "+972500000004"),
		},
		getLeaderboardResult: []store.ParticipantScore{
			finalScore("p1", "A", 300),
			finalScore("p2", "B", 200),
			finalScore("p3", "C", 100),
			finalScore("p4", "D", 0),
		},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForFinishedGame(context.Background(), testGameID)
	if err != nil {
		t.Fatalf("ResultsForFinishedGame() error = %v, want nil", err)
	}
	if len(got.WinnerNames) != 1 || got.WinnerNames[0] != "A" {
		t.Errorf("WinnerNames = %v, want exactly [A]", got.WinnerNames)
	}
	if got.WinnerScore != 300 {
		t.Errorf("WinnerScore = %d, want 300", got.WinnerScore)
	}
	winners := 0
	for _, r := range got.Recipients {
		if r.IsWinner {
			winners++
			if r.Phone != "+972500000001" {
				t.Errorf("IsWinner set on %q, want only +972500000001", r.Phone)
			}
		}
	}
	if winners != 1 {
		t.Errorf("%d recipients marked IsWinner, want exactly 1", winners)
	}
}

func TestResultsForFinishedGameTiedWinnersNamesAllTiedAtRankOne(t *testing.T) {
	st := &stubStore{
		participants: []gen.Participant{
			finalPlayer("p1", "+972500000001"),
			finalPlayer("p2", "+972500000002"),
			finalPlayer("p3", "+972500000003"),
		},
		getLeaderboardResult: []store.ParticipantScore{
			finalScore("p1", "A", 250),
			finalScore("p2", "B", 250),
			finalScore("p3", "C", 100),
		},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForFinishedGame(context.Background(), testGameID)
	if err != nil {
		t.Fatalf("ResultsForFinishedGame() error = %v, want nil", err)
	}
	// Leaderboard order, not roster order — RankLeaderboard's stable sort
	// keeps tied players in the order GetLeaderboard returned them.
	if len(got.WinnerNames) != 2 || got.WinnerNames[0] != "A" || got.WinnerNames[1] != "B" {
		t.Errorf("WinnerNames = %v, want [A B] in leaderboard order", got.WinnerNames)
	}
	if got.WinnerScore != 250 {
		t.Errorf("WinnerScore = %d, want 250", got.WinnerScore)
	}
	if !recipientByPhone(t, got.Recipients, "+972500000001").IsWinner {
		t.Error("p1 IsWinner = false, want true (tied at the top)")
	}
	if !recipientByPhone(t, got.Recipients, "+972500000002").IsWinner {
		t.Error("p2 IsWinner = false, want true (tied at the top)")
	}
	if recipientByPhone(t, got.Recipients, "+972500000003").IsWinner {
		t.Error("p3 IsWinner = true, want false (not tied at the top)")
	}
}

func TestResultsForFinishedGameEveryPlayerGetsOwnRankAndScore(t *testing.T) {
	scores := []store.ParticipantScore{
		finalScore("p1", "A", 250),
		finalScore("p2", "B", 250),
		finalScore("p3", "C", 100),
	}
	st := &stubStore{
		participants: []gen.Participant{
			finalPlayer("p1", "+972500000001"),
			finalPlayer("p2", "+972500000002"),
			finalPlayer("p3", "+972500000003"),
		},
		getLeaderboardResult: scores,
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForFinishedGame(context.Background(), testGameID)
	if err != nil {
		t.Fatalf("ResultsForFinishedGame() error = %v, want nil", err)
	}
	// Cross-check against RankLeaderboard itself rather than hand-written
	// numbers — the standard-competition 1,1,3 shape (never 1,1,2) is
	// scoring.go's contract, and this asserts the recipients mirror it.
	wantByPhone := map[string]LeaderboardEntry{}
	phoneByParticipant := map[string]string{"p1": "+972500000001", "p2": "+972500000002", "p3": "+972500000003"}
	for _, entry := range RankLeaderboard(scores) {
		wantByPhone[phoneByParticipant[entry.ParticipantID]] = entry
	}
	if wantByPhone["+972500000003"].Rank != 3 {
		t.Fatalf("fixture sanity: want a 1,1,3 shape, got rank %d for the third player", wantByPhone["+972500000003"].Rank)
	}
	for phone, want := range wantByPhone {
		r := recipientByPhone(t, got.Recipients, phone)
		if r.Rank != want.Rank || r.Score != want.Score {
			t.Errorf("recipient %s = Rank %d Score %d, want Rank %d Score %d", phone, r.Rank, r.Score, want.Rank, want.Score)
		}
	}
}

func TestResultsForFinishedGameSpectatorsAreRecipientsWithNoRankOrScore(t *testing.T) {
	st := &stubStore{
		participants: []gen.Participant{
			finalPlayer("p1", "+972500000001"),
			finalSpectator("p2", "+972500000002"),
		},
		// GetLeaderboard filters role = 'player', so the spectator is
		// legitimately absent here — this must NOT be an error.
		getLeaderboardResult: []store.ParticipantScore{
			finalScore("p1", "A", 300),
		},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForFinishedGame(context.Background(), testGameID)
	if err != nil {
		t.Fatalf("ResultsForFinishedGame() error = %v, want nil (a spectator's absence from the leaderboard is correct)", err)
	}
	spec := recipientByPhone(t, got.Recipients, "+972500000002")
	if !spec.IsSpectator || spec.Rank != 0 || spec.Score != 0 || spec.IsWinner {
		t.Errorf("spectator recipient = %+v, want IsSpectator=true Rank=0 Score=0 IsWinner=false", spec)
	}
}

func TestResultsForFinishedGameAllScoresZeroYieldsNoWinner(t *testing.T) {
	st := &stubStore{
		participants: []gen.Participant{
			finalPlayer("p1", "+972500000001"),
			finalPlayer("p2", "+972500000002"),
			finalPlayer("p3", "+972500000003"),
		},
		// The stopped-before-any-Reveal case: every points_awarded is
		// still NULL, so GetLeaderboard reports every player at 0 and
		// RankLeaderboard hands all of them rank 1.
		getLeaderboardResult: []store.ParticipantScore{
			finalScore("p1", "A", 0),
			finalScore("p2", "B", 0),
			finalScore("p3", "C", 0),
		},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForFinishedGame(context.Background(), testGameID)
	if err != nil {
		t.Fatalf("ResultsForFinishedGame() error = %v, want nil", err)
	}
	if len(got.WinnerNames) != 0 {
		t.Errorf("WinnerNames = %v, want empty — nobody scored, so nobody won", got.WinnerNames)
	}
	if got.WinnerScore != 0 {
		t.Errorf("WinnerScore = %d, want 0", got.WinnerScore)
	}
	if len(got.Recipients) != 3 {
		t.Fatalf("Recipients has %d entries, want 3 — everyone still gets a closing message", len(got.Recipients))
	}
	for _, r := range got.Recipients {
		if r.IsWinner {
			t.Errorf("recipient %+v is marked IsWinner, want none when the top score is 0", r)
		}
	}
}

func TestResultsForFinishedGameNoParticipantsReturnsEmptyNonNilSlices(t *testing.T) {
	st := &stubStore{participants: []gen.Participant{}, getLeaderboardResult: []store.ParticipantScore{}}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForFinishedGame(context.Background(), testGameID)
	if err != nil {
		t.Fatalf("ResultsForFinishedGame() error = %v, want nil", err)
	}
	if got.Recipients == nil || len(got.Recipients) != 0 {
		t.Errorf("Recipients = %+v, want a non-nil empty slice", got.Recipients)
	}
	if got.WinnerNames == nil || len(got.WinnerNames) != 0 {
		t.Errorf("WinnerNames = %+v, want a non-nil empty slice", got.WinnerNames)
	}
}

func TestResultsForFinishedGamePlayerAbsentFromLeaderboardReturnsError(t *testing.T) {
	st := &stubStore{
		participants: []gen.Participant{
			finalPlayer("p1", "+972500000001"),
			finalPlayer("p2", "+972500000002"),
		},
		// p2 is a player and so MUST appear (GetLeaderboard LEFT JOINs
		// every role='player' row; a scoreless one appears at score 0).
		// Its absence is an invariant violation, not a scoreless player.
		getLeaderboardResult: []store.ParticipantScore{
			finalScore("p1", "A", 300),
		},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	if _, err := e.ResultsForFinishedGame(context.Background(), testGameID); err == nil {
		t.Fatal("ResultsForFinishedGame() error = nil, want an error for a player missing from the leaderboard")
	}
}

func TestResultsForFinishedGameUnknownRoleReturnsError(t *testing.T) {
	st := &stubStore{
		participants: []gen.Participant{
			{ID: "p1", GameID: testGameID, Phone: "+972500000001", Role: "organizer"},
		},
		getLeaderboardResult: []store.ParticipantScore{},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	if _, err := e.ResultsForFinishedGame(context.Background(), testGameID); err == nil {
		t.Fatal("ResultsForFinishedGame() error = nil, want an error — an unrecognized role must not be treated as a spectator")
	}
}

func TestResultsForFinishedGamePropagatesStoreErrors(t *testing.T) {
	sentinel := errors.New("boom")
	cases := []struct {
		name string
		st   *stubStore
	}{
		{"ListParticipants", &stubStore{participantsErr: sentinel}},
		{"GetLeaderboard", &stubStore{participants: []gen.Participant{finalPlayer("p1", "+972500000001")}, getLeaderboardErr: sentinel}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEngine(tc.st, "+972 50-000-0000", nil)
			_, err := e.ResultsForFinishedGame(context.Background(), testGameID)
			if !errors.Is(err, sentinel) {
				t.Fatalf("ResultsForFinishedGame() err = %v, want the %s error propagated unwrapped", err, tc.name)
			}
		})
	}
}
