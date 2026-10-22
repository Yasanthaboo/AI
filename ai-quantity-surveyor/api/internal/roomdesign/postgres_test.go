package roomdesign

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func TestPostgresRoomDesignStorageAndQuota(t *testing.T) {
	connection := os.Getenv("SURVEYOR_JOB_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("disposable room-design test database not configured")
	}
	parsed, err := url.Parse(connection)
	if err != nil || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "surveyor_job_test") {
		t.Fatal("room-design tests require a disposable surveyor_job_test database")
	}
	projects, err := project.NewPostgresService(connection)
	if err != nil {
		t.Fatal(err)
	}
	defer projects.Close()
	created, err := projects.CreateProjectForOwner(project.Details{Name: "Disposable design fixture"}, "design-user")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewPostgresRepository(projects.DB())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	concepts := []Concept{
		{ID: newID(), ProjectID: created.ID, AnalysisID: "analysis", ConfirmationRevision: 1, RoomID: "room", RoomName: "Kitchen", Style: "Contemporary", Palette: "Warm neutrals", Model: "test-model", CreatedAt: time.Now().UTC()},
		{ID: newID(), ProjectID: created.ID, AnalysisID: "analysis", ConfirmationRevision: 1, RoomID: "room", RoomName: "Kitchen", Style: "Scandinavian", Palette: "Coastal", Model: "test-model", CreatedAt: time.Now().UTC()},
	}
	if err := repository.SaveBatch(ctx, concepts, [][]byte{[]byte("png-one"), []byte("png-two")}); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReserveDaily(ctx, "design-user", time.Now().UTC(), 2, 2); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReserveDaily(ctx, "design-user", time.Now().UTC(), 1, 2); err != ErrDailyLimit {
		t.Fatalf("expected daily limit, got %v", err)
	}
	if err := repository.Select(ctx, created.ID, "room", 1, concepts[1].ID); err != nil {
		t.Fatal(err)
	}
	listed, err := repository.List(ctx, created.ID, "room", 1)
	if err != nil || len(listed) != 2 {
		t.Fatalf("design list = %+v, %v", listed, err)
	}
	selected := 0
	for _, concept := range listed {
		if concept.Selected {
			selected++
		}
	}
	if selected != 1 {
		t.Fatalf("selected concept count = %d", selected)
	}
	image, found, err := repository.Image(ctx, concepts[1].ID)
	if err != nil || !found || string(image) != "png-two" {
		t.Fatalf("stored image = %q, %v, %v", image, found, err)
	}
	if err := repository.Delete(ctx, concepts[0].ID); err != nil {
		t.Fatal(err)
	}
}
