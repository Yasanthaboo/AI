package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrUnauthenticated = errors.New("unauthenticated")
var ErrUnavailable = errors.New("identity service unavailable")

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Verifier struct {
	userEndpoint string
	apiKey       string
	client       *http.Client
}

func NewSupabaseVerifier(endpoint, apiKey string) (*Verifier, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Supabase URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" {
		return nil, errors.New("Supabase URL must use HTTPS outside localhost")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("Supabase API key is required")
	}
	return &Verifier{
		userEndpoint: strings.TrimRight(parsed.String(), "/") + "/auth/v1/user",
		apiKey:       apiKey,
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (verifier *Verifier) Verify(ctx context.Context, accessToken string) (User, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" || strings.ContainsAny(accessToken, " \t\r\n") {
		return User{}, ErrUnauthenticated
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, verifier.userEndpoint, nil)
	if err != nil {
		return User{}, ErrUnavailable
	}
	request.Header.Set("apikey", verifier.apiKey)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := verifier.client.Do(request)
	if err != nil {
		return User{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusBadRequest {
		return User{}, ErrUnauthenticated
	}
	if response.StatusCode != http.StatusOK {
		return User{}, ErrUnavailable
	}
	var user User
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<10)).Decode(&user); err != nil || strings.TrimSpace(user.ID) == "" {
		return User{}, ErrUnavailable
	}
	return user, nil
}

type principalKey struct{}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, principalKey{}, user)
}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(principalKey{}).(User)
	return user, ok && user.ID != ""
}

func (verifier *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodOptions || request.URL.Path == "/health" {
			next.ServeHTTP(writer, request)
			return
		}
		header := request.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(writer, "authentication required", http.StatusUnauthorized)
			return
		}
		user, err := verifier.Verify(request.Context(), strings.TrimPrefix(header, "Bearer "))
		if errors.Is(err, ErrUnauthenticated) {
			http.Error(writer, "authentication required", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(writer, fmt.Sprintf("%s", ErrUnavailable), http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(writer, request.WithContext(WithUser(request.Context(), user)))
	})
}
