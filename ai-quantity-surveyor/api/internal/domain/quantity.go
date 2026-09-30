package domain

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Unit string

const (
	UnitFeet        Unit = "ft"
	UnitInches      Unit = "in"
	UnitMeters      Unit = "m"
	UnitCentimeters Unit = "cm"
)

type Measurement struct {
	Value float64 `json:"value"`
	Unit  Unit    `json:"unit"`
}

type Opening struct {
	ID     string      `json:"id"`
	RoomID string      `json:"roomId"`
	Width  Measurement `json:"width"`
	Height Measurement `json:"height"`
}

type Room struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Length     Measurement `json:"length"`
	Width      Measurement `json:"width"`
	WallHeight Measurement `json:"wallHeight"`
	Doors      []Opening   `json:"doors"`
	Windows    []Opening   `json:"windows"`
	NoOpenings bool        `json:"noOpeningsConfirmed"`
}

type ConfirmedFloorPlan struct {
	Rooms []Room `json:"rooms"`
}

type Rates struct {
	PlasterUSDPerSquareMeter float64 `json:"plasterUsdPerSquareMeter"`
	PaintUSDPerSquareMeter   float64 `json:"paintUsdPerSquareMeter"`
}

type RoomCalculation struct {
	RoomID          string  `json:"roomId"`
	RoomName        string  `json:"roomName"`
	FloorArea       float64 `json:"floorArea"`
	GrossWallArea   float64 `json:"grossWallArea"`
	DoorDeduction   float64 `json:"doorDeduction"`
	WindowDeduction float64 `json:"windowDeduction"`
	NetPlasterArea  float64 `json:"netPlasterArea"`
	PaintArea       float64 `json:"paintArea"`
	PlasterCostUSD  float64 `json:"plasterCostUsd"`
	PaintCostUSD    float64 `json:"paintCostUsd"`
}

type CalculationResult struct {
	Rooms                []RoomCalculation `json:"rooms"`
	TotalPlasterQuantity float64           `json:"totalPlasterQuantity"`
	TotalPaintQuantity   float64           `json:"totalPaintQuantity"`
	TotalPlasterCostUSD  float64           `json:"totalPlasterCostUsd"`
	TotalPaintCostUSD    float64           `json:"totalPaintCostUsd"`
	GrandTotalUSD        float64           `json:"grandTotalUsd"`
}

func ConvertToMeters(measurement Measurement) (float64, error) {
	if !isPositiveFinite(measurement.Value) {
		return 0, errors.New("measurement must be a positive finite number")
	}

	factor, ok := map[Unit]float64{
		UnitFeet:        0.3048,
		UnitInches:      0.0254,
		UnitMeters:      1,
		UnitCentimeters: 0.01,
	}[measurement.Unit]
	if !ok {
		return 0, fmt.Errorf("unsupported unit %q", measurement.Unit)
	}
	return measurement.Value * factor, nil
}

// ParseMeasurement accepts decimal input and architectural fractions such as 10' 6".
func ParseMeasurement(value string, unit Unit) (Measurement, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return Measurement{}, errors.New("measurement is required")
	}

	if strings.Contains(trimmed, "'") || strings.Contains(trimmed, "\"") {
		feet, inches, err := parseArchitecturalFraction(trimmed)
		if err != nil {
			return Measurement{}, err
		}
		return Measurement{Value: feet + inches/12, Unit: UnitFeet}, nil
	}

	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return Measurement{}, fmt.Errorf("invalid measurement %q", value)
	}
	measurement := Measurement{Value: parsed, Unit: unit}
	if _, err := ConvertToMeters(measurement); err != nil {
		return Measurement{}, err
	}
	return measurement, nil
}

func Calculate(plan ConfirmedFloorPlan, rates Rates) (CalculationResult, error) {
	if len(plan.Rooms) == 0 {
		return CalculationResult{}, errors.New("at least one room is required")
	}
	if !isPositiveFinite(rates.PlasterUSDPerSquareMeter) || !isPositiveFinite(rates.PaintUSDPerSquareMeter) {
		return CalculationResult{}, errors.New("plaster and paint rates must be positive finite numbers")
	}

	result := CalculationResult{Rooms: make([]RoomCalculation, 0, len(plan.Rooms))}
	for _, room := range plan.Rooms {
		calculation, err := calculateRoom(room, rates)
		if err != nil {
			return CalculationResult{}, err
		}
		result.Rooms = append(result.Rooms, calculation)
		result.TotalPlasterQuantity += calculation.NetPlasterArea
		result.TotalPaintQuantity += calculation.PaintArea
		result.TotalPlasterCostUSD += calculation.PlasterCostUSD
		result.TotalPaintCostUSD += calculation.PaintCostUSD
	}
	result.GrandTotalUSD = result.TotalPlasterCostUSD + result.TotalPaintCostUSD
	return result, nil
}

func ConvertCurrency(usdAmount, exchangeRate float64) (float64, error) {
	if !isPositiveFinite(exchangeRate) {
		return 0, errors.New("exchange rate must be a positive finite number")
	}
	if !isFinite(usdAmount) || usdAmount < 0 {
		return 0, errors.New("USD amount must be a non-negative finite number")
	}
	return usdAmount * exchangeRate, nil
}

func calculateRoom(room Room, rates Rates) (RoomCalculation, error) {
	length, err := ConvertToMeters(room.Length)
	if err != nil {
		return RoomCalculation{}, fmt.Errorf("room %q length: %w", room.Name, err)
	}
	width, err := ConvertToMeters(room.Width)
	if err != nil {
		return RoomCalculation{}, fmt.Errorf("room %q width: %w", room.Name, err)
	}
	wallHeight, err := ConvertToMeters(room.WallHeight)
	if err != nil {
		return RoomCalculation{}, fmt.Errorf("room %q wall height: %w", room.Name, err)
	}

	calculation := RoomCalculation{
		RoomID:        room.ID,
		RoomName:      room.Name,
		FloorArea:     length * width,
		GrossWallArea: 2 * (length + width) * wallHeight,
	}
	for _, door := range room.Doors {
		area, err := openingArea(door)
		if err != nil {
			return RoomCalculation{}, fmt.Errorf("room %q door %q: %w", room.Name, door.ID, err)
		}
		calculation.DoorDeduction += area
	}
	for _, window := range room.Windows {
		area, err := openingArea(window)
		if err != nil {
			return RoomCalculation{}, fmt.Errorf("room %q window %q: %w", room.Name, window.ID, err)
		}
		calculation.WindowDeduction += area
	}
	calculation.NetPlasterArea = calculation.GrossWallArea - calculation.DoorDeduction - calculation.WindowDeduction
	if calculation.NetPlasterArea < 0 {
		return RoomCalculation{}, fmt.Errorf("room %q deductions exceed gross wall area", room.Name)
	}
	calculation.PaintArea = calculation.NetPlasterArea
	calculation.PlasterCostUSD = calculation.NetPlasterArea * rates.PlasterUSDPerSquareMeter
	calculation.PaintCostUSD = calculation.PaintArea * rates.PaintUSDPerSquareMeter
	return calculation, nil
}

func openingArea(opening Opening) (float64, error) {
	width, err := ConvertToMeters(opening.Width)
	if err != nil {
		return 0, fmt.Errorf("width: %w", err)
	}
	height, err := ConvertToMeters(opening.Height)
	if err != nil {
		return 0, fmt.Errorf("height: %w", err)
	}
	return width * height, nil
}

func parseArchitecturalFraction(value string) (float64, float64, error) {
	cleaned := strings.NewReplacer("'", " ", "\"", " ").Replace(value)
	parts := strings.Fields(cleaned)
	if len(parts) == 0 || len(parts) > 2 {
		return 0, 0, fmt.Errorf("invalid architectural measurement %q", value)
	}
	feet, err := parseNumber(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid feet value in %q", value)
	}
	var inches float64
	if len(parts) == 2 {
		inches, err = parseNumber(parts[1])
		if err != nil || inches < 0 || inches >= 12 {
			return 0, 0, fmt.Errorf("invalid inches value in %q", value)
		}
	}
	if !isPositiveFinite(feet + inches/12) {
		return 0, 0, errors.New("architectural measurement must be positive")
	}
	return feet, inches, nil
}

func parseNumber(value string) (float64, error) {
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		if len(parts) != 2 {
			return 0, errors.New("invalid fraction")
		}
		numerator, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		denominator, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || denominator == 0 {
			return 0, errors.New("invalid fraction denominator")
		}
		return numerator / denominator, nil
	}
	return strconv.ParseFloat(value, 64)
}

func isPositiveFinite(value float64) bool { return isFinite(value) && value > 0 }

func isFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
