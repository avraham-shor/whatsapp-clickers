package wa

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
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

// InboundMessage is the normalized shape every wa consumer works with,
// independent of Meta's webhook envelope.
type InboundMessage struct {
	WaMessageID string
	From        string // E.164 digits, no leading "+" (Meta's messages[].from)
	ProfileName string // empty when Meta omits the parallel contacts[] entry
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
	appSecret   string
	verifyToken string
	dedupe      Deduper
	inbound     InboundHandler
	logger      *slog.Logger
}

// NewWebhookHandler builds the /webhooks/whatsapp handler. logger may be
// nil, in which case slog.Default() is used — pass a logger built on
// slog.NewTextHandler(buf, nil) in tests to assert on WARN/redaction.
func NewWebhookHandler(appSecret, verifyToken string, dedupe Deduper, inbound InboundHandler, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &webhookHandler{
		appSecret:   appSecret,
		verifyToken: verifyToken,
		dedupe:      dedupe,
		inbound:     inbound,
		logger:      logger,
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

	tokenMatches := subtle.ConstantTimeCompare([]byte(token), []byte(h.verifyToken)) == 1
	if mode == "subscribe" && tokenMatches {
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

	h.processPayload(r.Context(), body)
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
	Contacts []metaContact `json:"contacts"`
	Messages []metaMessage `json:"messages"`
	// Statuses (delivery receipts) and any other field are intentionally
	// not modeled — json.Unmarshal ignores unknown keys, and we silently
	// acknowledge them (no messages means nothing to process).
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
		h.logger.Warn("webhook payload unparseable", "error", err.Error())
		return
	}
	for _, entry := range envelope.Entry {
		for _, change := range entry.Changes {
			for _, m := range change.Value.Messages {
				h.processMessage(ctx, change.Value.Contacts, m)
			}
		}
	}
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

	dbCtx, cancel := context.WithTimeout(ctx, dedupeDBTimeout)
	defer cancel()
	first, err := h.dedupe.MarkWaMessageProcessed(dbCtx, msg.WaMessageID)
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

	h.inbound.Handle(ctx, msg)
	h.logger.Info("inbound message accepted",
		"wa_message_id", WaMessageIDDigest(msg.WaMessageID), "type", msg.Type, "phone_last4", PhoneLast4(msg.From))
}

func profileNameFor(contacts []metaContact, from string) string {
	for _, c := range contacts {
		if c.WaID == from {
			return c.Profile.Name
		}
	}
	return ""
}
