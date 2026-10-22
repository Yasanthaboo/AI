package roomdesign

import (
	"context"
	"errors"
)

type Provider struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Model string `json:"model"`
}

type ProviderGenerator struct {
	Default    string
	Options    []Provider
	Generators map[string]Generator
}

func DescribeGenerator(generator Generator) Provider {
	switch configured := generator.(type) {
	case *OpenAIGenerator:
		return Provider{ID: "openai", Label: "OpenAI Images", Model: configured.model}
	case *NvidiaFluxGenerator:
		return Provider{ID: "nvidia", Label: "NVIDIA FLUX", Model: configured.Model}
	case *LocalImageGenerator:
		return Provider{ID: "local", Label: "Local image model", Model: configured.model}
	case *FallbackGenerator:
		return DescribeGenerator(configured.Primary)
	case *ProviderGenerator:
		for _, option := range configured.Options {
			if option.ID == configured.Default {
				return option
			}
		}
	}
	return Provider{ID: "default", Label: "Configured image provider"}
}

func (generator *ProviderGenerator) Select(provider string) (Generator, error) {
	if provider == "" {
		provider = generator.Default
	}
	selected, found := generator.Generators[provider]
	if !found || selected == nil {
		return nil, errors.New("the selected image provider is unavailable")
	}
	return selected, nil
}

func (generator *ProviderGenerator) Generate(ctx context.Context, input Prompt, count int) ([]GeneratedImage, error) {
	selected, err := generator.Select("")
	if err != nil {
		return nil, err
	}
	return selected.Generate(ctx, input, count)
}

func (service *Service) Providers() []Provider {
	if service.generator == nil {
		return []Provider{}
	}
	if generator, ok := service.generator.(*ProviderGenerator); ok {
		return append([]Provider{}, generator.Options...)
	}
	return []Provider{DescribeGenerator(service.generator)}
}

func (service *Service) DefaultProvider() string {
	if service.generator == nil {
		return ""
	}
	return DescribeGenerator(service.generator).ID
}
