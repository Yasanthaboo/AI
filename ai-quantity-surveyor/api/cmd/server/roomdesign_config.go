package main

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/example/ai-quantity-surveyor/api/internal/roomdesign"
)

func selectableRoomDesignConfigurationFromEnvironment() (roomdesign.Generator, int, error) {
	primary, limit, err := roomDesignConfigurationFromEnvironment()
	if err != nil || primary == nil {
		return primary, limit, err
	}
	defaultOption := roomdesign.DescribeGenerator(primary)
	router := &roomdesign.ProviderGenerator{Default: defaultOption.ID, Options: []roomdesign.Provider{defaultOption}, Generators: map[string]roomdesign.Generator{defaultOption.ID: primary}}
	add := func(generator roomdesign.Generator) {
		option := roomdesign.DescribeGenerator(generator)
		if _, exists := router.Generators[option.ID]; exists {
			return
		}
		if fallback, ok := primary.(*roomdesign.FallbackGenerator); ok && option.ID != "local" {
			generator = &roomdesign.FallbackGenerator{Primary: generator, Fallback: fallback.Fallback}
		}
		router.Options = append(router.Options, option)
		router.Generators[option.ID] = generator
	}
	if key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); key != "" {
		add(roomdesign.NewOpenAIGenerator(key, os.Getenv("OPENAI_IMAGE_MODEL")))
	}
	if key, endpoint := strings.TrimSpace(os.Getenv("NVIDIA_API_KEY")), strings.TrimSpace(os.Getenv("NVIDIA_NIM_IMAGE_ENDPOINT")); key != "" && endpoint != "" {
		if generator, err := roomdesign.NewNvidiaFluxGenerator(key, os.Getenv("NVIDIA_FLUX_MODEL"), endpoint); err == nil {
			add(generator)
		}
	}
	if endpoint, model := strings.TrimSpace(os.Getenv("LOCAL_IMAGE_ENDPOINT")), strings.TrimSpace(os.Getenv("LOCAL_IMAGE_MODEL")); endpoint != "" && model != "" {
		if generator, err := roomdesign.NewLocalImageGenerator(endpoint, model); err == nil {
			add(generator)
		}
	}
	return router, limit, nil
}

func roomDesignConfigurationFromEnvironment() (roomdesign.Generator, int, error) {
	fallback := strings.ToLower(strings.TrimSpace(os.Getenv("ROOM_DESIGN_FALLBACK")))
	if fallback != "" && fallback != "local" {
		return nil, 0, errors.New("ROOM_DESIGN_FALLBACK must be local or empty")
	}
	generator, limit, err := primaryRoomDesignConfigurationFromEnvironment()
	if err != nil || fallback == "" {
		return generator, limit, err
	}
	if generator == nil || strings.EqualFold(strings.TrimSpace(os.Getenv("ROOM_DESIGN_PROVIDER")), "local") {
		return nil, 0, errors.New("local fallback requires a configured OpenAI or NVIDIA primary provider")
	}
	local, err := roomdesign.NewLocalImageGenerator(os.Getenv("LOCAL_IMAGE_ENDPOINT"), os.Getenv("LOCAL_IMAGE_MODEL"))
	if err != nil {
		return nil, 0, err
	}
	return &roomdesign.FallbackGenerator{Primary: generator, Fallback: local}, limit, nil
}

func primaryRoomDesignConfigurationFromEnvironment() (roomdesign.Generator, int, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("ROOM_DESIGN_PROVIDER")))
	openAIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	nvidiaKey := strings.TrimSpace(os.Getenv("NVIDIA_API_KEY"))
	if provider == "" {
		switch {
		case openAIKey != "":
			provider = "openai"
		case nvidiaKey != "" && strings.TrimSpace(os.Getenv("NVIDIA_NIM_IMAGE_ENDPOINT")) != "":
			provider = "nvidia"
		default:
			return nil, 0, nil
		}
	}
	if provider != "nvidia" && provider != "openai" && provider != "local" {
		return nil, 0, errors.New("ROOM_DESIGN_PROVIDER must be nvidia, openai, or local")
	}
	limit := 4
	if raw := strings.TrimSpace(os.Getenv("ROOM_DESIGN_DAILY_IMAGE_LIMIT")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > 100 {
			return nil, 0, errors.New("ROOM_DESIGN_DAILY_IMAGE_LIMIT must be between 0 and 100")
		}
		limit = parsed
	}
	if provider == "local" {
		generator, err := roomdesign.NewLocalImageGenerator(os.Getenv("LOCAL_IMAGE_ENDPOINT"), os.Getenv("LOCAL_IMAGE_MODEL"))
		return generator, limit, err
	}
	if provider == "nvidia" {
		if nvidiaKey == "" {
			return nil, 0, errors.New("NVIDIA_API_KEY is required when ROOM_DESIGN_PROVIDER=nvidia")
		}
		endpoint := strings.TrimSpace(os.Getenv("NVIDIA_NIM_IMAGE_ENDPOINT"))
		if endpoint == "" {
			return nil, 0, errors.New("NVIDIA_NIM_IMAGE_ENDPOINT is required when ROOM_DESIGN_PROVIDER=nvidia")
		}
		generator, err := roomdesign.NewNvidiaFluxGenerator(nvidiaKey, os.Getenv("NVIDIA_FLUX_MODEL"), endpoint)
		return generator, limit, err
	}
	if openAIKey == "" {
		return nil, 0, errors.New("OPENAI_API_KEY is required when ROOM_DESIGN_PROVIDER=openai")
	}
	return roomdesign.NewOpenAIGenerator(openAIKey, os.Getenv("OPENAI_IMAGE_MODEL")), limit, nil
}
