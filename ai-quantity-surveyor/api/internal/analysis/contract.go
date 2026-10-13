package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/domain"
)

type State string

const (
	StateAnalyzing State = "Analyzing"
	StateReady     State = "AnalysisReady"
	StateFailed    State = "AnalysisFailed"
)

type Candidate struct {
	ID                   string         `json:"id"`
	FloorPlanID          string         `json:"floorPlanId"`
	ConfirmationRevision int            `json:"confirmationRevision,omitempty"`
	DrawingAspect        float64        `json:"drawingAspect,omitempty"`
	Engine               string         `json:"engine,omitempty"`
	Model                string         `json:"model,omitempty"`
	StartedAt            *time.Time     `json:"startedAt,omitempty"`
	CompletedAt          *time.Time     `json:"completedAt,omitempty"`
	Rooms                []Room         `json:"rooms"`
	Doors                []Opening      `json:"doors"`
	Windows              []Opening      `json:"windows"`
	State                State          `json:"analysisState"`
	ErrorMessage         string         `json:"errorMessage,omitempty"`
	Source               *Source        `json:"source,omitempty"`
	Features             []Feature      `json:"features,omitempty"`
	ReviewIssues         []ReviewIssue  `json:"reviewIssues,omitempty"`
	Corrections          []Correction   `json:"corrections,omitempty"`
	Error                *AnalysisError `json:"error,omitempty"`
}

type Source struct {
	FloorPlanID   string `json:"floorPlanId"`
	PageNumber    int    `json:"pageNumber"`
	SelectionMode string `json:"selectionMode"`
	ImageDigest   string `json:"imageDigest"`
	PixelWidth    int    `json:"pixelWidth"`
	PixelHeight   int    `json:"pixelHeight"`
}

type Evidence struct {
	AnalysisID   string   `json:"analysisId"`
	FloorPlanID  string   `json:"floorPlanId"`
	PageNumber   int      `json:"pageNumber"`
	Region       *Bounds  `json:"region,omitempty"`
	ObservedText string   `json:"observedText,omitempty"`
	Confidence   *float64 `json:"confidence,omitempty"`
	Uncertainty  string   `json:"uncertainty,omitempty"`
	Origin       string   `json:"origin"`
}

type FeatureType string

const (
	FeatureStair   FeatureType = "stair"
	FeatureBalcony FeatureType = "balcony"
)

type Feature struct {
	ID          string      `json:"id"`
	Type        FeatureType `json:"type"`
	Label       string      `json:"label"`
	RoomID      string      `json:"roomId,omitempty"`
	Evidence    *Evidence   `json:"evidence,omitempty"`
	Confidence  *float64    `json:"confidence,omitempty"`
	Uncertainty string      `json:"uncertainty,omitempty"`
	Disposition string      `json:"disposition"`
	Origin      string      `json:"origin,omitempty"`
}

type ReviewIssue struct {
	EntityID string `json:"entityId"`
	Field    string `json:"field,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

type Correction struct {
	EntityID       string          `json:"entityId"`
	Field          string          `json:"field"`
	OriginalValue  json.RawMessage `json:"originalValue"`
	ConfirmedValue json.RawMessage `json:"confirmedValue"`
	Origin         string          `json:"origin"`
}

type AnalysisError struct {
	ErrorCode string     `json:"errorCode"`
	Message   string     `json:"message"`
	Retryable bool       `json:"retryable"`
	RetryAt   *time.Time `json:"retryAt,omitempty"`
}

type JobControl struct {
	OwnerID        string    `json:"-"`
	AttemptToken   string    `json:"-"`
	LeaseExpiresAt time.Time `json:"-"`
	HeartbeatAt    time.Time `json:"-"`
}

// Bounds is a room's extent on the drawing as fractions of its width and height.
type Bounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Room struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Length      *domain.Measurement `json:"length"`
	Width       *domain.Measurement `json:"width"`
	WallHeight  *domain.Measurement `json:"wallHeight"`
	Bounds      *Bounds             `json:"bounds,omitempty"`
	Confidence  float64             `json:"confidence"`
	Uncertainty string              `json:"uncertainty,omitempty"`
	// Assumptions names fields the user accepted from configured defaults, e.g. "wallHeight".
	Assumptions []string            `json:"assumptions,omitempty"`
	Evidence    map[string]Evidence `json:"evidence,omitempty"`
}

var assumableFields = map[string]bool{"wallHeight": true}

func sanitizeAssumptions(candidate *Candidate) {
	for index := range candidate.Rooms {
		var kept []string
		for _, field := range candidate.Rooms[index].Assumptions {
			if assumableFields[field] {
				kept = append(kept, field)
			}
		}
		candidate.Rooms[index].Assumptions = kept
	}
}

// ModelNamer is implemented by analyzers that can report the model they call.
type ModelNamer interface {
	ModelName() string
}

// sanitizeBounds drops unusable room bounds and rescales 0-1000 model coordinates.
func sanitizeBounds(candidate *Candidate) {
	for index := range candidate.Rooms {
		bounds := candidate.Rooms[index].Bounds
		if bounds == nil {
			continue
		}
		values := []float64{bounds.X, bounds.Y, bounds.Width, bounds.Height}
		scale := 1.0
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1000 {
				candidate.Rooms[index].Bounds = nil
				break
			}
			if value > 1.05 {
				scale = 1000
			}
		}
		if candidate.Rooms[index].Bounds == nil {
			continue
		}
		clean := Bounds{X: bounds.X / scale, Y: bounds.Y / scale, Width: bounds.Width / scale, Height: bounds.Height / scale}
		clean.X, clean.Y = math.Min(clean.X, 1), math.Min(clean.Y, 1)
		clean.Width, clean.Height = math.Min(clean.Width, 1-clean.X), math.Min(clean.Height, 1-clean.Y)
		if clean.Width < 0.005 || clean.Height < 0.005 {
			candidate.Rooms[index].Bounds = nil
			continue
		}
		candidate.Rooms[index].Bounds = &clean
	}
	if math.IsNaN(candidate.DrawingAspect) || math.IsInf(candidate.DrawingAspect, 0) || candidate.DrawingAspect < 0.05 || candidate.DrawingAspect > 20 {
		candidate.DrawingAspect = 0
	}
}

type Opening struct {
	ID          string              `json:"id"`
	RoomID      string              `json:"roomId,omitempty"`
	Width       *domain.Measurement `json:"width"`
	Height      *domain.Measurement `json:"height"`
	Confidence  float64             `json:"confidence"`
	Uncertainty string              `json:"uncertainty,omitempty"`
	Evidence    map[string]Evidence `json:"evidence,omitempty"`
}

const extractionPrompt = `Extract rooms, rectangular dimensions, wall heights, doors, windows, units, and confidence from this floor plan. Return a JSON object with analysisState "AnalysisReady", rooms, doors, and windows. Rooms have id, name, length, width, wallHeight, bounds, confidence (0 to 1), and uncertainty. Use the room label on the drawing as name. Read length and width from the dimension text written in or beside each room (for example 4.20 x 3.60 or 12'6" x 10'); convert feet-and-inches to decimal feet. bounds is the room's axis-aligned outline on the drawing as {"x", "y", "width", "height"} fractions (0 to 1) of the drawing width and height, measured from the top-left corner; adjacent rooms should share edges. Doors and windows have id, roomId, width, height, confidence, and uncertainty. Each measurement is {"value": number, "unit": "m|cm|ft|in"}. Never invent missing values: use null and an uncertainty reason. Do not calculate quantities or costs.`

type Analyzer interface {
	Analyze(context.Context, []byte, string) (Candidate, error)
}

func Validate(candidate Candidate) error {
	if err := ValidateCandidate(candidate); err != nil {
		return err
	}
	if candidate.State != StateReady {
		return fmt.Errorf("analysis state must be %q", StateReady)
	}
	if len(candidate.Rooms) == 0 {
		return errors.New("analysis must contain at least one room")
	}
	for _, room := range candidate.Rooms {
		if strings.TrimSpace(room.ID) == "" || strings.TrimSpace(room.Name) == "" {
			return errors.New("each room requires an id and name")
		}
		if err := validateMeasurement(room.Length, room.Confidence, room.Uncertainty, "length"); err != nil {
			return fmt.Errorf("room %q: %w", room.Name, err)
		}
		if err := validateMeasurement(room.Width, room.Confidence, room.Uncertainty, "width"); err != nil {
			return fmt.Errorf("room %q: %w", room.Name, err)
		}
		if err := validateMeasurement(room.WallHeight, room.Confidence, room.Uncertainty, "wall height"); err != nil {
			return fmt.Errorf("room %q: %w", room.Name, err)
		}
	}
	for _, opening := range append(candidate.Doors, candidate.Windows...) {
		if strings.TrimSpace(opening.ID) == "" {
			return errors.New("each opening requires an id")
		}
		if opening.RoomID == "" {
			return fmt.Errorf("opening %q requires a room association", opening.ID)
		}
		if err := validateMeasurement(opening.Width, opening.Confidence, opening.Uncertainty, "width"); err != nil {
			return fmt.Errorf("opening %q: %w", opening.ID, err)
		}
		if err := validateMeasurement(opening.Height, opening.Confidence, opening.Uncertainty, "height"); err != nil {
			return fmt.Errorf("opening %q: %w", opening.ID, err)
		}
	}
	return nil
}

func ValidateCandidate(candidate Candidate) error {
	if candidate.State != StateReady {
		return fmt.Errorf("analysis state must be %q", StateReady)
	}
	if len(candidate.Rooms) == 0 {
		return errors.New("analysis must contain at least one room")
	}
	ids := make(map[string]bool)
	rooms := make(map[string]bool)
	register := func(id string) error {
		if strings.TrimSpace(id) == "" {
			return errors.New("each entity requires an id")
		}
		if ids[id] {
			return fmt.Errorf("duplicate entity id %q", id)
		}
		ids[id] = true
		return nil
	}
	for _, room := range candidate.Rooms {
		if err := register(room.ID); err != nil {
			return err
		}
		if strings.TrimSpace(room.Name) == "" {
			return fmt.Errorf("room %q requires a name", room.ID)
		}
		if err := validateConfidence(room.Confidence); err != nil {
			return fmt.Errorf("room %q: %w", room.ID, err)
		}
		rooms[room.ID] = true
	}
	for _, openings := range [][]Opening{candidate.Doors, candidate.Windows} {
		for _, opening := range openings {
			if err := register(opening.ID); err != nil {
				return err
			}
			if opening.RoomID != "" && !rooms[opening.RoomID] {
				return fmt.Errorf("opening %q references unknown room %q", opening.ID, opening.RoomID)
			}
			if err := validateConfidence(opening.Confidence); err != nil {
				return fmt.Errorf("opening %q: %w", opening.ID, err)
			}
		}
	}
	for _, feature := range candidate.Features {
		if err := register(feature.ID); err != nil {
			return err
		}
		if feature.Type != FeatureStair && feature.Type != FeatureBalcony {
			return fmt.Errorf("feature %q has unsupported type %q", feature.ID, feature.Type)
		}
		if strings.TrimSpace(feature.Label) == "" {
			return fmt.Errorf("feature %q requires a label", feature.ID)
		}
		if feature.RoomID != "" && !rooms[feature.RoomID] {
			return fmt.Errorf("feature %q references unknown room %q", feature.ID, feature.RoomID)
		}
		if feature.Confidence != nil {
			if err := validateConfidence(*feature.Confidence); err != nil {
				return fmt.Errorf("feature %q: %w", feature.ID, err)
			}
		}
	}
	return nil
}

func validateConfidence(confidence float64) error {
	if math.IsNaN(confidence) || math.IsInf(confidence, 0) || confidence < 0 || confidence > 1 {
		return errors.New("confidence must be finite and between 0 and 1")
	}
	return nil
}

func validateMeasurement(measurement *domain.Measurement, confidence float64, uncertainty, label string) error {
	if measurement == nil {
		return fmt.Errorf("%s is missing", label)
	}
	if measurement.Value <= 0 || math.IsNaN(measurement.Value) || math.IsInf(measurement.Value, 0) {
		return fmt.Errorf("%s must be positive and finite", label)
	}
	if err := validateConfidence(confidence); err != nil {
		return err
	}
	if confidence < 1 && strings.TrimSpace(uncertainty) == "" {
		return fmt.Errorf("%s is uncertain but has no reason", label)
	}
	if _, err := domain.ConvertToMeters(*measurement); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}
