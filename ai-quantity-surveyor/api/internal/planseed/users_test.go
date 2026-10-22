package planseed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func seedEnvironment(endpoint string) map[string]string {
	return map[string]string{"SEED_PLAN_USERS": "true", "SUPABASE_URL": endpoint, "SUPABASE_SERVICE_ROLE_KEY": "admin-fixture-key", "SEED_PLAN_FREE_PASSWORD": "free-fixture-password-123", "SEED_PLAN_PRO_PASSWORD": "pro-fixture-password-123", "SEED_PLAN_STUDIO_PASSWORD": "studio-fixture-password-123"}
}

func TestSeedConfigValidatesAllSecretsBeforeNetwork(t *testing.T) {
	values := seedEnvironment("https://example.supabase.co")
	read := func(key string) string { return values[key] }
	config, err := FromEnvironment(read)
	if err != nil || !config.Enabled || len(config.accounts) != 3 {
		t.Fatalf("seed configuration: %v", err)
	}
	if strings.Contains(fmt.Sprint(config), values["SUPABASE_SERVICE_ROLE_KEY"]) {
		t.Fatal("configuration exposed credentials")
	}
	for _, key := range []string{"SUPABASE_SERVICE_ROLE_KEY", "SEED_PLAN_FREE_PASSWORD", "SEED_PLAN_PRO_PASSWORD", "SEED_PLAN_STUDIO_PASSWORD"} {
		original := values[key]
		values[key] = ""
		if _, err := FromEnvironment(read); err == nil {
			t.Fatalf("missing %s accepted", key)
		}
		values[key] = original
	}
	values["SEED_PLAN_PRO_PASSWORD"] = values["SEED_PLAN_FREE_PASSWORD"]
	if _, err := FromEnvironment(read); err == nil {
		t.Fatal("shared passwords accepted")
	}
	values["SEED_PLAN_USERS"] = "false"
	if config, err := FromEnvironment(read); err != nil || config.Enabled {
		t.Fatal("disabled seed required admin configuration")
	}
}

func TestSeedUsersAreIdempotentAndDoNotResetPasswords(t *testing.T) {
	users := []adminUser{}
	creates := 0
	confirmed := "2026-10-22T00:00:00Z"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer admin-fixture-key" || request.Header.Get("apikey") != "admin-fixture-key" {
			t.Error("missing admin authorization")
		}
		if request.Method == http.MethodGet {
			_ = json.NewEncoder(writer).Encode(map[string]any{"users": users})
			return
		}
		if request.Method != http.MethodPost || request.URL.Path != "/auth/v1/admin/users" {
			t.Errorf("unexpected user mutation: %s %s", request.Method, request.URL.Path)
		}
		var input struct {
			Email    string         `json:"email"`
			Password string         `json:"password"`
			Confirm  bool           `json:"email_confirm"`
			Metadata map[string]any `json:"app_metadata"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if len(input.Password) < 16 || !input.Confirm || input.Metadata["plan_preview_seed"] != marker {
			t.Error("unsafe preview creation")
		}
		creates++
		user := adminUser{ID: fmt.Sprintf("seed-user-%d", creates), Email: input.Email, AppMetadata: input.Metadata, EmailConfirmedAt: &confirmed}
		users = append(users, user)
		writer.WriteHeader(201)
		_ = json.NewEncoder(writer).Encode(user)
	}))
	defer server.Close()
	values := seedEnvironment(server.URL)
	config, err := FromEnvironment(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	first, err := EnsureUsers(context.Background(), config)
	if err != nil || len(first) != 3 || creates != 3 {
		t.Fatalf("first seed: creates=%d err=%v", creates, err)
	}
	second, err := EnsureUsers(context.Background(), config)
	if err != nil || len(second) != 3 || creates != 3 {
		t.Fatalf("rerun created/reset accounts: creates=%d err=%v", creates, err)
	}
	for index, user := range second {
		if user.ID != first[index].ID || user.Created {
			t.Fatal("identity changed on rerun")
		}
	}
	users[1].AppMetadata = map[string]any{}
	if _, err := EnsureUsers(context.Background(), config); err == nil || creates != 3 {
		t.Fatal("unmanaged identity adopted or accounts partially recreated")
	}
}

func TestSeedAdminErrorsDoNotExposeSecretsOrFollowRedirects(t *testing.T) {
	targetCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/target" {
			targetCalls++
			return
		}
		http.Redirect(writer, request, "/target", 307)
	}))
	defer server.Close()
	values := seedEnvironment(server.URL)
	config, _ := FromEnvironment(func(key string) string { return values[key] })
	_, err := EnsureUsers(context.Background(), config)
	if err == nil || targetCalls != 0 || strings.Contains(err.Error(), config.adminKey) {
		t.Fatalf("unsafe redirect/error handling: %v", err)
	}
}
