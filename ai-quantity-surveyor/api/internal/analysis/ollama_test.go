package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaAnalyzerParsesStructuredResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization header = %q", request.Header.Get("Authorization"))
		}
		var sent struct {
			Format map[string]any `json:"format"`
		}
		if err := json.NewDecoder(request.Body).Decode(&sent); err != nil || sent.Format["properties"] == nil {
			t.Errorf("request must send the candidate JSON schema as format: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		content := "```json\n{\"state\":\"AnalysisReady\",\"rooms\":[{\"id\":\"r1\",\"name\":\"Room\",\"length\":{\"value\":4,\"unit\":\"m\"},\"width\":{\"value\":5,\"unit\":\"m\"},\"wallHeight\":{\"value\":2.8,\"unit\":\"m\"},\"confidence\":{\"score\":1}}]}\n```"
		body, _ := json.Marshal(map[string]map[string]string{"message": {"content": content}})
		_, _ = writer.Write(body)
	}))
	defer server.Close()
	candidate, err := (OllamaAnalyzer{Endpoint: server.URL, APIKey: "test-key", Client: server.Client()}).Analyze(context.Background(), []byte("image"), "plan.png")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidate.Rooms) != 1 || candidate.State != StateReady {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
}

func TestNormalizeConfidenceFlattensStructuredUncertainty(t *testing.T) {
	normalized, err := normalizeConfidence(`{"analysisState":"AnalysisReady","rooms":[
		{"id":"r1","name":"Hall","confidence":0.6,"uncertainty":{"wallHeight":"not shown","length":"blurred text"}},
		{"id":"r2","name":"Bath","confidence":0.7,"uncertainty":["scale unclear",null]},
		{"id":"r3","name":"Store","confidence":1,"uncertainty":null}],
		"doors":[{"id":"d1","confidence":0.5,"uncertainty":{"reason":"partly hidden"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	var candidate Candidate
	if err := json.Unmarshal(normalized, &candidate); err != nil {
		t.Fatal(err)
	}
	if got := candidate.Rooms[0].Uncertainty; got != "length: blurred text; wallHeight: not shown" {
		t.Fatalf("room uncertainty = %q", got)
	}
	if candidate.Rooms[1].Uncertainty != "scale unclear" || candidate.Rooms[2].Uncertainty != "" || candidate.Doors[0].Uncertainty != "reason: partly hidden" {
		t.Fatalf("unexpected uncertainties: %+v %+v", candidate.Rooms, candidate.Doors)
	}
}

func TestNormalizeConfidenceWrapsTopLevelRoomArray(t *testing.T) {
	normalized, err := normalizeConfidence(`[{"length":{"value":4,"unit":"m"},"width":{"value":5,"unit":"m"},"wallHeight":{"value":2.8,"unit":"m"},"confidence":{"score":1}}]`)
	if err != nil {
		t.Fatal(err)
	}
	var candidate Candidate
	if err := json.Unmarshal(normalized, &candidate); err != nil {
		t.Fatal(err)
	}
	if candidate.State != StateReady || len(candidate.Rooms) != 1 || candidate.Rooms[0].ID != "room-1" || candidate.Rooms[0].Name != "Room 1" || candidate.Rooms[0].Confidence != 1 {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
}
