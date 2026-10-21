package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"

	"github.com/example/ai-quantity-surveyor/api/internal/materials"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func decodeMaterialBody(writer http.ResponseWriter, request *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		var oversized *http.MaxBytesError
		status := 400
		if errors.As(err, &oversized) {
			status = 413
		}
		writeError(writer, status, "InvalidRequest", "Material request is invalid or exceeds 64 KiB.", false)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(writer, 400, "InvalidRequest", "Use one JSON request object.", false)
		return false
	}
	return true
}

func writeMaterialError(writer http.ResponseWriter, err error) {
	var failure *materials.Error
	if errors.As(err, &failure) {
		writeError(writer, failure.Status, failure.Code, failure.Message, false)
		return
	}
	writeError(writer, 503, "MaterialStoreUnavailable", "Material data unavailable; retry later.", true)
}

func registerMaterialHandlers(mux *http.ServeMux, projects project.Store, service *materials.Service) {
	mux.HandleFunc("GET /api/material-capabilities", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, 200, map[string]bool{"plasterMaterials": service.PlasterMaterialsEnabled()})
	})
	mux.HandleFunc("POST /api/projects/{id}/material-budgets", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := projectForRequest(projects, request, request.PathValue("id")); !found {
			writeError(writer, 404, "NotFound", "Project not found.", false)
			return
		}
		var input materials.BudgetRequest
		if !decodeMaterialBody(writer, request, &input) {
			return
		}
		budget, err := service.CreateBudget(request.Context(), request.PathValue("id"), input)
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		writeJSON(writer, 201, budget)
	})
	mux.HandleFunc("GET /api/projects/{id}/material-budgets", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := projectForRequest(projects, request, request.PathValue("id")); !found {
			writeError(writer, 404, "NotFound", "Project not found.", false)
			return
		}
		budgets, err := service.ListBudgets(request.Context(), request.PathValue("id"))
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		sort.Slice(budgets, func(first, second int) bool { return budgets[first].Version > budgets[second].Version })
		writeJSON(writer, 200, budgets)
	})
	mux.HandleFunc("GET /api/material-budgets/{id}", func(writer http.ResponseWriter, request *http.Request) {
		budget, found, err := service.GetBudget(request.Context(), request.PathValue("id"))
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		if !found || !projectOwnedByRequest(projects, request, budget.ProjectID) {
			writeError(writer, 404, "NotFound", "Budget not found.", false)
			return
		}
		writeJSON(writer, 200, budget)
	})
	for _, path := range []string{"GET /api/material-products", "GET /api/material-offers"} {
		mux.HandleFunc(path, func(writer http.ResponseWriter, request *http.Request) {
			country, city := request.URL.Query().Get("country"), request.URL.Query().Get("city")
			if (country != "" && country != "LK") || (city != "" && city != "Colombo") {
				writeJSON(writer, 200, []materials.Offer{})
				return
			}
			offers, err := service.Offers(request.Context())
			if err != nil {
				writeMaterialError(writer, err)
				return
			}
			filtered := make([]materials.Offer, 0)
			for _, offer := range offers {
				if category := request.URL.Query().Get("category"); category == "" || category == offer.Recipe.Category {
					filtered = append(filtered, offer)
				}
			}
			writeJSON(writer, 200, filtered)
		})
	}
	mux.HandleFunc("POST /api/material-budgets/{id}/comparisons/preview", func(writer http.ResponseWriter, request *http.Request) {
		budget, found, err := service.GetBudget(request.Context(), request.PathValue("id"))
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		if !found || !projectOwnedByRequest(projects, request, budget.ProjectID) {
			writeError(writer, 404, "NotFound", "Material budget not found.", false)
			return
		}
		var input materials.PreviewRequest
		if !decodeMaterialBody(writer, request, &input) {
			return
		}
		preview, err := service.Preview(request.Context(), request.PathValue("id"), input)
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		writeJSON(writer, 200, preview)
	})
	mux.HandleFunc("POST /api/material-budgets/{id}/comparisons", func(writer http.ResponseWriter, request *http.Request) {
		budget, found, err := service.GetBudget(request.Context(), request.PathValue("id"))
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		if !found || !projectOwnedByRequest(projects, request, budget.ProjectID) {
			writeError(writer, 404, "NotFound", "Material budget not found.", false)
			return
		}
		var input materials.ComparisonRequest
		if !decodeMaterialBody(writer, request, &input) {
			return
		}
		comparison, err := service.SaveComparison(request.Context(), request.PathValue("id"), input)
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		writeJSON(writer, 201, comparison)
	})
	mux.HandleFunc("GET /api/material-comparisons/{id}", func(writer http.ResponseWriter, request *http.Request) {
		comparison, found, err := service.GetComparison(request.Context(), request.PathValue("id"))
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		if !found || !projectOwnedByRequest(projects, request, comparison.ProjectID) {
			writeError(writer, 404, "NotFound", "Comparison not found.", false)
			return
		}
		writeJSON(writer, 200, comparison)
	})
	mux.HandleFunc("GET /api/material-comparisons/{id}/export", func(writer http.ResponseWriter, request *http.Request) {
		comparison, found, err := service.GetComparison(request.Context(), request.PathValue("id"))
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		if !found || !projectOwnedByRequest(projects, request, comparison.ProjectID) {
			writeError(writer, 404, "NotFound", "Comparison not found.", false)
			return
		}
		content, err := service.Export(request.Context(), request.PathValue("id"))
		if err != nil {
			writeMaterialError(writer, err)
			return
		}
		writer.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		writer.Header().Set("Content-Disposition", `attachment; filename="material-comparison.xlsx"`)
		_, _ = writer.Write(content)
	})
}
