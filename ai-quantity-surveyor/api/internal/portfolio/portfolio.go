package portfolio

import (
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

const (
	StageDraft     = "Draft"
	StageUploaded  = "Uploaded"
	StageAnalyzed  = "Analyzed"
	StageConfirmed = "Confirmed"
	StageEstimated = "Estimated"
)

type Analyses interface {
	LatestForFloorPlan(string) (analysis.Candidate, bool)
	LatestConfirmation(string) (analysis.Confirmation, bool)
}

type Estimates interface {
	List(string) []estimate.Version
}

type Summary struct {
	Project           project.Project    `json:"project"`
	Stage             string             `json:"stage"`
	FloorPlan         *project.FloorPlan `json:"floorPlan,omitempty"`
	AnalysisID        string             `json:"analysisId,omitempty"`
	Engine            string             `json:"engine,omitempty"`
	ConfirmedRevision int                `json:"confirmedRevision,omitempty"`
	RoomCount         int                `json:"roomCount"`
	OpeningCount      int                `json:"openingCount"`
	EstimateCount     int                `json:"estimateCount"`
	LatestTotal       *float64           `json:"latestTotal,omitempty"`
	LatestCurrency    string             `json:"latestCurrency,omitempty"`
	UpdatedAt         time.Time          `json:"updatedAt"`
}

// Summarize derives a project's stage and latest figures; estimates count only for its current analysis.
func Summarize(store project.Store, analyses Analyses, estimates Estimates, source project.Project) Summary {
	summary := Summary{Project: source, Stage: StageDraft, UpdatedAt: source.CreatedAt}
	touch := func(moment time.Time) {
		if moment.After(summary.UpdatedAt) {
			summary.UpdatedAt = moment
		}
	}
	floorPlan, ok := store.FloorPlanForProject(source.ID)
	if !ok {
		return summary
	}
	summary.FloorPlan, summary.Stage = &floorPlan, StageUploaded
	touch(floorPlan.UploadedAt)
	candidate, ok := analyses.LatestForFloorPlan(floorPlan.ID)
	if !ok {
		return summary
	}
	summary.AnalysisID, summary.Engine, summary.Stage = candidate.ID, candidate.Engine, StageAnalyzed
	summary.RoomCount, summary.OpeningCount = len(candidate.Rooms), len(candidate.Doors)+len(candidate.Windows)
	if candidate.CompletedAt != nil {
		touch(*candidate.CompletedAt)
	}
	if confirmation, ok := analyses.LatestConfirmation(candidate.ID); ok {
		summary.ConfirmedRevision, summary.Stage = confirmation.Revision, StageConfirmed
		summary.RoomCount, summary.OpeningCount = len(confirmation.Candidate.Rooms), len(confirmation.Candidate.Doors)+len(confirmation.Candidate.Windows)
		touch(confirmation.ConfirmedAt)
	}
	for _, version := range estimates.List(source.ID) {
		if version.SourceAnalysisID != candidate.ID {
			continue
		}
		if summary.EstimateCount == 0 {
			total := version.ConvertedGrandTotal
			summary.LatestTotal, summary.LatestCurrency, summary.Stage = &total, version.TargetCurrency, StageEstimated
		}
		summary.EstimateCount++
		touch(version.CalculatedAt)
	}
	return summary
}
