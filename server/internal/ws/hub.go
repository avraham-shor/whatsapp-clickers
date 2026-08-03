// Package ws is the single realtime channel every live surface uses: a
// per-game hub broadcasting full-snapshot messages to WebSocket clients.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

const (
	// outboundQueueCapacity absorbs a short burst of broadcasts for one
	// connection before its write is dropped — a full channel means that one
	// client's write side isn't keeping up, not that the hub should block
	// the broadcaster for everyone else.
	outboundQueueCapacity = 8
	// pingInterval keeps an idle connection (nothing broadcast between
	// stories, e.g. an open, empty lobby) from going undetected-dead: a
	// stalled-but-reachable peer never errors a plain Read/Write, and a
	// silent connection risks being severed by an intermediary idle timeout
	// (e.g. Railway's edge) with nothing here to notice.
	pingInterval = 30 * time.Second
	// writeTimeout bounds every write and ping so a stalled peer can't pin
	// this connection's writer goroutine (and its hub slot) forever.
	writeTimeout = 10 * time.Second
)

// socketWriter is the minimal coder/websocket.Conn surface the hub needs;
// *websocket.Conn satisfies it. Consumer-defined so hub tests can inject a
// fake connection without a real network round trip.
type socketWriter interface {
	Write(ctx context.Context, typ websocket.MessageType, p []byte) error
	Ping(ctx context.Context) error
}

// envelope is the one WS wire message shape: server→client only, full
// snapshots (never diffs).
type envelope struct {
	Type  string        `json:"type"`
	Seq   uint64        `json:"seq"`
	State game.Snapshot `json:"state"`
}

// conn is one hub-registered connection. coder/websocket connections are
// not safe for concurrent writes, so every write for a connection — the
// initial on-connect snapshot and every later broadcast alike — is
// serialized through this one channel and its dedicated writer goroutine;
// they can never race.
type conn struct {
	send   chan []byte
	cancel context.CancelFunc
}

// enqueue queues payload for delivery to this connection. Returns false if
// the outbound channel is already full — mirrors Broadcast's drop-on-full
// posture rather than blocking the caller (the caller is the HTTP handler
// goroutine for the initial snapshot; blocking it would strand the
// connection registered with nothing left to ever drive its read loop and
// thus its eventual unregister).
func (c *conn) enqueue(payload []byte) bool {
	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

// Hub is the per-game connection registry and broadcast fan-out. In-memory
// state here is a cache of DB truth: it holds nothing that survives a
// restart and never diverges from Postgres as the source ("no Redis...
// always rebuildable from Postgres").
type Hub struct {
	logger *slog.Logger

	mu    sync.Mutex
	conns map[string]map[*conn]struct{}
	seqs  map[string]uint64
}

// NewHub builds an empty Hub. logger may be nil, in which case
// slog.Default() is used.
func NewHub(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		logger: logger,
		conns:  make(map[string]map[*conn]struct{}),
		seqs:   make(map[string]uint64),
	}
}

// currentSeq returns the last broadcast seq for gameID (0 if none yet) —
// used to stamp the initial on-connect snapshot with the hub's current
// sequence rather than a fresh one (connecting doesn't advance state).
func (h *Hub) currentSeq(gameID string) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seqs[gameID]
}

// register starts a writer goroutine draining a new connection's outbound
// channel and adds it to gameID's fan-out set. ctx bounds every write for
// this connection's lifetime; the caller passes the request context, which
// stays live for as long as the hijacked connection's handler blocks in its
// read loop.
func (h *Hub) register(ctx context.Context, gameID string, sock socketWriter) *conn {
	connCtx, cancel := context.WithCancel(ctx)
	c := &conn{send: make(chan []byte, outboundQueueCapacity), cancel: cancel}

	h.mu.Lock()
	if h.conns[gameID] == nil {
		h.conns[gameID] = make(map[*conn]struct{})
	}
	h.conns[gameID][c] = struct{}{}
	h.mu.Unlock()

	go h.writeLoop(connCtx, sock, c)
	return c
}

// unregister removes c from gameID's fan-out set and stops its writer
// goroutine. Safe to call exactly once per register. When this was the
// game's last connection, its seqs entry is dropped too — the in-memory
// hub holds nothing that survives a restart, and an idle/finished game with
// nobody connected shouldn't accumulate a permanent map entry for the life
// of the process; a later reconnect simply starts that game's sequence
// fresh (any earlier lastSeq the client held was reset to -1 on its own
// reconnect anyway).
func (h *Hub) unregister(gameID string, c *conn) {
	h.mu.Lock()
	delete(h.conns[gameID], c)
	if len(h.conns[gameID]) == 0 {
		delete(h.conns, gameID)
		delete(h.seqs, gameID)
	}
	h.mu.Unlock()
	c.cancel()
}

// Broadcast marshals one {"type":"snapshot","seq":N,"state":snapshot}
// envelope and fans it out to every connection registered for gameID. A
// full/slow connection's write is dropped rather than blocking the
// broadcaster for every other client (mirrors the dispatcher's "one
// recipient's trouble never blocks the loop" rule).
func (h *Hub) Broadcast(gameID string, snapshot game.Snapshot) {
	h.mu.Lock()
	h.seqs[gameID]++
	seq := h.seqs[gameID]
	conns := make([]*conn, 0, len(h.conns[gameID]))
	for c := range h.conns[gameID] {
		conns = append(conns, c)
	}
	h.mu.Unlock()

	payload, err := json.Marshal(envelope{Type: "snapshot", Seq: seq, State: snapshot})
	if err != nil {
		// Snapshot's fields are all plain strings/ints/slices — Marshal
		// cannot fail on this type in practice; treat it as fatal to this
		// broadcast rather than silently dropping every client.
		h.logger.Error("marshal snapshot envelope failed", "game_id", gameID, "error", err)
		return
	}

	for _, c := range conns {
		select {
		case c.send <- payload:
		default:
			h.logger.Warn("outbound socket queue full, snapshot dropped", "game_id", gameID)
		}
	}
}

func (h *Hub) writeLoop(ctx context.Context, sock socketWriter, c *conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-c.send:
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := sock.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				// The handler's read-discard loop will observe the same
				// broken connection and drive unregister/CloseNow; this
				// goroutine just stops writing.
				return
			}
		case <-ticker.C:
			// A stalled-but-reachable peer (frozen client, closed TCP
			// window) never errors a plain Write on its own — this ping is
			// what actually exercises the connection and, on failure,
			// unblocks the same teardown path as a failed Write. It also
			// keeps a genuinely idle lobby (no broadcasts between stories)
			// from going silent long enough for an intermediary idle
			// timeout to sever it.
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := sock.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
