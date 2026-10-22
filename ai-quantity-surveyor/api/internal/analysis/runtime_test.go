package analysis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countingAnalyzer struct{ calls atomic.Int32 }

func (analyzer *countingAnalyzer) Analyze(_ context.Context, content []byte, fileName string) (Candidate, error) {
	analyzer.calls.Add(1)
	if fileName != "selected-page.png" || len(content) < 8 || string(content[1:4]) != "PNG" {
		return Candidate{}, errors.New("provider did not receive canonical PNG")
	}
	return (DemoAnalyzer{}).Analyze(context.Background(), content, fileName)
}

func waitManaged(t *testing.T, service *Service, id string) Candidate {
	t.Helper()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		candidate, found, err := service.Read(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if found && candidate.State != StateAnalyzing {
			return candidate
		}
		select {
		case <-deadline:
			t.Fatal("managed analysis did not finish")
		case <-ticker.C:
		}
	}
}

func TestManagedAnalysisCanonicalSourceAndQuotaFailure(t *testing.T) {
	analyzer := &countingAnalyzer{}
	service := NewServiceWithEngines(analyzer, map[string]Analyzer{"gemini": analyzer}, nil)
	store := NewMemoryJobStore()
	if err := service.EnableRuntime(RuntimeConfig{Jobs: store, Preparer: NewPagePreparer(), QuotaScope: "test", DailyLimits: map[string]int{"gemini": 1}, DefaultEngine: "gemini"}); err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	content := pageImage(t, 40, 20)
	first, err := service.StartWithPage(context.Background(), "gemini", "drawing", content, "plan.png", nil)
	if err != nil {
		t.Fatal(err)
	}
	ready := waitManaged(t, service, first.ID)
	if ready.State != StateReady || ready.Source == nil || ready.Source.PageNumber != 1 {
		t.Fatalf("wrong managed result: %+v", ready)
	}
	image, found, err := service.SourceImage(context.Background(), first.ID)
	if err != nil || !found || image.Source.ImageDigest != ready.Source.ImageDigest {
		t.Fatal("prepared source not retrievable")
	}
	second, err := service.StartWithPage(context.Background(), "gemini", "drawing", content, "plan.png", nil)
	if err != nil {
		t.Fatal(err)
	}
	failed := waitManaged(t, service, second.ID)
	if failed.State != StateFailed || failed.Error == nil || failed.Error.ErrorCode != "ProviderQuotaExceeded" || failed.Error.RetryAt == nil || analyzer.calls.Load() != 1 {
		t.Fatalf("quota failure dispatched or lost metadata: %+v calls=%d", failed, analyzer.calls.Load())
	}
}

func TestManagedAnalysisMissingQuotaFailsBeforeAcceptance(t *testing.T) {
	analyzer := &countingAnalyzer{}
	service := NewServiceWithEngines(analyzer, map[string]Analyzer{"gemini": analyzer}, nil)
	if err := service.EnableRuntime(RuntimeConfig{Jobs: NewMemoryJobStore(), Preparer: NewPagePreparer(), DefaultEngine: "gemini"}); err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	_, err := service.StartWithPage(context.Background(), "gemini", "drawing", pageImage(t, 20, 10), "plan.png", nil)
	var failure *RuntimeError
	if !errors.As(err, &failure) || failure.Status != 503 || analyzer.calls.Load() != 0 {
		t.Fatalf("invalid quota preflight: %v", err)
	}
}

type terminalFailureStore struct {
	*MemoryJobStore
	failFinish    atomic.Bool
	failRecovery  atomic.Bool
	finishOnce    sync.Once
	finishFailure chan struct{}
}

func (store *terminalFailureStore) FinishJob(ctx context.Context, candidate Candidate, control JobControl) error {
	if store.failFinish.Load() {
		store.finishOnce.Do(func() { close(store.finishFailure) })
		return errors.New("terminal write unavailable")
	}
	return store.MemoryJobStore.FinishJob(ctx, candidate, control)
}

func (store *terminalFailureStore) RecoverExpired(ctx context.Context) error {
	if store.failRecovery.Load() {
		return errors.New("recovery store unavailable")
	}
	return store.MemoryJobStore.RecoverExpired(ctx)
}

type failingPageProcessor struct{}

func (failingPageProcessor) Metadata(context.Context, []byte, string) (PageMetadata, error) {
	return PageMetadata{PageCount: 1, ContentType: "image/png"}, nil
}

func (failingPageProcessor) Prepare(context.Context, []byte, string, int, string) (PreparedPage, error) {
	return PreparedPage{}, errors.New("renderer failed")
}

func TestManagedAnalysisStatusOutageRecoversAfterTerminalWriteFailure(t *testing.T) {
	store := &terminalFailureStore{MemoryJobStore: NewMemoryJobStore(), finishFailure: make(chan struct{})}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	store.clock = func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now }
	store.failFinish.Store(true)
	store.failRecovery.Store(true)
	analyzer := &countingAnalyzer{}
	service := NewServiceWithEngines(analyzer, map[string]Analyzer{"demo": analyzer}, nil)
	if err := service.EnableRuntime(RuntimeConfig{Jobs: store, Preparer: failingPageProcessor{}, DefaultEngine: "demo"}); err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	started, err := service.StartWithPage(context.Background(), "demo", "drawing", pageImage(t, 20, 10), "plan.png", nil)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-store.finishFailure:
	case <-time.After(5 * time.Second):
		t.Fatal("terminal write failure was not attempted")
	}
	_, found, err := service.Read(context.Background(), started.ID)
	var failure *RuntimeError
	if found || !errors.As(err, &failure) || failure.Status != 503 {
		t.Fatalf("status outage should return 503: found=%t err=%v", found, err)
	}
	store.failRecovery.Store(false)
	clockMu.Lock()
	now = now.Add(61 * time.Second)
	clockMu.Unlock()
	failed, found, err := service.Read(context.Background(), started.ID)
	if err != nil || !found || failed.State != StateFailed || failed.Error == nil || !failed.Error.Retryable || analyzer.calls.Load() != 0 {
		t.Fatalf("expired terminal-write failure was not recovered safely: candidate=%+v found=%t err=%v calls=%d", failed, found, err, analyzer.calls.Load())
	}
}
