package game

import (
	"context"
	"errors"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// resultsGame is a Game with PointsPerCorrect=100, matching this file's own
// fixture questions/answers below.
func resultsGame() gen.Game {
	return gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateRevealed, PointsPerCorrect: 100}
}

// mcqResultsQuestion is a single mcq question at position 1 with a real
// CorrectOption — oneQuestion()/twoQuestions() (engine_test.go) default
// CorrectOption to the Go zero value 0, which ResultsForRevealedQuestion
// now rejects as out-of-range, so this file defines its own fixtures.
func mcqResultsQuestion() gen.Question {
	return gen.Question{ID: "q1", GameID: testGameID, Position: 1, Type: "mcq", Text: "כמה זה 1+1?", Options: []string{"אחת", "שתיים", "שלוש", "ארבע"}, CorrectOption: 2}
}

// freeTextResultsQuestion is a single free_text question at position.
// AcceptedAnswers[0] ("ירושלים") is the primary form ResultsForRevealedQuestion
// must select over the secondary ("יְרוּשָׁלַיִם").
func freeTextResultsQuestion(position int32) gen.Question {
	return gen.Question{ID: "q2", GameID: testGameID, Position: position, Type: "free_text", Text: "מה בירת ישראל?", AcceptedAnswers: []string{"ירושלים", "יְרוּשָׁלַיִם"}}
}

func resultRow(participantID, phone string, isCorrect bool, points int32) store.AnswerResultRow {
	return store.AnswerResultRow{ParticipantID: participantID, Phone: phone, IsCorrect: isCorrect, Points: points}
}

func personalResultByPhone(t *testing.T, results []PersonalResult, phone string) PersonalResult {
	t.Helper()
	for _, r := range results {
		if r.Phone == phone {
			return r
		}
	}
	t.Fatalf("no PersonalResult for phone %q in %+v", phone, results)
	return PersonalResult{}
}

func TestResultsForRevealedQuestionMCQSplitsBaseAndBonusPointsByRank(t *testing.T) {
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{mcqResultsQuestion()},
		listAnswerResultsForQuestionResult: []store.AnswerResultRow{
			resultRow("p1", "+972500000001", true, 150), // 100 base + 50 bonus
			resultRow("p2", "+972500000002", true, 130), // 100 base + 30 bonus
			resultRow("p3", "+972500000003", true, 110), // 100 base + 10 bonus
			resultRow("p4", "+972500000004", false, 0),
		},
		getLeaderboardResult: []store.ParticipantScore{
			{ParticipantID: "p1", DisplayName: "A", Score: 300},
			{ParticipantID: "p2", DisplayName: "B", Score: 200},
			{ParticipantID: "p3", DisplayName: "C", Score: 100},
			{ParticipantID: "p4", DisplayName: "D", Score: 0},
		},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err != nil {
		t.Fatalf("ResultsForRevealedQuestion() error = %v, want nil", err)
	}
	if len(got.Results) != 4 {
		t.Fatalf("Results has %d entries, want 4", len(got.Results))
	}

	cases := []struct {
		phone       string
		wantBase    int32
		wantBonus   int32
		wantRank    int
		wantCorrect bool
	}{
		{"+972500000001", 100, 50, 1, true},
		{"+972500000002", 100, 30, 2, true},
		{"+972500000003", 100, 10, 3, true},
		{"+972500000004", 0, 0, 4, false},
	}
	for _, tc := range cases {
		r := personalResultByPhone(t, got.Results, tc.phone)
		if r.IsCorrect != tc.wantCorrect || r.BasePoints != tc.wantBase || r.BonusPoints != tc.wantBonus || r.Rank != tc.wantRank {
			t.Errorf("result for %s = %+v, want IsCorrect=%v BasePoints=%d BonusPoints=%d Rank=%d",
				tc.phone, r, tc.wantCorrect, tc.wantBase, tc.wantBonus, tc.wantRank)
		}
	}
}

func TestResultsForRevealedQuestionCorrectWithoutBonusHasZeroBonusPoints(t *testing.T) {
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{mcqResultsQuestion()},
		listAnswerResultsForQuestionResult: []store.AnswerResultRow{
			resultRow("p1", "+972500000001", true, 100), // exactly PointsPerCorrect, no bonus room left
		},
		getLeaderboardResult: []store.ParticipantScore{
			{ParticipantID: "p1", DisplayName: "A", Score: 100},
		},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err != nil {
		t.Fatalf("ResultsForRevealedQuestion() error = %v, want nil", err)
	}
	r := personalResultByPhone(t, got.Results, "+972500000001")
	if r.BasePoints != 100 || r.BonusPoints != 0 {
		t.Errorf("result = %+v, want BasePoints=100 BonusPoints=0", r)
	}
}

func TestResultsForRevealedQuestionMCQWrongUsesCorrectOptionText(t *testing.T) {
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{mcqResultsQuestion()},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err != nil {
		t.Fatalf("ResultsForRevealedQuestion() error = %v, want nil", err)
	}
	want := mcqResultsQuestion().Options[mcqResultsQuestion().CorrectOption-1]
	if got.CorrectAnswer != want {
		t.Errorf("CorrectAnswer = %q, want %q (Options[CorrectOption-1])", got.CorrectAnswer, want)
	}
}

func TestResultsForRevealedQuestionFreeTextWrongUsesPrimaryAcceptedAnswer(t *testing.T) {
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{freeTextResultsQuestion(1)},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err != nil {
		t.Fatalf("ResultsForRevealedQuestion() error = %v, want nil", err)
	}
	if got.CorrectAnswer != "ירושלים" {
		t.Errorf("CorrectAnswer = %q, want the primary (first) accepted answer %q", got.CorrectAnswer, "ירושלים")
	}
}

func TestResultsForRevealedQuestionIsLastQuestionTrueWhenPositionEqualsQuestionCount(t *testing.T) {
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{mcqResultsQuestion()},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err != nil {
		t.Fatalf("ResultsForRevealedQuestion() error = %v, want nil", err)
	}
	if !got.IsLastQuestion {
		t.Error("IsLastQuestion = false, want true (position equals the game's question count)")
	}
}

func TestResultsForRevealedQuestionIsLastQuestionFalseOtherwise(t *testing.T) {
	second := mcqResultsQuestion()
	second.ID = "q2"
	second.Position = 2
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{mcqResultsQuestion(), second},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err != nil {
		t.Fatalf("ResultsForRevealedQuestion() error = %v, want nil", err)
	}
	if got.IsLastQuestion {
		t.Error("IsLastQuestion = true, want false (a question remains after position 1)")
	}
}

func TestResultsForRevealedQuestionNoAnswersReturnsEmptyResultsSlice(t *testing.T) {
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{mcqResultsQuestion()},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err != nil {
		t.Fatalf("ResultsForRevealedQuestion() error = %v, want nil", err)
	}
	if got.Results == nil {
		t.Error("Results is nil, want a non-nil empty slice")
	}
	if len(got.Results) != 0 {
		t.Errorf("Results has %d entries, want 0", len(got.Results))
	}
}

func TestResultsForRevealedQuestionPropagatesStoreErrors(t *testing.T) {
	sentinel := errors.New("boom")
	cases := []struct {
		name string
		st   *stubStore
	}{
		{"GetGameForOrganizer", &stubStore{gameErr: sentinel}},
		{"ListQuestionsByGame", &stubStore{game: resultsGame(), listQuestionsByGameErr: sentinel}},
		{"GetLeaderboard", &stubStore{game: resultsGame(), listQuestionsByGameResult: []gen.Question{mcqResultsQuestion()}, getLeaderboardErr: sentinel}},
		{"ListAnswerResultsForQuestion", &stubStore{game: resultsGame(), listQuestionsByGameResult: []gen.Question{mcqResultsQuestion()}, listAnswerResultsForQuestionErr: sentinel}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEngine(tc.st, "+972 50-000-0000", nil)
			_, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
			if !errors.Is(err, sentinel) {
				t.Errorf("ResultsForRevealedQuestion() error = %v, want %v", err, sentinel)
			}
		})
	}
}

func TestResultsForRevealedQuestionOutOfRangeCorrectOptionReturnsError(t *testing.T) {
	bad := mcqResultsQuestion()
	bad.CorrectOption = 0
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{bad},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err == nil {
		t.Fatal("ResultsForRevealedQuestion() error = nil, want an error for an out-of-range correct_option")
	}
}

// --- Code review, story 3.8 ---

// isLastQuestion mirrors NextQuestion's "is there a question at
// position+1" test rather than comparing position to len(questions).
// Positions are not guaranteed dense (00003 declines UNIQUE (game_id,
// position)), and under the old count-based test this shape reported the
// real final question as "not last" — telling the room "עוד הכול פתוח"
// immediately before FinishGame ran.
func TestResultsForRevealedQuestionIsLastQuestionIgnoresQuestionCountWhenPositionsAreNotDense(t *testing.T) {
	dup := freeTextResultsQuestion(2)
	dup.ID = "q2-dup"
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{mcqResultsQuestion(), freeTextResultsQuestion(2), dup},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	got, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 2)
	if err != nil {
		t.Fatalf("ResultsForRevealedQuestion() error = %v", err)
	}
	if !got.IsLastQuestion {
		t.Error("IsLastQuestion = false, want true — nothing sits at position 3, so NextQuestion would finish the game")
	}
}

// A participant present in the answer rows but absent from the leaderboard
// must fail loudly rather than render "מקום 0 בטבלה" onto a real phone.
func TestResultsForRevealedQuestionParticipantMissingFromLeaderboardReturnsError(t *testing.T) {
	st := &stubStore{
		game:                      resultsGame(),
		listQuestionsByGameResult: []gen.Question{mcqResultsQuestion()},
		listAnswerResultsForQuestionResult: []store.AnswerResultRow{
			resultRow("p1", "+972500000001", true, 150),
			resultRow("ghost", "+972500000009", false, 0),
		},
		getLeaderboardResult: []store.ParticipantScore{
			{ParticipantID: "p1", DisplayName: "A", Score: 150},
		},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.ResultsForRevealedQuestion(context.Background(), testGameID, testOrganizerID, 1)
	if err == nil {
		t.Fatal("ResultsForRevealedQuestion() error = nil, want an error for an answerer with no leaderboard entry")
	}
}
