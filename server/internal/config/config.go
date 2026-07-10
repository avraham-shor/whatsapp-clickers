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
	AnthropicAPIKey       string
	SessionSecret         string
}

const defaultPort = "8080"

// Load reads the process environment. It returns an error naming every
// missing required variable so the operator can fix them all in one pass.
func Load() (*Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (*Config, error) {
	var missing []string
	require := func(name string) string {
		v, ok := lookup(name)
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

	cfg.Port = defaultPort
	if port, ok := lookup("PORT"); ok && port != "" {
		cfg.Port = port
	}
	return cfg, nil
}
