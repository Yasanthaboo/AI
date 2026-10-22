package main

import (
	"testing"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
)

func TestAnalysisProviderStatusExplainsDisabledOllama(t *testing.T) {
	engines := map[string]analysis.Analyzer{"demo": analysis.DemoAnalyzer{}, "ollama": analysis.OllamaAnalyzer{Endpoint: "https://ollama.example", Model: "vision-fixture"}}
	providers := analysisProviders(engines, map[string]int{}, true, "quota-scope")
	if providers[1].ID != "ollama" || providers[1].Enabled || providers[1].Model != "vision-fixture" || providers[1].Reason != "Daily call limit is disabled." {
		t.Fatalf("Ollama status: %+v", providers[1])
	}
	providers = analysisProviders(engines, map[string]int{"ollama": 5}, true, "quota-scope")
	if !providers[1].Enabled {
		t.Fatal("configured Ollama remained hidden")
	}
	providers = analysisProviders(engines, map[string]int{"ollama": 5}, false, "quota-scope")
	if providers[1].Enabled {
		t.Fatal("availability bypassed persistent quotas")
	}
}
