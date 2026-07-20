// Package wa is the WhatsApp Cloud API transport layer: webhook intake,
// outbound send, and the rate-limited dispatcher. It carries no game logic
// (see 2.2+ for that) and depends on nothing but stdlib + x/time/rate —
// dependencies it needs arrive as consumer-defined interfaces.
package wa

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
