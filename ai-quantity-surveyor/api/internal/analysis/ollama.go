package analysis

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var measurementSchema = map[string]any{"anyOf": []any{
	map[string]any{"type": "null"},
	map[string]any{
		"type":       "object",
		"properties": map[string]any{"value": map[string]any{"type": "number"}, "unit": map[string]any{"type": "string", "enum": []string{"m", "cm", "ft", "in"}}},
		"required":   []string{"value", "unit"},
	},
}}

var openingSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{"type": "string"}, "roomId": map[string]any{"type": "string"},
		"width": measurementSchema, "height": measurementSchema,
		"confidence": map[string]any{"type": "number"}, "uncertainty": map[string]any{"type": "string"},
	},
	"required": []string{"id", "width", "height", "confidence"},
}

// candidateSchema is sent as Ollama's structured-output format so field names match Candidate.
var candidateSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"analysisState": map[string]any{"type": "string", "enum": []string{string(StateReady)}},
		"rooms": map[string]any{"type": "array", "items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"},
				"length": measurementSchema, "width": measurementSchema, "wallHeight": measurementSchema,
				"bounds": map[string]any{
					"type":       "object",
					"properties": map[string]any{"x": map[string]any{"type": "number"}, "y": map[string]any{"type": "number"}, "width": map[string]any{"type": "number"}, "height": map[string]any{"type": "number"}},
					"required":   []string{"x", "y", "width", "height"},
				},
				"confidence": map[string]any{"type": "number"}, "uncertainty": map[string]any{"type": "string"},
			},
			"required": []string{"id", "name", "length", "width", "wallHeight", "bounds", "confidence"},
		}},
		"doors":   map[string]any{"type": "array", "items": openingSchema},
		"windows": map[string]any{"type": "array", "items": openingSchema},
	},
	"required": []string{"analysisState", "rooms", "doors", "windows"},
}

type OllamaAnalyzer struct {
	Endpoint string
	Model    string
	APIKey   string
	Client   *http.Client
}

func (analyzer OllamaAnalyzer) Analyze(ctx context.Context, content []byte, fileName string) (Candidate, error) {
	extension := ""
	if index := strings.LastIndex(fileName, "."); index >= 0 {
		extension = fileName[index:]
	}
	if strings.EqualFold(extension, ".pdf") {
		return Candidate{}, fmt.Errorf("PDF vision analysis requires rasterization before Ollama")
	}
	if len(content) == 0 {
		return Candidate{}, fmt.Errorf("drawing content is empty")
	}
	endpoint := analyzer.Endpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	model := analyzer.Model
	if model == "" {
		model = "llama3.2-vision"
	}
	requestBody := map[string]any{
		"model":  model,
		"stream": false,
		"format": candidateSchema,
		"messages": []map[string]any{{
			"role":    "user",
			"content": extractionPrompt,
			"images":  []string{base64.StdEncoding.EncodeToString(content)},
		}},
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return Candidate{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Candidate{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if analyzer.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+analyzer.APIKey)
	}
	client := analyzer.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	response, err := client.Do(request)
	if err != nil {
		return Candidate{}, fmt.Errorf("vision provider request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errorBody, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		if readErr == nil && strings.TrimSpace(string(errorBody)) != "" {
			return Candidate{}, fmt.Errorf("vision provider returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(errorBody)))
		}
		return Candidate{}, fmt.Errorf("vision provider returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return Candidate{}, fmt.Errorf("invalid vision provider response: %w", err)
	}
	var candidate Candidate
	responseContent := strings.TrimSpace(envelope.Message.Content)
	responseContent = strings.TrimPrefix(responseContent, "```json")
	responseContent = strings.TrimPrefix(responseContent, "```")
	responseContent = strings.TrimSuffix(responseContent, "```")
	responseContent = strings.TrimSpace(responseContent)
	normalizedContent, err := normalizeConfidence(responseContent)
	if err != nil {
		return Candidate{}, fmt.Errorf("vision response content is not analysis JSON: %w", err)
	}
	if err := json.Unmarshal(normalizedContent, &candidate); err != nil {
		return Candidate{}, fmt.Errorf("vision response content is not analysis JSON: %w", err)
	}
	if candidate.State == "" {
		candidate.State = StateReady
	}
	return candidate, nil
}

func normalizeConfidence(content string) ([]byte, error) {
	var value any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return nil, err
	}
	if rooms, ok := value.([]any); ok {
		value = map[string]any{"analysisState": StateReady, "rooms": rooms}
	}
	var normalize func(any) any
	normalize = func(current any) any {
		switch typed := current.(type) {
		case []any:
			for index := range typed {
				typed[index] = normalize(typed[index])
			}
		case map[string]any:
			for key, nested := range typed {
				typed[key] = normalize(nested)
			}
			if rooms, ok := typed["rooms"].([]any); ok {
				for index, room := range rooms {
					if roomData, ok := room.(map[string]any); ok {
						if strings.TrimSpace(stringValue(roomData["id"])) == "" {
							roomData["id"] = fmt.Sprintf("room-%d", index+1)
						}
						if strings.TrimSpace(stringValue(roomData["name"])) == "" {
							roomData["name"] = fmt.Sprintf("Room %d", index+1)
						}
					}
				}
			}
			if confidence, ok := typed["confidence"].(map[string]any); ok {
				for _, key := range []string{"score", "value", "confidence"} {
					if score, exists := confidence[key].(float64); exists {
						typed["confidence"] = score
						break
					}
				}
			}
		}
		return current
	}
	return json.Marshal(normalize(value))
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
