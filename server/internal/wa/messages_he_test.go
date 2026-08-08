package wa

import (
	"fmt"
	"strings"
	"testing"
)

// The four canonical copy rows from EXPERIENCE.md's "WhatsApp message
// templates" table, held verbatim (with %s placeholders where dynamic) and
// WITHOUT bidi isolates — mirrors inbound_test.go's canonicalHelpCopy
// pattern. Each *Message function's isolates are stripped back out before
// comparison, so an editor mangling the mixed-direction literal fails the
// build instead of shipping scrambled Hebrew.
const (
	canonicalWelcomeCopy            = "היי %s, נרשמת! 🎉 השאירו את הצ'אט הזה פתוח — השאלות יגיעו לכאן. (לא %s? שלחו לדוגמה — שם: רחל לוי)"
	canonicalPreLobbyCopy           = "הקוד נכון! ההרשמה עוד לא נפתחה — שלחו שוב את ההודעה כשהמארגן מכריז שמתחילים."
	canonicalInvalidCodeCopy        = "הקוד %s לא נמצא. בדקו את הקוד עם המארגן ושלחו שוב: %s ואחריו הקוד."
	canonicalNameUpdatedCopy        = "עודכן ✓ מעכשיו: %s"
	canonicalSpectatorNoticeCopy    = "המשחק כבר התחיל! נרשמת כצופה — התוצאות יגיעו לכאן בסוף המשחק 🏆"
	canonicalQuestionMCQCopy        = "שאלה %s מתוך %s:\n%s\nא. %s\nב. %s\nג. %s\nד. %s\nהשיבו באות (א–ד) או בספרה (%s) — יש לכם %s שניות!"
	canonicalQuestionFreeTextCopy   = "שאלה %s מתוך %s:\n%s\nכתבו את התשובה בהודעה — יש לכם %s שניות!"
	canonicalAcknowledgmentCopy     = "התקבל ✓ — בהצלחה!"
	canonicalAlreadyAnsweredCopy    = "כבר ענית ✓ התשובה הראשונה היא שקובעת."
	canonicalFormatHintCopy         = "כדי לענות שלחו אות (א–ד) או ספרה (%s) — עוד יש זמן!"
	canonicalTooLongCopy            = "התשובה ארוכה מדי — עד %s תווים. שלחו שוב, בקצרה!"
	canonicalQuestionClosedCopy     = "השאלה נסגרה — מתכוננים לשאלה הבאה!"
	canonicalResultCorrectCopy      = "נכון! 🎉 +%s נקודות\nמקום %s בטבלה"
	canonicalResultCorrectBonusCopy = "נכון! 🎉 +%s נקודות\n⚡ בונוס מהירות +%s\nמקום %s בטבלה"
	canonicalResultWrongCopy        = "לא נכון הפעם. התשובה: %s\nמקום %s בטבלה — עוד הכול פתוח!"
	canonicalResultWrongLastCopy    = "לא נכון הפעם. התשובה: %s\nמקום %s בטבלה"

	canonicalFinalResultsCopy             = "המשחק נגמר! 🏆 הזוכה: %s עם %s נקודות.\nסיימת במקום %s עם %s נקודות — כל הכבוד!"
	canonicalFinalResultsTieCopy          = "המשחק נגמר! 🏆 הזוכים: %s עם %s נקודות.\nסיימת במקום %s עם %s נקודות — כל הכבוד!"
	canonicalFinalResultsSpectatorCopy    = "המשחק נגמר! 🏆 הזוכה: %s עם %s נקודות."
	canonicalFinalResultsSpectatorTieCopy = "המשחק נגמר! 🏆 הזוכים: %s עם %s נקודות."
	canonicalFinalResultsNoWinnerCopy     = "המשחק נגמר! הפעם לא נצברו נקודות — נתראה במשחק הבא!"
	canonicalWinnerFinalCopy              = "מזל טוב, %s! 🏆 ניצחת עם %s נקודות!"
	canonicalWinnerFinalTieCopy           = "מזל טוב, %s! 🏆 ניצחתם עם %s נקודות!"

	// canonicalNameConjunction is the tie forms' name separator, held
	// here so the joinNames tests below assert against the table's own
	// form rather than against joinNames' implementation constant.
	canonicalNameConjunction = " ו-"
)

func stripIsolates(s string) string {
	return strings.NewReplacer(lriMark, "", pdiMark, "").Replace(s)
}

// --- Welcome ---

// TestWelcomeMessageMatchesCanonicalCopy uses an intentionally Latin-script
// display name (a real case — mixed-language families, per Dev Notes) so a
// missing bidi isolate would actually be caught: a pure-Hebrew fixture would
// "look right" either way.
func TestWelcomeMessageMatchesCanonicalCopy(t *testing.T) {
	const name = "David Cohen"
	stripped := stripIsolates(welcomeMessage(name))
	want := fmt.Sprintf(canonicalWelcomeCopy, name, name)
	if stripped != want {
		t.Errorf("Welcome copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestWelcomeMessageIsolatesLTRTokens(t *testing.T) {
	const name = "David Cohen"
	got := welcomeMessage(name)
	isolated := lriMark + name + pdiMark
	if strings.Count(got, isolated) != 2 {
		t.Errorf("welcomeMessage(%q) does not isolate both occurrences of the display name: %q", name, got)
	}
}

// --- Pre-lobby ---

func TestPreLobbyMessageMatchesCanonicalCopy(t *testing.T) {
	if got := preLobbyMessage(); got != canonicalPreLobbyCopy {
		t.Errorf("Pre-lobby copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", got, canonicalPreLobbyCopy)
	}
}

// --- Invalid code ---

func TestInvalidCodeMessageMatchesCanonicalCopy(t *testing.T) {
	const code = "COHEN24"
	stripped := stripIsolates(invalidCodeMessage(code))
	want := fmt.Sprintf(canonicalInvalidCodeCopy, code, "JOIN")
	if stripped != want {
		t.Errorf("Invalid-code copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestInvalidCodeMessageIsolatesLTRTokens(t *testing.T) {
	got := invalidCodeMessage("COHEN24")
	for _, token := range []string{"COHEN24", "JOIN"} {
		isolated := lriMark + token + pdiMark
		if !strings.Contains(got, isolated) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", token, got)
		}
	}
}

// --- Name updated ---

// TestNameUpdatedMessageMatchesCanonicalCopy also uses a Latin-script name,
// same rationale as the Welcome test above.
func TestNameUpdatedMessageMatchesCanonicalCopy(t *testing.T) {
	const name = "David Cohen"
	stripped := stripIsolates(nameUpdatedMessage(name))
	want := fmt.Sprintf(canonicalNameUpdatedCopy, name)
	if stripped != want {
		t.Errorf("Name-updated copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestNameUpdatedMessageIsolatesLTRTokens(t *testing.T) {
	const name = "David Cohen"
	got := nameUpdatedMessage(name)
	isolated := lriMark + name + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", name, got)
	}
}

// --- Spectator notice ---

func TestSpectatorNoticeMessageMatchesCanonicalCopy(t *testing.T) {
	if got := spectatorNoticeMessage(); got != canonicalSpectatorNoticeCopy {
		t.Errorf("Spectator-notice copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", got, canonicalSpectatorNoticeCopy)
	}
}

// --- Question - MCQ ---

func TestQuestionMCQMessageMatchesCanonicalCopy(t *testing.T) {
	options := []string{"אחת", "שתיים", "שלוש", "ארבע"}
	stripped := stripIsolates(questionMCQMessage(2, 5, "כמה זה 1+1?", options, 20))
	want := fmt.Sprintf(canonicalQuestionMCQCopy, "2", "5", "כמה זה 1+1?", options[0], options[1], options[2], options[3], "1–4", "20")
	if stripped != want {
		t.Errorf("Question-MCQ copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestQuestionMCQMessageIsolatesDigitTokens(t *testing.T) {
	options := []string{"אחת", "שתיים", "שלוש", "ארבע"}
	got := questionMCQMessage(2, 5, "כמה זה 1+1?", options, 20)
	for _, token := range []string{"2", "5", "1–4", "20"} {
		isolated := lriMark + token + pdiMark
		if !strings.Contains(got, isolated) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", token, got)
		}
	}
}

// --- Question - Free-Text ---

func TestQuestionFreeTextMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(questionFreeTextMessage(3, 5, "מה בירת ישראל?", 30))
	want := fmt.Sprintf(canonicalQuestionFreeTextCopy, "3", "5", "מה בירת ישראל?", "30")
	if stripped != want {
		t.Errorf("Question-Free-Text copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestQuestionFreeTextMessageIsolatesDigitTokens(t *testing.T) {
	got := questionFreeTextMessage(3, 5, "מה בירת ישראל?", 30)
	for _, token := range []string{"3", "5", "30"} {
		isolated := lriMark + token + pdiMark
		if !strings.Contains(got, isolated) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", token, got)
		}
	}
}

// --- Acknowledgment ---

func TestAcknowledgmentMessageMatchesCanonicalCopy(t *testing.T) {
	if got := ackMessage(); got != canonicalAcknowledgmentCopy {
		t.Errorf("Acknowledgment copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", got, canonicalAcknowledgmentCopy)
	}
}

// --- Already answered ---

func TestAlreadyAnsweredMessageMatchesCanonicalCopy(t *testing.T) {
	if got := alreadyAnsweredMessage(); got != canonicalAlreadyAnsweredCopy {
		t.Errorf("Already-answered copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", got, canonicalAlreadyAnsweredCopy)
	}
}

// --- Format hint (MCQ) ---

func TestFormatHintMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(formatHintMessage())
	want := fmt.Sprintf(canonicalFormatHintCopy, "1–4")
	if stripped != want {
		t.Errorf("Format-hint copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestFormatHintMessageIsolatesDigitToken(t *testing.T) {
	got := formatHintMessage()
	isolated := lriMark + "1–4" + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", "1–4", got)
	}
}

// --- Too long (Free-Text) ---

func TestTooLongMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(tooLongMessage())
	want := fmt.Sprintf(canonicalTooLongCopy, "200")
	if stripped != want {
		t.Errorf("Too-long copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestTooLongMessageIsolatesDigitToken(t *testing.T) {
	got := tooLongMessage()
	isolated := lriMark + "200" + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", "200", got)
	}
}

// --- Question closed ---

func TestQuestionClosedMessageMatchesCanonicalCopy(t *testing.T) {
	if got := questionClosedMessage(); got != canonicalQuestionClosedCopy {
		t.Errorf("Question-closed copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", got, canonicalQuestionClosedCopy)
	}
}

// --- Result - correct ---

func TestResultCorrectMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(resultCorrectMessage(100, 1))
	want := fmt.Sprintf(canonicalResultCorrectCopy, "100", "1")
	if stripped != want {
		t.Errorf("Result-correct copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestResultCorrectMessageIsolatesDigitTokens(t *testing.T) {
	got := resultCorrectMessage(100, 1)
	for _, token := range []string{"100", "1"} {
		isolated := lriMark + token + pdiMark
		if !strings.Contains(got, isolated) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", token, got)
		}
	}
}

// --- Result - correct + bonus ---

func TestResultCorrectBonusMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(resultCorrectBonusMessage(100, 50, 1))
	want := fmt.Sprintf(canonicalResultCorrectBonusCopy, "100", "50", "1")
	if stripped != want {
		t.Errorf("Result-correct-bonus copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestResultCorrectBonusMessageIsolatesDigitTokens(t *testing.T) {
	got := resultCorrectBonusMessage(100, 50, 1)
	for _, token := range []string{"100", "50", "1"} {
		isolated := lriMark + token + pdiMark
		if !strings.Contains(got, isolated) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", token, got)
		}
	}
}

// --- Result - wrong ---

func TestResultWrongMessageMatchesCanonicalCopy(t *testing.T) {
	const correctAnswer = "Jerusalem"
	stripped := stripIsolates(resultWrongMessage(correctAnswer, 3))
	want := fmt.Sprintf(canonicalResultWrongCopy, correctAnswer, "3")
	if stripped != want {
		t.Errorf("Result-wrong copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestResultWrongMessageIsolatesRankToken(t *testing.T) {
	got := resultWrongMessage("Jerusalem", 3)
	isolated := lriMark + "3" + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", "3", got)
	}
}

// TestResultWrongMessageDoesNotIsolateCorrectAnswer uses a Latin-script
// correctAnswer fixture so a missing/wrongly-present isolate would actually
// be caught — mirrors TestWelcomeMessageIsolatesLTRTokens's reasoning for
// why a pure-Hebrew fixture wouldn't catch it either way. Per this file's
// header rule, correctAnswer is content (an Accepted Answer/MCQ option
// text), not an LTR token like a code or digit, so it must pass through
// raw, unlike every digit placeholder in this file.
func TestResultWrongMessageDoesNotIsolateCorrectAnswer(t *testing.T) {
	const correctAnswer = "Jerusalem"
	got := resultWrongMessage(correctAnswer, 3)
	if !strings.Contains(got, correctAnswer) {
		t.Fatalf("resultWrongMessage(%q, ...) = %q, want it to contain the raw correct answer", correctAnswer, got)
	}
	if strings.Contains(got, lriMark+correctAnswer+pdiMark) {
		t.Errorf("resultWrongMessage(%q, ...) isolates the correct answer, want it passed through raw: %q", correctAnswer, got)
	}
}

// --- Result - wrong, last question ---

func TestResultWrongLastMessageMatchesCanonicalCopy(t *testing.T) {
	const correctAnswer = "Jerusalem"
	stripped := stripIsolates(resultWrongLastMessage(correctAnswer, 3))
	want := fmt.Sprintf(canonicalResultWrongLastCopy, correctAnswer, "3")
	if stripped != want {
		t.Errorf("Result-wrong-last copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestResultWrongLastMessageIsolatesRankToken(t *testing.T) {
	got := resultWrongLastMessage("Jerusalem", 3)
	isolated := lriMark + "3" + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", "3", got)
	}
}

// TestResultWrongLastMessageDoesNotIsolateCorrectAnswer mirrors
// TestResultWrongMessageDoesNotIsolateCorrectAnswer above.
func TestResultWrongLastMessageDoesNotIsolateCorrectAnswer(t *testing.T) {
	const correctAnswer = "Jerusalem"
	got := resultWrongLastMessage(correctAnswer, 3)
	if !strings.Contains(got, correctAnswer) {
		t.Fatalf("resultWrongLastMessage(%q, ...) = %q, want it to contain the raw correct answer", correctAnswer, got)
	}
	if strings.Contains(got, lriMark+correctAnswer+pdiMark) {
		t.Errorf("resultWrongLastMessage(%q, ...) isolates the correct answer, want it passed through raw: %q", correctAnswer, got)
	}
}

// --- joinNames (story 3.9) ---
//
// Latin-script fixtures throughout, mirroring
// TestWelcomeMessageIsolatesLTRTokens's stated reasoning: a pure-Hebrew
// name "looks right" whether or not the isolate is there, so it cannot
// catch a missing one.

func TestJoinNamesSingleNameHasNoConjunction(t *testing.T) {
	got := stripIsolates(joinNames([]string{"David Cohen"}))
	if got != "David Cohen" {
		t.Errorf("joinNames(one name) = %q, want %q", got, "David Cohen")
	}
	if strings.Contains(got, canonicalNameConjunction) {
		t.Errorf("joinNames(one name) = %q, want no conjunction", got)
	}
}

func TestJoinNamesTwoNamesUseTheConjunction(t *testing.T) {
	got := stripIsolates(joinNames([]string{"David Cohen", "Rachel Levi"}))
	want := "David Cohen" + canonicalNameConjunction + "Rachel Levi"
	if got != want {
		t.Errorf("joinNames(two names) = %q, want %q", got, want)
	}
}

func TestJoinNamesThreeNamesCommaSeparateAllButTheLast(t *testing.T) {
	got := stripIsolates(joinNames([]string{"David Cohen", "Rachel Levi", "Noa Bar"}))
	want := "David Cohen, Rachel Levi" + canonicalNameConjunction + "Noa Bar"
	if got != want {
		t.Errorf("joinNames(three names) = %q, want %q", got, want)
	}
}

// TestJoinNamesIsolatesEveryName pins that each name is isolated
// individually and that the conjunction sits OUTSIDE the isolates — one
// isolate wrapped around the whole joined list would let the bidi
// algorithm reorder the names against the surrounding Hebrew.
func TestJoinNamesIsolatesEveryName(t *testing.T) {
	names := []string{"David Cohen", "Rachel Levi", "Noa Bar"}
	got := joinNames(names)
	for _, n := range names {
		if !strings.Contains(got, lriMark+n+pdiMark) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", n, got)
		}
	}
	if strings.Contains(got, lriMark+canonicalNameConjunction) || strings.Contains(got, canonicalNameConjunction+pdiMark) {
		t.Errorf("the conjunction sits inside an isolate, want it outside: %q", got)
	}
	if strings.Count(got, lriMark) != len(names) || strings.Count(got, pdiMark) != len(names) {
		t.Errorf("joinNames(%v) = %q, want exactly one isolate pair per name", names, got)
	}
}

// --- Final results ---

func TestFinalResultsMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(finalResultsMessage([]string{"David Cohen"}, 300, 2, 200))
	want := fmt.Sprintf(canonicalFinalResultsCopy, "David Cohen", "300", "2", "200")
	if stripped != want {
		t.Errorf("Final-results copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestFinalResultsMessageIsolatesDigitTokens(t *testing.T) {
	got := finalResultsMessage([]string{"David Cohen"}, 300, 2, 200)
	for _, token := range []string{"300", "2", "200"} {
		isolated := lriMark + token + pdiMark
		if !strings.Contains(got, isolated) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", token, got)
		}
	}
}

// --- Final results, tie ---

func TestFinalResultsTieMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(finalResultsTieMessage([]string{"David Cohen", "Rachel Levi"}, 300, 3, 150))
	want := fmt.Sprintf(canonicalFinalResultsTieCopy, "David Cohen"+canonicalNameConjunction+"Rachel Levi", "300", "3", "150")
	if stripped != want {
		t.Errorf("Final-results-tie copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestFinalResultsTieMessageIsolatesDigitTokens(t *testing.T) {
	got := finalResultsTieMessage([]string{"David Cohen", "Rachel Levi"}, 300, 3, 150)
	for _, token := range []string{"300", "3", "150"} {
		isolated := lriMark + token + pdiMark
		if !strings.Contains(got, isolated) {
			t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", token, got)
		}
	}
}

// --- Final results, spectator ---

func TestFinalResultsSpectatorMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(finalResultsSpectatorMessage([]string{"David Cohen"}, 300))
	want := fmt.Sprintf(canonicalFinalResultsSpectatorCopy, "David Cohen", "300")
	if stripped != want {
		t.Errorf("Final-results-spectator copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestFinalResultsSpectatorMessageIsolatesDigitTokens(t *testing.T) {
	got := finalResultsSpectatorMessage([]string{"David Cohen"}, 300)
	isolated := lriMark + "300" + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", "300", got)
	}
}

// --- Final results, spectator tie ---

func TestFinalResultsSpectatorTieMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(finalResultsSpectatorTieMessage([]string{"David Cohen", "Rachel Levi"}, 300))
	want := fmt.Sprintf(canonicalFinalResultsSpectatorTieCopy, "David Cohen"+canonicalNameConjunction+"Rachel Levi", "300")
	if stripped != want {
		t.Errorf("Final-results-spectator-tie copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestFinalResultsSpectatorTieMessageIsolatesDigitTokens(t *testing.T) {
	got := finalResultsSpectatorTieMessage([]string{"David Cohen", "Rachel Levi"}, 300)
	isolated := lriMark + "300" + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", "300", got)
	}
}

// --- Final results, no winner ---

func TestFinalResultsNoWinnerMessageMatchesCanonicalCopy(t *testing.T) {
	if got := finalResultsNoWinnerMessage(); got != canonicalFinalResultsNoWinnerCopy {
		t.Errorf("Final-results-no-winner copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", got, canonicalFinalResultsNoWinnerCopy)
	}
}

// TestFinalResultsNoWinnerMessageIsolatesDigitTokens is the
// IsolatesDigitTokens half of this row's pair: the row deliberately has
// NO placeholders, so there is nothing to isolate and nothing that could
// need it. Asserting the absence of isolates pins that — a future edit
// that reintroduces a score or rank token here would have to revisit
// this test, which is exactly the review this row's existence
// (suppressing a meaningless "place 1" for everyone) depends on.
func TestFinalResultsNoWinnerMessageIsolatesDigitTokens(t *testing.T) {
	got := finalResultsNoWinnerMessage()
	if strings.Contains(got, lriMark) || strings.Contains(got, pdiMark) {
		t.Errorf("no-winner copy carries bidi isolates but has no dynamic token to isolate: %q", got)
	}
}

// TestFinalResultsNoWinnerMessageCarriesNoTrophy pins EXPERIENCE.md's
// "🏆 discipline" — the trophy is reserved for the winner moment, and
// this row exists precisely because there is no winner.
func TestFinalResultsNoWinnerMessageCarriesNoTrophy(t *testing.T) {
	if got := finalResultsNoWinnerMessage(); strings.Contains(got, "🏆") {
		t.Errorf("no-winner copy carries the trophy emoji, want it reserved for the winner moment: %q", got)
	}
}

// --- Winner's final message ---

func TestWinnerFinalMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(winnerFinalMessage([]string{"David Cohen"}, 300))
	want := fmt.Sprintf(canonicalWinnerFinalCopy, "David Cohen", "300")
	if stripped != want {
		t.Errorf("Winner-final copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestWinnerFinalMessageIsolatesDigitTokens(t *testing.T) {
	got := winnerFinalMessage([]string{"David Cohen"}, 300)
	isolated := lriMark + "300" + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", "300", got)
	}
}

// --- Winner's final message, tie ---

func TestWinnerFinalTieMessageMatchesCanonicalCopy(t *testing.T) {
	stripped := stripIsolates(winnerFinalTieMessage([]string{"David Cohen", "Rachel Levi"}, 300))
	want := fmt.Sprintf(canonicalWinnerFinalTieCopy, "David Cohen"+canonicalNameConjunction+"Rachel Levi", "300")
	if stripped != want {
		t.Errorf("Winner-final-tie copy drifted from the EXPERIENCE.md templates table\n got: %q\nwant: %q", stripped, want)
	}
}

func TestWinnerFinalTieMessageIsolatesDigitTokens(t *testing.T) {
	got := winnerFinalTieMessage([]string{"David Cohen", "Rachel Levi"}, 300)
	isolated := lriMark + "300" + pdiMark
	if !strings.Contains(got, isolated) {
		t.Errorf("%q is not wrapped in LRI/PDI isolates: %q", "300", got)
	}
}
