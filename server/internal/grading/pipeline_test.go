package grading

import "testing"

func TestGradeMCQ(t *testing.T) {
	cases := []struct {
		name          string
		response      string
		correctOption int
		want          bool
	}{
		{"option 1 match", "1", 1, true},
		{"option 1 mismatch", "2", 1, false},
		{"option 2 match", "2", 2, true},
		{"option 2 mismatch", "1", 2, false},
		{"option 3 match", "3", 3, true},
		{"option 3 mismatch", "4", 3, false},
		{"option 4 match", "4", 4, true},
		{"option 4 mismatch", "1", 4, false},
		{"out-of-range response never matches", "5", 4, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GradeMCQ(tc.response, tc.correctOption); got != tc.want {
				t.Errorf("GradeMCQ(%q, %d) = %v, want %v", tc.response, tc.correctOption, got, tc.want)
			}
		})
	}
}

func TestGradeExact(t *testing.T) {
	cases := []struct {
		name            string
		response        string
		acceptedAnswers []string
		want            bool
	}{
		{"single accepted answer match", "ירושלים", []string{"ירושלים"}, true},
		{"multi-value matches first entry", "א", []string{"א", "ב", "ג"}, true},
		{"multi-value matches middle entry", "ב", []string{"א", "ב", "ג"}, true},
		{"multi-value matches last entry", "ג", []string{"א", "ב", "ג"}, true},
		{"no match", "דלת", []string{"א", "ב", "ג"}, false},
		{"trailing whitespace the caller didn't trim does not match", "ירושלים ", []string{"ירושלים"}, false},
		{"empty response against non-empty accepted answer does not match", "", []string{"ירושלים"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GradeExact(tc.response, tc.acceptedAnswers); got != tc.want {
				t.Errorf("GradeExact(%q, %v) = %v, want %v", tc.response, tc.acceptedAnswers, got, tc.want)
			}
		})
	}
}
