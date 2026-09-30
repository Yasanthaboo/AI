package analysis

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartStoresAnalysisAndCompletesWithFakeAnalyzer(t *testing.T) {
	service := NewService(DemoAnalyzer{})
	started, err := service.Start("floor-plan-1", []byte("drawing"), "plan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		result, ok := service.Get(started.ID)
		if ok && result.State != StateAnalyzing {
			if result.State == StateFailed {
				t.Fatal(result.ErrorMessage)
			}
			if result.State != StateReady {
				t.Fatalf("state = %q, want %q", result.State, StateReady)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("analysis did not complete")
}

type namedAnalyzer string

func (analyzer namedAnalyzer) Analyze(_ context.Context, _ []byte, _ string) (Candidate, error) {
	return Candidate{State: StateReady, Rooms: []Room{{ID: "room-1", Name: string(analyzer)}}}, nil
}

func TestStartWithEngineRoutesEachRequest(t *testing.T) {
	service := NewServiceWithEngines(namedAnalyzer("demo"), map[string]Analyzer{"demo": namedAnalyzer("demo"), "gemini": namedAnalyzer("gemini")}, nil)
	for _, engine := range []string{"demo", "gemini"} {
		started, err := service.StartWithEngine(engine, "floor-plan-1", []byte("drawing"), "plan.png")
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(time.Second)
		for {
			result, ok := service.Get(started.ID)
			if ok && result.State != StateAnalyzing {
				if result.State != StateReady || len(result.Rooms) != 1 || result.Rooms[0].Name != engine {
					t.Fatalf("engine %q result: %+v", engine, result)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("analysis did not complete")
			}
			time.Sleep(time.Millisecond)
		}
	}
	if _, err := service.StartWithEngine("unknown", "floor-plan-1", []byte("drawing"), "plan.png"); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unavailable engine error, got %v", err)
	}
}

type incompleteAnalyzer struct{}

func (incompleteAnalyzer) Analyze(_ context.Context, _ []byte, _ string) (Candidate, error) {
	return Candidate{State: StateReady, Rooms: []Room{{ID: "room-1", Name: "Room 1"}}}, nil
}

func TestStartKeepsIncompleteExtractionForReview(t *testing.T) {
	service := NewService(incompleteAnalyzer{})
	started, err := service.Start("floor-plan-1", []byte("drawing"), "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		result, ok := service.Get(started.ID)
		if ok && result.State != StateAnalyzing {
			if result.State != StateReady || result.Rooms[0].Length != nil {
				t.Fatalf("unexpected review result: %+v", result)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("analysis did not complete")
}

func TestConfirmPreservesDistinctPreviewRevisions(t *testing.T) {
	service := NewService(DemoAnalyzer{})
	started, err := service.Start("floor-plan-1", []byte("drawing"), "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if candidate, ok := service.Get(started.ID); ok && candidate.State == StateReady {
			break
		}
		time.Sleep(time.Millisecond)
	}
	confirmed, ok := service.Get(started.ID)
	if !ok || confirmed.State != StateReady {
		t.Fatal("analysis did not complete")
	}
	confirmed.Rooms[0].Name = "First"
	confirmed.Rooms[0].Confidence = 1
	confirmed.Rooms[0].Uncertainty = ""
	if _, err := service.Confirm(started.ID, confirmed); err != nil {
		t.Fatal(err)
	}
	confirmed.Rooms[0].Name = "Revised"
	if _, err := service.Confirm(started.ID, confirmed); err != nil {
		t.Fatal(err)
	}
	first, ok := service.ConfirmationRevision(started.ID, 1)
	latest, exists := service.LatestConfirmation(started.ID)
	if !ok || !exists || first.Candidate.Rooms[0].Name != "First" || latest.Revision != 2 || latest.Candidate.Rooms[0].Name != "Revised" {
		t.Fatalf("unexpected confirmation history: first=%+v latest=%+v", first, latest)
	}
	first.Candidate.Rooms[0].Name = "tampered"
	first.Candidate.Rooms[0].Length.Value = 999
	again, ok := service.ConfirmationRevision(started.ID, 1)
	if !ok || again.Candidate.Rooms[0].Name != "First" || again.Candidate.Rooms[0].Length.Value != 4 {
		t.Fatalf("revision was mutated by a reader: %+v", again)
	}
}

type revisionRepository struct {
	mu        sync.Mutex
	candidate Candidate
	snapshots []Confirmation
}

func (repository *revisionRepository) Save(candidate Candidate, confirmed bool) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if !confirmed {
		repository.candidate = candidate
	}
	return nil
}

func (repository *revisionRepository) Get(id string) (Candidate, bool) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.candidate, repository.candidate.ID == id
}

func (repository *revisionRepository) Confirmed(id string) (Candidate, bool) {
	snapshot, ok := repository.LatestConfirmation(id)
	return snapshot.Candidate, ok
}

func (repository *revisionRepository) SaveConfirmation(snapshot Confirmation) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	copy, err := copyCandidate(snapshot.Candidate)
	if err != nil {
		return err
	}
	snapshot.Candidate = copy
	repository.snapshots = append(repository.snapshots, snapshot)
	return nil
}

func (repository *revisionRepository) LatestConfirmation(id string) (Confirmation, bool) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if len(repository.snapshots) == 0 || repository.snapshots[len(repository.snapshots)-1].AnalysisID != id {
		return Confirmation{}, false
	}
	snapshot := repository.snapshots[len(repository.snapshots)-1]
	snapshot.Candidate, _ = copyCandidate(snapshot.Candidate)
	return snapshot, true
}

func (repository *revisionRepository) ConfirmationRevision(id string, revision int) (Confirmation, bool) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, snapshot := range repository.snapshots {
		if snapshot.AnalysisID == id && snapshot.Revision == revision {
			snapshot.Candidate, _ = copyCandidate(snapshot.Candidate)
			return snapshot, true
		}
	}
	return Confirmation{}, false
}

func TestConfirmationRevisionsSurviveServiceRestart(t *testing.T) {
	repository := &revisionRepository{}
	service := NewServiceWithRepository(DemoAnalyzer{}, repository)
	started, err := service.Start("floor-plan-1", []byte("drawing"), "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		candidate, ok := service.Get(started.ID)
		if ok && candidate.State == StateReady {
			candidate.Rooms[0].Confidence = 1
			candidate.Rooms[0].Uncertainty = ""
			if _, err := service.Confirm(started.ID, candidate); err != nil {
				t.Fatal(err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("analysis did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	restarted := NewServiceWithRepository(DemoAnalyzer{}, repository)
	first, ok := restarted.LatestConfirmation(started.ID)
	if !ok || first.Revision != 1 {
		t.Fatalf("missing confirmed snapshot after restart: %+v", first)
	}
	first.Candidate.Rooms[0].Name = "Second"
	if _, err := restarted.Confirm(started.ID, first.Candidate); err != nil {
		t.Fatal(err)
	}
	previous, ok := restarted.ConfirmationRevision(started.ID, 1)
	current, exists := restarted.LatestConfirmation(started.ID)
	if !ok || !exists || previous.Candidate.Rooms[0].Name == current.Candidate.Rooms[0].Name || current.Revision != 2 {
		t.Fatalf("restarted confirmation lost history: previous=%+v current=%+v", previous, current)
	}
}
