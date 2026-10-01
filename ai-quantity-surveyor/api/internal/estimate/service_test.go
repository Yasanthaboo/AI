package estimate

import (
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/domain"
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
	if first.Rates == nil || first.Rates.PlasterUSDPerSquareMeter != 10 || second.Rates.PlasterUSDPerSquareMeter != 12 {
		t.Fatalf("rates were not stored on versions: %+v %+v", first.Rates, second.Rates)
	}

	finished, err := service.Create("project-1", Request{AnalysisID: started.ID, PlasterUSDPerSquareMeter: 10, PaintUSDPerSquareMeter: 5, FlooringUSDPerSquareMeter: 20, WastePercent: 5, TargetCurrency: "USD", ExchangeRate: 1,
		Finishes: map[string]domain.RoomFinish{"room-3": {ExcludeFlooring: true, TilePercent: 50}, "room-2": {}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(finished.Finishes) != 1 || finished.Finishes["room-3"].TilePercent != 50 {
		t.Fatalf("only non-default finishes should be stored: %+v", finished.Finishes)
	}
	for _, room := range finished.Result.Rooms {
		if room.RoomID == "room-3" && (room.FlooringArea != 0 || room.TileArea == 0) {
			t.Fatalf("finish overrides were not applied: %+v", room)
		}
		if room.RoomID == "room-1" && room.FlooringCostUSD == 0 {
			t.Fatalf("flooring should be priced for rooms without overrides: %+v", room)
		}
	}
	if _, err := service.Create("project-1", Request{AnalysisID: started.ID, PlasterUSDPerSquareMeter: 10, PaintUSDPerSquareMeter: 5, WastePercent: 80, TargetCurrency: "USD", ExchangeRate: 1}); err == nil {
		t.Fatal("expected excessive waste to be rejected")
	}
}
