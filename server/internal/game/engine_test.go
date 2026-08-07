package game

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

const (
	testGameID      = "11111111-2222-3333-4444-555555555555"
	testOrganizerID = "org-1"
)

// testCutoff is a fixed, arbitrary answer_cutoff_at value used across the
// question-progress tests below.
var testCutoff = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// stubStore implements Store with canned results and records every call so
// tests can assert what reached (or didn't reach) the store.
type stubStore struct {
	game    gen.Game
	gameErr error

	openGameLobbyResult gen.Game
	openGameLobbyErr    error
	openGameLobbyCalls  int

	participants    []gen.Participant
	participantsErr error

	getGameByJoinCodeResult gen.Game
	getGameByJoinCodeErr    error
	getGameByJoinCodeCalls  int

	getGameByIDResult gen.Game
	getGameByIDErr    error
	getGameByIDCalls  int

	createParticipantResult        gen.Participant
	createParticipantCreated       bool
	createParticipantErr           error
	createParticipantCalls         int
	createParticipantRole          string
	createParticipantAllowedStates []string

	updateParticipantNameByPhoneResult gen.Participant
	updateParticipantNameByPhoneErr    error
	updateParticipantNameByPhoneCalls  int
	updateParticipantNameByPhonePhone  string
	updateParticipantNameByPhoneName   string

	listQuestionsByGameResult []gen.Question
	listQuestionsByGameErr    error
	listQuestionsByGameCalls  int

	startGameFirstQuestionResult gen.Game
	startGameFirstQuestionErr    error
	startGameFirstQuestionCalls  int

	closeCurrentQuestionResult gen.Game
	closeCurrentQuestionErr    error
	closeCurrentQuestionCalls  int

	revealCurrentQuestionAndAwardPointsResult gen.Game
	revealCurrentQuestionAndAwardPointsErr    error
	revealCurrentQuestionAndAwardPointsCalls  int
	revealCurrentQuestionAndAwardPointsArg    []store.AnswerPointsParams

	listAnswersForScoringResult   []store.AnswerForScoring
	listAnswersForScoringErr      error
	listAnswersForScoringGameID   string
	listAnswersForScoringPosition int32

	getLeaderboardResult []store.ParticipantScore
	getLeaderboardErr    error

	listAnswerResultsForQuestionResult []store.AnswerResultRow
	listAnswerResultsForQuestionErr    error

	openNextQuestionResult   gen.Game
	openNextQuestionErr      error
	openNextQuestionCalls    int
	openNextQuestionPosition int32

	finishGameResult gen.Game
	finishGameErr    error
	finishGameCalls  int

	getOpenQuestionForPlayerResult gen.GetOpenQuestionForPlayerRow
	getOpenQuestionForPlayerErr    error

	recordAnswerResult gen.Answer
	recordAnswerErr    error
	recordAnswerCalls  int
	recordAnswerArg    store.RecordAnswerParams

	countAnswersByQuestionResult int64
	countAnswersByQuestionErr    error

	countUngradedAnswersForCurrentQuestionResult int64
	countUngradedAnswersForCurrentQuestionErr    error
	countUngradedAnswersForCurrentQuestionCalls  int

	updateAnswerGradeErr   error
	updateAnswerGradeCalls int
	updateAnswerGradeArg   struct {
		answerID  string
		isCorrect bool
		stage     string
	}
}

func (s *stubStore) GetGameForOrganizer(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	return s.game, s.gameErr
}

func (s *stubStore) OpenGameLobby(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	s.openGameLobbyCalls++
	return s.openGameLobbyResult, s.openGameLobbyErr
}

func (s *stubStore) ListParticipants(ctx context.Context, gameID string) ([]gen.Participant, error) {
	return s.participants, s.participantsErr
}

func (s *stubStore) GetGameByJoinCode(ctx context.Context, joinCode string) (gen.Game, error) {
	s.getGameByJoinCodeCalls++
	return s.getGameByJoinCodeResult, s.getGameByJoinCodeErr
}

func (s *stubStore) GetGameByID(ctx context.Context, gameID string) (gen.Game, error) {
	s.getGameByIDCalls++
	return s.getGameByIDResult, s.getGameByIDErr
}

func (s *stubStore) CreateParticipant(ctx context.Context, gameID, phone, displayName, role string, allowedStates []string) (gen.Participant, bool, error) {
	s.createParticipantCalls++
	s.createParticipantRole = role
	s.createParticipantAllowedStates = allowedStates
	if s.createParticipantErr != nil {
		return gen.Participant{}, false, s.createParticipantErr
	}
	p := s.createParticipantResult
	if s.createParticipantCreated {
		// A fresh INSERT ... RETURNING echoes back exactly what was
		// written — lets tests assert Join correctly threads the resolved
		// name through without the stub needing to duplicate that logic.
		p.DisplayName = displayName
	}
	// On a conflict (created == false), the real store's ON CONFLICT DO
	// NOTHING + refetch returns the EXISTING row's stored name, not the
	// name this call was asked to write — createParticipantResult.DisplayName
	// stands in for that pre-existing value (AC-3: idempotent repeat replies
	// with the current stored name, not necessarily the name resent).
	return p, s.createParticipantCreated, nil
}

func (s *stubStore) UpdateParticipantNameByPhone(ctx context.Context, phone, displayName string) (gen.Participant, error) {
	s.updateParticipantNameByPhoneCalls++
	s.updateParticipantNameByPhonePhone = phone
	s.updateParticipantNameByPhoneName = displayName
	return s.updateParticipantNameByPhoneResult, s.updateParticipantNameByPhoneErr
}

func (s *stubStore) ListQuestionsByGame(ctx context.Context, gameID, organizerID string) ([]gen.Question, error) {
	s.listQuestionsByGameCalls++
	return s.listQuestionsByGameResult, s.listQuestionsByGameErr
}

func (s *stubStore) StartGameFirstQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	s.startGameFirstQuestionCalls++
	return s.startGameFirstQuestionResult, s.startGameFirstQuestionErr
}

func (s *stubStore) CloseCurrentQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	s.closeCurrentQuestionCalls++
	return s.closeCurrentQuestionResult, s.closeCurrentQuestionErr
}

func (s *stubStore) RevealCurrentQuestionAndAwardPoints(ctx context.Context, gameID, organizerID string, points []store.AnswerPointsParams) (gen.Game, error) {
	s.revealCurrentQuestionAndAwardPointsCalls++
	s.revealCurrentQuestionAndAwardPointsArg = points
	return s.revealCurrentQuestionAndAwardPointsResult, s.revealCurrentQuestionAndAwardPointsErr
}

func (s *stubStore) ListAnswersForScoring(ctx context.Context, gameID string, position int32) ([]store.AnswerForScoring, error) {
	s.listAnswersForScoringGameID = gameID
	s.listAnswersForScoringPosition = position
	return s.listAnswersForScoringResult, s.listAnswersForScoringErr
}

func (s *stubStore) GetLeaderboard(ctx context.Context, gameID string) ([]store.ParticipantScore, error) {
	return s.getLeaderboardResult, s.getLeaderboardErr
}

func (s *stubStore) ListAnswerResultsForQuestion(ctx context.Context, gameID string, position int32) ([]store.AnswerResultRow, error) {
	return s.listAnswerResultsForQuestionResult, s.listAnswerResultsForQuestionErr
}

func (s *stubStore) OpenNextQuestion(ctx context.Context, gameID, organizerID string, position int32) (gen.Game, error) {
	s.openNextQuestionCalls++
	s.openNextQuestionPosition = position
	return s.openNextQuestionResult, s.openNextQuestionErr
}

func (s *stubStore) FinishGame(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	s.finishGameCalls++
	return s.finishGameResult, s.finishGameErr
}

func (s *stubStore) GetOpenQuestionForPlayer(ctx context.Context, phone string) (gen.GetOpenQuestionForPlayerRow, error) {
	return s.getOpenQuestionForPlayerResult, s.getOpenQuestionForPlayerErr
}

func (s *stubStore) RecordAnswer(ctx context.Context, arg store.RecordAnswerParams) (gen.Answer, error) {
	s.recordAnswerCalls++
	s.recordAnswerArg = arg
	return s.recordAnswerResult, s.recordAnswerErr
}

func (s *stubStore) CountAnswersByQuestion(ctx context.Context, questionID string) (int64, error) {
	return s.countAnswersByQuestionResult, s.countAnswersByQuestionErr
}

func (s *stubStore) CountUngradedAnswersForCurrentQuestion(ctx context.Context, gameID string) (int64, error) {
	s.countUngradedAnswersForCurrentQuestionCalls++
	return s.countUngradedAnswersForCurrentQuestionResult, s.countUngradedAnswersForCurrentQuestionErr
}

func (s *stubStore) UpdateAnswerGrade(ctx context.Context, answerID string, isCorrect bool, stage string) error {
	s.updateAnswerGradeCalls++
	s.updateAnswerGradeArg.answerID = answerID
	s.updateAnswerGradeArg.isCorrect = isCorrect
	s.updateAnswerGradeArg.stage = stage
	return s.updateAnswerGradeErr
}

func draftStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateDraft, JoinCode: "AB2CD3"}
	return &stubStore{game: g, openGameLobbyResult: gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateLobby, JoinCode: "AB2CD3"}}
}

// oneQuestion is a single mcq question at position 1, reused across the
// lobby/question-open/question-closed stubs below.
func oneQuestion() []gen.Question {
	return []gen.Question{
		{ID: "q1", GameID: testGameID, Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20},
	}
}

// twoQuestions is two mcq questions at positions 1 and 2.
func twoQuestions() []gen.Question {
	return []gen.Question{
		{ID: "q1", GameID: testGameID, Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20},
		{ID: "q2", GameID: testGameID, Position: 2, Type: "mcq", Text: "2+2?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20},
	}
}

func lobbyStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateLobby, JoinCode: "AB2CD3"}
	return &stubStore{
		game:                      g,
		listQuestionsByGameResult: oneQuestion(),
		startGameFirstQuestionResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateQuestionOpen, JoinCode: "AB2CD3",
			CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff,
		},
	}
}

func questionOpenStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateQuestionOpen, JoinCode: "AB2CD3", CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff}
	return &stubStore{
		game:                      g,
		listQuestionsByGameResult: oneQuestion(),
		closeCurrentQuestionResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateQuestionClosed, JoinCode: "AB2CD3",
			CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff,
		},
	}
}

func questionClosedStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateQuestionClosed, JoinCode: "AB2CD3", CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff}
	return &stubStore{
		game:                      g,
		listQuestionsByGameResult: oneQuestion(),
		revealCurrentQuestionAndAwardPointsResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateRevealed, JoinCode: "AB2CD3",
			CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff,
		},
	}
}

// revealedStub is revealed with a second question waiting at position 2.
func revealedStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateRevealed, JoinCode: "AB2CD3", CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff}
	return &stubStore{
		game:                      g,
		listQuestionsByGameResult: twoQuestions(),
		openNextQuestionResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateQuestionOpen, JoinCode: "AB2CD3",
			CurrentQuestionPosition: 2, AnswerCutoffAt: testCutoff,
		},
	}
}

// revealedLastQuestionStub is revealed with no question waiting at position 2.
func revealedLastQuestionStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateRevealed, JoinCode: "AB2CD3", CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff}
	return &stubStore{
		game:                      g,
		listQuestionsByGameResult: oneQuestion(),
		// FinishGame resets current_question_position back to 0 (mirrors
		// draft/lobby/finished — see the migration's own invariant), so a
		// finished game must never carry a stale currentQuestion.
		finishGameResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateFinished, JoinCode: "AB2CD3",
		},
	}
}

func TestOpenLobbyFromDraftReturnsLobbySnapshot(t *testing.T) {
	st := draftStub()
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.OpenLobby(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("OpenLobby() err = %v, want nil", err)
	}
	if snap.State != StateLobby || snap.GameID != testGameID || snap.JoinCode != "AB2CD3" {
		t.Errorf("snapshot = %+v, want lobby state with gameID/joinCode carried through", snap)
	}
	if snap.PlatformNumber != "+972 50-000-0000" {
		t.Errorf("PlatformNumber = %q, want the configured platform number", snap.PlatformNumber)
	}
	if snap.ParticipantCount != 0 || snap.Participants == nil || len(snap.Participants) != 0 {
		t.Errorf("Participants = %+v, want a non-nil empty slice this story", snap.Participants)
	}
	if snap.CurrentQuestion != nil {
		t.Errorf("CurrentQuestion = %+v, want nil in lobby", snap.CurrentQuestion)
	}
	if st.openGameLobbyCalls != 1 {
		t.Errorf("OpenGameLobby called %d times, want 1", st.openGameLobbyCalls)
	}
	if snap.Leaderboard == nil || len(snap.Leaderboard) != 0 {
		t.Errorf("Leaderboard = %+v, want a non-nil empty slice when GetLeaderboard returns nothing", snap.Leaderboard)
	}
}

// Expectations here are hard-coded rather than derived from
// RankLeaderboard: building want by calling the unit under test made this
// test move in lockstep with any regression in it (a no-op sort passed).
// The store order below is deliberately NOT the display order, so the
// snapshot's ranking work is what the assertion actually observes.
func TestBuildSnapshotIncludesRankedLeaderboard(t *testing.T) {
	st := draftStub()
	st.getLeaderboardResult = []store.ParticipantScore{
		{ParticipantID: "p3", DisplayName: "Carol", Score: 50},
		{ParticipantID: "p1", DisplayName: "Alice", Score: 100},
		{ParticipantID: "p2", DisplayName: "Bob", Score: 100},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.OpenLobby(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("OpenLobby() err = %v, want nil", err)
	}
	want := []LeaderboardEntry{
		{ParticipantID: "p1", DisplayName: "Alice", Score: 100, Rank: 1},
		{ParticipantID: "p2", DisplayName: "Bob", Score: 100, Rank: 1},
		{ParticipantID: "p3", DisplayName: "Carol", Score: 50, Rank: 3},
	}
	if len(snap.Leaderboard) != len(want) {
		t.Fatalf("Leaderboard = %+v, want %+v", snap.Leaderboard, want)
	}
	for i := range want {
		if snap.Leaderboard[i] != want[i] {
			t.Errorf("Leaderboard[%d] = %+v, want %+v", i, snap.Leaderboard[i], want[i])
		}
	}
}

// The degraded path: buildSnapshot fails after the transition already
// committed, so snapshotAfterCommit falls back to emptySnapshot. The
// leaderboard must still be a non-nil empty slice — web's LobbySnapshot
// types it as non-nullable. Nothing else covered this: the non-nil
// assertion in TestOpenLobbyFromDraftReturnsLobbySnapshot exercises the
// buildSnapshot SUCCESS path, where RankLeaderboard supplies the empty
// slice, so deleting emptySnapshot's own default changed no test.
func TestSnapshotAfterCommitDegradesWithNonNilLeaderboard(t *testing.T) {
	st := questionClosedStub()
	st.listQuestionsByGameErr = errors.New("questions unavailable")
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Reveal() err = %v, want nil (a failed snapshot build must not fail the committed transition)", err)
	}
	if snap.Leaderboard == nil {
		t.Error("Leaderboard = nil on the degraded snapshot, want a non-nil empty slice")
	}
	if len(snap.Leaderboard) != 0 {
		t.Errorf("Leaderboard = %+v, want empty on the degraded snapshot", snap.Leaderboard)
	}
}

func TestRevealPropagatesListAnswersForScoringError(t *testing.T) {
	st := questionClosedStub()
	boom := errors.New("scoring read failed")
	st.listAnswersForScoringErr = boom
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, boom) {
		t.Fatalf("Reveal() err = %v, want the store error propagated unwrapped", err)
	}
	if st.revealCurrentQuestionAndAwardPointsCalls != 0 {
		t.Errorf("RevealCurrentQuestionAndAwardPoints called %d times, want 0 (a failed scoring read must not fall through to the write)", st.revealCurrentQuestionAndAwardPointsCalls)
	}
}

// Engine.Snapshot is the one buildSnapshot caller that does NOT degrade
// (ws.Handler's read path), so it is where the new GetLeaderboard error
// return is observable. Worth pinning: a leaderboard read failure now
// fails the whole snapshot, discarding participants, question count and
// current question along with it.
func TestSnapshotPropagatesGetLeaderboardError(t *testing.T) {
	st := draftStub()
	boom := errors.New("leaderboard unavailable")
	st.getLeaderboardErr = boom
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, boom) {
		t.Fatalf("Snapshot() err = %v, want the GetLeaderboard error propagated unwrapped", err)
	}
}

func TestOpenLobbyFromNonDraftReturnsErrNotDraftWithoutWriting(t *testing.T) {
	st := draftStub()
	st.game.State = StateLobby
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.OpenLobby(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotDraft) {
		t.Fatalf("OpenLobby() err = %v, want ErrNotDraft", err)
	}
	if st.openGameLobbyCalls != 0 {
		t.Errorf("OpenGameLobby called %d times, want 0 (non-draft rejected before the write)", st.openGameLobbyCalls)
	}
}

func TestOpenLobbyMissingOrForeignGameReturnsErrNotFound(t *testing.T) {
	st := &stubStore{gameErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.OpenLobby(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("OpenLobby() err = %v, want store.ErrNotFound", err)
	}
	if st.openGameLobbyCalls != 0 {
		t.Errorf("OpenGameLobby called %d times, want 0", st.openGameLobbyCalls)
	}
}

func TestOpenLobbyRaceLossReturnsErrNotDraft(t *testing.T) {
	// Simulates a concurrent double-click: the read sees draft, but the
	// atomic UPDATE's WHERE state = 'draft' matches zero rows because
	// another request won the race in between.
	st := draftStub()
	st.openGameLobbyErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.OpenLobby(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotDraft) {
		t.Fatalf("OpenLobby() err = %v, want ErrNotDraft (race loss reinterpreted, not raw ErrNotFound)", err)
	}
}

func TestOpenLobbyDegradesToEmptyParticipantsWhenSnapshotBuildFailsAfterCommit(t *testing.T) {
	// The transition itself (OpenGameLobby) already committed — a failure
	// listing participants afterward must not be reported as a failed
	// transition (the caller would retry into a confusing 409).
	st := draftStub()
	st.participantsErr = errors.New("boom")
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.OpenLobby(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("OpenLobby() err = %v, want nil (degrade, not fail)", err)
	}
	if snap.State != StateLobby || snap.GameID != testGameID {
		t.Errorf("snapshot = %+v, want the committed lobby state carried through", snap)
	}
	if snap.ParticipantCount != 0 || snap.Participants == nil || len(snap.Participants) != 0 {
		t.Errorf("Participants = %+v, want a non-nil empty slice on degrade", snap.Participants)
	}
	if st.openGameLobbyCalls != 1 {
		t.Errorf("OpenGameLobby called %d times, want 1 (the transition still happened)", st.openGameLobbyCalls)
	}
}

func TestSnapshotReturnsCurrentStateWithParticipantCount(t *testing.T) {
	st := draftStub()
	st.game.State = StateLobby
	st.participants = []gen.Participant{
		{ID: "p1", GameID: testGameID, DisplayName: "דנה"},
		{ID: "p2", GameID: testGameID, DisplayName: "יוסי"},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Snapshot() err = %v, want nil", err)
	}
	if snap.ParticipantCount != 2 || len(snap.Participants) != 2 {
		t.Fatalf("snapshot participants = %+v, want 2 entries", snap)
	}
	if snap.Participants[0].ID != "p1" || snap.Participants[0].DisplayName != "דנה" {
		t.Errorf("first participant = %+v, want p1/דנה", snap.Participants[0])
	}
}

// TestSnapshotAnsweredCountErrorPropagates covers buildSnapshot's new
// CountAnswersByQuestion call failing while resolving the current question —
// propagates the same as the pre-existing ListParticipants/ListQuestionsByGame
// errors in this function (no special-case degradation here; the
// snapshotAfterCommit callers already degrade a buildSnapshot failure to an
// empty snapshot on their own, unrelated to this direct Snapshot() call).
func TestSnapshotAnsweredCountErrorPropagates(t *testing.T) {
	st := questionOpenStub()
	st.countAnswersByQuestionErr = errors.New("boom")
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if err == nil {
		t.Fatal("Snapshot() err = nil, want a propagated error")
	}
}

func TestSnapshotMissingOrForeignGameReturnsErrNotFound(t *testing.T) {
	st := &stubStore{gameErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Snapshot() err = %v, want store.ErrNotFound", err)
	}
}

// --- StartGame ---

func TestStartGameFromLobbyReturnsQuestionOpenSnapshot(t *testing.T) {
	st := lobbyStub()
	st.countAnswersByQuestionResult = 3
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.StartGame(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("StartGame() err = %v, want nil", err)
	}
	if snap.State != StateQuestionOpen {
		t.Errorf("State = %q, want question_open", snap.State)
	}
	if snap.QuestionCount != 1 {
		t.Errorf("QuestionCount = %d, want 1", snap.QuestionCount)
	}
	if snap.CurrentQuestion == nil || snap.CurrentQuestion.ID != "q1" || snap.CurrentQuestion.Position != 1 {
		t.Fatalf("CurrentQuestion = %+v, want q1 at position 1", snap.CurrentQuestion)
	}
	if snap.CurrentQuestion.AnswerCutoffAt != testCutoff.Format(time.RFC3339) {
		t.Errorf("AnswerCutoffAt = %q, want %q", snap.CurrentQuestion.AnswerCutoffAt, testCutoff.Format(time.RFC3339))
	}
	if snap.CurrentQuestion.AnsweredCount != 3 {
		t.Errorf("AnsweredCount = %d, want 3", snap.CurrentQuestion.AnsweredCount)
	}
	if st.startGameFirstQuestionCalls != 1 {
		t.Errorf("StartGameFirstQuestion called %d times, want 1", st.startGameFirstQuestionCalls)
	}
}

func TestStartGameFromNonLobbyReturnsErrNotLobbyWithoutWriting(t *testing.T) {
	st := lobbyStub()
	st.game.State = StateDraft
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.StartGame(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotLobby) {
		t.Fatalf("StartGame() err = %v, want ErrNotLobby", err)
	}
	if st.startGameFirstQuestionCalls != 0 {
		t.Errorf("StartGameFirstQuestion called %d times, want 0", st.startGameFirstQuestionCalls)
	}
	if st.listQuestionsByGameCalls != 0 {
		t.Errorf("ListQuestionsByGame called %d times, want 0 (rejected before checking questions)", st.listQuestionsByGameCalls)
	}
}

func TestStartGameWithNoQuestionsReturnsErrNoQuestionsWithoutWriting(t *testing.T) {
	st := lobbyStub()
	st.listQuestionsByGameResult = nil
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.StartGame(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNoQuestions) {
		t.Fatalf("StartGame() err = %v, want ErrNoQuestions", err)
	}
	if st.startGameFirstQuestionCalls != 0 {
		t.Errorf("StartGameFirstQuestion called %d times, want 0", st.startGameFirstQuestionCalls)
	}
}

func TestStartGameRaceLossReturnsErrNotLobby(t *testing.T) {
	st := lobbyStub()
	st.startGameFirstQuestionErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.StartGame(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotLobby) {
		t.Fatalf("StartGame() err = %v, want ErrNotLobby (race loss reinterpreted)", err)
	}
}

func TestStartGameMissingOrForeignGameReturnsErrNotFound(t *testing.T) {
	st := &stubStore{gameErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.StartGame(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("StartGame() err = %v, want store.ErrNotFound", err)
	}
}

// --- CloseQuestion ---

func TestCloseQuestionFromQuestionOpenReturnsQuestionClosedSnapshot(t *testing.T) {
	st := questionOpenStub()
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.CloseQuestion(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("CloseQuestion() err = %v, want nil", err)
	}
	if snap.State != StateQuestionClosed {
		t.Errorf("State = %q, want question_closed", snap.State)
	}
	if st.closeCurrentQuestionCalls != 1 {
		t.Errorf("CloseCurrentQuestion called %d times, want 1", st.closeCurrentQuestionCalls)
	}
}

func TestCloseQuestionFromNonQuestionOpenReturnsErrNotQuestionOpenWithoutWriting(t *testing.T) {
	st := questionOpenStub()
	st.game.State = StateLobby
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.CloseQuestion(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotQuestionOpen) {
		t.Fatalf("CloseQuestion() err = %v, want ErrNotQuestionOpen", err)
	}
	if st.closeCurrentQuestionCalls != 0 {
		t.Errorf("CloseCurrentQuestion called %d times, want 0", st.closeCurrentQuestionCalls)
	}
}

func TestCloseQuestionRaceLossReturnsErrNotQuestionOpen(t *testing.T) {
	st := questionOpenStub()
	st.closeCurrentQuestionErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.CloseQuestion(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotQuestionOpen) {
		t.Fatalf("CloseQuestion() err = %v, want ErrNotQuestionOpen (race loss reinterpreted)", err)
	}
}

// --- Reveal ---

func TestRevealFromQuestionClosedReturnsRevealedSnapshot(t *testing.T) {
	st := questionClosedStub()
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Reveal() err = %v, want nil", err)
	}
	if snap.State != StateRevealed {
		t.Errorf("State = %q, want revealed", snap.State)
	}
	if st.revealCurrentQuestionAndAwardPointsCalls != 1 {
		t.Errorf("RevealCurrentQuestionAndAwardPoints called %d times, want 1", st.revealCurrentQuestionAndAwardPointsCalls)
	}
}

func TestRevealFromNonQuestionClosedReturnsErrNotQuestionClosedWithoutWriting(t *testing.T) {
	st := questionClosedStub()
	st.game.State = StateQuestionOpen
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotQuestionClosed) {
		t.Fatalf("Reveal() err = %v, want ErrNotQuestionClosed", err)
	}
	if st.revealCurrentQuestionAndAwardPointsCalls != 0 {
		t.Errorf("RevealCurrentQuestionAndAwardPoints called %d times, want 0", st.revealCurrentQuestionAndAwardPointsCalls)
	}
}

func TestRevealRaceLossReturnsErrNotQuestionClosed(t *testing.T) {
	st := questionClosedStub()
	st.revealCurrentQuestionAndAwardPointsErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotQuestionClosed) {
		t.Fatalf("Reveal() err = %v, want ErrNotQuestionClosed (race loss reinterpreted)", err)
	}
}

func TestRevealSucceedsWhenNoOutstandingGrades(t *testing.T) {
	st := questionClosedStub()
	st.countUngradedAnswersForCurrentQuestionResult = 0
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Reveal() err = %v, want nil", err)
	}
	if snap.State != StateRevealed {
		t.Errorf("State = %q, want revealed", snap.State)
	}
	if st.countUngradedAnswersForCurrentQuestionCalls != 1 {
		t.Errorf("CountUngradedAnswersForCurrentQuestion called %d times, want 1", st.countUngradedAnswersForCurrentQuestionCalls)
	}
	if st.revealCurrentQuestionAndAwardPointsCalls != 1 {
		t.Errorf("RevealCurrentQuestionAndAwardPoints called %d times, want 1", st.revealCurrentQuestionAndAwardPointsCalls)
	}
}

func TestRevealReturnsErrGradingIncompleteWhenOutstandingGradesExist(t *testing.T) {
	st := questionClosedStub()
	st.countUngradedAnswersForCurrentQuestionResult = 1
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrGradingIncomplete) {
		t.Fatalf("Reveal() err = %v, want ErrGradingIncomplete", err)
	}
	if st.revealCurrentQuestionAndAwardPointsCalls != 0 {
		t.Errorf("RevealCurrentQuestionAndAwardPoints called %d times, want 0 (the guarded write must never be attempted)", st.revealCurrentQuestionAndAwardPointsCalls)
	}
}

func TestRevealPropagatesCountUngradedAnswersError(t *testing.T) {
	boom := errors.New("boom")
	st := questionClosedStub()
	st.countUngradedAnswersForCurrentQuestionErr = boom
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	// errors.Is against the injected sentinel, not merely "some error that
	// isn't one of ours" — the weaker form passed for any newly-introduced
	// third sentinel (3.2's review precedent, and this story's Task 4).
	if !errors.Is(err, boom) {
		t.Fatalf("Reveal() err = %v, want the store error propagated unwrapped", err)
	}
	if st.revealCurrentQuestionAndAwardPointsCalls != 0 {
		t.Errorf("RevealCurrentQuestionAndAwardPoints called %d times, want 0 (a failed count must not fall through to the write)", st.revealCurrentQuestionAndAwardPointsCalls)
	}
}

func TestRevealComputesAndPersistsSpeedBonusPoints(t *testing.T) {
	st := questionClosedStub()
	st.game.PointsPerCorrect = 100
	st.game.SpeedBonusFirst = 50
	st.game.SpeedBonusSecond = 30
	st.game.SpeedBonusThird = 10
	answers := []store.AnswerForScoring{
		{AnswerID: "a1", IsCorrect: true, ReceivedAt: testCutoff.Add(-3 * time.Second), Seq: 1},
		{AnswerID: "a2", IsCorrect: true, ReceivedAt: testCutoff.Add(-2 * time.Second), Seq: 2},
		{AnswerID: "a3", IsCorrect: false, ReceivedAt: testCutoff.Add(-1 * time.Second), Seq: 3},
	}
	st.listAnswersForScoringResult = answers
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, _, err := e.Reveal(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Reveal() err = %v, want nil", err)
	}

	// Hard-coded rather than derived from AwardPoints: building want by
	// calling the unit under test made this assertion move in lockstep
	// with the arithmetic it exists to pin.
	want := map[string]int32{
		"a1": 150, // first correct: 100 + SpeedBonusFirst
		"a2": 130, // second correct: 100 + SpeedBonusSecond
		"a3": 0,   // incorrect
	}
	got := pointsByAnswerID(st.revealCurrentQuestionAndAwardPointsArg)
	if len(got) != len(want) {
		t.Fatalf("persisted %d awards (%+v), want %d — every answer must be scored, including incorrect ones (migration 00014: non-NULL means revealed-and-scored)", len(got), st.revealCurrentQuestionAndAwardPointsArg, len(want))
	}
	for id, wantPoints := range want {
		if got[id] != wantPoints {
			t.Errorf("persisted points[%q] = %d, want %d", id, got[id], wantPoints)
		}
	}

	// The scoring read must target the question actually being revealed.
	// Nothing else pins this: the engine passes g.CurrentQuestionPosition,
	// and a hard-coded position passed every test before this assertion.
	if st.listAnswersForScoringGameID != testGameID {
		t.Errorf("ListAnswersForScoring gameID = %q, want %q", st.listAnswersForScoringGameID, testGameID)
	}
	if st.listAnswersForScoringPosition != st.game.CurrentQuestionPosition {
		t.Errorf("ListAnswersForScoring position = %d, want %d (the revealed question's position)", st.listAnswersForScoringPosition, st.game.CurrentQuestionPosition)
	}
}

// --- NextQuestion ---

func TestNextQuestionOpensNextQuestionWhenOneExists(t *testing.T) {
	st := revealedStub()
	st.countAnswersByQuestionResult = 5
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.NextQuestion(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("NextQuestion() err = %v, want nil", err)
	}
	if snap.State != StateQuestionOpen {
		t.Errorf("State = %q, want question_open", snap.State)
	}
	if snap.CurrentQuestion == nil || snap.CurrentQuestion.Position != 2 {
		t.Fatalf("CurrentQuestion = %+v, want position 2", snap.CurrentQuestion)
	}
	if snap.CurrentQuestion.AnsweredCount != 5 {
		t.Errorf("AnsweredCount = %d, want 5", snap.CurrentQuestion.AnsweredCount)
	}
	if st.openNextQuestionCalls != 1 || st.openNextQuestionPosition != 2 {
		t.Errorf("OpenNextQuestion called %d times with position %d, want 1 call at position 2", st.openNextQuestionCalls, st.openNextQuestionPosition)
	}
	if st.finishGameCalls != 0 {
		t.Errorf("FinishGame called %d times, want 0", st.finishGameCalls)
	}
}

func TestNextQuestionFinishesGameWhenNoNextQuestionExists(t *testing.T) {
	st := revealedLastQuestionStub()
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.NextQuestion(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("NextQuestion() err = %v, want nil", err)
	}
	if snap.State != StateFinished {
		t.Errorf("State = %q, want finished", snap.State)
	}
	if snap.CurrentQuestion != nil {
		t.Errorf("CurrentQuestion = %+v, want nil once finished", snap.CurrentQuestion)
	}
	if st.finishGameCalls != 1 {
		t.Errorf("FinishGame called %d times, want 1", st.finishGameCalls)
	}
	if st.openNextQuestionCalls != 0 {
		t.Errorf("OpenNextQuestion called %d times, want 0", st.openNextQuestionCalls)
	}
}

func TestNextQuestionFromNonRevealedReturnsErrNotRevealedWithoutWriting(t *testing.T) {
	st := revealedStub()
	st.game.State = StateQuestionClosed
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.NextQuestion(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotRevealed) {
		t.Fatalf("NextQuestion() err = %v, want ErrNotRevealed", err)
	}
	if st.openNextQuestionCalls != 0 || st.finishGameCalls != 0 {
		t.Errorf("write called (open=%d finish=%d), want 0/0", st.openNextQuestionCalls, st.finishGameCalls)
	}
}

func TestNextQuestionRaceLossReturnsErrNotRevealed(t *testing.T) {
	st := revealedStub()
	st.openNextQuestionErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.NextQuestion(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotRevealed) {
		t.Fatalf("NextQuestion() err = %v, want ErrNotRevealed (race loss reinterpreted)", err)
	}
}

// --- StopGame ---

func TestStopGameAllowedStatesFinishTheGame(t *testing.T) {
	for _, state := range []State{StateQuestionOpen, StateQuestionClosed, StateRevealed} {
		st := &stubStore{
			game:             gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: state, JoinCode: "AB2CD3"},
			finishGameResult: gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateFinished, JoinCode: "AB2CD3"},
		}
		e := NewEngine(st, "+972 50-000-0000", nil)

		snap, err := e.StopGame(context.Background(), testGameID, testOrganizerID)
		if err != nil {
			t.Errorf("StopGame() from %s err = %v, want nil", state, err)
			continue
		}
		if snap.State != StateFinished {
			t.Errorf("StopGame() from %s state = %q, want finished", state, snap.State)
		}
		if snap.CurrentQuestion != nil {
			t.Errorf("StopGame() from %s: CurrentQuestion = %+v, want nil once finished", state, snap.CurrentQuestion)
		}
		if st.finishGameCalls != 1 {
			t.Errorf("StopGame() from %s: FinishGame called %d times, want 1", state, st.finishGameCalls)
		}
	}
}

func TestStopGameDisallowedStatesReturnErrNotStoppableWithoutWriting(t *testing.T) {
	for _, state := range []State{StateDraft, StateLobby, StateFinished} {
		st := &stubStore{game: gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: state, JoinCode: "AB2CD3"}}
		e := NewEngine(st, "+972 50-000-0000", nil)

		_, err := e.StopGame(context.Background(), testGameID, testOrganizerID)
		if !errors.Is(err, ErrNotStoppable) {
			t.Errorf("StopGame() from %s err = %v, want ErrNotStoppable", state, err)
		}
		if st.finishGameCalls != 0 {
			t.Errorf("StopGame() from %s: FinishGame called %d times, want 0", state, st.finishGameCalls)
		}
	}
}

func TestStopGameRaceLossReturnsErrNotStoppable(t *testing.T) {
	st := questionOpenStub()
	st.finishGameErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.StopGame(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotStoppable) {
		t.Fatalf("StopGame() err = %v, want ErrNotStoppable (race loss reinterpreted)", err)
	}
}

func TestStopGameMissingOrForeignGameReturnsErrNotFound(t *testing.T) {
	st := &stubStore{gameErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.StopGame(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("StopGame() err = %v, want store.ErrNotFound", err)
	}
}

// --- PlayerRecipients ---

func TestPlayerRecipientsReturnsOnlyPlayerPhones(t *testing.T) {
	st := &stubStore{participants: []gen.Participant{
		{ID: "p1", GameID: testGameID, Phone: "+972500000001", Role: RolePlayer},
		{ID: "p2", GameID: testGameID, Phone: "+972500000002", Role: RoleSpectator},
		{ID: "p3", GameID: testGameID, Phone: "+972500000003", Role: RolePlayer},
	}}
	e := NewEngine(st, "+972 50-000-0000", nil)

	phones, err := e.PlayerRecipients(context.Background(), testGameID)
	if err != nil {
		t.Fatalf("PlayerRecipients() err = %v, want nil", err)
	}
	want := []string{"+972500000001", "+972500000003"}
	if len(phones) != len(want) || phones[0] != want[0] || phones[1] != want[1] {
		t.Errorf("PlayerRecipients() = %v, want %v", phones, want)
	}
}

func TestPlayerRecipientsAllSpectatorsReturnsEmptyNonNilSlice(t *testing.T) {
	st := &stubStore{participants: []gen.Participant{
		{ID: "p1", GameID: testGameID, Phone: "+972500000001", Role: RoleSpectator},
	}}
	e := NewEngine(st, "+972 50-000-0000", nil)

	phones, err := e.PlayerRecipients(context.Background(), testGameID)
	if err != nil {
		t.Fatalf("PlayerRecipients() err = %v, want nil", err)
	}
	if phones == nil || len(phones) != 0 {
		t.Errorf("PlayerRecipients() = %v, want a non-nil empty slice", phones)
	}
}

func TestPlayerRecipientsStoreErrorPropagates(t *testing.T) {
	wantErr := errors.New("boom")
	st := &stubStore{participantsErr: wantErr}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.PlayerRecipients(context.Background(), testGameID)
	if !errors.Is(err, wantErr) {
		t.Fatalf("PlayerRecipients() err = %v, want %v (propagated, wrapped or not)", err, wantErr)
	}
}
