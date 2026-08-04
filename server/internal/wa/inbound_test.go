package wa

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
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

// stubRegistrar canned-answers Join/Rename and records the arguments it was
// called with, same shape as stubReplier.
type stubRegistrar struct {
	joinResult  game.JoinResult
	joinErr     error
	joinCalls   int
	joinCode    string
	joinPhone   string
	joinProfile string

	renameResult game.RenameResult
	renameErr    error
	renameCalls  int
	renamePhone  string
	renameName   string

	recordAnswerResult     game.AnswerResult
	recordAnswerErr        error
	recordAnswerCalls      int
	recordAnswerPhone      string
	recordAnswerRawText    string
	recordAnswerReceivedAt time.Time
}

func (s *stubRegistrar) Join(ctx context.Context, joinCode, phone, profileName string) (game.JoinResult, error) {
	s.joinCalls++
	s.joinCode = joinCode
	s.joinPhone = phone
	s.joinProfile = profileName
	return s.joinResult, s.joinErr
}

func (s *stubRegistrar) Rename(ctx context.Context, phone, displayName string) (game.RenameResult, error) {
	s.renameCalls++
	s.renamePhone = phone
	s.renameName = displayName
	return s.renameResult, s.renameErr
}

func (s *stubRegistrar) RecordAnswer(ctx context.Context, phone, rawText string, receivedAt time.Time) (game.AnswerResult, error) {
	s.recordAnswerCalls++
	s.recordAnswerPhone = phone
	s.recordAnswerRawText = rawText
	s.recordAnswerReceivedAt = receivedAt
	return s.recordAnswerResult, s.recordAnswerErr
}

// stubBroadcaster records Broadcast calls.
type stubBroadcaster struct {
	calls []stubBroadcast
}

type stubBroadcast struct {
	gameID   string
	snapshot game.Snapshot
}

func (s *stubBroadcaster) Broadcast(gameID string, snapshot game.Snapshot) {
	s.calls = append(s.calls, stubBroadcast{gameID: gameID, snapshot: snapshot})
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
			// recordAnswerErr = ErrNoOpenQuestion: none of these scenarios set
			// up an open question, so kindText messages must fall through to
			// exactly today's Universal Reply behavior (see handleTextOrAnswer).
			r := NewInboundRouter(replier, &stubRegistrar{recordAnswerErr: game.ErrNoOpenQuestion}, &stubBroadcaster{}, logger)

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
		{Type: "text", TextBody: "JOIN COHEN24"},
		{Type: "text", TextBody: "שם: רחל לוי"},
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

// --- Parser unit cases: parseJoinCode / parseRenameName / classify ---

func TestParseJoinCode(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode string
		wantOK   bool
	}{
		{"canonical", "JOIN COHEN24", "COHEN24", true},
		{"lowercase keyword", "join cohen24", "COHEN24", true},
		{"extra whitespace", "  JOIN   cohen24  ", "COHEN24", true},
		{"trailing words ignored", "JOIN COHEN24 please", "COHEN24", true},
		{"no space, not recognized", "JOINCOHEN24", "", false},
		{"no code, not recognized", "JOIN", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, ok := parseJoinCode(tc.body)
			if ok != tc.wantOK || code != tc.wantCode {
				t.Errorf("parseJoinCode(%q) = (%q, %v), want (%q, %v)", tc.body, code, ok, tc.wantCode, tc.wantOK)
			}
			wantKind := kindText
			if tc.wantOK {
				wantKind = kindJoin
			}
			if got := classify(InboundMessage{Type: "text", TextBody: tc.body}); got != wantKind {
				t.Errorf("classify(%q) = %q, want %q", tc.body, got, wantKind)
			}
		})
	}
}

func TestParseRenameName(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantName string
		wantOK   bool
	}{
		{"canonical", "שם: רחל לוי", "רחל לוי", true},
		{"no space after colon", "שם:רחל", "רחל", true},
		{"leading whitespace before prefix tolerated", "  שם: רחל", "רחל", true},
		{"empty name, not recognized", "שם:", "", false},
		{"whitespace-only name, not recognized", "שם:   ", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, ok := parseRenameName(tc.body)
			if ok != tc.wantOK || name != tc.wantName {
				t.Errorf("parseRenameName(%q) = (%q, %v), want (%q, %v)", tc.body, name, ok, tc.wantName, tc.wantOK)
			}
			wantKind := kindText
			if tc.wantOK {
				wantKind = kindRename
			}
			if got := classify(InboundMessage{Type: "text", TextBody: tc.body}); got != wantKind {
				t.Errorf("classify(%q) = %q, want %q", tc.body, got, wantKind)
			}
		})
	}
}

// --- JOIN dispatch ---

const joinSender = "972501234567"

func TestHandleJoinWelcomeBroadcastsWhenCreatedWithSnapshot(t *testing.T) {
	replier := &stubReplier{}
	broadcaster := &stubBroadcaster{}
	snap := game.Snapshot{GameID: "game-1", ParticipantCount: 1}
	registrar := &stubRegistrar{joinResult: game.JoinResult{Outcome: game.JoinWelcome, GameID: "game-1", DisplayName: "דנה", Created: true, Snapshot: snap}}
	r := NewInboundRouter(replier, registrar, broadcaster, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "JOIN COHEN24", ProfileName: "דנה"})

	if len(replier.calls) != 1 || replier.calls[0].body != welcomeMessage("דנה") {
		t.Fatalf("reply = %+v, want exactly one Welcome message", replier.calls)
	}
	if len(broadcaster.calls) != 1 || broadcaster.calls[0].gameID != "game-1" {
		t.Errorf("broadcast calls = %+v, want exactly one Broadcast to game-1", broadcaster.calls)
	}
	if registrar.joinCode != "COHEN24" {
		t.Errorf("registrar.Join called with code %q, want %q", registrar.joinCode, "COHEN24")
	}
}

func TestHandleJoinIdempotentRepeatSendsWelcomeWithoutBroadcast(t *testing.T) {
	replier := &stubReplier{}
	broadcaster := &stubBroadcaster{}
	registrar := &stubRegistrar{joinResult: game.JoinResult{Outcome: game.JoinWelcome, GameID: "game-1", DisplayName: "דנה", Created: false}}
	r := NewInboundRouter(replier, registrar, broadcaster, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "JOIN COHEN24"})

	if len(replier.calls) != 1 || replier.calls[0].body != welcomeMessage("דנה") {
		t.Fatalf("reply = %+v, want the same Welcome copy", replier.calls)
	}
	if len(broadcaster.calls) != 0 {
		t.Errorf("broadcast calls = %+v, want none (idempotent repeat)", broadcaster.calls)
	}
}

func TestHandleJoinPreLobbyDoesNotBroadcast(t *testing.T) {
	replier := &stubReplier{}
	broadcaster := &stubBroadcaster{}
	registrar := &stubRegistrar{joinResult: game.JoinResult{Outcome: game.JoinPreLobby, GameID: "game-1"}}
	r := NewInboundRouter(replier, registrar, broadcaster, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "JOIN COHEN24"})

	if len(replier.calls) != 1 || replier.calls[0].body != preLobbyMessage() {
		t.Fatalf("reply = %+v, want the Pre-lobby message", replier.calls)
	}
	if len(broadcaster.calls) != 0 {
		t.Errorf("broadcast calls = %+v, want none", broadcaster.calls)
	}
}

func TestHandleJoinInvalidCodeSendsInvalidCodeMessageWithUppercasedCode(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{joinErr: game.ErrInvalidJoinCode}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "join wrongcode"})

	if len(replier.calls) != 1 || replier.calls[0].body != invalidCodeMessage("WRONGCODE") {
		t.Fatalf("reply = %+v, want the Invalid-code message with the uppercased code", replier.calls)
	}
}

func TestHandleJoinSpectatorSendsSpectatorNotice(t *testing.T) {
	replier := &stubReplier{}
	broadcaster := &stubBroadcaster{}
	registrar := &stubRegistrar{joinResult: game.JoinResult{Outcome: game.JoinSpectator, GameID: "game-1", DisplayName: "דנה", Created: true}}
	r := NewInboundRouter(replier, registrar, broadcaster, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "JOIN COHEN24"})

	if len(replier.calls) != 1 || replier.calls[0].body != spectatorNoticeMessage() {
		t.Fatalf("reply = %+v, want the Spectator-notice message", replier.calls)
	}
	if len(broadcaster.calls) != 0 {
		t.Errorf("broadcast calls = %+v, want none (spectator join never broadcasts)", broadcaster.calls)
	}
}

func TestHandleJoinFinishedGameSendsHelp(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{joinErr: game.ErrGameFinished}
	logger, buf := newTestLogger()
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, logger)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "JOIN COHEN24"})

	if len(replier.calls) != 1 || replier.calls[0].body != helpMessage() {
		t.Fatalf("reply = %+v, want the Help message", replier.calls)
	}
	if strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("ErrGameFinished is an expected reply path, not a failure — must not WARN: %s", buf.String())
	}
}

func TestHandleJoinUnexpectedErrorSendsHelpAndWarns(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{joinErr: errors.New("boom")}
	logger, buf := newTestLogger()
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, logger)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "JOIN COHEN24"})

	if len(replier.calls) != 1 || replier.calls[0].body != helpMessage() {
		t.Fatalf("reply = %+v, want the Help message", replier.calls)
	}
	if !strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("an unexpected registrar error must WARN: %s", buf.String())
	}
}

// --- שם: (rename) dispatch ---

func TestHandleRenameSuccessSendsNameUpdatedMessage(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{renameResult: game.RenameResult{DisplayName: "דוד"}}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "שם: דוד"})

	if len(replier.calls) != 1 || replier.calls[0].body != nameUpdatedMessage("דוד") {
		t.Fatalf("reply = %+v, want the Name-updated message", replier.calls)
	}
	if registrar.renameName != "דוד" {
		t.Errorf("registrar.Rename called with name %q, want %q", registrar.renameName, "דוד")
	}
}

func TestHandleRenameUnregisteredSenderSendsHelp(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{renameErr: game.ErrParticipantNotFound}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "שם: דוד"})

	if len(replier.calls) != 1 || replier.calls[0].body != helpMessage() {
		t.Fatalf("reply = %+v, want the Help message", replier.calls)
	}
}

// --- Answer intake dispatch (story 3.3) ---

func TestHandleAnswerAcceptedSendsAck(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: game.AnswerAccepted}}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "1"})

	if len(replier.calls) != 1 || replier.calls[0].body != ackMessage() {
		t.Fatalf("reply = %+v, want the Acknowledgment message", replier.calls)
	}
}

// TestHandleAnswerAcceptedBroadcastsSnapshot covers the review finding: an
// accepted answer must broadcast the fresh snapshot so the control panel's
// live answered-count (AC-5) updates without waiting on an unrelated event.
func TestHandleAnswerAcceptedBroadcastsSnapshot(t *testing.T) {
	replier := &stubReplier{}
	snap := game.Snapshot{GameID: "game-1"}
	registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: game.AnswerAccepted, GameID: "game-1", Snapshot: snap}}
	broadcaster := &stubBroadcaster{}
	r := NewInboundRouter(replier, registrar, broadcaster, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "1"})

	if len(broadcaster.calls) != 1 || broadcaster.calls[0].gameID != "game-1" || broadcaster.calls[0].snapshot.GameID != "game-1" {
		t.Fatalf("broadcaster.calls = %+v, want one Broadcast(%q, snapshot)", broadcaster.calls, "game-1")
	}
}

// TestHandleAnswerAcceptedSkipsBroadcastWhenSnapshotEmpty covers the
// degrade path: game.RecordAnswer returns a zero Snapshot when the
// post-answer build failed (already logged there), and handleTextOrAnswer
// must not broadcast an empty/wrong snapshot in that case.
func TestHandleAnswerAcceptedSkipsBroadcastWhenSnapshotEmpty(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: game.AnswerAccepted}}
	broadcaster := &stubBroadcaster{}
	r := NewInboundRouter(replier, registrar, broadcaster, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "1"})

	if len(broadcaster.calls) != 0 {
		t.Errorf("broadcaster.calls = %+v, want none", broadcaster.calls)
	}
}

// TestHandleAnswerNonAcceptedOutcomesNeverBroadcast pins that only
// AnswerAccepted triggers a broadcast — the other four outcomes never
// change the answered-count, so there is nothing to broadcast.
func TestHandleAnswerNonAcceptedOutcomesNeverBroadcast(t *testing.T) {
	outcomes := []game.AnswerOutcome{game.AnswerAlreadyAnswered, game.AnswerFormatHint, game.AnswerTooLong, game.AnswerClosed}
	for _, outcome := range outcomes {
		t.Run(string(outcome), func(t *testing.T) {
			replier := &stubReplier{}
			registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: outcome}}
			broadcaster := &stubBroadcaster{}
			r := NewInboundRouter(replier, registrar, broadcaster, nil)

			r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "1"})

			if len(broadcaster.calls) != 0 {
				t.Errorf("broadcaster.calls = %+v, want none for outcome %q", broadcaster.calls, outcome)
			}
		})
	}
}

func TestHandleAnswerAlreadyAnsweredSendsAlreadyAnswered(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: game.AnswerAlreadyAnswered}}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "2"})

	if len(replier.calls) != 1 || replier.calls[0].body != alreadyAnsweredMessage() {
		t.Fatalf("reply = %+v, want the Already-answered message", replier.calls)
	}
}

func TestHandleAnswerFormatHintSendsFormatHint(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: game.AnswerFormatHint}}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "5"})

	if len(replier.calls) != 1 || replier.calls[0].body != formatHintMessage() {
		t.Fatalf("reply = %+v, want the Format-hint message", replier.calls)
	}
}

func TestHandleAnswerTooLongSendsTooLong(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: game.AnswerTooLong}}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: strings.Repeat("א", 201)})

	if len(replier.calls) != 1 || replier.calls[0].body != tooLongMessage() {
		t.Fatalf("reply = %+v, want the Too-long message", replier.calls)
	}
}

func TestHandleAnswerClosedSendsQuestionClosed(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: game.AnswerClosed}}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "1"})

	if len(replier.calls) != 1 || replier.calls[0].body != questionClosedMessage() {
		t.Fatalf("reply = %+v, want the Question-closed message", replier.calls)
	}
}

// TestHandleTextFallsThroughToHelpWhenNoOpenQuestion proves kindText's
// pre-3.3 behavior is preserved for everyone not mid-question.
func TestHandleTextFallsThroughToHelpWhenNoOpenQuestion(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerErr: game.ErrNoOpenQuestion}
	logger, buf := newTestLogger()
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, logger)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "מה זה כאן?"})

	if len(replier.calls) != 1 || replier.calls[0].body != helpMessage() {
		t.Fatalf("reply = %+v, want the Help message", replier.calls)
	}
	if !strings.Contains(buf.String(), "universal reply queued") {
		t.Errorf("missing the routing INFO line: %s", buf.String())
	}
}

func TestHandleTextDegradesToHelpOnUnexpectedError(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerErr: errors.New("boom")}
	logger, buf := newTestLogger()
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, logger)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "1"})

	if len(replier.calls) != 1 || replier.calls[0].body != helpMessage() {
		t.Fatalf("reply = %+v, want the Help message", replier.calls)
	}
	if !strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("an unexpected registrar error must WARN: %s", buf.String())
	}
}

func TestHandleTextPassesReceivedAtToRegistrar(t *testing.T) {
	replier := &stubReplier{}
	registrar := &stubRegistrar{recordAnswerResult: game.AnswerResult{Outcome: game.AnswerAccepted}}
	r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)
	receivedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	r.Handle(context.Background(), InboundMessage{From: joinSender, Type: "text", TextBody: "1", ReceivedAt: receivedAt})

	if !registrar.recordAnswerReceivedAt.Equal(receivedAt) {
		t.Errorf("ReceivedAt passed to registrar = %v, want %v (must originate from the webhook layer, not time.Now())", registrar.recordAnswerReceivedAt, receivedAt)
	}
}

// TestNonTextKindsNeverReachRecordAnswer pins the routing-order decision:
// JOIN/rename keep priority over answer detection, and media/empty
// messages are never answer attempts — only kindText reaches RecordAnswer.
func TestNonTextKindsNeverReachRecordAnswer(t *testing.T) {
	cases := []struct {
		name string
		msg  InboundMessage
	}{
		{"join", InboundMessage{From: joinSender, Type: "text", TextBody: "JOIN COHEN24"}},
		{"rename", InboundMessage{From: joinSender, Type: "text", TextBody: "שם: דוד"}},
		{"empty", InboundMessage{From: joinSender, Type: "text", TextBody: ""}},
		{"non-text", InboundMessage{From: joinSender, Type: "image"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replier := &stubReplier{}
			registrar := &stubRegistrar{}
			r := NewInboundRouter(replier, registrar, &stubBroadcaster{}, nil)

			r.Handle(context.Background(), tc.msg)

			if registrar.recordAnswerCalls != 0 {
				t.Errorf("RecordAnswer called %d times for %s, want 0", registrar.recordAnswerCalls, tc.name)
			}
		})
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
	r := NewInboundRouter(replier, &stubRegistrar{}, &stubBroadcaster{}, logger)

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
	r := NewInboundRouter(replier, &stubRegistrar{}, &stubBroadcaster{}, logger)

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
	var h InboundHandler = NewInboundRouter(&stubReplier{}, &stubRegistrar{}, &stubBroadcaster{}, nil)
	if h == nil {
		t.Fatal("InboundRouter does not satisfy InboundHandler")
	}
}

// TestDispatcherSatisfiesReplier pins the production wiring: main.go passes a
// *Dispatcher where a Replier is required.
func TestDispatcherSatisfiesReplier(t *testing.T) {
	var _ Replier = NewDispatcher(&stubSender{failUntilAttempt: -1}, nil)
}
