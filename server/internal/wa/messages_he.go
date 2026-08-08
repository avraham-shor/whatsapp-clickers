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
//	Result - correct         -> story 3.8 (below)
//	Result - correct + bonus -> story 3.8 (below)
//	Result - wrong           -> story 3.8 (below)
//	Result - wrong, last     -> story 3.8 (below)
//	Final results            -> story 3.9 (below)
//	Final results - spectator -> story 3.9 (below)
//	Final results - no winner -> story 3.9 (below)
//	Winner's final message   -> story 3.9 (below)
//
// Voice (UX-DR15): warm-playful, short, gender-neutral (plural imperatives,
// never the masculine-singular greeting form), symbolic emoji only and at
// most one per message. Nothing here is exported: wa owns all outbound copy,
// and the dependency direction means no other package ever needs these
// strings.

import (
	"fmt"
	"strconv"
	"strings"
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

// msgResultCorrectTemplate is the Result — correct row (no Speed Bonus).
const msgResultCorrectTemplate = "נכון! 🎉 +%s נקודות\nמקום %s בטבלה"

// resultCorrectMessage returns the finished Result-correct copy. points
// is the Game's configured points-per-correct-answer (no bonus); rank is
// the Participant's post-reveal cumulative Leaderboard rank.
func resultCorrectMessage(points int32, rank int) string {
	return fmt.Sprintf(msgResultCorrectTemplate, ltr(strconv.Itoa(int(points))), ltr(strconv.Itoa(rank)))
}

// msgResultCorrectBonusTemplate is the Result — correct + bonus row. Two
// emoji (🎉 and ⚡) are allowed here — the templates table's stated
// exception for winner/final-results/speed-bonus messages (EXPERIENCE.md
// A20); every other row in this file stays at one.
const msgResultCorrectBonusTemplate = "נכון! 🎉 +%s נקודות\n⚡ בונוס מהירות +%s\nמקום %s בטבלה"

// resultCorrectBonusMessage returns the finished Result-correct-with-bonus
// copy. basePoints is points-per-correct-answer; bonusPoints is the
// Speed Bonus component alone — shown as two separate numbers, never
// their sum.
func resultCorrectBonusMessage(basePoints, bonusPoints int32, rank int) string {
	return fmt.Sprintf(msgResultCorrectBonusTemplate, ltr(strconv.Itoa(int(basePoints))), ltr(strconv.Itoa(int(bonusPoints))), ltr(strconv.Itoa(rank)))
}

// msgResultWrongTemplate is the Result — wrong row for every question
// except the game's last (EXPERIENCE.md A5's "עוד הכול פתוח" closing
// line). correctAnswer is inserted raw, NOT isolated: it is Hebrew
// content (an Accepted Answer or MCQ option text), the same treatment
// questionMCQMessage/questionFreeTextMessage give the question text
// itself — content, not an LTR token like a code or digit.
const msgResultWrongTemplate = "לא נכון הפעם. התשובה: %s\nמקום %s בטבלה — עוד הכול פתוח!"

func resultWrongMessage(correctAnswer string, rank int) string {
	return fmt.Sprintf(msgResultWrongTemplate, correctAnswer, ltr(strconv.Itoa(rank)))
}

// msgResultWrongLastTemplate is the Result — wrong, last-question row:
// the same content as msgResultWrongTemplate minus "עוד הכול פתוח"
// (EXPERIENCE.md A5 — there is no more game left to stay open about).
const msgResultWrongLastTemplate = "לא נכון הפעם. התשובה: %s\nמקום %s בטבלה"

func resultWrongLastMessage(correctAnswer string, rank int) string {
	return fmt.Sprintf(msgResultWrongLastTemplate, correctAnswer, ltr(strconv.Itoa(rank)))
}

// joinNames renders one or more display names as a Hebrew list: a single
// name alone, two joined by the vav conjunction, three or more
// comma-separated with the conjunction before the last. Every name is
// ltr()-isolated individually, same reasoning as welcomeMessage: a
// WhatsApp profile name can legitimately be Latin-script (mixed-language
// families), and isolating a Hebrew one is harmless.
//
// No cap on the number of names. EXPERIENCE.md's A16 "up to three names"
// is an Audience-Display layout constraint (finite projector space), not
// a copy rule; a WhatsApp message has no such limit, and truncating the
// list would drop a real winner's name from the one message that names
// them.
//
// Callers guarantee len(names) > 0 — the no-winner case is a different
// template entirely (msgFinalResultsNoWinner), selected upstream in
// FinalNotifier.DispatchGameFinished.
func joinNames(names []string) string {
	isolated := make([]string, 0, len(names))
	for _, n := range names {
		isolated = append(isolated, ltr(n))
	}
	if len(isolated) == 1 {
		return isolated[0]
	}
	last := len(isolated) - 1
	return strings.Join(isolated[:last], ", ") + nameConjunction + isolated[last]
}

// nameConjunction is the vav-prefix separator before the final name in a
// multi-name list, per the templates table's tie forms. It lives here,
// not inline in joinNames, so every Hebrew fragment in this package stays
// a named constant in this file.
const nameConjunction = " ו-"

// msgFinalResultsTemplate is the Final results row for a player when
// exactly one participant holds first place: the winner line, then the
// recipient's own placing. Two emoji are permitted on this row
// (EXPERIENCE.md A20's winner/final-results exception); it uses one.
const msgFinalResultsTemplate = "המשחק נגמר! 🏆 הזוכה: %s עם %s נקודות.\nסיימת במקום %s עם %s נקודות — כל הכבוד!"

// finalResultsMessage returns the finished Final-results copy for a
// player. winnerNames must hold exactly one name; winnerScore is the
// winning cumulative score; rank/score are this recipient's own final
// placing. A winner receives this too, then the winner variant on top
// (epic AC-3's "additionally").
func finalResultsMessage(winnerNames []string, winnerScore int32, rank int, score int32) string {
	return fmt.Sprintf(msgFinalResultsTemplate,
		joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))),
		ltr(strconv.Itoa(rank)), ltr(strconv.Itoa(int(score))))
}

// msgFinalResultsTieTemplate is the Final results row's tie form: the
// plural winner noun, all tied names joined (EXPERIENCE.md A16). The
// score appears once — it is by definition the same for every tied
// winner.
const msgFinalResultsTieTemplate = "המשחק נגמר! 🏆 הזוכים: %s עם %s נקודות.\nסיימת במקום %s עם %s נקודות — כל הכבוד!"

func finalResultsTieMessage(winnerNames []string, winnerScore int32, rank int, score int32) string {
	return fmt.Sprintf(msgFinalResultsTieTemplate,
		joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))),
		ltr(strconv.Itoa(rank)), ltr(strconv.Itoa(int(score))))
}

// msgFinalResultsSpectatorTemplate is the Final results - spectator row:
// the winner line alone. A Spectator has no score and no rank
// (GetLeaderboard filters role = 'player'), so the personal second line
// of msgFinalResultsTemplate has nothing to render.
const msgFinalResultsSpectatorTemplate = "המשחק נגמר! 🏆 הזוכה: %s עם %s נקודות."

func finalResultsSpectatorMessage(winnerNames []string, winnerScore int32) string {
	return fmt.Sprintf(msgFinalResultsSpectatorTemplate, joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))))
}

// msgFinalResultsSpectatorTieTemplate is the spectator row's tie form.
const msgFinalResultsSpectatorTieTemplate = "המשחק נגמר! 🏆 הזוכים: %s עם %s נקודות."

func finalResultsSpectatorTieMessage(winnerNames []string, winnerScore int32) string {
	return fmt.Sprintf(msgFinalResultsSpectatorTieTemplate, joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))))
}

// msgFinalResultsNoWinner is the Final results - no winner row, sent to
// players and spectators alike when no participant finished with a
// positive score (a game stopped before the first Reveal, or one with no
// players at all). No trophy: EXPERIENCE.md reserves it for the winner
// moment, and there is none. No placeholders — a rank line would read
// "place 1" for every single recipient, which is exactly the outcome
// this row exists to avoid.
const msgFinalResultsNoWinner = "המשחק נגמר! הפעם לא נצברו נקודות — נתראה במשחק הבא!"

func finalResultsNoWinnerMessage() string {
	return msgFinalResultsNoWinner
}

// msgWinnerFinalTemplate is the Winner's final message row, sent only to
// the winner and only on top of their Final-results message — UJ-2's
// emotional climax. Singular verb form for a sole winner.
const msgWinnerFinalTemplate = "מזל טוב, %s! 🏆 ניצחת עם %s נקודות!"

func winnerFinalMessage(winnerNames []string, winnerScore int32) string {
	return fmt.Sprintf(msgWinnerFinalTemplate, joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))))
}

// msgWinnerFinalTieTemplate is the Winner's final message tie form:
// every tied winner receives the same jointly-addressed message naming
// all of them, with the plural verb (EXPERIENCE.md A16).
const msgWinnerFinalTieTemplate = "מזל טוב, %s! 🏆 ניצחתם עם %s נקודות!"

func winnerFinalTieMessage(winnerNames []string, winnerScore int32) string {
	return fmt.Sprintf(msgWinnerFinalTieTemplate, joinNames(winnerNames), ltr(strconv.Itoa(int(winnerScore))))
}
