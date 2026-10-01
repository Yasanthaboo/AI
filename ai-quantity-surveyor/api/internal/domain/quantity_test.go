package domain

import (
	"math"
	"testing"
)

func TestCalculateUsesDeterministicWallFormulas(t *testing.T) {
	plan := ConfirmedFloorPlan{Rooms: []Room{{
		ID:         "room-1",
		Name:       "Living room",
		Length:     Measurement{Value: 4, Unit: UnitMeters},
		Width:      Measurement{Value: 5, Unit: UnitMeters},
		WallHeight: Measurement{Value: 2, Unit: UnitMeters},
		Doors:      []Opening{{Width: Measurement{Value: 1, Unit: UnitMeters}, Height: Measurement{Value: 2, Unit: UnitMeters}}},
		Windows:    []Opening{{Width: Measurement{Value: 1, Unit: UnitMeters}, Height: Measurement{Value: 1, Unit: UnitMeters}}},
	}}}

	result, err := Calculate(plan, Rates{PlasterUSDPerSquareMeter: 10, PaintUSDPerSquareMeter: 5})
	if err != nil {
		t.Fatal(err)
	}
	room := result.Rooms[0]
	assertFloat(t, room.FloorArea, 20)
	assertFloat(t, room.GrossWallArea, 36)
	assertFloat(t, room.DoorDeduction, 2)
	assertFloat(t, room.WindowDeduction, 1)
	assertFloat(t, room.NetPlasterArea, 33)
	assertFloat(t, room.PaintArea, 33)
	assertFloat(t, result.GrandTotalUSD, 495)
}

func TestConvertToMetersSupportsApprovedUnits(t *testing.T) {
	tests := []struct {
		unit  Unit
		value float64
		want  float64
	}{
		{UnitFeet, 1, 0.3048},
		{UnitInches, 1, 0.0254},
		{UnitMeters, 1, 1},
		{UnitCentimeters, 1, 0.01},
	}
	for _, test := range tests {
		got, err := ConvertToMeters(Measurement{Value: test.value, Unit: test.unit})
		if err != nil {
			t.Errorf("%s: %v", test.unit, err)
			continue
		}
		assertFloat(t, got, test.want)
	}
}

func TestParseMeasurementAcceptsArchitecturalFraction(t *testing.T) {
	measurement, err := ParseMeasurement("10' 6\"", UnitFeet)
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, measurement.Value, 10.5)
	if measurement.Unit != UnitFeet {
		t.Fatalf("unit = %q, want %q", measurement.Unit, UnitFeet)
	}
}

func TestCalculateRejectsNegativeNetPlasterArea(t *testing.T) {
	plan := ConfirmedFloorPlan{Rooms: []Room{{
		Name:       "Small room",
		Length:     Measurement{Value: 1, Unit: UnitMeters},
		Width:      Measurement{Value: 1, Unit: UnitMeters},
		WallHeight: Measurement{Value: 1, Unit: UnitMeters},
		Doors:      []Opening{{Width: Measurement{Value: 3, Unit: UnitMeters}, Height: Measurement{Value: 3, Unit: UnitMeters}}},
	}}}
	if _, err := Calculate(plan, Rates{PlasterUSDPerSquareMeter: 1, PaintUSDPerSquareMeter: 1}); err == nil {
		t.Fatal("expected negative net plaster area to be rejected")
	}
}

func TestConvertCurrencyRejectsUnavailableRate(t *testing.T) {
	if _, err := ConvertCurrency(100, 0); err == nil {
		t.Fatal("expected zero exchange rate to be rejected")
	}
	converted, err := ConvertCurrency(100, 1.25)
	if err != nil {
		t.Fatal(err)
	}
	assertFloat(t, converted, 125)
}

func TestCalculateFinishesQuantitiesWasteAndExclusions(t *testing.T) {
	meters := func(value float64) Measurement { return Measurement{Value: value, Unit: UnitMeters} }
	plan := ConfirmedFloorPlan{Rooms: []Room{
		{ID: "a", Name: "Living", Length: meters(4), Width: meters(5), WallHeight: meters(2), Doors: []Opening{{Width: meters(1), Height: meters(2)}}},
		{ID: "b", Name: "Bath", Length: meters(2), Width: meters(2), WallHeight: meters(2.5), Finish: RoomFinish{TilePercent: 60, ExcludeFlooring: true}},
	}}
	rates := Rates{PlasterUSDPerSquareMeter: 10, PaintUSDPerSquareMeter: 5, FlooringUSDPerSquareMeter: 20, CeilingUSDPerSquareMeter: 8, SkirtingUSDPerMeter: 3, TilingUSDPerSquareMeter: 30, WastePercent: 10}
	result, err := Calculate(plan, rates)
	if err != nil {
		t.Fatal(err)
	}
	living, bath := result.Rooms[0], result.Rooms[1]
	assertFloat(t, living.Perimeter, 18)
	assertFloat(t, living.NetWallArea, 34)
	assertFloat(t, living.SkirtingLength, 17)
	assertFloat(t, living.FlooringArea, 20)
	assertFloat(t, living.CeilingArea, 20)
	assertFloat(t, living.PlasterCostUSD, 34*1.1*10)
	assertFloat(t, living.FlooringCostUSD, 20*1.1*20)
	assertFloat(t, living.SkirtingCostUSD, 17*1.1*3)
	assertFloat(t, bath.TileArea, 20*0.6)
	assertFloat(t, bath.PaintArea, 20-12)
	assertFloat(t, bath.NetPlasterArea, 20)
	assertFloat(t, bath.FlooringArea, 0)
	assertFloat(t, bath.FlooringCostUSD, 0)
	assertFloat(t, bath.TilingCostUSD, 12*1.1*30)
	assertFloat(t, result.GrandTotalUSD, living.TotalCostUSD+bath.TotalCostUSD)
	assertFloat(t, result.TotalTilingQuantity, 12)
}

func TestCalculateRejectsInvalidFinishInputs(t *testing.T) {
	meters := func(value float64) Measurement { return Measurement{Value: value, Unit: UnitMeters} }
	plan := ConfirmedFloorPlan{Rooms: []Room{{Name: "Room", Length: meters(3), Width: meters(3), WallHeight: meters(2.4), Finish: RoomFinish{TilePercent: 120}}}}
	if _, err := Calculate(plan, Rates{PlasterUSDPerSquareMeter: 1, PaintUSDPerSquareMeter: 1}); err == nil {
		t.Fatal("expected tiling above 100 percent to be rejected")
	}
	plan.Rooms[0].Finish = RoomFinish{}
	if _, err := Calculate(plan, Rates{PlasterUSDPerSquareMeter: 1, PaintUSDPerSquareMeter: 1, WastePercent: 60}); err == nil {
		t.Fatal("expected waste above 50 percent to be rejected")
	}
	if _, err := Calculate(plan, Rates{PlasterUSDPerSquareMeter: 1, PaintUSDPerSquareMeter: 1, FlooringUSDPerSquareMeter: -1}); err == nil {
		t.Fatal("expected negative finish rate to be rejected")
	}
}

func assertFloat(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %f, want %f", got, want)
	}
}
