package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSecurityHeadersAndServerBounds(t *testing.T) {
	t.Setenv("ALLOWED_ORIGIN", "https://web.example.test")
	called := false
	handler := withCORS(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { called = true; writer.WriteHeader(200) }))
	request := httptest.NewRequest("OPTIONS", "/api/room-designs/id", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if called || response.Code != 204 || !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), "DELETE") {
		t.Fatal("invalid preflight handling")
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "https://web.example.test" {
		t.Fatal("untrusted origin was reflected")
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("X-Frame-Options") != "DENY" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("missing security or private-cache headers")
	}
	server := apiHTTPServer(":0", handler)
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout == 0 || server.IdleTimeout == 0 || server.MaxHeaderBytes > 64<<10 || server.WriteTimeout < 6*time.Minute {
		t.Fatal("server resource bounds are missing or break provider fallback")
	}
}
