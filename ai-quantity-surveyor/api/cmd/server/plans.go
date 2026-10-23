package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/example/ai-quantity-surveyor/api/internal/plans"
)

func planDemoModeFromEnvironment() (bool, error) {
	raw := os.Getenv("PLAN_DEMO_MODE")
	if raw == "" || raw == "false" {
		return false, nil
	}
	if raw != "true" {
		return false, errors.New("PLAN_DEMO_MODE must be true or false")
	}
	if environment := os.Getenv("APP_ENV"); environment != "demo" && environment != "development" {
		return false, errors.New("PLAN_DEMO_MODE=true requires APP_ENV=demo or development; never enable it in production")
	}
	return true, nil
}

func registerPlanHandlers(mux *http.ServeMux, store *plans.Store, demo bool) {
	mux.HandleFunc("GET /api/plans", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, plans.Catalog())
	})
	mux.HandleFunc("GET /api/account/plan", func(writer http.ResponseWriter, request *http.Request) {
		account, err := store.Account(request.Context(), requestOwnerID(request), demo)
		if err != nil {
			writeError(writer, 503, "PlanUnavailable", "Account plan is unavailable; retry later.", true)
			return
		}
		writeJSON(writer, http.StatusOK, account)
	})
	mux.HandleFunc("PUT /api/account/plan", func(writer http.ResponseWriter, request *http.Request) {
		if !demo {
			writeError(writer, 403, "PlanSwitchDisabled", "Self-service plan changes are available only in an explicitly enabled demo environment.", false)
			return
		}
		var input struct {
			PlanID string `json:"planId"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(writer, 400, "InvalidPlan", "Choose a valid plan.", false)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			writeError(writer, 400, "InvalidPlan", "Request must contain one plan selection.", false)
			return
		}
		if _, valid := plans.Find(input.PlanID); !valid {
			writeError(writer, 400, "InvalidPlan", "Choose Free, Pro, or Studio.", false)
			return
		}
		account, err := store.SwitchDemo(request.Context(), requestOwnerID(request), input.PlanID, demo)
		if err != nil {
			writeError(writer, 503, "PlanUnavailable", "Could not save the account plan; retry later.", true)
			return
		}
		writeJSON(writer, http.StatusOK, account)
	})
}

func withPlanAccess(next http.Handler, store *plans.Store, demo bool, defaultEngine string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		var feature plans.Feature
		switch {
		case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/exterior-options"):
			feature = plans.Model3D
		case strings.HasPrefix(path, "/api/material-") || (strings.HasPrefix(path, "/api/projects/") && strings.Contains(path, "/material-budgets")):
			feature = plans.Materials
		case request.Method == http.MethodGet && strings.HasPrefix(path, "/api/estimates/") && strings.HasSuffix(path, "/export"):
			feature = plans.Exports
		case request.Method == http.MethodPost && strings.HasPrefix(path, "/api/projects/") && strings.Contains(path, "/rooms/") && strings.HasSuffix(path, "/designs"):
			feature = plans.DesignStudio
		case request.Method == http.MethodPost && strings.HasPrefix(path, "/api/floor-plans/") && strings.HasSuffix(path, "/analysis"):
			body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 64<<10))
			if err != nil {
				writeError(writer, 400, "InvalidRequest", "Analysis request is invalid or too large.", false)
				return
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			var input struct {
				Engine string `json:"engine"`
			}
			if len(bytes.TrimSpace(body)) > 0 {
				if err := json.Unmarshal(body, &input); err != nil {
					writeError(writer, 400, "InvalidRequest", "Analysis request must be valid JSON.", false)
					return
				}
			}
			engine := input.Engine
			if engine == "" {
				engine = defaultEngine
			}
			if engine != "demo" {
				feature = plans.AIAnalysis
			}
		}
		if feature != "" {
			account, err := store.Account(request.Context(), requestOwnerID(request), demo)
			if err != nil {
				writeError(writer, 503, "PlanUnavailable", "Account plan is unavailable; retry later.", true)
				return
			}
			if !account.Plan.Allows(feature) {
				writeJSON(writer, http.StatusForbidden, map[string]any{"errorCode": "PlanUpgradeRequired", "message": "Your current plan does not include this feature. Review the available plans.", "feature": feature, "retryable": false})
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}
