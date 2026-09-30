package analysis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/example/ai-quantity-surveyor/api/internal/domain"
)

type State string

const (
	StateAnalyzing State = "Analyzing"
	StateReady     State = "AnalysisReady"
	StateFailed    State = "AnalysisFailed"
)

type Candidate struct {
	ID                   string    `json:"id"`
	FloorPlanID          string    `json:"floorPlanId"`
	ConfirmationRevision int       `json:"confirmationRevision,omitempty"`
	DrawingAspect        float64   `json:"drawingAspect,omitempty"`
	Rooms                []Room    `json:"rooms"`
	Doors                []Opening `json:"doors"`
	Windows              []Opening `json:"windows"`
	State                State     `json:"analysisState"`
	ErrorMessage         string    `json:"errorMessage,omitempty"`
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
}

const extractionPrompt = `Extract rooms, rectangular dimensions, wall heights, doors, windows, units, and confidence from this floor plan. Return a JSON object with analysisState "AnalysisReady", rooms, doors, and windows. Rooms have id, name, length, width, wallHeight, bounds, confidence (0 to 1), and uncertainty. Use the room label on the drawing as name. Read length and width from the dimension text written in or beside each room (for example 4.20 x 3.60 or 12'6" x 10'); convert feet-and-inches to decimal feet. bounds is the room's axis-aligned outline on the drawing as {"x", "y", "width", "height"} fractions (0 to 1) of the drawing width and height, measured from the top-left corner; adjacent rooms should share edges. Doors and windows have id, roomId, width, height, confidence, and uncertainty. Each measurement is {"value": number, "unit": "m|cm|ft|in"}. Never invent missing values: use null and an uncertainty reason. Do not calculate quantities or costs.`

type Analyzer interface {
	Analyze(context.Context, []byte, string) (Candidate, error)
}

func Validate(candidate Candidate) error {
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
		if err := validateMeasurement(opening.Width, opening.Confidence, opening.Uncertainty, "width"); err != nil {
			return fmt.Errorf("opening %q: %w", opening.ID, err)
		}
		if err := validateMeasurement(opening.Height, opening.Confidence, opening.Uncertainty, "height"); err != nil {
			return fmt.Errorf("opening %q: %w", opening.ID, err)
		}
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
	if confidence < 0 || confidence > 1 {
		return errors.New("confidence must be between 0 and 1")
	}
	if confidence < 1 && strings.TrimSpace(uncertainty) == "" {
		return fmt.Errorf("%s is uncertain but has no reason", label)
	}
	if _, err := domain.ConvertToMeters(*measurement); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}
