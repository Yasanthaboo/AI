package analysis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/domain"
)

type Service struct {
	mu         sync.RWMutex
	analyzer   Analyzer
	engines    map[string]Analyzer
	results    map[string]Candidate
	confirmed  map[string]Candidate
	snapshots  map[string][]Confirmation
	repository Repository
}

type Confirmation struct {
	AnalysisID  string    `json:"analysisId"`
	Revision    int       `json:"revision"`
	ConfirmedAt time.Time `json:"confirmedAt"`
	Candidate   Candidate `json:"candidate"`
}

type Repository interface {
	Save(Candidate, bool) error
	Get(string) (Candidate, bool)
	Confirmed(string) (Candidate, bool)
}

func NewService(analyzer Analyzer) *Service {
	return &Service{analyzer: analyzer, results: map[string]Candidate{}, confirmed: map[string]Candidate{}, snapshots: map[string][]Confirmation{}}
}

func NewServiceWithRepository(analyzer Analyzer, repository Repository) *Service {
	service := NewService(analyzer)
	service.repository = repository
	return service
}

func NewServiceWithEngines(defaultAnalyzer Analyzer, engines map[string]Analyzer, repository Repository) *Service {
	service := NewService(defaultAnalyzer)
	service.engines = engines
	service.repository = repository
	return service
}

func (service *Service) Start(floorPlanID string, content []byte, fileName string) (Candidate, error) {
	return service.start("", service.analyzer, floorPlanID, content, fileName)
}

func (service *Service) StartWithEngine(engine, floorPlanID string, content []byte, fileName string) (Candidate, error) {
	analyzer, ok := service.engines[engine]
	if !ok || analyzer == nil {
		return Candidate{}, fmt.Errorf("analysis engine %q is not available", engine)
	}
	return service.start(engine, analyzer, floorPlanID, content, fileName)
}

func (service *Service) start(engine string, analyzer Analyzer, floorPlanID string, content []byte, fileName string) (Candidate, error) {
	if floorPlanID == "" || len(content) == 0 {
		return Candidate{}, fmt.Errorf("floor plan content is required")
	}
	now := time.Now().UTC()
	candidate := Candidate{ID: newID(), FloorPlanID: floorPlanID, State: StateAnalyzing, Engine: engine, StartedAt: &now}
	if namer, ok := analyzer.(ModelNamer); ok {
		candidate.Model = namer.ModelName()
	}
	service.mu.Lock()
	service.results[candidate.ID] = candidate
	service.mu.Unlock()
	if service.repository != nil {
		_ = service.repository.Save(candidate, false)
	}
	go service.run(analyzer, candidate, content, fileName)
	return candidate, nil
}

func (service *Service) Get(id string) (Candidate, bool) {
	service.mu.RLock()
	candidate, ok := service.results[id]
	service.mu.RUnlock()
	if !ok && service.repository != nil {
		return service.repository.Get(id)
	}
	return candidate, ok
}

func (service *Service) Confirm(id string, candidate Candidate) (Candidate, error) {
	if err := Validate(candidate); err != nil {
		return Candidate{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	original, ok := service.results[id]
	if !ok && service.repository != nil {
		original, ok = service.repository.Get(id)
	}
	if !ok {
		return Candidate{}, fmt.Errorf("analysis %q not found", id)
	}
	if original.State != StateReady {
		return Candidate{}, fmt.Errorf("analysis %q is not ready for confirmation", id)
	}
	candidate.ID = original.ID
	candidate.FloorPlanID = original.FloorPlanID
	candidate.DrawingAspect = original.DrawingAspect
	candidate.Engine, candidate.Model = original.Engine, original.Model
	candidate.StartedAt, candidate.CompletedAt = original.StartedAt, original.CompletedAt
	sanitizeBounds(&candidate)
	sanitizeAssumptions(&candidate)
	revision := len(service.snapshots[id]) + 1
	if repository, ok := service.repository.(ConfirmationRepository); ok {
		if latest, exists := repository.LatestConfirmation(id); exists && latest.Revision >= revision {
			revision = latest.Revision + 1
		}
	}
	candidate.ConfirmationRevision = revision
	stored, err := copyCandidate(candidate)
	if err != nil {
		return Candidate{}, err
	}
	snapshot := Confirmation{AnalysisID: id, Revision: revision, ConfirmedAt: time.Now().UTC(), Candidate: stored}
	if repository, ok := service.repository.(ConfirmationRepository); ok {
		if err := repository.SaveConfirmation(snapshot); err != nil {
			return Candidate{}, err
		}
	}
	service.confirmed[id] = stored
	service.snapshots[id] = append(service.snapshots[id], snapshot)
	if service.repository != nil {
		if _, ok := service.repository.(ConfirmationRepository); !ok {
			if err := service.repository.Save(candidate, true); err != nil {
				return Candidate{}, err
			}
		}
	}
	return candidate, nil
}

type ConfirmationRepository interface {
	SaveConfirmation(Confirmation) error
	LatestConfirmation(string) (Confirmation, bool)
	ConfirmationRevision(string, int) (Confirmation, bool)
	Confirmations(string) []Confirmation
}

// FloorPlanRepository finds the most recently started ready analysis of a floor plan.
type FloorPlanRepository interface {
	LatestForFloorPlan(string) (Candidate, bool)
}

// Confirmations returns every confirmation revision of an analysis, oldest first.
func (service *Service) Confirmations(id string) []Confirmation {
	if repository, ok := service.repository.(ConfirmationRepository); ok {
		return repository.Confirmations(id)
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	result := make([]Confirmation, 0, len(service.snapshots[id]))
	for _, snapshot := range service.snapshots[id] {
		if candidate, err := copyCandidate(snapshot.Candidate); err == nil {
			snapshot.Candidate = candidate
			result = append(result, snapshot)
		}
	}
	return result
}

// LatestForFloorPlan returns the most recently started ready analysis of a floor plan.
func (service *Service) LatestForFloorPlan(floorPlanID string) (Candidate, bool) {
	if repository, ok := service.repository.(FloorPlanRepository); ok {
		return repository.LatestForFloorPlan(floorPlanID)
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	var matches []Candidate
	for _, candidate := range service.results {
		if candidate.FloorPlanID == floorPlanID && candidate.State == StateReady {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return Candidate{}, false
	}
	sort.Slice(matches, func(left, right int) bool { return startedAt(matches[left]).After(startedAt(matches[right])) })
	return matches[0], true
}

func startedAt(candidate Candidate) time.Time {
	if candidate.StartedAt == nil {
		return time.Time{}
	}
	return *candidate.StartedAt
}

func (service *Service) LatestConfirmation(id string) (Confirmation, bool) {
	service.mu.RLock()
	snapshots := service.snapshots[id]
	service.mu.RUnlock()
	if len(snapshots) > 0 {
		candidate, err := copyCandidate(snapshots[len(snapshots)-1].Candidate)
		if err == nil {
			return Confirmation{AnalysisID: id, Revision: snapshots[len(snapshots)-1].Revision, Candidate: candidate}, true
		}
	}
	if repository, ok := service.repository.(ConfirmationRepository); ok {
		return repository.LatestConfirmation(id)
	}
	return Confirmation{}, false
}

func (service *Service) ConfirmationRevision(id string, revision int) (Confirmation, bool) {
	service.mu.RLock()
	for _, snapshot := range service.snapshots[id] {
		if snapshot.Revision == revision {
			service.mu.RUnlock()
			candidate, err := copyCandidate(snapshot.Candidate)
			if err == nil {
				return Confirmation{AnalysisID: id, Revision: revision, Candidate: candidate}, true
			}
			return Confirmation{}, false
		}
	}
	service.mu.RUnlock()
	if repository, ok := service.repository.(ConfirmationRepository); ok {
		return repository.ConfirmationRevision(id, revision)
	}
	return Confirmation{}, false
}

func (service *Service) Confirmed(id string) (Candidate, bool) {
	service.mu.RLock()
	candidate, ok := service.confirmed[id]
	service.mu.RUnlock()
	if !ok && service.repository != nil {
		if repository, ok := service.repository.(ConfirmationRepository); ok {
			if snapshot, exists := repository.LatestConfirmation(id); exists {
				return snapshot.Candidate, true
			}
		}
		return service.repository.Confirmed(id)
	}
	return candidate, ok
}

func copyCandidate(candidate Candidate) (Candidate, error) {
	encoded, err := json.Marshal(candidate)
	if err != nil {
		return Candidate{}, err
	}
	var result Candidate
	if err := json.Unmarshal(encoded, &result); err != nil {
		return Candidate{}, err
	}
	return result, nil
}

func (service *Service) run(analyzer Analyzer, started Candidate, content []byte, fileName string) {
	id, floorPlanID := started.ID, started.FloorPlanID
	candidate, err := analyzer.Analyze(context.Background(), content, fileName)
	if err != nil {
		candidate = Candidate{ID: id, FloorPlanID: floorPlanID, State: StateFailed, ErrorMessage: err.Error()}
	} else {
		candidate.ID = id
		candidate.FloorPlanID = floorPlanID
		if candidate.DrawingAspect == 0 {
			candidate.DrawingAspect = drawingAspect(content)
		}
		sanitizeBounds(&candidate)
		// Assumptions are recorded only by the user at confirmation, never by an engine.
		for index := range candidate.Rooms {
			candidate.Rooms[index].Assumptions = nil
		}
	}
	completed := time.Now().UTC()
	candidate.Engine, candidate.Model, candidate.StartedAt, candidate.CompletedAt = started.Engine, started.Model, started.StartedAt, &completed
	service.mu.Lock()
	service.results[id] = candidate
	service.mu.Unlock()
	if service.repository != nil {
		_ = service.repository.Save(candidate, false)
	}
}

type DemoAnalyzer struct{}

func (DemoAnalyzer) ModelName() string { return "demo" }

func (DemoAnalyzer) Analyze(_ context.Context, _ []byte, _ string) (Candidate, error) {
	meters := func(value float64) *domain.Measurement {
		return &domain.Measurement{Value: value, Unit: domain.UnitMeters}
	}
	room := func(id, name string, length, width float64, bounds Bounds) Room {
		return Room{ID: id, Name: name, Confidence: 1, Length: meters(length), Width: meters(width), WallHeight: meters(2.8), Bounds: &bounds}
	}
	living := room("room-1", "Living room", 4, 5, Bounds{X: 0.05, Y: 0.1, Width: 0.4, Height: 0.5})
	living.Confidence = 0.72
	living.Uncertainty = "Confirm dimensions against drawing"
	return Candidate{State: StateReady, DrawingAspect: 1, Rooms: []Room{
		living,
		room("room-2", "Kitchen", 3.5, 3, Bounds{X: 0.45, Y: 0.1, Width: 0.35, Height: 0.3}),
		room("room-3", "Bathroom", 2, 2, Bounds{X: 0.45, Y: 0.4, Width: 0.2, Height: 0.2}),
		room("room-4", "Bedroom", 4, 3, Bounds{X: 0.05, Y: 0.6, Width: 0.4, Height: 0.3}),
	}, Doors: []Opening{
		{ID: "door-1", RoomID: "room-1", Width: meters(0.9), Height: meters(2.1), Confidence: 1},
	}, Windows: []Opening{
		{ID: "window-1", RoomID: "room-2", Width: meters(1.2), Height: meters(1.2), Confidence: 1},
	}}, nil
}

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
