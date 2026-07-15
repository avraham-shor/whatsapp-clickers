// Command provision creates or updates an organizer account — the documented
// provisioning path for the pilot (no self-serve signup). The password is
// read from stdin (piped or typed), never from argv and never logged:
//
//	echo -n 'the-password' | go run ./cmd/provision -username avraham
//
// Unlike the server it requires only DATABASE_URL, so it can run against the
// Railway database without the full production env.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

// minPasswordLen is the NIST SP 800-63B floor; the operator picks the actual
// strength, but a one-character password must not slip through.
const minPasswordLen = 8

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "provision:", err)
		os.Exit(1)
	}
}

func run() error {
	usernameFlag := flag.String("username", "", "organizer username (required)")
	flag.Parse()
	username := strings.TrimSpace(*usernameFlag)
	if username == "" {
		return errors.New("the -username flag is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL must be set (see server/.env.example)")
	}

	password, err := readPassword()
	if err != nil {
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := store.NewPool(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	st := store.New(pool)
	org, err := st.UpsertOrganizer(ctx, username, hash)
	if err != nil {
		return fmt.Errorf("upsert organizer (has the server run its migrations?): %w", err)
	}
	// A password reset must invalidate any session minted under the old
	// credentials — including a leaked one.
	if err := st.DeleteSessionsByOrganizerID(ctx, org.ID); err != nil {
		return fmt.Errorf("invalidate existing sessions: %w", err)
	}
	fmt.Printf("organizer provisioned: id=%s username=%s (existing sessions invalidated)\n", org.ID, org.Username)
	return nil
}

// readPassword takes the password from stdin. When stdin is a terminal it
// prompts on stderr; piped input works unchanged.
func readPassword() (string, error) {
	if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprint(os.Stderr, "Password (input is echoed): ")
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	password := strings.TrimRight(line, "\r\n")
	if utf8.RuneCountInString(password) < minPasswordLen {
		return "", fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	return password, nil
}
