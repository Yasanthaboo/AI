package main

import (
	"testing"

	"github.com/example/ai-quantity-surveyor/api/internal/roomdesign"
)

func TestRoomDesignConfigurationRequiresKeyAndValidQuota(t *testing.T) {
	t.Setenv("ROOM_DESIGN_FALLBACK", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("NVIDIA_API_KEY", "")
	t.Setenv("ROOM_DESIGN_PROVIDER", "")
	t.Setenv("ROOM_DESIGN_DAILY_IMAGE_LIMIT", "")
	if generator, limit, err := roomDesignConfigurationFromEnvironment(); err != nil || generator != nil || limit != 0 {
		t.Fatalf("unconfigured room design = %v, %d, %v", generator, limit, err)
	}

	t.Setenv("ROOM_DESIGN_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "configured-test-key")
	if generator, limit, err := roomDesignConfigurationFromEnvironment(); err != nil || generator == nil || limit != 4 {
		t.Fatalf("default room design config = %v, %d, %v", generator, limit, err)
	}
	t.Setenv("ROOM_DESIGN_PROVIDER", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("NVIDIA_API_KEY", "analysis-only-key")
	t.Setenv("NVIDIA_NIM_IMAGE_ENDPOINT", "")
	if generator, limit, err := roomDesignConfigurationFromEnvironment(); err != nil || generator != nil || limit != 0 {
		t.Fatalf("analysis-only NVIDIA config enabled image generation: %v, %d, %v", generator, limit, err)
	}
	t.Setenv("ROOM_DESIGN_PROVIDER", "nvidia")
	t.Setenv("NVIDIA_API_KEY", "configured-test-key")
	t.Setenv("NVIDIA_NIM_IMAGE_ENDPOINT", "https://images.example.test/flux")
	if generator, limit, err := roomDesignConfigurationFromEnvironment(); err != nil || limit != 4 {
		t.Fatalf("NVIDIA room design config = %v, %d, %v", generator, limit, err)
	} else if _, ok := generator.(*roomdesign.NvidiaFluxGenerator); !ok {
		t.Fatalf("NVIDIA provider got %T", generator)
	}
	t.Setenv("NVIDIA_API_KEY", "")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("explicit NVIDIA provider accepted missing API key")
	}
	t.Setenv("NVIDIA_API_KEY", "configured-test-key")
	t.Setenv("NVIDIA_NIM_IMAGE_ENDPOINT", "")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("explicit NVIDIA image provider accepted missing endpoint")
	}
	t.Setenv("ROOM_DESIGN_PROVIDER", "unknown")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("unknown room design provider accepted")
	}
	t.Setenv("ROOM_DESIGN_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "configured-test-key")
	t.Setenv("ROOM_DESIGN_DAILY_IMAGE_LIMIT", "101")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("oversized daily image limit accepted")
	}
}

func TestRoomDesignLocalFallbackConfiguration(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("NVIDIA_API_KEY", "")
	t.Setenv("ROOM_DESIGN_PROVIDER", "local")
	t.Setenv("ROOM_DESIGN_FALLBACK", "")
	t.Setenv("ROOM_DESIGN_DAILY_IMAGE_LIMIT", "6")
	t.Setenv("LOCAL_IMAGE_ENDPOINT", "http://host.docker.internal:1234/v1/images/generations")
	t.Setenv("LOCAL_IMAGE_MODEL", "sdxl-base-1.0")
	generator, limit, err := roomDesignConfigurationFromEnvironment()
	if err != nil || limit != 6 {
		t.Fatalf("local configuration: %d %v", limit, err)
	}
	if _, ok := generator.(*roomdesign.LocalImageGenerator); !ok {
		t.Fatalf("local provider got %T", generator)
	}
	t.Setenv("ROOM_DESIGN_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "configured-test-key")
	t.Setenv("ROOM_DESIGN_FALLBACK", "local")
	generator, limit, err = roomDesignConfigurationFromEnvironment()
	if err != nil || limit != 6 {
		t.Fatalf("fallback configuration: %d %v", limit, err)
	}
	if _, ok := generator.(*roomdesign.FallbackGenerator); !ok {
		t.Fatalf("fallback provider got %T", generator)
	}
	t.Setenv("LOCAL_IMAGE_MODEL", "")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("fallback without a model label accepted")
	}
	t.Setenv("LOCAL_IMAGE_MODEL", "sdxl-base-1.0")
	t.Setenv("LOCAL_IMAGE_ENDPOINT", "https://public.example/images")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("nonlocal fallback endpoint accepted")
	}
	t.Setenv("LOCAL_IMAGE_ENDPOINT", "http://host.docker.internal:1234/v1/images/generations")
	t.Setenv("ROOM_DESIGN_PROVIDER", "local")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("local-to-local fallback accepted")
	}
	t.Setenv("ROOM_DESIGN_PROVIDER", "")
	t.Setenv("OPENAI_API_KEY", "")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("fallback without a primary accepted")
	}
	t.Setenv("ROOM_DESIGN_FALLBACK", "unknown")
	if _, _, err := roomDesignConfigurationFromEnvironment(); err == nil {
		t.Fatal("unknown fallback accepted")
	}
}
