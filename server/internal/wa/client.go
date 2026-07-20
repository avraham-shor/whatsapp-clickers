package wa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// defaultBaseURL pins Graph API v25.0 (current as of 2026-07; verified
// released 2026-02-18, versions run ~2 years). One overridable constant to
// bump on future version upgrades; tests inject an httptest URL through
// the same knob via WithBaseURL.
const defaultBaseURL = "https://graph.facebook.com/v25.0"

// sendTimeout bounds one SendText call so a hung Meta connection cannot
// wedge a dispatcher worker.
const sendTimeout = 10 * time.Second

// Client is a minimal Cloud API client scoped to what this story needs:
// sending a single text message.
type Client struct {
	accessToken   string
	phoneNumberID string
	baseURL       string
	httpClient    *http.Client
}

// ClientOption configures optional Client behavior.
type ClientOption func(*Client)

// WithBaseURL overrides the default Graph API base URL — used by tests
// (httptest servers) and future version bumps.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) { c.baseURL = baseURL }
}

// WithHTTPClient overrides the default *http.Client (tests / custom transports).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient builds a Client for one WhatsApp business phone number.
func NewClient(accessToken, phoneNumberID string, opts ...ClientOption) *Client {
	c := &Client{
		accessToken:   accessToken,
		phoneNumberID: phoneNumberID,
		baseURL:       defaultBaseURL,
		httpClient:    &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type sendTextRequest struct {
	MessagingProduct string       `json:"messaging_product"`
	RecipientType    string       `json:"recipient_type"`
	To               string       `json:"to"`
	Type             string       `json:"type"`
	Text             sendTextBody `json:"text"`
}

type sendTextBody struct {
	PreviewURL bool   `json:"preview_url"`
	Body       string `json:"body"`
}

// metaErrorResponse mirrors the Cloud API's error envelope shape.
type metaErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// SendText sends a single WhatsApp text message. On a non-2xx response the
// returned error carries Meta's status/code/message but never the
// recipient number — callers (the dispatcher) log this error verbatim.
func (c *Client) SendText(ctx context.Context, to, body string) error {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	payload := sendTextRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "text",
		Text:             sendTextBody{PreviewURL: false, Body: body},
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode send-text payload: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", c.baseURL, c.phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build send-text request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send-text request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var metaErr metaErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&metaErr) // best-effort; body may not be JSON
		if metaErr.Error.Message != "" {
			return fmt.Errorf("whatsapp send failed: status %d code %d: %s", resp.StatusCode, metaErr.Error.Code, metaErr.Error.Message)
		}
		return fmt.Errorf("whatsapp send failed: status %d", resp.StatusCode)
	}
	return nil
}
