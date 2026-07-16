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

const (
	questionTypeMCQ      = "mcq"
	questionTypeFreeText = "free_text"

	// defaultTimeLimitSeconds is the A8 default; the UI pre-fills it and
	// carries the 30–45s accessibility guidance.
	defaultTimeLimitSeconds = 20

	maxQuestionTextRunes   = 500
	maxOptionRunes         = 200
	mcqOptionCount         = 4
	maxAcceptedAnswers     = 20
	maxAcceptedAnswerRunes = 200
	minTimeLimitSeconds    = 5
	maxTimeLimitSeconds    = 300
)

// questionPayload is the wire shape of a question. Fields irrelevant to the
// type are omitted — the DB sentinels ('{}', 0) never reach the wire.
type questionPayload struct {
	ID               string   `json:"id"`
	Position         int32    `json:"position"`
	Type             string   `json:"type"`
	Text             string   `json:"text"`
	Options          []string `json:"options,omitempty"`
	CorrectOption    int32    `json:"correctOption,omitempty"`
	AcceptedAnswers  []string `json:"acceptedAnswers,omitempty"`
	TimeLimitSeconds int32    `json:"timeLimitSeconds"`
	// No omitempty: always on the wire so the TS type keeps a non-optional
	// boolean (provenance badge marker, false for custom questions).
	ImportedFromBank bool      `json:"importedFromBank"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func newQuestionPayload(question gen.Question) questionPayload {
	return questionPayload{
		ID:               question.ID,
		Position:         question.Position,
		Type:             question.Type,
		Text:             question.Text,
		Options:          question.Options,
		CorrectOption:    question.CorrectOption,
		AcceptedAnswers:  question.AcceptedAnswers,
		TimeLimitSeconds: question.TimeLimitSeconds,
		ImportedFromBank: question.ImportedFromBank,
		CreatedAt:        question.CreatedAt.UTC(),
		UpdatedAt:        question.UpdatedAt.UTC(),
	}
}

// questionRequest is the create/update body. TimeLimitSeconds is a pointer
// so an omitted field falls back to the A8 default instead of failing.
type questionRequest struct {
	Type             string   `json:"type"`
	Text             string   `json:"text"`
	Options          []string `json:"options"`
	CorrectOption    int32    `json:"correctOption"`
	AcceptedAnswers  []string `json:"acceptedAnswers"`
	TimeLimitSeconds *int32   `json:"timeLimitSeconds"`
}

// validatedQuestion carries the trimmed, normalized result of validation.
// Options and AcceptedAnswers are always non-nil: both columns are NOT NULL
// and a nil slice would bind as SQL NULL.
type validatedQuestion struct {
	Type             string
	Text             string
	Options          []string
	CorrectOption    int32
	AcceptedAnswers  []string
	TimeLimitSeconds int32
}

// validateQuestion trims and checks a question body against the boundary
// rules (trim first). A non-empty message names the failing field and maps
// to 400 VALIDATION_FAILED; DB CHECKs are the last line, not the first.
func validateQuestion(req questionRequest) (validatedQuestion, string) {
	v := validatedQuestion{Options: []string{}, AcceptedAnswers: []string{}}
	if req.Type != questionTypeMCQ && req.Type != questionTypeFreeText {
		return v, "type must be mcq or free_text"
	}
	v.Type = req.Type
	v.Text = strings.TrimSpace(req.Text)
	if n := utf8.RuneCountInString(v.Text); n < 1 || n > maxQuestionTextRunes {
		return v, "text must be 1-500 characters"
	}
	v.TimeLimitSeconds = defaultTimeLimitSeconds
	if req.TimeLimitSeconds != nil {
		v.TimeLimitSeconds = *req.TimeLimitSeconds
	}
	if v.TimeLimitSeconds < minTimeLimitSeconds || v.TimeLimitSeconds > maxTimeLimitSeconds {
		return v, "timeLimitSeconds must be between 5 and 300"
	}
	switch v.Type {
	case questionTypeMCQ:
		if len(req.AcceptedAnswers) != 0 {
			return v, "acceptedAnswers must be empty for mcq questions"
		}
		if len(req.Options) != mcqOptionCount {
			return v, "options must contain exactly 4 entries"
		}
		for _, option := range req.Options {
			trimmed := strings.TrimSpace(option)
			if n := utf8.RuneCountInString(trimmed); n < 1 || n > maxOptionRunes {
				return v, "options entries must be 1-200 characters"
			}
			v.Options = append(v.Options, trimmed)
		}
		if req.CorrectOption < 1 || req.CorrectOption > mcqOptionCount {
			return v, "correctOption must be between 1 and 4"
		}
		v.CorrectOption = req.CorrectOption
	case questionTypeFreeText:
		if len(req.Options) != 0 {
			return v, "options must be empty for free_text questions"
		}
		if req.CorrectOption != 0 {
			return v, "correctOption must be omitted for free_text questions"
		}
		if len(req.AcceptedAnswers) < 1 || len(req.AcceptedAnswers) > maxAcceptedAnswers {
			return v, "acceptedAnswers must contain 1-20 entries"
		}
		for _, answer := range req.AcceptedAnswers {
			trimmed := strings.TrimSpace(answer)
			if n := utf8.RuneCountInString(trimmed); n < 1 || n > maxAcceptedAnswerRunes {
				return v, "acceptedAnswers entries must be 1-200 characters"
			}
			v.AcceptedAnswers = append(v.AcceptedAnswers, trimmed)
		}
	}
	return v, ""
}

// questionIDParam validates the {questionID} path parameter; malformed IDs
// are 404 QUESTION_NOT_FOUND, mirroring gameIDParam.
func questionIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	questionID := chi.URLParam(r, "questionID")
	if !isUUID(questionID) {
		writeError(w, http.StatusNotFound, "QUESTION_NOT_FOUND", "no such question in this game")
		return "", false
	}
	return questionID, true
}

func handleCreateQuestion(games GameStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		var req questionRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, ok := requireDraftGame(ctx, w, games, gameID, organizerID); !ok {
			return
		}
		v, msg := validateQuestion(req)
		if msg != "" {
			writeValidationError(w, msg)
			return
		}
		question, err := games.CreateQuestion(ctx, store.CreateQuestionParams{
			GameID:           gameID,
			OrganizerID:      organizerID,
			Type:             v.Type,
			Text:             v.Text,
			Options:          v.Options,
			CorrectOption:    v.CorrectOption,
			AcceptedAnswers:  v.AcceptedAnswers,
			TimeLimitSeconds: v.TimeLimitSeconds,
		})
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		slog.Info("question created", "game_id", gameID, "question_id", question.ID)
		writeJSON(w, http.StatusCreated, newQuestionPayload(question))
	}
}

func handleUpdateQuestion(games GameStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		questionID, ok := questionIDParam(w, r)
		if !ok {
			return
		}
		var req questionRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, ok := requireDraftGame(ctx, w, games, gameID, organizerID); !ok {
			return
		}
		v, msg := validateQuestion(req)
		if msg != "" {
			writeValidationError(w, msg)
			return
		}
		// Type is the immutability key: it sits in the UPDATE's WHERE, so a
		// type-mismatched edit matches no row and lands here as a 404.
		question, err := games.UpdateQuestion(ctx, store.UpdateQuestionParams{
			ID:               questionID,
			GameID:           gameID,
			OrganizerID:      organizerID,
			Type:             v.Type,
			Text:             v.Text,
			Options:          v.Options,
			CorrectOption:    v.CorrectOption,
			AcceptedAnswers:  v.AcceptedAnswers,
			TimeLimitSeconds: v.TimeLimitSeconds,
		})
		if err != nil {
			writeStoreError(w, err, "QUESTION_NOT_FOUND")
			return
		}
		slog.Info("question updated", "game_id", gameID, "question_id", questionID)
		writeJSON(w, http.StatusOK, newQuestionPayload(question))
	}
}

func handleDeleteQuestion(games GameStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		questionID, ok := questionIDParam(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, ok := requireDraftGame(ctx, w, games, gameID, organizerID); !ok {
			return
		}
		if err := games.DeleteQuestion(ctx, questionID, gameID, organizerID); err != nil {
			writeStoreError(w, err, "QUESTION_NOT_FOUND")
			return
		}
		slog.Info("question deleted", "game_id", gameID, "question_id", questionID)
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleReorderQuestions(games GameStore) http.HandlerFunc {
	type reorderRequest struct {
		QuestionIDs []string `json:"questionIds"`
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
		var req reorderRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		for _, id := range req.QuestionIDs {
			if !isUUID(id) {
				writeValidationError(w, "questionIds must contain valid question ids")
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, ok := requireDraftGame(ctx, w, games, gameID, organizerID); !ok {
			return
		}
		// The exact-permutation check runs inside the store's transaction —
		// ErrReorderMismatch maps to 400 in writeStoreError.
		if err := games.ReorderQuestions(ctx, gameID, organizerID, req.QuestionIDs); err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		slog.Info("questions reordered", "game_id", gameID)
		w.WriteHeader(http.StatusNoContent)
	}
}
