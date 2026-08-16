package grading

import "testing"

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"identical strings", "abc", "abc", 0},
		{"both empty", "", "", 0},
		{"empty vs non-empty", "", "abc", 3},
		{"non-empty vs empty", "abc", "", 3},
		{"single substitution", "abc", "abd", 1},
		{"single insertion", "abc", "abcd", 1},
		{"single deletion", "abc", "ab", 1},
		{"completely different same-length strings", "abc", "xyz", 3},

		// Distance is counted in RUNES, never bytes — every Hebrew letter
		// is two UTF-8 bytes, so a byte-based implementation would report
		// double these numbers and silently halve the effective threshold
		// for every real answer (code review, story 3.5).
		{"hebrew single substitution", "שלום", "שלוב", 1},
		{"hebrew single deletion", "ירושלים", "ירושלם", 1},
		{"hebrew identical strings", "ירושלים", "ירושלים", 0},
		{"hebrew empty vs non-empty counts runes not bytes", "", "שלום", 4},
		{"hebrew completely different same-length strings", "שלום", "אבגד", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Levenshtein(tc.a, tc.b); got != tc.want {
				t.Errorf("Levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestGradeFuzzy(t *testing.T) {
	cases := []struct {
		name            string
		response        string
		acceptedAnswers []string
		want            bool
	}{
		{"trailing space matches", "צרורה ", []string{"צרורה"}, true},
		{"single-letter typo matches within threshold", "ירוסלים", []string{"ירושלים"}, true},
		{"unrelated word does not match", "תל אביב", []string{"ירושלים"}, false},
		{"normalization engages: punctuation stripped on response side", "שלום!", []string{"שלום"}, true},
		{"matches the second of two accepted answers when the first doesn't", "תל אביב", []string{"ירושלים", "תל אביב"}, true},
		{"threshold boundary: one more edit than allowed does not match", "שגכמ", []string{"שלום"}, false},

		// --- Threshold tiers (code review, story 3.5) ---
		// Each tier is pinned in BOTH directions. Before this, every case
		// above still passed with levenshteinThreshold hardwired to 1, so
		// two of the three tiers were asserted nowhere.
		{"tier 1 (<=2 runes): exact only, no edit tolerated", "45", []string{"44"}, false},
		{"tier 1 (<=2 runes): a zero-distance match still matches", "44", []string{"44"}, true},
		{"tier 2 (3-6 runes): one edit matches", "אברחם", []string{"אברהם"}, true},
		{"tier 2 (3-6 runes): two edits do not", "אבטחם", []string{"אברהם"}, false},
		{"tier 3 (7-12 runes): two edits match", "ורוסלים", []string{"ירושלים"}, true},
		{"tier 3 (7-12 runes): three edits do not", "ורוסליק", []string{"ירושלים"}, false},
		{"tier 4 (13+ runes): three edits match", "ארבעים וארזחק", []string{"ארבעים וארבעה"}, true},
		{"tier 4 (13+ runes): four edits do not", "ארבעים ואקזחק", []string{"ארבעים וארבעה"}, false},

		// --- Seed-pack regressions (code review, story 3.5) ---
		// Every case here graded a WRONG answer as correct under the
		// original tiers, against questions that ship in every database
		// (00005_question_packages.sql).
		{"the off-by-one on the Hanukkah candles question is not correct", "45", []string{"44", "ארבעים וארבעה"}, false},
		{"the word for yes is not a correct chapter count", "כן", []string{"150", "מאה חמישים", "ק\"נ"}, false},
		{"a different son is not a correct answer", "מנשה ואשר", []string{"מנשה ואפרים", "אפרים ומנשה"}, false},

		// --- Degenerate Accepted Answers (code review, story 3.5) ---
		// Reachable through the normal authoring path: validateQuestion
		// accepts any 1-rune entry, and ImportPackageQuestions applies no
		// Go-side validation at all.
		{"an accepted answer that normalizes to empty never matches", "א", []string{"?"}, false},
		{"every accepted answer normalizing to empty still never matches", "שלום", []string{"?", "-"}, false},
		{"a one-rune accepted answer rejects a different one-rune response", "8", []string{"7"}, false},
		{"a one-rune accepted answer still matches itself", "7", []string{"7"}, true},
		{"an empty response never fuzzy-matches", "", []string{"7"}, false},
		{"a punctuation-only response never fuzzy-matches", "???", []string{"7"}, false},

		// --- Normalization reaching both sides (code review, story 3.5) ---
		{"an emoji in the response does not cost the match", "ירושלים ❤️", []string{"ירושלים"}, true},
		{"invisible marks in the ACCEPTED answer do not break matching", "44", []string{"‏44‎"}, true},
		{"a hyphenated accepted answer matches the spaced spelling", "תל אביב", []string{"תל-אביב"}, true},
		{"a latin answer matches regardless of capitalization", "nasa", []string{"NASA"}, true},

		// --- Known residual of the chosen calibration ---
		// Flagged in review, accepted deliberately: at 3-6 runes one edit
		// is still tolerated for WORD tokens, so a short-word answer in
		// that band still accepts its nearest neighbour. Tightening
		// further would cost genuine typo tolerance on ordinary short
		// Hebrew words, so this is pinned as CURRENT behavior, not as
		// desired behavior — revisit with NFR-9 evidence from the pilot.
		// (The numeric sibling of this residual — "100" fuzzy-matching
		// "150" — is NOT a residual; it was a production defect, fixed
		// below by the numeric-token rule.)
		{"RESIDUAL: a four-rune place name still accepts a shorter word", "סין", []string{"סיני"}, true},

		// --- Numeric tokens require an exact match (production defect) ---
		// Confirmed in a real game: accepted "150" fuzzy-graded "100" and
		// "050" as correct, because a 3-rune numeric answer still fell in
		// the length<=6/threshold=1 tier. A digit string has no letters
		// left to anchor "same word, one typo" — every digit is fully
		// load-bearing — so any all-digits token now requires an exact,
		// same-position match; word tokens are unaffected (numericTokensMatch).
		{"a numeric answer rejects a substituted neighbour (the reported defect)", "100", []string{"150"}, false},
		{"a numeric answer rejects another substituted neighbour (the reported defect)", "050", []string{"150"}, false},
		{"a numeric answer rejects a deletion that changes the value by a factor of 10", "50", []string{"150"}, false},
		{"a numeric answer still matches itself exactly", "150", []string{"150"}, true},
		{"a longer numeric answer still rejects a substituted neighbour", "1400", []string{"1500"}, false},
		{"a mixed numeric+word answer rejects a wrong number even with the word intact", "100 שקלים", []string{"150 שקלים"}, false},
		{"a mixed numeric+word answer still tolerates a word-level typo when the number is exact", "150 שקל", []string{"150 שקלים"}, true},

		// --- Comma/period-grouped numbers merge into one numeric token
		// (adversarial review of the fix above) ---
		// Normalize turns "," and "." into spaces same as any other
		// punctuation, so without merging, a thousands-separated
		// Accepted Answer like "1,000" would token-split into "1" and
		// "000" and reject a plainly-typed "1000" outright — a
		// regression the pre-fix whole-string Levenshtein comparison
		// did not have (it fuzzy-matched "1000" against "1,000" within
		// threshold). numericTokens re-merges adjacent all-digits
		// tokens so both spellings compare as one exact numeric token.
		{"a comma-grouped accepted answer still matches the plain digit spelling", "1000", []string{"1,000"}, true},
		{"a comma-grouped response still matches the plain digit accepted answer", "1,000", []string{"1000"}, true},
		{"a comma-grouped answer still rejects a genuinely different number", "2000", []string{"1,000"}, false},
		{"a comma-grouped answer in a mixed accepted answer still requires the exact value", "1500 שקלים", []string{"1,000 שקלים"}, false},

		// --- Numeric-token alignment (adversarial review of the fix above) ---
		{"a numeric token not in the first position still requires an exact match", "שקל 100", []string{"שקל 150"}, false},
		{"a numeric token not in the first position still matches when exact", "שקל 150", []string{"שקל 150"}, true},
		// numericTokensMatch only bounds-checks a response position it
		// actually needs (a numeric accepted token); a missing trailing
		// WORD token is caught by the ordinary whole-string Levenshtein
		// distance afterward, not by the token gate itself — pinned here
		// so that guarantee stays true if either check is ever touched.
		{"a missing trailing word still fails on the overall length gap, even with the number exact", "150", []string{"150 שקלים"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GradeFuzzy(tc.response, tc.acceptedAnswers); got != tc.want {
				t.Errorf("GradeFuzzy(%q, %v) = %v, want %v", tc.response, tc.acceptedAnswers, got, tc.want)
			}
		})
	}
}
