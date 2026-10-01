package portfolio

import (
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func TestSummarizeDerivesStageAndLatestTotal(t *testing.T) {
	store := project.NewService()
	analyses := analysis.NewServiceWithEngines(analysis.DemoAnalyzer{}, map[string]analysis.Analyzer{"demo": analysis.DemoAnalyzer{}}, nil)
	estimates := estimate.NewService(analyses)
	created, _ := store.CreateProject(project.Details{Name: "House"})
	stage := func() Summary {
		current, _ := store.GetProject(created.ID)
		return Summarize(store, analyses, estimates, current)
	}
	if got := stage(); got.Stage != StageDraft || got.FloorPlan != nil {
		t.Fatalf("draft summary = %+v", got)
	}
	floorPlan, err := store.SaveFloorPlan(created.ID, "plan.png", "image/png", []byte("\x89PNG\r\n\x1a\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := stage(); got.Stage != StageUploaded {
		t.Fatalf("uploaded summary = %+v", got)
	}
	started, _ := analyses.StartWithEngine("demo", floorPlan.ID, []byte("drawing"), "plan.png")
	var candidate analysis.Candidate
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if candidate, _ = analyses.Get(started.ID); candidate.State == analysis.StateReady {
			break
		}
	}
	if got := stage(); got.Stage != StageAnalyzed || got.AnalysisID != started.ID || got.Engine != "demo" || got.RoomCount != 4 {
		t.Fatalf("analyzed summary = %+v", got)
	}
	candidate.Rooms[0].Confidence, candidate.Rooms[0].Uncertainty = 1, ""
	if _, err := analyses.Confirm(started.ID, candidate); err != nil {
		t.Fatal(err)
	}
	if got := stage(); got.Stage != StageConfirmed || got.ConfirmedRevision != 1 {
		t.Fatalf("confirmed summary = %+v", got)
	}
	if _, err := estimates.Create(created.ID, estimate.Request{AnalysisID: started.ID, PlasterUSDPerSquareMeter: 10, PaintUSDPerSquareMeter: 5, TargetCurrency: "EUR", ExchangeRate: 2}); err != nil {
		t.Fatal(err)
	}
	got := stage()
	if got.Stage != StageEstimated || got.EstimateCount != 1 || got.LatestTotal == nil || *got.LatestTotal <= 0 || got.LatestCurrency != "EUR" || got.UpdatedAt.Before(got.Project.CreatedAt) {
		t.Fatalf("estimated summary = %+v", got)
	}
}
