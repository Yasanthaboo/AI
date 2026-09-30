package analysis

import (
	"testing"

	"github.com/example/ai-quantity-surveyor/api/internal/domain"
)

func TestValidateAcceptsStructuredReadyCandidate(t *testing.T) {
	measurement := func(value float64) *domain.Measurement {
		return &domain.Measurement{Value: value, Unit: domain.UnitMeters}
	}
	candidate := Candidate{State: StateReady, Rooms: []Room{{ID: "room-1", Name: "Living room", Length: measurement(4), Width: measurement(5), WallHeight: measurement(2.8), Confidence: 1}}}
	if err := Validate(candidate); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsMissingOrUncertainData(t *testing.T) {
	candidate := Candidate{State: StateReady, Rooms: []Room{{ID: "room-1", Name: "Living room", Confidence: 0.4}}}
	if err := Validate(candidate); err == nil {
		t.Fatal("expected missing measurement error")
	}
	candidate.Rooms = []Room{{ID: "room-1", Name: "Living room", Length: &domain.Measurement{Value: 4, Unit: domain.UnitMeters}, Width: &domain.Measurement{Value: 5, Unit: domain.UnitMeters}, WallHeight: &domain.Measurement{Value: 2.8, Unit: domain.UnitMeters}, Confidence: 0.4}}
	if err := Validate(candidate); err == nil {
		t.Fatal("expected uncertainty reason error")
	}
}
