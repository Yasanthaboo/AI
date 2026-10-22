package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/ai-quantity-surveyor/api/internal/auth"
	"github.com/example/ai-quantity-surveyor/api/internal/plans"
)

func TestPlanAccessAndSelfSwitch(t *testing.T) {
	store, err := plans.NewStore(nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerPlanHandlers(mux, store, true)
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) { writer.WriteHeader(204) })
	handler := withPlanAccess(mux, store, true, "nvidia")
	call := func(user, method, path, body string) int {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request = request.WithContext(auth.WithUser(request.Context(), auth.User{ID: user}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}
	for _, scenario := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/api/account/plan", "", 200},
		{"GET", "/api/plans", "", 200},
		{"GET", "/api/projects/project/exterior-options", "", 403},
		{"POST", "/api/projects/project/exterior-options", `{}`, 403},
		{"POST", "/api/floor-plans/plan/analysis", `{"engine":"demo"}`, 204},
		{"POST", "/api/floor-plans/plan/analysis", `{}`, 403},
		{"POST", "/api/floor-plans/plan/analysis", `{"engine":"nvidia"}`, 403},
		{"GET", "/api/material-offers", "", 403},
		{"GET", "/api/material-budgets/budget", "", 403},
		{"GET", "/api/projects/project/material-budgets", "", 403},
		{"POST", "/api/projects/project/material-budgets", `{}`, 403},
		{"GET", "/api/estimates/estimate/export", "", 403},
		{"POST", "/api/projects/project/rooms/room/designs", `{}`, 403},
		{"GET", "/api/room-designs/saved/image", "", 204},
		{"PUT", "/api/account/plan", `{"planId":"studio","userId":"other"}`, 400},
		{"PUT", "/api/account/plan", `{"planId":"unknown"}`, 400},
		{"PUT", "/api/account/plan", `{"planId":"pro"} {}`, 400},
		{"PUT", "/api/account/plan", `{"planId":"pro"}`, 200},
		{"POST", "/api/projects/project/exterior-options", `{}`, 204},
		{"GET", "/api/material-offers", "", 204},
		{"POST", "/api/projects/project/material-budgets", `{}`, 204},
		{"POST", "/api/floor-plans/plan/analysis", `{"engine":"nvidia"}`, 204},
		{"GET", "/api/estimates/estimate/export", "", 204},
		{"POST", "/api/projects/project/rooms/room/designs", `{}`, 403},
		{"PUT", "/api/account/plan", `{"planId":"studio"}`, 200},
		{"POST", "/api/projects/project/rooms/room/designs", `{}`, 204},
	} {
		if status := call("user-a", scenario.method, scenario.path, scenario.body); status != scenario.status {
			t.Fatalf("%s %s: got %d want %d", scenario.method, scenario.path, status, scenario.status)
		}
	}
	if status := call("user-b", "POST", "/api/projects/project/rooms/room/designs", `{}`); status != 403 {
		t.Fatal("another user's plan was applied")
	}
	production := http.NewServeMux()
	registerPlanHandlers(production, store, false)
	response := httptest.NewRecorder()
	production.ServeHTTP(response, httptest.NewRequest("PUT", "/api/account/plan", strings.NewReader(`{"planId":"studio"}`)))
	if response.Code != 403 {
		t.Fatal("production self-upgrade accepted")
	}
}

func TestPlanDemoRequiresNonProductionEnvironment(t *testing.T) {
	t.Setenv("PLAN_DEMO_MODE", "true")
	t.Setenv("APP_ENV", "production")
	if _, err := planDemoModeFromEnvironment(); err == nil {
		t.Fatal("production demo switching accepted")
	}
	t.Setenv("APP_ENV", "demo")
	if demo, err := planDemoModeFromEnvironment(); err != nil || !demo {
		t.Fatalf("demo flag: %v %v", demo, err)
	}
	t.Setenv("PLAN_DEMO_MODE", "false")
	if demo, err := planDemoModeFromEnvironment(); err != nil || demo {
		t.Fatalf("production defaults: %v %v", demo, err)
	}
}
