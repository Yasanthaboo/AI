package main

import (
	"net/http"
	"strconv"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/exterior"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func registerExteriorHandlers(mux *http.ServeMux, projects project.Store, analyses *analysis.Service, store *exterior.Store) {
	validSource := func(writer http.ResponseWriter, request *http.Request, analysisID string, revision int) bool {
		projectID := request.PathValue("id")
		if _, found := projectForRequest(projects, request, projectID); !found {
			writeError(writer, 404, "NotFound", "Project not found.", false)
			return false
		}
		if analysisID == "" || revision < 1 {
			writeError(writer, 400, "InvalidExteriorSource", "A source analysis and positive confirmation revision are required.", false)
			return false
		}
		ownedProject, found := analysisProjectForRequest(projects, analyses, request, analysisID)
		if !found || ownedProject.ID != projectID {
			writeError(writer, 404, "NotFound", "Analysis not found for this project.", false)
			return false
		}
		if snapshot, found := analyses.ConfirmationRevision(analysisID, revision); !found || snapshot.AnalysisID != analysisID || len(snapshot.Candidate.Rooms) == 0 {
			writeError(writer, 409, "ConfirmationRequired", "This confirmed revision is unavailable.", false)
			return false
		}
		return true
	}
	mux.HandleFunc("GET /api/projects/{id}/exterior-options", func(writer http.ResponseWriter, request *http.Request) {
		revision, _ := strconv.Atoi(request.URL.Query().Get("revision"))
		analysisID := request.URL.Query().Get("analysisId")
		if !validSource(writer, request, analysisID, revision) {
			return
		}
		options, err := store.List(request.Context(), request.PathValue("id"), analysisID, revision)
		if err != nil {
			writeError(writer, 503, "ExteriorStoreUnavailable", "Saved exterior options are unavailable.", true)
			return
		}
		writeJSON(writer, 200, options)
	})
	mux.HandleFunc("POST /api/projects/{id}/exterior-options", func(writer http.ResponseWriter, request *http.Request) {
		if _, found := projectForRequest(projects, request, request.PathValue("id")); !found {
			writeError(writer, 404, "NotFound", "Project not found.", false)
			return
		}
		var input exterior.Input
		if !decodeRoomDesignBody(writer, request, &input) {
			return
		}
		if err := input.Validate(); err != nil {
			writeError(writer, 400, "InvalidExteriorOption", err.Error(), false)
			return
		}
		if !validSource(writer, request, input.AnalysisID, input.ConfirmationRevision) {
			return
		}
		option, err := store.Save(request.Context(), request.PathValue("id"), input)
		if err != nil {
			writeError(writer, 503, "ExteriorStoreUnavailable", "Exterior option could not be saved.", true)
			return
		}
		writeJSON(writer, 201, option)
	})
}
