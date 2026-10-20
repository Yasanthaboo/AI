package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/materials"
	"github.com/example/ai-quantity-surveyor/api/internal/portfolio"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func main() {
	var service project.Store
	var closeStore func() error
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		postgresService, err := project.NewPostgresService(databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		service = postgresService
		closeStore = postgresService.Close
	} else {
		localService, err := project.NewPersistentService("data/projects.json")
		if err != nil {
			log.Fatal(err)
		}
		service = localService
	}
	if closeStore != nil {
		defer closeStore()
	}
	var analyzer analysis.Analyzer = analysis.DemoAnalyzer{}
	engines := map[string]analysis.Analyzer{"demo": analyzer}
	availableEngines := []string{"demo"}
	if endpoint := os.Getenv("OLLAMA_ENDPOINT"); endpoint != "" {
		analyzer = analysis.OllamaAnalyzer{Endpoint: endpoint, Model: os.Getenv("OLLAMA_MODEL"), APIKey: os.Getenv("OLLAMA_API_KEY")}
		engines["ollama"] = analyzer
		availableEngines = append(availableEngines, "ollama")
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		engines["gemini"] = analysis.GeminiAnalyzer{APIKey: key, Model: os.Getenv("GEMINI_MODEL")}
		availableEngines = append(availableEngines, "gemini")
	}
	analysisService := analysis.NewServiceWithEngines(analyzer, engines, nil)
	var jobStore analysis.JobStore = analysis.NewMemoryJobStore()
	durableJobs := false
	estimateService := estimate.NewService(analysisService)
	var materialRepository materials.Repository = materials.NewMemoryRepository()
	if postgresService, ok := service.(*project.PostgresService); ok {
		analysisRepository, repositoryErr := analysis.NewPostgresRepository(postgresService.DB())
		if repositoryErr != nil {
			log.Fatal(repositoryErr)
		}
		analysisService = analysis.NewServiceWithEngines(analyzer, engines, analysisRepository)
		jobStore = analysisRepository
		durableJobs = true
		estimateRepository, repositoryErr := estimate.NewPostgresRepository(postgresService.DB())
		if repositoryErr != nil {
			log.Fatal(repositoryErr)
		}
		estimateService = estimate.NewServiceWithRepository(analysisService, estimateRepository)
		materialRepository, repositoryErr = materials.NewPostgresRepository(postgresService.DB())
		if repositoryErr != nil {
			log.Fatal(repositoryErr)
		}
	}
	limits, quotaErr := providerLimitsFromEnvironment()
	if quotaErr != nil {
		log.Fatal(quotaErr)
	}
	quotaScope := os.Getenv("PROVIDER_QUOTA_SCOPE")
	if !durableJobs {
		limits = map[string]int{}
	}
	availableEngines = []string{"demo"}
	defaultEngine := "demo"
	for _, engine := range []string{"ollama", "gemini"} {
		if engines[engine] != nil && limits[engine] > 0 && quotaScope != "" {
			availableEngines = append(availableEngines, engine)
			if engine == "ollama" {
				defaultEngine = engine
			}
		}
	}
	if err := analysisService.EnableRuntime(analysis.RuntimeConfig{Jobs: jobStore, Preparer: analysis.NewPagePreparer(), QuotaScope: quotaScope, DailyLimits: limits, DefaultEngine: defaultEngine}); err != nil {
		log.Fatal(err)
	}
	defer analysisService.Close()
	mux := http.NewServeMux()
	materialService := materials.NewService(estimateService, analysisService, service, materialRepository)
	materialService.EnablePlasterMaterials(os.Getenv("SURVEYOR_ENABLE_PLASTER_MATERIALS") == "true")
	registerMaterialHandlers(mux, service, materialService)
	mux.HandleFunc("GET /health", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/analysis-engines", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, availableEngines)
	})
	mux.HandleFunc("POST /api/projects", func(writer http.ResponseWriter, request *http.Request) {
		var input project.Details
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64<<10)).Decode(&input); err != nil {
			writeError(writer, http.StatusBadRequest, "InvalidRequest", "request body must be valid JSON", false)
			return
		}
		created, err := service.CreateProject(input)
		if err != nil {
			writeError(writer, http.StatusBadRequest, "ValidationError", err.Error(), false)
			return
		}
		writeJSON(writer, http.StatusCreated, created)
	})
	mux.HandleFunc("GET /api/projects/{id}", func(writer http.ResponseWriter, request *http.Request) {
		projectData, ok := service.GetProject(request.PathValue("id"))
		if !ok {
			writeError(writer, http.StatusNotFound, "NotFound", "project not found", false)
			return
		}
		writeJSON(writer, http.StatusOK, projectData)
	})
	registerPortfolioHandlers(mux, service, analysisService, estimateService)
	mux.HandleFunc("POST /api/projects/{id}/floor-plan", func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(project.MaxFloorPlanSize); err != nil {
			writeError(writer, http.StatusBadRequest, "UploadFailed", "could not read uploaded floor plan", true)
			return
		}
		file, header, err := request.FormFile("file")
		if err != nil {
			writeError(writer, http.StatusBadRequest, "UploadFailed", "multipart field 'file' is required", true)
			return
		}
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, project.MaxFloorPlanSize+1))
		if err != nil {
			writeError(writer, http.StatusBadRequest, "UploadFailed", "could not read uploaded floor plan", true)
			return
		}
		planned, err := service.SaveFloorPlan(request.PathValue("id"), header.Filename, header.Header.Get("Content-Type"), content)
		if err != nil {
			writeError(writer, http.StatusBadRequest, "ValidationError", err.Error(), false)
			return
		}
		writeJSON(writer, http.StatusCreated, planned)
	})
	registerAnalysisHandlers(mux, service, analysisService)
	registerConfirmationHandlers(mux, analysisService)
	mux.HandleFunc("POST /api/projects/{id}/estimates", func(writer http.ResponseWriter, request *http.Request) {
		var input estimate.Request
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			writeError(writer, http.StatusBadRequest, "InvalidRequest", "request body must be valid estimate data", false)
			return
		}
		created, err := estimateService.Create(request.PathValue("id"), input)
		if err != nil {
			writeError(writer, http.StatusBadRequest, "CalculationError", err.Error(), false)
			return
		}
		writeJSON(writer, http.StatusCreated, created)
	})
	mux.HandleFunc("GET /api/projects/{id}/estimates", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, estimateService.List(request.PathValue("id")))
	})
	mux.HandleFunc("GET /api/estimates/{id}", func(writer http.ResponseWriter, request *http.Request) {
		for _, projectID := range []string{} {
			_ = projectID
		}
		result, ok := estimateService.GetByID(request.PathValue("id"))
		if !ok {
			writeError(writer, http.StatusNotFound, "NotFound", "estimate not found", false)
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
	mux.HandleFunc("GET /api/estimates/{id}/export", func(writer http.ResponseWriter, request *http.Request) {
		workbook, err := estimateService.Export(request.PathValue("id"))
		if err != nil {
			writeError(writer, http.StatusNotFound, "ExportFailed", err.Error(), false)
			return
		}
		writer.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		writer.Header().Set("Content-Disposition", `attachment; filename="estimate.xlsx"`)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(workbook)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("AI Quantity Surveyor API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3001"
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		writer.Header().Set("Access-Control-Expose-Headers", "ETag, Retry-After")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func providerLimitsFromEnvironment() (map[string]int, error) {
	limits := make(map[string]int)
	for engine, key := range map[string]string{"gemini": "GEMINI_DAILY_CALL_LIMIT", "ollama": "OLLAMA_DAILY_CALL_LIMIT"} {
		value := os.Getenv(key)
		if value == "" {
			continue
		}
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 0 {
			return nil, errors.New(key + " must be a nonnegative integer")
		}
		limits[engine] = limit
	}
	return limits, nil
}

func registerAnalysisHandlers(mux *http.ServeMux, store project.Store, analyses *analysis.Service) {
	loadDrawing := func(writer http.ResponseWriter, request *http.Request) (project.FloorPlan, []byte, bool) {
		floorPlan, exists := store.GetFloorPlan(request.PathValue("id"))
		if !exists {
			writeError(writer, http.StatusNotFound, "NotFound", "floor plan not found", false)
			return project.FloorPlan{}, nil, false
		}
		content, exists := store.GetFile(floorPlan.ID)
		if !exists {
			writeError(writer, http.StatusNotFound, "FileNotFound", "stored floor plan file not found", true)
			return project.FloorPlan{}, nil, false
		}
		return floorPlan, content, true
	}
	mux.HandleFunc("GET /api/floor-plans/{id}/pages", func(writer http.ResponseWriter, request *http.Request) {
		floorPlan, content, ok := loadDrawing(writer, request)
		if !ok {
			return
		}
		metadata, err := analyses.Metadata(request.Context(), content, floorPlan.FileName)
		if err != nil {
			writeAnalysisError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, struct {
			FloorPlanID string `json:"floorPlanId"`
			analysis.PageMetadata
		}{FloorPlanID: floorPlan.ID, PageMetadata: metadata})
	})
	mux.HandleFunc("POST /api/floor-plans/{id}/analysis", func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Engine     string `json:"engine"`
			PageNumber *int   `json:"pageNumber"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil && err != io.EOF {
			var oversized *http.MaxBytesError
			if errors.As(err, &oversized) {
				writeError(writer, http.StatusRequestEntityTooLarge, "InvalidRequest", "analysis request exceeds 64 KiB", false)
			} else {
				writeError(writer, http.StatusBadRequest, "InvalidRequest", "analysis request must be valid JSON", false)
			}
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			var oversized *http.MaxBytesError
			if errors.As(err, &oversized) {
				writeError(writer, http.StatusRequestEntityTooLarge, "InvalidRequest", "analysis request exceeds 64 KiB", false)
			} else {
				writeError(writer, http.StatusBadRequest, "InvalidRequest", "analysis request must contain one JSON value", false)
			}
			return
		}
		floorPlan, content, ok := loadDrawing(writer, request)
		if !ok {
			return
		}
		started, err := analyses.StartWithPage(request.Context(), input.Engine, floorPlan.ID, content, floorPlan.FileName, input.PageNumber)
		if err != nil {
			writeAnalysisError(writer, err)
			return
		}
		writeJSON(writer, http.StatusAccepted, started)
	})
	mux.HandleFunc("GET /api/analyses/{id}", func(writer http.ResponseWriter, request *http.Request) {
		candidate, found, err := analyses.Read(request.Context(), request.PathValue("id"))
		if err != nil {
			writeAnalysisError(writer, err)
			return
		}
		if !found {
			writeError(writer, http.StatusNotFound, "NotFound", "analysis not found", false)
			return
		}
		writeJSON(writer, http.StatusOK, candidate)
	})
	mux.HandleFunc("GET /api/analyses/{id}/source-image", func(writer http.ResponseWriter, request *http.Request) {
		page, found, err := analyses.SourceImage(request.Context(), request.PathValue("id"))
		if err != nil {
			writeAnalysisError(writer, err)
			return
		}
		if !found {
			writeError(writer, http.StatusNotFound, "SourceImageUnavailable", "canonical source image is unavailable", false)
			return
		}
		etag := `"` + page.Source.ImageDigest + `"`
		writer.Header().Set("ETag", etag)
		writer.Header().Set("Cache-Control", "private, max-age=3600")
		writer.Header().Set("Content-Type", "image/png")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		if request.Header.Get("If-None-Match") == etag {
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = writer.Write(page.Content)
	})
}

func writeAnalysisError(writer http.ResponseWriter, failure error) {
	var runtimeError *analysis.RuntimeError
	if errors.As(failure, &runtimeError) {
		if runtimeError.Failure.RetryAt != nil {
			seconds := max(1, int(time.Until(*runtimeError.Failure.RetryAt).Seconds())+1)
			writer.Header().Set("Retry-After", strconv.Itoa(seconds))
		}
		writeError(writer, runtimeError.Status, runtimeError.Failure.ErrorCode, runtimeError.Failure.Message, runtimeError.Failure.Retryable)
		return
	}
	var preparation *analysis.PreparationError
	if errors.As(failure, &preparation) {
		status := http.StatusBadRequest
		if preparation.Retryable {
			status = http.StatusServiceUnavailable
		}
		writeError(writer, status, preparation.Code, preparation.Message, preparation.Retryable)
		return
	}
	writeError(writer, http.StatusServiceUnavailable, "AnalysisUnavailable", "analysis service unavailable; retry later", true)
}

func registerPortfolioHandlers(mux *http.ServeMux, store project.Store, analyses *analysis.Service, estimates *estimate.Service) {
	mux.HandleFunc("GET /api/projects", func(writer http.ResponseWriter, request *http.Request) {
		projects := store.ListProjects()
		summaries := make([]portfolio.Summary, 0, len(projects))
		for _, item := range projects {
			summaries = append(summaries, portfolio.Summarize(store, analyses, estimates, item))
		}
		sort.SliceStable(summaries, func(left, right int) bool { return summaries[left].UpdatedAt.After(summaries[right].UpdatedAt) })
		writeJSON(writer, http.StatusOK, summaries)
	})
	mux.HandleFunc("GET /api/projects/{id}/summary", func(writer http.ResponseWriter, request *http.Request) {
		item, ok := store.GetProject(request.PathValue("id"))
		if !ok {
			writeError(writer, http.StatusNotFound, "NotFound", "project not found", false)
			return
		}
		writeJSON(writer, http.StatusOK, portfolio.Summarize(store, analyses, estimates, item))
	})
	mux.HandleFunc("PATCH /api/projects/{id}", func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Archived *bool `json:"archived"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4<<10)).Decode(&input); err != nil || input.Archived == nil {
			writeError(writer, http.StatusBadRequest, "InvalidRequest", `request body must be {"archived": true|false}`, false)
			return
		}
		updated, err := store.SetArchived(request.PathValue("id"), *input.Archived)
		if err != nil {
			writeError(writer, http.StatusNotFound, "NotFound", "project not found", false)
			return
		}
		writeJSON(writer, http.StatusOK, updated)
	})
	mux.HandleFunc("POST /api/projects/{id}/duplicate", func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := store.GetProject(request.PathValue("id")); !ok {
			writeError(writer, http.StatusNotFound, "NotFound", "project not found", false)
			return
		}
		copied, err := project.Duplicate(store, request.PathValue("id"))
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "DuplicateFailed", "project could not be duplicated", true)
			return
		}
		writeJSON(writer, http.StatusCreated, copied)
	})
	mux.HandleFunc("GET /api/floor-plans/{id}/file", func(writer http.ResponseWriter, request *http.Request) {
		floorPlan, ok := store.GetFloorPlan(request.PathValue("id"))
		content, found := store.GetFile(request.PathValue("id"))
		// Serve by the validated extension, never the client-supplied content type.
		contentType := map[string]string{".pdf": "application/pdf", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg"}[strings.ToLower(filepath.Ext(floorPlan.FileName))]
		if !ok || !found || contentType == "" {
			writeError(writer, http.StatusNotFound, "NotFound", "floor plan not found", false)
			return
		}
		writer.Header().Set("Content-Type", contentType)
		writer.Header().Set("Content-Disposition", "inline")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Cache-Control", "private, max-age=3600")
		_, _ = writer.Write(content)
	})
}

func registerConfirmationHandlers(mux *http.ServeMux, analysisService *analysis.Service) {
	mux.HandleFunc("PUT /api/analyses/{id}/confirmed-data", func(writer http.ResponseWriter, request *http.Request) {
		var candidate analysis.Candidate
		if err := json.NewDecoder(request.Body).Decode(&candidate); err != nil {
			writeError(writer, http.StatusBadRequest, "InvalidRequest", "request body must be valid analysis data", false)
			return
		}
		confirmed, err := analysisService.Confirm(request.PathValue("id"), candidate)
		if err != nil {
			writeError(writer, http.StatusBadRequest, "ValidationError", err.Error(), false)
			return
		}
		writeJSON(writer, http.StatusOK, confirmed)
	})
	mux.HandleFunc("GET /api/analyses/{id}/confirmations", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, append([]analysis.Confirmation{}, analysisService.Confirmations(request.PathValue("id"))...))
	})
	mux.HandleFunc("GET /api/analyses/{id}/confirmed-data", func(writer http.ResponseWriter, request *http.Request) {
		result, ok := analysisService.LatestConfirmation(request.PathValue("id"))
		if !ok {
			writeError(writer, http.StatusNotFound, "NotFound", "confirmed analysis not found", false)
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
	mux.HandleFunc("GET /api/analyses/{id}/confirmed-data/{revision}", func(writer http.ResponseWriter, request *http.Request) {
		revision, err := strconv.Atoi(request.PathValue("revision"))
		if err != nil || revision < 1 {
			writeError(writer, http.StatusBadRequest, "InvalidRequest", "revision must be a positive integer", false)
			return
		}
		result, ok := analysisService.ConfirmationRevision(request.PathValue("id"), revision)
		if !ok {
			writeError(writer, http.StatusNotFound, "NotFound", "confirmed revision not found", false)
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, code, message string, retryable bool) {
	writeJSON(writer, status, map[string]any{"errorCode": code, "message": message, "retryable": retryable})
}
