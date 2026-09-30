package estimate

import (
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
)

func TestCreateRequiresConfirmationAndPreservesVersions(t *testing.T) {
	analysisService := analysis.NewService(analysis.DemoAnalyzer{})
	started, err := analysisService.Start("fp-1", []byte("drawing"), "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if result, ok := analysisService.Get(started.ID); ok && result.State == analysis.StateReady {
			_, err = analysisService.Confirm(started.ID, result)
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(analysisService)
	first, err := service.Create("project-1", Request{AnalysisID: started.ID, PlasterUSDPerSquareMeter: 10, PaintUSDPerSquareMeter: 5, TargetCurrency: "EUR", ExchangeRate: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create("project-1", Request{AnalysisID: started.ID, PlasterUSDPerSquareMeter: 12, PaintUSDPerSquareMeter: 5, TargetCurrency: "EUR", ExchangeRate: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || second.Version != 2 || len(service.List("project-1")) != 2 {
		t.Fatalf("versions were not preserved: %+v", service.List("project-1"))
	}
}
