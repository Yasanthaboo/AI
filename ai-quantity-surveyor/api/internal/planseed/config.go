package planseed

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

const marker = "quantity-surveyor-plan-preview-v1"

type accountConfig struct {
	planID   string
	email    string
	password string
}

type Config struct {
	Enabled     bool
	supabaseURL string
	adminKey    string
	accounts    []accountConfig
}

func (config Config) String() string { return "plan preview seed configuration (credentials redacted)" }

func FromEnvironment(getenv func(string) string) (Config, error) {
	flag := strings.TrimSpace(getenv("SEED_PLAN_USERS"))
	if flag == "" || flag == "false" {
		return Config{}, nil
	}
	if flag != "true" {
		return Config{}, errors.New("SEED_PLAN_USERS must be true or false")
	}
	config := Config{Enabled: true, supabaseURL: strings.TrimRight(strings.TrimSpace(getenv("SUPABASE_URL")), "/"), adminKey: strings.TrimSpace(getenv("SUPABASE_SERVICE_ROLE_KEY"))}
	endpoint, err := url.Parse(config.supabaseURL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return Config{}, errors.New("SUPABASE_URL must be an absolute HTTP(S) URL without credentials, query or fragment")
	}
	if endpoint.Scheme != "https" && endpoint.Hostname() != "localhost" && endpoint.Hostname() != "127.0.0.1" {
		return Config{}, errors.New("Supabase admin seeding requires HTTPS outside localhost")
	}
	if config.adminKey == "" || config.adminKey == strings.TrimSpace(getenv("SUPABASE_ANON_KEY")) {
		return Config{}, errors.New("SUPABASE_SERVICE_ROLE_KEY must be a server-only admin key, not the public anon key")
	}
	emails := map[string]bool{}
	passwords := map[string]bool{}
	for _, planID := range []string{"free", "pro", "studio"} {
		prefix := "SEED_PLAN_" + strings.ToUpper(planID)
		email := strings.ToLower(strings.TrimSpace(getenv(prefix + "_EMAIL")))
		if email == "" {
			email = "demo-" + planID + "@quantity-surveyor.test"
		}
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || emails[email] {
			return Config{}, fmt.Errorf("%s_EMAIL must be a distinct plain email address", prefix)
		}
		password := getenv(prefix + "_PASSWORD")
		if len(password) < 16 || len(password) > 256 || strings.ContainsAny(password, "\r\n\x00") || passwords[password] {
			return Config{}, fmt.Errorf("%s_PASSWORD must be a unique 16-256 character secret without control lines", prefix)
		}
		emails[email] = true
		passwords[password] = true
		config.accounts = append(config.accounts, accountConfig{planID: planID, email: email, password: password})
	}
	return config, nil
}
