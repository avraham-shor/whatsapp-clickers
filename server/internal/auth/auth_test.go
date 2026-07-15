package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

func TestHashVerifyRoundtrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash %q does not carry the expected PHC prefix", hash)
	}
	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword rejected the original password")
	}
}

func TestVerifyWrongPasswordFails(t *testing.T) {
	hash, err := HashPassword("right-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("VerifyPassword accepted a wrong password")
	}
}

func TestVerifyTamperedHashFailsCleanly(t *testing.T) {
	for name, phc := range map[string]string{
		"empty":          "",
		"not phc":        "plainly-not-a-hash",
		"wrong algo":     "$argon2i$v=19$m=19456,t=2,p=1$c2FsdHNhbHQ$a2V5a2V5",
		"bad params":     "$argon2id$v=19$m=abc,t=2,p=1$c2FsdHNhbHQ$a2V5a2V5",
		"bad salt b64":   "$argon2id$v=19$m=19456,t=2,p=1$!!!$a2V5a2V5",
		"bad key b64":    "$argon2id$v=19$m=19456,t=2,p=1$c2FsdHNhbHQ$!!!",
		"missing fields": "$argon2id$v=19$m=19456,t=2,p=1$c2FsdHNhbHQ",
	} {
		ok, err := VerifyPassword("whatever", phc)
		if err == nil {
			t.Errorf("%s: VerifyPassword returned no error for tampered hash %q", name, phc)
		}
		if ok {
			t.Errorf("%s: VerifyPassword accepted tampered hash %q", name, phc)
		}
	}
}

func TestVerifyDangerousParamsFailCleanly(t *testing.T) {
	// t=0 / p=0 would panic argon2.IDKey, and m= sizes an allocation —
	// a tampered PHC string must come back as malformed, never execute.
	for name, phc := range map[string]string{
		"zero time":    "$argon2id$v=19$m=19456,t=0,p=1$c2FsdHNhbHQ$a2V5a2V5",
		"zero threads": "$argon2id$v=19$m=19456,t=2,p=0$c2FsdHNhbHQ$a2V5a2V5",
		"zero memory":  "$argon2id$v=19$m=0,t=2,p=1$c2FsdHNhbHQ$a2V5a2V5",
		"huge memory":  "$argon2id$v=19$m=4294967295,t=2,p=1$c2FsdHNhbHQ$a2V5a2V5",
		"empty salt":   "$argon2id$v=19$m=19456,t=2,p=1$$a2V5a2V5",
	} {
		ok, err := VerifyPassword("whatever", phc)
		if err == nil {
			t.Errorf("%s: VerifyPassword returned no error for dangerous params %q", name, phc)
		}
		if ok {
			t.Errorf("%s: VerifyPassword accepted dangerous params %q", name, phc)
		}
	}
}

func TestTokenHashDeterministicPerSecret(t *testing.T) {
	a := NewService(&stubStore{}, "secret-one-secret-one-secret-one!")
	b := NewService(&stubStore{}, "secret-two-secret-two-secret-two!")

	if a.hashToken("token") != a.hashToken("token") {
		t.Error("hashToken is not deterministic for the same secret")
	}
	if a.hashToken("token") == b.hashToken("token") {
		t.Error("hashToken collides across different secrets")
	}
	if a.hashToken("token") == a.hashToken("other-token") {
		t.Error("hashToken collides across different tokens")
	}
}

// stubStore implements Store in-memory, following the httpapi stub pattern.
type stubStore struct {
	organizers map[string]gen.Organizer // by username
	sessions   map[string]gen.Session   // by token_hash
	deleted    []string                 // token hashes passed to DeleteSessionByTokenHash
	expiredRan bool
	expiredErr error // returned by DeleteExpiredSessions
}

func (s *stubStore) GetOrganizerByUsername(ctx context.Context, username string) (gen.Organizer, error) {
	org, ok := s.organizers[username]
	if !ok {
		return gen.Organizer{}, store.ErrNotFound
	}
	return org, nil
}

func (s *stubStore) CreateSession(ctx context.Context, organizerID, tokenHash string, expiresAt time.Time) (gen.Session, error) {
	if s.sessions == nil {
		s.sessions = map[string]gen.Session{}
	}
	sess := gen.Session{ID: "sess-1", OrganizerID: organizerID, TokenHash: tokenHash, ExpiresAt: expiresAt}
	s.sessions[tokenHash] = sess
	return sess, nil
}

func (s *stubStore) GetSessionOrganizer(ctx context.Context, tokenHash string) (gen.Organizer, error) {
	sess, ok := s.sessions[tokenHash]
	if !ok || !sess.ExpiresAt.After(time.Now()) {
		return gen.Organizer{}, store.ErrNotFound
	}
	for _, org := range s.organizers {
		if org.ID == sess.OrganizerID {
			return org, nil
		}
	}
	return gen.Organizer{}, store.ErrNotFound
}

func (s *stubStore) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	s.deleted = append(s.deleted, tokenHash)
	delete(s.sessions, tokenHash)
	return nil
}

func (s *stubStore) DeleteExpiredSessions(ctx context.Context) error {
	s.expiredRan = true
	return s.expiredErr
}

func seededStore(t *testing.T, username, password string) *stubStore {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return &stubStore{
		organizers: map[string]gen.Organizer{
			username: {ID: "org-1", Username: username, PasswordHash: hash},
		},
	}
}

func TestLoginSuccessCreatesSession(t *testing.T) {
	st := seededStore(t, "avraham", "the-password")
	svc := NewService(st, "0123456789abcdef0123456789abcdef")

	token, org, err := svc.Login(context.Background(), "avraham", "the-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if org.ID != "org-1" || org.Username != "avraham" {
		t.Errorf("Login organizer = %+v, want org-1/avraham", org)
	}
	if token == "" || strings.ContainsAny(token, "+/=") {
		t.Errorf("token %q is not base64url-without-padding", token)
	}
	sess, ok := st.sessions[svc.hashToken(token)]
	if !ok {
		t.Fatal("no session row stored under HMAC(token)")
	}
	if sess.OrganizerID != "org-1" {
		t.Errorf("session organizer = %q, want org-1", sess.OrganizerID)
	}
	if ttl := time.Until(sess.ExpiresAt); ttl < 29*24*time.Hour || ttl > 31*24*time.Hour {
		t.Errorf("session TTL = %v, want ~30 days", ttl)
	}
	if !st.expiredRan {
		t.Error("Login did not run expired-session housekeeping")
	}
}

func TestLoginSurvivesCleanupFailure(t *testing.T) {
	// Housekeeping is best-effort: a failing DeleteExpiredSessions must be
	// logged, never turned into a login failure.
	st := seededStore(t, "avraham", "the-password")
	st.expiredErr = errors.New("cleanup query broken")
	svc := NewService(st, "0123456789abcdef0123456789abcdef")

	token, org, err := svc.Login(context.Background(), "avraham", "the-password")
	if err != nil {
		t.Fatalf("Login failed because housekeeping failed: %v", err)
	}
	if token == "" || org.Username != "avraham" {
		t.Errorf("Login returned token=%q org=%+v, want a session for avraham", token, org)
	}
}

func TestLoginFailureModesShareOneSentinel(t *testing.T) {
	st := seededStore(t, "avraham", "the-password")
	svc := NewService(st, "0123456789abcdef0123456789abcdef")

	_, _, unknownErr := svc.Login(context.Background(), "nobody", "the-password")
	if !errors.Is(unknownErr, ErrInvalidCredentials) {
		t.Errorf("unknown-user error = %v, want ErrInvalidCredentials", unknownErr)
	}
	_, _, wrongErr := svc.Login(context.Background(), "avraham", "not-the-password")
	if !errors.Is(wrongErr, ErrInvalidCredentials) {
		t.Errorf("wrong-password error = %v, want ErrInvalidCredentials", wrongErr)
	}
	if unknownErr.Error() != wrongErr.Error() {
		t.Errorf("failure modes differ: %q vs %q (enumeration risk)", unknownErr, wrongErr)
	}
	if len(st.sessions) != 0 {
		t.Error("failed login left a session row behind")
	}
}

func TestAuthenticateRoundtrip(t *testing.T) {
	st := seededStore(t, "avraham", "the-password")
	svc := NewService(st, "0123456789abcdef0123456789abcdef")

	token, _, err := svc.Login(context.Background(), "avraham", "the-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	org, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate with fresh token: %v", err)
	}
	if org.Username != "avraham" {
		t.Errorf("Authenticate organizer = %+v, want avraham", org)
	}

	if _, err := svc.Authenticate(context.Background(), "forged-token"); !errors.Is(err, ErrNoSession) {
		t.Errorf("Authenticate with forged token = %v, want ErrNoSession", err)
	}
}

func TestAuthenticateExpiredSessionRejected(t *testing.T) {
	st := seededStore(t, "avraham", "the-password")
	svc := NewService(st, "0123456789abcdef0123456789abcdef")

	token, _, err := svc.Login(context.Background(), "avraham", "the-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	// Force the stored session into the past; the lookup must treat it as gone.
	h := svc.hashToken(token)
	sess := st.sessions[h]
	sess.ExpiresAt = time.Now().Add(-time.Minute)
	st.sessions[h] = sess

	if _, err := svc.Authenticate(context.Background(), token); !errors.Is(err, ErrNoSession) {
		t.Errorf("Authenticate with expired session = %v, want ErrNoSession", err)
	}
}

func TestLogoutDeletesSession(t *testing.T) {
	st := seededStore(t, "avraham", "the-password")
	svc := NewService(st, "0123456789abcdef0123456789abcdef")

	token, _, err := svc.Login(context.Background(), "avraham", "the-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := svc.Logout(context.Background(), token); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if len(st.deleted) != 1 || st.deleted[0] != svc.hashToken(token) {
		t.Errorf("Logout deleted %v, want exactly [HMAC(token)]", st.deleted)
	}
	if _, err := svc.Authenticate(context.Background(), token); !errors.Is(err, ErrNoSession) {
		t.Errorf("Authenticate after logout = %v, want ErrNoSession", err)
	}
}
