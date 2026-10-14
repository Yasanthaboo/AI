package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/portfolio"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

type handlerPageProcessor struct{}

func (handlerPageProcessor) Metadata(context.Context, []byte, string) (analysis.PageMetadata, error) {
	return analysis.PageMetadata{PageCount: 2, ContentType: "application/pdf"}, nil
}

func (handlerPageProcessor) Prepare(_ context.Context, _ []byte, _ string, page int, mode string) (analysis.PreparedPage, error) {
	content := []byte("canonical-page")
	digest := sha256.Sum256(content)
	return analysis.PreparedPage{Content: content, Source: analysis.Source{PageNumber: page, SelectionMode: mode, ImageDigest: hex.EncodeToString(digest[:]), PixelWidth: 20, PixelHeight: 10}}, nil
}

func TestManagedAnalysisHandlers(t *testing.T) {
	store := project.NewService()
	created, _ := store.CreateProject(project.Details{Name: "Sample"})
	floorPlan, err := store.SaveFloorPlan(created.ID, "drawing.pdf", "application/pdf", []byte("%PDF-1.4\nfixture"))
	if err != nil {
		t.Fatal(err)
	}
	service := analysis.NewServiceWithEngines(analysis.DemoAnalyzer{}, map[string]analysis.Analyzer{"demo": analysis.DemoAnalyzer{}, "gemini": analysis.DemoAnalyzer{}}, nil)
	if err := service.EnableRuntime(analysis.RuntimeConfig{Jobs: analysis.NewMemoryJobStore(), Preparer: handlerPageProcessor{}, DefaultEngine: "demo"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	mux := http.NewServeMux()
	registerAnalysisHandlers(mux, store, service)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		return response
	}
	path := "/api/floor-plans/" + floorPlan.ID
	if response := call(http.MethodGet, path+"/pages", ""); response.Code != 200 || !strings.Contains(response.Body.String(), `"pageCount":2`) {
		t.Fatalf("metadata: %d %s", response.Code, response.Body.String())
	}
	for _, body := range []string{`{"pageNumber":0}`, `{"pageNumber":3}`, `{"pageNumber":1.5}`, `{} {}`, `{"unknown":true}`} {
		if response := call(http.MethodPost, path+"/analysis", body); response.Code != 400 {
			t.Fatalf("accepted invalid input %s: %d", body, response.Code)
		}
	}
	if response := call(http.MethodPost, path+"/analysis", `{"engine":"gemini"}`); response.Code != 503 || !strings.Contains(response.Body.String(), "ProviderQuotaNotConfigured") {
		t.Fatalf("missing quota: %d %s", response.Code, response.Body.String())
	}
	response := call(http.MethodPost, path+"/analysis", `{"engine":"demo","pageNumber":2}`)
	if response.Code != 202 {
		t.Fatalf("start: %d %s", response.Code, response.Body.String())
	}
	var started analysis.Candidate
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	var candidate analysis.Candidate
	for deadline := time.Now().Add(2 * time.Second); ; {
		response = call(http.MethodGet, "/api/analyses/"+started.ID, "")
		if response.Code != 200 {
			t.Fatalf("poll: %d %s", response.Code, response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &candidate); err != nil {
			t.Fatal(err)
		}
		if candidate.State != analysis.StateAnalyzing {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("analysis did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if candidate.State != analysis.StateReady || candidate.Source == nil || candidate.Source.PageNumber != 2 {
		t.Fatalf("wrong selected page: %+v", candidate)
	}
	image := call(http.MethodGet, "/api/analyses/"+started.ID+"/source-image", "")
	if image.Code != 200 || image.Body.String() != "canonical-page" || image.Header().Get("Content-Type") != "image/png" || image.Header().Get("ETag") == "" {
		t.Fatalf("source image: %d %s", image.Code, image.Body.String())
	}
	conditional := httptest.NewRequest(http.MethodGet, "/api/analyses/"+started.ID+"/source-image", nil)
	conditional.Header.Set("If-None-Match", image.Header().Get("ETag"))
	cached := httptest.NewRecorder()
	mux.ServeHTTP(cached, conditional)
	if cached.Code != 304 || cached.Body.Len() != 0 {
		t.Fatal("source cache conditional failed")
	}
	if got := call(http.MethodGet, "/api/analyses/missing", "").Code; got != 404 {
		t.Fatalf("missing status: %d", got)
	}
	if got := call(http.MethodPost, path+"/analysis", strings.Repeat(" ", 65537)+`{}`).Code; got != 413 {
		t.Fatalf("oversized body: %d", got)
	}
}

func TestProviderLimitsFromEnvironment(t *testing.T) {
	t.Setenv("GEMINI_DAILY_CALL_LIMIT", "10")
	t.Setenv("OLLAMA_DAILY_CALL_LIMIT", "0")
	limits, err := providerLimitsFromEnvironment()
	if err != nil || limits["gemini"] != 10 || limits["ollama"] != 0 {
		t.Fatalf("limits: %v %v", limits, err)
	}
	t.Setenv("GEMINI_DAILY_CALL_LIMIT", "invalid")
	if _, err := providerLimitsFromEnvironment(); err == nil {
		t.Fatal("invalid provider quota accepted")
	}
}

type quotaRejectedStore struct{ *analysis.MemoryJobStore }

func (quotaRejectedStore) ReserveCall(context.Context, string, analysis.JobControl, string, string, string, int) error {
	reset := time.Now().UTC().Add(time.Hour)
	return &analysis.RuntimeError{Status: 429, Failure: analysis.AnalysisError{ErrorCode: "ProviderQuotaExceeded", Message: "Daily limit reached", Retryable: true, RetryAt: &reset}}
}

type handlerCountingAnalyzer struct{ calls *atomic.Int32 }

func (analyzer handlerCountingAnalyzer) Analyze(ctx context.Context, content []byte, fileName string) (analysis.Candidate, error) {
	analyzer.calls.Add(1)
	return (analysis.DemoAnalyzer{}).Analyze(ctx, content, fileName)
}

func TestAcceptedQuotaFailureIsPolledWithoutProviderCall(t *testing.T) {
	store := project.NewService()
	created, _ := store.CreateProject(project.Details{Name: "Sample quota"})
	floorPlan, err := store.SaveFloorPlan(created.ID, "drawing.pdf", "application/pdf", []byte("%PDF-1.4\nfixture"))
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	analyzer := handlerCountingAnalyzer{calls: &calls}
	service := analysis.NewServiceWithEngines(analyzer, map[string]analysis.Analyzer{"gemini": analyzer}, nil)
	if err := service.EnableRuntime(analysis.RuntimeConfig{Jobs: quotaRejectedStore{analysis.NewMemoryJobStore()}, Preparer: handlerPageProcessor{}, DefaultEngine: "gemini", QuotaScope: "test", DailyLimits: map[string]int{"gemini": 1}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	mux := http.NewServeMux()
	registerAnalysisHandlers(mux, store, service)
	accepted := httptest.NewRecorder()
	mux.ServeHTTP(accepted, httptest.NewRequest(http.MethodPost, "/api/floor-plans/"+floorPlan.ID+"/analysis", strings.NewReader(`{"engine":"gemini","pageNumber":2}`)))
	if accepted.Code != 202 {
		t.Fatalf("not accepted: %d %s", accepted.Code, accepted.Body.String())
	}
	var started analysis.Candidate
	if err := json.Unmarshal(accepted.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Second); ; {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/analyses/"+started.ID, nil))
		if response.Code != 200 {
			t.Fatalf("stored failure changed polling HTTP status: %d", response.Code)
		}
		var candidate analysis.Candidate
		if err := json.Unmarshal(response.Body.Bytes(), &candidate); err != nil {
			t.Fatal(err)
		}
		if candidate.State == analysis.StateFailed {
			if candidate.Error == nil || candidate.Error.ErrorCode != "ProviderQuotaExceeded" || candidate.Error.RetryAt == nil || candidate.ErrorMessage == "" {
				t.Fatalf("failure metadata missing: %+v", candidate)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("quota rejection was not persisted")
		}
		time.Sleep(time.Millisecond)
	}
	if calls.Load() != 0 || accepted.Code != 202 {
		t.Fatal("quota rejection invoked provider or rewrote acceptance")
	}
}

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

func TestCORSUsesConfiguredOrigin(t *testing.T) {
	t.Setenv("ALLOWED_ORIGIN", "https://demo.example.com")
	handler := withCORS(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://demo.example.com" {
		t.Fatalf("allowed origin = %q", got)
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
