package analysis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiAnalyzerSendsDrawingAndParsesResponse(t *testing.T) {
	for _, test := range []struct{ name, mimeType string }{{"plan.png", "image/png"}, {"plan.pdf", "application/pdf"}} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method == http.MethodGet {
					if request.URL.Path != "/models/test-model" || request.Header.Get("x-goog-api-key") != "test-key" {
						t.Errorf("unexpected model check: %s, key %q", request.URL.Path, request.Header.Get("x-goog-api-key"))
					}
					writer.WriteHeader(http.StatusOK)
					return
				}
				if request.URL.Path != "/models/test-model:generateContent" || request.Header.Get("x-goog-api-key") != "test-key" {
					t.Errorf("unexpected request: %s, key %q", request.URL.Path, request.Header.Get("x-goog-api-key"))
				}
				var body struct {
					Contents []struct {
						Parts []struct {
							InlineData struct {
								MimeType string `json:"mime_type"`
								Data     string `json:"data"`
							} `json:"inline_data"`
						} `json:"parts"`
					} `json:"contents"`
					GenerationConfig struct {
						ResponseMimeType string `json:"responseMimeType"`
					} `json:"generationConfig"`
				}
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 2 {
					t.Fatalf("missing content parts: %+v", body)
				}
				image := body.Contents[0].Parts[1].InlineData
				if image.MimeType != test.mimeType || image.Data != base64.StdEncoding.EncodeToString([]byte("drawing")) || body.GenerationConfig.ResponseMimeType != "application/json" {
					t.Errorf("incorrect Gemini payload: %+v", body)
				}
				_ = json.NewEncoder(writer).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]string{"text": `{"rooms":[{"id":"r1","name":"Kitchen","length":{"value":4,"unit":"m"},"width":null,"wallHeight":null,"confidence":0.5,"uncertainty":"Not labeled"}],"doors":[],"windows":[]}`}}}}}})
			}))
			defer server.Close()
			candidate, err := (GeminiAnalyzer{Endpoint: server.URL, Model: "test-model", APIKey: "test-key", Client: server.Client()}).Analyze(context.Background(), []byte("drawing"), test.name)
			if err != nil || candidate.State != StateReady || len(candidate.Rooms) != 1 || candidate.Rooms[0].Name != "Kitchen" || candidate.Rooms[0].Width != nil {
				t.Fatalf("unexpected candidate: %+v, err: %v", candidate, err)
			}
		})
	}
}

func TestGeminiAnalyzerRejectsMissingKeyAndEmptyResponse(t *testing.T) {
	if _, err := (GeminiAnalyzer{}).Analyze(context.Background(), []byte("drawing"), "plan.png"); err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected missing key error, got %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			if request.URL.Path != "/models/gemini-3.8-flash" {
				t.Errorf("unexpected default model path: %s", request.URL.Path)
			}
			writer.WriteHeader(http.StatusOK)
			return
		}
		_, _ = writer.Write([]byte(`{"candidates":[]}`))
	}))
	defer server.Close()
	if _, err := (GeminiAnalyzer{Endpoint: server.URL, APIKey: "test-key"}).Analyze(context.Background(), []byte("drawing"), "plan.png"); err == nil || !strings.Contains(err.Error(), "no analysis") {
		t.Fatalf("expected empty response error, got %v", err)
	}
}

func TestGeminiAnalyzerReportsModelErrorWithoutLeakingKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"error":{"message":"models/unknown not found; key test-key"}}`))
	}))
	defer server.Close()
	_, err := (GeminiAnalyzer{Endpoint: server.URL, Model: "unknown", APIKey: "test-key"}).Analyze(context.Background(), []byte("drawing"), "plan.png")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404: models/unknown not found") || strings.Contains(err.Error(), "test-key") {
		t.Fatalf("unexpected Gemini error: %v", err)
	}
}

func TestGeminiAnalyzerDoesNotRequireModelMetadataAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Error("model metadata was queried before generating content")
		}
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"rooms\":[]}"}]}}]}`))
	}))
	defer server.Close()
	_, err := (GeminiAnalyzer{Endpoint: server.URL, APIKey: "test-key"}).Analyze(context.Background(), []byte("drawing"), "plan.png")
	if err != nil {
		t.Fatalf("generation failed without model metadata access: %v", err)
	}
}

func TestGeminiAnalyzerReportsProviderErrorWithoutLeakingKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			writer.WriteHeader(http.StatusOK)
			return
		}
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":{"message":"Invalid drawing; key test-key"}}`))
	}))
	defer server.Close()
	_, err := (GeminiAnalyzer{Endpoint: server.URL, APIKey: "test-key"}).Analyze(context.Background(), []byte("drawing"), "plan.png")
	if err == nil || !strings.Contains(err.Error(), "HTTP 400: Invalid drawing") || strings.Contains(err.Error(), "test-key") {
		t.Fatalf("unexpected provider error: %v", err)
	}
}
