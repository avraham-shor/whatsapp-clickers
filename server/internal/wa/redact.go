// Package wa is the WhatsApp Cloud API transport layer: webhook intake,
// outbound send, and the rate-limited dispatcher. It carries no game logic
// (see 2.2+ for that) and depends on nothing but stdlib + x/time/rate —
// dependencies it needs arrive as consumer-defined interfaces.
package wa

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

// PhoneLast4 returns the last 4 digits of a WhatsApp MSISDN for logging.
// Full phone numbers must never appear in logs (NFR-4); every log
// attribute involving a phone number uses this helper's output under the
// "phone_last4" key.
func PhoneLast4(msisdn string) string {
	if len(msisdn) < 4 {
		return "****"
	}
	return msisdn[len(msisdn)-4:]
}

// longDigitRun matches a run of 7+ digits — contiguous, or separated by the
// spaces, dots, hyphens, parentheses and leading "+" that phone numbers are
// routinely formatted with. The original P6 pattern was `\d{7,}`, which let
// "+972 50 123 4567" through untouched because no run reached 7 contiguous
// digits.
//
// This deliberately over-matches: two adjacent numeric tokens in one string
// (e.g. "code 131030 at 2026-07-30") can merge into a single match and be
// redacted together. That is the correct failure direction for a PII guard on
// an untrusted external string — the alternative failure mode is a
// Participant's phone number in the logs. Standalone short codes (HTTP status,
// Meta error code) are still left intact.
var longDigitRun = regexp.MustCompile(`\+?\d(?:[ .()\-]*\d){6,}`)

// RedactDigits masks every 7+ digit run in s with "[redacted]". Apply it to
// untrusted external strings (e.g. Meta's send-error message) before logging
// them, so a recipient MSISDN that Meta echoes back can never reach the logs
// (NFR-4).
func RedactDigits(s string) string {
	return longDigitRun.ReplaceAllString(s, "[redacted]")
}

// WaMessageIDDigest returns a log-safe, stable digest of a Meta wamid.
//
// Meta's wamid is not opaque: it base64-encodes the sender's full MSISDN in
// its leading segment. Verified 2026-07-30 against a live delivery — a wamid
// of the form "wamid.HBgM<base64-of-MSISDN>FQIAEhgg<base64-of-message-uuid>"
// yields the complete sender number when its payload is base64-decoded.
// (Illustrative only; no real number is reproduced in this repo.)
// Logging the raw wamid would therefore leak every
// Participant's phone number (NFR-4) despite the phone_last4 discipline, and
// RedactDigits cannot catch it because the leak is base64, not a digit run.
//
// This digest is also what the dedupe ledger stores and keys on, so the raw
// wamid never reaches durable storage either (migration 00007 — the original
// schema kept it, on the since-disproved premise that wamids are opaque).
// Dedupe is unaffected: the value is only ever compared for equality, so
// log lines and ledger rows correlate directly on the same string.
func WaMessageIDDigest(waMessageID string) string {
	if waMessageID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(waMessageID))
	return "wamid_" + hex.EncodeToString(sum[:])[:16]
}
