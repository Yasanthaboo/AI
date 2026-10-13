package domain

import (
	"math"
	"strings"
	"testing"
)

func paintMaterial(room string, area float64) MaterialAllocation {
	return MaterialAllocation{RoomID: room, Trade: MaterialPaint, AreaM2: area, Recipe: MaterialRecipe{Category: "paint", Specification: "Interior matt", Coats: 2, CoverageM2PerLitrePerCoat: 10, PackAmount: 5, PackUnit: "litre", InputOrigin: "manual"}}
}

func TestCalculateMaterialsAcceptanceExamples(t *testing.T) {
	allocations := []MaterialAllocation{paintMaterial("living", 100), {RoomID: "bath", Trade: MaterialWallTile, AreaM2: 20, Recipe: MaterialRecipe{Category: "tile", Specification: "Reviewed wall tile", PackAmount: 1.5, PackUnit: "m2", InputOrigin: "manual"}}}
	results, err := CalculateMaterials(allocations, 10)
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, results[0].RequiredAmount, 22)
	assertFloat(t, results[0].PurchasedAmount, 25)
	assertFloat(t, results[0].Surplus, 3)
	assertFloat(t, results[1].RequiredAmount, 22)
	assertFloat(t, results[1].PurchasedAmount, 22.5)
	assertFloat(t, results[1].Surplus, 0.5)
	if results[0].PackCount != 5 || results[1].PackCount != 15 {
		t.Fatal("incorrect whole-pack counts")
	}
}

func TestCalculateMaterialsAggregatesOnlyIdenticalRecipes(t *testing.T) {
	first, second := paintMaterial("first", 10), paintMaterial("second", 10)
	results, err := CalculateMaterials([]MaterialAllocation{first, second}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].PackCount != 1 || len(results[0].RoomIDs) != 2 {
		t.Fatalf("incorrect aggregation: %+v", results)
	}
	first.CompatibilityGroup, second.CompatibilityGroup = "same", "same"
	second.Recipe.Specification = "Different finish"
	results, err = CalculateMaterials([]MaterialAllocation{first, second}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatal("caller group merged incompatible products")
	}
}

func TestCalculateMaterialsRejectsDuplicateAllocations(t *testing.T) {
	first, second := paintMaterial("living", 10), paintMaterial("living", 20)
	second.Recipe.Specification = "Another product"
	if _, err := CalculateMaterials([]MaterialAllocation{first, second}, 0); err == nil || !strings.Contains(err.Error(), "DuplicateMaterialAllocation") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestCalculateMaterialsNormalizationBoundary(t *testing.T) {
	for _, amount := range []float64{0.00000049, 0.00000050, 0.00000051} {
		allocation := paintMaterial("living", 1)
		allocation.Recipe.PackAmount = amount
		_, err := CalculateMaterials([]MaterialAllocation{allocation}, 0)
		if amount < 0.0000005 && (err == nil || !strings.Contains(err.Error(), "packAmount")) {
			t.Fatalf("expected normalized pack error for %g, got %v", amount, err)
		}
		if amount >= 0.0000005 && err != nil {
			t.Fatal(err)
		}
	}
	allocation := paintMaterial("living", 0.00000049)
	if _, err := CalculateMaterials([]MaterialAllocation{allocation}, 0); err == nil || !strings.Contains(err.Error(), "areaM2") {
		t.Fatalf("expected precision error, got %v", err)
	}
	allocation = paintMaterial("living", 1)
	allocation.Recipe.CoverageM2PerLitrePerCoat = 0.00000049
	if _, err := CalculateMaterials([]MaterialAllocation{allocation}, 0); err == nil || !strings.Contains(err.Error(), "coverage") {
		t.Fatalf("expected coverage precision error, got %v", err)
	}
}

func TestCalculateMaterialsExactPackBoundaryAndExcludedItems(t *testing.T) {
	results, err := CalculateMaterials([]MaterialAllocation{paintMaterial("living", 25)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].PackCount != 1 || results[0].Surplus != 0 {
		t.Fatalf("exact pack rounded up: %+v", results[0])
	}
	results, err = CalculateMaterials([]MaterialAllocation{paintMaterial("living", 25.000001)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].PackCount != 2 {
		t.Fatal("amount above exact boundary did not round up")
	}
	results, err = CalculateMaterials([]MaterialAllocation{{RoomID: "excluded", Trade: MaterialFloorTile, AreaM2: 10, Excluded: true}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].RequiredAmount != 0 || results[0].PackCount != 0 {
		t.Fatal("excluded material was purchased")
	}
}

func TestCalculateMaterialsRejectsInvalidInputs(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), -1, 51} {
		if _, err := CalculateMaterials([]MaterialAllocation{paintMaterial("living", 10)}, value); err == nil {
			t.Fatalf("accepted invalid waste %v", value)
		}
	}
	allocation := paintMaterial("living", 10)
	allocation.Recipe.PackUnit = "m2"
	if _, err := CalculateMaterials([]MaterialAllocation{allocation}, 0); err == nil {
		t.Fatal("accepted incompatible pack units")
	}
}
