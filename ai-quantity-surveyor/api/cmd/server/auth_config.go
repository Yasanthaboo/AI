package main

import (
	"errors"
	"os"
	"strings"

	"github.com/example/ai-quantity-surveyor/api/internal/auth"
)

func authConfigurationFromEnvironment() (*auth.Verifier, string, error) {
	endpoint := strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("SUPABASE_ANON_KEY"))
	if (endpoint == "") != (apiKey == "") {
		return nil, "", errors.New("SUPABASE_URL and SUPABASE_ANON_KEY must be configured together")
	}
	if endpoint == "" {
		if os.Getenv("AUTH_ALLOW_ANONYMOUS") != "true" {
			return nil, "", errors.New("Supabase auth is required; AUTH_ALLOW_ANONYMOUS=true is only for isolated development and tests")
		}
		return nil, localOwnerID, nil
	}
	initialAdminID := strings.TrimSpace(os.Getenv("INITIAL_ADMIN_USER_ID"))
	if initialAdminID == "" {
		return nil, "", errors.New("INITIAL_ADMIN_USER_ID is required when Supabase auth is enabled")
	}
	verifier, err := auth.NewSupabaseVerifier(endpoint, apiKey)
	if err != nil {
		return nil, "", err
	}
	return verifier, initialAdminID, nil
}
