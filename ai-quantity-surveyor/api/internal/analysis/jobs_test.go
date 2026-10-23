package analysis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestJobLeaseSourceAndRecovery(t *testing.T) {
	store := NewMemoryJobStore()
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	store.clock = func() time.Time { return now }
	control := JobControl{OwnerID: "owner", AttemptToken: "attempt"}
	if err := store.CreateJob(context.Background(), Candidate{ID: "job", FloorPlanID: "drawing", State: StateAnalyzing}, control); err != nil {
		t.Fatal(err)
	}
	page, err := NewPagePreparer().Prepare(context.Background(), pageImage(t, 100, 50), "plan.png", 1, "image")
	if err != nil {
		t.Fatal(err)
	}
	page.Source.FloorPlanID = "drawing"
	if err := store.PublishSource(context.Background(), "job", control, page); err != nil {
		t.Fatal(err)
	}
	if err := store.PublishSource(context.Background(), "job", control, page); err == nil {
		t.Fatal("source overwritten")
	}
	if err := store.RecoverExpired(context.Background()); err != nil {
		t.Fatal(err)
	}
	current, _, _ := store.ReadJob(context.Background(), "job")
	if current.State != StateAnalyzing {
		t.Fatal("live owner recovered")
	}
	now = now.Add(61 * time.Second)
	if err := store.RecoverExpired(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishJob(context.Background(), Candidate{ID: "job", State: StateReady}, control); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale completion accepted: %v", err)
	}
	current, _, _ = store.ReadJob(context.Background(), "job")
	if current.State != StateFailed || current.Error == nil || !current.Error.Retryable {
		t.Fatalf("wrong recovery: %+v", current)
	}
	stored, _, _ := store.ReadSource(context.Background(), "job")
	stored.Content[0] = 0
	again, _, _ := store.ReadSource(context.Background(), "job")
	if again.Content[0] == 0 {
		t.Fatal("reader mutated source")
	}
}

func TestProviderQuotaConcurrentReservationAndUTCRollover(t *testing.T) {
	store := NewMemoryJobStore()
	now := time.Date(2026, 10, 13, 23, 59, 30, 0, time.UTC)
	store.clock = func() time.Time { return now }
	control := JobControl{OwnerID: "owner", AttemptToken: "lease"}
	if err := store.CreateJob(context.Background(), Candidate{ID: "job", FloorPlanID: "drawing", State: StateAnalyzing}, control); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for index := 0; index < 20; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := store.ReserveCall(context.Background(), "job", control, "scope", "gemini", newID(), 3); err == nil {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	workers.Wait()
	if accepted != 3 {
		t.Fatalf("quota accepted %d instead of 3", accepted)
	}
	now = now.Add(31 * time.Second)
	if err := store.RenewJob(context.Background(), "job", control); err != nil {
		t.Fatal(err)
	}
	if err := store.ReserveCall(context.Background(), "job", control, "scope", "gemini", "new-day", 3); err != nil {
		t.Fatal(err)
	}
	if err := store.ReserveCall(context.Background(), "job", control, "scope", "gemini", "new-day", 3); err == nil {
		t.Fatal("attempt dispatched twice")
	}
	if err := store.ReserveCall(context.Background(), "job", control, "scope", "ollama", "disabled", 0); err == nil {
		t.Fatal("missing quota enabled provider")
	}
}
