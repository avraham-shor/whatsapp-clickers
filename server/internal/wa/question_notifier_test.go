package wa

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

func mcqQuestion() game.CurrentQuestion {
	return game.CurrentQuestion{
		ID:               "q1",
		Position:         1,
		Type:             questionTypeMCQ,
		Text:             "כמה זה 1+1?",
		Options:          []string{"אחת", "שתיים", "שלוש", "ארבע"},
		TimeLimitSeconds: 20,
	}
}

func freeTextQuestion() game.CurrentQuestion {
	return game.CurrentQuestion{
		ID:               "q2",
		Position:         2,
		Type:             questionTypeFreeText,
		Text:             "מה בירת ישראל?",
		TimeLimitSeconds: 30,
	}
}

func TestDispatchQuestionOpenedMCQEnqueuesSameBodyToEveryRecipient(t *testing.T) {
	replier := &stubReplier{}
	n := NewQuestionNotifier(replier, nil)
	recipients := []string{"+972500000001", "+972500000002"}

	n.DispatchQuestionOpened("game-1", mcqQuestion(), 5, recipients)

	if len(replier.calls) != 2 {
		t.Fatalf("Enqueue called %d times, want 2", len(replier.calls))
	}
	want := questionMCQMessage(1, 5, mcqQuestion().Text, mcqQuestion().Options, 20)
	for i, call := range replier.calls {
		if call.to != recipients[i] || call.body != want {
			t.Errorf("call[%d] = %+v, want to=%q body=%q", i, call, recipients[i], want)
		}
	}
}

func TestDispatchQuestionOpenedFreeTextEnqueuesSameBodyToEveryRecipient(t *testing.T) {
	replier := &stubReplier{}
	n := NewQuestionNotifier(replier, nil)
	recipients := []string{"+972500000001", "+972500000002"}

	n.DispatchQuestionOpened("game-1", freeTextQuestion(), 5, recipients)

	if len(replier.calls) != 2 {
		t.Fatalf("Enqueue called %d times, want 2", len(replier.calls))
	}
	want := questionFreeTextMessage(2, 5, freeTextQuestion().Text, 30)
	for i, call := range replier.calls {
		if call.to != recipients[i] || call.body != want {
			t.Errorf("call[%d] = %+v, want to=%q body=%q", i, call, recipients[i], want)
		}
	}
}

func TestDispatchQuestionOpenedSkipsBlankPhoneRecipientAndTrimsWhitespace(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	replier := &stubReplier{}
	n := NewQuestionNotifier(replier, logger)
	recipients := []string{" +972500000001 ", "  ", ""}

	n.DispatchQuestionOpened("game-1", mcqQuestion(), 5, recipients)

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

func TestDispatchQuestionOpenedMCQWrongOptionCountEnqueuesNothingAndLogsError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	replier := &stubReplier{}
	n := NewQuestionNotifier(replier, logger)
	q := mcqQuestion()
	q.Options = []string{"אחת", "שתיים", "שלוש"}

	n.DispatchQuestionOpened("game-1", q, 5, []string{"+972500000001"})

	if len(replier.calls) != 0 {
		t.Fatalf("Enqueue called %d times, want 0 for an mcq question with != 4 options", len(replier.calls))
	}
	if !strings.Contains(buf.String(), "level=ERROR") {
		t.Errorf("log output = %q, want an ERROR line", buf.String())
	}
}

func TestDispatchQuestionOpenedUnknownTypeEnqueuesNothingAndLogsError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	replier := &stubReplier{}
	n := NewQuestionNotifier(replier, logger)
	q := mcqQuestion()
	q.Type = "unknown_type"

	n.DispatchQuestionOpened("game-1", q, 5, []string{"+972500000001"})

	if len(replier.calls) != 0 {
		t.Fatalf("Enqueue called %d times, want 0 for an unrecognized question type", len(replier.calls))
	}
	if !strings.Contains(buf.String(), "level=ERROR") {
		t.Errorf("log output = %q, want an ERROR line", buf.String())
	}
}

func TestDispatchQuestionOpenedEmptyRecipientsLogsZeroCount(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	replier := &stubReplier{}
	n := NewQuestionNotifier(replier, logger)

	n.DispatchQuestionOpened("game-1", mcqQuestion(), 5, nil)

	if len(replier.calls) != 0 {
		t.Fatalf("Enqueue called %d times, want 0", len(replier.calls))
	}
	if !strings.Contains(buf.String(), "recipient_count=0") {
		t.Errorf("log output = %q, want recipient_count=0", buf.String())
	}
}
