package roomdesign

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LocalImageGenerator struct {
	endpoint string
	model    string
	client   *http.Client
}

func NewLocalImageGenerator(endpoint, model string) (*LocalImageGenerator, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("LOCAL_IMAGE_ENDPOINT must be an absolute local HTTP(S) image-generation URL")
	}
	host := strings.ToLower(parsed.Hostname())
	address := net.ParseIP(host)
	if host != "localhost" && host != "host.docker.internal" && (address == nil || (!address.IsLoopback() && !address.IsPrivate())) {
		return nil, errors.New("LOCAL_IMAGE_ENDPOINT must use localhost, host.docker.internal, or a private IP address")
	}
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 200 {
		return nil, errors.New("LOCAL_IMAGE_MODEL must identify the loaded local model (1-200 characters)")
	}
	return &LocalImageGenerator{endpoint: parsed.String(), model: "local/" + model, client: &http.Client{
		Timeout:       180 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (generator *LocalImageGenerator) Generate(ctx context.Context, input Prompt, count int) ([]GeneratedImage, error) {
	if generator == nil || generator.client == nil || count != 2 {
		return nil, errors.New("local image generator is not configured for two concepts")
	}
	prompt := formatImagePrompt(input)
	if strings.Contains(prompt, "<sd_cpp_extra_args>") || strings.Contains(prompt, "</sd_cpp_extra_args>") {
		return nil, errors.New("local image preferences cannot contain server control tags")
	}
	body, err := json.Marshal(map[string]any{"prompt": prompt, "n": count, "size": "1024x1024", "output_format": "png"})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, generator.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := generator.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &ProviderError{Provider: "Local image server", StatusCode: response.StatusCode}
	}
	var result struct {
		Data []struct {
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 24<<20)).Decode(&result); err != nil || len(result.Data) != count {
		return nil, errors.New("local image server returned an invalid or incomplete concept set")
	}
	images := make([]GeneratedImage, 0, count)
	for _, item := range result.Data {
		content, err := base64.StdEncoding.DecodeString(item.Base64)
		if err != nil || len(content) == 0 || len(content) > 8<<20 {
			return nil, errors.New("local image server returned an invalid or oversized image")
		}
		configuration, err := png.DecodeConfig(bytes.NewReader(content))
		if err != nil || configuration.Width < 1 || configuration.Height < 1 || configuration.Width > 4096 || configuration.Height > 4096 {
			return nil, errors.New("local image server returned an invalid or oversized PNG")
		}
		if _, err := png.Decode(bytes.NewReader(content)); err != nil {
			return nil, errors.New("local image server returned a corrupt PNG")
		}
		images = append(images, GeneratedImage{PNG: content, Model: generator.model})
	}
	return images, nil
}
