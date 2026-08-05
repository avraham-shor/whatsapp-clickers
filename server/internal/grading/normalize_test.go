package grading

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"already-normalized string unchanged", "אבג דבר", "אבג דבר"},
		{"surrounding whitespace trimmed", "  דבר  ", "דבר"},
		{"internal whitespace collapsed", "דבר  ,  אחר", "דבר אחר"},
		{"final kaf mapped to medial", "ך", "כ"},
		{"final mem mapped to medial", "ם", "מ"},
		{"final nun mapped to medial", "ן", "נ"},
		{"final pe mapped to medial", "ף", "פ"},
		{"final tsadi mapped to medial", "ץ", "צ"},
		{"medial letters are left alone", "כמנפצ", "כמנפצ"},
		{"nikud stripped", "שָׁלוֹם", "שלומ"},
		{"exclamation mark stripped", "שלום!", "שלומ"},
		{"empty string", "", ""},
		{"punctuation-only string produces empty result, not a stray space", "!,.", ""},

		// Punctuation becomes a separator rather than vanishing, so a
		// hyphenated or comma-joined Accepted Answer lines up with the
		// spaced form a participant types instead of spending the
		// Levenshtein budget on the separator (code review, story 3.5).
		{"comma becomes a separator", "שלום,עולם", "שלומ עולמ"},
		{"hyphen becomes a separator, matching the spaced spelling", "תל-אביב", "תל אביב"},
		{"maqaf becomes a separator", "תל־אביב", "תל אביב"},

		// Intra-word marks are the exception — an acronym is one token.
		{"geresh stripped without splitting the word", "צה׳ל", "צהל"},
		{"gershayim stripped without splitting the word", "צה״ל", "צהל"},
		{"ascii quote normalizes identically to gershayim", "ק\"נ", "קנ"},
		{"typographic apostrophe stripped without splitting", "צה’ל", "צהל"},

		// Whitespace shapes a WhatsApp reply actually arrives in.
		{"whitespace-only string produces empty result", "   ", ""},
		{"tab and newline collapse like spaces", "דבר\tאחר\nנוסף", "דבר אחר נוספ"},
		{"non-breaking space collapses like a space", "דבר אחר", "דבר אחר"},

		// Invisible characters: an Accepted Answer pasted from a Hebrew
		// document carries these, and only Normalize ever sees that side.
		// Written as escapes: these are invisible in an editor, and a
		// literal U+FEFF is rejected outright by the Go compiler.
		{"rtl and ltr marks stripped", "‏ירושלים‎", "ירושלימ"},
		{"zero-width joiner stripped", "דבר‍אחר", "דבראחר"},

		// Symbols are category S, not P — unicode.IsPunct alone misses them.
		{"emoji becomes a separator and its variation selector is dropped", "ירושלים ❤️", "ירושלימ"},
		{"math and currency symbols become separators", "5+5=10", "5 5 10"},

		// Latin answers must not grade by capitalization.
		{"latin text folds to lower case", "NASA", "nasa"},
		{"mixed script folds only the latin half", "NASA ירושלים", "nasa ירושלימ"},
		{"digits are left alone", "150", "150"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
