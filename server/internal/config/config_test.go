package config

import (
	"strings"
	"testing"
)

func lookupFromMap(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := env[name]
		return v, ok
	}
}

func fullEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":             "postgres://localhost:5432/app",
		"WHATSAPP_ACCESS_TOKEN":    "token",
		"WHATSAPP_PHONE_NUMBER_ID": "12345",
		"WHATSAPP_APP_SECRET":      "secret",
		"WHATSAPP_VERIFY_TOKEN":    "verify",
		"ANTHROPIC_API_KEY":        "key",
		"SESSION_SECRET":           "test-session-secret-0123456789abcdef",
	}
}

func TestLoadAllRequiredPresent(t *testing.T) {
	cfg, err := load(lookupFromMap(fullEnv()))
	if err != nil {
		t.Fatalf("load() returned error with full env: %v", err)
	}
	if cfg.DatabaseURL != "postgres://localhost:5432/app" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://localhost:5432/app")
	}
	if cfg.SessionSecret != "test-session-secret-0123456789abcdef" {
		t.Errorf("SessionSecret = %q, want %q", cfg.SessionSecret, "test-session-secret-0123456789abcdef")
	}
}

func TestLoadPortDefaultsTo8080(t *testing.T) {
	cfg, err := load(lookupFromMap(fullEnv()))
	if err != nil {
		t.Fatalf("load() returned error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "8080")
	}
}

func TestLoadPortOverride(t *testing.T) {
	env := fullEnv()
	env["PORT"] = "3000"
	cfg, err := load(lookupFromMap(env))
	if err != nil {
		t.Fatalf("load() returned error: %v", err)
	}
	if cfg.Port != "3000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "3000")
	}
}

func TestLoadMissingVarsNamedInError(t *testing.T) {
	env := fullEnv()
	delete(env, "DATABASE_URL")
	delete(env, "SESSION_SECRET")

	_, err := load(lookupFromMap(env))
	if err == nil {
		t.Fatal("load() succeeded with missing required vars, want error")
	}
	for _, name := range []string{"DATABASE_URL", "SESSION_SECRET"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name missing var %s", err.Error(), name)
		}
	}
	if strings.Contains(err.Error(), "WHATSAPP_ACCESS_TOKEN") {
		t.Errorf("error %q names a var that is present", err.Error())
	}
}

func TestLoadEmptyValueCountsAsMissing(t *testing.T) {
	env := fullEnv()
	env["ANTHROPIC_API_KEY"] = ""

	_, err := load(lookupFromMap(env))
	if err == nil {
		t.Fatal("load() succeeded with empty required var, want error")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("error %q does not name empty var ANTHROPIC_API_KEY", err.Error())
	}
}

func TestLoadSessionSecretTooShortRejected(t *testing.T) {
	env := fullEnv()
	env["SESSION_SECRET"] = "only-31-characters-long-secret!" // 31 chars

	_, err := load(lookupFromMap(env))
	if err == nil {
		t.Fatal("load() succeeded with short SESSION_SECRET, want error")
	}
	if !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Errorf("error %q does not name SESSION_SECRET", err.Error())
	}
	if !strings.Contains(err.Error(), "openssl rand") {
		t.Errorf("error %q does not tell the operator how to generate a secret", err.Error())
	}
}

func TestLoadSessionSecretPlaceholderRejected(t *testing.T) {
	env := fullEnv()
	// Long enough to pass the length gate — must still be rejected because
	// it is the documented placeholder.
	env["SESSION_SECRET"] = "change-me-long-random-string-padded-well-past-32"

	_, err := load(lookupFromMap(env))
	if err == nil {
		t.Fatal("load() succeeded with placeholder SESSION_SECRET, want error")
	}
	if !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Errorf("error %q does not name SESSION_SECRET", err.Error())
	}
	if !strings.Contains(err.Error(), "openssl rand") {
		t.Errorf("error %q does not tell the operator how to generate a secret", err.Error())
	}
}

func TestLoadSessionSecretExactMinimumLengthPasses(t *testing.T) {
	env := fullEnv()
	// Exactly the 32-char minimum — pins the boundary so an off-by-one in
	// the length comparison is caught from both sides (31 is rejected above).
	env["SESSION_SECRET"] = strings.Repeat("x", 32)

	cfg, err := load(lookupFromMap(env))
	if err != nil {
		t.Fatalf("load() rejected a SESSION_SECRET of exactly 32 chars: %v", err)
	}
	if cfg.SessionSecret != env["SESSION_SECRET"] {
		t.Errorf("SessionSecret = %q, want the provided 32-char value", cfg.SessionSecret)
	}
}

func TestLoadSessionSecretRealValuePasses(t *testing.T) {
	env := fullEnv()
	env["SESSION_SECRET"] = "kJ8vN2xQ5wR9tY3uI6oP1aS4dF7gH0jZbC8eM5nV2xL9"

	cfg, err := load(lookupFromMap(env))
	if err != nil {
		t.Fatalf("load() rejected a real SESSION_SECRET: %v", err)
	}
	if cfg.SessionSecret != env["SESSION_SECRET"] {
		t.Errorf("SessionSecret = %q, want the provided value", cfg.SessionSecret)
	}
}

func TestLoadWhatsAppDummyRejected(t *testing.T) {
	env := fullEnv()
	env["WHATSAPP_ACCESS_TOKEN"] = "dummy"

	_, err := load(lookupFromMap(env))
	if err == nil {
		t.Fatal("load() succeeded with dummy WHATSAPP_ACCESS_TOKEN, want error")
	}
	if !strings.Contains(err.Error(), "WHATSAPP_ACCESS_TOKEN") {
		t.Errorf("error %q does not name WHATSAPP_ACCESS_TOKEN", err.Error())
	}
	if !strings.Contains(err.Error(), "README") {
		t.Errorf("error %q does not point at the README Meta runbook", err.Error())
	}
}

func TestLoadWhatsAppChangeMePrefixRejected(t *testing.T) {
	for _, name := range []string{"WHATSAPP_ACCESS_TOKEN", "WHATSAPP_PHONE_NUMBER_ID", "WHATSAPP_APP_SECRET", "WHATSAPP_VERIFY_TOKEN"} {
		env := fullEnv()
		env[name] = "change-me-please"

		_, err := load(lookupFromMap(env))
		if err == nil {
			t.Fatalf("load() succeeded with change-me %s, want error", name)
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err.Error(), name)
		}
	}
}

func TestLoadWhatsAppAllFourDummyNamedInOneError(t *testing.T) {
	env := fullEnv()
	env["WHATSAPP_ACCESS_TOKEN"] = "dummy"
	env["WHATSAPP_PHONE_NUMBER_ID"] = "dummy"
	env["WHATSAPP_APP_SECRET"] = "dummy"
	env["WHATSAPP_VERIFY_TOKEN"] = "dummy"

	_, err := load(lookupFromMap(env))
	if err == nil {
		t.Fatal("load() succeeded with all WhatsApp vars dummy, want error")
	}
	for _, name := range []string{"WHATSAPP_ACCESS_TOKEN", "WHATSAPP_PHONE_NUMBER_ID", "WHATSAPP_APP_SECRET", "WHATSAPP_VERIFY_TOKEN"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s (want all four offenders in one error)", err.Error(), name)
		}
	}
}

func TestLoadWhatsAppValidValuesPass(t *testing.T) {
	cfg, err := load(lookupFromMap(fullEnv()))
	if err != nil {
		t.Fatalf("load() rejected real-looking WhatsApp values: %v", err)
	}
	if cfg.WhatsAppAccessToken != "token" {
		t.Errorf("WhatsAppAccessToken = %q, want %q", cfg.WhatsAppAccessToken, "token")
	}
}

func TestLoadAnthropicKeyDummyStillBoots(t *testing.T) {
	env := fullEnv()
	env["ANTHROPIC_API_KEY"] = "dummy"

	_, err := load(lookupFromMap(env))
	if err != nil {
		t.Fatalf("load() rejected dummy ANTHROPIC_API_KEY, want it to still boot (Epic 3 concern): %v", err)
	}
}

func TestLoadReadsProcessEnv(t *testing.T) {
	for name, value := range fullEnv() {
		t.Setenv(name, value)
	}
	t.Setenv("PORT", "9999")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error with full process env: %v", err)
	}
	if cfg.Port != "9999" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9999")
	}
}

// --- 2026-08-02 code review: placeholder validation was exact-match and
// case-sensitive, with no whitespace trim ---

// The README's local workflow sources server/.env through Git Bash
// (`set -a; . server/.env`). A CRLF-authored .env on Windows leaves a trailing
// carriage return on every value, so "dummy\r" slipped past the placeholder
// check and the server booted on placeholder credentials — the exact failure
// Task 1 exists to prevent. A stray \r on the app secret would also fail every
// HMAC with an opaque "mismatch" that reads like forgery, not a config error.
func TestLoadWhatsAppPlaceholderWithTrailingWhitespaceRejected(t *testing.T) {
	for _, suffix := range []string{"\r", " ", "\t", "\r\n"} {
		env := fullEnv()
		env["WHATSAPP_ACCESS_TOKEN"] = "dummy" + suffix
		if _, err := load(lookupFromMap(env)); err == nil {
			t.Errorf("load() accepted placeholder %q", "dummy"+suffix)
		} else if !strings.Contains(err.Error(), "WHATSAPP_ACCESS_TOKEN") {
			t.Errorf("error does not name the offending variable: %v", err)
		}
	}
}

func TestLoadWhatsAppPlaceholderCaseInsensitive(t *testing.T) {
	for _, value := range []string{"Dummy", "DUMMY", "Change-Me-Please", "CHANGE-ME"} {
		env := fullEnv()
		env["WHATSAPP_APP_SECRET"] = value
		if _, err := load(lookupFromMap(env)); err == nil {
			t.Errorf("load() accepted placeholder %q", value)
		} else if !strings.Contains(err.Error(), "WHATSAPP_APP_SECRET") {
			t.Errorf("error does not name the offending variable: %v", err)
		}
	}
}

// Trimming must reach the Config, not just the validation: an untrimmed secret
// would be used verbatim as the HMAC key and break every signature check.
func TestLoadTrimsWhitespaceFromValues(t *testing.T) {
	env := fullEnv()
	env["WHATSAPP_APP_SECRET"] = "  real-secret\r\n"
	cfg, err := load(lookupFromMap(env))
	if err != nil {
		t.Fatalf("load() returned error: %v", err)
	}
	if cfg.WhatsAppAppSecret != "real-secret" {
		t.Errorf("WhatsAppAppSecret = %q, want %q (untrimmed values break HMAC verification)", cfg.WhatsAppAppSecret, "real-secret")
	}
}

// WHATSAPP_API_BASE_URL is the one optional WhatsApp variable: unset must
// boot cleanly and leave the client's pinned default in force.
func TestLoadAPIBaseURLUnsetIsEmpty(t *testing.T) {
	cfg, err := load(lookupFromMap(fullEnv()))
	if err != nil {
		t.Fatalf("load() returned error without WHATSAPP_API_BASE_URL: %v", err)
	}
	if cfg.WhatsAppAPIBaseURL != "" {
		t.Errorf("WhatsAppAPIBaseURL = %q, want empty when unset", cfg.WhatsAppAPIBaseURL)
	}
}

func TestLoadAPIBaseURLCarriedThrough(t *testing.T) {
	env := fullEnv()
	env["WHATSAPP_API_BASE_URL"] = "  http://127.0.0.1:9099/v25.0\r\n"
	cfg, err := load(lookupFromMap(env))
	if err != nil {
		t.Fatalf("load() returned error: %v", err)
	}
	if cfg.WhatsAppAPIBaseURL != "http://127.0.0.1:9099/v25.0" {
		t.Errorf("WhatsAppAPIBaseURL = %q, want the trimmed override", cfg.WhatsAppAPIBaseURL)
	}
}

// The override is deliberately exempt from placeholder validation — it is a
// test-harness knob, not a secret.
func TestLoadAPIBaseURLNotPlaceholderValidated(t *testing.T) {
	env := fullEnv()
	env["WHATSAPP_API_BASE_URL"] = "dummy"
	cfg, err := load(lookupFromMap(env))
	if err != nil {
		t.Fatalf("load() rejected a non-secret override value: %v", err)
	}
	if cfg.WhatsAppAPIBaseURL != "dummy" {
		t.Errorf("WhatsAppAPIBaseURL = %q, want %q", cfg.WhatsAppAPIBaseURL, "dummy")
	}
}

// A whitespace-only value is as absent as an empty one.
func TestLoadWhitespaceOnlyValueCountsAsMissing(t *testing.T) {
	env := fullEnv()
	env["WHATSAPP_VERIFY_TOKEN"] = "   "
	_, err := load(lookupFromMap(env))
	if err == nil {
		t.Fatal("load() accepted a whitespace-only WHATSAPP_VERIFY_TOKEN")
	}
	if !strings.Contains(err.Error(), "WHATSAPP_VERIFY_TOKEN") {
		t.Errorf("error does not name the variable: %v", err)
	}
}
