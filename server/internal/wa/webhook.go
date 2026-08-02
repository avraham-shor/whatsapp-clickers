package wa

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// maxWebhookBodyBytes caps inbound webhook payloads. Meta payloads are
// small; the cap is hostile-input hygiene, not a real-world limit.
const maxWebhookBodyBytes = 256 * 1024

// dedupeDBTimeout bounds the per-message dedupe INSERT so a hung Postgres
// cannot pin the synchronous webhook goroutine past Meta's response window
// (which would trigger the retry-storm the 200-always design avoids). Mirrors
// the 5s deadline every httpapi DB handler already imposes.
const dedupeDBTimeout = 5 * time.Second

// payloadBudget bounds the whole POST handler, not one message. dedupeDBTimeout
// alone is per-message, so a degraded Postgres plus a multi-message batch could
// hold the goroutine for N*5s — past Meta's response window, which produces the
// retry storm the 200-always design exists to avoid. Sized to stay inside the
// server's 30s WriteTimeout with room for the response write.
const payloadBudget = 20 * time.Second

// InboundMessage is the normalized shape every wa consumer works with,
// independent of Meta's webhook envelope.
type InboundMessage struct {
	WaMessageID string
	From        string // E.164 digits, no leading "+" (Meta's messages[].from)
	ProfileName string // pushname; empty when contacts[] is absent OR no wa_id matches From
	Type        string // Meta's messages[].type verbatim: "text", "image", "sticker", ...
	TextBody    string // text.body for Type=="text"; empty otherwise
	ReceivedAt  time.Time
}

// Deduper records a WhatsApp message ID as processed, reporting whether
// this was the first delivery. Consumer-defined so wa never imports store.
type Deduper interface {
	MarkWaMessageProcessed(ctx context.Context, waMessageID string) (bool, error)
}

// InboundHandler consumes a normalized inbound message. No error return:
// intake never fails on handler behavior — handlers own their own logging.
type InboundHandler interface {
	Handle(ctx context.Context, msg InboundMessage)
}

// webhookHandler serves both the GET registration handshake and the POST
// inbound delivery on the same mount point.
type webhookHandler struct {
	appSecret     string
	verifyToken   string
	phoneNumberID string
	dedupe        Deduper
	inbound       InboundHandler
	logger        *slog.Logger
}

// NewWebhookHandler builds the /webhooks/whatsapp handler. logger may be
// nil, in which case slog.Default() is used — pass a logger built on
// slog.NewTextHandler(buf, nil) in tests to assert on WARN/redaction.
//
// phoneNumberID is our own business phone number ID. A valid HMAC only proves
// the app-secret holder signed the body, and the app secret is app-scoped, not
// number-scoped: every phone number subscribed to this Meta app produces
// payloads that verify. Messages whose value.metadata.phone_number_id names a
// different number are dropped. Empty disables the check.
func NewWebhookHandler(appSecret, verifyToken, phoneNumberID string, dedupe Deduper, inbound InboundHandler, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &webhookHandler{
		appSecret:     appSecret,
		verifyToken:   verifyToken,
		phoneNumberID: phoneNumberID,
		dedupe:        dedupe,
		inbound:       inbound,
		logger:        logger,
	}
}

func (h *webhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleVerify(w, r)
	case http.MethodPost:
		h.handleInbound(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleVerify answers Meta's webhook registration handshake.
func (h *webhookHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("hub.mode")
	token := q.Get("hub.verify_token")
	challenge := q.Get("hub.challenge")

	// Every rejection is logged with a reason. This handshake is re-run on a
	// fresh quick-tunnel URL each dev session and is the single most common
	// setup failure; without a log line the operator cannot tell "never
	// arrived" from "wrong token" from "wrong mode".
	tokenMatches := subtle.ConstantTimeCompare([]byte(token), []byte(h.verifyToken)) == 1
	switch {
	case mode != "subscribe":
		h.logger.Warn("verify handshake rejected", "reason", "unexpected hub.mode")
	case !tokenMatches:
		h.logger.Warn("verify handshake rejected", "reason", "verify token mismatch")
	case challenge == "":
		// Echoing an empty challenge yields a 200 that Meta still rejects,
		// with nothing in the log to explain it.
		h.logger.Warn("verify handshake rejected", "reason", "missing hub.challenge")
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge))
		return
	}
	w.WriteHeader(http.StatusForbidden)
}

// handleInbound verifies the HMAC signature over the raw body before any
// JSON parsing, then always answers 200 for an authentic payload — even an
// unparseable or unrecognized one — so Meta never retry-storms us.
func (h *webhookHandler) handleInbound(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Warn("webhook body read failed", "error", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sigHeader := r.Header.Get("X-Hub-Signature-256")
	ok, reason := verifySignature(body, h.appSecret, sigHeader)
	if !ok {
		h.logger.Warn("webhook signature rejected", "reason", reason)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Detach from the request context before processing. The body is read and
	// verified, and Meta gets its 200 either way — a client disconnect or a
	// fired WriteTimeout must not abort work already accepted. Left attached,
	// a dropped connection makes the dedupe INSERT fail with context.Canceled,
	// which the fail-open branch below would process anyway without writing a
	// ledger row: Meta's retry would then be processed a second time, defeating
	// AC-3 via a transient disconnect rather than a DB outage.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), payloadBudget)
	defer cancel()
	h.processPayload(ctx, body)
	w.WriteHeader(http.StatusOK)
}

// verifySignature checks the raw body against Meta's X-Hub-Signature-256
// header using the app secret. It returns a reason string for WARN logs —
// never the body or the signature itself.
func verifySignature(body []byte, secret, header string) (bool, string) {
	if header == "" {
		return false, "missing header"
	}
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false, "malformed prefix"
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return false, "invalid hex"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return false, "mismatch"
	}
	return true, ""
}

// metaEnvelope mirrors the standard Meta webhook payload shape:
// entry[].changes[].value.{contacts,messages,statuses,...}.
type metaEnvelope struct {
	Entry []metaEntry `json:"entry"`
}

type metaEntry struct {
	Changes []metaChange `json:"changes"`
}

type metaChange struct {
	Value metaValue `json:"value"`
}

type metaValue struct {
	Metadata metaMetadata  `json:"metadata"`
	Contacts []metaContact `json:"contacts"`
	Messages []metaMessage `json:"messages"`
	// Statuses (delivery receipts) and any other field are intentionally
	// not modeled — json.Unmarshal ignores unknown keys, and we silently
	// acknowledge them (no messages means nothing to process).
}

// metaMetadata carries the business number the change was delivered for.
// PhoneNumberID is Meta's opaque per-number identifier, not an MSISDN.
type metaMetadata struct {
	PhoneNumberID string `json:"phone_number_id"`
}

type metaContact struct {
	WaID    string `json:"wa_id"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
}

type metaMessage struct {
	From string `json:"from"`
	ID   string `json:"id"`
	Type string `json:"type"`
	Text struct {
		Body string `json:"body"`
	} `json:"text"`
}

// processPayload parses the envelope and dispatches every messages[] entry.
// Unparseable JSON and non-message changes (statuses, etc.) are dropped
// silently at INFO level — a healthy game generates thousands of statuses.
func (h *webhookHandler) processPayload(ctx context.Context, body []byte) {
	var envelope metaEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		// json.Unmarshal does not abort on a type mismatch: it records the
		// first one, keeps decoding, and returns it with the envelope fully
		// populated. Returning here would throw away every well-formed message
		// in the batch — and since an authentic payload always gets a 200,
		// Meta would never redeliver them. One unmodeled field in a future
		// Graph API version would silently blackhole all inbound traffic while
		// the endpoint reported healthy.
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) {
			h.logger.Warn("webhook payload unparseable", "error", err.Error())
			return
		}
		h.logger.Warn("webhook payload partially decoded, processing valid messages",
			"error", err.Error(), "field", typeErr.Field)
	}
	for _, entry := range envelope.Entry {
		for _, change := range entry.Changes {
			if !h.numberMatches(change.Value.Metadata.PhoneNumberID) {
				h.logger.Warn("webhook change for another phone number, dropped",
					"meta_number_id", change.Value.Metadata.PhoneNumberID,
					"messages", len(change.Value.Messages))
				continue
			}
			for _, m := range change.Value.Messages {
				if err := ctx.Err(); err != nil {
					// The payload budget is spent (a degraded DB burning
					// dedupeDBTimeout per message gets here). Stop rather than
					// run past Meta's response window; unprocessed messages
					// have no ledger row, so the retry picks them up.
					h.logger.Warn("payload budget exhausted, remaining messages left for retry",
						"error", err.Error())
					return
				}
				h.processMessage(ctx, change.Value.Contacts, m)
			}
		}
	}
}

// numberMatches reports whether a change was delivered for our own business
// number. A valid HMAC proves only that the app-secret holder signed the body,
// and the app secret is app-scoped: any number subscribed to this Meta app
// verifies. An empty configured ID disables the check.
func (h *webhookHandler) numberMatches(deliveredFor string) bool {
	if h.phoneNumberID == "" || deliveredFor == "" {
		return true
	}
	return deliveredFor == h.phoneNumberID
}

func (h *webhookHandler) processMessage(ctx context.Context, contacts []metaContact, m metaMessage) {
	msg := InboundMessage{
		WaMessageID: m.ID,
		From:        m.From,
		ProfileName: profileNameFor(contacts, m.From),
		Type:        m.Type,
		ReceivedAt:  time.Now().UTC(), // FR-7's authoritative receipt clock is ours, never Meta's timestamp.
	}
	if m.Type == "text" {
		msg.TextBody = m.Text.Body
	}

	// Meta always supplies a message id; an empty one is a malformed payload.
	// Dropping it (rather than deduping on "") avoids a first empty-id message
	// poisoning the empty-string PK and swallowing every later id-less message
	// as a false duplicate.
	if msg.WaMessageID == "" {
		h.logger.Warn("inbound message missing id, dropped",
			"type", msg.Type, "phone_last4", PhoneLast4(msg.From))
		return
	}

	// The ledger keys on the digest, never the raw wamid: a wamid base64-encodes
	// the sender's MSISDN, so storing it raw kept a recoverable phone number at
	// rest indefinitely (migration 00007). Dedupe semantics are unchanged — the
	// value is only ever compared for equality.
	dbCtx, cancel := context.WithTimeout(ctx, dedupeDBTimeout)
	defer cancel()
	first, err := h.dedupe.MarkWaMessageProcessed(dbCtx, WaMessageIDDigest(msg.WaMessageID))
	switch {
	case err != nil:
		// A DB blip must not silence the chat: process anyway (SM-C2
		// posture). Duplicate risk during the blip is accepted and logged.
		h.logger.Warn("dedupe check failed, processing anyway",
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID), "dedupe_degraded", true, "error", err.Error())
	case !first:
		h.logger.Info("duplicate webhook delivery skipped",
			"wa_message_id", WaMessageIDDigest(msg.WaMessageID))
		return
	}

	if !h.callHandler(ctx, msg) {
		return
	}
	h.logger.Info("inbound message accepted",
		"wa_message_id", WaMessageIDDigest(msg.WaMessageID), "type", msg.Type, "phone_last4", PhoneLast4(msg.From))
}

// callHandler invokes the inbound handler, containing any panic it raises.
// It reports whether the handler returned normally.
//
// The ledger row is committed before this call (dedupe-before-handler is a
// locked decision), and the repo has no panic-recovery middleware. Without
// this guard a panic unwinds to net/http's per-connection recover: the 200 is
// never written, Meta retries, the retry finds the wamid already recorded and
// logs "duplicate webhook delivery skipped" at INFO — the message is lost
// permanently and the only evidence reads as healthy. The panic also never
// reaches slog, so NFR-8 observability shows nothing. Story 2.2 replaces the
// stub handler with a real parser on this exact seam.
func (h *webhookHandler) callHandler(ctx context.Context, msg InboundMessage) (ok bool) {
	defer func() {
		if rec := recover(); rec != nil {
			h.logger.Warn("inbound handler panicked, message dropped",
				"wa_message_id", WaMessageIDDigest(msg.WaMessageID),
				"type", msg.Type,
				"phone_last4", PhoneLast4(msg.From),
				"panic", RedactDigits(fmt.Sprint(rec)))
			ok = false
		}
	}()
	h.inbound.Handle(ctx, msg)
	return true
}

// profileNameFor returns the pushname Meta supplies alongside the message.
// It returns "" both when contacts[] is absent and when no entry's wa_id
// matches the sender — callers must treat empty as "unknown", not as proof
// that Meta omitted the block.
func profileNameFor(contacts []metaContact, from string) string {
	for _, c := range contacts {
		if c.WaID == from {
			return c.Profile.Name
		}
	}
	return ""
}
