package game

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/grading"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// answerStub builds a stubStore wired for a successful RecordAnswer flow on
// an open question of the given type: IsOpen true, AlreadyAnswered false,
// and a GetGameByID result so the post-accept snapshot build (broadcast,
// AC-5) succeeds too. Individual tests override fields for their own
// scenario.
func answerStub(questionType string) *stubStore {
	return &stubStore{
		getOpenQuestionForPlayerResult: gen.GetOpenQuestionForPlayerRow{
			GameID:          testGameID,
			QuestionID:      "q1",
			QuestionType:    questionType,
			ParticipantID:   "p1",
			IsOpen:          true,
			AlreadyAnswered: false,
		},
		recordAnswerResult: gen.Answer{ID: "a1"},
		getGameByIDResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateQuestionOpen, JoinCode: "AB2CD3",
		},
	}
}

func TestRecordAnswerNoOpenQuestionReturnsErrNoOpenQuestion(t *testing.T) {
	st := &stubStore{getOpenQuestionForPlayerErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.RecordAnswer(context.Background(), testPhone, "1", time.Now())
	if !errors.Is(err, ErrNoOpenQuestion) {
		t.Fatalf("RecordAnswer() err = %v, want ErrNoOpenQuestion", err)
	}
	if st.recordAnswerCalls != 0 {
		t.Errorf("RecordAnswer (store) called %d times, want 0", st.recordAnswerCalls)
	}
}

func TestRecordAnswerMCQDigitAccepted(t *testing.T) {
	for raw, want := range map[string]string{"1": "1", "2": "2", "3": "3", "4": "4"} {
		t.Run(raw, func(t *testing.T) {
			st := answerStub("mcq")
			e := NewEngine(st, "+972 50-000-0000", nil)

			result, err := e.RecordAnswer(context.Background(), testPhone, raw, time.Now())
			if err != nil {
				t.Fatalf("RecordAnswer() err = %v, want nil", err)
			}
			if result.Outcome != AnswerAccepted {
				t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
			}
			if st.recordAnswerArg.Response != want {
				t.Errorf("Response = %q, want %q", st.recordAnswerArg.Response, want)
			}
		})
	}
}

func TestRecordAnswerMCQLetterAccepted(t *testing.T) {
	cases := map[string]string{"א": "1", "ב": "2", "ג": "3", "ד": "4"}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			st := answerStub("mcq")
			e := NewEngine(st, "+972 50-000-0000", nil)

			result, err := e.RecordAnswer(context.Background(), testPhone, raw, time.Now())
			if err != nil {
				t.Fatalf("RecordAnswer() err = %v, want nil", err)
			}
			if result.Outcome != AnswerAccepted {
				t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
			}
			if st.recordAnswerArg.Response != want {
				t.Errorf("Response = %q, want %q", st.recordAnswerArg.Response, want)
			}
		})
	}
}

// TestRecordAnswerAlreadyAnsweredPreCheckSkipsFormatValidation covers the
// review finding: a malformed reply from someone who already answered must
// be told their first answer counts (AC-3), not get the format-hint reply
// implying they can still answer — even though the content itself would
// otherwise fail MCQ parsing.
func TestRecordAnswerAlreadyAnsweredPreCheckSkipsFormatValidation(t *testing.T) {
	st := answerStub("mcq")
	st.getOpenQuestionForPlayerResult.AlreadyAnswered = true

	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "not a valid option", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAlreadyAnswered {
		t.Errorf("Outcome = %q, want AnswerAlreadyAnswered", result.Outcome)
	}
	if st.recordAnswerCalls != 0 {
		t.Errorf("RecordAnswer (store) called %d times, want 0", st.recordAnswerCalls)
	}
}

// TestRecordAnswerClosedPreCheckSkipsFormatValidation covers the review
// finding: a malformed or over-length reply arriving after the question
// closed must get "השאלה נסגרה" (AC-4), not the format-hint/too-long reply
// implying there's still time to answer.
func TestRecordAnswerClosedPreCheckSkipsFormatValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"mcq unparseable", "not a valid option"},
		{"free text over limit", strings.Repeat("א", 201)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			questionType := "mcq"
			if tc.name == "free text over limit" {
				questionType = "free_text"
			}
			st := answerStub(questionType)
			st.getOpenQuestionForPlayerResult.IsOpen = false

			e := NewEngine(st, "+972 50-000-0000", nil)

			result, err := e.RecordAnswer(context.Background(), testPhone, tc.raw, time.Now())
			if err != nil {
				t.Fatalf("RecordAnswer() err = %v, want nil", err)
			}
			if result.Outcome != AnswerClosed {
				t.Errorf("Outcome = %q, want AnswerClosed", result.Outcome)
			}
			if st.recordAnswerCalls != 0 {
				t.Errorf("RecordAnswer (store) called %d times, want 0", st.recordAnswerCalls)
			}
		})
	}
}

// TestRecordAnswerMCQUnparseableReturnsFormatHint also pins the REJECTION
// side of parseMCQOption's code-point range check (story 3.5 replaced the
// literal-keyed map with a U+05D0 offset so this file carries no Hebrew
// literal — CI copy-centralization). TestRecordAnswerMCQLetterAccepted
// already pins the four accepted letters; the cases that matter after that
// rewrite are the ones just outside the range, since a map miss and an
// off-by-one in "offset <= 3" fail in different directions: HE (U+05D4) is
// the very next code point after DALET and must NOT become option 5, and a
// multi-rune reply must not decode to its first rune.
func TestRecordAnswerMCQUnparseableReturnsFormatHint(t *testing.T) {
	for _, raw := range []string{"5", "א.", "hello", "0", "ה", "אב", "אא"} {
		t.Run(raw, func(t *testing.T) {
			st := answerStub("mcq")
			e := NewEngine(st, "+972 50-000-0000", nil)

			result, err := e.RecordAnswer(context.Background(), testPhone, raw, time.Now())
			if err != nil {
				t.Fatalf("RecordAnswer() err = %v, want nil", err)
			}
			if result.Outcome != AnswerFormatHint {
				t.Errorf("Outcome = %q, want AnswerFormatHint", result.Outcome)
			}
			if st.recordAnswerCalls != 0 {
				t.Errorf("RecordAnswer (store) called %d times, want 0", st.recordAnswerCalls)
			}
		})
	}
}

func TestRecordAnswerFreeTextWithinLimitAccepted(t *testing.T) {
	st := answerStub("free_text")
	e := NewEngine(st, "+972 50-000-0000", nil)
	text := strings.Repeat("א", 200)

	result, err := e.RecordAnswer(context.Background(), testPhone, text, time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if st.recordAnswerArg.Response != text {
		t.Errorf("Response = %q, want the trimmed input text", st.recordAnswerArg.Response)
	}
}

func TestRecordAnswerFreeTextOverLimitReturnsTooLong(t *testing.T) {
	st := answerStub("free_text")
	e := NewEngine(st, "+972 50-000-0000", nil)
	text := strings.Repeat("א", 201)

	result, err := e.RecordAnswer(context.Background(), testPhone, text, time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerTooLong {
		t.Errorf("Outcome = %q, want AnswerTooLong", result.Outcome)
	}
	if st.recordAnswerCalls != 0 {
		t.Errorf("RecordAnswer (store) called %d times, want 0", st.recordAnswerCalls)
	}
}

func TestRecordAnswerFreeTextTrimsWhitespace(t *testing.T) {
	st := answerStub("free_text")
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "  שלום  ", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if st.recordAnswerArg.Response != "שלום" {
		t.Errorf("Response = %q, want trimmed %q", st.recordAnswerArg.Response, "שלום")
	}
}

func TestRecordAnswerAlreadyAnsweredMapsToOutcome(t *testing.T) {
	st := answerStub("mcq")
	st.recordAnswerErr = store.ErrAlreadyAnswered
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "1", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAlreadyAnswered {
		t.Errorf("Outcome = %q, want AnswerAlreadyAnswered", result.Outcome)
	}
}

func TestRecordAnswerClosedMapsToOutcome(t *testing.T) {
	st := answerStub("mcq")
	st.recordAnswerErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "1", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerClosed {
		t.Errorf("Outcome = %q, want AnswerClosed", result.Outcome)
	}
}

func TestRecordAnswerStoreErrorPropagates(t *testing.T) {
	st := answerStub("mcq")
	st.recordAnswerErr = errors.New("boom")
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.RecordAnswer(context.Background(), testPhone, "1", time.Now())
	if err == nil {
		t.Fatal("RecordAnswer() err = nil, want a propagated error")
	}
}

func TestRecordAnswerPassesReceivedAtThrough(t *testing.T) {
	st := answerStub("mcq")
	e := NewEngine(st, "+972 50-000-0000", nil)
	receivedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	_, err := e.RecordAnswer(context.Background(), testPhone, "1", receivedAt)
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if !st.recordAnswerArg.ReceivedAt.Equal(receivedAt) {
		t.Errorf("ReceivedAt = %v, want %v", st.recordAnswerArg.ReceivedAt, receivedAt)
	}
}

// TestRecordAnswerAcceptedIncludesBroadcastSnapshot covers the review
// finding that an accepted answer must carry a fresh Snapshot for
// wa.handleTextOrAnswer to broadcast — otherwise the control panel's live
// answered-count (AC-5) never updates on incoming WhatsApp answers.
func TestRecordAnswerAcceptedIncludesBroadcastSnapshot(t *testing.T) {
	st := answerStub("mcq")
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "1", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.GameID != testGameID {
		t.Errorf("GameID = %q, want %q", result.GameID, testGameID)
	}
	if result.Snapshot.GameID != testGameID {
		t.Errorf("Snapshot.GameID = %q, want %q (caller broadcasts only when non-empty)", result.Snapshot.GameID, testGameID)
	}
}

// --- Grading (stories 3.4-3.5) ---

func TestRecordAnswerMCQCorrectSetsIsCorrectTrue(t *testing.T) {
	st := answerStub("mcq")
	st.getOpenQuestionForPlayerResult.CorrectOption = 2
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "2", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if !st.recordAnswerArg.IsCorrect {
		t.Error("IsCorrect = false, want true")
	}
	if st.recordAnswerArg.Stage != grading.StageMCQ {
		t.Errorf("Stage = %q, want %q", st.recordAnswerArg.Stage, grading.StageMCQ)
	}
}

func TestRecordAnswerMCQIncorrectSetsIsCorrectFalse(t *testing.T) {
	st := answerStub("mcq")
	st.getOpenQuestionForPlayerResult.CorrectOption = 2
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "1", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if st.recordAnswerArg.IsCorrect {
		t.Error("IsCorrect = true, want false")
	}
	if st.recordAnswerArg.Stage != grading.StageMCQ {
		t.Errorf("Stage = %q, want %q", st.recordAnswerArg.Stage, grading.StageMCQ)
	}
}

func TestRecordAnswerFreeTextExactMatchSetsIsCorrectTrue(t *testing.T) {
	st := answerStub("free_text")
	st.getOpenQuestionForPlayerResult.AcceptedAnswers = []string{"ירושלים", "ירושלים עיר הקודש"}
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "ירושלים", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if !st.recordAnswerArg.IsCorrect {
		t.Error("IsCorrect = false, want true")
	}
	if st.recordAnswerArg.Stage != grading.StageExact {
		t.Errorf("Stage = %q, want %q", st.recordAnswerArg.Stage, grading.StageExact)
	}
}

// TestRecordAnswerFreeTextNoMatchSetsIsCorrectFalse covers this story's
// design decision (Dev Notes): a response missing both Exact and Fuzzy is
// graded false at stage = fuzzy — the last stage that ran, not left
// pending — Reveal must not stay permanently blocked waiting for a stage
// that doesn't exist yet (AI, story 3.6).
func TestRecordAnswerFreeTextNoMatchSetsIsCorrectFalse(t *testing.T) {
	st := answerStub("free_text")
	st.getOpenQuestionForPlayerResult.AcceptedAnswers = []string{"ירושלים"}
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "תל אביב", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if st.recordAnswerArg.IsCorrect {
		t.Error("IsCorrect = true, want false")
	}
	if st.recordAnswerArg.Stage != grading.StageFuzzy {
		t.Errorf("Stage = %q, want %q (graded, not left ungraded)", st.recordAnswerArg.Stage, grading.StageFuzzy)
	}
}

// TestRecordAnswerFreeTextFuzzyMatchSetsIsCorrectTrue covers this story's
// AC-1/AC-2: an Exact miss that fuzzily matches (here, a single-letter
// typo) is graded correct at stage = fuzzy.
func TestRecordAnswerFreeTextFuzzyMatchSetsIsCorrectTrue(t *testing.T) {
	st := answerStub("free_text")
	st.getOpenQuestionForPlayerResult.AcceptedAnswers = []string{"ירושלים"}
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "ירוסלים", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if !st.recordAnswerArg.IsCorrect {
		t.Error("IsCorrect = false, want true")
	}
	if st.recordAnswerArg.Stage != grading.StageFuzzy {
		t.Errorf("Stage = %q, want %q", st.recordAnswerArg.Stage, grading.StageFuzzy)
	}
}

// Written as rune values, not literals: the characters are invisible in an
// editor and a literal U+FEFF is rejected by the compiler.
var (
	testLRM = string(rune(0x200e)) // LEFT-TO-RIGHT MARK
	testRLM = string(rune(0x200f)) // RIGHT-TO-LEFT MARK
	testBOM = string(rune(0xfeff)) // ZERO WIDTH NO-BREAK SPACE
)

// TestRecordAnswerFreeTextIgnoresInvisibleFormatMarks covers a code review
// finding: Hebrew mobile keyboards and copy-paste inject category-Cf marks
// that strings.TrimSpace does not remove, so without stripFormatMarks they
// reach GradeExact's byte comparison and mark a visually identical answer
// incorrect — with the participant still receiving the normal ack.
func TestRecordAnswerFreeTextIgnoresInvisibleFormatMarks(t *testing.T) {
	st := answerStub("free_text")
	st.getOpenQuestionForPlayerResult.AcceptedAnswers = []string{"ירושלים"}
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, testRLM+"ירושלים"+testLRM, time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if !st.recordAnswerArg.IsCorrect {
		t.Error("IsCorrect = false, want true (invisible marks must not defeat an exact match)")
	}
	// The persisted response must be the stripped form too — a stored
	// answer carrying invisible marks would re-break any later comparison
	// (Story 3.7 scoring, 3.8 personal results).
	if st.recordAnswerArg.Response != "ירושלים" {
		t.Errorf("Response = %q, want the stripped form", st.recordAnswerArg.Response)
	}
}

// TestRecordAnswerMCQIgnoresInvisibleFormatMarks covers the same class on
// the mcq branch, where a mark instead defeats parseMCQOption and the
// sender gets the format hint for a reply that looks perfectly valid.
func TestRecordAnswerMCQIgnoresInvisibleFormatMarks(t *testing.T) {
	st := answerStub("mcq")
	st.getOpenQuestionForPlayerResult.CorrectOption = 2
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, testBOM+"2"+testRLM, time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Fatalf("Outcome = %q, want AnswerAccepted (marks must not earn a format hint)", result.Outcome)
	}
	if !st.recordAnswerArg.IsCorrect {
		t.Error("IsCorrect = false, want true")
	}
}

// TestRecordAnswerAcceptedSnapshotBuildFailureSkipsBroadcast covers the
// degrade path: the answer already committed, so a failure building the
// post-answer snapshot must not fail RecordAnswer's caller — it must just
// return a zero Snapshot (GameID == "") so wa.handleTextOrAnswer skips the
// broadcast, same posture as joinLobby's post-join snapshot failure.
func TestRecordAnswerAcceptedSnapshotBuildFailureSkipsBroadcast(t *testing.T) {
	st := answerStub("mcq")
	st.getGameByIDErr = errors.New("boom")
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.RecordAnswer(context.Background(), testPhone, "1", time.Now())
	if err != nil {
		t.Fatalf("RecordAnswer() err = %v, want nil (the answer already committed)", err)
	}
	if result.Outcome != AnswerAccepted {
		t.Errorf("Outcome = %q, want AnswerAccepted", result.Outcome)
	}
	if result.Snapshot.GameID != "" {
		t.Errorf("Snapshot.GameID = %q, want empty (no snapshot to broadcast)", result.Snapshot.GameID)
	}
}
