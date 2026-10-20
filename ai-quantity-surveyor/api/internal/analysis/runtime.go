package analysis

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type PageProcessor interface {
	Metadata(context.Context, []byte, string) (PageMetadata, error)
	Prepare(context.Context, []byte, string, int, string) (PreparedPage, error)
}

type RuntimeConfig struct {
	Jobs          JobStore
	Preparer      PageProcessor
	QuotaScope    string
	DailyLimits   map[string]int
	DefaultEngine string
}

type analysisRuntime struct {
	jobs          JobStore
	preparer      PageProcessor
	scope         string
	limits        map[string]int
	defaultEngine string
	owner         string
	slots         chan struct{}
	render        chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
}

func (service *Service) EnableRuntime(config RuntimeConfig) error {
	if service.runtime != nil || config.Jobs == nil || config.Preparer == nil {
		return fmt.Errorf("analysis runtime requires unique store and preparer")
	}
	ctx, cancel := context.WithCancel(context.Background())
	limits := make(map[string]int)
	for provider, limit := range config.DailyLimits {
		limits[provider] = limit
	}
	service.runtime = &analysisRuntime{jobs: config.Jobs, preparer: config.Preparer, scope: config.QuotaScope, limits: limits, defaultEngine: config.DefaultEngine, owner: newID(), slots: make(chan struct{}, 2), render: make(chan struct{}, 1), ctx: ctx, cancel: cancel}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				check, stop := context.WithTimeout(ctx, 5*time.Second)
				_ = config.Jobs.RecoverExpired(check)
				stop()
			}
		}
	}()
	return nil
}

func (service *Service) Close() {
	if service.runtime != nil {
		service.runtime.cancel()
	}
}

func (service *Service) Metadata(ctx context.Context, content []byte, fileName string) (PageMetadata, error) {
	if service.runtime == nil {
		return PageMetadata{}, runtimeFailure(503, "PreparationUnavailable", "Page preparation is unavailable.", false)
	}
	select {
	case service.runtime.render <- struct{}{}:
	default:
		return PageMetadata{}, runtimeFailure(429, "CapacityExceeded", "Page preparation is busy; retry shortly.", true)
	}
	defer func() { <-service.runtime.render }()
	return service.runtime.preparer.Metadata(ctx, content, fileName)
}

func (service *Service) StartWithPage(ctx context.Context, engine, floorPlanID string, content []byte, fileName string, page *int) (Candidate, error) {
	if service.runtime == nil {
		return Candidate{}, runtimeFailure(503, "PreparationUnavailable", "Managed analysis is unavailable.", false)
	}
	if engine == "" {
		engine = service.runtime.defaultEngine
	}
	analyzer, ok := service.engines[engine]
	if !ok || analyzer == nil {
		return Candidate{}, runtimeFailure(400, "EngineUnavailable", "Selected analysis engine is unavailable.", false)
	}
	return service.startManaged(ctx, engine, analyzer, floorPlanID, content, fileName, page)
}

func (service *Service) startManaged(ctx context.Context, engine string, analyzer Analyzer, floorPlanID string, content []byte, fileName string, selected *int) (Candidate, error) {
	runtime := service.runtime
	if engine == "" {
		engine = runtime.defaultEngine
	}
	if floorPlanID == "" || len(content) == 0 {
		return Candidate{}, runtimeFailure(400, "InvalidRequest", "Floor plan content is required.", false)
	}
	if runtime.ctx.Err() != nil {
		return Candidate{}, runtimeFailure(503, "ServiceUnavailable", "Analysis service is shutting down.", true)
	}
	if engine != "demo" && (runtime.limits[engine] <= 0 || runtime.scope == "") {
		return Candidate{}, runtimeFailure(503, "ProviderQuotaNotConfigured", "Provider daily quota is not configured.", false)
	}
	select {
	case runtime.slots <- struct{}{}:
	default:
		return Candidate{}, runtimeFailure(429, "CapacityExceeded", "Analysis capacity reached; retry shortly.", true)
	}
	accepted := false
	defer func() {
		if !accepted {
			<-runtime.slots
		}
	}()
	metadata, err := service.Metadata(ctx, content, fileName)
	if err != nil {
		return Candidate{}, err
	}
	page, mode := 1, "legacy-default"
	if selected != nil {
		page, mode = *selected, "explicit"
	}
	if page < 1 || page > metadata.PageCount {
		return Candidate{}, runtimeFailure(400, "InvalidSourcePage", "Selected page is outside the drawing.", false)
	}
	if metadata.ContentType != "application/pdf" {
		mode = "image"
	}
	now := time.Now().UTC()
	candidate := Candidate{ID: newID(), FloorPlanID: floorPlanID, State: StateAnalyzing, Engine: engine, StartedAt: &now}
	if namer, ok := analyzer.(ModelNamer); ok {
		candidate.Model = namer.ModelName()
	}
	control := JobControl{OwnerID: runtime.owner, AttemptToken: newID()}
	if err := runtime.jobs.CreateJob(ctx, candidate, control); err != nil {
		return Candidate{}, runtimeFailure(503, "StatusStoreUnavailable", "Could not persist analysis; retry later.", true)
	}
	accepted = true
	go service.runManaged(analyzer, candidate, control, content, fileName, page, mode)
	return candidate, nil
}

func (service *Service) Read(ctx context.Context, id string) (Candidate, bool, error) {
	if service.runtime == nil {
		candidate, found := service.Get(id)
		return candidate, found, nil
	}
	if err := service.runtime.jobs.RecoverExpired(ctx); err != nil {
		return Candidate{}, false, runtimeFailure(503, "StatusStoreUnavailable", "Analysis status unavailable; retry later.", true)
	}
	candidate, found, err := service.runtime.jobs.ReadJob(ctx, id)
	if err != nil {
		return Candidate{}, false, runtimeFailure(503, "StatusStoreUnavailable", "Analysis status unavailable; retry later.", true)
	}
	return candidate, found, nil
}

func (service *Service) SourceImage(ctx context.Context, id string) (PreparedPage, bool, error) {
	if service.runtime == nil {
		return PreparedPage{}, false, runtimeFailure(503, "PreparationUnavailable", "Source image unavailable.", true)
	}
	page, found, err := service.runtime.jobs.ReadSource(ctx, id)
	if err != nil {
		return PreparedPage{}, false, runtimeFailure(503, "StatusStoreUnavailable", "Source image store unavailable; retry later.", true)
	}
	return page, found, nil
}

func (service *Service) runManaged(analyzer Analyzer, started Candidate, control JobControl, content []byte, fileName string, pageNumber int, mode string) {
	runtime := service.runtime
	defer func() { <-runtime.slots }()
	ctx, cancel := context.WithTimeout(runtime.ctx, 6*time.Minute)
	defer cancel()
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				renew, stop := context.WithTimeout(ctx, 5*time.Second)
				err := runtime.jobs.RenewJob(renew, started.ID, control)
				stop()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	finished := started
	finish := func(failure error) {
		completed := time.Now().UTC()
		finished.CompletedAt = &completed
		if failure != nil {
			finished.State = StateFailed
			var detailed *RuntimeError
			var preparation *PreparationError
			if errors.As(failure, &detailed) {
				finished.Error = &detailed.Failure
			} else if errors.As(failure, &preparation) {
				finished.Error = &AnalysisError{ErrorCode: preparation.Code, Message: preparation.Message, Retryable: preparation.Retryable}
			} else {
				finished.Error = &AnalysisError{ErrorCode: "AnalysisFailed", Message: failure.Error(), Retryable: true}
			}
			finished.ErrorMessage = finished.Error.Message
		}
		persist, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := runtime.jobs.FinishJob(persist, finished, control); err == nil {
			service.mu.Lock()
			service.results[finished.ID] = finished
			service.mu.Unlock()
		}
	}
	select {
	case runtime.render <- struct{}{}:
	case <-ctx.Done():
		finish(ctx.Err())
		return
	}
	page, err := runtime.preparer.Prepare(ctx, content, fileName, pageNumber, mode)
	<-runtime.render
	if err != nil {
		finish(err)
		return
	}
	page.Source.FloorPlanID = started.FloorPlanID
	if err := runtime.jobs.PublishSource(ctx, started.ID, control, page); err != nil {
		finish(runtimeFailure(503, "StatusStoreUnavailable", "Could not store the prepared source; retry later.", true))
		return
	}
	finished.Source = &page.Source
	if started.Engine != "demo" {
		err := runtime.jobs.ReserveCall(ctx, started.ID, control, runtime.scope, started.Engine, newID(), runtime.limits[started.Engine])
		if err != nil {
			var detailed *RuntimeError
			if !errors.As(err, &detailed) && !errors.Is(err, ErrLeaseLost) {
				err = runtimeFailure(503, "QuotaStoreUnavailable", "Provider quota store unavailable; retry later.", true)
			}
			finish(err)
			return
		}
	}
	if err := ctx.Err(); err != nil {
		finish(err)
		return
	}
	candidate, err := analyzer.Analyze(ctx, page.Content, "selected-page.png")
	if err != nil {
		finish(err)
		return
	}
	if err := ValidateCandidate(candidate); err != nil {
		finish(runtimeFailure(400, "InvalidAnalysis", err.Error(), true))
		return
	}
	candidate.ID, candidate.FloorPlanID, candidate.Source = started.ID, started.FloorPlanID, &page.Source
	candidate.Engine, candidate.Model, candidate.StartedAt = started.Engine, started.Model, started.StartedAt
	candidate.Corrections, candidate.Error, candidate.ErrorMessage = nil, nil, ""
	candidate.DrawingAspect = float64(page.Source.PixelHeight) / float64(page.Source.PixelWidth)
	for index := range candidate.Rooms {
		candidate.Rooms[index].Assumptions = nil
	}
	sanitizeBounds(&candidate)
	NormalizeEngineEvidence(&candidate)
	finished = candidate
	finish(nil)
}
