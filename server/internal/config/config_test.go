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
		"SESSION_SECRET":           "session",
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
	if cfg.SessionSecret != "session" {
		t.Errorf("SessionSecret = %q, want %q", cfg.SessionSecret, "session")
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
