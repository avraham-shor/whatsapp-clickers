package wa

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a thread-safe io.Writer for slog output captured while
// dispatcher worker goroutines are writing concurrently with the test
// goroutine polling it (a plain bytes.Buffer would race here — slog's
// handlers serialize Handle() calls among themselves, but that mutex does
// not protect an external, unsynchronized String() read).
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// stubSender is a SenderClient stub grown in-test. failUntilAttempt makes
// SendText fail for calls up to and including that count for a given
// recipient, then succeed; -1 means never fail, 0 means always fail.
type stubSender struct {
	mu               sync.Mutex
	calls            []string // "to" of every SendText call, in order
	failUntilAttempt int
}

func (s *stubSender) SendText(ctx context.Context, to, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, to)
	if s.failUntilAttempt == 0 {
		return errors.New("stub: send failed")
	}
	if len(s.calls) <= s.failUntilAttempt {
		return errors.New("stub: send failed")
	}
	return nil
}

func (s *stubSender) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// noSleep replaces the dispatcher's injectable backoff with an immediate
// return, so the retry test runs in microseconds instead of seconds.
func noSleep(ctx context.Context, d time.Duration) {}

func newTestDispatcher(t *testing.T, sender SenderClient) (*Dispatcher, *syncBuffer) {
	t.Helper()
	buf := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	d := NewDispatcher(sender, logger)
	d.sleep = noSleep
	return d, buf
}

func TestDispatcherHappyPathDeliversAll(t *testing.T) {
	sender := &stubSender{failUntilAttempt: -1} // never fails
	d, _ := newTestDispatcher(t, sender)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(runDone)
	}()

	d.Enqueue("972500000001", "one")
	d.Enqueue("972500000002", "two")
	d.Enqueue("972500000003", "three")

	waitForCallCount(t, sender, 3)
	cancel()
	<-runDone

	if got := sender.callCount(); got != 3 {
		t.Errorf("callCount = %d, want 3", got)
	}
}

func TestDispatcherFailingStubRetriesExactlyThreeTimesThenPermanentFailure(t *testing.T) {
	failing := &stubSender{failUntilAttempt: 0} // always fails
	d, buf := newTestDispatcher(t, failing)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(runDone)
	}()

	d.Enqueue("972500000009", "will fail")
	waitForCallCount(t, failing, 3)
	waitForLogContains(t, buf, "send failed permanently")
	cancel()
	<-runDone

	if got := failing.callCount(); got != 3 {
		t.Errorf("callCount = %d, want exactly 3 attempts", got)
	}
}

type selectiveSender struct {
	mu        sync.Mutex
	failFor   map[string]bool
	succeeded map[string]bool
}

func (s *selectiveSender) SendText(ctx context.Context, to, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failFor[to] {
		return errors.New("stub: permanent failure for this recipient")
	}
	if s.succeeded == nil {
		s.succeeded = map[string]bool{}
	}
	s.succeeded[to] = true
	return nil
}

func (s *selectiveSender) delivered(to string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.succeeded[to]
}

func TestDispatcherLaterMessagesStillDeliveredAfterAFailure(t *testing.T) {
	sender := &selectiveSender{failFor: map[string]bool{"972500000666": true}}
	d, _ := newTestDispatcher(t, sender)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(runDone)
	}()

	d.Enqueue("972500000666", "will fail permanently")
	d.Enqueue("972500000777", "should still be delivered")

	waitForDelivered(t, sender, "972500000777")
	cancel()
	<-runDone

	if !sender.delivered("972500000777") {
		t.Error("later message was not delivered after an earlier permanent failure")
	}
}

func TestDispatcherQueueFullDropsAndWarns(t *testing.T) {
	sender := &stubSender{failUntilAttempt: -1}
	d, buf := newTestDispatcher(t, sender)
	// Shrink the queue so the test doesn't need 1000 sends to fill it.
	// Workers are NOT started (Run is never called), so nothing drains it.
	d.queue = make(chan outboundMessage, 2)

	d.Enqueue("972500000001", "one")
	d.Enqueue("972500000002", "two")
	d.Enqueue("972500000003", "three (should be dropped)") // queue full

	if len(d.queue) != 2 {
		t.Fatalf("queue length = %d, want 2 (full, not blocked)", len(d.queue))
	}
	if !strings.Contains(buf.String(), "outbound queue full, message dropped") {
		t.Errorf("log missing queue-full WARN: %s", buf.String())
	}
}

func TestDispatcherContextCancelStopsWorkers(t *testing.T) {
	sender := &stubSender{failUntilAttempt: -1}
	d, _ := newTestDispatcher(t, sender)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(runDone)
	}()

	cancel()
	select {
	case <-runDone:
		// Run returned — workers stopped.
	case <-time.After(2 * time.Second):
		t.Fatal("Run(ctx) did not return after ctx cancellation")
	}
}

// waitForCallCount polls until the sender has recorded at least n calls, or
// fails the test after a timeout — avoids sleep-based flakiness.
func waitForCallCount(t *testing.T, sender *stubSender, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sender.callCount() >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d calls, got %d", n, sender.callCount())
}

func waitForDelivered(t *testing.T, sender *selectiveSender, to string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sender.delivered(to) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for delivery to %s", to)
}

func waitForLogContains(t *testing.T, buf *syncBuffer, substr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), substr) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for log to contain %q, got: %s", substr, buf.String())
}
