package roomdesign

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/domain"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

var ErrUnavailable = errors.New("room design generation is unavailable")
var ErrNotFound = errors.New("room design not found")
var ErrDailyLimit = errors.New("daily image generation limit reached")

type Error struct {
	Status  int
	Code    string
	Message string
}

func (failure *Error) Error() string { return failure.Message }

func fail(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}

type Request struct {
	Provider             string `json:"provider,omitempty"`
	RoomID               string `json:"roomId"`
	ConfirmationRevision int    `json:"confirmationRevision"`
	Style                string `json:"style"`
	Palette              string `json:"palette"`
	Preferences          string `json:"preferences"`
}

type OpeningInput struct {
	Kind   string `json:"kind"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

type Prompt struct {
	RoomName    string
	Length      string
	Width       string
	WallHeight  string
	Openings    []OpeningInput
	Style       string
	Palette     string
	Preferences string
}

type Concept struct {
	ID                   string    `json:"id"`
	ProjectID            string    `json:"projectId"`
	AnalysisID           string    `json:"analysisId"`
	ConfirmationRevision int       `json:"confirmationRevision"`
	RoomID               string    `json:"roomId"`
	RoomName             string    `json:"roomName"`
	Style                string    `json:"style"`
	Palette              string    `json:"palette"`
	Preferences          string    `json:"preferences,omitempty"`
	Model                string    `json:"model"`
	Selected             bool      `json:"selected"`
	CreatedAt            time.Time `json:"createdAt"`
}

type GeneratedImage struct {
	PNG   []byte
	Model string
}

type Generator interface {
	Generate(context.Context, Prompt, int) ([]GeneratedImage, error)
}

type Repository interface {
	ReserveDaily(context.Context, string, time.Time, int, int) error
	SaveBatch(context.Context, []Concept, [][]byte) error
	List(context.Context, string, string, int) ([]Concept, error)
	Get(context.Context, string) (Concept, bool, error)
	Image(context.Context, string) ([]byte, bool, error)
	Select(context.Context, string, string, int, string) error
	Delete(context.Context, string) error
}

type Service struct {
	projects   project.Store
	analyses   *analysis.Service
	repository Repository
	generator  Generator
	dailyLimit int
}

func NewService(projects project.Store, analyses *analysis.Service, repository Repository, generator Generator, dailyLimit int) *Service {
	return &Service{projects: projects, analyses: analyses, repository: repository, generator: generator, dailyLimit: dailyLimit}
}

func (service *Service) Enabled() bool   { return service.generator != nil && service.dailyLimit > 0 }
func (service *Service) DailyLimit() int { return service.dailyLimit }

func (service *Service) Generate(ctx context.Context, ownerID, projectID string, input Request) ([]Concept, error) {
	if service.generator == nil || service.dailyLimit <= 0 {
		return nil, fail(503, "DesignGenerationUnavailable", "AI room design is not configured.")
	}
	if input.ConfirmationRevision < 1 || strings.TrimSpace(input.RoomID) == "" || !validChoice(input.Style, styles) || !validChoice(input.Palette, palettes) || len(input.Preferences) > 300 {
		return nil, fail(400, "InvalidDesignRequest", "Choose a confirmed room, design style, and palette; preferences must be 300 characters or fewer.")
	}
	generator := service.generator
	if router, ok := generator.(*ProviderGenerator); ok {
		var err error
		generator, err = router.Select(input.Provider)
		if err != nil {
			return nil, fail(400, "InvalidImageProvider", "Choose an available image provider.")
		}
	} else if input.Provider != "" && input.Provider != DescribeGenerator(generator).ID {
		return nil, fail(400, "InvalidImageProvider", "Choose an available image provider.")
	}
	projectRecord, found := service.projects.GetProjectForOwner(projectID, ownerID)
	if !found {
		return nil, fail(404, "NotFound", "Project not found.")
	}
	floorPlan, found := service.projects.FloorPlanForProject(projectRecord.ID)
	if !found {
		return nil, fail(409, "ConfirmationRequired", "A confirmed floor plan is required before generating designs.")
	}
	candidate, found := service.analyses.LatestForFloorPlan(floorPlan.ID)
	if !found {
		return nil, fail(409, "ConfirmationRequired", "A confirmed floor plan is required before generating designs.")
	}
	snapshot, found := service.analyses.ConfirmationRevision(candidate.ID, input.ConfirmationRevision)
	if !found || snapshot.AnalysisID != candidate.ID || snapshot.Candidate.FloorPlanID != floorPlan.ID {
		return nil, fail(409, "ConfirmationRevisionUnavailable", "The requested confirmed revision is unavailable for this project.")
	}
	var room *analysis.Room
	for index := range snapshot.Candidate.Rooms {
		if snapshot.Candidate.Rooms[index].ID == input.RoomID {
			room = &snapshot.Candidate.Rooms[index]
			break
		}
	}
	if room == nil {
		return nil, fail(404, "RoomNotFound", "Room not found in the confirmed revision.")
	}
	prompt, err := buildPrompt(*room, snapshot.Candidate.Doors, snapshot.Candidate.Windows, input)
	if err != nil {
		return nil, fail(409, "RoomDimensionsUnavailable", err.Error())
	}
	if err := service.repository.ReserveDaily(ctx, ownerID, time.Now().UTC(), 2, service.dailyLimit); err != nil {
		if errors.Is(err, ErrDailyLimit) {
			return nil, fail(429, "DesignDailyLimitReached", "Daily design image limit reached; try again tomorrow.")
		}
		return nil, fail(503, "DesignQuotaUnavailable", "Design generation quota is unavailable; retry later.")
	}
	images, err := generator.Generate(ctx, prompt, 2)
	if err != nil {
		log.Printf("room design provider generation failed: %v", err)
		return nil, fail(502, "DesignGenerationFailed", generationFailureMessage(err))
	}
	if len(images) != 2 {
		return nil, fail(502, "DesignGenerationFailed", "The image provider returned an incomplete concept set.")
	}
	concepts := make([]Concept, 2)
	contents := make([][]byte, 2)
	for index, image := range images {
		if !validPNG(image.PNG) || len(image.PNG) > 8<<20 || image.Model == "" {
			return nil, fail(502, "DesignGenerationFailed", "The image provider returned an invalid concept.")
		}
		concepts[index] = Concept{ID: newID(), ProjectID: projectID, AnalysisID: candidate.ID, ConfirmationRevision: snapshot.Revision, RoomID: room.ID, RoomName: room.Name, Style: input.Style, Palette: input.Palette, Preferences: strings.TrimSpace(input.Preferences), Model: image.Model, CreatedAt: time.Now().UTC()}
		contents[index] = append([]byte(nil), image.PNG...)
	}
	if err := service.repository.SaveBatch(ctx, concepts, contents); err != nil {
		return nil, fail(503, "DesignStoreUnavailable", "Generated concepts could not be saved; retry later.")
	}
	return concepts, nil
}

func generationFailureMessage(err error) string {
	var certificateFailure *tls.CertificateVerificationError
	if errors.As(err, &certificateFailure) {
		return "The image provider TLS certificate could not be verified. Check the configured image endpoint hostname and certificate; retrying will not fix this configuration error."
	}
	var providerFailure *ProviderError
	if !errors.As(err, &providerFailure) {
		return "Could not generate room concepts; try again later."
	}
	if providerFailure.Code == "moderation_blocked" {
		return "The image provider blocked this request. Adjust the design preferences and try again."
	}
	switch providerFailure.StatusCode {
	case 401, 403:
		return "The image provider rejected authorization. Check its API key, model access, and organization verification."
	case 429:
		return "The image provider rate limit or account quota was reached. Check provider usage and retry later."
	default:
		return fmt.Sprintf("The image provider returned HTTP %d; check API logs and retry later.", providerFailure.StatusCode)
	}
}

func validPNG(content []byte) bool {
	return len(content) >= 8 && bytes.Equal(content[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
}

func (service *Service) List(ctx context.Context, projectID, roomID string, revision int) ([]Concept, error) {
	return service.repository.List(ctx, projectID, roomID, revision)
}

func (service *Service) Get(ctx context.Context, id string) (Concept, bool, error) {
	return service.repository.Get(ctx, id)
}

func (service *Service) Image(ctx context.Context, id string) ([]byte, bool, error) {
	return service.repository.Image(ctx, id)
}

func (service *Service) Select(ctx context.Context, projectID, roomID string, revision int, id string) error {
	err := service.repository.Select(ctx, projectID, roomID, revision, id)
	if errors.Is(err, ErrNotFound) {
		return fail(404, "NotFound", "Design concept not found.")
	}
	if err != nil {
		return fail(503, "DesignStoreUnavailable", "Could not save the selected concept.")
	}
	return nil
}

func (service *Service) Delete(ctx context.Context, id string) error {
	err := service.repository.Delete(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return fail(404, "NotFound", "Design concept not found.")
	}
	if err != nil {
		return fail(503, "DesignStoreUnavailable", "Could not remove the design concept.")
	}
	return nil
}

var styles = map[string]bool{"Contemporary": true, "Scandinavian": true, "Minimalist": true, "Industrial": true, "Tropical": true, "Traditional": true}
var palettes = map[string]bool{"Warm neutrals": true, "Cool neutrals": true, "Earth tones": true, "Coastal": true, "Monochrome": true, "Colorful": true}

func Styles() []string {
	return []string{"Contemporary", "Scandinavian", "Minimalist", "Industrial", "Tropical", "Traditional"}
}
func Palettes() []string {
	return []string{"Warm neutrals", "Cool neutrals", "Earth tones", "Coastal", "Monochrome", "Colorful"}
}

func validChoice(value string, allowed map[string]bool) bool { return allowed[value] }

func buildPrompt(room analysis.Room, doors, windows []analysis.Opening, input Request) (Prompt, error) {
	length, err := formatMeasurement(room.Length)
	if err != nil {
		return Prompt{}, fmt.Errorf("confirmed room length is unavailable")
	}
	width, err := formatMeasurement(room.Width)
	if err != nil {
		return Prompt{}, fmt.Errorf("confirmed room width is unavailable")
	}
	height, err := formatMeasurement(room.WallHeight)
	if err != nil {
		return Prompt{}, fmt.Errorf("confirmed wall height is unavailable")
	}
	openings := make([]OpeningInput, 0)
	for _, entry := range []struct {
		kind  string
		items []analysis.Opening
	}{{"door", doors}, {"window", windows}} {
		for _, opening := range entry.items {
			if opening.RoomID != room.ID {
				continue
			}
			openingWidth, widthErr := formatMeasurement(opening.Width)
			openingHeight, heightErr := formatMeasurement(opening.Height)
			if widthErr == nil && heightErr == nil {
				openings = append(openings, OpeningInput{Kind: entry.kind, Width: openingWidth, Height: openingHeight})
			}
		}
	}
	preferences := strings.Map(func(value rune) rune {
		if value < 32 || value == 127 {
			return ' '
		}
		return value
	}, strings.TrimSpace(input.Preferences))
	if len(preferences) > 300 {
		return Prompt{}, errors.New("design preferences are too long")
	}
	return Prompt{RoomName: strings.TrimSpace(room.Name), Length: length, Width: width, WallHeight: height, Openings: openings, Style: input.Style, Palette: input.Palette, Preferences: preferences}, nil
}

func formatMeasurement(measurement *domain.Measurement) (string, error) {
	if measurement == nil || measurement.Value <= 0 || math.IsNaN(measurement.Value) || math.IsInf(measurement.Value, 0) {
		return "", errors.New("confirmed dimension is unavailable")
	}
	unit := string(measurement.Unit)
	switch measurement.Unit {
	case domain.UnitFeet, domain.UnitInches, domain.UnitMeters, domain.UnitCentimeters:
	default:
		return "", errors.New("confirmed dimension unit is unsupported")
	}
	return fmt.Sprintf("%.4g %s", measurement.Value, unit), nil
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
