package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

const testPackageID = "11111111-1111-4111-8111-111111111111"

const importPath = "/api/games/" + testGameID + "/questions/import-package"

func validImportBody() string {
	return `{"packageId":"` + testPackageID + `"}`
}

func TestListQuestionPackages(t *testing.T) {
	games := noGames()
	games.packages = []gen.ListQuestionPackagesRow{{
		ID:            testPackageID,
		Title:         "טריוויה לכל המשפחה",
		QuestionCount: 10,
		Preview:       "איזו חיה היא הגבוהה ביותר בעולם?",
	}}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/question-packages", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/question-packages = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		Items []struct {
			ID            string `json:"id"`
			Title         string `json:"title"`
			QuestionCount int    `json:"questionCount"`
			Preview       string `json:"preview"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("packages response is not valid JSON: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %+v, want exactly one package", body.Items)
	}
	item := body.Items[0]
	if item.ID != testPackageID || item.Title != "טריוויה לכל המשפחה" ||
		item.QuestionCount != 10 || item.Preview != "איזו חיה היא הגבוהה ביותר בעולם?" {
		t.Errorf("item = %+v, want id/title/questionCount/preview from the store row", item)
	}
	// Timestamps stay off the wire — the browse view doesn't render them.
	if strings.Contains(rec.Body.String(), "createdAt") {
		t.Errorf("packages payload carries timestamps: %s", rec.Body)
	}
}

func TestListQuestionPackagesEmptyBankReturnsEmptyItems(t *testing.T) {
	games := noGames()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/question-packages", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET empty bank = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("packages response is not valid JSON: %v", err)
	}
	if string(body.Items) != "[]" {
		t.Errorf("items = %s, want [] (empty array, not null)", body.Items)
	}
}

func TestPackageRoutesWithoutSessionReturn401(t *testing.T) {
	router := NewRouter(stubPinger{}, noAuth(), draftGame(), testStatic(), nil, nil, nil, nil, nil)
	for name, req := range map[string]*http.Request{
		"list packages":  httptest.NewRequest(http.MethodGet, "/api/question-packages", nil),
		"import package": httptest.NewRequest(http.MethodPost, importPath, strings.NewReader(validImportBody())),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without session = %d, want 401", name, rec.Code)
		}
	}
}

func TestImportPackageHappyPath(t *testing.T) {
	games := draftGame()
	// Two existing questions → copies appended at positions 3 and 4, returned
	// in position order with the provenance flag set.
	games.importedQs = []gen.Question{
		{
			ID:               "aaaaaaaa-0000-0000-0000-000000000001",
			GameID:           testGameID,
			Position:         3,
			Type:             "mcq",
			Text:             "מי מהדמויות הבאות אינו אחד משלושת האבות?",
			Options:          []string{"אברהם", "יצחק", "יעקב", "משה"},
			CorrectOption:    4,
			AcceptedAnswers:  []string{},
			TimeLimitSeconds: 20,
			ImportedFromBank: true,
		},
		{
			ID:               "aaaaaaaa-0000-0000-0000-000000000002",
			GameID:           testGameID,
			Position:         4,
			Type:             "free_text",
			Text:             "על איזה הר ניתנה התורה?",
			Options:          []string{},
			AcceptedAnswers:  []string{"הר סיני", "סיני"},
			TimeLimitSeconds: 25,
			ImportedFromBank: true,
		},
	}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, importPath, validImportBody()))

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST import-package = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	if len(games.imports) != 1 || games.imports[0] != [3]string{testGameID, "org-1", testPackageID} {
		t.Fatalf("store received imports %v, want (gameID, organizerID, packageID)", games.imports)
	}
	var body struct {
		Items []struct {
			Position         int  `json:"position"`
			ImportedFromBank bool `json:"importedFromBank"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("import response is not valid JSON: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("items = %+v, want the two copied questions", body.Items)
	}
	if body.Items[0].Position != 3 || body.Items[1].Position != 4 {
		t.Errorf("positions = %d,%d, want 3,4 in position order", body.Items[0].Position, body.Items[1].Position)
	}
	if !body.Items[0].ImportedFromBank || !body.Items[1].ImportedFromBank {
		t.Errorf("items = %+v, want importedFromBank true on every copy", body.Items)
	}
}

func TestImportUnknownPackageReturns404(t *testing.T) {
	games := draftGame()
	games.importErr = store.ErrNotFound
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, importPath, validImportBody()))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("import unknown package = %d, want 404 (body %s)", rec.Code, rec.Body)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "PACKAGE_NOT_FOUND" {
		t.Errorf("error code = %q, want PACKAGE_NOT_FOUND", code)
	}
}

func TestImportMalformedPackageIDReturns400WithoutStoreCall(t *testing.T) {
	// Body IDs fail validation with 400 (the reorder questionIds precedent),
	// unlike path params which degrade to domain 404s.
	for name, body := range map[string]string{
		"malformed": `{"packageId":"not-a-uuid"}`,
		"missing":   `{}`,
	} {
		games := draftGame()
		rec := httptest.NewRecorder()
		gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, importPath, body))

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s packageId = %d, want 400 (body %s)", name, rec.Code, rec.Body)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "VALIDATION_FAILED" {
			t.Errorf("%s: error code = %q, want VALIDATION_FAILED", name, code)
		}
		var envelope struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("%s: error response is not valid JSON: %v", name, err)
		}
		if !strings.Contains(envelope.Error.Message, "packageId") {
			t.Errorf("%s: message = %q, want it to name the packageId field", name, envelope.Error.Message)
		}
		if len(games.imports) != 0 {
			t.Errorf("%s: invalid packageId reached the store", name)
		}
	}
}

func TestImportOnNonDraftGameReturns409(t *testing.T) {
	// The draft check precedes packageId validation (the 1.4 review
	// regression class): a non-draft game answers 409 even with a garbage
	// packageId. Invalid JSON still 400s at decode, which precedes everything.
	for name, body := range map[string]string{
		"valid body":          validImportBody(),
		"malformed packageId": `{"packageId":"not-a-uuid"}`,
	} {
		games := draftGame()
		games.game.State = "lobby"
		rec := httptest.NewRecorder()
		gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, importPath, body))

		if rec.Code != http.StatusConflict {
			t.Errorf("%s on non-draft game = %d, want 409 (body %s)", name, rec.Code, rec.Body)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_EDITABLE" {
			t.Errorf("%s: error code = %q, want GAME_NOT_EDITABLE", name, code)
		}
		if len(games.imports) != 0 {
			t.Errorf("%s: non-draft import reached the store", name)
		}
	}
}

func TestImportOnForeignGameReturns404(t *testing.T) {
	games := &stubGames{gameErr: store.ErrNotFound}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, importPath, validImportBody()))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("import on foreign game = %d, want 404 (body %s)", rec.Code, rec.Body)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(games.imports) != 0 {
		t.Error("foreign-game import reached the store")
	}
}

func TestImportMalformedGameIDReturns404WithoutStoreCall(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/not-a-uuid/questions/import-package", validImportBody()))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("import with malformed game id = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(games.gotGame) != 0 || len(games.imports) != 0 {
		t.Error("malformed game id reached the store instead of short-circuiting")
	}
}

func TestImportInvalidJSONReturns400(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, importPath, `{not json`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON import body = %d, want 400", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "INVALID_REQUEST" {
		t.Errorf("error code = %q, want INVALID_REQUEST", code)
	}
	if len(games.imports) != 0 {
		t.Error("invalid JSON reached the store")
	}
}

func TestImportBodyTooLargeReturns413(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"packageId":"` + strings.Repeat("x", 65<<10) + `"}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, importPath, body))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized import body = %d, want 413", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "REQUEST_TOO_LARGE" {
		t.Errorf("error code = %q, want REQUEST_TOO_LARGE", code)
	}
}
