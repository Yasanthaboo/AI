package project

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestProjectOwnershipAndOwnerlessMigration(t *testing.T) {
	service := NewService()
	first, err := service.CreateProjectForOwner(Details{Name: "Private project"}, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, found := service.GetProjectForOwner(first.ID, "user-b"); found {
		t.Fatal("foreign owner read project")
	}
	if projects := service.ListProjectsForOwner("user-b"); len(projects) != 0 {
		t.Fatalf("foreign owner listed projects: %+v", projects)
	}
	if _, err := service.SetArchivedForOwner(first.ID, "user-b", true); err == nil {
		t.Fatal("foreign owner archived project")
	}
	if _, err := DuplicateForOwner(service, first.ID, "user-b"); err == nil {
		t.Fatal("foreign owner duplicated project")
	}

	legacy, err := service.CreateProject(Details{Name: "Legacy project"})
	if err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	legacy.OwnerID = ""
	service.projects[legacy.ID] = legacy
	service.mu.Unlock()
	storePath := filepath.Join(t.TempDir(), "projects.json")
	service.persistencePath = storePath
	if err := service.AssignOwnerlessProjects("admin-user"); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewPersistentService(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := reopened.GetProjectForOwner(legacy.ID, "admin-user"); !found {
		t.Fatal("adopted owner was not persisted")
	}
	if _, found := reopened.GetProjectForOwner(first.ID, "admin-user"); found {
		t.Fatal("migration reassigned an already-owned project")
	}
}

func TestPostgresOwnerMigrationAndIsolation(t *testing.T) {
	connection := os.Getenv("SURVEYOR_JOB_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("disposable project-owner test database not configured")
	}
	parsed, err := url.Parse(connection)
	if err != nil || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "surveyor_job_test") {
		t.Fatal("project-owner migration requires a disposable surveyor_job_test database")
	}
	store, err := NewPostgresService(connection)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	legacy, err := store.CreateProject(Details{Name: "Owner migration fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AssignOwnerlessProjects("admin-user"); err != nil {
		t.Fatal(err)
	}
	if _, found := store.GetProjectForOwner(legacy.ID, "admin-user"); !found {
		t.Fatal("legacy project was not assigned to initial admin")
	}

	owned, err := store.CreateProjectForOwner(Details{Name: "Owner A fixture"}, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, found := store.GetProjectForOwner(owned.ID, "user-b"); found {
		t.Fatal("PostgreSQL returned project to foreign owner")
	}
	foundOwned := false
	for _, item := range store.ListProjectsForOwner("user-a") {
		if item.ID == owned.ID {
			foundOwned = true
		}
	}
	if !foundOwned {
		t.Fatal("PostgreSQL owner-filtered list omitted project")
	}
	if _, err := store.SetArchivedForOwner(owned.ID, "user-b", true); err == nil {
		t.Fatal("PostgreSQL allowed a foreign owner to archive project")
	}
}
