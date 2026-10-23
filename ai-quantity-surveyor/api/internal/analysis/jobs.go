package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrLeaseLost = errors.New("analysis job lease lost")

type RuntimeError struct {
	Status  int
	Failure AnalysisError
}

func (failure *RuntimeError) Error() string { return failure.Failure.Message }

func runtimeFailure(status int, code, message string, retryable bool) *RuntimeError {
	return &RuntimeError{Status: status, Failure: AnalysisError{ErrorCode: code, Message: message, Retryable: retryable}}
}

type JobStore interface {
	CreateJob(context.Context, Candidate, JobControl) error
	ReadJob(context.Context, string) (Candidate, bool, error)
	RenewJob(context.Context, string, JobControl) error
	PublishSource(context.Context, string, JobControl, PreparedPage) error
	ReadSource(context.Context, string) (PreparedPage, bool, error)
	FinishJob(context.Context, Candidate, JobControl) error
	RecoverExpired(context.Context) error
	ReserveCall(context.Context, string, JobControl, string, string, string, int) error
}

type MemoryJobStore struct {
	mu       sync.Mutex
	clock    func() time.Time
	jobs     map[string]Candidate
	controls map[string]JobControl
	sources  map[string]PreparedPage
	usage    map[string]int
	attempts map[string]bool
}

func NewMemoryJobStore() *MemoryJobStore {
	return &MemoryJobStore{clock: time.Now, jobs: map[string]Candidate{}, controls: map[string]JobControl{}, sources: map[string]PreparedPage{}, usage: map[string]int{}, attempts: map[string]bool{}}
}

func validateSource(page PreparedPage, floorPlanID string) error {
	digest := sha256.Sum256(page.Content)
	if len(page.Content) == 0 || len(page.Content) > maxPageBytes || page.Source.FloorPlanID != floorPlanID || page.Source.PageNumber < 1 || page.Source.PixelWidth <= 0 || page.Source.PixelHeight <= 0 || page.Source.ImageDigest != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("invalid canonical source identity")
	}
	return nil
}

func (store *MemoryJobStore) owned(id string, control JobControl) bool {
	current, ok := store.controls[id]
	return ok && current.OwnerID == control.OwnerID && current.AttemptToken == control.AttemptToken && current.LeaseExpiresAt.After(store.clock()) && store.jobs[id].State == StateAnalyzing
}

func (store *MemoryJobStore) CreateJob(ctx context.Context, candidate Candidate, control JobControl) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.jobs[candidate.ID]; exists || candidate.ID == "" || candidate.FloorPlanID == "" || candidate.State != StateAnalyzing || control.OwnerID == "" || control.AttemptToken == "" {
		return fmt.Errorf("invalid or duplicate analysis job")
	}
	cloned, err := copyCandidate(candidate)
	if err != nil {
		return err
	}
	now := store.clock()
	control.HeartbeatAt, control.LeaseExpiresAt = now, now.Add(time.Minute)
	store.jobs[candidate.ID], store.controls[candidate.ID] = cloned, control
	return nil
}

func (store *MemoryJobStore) ReadJob(ctx context.Context, id string) (Candidate, bool, error) {
	if err := ctx.Err(); err != nil {
		return Candidate{}, false, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	candidate, exists := store.jobs[id]
	if !exists {
		return Candidate{}, false, nil
	}
	cloned, err := copyCandidate(candidate)
	return cloned, true, err
}

func (store *MemoryJobStore) RenewJob(ctx context.Context, id string, control JobControl) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.owned(id, control) {
		return ErrLeaseLost
	}
	now := store.clock()
	control.HeartbeatAt, control.LeaseExpiresAt = now, now.Add(time.Minute)
	store.controls[id] = control
	return nil
}

func (store *MemoryJobStore) PublishSource(ctx context.Context, id string, control JobControl, page PreparedPage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.owned(id, control) {
		return ErrLeaseLost
	}
	if err := validateSource(page, store.jobs[id].FloorPlanID); err != nil {
		return err
	}
	if _, exists := store.sources[id]; exists {
		return fmt.Errorf("source image is immutable")
	}
	page.Content = append([]byte(nil), page.Content...)
	store.sources[id] = page
	candidate := store.jobs[id]
	source := page.Source
	candidate.Source = &source
	store.jobs[id] = candidate
	return nil
}

func (store *MemoryJobStore) ReadSource(ctx context.Context, id string) (PreparedPage, bool, error) {
	if err := ctx.Err(); err != nil {
		return PreparedPage{}, false, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	page, exists := store.sources[id]
	page.Content = append([]byte(nil), page.Content...)
	return page, exists, nil
}

func (store *MemoryJobStore) FinishJob(ctx context.Context, candidate Candidate, control JobControl) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.owned(candidate.ID, control) {
		return ErrLeaseLost
	}
	if candidate.State != StateReady && candidate.State != StateFailed {
		return fmt.Errorf("job completion must be terminal")
	}
	original := store.jobs[candidate.ID]
	candidate.FloorPlanID, candidate.Source = original.FloorPlanID, original.Source
	cloned, err := copyCandidate(candidate)
	if err != nil {
		return err
	}
	store.jobs[candidate.ID] = cloned
	delete(store.controls, candidate.ID)
	return nil
}

func (store *MemoryJobStore) RecoverExpired(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.clock().UTC()
	for id, control := range store.controls {
		if control.LeaseExpiresAt.After(now) || store.jobs[id].State != StateAnalyzing {
			continue
		}
		candidate := store.jobs[id]
		candidate.State, candidate.CompletedAt = StateFailed, &now
		candidate.Error = &AnalysisError{ErrorCode: "AnalysisInterrupted", Message: "Analysis interrupted; retry with a new analysis.", Retryable: true}
		candidate.ErrorMessage = candidate.Error.Message
		store.jobs[id] = candidate
		delete(store.controls, id)
	}
	return nil
}

func (store *MemoryJobStore) ReserveCall(ctx context.Context, id string, control JobControl, scope, provider, attempt string, limit int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if limit <= 0 || scope == "" || provider == "" {
		return runtimeFailure(503, "ProviderQuotaNotConfigured", "Provider daily quota is not configured.", false)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.owned(id, control) {
		return ErrLeaseLost
	}
	key := fmt.Sprintf("%q:%q:%q:%q", scope, provider, id, attempt)
	if attempt == "" || store.attempts[key] {
		return fmt.Errorf("provider attempt already dispatched or invalid")
	}
	now := store.clock().UTC()
	bucket := fmt.Sprintf("%q:%q:%s", scope, provider, now.Format("2006-01-02"))
	if store.usage[bucket] >= limit {
		reset := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		failure := runtimeFailure(429, "ProviderQuotaExceeded", "Provider daily quota reached; retry after the UTC reset.", true)
		failure.Failure.RetryAt = &reset
		return failure
	}
	store.usage[bucket]++
	store.attempts[key] = true
	return nil
}
