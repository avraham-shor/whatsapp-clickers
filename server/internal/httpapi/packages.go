package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// packagePayload is the wire shape of a Question Bank package card: title,
// count, and preview (A13) — no timestamps, the browse view doesn't render
// them.
type packagePayload struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	QuestionCount int64  `json:"questionCount"`
	Preview       string `json:"preview"`
}

func handleListQuestionPackages(games GameStore) http.HandlerFunc {
	type listResponse struct {
		Items []packagePayload `json:"items"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireOrganizer(w, r); !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		rows, err := games.ListQuestionPackages(ctx)
		if err != nil {
			writeStoreError(w, err, "PACKAGE_NOT_FOUND")
			return
		}
		items := make([]packagePayload, 0, len(rows))
		for _, row := range rows {
			items = append(items, packagePayload{
				ID:            row.ID,
				Title:         row.Title,
				QuestionCount: row.QuestionCount,
				Preview:       row.Preview,
			})
		}
		writeJSON(w, http.StatusOK, listResponse{Items: items})
	}
}

func handleImportPackage(games GameStore) http.HandlerFunc {
	type importRequest struct {
		PackageID string `json:"packageId"`
	}
	type importResponse struct {
		Items []questionPayload `json:"items"`
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
		var req importRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		// Draft/ownership before body validation (review-pinned order): a
		// non-draft or foreign game answers 409/404 regardless of body.
		if _, ok := requireDraftGame(ctx, w, games, gameID, organizerID); !ok {
			return
		}
		// Body IDs fail validation with 400 (the reorder questionIds
		// precedent) — only path params degrade to domain 404s.
		if !isUUID(req.PackageID) {
			writeValidationError(w, "packageId must be a valid question package id")
			return
		}
		questions, err := games.ImportPackageQuestions(ctx, gameID, organizerID, req.PackageID)
		if err != nil {
			writeStoreError(w, err, "PACKAGE_NOT_FOUND")
			return
		}
		items := make([]questionPayload, 0, len(questions))
		for _, question := range questions {
			items = append(items, newQuestionPayload(question))
		}
		slog.Info("package imported", "game_id", gameID, "package_id", req.PackageID, "question_count", len(items))
		writeJSON(w, http.StatusCreated, importResponse{Items: items})
	}
}
