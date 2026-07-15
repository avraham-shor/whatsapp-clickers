package store

import (
	"strings"
	"testing"
)

func TestNewJoinCodeShape(t *testing.T) {
	code, err := NewJoinCode()
	if err != nil {
		t.Fatalf("NewJoinCode() error: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("NewJoinCode() = %q, want 6 characters", code)
	}
	for _, r := range code {
		if !strings.ContainsRune(joinCodeCharset, r) {
			t.Errorf("NewJoinCode() = %q contains %q, outside the charset", code, r)
		}
	}
}

func TestNewJoinCodeCharsetExcludesAmbiguous(t *testing.T) {
	// I/O/0/1 are excluded so a code read aloud across a room is unambiguous.
	for _, forbidden := range "IO01" {
		if strings.ContainsRune(joinCodeCharset, forbidden) {
			t.Errorf("charset %q must not contain %q", joinCodeCharset, forbidden)
		}
	}
	// 32 characters divide 256 evenly — byte-mod sampling stays uniform.
	if len(joinCodeCharset) != 32 {
		t.Errorf("charset has %d characters, want exactly 32", len(joinCodeCharset))
	}
}

func TestNewJoinCodeUppercaseOnly(t *testing.T) {
	code, err := NewJoinCode()
	if err != nil {
		t.Fatalf("NewJoinCode() error: %v", err)
	}
	if code != strings.ToUpper(code) {
		t.Errorf("NewJoinCode() = %q, want uppercase only", code)
	}
}

func TestNewJoinCodeVariesAcrossCalls(t *testing.T) {
	// 100 draws from a 32^6 (~1.07e9) space colliding means the generator is
	// not actually random — treat any duplicate as failure.
	seen := make(map[string]bool, 100)
	for range 100 {
		code, err := NewJoinCode()
		if err != nil {
			t.Fatalf("NewJoinCode() error: %v", err)
		}
		if seen[code] {
			t.Fatalf("NewJoinCode() repeated %q within 100 calls", code)
		}
		seen[code] = true
	}
}
