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
	canonicalWelcomeCopy     = "היי %s, נרשמת! 🎉 השאירו את הצ'אט הזה פתוח — השאלות יגיעו לכאן. (לא %s? שלחו לדוגמה — שם: רחל לוי)"
	canonicalPreLobbyCopy    = "הקוד נכון! ההרשמה עוד לא נפתחה — שלחו שוב את ההודעה כשהמארגן מכריז שמתחילים."
	canonicalInvalidCodeCopy = "הקוד %s לא נמצא. בדקו את הקוד עם המארגן ושלחו שוב: %s ואחריו הקוד."
	canonicalNameUpdatedCopy = "עודכן ✓ מעכשיו: %s"
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
