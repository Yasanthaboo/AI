package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/example/ai-quantity-surveyor/api/internal/project"
	"github.com/example/ai-quantity-surveyor/api/internal/roomdesign"
)

func decodeRoomDesignBody(writer http.ResponseWriter, request *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		var oversized *http.MaxBytesError
		status := http.StatusBadRequest
		if errors.As(err, &oversized) {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(writer, status, "InvalidRequest", "Room design request is invalid or exceeds 8 KiB.", false)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(writer, http.StatusBadRequest, "InvalidRequest", "Use one JSON request object.", false)
		return false
	}
	return true
}

func writeRoomDesignError(writer http.ResponseWriter, err error) {
	var failure *roomdesign.Error
	if errors.As(err, &failure) {
		writeError(writer, failure.Status, failure.Code, failure.Message, failure.Status >= http.StatusInternalServerError)
		return
	}
	writeError(writer, http.StatusServiceUnavailable, "DesignStoreUnavailable", "Room design service unavailable; retry later.", true)
}

func registerRoomDesignHandlers(mux *http.ServeMux, projects project.Store, service *roomdesign.Service) {
	mux.HandleFunc("GET /api/room-design-capabilities", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]any{"enabled": service.Enabled(), "dailyImageLimit": service.DailyLimit(), "styles": roomdesign.Styles(), "palettes": roomdesign.Palettes()})
	})
	mux.HandleFunc("GET /api/projects/{id}/rooms/{roomId}/designs", func(writer http.ResponseWriter, request *http.Request) {
		projectID, roomID := request.PathValue("id"), request.PathValue("roomId")
		if _, found := projectForRequest(projects, request, projectID); !found {
			writeError(writer, http.StatusNotFound, "NotFound", "Project not found.", false)
			return
		}
		revision, err := strconv.Atoi(request.URL.Query().Get("revision"))
		if err != nil || revision < 1 {
			writeError(writer, http.StatusBadRequest, "InvalidDesignRequest", "A positive confirmation revision is required.", false)
			return
		}
		concepts, err := service.List(request.Context(), projectID, roomID, revision)
		if err != nil {
			writeRoomDesignError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, concepts)
	})
	mux.HandleFunc("POST /api/projects/{id}/rooms/{roomId}/designs", func(writer http.ResponseWriter, request *http.Request) {
		projectID := request.PathValue("id")
		if _, found := projectForRequest(projects, request, projectID); !found {
			writeError(writer, http.StatusNotFound, "NotFound", "Project not found.", false)
			return
		}
		var input roomdesign.Request
		if !decodeRoomDesignBody(writer, request, &input) {
			return
		}
		input.RoomID = request.PathValue("roomId")
		concepts, err := service.Generate(request.Context(), requestOwnerID(request), projectID, input)
		if err != nil {
			writeRoomDesignError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, concepts)
	})
	mux.HandleFunc("GET /api/room-designs/{id}/image", func(writer http.ResponseWriter, request *http.Request) {
		concept, found, err := service.Get(request.Context(), request.PathValue("id"))
		if err != nil {
			writeRoomDesignError(writer, err)
			return
		}
		if !found || !projectOwnedByRequest(projects, request, concept.ProjectID) {
			writeError(writer, http.StatusNotFound, "NotFound", "Design concept not found.", false)
			return
		}
		image, found, err := service.Image(request.Context(), concept.ID)
		if err != nil {
			writeRoomDesignError(writer, err)
			return
		}
		if !found {
			writeError(writer, http.StatusNotFound, "NotFound", "Design image not found.", false)
			return
		}
		writer.Header().Set("Content-Type", "image/png")
		writer.Header().Set("Content-Disposition", "inline")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Cache-Control", "private, no-store")
		_, _ = writer.Write(image)
	})
	mux.HandleFunc("POST /api/room-designs/{id}/selection", func(writer http.ResponseWriter, request *http.Request) {
		concept, found, err := service.Get(request.Context(), request.PathValue("id"))
		if err != nil {
			writeRoomDesignError(writer, err)
			return
		}
		if !found || !projectOwnedByRequest(projects, request, concept.ProjectID) {
			writeError(writer, http.StatusNotFound, "NotFound", "Design concept not found.", false)
			return
		}
		if err := service.Select(request.Context(), concept.ProjectID, concept.RoomID, concept.ConfirmationRevision, concept.ID); err != nil {
			writeRoomDesignError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]bool{"selected": true})
	})
	mux.HandleFunc("DELETE /api/room-designs/{id}", func(writer http.ResponseWriter, request *http.Request) {
		concept, found, err := service.Get(request.Context(), request.PathValue("id"))
		if err != nil {
			writeRoomDesignError(writer, err)
			return
		}
		if !found || !projectOwnedByRequest(projects, request, concept.ProjectID) {
			writeError(writer, http.StatusNotFound, "NotFound", "Design concept not found.", false)
			return
		}
		if err := service.Delete(request.Context(), concept.ID); err != nil {
			writeRoomDesignError(writer, err)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})
}
