package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// SessionTTL is fixed at 30 days for the pilot; it bounds both the DB row
// and the cookie Max-Age.
const SessionTTL = 30 * 24 * time.Hour

// ErrInvalidCredentials covers unknown-username and wrong-password alike so
// responses cannot enumerate accounts.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrNoSession means the token does not resolve to a live session.
var ErrNoSession = errors.New("no valid session")

// ErrTokenGeneration marks a crypto/rand failure while minting a session
// token — an internal fault, not a database one, so handlers can map it
// to 500 instead of 503.
var ErrTokenGeneration = errors.New("generate session token")

// Organizer is the authenticated identity handed to HTTP handlers.
type Organizer struct {
	ID       string
	Username string
}

// Store is the narrow persistence surface auth needs; *store.Store satisfies it.
type Store interface {
	GetOrganizerByUsername(ctx context.Context, username string) (gen.Organizer, error)
	CreateSession(ctx context.Context, organizerID, tokenHash string, expiresAt time.Time) (gen.Session, error)
	GetSessionOrganizer(ctx context.Context, tokenHash string) (gen.Organizer, error)
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context) error
}

// dummyHash is verified against when the username is unknown, so both login
// failure modes cost one argon2id derivation (no enumeration by timing).
var dummyHash = func() string {
	h, err := HashPassword("timing-equalizer-not-a-real-account")
	if err != nil {
		panic(fmt.Sprintf("auth: compute dummy hash: %v", err))
	}
	return h
}()

// Service implements login, session authentication, and logout.
type Service struct {
	store  Store
	secret []byte
}

// NewService builds a Service; sessionSecret is the validated SESSION_SECRET.
func NewService(st Store, sessionSecret string) *Service {
	return &Service{store: st, secret: []byte(sessionSecret)}
}

// hashToken derives the DB-side key for a raw cookie token. Storing only
// hex(HMAC-SHA256(secret, token)) means a read-only DB leak cannot forge or
// replay sessions, and rotating the secret invalidates them all at once.
func (s *Service) hashToken(rawToken string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(rawToken))
	return hex.EncodeToString(mac.Sum(nil))
}

// Login verifies credentials and mints a session. The returned token is the
// raw cookie value (base64url); only its HMAC touches the database.
func (s *Service) Login(ctx context.Context, username, password string) (string, Organizer, error) {
	org, err := s.store.GetOrganizerByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Unknown usernames still pay for one argon2id verify —
			// uniform timing with the wrong-password path.
			_, _ = VerifyPassword(password, dummyHash)
			return "", Organizer{}, ErrInvalidCredentials
		}
		return "", Organizer{}, fmt.Errorf("look up organizer: %w", err)
	}
	ok, err := VerifyPassword(password, org.PasswordHash)
	if err != nil || !ok {
		return "", Organizer{}, ErrInvalidCredentials
	}

	// Lazy housekeeping (no cron in pilot); failure must not block login,
	// but it must leave a signal for the operator.
	if err := s.store.DeleteExpiredSessions(ctx); err != nil {
		slog.Warn("expired-session cleanup failed", "error", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", Organizer{}, fmt.Errorf("%w: %v", ErrTokenGeneration, err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if _, err := s.store.CreateSession(ctx, org.ID, s.hashToken(token), time.Now().Add(SessionTTL)); err != nil {
		return "", Organizer{}, fmt.Errorf("create session: %w", err)
	}
	return token, Organizer{ID: org.ID, Username: org.Username}, nil
}

// Authenticate resolves a raw cookie token to its organizer. Expiry is
// enforced by the SQL lookup, so it survives server restarts unchanged.
func (s *Service) Authenticate(ctx context.Context, token string) (Organizer, error) {
	org, err := s.store.GetSessionOrganizer(ctx, s.hashToken(token))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Organizer{}, ErrNoSession
		}
		return Organizer{}, fmt.Errorf("look up session: %w", err)
	}
	return Organizer{ID: org.ID, Username: org.Username}, nil
}

// Logout deletes the session row server-side; unknown tokens are a no-op.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSessionByTokenHash(ctx, s.hashToken(token))
}
