package main

import "testing"

func TestAuthConfigurationRequiresExplicitSecureDeploymentSettings(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Run("production refuses anonymous opt in", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("SUPABASE_URL", "")
		t.Setenv("SUPABASE_ANON_KEY", "")
		t.Setenv("AUTH_ALLOW_ANONYMOUS", "true")
		if _, _, err := authConfigurationFromEnvironment(); err == nil {
			t.Fatal("production anonymous access accepted")
		}
	})
	t.Run("anonymous development requires explicit opt in", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")
		t.Setenv("SUPABASE_URL", "")
		t.Setenv("SUPABASE_ANON_KEY", "")
		t.Setenv("INITIAL_ADMIN_USER_ID", "")
		t.Setenv("AUTH_ALLOW_ANONYMOUS", "")
		if _, _, err := authConfigurationFromEnvironment(); err == nil {
			t.Fatal("anonymous API started without explicit opt in")
		}
		t.Setenv("AUTH_ALLOW_ANONYMOUS", "true")
		verifier, ownerID, err := authConfigurationFromEnvironment()
		if err != nil || verifier != nil || ownerID != localOwnerID {
			t.Fatalf("local auth configuration = %v, %q, %v", verifier, ownerID, err)
		}
	})

	t.Run("database requires identity provider", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://example")
		t.Setenv("SUPABASE_URL", "")
		t.Setenv("SUPABASE_ANON_KEY", "")
		t.Setenv("AUTH_ALLOW_ANONYMOUS", "")
		if _, _, err := authConfigurationFromEnvironment(); err == nil {
			t.Fatal("database deployment started without authentication")
		}
	})

	t.Run("isolated anonymous test mode", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://example")
		t.Setenv("SUPABASE_URL", "")
		t.Setenv("SUPABASE_ANON_KEY", "")
		t.Setenv("AUTH_ALLOW_ANONYMOUS", "true")
		verifier, ownerID, err := authConfigurationFromEnvironment()
		if err != nil || verifier != nil || ownerID != localOwnerID {
			t.Fatalf("test auth configuration = %v, %q, %v", verifier, ownerID, err)
		}
	})

	t.Run("configured Supabase identity", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")
		t.Setenv("SUPABASE_URL", "https://example.supabase.co")
		t.Setenv("SUPABASE_ANON_KEY", "public-key")
		t.Setenv("INITIAL_ADMIN_USER_ID", "initial-admin")
		verifier, ownerID, err := authConfigurationFromEnvironment()
		if err != nil || verifier == nil || ownerID != "initial-admin" {
			t.Fatalf("Supabase auth configuration = %v, %q, %v", verifier, ownerID, err)
		}
	})
}
