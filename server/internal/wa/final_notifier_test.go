package wa

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

// bodiesFor returns every body enqueued for phone, in enqueue order — the
// winner tests below assert on both the count and the order of a single
// recipient's messages.
func bodiesFor(calls []stubReply, phone string) []string {
	var out []string
	for _, c := range calls {
		if c.to == phone {
			out = append(out, c.body)
		}
	}
	return out
}

func TestDispatchGameFinishedPlayerGetsFinalResultsWithOwnRankAndScore(t *testing.T) {
	replier := &stubReplier{}
	n := NewFinalNotifier(replier, nil)
	results := game.FinalResults{
		Recipients: []game.FinalRecipient{
			{Phone: "+972500000001", IsWinner: true, Rank: 1, Score: 300},
			{Phone: "+972500000002", Rank: 2, Score: 200},
		},
		WinnerNames: []string{"David Cohen"},
		WinnerScore: 300,
	}

	n.DispatchGameFinished("game-1", results)

	// The non-winner's single message must carry THEIR rank and score,
	// not the winner's.
	got := bodiesFor(replier.calls, "+972500000002")
	if len(got) != 1 {
		t.Fatalf("non-winner got %d messages, want exactly 1", len(got))
	}
	want := finalResultsMessage([]string{"David Cohen"}, 300, 2, 200)
	if got[0] != want {
		t.Errorf("non-winner body = %q, want %q", got[0], want)
	}
}

func TestDispatchGameFinishedSpectatorGetsWinnerLineOnly(t *testing.T) {
	replier := &stubReplier{}
	n := NewFinalNotifier(replier, nil)
	results := game.FinalResults{
		Recipients: []game.FinalRecipient{
			{Phone: "+972500000001", IsWinner: true, Rank: 1, Score: 300},
			{Phone: "+972500000009", IsSpectator: true},
		},
		WinnerNames: []string{"David Cohen"},
		WinnerScore: 300,
	}

	n.DispatchGameFinished("game-1", results)

	got := bodiesFor(replier.calls, "+972500000009")
	if len(got) != 1 {
		t.Fatalf("spectator got %d messages, want exactly 1", len(got))
	}
	want := finalResultsSpectatorMessage([]string{"David Cohen"}, 300)
	if got[0] != want {
		t.Errorf("spectator body = %q, want the winner line alone %q", got[0], want)
	}
	// The personal-placing line is the second line of the player
	// template; a spectator has no rank or score, so it must be absent.
	if strings.Contains(got[0], "\n") {
		t.Errorf("spectator body = %q, want no personal-placing line", got[0])
	}
}

func TestDispatchGameFinishedWinnerGetsBothMessagesFinalResultsFirst(t *testing.T) {
	replier := &stubReplier{}
	n := NewFinalNotifier(replier, nil)
	results := game.FinalResults{
		Recipients: []game.FinalRecipient{
			{Phone: "+972500000001", IsWinner: true, Rank: 1, Score: 300},
			{Phone: "+972500000002", Rank: 2, Score: 200},
		},
		WinnerNames: []string{"David Cohen"},
		WinnerScore: 300,
	}

	n.DispatchGameFinished("game-1", results)

	got := bodiesFor(replier.calls, "+972500000001")
	if len(got) != 2 {
		t.Fatalf("winner got %d messages, want exactly 2 (final results + the winner variant)", len(got))
	}
	if want := finalResultsMessage([]string{"David Cohen"}, 300, 1, 300); got[0] != want {
		t.Errorf("winner's first body = %q, want the final-results message %q", got[0], want)
	}
	if want := winnerFinalMessage([]string{"David Cohen"}, 300); got[1] != want {
		t.Errorf("winner's second body = %q, want the winner message %q", got[1], want)
	}
}

func TestDispatchGameFinishedTieUsesPluralFormsForEveryoneAndBothWinners(t *testing.T) {
	replier := &stubReplier{}
	n := NewFinalNotifier(replier, nil)
	winners := []string{"David Cohen", "Rachel Levi"}
	results := game.FinalResults{
		Recipients: []game.FinalRecipient{
			{Phone: "+972500000001", IsWinner: true, Rank: 1, Score: 250},
			{Phone: "+972500000002", IsWinner: true, Rank: 1, Score: 250},
			{Phone: "+972500000003", Rank: 3, Score: 100},
			{Phone: "+972500000009", IsSpectator: true},
		},
		WinnerNames: winners,
		WinnerScore: 250,
	}

	n.DispatchGameFinished("game-1", results)

	// Every player's final-results body uses the tie template, with their
	// own placing.
	for _, tc := range []struct {
		phone string
		rank  int
		score int32
	}{
		{"+972500000001", 1, 250},
		{"+972500000002", 1, 250},
		{"+972500000003", 3, 100},
	} {
		got := bodiesFor(replier.calls, tc.phone)
		if len(got) == 0 {
			t.Fatalf("%s got no messages", tc.phone)
		}
		if want := finalResultsTieMessage(winners, 250, tc.rank, tc.score); got[0] != want {
			t.Errorf("%s final-results body = %q, want the tie form %q", tc.phone, got[0], want)
		}
	}
	if got := bodiesFor(replier.calls, "+972500000009"); len(got) != 1 || got[0] != finalResultsSpectatorTieMessage(winners, 250) {
		t.Errorf("spectator bodies = %q, want exactly the spectator tie form", got)
	}

	// Both winners additionally get the jointly-addressed tie winner
	// message; the third player gets nothing extra.
	wantWinner := winnerFinalTieMessage(winners, 250)
	for _, phone := range []string{"+972500000001", "+972500000002"} {
		got := bodiesFor(replier.calls, phone)
		if len(got) != 2 {
			t.Fatalf("tied winner %s got %d messages, want exactly 2", phone, len(got))
		}
		if got[1] != wantWinner {
			t.Errorf("tied winner %s second body = %q, want %q", phone, got[1], wantWinner)
		}
	}
	if got := bodiesFor(replier.calls, "+972500000003"); len(got) != 1 {
		t.Errorf("non-winner got %d messages, want exactly 1", len(got))
	}
}

func TestDispatchGameFinishedNoWinnerSendsNoWinnerCopyAndZeroWinnerMessages(t *testing.T) {
	replier := &stubReplier{}
	n := NewFinalNotifier(replier, nil)
	results := game.FinalResults{
		Recipients: []game.FinalRecipient{
			{Phone: "+972500000001", Rank: 1, Score: 0},
			{Phone: "+972500000002", Rank: 1, Score: 0},
			{Phone: "+972500000009", IsSpectator: true},
		},
		WinnerNames: []string{},
		WinnerScore: 0,
	}

	n.DispatchGameFinished("game-1", results)

	// Exactly one message per recipient proves the winner pass never ran.
	if len(replier.calls) != 3 {
		t.Fatalf("Enqueue called %d times, want exactly 3 — one per recipient, no winner messages", len(replier.calls))
	}
	want := finalResultsNoWinnerMessage()
	for _, c := range replier.calls {
		if c.body != want {
			t.Errorf("body for %s = %q, want the no-winner copy %q", c.to, c.body, want)
		}
	}
}

func TestDispatchGameFinishedSkipsBlankPhoneRecipientAndTrimsWhitespace(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	replier := &stubReplier{}
	n := NewFinalNotifier(replier, logger)
	results := game.FinalResults{
		Recipients: []game.FinalRecipient{
			{Phone: " +972500000001 ", Rank: 2, Score: 200},
			{Phone: "  ", Rank: 3, Score: 100},
			// A blank-phone WINNER: must be skipped in BOTH passes, and
			// must not produce a second WARN for the same recipient.
			{Phone: "", IsWinner: true, Rank: 1, Score: 300},
		},
		WinnerNames: []string{"David Cohen"},
		WinnerScore: 300,
	}

	n.DispatchGameFinished("game-1", results)

	if len(replier.calls) != 1 {
		t.Fatalf("Enqueue called %d times, want 1 (blank phones skipped in both passes)", len(replier.calls))
	}
	if replier.calls[0].to != "+972500000001" {
		t.Errorf("Enqueue called for %q, want the trimmed phone +972500000001", replier.calls[0].to)
	}
	if strings.Count(buf.String(), "level=WARN") != 2 {
		t.Errorf("log output = %q, want exactly 2 WARN lines — one per blank-phone recipient, not one per pass", buf.String())
	}
}

func TestDispatchGameFinishedEmptyRecipientsEnqueuesNothing(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	replier := &stubReplier{}
	n := NewFinalNotifier(replier, logger)

	n.DispatchGameFinished("game-1", game.FinalResults{})

	if len(replier.calls) != 0 {
		t.Fatalf("Enqueue called %d times, want 0", len(replier.calls))
	}
	if !strings.Contains(buf.String(), "recipient_count=0") {
		t.Errorf("log output = %q, want recipient_count=0", buf.String())
	}
	if !strings.Contains(buf.String(), "winner_count=0") {
		t.Errorf("log output = %q, want winner_count=0", buf.String())
	}
}
