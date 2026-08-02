package wa

import (
	"context"
	"log/slog"
	"strings"
)

// Replier queues an outbound message for delivery. Consumer-defined (the
// Deduper/SenderClient precedent) so the router is testable without a live
// Dispatcher; *Dispatcher satisfies it.
type Replier interface {
	Enqueue(to, body string)
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
)

// allInboundKinds is the registry every kind must join. Stories that add a
// kind (JOIN and the name command in 2.4, answers in 3.3) add it here, to
// replyFor's switch, and to their own reply-routing tests. The registry test
// proves only that every registered kind yields non-empty copy — it cannot
// tell a dedicated switch arm from the default (both return real copy), so
// correct per-kind routing is the adding story's tests' burden, not this
// registry's.
var allInboundKinds = []inboundKind{kindText, kindEmpty, kindNonText}

// InboundRouter turns a normalized inbound message into exactly one outbound
// reply. It is deliberately NOT named InboundHandler — that identifier is the
// consumer-defined interface in webhook.go that this type implements.
type InboundRouter struct {
	replier Replier
	logger  *slog.Logger
}

var _ InboundHandler = (*InboundRouter)(nil)

// NewInboundRouter builds the router. logger may be nil, in which case
// slog.Default() is used — matching NewWebhookHandler/NewDispatcher, and
// letting tests inject a captured logger to assert on redaction and levels.
func NewInboundRouter(replier Replier, logger *slog.Logger) *InboundRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &InboundRouter{replier: replier, logger: logger}
}

// classify is total over InboundMessage: every message maps to a registered
// kind. Type is checked first because webhook.go only populates TextBody when
// Type == "text" — a sticker always carries an empty body, and calling that
// "empty" would conflate two different inbound classes.
func classify(msg InboundMessage) inboundKind {
	if msg.Type != "text" {
		return kindNonText
	}
	if strings.TrimSpace(msg.TextBody) == "" {
		return kindEmpty
	}
	return kindText
}

// replyFor is exhaustive over the registry. Every kind answers with the Help
// copy in story 2.2 — JOIN (2.4) and answers (3.3) branch here later.
//
// The default arm is belt and braces, not dead code: a future kind added to
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
// ctx is unused today (Enqueue is fire-and-forget and never blocks); the
// parameter satisfies InboundHandler and 2.4's registration lookup will use
// it. Handle runs synchronously inside the webhook HTTP request, so it must
// stay fast and must not panic — webhook.go contains panics, but a contained
// panic still costs the Participant their reply.
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
	r.replier.Enqueue(msg.From, replyFor(kind))
	r.logger.Info("universal reply queued",
		"kind", string(kind),
		"phone_last4", PhoneLast4(msg.From),
		"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
}
