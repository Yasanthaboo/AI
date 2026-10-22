package roomdesign

import (
	"context"
	"testing"
)

func TestProviderSelectionUsesOnlyConfiguredGenerators(t *testing.T) {
	primary := &fallbackFixture{}
	other := &fallbackFixture{}
	generator := &ProviderGenerator{Default: "nvidia", Options: []Provider{{ID: "nvidia", Label: "NVIDIA", Model: "flux"}, {ID: "openai", Label: "OpenAI", Model: "gpt-image-1"}}, Generators: map[string]Generator{"nvidia": primary, "openai": other}}
	if _, err := generator.Generate(context.Background(), Prompt{}, 2); err != nil || primary.calls != 1 || other.calls != 0 {
		t.Fatalf("default routing failed: %v", err)
	}
	selected, err := generator.Select("openai")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selected.Generate(context.Background(), Prompt{}, 2); err != nil || other.calls != 1 {
		t.Fatal("explicit provider not selected")
	}
	if _, err := generator.Select("https://untrusted.example/images"); err == nil {
		t.Fatal("arbitrary provider endpoint accepted")
	}
	service := NewService(nil, nil, nil, generator, 10)
	if service.DefaultProvider() != "nvidia" || len(service.Providers()) != 2 {
		t.Fatal("capability metadata does not match configured providers")
	}
	if _, err := service.Generate(context.Background(), "owner", "project", Request{Provider: "unconfigured", RoomID: "room", ConfirmationRevision: 1, Style: "Contemporary", Palette: "Warm neutrals"}); err == nil {
		t.Fatal("unconfigured provider reached ownership/quota/provider processing")
	}
}
