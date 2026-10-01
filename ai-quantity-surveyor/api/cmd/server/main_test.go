package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/portfolio"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func TestConfirmationReadHandlers(t *testing.T) {
	service := analysis.NewService(analysis.DemoAnalyzer{})
	mux := http.NewServeMux()
	registerConfirmationHandlers(mux, service)
	started, err := service.Start("floor-1", []byte("drawing"), "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) *httptest.ResponseRecorder {
		writer := httptest.NewRecorder()
		mux.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, path, nil))
		return writer
	}
	path := "/api/analyses/" + started.ID + "/confirmed-data"
	if got := read(path).Code; got != http.StatusNotFound {
		t.Fatalf("unconfirmed status = %d", got)
	}
	deadline := time.Now().Add(time.Second)
	var candidate analysis.Candidate
	for {
		candidate, _ = service.Get(started.ID)
		if candidate.State == analysis.StateReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("analysis did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	candidate.Rooms[0].Confidence = 1
	candidate.Rooms[0].Uncertainty = ""
	if _, err := service.Confirm(started.ID, candidate); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{path, path + "/1"} {
		response := read(endpoint)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", endpoint, response.Code)
		}
		var result analysis.Confirmation
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Revision != 1 || result.AnalysisID != started.ID || len(result.Candidate.Rooms) != 4 {
			t.Fatalf("invalid confirmation response: %+v, %v", result, err)
		}
	}
	if got := read(path + "/2").Code; got != http.StatusNotFound {
		t.Fatalf("unknown revision status = %d", got)
	}
	if got := read(path + "/invalid").Code; got != http.StatusBadRequest {
		t.Fatalf("invalid revision status = %d", got)
	}
	if got := read("/api/analyses/other/confirmed-data").Code; got != http.StatusNotFound {
		t.Fatalf("unknown analysis status = %d", got)
	}
	var history []analysis.Confirmation
	if err := json.Unmarshal(read("/api/analyses/"+started.ID+"/confirmations").Body.Bytes(), &history); err != nil || len(history) != 1 || history[0].ConfirmedAt.IsZero() {
		t.Fatalf("history = %+v, %v", history, err)
	}
	if body := read("/api/analyses/other/confirmations").Body.String(); strings.TrimSpace(body) != "[]" {
		t.Fatalf("empty history body = %q", body)
	}
}

func TestPortfolioHandlers(t *testing.T) {
	store := project.NewService()
	analyses := analysis.NewService(analysis.DemoAnalyzer{})
	mux := http.NewServeMux()
	registerPortfolioHandlers(mux, store, analyses, estimate.NewService(analyses))
	call := func(method, path, body string) *httptest.ResponseRecorder {
		writer := httptest.NewRecorder()
		mux.ServeHTTP(writer, httptest.NewRequest(method, path, strings.NewReader(body)))
		return writer
	}
	created, _ := store.CreateProject(project.Details{Name: "House"})
	floorPlan, err := store.SaveFloorPlan(created.ID, "plan.png", "text/html", []byte("\x89PNG\r\n\x1a\n"))
	if err != nil {
		t.Fatal(err)
	}
	var summaries []portfolio.Summary
	if err := json.Unmarshal(call(http.MethodGet, "/api/projects", "").Body.Bytes(), &summaries); err != nil || len(summaries) != 1 || summaries[0].Stage != portfolio.StageUploaded {
		t.Fatalf("summaries = %+v, %v", summaries, err)
	}
	if got := call(http.MethodGet, "/api/projects/"+created.ID+"/summary", "").Code; got != http.StatusOK {
		t.Fatalf("summary status = %d", got)
	}
	if got := call(http.MethodPatch, "/api/projects/"+created.ID, `{}`).Code; got != http.StatusBadRequest {
		t.Fatalf("empty patch status = %d", got)
	}
	if got := call(http.MethodPatch, "/api/projects/missing", `{"archived":true}`).Code; got != http.StatusNotFound {
		t.Fatalf("missing patch status = %d", got)
	}
	if response := call(http.MethodPatch, "/api/projects/"+created.ID, `{"archived":true}`); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"archived":true`) {
		t.Fatalf("archive response = %d %s", response.Code, response.Body.String())
	}
	duplicate := call(http.MethodPost, "/api/projects/"+created.ID+"/duplicate", "")
	if duplicate.Code != http.StatusCreated || !strings.Contains(duplicate.Body.String(), "House (copy)") {
		t.Fatalf("duplicate response = %d %s", duplicate.Code, duplicate.Body.String())
	}
	file := call(http.MethodGet, "/api/floor-plans/"+floorPlan.ID+"/file", "")
	if file.Code != http.StatusOK || file.Header().Get("Content-Type") != "image/png" || file.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("file response = %d %v", file.Code, file.Header())
	}
	if got := call(http.MethodGet, "/api/floor-plans/missing/file", "").Code; got != http.StatusNotFound {
		t.Fatalf("missing file status = %d", got)
	}
}
