package wa

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- stubs (grown in-test, no mock framework) ---

type stubDeduper struct {
	// first maps a wa_message_id to the bool MarkWaMessageProcessed should
	// report; missing keys default to true (first delivery).
	first map[string]bool
	err   error
	calls []string
}

func (d *stubDeduper) MarkWaMessageProcessed(ctx context.Context, waMessageID string) (bool, error) {
	d.calls = append(d.calls, waMessageID)
	if d.err != nil {
		return false, d.err
	}
	if v, ok := d.first[waMessageID]; ok {
		return v, nil
	}
	return true, nil
}

type stubInboundHandler struct {
	calls []InboundMessage
}

func (h *stubInboundHandler) Handle(ctx context.Context, msg InboundMessage) {
	h.calls = append(h.calls, msg)
}

func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func sign(t *testing.T, secret string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// --- GET (Meta registration handshake) ---

func TestWebhookGetHandshakeSuccess(t *testing.T) {
	h := NewWebhookHandler("secret", "verify-token", &stubDeduper{}, &stubInboundHandler{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=12345", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "12345" {
		t.Errorf("body = %q, want %q (exact challenge echo)", rec.Body.String(), "12345")
	}
}

func TestWebhookGetHandshakeRejected(t *testing.T) {
	cases := map[string]string{
		"wrong token":   "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=123",
		"missing token": "/webhooks/whatsapp?hub.mode=subscribe&hub.challenge=123",
		"wrong mode":    "/webhooks/whatsapp?hub.mode=unsubscribe&hub.verify_token=verify-token&hub.challenge=123",
		"missing mode":  "/webhooks/whatsapp?hub.verify_token=verify-token&hub.challenge=123",
	}
	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewWebhookHandler("secret", "verify-token", &stubDeduper{}, &stubInboundHandler{}, nil)
			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty", rec.Body.String())
			}
		})
	}
}

// --- POST (inbound delivery) ---

func TestWebhookPostValidTextMessage(t *testing.T) {
	secret := "app-secret"
	dedupe := &stubDeduper{}
	inbound := &stubInboundHandler{}
	h := NewWebhookHandler(secret, "verify-token", dedupe, inbound, nil)

	body := []byte(`{"entry":[{"changes":[{"value":{
		"contacts":[{"wa_id":"972500000000","profile":{"name":"Avraham"}}],
		"messages":[{"from":"972500000000","id":"wamid.ABC123","type":"text","text":{"body":"שלום"}}]
	}}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(inbound.calls) != 1 {
		t.Fatalf("handler called %d times, want 1", len(inbound.calls))
	}
	got := inbound.calls[0]
	if got.WaMessageID != "wamid.ABC123" {
		t.Errorf("WaMessageID = %q, want wamid.ABC123", got.WaMessageID)
	}
	if got.From != "972500000000" {
		t.Errorf("From = %q, want 972500000000", got.From)
	}
	if got.ProfileName != "Avraham" {
		t.Errorf("ProfileName = %q, want Avraham", got.ProfileName)
	}
	if got.Type != "text" {
		t.Errorf("Type = %q, want text", got.Type)
	}
	if got.TextBody != "שלום" {
		t.Errorf("TextBody = %q, want שלום", got.TextBody)
	}
	if got.ReceivedAt.IsZero() {
		t.Error("ReceivedAt not set")
	}
	if len(dedupe.calls) != 1 || dedupe.calls[0] != "wamid.ABC123" {
		t.Errorf("dedupe calls = %v, want [wamid.ABC123]", dedupe.calls)
	}
}

func TestWebhookPostInvalidSignatureRejected(t *testing.T) {
	const correctSecret = "correct-secret"
	body := []byte(`{"entry":[]}`)
	validSig := sign(t, correctSecret, body)

	cases := map[string]string{
		"wrong secret":     sign(t, "wrong-secret", body),
		"tampered body":    sign(t, correctSecret, []byte(`{"entry":[{"tampered":true}]}`)),
		"missing header":   "",
		"malformed prefix": "sha1=" + strings.TrimPrefix(validSig, "sha256="),
		"non-hex":          "sha256=not-hex-zzzz",
	}
	for name, sig := range cases {
		t.Run(name, func(t *testing.T) {
			dedupe := &stubDeduper{}
			inbound := &stubInboundHandler{}
			h := NewWebhookHandler(correctSecret, "verify-token", dedupe, inbound, nil)

			req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
			if sig != "" {
				req.Header.Set("X-Hub-Signature-256", sig)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", rec.Code)
			}
			if len(inbound.calls) != 0 {
				t.Errorf("handler called %d times, want 0", len(inbound.calls))
			}
			if len(dedupe.calls) != 0 {
				t.Errorf("dedupe called %d times, want 0", len(dedupe.calls))
			}
		})
	}
}

func TestWebhookPostDuplicateSkipsHandler(t *testing.T) {
	secret := "secret"
	dedupe := &stubDeduper{first: map[string]bool{"wamid.DUP": false}}
	inbound := &stubInboundHandler{}
	h := NewWebhookHandler(secret, "verify-token", dedupe, inbound, nil)

	body := []byte(`{"entry":[{"changes":[{"value":{"messages":[{"from":"972500000000","id":"wamid.DUP","type":"text","text":{"body":"hi"}}]}}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(inbound.calls) != 0 {
		t.Errorf("handler called %d times, want 0 (duplicate)", len(inbound.calls))
	}
}

func TestWebhookPostMissingMessageIDDropped(t *testing.T) {
	secret := "secret"
	dedupe := &stubDeduper{}
	inbound := &stubInboundHandler{}
	logger, buf := newTestLogger()
	h := NewWebhookHandler(secret, "verify-token", dedupe, inbound, logger)

	// Authentic payload whose message has no "id" — malformed; must be dropped
	// rather than deduped on the empty-string key.
	body := []byte(`{"entry":[{"changes":[{"value":{"messages":[{"from":"972500000000","type":"text","text":{"body":"hi"}}]}}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(inbound.calls) != 0 {
		t.Errorf("handler called %d times, want 0 (missing id)", len(inbound.calls))
	}
	if len(dedupe.calls) != 0 {
		t.Errorf("dedupe called %d times, want 0 (no empty-string key insert)", len(dedupe.calls))
	}
	if !strings.Contains(buf.String(), "inbound message missing id") {
		t.Errorf("log missing the drop WARN: %s", buf.String())
	}
}

func TestWebhookPostStatusesOnlyIgnored(t *testing.T) {
	secret := "secret"
	dedupe := &stubDeduper{}
	inbound := &stubInboundHandler{}
	h := NewWebhookHandler(secret, "verify-token", dedupe, inbound, nil)

	body := []byte(`{"entry":[{"changes":[{"value":{"statuses":[{"id":"wamid.STATUS","status":"delivered"}]}}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(inbound.calls) != 0 || len(dedupe.calls) != 0 {
		t.Errorf("handler/dedupe called on statuses-only payload: handler=%d dedupe=%d", len(inbound.calls), len(dedupe.calls))
	}
}

func TestWebhookPostUnparseableJSONStill200(t *testing.T) {
	secret := "secret"
	h := NewWebhookHandler(secret, "verify-token", &stubDeduper{}, &stubInboundHandler{}, nil)

	body := []byte(`not json {`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (authentic-but-unparseable must not retry-loop)", rec.Code)
	}
}

func TestWebhookPostNonTextMessageEmptyBody(t *testing.T) {
	secret := "secret"
	inbound := &stubInboundHandler{}
	h := NewWebhookHandler(secret, "verify-token", &stubDeduper{}, inbound, nil)

	body := []byte(`{"entry":[{"changes":[{"value":{"messages":[{"from":"972500000000","id":"wamid.IMG","type":"image"}]}}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if len(inbound.calls) != 1 {
		t.Fatalf("handler called %d times, want 1", len(inbound.calls))
	}
	got := inbound.calls[0]
	if got.Type != "image" {
		t.Errorf("Type = %q, want image", got.Type)
	}
	if got.TextBody != "" {
		t.Errorf("TextBody = %q, want empty for non-text message", got.TextBody)
	}
}

func TestWebhookPostMultiMessageBatchInOrder(t *testing.T) {
	secret := "secret"
	inbound := &stubInboundHandler{}
	h := NewWebhookHandler(secret, "verify-token", &stubDeduper{}, inbound, nil)

	body := []byte(`{"entry":[{"changes":[{"value":{"messages":[
		{"from":"972500000001","id":"wamid.ONE","type":"text","text":{"body":"one"}},
		{"from":"972500000002","id":"wamid.TWO","type":"text","text":{"body":"two"}}
	]}}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if len(inbound.calls) != 2 {
		t.Fatalf("handler called %d times, want 2", len(inbound.calls))
	}
	if inbound.calls[0].WaMessageID != "wamid.ONE" || inbound.calls[1].WaMessageID != "wamid.TWO" {
		t.Errorf("messages out of order: %+v", inbound.calls)
	}
}

func TestWebhookPostOversizedBodyRejected(t *testing.T) {
	secret := "secret"
	inbound := &stubInboundHandler{}
	h := NewWebhookHandler(secret, "verify-token", &stubDeduper{}, inbound, nil)

	huge := bytes.Repeat([]byte("a"), 300*1024) // > 256 KiB cap
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(huge))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code < 400 || rec.Code >= 500 {
		t.Errorf("status = %d, want 4xx", rec.Code)
	}
	if len(inbound.calls) != 0 {
		t.Errorf("handler called on oversized body")
	}
}

func TestWebhookPostDedupeFailureDegradesAndProcesses(t *testing.T) {
	secret := "secret"
	dedupe := &stubDeduper{err: errors.New("db down")}
	inbound := &stubInboundHandler{}
	logger, buf := newTestLogger()
	h := NewWebhookHandler(secret, "verify-token", dedupe, inbound, logger)

	body := []byte(`{"entry":[{"changes":[{"value":{"messages":[{"from":"972500000000","id":"wamid.DBDOWN","type":"text","text":{"body":"hi"}}]}}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(inbound.calls) != 1 {
		t.Fatalf("handler called %d times, want 1 (process anyway on dedupe failure)", len(inbound.calls))
	}
	if !strings.Contains(buf.String(), "dedupe_degraded=true") {
		t.Errorf("log missing dedupe_degraded=true: %s", buf.String())
	}
}

func TestWebhookPostLogsOnlyPhoneLast4(t *testing.T) {
	secret := "secret"
	logger, buf := newTestLogger()
	h := NewWebhookHandler(secret, "verify-token", &stubDeduper{}, &stubInboundHandler{}, logger)

	from := "972501234567"
	body := []byte(fmt.Sprintf(`{"entry":[{"changes":[{"value":{"messages":[{"from":"%s","id":"wamid.PHONE","type":"text","text":{"body":"hi"}}]}}]}]}`, from))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	logs := buf.String()
	if strings.Contains(logs, from) {
		t.Errorf("log output contains full phone number: %s", logs)
	}
	if !strings.Contains(logs, PhoneLast4(from)) {
		t.Errorf("log output missing phone_last4 %q: %s", PhoneLast4(from), logs)
	}
}
