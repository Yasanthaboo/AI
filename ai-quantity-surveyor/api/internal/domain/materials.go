package domain

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

const MaterialFormulaVersion = "finish-materials-v1"

type MaterialTrade string

const (
	MaterialPaint     MaterialTrade = "paint"
	MaterialWallTile  MaterialTrade = "wallTile"
	MaterialFloorTile MaterialTrade = "floorTile"
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
		if allocation.Trade != MaterialPaint && allocation.Trade != MaterialWallTile && allocation.Trade != MaterialFloorTile {
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
		}
		if allocation.Excluded || area.Sign() == 0 {
			results = append(results, MaterialRequirement{ID: fmt.Sprintf("material-%d", len(results)+1), RoomIDs: []string{allocation.RoomID}, Trade: allocation.Trade, Unit: unit, FormulaVersion: MaterialFormulaVersion})
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
