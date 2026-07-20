package store

import "context"

// MarkWaMessageProcessed records a WhatsApp message ID as delivered,
// returning true on the first delivery and false when the ID was already
// present (a Meta retry). The underlying INSERT ... ON CONFLICT DO NOTHING
// is one atomic statement, so concurrent deliveries of the same message ID
// cannot race past each other.
func (s *Store) MarkWaMessageProcessed(ctx context.Context, waMessageID string) (bool, error) {
	rows, err := s.q.MarkWaMessageProcessed(ctx, waMessageID)
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}
