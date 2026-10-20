package materials

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/domain"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

type Error struct {
	Status  int
	Code    string
	Message string
}

func (failure *Error) Error() string { return failure.Message }
func fail(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}

type Allocation struct {
	RoomID             string                `json:"roomId"`
	Trade              domain.MaterialTrade  `json:"trade"`
	CompatibilityGroup string                `json:"compatibilityGroup,omitempty"`
	Recipe             domain.MaterialRecipe `json:"recipe"`
}

type BudgetRequest struct {
	EstimateID           string       `json:"estimateId"`
	MaterialWastePercent float64      `json:"materialWastePercent"`
	Allocations          []Allocation `json:"allocations"`
}

type Budget struct {
	ID                         string                       `json:"id"`
	ProjectID                  string                       `json:"projectId"`
	Version                    int                          `json:"version"`
	EstimateID                 string                       `json:"estimateId"`
	SourceAnalysisID           string                       `json:"sourceAnalysisId"`
	SourceConfirmationRevision int                          `json:"sourceConfirmationRevision"`
	SourceFloorPlanID          string                       `json:"sourceFloorPlanId"`
	SourcePageNumber           int                          `json:"sourcePageNumber,omitempty"`
	SourceImageDigest          string                       `json:"sourceImageDigest,omitempty"`
	FormulaVersion             string                       `json:"formulaVersion"`
	MaterialWastePercent       float64                      `json:"materialWastePercent"`
	Allocations                []Allocation                 `json:"allocations"`
	Requirements               []domain.MaterialRequirement `json:"requirements"`
	QuantityResult             domain.CalculationResult     `json:"quantityResult"`
	CreatedAt                  time.Time                    `json:"createdAt"`
}

type EstimateReader interface {
	Get(string, string) (estimate.Version, bool)
}
type ConfirmationReader interface {
	ConfirmationRevision(string, int) (analysis.Confirmation, bool)
}
type DrawingReader interface {
	GetFloorPlan(string) (project.FloorPlan, bool)
}
type Repository interface {
	SaveBudget(context.Context, Budget) (Budget, error)
	GetBudget(context.Context, string) (Budget, bool, error)
	ListBudgets(context.Context, string) ([]Budget, error)
	ListOffers(context.Context) ([]Offer, error)
	SaveComparison(context.Context, Comparison) (Comparison, error)
	GetComparison(context.Context, string) (Comparison, bool, error)
}

type Service struct {
	estimates               EstimateReader
	confirmations           ConfirmationReader
	drawings                DrawingReader
	repository              Repository
	plasterMaterialsEnabled bool
}

func NewService(estimates EstimateReader, confirmations ConfirmationReader, drawings DrawingReader, repository Repository) *Service {
	return &Service{estimates: estimates, confirmations: confirmations, drawings: drawings, repository: repository}
}

func (service *Service) EnablePlasterMaterials(enabled bool) {
	service.plasterMaterialsEnabled = enabled
}
func (service *Service) PlasterMaterialsEnabled() bool { return service.plasterMaterialsEnabled }

func (service *Service) CreateBudget(ctx context.Context, projectID string, input BudgetRequest) (Budget, error) {
	if !service.plasterMaterialsEnabled {
		for _, allocation := range input.Allocations {
			if allocation.Trade == domain.MaterialPlaster || allocation.Trade == domain.MaterialPremixPlaster {
				return Budget{}, fail(409, "PlasterMaterialsDisabled", "Plaster-material budgeting is disabled by runtime configuration.")
			}
		}
	}
	if len(input.Allocations) == 0 || len(input.Allocations) > 256 {
		return Budget{}, fail(400, "InvalidMaterialRecipe", "Use between 1 and 256 allocations.")
	}
	version, found := service.estimates.Get(projectID, input.EstimateID)
	if !found || version.ProjectID != projectID {
		return Budget{}, fail(404, "NotFound", "Estimate does not belong to this project.")
	}
	if version.SourceConfirmationRevision <= 0 || version.SourceFloorPlanID == "" || version.SourcePageNumber <= 0 || version.SourceImageDigest == "" {
		return Budget{}, fail(409, "SourceRevisionUnavailable", "Create a new estimate from an exact confirmed revision before budgeting.")
	}
	snapshot, found := service.confirmations.ConfirmationRevision(version.SourceAnalysisID, version.SourceConfirmationRevision)
	if !found || snapshot.AnalysisID != version.SourceAnalysisID || snapshot.Candidate.FloorPlanID != version.SourceFloorPlanID || snapshot.Candidate.Source == nil || snapshot.Candidate.Source.FloorPlanID != version.SourceFloorPlanID || snapshot.Candidate.Source.PageNumber != version.SourcePageNumber || snapshot.Candidate.Source.ImageDigest != version.SourceImageDigest {
		return Budget{}, fail(409, "SourceRevisionUnavailable", "Estimate confirmation provenance cannot be verified.")
	}
	drawing, found := service.drawings.GetFloorPlan(version.SourceFloorPlanID)
	if !found || drawing.ProjectID != projectID {
		return Budget{}, fail(409, "SourceRevisionUnavailable", "Confirmed drawing does not belong to this project.")
	}
	rooms := make(map[string]domain.RoomCalculation)
	for _, room := range version.Result.Rooms {
		rooms[room.RoomID] = room
	}
	allocations := make([]domain.MaterialAllocation, 0, len(input.Allocations))
	seen := make(map[string]map[domain.MaterialTrade]bool)
	for _, entry := range input.Allocations {
		if seen[entry.RoomID] == nil {
			seen[entry.RoomID] = make(map[domain.MaterialTrade]bool)
		}
		if seen[entry.RoomID][entry.Trade] {
			return Budget{}, fail(400, "DuplicateMaterialAllocation", "Duplicate allocation for room "+entry.RoomID+" and trade "+string(entry.Trade)+".")
		}
		seen[entry.RoomID][entry.Trade] = true
		room, found := rooms[entry.RoomID]
		if !found {
			return Budget{}, fail(400, "InvalidMaterialRecipe", "Unknown estimate room: "+entry.RoomID)
		}
		area := 0.0
		switch entry.Trade {
		case domain.MaterialPaint:
			area = room.PaintArea
		case domain.MaterialWallTile:
			area = room.TileArea
		case domain.MaterialFloorTile:
			area = room.FlooringArea
		case domain.MaterialPlaster, domain.MaterialPremixPlaster:
			area = room.NetPlasterArea
		default:
			return Budget{}, fail(400, "InvalidMaterialRecipe", "Unsupported material trade.")
		}
		allocations = append(allocations, domain.MaterialAllocation{RoomID: entry.RoomID, Trade: entry.Trade, AreaM2: area, CompatibilityGroup: entry.CompatibilityGroup, Recipe: entry.Recipe})
	}
	requirements, err := domain.CalculateMaterials(allocations, input.MaterialWastePercent)
	if err != nil {
		return Budget{}, fail(400, "InvalidMaterialRecipe", err.Error())
	}
	budget := Budget{ID: newID(), ProjectID: projectID, EstimateID: version.ID, SourceAnalysisID: version.SourceAnalysisID, SourceConfirmationRevision: version.SourceConfirmationRevision, SourceFloorPlanID: version.SourceFloorPlanID, SourcePageNumber: version.SourcePageNumber, SourceImageDigest: version.SourceImageDigest, FormulaVersion: domain.MaterialFormulaVersion, MaterialWastePercent: input.MaterialWastePercent, Allocations: input.Allocations, Requirements: requirements, QuantityResult: version.Result, CreatedAt: time.Now().UTC()}
	stored, err := service.repository.SaveBudget(ctx, budget)
	if err != nil {
		return Budget{}, fail(503, "MaterialStoreUnavailable", "Material budget could not be saved; retry later.")
	}
	return stored, nil
}

func (service *Service) GetBudget(ctx context.Context, id string) (Budget, bool, error) {
	return service.repository.GetBudget(ctx, id)
}
func (service *Service) ListBudgets(ctx context.Context, projectID string) ([]Budget, error) {
	return service.repository.ListBudgets(ctx, projectID)
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}

func clone[T any](value T) (T, error) {
	var result T
	encoded, err := json.Marshal(value)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(encoded, &result)
	return result, err
}

type MemoryRepository struct {
	mu          sync.Mutex
	budgets     map[string]Budget
	offers      []Offer
	comparisons map[string]Comparison
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{budgets: map[string]Budget{}, offers: fixtureOffers(), comparisons: map[string]Comparison{}}
}
func (repository *MemoryRepository) SaveBudget(ctx context.Context, budget Budget) (Budget, error) {
	if err := ctx.Err(); err != nil {
		return Budget{}, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if _, exists := repository.budgets[budget.ID]; exists {
		return Budget{}, fmt.Errorf("budget is immutable")
	}
	for _, previous := range repository.budgets {
		if previous.ProjectID == budget.ProjectID {
			budget.Version = max(budget.Version, previous.Version)
		}
	}
	budget.Version++
	stored, err := clone(budget)
	if err != nil {
		return Budget{}, err
	}
	repository.budgets[budget.ID] = stored
	return clone(stored)
}
func (repository *MemoryRepository) GetBudget(ctx context.Context, id string) (Budget, bool, error) {
	if err := ctx.Err(); err != nil {
		return Budget{}, false, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	budget, found := repository.budgets[id]
	result, err := clone(budget)
	return result, found, err
}
func (repository *MemoryRepository) ListBudgets(ctx context.Context, projectID string) ([]Budget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]Budget, 0)
	for _, budget := range repository.budgets {
		if budget.ProjectID == projectID {
			result = append(result, budget)
		}
	}
	return clone(result)
}

func (service *Service) SaveComparison(ctx context.Context, budgetID string, input ComparisonRequest) (Comparison, error) {
	if input.Location.Country != "LK" || input.Location.City != "Colombo" {
		return Comparison{}, fail(400, "LocationUnsupported", "Select Colombo, Sri Lanka.")
	}
	budget, found, err := service.repository.GetBudget(ctx, budgetID)
	if err != nil || !found {
		return Comparison{}, fail(404, "NotFound", "Budget unavailable.")
	}
	value := Comparison{ID: newID(), BudgetID: budgetID, ProjectID: budget.ProjectID, Location: input.Location, Currency: "LKR", Conversion: input.Conversion, CreatedAt: time.Now().UTC()}
	value.Basket.Items = make([]PricedItem, len(input.Selections))
	for index, selection := range input.Selections {
		value.Basket.Items[index].RequirementID = selection.RequirementID
		value.Basket.Items[index].Offer.ID = selection.ObservationID
	}
	result, err := service.repository.SaveComparison(ctx, value)
	if err != nil {
		if _, typed := err.(*Error); typed {
			return Comparison{}, err
		}
		return Comparison{}, fail(503, "MaterialStoreUnavailable", "Comparison could not be saved.")
	}
	return result, nil
}
func (service *Service) GetComparison(ctx context.Context, id string) (Comparison, bool, error) {
	return service.repository.GetComparison(ctx, id)
}
func (repository *MemoryRepository) ListOffers(ctx context.Context) ([]Offer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return clone(repository.offers)
}
func selectionsFrom(value Comparison) []Selection {
	result := make([]Selection, len(value.Basket.Items))
	for index, item := range value.Basket.Items {
		result[index] = Selection{RequirementID: item.RequirementID, ObservationID: item.Offer.ID}
	}
	return result
}
func (repository *MemoryRepository) SaveComparison(ctx context.Context, value Comparison) (Comparison, error) {
	if err := ctx.Err(); err != nil {
		return Comparison{}, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if _, exists := repository.comparisons[value.ID]; exists {
		return Comparison{}, fmt.Errorf("comparison is immutable")
	}
	budget, found := repository.budgets[value.BudgetID]
	if !found {
		return Comparison{}, fail(404, "NotFound", "Budget unavailable.")
	}
	basket, err := priceBasket(budget, repository.offers, selectionsFrom(value), value.Conversion, time.Now().UTC(), true)
	if err != nil {
		return Comparison{}, err
	}
	value.Basket = basket
	for _, previous := range repository.comparisons {
		if previous.BudgetID == value.BudgetID {
			value.Version = max(value.Version, previous.Version)
		}
	}
	value.Version++
	stored, err := clone(value)
	if err != nil {
		return Comparison{}, err
	}
	repository.comparisons[value.ID] = stored
	return clone(stored)
}
func (repository *MemoryRepository) GetComparison(ctx context.Context, id string) (Comparison, bool, error) {
	if err := ctx.Err(); err != nil {
		return Comparison{}, false, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	value, found := repository.comparisons[id]
	result, err := clone(value)
	return result, found, err
}
