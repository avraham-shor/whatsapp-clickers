package game

import (
	"context"
	"errors"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

const testPhone = "972501234567"

// joinStub builds a stubStore whose GetGameByJoinCode returns a game in the
// given state, wired for a successful CreateParticipant + buildSnapshot by
// default. Individual tests override fields for their own scenario.
func joinStub(state string) *stubStore {
	return &stubStore{
		getGameByJoinCodeResult:  gen.Game{ID: testGameID, State: state, JoinCode: "AB2CD3"},
		createParticipantResult:  gen.Participant{ID: "p1", GameID: testGameID, Role: RolePlayer},
		createParticipantCreated: true,
		participants:             []gen.Participant{{ID: "p1", GameID: testGameID, DisplayName: "דנה"}},
	}
}

func TestJoinFromLobbyFirstJoinReturnsWelcomeWithSnapshot(t *testing.T) {
	st := joinStub(StateLobby)
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if err != nil {
		t.Fatalf("Join() err = %v, want nil", err)
	}
	if result.Outcome != JoinWelcome {
		t.Errorf("Outcome = %q, want JoinWelcome", result.Outcome)
	}
	if !result.Created {
		t.Error("Created = false, want true (first join)")
	}
	if result.DisplayName != "דנה" {
		t.Errorf("DisplayName = %q, want the profile name", result.DisplayName)
	}
	if result.Snapshot.GameID == "" {
		t.Error("Snapshot.GameID is empty, want it populated on a first join")
	}
	if result.Snapshot.ParticipantCount != 1 {
		t.Errorf("Snapshot.ParticipantCount = %d, want 1", result.Snapshot.ParticipantCount)
	}
	if st.createParticipantCalls != 1 {
		t.Errorf("CreateParticipant called %d times, want 1", st.createParticipantCalls)
	}
	if st.createParticipantRole != RolePlayer {
		t.Errorf("CreateParticipant role = %q, want RolePlayer", st.createParticipantRole)
	}
	if len(st.createParticipantAllowedStates) != 1 || st.createParticipantAllowedStates[0] != StateLobby {
		t.Errorf("CreateParticipant allowedStates = %v, want [lobby]", st.createParticipantAllowedStates)
	}
}

// TestJoinLobbyRaceLossDegradesToGenericFailure covers the check-then-write
// race this guard closes (deferred from 2.4, closed by story 3.1's code
// review): a lobby->question_open transition landing between Join's state
// read and CreateParticipant's write makes the guarded insert match zero
// rows. With no pre-existing participant row, that surfaces as
// store.ErrNotFound — Join propagates it unchanged, and wa/inbound.go's
// default branch already degrades any unrecognized Join error to the
// generic Help reply, so no new outcome/error type is needed here.
func TestJoinLobbyRaceLossDegradesToGenericFailure(t *testing.T) {
	st := joinStub(StateLobby)
	st.createParticipantErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Join() err = %v, want store.ErrNotFound (race loss)", err)
	}
}

func TestJoinFromLobbyEmptyProfileNameFallsBackToPhoneDigits(t *testing.T) {
	st := joinStub(StateLobby)
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.Join(context.Background(), "AB2CD3", testPhone, "")
	if err != nil {
		t.Fatalf("Join() err = %v, want nil", err)
	}
	if want := fallbackParticipantName(testPhone); result.DisplayName != want {
		t.Errorf("DisplayName = %q, want fallback %q", result.DisplayName, want)
	}
}

func TestJoinFromLobbyIdempotentRepeatSkipsBroadcast(t *testing.T) {
	st := joinStub(StateLobby)
	st.createParticipantCreated = false
	// Distinct from the resent profile name below: on a real conflict the
	// store's ON CONFLICT DO NOTHING + refetch returns the EXISTING row's
	// stored name, not whatever this call tried to write (AC-3).
	st.createParticipantResult.DisplayName = "דנה הישנה"
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if err != nil {
		t.Fatalf("Join() err = %v, want nil", err)
	}
	if result.Outcome != JoinWelcome {
		t.Errorf("Outcome = %q, want JoinWelcome even on a repeat", result.Outcome)
	}
	if result.Created {
		t.Error("Created = true, want false (idempotent repeat)")
	}
	if result.DisplayName != "דנה הישנה" {
		t.Errorf("DisplayName = %q, want the existing stored name %q, not the resent profile name", result.DisplayName, "דנה הישנה")
	}
	if result.Snapshot.GameID != "" {
		t.Errorf("Snapshot = %+v, want the zero value (nothing changed, nothing to broadcast)", result.Snapshot)
	}
}

func TestJoinUnknownCodeReturnsErrInvalidJoinCodeWithoutWriting(t *testing.T) {
	st := &stubStore{getGameByJoinCodeErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Join(context.Background(), "NOPE99", testPhone, "דנה")
	if !errors.Is(err, ErrInvalidJoinCode) {
		t.Fatalf("Join() err = %v, want ErrInvalidJoinCode", err)
	}
	if st.createParticipantCalls != 0 {
		t.Errorf("CreateParticipant called %d times, want 0", st.createParticipantCalls)
	}
}

func TestJoinDraftGameReturnsPreLobbyWithoutWriting(t *testing.T) {
	st := joinStub(StateDraft)
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if err != nil {
		t.Fatalf("Join() err = %v, want nil", err)
	}
	if result.Outcome != JoinPreLobby {
		t.Errorf("Outcome = %q, want JoinPreLobby", result.Outcome)
	}
	if result.GameID != testGameID {
		t.Errorf("GameID = %q, want %q", result.GameID, testGameID)
	}
	if result.DisplayName != "" {
		t.Errorf("DisplayName = %q, want empty (not registered)", result.DisplayName)
	}
	if st.createParticipantCalls != 0 {
		t.Errorf("CreateParticipant called %d times, want 0", st.createParticipantCalls)
	}
}

func TestJoinDuringLiveStatesRegistersSpectator(t *testing.T) {
	states := []string{StateQuestionOpen, StateQuestionClosed, StateRevealed, StateLeaderboard}
	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			st := joinStub(state)
			e := NewEngine(st, "+972 50-000-0000", nil)

			result, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
			if err != nil {
				t.Fatalf("Join() err = %v, want nil", err)
			}
			if result.Outcome != JoinSpectator {
				t.Errorf("Outcome = %q, want JoinSpectator", result.Outcome)
			}
			if !result.Created {
				t.Error("Created = false, want true (first join)")
			}
			if result.DisplayName != "דנה" {
				t.Errorf("DisplayName = %q, want the profile name", result.DisplayName)
			}
			if result.Snapshot.GameID != "" {
				t.Errorf("Snapshot = %+v, want the zero value (spectator join never broadcasts)", result.Snapshot)
			}
			if st.createParticipantCalls != 1 {
				t.Errorf("CreateParticipant called %d times, want 1", st.createParticipantCalls)
			}
			if st.createParticipantRole != RoleSpectator {
				t.Errorf("CreateParticipant role = %q, want RoleSpectator", st.createParticipantRole)
			}
			if len(st.createParticipantAllowedStates) != len(spectatorAllowedStates) {
				t.Errorf("CreateParticipant allowedStates = %v, want %v", st.createParticipantAllowedStates, spectatorAllowedStates)
			}
		})
	}
}

// TestJoinSpectatorRaceLossDegradesToGenericFailure covers the check-then-
// write race this guard closes (deferred from 2.5, closed by story 3.1's
// code review): a live->finished transition landing between Join's state
// read and CreateParticipant's write makes the guarded insert match zero
// rows, which — with no pre-existing row — surfaces as store.ErrNotFound
// rather than silently registering a spectator into a finished game.
func TestJoinSpectatorRaceLossDegradesToGenericFailure(t *testing.T) {
	st := joinStub(StateRevealed)
	st.createParticipantErr = store.ErrNotFound
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Join() err = %v, want store.ErrNotFound (race loss)", err)
	}
}

func TestJoinSpectatorEmptyProfileNameFallsBackToPhoneDigits(t *testing.T) {
	st := joinStub(StateQuestionOpen)
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.Join(context.Background(), "AB2CD3", testPhone, "")
	if err != nil {
		t.Fatalf("Join() err = %v, want nil", err)
	}
	if want := fallbackParticipantName(testPhone); result.DisplayName != want {
		t.Errorf("DisplayName = %q, want fallback %q", result.DisplayName, want)
	}
}

func TestJoinSpectatorIdempotentRepeat(t *testing.T) {
	st := joinStub(StateQuestionOpen)
	st.createParticipantCreated = false
	// Distinct from the resent profile name below: on a real conflict the
	// store's ON CONFLICT DO NOTHING + refetch returns the EXISTING row's
	// stored name, not whatever this call tried to write (AC-3).
	st.createParticipantResult.DisplayName = "דנה הישנה"
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if err != nil {
		t.Fatalf("Join() err = %v, want nil", err)
	}
	if result.Outcome != JoinSpectator {
		t.Errorf("Outcome = %q, want JoinSpectator even on a repeat", result.Outcome)
	}
	if result.Created {
		t.Error("Created = true, want false (idempotent repeat)")
	}
	if result.DisplayName != "דנה הישנה" {
		t.Errorf("DisplayName = %q, want the existing stored name %q, not the resent profile name", result.DisplayName, "דנה הישנה")
	}
	if result.Snapshot.GameID != "" {
		t.Errorf("Snapshot = %+v, want the zero value (nothing changed, nothing to broadcast)", result.Snapshot)
	}
}

func TestJoinFinishedGameReturnsErrGameFinishedWithoutWriting(t *testing.T) {
	st := joinStub(StateFinished)
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if !errors.Is(err, ErrGameFinished) {
		t.Fatalf("Join() err = %v, want ErrGameFinished", err)
	}
	if st.createParticipantCalls != 0 {
		t.Errorf("CreateParticipant called %d times, want 0", st.createParticipantCalls)
	}
}

func TestJoinUnknownStateFailsClosedWithoutWriting(t *testing.T) {
	st := joinStub("cancelled")
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if err == nil {
		t.Fatal("Join() err = nil, want an unexpected-state error")
	}
	if errors.Is(err, ErrGameFinished) || errors.Is(err, ErrInvalidJoinCode) {
		t.Fatalf("Join() err = %v, want a plain unexpected-state error, not a known sentinel", err)
	}
	if st.createParticipantCalls != 0 {
		t.Errorf("CreateParticipant called %d times, want 0", st.createParticipantCalls)
	}
}

func TestJoinDegradesToSkippedBroadcastWhenSnapshotBuildFailsAfterCommit(t *testing.T) {
	st := joinStub(StateLobby)
	st.participantsErr = errors.New("boom")
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.Join(context.Background(), "AB2CD3", testPhone, "דנה")
	if err != nil {
		t.Fatalf("Join() err = %v, want nil (degrade, not fail — the participant row already committed)", err)
	}
	if !result.Created {
		t.Error("Created = false, want true (the insert itself succeeded)")
	}
	if result.Snapshot.GameID != "" {
		t.Errorf("Snapshot = %+v, want the zero value (broadcast skipped, not a wrong count)", result.Snapshot)
	}
}

func TestRenameSuccessUpdatesDisplayName(t *testing.T) {
	st := &stubStore{updateParticipantNameByPhoneResult: gen.Participant{DisplayName: "דוד"}}
	e := NewEngine(st, "+972 50-000-0000", nil)

	result, err := e.Rename(context.Background(), testPhone, "דוד")
	if err != nil {
		t.Fatalf("Rename() err = %v, want nil", err)
	}
	if result.DisplayName != "דוד" {
		t.Errorf("DisplayName = %q, want %q", result.DisplayName, "דוד")
	}
	if st.updateParticipantNameByPhoneCalls != 1 {
		t.Errorf("UpdateParticipantNameByPhone called %d times, want 1", st.updateParticipantNameByPhoneCalls)
	}
	if st.updateParticipantNameByPhonePhone != testPhone || st.updateParticipantNameByPhoneName != "דוד" {
		t.Errorf("UpdateParticipantNameByPhone called with (%q, %q), want (%q, %q)",
			st.updateParticipantNameByPhonePhone, st.updateParticipantNameByPhoneName, testPhone, "דוד")
	}
}

func TestRenameUnregisteredPhoneReturnsErrParticipantNotFound(t *testing.T) {
	st := &stubStore{updateParticipantNameByPhoneErr: store.ErrNotFound}
	e := NewEngine(st, "+972 50-000-0000", nil)

	_, err := e.Rename(context.Background(), testPhone, "דוד")
	if !errors.Is(err, ErrParticipantNotFound) {
		t.Fatalf("Rename() err = %v, want ErrParticipantNotFound", err)
	}
}
