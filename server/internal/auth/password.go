// Package auth owns credential verification and server-side sessions.
// Handlers stay thin; everything security-sensitive lives here.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// OWASP Password Storage Cheat Sheet baseline for argon2id.
const (
	argonMemoryKiB = 19456
	argonTime      = 2
	argonThreads   = 1
	argonSaltLen   = 16
	argonKeyLen    = 32

	// maxArgonMemoryKiB (2 GiB) bounds the m= parameter parsed from stored
	// hashes so a corrupted row cannot demand an arbitrary allocation.
	maxArgonMemoryKiB = 1 << 21
)

var errMalformedHash = errors.New("malformed argon2id hash")

// HashPassword derives an argon2id key and encodes it as a PHC string, so
// the parameters travel with the hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the PHC-encoded hash.
// Parameters are parsed from the stored hash — not assumed — so they can be
// strengthened later without invalidating existing rows.
func VerifyPassword(password, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errMalformedHash
	}
	var (
		memory   uint32
		timeCost uint32
		threads  uint8
	)
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false, errMalformedHash
	}
	// argon2.IDKey panics on zero time/threads, and memory sizes the
	// allocation — a tampered hash must fail as malformed, never run.
	if timeCost == 0 || threads == 0 || memory < 8*uint32(threads) || memory > maxArgonMemoryKiB {
		return false, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false, errMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errMalformedHash
	}
	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
