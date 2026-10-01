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

func TestListArchiveAndDuplicate(t *testing.T) {
	service := NewService()
	source, _ := service.CreateProject(Details{Name: "House 01", Client: "Client A", Location: "Colombo"})
	if _, err := service.SaveFloorPlan(source.ID, "plan.png", "image/png", []byte("\x89PNG\r\n\x1a\n")); err != nil {
		t.Fatal(err)
	}
	archived, err := service.SetArchived(source.ID, true)
	if err != nil || !archived.Archived {
		t.Fatalf("archive failed: %+v %v", archived, err)
	}
	if _, err := service.SetArchived("missing", true); err == nil {
		t.Fatal("expected missing project error")
	}
	copied, err := Duplicate(service, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ID == source.ID || copied.Name != "House 01 (copy)" || copied.Client != "Client A" || copied.Archived || copied.Status != "Uploaded" {
		t.Fatalf("unexpected duplicate: %+v", copied)
	}
	plan, ok := service.FloorPlanForProject(copied.ID)
	if !ok || plan.FileName != "plan.png" {
		t.Fatalf("drawing was not copied: %+v", plan)
	}
	if projects := service.ListProjects(); len(projects) != 2 {
		t.Fatalf("projects = %+v", projects)
	}
	if _, err := Duplicate(service, "missing"); err == nil {
		t.Fatal("expected missing project error")
	}
}
