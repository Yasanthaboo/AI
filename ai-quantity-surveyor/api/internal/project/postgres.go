package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresService struct{ db *sql.DB }

func (service *PostgresService) DB() *sql.DB { return service.db }

func NewPostgresService(databaseURL string) (*PostgresService, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	service := &PostgresService{db: db}
	if err := service.ensureSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return service, nil
}

func (service *PostgresService) ensureSchema(ctx context.Context) error {
	_, err := service.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS projects (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, status TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS floor_plans (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL UNIQUE REFERENCES projects(id), file_name TEXT NOT NULL,
 content_type TEXT NOT NULL, file_size BIGINT NOT NULL, uploaded_at TIMESTAMPTZ NOT NULL, content BYTEA NOT NULL
);
ALTER TABLE projects ADD COLUMN IF NOT EXISTS client TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS location TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS archived BOOLEAN NOT NULL DEFAULT FALSE;`)
	if err != nil {
		return fmt.Errorf("ensure project schema: %w", err)
	}
	return nil
}

func (service *PostgresService) CreateProject(details Details) (Project, error) {
	project, err := details.project()
	if err != nil {
		return Project{}, err
	}
	_, err = service.db.Exec(`INSERT INTO projects (id, name, client, location, description, created_at, status) VALUES ($1, $2, $3, $4, $5, $6, $7)`, project.ID, project.Name, project.Client, project.Location, project.Description, project.CreatedAt, project.Status)
	return project, err
}

const projectColumns = `id, name, client, location, description, created_at, status, archived`

func scanProject(row interface{ Scan(...any) error }) (Project, error) {
	var project Project
	err := row.Scan(&project.ID, &project.Name, &project.Client, &project.Location, &project.Description, &project.CreatedAt, &project.Status, &project.Archived)
	return project, err
}

func (service *PostgresService) GetProject(id string) (Project, bool) {
	project, err := scanProject(service.db.QueryRow(`SELECT `+projectColumns+` FROM projects WHERE id = $1`, id))
	return project, err == nil
}

func (service *PostgresService) ListProjects() []Project {
	rows, err := service.db.Query(`SELECT ` + projectColumns + ` FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		if project, err := scanProject(rows); err == nil {
			projects = append(projects, project)
		}
	}
	return projects
}

func (service *PostgresService) SetArchived(id string, archived bool) (Project, error) {
	result, err := service.db.Exec(`UPDATE projects SET archived = $2 WHERE id = $1`, id, archived)
	if err != nil {
		return Project{}, err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return Project{}, fmt.Errorf("project %q not found", id)
	}
	project, _ := service.GetProject(id)
	return project, nil
}

func (service *PostgresService) FloorPlanForProject(projectID string) (FloorPlan, bool) {
	var floorPlan FloorPlan
	err := service.db.QueryRow(`SELECT id, project_id, file_name, content_type, file_size, uploaded_at FROM floor_plans WHERE project_id = $1`, projectID).Scan(&floorPlan.ID, &floorPlan.ProjectID, &floorPlan.FileName, &floorPlan.ContentType, &floorPlan.FileSize, &floorPlan.UploadedAt)
	return floorPlan, err == nil
}

func (service *PostgresService) SaveFloorPlan(projectID, fileName, contentType string, content []byte) (FloorPlan, error) {
	if int64(len(content)) > MaxFloorPlanSize {
		return FloorPlan{}, errors.New("floor plan exceeds the 10 MB limit")
	}
	if !isSupportedFile(fileName, contentType, content) {
		return FloorPlan{}, errors.New("floor plan must be a PDF, PNG, JPG, or JPEG")
	}
	if _, ok := service.GetProject(projectID); !ok {
		return FloorPlan{}, fmt.Errorf("project %q not found", projectID)
	}
	floorPlan := FloorPlan{ID: newID(), ProjectID: projectID, FileName: filepath.Base(fileName), ContentType: contentType, FileSize: int64(len(content)), UploadedAt: time.Now().UTC()}
	_, err := service.db.Exec(`INSERT INTO floor_plans (id, project_id, file_name, content_type, file_size, uploaded_at, content) VALUES ($1, $2, $3, $4, $5, $6, $7)`, floorPlan.ID, floorPlan.ProjectID, floorPlan.FileName, floorPlan.ContentType, floorPlan.FileSize, floorPlan.UploadedAt, content)
	if err != nil {
		return FloorPlan{}, fmt.Errorf("save floor plan: %w", err)
	}
	_, err = service.db.Exec(`UPDATE projects SET status = 'Uploaded' WHERE id = $1`, projectID)
	return floorPlan, err
}

func (service *PostgresService) GetFloorPlan(id string) (FloorPlan, bool) {
	var floorPlan FloorPlan
	err := service.db.QueryRow(`SELECT id, project_id, file_name, content_type, file_size, uploaded_at FROM floor_plans WHERE id = $1`, id).Scan(&floorPlan.ID, &floorPlan.ProjectID, &floorPlan.FileName, &floorPlan.ContentType, &floorPlan.FileSize, &floorPlan.UploadedAt)
	return floorPlan, err == nil
}

func (service *PostgresService) GetFile(id string) ([]byte, bool) {
	var content []byte
	err := service.db.QueryRow(`SELECT content FROM floor_plans WHERE id = $1`, id).Scan(&content)
	return content, err == nil
}

func (service *PostgresService) Close() error { return service.db.Close() }
