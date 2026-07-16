package store

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ListQuestionPackages returns every Question Bank package with its question
// count and preview line. The bank is first-party and shared — no ownership
// scoping.
func (s *Store) ListQuestionPackages(ctx context.Context) ([]gen.ListQuestionPackagesRow, error) {
	return s.q.ListQuestionPackages(ctx)
}

// ImportPackageQuestions copies a package's questions into the organizer's
// game, appended after the existing questions in package order. A missing
// package is ErrNotFound; a missing or foreign game copies nothing (the
// ownership WHERE) and returns an empty slice with a nil error — ownership
// and existence are enforced by the handler's requireDraftGame, which runs
// first. No explicit transaction: both statements are single atomic
// queries and packages have no delete path; the MAX+1 concurrency class is
// the same documented deferred item as CreateQuestion.
func (s *Store) ImportPackageQuestions(ctx context.Context, gameID, organizerID, packageID string) ([]gen.Question, error) {
	if _, err := s.q.GetQuestionPackage(ctx, packageID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	questions, err := s.q.ImportPackageQuestions(ctx, gen.ImportPackageQuestionsParams{
		PackageID:   packageID,
		GameID:      gameID,
		OrganizerID: organizerID,
	})
	if err != nil {
		return nil, err
	}
	// RETURNING * row order is not guaranteed — return in position order.
	sort.Slice(questions, func(i, j int) bool {
		return questions[i].Position < questions[j].Position
	})
	return questions, nil
}
