package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// GameStore is the games/questions surface handlers depend on; *store.Store
// satisfies it.
type GameStore interface {
	CreateGame(ctx context.Context, organizerID, title string) (gen.Game, error)
	ListGamesByOrganizer(ctx context.Context, organizerID string) ([]gen.ListGamesByOrganizerRow, error)
	GetGameForOrganizer(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	ListQuestionsByGame(ctx context.Context, gameID, organizerID string) ([]gen.Question, error)
	CreateQuestion(ctx context.Context, arg store.CreateQuestionParams) (gen.Question, error)
	UpdateQuestion(ctx context.Context, arg store.UpdateQuestionParams) (gen.Question, error)
	DeleteQuestion(ctx context.Context, questionID, gameID, organizerID string) error
	ReorderQuestions(ctx context.Context, gameID, organizerID string, orderedIDs []string) error
	UpdateGameScoring(ctx context.Context, arg store.UpdateGameScoringParams) (gen.Game, error)
}

// gameStateDraft is the only state in which a game's content is editable.
const gameStateDraft = "draft"

// maxTitleRunes bounds the game title (runes, not bytes — titles are Hebrew).
const maxTitleRunes = 120

// Scoring bounds: 0 is meaningful (a zero bonus is disabled, per AC); the cap
// mirrors the DB CHECK and keeps cumulative int32 scores safe.
const (
	minScoringValue = 0
	maxScoringValue = 10000
)

// gamePayload is the wire shape of a game (camelCase, direct payload).
// Scoring fields carry no omitempty: 0 is a meaningful value (a disabled
// bonus) and must serialize.
type gamePayload struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	JoinCode         string    `json:"joinCode"`
	State            string    `json:"state"`
	QuestionCount    int64     `json:"questionCount"`
	PointsPerCorrect int32     `json:"pointsPerCorrect"`
	SpeedBonusFirst  int32     `json:"speedBonusFirst"`
	SpeedBonusSecond int32     `json:"speedBonusSecond"`
	SpeedBonusThird  int32     `json:"speedBonusThird"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// gameDetailPayload embeds the questions so one fetch renders the whole
// editor (documented composition decision; lists stay {"items":...}).
type gameDetailPayload struct {
	gamePayload
	Questions []questionPayload `json:"questions"`
}

func newGamePayload(game gen.Game, questionCount int64) gamePayload {
	return gamePayload{
		ID:               game.ID,
		Title:            game.Title,
		JoinCode:         game.JoinCode,
		State:            game.State,
		QuestionCount:    questionCount,
		PointsPerCorrect: game.PointsPerCorrect,
		SpeedBonusFirst:  game.SpeedBonusFirst,
		SpeedBonusSecond: game.SpeedBonusSecond,
		SpeedBonusThird:  game.SpeedBonusThird,
		CreatedAt:        game.CreatedAt.UTC(),
		UpdatedAt:        game.UpdatedAt.UTC(),
	}
}

func newGameDetailPayload(game gen.Game, questions []gen.Question) gameDetailPayload {
	// Non-nil so the wire always carries [], never null.
	items := make([]questionPayload, 0, len(questions))
	for _, question := range questions {
		items = append(items, newQuestionPayload(question))
	}
	return gameDetailPayload{
		gamePayload: newGamePayload(game, int64(len(questions))),
		Questions:   items,
	}
}

// requireOrganizer extracts the authenticated organizer; RequireOrganizer
// middleware guarantees it, but guard anyway (handleMe precedent).
func requireOrganizer(w http.ResponseWriter, r *http.Request) (string, bool) {
	org, ok := OrganizerFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "valid session required")
		return "", false
	}
	return org.ID, true
}

// gameIDParam validates the {gameID} path parameter; a malformed ID is a
// 404 GAME_NOT_FOUND (indistinguishable from a missing game), not a DB error.
func gameIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	gameID := chi.URLParam(r, "gameID")
	if !isUUID(gameID) {
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "no such game for this organizer")
		return "", false
	}
	return gameID, true
}

// requireDraftGame loads the organizer's game and enforces the draft-only
// mutation guardrail; a false return means the response is written.
func requireDraftGame(ctx context.Context, w http.ResponseWriter, games GameStore, gameID, organizerID string) (gen.Game, bool) {
	game, err := games.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		writeStoreError(w, err, "GAME_NOT_FOUND")
		return gen.Game{}, false
	}
	if game.State != gameStateDraft {
		writeError(w, http.StatusConflict, "GAME_NOT_EDITABLE", "game content can only be edited while in draft state")
		return gen.Game{}, false
	}
	return game, true
}

func handleCreateGame(games GameStore) http.HandlerFunc {
	type createGameRequest struct {
		Title string `json:"title"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		var req createGameRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		title := strings.TrimSpace(req.Title)
		if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleRunes {
			writeValidationError(w, "title must be 1-120 characters")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		game, err := games.CreateGame(ctx, organizerID, title)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		slog.Info("game created", "game_id", game.ID, "organizer_id", organizerID)
		writeJSON(w, http.StatusCreated, newGameDetailPayload(game, nil))
	}
}

func handleListGames(games GameStore) http.HandlerFunc {
	type listResponse struct {
		Items []gamePayload `json:"items"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		rows, err := games.ListGamesByOrganizer(ctx, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		items := make([]gamePayload, 0, len(rows))
		for _, row := range rows {
			items = append(items, newGamePayload(gen.Game{
				ID:               row.ID,
				OrganizerID:      row.OrganizerID,
				Title:            row.Title,
				JoinCode:         row.JoinCode,
				State:            row.State,
				PointsPerCorrect: row.PointsPerCorrect,
				SpeedBonusFirst:  row.SpeedBonusFirst,
				SpeedBonusSecond: row.SpeedBonusSecond,
				SpeedBonusThird:  row.SpeedBonusThird,
				CreatedAt:        row.CreatedAt,
				UpdatedAt:        row.UpdatedAt,
			}, row.QuestionCount))
		}
		writeJSON(w, http.StatusOK, listResponse{Items: items})
	}
}

func handleGetGame(games GameStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		game, err := games.GetGameForOrganizer(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		questions, err := games.ListQuestionsByGame(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		writeJSON(w, http.StatusOK, newGameDetailPayload(game, questions))
	}
}

// scoringPayload is the direct payload of the scoring sub-resource — PUT
// returns what was stored. No omitempty: 0 means a disabled bonus.
type scoringPayload struct {
	PointsPerCorrect int32 `json:"pointsPerCorrect"`
	SpeedBonusFirst  int32 `json:"speedBonusFirst"`
	SpeedBonusSecond int32 `json:"speedBonusSecond"`
	SpeedBonusThird  int32 `json:"speedBonusThird"`
}

func handleUpdateScoring(games GameStore) http.HandlerFunc {
	// All four fields are pointers and required: with plain int32 an omitted
	// field silently decodes to 0, and 0 is meaningful (zero disables a
	// bonus) — absence must be an explicit 400, never an accidental zero.
	type scoringRequest struct {
		PointsPerCorrect *int32 `json:"pointsPerCorrect"`
		SpeedBonusFirst  *int32 `json:"speedBonusFirst"`
		SpeedBonusSecond *int32 `json:"speedBonusSecond"`
		SpeedBonusThird  *int32 `json:"speedBonusThird"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		var req scoringRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		// Draft/ownership before body validation (handleUpdateQuestion order):
		// a foreign or non-draft game answers 404/409 regardless of body.
		if _, ok := requireDraftGame(ctx, w, games, gameID, organizerID); !ok {
			return
		}
		for _, field := range []struct {
			name  string
			value *int32
		}{
			{"pointsPerCorrect", req.PointsPerCorrect},
			{"speedBonusFirst", req.SpeedBonusFirst},
			{"speedBonusSecond", req.SpeedBonusSecond},
			{"speedBonusThird", req.SpeedBonusThird},
		} {
			if field.value == nil {
				writeValidationError(w, field.name+" is required")
				return
			}
			if *field.value < minScoringValue || *field.value > maxScoringValue {
				writeValidationError(w, field.name+" must be between 0 and 10000")
				return
			}
		}
		game, err := games.UpdateGameScoring(ctx, store.UpdateGameScoringParams{
			GameID:           gameID,
			OrganizerID:      organizerID,
			PointsPerCorrect: *req.PointsPerCorrect,
			SpeedBonusFirst:  *req.SpeedBonusFirst,
			SpeedBonusSecond: *req.SpeedBonusSecond,
			SpeedBonusThird:  *req.SpeedBonusThird,
		})
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		slog.Info("game scoring updated", "game_id", gameID, "organizer_id", organizerID)
		writeJSON(w, http.StatusOK, scoringPayload{
			PointsPerCorrect: game.PointsPerCorrect,
			SpeedBonusFirst:  game.SpeedBonusFirst,
			SpeedBonusSecond: game.SpeedBonusSecond,
			SpeedBonusThird:  game.SpeedBonusThird,
		})
	}
}
