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
	canonicalWelcomeCopy          = "היי %s, נרשמת! 🎉 השאירו את הצ'אט הזה פתוח — השאלות יגיעו לכאן. (לא %s? שלחו לדוגמה — שם: רחל לוי)"
	canonicalPreLobbyCopy         = "הקוד נכון! ההרשמה עוד לא נפתחה — שלחו שוב את ההודעה כשהמארגן מכריז שמתחילים."
	canonicalInvalidCodeCopy      = "הקוד %s לא נמצא. בדקו את הקוד עם המארגן ושלחו שוב: %s ואחריו הקוד."
	canonicalNameUpdatedCopy      = "עודכן ✓ מעכשיו: %s"
	canonicalSpectatorNoticeCopy  = "המשחק כבר התחיל! נרשמת כצופה — התוצאות יגיעו לכאן בסוף המשחק 🏆"
	canonicalQuestionMCQCopy      = "שאלה %s מתוך %s:\n%s\nא. %s\nב. %s\nג. %s\nד. %s\nהשיבו באות (א–ד) או בספרה (%s) — יש לכם %s שניות!"
	canonicalQuestionFreeTextCopy = "שאלה %s מתוך %s:\n%s\nכתבו את התשובה בהודעה — יש לכם %s שניות!"
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
