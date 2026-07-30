package wa

import (
	"strings"
	"testing"
)

func TestPhoneLast4(t *testing.T) {
	cases := map[string]string{
		"972500001234": "1234",
		"1234":         "1234",
		"12":           "****",
		"":             "****",
		"abcd1234":     "1234",
	}
	for in, want := range cases {
		if got := PhoneLast4(in); got != want {
			t.Errorf("PhoneLast4(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactDigits(t *testing.T) {
	cases := map[string]string{
		"Recipient 972500000000 is not in allowed list": "Recipient [redacted] is not in allowed list",
		"Invalid parameter":                             "Invalid parameter",
		"multiple 12345678 and 98765432 runs":           "multiple [redacted] and [redacted] runs",
		"error code 131030 stays":                       "error code 131030 stays",      // 6 digits: below the 7+ threshold, codes survive
		"boundary 1234567 redacted":                     "boundary [redacted] redacted", // exactly 7 digits
	}
	for in, want := range cases {
		if got := RedactDigits(in); got != want {
			t.Errorf("RedactDigits(%q) = %q, want %q", in, got, want)
		}
	}
}

// leakyWamid mirrors the structure of a real wamid captured from a live Meta
// delivery on 2026-07-30 — "wamid." + base64(MSISDN) + base64(message uuid) —
// but encodes the reserved test number 972500000000 rather than anyone's real
// one. The structure is the point: the sender's full phone number is
// recoverable from the id, which is the leak this digest exists to close.
const (
	syntheticMSISDN       = "972500000000"
	syntheticMSISDNBase64 = "OTcyNTAwMDAwMDAw" // base64(syntheticMSISDN)
	leakyWamid            = "wamid.HBgM" + syntheticMSISDNBase64 +
		"FQIAEhggQUNGQTQwRjQ5REFDMDU0NEM1QTkxRDlBQTgzNzIyODMA"
)

func TestWaMessageIDDigestHidesEmbeddedPhone(t *testing.T) {
	// Guard the premise: if this ever stops holding, the test below is vacuous.
	if !strings.Contains(leakyWamid, syntheticMSISDNBase64) {
		t.Fatal("fixture no longer embeds the encoded MSISDN — test would prove nothing")
	}

	got := WaMessageIDDigest(leakyWamid)

	// Neither the plain MSISDN nor Meta's base64 encoding of it may survive.
	for _, leak := range []string{syntheticMSISDN, syntheticMSISDNBase64, "0000000", "0000"} {
		if strings.Contains(got, leak) {
			t.Errorf("digest %q leaks %q", got, leak)
		}
	}
	if strings.Contains(got, leakyWamid) || strings.Contains(leakyWamid, got) {
		t.Errorf("digest %q is not independent of the raw wamid", got)
	}
}

func TestWaMessageIDDigestStableAndDistinct(t *testing.T) {
	if a, b := WaMessageIDDigest(leakyWamid), WaMessageIDDigest(leakyWamid); a != b {
		t.Errorf("digest not stable: %q != %q — log lines for one message would not correlate", a, b)
	}
	if a, b := WaMessageIDDigest("wamid.AAA"), WaMessageIDDigest("wamid.BBB"); a == b {
		t.Errorf("distinct wamids collided on %q", a)
	}
	if got := WaMessageIDDigest(""); got != "" {
		t.Errorf("WaMessageIDDigest(\"\") = %q, want \"\"", got)
	}
	// Fixed shape keeps log lines scannable: "wamid_" + 16 hex chars.
	got := WaMessageIDDigest(leakyWamid)
	if !strings.HasPrefix(got, "wamid_") || len(got) != len("wamid_")+16 {
		t.Errorf("unexpected digest shape %q", got)
	}
}
