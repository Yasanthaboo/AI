package analysis

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/example/ai-quantity-surveyor/api/internal/domain"
)

func TestCandidateContractRoundTrip(t *testing.T) {
	for _, fixture := range []struct {
		name string
		data string
	}{
		{name: "legacy", data: `{"id":"analysis-1","floorPlanId":"drawing-1","analysisState":"AnalysisReady","rooms":[{"id":"room-1","name":"Living","length":null,"width":null,"wallHeight":null,"confidence":0.4,"uncertainty":"unreadable"}],"doors":[],"windows":[]}`},
		{name: "evidence", data: `{"id":"analysis-1","floorPlanId":"drawing-1","analysisState":"AnalysisReady","source":{"floorPlanId":"drawing-1","pageNumber":2,"selectionMode":"explicit","imageDigest":"digest","pixelWidth":1000,"pixelHeight":800},"rooms":[{"id":"room-1","name":"Living","length":null,"width":null,"wallHeight":null,"confidence":0.4,"evidence":{"length":{"analysisId":"analysis-1","floorPlanId":"drawing-1","pageNumber":2,"region":{"x":0.1,"y":0.2,"width":0.2,"height":0.1},"observedText":"4.2","origin":"engine","confidence":0.7}}}],"features":[{"id":"stair-1","type":"stair","label":"Stair","origin":"manual","disposition":"pending"}],"reviewIssues":[{"entityId":"room-1","field":"length","code":"MissingMeasurement","message":"Review length","blocking":true}],"corrections":[{"entityId":"room-1","field":"length","originalValue":null,"confirmedValue":{"value":4.2,"unit":"m"},"origin":"user"}]}`},
		{name: "failure", data: `{"id":"analysis-1","analysisState":"AnalysisFailed","errorMessage":"Quota reached","error":{"errorCode":"ProviderQuotaExceeded","message":"Quota reached","retryable":true,"retryAt":"2026-10-14T00:00:00Z"}}`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var candidate Candidate
			if err := json.Unmarshal([]byte(fixture.data), &candidate); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			var restored Candidate
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(candidate, restored) {
				t.Fatal("candidate changed during JSON round trip")
			}
			if fixture.name == "legacy" && (restored.Source != nil || restored.Error != nil || len(restored.Features) != 0) {
				t.Fatal("legacy data gained invented fields")
			}
			if fixture.name == "evidence" && restored.Features[0].Confidence != nil {
				t.Fatal("manual feature gained invented confidence")
			}
			if fixture.name == "evidence" && restored.Features[0].Origin != "manual" {
				t.Fatal("manual feature origin was lost")
			}
		})
	}
}

func TestJobControlNeverSerialized(t *testing.T) {
	encoded, err := json.Marshal(JobControl{OwnerID: "internal-owner", AttemptToken: "internal-token"})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("internal job control leaked: %s", encoded)
	}
}

func TestValidateCandidateAllowsIncompleteReview(t *testing.T) {
	candidate := Candidate{State: StateReady, Rooms: []Room{{ID: "room-1", Name: "Living", Confidence: 0.4, Uncertainty: "Unreadable dimensions"}}, Windows: []Opening{{ID: "window-1", Confidence: 0.2, Uncertainty: "Association unknown"}}, Features: []Feature{{ID: "stair-1", Type: FeatureStair, Label: "Stair", Origin: "manual", Disposition: "pending"}}}
	if err := ValidateCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	if err := Validate(candidate); err == nil {
		t.Fatal("incomplete review was accepted as confirmed measurements")
	}
}

func TestValidateCandidateRejectsStructuralErrors(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		change func(*Candidate)
		want   string
	}{
		{name: "duplicate", change: func(candidate *Candidate) { candidate.Windows = []Opening{{ID: "room-1"}} }, want: "duplicate"},
		{name: "association", change: func(candidate *Candidate) { candidate.Windows = []Opening{{ID: "window-1", RoomID: "unknown"}} }, want: "unknown room"},
		{name: "nonfinite", change: func(candidate *Candidate) { candidate.Rooms[0].Confidence = math.NaN() }, want: "finite"},
		{name: "feature", change: func(candidate *Candidate) {
			candidate.Features = []Feature{{ID: "feature-1", Type: "roof", Label: "Roof"}}
		}, want: "unsupported type"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			candidate := Candidate{State: StateReady, Rooms: []Room{{ID: "room-1", Name: "Living", Confidence: 1}}}
			fixture.change(&candidate)
			if err := ValidateCandidate(candidate); err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("expected %q, got %v", fixture.want, err)
			}
		})
	}
}

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
