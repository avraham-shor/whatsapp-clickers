package game

import (
	"context"
	"errors"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

const (
	testGameID      = "11111111-2222-3333-4444-555555555555"
	testOrganizerID = "org-1"
)

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

	createParticipantResult  gen.Participant
	createParticipantCreated bool
	createParticipantErr     error
	createParticipantCalls   int
	createParticipantRole    string

	updateParticipantNameByPhoneResult gen.Participant
	updateParticipantNameByPhoneErr    error
	updateParticipantNameByPhoneCalls  int
	updateParticipantNameByPhonePhone  string
	updateParticipantNameByPhoneName   string
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

func (s *stubStore) CreateParticipant(ctx context.Context, gameID, phone, displayName, role string) (gen.Participant, bool, error) {
	s.createParticipantCalls++
	s.createParticipantRole = role
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

func draftStub() *stubStore {
	g := gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateDraft, JoinCode: "AB2CD3"}
	return &stubStore{game: g, openGameLobbyResult: gen.Game{ID: testGameID, OrganizerID: testOrganizerID, State: StateLobby, JoinCode: "AB2CD3"}}
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
	if st.openGameLobbyCalls != 1 {
		t.Errorf("OpenGameLobby called %d times, want 1", st.openGameLobbyCalls)
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

func TestSnapshotMissingOrForeignGameReturnsErrNotFound(t *testing.T) {
	st := &stubStore{gameErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Snapshot(context.Background(), testGameID, testOrganizerID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Snapshot() err = %v, want store.ErrNotFound", err)
	}
}
