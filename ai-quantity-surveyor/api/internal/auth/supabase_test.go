package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupabaseVerifierValidatesTokenAndReturnsUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/auth/v1/user" || request.Header.Get("apikey") != "public-key" || request.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("unexpected verification request: %s %s", request.Method, request.URL)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"user-123","email":"person@example.test"}`))
	}))
	defer server.Close()

	verifier, err := NewSupabaseVerifier(server.URL, "public-key")
	if err != nil {
		t.Fatal(err)
	}
	user, err := verifier.Verify(context.Background(), "access-token")
	if err != nil || user.ID != "user-123" || user.Email != "person@example.test" {
		t.Fatalf("verified user = %+v, %v", user, err)
	}
}

func TestSupabaseVerifierRejectsInvalidAndUnavailableIdentities(t *testing.T) {
	t.Run("invalid token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Error(writer, "invalid", http.StatusUnauthorized)
		}))
		defer server.Close()
		verifier, err := NewSupabaseVerifier(server.URL, "public-key")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := verifier.Verify(context.Background(), "bad-token"); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("expected unauthenticated, got %v", err)
		}
	})

	t.Run("missing user id", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(`{"email":"person@example.test"}`))
		}))
		defer server.Close()
		verifier, err := NewSupabaseVerifier(server.URL, "public-key")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := verifier.Verify(context.Background(), "access-token"); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected unavailable, got %v", err)
		}
	})

	t.Run("identity service failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		verifier, err := NewSupabaseVerifier(server.URL, "public-key")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := verifier.Verify(context.Background(), "access-token"); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected unavailable, got %v", err)
		}
	})
}

func TestSupabaseVerifierRequiresSecureURLAndAPIKey(t *testing.T) {
	for _, input := range []struct{ endpoint, key string }{
		{"http://identity.example.test", "public-key"},
		{"https://identity.example.test", ""},
		{"https://user:password@identity.example.test", "public-key"},
	} {
		if _, err := NewSupabaseVerifier(input.endpoint, input.key); err == nil {
			t.Fatalf("accepted invalid Supabase config: %+v", input)
		}
	}
}

func TestAuthMiddlewareAddsPrincipalOnlyAfterVerification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"id":"user-123"}`))
	}))
	defer server.Close()
	verifier, err := NewSupabaseVerifier(server.URL, "public-key")
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user, ok := UserFromContext(request.Context())
		if !ok || user.ID != "user-123" {
			t.Fatal("verified principal missing from request context")
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	handler := verifier.Middleware(next)

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/projects", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", missing.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	verified := httptest.NewRecorder()
	handler.ServeHTTP(verified, request)
	if verified.Code != http.StatusNoContent {
		t.Fatalf("verified request status = %d", verified.Code)
	}
}
