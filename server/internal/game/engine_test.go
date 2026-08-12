package game

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

	// The sixth transition (story 4.5). Zero value is the empty game, which
	// no pre-4.5 test reaches: the call counter is what proves the write
	// never runs from a state that must reject it.
	showLeaderboardResult gen.Game
	showLeaderboardErr    error
	showLeaderboardCalls  int

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

	// The two reveal-only reads (story 4.4). Zero values mean "no rows /
	// 0 correct", so every pre-4.4 test compiles and passes unchanged;
	// the call counters are what prove the reads never run before the
	// reveal.
	listAnswerCountsByResponseResult []gen.ListAnswerCountsByResponseRow
	listAnswerCountsByResponseErr    error
	listAnswerCountsByResponseCalls  int

	countCorrectAnswersByQuestionResult int32
	countCorrectAnswersByQuestionErr    error
	countCorrectAnswersByQuestionCalls  int

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

	updateGameDisplaySettingsResult gen.Game
	updateGameDisplaySettingsErr    error
	updateGameDisplaySettingsCalls  int
	updateGameDisplaySettingsArg    struct {
		gameID        string
		organizerID   string
		reducedMotion bool
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

func (s *stubStore) ShowLeaderboard(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	s.showLeaderboardCalls++
	return s.showLeaderboardResult, s.showLeaderboardErr
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

func (s *stubStore) ListAnswerCountsByResponse(ctx context.Context, questionID string) ([]gen.ListAnswerCountsByResponseRow, error) {
	s.listAnswerCountsByResponseCalls++
	return s.listAnswerCountsByResponseResult, s.listAnswerCountsByResponseErr
}

func (s *stubStore) CountCorrectAnswersByQuestion(ctx context.Context, questionID string) (int32, error) {
	s.countCorrectAnswersByQuestionCalls++
	return s.countCorrectAnswersByQuestionResult, s.countCorrectAnswersByQuestionErr
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

func (s *stubStore) UpdateGameDisplaySettings(ctx context.Context, gameID, organizerID string, reducedMotion bool) (gen.Game, error) {
	s.updateGameDisplaySettingsCalls++
	s.updateGameDisplaySettingsArg.gameID = gameID
	s.updateGameDisplaySettingsArg.organizerID = organizerID
	s.updateGameDisplaySettingsArg.reducedMotion = reducedMotion
	return s.updateGameDisplaySettingsResult, s.updateGameDisplaySettingsErr
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
		// ShowLeaderboard does NOT move current_question_position (story
		// 4.5, derived requirement 4) — the committed row this stub returns
		// carries position 1 for exactly that reason, and the test below
		// asserts it rather than trusting it.
		showLeaderboardResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateLeaderboard, JoinCode: "AB2CD3",
			CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff,
		},
		openNextQuestionResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateQuestionOpen, JoinCode: "AB2CD3",
			CurrentQuestionPosition: 2, AnswerCutoffAt: testCutoff,
		},
	}
}

// leaderboardStub is the Leaderboard pause on question 1, with a second
// question waiting at position 2 — the state story 4.5 made reachable.
func leaderboardStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateLeaderboard, JoinCode: "AB2CD3", CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff}
	return &stubStore{
		game:                      g,
		listQuestionsByGameResult: twoQuestions(),
		openNextQuestionResult: gen.Game{
			ID: testGameID, OrganizerID: testOrganizerID, State: StateQuestionOpen, JoinCode: "AB2CD3",
			CurrentQuestionPosition: 2, AnswerCutoffAt: testCutoff,
		},
		finishGameResult: gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateFinished, JoinCode: "AB2CD3"},
	}
}

// leaderboardLastQuestionStub is the Leaderboard pause on the LAST question:
// the only way out is finishing the game.
func leaderboardLastQuestionStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateLeaderboard, JoinCode: "AB2CD3", CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff}
	return &stubStore{
		game:                      g,
		listQuestionsByGameResult: oneQuestion(),
		finishGameResult:          gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateFinished, JoinCode: "AB2CD3"},
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

// --- The reveal payload (story 4.4) ---

// revealedSnapshotStub is a revealed game whose current question is the
// single mcq at position 1, with correct_option 2 — the read-side stub for
// the reveal payload tests below (revealedStub above is the NextQuestion
// transition's stub and carries a second question).
func revealedSnapshotStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateRevealed, JoinCode: "AB2CD3", CurrentQuestionPosition: 1, AnswerCutoffAt: testCutoff}
	questions := oneQuestion()
	questions[0].CorrectOption = 2
	return &stubStore{game: g, listQuestionsByGameResult: questions}
}

// TestSnapshotBeforeRevealCarriesNoRevealAndReadsNothing is the security
// property the whole nested-payload design rests on (story 4.4, derived
// requirement 5): the correct answer is nowhere on the wire until the state
// is revealed. Asserting only the nil would miss a read that still runs —
// costing a round trip on the hot path and putting the answer in a log line
// — so the call counters are asserted too.
func TestSnapshotBeforeRevealCarriesNoRevealAndReadsNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		st   *stubStore
	}{
		{"question_open", questionOpenStub()},
		{"question_closed", questionClosedStub()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.st.listAnswerCountsByResponseResult = []gen.ListAnswerCountsByResponseRow{{Response: "2", AnswerCount: 7}}
			tc.st.countCorrectAnswersByQuestionResult = 7
			e := NewEngine(tc.st, "+972 50-000-0000", nil)

			snap, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
			if err != nil {
				t.Fatalf("Snapshot() err = %v", err)
			}
			if snap.CurrentQuestion == nil {
				t.Fatal("CurrentQuestion = nil, want the question at position 1")
			}
			if snap.CurrentQuestion.Reveal != nil {
				t.Errorf("Reveal = %+v, want nil before the reveal", snap.CurrentQuestion.Reveal)
			}
			if tc.st.listAnswerCountsByResponseCalls != 0 || tc.st.countCorrectAnswersByQuestionCalls != 0 {
				t.Errorf("reveal reads ran before the reveal: distribution=%d correct=%d, want 0/0",
					tc.st.listAnswerCountsByResponseCalls, tc.st.countCorrectAnswersByQuestionCalls)
			}
		})
	}
}

func TestSnapshotAtRevealedCarriesMCQDistribution(t *testing.T) {
	st := revealedSnapshotStub()
	st.countAnswersByQuestionResult = 10
	st.listAnswerCountsByResponseResult = []gen.ListAnswerCountsByResponseRow{
		{Response: "1", AnswerCount: 2},
		{Response: "2", AnswerCount: 5},
		{Response: "4", AnswerCount: 3},
	}
	st.countCorrectAnswersByQuestionResult = 5
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Snapshot() err = %v", err)
	}
	rev := snap.CurrentQuestion.Reveal
	if rev == nil {
		t.Fatal("Reveal = nil at revealed, want the payload")
	}
	if rev.CorrectOption != 2 {
		t.Errorf("CorrectOption = %d, want 2", rev.CorrectOption)
	}
	if rev.CorrectCount != 5 {
		t.Errorf("CorrectCount = %d, want 5", rev.CorrectCount)
	}
	// Index-aligned with Options, zero-filled for an option nobody chose —
	// so the wire carries [2 5 0 3] and never a short or nil array.
	want := []int{2, 5, 0, 3}
	if len(rev.OptionCounts) != len(want) {
		t.Fatalf("OptionCounts = %v, want %v", rev.OptionCounts, want)
	}
	for i, n := range want {
		if rev.OptionCounts[i] != n {
			t.Errorf("OptionCounts[%d] = %d, want %d", i, rev.OptionCounts[i], n)
		}
	}
	if rev.AcceptedAnswer != "" {
		t.Errorf("AcceptedAnswer = %q on an mcq, want empty", rev.AcceptedAnswer)
	}
}

// TestSnapshotAtRevealedDropsOutOfRangeResponses covers the rows the SQL
// comment says are returned like any other: a response outside
// 1..len(options) must land on NO bar rather than on the wrong one.
func TestSnapshotAtRevealedDropsOutOfRangeResponses(t *testing.T) {
	st := revealedSnapshotStub()
	st.listAnswerCountsByResponseResult = []gen.ListAnswerCountsByResponseRow{
		{Response: "0", AnswerCount: 4},
		{Response: "5", AnswerCount: 6},
		{Response: "ג", AnswerCount: 9},
		{Response: "3", AnswerCount: 1},
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Snapshot() err = %v", err)
	}
	rev := snap.CurrentQuestion.Reveal
	if rev == nil {
		t.Fatal("Reveal = nil at revealed, want the payload")
	}
	want := []int{0, 0, 1, 0}
	if len(rev.OptionCounts) != len(want) {
		t.Fatalf("OptionCounts = %v, want %v", rev.OptionCounts, want)
	}
	for i, n := range want {
		if rev.OptionCounts[i] != n {
			t.Errorf("OptionCounts[%d] = %d, want %d (out-of-range responses must land on no bar)", i, rev.OptionCounts[i], n)
		}
	}
}

func TestSnapshotAtRevealedCarriesFreeTextAcceptedAnswer(t *testing.T) {
	st := revealedSnapshotStub()
	st.listQuestionsByGameResult = []gen.Question{
		{ID: "q1", GameID: testGameID, Position: 1, Type: "free_text", Text: "capital?", AcceptedAnswers: []string{"Jerusalem", "Yerushalayim"}, TimeLimitSeconds: 20},
	}
	st.countCorrectAnswersByQuestionResult = 4
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Snapshot() err = %v", err)
	}
	rev := snap.CurrentQuestion.Reveal
	if rev == nil {
		t.Fatal("Reveal = nil at revealed, want the payload")
	}
	if rev.AcceptedAnswer != "Jerusalem" {
		t.Errorf("AcceptedAnswer = %q, want the FIRST accepted answer", rev.AcceptedAnswer)
	}
	if rev.CorrectCount != 4 {
		t.Errorf("CorrectCount = %d, want 4", rev.CorrectCount)
	}
	// Nil, so omitempty drops the key entirely — matching how Options
	// itself is already absent on a free-text question.
	if rev.OptionCounts != nil {
		t.Errorf("OptionCounts = %v on free_text, want nil", rev.OptionCounts)
	}
	if rev.CorrectOption != 0 {
		t.Errorf("CorrectOption = %d on free_text, want the omitempty-dropped 0", rev.CorrectOption)
	}
	// The distribution read is mcq-only: a free-text question has no bars.
	if st.listAnswerCountsByResponseCalls != 0 {
		t.Errorf("distribution read ran %d times on free_text, want 0", st.listAnswerCountsByResponseCalls)
	}
}

// TestSnapshotRevealReadErrorPropagates mirrors
// TestSnapshotAnsweredCountErrorPropagates: a failed reveal read fails the
// whole build rather than degrading here. The callers that must not fail
// already route through snapshotAfterCommit, which degrades to
// emptySnapshot — the display then renders its waiting copy for a beat,
// which is the documented degraded path (story 4.4, derived requirement 9).
func TestSnapshotRevealReadErrorPropagates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(*stubStore)
	}{
		{"distribution", func(s *stubStore) { s.listAnswerCountsByResponseErr = errors.New("boom") }},
		{"correct count", func(s *stubStore) { s.countCorrectAnswersByQuestionErr = errors.New("boom") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := revealedSnapshotStub()
			tc.apply(st)
			e := NewEngine(st, "+972 50-000-0000", nil)

			if _, err := e.Snapshot(context.Background(), testGameID, testOrganizerID); err == nil {
				t.Fatal("Snapshot() err = nil, want a propagated error")
			}
		})
	}
}

// TestEmptySnapshotCarriesNoReveal states the property rather than adding a
// field to guarantee it: emptySnapshot's CurrentQuestion is nil, so a
// degraded post-commit snapshot at revealed carries no reveal by
// construction. This is the frame derived requirement 9's first guard
// handles.
func TestEmptySnapshotCarriesNoReveal(t *testing.T) {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateRevealed, JoinCode: "AB2CD3"}
	snap := emptySnapshot("+972 50-000-0000", g)
	if snap.State != StateRevealed {
		t.Errorf("State = %q, want revealed", snap.State)
	}
	if snap.CurrentQuestion != nil {
		t.Errorf("CurrentQuestion = %+v, want nil", snap.CurrentQuestion)
	}
}

// TestSnapshotAfterRevealCarriesNoReveal covers the two states on the far side
// of the reveal. Both keep current_question_position > 0, so buildSnapshot
// still constructs a CurrentQuestion — the reveal payload simply vanishes from
// the wire the moment the game moves on. That is intended (the room is looking
// at the leaderboard or the winner, not at the marked answer), but nothing
// asserted it, so a future change to the state gate could take these with it
// unnoticed. The call counters are asserted for the same reason they are
// before the reveal: a read that still runs costs a round trip and puts the
// answer in a log line. (Code review, 2026-08-12.)
func TestSnapshotAfterRevealCarriesNoReveal(t *testing.T) {
	for _, state := range []string{StateLeaderboard, StateFinished} {
		t.Run(state, func(t *testing.T) {
			st := revealedSnapshotStub()
			st.game.State = state
			st.listAnswerCountsByResponseResult = []gen.ListAnswerCountsByResponseRow{{Response: "2", AnswerCount: 7}}
			st.countCorrectAnswersByQuestionResult = 7
			e := NewEngine(st, "+972 50-000-0000", nil)

			snap, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
			if err != nil {
				t.Fatalf("Snapshot() err = %v", err)
			}
			if snap.CurrentQuestion == nil {
				t.Fatal("CurrentQuestion = nil, want the question at position 1")
			}
			if snap.CurrentQuestion.Reveal != nil {
				t.Errorf("Reveal = %+v at %s, want nil — the gate is revealed and nothing else",
					snap.CurrentQuestion.Reveal, state)
			}
			if st.listAnswerCountsByResponseCalls != 0 || st.countCorrectAnswersByQuestionCalls != 0 {
				t.Errorf("reveal reads ran at %s: distribution=%d correct=%d, want 0/0",
					state, st.listAnswerCountsByResponseCalls, st.countCorrectAnswersByQuestionCalls)
			}
		})
	}
}

// TestSnapshotFreeTextWithNoAcceptedAnswersDoesNotPanic exercises
// buildQuestionReveal's len(q.AcceptedAnswers) > 0 guard, which nothing else
// reaches. questions_type_shape enforces cardinality >= 1, so this is a row
// that predates the constraint — the guard exists precisely so such a row
// renders an empty card instead of panicking a live game, and an unasserted
// guard is one a future simplification deletes. (Code review, 2026-08-12.)
func TestSnapshotFreeTextWithNoAcceptedAnswersDoesNotPanic(t *testing.T) {
	st := revealedSnapshotStub()
	st.listQuestionsByGameResult = []gen.Question{
		{ID: "q1", GameID: testGameID, Position: 1, Type: "free_text", Text: "capital?", AcceptedAnswers: nil, TimeLimitSeconds: 20},
	}
	st.countCorrectAnswersByQuestionResult = 2
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("Snapshot() err = %v", err)
	}
	rev := snap.CurrentQuestion.Reveal
	if rev == nil {
		t.Fatal("Reveal = nil at revealed, want the payload")
	}
	if rev.AcceptedAnswer != "" {
		t.Errorf("AcceptedAnswer = %q, want empty", rev.AcceptedAnswer)
	}
	if rev.CorrectCount != 2 {
		t.Errorf("CorrectCount = %d, want 2 — the count is independent of the answer text", rev.CorrectCount)
	}
}

// TestRevealWireContract asserts on the MARSHALLED JSON rather than on the Go
// struct, because the wire is where this story's design actually lives: three
// omitempty decisions plus one deliberate absence of omitempty. Every other
// reveal test reads the struct, where all four are invisible — so before this
// test, dropping `omitempty` from OptionCounts, or adding it to CorrectCount,
// left the entire Go suite green while changing what the display receives.
// The now-deleted E2E was the only thing that ever checked it.
// (Code review, 2026-08-12.)
func TestRevealWireContract(t *testing.T) {
	marshal := func(t *testing.T, st *stubStore) string {
		t.Helper()
		e := NewEngine(st, "+972 50-000-0000", nil)
		snap, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
		if err != nil {
			t.Fatalf("Snapshot() err = %v", err)
		}
		b, err := json.Marshal(snap.CurrentQuestion)
		if err != nil {
			t.Fatalf("json.Marshal() err = %v", err)
		}
		return string(b)
	}

	t.Run("before the reveal the key is present and null", func(t *testing.T) {
		got := marshal(t, questionOpenStub())
		// An absent key and an explicit null both read as null in TS, but the
		// explicit null is what makes a captured frame self-documenting — and
		// it is the property the pointer-without-omitempty exists to give.
		if !strings.Contains(got, `"reveal":null`) {
			t.Errorf("frame = %s, want an explicit \"reveal\":null", got)
		}
		for _, leak := range []string{"correctOption", "acceptedAnswer", "optionCounts", "correctCount"} {
			if strings.Contains(got, leak) {
				t.Errorf("frame leaks %q before the reveal: %s", leak, got)
			}
		}
	})

	t.Run("mcq carries the distribution and a zero correctCount", func(t *testing.T) {
		st := revealedSnapshotStub()
		st.listAnswerCountsByResponseResult = []gen.ListAnswerCountsByResponseRow{{Response: "1", AnswerCount: 2}}
		st.countCorrectAnswersByQuestionResult = 0
		got := marshal(t, st)

		if !strings.Contains(got, `"correctOption":2`) {
			t.Errorf("frame = %s, want correctOption 2", got)
		}
		// Zero-filled and non-nil, so a question nobody chose an option on
		// still carries four bars rather than null.
		if !strings.Contains(got, `"optionCounts":[2,0,0,0]`) {
			t.Errorf("frame = %s, want optionCounts [2,0,0,0]", got)
		}
		// The one field with NO omitempty, and this is why: 0 correct is a real
		// and interesting number, and a dropped key renders as undefined.
		if !strings.Contains(got, `"correctCount":0`) {
			t.Errorf("frame = %s, want an explicit correctCount 0", got)
		}
		if strings.Contains(got, "acceptedAnswer") {
			t.Errorf("frame = %s, want acceptedAnswer omitted on mcq", got)
		}
	})

	t.Run("free_text omits the mcq-only fields entirely", func(t *testing.T) {
		st := revealedSnapshotStub()
		st.listQuestionsByGameResult = []gen.Question{
			{ID: "q1", GameID: testGameID, Position: 1, Type: "free_text", Text: "capital?", AcceptedAnswers: []string{"Jerusalem", "Yerushalayim"}, TimeLimitSeconds: 20},
		}
		st.countCorrectAnswersByQuestionResult = 3
		got := marshal(t, st)

		if !strings.Contains(got, `"acceptedAnswer":"Jerusalem"`) {
			t.Errorf("frame = %s, want the FIRST accepted answer", got)
		}
		if strings.Contains(got, "optionCounts") {
			t.Errorf("frame = %s, want optionCounts absent on free_text", got)
		}
		if strings.Contains(got, "correctOption") {
			t.Errorf("frame = %s, want correctOption absent on free_text (omitempty drops the sentinel 0)", got)
		}
		if !strings.Contains(got, `"correctCount":3`) {
			t.Errorf("frame = %s, want correctCount 3", got)
		}
	})
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

// --- ShowLeaderboard (story 4.5) ---

func TestShowLeaderboardFromRevealedKeepsTheRevealedQuestion(t *testing.T) {
	st := revealedStub()
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.ShowLeaderboard(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("ShowLeaderboard() err = %v, want nil", err)
	}
	if snap.State != StateLeaderboard {
		t.Errorf("State = %q, want leaderboard", snap.State)
	}
	// Derived requirement 4: the Leaderboard is a pause ON the revealed
	// question, and the Audience Display keys its movement baseline on that
	// question's id. A transition that advanced the position would move the
	// baseline key underneath the display and silently erase every arrow.
	if snap.CurrentQuestion == nil {
		t.Fatalf("CurrentQuestion = nil, want the revealed question still resolved")
	}
	if snap.CurrentQuestion.Position != 1 {
		t.Errorf("CurrentQuestion.Position = %d, want 1 (unchanged by the transition)", snap.CurrentQuestion.Position)
	}
	if snap.CurrentQuestion.ID != "q1" {
		t.Errorf("CurrentQuestion.ID = %q, want q1", snap.CurrentQuestion.ID)
	}
	if st.showLeaderboardCalls != 1 {
		t.Errorf("ShowLeaderboard called %d times, want 1", st.showLeaderboardCalls)
	}
	if st.openNextQuestionCalls != 0 || st.finishGameCalls != 0 {
		t.Errorf("other transition writes ran (open=%d finish=%d), want 0/0", st.openNextQuestionCalls, st.finishGameCalls)
	}
}

func TestShowLeaderboardFromNonRevealedReturnsErrNotRevealedWithoutWriting(t *testing.T) {
	for _, state := range []State{StateDraft, StateLobby, StateQuestionOpen, StateQuestionClosed, StateLeaderboard, StateFinished} {
		st := revealedStub()
		st.game.State = state
		e := NewEngine(st, "+972 50-000-0000", nil)

		_, err := e.ShowLeaderboard(context.Background(), testGameID, testOrganizerID)
		if !errors.Is(err, ErrNotRevealed) {
			t.Errorf("ShowLeaderboard() from %s err = %v, want ErrNotRevealed", state, err)
		}
		// The call counter, not only the error: an engine that returned the
		// right error AFTER writing would pass an error-only assertion.
		if st.showLeaderboardCalls != 0 {
			t.Errorf("ShowLeaderboard() from %s: store write called %d times, want 0", state, st.showLeaderboardCalls)
		}
	}
}

func TestShowLeaderboardRaceLossReturnsErrNotRevealed(t *testing.T) {
	st := revealedStub()
	st.showLeaderboardErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.ShowLeaderboard(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, ErrNotRevealed) {
		t.Fatalf("ShowLeaderboard() err = %v, want ErrNotRevealed (race loss reinterpreted)", err)
	}
}

func TestShowLeaderboardMissingOrForeignGameReturnsErrNotFound(t *testing.T) {
	st := &stubStore{gameErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.ShowLeaderboard(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ShowLeaderboard() err = %v, want store.ErrNotFound", err)
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

// The Leaderboard is not a trap: both ways out of it are separate store
// writes, so each needs its own case (story 4.5, derived requirement 5).
func TestNextQuestionFromLeaderboardOpensTheNextQuestion(t *testing.T) {
	st := leaderboardStub()
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.NextQuestion(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("NextQuestion() from leaderboard err = %v, want nil", err)
	}
	if snap.State != StateQuestionOpen {
		t.Errorf("State = %q, want question_open", snap.State)
	}
	if snap.CurrentQuestion == nil || snap.CurrentQuestion.Position != 2 {
		t.Fatalf("CurrentQuestion = %+v, want position 2", snap.CurrentQuestion)
	}
	if st.openNextQuestionCalls != 1 || st.openNextQuestionPosition != 2 {
		t.Errorf("OpenNextQuestion called %d times at position %d, want 1 call at position 2", st.openNextQuestionCalls, st.openNextQuestionPosition)
	}
	if st.finishGameCalls != 0 {
		t.Errorf("FinishGame called %d times, want 0", st.finishGameCalls)
	}
}

func TestNextQuestionFromLeaderboardOnLastQuestionFinishesTheGame(t *testing.T) {
	st := leaderboardLastQuestionStub()
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.NextQuestion(context.Background(), testGameID, testOrganizerID)
	if err != nil {
		t.Fatalf("NextQuestion() from leaderboard err = %v, want nil", err)
	}
	if snap.State != StateFinished {
		t.Errorf("State = %q, want finished", snap.State)
	}
	if st.finishGameCalls != 1 {
		t.Errorf("FinishGame called %d times, want 1", st.finishGameCalls)
	}
	if st.openNextQuestionCalls != 0 {
		t.Errorf("OpenNextQuestion called %d times, want 0", st.openNextQuestionCalls)
	}
}

// The negative control that makes the two cases above meaningful: widening
// NextQuestion's guard to admit leaderboard must not admit anything else.
// question_open is the sharp one — an organizer who double-clicks past a
// live question would skip it entirely.
func TestNextQuestionFromNonRevealedReturnsErrNotRevealedWithoutWriting(t *testing.T) {
	for _, state := range []State{StateDraft, StateLobby, StateQuestionOpen, StateQuestionClosed, StateFinished} {
		st := revealedStub()
		st.game.State = state
		e := NewEngine(st, "+972 50-000-0000", nil)

		_, err := e.NextQuestion(context.Background(), testGameID, testOrganizerID)
		if !errors.Is(err, ErrNotRevealed) {
			t.Errorf("NextQuestion() from %s err = %v, want ErrNotRevealed", state, err)
		}
		if st.openNextQuestionCalls != 0 || st.finishGameCalls != 0 {
			t.Errorf("NextQuestion() from %s: write called (open=%d finish=%d), want 0/0", state, st.openNextQuestionCalls, st.finishGameCalls)
		}
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
	// StateLeaderboard joined this list in story 4.5: it is the state that
	// story made reachable, and an organizer standing on the Leaderboard
	// with no working "עצור" would be stuck in front of a room.
	for _, state := range []State{StateQuestionOpen, StateQuestionClosed, StateRevealed, StateLeaderboard} {
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

func TestSetDisplaySettingsPersistsAndSnapshots(t *testing.T) {
	st := draftStub()
	st.updateGameDisplaySettingsResult = gen.Game{
		ID: testGameID, OrganizerID: testOrganizerID, State: StateLobby,
		JoinCode: "AB2CD3", ReducedMotion: true,
	}
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.SetDisplaySettings(context.Background(), testGameID, testOrganizerID, true)
	if err != nil {
		t.Fatalf("SetDisplaySettings() err = %v, want nil", err)
	}
	if !snap.DisplaySettings.ReducedMotion {
		t.Errorf("snapshot.DisplaySettings = %+v, want ReducedMotion true", snap.DisplaySettings)
	}
	if snap.State != StateLobby || snap.GameID != testGameID {
		t.Errorf("snapshot = %+v, want the written row's state/id carried through", snap)
	}
	if st.updateGameDisplaySettingsCalls != 1 {
		t.Errorf("UpdateGameDisplaySettings called %d times, want 1", st.updateGameDisplaySettingsCalls)
	}
	arg := st.updateGameDisplaySettingsArg
	if arg.gameID != testGameID || arg.organizerID != testOrganizerID || !arg.reducedMotion {
		t.Errorf("UpdateGameDisplaySettings arg = %+v, want (%s, %s, true)", arg, testGameID, testOrganizerID)
	}
}

func TestSetDisplaySettingsStoreErrorPropagates(t *testing.T) {
	st := draftStub()
	st.updateGameDisplaySettingsErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.SetDisplaySettings(context.Background(), testGameID, testOrganizerID, true)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("SetDisplaySettings() err = %v, want store.ErrNotFound", err)
	}
	// Snapshot holds slices, so compare the fields that would be populated
	// on success rather than the struct itself.
	if snap.GameID != "" || snap.State != "" || snap.Participants != nil || snap.Leaderboard != nil {
		t.Errorf("snapshot = %+v, want the zero Snapshot on error", snap)
	}
}

// SetDisplaySettings is the one write path that must NOT degrade a failed
// snapshot build to emptySnapshot. Its caller broadcasts what it returns,
// and emptySnapshot blanks the roster, the current question and the
// leaderboard — so degrading here would let a cosmetic toggle wipe the
// room's screen, including on a finished game that never re-broadcasts to
// self-correct. Nothing committed but one boolean, so the error is the
// honest answer. (Code review, 2026-08-09.)
func TestSetDisplaySettingsDoesNotDegradeOnBuildFailure(t *testing.T) {
	st := draftStub()
	st.updateGameDisplaySettingsResult = gen.Game{
		ID: testGameID, OrganizerID: testOrganizerID, State: StateLobby,
		JoinCode: "AB2CD3", ReducedMotion: true,
	}
	boom := errors.New("questions unavailable")
	st.listQuestionsByGameErr = boom
	e := NewEngine(st, "+972 50-000-0000", nil)

	snap, err := e.SetDisplaySettings(context.Background(), testGameID, testOrganizerID, true)
	if !errors.Is(err, boom) {
		t.Fatalf("SetDisplaySettings() err = %v, want the build error propagated (never a degraded snapshot)", err)
	}
	if snap.GameID != "" || snap.State != "" {
		t.Errorf("snapshot = %+v, want the zero Snapshot — a degraded emptySnapshot here would be broadcast to the room", snap)
	}
	if st.updateGameDisplaySettingsCalls != 1 {
		t.Errorf("UpdateGameDisplaySettings called %d times, want 1 (the write still commits; only the snapshot fails)", st.updateGameDisplaySettingsCalls)
	}
}

// The degraded fallback must not silently flip the room's motion setting
// back to animated: emptySnapshot carries reduced_motion from the row for
// the same reason it carries State/JoinCode. This is the only test that
// can catch that one line going missing.
func TestEmptySnapshotCarriesDisplaySettings(t *testing.T) {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateLobby, JoinCode: "AB2CD3", ReducedMotion: true}

	snap := emptySnapshot("+972 50-000-0000", g)

	if !snap.DisplaySettings.ReducedMotion {
		t.Errorf("emptySnapshot DisplaySettings = %+v, want ReducedMotion true carried from the row", snap.DisplaySettings)
	}
}
