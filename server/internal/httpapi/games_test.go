package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

const (
	testGameID     = "11111111-2222-3333-4444-555555555555"
	testQuestionID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

// stubGames implements GameStore with canned results and records every
// mutation so tests can assert what reached the store.
type stubGames struct {
	game        gen.Game
	gameErr     error
	rows        []gen.ListGamesByOrganizerRow
	listErr     error
	questions   []gen.Question
	question    gen.Question
	questionErr error
	deleteErr   error
	reorderErr  error

	createdTitles []string
	listedFor     []string
	gotGame       [][2]string
	createdQs     []store.CreateQuestionParams
	updatedQs     []store.UpdateQuestionParams
	deletedQs     [][3]string
	reorders      [][]string
}

func (s *stubGames) CreateGame(ctx context.Context, organizerID, title string) (gen.Game, error) {
	s.createdTitles = append(s.createdTitles, title)
	return s.game, s.gameErr
}

func (s *stubGames) ListGamesByOrganizer(ctx context.Context, organizerID string) ([]gen.ListGamesByOrganizerRow, error) {
	s.listedFor = append(s.listedFor, organizerID)
	return s.rows, s.listErr
}

func (s *stubGames) GetGameForOrganizer(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	s.gotGame = append(s.gotGame, [2]string{gameID, organizerID})
	return s.game, s.gameErr
}

func (s *stubGames) ListQuestionsByGame(ctx context.Context, gameID, organizerID string) ([]gen.Question, error) {
	return s.questions, s.listErr
}

func (s *stubGames) CreateQuestion(ctx context.Context, arg store.CreateQuestionParams) (gen.Question, error) {
	s.createdQs = append(s.createdQs, arg)
	return s.question, s.questionErr
}

func (s *stubGames) UpdateQuestion(ctx context.Context, arg store.UpdateQuestionParams) (gen.Question, error) {
	s.updatedQs = append(s.updatedQs, arg)
	return s.question, s.questionErr
}

func (s *stubGames) DeleteQuestion(ctx context.Context, questionID, gameID, organizerID string) error {
	s.deletedQs = append(s.deletedQs, [3]string{questionID, gameID, organizerID})
	return s.deleteErr
}

func (s *stubGames) ReorderQuestions(ctx context.Context, gameID, organizerID string, orderedIDs []string) error {
	s.reorders = append(s.reorders, orderedIDs)
	return s.reorderErr
}

// noGames is the GameStore for tests that never touch game routes.
func noGames() *stubGames { return &stubGames{} }

// draftGame returns a stub seeded with an owned draft game, the common
// starting state for mutation tests.
func draftGame() *stubGames {
	return &stubGames{game: gen.Game{
		ID:          testGameID,
		OrganizerID: "org-1",
		Title:       "ערב טריוויה",
		JoinCode:    "AB2CD3",
		State:       "draft",
		CreatedAt:   time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
	}}
}

// gamesRouter builds a router with an authenticated org-1 session.
func gamesRouter(games GameStore) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, games, testStatic())
}

// authedRequest carries the session cookie the stubAuth accepts.
func authedRequest(method, target, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	req.AddCookie(&http.Cookie{Name: "wc_session", Value: "raw-token-value"})
	return req
}

func TestCreateGameReturnsJoinCode(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", `{"title":"  ערב טריוויה  "}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/games = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		ID            string          `json:"id"`
		Title         string          `json:"title"`
		JoinCode      string          `json:"joinCode"`
		State         string          `json:"state"`
		QuestionCount int             `json:"questionCount"`
		Questions     json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("create response is not valid JSON: %v", err)
	}
	if body.JoinCode != "AB2CD3" || body.State != "draft" || body.ID != testGameID {
		t.Errorf("create body = %+v, want the stored game with its join code", body)
	}
	if string(body.Questions) != "[]" {
		t.Errorf("questions = %s, want [] (empty array, not null)", body.Questions)
	}
	if len(games.createdTitles) != 1 || games.createdTitles[0] != "ערב טריוויה" {
		t.Errorf("store received titles %v, want the trimmed title", games.createdTitles)
	}
}

func TestCreateGameTitleValidation(t *testing.T) {
	for name, body := range map[string]string{
		"empty":      `{"title":""}`,
		"whitespace": `{"title":"   "}`,
		"too long":   `{"title":"` + strings.Repeat("א", 121) + `"}`,
		"missing":    `{}`,
	} {
		games := draftGame()
		rec := httptest.NewRecorder()
		gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", body))

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: POST /api/games = %d, want 400", name, rec.Code)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "VALIDATION_FAILED" {
			t.Errorf("%s: error code = %q, want VALIDATION_FAILED", name, code)
		}
		if len(games.createdTitles) != 0 {
			t.Errorf("%s: invalid title reached the store", name)
		}
	}
}

func TestCreateGameTitleBoundary120RunesAccepted(t *testing.T) {
	// Hebrew is multi-byte — the limit must count runes, not bytes.
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"title":"` + strings.Repeat("א", 120) + `"}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("120-rune title = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
}

func TestListGamesScopedToSessionOrganizer(t *testing.T) {
	games := &stubGames{rows: []gen.ListGamesByOrganizerRow{{
		ID:            testGameID,
		OrganizerID:   "org-1",
		Title:         "ערב טריוויה",
		JoinCode:      "AB2CD3",
		State:         "draft",
		QuestionCount: 3,
	}}}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/games = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if len(games.listedFor) != 1 || games.listedFor[0] != "org-1" {
		t.Fatalf("list queried for %v, want exactly the session organizer org-1", games.listedFor)
	}
	var body struct {
		Items []struct {
			JoinCode      string `json:"joinCode"`
			QuestionCount int    `json:"questionCount"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("list response is not valid JSON: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].JoinCode != "AB2CD3" || body.Items[0].QuestionCount != 3 {
		t.Errorf("items = %+v, want one game with joinCode AB2CD3 and questionCount 3", body.Items)
	}
}

func TestGetGameIncludesQuestions(t *testing.T) {
	games := draftGame()
	games.questions = []gen.Question{{
		ID:               testQuestionID,
		GameID:           testGameID,
		Position:         1,
		Type:             "mcq",
		Text:             "מה בירת צרפת?",
		Options:          []string{"פריז", "ליון", "מרסיי", "ניס"},
		CorrectOption:    1,
		AcceptedAnswers:  []string{},
		TimeLimitSeconds: 20,
	}}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/"+testGameID, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/games/{id} = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		JoinCode  string `json:"joinCode"`
		Questions []struct {
			Type            string   `json:"type"`
			CorrectOption   int      `json:"correctOption"`
			Options         []string `json:"options"`
			AcceptedAnswers []string `json:"acceptedAnswers"`
		} `json:"questions"`
		QuestionCount int `json:"questionCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("game detail is not valid JSON: %v", err)
	}
	if body.JoinCode != "AB2CD3" || len(body.Questions) != 1 || body.QuestionCount != 1 {
		t.Fatalf("detail = %+v, want the game with its one question", body)
	}
	q := body.Questions[0]
	if q.Type != "mcq" || q.CorrectOption != 1 || len(q.Options) != 4 {
		t.Errorf("question = %+v, want mcq with 4 options and correctOption 1", q)
	}
	if q.AcceptedAnswers != nil {
		t.Errorf("acceptedAnswers = %v, want omitted for mcq (sentinels never reach the wire)", q.AcceptedAnswers)
	}
}

func TestForeignGameReturns404Envelope(t *testing.T) {
	// Ownership scoping (AC-5): a foreign game is indistinguishable from a
	// missing one — 404 GAME_NOT_FOUND, never 403.
	games := &stubGames{gameErr: store.ErrNotFound}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/"+testGameID, ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET foreign game = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
}

func TestMalformedGameIDReturns404WithoutStoreCall(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/not-a-uuid", ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/games/not-a-uuid = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(games.gotGame) != 0 {
		t.Error("malformed id reached the store instead of short-circuiting")
	}
}

func TestGameMutationsWithoutSessionReturn401(t *testing.T) {
	router := NewRouter(stubPinger{}, noAuth(), draftGame(), testStatic())
	for name, req := range map[string]*http.Request{
		"create game":     httptest.NewRequest(http.MethodPost, "/api/games", strings.NewReader(`{"title":"x"}`)),
		"list games":      httptest.NewRequest(http.MethodGet, "/api/games", nil),
		"get game":        httptest.NewRequest(http.MethodGet, "/api/games/"+testGameID, nil),
		"create question": httptest.NewRequest(http.MethodPost, "/api/games/"+testGameID+"/questions", strings.NewReader(`{}`)),
		"delete question": httptest.NewRequest(http.MethodDelete, "/api/games/"+testGameID+"/questions/"+testQuestionID, nil),
		"reorder":         httptest.NewRequest(http.MethodPost, "/api/games/"+testGameID+"/questions/reorder", strings.NewReader(`{}`)),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without session = %d, want 401", name, rec.Code)
		}
	}
}

func TestCreateGameBodyTooLargeReturns413(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"title":"` + strings.Repeat("x", 65<<10) + `"}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", body))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized create-game body = %d, want 413", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "REQUEST_TOO_LARGE" {
		t.Errorf("error code = %q, want REQUEST_TOO_LARGE", code)
	}
}

func TestCreateGameStoreFailureReturns503(t *testing.T) {
	games := &stubGames{gameErr: errors.New("connection refused")}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", `{"title":"ערב"}`))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("create game with dead DB = %d, want 503", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "DB_UNAVAILABLE" {
		t.Errorf("error code = %q, want DB_UNAVAILABLE", code)
	}
}
