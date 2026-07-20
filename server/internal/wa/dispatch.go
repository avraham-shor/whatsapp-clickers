package wa

import (
	"context"
	"log/slog"
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
	// dispatchRateLimit is msg/sec, shared globally across all workers via
	// one rate.Limiter — not per-worker. 70, not Meta's 80/sec ceiling:
	// the AC requires staying *under* 80, and inbound traffic shares the
	// same per-number throughput budget. 12.5% headroom still clears an
	// 80-participant burst in ~1.2s, well inside FR-4's 5s/95% target.
	dispatchRateLimit = 70
	// dispatchMaxAttempts is the total number of SendText attempts per
	// message (not retries-after-the-first): 3 attempts, 2 backoff waits
	// between them (dispatchBackoff below).
	dispatchMaxAttempts = 3
)

// dispatchBackoff holds the wait between attempts 1->2 and 2->3. Only two
// waits exist because dispatchMaxAttempts caps the total at 3 sends.
var dispatchBackoff = []time.Duration{1 * time.Second, 2 * time.Second}

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
// limiter so total throughput stays under Meta's per-number ceiling
// regardless of pool size.
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
	select {
	case d.queue <- outboundMessage{to: to, body: body}:
	default:
		d.logger.Warn("outbound queue full, message dropped", "phone_last4", PhoneLast4(to))
	}
}

// Run starts the worker pool and blocks until ctx is cancelled. In-flight
// sends finish (bounded by their own per-request timeout); anything still
// queued is dropped with one summary WARN — an accepted pilot posture,
// since a redeploy mid-game already interrupts the room.
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
			return // ctx cancelled while waiting for a rate-limit slot
		}
		lastErr = d.client.SendText(ctx, msg.to, msg.body)
		if lastErr == nil {
			return
		}
		if attempt < dispatchMaxAttempts {
			d.logger.Warn("send retry", "phone_last4", PhoneLast4(msg.to), "attempt", attempt, "error", lastErr.Error())
			d.sleep(ctx, dispatchBackoff[attempt-1])
			if ctx.Err() != nil {
				return
			}
		}
	}
	// Never ERROR (NFR-8): a healthy run has zero ERRORs, and this never
	// propagates to the caller — fire-and-forget by design.
	d.logger.Warn("send failed permanently", "phone_last4", PhoneLast4(msg.to), "error", lastErr.Error())
}
