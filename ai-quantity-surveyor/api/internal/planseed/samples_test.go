package planseed

import (
	"bytes"
	"context"
	"image/png"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func TestSampleWorkspacesAreIsolatedAndIdempotent(t *testing.T) {
	projects := project.NewService()
	analyses := analysis.NewServiceWithEngines(analysis.DemoAnalyzer{}, map[string]analysis.Analyzer{"demo": analysis.DemoAnalyzer{}}, nil)
	if err := analyses.EnableRuntime(analysis.RuntimeConfig{Jobs: analysis.NewMemoryJobStore(), Preparer: analysis.NewPagePreparer(), DefaultEngine: "demo"}); err != nil {
		t.Fatal(err)
	}
	defer analyses.Close()
	estimates := estimate.NewService(analyses)
	registry, _ := newWorkspaceRegistry(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, user := range []User{{ID: "free-user", PlanID: "free"}, {ID: "pro-user", PlanID: "pro"}, {ID: "studio-user", PlanID: "studio"}} {
		if err := ensureSample(ctx, user, projects, analyses, estimates, registry); err != nil {
			t.Fatal(err)
		}
		owned := projects.ListProjectsForOwner(user.ID)
		if len(owned) != 1 || owned[0].Description != sampleDescription {
			t.Fatal("sample workspace missing or shared")
		}
		versions := estimates.List(owned[0].ID)
		if len(versions) != 1 || versions[0].SourceConfirmationRevision != 1 || versions[0].SourceImageDigest == "" || versions[0].Result.GrandTotalUSD <= 0 {
			t.Fatalf("sample estimate lacks confirmed canonical provenance: %+v", versions)
		}
		if err := ensureSample(ctx, user, projects, analyses, estimates, registry); err != nil {
			t.Fatal(err)
		}
		if len(projects.ListProjectsForOwner(user.ID)) != 1 || len(estimates.List(owned[0].ID)) != 1 {
			t.Fatal("rerun duplicated sample project or estimate")
		}
		if history := analyses.Confirmations(versions[0].SourceAnalysisID); len(history) != 1 {
			t.Fatal("rerun changed confirmation history")
		}
	}
	if len(projects.ListProjects()) != 3 {
		t.Fatal("incorrect seeded project count")
	}
	content, err := sampleDrawing()
	if err != nil {
		t.Fatal(err)
	}
	if image, err := png.Decode(bytes.NewReader(content)); err != nil || image.Bounds().Dx() != 500 {
		t.Fatal("synthetic drawing was not a valid sample PNG")
	}
}
