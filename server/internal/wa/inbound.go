package wa

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

// Replier queues an outbound message for delivery. Consumer-defined (the
// Deduper/SenderClient precedent) so the router is testable without a live
// Dispatcher; *Dispatcher satisfies it.
type Replier interface {
	Enqueue(to, body string)
}

// Registrar is the game-registration surface handleJoin/handleRename need;
// *game.Engine satisfies it. This is the first place wa imports game — the
// architecture sanctions it ("wa... call[s] engine methods"), and
// ws/handler.go's SnapshotReader already imports game.Snapshot the same way.
// game never imports wa (dependency direction is one-way).
type Registrar interface {
	Join(ctx context.Context, joinCode, phone, profileName string) (game.JoinResult, error)
	Rename(ctx context.Context, phone, displayName string) (game.RenameResult, error)
	RecordAnswer(ctx context.Context, phone, rawText string, receivedAt time.Time) (game.AnswerResult, error)
}

var _ Registrar = (*game.Engine)(nil)

// SnapshotBroadcaster is the fan-out surface handleJoin needs; *ws.Hub
// satisfies it. Structurally identical to httpapi.control.go's interface of
// the same name — redeclared locally per this codebase's established
// "each consumer defines its own narrow interface" convention.
type SnapshotBroadcaster interface {
	Broadcast(gameID string, snapshot game.Snapshot)
}

// inboundKind is the parse taxonomy. FR-2 requires that every inbound message
// gets an answer, so the taxonomy is total by construction: classify maps
// every possible InboundMessage into one of these, and replyFor returns copy
// for every one of them.
type inboundKind string

const (
	kindText    inboundKind = "text"     // non-empty text, no recognized command
	kindEmpty   inboundKind = "empty"    // type "text" with a blank/whitespace-only body
	kindNonText inboundKind = "non_text" // image, sticker, audio, video, document, location, contacts, reaction, unknown
	kindJoin    inboundKind = "join"     // "JOIN <code>"
	kindRename  inboundKind = "rename"   // the name-change command + a name
)

// allInboundKinds is the registry every kind must join. Stories that add a
// kind whose syntax alone identifies it (JOIN and the name command, both in
// 2.4) add it here, to replyFor's switch, and to their own reply-routing
// tests. The registry test proves only that every registered kind yields
// non-empty copy — it cannot tell a dedicated switch arm from the default
// (both return real copy), so correct per-kind routing is the adding
// story's tests' burden, not this registry's.
//
// Answer detection (3.3) deliberately does NOT follow this pattern — see
// handleTextOrAnswer below. Whether a message is an answer depends on
// game state classify() cannot see, so it stays inside kindText's existing
// arm as a pre-check rather than becoming a new inboundKind.
var allInboundKinds = []inboundKind{kindText, kindEmpty, kindNonText, kindJoin, kindRename}

// parseJoinCode recognizes "JOIN <code>" (case-insensitive keyword) and
// returns the uppercased code. Only the token right after JOIN is the code —
// a JOIN code never contains whitespace, so trailing words are ignored
// rather than rejecting the message.
func parseJoinCode(body string) (code string, ok bool) {
	fields := strings.Fields(body)
	if len(fields) < 2 || !strings.EqualFold(fields[0], "JOIN") {
		return "", false
	}
	return strings.ToUpper(fields[1]), true
}

// parseRenameName recognizes the name-change command prefix (renamePrefix,
// defined in messages_he.go) followed by a name, requiring a non-empty name
// after the prefix — an empty name does not count and falls through to
// kindText (-> Help). Leading whitespace before the prefix is tolerated
// (mirrors parseJoinCode's whitespace tolerance) — a stray leading space is a
// common mobile-keyboard slip and should not silently degrade to Help.
func parseRenameName(body string) (name string, ok bool) {
	trimmed := strings.TrimLeft(body, " \t\n\r")
	if !strings.HasPrefix(trimmed, renamePrefix) {
		return "", false
	}
	name = strings.TrimSpace(strings.TrimPrefix(trimmed, renamePrefix))
	if name == "" {
		return "", false
	}
	return name, true
}

// InboundRouter turns a normalized inbound message into exactly one outbound
// reply. It is deliberately NOT named InboundHandler — that identifier is the
// consumer-defined interface in webhook.go that this type implements.
type InboundRouter struct {
	replier     Replier
	registrar   Registrar
	broadcaster SnapshotBroadcaster
	logger      *slog.Logger
}

var _ InboundHandler = (*InboundRouter)(nil)

// NewInboundRouter builds the router. logger may be nil, in which case
// slog.Default() is used — matching NewWebhookHandler/NewDispatcher, and
// letting tests inject a captured logger to assert on redaction and levels.
func NewInboundRouter(replier Replier, registrar Registrar, broadcaster SnapshotBroadcaster, logger *slog.Logger) *InboundRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &InboundRouter{replier: replier, registrar: registrar, broadcaster: broadcaster, logger: logger}
}

// classify is total over InboundMessage: every message maps to a registered
// kind. Type is checked first because webhook.go only populates TextBody when
// Type == "text" — a sticker always carries an empty body, and calling that
// "empty" would conflate two different inbound classes. JOIN/name-change
// bodies are never empty, so those checks run after the empty check, before
// falling to kindText.
func classify(msg InboundMessage) inboundKind {
	if msg.Type != "text" {
		return kindNonText
	}
	if strings.TrimSpace(msg.TextBody) == "" {
		return kindEmpty
	}
	if _, ok := parseJoinCode(msg.TextBody); ok {
		return kindJoin
	}
	if _, ok := parseRenameName(msg.TextBody); ok {
		return kindRename
	}
	return kindText
}

// replyFor is exhaustive over the registry. kindJoin/kindRename are routed to
// their own handlers before replyFor is ever consulted for them (see Handle);
// this default arm is belt and braces, not dead code: a future kind added to
// the registry but forgotten in the switch still replies rather than going
// silent. Nothing mechanical flags that omission — the registry test asserts
// only non-empty copy, which the default satisfies — so a story wiring a new
// kind must pin its routing in its own tests; this arm just keeps the chat
// alive if it doesn't.
func replyFor(kind inboundKind) string {
	switch kind {
	case kindText, kindEmpty, kindNonText:
		return helpMessage()
	default:
		return helpMessage()
	}
}

// Handle implements InboundHandler. It enqueues exactly one reply on every
// path except a message with no sender — the one case where there is no
// address to answer.
//
// Handle runs synchronously inside the webhook HTTP request, so it must stay
// fast and must not panic — webhook.go contains panics, but a contained panic
// still costs the Participant their reply.
func (r *InboundRouter) Handle(ctx context.Context, msg InboundMessage) {
	if strings.TrimSpace(msg.From) == "" {
		// Malformed payload, not an inbound class: there is nobody to answer.
		// Whitespace-only is as absent as empty — it would only die later in
		// Enqueue's guard after passing here. This is the only silent path in
		// the router (AC-2 holds — every real inbound class replies).
		r.logger.Warn("inbound message has no sender, cannot reply",
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID),
			"type", msg.Type)
		return
	}

	kind := classify(msg)
	switch kind {
	case kindJoin:
		r.handleJoin(ctx, msg)
		return
	case kindRename:
		r.handleRename(ctx, msg)
		return
	case kindText:
		r.handleTextOrAnswer(ctx, msg)
		return
	}
	r.replier.Enqueue(msg.From, replyFor(kind))
	r.logger.Info("universal reply queued",
		"kind", string(kind),
		"phone_last4", PhoneLast4(msg.From),
		"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
}

// handleTextOrAnswer tries msg as an answer to phone's currently open
// question before falling back to the generic Help reply. Unlike
// handleJoin/handleRename, kindText's syntax alone cannot distinguish an
// answer attempt from ordinary chat — "1", a bare word, or a full
// sentence are all valid Free-Text answer content, and even a bare MCQ
// digit is only an answer when the sender genuinely has an open MCQ
// question right now. Rather than teaching classify() (a pure function
// with no store access, by design) to recognize answers, this handler
// always asks the engine first; ErrNoOpenQuestion degrades to exactly
// today's kindText behavior.
func (r *InboundRouter) handleTextOrAnswer(ctx context.Context, msg InboundMessage) {
	result, err := r.registrar.RecordAnswer(ctx, msg.From, msg.TextBody, msg.ReceivedAt)
	if errors.Is(err, game.ErrNoOpenQuestion) {
		r.replier.Enqueue(msg.From, replyFor(kindText))
		r.logger.Info("universal reply queued",
			"kind", string(kindText),
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
		return
	}
	if err != nil {
		r.replier.Enqueue(msg.From, helpMessage())
		r.logger.Warn("answer intake failed, degrading to help",
			"error", err.Error(),
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
		return
	}
	var reply string
	switch result.Outcome {
	case game.AnswerAccepted:
		reply = ackMessage()
		if result.Snapshot.GameID != "" {
			// Broadcast before responding — mirrors handleJoin's ordering,
			// so the dashboard's live answered-count (AC-5) never lags the
			// WhatsApp reply. GameID == "" means the post-answer snapshot
			// build failed and was already logged; skip broadcasting a
			// snapshot that doesn't exist (review finding, story 3.3).
			r.broadcaster.Broadcast(result.Snapshot.GameID, result.Snapshot)
		}
	case game.AnswerAlreadyAnswered:
		reply = alreadyAnsweredMessage()
	case game.AnswerFormatHint:
		reply = formatHintMessage()
	case game.AnswerTooLong:
		reply = tooLongMessage()
	case game.AnswerClosed:
		reply = questionClosedMessage()
	default:
		reply = helpMessage()
	}
	r.replier.Enqueue(msg.From, reply)
	r.logger.Info("answer processed",
		"outcome", string(result.Outcome),
		"phone_last4", PhoneLast4(msg.From),
		"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
}

// handleJoin processes a "JOIN <code>" message. It re-parses the code —
// classify already proved it matches; cheap, and keeps classify's signature
// a pure func(InboundMessage) inboundKind.
func (r *InboundRouter) handleJoin(ctx context.Context, msg InboundMessage) {
	code, _ := parseJoinCode(msg.TextBody)
	result, err := r.registrar.Join(ctx, code, msg.From, msg.ProfileName)
	switch {
	case err == nil:
		var reply string
		switch result.Outcome {
		case game.JoinWelcome:
			reply = welcomeMessage(result.DisplayName)
		case game.JoinPreLobby:
			reply = preLobbyMessage()
		case game.JoinSpectator:
			reply = spectatorNoticeMessage()
		default:
			// Belt and braces: an outcome this router does not yet know
			// about still gets a reply rather than silence.
			reply = helpMessage()
		}
		if result.Created && result.Snapshot.GameID != "" {
			// Broadcast before responding — mirrors control.go
			// handleOpenLobby's ordering, so the dashboard never lags the
			// WhatsApp reply. Guard and argument both key off
			// Snapshot.GameID (not result.GameID) so they cannot diverge.
			r.broadcaster.Broadcast(result.Snapshot.GameID, result.Snapshot)
		}
		r.replier.Enqueue(msg.From, reply)
		r.logger.Info("join processed",
			"outcome", string(result.Outcome),
			"created", result.Created,
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
	case errors.Is(err, game.ErrInvalidJoinCode):
		r.replier.Enqueue(msg.From, invalidCodeMessage(code))
		r.logger.Info("join rejected, invalid code",
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
	case errors.Is(err, game.ErrGameFinished):
		r.replier.Enqueue(msg.From, helpMessage())
		r.logger.Info("join rejected, game already finished",
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
	default:
		r.replier.Enqueue(msg.From, helpMessage())
		r.logger.Warn("join failed, degrading to help",
			"error", err.Error(),
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
	}
}

// handleRename processes a name-change command message.
func (r *InboundRouter) handleRename(ctx context.Context, msg InboundMessage) {
	name, _ := parseRenameName(msg.TextBody)
	result, err := r.registrar.Rename(ctx, msg.From, name)
	switch {
	case err == nil:
		r.replier.Enqueue(msg.From, nameUpdatedMessage(result.DisplayName))
		r.logger.Info("rename processed",
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
	case errors.Is(err, game.ErrParticipantNotFound):
		r.replier.Enqueue(msg.From, helpMessage())
		r.logger.Info("rename rejected, unregistered sender",
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
	default:
		r.replier.Enqueue(msg.From, helpMessage())
		r.logger.Warn("rename failed, degrading to help",
			"error", err.Error(),
			"phone_last4", PhoneLast4(msg.From),
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
	}
}
