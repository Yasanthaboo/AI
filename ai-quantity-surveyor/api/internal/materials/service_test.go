package materials

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/domain"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
	"github.com/xuri/excelize/v2"
)

type fixtureSources struct{ version estimate.Version }

func (fixture fixtureSources) Get(projectID, id string) (estimate.Version, bool) {
	return fixture.version, projectID == fixture.version.ProjectID && id == fixture.version.ID
}
func (fixture fixtureSources) ConfirmationRevision(id string, revision int) (analysis.Confirmation, bool) {
	source := &analysis.Source{FloorPlanID: fixture.version.SourceFloorPlanID, PageNumber: 2, ImageDigest: "fixture-digest"}
	return analysis.Confirmation{AnalysisID: id, Revision: revision, Candidate: analysis.Candidate{FloorPlanID: fixture.version.SourceFloorPlanID, Source: source}}, id == fixture.version.SourceAnalysisID && revision == fixture.version.SourceConfirmationRevision
}
func (fixture fixtureSources) GetFloorPlan(id string) (project.FloorPlan, bool) {
	return project.FloorPlan{ID: id, ProjectID: fixture.version.ProjectID}, id == fixture.version.SourceFloorPlanID
}

func TestBudgetsUseFrozenSourceAndImmutableVersions(t *testing.T) {
	source := fixtureSources{version: estimate.Version{ID: "estimate", ProjectID: "project", SourceAnalysisID: "analysis", SourceConfirmationRevision: 1, SourceFloorPlanID: "drawing", SourcePageNumber: 2, SourceImageDigest: "fixture-digest", Result: domain.CalculationResult{Rooms: []domain.RoomCalculation{{RoomID: "room", PaintArea: 100}}}}}
	service := NewService(source, source, source, NewMemoryRepository())
	input := BudgetRequest{EstimateID: "estimate", MaterialWastePercent: 10, Allocations: []Allocation{{RoomID: "room", Trade: domain.MaterialPaint, Recipe: domain.MaterialRecipe{Category: "paint", Specification: "Interior matt", Coats: 2, CoverageM2PerLitrePerCoat: 10, PackAmount: 5, PackUnit: "litre", InputOrigin: "manual"}}}}
	first, err := service.CreateBudget(context.Background(), "project", input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Requirements[0].RequiredAmount != 22 || first.Requirements[0].PackCount != 5 || first.Version != 1 {
		t.Fatalf("incorrect budget: %+v", first)
	}
	first.Requirements[0].PackCount = 999
	stored, _, err := service.GetBudget(context.Background(), first.ID)
	if err != nil || stored.Requirements[0].PackCount != 5 {
		t.Fatal("caller mutated stored budget")
	}
	second, err := service.CreateBudget(context.Background(), "project", input)
	if err != nil || second.Version != 2 {
		t.Fatalf("versioning failed: %+v %v", second, err)
	}
	if _, err := service.CreateBudget(context.Background(), "foreign", input); err == nil {
		t.Fatal("foreign project accepted")
	}
	input.Allocations = append(input.Allocations, input.Allocations[0])
	if _, err := service.CreateBudget(context.Background(), "project", input); err == nil || err.(*Error).Code != "DuplicateMaterialAllocation" {
		t.Fatal("duplicate allocation accepted")
	}
	budgets, _ := service.ListBudgets(context.Background(), "project")
	if len(budgets) != 2 {
		t.Fatal("failed allocation created a version")
	}
}

func TestPricingRequiresCompleteBasketAndCountsDeliveryOnce(t *testing.T) {
	offers := fixtureOffers()
	budget := Budget{MaterialWastePercent: 10, Requirements: []domain.MaterialRequirement{
		{ID: "paint", Trade: domain.MaterialPaint, SourceAreaM2: 100, RequiredAmount: 22, Recipe: offers[0].Recipe},
		{ID: "tile", Trade: domain.MaterialFloorTile, SourceAreaM2: 20, RequiredAmount: 22, Recipe: offers[2].Recipe},
	}}
	selections := []Selection{{RequirementID: "paint", ObservationID: offers[0].ID}, {RequirementID: "tile", ObservationID: offers[2].ID}}
	if _, err := priceBasket(budget, offers, selections[:1], nil, time.Now(), false); err == nil {
		t.Fatal("partial basket appeared complete")
	}
	basket, err := priceBasket(budget, offers, selections, nil, time.Now(), true)
	if err != nil || basket.KnownDelivery != 1200 || basket.MaterialSubtotal != 90500 || basket.DeliveredTotal == nil || *basket.DeliveredTotal != 91700 {
		t.Fatalf("delivery counted incorrectly: %+v %v", basket, err)
	}
	offers[0].TaxIncluded = nil
	basket, err = priceBasket(budget, offers, selections, nil, time.Now(), false)
	if err != nil || basket.DeliveredTotal != nil || !strings.Contains(strings.Join(basket.Warnings, " "), "Tax status unknown") {
		t.Fatalf("unknown tax was treated as included: %+v %v", basket, err)
	}
	taxExcluded := false
	offers[0].TaxIncluded = &taxExcluded
	basket, err = priceBasket(budget, offers, selections, nil, time.Now(), false)
	if err != nil || basket.DeliveredTotal != nil || !strings.Contains(strings.Join(basket.Warnings, " "), "Tax excluded") {
		t.Fatalf("excluded tax was treated as included: %+v %v", basket, err)
	}
	if _, err := priceBasket(budget, offers, append(selections, selections[0]), nil, time.Now(), false); err == nil {
		t.Fatal("duplicate purchase accepted")
	}
	offers[0].Recipe.Specification = "Non-equivalent coating"
	if _, err := priceBasket(budget, offers, selections, nil, time.Now(), false); err == nil {
		t.Fatal("incompatible specification accepted")
	}
}

func TestCurrencyRequiresCapturedConversion(t *testing.T) {
	offers := fixtureOffers()[:1]
	budget := Budget{MaterialWastePercent: 10, Requirements: []domain.MaterialRequirement{{ID: "paint", Trade: domain.MaterialPaint, SourceAreaM2: 100, RequiredAmount: 22, Recipe: offers[0].Recipe}}}
	offers[0].Currency, offers[0].Price, offers[0].Delivery = "USD", 10.005, nil
	selections := []Selection{{RequirementID: "paint", ObservationID: offers[0].ID}}
	if _, err := priceBasket(budget, offers, selections, nil, time.Now(), false); err == nil {
		t.Fatal("foreign currency compared without conversion")
	}
	basket, err := priceBasket(budget, offers, selections, &Conversion{BaseCurrency: "USD", QuoteCurrency: "LKR", Rate: 300, CapturedAt: time.Now()}, time.Now(), false)
	if err != nil || basket.MaterialSubtotal != 15007.5 || basket.DeliveredTotal != nil {
		t.Fatalf("conversion: %+v %v", basket, err)
	}
	if roundMoney(big.NewRat(1005, 1000)) != 1.01 {
		t.Fatal("decimal half-cent rounded incorrectly")
	}
}

func TestExportPreservesUnknownsProvenanceAndLiteralSourceStrings(t *testing.T) {
	source := fixtureSources{version: estimate.Version{ID: "estimate", ProjectID: "project", SourceAnalysisID: "analysis", SourceConfirmationRevision: 1, SourceFloorPlanID: "drawing", SourcePageNumber: 2, SourceImageDigest: "fixture-digest", Result: domain.CalculationResult{Rooms: []domain.RoomCalculation{{RoomID: "room", PaintArea: 100}}}}}
	repository := NewMemoryRepository()
	repository.offers[1].Supplier = "=HYPERLINK(\"https://example.invalid\",\"sample\")"
	repository.offers[1].TaxIncluded = nil
	service := NewService(source, source, source, repository)
	budget, err := service.CreateBudget(context.Background(), "project", BudgetRequest{EstimateID: "estimate", MaterialWastePercent: 10, Allocations: []Allocation{{RoomID: "room", Trade: domain.MaterialPaint, Recipe: repository.offers[1].Recipe}}})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.SaveComparison(context.Background(), budget.ID, ComparisonRequest{Location: Location{Country: "LK", City: "Colombo"}, Selections: []Selection{{RequirementID: budget.Requirements[0].ID, ObservationID: repository.offers[1].ID}}})
	if err != nil {
		t.Fatal(err)
	}
	content, err := service.Export(context.Background(), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for _, check := range []struct{ sheet, cell, expected string }{{"Offers", "B2", repository.offers[1].Supplier}, {"Sources", "E2", "Unknown"}, {"Sources", "F2", "Unknown"}, {"Assumptions", "B3", "Incomplete: unknown charges or stale source"}, {"Assumptions", "D7", "fixture-digest"}} {
		value, err := book.GetCellValue(check.sheet, check.cell)
		if err != nil || value != check.expected {
			t.Fatalf("%s %s: %q %v", check.sheet, check.cell, value, err)
		}
		formula, err := book.GetCellFormula(check.sheet, check.cell)
		if err != nil || formula != "" {
			t.Fatal("source text became an executable formula")
		}
	}
}

func TestLegacyEstimateCannotBeRelabeledForBudget(t *testing.T) {
	source := fixtureSources{version: estimate.Version{ID: "old", ProjectID: "project"}}
	service := NewService(source, source, source, NewMemoryRepository())
	_, err := service.CreateBudget(context.Background(), "project", BudgetRequest{EstimateID: "old", Allocations: []Allocation{{RoomID: "room"}}})
	if err == nil || err.(*Error).Code != "SourceRevisionUnavailable" {
		t.Fatalf("legacy source accepted: %v", err)
	}
}

func TestBudgetRejectsConfirmedPageOrDigestDrift(t *testing.T) {
	version := estimate.Version{ID: "estimate", ProjectID: "project", SourceAnalysisID: "analysis", SourceConfirmationRevision: 1, SourceFloorPlanID: "drawing", SourcePageNumber: 2, SourceImageDigest: "fixture-digest"}
	input := BudgetRequest{EstimateID: version.ID, Allocations: []Allocation{{RoomID: "room", Trade: domain.MaterialPaint, Recipe: domain.MaterialRecipe{Category: "paint", Specification: "Interior matt", Coats: 2, CoverageM2PerLitrePerCoat: 10, PackAmount: 5, PackUnit: "litre", InputOrigin: "manual"}}}}
	version.SourcePageNumber++
	_, err := NewService(fixtureSources{version: version}, fixtureSources{version: version}, fixtureSources{version: version}, NewMemoryRepository()).CreateBudget(context.Background(), "project", input)
	if err == nil || err.(*Error).Code != "SourceRevisionUnavailable" {
		t.Fatalf("mismatched page accepted: %v", err)
	}
	version.SourcePageNumber = 2
	version.SourceImageDigest = "other-page-digest"
	_, err = NewService(fixtureSources{version: version}, fixtureSources{version: version}, fixtureSources{version: version}, NewMemoryRepository()).CreateBudget(context.Background(), "project", input)
	if err == nil || err.(*Error).Code != "SourceRevisionUnavailable" {
		t.Fatalf("mismatched image digest accepted: %v", err)
	}
}

func TestPlasterBudgetUsesOnlyConfirmedNetPlasterArea(t *testing.T) {
	version := estimate.Version{ID: "plaster-estimate", ProjectID: "project", SourceAnalysisID: "analysis", SourceConfirmationRevision: 1, SourceFloorPlanID: "drawing", SourcePageNumber: 2, SourceImageDigest: "fixture-digest", Result: domain.CalculationResult{Rooms: []domain.RoomCalculation{{RoomID: "hall", PaintArea: 100, NetPlasterArea: 42, FlooringArea: 30}}}}
	service := NewService(fixtureSources{version: version}, fixtureSources{version: version}, fixtureSources{version: version}, NewMemoryRepository())
	service.EnablePlasterMaterials(true)
	recipe := domain.MaterialRecipe{Category: "plasterMix", CementSpecification: "Reviewed cement", SandSpecification: "Reviewed sand", ThicknessMM: 12, CementParts: 1, SandParts: 4, DryVolumeFactor: 1.33, CementDensityKgPerM3: 1440, CementPackAmountKg: 50, SandPackAmountM3: 1, InputOrigin: "manual"}
	budget, err := service.CreateBudget(context.Background(), "project", BudgetRequest{EstimateID: version.ID, MaterialWastePercent: 10, Allocations: []Allocation{{RoomID: "hall", Trade: domain.MaterialPlaster, Recipe: recipe}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(budget.Requirements) != 2 {
		t.Fatalf("expected cement and sand: %+v", budget.Requirements)
	}
	for _, requirement := range budget.Requirements {
		if requirement.SourceAreaM2 != 42 {
			t.Fatalf("used paint/gross/user area instead of confirmed net plaster: %+v", requirement)
		}
	}
}

func TestPlasterBudgetRemainsDisabledByDefault(t *testing.T) {
	service := NewService(nil, nil, nil, NewMemoryRepository())
	_, err := service.CreateBudget(context.Background(), "project", BudgetRequest{Allocations: []Allocation{{RoomID: "hall", Trade: domain.MaterialPlaster}}})
	if err == nil || err.(*Error).Status != 409 || err.(*Error).Code != "PlasterMaterialsDisabled" {
		t.Fatalf("unratified plaster option was enabled: %v", err)
	}
}

func TestFixturePreviewDoesNotSaveAndUnknownDeliveryStaysUnknown(t *testing.T) {
	source := fixtureSources{version: estimate.Version{ID: "estimate", ProjectID: "project", SourceAnalysisID: "analysis", SourceConfirmationRevision: 1, SourceFloorPlanID: "drawing", SourcePageNumber: 2, SourceImageDigest: "fixture-digest", Result: domain.CalculationResult{Rooms: []domain.RoomCalculation{{RoomID: "room", PaintArea: 100}}}}}
	repository := NewMemoryRepository()
	service := NewService(source, source, source, repository)
	offers, _ := service.Offers(context.Background())
	budget, err := service.CreateBudget(context.Background(), "project", BudgetRequest{EstimateID: "estimate", MaterialWastePercent: 10, Allocations: []Allocation{{RoomID: "room", Trade: domain.MaterialPaint, Recipe: offers[0].Recipe}}})
	if err != nil {
		t.Fatal(err)
	}
	selection := Selection{RequirementID: budget.Requirements[0].ID, ObservationID: "fixture-paint-b"}
	preview, err := service.Preview(context.Background(), budget.ID, PreviewRequest{Location: Location{Country: "LK", City: "Colombo"}, Alternatives: [][]Selection{{selection}}})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Alternatives[0].MaterialSubtotal != 39500 || preview.Alternatives[0].DeliveredTotal != nil || len(repository.comparisons) != 0 {
		t.Fatalf("preview hid unknowns or persisted: %+v", preview)
	}
	saved, err := service.SaveComparison(context.Background(), budget.ID, ComparisonRequest{Location: Location{Country: "LK", City: "Colombo"}, Selections: []Selection{selection}})
	if err != nil || saved.Version != 1 {
		t.Fatalf("save failed: %+v %v", saved, err)
	}
	saved.Basket.MaterialSubtotal = 0
	stored, _, _ := service.GetComparison(context.Background(), saved.ID)
	if stored.Basket.MaterialSubtotal != 39500 {
		t.Fatal("caller mutated saved comparison")
	}
	repository.offers[0].ObservedAt = time.Now().Add(-48 * time.Hour)
	selection.ObservationID = repository.offers[0].ID
	if _, err := service.SaveComparison(context.Background(), budget.ID, ComparisonRequest{Location: Location{Country: "LK", City: "Colombo"}, Selections: []Selection{selection}}); err == nil {
		t.Fatal("expired selection saved")
	}
}

func TestPostgresMaterialVersionsRollbackAndReopen(t *testing.T) {
	connection := os.Getenv("SURVEYOR_JOB_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("disposable material test database not configured")
	}
	parsed, err := url.Parse(connection)
	if err != nil || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "surveyor_job_test") {
		t.Fatal("material tests require a disposable surveyor_job_test database")
	}
	store, err := project.NewPostgresService(connection)
	if err != nil {
		t.Fatal("could not open disposable material database")
	}
	defer store.DB().Close()
	store.DB().SetMaxOpenConns(8)
	estimates, err := estimate.NewPostgresRepository(store.DB())
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateProject(project.Details{Name: "Disposable material fixture"})
	if err != nil {
		t.Fatal(err)
	}
	drawing, err := store.SaveFloorPlan(created.ID, "fixture.png", "image/png", []byte("sample"))
	if err != nil {
		t.Fatal(err)
	}
	source := fixtureSources{version: estimate.Version{ID: newID(), ProjectID: created.ID, Version: 1, SourceAnalysisID: newID(), SourceConfirmationRevision: 1, SourceFloorPlanID: drawing.ID, SourcePageNumber: 2, SourceImageDigest: "fixture-digest", Result: domain.CalculationResult{Rooms: []domain.RoomCalculation{{RoomID: "room", PaintArea: 100}}}}}
	if err := estimates.Save(source.version); err != nil {
		t.Fatal(err)
	}
	repository, err := NewPostgresRepository(store.DB())
	if err != nil {
		t.Fatal(err)
	}
	offer := fixtureOffers()[0]
	offer.ID = "fixture-" + newID()
	payload, err := json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO material_price_observations(id,payload) VALUES($1,$2)`, offer.ID, payload); err != nil {
		t.Fatal(err)
	}
	service := NewService(source, source, store, repository)
	input := BudgetRequest{EstimateID: source.version.ID, MaterialWastePercent: 10, Allocations: []Allocation{{RoomID: "room", Trade: domain.MaterialPaint, Recipe: offer.Recipe}}}
	ctx := context.Background()
	var workers sync.WaitGroup
	budgets := make(chan Budget, 8)
	failures := make(chan error, 16)
	for index := 0; index < 8; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			value, err := service.CreateBudget(ctx, created.ID, input)
			if err != nil {
				failures <- err
			} else {
				budgets <- value
			}
		}()
	}
	workers.Wait()
	close(budgets)
	versions := make(map[int]bool)
	var first Budget
	for value := range budgets {
		if versions[value.Version] {
			t.Fatal("concurrent budget version collision")
		}
		versions[value.Version] = true
		first = value
	}
	if len(versions) != 8 {
		t.Fatal("not all concurrent budgets committed")
	}
	if _, err := repository.SaveBudget(ctx, first); err == nil {
		t.Fatal("duplicate immutable budget insert accepted")
	}
	listed, err := repository.ListBudgets(ctx, created.ID)
	if err != nil || len(listed) != 8 {
		t.Fatal("failed insert published a partial budget")
	}
	selection := ComparisonRequest{Location: Location{Country: "LK", City: "Colombo"}, Selections: []Selection{{RequirementID: first.Requirements[0].ID, ObservationID: offer.ID}}}
	comparisons := make(chan Comparison, 8)
	for index := 0; index < 8; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			value, err := service.SaveComparison(ctx, first.ID, selection)
			if err != nil {
				failures <- err
			} else {
				comparisons <- value
			}
		}()
	}
	workers.Wait()
	close(comparisons)
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	versions = make(map[int]bool)
	var saved Comparison
	for value := range comparisons {
		if versions[value.Version] {
			t.Fatal("concurrent comparison version collision")
		}
		versions[value.Version] = true
		saved = value
	}
	if len(versions) != 8 {
		t.Fatal("not all concurrent comparisons committed")
	}
	if _, err := repository.SaveComparison(ctx, saved); err == nil {
		t.Fatal("duplicate immutable comparison insert accepted")
	}
	var count int
	if err := store.DB().QueryRow(`SELECT count(*) FROM material_comparison_versions WHERE budget_id=$1`, first.ID).Scan(&count); err != nil || count != 8 {
		t.Fatal("failed comparison insert partially published")
	}
	newer := offer
	newer.ID, newer.Price = "fixture-"+newID(), 9999
	payload, err = json.Marshal(newer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`INSERT INTO material_price_observations(id,payload) VALUES($1,$2)`, newer.ID, payload); err != nil {
		t.Fatal(err)
	}
	reopened, err := project.NewPostgresService(connection)
	if err != nil {
		t.Fatal("could not reopen disposable database")
	}
	defer reopened.DB().Close()
	loaded, err := NewPostgresRepository(reopened.DB())
	if err != nil {
		t.Fatal(err)
	}
	frozen, found, err := loaded.GetBudget(ctx, first.ID)
	if err != nil || !found || frozen.SourceConfirmationRevision != 1 || frozen.Requirements[0].RequiredAmount != 22 {
		t.Fatal("budget provenance/quantity lost after reconnect")
	}
	comparison, found, err := loaded.GetComparison(ctx, saved.ID)
	if err != nil || !found || comparison.Basket.MaterialSubtotal != 42500 || comparison.Basket.Items[0].Offer.ObservedAt != offer.ObservedAt || comparison.Basket.Items[0].Offer.Price != 8500 {
		t.Fatal("new observation or reconnect rewrote saved comparison")
	}
}
