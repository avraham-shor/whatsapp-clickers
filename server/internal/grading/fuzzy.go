package grading

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Levenshtein returns the edit distance (insertions, deletions,
// substitutions) between a and b, counted in runes (Hebrew text) —
// never bytes. Two-row dynamic-programming implementation, O(len(a)*
// len(b)) time, O(min(len(a),len(b))) space — pure Go, no external
// dependency (architecture: "pure Go, no external service"; this
// codebase has no Levenshtein library in go.mod and none is added).
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) > len(rb) {
		ra, rb = rb, ra
	}
	prev := make([]int, len(ra)+1)
	curr := make([]int, len(ra)+1)
	for i := range prev {
		prev[i] = i
	}
	for i := 1; i <= len(rb); i++ {
		curr[0] = i
		for j := 1; j <= len(ra); j++ {
			cost := 1
			if rb[i-1] == ra[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(ra)]
}

// levenshteinThreshold returns the maximum edit distance still
// considered a Fuzzy match for an accepted answer of the given
// (normalized, rune) length — short answers tolerate one typo, longer
// answers tolerate proportionally more. [ASSUMPTION — no specific
// threshold is set in the PRD/architecture; this is a starting point
// for the pilot. Revisit if Organizer-reported wrong grades (NFR-9,
// both directions — too lenient or too strict) show it needs tuning;
// NFR-9 also forbids tuning the *AI* stage toward leniency, but says
// nothing about Fuzzy, so this threshold is fair game to adjust.]
//
// Recalibrated in story 3.5's code review after the original tiers
// (1 / <=4, 2 / <=10, 3 / beyond) were shown to mis-grade the seed pack
// that ships in every database: a 2-rune numeric answer tolerating one
// edit accepted every neighbouring number, so "44" accepted "45" — the
// single most likely wrong answer to that question — and the 2-rune
// letter-numeral form of "150" accepted the Hebrew word for "yes". Below
// three runes a single edit is not a typo allowance, it is a different
// answer, so the floor is now exact-only. GradeFuzzy additionally
// requires distance < length, which keeps a short accepted answer from
// matching a response that shares nothing with it.
//
// That length-based floor only closed the <=2-rune case, and a production
// game later showed the principle was stated as a LENGTH rule when it is
// actually a TYPE rule: "150" is 3 runes, clears this length floor, and at
// the length<=6 / threshold=1 tier still fuzzy-accepted "100", "050" and
// "250" (substitution) and even "50" (deletion) — every one a different
// number, not a typo of "150". A word tolerates a one-letter slip because
// the remaining letters still anchor which word was meant; a digit string
// has no such anchor; every digit is fully load-bearing. So the type rule
// (numericTokensMatch, below) now enforces exact match on any
// all-digits token regardless of length, and this length-scaled threshold
// governs word tokens only.
func levenshteinThreshold(length int) int {
	switch {
	case length <= 2:
		return 0
	case length <= 6:
		return 1
	case length <= 12:
		return 2
	default:
		return 3
	}
}

// isNumericToken reports whether tok is non-empty and consists
// entirely of Unicode decimal digits. A digit string has no letters
// left to anchor "same word, one typo" the way a misspelled word does
// — every digit changes the value — so a numeric token gets no edit
// tolerance at all (see the type-rule note on levenshteinThreshold).
func isNumericToken(tok string) bool {
	if tok == "" {
		return false
	}
	for _, r := range tok {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// numericTokens splits s (already Normalize()d) into whitespace tokens
// and merges every run of consecutive all-digits tokens into one,
// concatenated with no separator. Normalize turns punctuation —
// including a thousands-separator comma or a decimal point — into a
// plain space along with everything else, so "1,000" and "1000" would
// otherwise land on opposite sides of the exact/fuzzy line for no
// reason a participant would recognize as meaningful: the same number,
// typed with or without a separator, must compare as the same numeric
// token. Deliberate, accepted consequence: a decimal ("3.5") merges
// indistinguishably from its digit concatenation ("35"), same as a
// thousands-grouped integer would. This game's shipped Free-Text
// answers are counts/years/currency amounts, not decimals, so that
// collision is accepted rather than solved — revisit if a decimal-
// valued Accepted Answer appears.
func numericTokens(s string) []string {
	fields := strings.Fields(s)
	merged := make([]string, 0, len(fields))
	for i := 0; i < len(fields); {
		if !isNumericToken(fields[i]) {
			merged = append(merged, fields[i])
			i++
			continue
		}
		var run strings.Builder
		for i < len(fields) && isNumericToken(fields[i]) {
			run.WriteString(fields[i])
			i++
		}
		merged = append(merged, run.String())
	}
	return merged
}

// numericTokensMatch reports whether every all-digits token in
// normAccepted (after numericTokens' comma/period-group merging) has
// an exact, same-position match in responseTokens. Non-numeric
// accepted tokens are not inspected here at all — they keep the
// length-scaled Levenshtein tolerance GradeFuzzy applies afterward on
// the whole normalized strings. An accepted answer with no numeric
// token trivially matches (the loop body never runs), so this changes
// nothing for word-only answers.
func numericTokensMatch(normAccepted string, responseTokens []string) bool {
	for i, tok := range numericTokens(normAccepted) {
		if !isNumericToken(tok) {
			continue
		}
		if i >= len(responseTokens) || responseTokens[i] != tok {
			return false
		}
	}
	return true
}

// GradeFuzzy reports whether response fuzzily matches any of
// acceptedAnswers: both sides are normalized (Normalize) and compared
// by Levenshtein distance against a length-scaled threshold
// (levenshteinThreshold, keyed on the normalized accepted answer's
// rune length) — except any all-digits token (after numericTokens'
// comma/period-group merging), which numericTokensMatch requires to
// match the response's same-position token exactly, with no edit
// tolerance regardless of length (production defect: "150" was
// fuzzy-accepting "100", "050" and "250", and even "50" by deletion —
// see levenshteinThreshold's type-rule note). Word tokens in a mixed
// answer ("150 שקלים") keep their usual tolerance, so "150 שקל" still
// matches while "100 שקלים" does not. Called only after GradeExact
// misses (game.RecordAnswer) — an Exact hit never reaches here, but
// GradeFuzzy does not itself depend on that ordering (it renormalizes
// and would report a match for an Exact hit too).
//
// Known residual (adversarial review of this change, not yet closed):
// a single Hebrew letter fused onto a number with no space — a prefix
// like ב/כ/ל/מ/ו/ש directly on the digits ("כ150", "ב1948"), which
// Hebrew writes routinely — is not itself all-digits, so isNumericToken
// excludes it and it falls back to ordinary word-length fuzzy tolerance,
// reopening this same class of defect under a different token shape
// ("כ100" fuzzy-accepts "כ150"). Not a regression: this token shape had
// no numeric protection before this change either. Deferred rather than
// widened here — see deferred-work.md.
//
// Two further guards, both from story 3.5's code review, keep a
// degenerate Accepted Answer from turning a question into free points:
//
//   - An accepted answer that normalizes to empty is skipped. This is
//     reachable through the normal authoring path — validateQuestion
//     accepts any 1-rune entry, so "?" passes, and ImportPackageQuestions
//     copies bank answers with no Go-side validation at all while the DB
//     CHECKs only ever test cardinality, never element content.
//   - A match must satisfy distance < length as well as the threshold.
//     Without it, an accepted answer at or below the threshold's own
//     length matches a response sharing none of its characters — and an
//     empty response, which is reachable because wa.classify filters
//     blank bodies with strings.TrimSpace, which does not remove the
//     category-Cf marks that game.stripFormatMarks later does.
func GradeFuzzy(response string, acceptedAnswers []string) bool {
	normResponse := Normalize(response)
	if normResponse == "" {
		return false
	}
	responseTokens := numericTokens(normResponse)
	for _, accepted := range acceptedAnswers {
		normAccepted := Normalize(accepted)
		length := utf8.RuneCountInString(normAccepted)
		if length == 0 {
			continue
		}
		if !numericTokensMatch(normAccepted, responseTokens) {
			continue
		}
		distance := Levenshtein(normResponse, normAccepted)
		if distance < length && distance <= levenshteinThreshold(length) {
			return true
		}
	}
	return false
}
