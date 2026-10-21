package roomdesign

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
)

type FallbackGenerator struct {
	Primary  Generator
	Fallback Generator
}

func (generator *FallbackGenerator) Generate(ctx context.Context, input Prompt, count int) ([]GeneratedImage, error) {
	images, err := generator.Primary.Generate(ctx, input, count)
	if err == nil || generator.Fallback == nil || !canFallback(ctx, err) {
		return images, err
	}
	log.Printf("room design primary unavailable; attempting configured local image fallback")
	images, err = generator.Fallback.Generate(ctx, input, count)
	if err != nil {
		return nil, fmt.Errorf("local image fallback failed: %w", err)
	}
	return images, nil
}

func canFallback(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	var certificateFailure *tls.CertificateVerificationError
	if errors.As(err, &certificateFailure) {
		return false
	}
	var providerFailure *ProviderError
	if errors.As(err, &providerFailure) {
		code := strings.ToLower(providerFailure.Code + " " + providerFailure.Type)
		if strings.Contains(code, "moderation") || strings.Contains(code, "content_filter") || strings.Contains(code, "safety") {
			return false
		}
		return providerFailure.StatusCode == 429 || (providerFailure.StatusCode >= 500 && providerFailure.StatusCode <= 599)
	}
	var networkFailure net.Error
	return errors.As(err, &networkFailure)
}
