package estimate

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/domain"
)

type Request struct {
	AnalysisID                string                       `json:"analysisId"`
	PlasterUSDPerSquareMeter  float64                      `json:"plasterUsdPerSquareMeter"`
	PaintUSDPerSquareMeter    float64                      `json:"paintUsdPerSquareMeter"`
	FlooringUSDPerSquareMeter float64                      `json:"flooringUsdPerSquareMeter"`
	CeilingUSDPerSquareMeter  float64                      `json:"ceilingUsdPerSquareMeter"`
	SkirtingUSDPerMeter       float64                      `json:"skirtingUsdPerMeter"`
	TilingUSDPerSquareMeter   float64                      `json:"tilingUsdPerSquareMeter"`
	WastePercent              float64                      `json:"wastePercent"`
	Finishes                  map[string]domain.RoomFinish `json:"finishes"`
	TargetCurrency            string                       `json:"targetCurrency"`
	ExchangeRate              float64                      `json:"exchangeRate"`
	ExchangeRateFetchedAt     string                       `json:"exchangeRateFetchedAt"`
}

func (request Request) rates() domain.Rates {
	return domain.Rates{
		PlasterUSDPerSquareMeter: request.PlasterUSDPerSquareMeter, PaintUSDPerSquareMeter: request.PaintUSDPerSquareMeter,
		FlooringUSDPerSquareMeter: request.FlooringUSDPerSquareMeter, CeilingUSDPerSquareMeter: request.CeilingUSDPerSquareMeter,
		SkirtingUSDPerMeter: request.SkirtingUSDPerMeter, TilingUSDPerSquareMeter: request.TilingUSDPerSquareMeter, WastePercent: request.WastePercent,
	}
}

type Version struct {
	ID                    string                       `json:"id"`
	ProjectID             string                       `json:"projectId"`
	Version               int                          `json:"version"`
	SourceAnalysisID      string                       `json:"sourceAnalysisId"`
	TargetCurrency        string                       `json:"targetCurrency"`
	ExchangeRate          float64                      `json:"exchangeRate"`
	ExchangeRateFetchedAt string                       `json:"exchangeRateFetchedAt"`
	Rates                 *domain.Rates                `json:"rates,omitempty"`
	Finishes              map[string]domain.RoomFinish `json:"finishes,omitempty"`
	Result                domain.CalculationResult     `json:"result"`
	ConvertedPlasterCost  float64                      `json:"convertedPlasterCost"`
	ConvertedPaintCost    float64                      `json:"convertedPaintCost"`
	ConvertedGrandTotal   float64                      `json:"convertedGrandTotal"`
	CalculatedAt          time.Time                    `json:"calculatedAt"`
}

type Service struct {
	mu         sync.RWMutex
	versions   map[string][]Version
	analysis   *analysis.Service
	repository Repository
}

type Repository interface {
	Save(Version) error
	Get(string) (Version, bool)
	List(string) []Version
}

func NewService(analysisService *analysis.Service) *Service {
	return &Service{versions: map[string][]Version{}, analysis: analysisService}
}

func NewServiceWithRepository(analysisService *analysis.Service, repository Repository) *Service {
	service := NewService(analysisService)
	service.repository = repository
	return service
}

func (service *Service) Create(projectID string, request Request) (Version, error) {
	if request.TargetCurrency == "" {
		return Version{}, errors.New("target currency is required")
	}
	confirmed, ok := service.analysis.Confirmed(request.AnalysisID)
	if !ok {
		return Version{}, errors.New("confirmed floor-plan data is required")
	}
	plan := confirmedPlan(confirmed)
	finishes := map[string]domain.RoomFinish{}
	for index, room := range plan.Rooms {
		if finish, ok := request.Finishes[room.ID]; ok {
			plan.Rooms[index].Finish = finish
			if finish != (domain.RoomFinish{}) {
				finishes[room.ID] = finish
			}
		}
	}
	rates := request.rates()
	result, err := domain.Calculate(plan, rates)
	if err != nil {
		return Version{}, err
	}
	plaster, err := domain.ConvertCurrency(result.TotalPlasterCostUSD, request.ExchangeRate)
	if err != nil {
		return Version{}, err
	}
	paint, err := domain.ConvertCurrency(result.TotalPaintCostUSD, request.ExchangeRate)
	if err != nil {
		return Version{}, err
	}
	grand, err := domain.ConvertCurrency(result.GrandTotalUSD, request.ExchangeRate)
	if err != nil {
		return Version{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	version := Version{ID: fmt.Sprintf("estimate-%d", time.Now().UnixNano()), ProjectID: projectID, Version: len(service.versions[projectID]) + 1, SourceAnalysisID: request.AnalysisID, TargetCurrency: request.TargetCurrency, ExchangeRate: request.ExchangeRate, ExchangeRateFetchedAt: request.ExchangeRateFetchedAt, Rates: &rates, Finishes: finishes, Result: result, ConvertedPlasterCost: plaster, ConvertedPaintCost: paint, ConvertedGrandTotal: grand, CalculatedAt: time.Now().UTC()}
	service.versions[projectID] = append(service.versions[projectID], version)
	if service.repository != nil {
		if err := service.repository.Save(version); err != nil {
			return Version{}, err
		}
	}
	return version, nil
}

func (service *Service) Get(projectID, id string) (Version, bool) {
	service.mu.RLock()
	defer service.mu.RUnlock()
	for _, version := range service.versions[projectID] {
		if version.ID == id {
			return version, true
		}
	}
	if service.repository != nil {
		return service.repository.Get(id)
	}
	return Version{}, false
}

func (service *Service) List(projectID string) []Version {
	service.mu.RLock()
	defer service.mu.RUnlock()
	versions := append([]Version(nil), service.versions[projectID]...)
	for left, right := 0, len(versions)-1; left < right; left, right = left+1, right-1 {
		versions[left], versions[right] = versions[right], versions[left]
	}
	if len(versions) == 0 && service.repository != nil {
		return service.repository.List(projectID)
	}
	return versions
}

func (service *Service) GetByID(id string) (Version, bool) {
	service.mu.RLock()
	defer service.mu.RUnlock()
	for _, versions := range service.versions {
		for _, version := range versions {
			if version.ID == id {
				return version, true
			}
		}
	}
	if service.repository != nil {
		return service.repository.Get(id)
	}
	return Version{}, false
}

type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	repository := &PostgresRepository{db: db}
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS estimate_versions (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, version_number INTEGER NOT NULL, payload JSONB NOT NULL)`)
	return repository, err
}

func (repository *PostgresRepository) Save(version Version) error {
	payload, err := json.Marshal(version)
	if err != nil {
		return err
	}
	_, err = repository.db.Exec(`INSERT INTO estimate_versions (id, project_id, version_number, payload) VALUES ($1, $2, $3, $4)`, version.ID, version.ProjectID, version.Version, payload)
	return err
}

func (repository *PostgresRepository) Get(id string) (Version, bool) {
	var payload []byte
	if repository.db.QueryRow(`SELECT payload FROM estimate_versions WHERE id = $1`, id).Scan(&payload) != nil {
		return Version{}, false
	}
	var version Version
	if json.Unmarshal(payload, &version) != nil {
		return Version{}, false
	}
	return version, true
}

func (repository *PostgresRepository) List(projectID string) []Version {
	rows, err := repository.db.Query(`SELECT payload FROM estimate_versions WHERE project_id = $1 ORDER BY version_number DESC`, projectID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var versions []Version
	for rows.Next() {
		var payload []byte
		if rows.Scan(&payload) == nil {
			var version Version
			if json.Unmarshal(payload, &version) == nil {
				versions = append(versions, version)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return versions
}

func confirmedPlan(candidate analysis.Candidate) domain.ConfirmedFloorPlan {
	plan := domain.ConfirmedFloorPlan{Rooms: make([]domain.Room, 0, len(candidate.Rooms))}
	for _, room := range candidate.Rooms {
		converted := domain.Room{ID: room.ID, Name: room.Name, Length: *room.Length, Width: *room.Width, WallHeight: *room.WallHeight}
		for _, opening := range candidate.Doors {
			if opening.RoomID == room.ID && opening.Width != nil && opening.Height != nil {
				converted.Doors = append(converted.Doors, domain.Opening{ID: opening.ID, RoomID: room.ID, Width: *opening.Width, Height: *opening.Height})
			}
		}
		for _, opening := range candidate.Windows {
			if opening.RoomID == room.ID && opening.Width != nil && opening.Height != nil {
				converted.Windows = append(converted.Windows, domain.Opening{ID: opening.ID, RoomID: room.ID, Width: *opening.Width, Height: *opening.Height})
			}
		}
		plan.Rooms = append(plan.Rooms, converted)
	}
	return plan
}
