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

func assertFloat(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %f, want %f", got, want)
	}
}
