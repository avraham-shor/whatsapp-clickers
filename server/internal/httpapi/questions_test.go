package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

const questionsPath = "/api/games/" + testGameID + "/questions"

func validMCQBody() string {
	return `{"type":"mcq","text":"מה בירת צרפת?","options":["פריז","ליון","מרסיי","ניס"],"correctOption":1,"timeLimitSeconds":20}`
}

func validFreeTextBody() string {
	return `{"type":"free_text","text":"מי כתב את התנ\"ך?","acceptedAnswers":["משה","משה רבנו"],"timeLimitSeconds":30}`
}

func TestCreateMCQQuestion(t *testing.T) {
	games := draftGame()
	games.question = gen.Question{
		ID:               testQuestionID,
		GameID:           testGameID,
		Position:         1,
		Type:             "mcq",
		Text:             "מה בירת צרפת?",
		Options:          []string{"פריז", "ליון", "מרסיי", "ניס"},
		CorrectOption:    1,
		AcceptedAnswers:  []string{},
		TimeLimitSeconds: 20,
	}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath, validMCQBody()))

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST questions = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	if len(games.createdQs) != 1 {
		t.Fatalf("store received %d creates, want 1", len(games.createdQs))
	}
	created := games.createdQs[0]
	if created.Type != "mcq" || created.CorrectOption != 1 || len(created.Options) != 4 {
		t.Errorf("store params = %+v, want the validated mcq", created)
	}
	if created.AcceptedAnswers == nil || len(created.AcceptedAnswers) != 0 {
		t.Errorf("acceptedAnswers = %#v, want empty non-nil sentinel (NOT NULL column)", created.AcceptedAnswers)
	}
	var body struct {
		Type            string   `json:"type"`
		AcceptedAnswers []string `json:"acceptedAnswers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("question response is not valid JSON: %v", err)
	}
	if body.AcceptedAnswers != nil {
		t.Errorf("mcq response carries acceptedAnswers %v, want omitted", body.AcceptedAnswers)
	}
}

func TestCreateFreeTextQuestion(t *testing.T) {
	games := draftGame()
	games.question = gen.Question{
		ID:               testQuestionID,
		GameID:           testGameID,
		Position:         1,
		Type:             "free_text",
		Text:             "שאלה",
		Options:          []string{},
		AcceptedAnswers:  []string{"משה", "משה רבנו"},
		TimeLimitSeconds: 30,
	}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath, validFreeTextBody()))

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST free_text question = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	created := games.createdQs[0]
	if created.Type != "free_text" || len(created.AcceptedAnswers) != 2 || created.CorrectOption != 0 {
		t.Errorf("store params = %+v, want free_text with 2 accepted answers", created)
	}
	if created.Options == nil || len(created.Options) != 0 {
		t.Errorf("options = %#v, want empty non-nil sentinel (NOT NULL column)", created.Options)
	}
	var body struct {
		Options       []string `json:"options"`
		CorrectOption int      `json:"correctOption"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("question response is not valid JSON: %v", err)
	}
	if body.Options != nil || body.CorrectOption != 0 {
		t.Errorf("free_text response carries mcq fields %+v, want omitted", body)
	}
}

func TestCreateQuestionDefaultsTimeLimitTo20(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"type":"mcq","text":"שאלה","options":["א","ב","ג","ד"],"correctOption":2}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath, body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("question without timeLimitSeconds = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	if got := games.createdQs[0].TimeLimitSeconds; got != 20 {
		t.Errorf("timeLimitSeconds = %d, want the A8 default 20", got)
	}
}

func TestCreateQuestionValidationBoundaries(t *testing.T) {
	for name, body := range map[string]string{
		"mcq 3 options":           `{"type":"mcq","text":"ש","options":["א","ב","ג"],"correctOption":1}`,
		"mcq 5 options":           `{"type":"mcq","text":"ש","options":["א","ב","ג","ד","ה"],"correctOption":1}`,
		"mcq empty option":        `{"type":"mcq","text":"ש","options":["א","ב","ג","  "],"correctOption":1}`,
		"correctOption 0":         `{"type":"mcq","text":"ש","options":["א","ב","ג","ד"],"correctOption":0}`,
		"correctOption 5":         `{"type":"mcq","text":"ש","options":["א","ב","ג","ד"],"correctOption":5}`,
		"mcq with answers":        `{"type":"mcq","text":"ש","options":["א","ב","ג","ד"],"correctOption":1,"acceptedAnswers":["x"]}`,
		"free_text no answers":    `{"type":"free_text","text":"ש","acceptedAnswers":[]}`,
		"free_text 21 answers":    `{"type":"free_text","text":"ש","acceptedAnswers":[` + strings.TrimSuffix(strings.Repeat(`"ת",`, 21), ",") + `]}`,
		"free_text blank answer":  `{"type":"free_text","text":"ש","acceptedAnswers":["  "]}`,
		"free_text with options":  `{"type":"free_text","text":"ש","acceptedAnswers":["ת"],"options":["א"]}`,
		"free_text correctOption": `{"type":"free_text","text":"ש","acceptedAnswers":["ת"],"correctOption":1}`,
		"bad type":                `{"type":"essay","text":"ש"}`,
		"empty text":              `{"type":"mcq","text":"  ","options":["א","ב","ג","ד"],"correctOption":1}`,
		"text 501 runes":          `{"type":"mcq","text":"` + strings.Repeat("א", 501) + `","options":["א","ב","ג","ד"],"correctOption":1}`,
		"option 201 runes":        `{"type":"mcq","text":"ש","options":["` + strings.Repeat("א", 201) + `","ב","ג","ד"],"correctOption":1}`,
		"answer 201 runes":        `{"type":"free_text","text":"ש","acceptedAnswers":["` + strings.Repeat("א", 201) + `"]}`,
		"timeLimit 4":             `{"type":"mcq","text":"ש","options":["א","ב","ג","ד"],"correctOption":1,"timeLimitSeconds":4}`,
		"timeLimit 301":           `{"type":"mcq","text":"ש","options":["א","ב","ג","ד"],"correctOption":1,"timeLimitSeconds":301}`,
	} {
		games := draftGame()
		rec := httptest.NewRecorder()
		gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath, body))

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: POST questions = %d, want 400 (body %s)", name, rec.Code, rec.Body)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "VALIDATION_FAILED" {
			t.Errorf("%s: error code = %q, want VALIDATION_FAILED", name, code)
		}
		if len(games.createdQs) != 0 {
			t.Errorf("%s: invalid question reached the store", name)
		}
	}
}

func TestCreateQuestionTimeLimitBoundariesAccepted(t *testing.T) {
	for _, limit := range []int{5, 300} {
		games := draftGame()
		rec := httptest.NewRecorder()
		body := fmt.Sprintf(`{"type":"mcq","text":"ש","options":["א","ב","ג","ד"],"correctOption":1,"timeLimitSeconds":%d}`, limit)
		gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath, body))

		if rec.Code != http.StatusCreated {
			t.Errorf("timeLimitSeconds %d = %d, want 201 (body %s)", limit, rec.Code, rec.Body)
		}
	}
}

func TestQuestionMutationOnNonDraftGameReturns409(t *testing.T) {
	games := draftGame()
	games.game.State = "lobby"
	for name, req := range map[string]*http.Request{
		"create":  authedRequest(http.MethodPost, questionsPath, validMCQBody()),
		"update":  authedRequest(http.MethodPut, questionsPath+"/"+testQuestionID, validMCQBody()),
		"delete":  authedRequest(http.MethodDelete, questionsPath+"/"+testQuestionID, ""),
		"reorder": authedRequest(http.MethodPost, questionsPath+"/reorder", `{"questionIds":[]}`),
	} {
		rec := httptest.NewRecorder()
		gamesRouter(games).ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("%s on non-draft game = %d, want 409", name, rec.Code)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_EDITABLE" {
			t.Errorf("%s: error code = %q, want GAME_NOT_EDITABLE", name, code)
		}
	}
}

func TestUpdateQuestion(t *testing.T) {
	games := draftGame()
	games.question = gen.Question{ID: testQuestionID, Type: "mcq"}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, questionsPath+"/"+testQuestionID, validMCQBody()))

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT question = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if len(games.updatedQs) != 1 {
		t.Fatalf("store received %d updates, want 1", len(games.updatedQs))
	}
	updated := games.updatedQs[0]
	if updated.ID != testQuestionID || updated.GameID != testGameID || updated.OrganizerID != "org-1" {
		t.Errorf("update params = %+v, want ids scoped to question/game/organizer", updated)
	}
	if updated.Type != "mcq" {
		t.Errorf("update type = %q, want mcq (immutability key in WHERE)", updated.Type)
	}
}

func TestUpdateMissingQuestionReturns404(t *testing.T) {
	games := draftGame()
	games.questionErr = store.ErrNotFound
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, questionsPath+"/"+testQuestionID, validMCQBody()))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT missing question = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "QUESTION_NOT_FOUND" {
		t.Errorf("error code = %q, want QUESTION_NOT_FOUND", code)
	}
}

func TestDeleteQuestionReturns204(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodDelete, questionsPath+"/"+testQuestionID, ""))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE question = %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	if len(games.deletedQs) != 1 || games.deletedQs[0] != [3]string{testQuestionID, testGameID, "org-1"} {
		t.Errorf("delete params = %v, want question/game/organizer ids", games.deletedQs)
	}
}

func TestDeleteMissingQuestionReturns404(t *testing.T) {
	games := draftGame()
	games.deleteErr = store.ErrNotFound
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodDelete, questionsPath+"/"+testQuestionID, ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE missing question = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "QUESTION_NOT_FOUND" {
		t.Errorf("error code = %q, want QUESTION_NOT_FOUND", code)
	}
}

func TestReorderHappyPathPersistsOrder(t *testing.T) {
	games := draftGame()
	otherID := "99999999-8888-7777-6666-555555555555"
	rec := httptest.NewRecorder()
	body := `{"questionIds":["` + otherID + `","` + testQuestionID + `"]}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath+"/reorder", body))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST reorder = %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	if len(games.reorders) != 1 {
		t.Fatalf("store received %d reorders, want 1", len(games.reorders))
	}
	if got := games.reorders[0]; got[0] != otherID || got[1] != testQuestionID {
		t.Errorf("reorder ids = %v, want the requested order preserved", got)
	}
}

func TestReorderMismatchReturns400(t *testing.T) {
	// Missing/extra/duplicate IDs are detected transactionally in the store
	// and must surface as a 400 validation failure, not a 500.
	games := draftGame()
	games.reorderErr = store.ErrReorderMismatch
	rec := httptest.NewRecorder()
	body := `{"questionIds":["` + testQuestionID + `"]}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath+"/reorder", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched reorder = %d, want 400", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "VALIDATION_FAILED" {
		t.Errorf("error code = %q, want VALIDATION_FAILED", code)
	}
}

func TestReorderWithMalformedIDReturns400WithoutStoreCall(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"questionIds":["not-a-uuid"]}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath+"/reorder", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reorder with malformed id = %d, want 400", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "VALIDATION_FAILED" {
		t.Errorf("error code = %q, want VALIDATION_FAILED", code)
	}
	if len(games.reorders) != 0 {
		t.Error("malformed question id reached the store")
	}
}

func TestQuestionBodyTooLargeReturns413(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"type":"mcq","text":"` + strings.Repeat("x", 65<<10) + `"}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath, body))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized question body = %d, want 413", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "REQUEST_TOO_LARGE" {
		t.Errorf("error code = %q, want REQUEST_TOO_LARGE", code)
	}
}

func TestCreateQuestionOnForeignGameReturns404(t *testing.T) {
	games := &stubGames{gameErr: store.ErrNotFound}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, questionsPath, validMCQBody()))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("create question on foreign game = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(games.createdQs) != 0 {
		t.Error("question create reached the store despite foreign game")
	}
}
