package wa

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

func TestDispatchAnswerRevealedCorrectNoBonusEnqueuesCorrectMessage(t *testing.T) {
	replier := &stubReplier{}
	n := NewResultNotifier(replier, nil)
	results := []game.PersonalResult{
		{Phone: "+972500000001", IsCorrect: true, BasePoints: 100, BonusPoints: 0, Rank: 2},
	}

	n.DispatchAnswerRevealed("game-1", results, "ירושלים", false)

	if len(replier.calls) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(replier.calls))
	}
	want := resultCorrectMessage(100, 2)
	if replier.calls[0].to != "+972500000001" || replier.calls[0].body != want {
		t.Errorf("call = %+v, want to=%q body=%q", replier.calls[0], "+972500000001", want)
	}
}

func TestDispatchAnswerRevealedCorrectWithBonusEnqueuesBonusMessage(t *testing.T) {
	replier := &stubReplier{}
	n := NewResultNotifier(replier, nil)
	results := []game.PersonalResult{
		{Phone: "+972500000001", IsCorrect: true, BasePoints: 100, BonusPoints: 50, Rank: 1},
	}

	n.DispatchAnswerRevealed("game-1", results, "ירושלים", false)

	if len(replier.calls) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(replier.calls))
	}
	want := resultCorrectBonusMessage(100, 50, 1)
	if replier.calls[0].to != "+972500000001" || replier.calls[0].body != want {
		t.Errorf("call = %+v, want to=%q body=%q", replier.calls[0], "+972500000001", want)
	}
}

func TestDispatchAnswerRevealedWrongMidGameEnqueuesWrongMessageWithContinuationLine(t *testing.T) {
	replier := &stubReplier{}
	n := NewResultNotifier(replier, nil)
	results := []game.PersonalResult{
		{Phone: "+972500000001", IsCorrect: false, BasePoints: 0, BonusPoints: 0, Rank: 4},
	}

	n.DispatchAnswerRevealed("game-1", results, "ירושלים", false)

	if len(replier.calls) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(replier.calls))
	}
	want := resultWrongMessage("ירושלים", 4)
	if replier.calls[0].to != "+972500000001" || replier.calls[0].body != want {
		t.Errorf("call = %+v, want to=%q body=%q", replier.calls[0], "+972500000001", want)
	}
}

func TestDispatchAnswerRevealedWrongLastQuestionEnqueuesWrongMessageWithoutContinuationLine(t *testing.T) {
	replier := &stubReplier{}
	n := NewResultNotifier(replier, nil)
	results := []game.PersonalResult{
		{Phone: "+972500000001", IsCorrect: false, BasePoints: 0, BonusPoints: 0, Rank: 4},
	}

	n.DispatchAnswerRevealed("game-1", results, "ירושלים", true)

	if len(replier.calls) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(replier.calls))
	}
	want := resultWrongLastMessage("ירושלים", 4)
	if replier.calls[0].to != "+972500000001" || replier.calls[0].body != want {
		t.Errorf("call = %+v, want to=%q body=%q", replier.calls[0], "+972500000001", want)
	}
}

func TestDispatchAnswerRevealedSkipsBlankPhoneRecipientAndTrimsWhitespace(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	replier := &stubReplier{}
	n := NewResultNotifier(replier, logger)
	results := []game.PersonalResult{
		{Phone: " +972500000001 ", IsCorrect: true, BasePoints: 100, BonusPoints: 0, Rank: 1},
		{Phone: "  ", IsCorrect: true, BasePoints: 100, BonusPoints: 0, Rank: 2},
		{Phone: "", IsCorrect: false, BasePoints: 0, BonusPoints: 0, Rank: 3},
	}

	n.DispatchAnswerRevealed("game-1", results, "ירושלים", false)

	if len(replier.calls) != 1 {
		t.Fatalf("Enqueue called %d times, want 1 (blank phones skipped)", len(replier.calls))
	}
	if replier.calls[0].to != "+972500000001" {
		t.Errorf("Enqueue called for %q, want the trimmed phone +972500000001", replier.calls[0].to)
	}
	if !strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("log output = %q, want a WARN line for the 2 skipped blank phones", buf.String())
	}
}

func TestDispatchAnswerRevealedEmptyResultsLogsZeroCountAndEnqueuesNothing(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	replier := &stubReplier{}
	n := NewResultNotifier(replier, logger)

	n.DispatchAnswerRevealed("game-1", nil, "ירושלים", false)

	if len(replier.calls) != 0 {
		t.Fatalf("Enqueue called %d times, want 0", len(replier.calls))
	}
	if !strings.Contains(buf.String(), "recipient_count=0") {
		t.Errorf("log output = %q, want recipient_count=0", buf.String())
	}
}
