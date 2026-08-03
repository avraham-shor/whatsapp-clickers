package store

import (
	"context"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ListParticipants returns a game's participants in join order.
func (s *Store) ListParticipants(ctx context.Context, gameID string) ([]gen.Participant, error) {
	return s.q.ListParticipants(ctx, gameID)
}
