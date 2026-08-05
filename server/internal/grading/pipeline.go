// Package grading implements FR-15/FR-16's answer-correctness pipeline —
// MCQ mechanical grading and Free-Text's Exact → Fuzzy → AI stages
// (Fuzzy lands in Story 3.5, AI in Story 3.6; this story is Exact only).
// Pure functions, no store/DB dependency — game calls in, store persists
// the verdict game hands back (architecture: game imports grading).
package grading

import "strconv"

// Stage identifies which grading stage produced an answer's verdict —
// persisted verbatim as answers.stage (FR-16: "recording which stage
// matched"; NFR-9 audit trail). Plain string alias, not a distinct
// type — same reasoning as game.State: the DB column is TEXT + CHECK,
// and a distinct type would force casts at the sqlc.arg boundary for
// no safety gain.
type Stage = string

const (
	StageMCQ   Stage = "mcq"
	StageExact Stage = "exact"
	// StageFuzzy and StageAI join this list in Stories 3.5/3.6.
)

// GradeMCQ reports whether response (a normalized "1".."4" digit
// string — see game.parseMCQOption) matches correctOption (1-4,
// questions.correct_option).
func GradeMCQ(response string, correctOption int) bool {
	return response == strconv.Itoa(correctOption)
}

// GradeExact reports whether response (already trimmed by the caller)
// exactly matches any of acceptedAnswers by literal string equality —
// no normalization (trim beyond the caller's, final-letter forms,
// nikud, punctuation): that is the Fuzzy stage, Story 3.5. An empty
// acceptedAnswers never reaches this function for a real question
// (questions_type_shape requires cardinality >= 1 for free_text) — no
// special-case guard needed.
func GradeExact(response string, acceptedAnswers []string) bool {
	for _, accepted := range acceptedAnswers {
		if response == accepted {
			return true
		}
	}
	return false
}
