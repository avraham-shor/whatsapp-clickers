package wa

import (
	"context"
	"strings"
	"testing"
)

// canonicalHelpCopy is the Help (Universal Reply) row from EXPERIENCE.md's
// "WhatsApp message templates" table, held verbatim and WITHOUT bidi isolates.
// helpMessage() adds the isolates; the fidelity test strips them back out and
// compares against this constant, so an editor mangling the mixed-direction
// literal in messages_he.go fails the build instead of shipping scrambled
// Hebrew as a Participant's first impression of the platform.
const canonicalHelpCopy = "כאן משחק החידון! כדי להצטרף שלחו: JOIN ואחריו הקוד (לדוגמה: JOIN COHEN24). את הקוד מקבלים מהמארגן."

// stubReplier captures Enqueue calls. Grown in-test, no mock framework —
// Handle is called synchronously by the test goroutine, so no mutex (unlike
// dispatch_test.go's stubSender, which worker goroutines write to).
type stubReplier struct {
	calls []stubReply
}

type stubReply struct {
	to   string
	body string
}

func (s *stubReplier) Enqueue(to, body string) {
	s.calls = append(s.calls, stubReply{to: to, body: body})
}

// lriMark / pdiMark mirror messages_he.go's isolate constants. Written as \u
// escapes here for the same reason they are there: pasted, they are invisible
// in a diff and a stray copy-paste silently drops them.
const (
	lriMark = "\u2066" // LEFT-TO-RIGHT ISOLATE
	pdiMark = "\u2069" // POP DIRECTIONAL ISOLATE
)

// --- AC-1/AC-2/AC-3: the Universal Reply matrix (test-design scenario A7) ---

// TestInboundUniversalReplyMatrix walks every inbound class the platform can
// receive and asserts each one produces exactly one Help reply to its sender.
// This is AC-2's "no inbound message class results in silence" as executable
// spec: a new class that fails to reply fails here.
func TestInboundUniversalReplyMatrix(t *testing.T) {
	const sender = "972501234567"

	cases := []struct {
		name     string
		msg      InboundMessage
		wantKind inboundKind
	}{
		{"plain Hebrew noise", InboundMessage{From: sender, Type: "text", TextBody: "מה זה כאן?"}, kindText},
		{"plain Latin noise", InboundMessage{From: sender, Type: "text", TextBody: "hello?"}, kindText},
		{"single emoji", InboundMessage{From: sender, Type: "text", TextBody: "🎉"}, kindText},
		// JOIN and "שם:" are unrecognized text in 2.2 — story 2.4 gives them
		// meaning. Pinned so a later reader does not read this as a bug.
		{"JOIN code (unrecognized until 2.4)", InboundMessage{From: sender, Type: "text", TextBody: "JOIN ABC123"}, kindText},
		{"name change (unrecognized until 2.4)", InboundMessage{From: sender, Type: "text", TextBody: "שם: רחל"}, kindText},
		{"empty body", InboundMessage{From: sender, Type: "text", TextBody: ""}, kindEmpty},
		{"whitespace-only body", InboundMessage{From: sender, Type: "text", TextBody: "   \t\n  "}, kindEmpty},
		{"image", InboundMessage{From: sender, Type: "image"}, kindNonText},
		{"sticker", InboundMessage{From: sender, Type: "sticker"}, kindNonText},
		{"audio", InboundMessage{From: sender, Type: "audio"}, kindNonText},
		{"video", InboundMessage{From: sender, Type: "video"}, kindNonText},
		{"document", InboundMessage{From: sender, Type: "document"}, kindNonText},
		{"location", InboundMessage{From: sender, Type: "location"}, kindNonText},
		{"contacts", InboundMessage{From: sender, Type: "contacts"}, kindNonText},
		{"reaction", InboundMessage{From: sender, Type: "reaction"}, kindNonText},
		{"unknown future type", InboundMessage{From: sender, Type: "some_future_type"}, kindNonText},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replier := &stubReplier{}
			logger, buf := newTestLogger()
			r := NewInboundRouter(replier, logger)

			r.Handle(context.Background(), tc.msg)

			if len(replier.calls) != 1 {
				t.Fatalf("Enqueue called %d times, want exactly 1", len(replier.calls))
			}
			if replier.calls[0].to != sender {
				t.Errorf("reply addressed to %q, want %q", replier.calls[0].to, sender)
			}
			if replier.calls[0].body != helpMessage() {
				t.Errorf("reply body = %q, want the canonical Help copy", replier.calls[0].body)
			}
			if got := classify(tc.msg); got != tc.wantKind {
				t.Errorf("classify = %q, want %q", got, tc.wantKind)
			}
			logs := buf.String()
			if !strings.Contains(logs, "universal reply queued") {
				t.Errorf("missing the routing INFO line: %s", logs)
			}
			if !strings.Contains(logs, "kind="+string(tc.wantKind)) {
				t.Errorf("INFO line missing kind=%s: %s", tc.wantKind, logs)
			}
		})
	}
}

// --- AC-3: totality, enforced structurally rather than promised ---

// TestReplyForIsTotalOverRegistry is the guard that makes AC-3 mechanical: a
// story adding a kind to allInboundKinds without teaching replyFor about it
// fails here rather than shipping a silent branch.
func TestReplyForIsTotalOverRegistry(t *testing.T) {
	if len(allInboundKinds) == 0 {
		t.Fatal("the kind registry is empty — this test would prove nothing")
	}
	for _, kind := range allInboundKinds {
		if got := replyFor(kind); got == "" {
			t.Errorf("replyFor(%q) returned empty copy — that branch goes silent", kind)
		}
	}
}

// TestReplyForUnregisteredKindStillReplies pins the belt-and-braces default:
// even a kind that never joined the registry gets a reply rather than silence.
func TestReplyForUnregisteredKindStillReplies(t *testing.T) {
	if got := replyFor(inboundKind("never_registered")); got == "" {
		t.Error("replyFor's default branch returned empty copy — the chat would go silent")
	}
}

func TestClassifyAlwaysReturnsRegisteredKind(t *testing.T) {
	msgs := []InboundMessage{
		{Type: "text", TextBody: "hi"},
		{Type: "text", TextBody: ""},
		{Type: "text", TextBody: " "}, // non-breaking space
		{Type: "sticker"},
		{Type: ""},
		{Type: "some_future_type", TextBody: "ignored for non-text"},
	}
	for _, msg := range msgs {
		got := classify(msg)
		registered := false
		for _, kind := range allInboundKinds {
			if got == kind {
				registered = true
				break
			}
		}
		if !registered {
			t.Errorf("classify(%+v) = %q, which is not in allInboundKinds", msg, got)
		}
	}
}

// --- AC-4: copy fidelity and bidi isolation ---

func TestHelpMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := strings.NewReplacer(lriMark, "", pdiMark, "").Replace(helpMessage())
	if stripped != canonicalHelpCopy {
		t.Errorf("Help copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, canonicalHelpCopy)
	}
}

// TestHelpMessageIsolatesLTRTokens proves the isolates are present AND
// correctly placed, which the strip test above cannot: without them WhatsApp's
// bidi algorithm reorders the example code and the Participant reads garbage.
func TestHelpMessageIsolatesLTRTokens(t *testing.T) {
	got := helpMessage()
	for _, token := range []string{"JOIN", "JOIN COHEN24"} {
		isolated := lriMark + token + pdiMark
		if !strings.Contains(got, isolated) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", token, got)
		}
	}
}

// --- AC-5: the only silent path, and log discipline ---

func TestInboundEmptySenderDoesNotReply(t *testing.T) {
	replier := &stubReplier{}
	logger, buf := newTestLogger()
	r := NewInboundRouter(replier, logger)

	r.Handle(context.Background(), InboundMessage{From: "", Type: "text", TextBody: "hello", WaMessageID: leakyWamid})

	if len(replier.calls) != 0 {
		t.Errorf("Enqueue called %d times for a message with no sender, want 0", len(replier.calls))
	}
	if logs := buf.String(); !strings.Contains(logs, "inbound message has no sender") {
		t.Errorf("missing the no-sender WARN: %s", logs)
	}
}

// TestInboundLogDisciplineRedactsPII uses a realistic MSISDN and a realistic
// wamid shape on purpose: 2.1's wamid leak survived review precisely because
// every synthetic fixture was an arbitrary string that hid it (NFR-4).
func TestInboundLogDisciplineRedactsPII(t *testing.T) {
	replier := &stubReplier{}
	logger, buf := newTestLogger()
	r := NewInboundRouter(replier, logger)

	r.Handle(context.Background(), InboundMessage{
		From:        syntheticMSISDN,
		Type:        "text",
		TextBody:    "hello",
		WaMessageID: leakyWamid,
	})

	logs := buf.String()
	if strings.Contains(logs, syntheticMSISDN) {
		t.Errorf("log leaked the full MSISDN: %s", logs)
	}
	if strings.Contains(logs, leakyWamid) || strings.Contains(logs, syntheticMSISDNBase64) {
		t.Errorf("log leaked the raw wamid (phone recoverable via base64): %s", logs)
	}
	if !strings.Contains(logs, "phone_last4="+PhoneLast4(syntheticMSISDN)) {
		t.Errorf("log missing phone_last4: %s", logs)
	}
	if !strings.Contains(logs, "wa_message_id="+WaMessageIDDigest(leakyWamid)) {
		t.Errorf("log missing the wamid digest: %s", logs)
	}
	if strings.Contains(logs, "level=ERROR") {
		t.Errorf("healthy reply path emitted an ERROR line (NFR-8): %s", logs)
	}
}

// TestInboundRouterSatisfiesInboundHandler documents at runtime what the
// compile-time assertion in inbound.go enforces: the router is exactly the
// seam webhook.go calls.
func TestInboundRouterSatisfiesInboundHandler(t *testing.T) {
	var h InboundHandler = NewInboundRouter(&stubReplier{}, nil)
	if h == nil {
		t.Fatal("InboundRouter does not satisfy InboundHandler")
	}
}

// TestDispatcherSatisfiesReplier pins the production wiring: main.go passes a
// *Dispatcher where a Replier is required.
func TestDispatcherSatisfiesReplier(t *testing.T) {
	var _ Replier = NewDispatcher(&stubSender{failUntilAttempt: -1}, nil)
}
