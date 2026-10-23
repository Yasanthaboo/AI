package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/auth"
	"github.com/example/ai-quantity-surveyor/api/internal/exterior"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func TestExteriorRoutesRequireOwnedConfirmedRevision(t *testing.T) {
	projects := project.NewService()
	created, err := projects.CreateProjectForOwner(project.Details{Name: "Exterior fixture"}, "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	plan, err := projects.SaveFloorPlan(created.ID, "plan.png", "image/png", png)
	if err != nil {
		t.Fatal(err)
	}
	analyses := analysis.NewService(analysis.DemoAnalyzer{})
	started, err := analyses.Start(plan.ID, png, "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	var candidate analysis.Candidate
	for {
		var found bool
		candidate, found = analyses.Get(started.ID)
		if found && candidate.State == analysis.StateReady {
			break
		}
		select {
		case <-deadline:
			t.Fatal("demo analysis did not complete")
		case <-ticker.C:
		}
	}
	confirmed, err := analyses.Confirm(started.ID, candidate)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := exterior.NewStore(nil)
	mux := http.NewServeMux()
	registerExteriorHandlers(mux, projects, analyses, store)
	call := func(owner, method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: owner}))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	input := exterior.Input{AnalysisID: started.ID, ConfirmationRevision: confirmed.ConfirmationRevision, Roof: "hip", PitchDegrees: 30, OverhangM: 0.4, LengthM: 12, WidthM: 8, WallHeightM: 2.7, Finish: "metal", AssumptionsReviewed: true}
	body, _ := json.Marshal(input)
	path := "/api/projects/" + created.ID + "/exterior-options"
	if response := call("owner-b", "POST", path, string(body)); response.Code != 404 {
		t.Fatalf("foreign save=%d", response.Code)
	}
	if response := call("owner-a", "POST", path, string(body)); response.Code != 201 {
		t.Fatalf("save=%d %s", response.Code, response.Body.String())
	}
	input.ConfirmationRevision++
	body, _ = json.Marshal(input)
	if response := call("owner-a", "POST", path, string(body)); response.Code != 409 {
		t.Fatalf("unknown revision=%d", response.Code)
	}
	query := path + "?analysisId=" + started.ID + "&revision=1"
	if response := call("owner-b", "GET", query, ""); response.Code != 404 {
		t.Fatalf("foreign read=%d", response.Code)
	}
	if response := call("owner-a", "GET", query, ""); response.Code != 200 || !strings.Contains(response.Body.String(), `"roof":"hip"`) {
		t.Fatalf("saved read=%d %s", response.Code, response.Body.String())
	}
}
