package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

// recordingConn is a fake socketWriter that records every write and signals
// notify after each one — synchronization for the writer goroutine's
// asynchronous drain, since go test -race is unavailable in this
// environment and plain shared-var polling would be a real data race.
type recordingConn struct {
	mu      sync.Mutex
	written [][]byte
	notify  chan struct{}
}

func newRecordingConn() *recordingConn {
	return &recordingConn{notify: make(chan struct{}, 100)}
}

func (f *recordingConn) Write(ctx context.Context, typ websocket.MessageType, p []byte) error {
	f.mu.Lock()
	cp := append([]byte(nil), p...)
	f.written = append(f.written, cp)
	f.mu.Unlock()
	f.notify <- struct{}{}
	return nil
}

// Ping is a no-op — recordingConn tests never run long enough to reach a
// pingInterval tick, so a real ping never fires; the method exists only to
// satisfy socketWriter.
func (f *recordingConn) Ping(ctx context.Context) error {
	return nil
}

func (f *recordingConn) waitForWrite(t *testing.T) {
	t.Helper()
	select {
	case <-f.notify:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a write")
	}
}

func (f *recordingConn) messages() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.written...)
}

// blockingConn's Write blocks until release is closed, forcing the hub's
// outbound channel to fill so Broadcast's drop-on-full path is exercised
// deterministically.
type blockingConn struct {
	started chan struct{}
	release chan struct{}
}

func newBlockingConn() *blockingConn {
	return &blockingConn{started: make(chan struct{}, 1), release: make(chan struct{})}
}

func (b *blockingConn) Write(ctx context.Context, typ websocket.MessageType, p []byte) error {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-b.release
	return nil
}

// Ping is a no-op — same reasoning as recordingConn.Ping.
func (b *blockingConn) Ping(ctx context.Context) error {
	return nil
}

func decodeEnvelope(t *testing.T, payload []byte) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("payload is not a valid envelope: %v", err)
	}
	return env
}

func TestBroadcastFansOutToEveryRegisteredConn(t *testing.T) {
	h := NewHub(nil)
	c1 := newRecordingConn()
	c2 := newRecordingConn()
	h.register(context.Background(), "game-1", c1)
	h.register(context.Background(), "game-1", c2)

	h.Broadcast("game-1", game.Snapshot{GameID: "game-1", State: "lobby"})

	c1.waitForWrite(t)
	c2.waitForWrite(t)

	for _, c := range []*recordingConn{c1, c2} {
		msgs := c.messages()
		if len(msgs) != 1 {
			t.Fatalf("conn received %d messages, want 1", len(msgs))
		}
		env := decodeEnvelope(t, msgs[0])
		if env.Type != "snapshot" || env.Seq != 1 || env.State.State != "lobby" {
			t.Errorf("envelope = %+v, want snapshot/seq=1/state=lobby", env)
		}
	}
}

func TestBroadcastSeqIncrementsPerGameIndependently(t *testing.T) {
	h := NewHub(nil)
	cA := newRecordingConn()
	cB := newRecordingConn()
	h.register(context.Background(), "game-A", cA)
	h.register(context.Background(), "game-B", cB)

	h.Broadcast("game-A", game.Snapshot{GameID: "game-A"})
	h.Broadcast("game-B", game.Snapshot{GameID: "game-B"})
	h.Broadcast("game-A", game.Snapshot{GameID: "game-A"})

	cA.waitForWrite(t)
	cA.waitForWrite(t)
	cB.waitForWrite(t)

	aMsgs := cA.messages()
	if len(aMsgs) != 2 {
		t.Fatalf("game-A conn received %d messages, want 2", len(aMsgs))
	}
	if seq := decodeEnvelope(t, aMsgs[0]).Seq; seq != 1 {
		t.Errorf("game-A first broadcast seq = %d, want 1", seq)
	}
	if seq := decodeEnvelope(t, aMsgs[1]).Seq; seq != 2 {
		t.Errorf("game-A second broadcast seq = %d, want 2", seq)
	}

	bMsgs := cB.messages()
	if len(bMsgs) != 1 {
		t.Fatalf("game-B conn received %d messages, want 1", len(bMsgs))
	}
	if seq := decodeEnvelope(t, bMsgs[0]).Seq; seq != 1 {
		t.Errorf("game-B broadcast seq = %d, want 1 (independent of game-A)", seq)
	}

	if got := h.currentSeq("game-A"); got != 2 {
		t.Errorf("currentSeq(game-A) = %d, want 2", got)
	}
	if got := h.currentSeq("game-B"); got != 1 {
		t.Errorf("currentSeq(game-B) = %d, want 1", got)
	}
}

func TestBroadcastDropsOnFullChannelWithoutBlockingOrPanicking(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	h := NewHub(logger)

	blocker := newBlockingConn()
	h.register(context.Background(), "game-1", blocker)

	// The writer goroutine pulls the first payload immediately and blocks
	// inside Write; every payload after that sits in the buffered channel
	// (capacity outboundQueueCapacity) until it's full, then Broadcast must
	// drop rather than block.
	for i := 0; i < outboundQueueCapacity+3; i++ {
		h.Broadcast("game-1", game.Snapshot{GameID: "game-1"})
	}
	select {
	case <-blocker.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the blocking conn's write to start")
	}
	close(blocker.release)

	if !strings.Contains(logBuf.String(), "outbound socket queue full") {
		t.Errorf("log output = %q, want a drop-on-full WARN with game_id", logBuf.String())
	}
	if !strings.Contains(logBuf.String(), "game-1") {
		t.Errorf("log output = %q, want the dropped broadcast's game_id", logBuf.String())
	}
}

func TestEnqueueDropsOnFullChannelWithoutBlocking(t *testing.T) {
	// enqueue is the handler's path for the initial on-connect snapshot; it
	// must mirror Broadcast's drop-on-full posture rather than block —
	// blocking here would strand the caller (the HTTP handler goroutine)
	// before it ever reaches the read loop that drives unregister.
	c := &conn{send: make(chan []byte, 1)}
	if !c.enqueue([]byte("first")) {
		t.Fatal("enqueue() = false on an empty channel, want true")
	}
	if c.enqueue([]byte("second")) {
		t.Fatal("enqueue() = true on a full channel, want false (dropped)")
	}
}

func TestUnregisterStopsFurtherDelivery(t *testing.T) {
	h := NewHub(nil)
	c := newRecordingConn()
	conn := h.register(context.Background(), "game-1", c)

	h.Broadcast("game-1", game.Snapshot{GameID: "game-1"})
	c.waitForWrite(t)

	h.unregister("game-1", conn)
	h.Broadcast("game-1", game.Snapshot{GameID: "game-1"})

	// No second write should ever arrive — give the writer goroutine a
	// generous window to (wrongly) deliver one before asserting it didn't.
	select {
	case <-c.notify:
		t.Fatal("received a broadcast after unregister")
	case <-time.After(100 * time.Millisecond):
	}
	if len(c.messages()) != 1 {
		t.Errorf("messages after unregister = %d, want 1 (only the pre-unregister broadcast)", len(c.messages()))
	}
}

func TestUnregisterLastConnDropsTheSeqsEntry(t *testing.T) {
	h := NewHub(nil)
	c1 := newRecordingConn()
	c2 := newRecordingConn()
	conn1 := h.register(context.Background(), "game-1", c1)
	conn2 := h.register(context.Background(), "game-1", c2)

	h.Broadcast("game-1", game.Snapshot{GameID: "game-1"})
	c1.waitForWrite(t)
	c2.waitForWrite(t)

	h.unregister("game-1", conn1)
	h.mu.Lock()
	_, stillTracked := h.seqs["game-1"]
	h.mu.Unlock()
	if !stillTracked {
		t.Fatal("seqs entry dropped while a connection is still registered, want it kept")
	}

	h.unregister("game-1", conn2)
	h.mu.Lock()
	_, stillTracked = h.seqs["game-1"]
	h.mu.Unlock()
	if stillTracked {
		t.Error("seqs entry survived the last connection's unregister, want it dropped")
	}

	// A fresh connection after the game went idle starts the sequence over.
	if got := h.currentSeq("game-1"); got != 0 {
		t.Errorf("currentSeq after full unregister = %d, want 0 (reset)", got)
	}
}
