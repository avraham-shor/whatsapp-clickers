package wa

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// dispatchQueueCapacity absorbs a full 80-participant burst plus acks
	// with headroom; a pilot never approaches this depth.
	dispatchQueueCapacity = 1000
	// dispatchWorkerCount workers drain the queue concurrently, all
	// sharing one token-bucket limiter below.
	dispatchWorkerCount = 8
	// dispatchRateLimit is the sustained msg/sec, shared globally across all
	// workers via one rate.Limiter — not per-worker. 70, not Meta's 80/sec
	// ceiling: the AC requires staying *under* 80, and inbound traffic shares
	// the same per-number throughput budget.
	//
	// This bounds the *sustained* rate, not any one-second window. The limiter
	// is constructed with burst == rate, so after an idle period the bucket is
	// full and a straddling second can carry up to ~2x this figure; an
	// 80-message burst therefore drains in well under a second rather than the
	// ~1.2s a strict 70/s would take. Deliberate and deferred (see
	// deferred-work.md) — AC-4 governs the sustained rate, and the overshoot is
	// a pilot non-issue. Lower the burst if a strict instantaneous ceiling is
	// ever required.
	dispatchRateLimit = 70
	// dispatchMaxAttempts is the total number of SendText attempts per
	// message (not retries-after-the-first): 3 attempts, 2 backoff waits
	// between them (dispatchBackoff below).
	dispatchMaxAttempts = 3
	// drainPollInterval is how often Drain re-checks the queue depth. Short
	// enough that a shutdown drain costs no perceptible time, long enough
	// that the poll itself is free.
	drainPollInterval = 50 * time.Millisecond
)

// dispatchBackoff holds the wait between attempts 1->2 and 2->3. Only two
// waits exist because dispatchMaxAttempts caps the total at 3 sends.
var dispatchBackoff = []time.Duration{1 * time.Second, 2 * time.Second}

func init() {
	// One backoff wait sits between each pair of attempts, so the table must
	// have exactly dispatchMaxAttempts-1 entries. Bumping the attempt count
	// without extending the table would otherwise panic on dispatchBackoff
	// out-of-range at the first failing send; fail loudly at startup instead.
	if len(dispatchBackoff) != dispatchMaxAttempts-1 {
		panic("wa: dispatchBackoff must have dispatchMaxAttempts-1 entries")
	}
}

// SenderClient sends one WhatsApp text message. Consumer-defined so
// Dispatcher tests can inject a stub without a live Client.
type SenderClient interface {
	SendText(ctx context.Context, to, body string) error
}

type outboundMessage struct {
	to   string
	body string
}

// Dispatcher is a fire-and-forget, rate-limited outbound sender. Enqueue
// never blocks the caller (the game loop must never wait on delivery —
// NFR-2); Run drains the queue with a worker pool sharing one token-bucket
// limiter, so the sustained send rate is bounded globally regardless of pool
// size (see dispatchRateLimit for what that does and does not guarantee).
type Dispatcher struct {
	client  SenderClient
	logger  *slog.Logger
	queue   chan outboundMessage
	limiter *rate.Limiter
	// sleep is the injectable backoff wait — a context-aware sleep by
	// default, replaced with a no-op in tests so retries run in
	// microseconds instead of seconds.
	sleep func(ctx context.Context, d time.Duration)
}

// NewDispatcher builds a Dispatcher. logger may be nil, in which case
// slog.Default() is used.
func NewDispatcher(client SenderClient, logger *slog.Logger) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{
		client:  client,
		logger:  logger,
		queue:   make(chan outboundMessage, dispatchQueueCapacity),
		limiter: rate.NewLimiter(rate.Limit(dispatchRateLimit), dispatchRateLimit),
		sleep:   contextSleep,
	}
}

func contextSleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// Enqueue queues a message for delivery. It never blocks: a full queue
// drops the message with a WARN log (NFR-2) rather than backing up the
// caller — at pilot scale this fires only under pathology.
func (d *Dispatcher) Enqueue(to, body string) {
	// An unsendable message must not reach the queue: Meta rejects it, so it
	// would burn all dispatchMaxAttempts plus the full backoff table and one
	// rate token per attempt before the permanent-failure WARN — capacity
	// spent at exactly the moment a real burst needs it. Whitespace-only
	// counts as missing (Meta rejects it just the same); the payload itself
	// is enqueued untrimmed.
	if strings.TrimSpace(to) == "" || strings.TrimSpace(body) == "" {
		d.logger.Warn("outbound message missing recipient or body, dropped",
			"phone_last4", PhoneLast4(to),
			"has_recipient", to != "",
			"has_body", body != "")
		return
	}

	select {
	case d.queue <- outboundMessage{to: to, body: body}:
	default:
		d.logger.Warn("outbound queue full, message dropped", "phone_last4", PhoneLast4(to))
	}
}

// Run starts the worker pool and blocks until ctx is cancelled.
//
// In-flight sends do NOT finish: SendText derives its per-request timeout from
// this same ctx, so cancelling aborts the outbound HTTP request immediately.
// A message whose POST Meta had already accepted, but whose response never
// arrived, is therefore indeterminate — it may or may not have been delivered.
// Both post-cancel exit paths in send() log an "abandoned" WARN so that state
// is at least visible; they are not counted in dropped_count, which covers only
// messages still sitting in the queue. Accepted pilot posture: a redeploy
// mid-game already interrupts the room.
func (d *Dispatcher) Run(ctx context.Context) {
	var workers sync.WaitGroup
	workers.Add(dispatchWorkerCount)
	for range dispatchWorkerCount {
		go func() {
			defer workers.Done()
			d.worker(ctx)
		}()
	}
	workers.Wait()

	dropped := 0
	for {
		select {
		case <-d.queue:
			dropped++
		default:
			if dropped > 0 {
				d.logger.Warn("dispatcher shut down with unsent messages", "dropped_count", dropped)
			}
			return
		}
	}
}

// Drain blocks until the queue is empty or ctx is done. The shutdown sequence
// calls it after srv.Shutdown returns — at which point every handler has
// finished, so everything they queued is already in the queue — and before
// cancelling the dispatcher's own context.
//
// Known limit: an empty queue means nothing is *waiting*, not that in-flight
// sends have landed. Those are bounded by the client's per-request timeout and
// by whatever grace the caller allows after Drain returns.
func (d *Dispatcher) Drain(ctx context.Context) {
	ticker := time.NewTicker(drainPollInterval)
	defer ticker.Stop()
	for {
		if len(d.queue) == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-d.queue:
			d.send(ctx, msg)
		}
	}
}

func (d *Dispatcher) send(ctx context.Context, msg outboundMessage) {
	var lastErr error
	for attempt := 1; attempt <= dispatchMaxAttempts; attempt++ {
		if err := d.limiter.Wait(ctx); err != nil {
			// ctx cancelled while waiting for a rate-limit slot: never sent.
			d.logger.Warn("send abandoned at shutdown", "phone_last4", PhoneLast4(msg.to), "stage", "rate-limit wait")
			return
		}
		lastErr = d.client.SendText(ctx, msg.to, msg.body)
		if lastErr == nil {
			return
		}
		// A cancelled ctx (shutdown/redeploy) surfaces here as a send error,
		// but it is not a real delivery failure: no "send retry"/"send failed
		// permanently" WARN, which would misattribute it. Still logged — the
		// request was aborted mid-flight, so delivery is indeterminate and
		// silence would leave the operator no way to know it happened.
		if ctx.Err() != nil {
			d.logger.Warn("send abandoned at shutdown", "phone_last4", PhoneLast4(msg.to), "stage", "in flight", "attempt", attempt)
			return
		}
		if attempt < dispatchMaxAttempts {
			d.logger.Warn("send retry", "phone_last4", PhoneLast4(msg.to), "attempt", attempt, "error", lastErr.Error())
			d.sleep(ctx, dispatchBackoff[attempt-1])
			if ctx.Err() != nil {
				d.logger.Warn("send abandoned at shutdown", "phone_last4", PhoneLast4(msg.to), "stage", "backoff", "attempt", attempt)
				return
			}
		}
	}
	// Never ERROR (NFR-8): a healthy run has zero ERRORs, and this never
	// propagates to the caller — fire-and-forget by design.
	d.logger.Warn("send failed permanently", "phone_last4", PhoneLast4(msg.to), "error", lastErr.Error())
}
