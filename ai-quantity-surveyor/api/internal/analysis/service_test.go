package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/domain"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestConfirmationCannotForgeSourceOrCorrections(t *testing.T) {
	measurement := &domain.Measurement{Value: 4, Unit: domain.UnitMeters}
	original := Candidate{ID: "analysis-1", FloorPlanID: "floor-1", State: StateReady, Source: &Source{FloorPlanID: "floor-1", PageNumber: 2, ImageDigest: "server-digest"}, Rooms: []Room{{ID: "room-1", Name: "Living", Length: measurement, Width: measurement, WallHeight: measurement, Confidence: 1}}}
	service := NewService(DemoAnalyzer{})
	service.results[original.ID] = original
	editable, ok := service.Get(original.ID)
	if !ok {
		t.Fatal("candidate unavailable")
	}
	editable.Source.PageNumber = 3
	if _, err := service.Confirm(original.ID, editable); err == nil || !strings.Contains(err.Error(), "source identity") {
		t.Fatalf("forged source accepted: %v", err)
	}
	stored, _ := service.Get(original.ID)
	if stored.Source.PageNumber != 2 {
		t.Fatal("reader mutated stored source")
	}
	stored.Corrections = []Correction{{EntityID: "room-1", Field: "length", OriginalValue: json.RawMessage(`999`), ConfirmedValue: json.RawMessage(`4`), Origin: "user"}}
	stored.Error = &AnalysisError{ErrorCode: "Forged", Message: "client error"}
	confirmed, err := service.Confirm(original.ID, stored)
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmed.Corrections) != 0 || confirmed.Error != nil || confirmed.Source.ImageDigest != "server-digest" {
		t.Fatalf("client changed provenance: %+v", confirmed)
	}
}

func TestConfirmationDerivesMeasurementAuditAndPreservesEvidence(t *testing.T) {
	measure := func(value float64) *domain.Measurement {
		return &domain.Measurement{Value: value, Unit: domain.UnitMeters}
	}
	original := Candidate{ID: "audit-analysis", FloorPlanID: "audit-floor", State: StateReady, Source: &Source{FloorPlanID: "audit-floor", PageNumber: 2}, ReviewIssues: []ReviewIssue{{EntityID: "room-1", Field: "width", Code: "NeedsReview", Message: "Review raw extraction"}}, Rooms: []Room{{ID: "room-1", Name: "Hall", Length: measure(4), Width: measure(3), WallHeight: measure(2.7), Confidence: 0.75, Uncertainty: "Engine uncertainty", Evidence: map[string]Evidence{"length": {AnalysisID: "audit-analysis", FloorPlanID: "audit-floor", PageNumber: 2, Origin: "engine", ObservedText: "4 m"}}}}}
	service := NewService(DemoAnalyzer{})
	service.results[original.ID] = original
	editable, _ := service.Get(original.ID)
	editable.Rooms[0].Length = measure(5)
	editable.Rooms[0].Evidence["length"] = Evidence{Origin: "manual", ObservedText: "forged"}
	if _, err := service.Confirm(original.ID, editable); err == nil || !strings.Contains(err.Error(), "invalid manual source evidence") {
		t.Fatalf("invalid manual evidence was accepted: %v", err)
	}
	editable, _ = service.Get(original.ID)
	editable.Rooms[0].Length = measure(5)
	editable.ReviewIssues = []ReviewIssue{{EntityID: "room-1", Code: "Forged", Message: "Client supplied"}}
	confirmed, err := service.Confirm(original.ID, editable)
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmed.Corrections) != 1 || confirmed.Corrections[0].Field != "length" || string(confirmed.Corrections[0].OriginalValue) != `{"value":4,"unit":"m"}` || confirmed.Rooms[0].Evidence["length"].ObservedText != "4 m" {
		t.Fatalf("audit or evidence wrong: %+v", confirmed)
	}
	if len(confirmed.ReviewIssues) != 1 || confirmed.ReviewIssues[0].Code != "NeedsReview" || confirmed.Rooms[0].Confidence != 0.75 || confirmed.Rooms[0].Uncertainty != "Engine uncertainty" {
		t.Fatalf("client replaced engine review metadata: %+v", confirmed)
	}
	stored, _ := service.Get(original.ID)
	if stored.Rooms[0].Length.Value != 4 {
		t.Fatal("confirmation rewrote raw extraction")
	}
	confirmed.Rooms[0].Evidence["length"] = Evidence{ObservedText: "caller mutation"}
	stored, _ = service.Get(original.ID)
	if stored.Rooms[0].Evidence["length"].ObservedText != "4 m" {
		t.Fatal("confirmation response shared raw engine evidence")
	}
}

func TestConfirmationAcceptsOnlyBoundManualEvidenceAndPreservesOpeningMetadata(t *testing.T) {
	measure := func(value float64) *domain.Measurement {
		return &domain.Measurement{Value: value, Unit: domain.UnitMeters}
	}
	original := Candidate{ID: "manual-evidence-analysis", FloorPlanID: "manual-drawing", State: StateReady, Source: &Source{FloorPlanID: "manual-drawing", PageNumber: 3}, Rooms: []Room{{ID: "room", Name: "Hall", Length: measure(4), Width: measure(3), WallHeight: measure(2.7), Confidence: 0.7}}, Windows: []Opening{{ID: "window", RoomID: "room", Width: measure(1.2), Height: measure(1), Confidence: 0.6, Uncertainty: "Engine read uncertain"}}}
	service := NewService(DemoAnalyzer{})
	service.results[original.ID] = original
	editable, _ := service.Get(original.ID)
	confidence := 1.0
	editable.Rooms[0].Confidence = confidence
	editable.Rooms[0].Uncertainty = "client override"
	editable.Rooms[0].Evidence = map[string]Evidence{"length": {AnalysisID: original.ID, FloorPlanID: original.FloorPlanID, PageNumber: 3, Region: &Bounds{X: 0.1, Y: 0.2, Width: 0.2, Height: 0.1}, ObservedText: "4 m", Origin: "manual"}}
	editable.Windows[0].Confidence = confidence
	editable.Windows[0].Uncertainty = "client override"
	confirmed, err := service.Confirm(original.ID, editable)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Rooms[0].Confidence != 0.7 || confirmed.Rooms[0].Uncertainty != "" {
		t.Fatalf("room engine confidence/uncertainty was changed: %+v", confirmed.Rooms[0])
	}
	if confirmed.Windows[0].Confidence != 0.6 || confirmed.Windows[0].Uncertainty != "Engine read uncertain" {
		t.Fatalf("opening engine confidence/uncertainty was changed: %+v", confirmed.Windows[0])
	}
	manual := confirmed.Rooms[0].Evidence["length"]
	if manual.Origin != "manual" || manual.AnalysisID != original.ID || manual.FloorPlanID != original.FloorPlanID || manual.PageNumber != 3 || manual.Confidence != nil || manual.Region == nil {
		t.Fatalf("manual evidence was not source-bound: %+v", manual)
	}
	if len(confirmed.Corrections) != 1 || confirmed.Corrections[0].Field != "lengthEvidence" || string(confirmed.Corrections[0].OriginalValue) != "null" {
		t.Fatalf("manual source mark was not audited: %+v", confirmed.Corrections)
	}
}

func TestConfirmationRequiresFeatureReviewAndKeepsManualConfidenceUnknown(t *testing.T) {
	measure := &domain.Measurement{Value: 4, Unit: domain.UnitMeters}
	original := Candidate{ID: "feature-analysis", FloorPlanID: "drawing", State: StateReady, Rooms: []Room{{ID: "room", Name: "Hall", Length: measure, Width: measure, WallHeight: measure, Confidence: 1}}, Features: []Feature{{ID: "stair", Type: "stair", Label: "Stair", Origin: "engine", Disposition: "pending"}}}
	service := NewService(DemoAnalyzer{})
	service.results[original.ID] = original
	editable, _ := service.Get(original.ID)
	if _, err := service.Confirm(original.ID, editable); err == nil {
		t.Fatal("pending feature confirmed")
	}
	editable.Features = nil
	if _, err := service.Confirm(original.ID, editable); err == nil {
		t.Fatal("omission bypassed feature review")
	}
	confidence := 1.0
	editable.Features = []Feature{{ID: "stair", Type: "stair", Label: "Stair", Disposition: "removed"}, {ID: "balcony", Type: "balcony", Label: "Manual balcony", Disposition: "acknowledged", Confidence: &confidence}}
	confirmed, err := service.Confirm(original.ID, editable)
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmed.Corrections) != 2 || confirmed.Features[1].Origin != "manual" || confirmed.Features[1].Confidence != nil {
		t.Fatalf("feature audit: %+v", confirmed)
	}
}

func TestConfirmationIsNotPublishedWhenRevisionPersistenceFails(t *testing.T) {
	measurement := &domain.Measurement{Value: 4, Unit: domain.UnitMeters}
	original := Candidate{ID: "failed-audit-analysis", FloorPlanID: "drawing", State: StateReady, Rooms: []Room{{ID: "room", Name: "Hall", Length: measurement, Width: measurement, WallHeight: measurement, Confidence: 1}}}
	repository := &revisionRepository{candidate: original, failConfirmation: true}
	service := NewServiceWithRepository(DemoAnalyzer{}, repository)
	pending, _ := service.Get(original.ID)
	pending.Rooms[0].Length = &domain.Measurement{Value: 5, Unit: domain.UnitMeters}
	if _, err := service.Confirm(original.ID, pending); err == nil {
		t.Fatal("confirmation succeeded despite failed audit persistence")
	}
	if len(service.Confirmations(original.ID)) != 0 {
		t.Fatal("failed revision was published in confirmation history")
	}
	if _, found := service.Confirmed(original.ID); found {
		t.Fatal("failed revision became the confirmed result")
	}
	stored, found := service.Get(original.ID)
	if !found || stored.Rooms[0].Length.Value != 4 {
		t.Fatal("failed confirmation mutated the raw candidate")
	}
}

func TestPostgresConfirmationAuditReopensAndLegacyConfirmationRemainsReadable(t *testing.T) {
	connection := os.Getenv("SURVEYOR_JOB_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("disposable confirmation test database not configured")
	}
	parsed, err := url.Parse(connection)
	if err != nil || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "surveyor_job_test") {
		t.Fatal("confirmation tests require a disposable surveyor_job_test database")
	}
	open := func() (*sql.DB, *PostgresRepository) {
		db, err := sql.Open("pgx", connection)
		if err != nil {
			t.Fatal("could not open disposable confirmation database")
		}
		db.SetMaxOpenConns(4)
		repository, err := NewPostgresRepository(db)
		if err != nil {
			db.Close()
			t.Fatal(err)
		}
		return db, repository
	}
	db, repository := open()
	original := Candidate{ID: "confirm-" + newID(), FloorPlanID: "drawing-" + newID(), State: StateReady, Source: &Source{FloorPlanID: "confirmation-drawing", PageNumber: 2, ImageDigest: "original-digest"}, ReviewIssues: []ReviewIssue{{EntityID: "room", Field: "length", Code: "Review", Message: "Check source"}}, Rooms: []Room{{ID: "room", Name: "Hall", Length: &domain.Measurement{Value: 4, Unit: domain.UnitMeters}, Width: &domain.Measurement{Value: 3, Unit: domain.UnitMeters}, WallHeight: &domain.Measurement{Value: 2.7, Unit: domain.UnitMeters}, Confidence: 0.75}}}
	original.Source.FloorPlanID = original.FloorPlanID
	if err := repository.Save(original, false); err != nil {
		db.Close()
		t.Fatal(err)
	}
	service := NewServiceWithRepository(DemoAnalyzer{}, repository)
	pending, found := service.Get(original.ID)
	if !found {
		db.Close()
		t.Fatal("raw PostgreSQL candidate unavailable")
	}
	pending.Rooms[0].Length = &domain.Measurement{Value: 5, Unit: domain.UnitMeters}
	pending.Rooms[0].Confidence = 1
	pending.ReviewIssues = []ReviewIssue{{EntityID: "forged", Code: "Forged"}}
	confirmed, err := service.Confirm(original.ID, pending)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if confirmed.ConfirmationRevision != 1 || confirmed.Rooms[0].Confidence != 0.75 || len(confirmed.ReviewIssues) != 1 || confirmed.ReviewIssues[0].Code != "Review" || len(confirmed.Corrections) != 1 {
		db.Close()
		t.Fatalf("confirmation audit response: %+v", confirmed)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, repository = open()
	defer db.Close()
	stored, found := repository.ConfirmationRevision(original.ID, 1)
	if !found || stored.Candidate.Rooms[0].Length.Value != 5 || stored.Candidate.Rooms[0].Confidence != 0.75 || stored.Candidate.Corrections[0].Field != "length" || stored.Candidate.ReviewIssues[0].Code != "Review" {
		t.Fatalf("confirmation audit did not survive reconnect: %+v", stored)
	}
	legacy := Candidate{ID: "legacy-confirm-" + newID(), FloorPlanID: "legacy-drawing", State: StateReady, Rooms: []Room{{ID: "legacy-room", Name: "Legacy", Length: &domain.Measurement{Value: 4, Unit: domain.UnitMeters}, Width: &domain.Measurement{Value: 3, Unit: domain.UnitMeters}, WallHeight: &domain.Measurement{Value: 2.7, Unit: domain.UnitMeters}, Confidence: 1}}}
	if err := repository.Save(legacy, true); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, repository = open()
	legacySnapshot, found := repository.ConfirmationRevision(legacy.ID, 1)
	if !found || legacySnapshot.Revision != 1 || legacySnapshot.Candidate.Rooms[0].Name != "Legacy" {
		db.Close()
		t.Fatalf("legacy confirmation migration/read failed: %+v", legacySnapshot)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEngineEvidenceRejectsUnsupportedFields(t *testing.T) {
	candidate := Candidate{ID: "evidence-analysis", FloorPlanID: "drawing", Source: &Source{FloorPlanID: "drawing", PageNumber: 2}, Rooms: []Room{{ID: "room", Name: "Hall", Evidence: map[string]Evidence{"length": {AnalysisID: "evidence-analysis", FloorPlanID: "drawing", PageNumber: 2, Origin: "engine"}, "externalQuantity": {AnalysisID: "evidence-analysis", FloorPlanID: "drawing", PageNumber: 2, Origin: "engine"}}}}}
	NormalizeEngineEvidence(&candidate)
	if len(candidate.Rooms[0].Evidence) != 1 || candidate.Rooms[0].Evidence["length"].Origin != "engine" || len(candidate.ReviewIssues) != 1 || candidate.ReviewIssues[0].Code != "UnsupportedEvidence" {
		t.Fatalf("unsupported provenance was retained: %+v", candidate)
	}
}

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
	mu               sync.Mutex
	candidate        Candidate
	snapshots        []Confirmation
	failConfirmation bool
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
	if repository.failConfirmation {
		return errors.New("confirmation persistence failed")
	}
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

func (repository *revisionRepository) Confirmations(id string) []Confirmation {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	var result []Confirmation
	for _, snapshot := range repository.snapshots {
		if snapshot.AnalysisID == id {
			result = append(result, snapshot)
		}
	}
	return result
}

func waitReady(t *testing.T, service *Service, id string) Candidate {
	t.Helper()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if candidate, ok := service.Get(id); ok && candidate.State == StateReady {
			return candidate
		}
	}
	t.Fatal("analysis did not complete")
	return Candidate{}
}

func (analyzer namedAnalyzer) ModelName() string { return string(analyzer) + "-model" }

func TestAnalysisRecordsEngineModelTimingAndHistory(t *testing.T) {
	service := NewServiceWithEngines(DemoAnalyzer{}, map[string]Analyzer{"demo": DemoAnalyzer{}, "gemini": namedAnalyzer("gemini")}, nil)
	first, err := service.StartWithEngine("demo", "floor-plan-1", []byte("drawing"), "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	ready := waitReady(t, service, first.ID)
	if ready.Engine != "demo" || ready.Model != "demo" || ready.StartedAt == nil || ready.CompletedAt == nil || ready.CompletedAt.Before(*ready.StartedAt) {
		t.Fatalf("missing analysis metadata: %+v", ready)
	}
	ready.Rooms[0].Confidence, ready.Rooms[0].Uncertainty = 1, ""
	ready.Rooms[0].Assumptions = []string{"wallHeight", "invented"}
	confirmed, err := service.Confirm(first.ID, ready)
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmed.Rooms[0].Assumptions) != 1 || confirmed.Rooms[0].Assumptions[0] != "wallHeight" || confirmed.Engine != "demo" {
		t.Fatalf("confirmation did not keep metadata and known assumptions: %+v", confirmed.Rooms[0])
	}
	ready.Rooms = ready.Rooms[:2]
	if _, err := service.Confirm(first.ID, ready); err != nil {
		t.Fatal(err)
	}
	history := service.Confirmations(first.ID)
	if len(history) != 2 || history[0].Revision != 1 || len(history[1].Candidate.Rooms) != 2 || history[0].ConfirmedAt.IsZero() {
		t.Fatalf("unexpected confirmation history: %+v", history)
	}

	time.Sleep(2 * time.Millisecond)
	second, err := service.StartWithEngine("gemini", "floor-plan-1", []byte("drawing"), "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	waitReady(t, service, second.ID)
	latest, ok := service.LatestForFloorPlan("floor-plan-1")
	if !ok || latest.ID != second.ID || latest.Model != "gemini-model" {
		t.Fatalf("latest analysis = %+v", latest)
	}
	if _, ok := service.LatestForFloorPlan("other"); ok {
		t.Fatal("unexpected analysis for unknown floor plan")
	}
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
