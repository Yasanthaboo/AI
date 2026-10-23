package roomdesign

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultNvidiaFluxModel = "black-forest-labs/flux.1-dev"

type NvidiaFluxGenerator struct {
	APIKey   string
	Model    string
	Endpoint string
	Client   *http.Client
}

func NewNvidiaFluxGenerator(apiKey, model, endpoint string) (*NvidiaFluxGenerator, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("NVIDIA API key is required")
	}
	if strings.TrimSpace(model) == "" {
		model = defaultNvidiaFluxModel
	}
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("NVIDIA_NIM_IMAGE_ENDPOINT is required for NVIDIA room design")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("NVIDIA NIM image endpoint must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" {
		return nil, errors.New("NVIDIA NIM image endpoint must use HTTPS outside localhost")
	}
	return &NvidiaFluxGenerator{APIKey: apiKey, Model: model, Endpoint: parsed.String(), Client: &http.Client{
		Timeout:       90 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (generator *NvidiaFluxGenerator) Generate(ctx context.Context, input Prompt, count int) ([]GeneratedImage, error) {
	if generator == nil || generator.APIKey == "" || generator.Endpoint == "" || count != 2 {
		return nil, errors.New("NVIDIA image generator is not configured")
	}
	prompt := formatImagePrompt(input)
	images := make([]GeneratedImage, 0, count)
	for index := 0; index < count; index++ {
		body, err := json.Marshal(map[string]any{"prompt": prompt, "mode": "base", "width": 1024, "height": 1024, "steps": 28, "cfg_scale": 3.5, "seed": 0})
		if err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, generator.Endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+generator.APIKey)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")
		client := generator.Client
		if client == nil {
			client = &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		image, decodeErr := readNvidiaFluxImage(response)
		if decodeErr != nil {
			return nil, decodeErr
		}
		images = append(images, GeneratedImage{PNG: image, Model: generator.Model})
	}
	return images, nil
}

func readNvidiaFluxImage(response *http.Response) ([]byte, error) {
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var envelope struct {
			Error struct {
				Type string `json:"type"`
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 8<<10)).Decode(&envelope)
		return nil, &ProviderError{Provider: "NVIDIA NIM", StatusCode: response.StatusCode, Type: envelope.Error.Type, Code: envelope.Error.Code}
	}
	var result struct {
		Artifacts []struct {
			Base64       string `json:"base64"`
			FinishReason string `json:"finishReason"`
		} `json:"artifacts"`
		Image       string `json:"image"`
		ImageBase64 string `json:"image_base64"`
		Data        []struct {
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&result); err != nil {
		return nil, errors.New("invalid NVIDIA NIM image response")
	}
	encoded := result.Image
	if len(result.Artifacts) > 0 {
		if len(result.Artifacts) != 1 {
			return nil, errors.New("NVIDIA NIM returned an unexpected image count")
		}
		if result.Artifacts[0].FinishReason == "CONTENT_FILTERED" {
			return nil, &ProviderError{Provider: "NVIDIA NIM", StatusCode: http.StatusBadRequest, Code: "moderation_blocked"}
		}
		if result.Artifacts[0].FinishReason != "SUCCESS" {
			return nil, errors.New("NVIDIA NIM image generation did not succeed")
		}
		encoded = result.Artifacts[0].Base64
	}
	if encoded == "" {
		encoded = result.ImageBase64
	}
	if encoded == "" && len(result.Data) > 0 {
		encoded = result.Data[0].Base64
	}
	if strings.HasPrefix(encoded, "data:image/") {
		_, encoded, _ = strings.Cut(encoded, ",")
	}
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(content) == 0 || len(content) > 8<<20 {
		return nil, errors.New("NVIDIA NIM returned an invalid or oversized image")
	}
	configuration, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || (format != "jpeg" && format != "png") || configuration.Width < 1 || configuration.Height < 1 || configuration.Width > 4096 || configuration.Height > 4096 {
		return nil, errors.New("NVIDIA NIM returned an invalid or oversized image")
	}
	decoded, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, errors.New("NVIDIA NIM returned an invalid image")
	}
	if format == "png" {
		return content, nil
	}
	var converted bytes.Buffer
	if err := png.Encode(&converted, decoded); err != nil || converted.Len() > 8<<20 {
		return nil, errors.New("NVIDIA NIM image could not be converted to PNG within the size limit")
	}
	return converted.Bytes(), nil
}
