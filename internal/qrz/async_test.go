package qrz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAsyncEnricherLooksUpAndStoresProfile(t *testing.T) {
	store := newMemoryStore()
	lookup := &fakeLookup{profiles: map[string]Profile{
		"N7ZG": {Callsign: "N7ZG", LookupStatus: "found"},
	}}
	enricher, err := NewAsyncEnricher(context.Background(), lookup, store, WithQueueSize(4))
	if err != nil {
		t.Fatal(err)
	}
	enricher.Enqueue("n7zg")
	enricher.Stop()

	if _, ok := store.profiles["N7ZG"]; !ok {
		t.Fatalf("profile was not stored: %#v", store.profiles)
	}
	stats := enricher.Stats()
	if stats.Enqueued != 1 || stats.Found != 1 || stats.Inserted != 1 || stats.Updated != 1 || stats.Errors != 0 {
		t.Fatalf("unexpected stats: %s", stats)
	}
}

func TestAsyncEnricherStoresNotFoundProfile(t *testing.T) {
	store := newMemoryStore()
	lookup := &fakeLookup{err: ErrNotFound}
	enricher, err := NewAsyncEnricher(context.Background(), lookup, store, WithQueueSize(4))
	if err != nil {
		t.Fatal(err)
	}
	enricher.Enqueue("badcall")
	enricher.Stop()

	profile, ok := store.profiles["BADCALL"]
	if !ok {
		t.Fatalf("not-found profile was not stored: %#v", store.profiles)
	}
	if profile.LookupStatus != "not_found" {
		t.Fatalf("lookup status=%q", profile.LookupStatus)
	}
	stats := enricher.Stats()
	if stats.NotFound != 1 || stats.Inserted != 1 || stats.Updated != 1 || stats.Errors != 0 {
		t.Fatalf("unexpected stats: %s", stats)
	}
}

func TestAsyncEnricherSkipsCachedProfile(t *testing.T) {
	store := newMemoryStore()
	store.profiles["N7ZG"] = Profile{Callsign: "N7ZG", LookupStatus: "found"}
	lookup := &fakeLookup{}
	enricher, err := NewAsyncEnricher(context.Background(), lookup, store, WithQueueSize(4))
	if err != nil {
		t.Fatal(err)
	}
	enricher.Enqueue("N7ZG")
	enricher.Stop()

	if lookup.calls != 0 {
		t.Fatalf("lookup calls=%d", lookup.calls)
	}
	stats := enricher.Stats()
	if stats.Cached != 1 || stats.Inserted != 0 || stats.Errors != 0 {
		t.Fatalf("unexpected stats: %s", stats)
	}
}

func TestAsyncEnricherUpdatesPendingProfile(t *testing.T) {
	store := newMemoryStore()
	store.profiles["N7ZG"] = PendingProfile("N7ZG", time.Now())
	lookup := &fakeLookup{profiles: map[string]Profile{
		"N7ZG": {Callsign: "N7ZG", LookupStatus: "found"},
	}}
	enricher, err := NewAsyncEnricher(context.Background(), lookup, store, WithQueueSize(4))
	if err != nil {
		t.Fatal(err)
	}
	enricher.Enqueue("N7ZG")
	enricher.Stop()

	if got := store.profiles["N7ZG"].LookupStatus; got != "found" {
		t.Fatalf("lookup status=%q, want found", got)
	}
	stats := enricher.Stats()
	if stats.Inserted != 0 || stats.Updated != 1 || stats.Found != 1 || stats.Errors != 0 {
		t.Fatalf("unexpected stats: %s", stats)
	}
}

func TestAsyncEnricherDoesNotBlockWhenQueueIsFull(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := newMemoryStore()
	lookup := &blockingLookup{started: make(chan struct{})}
	enricher, err := NewAsyncEnricher(ctx, lookup, store, WithQueueSize(1))
	if err != nil {
		t.Fatal(err)
	}
	enricher.Enqueue("N7ZG")
	<-lookup.started

	started := time.Now()
	for i := 0; i < 1000; i++ {
		enricher.Enqueue(fmt.Sprintf("K%dABC", i))
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("enqueue blocked while QRZ worker was stalled")
	}
	cancel()
	enricher.Stop()
	if enricher.Stats().Dropped == 0 {
		t.Fatalf("expected dropped enrichment work, stats=%s", enricher.Stats())
	}
}

type fakeLookup struct {
	profiles map[string]Profile
	err      error
	calls    int
}

func (l *fakeLookup) Lookup(context.Context, string) (Profile, error) {
	l.calls++
	if l.err != nil {
		return Profile{}, l.err
	}
	if profile, ok := l.profiles["N7ZG"]; ok {
		return profile, nil
	}
	return Profile{}, errors.New("unexpected call")
}

type blockingLookup struct {
	once    sync.Once
	started chan struct{}
}

func (l *blockingLookup) Lookup(ctx context.Context, call string) (Profile, error) {
	l.once.Do(func() { close(l.started) })
	<-ctx.Done()
	return Profile{}, ctx.Err()
}

type memoryStore struct {
	profiles map[string]Profile
}

func newMemoryStore() *memoryStore {
	return &memoryStore{profiles: map[string]Profile{}}
}

func (s *memoryStore) HasProfile(_ context.Context, call string) (bool, error) {
	_, ok := s.profiles[call]
	return ok, nil
}

func (s *memoryStore) InsertProfile(_ context.Context, profile Profile) error {
	s.profiles[profile.Callsign] = profile
	return nil
}

func (s *memoryStore) LookupStatus(_ context.Context, call string) (string, bool, error) {
	profile, ok := s.profiles[call]
	return profile.LookupStatus, ok, nil
}

func (s *memoryStore) EnsurePendingProfile(_ context.Context, call string) (bool, error) {
	if _, ok := s.profiles[call]; ok {
		return false, nil
	}
	s.profiles[call] = PendingProfile(call, time.Now())
	return true, nil
}

func (s *memoryStore) UpdateProfile(_ context.Context, profile Profile) error {
	s.profiles[profile.Callsign] = profile
	return nil
}
