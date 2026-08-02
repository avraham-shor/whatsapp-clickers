// Package config loads and validates all environment-derived settings.
// The server refuses to boot when any required variable is missing (fail-fast).
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds every environment variable the server consumes.
type Config struct {
	Port                  string
	DatabaseURL           string
	WhatsAppAccessToken   string
	WhatsAppPhoneNumberID string
	WhatsAppAppSecret     string
	WhatsAppVerifyToken   string
	// WhatsAppAPIBaseURL overrides the Cloud API base URL. OPTIONAL and empty
	// in production, where the client's pinned default is what we want; it
	// exists so an E2E harness can point the server at a local fake provider
	// and assert that replies are actually *sent*, not merely that the webhook
	// returned 200. Neither required nor placeholder-validated.
	WhatsAppAPIBaseURL string
	AnthropicAPIKey    string
	SessionSecret      string
}

const defaultPort = "8080"

// minSessionSecretLen guards against weak HMAC keys for session tokens.
const minSessionSecretLen = 32

// Load reads the process environment. It returns an error naming every
// missing required variable so the operator can fix them all in one pass.
func Load() (*Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (*Config, error) {
	var missing []string
	require := func(name string) string {
		v, ok := lookup(name)
		// Trim before every downstream check. The README's local workflow
		// sources server/.env through Git Bash (`set -a; . server/.env`), and a
		// CRLF-authored .env on Windows leaves a trailing \r on every value:
		// "dummy\r" would slip past the placeholder validation below, and an
		// app secret carrying \r fails every HMAC with an opaque "mismatch"
		// that reads like forgery rather than a config error.
		v = strings.TrimSpace(v)
		if !ok || v == "" {
			missing = append(missing, name)
		}
		return v
	}

	cfg := &Config{
		DatabaseURL:           require("DATABASE_URL"),
		WhatsAppAccessToken:   require("WHATSAPP_ACCESS_TOKEN"),
		WhatsAppPhoneNumberID: require("WHATSAPP_PHONE_NUMBER_ID"),
		WhatsAppAppSecret:     require("WHATSAPP_APP_SECRET"),
		WhatsAppVerifyToken:   require("WHATSAPP_VERIFY_TOKEN"),
		AnthropicAPIKey:       require("ANTHROPIC_API_KEY"),
		SessionSecret:         require("SESSION_SECRET"),
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s (see server/.env.example)", strings.Join(missing, ", "))
	}

	if err := validateSessionSecret(cfg.SessionSecret); err != nil {
		return nil, err
	}

	if err := validateWhatsAppValues(cfg); err != nil {
		return nil, err
	}

	cfg.Port = defaultPort
	if port, ok := lookup("PORT"); ok && port != "" {
		cfg.Port = port
	}

	// Optional: unset means "use the client's pinned default". Trimmed like
	// every other value so a CRLF-authored .env cannot smuggle a \r into a URL.
	if baseURL, ok := lookup("WHATSAPP_API_BASE_URL"); ok {
		cfg.WhatsAppAPIBaseURL = strings.TrimSpace(baseURL)
	}
	return cfg, nil
}

// validateSessionSecret rejects secrets that cannot safely sign session
// tokens: the documented placeholder and anything shorter than 32 chars.
func validateSessionSecret(secret string) error {
	const hint = "generate one with `openssl rand -base64 48` (see server/.env.example)"
	if strings.HasPrefix(secret, "change-me") {
		return fmt.Errorf("SESSION_SECRET is still the documented placeholder; %s", hint)
	}
	if len(secret) < minSessionSecretLen {
		return fmt.Errorf("SESSION_SECRET must be at least %d characters, got %d; %s", minSessionSecretLen, len(secret), hint)
	}
	return nil
}

// validateWhatsAppValues rejects the documented .env.example placeholders
// for the four Meta WhatsApp Cloud API variables, collecting every offender
// into one error (fix-all-in-one-pass, mirrors validateSessionSecret).
// ANTHROPIC_API_KEY is deliberately NOT validated here: it stays the
// documented "dummy" placeholder until Epic 3 (story 3.6) consumes it, so
// validating it now would fail-fast boots for zero safety gain.
func validateWhatsAppValues(cfg *Config) error {
	candidates := []struct {
		name  string
		value string
	}{
		{"WHATSAPP_ACCESS_TOKEN", cfg.WhatsAppAccessToken},
		{"WHATSAPP_PHONE_NUMBER_ID", cfg.WhatsAppPhoneNumberID},
		{"WHATSAPP_APP_SECRET", cfg.WhatsAppAppSecret},
		{"WHATSAPP_VERIFY_TOKEN", cfg.WhatsAppVerifyToken},
	}

	// Case-insensitive: this is a last-resort safety net, and "Dummy" or
	// "CHANGE-ME" are the same mistake as the lowercase forms. Values are
	// already TrimSpace'd by require().
	var offenders []string
	for _, c := range candidates {
		lower := strings.ToLower(c.value)
		if lower == "dummy" || strings.HasPrefix(lower, "change-me") {
			offenders = append(offenders, c.name)
		}
	}
	if len(offenders) == 0 {
		return nil
	}
	return fmt.Errorf("still the documented placeholder value: %s; see the README \"Meta WhatsApp Business setup\" runbook for real values", strings.Join(offenders, ", "))
}
