package grading

import "unicode/utf8"

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

// GradeFuzzy reports whether response fuzzily matches any of
// acceptedAnswers: both sides are normalized (Normalize) and compared
// by Levenshtein distance against a length-scaled threshold
// (levenshteinThreshold, keyed on the normalized accepted answer's
// rune length). Called only after GradeExact misses (game.RecordAnswer)
// — an Exact hit never reaches here, but GradeFuzzy does not itself
// depend on that ordering (it renormalizes and would report a match
// for an Exact hit too).
//
// Two guards, both from story 3.5's code review, keep a degenerate
// Accepted Answer from turning a question into free points:
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
	for _, accepted := range acceptedAnswers {
		normAccepted := Normalize(accepted)
		length := utf8.RuneCountInString(normAccepted)
		if length == 0 {
			continue
		}
		distance := Levenshtein(normResponse, normAccepted)
		if distance < length && distance <= levenshteinThreshold(length) {
			return true
		}
	}
	return false
}
