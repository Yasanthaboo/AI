package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
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
}
