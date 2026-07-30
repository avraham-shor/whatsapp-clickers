package wa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestClientSendTextRequestShape(t *testing.T) {
	var gotPath, gotAuth, gotContentType string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"messages":[{"id":"wamid.SENT"}]}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", "PHONE123", WithBaseURL(srv.URL))
	if err := c.SendText(context.Background(), "972500000000", "hello"); err != nil {
		t.Fatalf("SendText() error: %v", err)
	}

	if !strings.Contains(gotPath, "PHONE123") {
		t.Errorf("path = %q, want it to contain the phone number ID", gotPath)
	}
	if !strings.HasSuffix(gotPath, "/messages") {
		t.Errorf("path = %q, want it to end in /messages", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}

	want := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                "972500000000",
		"type":              "text",
		"text": map[string]any{
			"preview_url": false,
			"body":        "hello",
		},
	}
	if !reflect.DeepEqual(gotBody, want) {
		t.Errorf("body = %#v, want %#v", gotBody, want)
	}
}

func TestClientSendTextSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"messages":[{"id":"wamid.SENT"}]}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", "PHONE123", WithBaseURL(srv.URL))
	if err := c.SendText(context.Background(), "972500000000", "hello"); err != nil {
		t.Errorf("SendText() error = %v, want nil on 2xx", err)
	}
}

func TestClientSendTextErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"Invalid parameter","code":100}}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", "PHONE123", WithBaseURL(srv.URL))
	err := c.SendText(context.Background(), "972500000000", "hello")
	if err == nil {
		t.Fatal("SendText() succeeded, want error on 4xx")
	}
	if !strings.Contains(err.Error(), "100") {
		t.Errorf("error %q does not contain the Meta error code", err.Error())
	}
	if !strings.Contains(err.Error(), "Invalid parameter") {
		t.Errorf("error %q does not contain the Meta error message", err.Error())
	}
	if strings.Contains(err.Error(), "972500000000") {
		t.Errorf("error %q contains the full recipient number", err.Error())
	}
}

func TestClientSendTextRedactsPhoneInMetaError(t *testing.T) {
	// Meta echoes the full recipient number inside error.message; the returned
	// error (which the dispatcher logs) must not carry it (NFR-4).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"Recipient 972500000000 is not in allowed list","code":131030}}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", "PHONE123", WithBaseURL(srv.URL))
	err := c.SendText(context.Background(), "972500000000", "hello")
	if err == nil {
		t.Fatal("SendText() succeeded, want error on 4xx")
	}
	if strings.Contains(err.Error(), "972500000000") {
		t.Errorf("error %q leaks the full recipient number from the Meta message", err.Error())
	}
	if !strings.Contains(err.Error(), "131030") {
		t.Errorf("error %q dropped the Meta error code (a short number must survive)", err.Error())
	}
	if !strings.Contains(err.Error(), "not in allowed list") {
		t.Errorf("error %q dropped the non-digit context from the Meta message", err.Error())
	}
}

func TestClientSendTextServerErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"Service unavailable","code":500}}`))
	}))
	defer srv.Close()

	c := NewClient("test-token", "PHONE123", WithBaseURL(srv.URL))
	err := c.SendText(context.Background(), "972500000000", "hello")
	if err == nil {
		t.Fatal("SendText() succeeded, want error on 5xx")
	}
}
