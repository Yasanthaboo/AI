package roomdesign

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/domain"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

type captureGenerator struct {
	calls  int
	prompt Prompt
	count  int
	err    error
}

var testPNG = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}

func (generator *captureGenerator) Generate(_ context.Context, prompt Prompt, count int) ([]GeneratedImage, error) {
	generator.calls++
	generator.prompt = prompt
	generator.count = count
	if generator.err != nil {
		return nil, generator.err
	}
	return []GeneratedImage{{PNG: append([]byte(nil), testPNG...), Model: "test-model"}, {PNG: append([]byte(nil), testPNG...), Model: "test-model"}}, nil
}

func TestBuildPromptUsesOnlyConfirmedRoomText(t *testing.T) {
	room := analysis.Room{ID: "room-1", Name: "Kitchen", Length: &domain.Measurement{Value: 4, Unit: domain.UnitMeters}, Width: &domain.Measurement{Value: 3, Unit: domain.UnitMeters}, WallHeight: &domain.Measurement{Value: 2.7, Unit: domain.UnitMeters}}
	doors := []analysis.Opening{{ID: "door-1", RoomID: "room-1", Width: &domain.Measurement{Value: 0.9, Unit: domain.UnitMeters}, Height: &domain.Measurement{Value: 2, Unit: domain.UnitMeters}}}
	windows := []analysis.Opening{{ID: "window-elsewhere", RoomID: "other-room", Width: &domain.Measurement{Value: 2, Unit: domain.UnitMeters}, Height: &domain.Measurement{Value: 1.2, Unit: domain.UnitMeters}}}
	prompt, err := buildPrompt(room, doors, windows, Request{Style: "Contemporary", Palette: "Warm neutrals", Preferences: "Island bench"})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.RoomName != "Kitchen" || prompt.Length != "4 m" || prompt.Width != "3 m" || prompt.WallHeight != "2.7 m" || len(prompt.Openings) != 1 || prompt.Openings[0].Kind != "door" || prompt.Preferences != "Island bench" {
		t.Fatalf("unexpected prompt input: %+v", prompt)
	}
}

func TestBuildPromptRejectsMissingConfirmedDimensions(t *testing.T) {
	room := analysis.Room{ID: "room-1", Name: "Kitchen", Length: &domain.Measurement{Value: 0, Unit: domain.UnitMeters}}
	if _, err := buildPrompt(room, nil, nil, Request{Style: "Contemporary", Palette: "Warm neutrals"}); err == nil {
		t.Fatal("missing confirmed dimension accepted")
	}
}

func TestChoicesAreRestricted(t *testing.T) {
	if validChoice("Contemporary", styles) == false || validChoice("invented style", styles) || validChoice("Warm neutrals", palettes) == false || validChoice("", palettes) {
		t.Fatal("design choice validation failed")
	}
}

func TestGenerationFailureMessageClassifiesProviderResponsesSafely(t *testing.T) {
	checks := []struct {
		failure error
		want    string
	}{
		{&ProviderError{Provider: "OpenAI", StatusCode: 403}, "The image provider rejected authorization."},
		{&ProviderError{Provider: "OpenAI", StatusCode: 429}, "The image provider rate limit or account quota was reached."},
		{&ProviderError{Provider: "OpenAI", StatusCode: 400, Code: "moderation_blocked"}, "The image provider blocked this request."},
		{&url.Error{Op: "Post", URL: "https://invalid.example/images", Err: &tls.CertificateVerificationError{Err: x509.HostnameError{Certificate: &x509.Certificate{DNSNames: []string{"provider.example"}}, Host: "invalid.example"}}}, "The image provider TLS certificate could not be verified."},
	}
	for _, check := range checks {
		if message := generationFailureMessage(check.failure); !strings.Contains(message, check.want) {
			t.Fatalf("message %q does not include %q", message, check.want)
		}
	}
}

func TestDailyQuotaCountsImagesNotRequests(t *testing.T) {
	repository := NewMemoryRepository()
	now := time.Date(2026, 10, 20, 18, 0, 0, 0, time.FixedZone("local", 5*60*60+30*60))
	if err := repository.ReserveDaily(context.Background(), "user-a", now, 2, 2); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReserveDaily(context.Background(), "user-a", now, 1, 2); !errors.Is(err, ErrDailyLimit) {
		t.Fatalf("expected quota rejection, got %v", err)
	}
	if err := repository.ReserveDaily(context.Background(), "user-b", now, 2, 2); err != nil {
		t.Fatalf("quota leaked across users: %v", err)
	}
	if err := repository.ReserveDaily(context.Background(), "user-a", now.Add(12*time.Hour), 2, 2); err != nil {
		t.Fatalf("quota did not roll over at UTC midnight: %v", err)
	}
}

func confirmedProjectFixture(t *testing.T) (*project.Service, *analysis.Service, project.Project, analysis.Candidate, int) {
	t.Helper()
	projects := project.NewService()
	created, err := projects.CreateProjectForOwner(project.Details{Name: "Room design fixture"}, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	floorPlan, err := projects.SaveFloorPlan(created.ID, "plan.png", "image/png", []byte("drawing bytes are not passed to generator"))
	if err != nil {
		t.Fatal(err)
	}
	analyses := analysis.NewService(analysis.DemoAnalyzer{})
	started, err := analyses.Start(floorPlan.ID, []byte("drawing bytes are not passed to generator"), floorPlan.FileName)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	var candidate analysis.Candidate
	for {
		candidate, _ = analyses.Get(started.ID)
		if candidate.State == analysis.StateReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("demo analysis did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := analyses.Confirm(started.ID, candidate); err != nil {
		t.Fatal(err)
	}
	return projects, analyses, created, candidate, 1
}

func TestGenerateStoresTwoConceptsAgainstConfirmedRevision(t *testing.T) {
	projects, analyses, created, candidate, revision := confirmedProjectFixture(t)
	room := candidate.Rooms[0]
	generator := &captureGenerator{}
	repository := NewMemoryRepository()
	service := NewService(projects, analyses, repository, generator, 2)
	concepts, err := service.Generate(context.Background(), "user-a", created.ID, Request{RoomID: room.ID, ConfirmationRevision: revision, Style: "Contemporary", Palette: "Warm neutrals", Preferences: "Reading corner"})
	if err != nil {
		t.Fatal(err)
	}
	if len(concepts) != 2 || generator.calls != 1 || generator.count != 2 || generator.prompt.RoomName != room.Name || generator.prompt.Preferences != "Reading corner" {
		t.Fatalf("generated concepts/prompt = %+v %+v", concepts, generator.prompt)
	}
	if concepts[0].AnalysisID != candidate.ID || concepts[0].ConfirmationRevision != revision || concepts[0].RoomID != room.ID || concepts[0].Model != "test-model" {
		t.Fatalf("missing source provenance: %+v", concepts[0])
	}
	if _, err := service.Generate(context.Background(), "user-b", created.ID, Request{RoomID: room.ID, ConfirmationRevision: revision, Style: "Contemporary", Palette: "Warm neutrals"}); err == nil {
		t.Fatal("foreign user generated a concept for this project")
	}
	if _, err := service.Generate(context.Background(), "user-a", created.ID, Request{RoomID: room.ID, ConfirmationRevision: 2, Style: "Contemporary", Palette: "Warm neutrals"}); err == nil {
		t.Fatal("unavailable confirmation revision was accepted")
	}
	if err := service.Select(context.Background(), created.ID, room.ID, revision, concepts[1].ID); err != nil {
		t.Fatal(err)
	}
	listed, err := service.List(context.Background(), created.ID, room.ID, revision)
	selectedCount := 0
	for _, concept := range listed {
		if concept.Selected {
			selectedCount++
		}
	}
	if err != nil || len(listed) != 2 || selectedCount != 1 {
		t.Fatalf("selected concept list = %+v, %v", listed, err)
	}
	if err := service.Delete(context.Background(), concepts[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := service.Get(context.Background(), concepts[0].ID); found {
		t.Fatal("deleted concept remains stored")
	}
}

func TestFailedGenerationDoesNotPersistConcepts(t *testing.T) {
	projects, analyses, created, candidate, revision := confirmedProjectFixture(t)
	generator := &captureGenerator{err: errors.New("provider unavailable")}
	repository := NewMemoryRepository()
	service := NewService(projects, analyses, repository, generator, 2)
	room := candidate.Rooms[0]
	if _, err := service.Generate(context.Background(), "user-a", created.ID, Request{RoomID: room.ID, ConfirmationRevision: revision, Style: "Contemporary", Palette: "Warm neutrals"}); err == nil {
		t.Fatal("provider failure did not fail generation")
	}
	concepts, err := repository.List(context.Background(), created.ID, room.ID, revision)
	if err != nil || len(concepts) != 0 {
		t.Fatalf("failed generation saved concepts: %+v %v", concepts, err)
	}
}
