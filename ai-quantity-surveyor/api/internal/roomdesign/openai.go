package roomdesign

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openAIImagesEndpoint = "https://api.openai.com/v1/images/generations"

type ProviderError struct {
	Provider   string
	StatusCode int
	Type       string
	Code       string
}

func (failure *ProviderError) Error() string {
	return fmt.Sprintf("%s image provider HTTP %d type=%s code=%s", failure.Provider, failure.StatusCode, failure.Type, failure.Code)
}

type OpenAIGenerator struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

func NewOpenAIGenerator(apiKey, model string) *OpenAIGenerator {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-image-1"
	}
	return newOpenAIGenerator(apiKey, model, openAIImagesEndpoint, &http.Client{
		Timeout: 90 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	})
}

func newOpenAIGenerator(apiKey, model, endpoint string, client *http.Client) *OpenAIGenerator {
	return &OpenAIGenerator{apiKey: apiKey, model: model, endpoint: endpoint, client: client}
}

func (generator *OpenAIGenerator) Generate(ctx context.Context, input Prompt, count int) ([]GeneratedImage, error) {
	if generator == nil || generator.apiKey == "" || count != 2 {
		return nil, errors.New("room image generator is not configured")
	}
	requestBody := struct {
		Model        string `json:"model"`
		Prompt       string `json:"prompt"`
		N            int    `json:"n"`
		Size         string `json:"size"`
		Quality      string `json:"quality"`
		OutputFormat string `json:"output_format"`
	}{Model: generator.model, Prompt: formatImagePrompt(input), N: count, Size: "1024x1024", Quality: "medium", OutputFormat: "png"}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, generator.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+generator.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := generator.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var envelope struct {
			Error struct {
				Type string `json:"type"`
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 8<<10)).Decode(&envelope)
		return nil, &ProviderError{Provider: "OpenAI", StatusCode: response.StatusCode, Type: envelope.Error.Type, Code: envelope.Error.Code}
	}
	var result struct {
		Data []struct {
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&result); err != nil {
		return nil, errors.New("invalid image provider response")
	}
	if len(result.Data) != count {
		return nil, errors.New("image provider returned an incomplete result")
	}
	images := make([]GeneratedImage, 0, count)
	for _, item := range result.Data {
		content, err := base64.StdEncoding.DecodeString(item.Base64)
		if err != nil || len(content) == 0 || len(content) > 8<<20 {
			return nil, errors.New("image provider returned an invalid image")
		}
		images = append(images, GeneratedImage{PNG: content, Model: generator.model})
	}
	return images, nil
}

func formatImagePrompt(input Prompt) string {
	openings := "no confirmed openings"
	if len(input.Openings) > 0 {
		parts := make([]string, 0, len(input.Openings))
		for _, opening := range input.Openings {
			parts = append(parts, fmt.Sprintf("%s %s wide by %s high", opening.Kind, opening.Width, opening.Height))
		}
		openings = strings.Join(parts, ", ")
	}
	preferences := strings.TrimSpace(input.Preferences)
	if preferences == "" {
		preferences = "no additional requirements"
	}
	return fmt.Sprintf("Create one photorealistic interior design concept image for a single residential room. The room is named %q and its confirmed dimensions are %s long, %s wide, with %s wall height. Confirmed openings: %s. Design style: %s. Color palette: %s. User preferences: %s. Show a believable furnished interior at human eye level, keep openings visible, and do not add text, labels, dimensions, floor plans, or structural claims. This is an illustrative design concept, not a measured or construction-ready visualization.", input.RoomName, input.Length, input.Width, input.WallHeight, openings, input.Style, input.Palette, preferences)
}
