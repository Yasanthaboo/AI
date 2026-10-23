package planseed

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const sampleDescription = "Synthetic plan preview fixture v1. Predefined demo dimensions, not a real customer drawing. No paid analysis or image generation was used."

type workspaceRegistry struct {
	db       *sql.DB
	mutex    sync.Mutex
	projects map[string]string
}

func newWorkspaceRegistry(db *sql.DB) (*workspaceRegistry, error) {
	if db != nil {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS plan_preview_workspaces (
 user_id TEXT PRIMARY KEY, project_id TEXT NOT NULL UNIQUE REFERENCES projects(id), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());`); err != nil {
			return nil, errors.New("could not initialize preview workspace registry")
		}
	}
	return &workspaceRegistry{db: db, projects: make(map[string]string)}, nil
}

func (registry *workspaceRegistry) get(ctx context.Context, userID string) (string, error) {
	if registry.db != nil {
		var projectID string
		err := registry.db.QueryRowContext(ctx, `SELECT project_id FROM plan_preview_workspaces WHERE user_id=$1`, userID).Scan(&projectID)
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return projectID, err
	}
	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	return registry.projects[userID], nil
}

func (registry *workspaceRegistry) bind(ctx context.Context, userID, projectID string) error {
	if registry.db != nil {
		if _, err := registry.db.ExecContext(ctx, `INSERT INTO plan_preview_workspaces(user_id,project_id) VALUES($1,$2) ON CONFLICT(user_id) DO NOTHING`, userID, projectID); err != nil {
			return err
		}
		stored, err := registry.get(ctx, userID)
		if err != nil {
			return err
		}
		if stored != projectID {
			return errors.New("preview workspace registry conflicts with the selected project")
		}
		return nil
	}
	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	if previous := registry.projects[userID]; previous != "" && previous != projectID {
		return errors.New("preview workspace already bound")
	}
	registry.projects[userID] = projectID
	return nil
}

func sampleDrawing() ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 500, 500))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{250, 252, 253, 255}), image.Point{}, draw.Src)
	text := func(x, y int, label string) {
		drawer := font.Drawer{Dst: canvas, Src: image.NewUniform(color.RGBA{20, 45, 55, 255}), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
		drawer.DrawString(label)
	}
	text(25, 20, "SYNTHETIC PLAN PREVIEW - NOT A REAL BUILDING")
	text(25, 38, "Predefined demo dimensions; wall height 2.8 m")
	for _, room := range []struct {
		bounds           image.Rectangle
		name, dimensions string
		fill             color.RGBA
	}{
		{image.Rect(25, 50, 225, 300), "Living room", "4 x 5 m", color.RGBA{217, 239, 242, 255}},
		{image.Rect(225, 50, 400, 200), "Kitchen", "3.5 x 3 m", color.RGBA{229, 239, 217, 255}},
		{image.Rect(225, 200, 325, 300), "Bathroom", "2 x 2 m", color.RGBA{239, 224, 217, 255}},
		{image.Rect(25, 300, 225, 450), "Bedroom", "4 x 3 m", color.RGBA{224, 229, 245, 255}},
	} {
		draw.Draw(canvas, room.bounds, image.NewUniform(color.RGBA{30, 55, 65, 255}), image.Point{}, draw.Src)
		draw.Draw(canvas, room.bounds.Inset(3), image.NewUniform(room.fill), image.Point{}, draw.Src)
		text(room.bounds.Min.X+9, room.bounds.Min.Y+25, room.name)
		text(room.bounds.Min.X+9, room.bounds.Min.Y+45, room.dimensions)
	}
	text(25, 475, "Sample only: openings and layout are indicative.")
	var content bytes.Buffer
	if err := png.Encode(&content, canvas); err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

func ensureSample(ctx context.Context, user User, projects project.Store, analyses *analysis.Service, estimates *estimate.Service, registry *workspaceRegistry) error {
	projectID, err := registry.get(ctx, user.ID)
	if err != nil {
		return errors.New("could not read preview workspace registry")
	}
	if projectID != "" {
		if _, found := projects.GetProjectForOwner(projectID, user.ID); !found {
			return errors.New("preview workspace owner does not match; manual review required")
		}
	} else {
		for _, existing := range projects.ListProjectsForOwner(user.ID) {
			if existing.Description == sampleDescription {
				projectID = existing.ID
				break
			}
		}
		if projectID == "" {
			created, err := projects.CreateProjectForOwner(project.Details{Name: "Sample home - " + user.PlanID + " preview", Client: "Synthetic demonstration", Location: "Sample only", Description: sampleDescription}, user.ID)
			if err != nil {
				return errors.New("could not create synthetic preview project")
			}
			projectID = created.ID
		}
		if err := registry.bind(ctx, user.ID, projectID); err != nil {
			return errors.New("could not bind synthetic preview project")
		}
	}
	plan, hasPlan := projects.FloorPlanForProject(projectID)
	if !hasPlan {
		content, err := sampleDrawing()
		if err != nil {
			return errors.New("could not prepare synthetic drawing")
		}
		plan, err = projects.SaveFloorPlan(projectID, "synthetic-plan-preview.png", "image/png", content)
		if err != nil {
			return errors.New("could not save synthetic drawing")
		}
	}
	candidate, ready := analyses.LatestForFloorPlan(plan.ID)
	if !ready {
		content, found := projects.GetFile(plan.ID)
		if !found {
			return errors.New("synthetic drawing content is unavailable")
		}
		started, err := analyses.StartWithEngine("demo", plan.ID, content, plan.FileName)
		if err != nil {
			return errors.New("could not start local demo analysis for the preview project")
		}
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			var found bool
			candidate, found = analyses.Get(started.ID)
			if found && candidate.State == analysis.StateReady {
				break
			}
			if found && candidate.State == analysis.StateFailed {
				return errors.New("local demo analysis of the synthetic preview failed")
			}
			select {
			case <-ctx.Done():
				return errors.New("preview analysis setup timed out")
			case <-ticker.C:
			}
		}
	}
	if _, confirmed := analyses.LatestConfirmation(candidate.ID); !confirmed {
		for index := range candidate.Rooms {
			candidate.Rooms[index].Confidence = 1
			candidate.Rooms[index].Uncertainty = ""
		}
		for index := range candidate.Features {
			candidate.Features[index].Disposition = "acknowledged"
		}
		if _, err := analyses.Confirm(candidate.ID, candidate); err != nil {
			return errors.New("could not confirm predefined synthetic fixture dimensions")
		}
	}
	for _, version := range estimates.List(projectID) {
		if version.SourceAnalysisID == candidate.ID {
			return nil
		}
	}
	_, err = estimates.Create(projectID, estimate.Request{AnalysisID: candidate.ID, PlasterUSDPerSquareMeter: 10, PaintUSDPerSquareMeter: 5, FlooringUSDPerSquareMeter: 20, CeilingUSDPerSquareMeter: 8, SkirtingUSDPerMeter: 3, TilingUSDPerSquareMeter: 15, TargetCurrency: "USD", ExchangeRate: 1, ExchangeRateFetchedAt: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return fmt.Errorf("could not create the synthetic %s preview estimate", user.PlanID)
	}
	return nil
}
