package qrz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNilLookup = errors.New("qrz async enricher requires a lookup client")
	ErrNilStore  = errors.New("qrz async enricher requires a profile store")
)

type ProfileLookup interface {
	Lookup(context.Context, string) (Profile, error)
}

type ProfileStore interface {
	LookupStatus(context.Context, string) (string, bool, error)
	EnsurePendingProfile(context.Context, string) (bool, error)
	UpdateProfile(context.Context, Profile) error
}

type AsyncEnricher struct {
	ctx           context.Context
	lookup        ProfileLookup
	store         ProfileStore
	queue         chan string
	workers       int
	lookupTimeout time.Duration
	profileHook   func(*Profile)
	logger        func(string, ...interface{})
	seen          sync.Map
	closeOnce     sync.Once
	wg            sync.WaitGroup

	enqueued int64
	dropped  int64
	cached   int64
	found    int64
	notFound int64
	inserted int64
	updated  int64
	errors   int64
}

type AsyncEnricherStats struct {
	Enqueued int64
	Dropped  int64
	Cached   int64
	Found    int64
	NotFound int64
	Inserted int64
	Updated  int64
	Errors   int64
}

type AsyncEnricherOption func(*AsyncEnricher)

func WithQueueSize(size int) AsyncEnricherOption {
	return func(e *AsyncEnricher) {
		if size > 0 {
			e.queue = make(chan string, size)
		}
	}
}

func WithWorkers(workers int) AsyncEnricherOption {
	return func(e *AsyncEnricher) {
		if workers > 0 {
			e.workers = workers
		}
	}
}

func WithLookupTimeout(timeout time.Duration) AsyncEnricherOption {
	return func(e *AsyncEnricher) {
		if timeout > 0 {
			e.lookupTimeout = timeout
		}
	}
}

func WithProfileHook(hook func(*Profile)) AsyncEnricherOption {
	return func(e *AsyncEnricher) {
		e.profileHook = hook
	}
}

func WithLogger(logger func(string, ...interface{})) AsyncEnricherOption {
	return func(e *AsyncEnricher) {
		if logger != nil {
			e.logger = logger
		}
	}
}

func NewAsyncEnricher(ctx context.Context, lookup ProfileLookup, store ProfileStore, opts ...AsyncEnricherOption) (*AsyncEnricher, error) {
	if lookup == nil {
		return nil, ErrNilLookup
	}
	if store == nil {
		return nil, ErrNilStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	e := &AsyncEnricher{
		ctx:           ctx,
		lookup:        lookup,
		store:         store,
		queue:         make(chan string, 256),
		workers:       1,
		lookupTimeout: 10 * time.Second,
		logger:        func(string, ...interface{}) {},
	}
	for _, opt := range opts {
		opt(e)
	}
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker()
	}
	return e, nil
}

func (e *AsyncEnricher) Enqueue(call string) {
	call = normalizeCallsign(call)
	if call == "" {
		return
	}
	if _, ok := e.seen.Load(call); ok {
		return
	}
	select {
	case e.queue <- call:
		e.seen.Store(call, struct{}{})
		atomic.AddInt64(&e.enqueued, 1)
	default:
		atomic.AddInt64(&e.dropped, 1)
	}
}

func (e *AsyncEnricher) Stop() {
	e.closeOnce.Do(func() {
		close(e.queue)
	})
	e.wg.Wait()
}

func (e *AsyncEnricher) Stats() AsyncEnricherStats {
	return AsyncEnricherStats{
		Enqueued: atomic.LoadInt64(&e.enqueued),
		Dropped:  atomic.LoadInt64(&e.dropped),
		Cached:   atomic.LoadInt64(&e.cached),
		Found:    atomic.LoadInt64(&e.found),
		NotFound: atomic.LoadInt64(&e.notFound),
		Inserted: atomic.LoadInt64(&e.inserted),
		Updated:  atomic.LoadInt64(&e.updated),
		Errors:   atomic.LoadInt64(&e.errors),
	}
}

func (e *AsyncEnricher) worker() {
	defer e.wg.Done()
	for {
		select {
		case <-e.ctx.Done():
			return
		case call, ok := <-e.queue:
			if !ok {
				return
			}
			e.process(call)
		}
	}
}

func (e *AsyncEnricher) process(call string) {
	status, exists, err := e.store.LookupStatus(e.ctx, call)
	if err != nil {
		e.recordError("qrz cache check call=%s err=%v", call, err)
		return
	}
	if exists && profileStatusComplete(status) {
		atomic.AddInt64(&e.cached, 1)
		return
	}
	inserted, err := e.store.EnsurePendingProfile(e.ctx, call)
	if err != nil {
		e.recordError("qrz pending insert call=%s err=%v", call, err)
		return
	}
	if inserted {
		atomic.AddInt64(&e.inserted, 1)
	}

	lookupCtx, cancel := context.WithTimeout(e.ctx, e.lookupTimeout)
	defer cancel()

	profile, err := e.lookup.Lookup(lookupCtx, call)
	if errors.Is(err, ErrNotFound) {
		profile = NotFoundProfile(call, time.Now())
		atomic.AddInt64(&e.notFound, 1)
	} else if err != nil {
		e.recordError("qrz lookup call=%s err=%v", call, err)
		return
	} else {
		atomic.AddInt64(&e.found, 1)
	}
	if e.profileHook != nil {
		e.profileHook(&profile)
	}
	if err := e.store.UpdateProfile(e.ctx, profile); err != nil {
		e.recordError("qrz cache update call=%s err=%v", call, err)
		return
	}
	atomic.AddInt64(&e.updated, 1)
}

func (e *AsyncEnricher) recordError(format string, args ...interface{}) {
	atomic.AddInt64(&e.errors, 1)
	e.logger(format, args...)
}

func (s AsyncEnricherStats) String() string {
	return fmt.Sprintf("enqueued=%d dropped=%d cached=%d found=%d not_found=%d inserted=%d updated=%d errors=%d",
		s.Enqueued, s.Dropped, s.Cached, s.Found, s.NotFound, s.Inserted, s.Updated, s.Errors)
}

func profileStatusComplete(status string) bool {
	return status == "found" || status == "not_found"
}
