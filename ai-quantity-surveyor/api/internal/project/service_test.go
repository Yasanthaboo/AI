package project

import "testing"

func TestSaveFloorPlanValidatesTypeSizeAndOneDrawingRule(t *testing.T) {
	service := NewService()
	created, err := service.CreateProject(Details{Name: "House 01", Client: "Client A"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Client != "Client A" {
		t.Fatalf("client not stored: %+v", created)
	}
	if _, err := service.CreateProject(Details{Name: "  "}); err == nil {
		t.Fatal("expected name validation")
	}
	content := []byte("%PDF-1.7")
	if _, err := service.SaveFloorPlan(created.ID, "plan.pdf", "application/pdf", content); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveFloorPlan(created.ID, "second.pdf", "application/pdf", content); err == nil {
		t.Fatal("expected one-drawing rule")
	}
	if _, err := service.SaveFloorPlan("missing", "plan.pdf", "application/pdf", content); err == nil {
		t.Fatal("expected missing project error")
	}
	if _, err := service.SaveFloorPlan("new", "plan.txt", "text/plain", content); err == nil {
		t.Fatal("expected invalid file error")
	}
}
