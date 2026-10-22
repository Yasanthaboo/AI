package main

import "github.com/example/ai-quantity-surveyor/api/internal/analysis"

type analysisProvider struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Model   string `json:"model"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
}

func analysisProviders(engines map[string]analysis.Analyzer, limits map[string]int, durable bool, scope string) []analysisProvider {
	result := make([]analysisProvider, 0, 4)
	for _, entry := range []struct{ id, label string }{{"demo", "Demo"}, {"ollama", "Ollama"}, {"gemini", "Google Gemini"}, {"nvidia", "NVIDIA Nemotron"}} {
		provider := analysisProvider{ID: entry.id, Label: entry.label}
		if namer, ok := engines[entry.id].(analysis.ModelNamer); ok {
			provider.Model = namer.ModelName()
		}
		switch {
		case entry.id == "demo":
			provider.Enabled = true
		case engines[entry.id] == nil:
			provider.Reason = "Provider is not configured on the API."
		case !durable:
			provider.Reason = "PostgreSQL is required for provider quotas."
		case scope == "":
			provider.Reason = "PROVIDER_QUOTA_SCOPE is required."
		case limits[entry.id] <= 0:
			provider.Reason = "Daily call limit is disabled."
		default:
			provider.Enabled = true
		}
		result = append(result, provider)
	}
	return result
}
