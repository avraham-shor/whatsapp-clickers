package grading

import (
	"strings"
	"unicode"
)

// finalHebrewLetters lists the code points of the five Hebrew final letter
// forms — FINAL KAF, FINAL MEM, FINAL NUN, FINAL PE, FINAL TSADI
// (architecture: "final-letter forms ... nikud stripping, punctuation").
// Named by letter name rather than spelled with the actual glyph (even in
// a comment) so this file stays entirely free of raw Hebrew characters —
// grading has no file exempted from CI's copy-centralization check the way
// wa/messages_he.go is, so no non-test file here can contain one.
//
// Unicode places each final form exactly one code point before its medial
// counterpart (verified in normalize_test.go), so mapping final -> medial
// is a +1 offset, not a lookup table: a response typed with the wrong form
// (WhatsApp autocorrect, or a participant unfamiliar with when finals
// apply) must not cost a Fuzzy match.
const (
	finalKaf   = 0x05DA
	finalMem   = 0x05DD
	finalNun   = 0x05DF
	finalPe    = 0x05E3
	finalTsadi = 0x05E5
)

// toMedialForm returns r's medial form and true if r is one of the five
// Hebrew final letters, or r and false otherwise.
func toMedialForm(r rune) (rune, bool) {
	switch r {
	case finalKaf, finalMem, finalNun, finalPe, finalTsadi:
		return r + 1, true
	}
	return r, false
}

// Intra-word marks: HEBREW PUNCTUATION GERESH (U+05F3) and GERSHAYIM
// (U+05F4), plus the ASCII and typographic apostrophes/quotes phones
// substitute for them. Named by code point for the same
// copy-centralization reason as the final letters above.
const (
	geresh    = 0x05F3
	gershayim = 0x05F4
)

// isIntraWordMark reports whether r is a mark that sits INSIDE a word
// rather than between words — an acronym's gershayim, a transliteration's
// geresh, an apostrophe. Unlike every other punctuation mark these are
// deleted outright rather than replaced by a space (see Normalize): an
// acronym is one token, and splitting it would both change its length and
// insert a word boundary no participant typed. The seed pack's "150 in
// Hebrew letter-numerals" accepted answer is exactly this shape, stored
// with an ASCII quote while a Hebrew keyboard emits U+05F4 — both spell
// the same token here.
func isIntraWordMark(r rune) bool {
	switch r {
	case '\'', '"', '‘', '’', geresh, gershayim:
		return true
	}
	return false
}

// Normalize prepares s for Fuzzy-stage comparison (FR-16). In order:
// invisible format characters (category Cf — RTL/LTR marks, zero-width
// joiners, BOM) and combining marks (category M — niqqud, cantillation,
// emoji variation selectors) are deleted; intra-word marks are deleted
// (isIntraWordMark); every other punctuation or symbol rune — including
// emoji, which are category S and NOT category P — becomes a space;
// Hebrew final letters map to their medial form; letters fold to lower
// case so Latin answers do not grade by capitalization; and whitespace
// collapses to single spaces with the ends trimmed.
//
// Punctuation becomes a space rather than vanishing so that a hyphenated
// or comma-joined Accepted Answer still lines up with the spaced form a
// participant actually types, instead of spending the Levenshtein budget
// on a separator neither side considers part of the word (code review,
// story 3.5).
//
// Cf is stripped here as well as in game.stripFormatMarks because that
// function only ever sees the participant's response — an Accepted Answer
// pasted from a Hebrew document carries the same invisible marks and
// reaches comparison only through this path, where it would otherwise
// fail Exact AND Fuzzy for every correctly-typed response (code review,
// story 3.5).
//
// Self-contained — does not assume the caller already trimmed, so it can
// be unit-tested and called directly with raw, untrimmed input (epic
// AC-2's trailing-whitespace case exercises this independently of
// game.RecordAnswer's own trim). Applied independently to both the
// participant's response and each Accepted Answer (GradeFuzzy).
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Cf, r) || unicode.Is(unicode.M, r) || isIntraWordMark(r):
			continue
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			b.WriteRune(' ')
			continue
		}
		if mapped, ok := toMedialForm(r); ok {
			r = mapped
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
