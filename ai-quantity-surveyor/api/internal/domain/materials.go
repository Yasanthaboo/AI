package domain

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

const MaterialFormulaVersion = "finish-materials-v1"
const PlasterMaterialFormulaVersion = "plaster-materials-v1"

type MaterialTrade string

const (
	MaterialPaint         MaterialTrade = "paint"
	MaterialWallTile      MaterialTrade = "wallTile"
	MaterialFloorTile     MaterialTrade = "floorTile"
	MaterialPlaster       MaterialTrade = "plaster"
	MaterialPremixPlaster MaterialTrade = "premixPlaster"
	MaterialPlasterCement MaterialTrade = "plasterCement"
	MaterialPlasterSand   MaterialTrade = "plasterSand"
)

type MaterialRecipe struct {
	Category                  string  `json:"category"`
	ProductID                 string  `json:"productId,omitempty"`
	Specification             string  `json:"specification"`
	Coats                     int     `json:"coats,omitempty"`
	CoverageM2PerLitrePerCoat float64 `json:"coverageM2PerLitrePerCoat,omitempty"`
	PackAmount                float64 `json:"packAmount"`
	PackUnit                  string  `json:"packUnit"`
	InputOrigin               string  `json:"inputOrigin"`
	SourceURL                 string  `json:"sourceUrl,omitempty"`
	ConsumptionPerM2          float64 `json:"consumptionPerM2,omitempty"`
	ThicknessMM               float64 `json:"thicknessMm,omitempty"`
	CementSpecification       string  `json:"cementSpecification,omitempty"`
	SandSpecification         string  `json:"sandSpecification,omitempty"`
	CementParts               float64 `json:"cementParts,omitempty"`
	SandParts                 float64 `json:"sandParts,omitempty"`
	DryVolumeFactor           float64 `json:"dryVolumeFactor,omitempty"`
	CementDensityKgPerM3      float64 `json:"cementDensityKgPerM3,omitempty"`
	CementPackAmountKg        float64 `json:"cementPackAmountKg,omitempty"`
	SandPackAmountM3          float64 `json:"sandPackAmountM3,omitempty"`
}

type MaterialAllocation struct {
	RoomID             string         `json:"roomId"`
	Trade              MaterialTrade  `json:"trade"`
	AreaM2             float64        `json:"areaM2"`
	Excluded           bool           `json:"excluded,omitempty"`
	CompatibilityGroup string         `json:"compatibilityGroup,omitempty"`
	Recipe             MaterialRecipe `json:"recipe"`
}

type MaterialRequirement struct {
	ID              string         `json:"id"`
	RoomIDs         []string       `json:"roomIds"`
	Trade           MaterialTrade  `json:"trade"`
	Recipe          MaterialRecipe `json:"recipe"`
	SourceAreaM2    float64        `json:"sourceAreaM2"`
	BaseAmount      float64        `json:"baseAmount"`
	RequiredAmount  float64        `json:"requiredAmount"`
	PackCount       int64          `json:"packCount"`
	PurchasedAmount float64        `json:"purchasedAmount"`
	Surplus         float64        `json:"surplus"`
	Unit            string         `json:"unit"`
	FormulaVersion  string         `json:"formulaVersion"`
}

type materialGroupKey struct {
	Trade  MaterialTrade
	Recipe MaterialRecipe
}

func CalculateMaterials(allocations []MaterialAllocation, wastePercent float64) ([]MaterialRequirement, error) {
	expanded := make([]MaterialAllocation, 0, len(allocations))
	seen := make(map[struct {
		room  string
		trade MaterialTrade
	}]bool)
	plasterSystems := make(map[string]MaterialTrade)
	for _, allocation := range allocations {
		if strings.TrimSpace(allocation.RoomID) == "" {
			return nil, fmt.Errorf("each allocation requires a room id")
		}
		key := struct {
			room  string
			trade MaterialTrade
		}{allocation.RoomID, allocation.Trade}
		if seen[key] {
			return nil, fmt.Errorf("DuplicateMaterialAllocation: room %q trade %q", allocation.RoomID, allocation.Trade)
		}
		seen[key] = true
		if allocation.Trade != MaterialPlaster && allocation.Trade != MaterialPremixPlaster {
			expanded = append(expanded, allocation)
			continue
		}
		if system, found := plasterSystems[allocation.RoomID]; found && system != allocation.Trade {
			return nil, fmt.Errorf("room %q cannot use both traditional and premixed plaster", allocation.RoomID)
		}
		plasterSystems[allocation.RoomID] = allocation.Trade
		if allocation.Excluded || allocation.AreaM2 == 0 {
			if allocation.Trade == MaterialPremixPlaster {
				expanded = append(expanded, allocation)
			} else {
				cementRecipe := MaterialRecipe{Category: "plasterCement", Specification: allocation.Recipe.CementSpecification, PackAmount: allocation.Recipe.CementPackAmountKg, PackUnit: "kg", InputOrigin: "manual"}
				sandRecipe := MaterialRecipe{Category: "plasterSand", Specification: allocation.Recipe.SandSpecification, PackAmount: allocation.Recipe.SandPackAmountM3, PackUnit: "m3", InputOrigin: "manual"}
				expanded = append(expanded, MaterialAllocation{RoomID: allocation.RoomID, Trade: MaterialPlasterCement, AreaM2: allocation.AreaM2, Excluded: allocation.Excluded, Recipe: cementRecipe}, MaterialAllocation{RoomID: allocation.RoomID, Trade: MaterialPlasterSand, AreaM2: allocation.AreaM2, Excluded: allocation.Excluded, Recipe: sandRecipe})
			}
			continue
		}
		if allocation.Trade == MaterialPremixPlaster {
			recipe := allocation.Recipe
			if recipe.Category != "premixedPlaster" || strings.TrimSpace(recipe.Specification) == "" || recipe.InputOrigin != "manual" || recipe.ConsumptionPerM2 <= 0 || recipe.ThicknessMM <= 0 || recipe.PackUnit != "kg" {
				return nil, fmt.Errorf("room %q: premixed plaster requires reviewed thickness, kg per m2, specification, and kg pack size", allocation.RoomID)
			}
			expanded = append(expanded, allocation)
			continue
		}
		recipe := allocation.Recipe
		if recipe.Category != "plasterMix" || recipe.InputOrigin != "manual" || strings.TrimSpace(recipe.CementSpecification) == "" || strings.TrimSpace(recipe.SandSpecification) == "" {
			return nil, fmt.Errorf("room %q: cement/sand plaster requires reviewed product specifications", allocation.RoomID)
		}
		thickness, err := normalizeMaterialAmount(recipe.ThicknessMM, true, "thicknessMm")
		if err != nil {
			return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
		}
		cementParts, err := normalizeMaterialAmount(recipe.CementParts, true, "cementParts")
		if err != nil {
			return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
		}
		sandParts, err := normalizeMaterialAmount(recipe.SandParts, true, "sandParts")
		if err != nil {
			return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
		}
		dryFactor, err := normalizeMaterialAmount(recipe.DryVolumeFactor, true, "dryVolumeFactor")
		if err != nil {
			return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
		}
		density, err := normalizeMaterialAmount(recipe.CementDensityKgPerM3, true, "cementDensityKgPerM3")
		if err != nil {
			return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
		}
		cementPack, err := normalizeMaterialAmount(recipe.CementPackAmountKg, true, "cementPackAmountKg")
		if err != nil {
			return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
		}
		sandPack, err := normalizeMaterialAmount(recipe.SandPackAmountM3, true, "sandPackAmountM3")
		if err != nil {
			return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
		}
		normalizedThicknessMM, _ := thickness.Float64()
		cementPartsValue, _ := cementParts.Float64()
		sandPartsValue, _ := sandParts.Float64()
		dryFactorValue, _ := dryFactor.Float64()
		densityValue, _ := density.Float64()
		cementPackValue, _ := cementPack.Float64()
		sandPackValue, _ := sandPack.Float64()
		thickness.Quo(thickness, big.NewRat(1000, 1))
		totalParts := new(big.Rat).Add(cementParts, sandParts)
		dryVolume := new(big.Rat).Mul(thickness, dryFactor)
		cementRate := new(big.Rat).Mul(dryVolume, cementParts)
		cementRate.Quo(cementRate, totalParts)
		cementRate.Mul(cementRate, density)
		sandRate := new(big.Rat).Mul(dryVolume, sandParts)
		sandRate.Quo(sandRate, totalParts)
		common := MaterialRecipe{ThicknessMM: normalizedThicknessMM, CementParts: cementPartsValue, SandParts: sandPartsValue, DryVolumeFactor: dryFactorValue, CementDensityKgPerM3: densityValue, InputOrigin: "manual"}
		cementRecipe := common
		cementRecipe.Category, cementRecipe.Specification, cementRecipe.PackAmount, cementRecipe.PackUnit, cementRecipe.ConsumptionPerM2 = "plasterCement", recipe.CementSpecification, cementPackValue, "kg", ratFloat(cementRate)
		sandRecipe := common
		sandRecipe.Category, sandRecipe.Specification, sandRecipe.PackAmount, sandRecipe.PackUnit, sandRecipe.ConsumptionPerM2 = "plasterSand", recipe.SandSpecification, sandPackValue, "m3", ratFloat(sandRate)
		expanded = append(expanded, MaterialAllocation{RoomID: allocation.RoomID, Trade: MaterialPlasterCement, AreaM2: allocation.AreaM2, Excluded: allocation.Excluded, CompatibilityGroup: allocation.CompatibilityGroup, Recipe: cementRecipe}, MaterialAllocation{RoomID: allocation.RoomID, Trade: MaterialPlasterSand, AreaM2: allocation.AreaM2, Excluded: allocation.Excluded, CompatibilityGroup: allocation.CompatibilityGroup, Recipe: sandRecipe})
	}
	return calculateExpandedMaterials(expanded, wastePercent)
}

func ratFloat(value *big.Rat) float64 { result, _ := value.Float64(); return result }

func calculateExpandedMaterials(allocations []MaterialAllocation, wastePercent float64) ([]MaterialRequirement, error) {
	if len(allocations) == 0 {
		return nil, fmt.Errorf("at least one material allocation is required")
	}
	if math.IsNaN(wastePercent) || math.IsInf(wastePercent, 0) || wastePercent < 0 || wastePercent > 50 {
		return nil, fmt.Errorf("material waste percentage must be finite and between 0 and 50")
	}
	waste, err := normalizeMaterialAmount(wastePercent, false, "wastePercent")
	if err != nil {
		return nil, err
	}
	multiplier := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(waste, big.NewRat(100, 1)))
	seen := make(map[struct {
		RoomID string
		Trade  MaterialTrade
	}]bool)
	groups := make(map[materialGroupKey]int)
	areas := make([]*big.Rat, 0)
	packs := make([]*big.Rat, 0)
	coverage := make([]*big.Rat, 0)
	results := make([]MaterialRequirement, 0)
	for _, allocation := range allocations {
		if strings.TrimSpace(allocation.RoomID) == "" {
			return nil, fmt.Errorf("each allocation requires a room id")
		}
		if allocation.Trade != MaterialPaint && allocation.Trade != MaterialWallTile && allocation.Trade != MaterialFloorTile && allocation.Trade != MaterialPremixPlaster && allocation.Trade != MaterialPlasterCement && allocation.Trade != MaterialPlasterSand {
			return nil, fmt.Errorf("room %q has unsupported material trade %q", allocation.RoomID, allocation.Trade)
		}
		key := struct {
			RoomID string
			Trade  MaterialTrade
		}{allocation.RoomID, allocation.Trade}
		if seen[key] {
			return nil, fmt.Errorf("DuplicateMaterialAllocation: room %q trade %q", allocation.RoomID, allocation.Trade)
		}
		seen[key] = true
		area, err := normalizeMaterialAmount(allocation.AreaM2, !allocation.Excluded && allocation.AreaM2 > 0, "areaM2")
		if err != nil {
			return nil, fmt.Errorf("room %q trade %q: %w", allocation.RoomID, allocation.Trade, err)
		}
		unit := "m2"
		if allocation.Trade == MaterialPaint {
			unit = "litre"
		} else if allocation.Trade == MaterialPlasterCement || allocation.Trade == MaterialPremixPlaster {
			unit = "kg"
		} else if allocation.Trade == MaterialPlasterSand {
			unit = "m3"
		}
		if allocation.Excluded || area.Sign() == 0 {
			formulaVersion := MaterialFormulaVersion
			if allocation.Trade == MaterialPlasterCement || allocation.Trade == MaterialPlasterSand || allocation.Trade == MaterialPremixPlaster {
				formulaVersion = PlasterMaterialFormulaVersion
			}
			results = append(results, MaterialRequirement{ID: fmt.Sprintf("material-%d", len(results)+1), RoomIDs: []string{allocation.RoomID}, Trade: allocation.Trade, Unit: unit, FormulaVersion: formulaVersion})
			areas = append(areas, new(big.Rat))
			packs = append(packs, nil)
			coverage = append(coverage, nil)
			continue
		}
		recipe := allocation.Recipe
		if strings.TrimSpace(recipe.Specification) == "" || (recipe.InputOrigin != "manual" && recipe.InputOrigin != "source") {
			return nil, fmt.Errorf("room %q: material recipe requires a reviewed specification and origin", allocation.RoomID)
		}
		if recipe.InputOrigin == "source" && strings.TrimSpace(recipe.SourceURL) == "" {
			return nil, fmt.Errorf("room %q: sourced recipe requires a source URL", allocation.RoomID)
		}
		pack, err := normalizeMaterialAmount(recipe.PackAmount, true, "packAmount")
		if err != nil {
			return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
		}
		var yield *big.Rat
		if allocation.Trade == MaterialPaint {
			if recipe.Category != "paint" || recipe.Coats <= 0 || recipe.PackUnit != "litre" {
				return nil, fmt.Errorf("room %q: paint requires positive integer coats and litre packs", allocation.RoomID)
			}
			yield, err = normalizeMaterialAmount(recipe.CoverageM2PerLitrePerCoat, true, "coverageM2PerLitrePerCoat")
			if err != nil {
				return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
			}
			recipe.CoverageM2PerLitrePerCoat, _ = yield.Float64()
		} else if allocation.Trade == MaterialPlasterCement || allocation.Trade == MaterialPlasterSand || allocation.Trade == MaterialPremixPlaster {
			unit := map[MaterialTrade]string{MaterialPlasterCement: "kg", MaterialPlasterSand: "m3", MaterialPremixPlaster: "kg"}[allocation.Trade]
			category := map[MaterialTrade]string{MaterialPlasterCement: "plasterCement", MaterialPlasterSand: "plasterSand", MaterialPremixPlaster: "premixedPlaster"}[allocation.Trade]
			if recipe.Category != category || recipe.PackUnit != unit || recipe.Coats != 0 || recipe.CoverageM2PerLitrePerCoat != 0 {
				return nil, fmt.Errorf("room %q: plaster material recipe has incompatible unit or category", allocation.RoomID)
			}
			yield, err = normalizeMaterialAmount(recipe.ConsumptionPerM2, true, "consumptionPerM2")
			if err != nil {
				return nil, fmt.Errorf("room %q: %w", allocation.RoomID, err)
			}
			recipe.ConsumptionPerM2, _ = yield.Float64()
		} else if recipe.Category != "tile" || recipe.PackUnit != "m2" || recipe.Coats != 0 || recipe.CoverageM2PerLitrePerCoat != 0 {
			return nil, fmt.Errorf("room %q: tile requires square-metre packs and no paint consumption fields", allocation.RoomID)
		}
		recipe.PackAmount, _ = pack.Float64()
		group := materialGroupKey{Trade: allocation.Trade, Recipe: recipe}
		if index, exists := groups[group]; exists {
			areas[index].Add(areas[index], area)
			results[index].RoomIDs = append(results[index].RoomIDs, allocation.RoomID)
			continue
		}
		groups[group] = len(results)
		results = append(results, MaterialRequirement{ID: fmt.Sprintf("material-%d", len(results)+1), RoomIDs: []string{allocation.RoomID}, Trade: allocation.Trade, Recipe: recipe, Unit: unit, FormulaVersion: MaterialFormulaVersion})
		areas = append(areas, area)
		packs = append(packs, pack)
		coverage = append(coverage, yield)
	}
	for index := range results {
		if areas[index].Sign() == 0 {
			continue
		}
		base := new(big.Rat).Set(areas[index])
		if results[index].Trade == MaterialPaint {
			base.Mul(base, big.NewRat(int64(results[index].Recipe.Coats), 1))
			base.Quo(base, coverage[index])
		} else if results[index].Trade == MaterialPlasterCement || results[index].Trade == MaterialPlasterSand || results[index].Trade == MaterialPremixPlaster {
			consumption, err := normalizeMaterialAmount(results[index].Recipe.ConsumptionPerM2, true, "consumptionPerM2")
			if err != nil {
				return nil, err
			}
			base.Mul(base, consumption)
		}
		required := new(big.Rat).Mul(base, multiplier)
		ratio := new(big.Rat).Quo(required, packs[index])
		count, remainder := new(big.Int), new(big.Int)
		count.QuoRem(ratio.Num(), ratio.Denom(), remainder)
		if remainder.Sign() != 0 {
			count.Add(count, big.NewInt(1))
		}
		if !count.IsInt64() || count.Int64() > 9007199254740991 {
			return nil, fmt.Errorf("material pack count exceeds supported integer range")
		}
		purchased := new(big.Rat).Mul(new(big.Rat).SetInt(count), packs[index])
		surplus := new(big.Rat).Sub(purchased, required)
		values := []*big.Rat{areas[index], base, required, purchased, surplus}
		output := make([]float64, len(values))
		for position, value := range values {
			output[position], _ = value.Float64()
			if math.IsInf(output[position], 0) || math.IsNaN(output[position]) {
				return nil, fmt.Errorf("material quantity exceeds supported finite range")
			}
		}
		results[index].SourceAreaM2 = output[0]
		results[index].BaseAmount = output[1]
		results[index].RequiredAmount = output[2]
		results[index].PackCount = count.Int64()
		results[index].PurchasedAmount = output[3]
		results[index].Surplus = output[4]
		if results[index].Trade == MaterialPlasterCement || results[index].Trade == MaterialPlasterSand || results[index].Trade == MaterialPremixPlaster {
			results[index].FormulaVersion = PlasterMaterialFormulaVersion
		}
	}
	return results, nil
}

func normalizeMaterialAmount(value float64, positive bool, field string) (*big.Rat, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || (positive && value == 0) {
		return nil, fmt.Errorf("%s must be finite and %s", field, map[bool]string{true: "positive", false: "nonnegative"}[positive])
	}
	raw, ok := new(big.Rat).SetString(strconv.FormatFloat(value, 'f', -1, 64))
	if !ok {
		return nil, fmt.Errorf("invalid %s", field)
	}
	scaled := new(big.Rat).Mul(raw, big.NewRat(1000000, 1))
	rounded, remainder := new(big.Int), new(big.Int)
	rounded.QuoRem(scaled.Num(), scaled.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(scaled.Denom()) >= 0 {
		rounded.Add(rounded, big.NewInt(1))
	}
	if positive && rounded.Sign() == 0 {
		return nil, fmt.Errorf("MaterialPrecisionUnsupported: %s becomes zero after normalization", field)
	}
	return new(big.Rat).SetFrac(rounded, big.NewInt(1000000)), nil
}
