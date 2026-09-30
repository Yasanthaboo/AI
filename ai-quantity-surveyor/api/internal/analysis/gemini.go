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

type GeminiAnalyzer struct {
	Endpoint string
	Model    string
	APIKey   string
	Client   *http.Client
}

func (analyzer GeminiAnalyzer) Analyze(ctx context.Context, content []byte, fileName string) (Candidate, error) {
	if len(content) == 0 {
		return Candidate{}, fmt.Errorf("drawing content is empty")
	}
	if analyzer.APIKey == "" {
		return Candidate{}, fmt.Errorf("Gemini API key is not configured")
	}
	mimeType := ""
	switch {
	case strings.HasSuffix(strings.ToLower(fileName), ".pdf"):
		mimeType = "application/pdf"
	case strings.HasSuffix(strings.ToLower(fileName), ".png"):
		mimeType = "image/png"
	case strings.HasSuffix(strings.ToLower(fileName), ".jpg"), strings.HasSuffix(strings.ToLower(fileName), ".jpeg"):
		mimeType = "image/jpeg"
	default:
		return Candidate{}, fmt.Errorf("unsupported drawing format")
	}
	model := analyzer.Model
	if model == "" {
		model = "gemini-3.8-flash"
	}
	endpoint := analyzer.Endpoint
	if endpoint == "" {
		endpoint = "https://generativelanguage.googleapis.com/v1beta"
	}
	client := analyzer.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	requestBody := map[string]any{
		"contents": []any{map[string]any{"parts": []any{
			map[string]any{"text": extractionPrompt},
			map[string]any{"inline_data": map[string]string{"mime_type": mimeType, "data": base64.StdEncoding.EncodeToString(content)}},
		}}},
		"generationConfig": map[string]string{"responseMimeType": "application/json"},
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return Candidate{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/models/"+model+":generateContent", bytes.NewReader(body))
	if err != nil {
		return Candidate{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-goog-api-key", analyzer.APIKey)
	response, err := client.Do(request)
	if err != nil {
		return Candidate{}, fmt.Errorf("Gemini request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var providerError struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&providerError) == nil {
			message := strings.ReplaceAll(strings.TrimSpace(providerError.Error.Message), analyzer.APIKey, "[redacted]")
			if message != "" {
				return Candidate{}, fmt.Errorf("Gemini returned HTTP %d: %s", response.StatusCode, message)
			}
		}
		return Candidate{}, fmt.Errorf("Gemini returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&envelope); err != nil {
		return Candidate{}, fmt.Errorf("invalid Gemini response: %w", err)
	}
	if len(envelope.Candidates) == 0 || len(envelope.Candidates[0].Content.Parts) == 0 {
		return Candidate{}, fmt.Errorf("Gemini response contains no analysis")
	}
	var text strings.Builder
	for _, part := range envelope.Candidates[0].Content.Parts {
		text.WriteString(part.Text)
	}
	normalized, err := normalizeConfidence(strings.TrimSpace(text.String()))
	if err != nil {
		return Candidate{}, fmt.Errorf("Gemini response is not analysis JSON: %w", err)
	}
	var candidate Candidate
	if err := json.Unmarshal(normalized, &candidate); err != nil {
		return Candidate{}, fmt.Errorf("Gemini response is not analysis JSON: %w", err)
	}
	if candidate.State == "" {
		candidate.State = StateReady
	}
	return candidate, nil
}
