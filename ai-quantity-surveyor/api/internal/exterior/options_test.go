package exterior

import (
	"context"
	"math"
	"testing"
)

func TestExteriorOptionsValidationAndRevisionIsolation(t *testing.T) {
	store, err := NewStore(nil)
	if err != nil {
		t.Fatal(err)
	}
	input := Input{AnalysisID: "analysis-a", ConfirmationRevision: 1, Roof: "gable", PitchDegrees: 30, OverhangM: 0.4, LengthM: 12, WidthM: 8, WallHeightM: 2.7, Finish: "metal", AssumptionsReviewed: true}
	for _, roof := range []string{"flat", "gable", "hip", "shed"} {
		candidate := input
		candidate.Roof = roof
		if roof == "flat" {
			candidate.PitchDegrees = 0
		}
		if _, err := store.Save(context.Background(), "project-a", candidate); err != nil {
			t.Fatalf("%s: %v", roof, err)
		}
	}
	for _, mutate := range []func(*Input){
		func(value *Input) { value.AssumptionsReviewed = false }, func(value *Input) { value.Roof = "invented" },
		func(value *Input) { value.LengthM = math.NaN() }, func(value *Input) { value.PitchDegrees = 90 },
		func(value *Input) { value.OverhangM = 3 }, func(value *Input) { value.WallHeightM = 0 },
		func(value *Input) { value.Roof = "flat" }, func(value *Input) { value.Finish = "unknown" },
	} {
		candidate := input
		mutate(&candidate)
		if _, err := store.Save(context.Background(), "project-a", candidate); err == nil {
			t.Fatal("invalid exterior assumption accepted")
		}
	}
	options, err := store.List(context.Background(), "project-a", "analysis-a", 1)
	if err != nil || len(options) != 4 || options[0].Assumptions != Assumptions {
		t.Fatalf("saved options: %+v %v", options, err)
	}
	for _, query := range []struct {
		project, analysis string
		revision          int
	}{{"project-b", "analysis-a", 1}, {"project-a", "analysis-b", 1}, {"project-a", "analysis-a", 2}} {
		options, err := store.List(context.Background(), query.project, query.analysis, query.revision)
		if err != nil || len(options) != 0 {
			t.Fatal("options leaked across source snapshots")
		}
	}
}
