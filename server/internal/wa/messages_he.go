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
//	Help (Universal Reply)   -> story 2.2 (below)
//	Welcome                  -> story 2.4 (below)
//	Pre-lobby reply          -> story 2.4 (below)
//	Invalid code             -> story 2.4 (below)
//	Name updated             -> story 2.4 (below)
//	Spectator notice         -> story 2.5 (below)
//	Question - MCQ           -> story 3.2 (below)
//	Question - Free-Text     -> story 3.2 (below)
//	Acknowledgment           -> story 3.3 (below)
//	Already answered         -> story 3.3 (below)
//	Format hint (MCQ)        -> story 3.3 (below)
//	Too long (Free-Text)     -> story 3.3 (below)
//	Question closed          -> story 3.3 (below)
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

import (
	"fmt"
	"strconv"
)

// renamePrefix is the שם: command prefix inbound.go's parseRenameName
// recognizes. It lives here, not inbound.go, because this is the one file
// exempt from CI's Hebrew-literal copy-centralization grep — inbound.go
// stays free of raw Hebrew literals even though this token is inbound
// parsing, not outbound copy.
const renamePrefix = "שם:"

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

// msgWelcomeTemplate is the Welcome row, sent on a first successful JOIN.
// Both %s placeholders take the same resolved display name — the second
// occurrence is the name-correction hint (EXPERIENCE.md A3/OQ-3). "שם: רחל
// לוי" is fixed literal Hebrew (an example, like the Help message's "JOIN
// COHEN24") — not a placeholder, no isolation needed.
//
// The display name is isolated with ltr(), same as a JOIN code: DESIGN.md/
// EXPERIENCE.md only name "JOIN, codes, digits" as requiring isolation, but a
// WhatsApp profile name can legitimately be Latin-script (mixed-language
// families) — isolating a Hebrew name is harmless, and isolating a Latin one
// is the only thing standing between a correct render and scrambled bidi
// reordering.
const msgWelcomeTemplate = "היי %s, נרשמת! 🎉 השאירו את הצ'אט הזה פתוח — השאלות יגיעו לכאן. (לא %s? שלחו לדוגמה — שם: רחל לוי)"

// welcomeMessage returns the finished Welcome copy for a newly (or
// repeatedly) registered Participant.
func welcomeMessage(displayName string) string {
	return fmt.Sprintf(msgWelcomeTemplate, ltr(displayName), ltr(displayName))
}

// msgPreLobbyReply is sent when a valid JOIN code's game is still in draft
// (the Organizer has not opened the lobby yet) — no placeholders.
const msgPreLobbyReply = "הקוד נכון! ההרשמה עוד לא נפתחה — שלחו שוב את ההודעה כשהמארגן מכריז שמתחילים."

// preLobbyMessage returns the Pre-lobby copy.
func preLobbyMessage() string {
	return msgPreLobbyReply
}

// msgInvalidCodeTemplate echoes the code the sender used (isolated, same as
// helpMessage's example code) alongside the fixed "JOIN" token.
const msgInvalidCodeTemplate = "הקוד %s לא נמצא. בדקו את הקוד עם המארגן ושלחו שוב: %s ואחריו הקוד."

// invalidCodeMessage returns the Invalid-code copy for the (uppercased) code
// the sender tried.
func invalidCodeMessage(code string) string {
	return fmt.Sprintf(msgInvalidCodeTemplate, ltr(code), ltr("JOIN"))
}

// msgNameUpdatedTemplate confirms a שם: rename.
const msgNameUpdatedTemplate = "עודכן ✓ מעכשיו: %s"

// nameUpdatedMessage returns the Name-updated confirmation copy.
func nameUpdatedMessage(displayName string) string {
	return fmt.Sprintf(msgNameUpdatedTemplate, ltr(displayName))
}

// msgSpectatorNoticeReply is sent when a valid JOIN arrives while the game
// is already running — question_open through leaderboard (EXPERIENCE.md's
// "Question open"/"Between questions²" columns). A finished game gets Help
// instead (story 2.5 AC 2) — this function is never called for one.
const msgSpectatorNoticeReply = "המשחק כבר התחיל! נרשמת כצופה — התוצאות יגיעו לכאן בסוף המשחק 🏆"

// spectatorNoticeMessage returns the Spectator-notice copy.
func spectatorNoticeMessage() string {
	return msgSpectatorNoticeReply
}

// msgQuestionMCQTemplate is the Question — MCQ row: question number/total,
// question text, the four lettered options in fixed order, then the
// answer-format + time-limit line. Every Western-digit run is isolated via
// ltr() at the call site — this file's header rule ("mandatory for every
// template here") is not placeholder-only: the template's own fixed "1–4"
// digit range is an LTR run embedded in RTL text exactly like a placeholder
// digit, so it is isolated the same way.
// [ASSUMPTION — EXPERIENCE.md's table shows the row as plain text with no
// isolation marks (the rule lives in this file's header, not the table);
// flag for confirmation if a native-speaker WhatsApp render check reveals
// this over- or under-isolates.]
const msgQuestionMCQTemplate = "שאלה %s מתוך %s:\n%s\nא. %s\nב. %s\nג. %s\nד. %s\nהשיבו באות (א–ד) או בספרה (%s) — יש לכם %s שניות!"

// questionMCQMessage returns the finished MCQ Question copy. number/total
// are 1-based (number is the question's Position; total is the game's
// question count). options must have exactly 4 elements — the questions
// table's own CHECK constraint (questions_type_shape, migration 00003)
// already guarantees this for every mcq row; not re-validated here
// (validate at boundaries, trust internal invariants).
func questionMCQMessage(number, total int, text string, options []string, timeLimitSeconds int) string {
	return fmt.Sprintf(msgQuestionMCQTemplate,
		ltr(strconv.Itoa(number)), ltr(strconv.Itoa(total)), text,
		options[0], options[1], options[2], options[3],
		ltr("1–4"), ltr(strconv.Itoa(timeLimitSeconds)))
}

// msgQuestionFreeTextTemplate is the Question — Free-Text row.
const msgQuestionFreeTextTemplate = "שאלה %s מתוך %s:\n%s\nכתבו את התשובה בהודעה — יש לכם %s שניות!"

// questionFreeTextMessage returns the finished Free-Text Question copy.
func questionFreeTextMessage(number, total int, text string, timeLimitSeconds int) string {
	return fmt.Sprintf(msgQuestionFreeTextTemplate,
		ltr(strconv.Itoa(number)), ltr(strconv.Itoa(total)), text, ltr(strconv.Itoa(timeLimitSeconds)))
}

// msgAcknowledgment is the Acknowledgment row — the PRD-mandated "התקבל ✓"
// (SM-4/FR-5), sent immediately after the answer row is committed. No
// placeholders; the grade is never revealed here (FR-6).
const msgAcknowledgment = "התקבל ✓ — בהצלחה!"

func ackMessage() string {
	return msgAcknowledgment
}

// msgAlreadyAnswered is the Already-answered row (FR-8: first answer wins).
const msgAlreadyAnswered = "כבר ענית ✓ התשובה הראשונה היא שקובעת."

func alreadyAnsweredMessage() string {
	return msgAlreadyAnswered
}

// msgFormatHintMCQTemplate is the Format-hint (MCQ) row. Its "1–4" digit
// range is an LTR run embedded in RTL text exactly like the identical
// range in msgQuestionMCQTemplate — isolated the same way, per this
// file's header rule.
const msgFormatHintMCQTemplate = "כדי לענות שלחו אות (א–ד) או ספרה (%s) — עוד יש זמן!"

func formatHintMessage() string {
	return fmt.Sprintf(msgFormatHintMCQTemplate, ltr("1–4"))
}

// msgTooLongFreeTextTemplate is the Too-long (Free-Text) row. "200" is a
// digit run, isolated per this file's header rule (same as every other
// digit token here).
const msgTooLongFreeTextTemplate = "התשובה ארוכה מדי — עד %s תווים. שלחו שוב, בקצרה!"

func tooLongMessage() string {
	return fmt.Sprintf(msgTooLongFreeTextTemplate, ltr("200"))
}

// msgQuestionClosed is the Question-closed row (FR-7: late answers).
const msgQuestionClosed = "השאלה נסגרה — מתכוננים לשאלה הבאה!"

func questionClosedMessage() string {
	return msgQuestionClosed
}
