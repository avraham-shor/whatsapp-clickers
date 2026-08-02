package wa

// This file is the ONLY home for outbound WhatsApp Hebrew copy. A Hebrew
// literal in any other non-test Go file is a named anti-pattern (architecture
// Communication Patterns + Enforcement Guidelines) and CI fails the build on
// it.
//
// The canonical row list is the UX spec, not this file:
// _bmad-output/planning-artifacts/ux-designs/ux-whatsapp-clickers-2026-07-05/
// EXPERIENCE.md -> "Voice and Tone" -> "WhatsApp message templates".
// A new message type is added to that table FIRST, then here.
//
// Rows land with the story that sends them — copy no code path can exercise
// cannot be reviewed for fidelity, and unused constants drift. Current map:
//
//	Help (Universal Reply)   -> story 2.2 (below; the only row implemented)
//	Welcome                  -> story 2.4
//	Pre-lobby reply          -> story 2.4
//	Invalid code             -> story 2.4
//	Name updated             -> story 2.4
//	Spectator notice         -> story 2.5
//	Question - MCQ           -> story 3.2
//	Question - Free-Text     -> story 3.2
//	Acknowledgment           -> story 3.3
//	Already answered         -> story 3.3
//	Format hint (MCQ)        -> story 3.3
//	Too long (Free-Text)     -> story 3.3
//	Question closed          -> story 3.3
//	Result - correct         -> story 3.8
//	Result - correct + bonus -> story 3.8
//	Result - wrong           -> story 3.8
//	Result - wrong, last     -> story 3.8
//	Final results            -> story 3.9
//	Winner's final message   -> story 3.9
//
// Voice (UX-DR15): warm-playful, short, gender-neutral (plural imperatives,
// never the masculine-singular greeting form), symbolic emoji only and at
// most one per message. Nothing here is exported: wa owns all outbound copy,
// and the dependency direction means no other package ever needs these
// strings.

import "fmt"

const (
	// lri and pdi isolate an LTR run so the Unicode bidi algorithm cannot
	// reorder it inside the surrounding RTL text. DESIGN.md Typography makes
	// this mandatory for every template here.
	//
	// Written as \u escapes on purpose: pasted, they are invisible in a diff,
	// and one stray copy-paste silently drops them — which ships a scrambled
	// example code as a Participant's first impression of the platform.
	lri = "\u2066" // LEFT-TO-RIGHT ISOLATE
	pdi = "\u2069" // POP DIRECTIONAL ISOLATE
)

// ltr isolates an LTR run for embedding in Hebrew copy. It is the plain-text
// equivalent of the web side's <bdi dir="ltr"> (game-editor-page.tsx).
func ltr(s string) string {
	return lri + s + pdi
}

// msgHelpTemplate is the Help (Universal Reply) row — the answer to anything
// the platform does not recognize, which in story 2.2 is everything (FR-2).
//
// The Hebrew is one contiguous literal with %s placeholders, never Hebrew
// fragments concatenated around bare Latin ones: mixed-direction concatenation
// is unreadable and mis-editable in every editor. The two placeholders are the
// JOIN keyword and a full example command, both isolated by helpMessage.
const msgHelpTemplate = "כאן משחק החידון! כדי להצטרף שלחו: %s ואחריו הקוד (לדוגמה: %s). את הקוד מקבלים מהמארגן."

// helpMessage returns the finished Help copy with both LTR runs isolated.
func helpMessage() string {
	return fmt.Sprintf(msgHelpTemplate, ltr("JOIN"), ltr("JOIN COHEN24"))
}
