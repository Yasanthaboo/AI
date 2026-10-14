package analysis

import (
	"context"
	"errors"
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
