package planseed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/plans"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func TestPreviewSeedPostgresRestartAndIdempotence(t *testing.T) {
	databaseURL := os.Getenv("SURVEYOR_PLAN_SEED_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SURVEYOR_PLAN_SEED_TEST_DATABASE_URL to a disposable preview seed database")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "surveyor_plan_seed_test") {
		t.Fatal("refusing a database outside the surveyor_plan_seed_test prefix")
	}
	confirmed := "2026-10-22T00:00:00Z"
	identities := []adminUser{}
	for _, tier := range []string{"free", "pro", "studio"} {
		identities = append(identities, adminUser{ID: "preview-" + tier, Email: "demo-" + tier + "@quantity-surveyor.test", AppMetadata: map[string]any{"plan_preview_seed": marker, "plan_preview_tier": tier}, EmailConfirmedAt: &confirmed})
	}
	admin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Error("existing preview password or identity was mutated")
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"users": identities})
	}))
	defer admin.Close()
	values := seedEnvironment(admin.URL)
	config, err := FromEnvironment(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	open := func() (*project.PostgresService, *analysis.Service, *estimate.Service, *plans.Store) {
		projects, err := project.NewPostgresService(databaseURL)
		if err != nil {
			t.Fatal("could not connect to disposable preview database")
		}
		repository, err := analysis.NewPostgresRepository(projects.DB())
		if err != nil {
			t.Fatal(err)
		}
		analyses := analysis.NewServiceWithEngines(analysis.DemoAnalyzer{}, map[string]analysis.Analyzer{"demo": analysis.DemoAnalyzer{}}, repository)
		if err := analyses.EnableRuntime(analysis.RuntimeConfig{Jobs: repository, Preparer: analysis.NewPagePreparer(), DefaultEngine: "demo"}); err != nil {
			t.Fatal(err)
		}
		estimateRepository, err := estimate.NewPostgresRepository(projects.DB())
		if err != nil {
			t.Fatal(err)
		}
		planStore, err := plans.NewStore(projects.DB())
		if err != nil {
			t.Fatal(err)
		}
		return projects, analyses, estimate.NewServiceWithRepository(analyses, estimateRepository), planStore
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	projects, analyses, estimates, planStore := open()
	users, err := Run(ctx, config, projects, analyses, estimates, planStore)
	if err != nil {
		analyses.Close()
		projects.Close()
		t.Fatal(err)
	}
	projectIDs := map[string]string{}
	for _, user := range users {
		owned := projects.ListProjectsForOwner(user.ID)
		if len(owned) != 1 {
			t.Fatal("preview user did not receive exactly one project")
		}
		projectIDs[user.ID] = owned[0].ID
		versions := estimates.List(owned[0].ID)
		if len(versions) != 1 || versions[0].SourceImageDigest == "" || versions[0].SourceConfirmationRevision != 1 {
			t.Fatal("seeded estimate lacks durable canonical provenance")
		}
		if _, found := projects.GetProjectForOwner(owned[0].ID, "unrelated-customer"); found {
			t.Fatal("preview project leaked to unrelated account")
		}
	}
	if _, err := projects.SetArchivedForOwner(projectIDs["preview-pro"], "preview-pro", true); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.DB().ExecContext(ctx, `UPDATE account_plans SET plan_id='free' WHERE user_id='preview-pro'`); err != nil {
		t.Fatal(err)
	}
	analyses.Close()
	projects.Close()
	projects, analyses, estimates, planStore = open()
	defer analyses.Close()
	defer projects.Close()
	users, err = Run(ctx, config, projects, analyses, estimates, planStore)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		owned := projects.ListProjectsForOwner(user.ID)
		if len(owned) != 1 || owned[0].ID != projectIDs[user.ID] || len(estimates.List(owned[0].ID)) != 1 {
			t.Fatal("restart duplicated or replaced preview workspace")
		}
		if user.ID == "preview-pro" && !owned[0].Archived {
			t.Fatal("seed reset tester project changes")
		}
		account, err := planStore.Account(ctx, user.ID, false)
		if err != nil || account.Plan.ID != user.PlanID || account.AssignmentSource != "manual" {
			t.Fatal("seed plan not restored in production mode")
		}
	}
}
