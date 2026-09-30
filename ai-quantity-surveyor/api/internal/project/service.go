package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const MaxFloorPlanSize int64 = 10 * 1024 * 1024

var supportedTypes = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
}

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Client      string    `json:"client,omitempty"`
	Location    string    `json:"location,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	Status      string    `json:"status"`
}

type Details struct {
	Name        string `json:"name"`
	Client      string `json:"client"`
	Location    string `json:"location"`
	Description string `json:"description"`
}

func (details Details) project() (Project, error) {
	project := Project{Name: strings.TrimSpace(details.Name), Client: strings.TrimSpace(details.Client), Location: strings.TrimSpace(details.Location), Description: strings.TrimSpace(details.Description)}
	if project.Name == "" {
		return Project{}, errors.New("project name is required")
	}
	if len(project.Name) > 120 || len(project.Client) > 120 || len(project.Location) > 200 || len(project.Description) > 2000 {
		return Project{}, errors.New("project details are too long")
	}
	project.ID = newID()
	project.CreatedAt = time.Now().UTC()
	project.Status = "Created"
	return project, nil
}

type FloorPlan struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	FileSize    int64     `json:"fileSize"`
	UploadedAt  time.Time `json:"uploadedAt"`
}

type Service struct {
	mu              sync.RWMutex
	projects        map[string]Project
	floorPlans      map[string]FloorPlan
	files           map[string][]byte
	persistencePath string
}

type Store interface {
	CreateProject(Details) (Project, error)
	GetProject(string) (Project, bool)
	SaveFloorPlan(string, string, string, []byte) (FloorPlan, error)
	GetFloorPlan(string) (FloorPlan, bool)
	GetFile(string) ([]byte, bool)
}

func NewService() *Service {
	return &Service{projects: map[string]Project{}, floorPlans: map[string]FloorPlan{}, files: map[string][]byte{}}
}

func NewPersistentService(path string) (*Service, error) {
	service := NewService()
	service.persistencePath = path
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return service, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project store: %w", err)
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode project store: %w", err)
	}
	service.projects = state.Projects
	service.floorPlans = state.FloorPlans
	service.files = state.Files
	if service.projects == nil {
		service.projects = map[string]Project{}
	}
	if service.floorPlans == nil {
		service.floorPlans = map[string]FloorPlan{}
	}
	if service.files == nil {
		service.files = map[string][]byte{}
	}
	return service, nil
}

type persistedState struct {
	Projects   map[string]Project   `json:"projects"`
	FloorPlans map[string]FloorPlan `json:"floorPlans"`
	Files      map[string][]byte    `json:"files"`
}

func (service *Service) CreateProject(details Details) (Project, error) {
	project, err := details.project()
	if err != nil {
		return Project{}, err
	}
	service.mu.Lock()
	service.projects[project.ID] = project
	err = service.persistLocked()
	service.mu.Unlock()
	if err != nil {
		return Project{}, err
	}
	return project, nil
}

func (service *Service) GetProject(id string) (Project, bool) {
	service.mu.RLock()
	project, ok := service.projects[id]
	service.mu.RUnlock()
	return project, ok
}

func (service *Service) SaveFloorPlan(projectID, fileName, contentType string, content []byte) (FloorPlan, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	project, ok := service.projects[projectID]
	if !ok {
		return FloorPlan{}, fmt.Errorf("project %q not found", projectID)
	}
	for _, existing := range service.floorPlans {
		if existing.ProjectID == projectID {
			return FloorPlan{}, errors.New("project already has a floor plan")
		}
	}
	if int64(len(content)) > MaxFloorPlanSize {
		return FloorPlan{}, errors.New("floor plan exceeds the 10 MB limit")
	}
	if !isSupportedFile(fileName, contentType, content) {
		return FloorPlan{}, errors.New("floor plan must be a PDF, PNG, JPG, or JPEG")
	}
	planned := FloorPlan{ID: newID(), ProjectID: projectID, FileName: filepath.Base(fileName), ContentType: contentType, FileSize: int64(len(content)), UploadedAt: time.Now().UTC()}
	service.floorPlans[planned.ID] = planned
	service.files[planned.ID] = append([]byte(nil), content...)
	project.Status = "Uploaded"
	service.projects[projectID] = project
	if err := service.persistLocked(); err != nil {
		return FloorPlan{}, err
	}
	return planned, nil
}

func (service *Service) GetFloorPlan(id string) (FloorPlan, bool) {
	service.mu.RLock()
	floorPlan, ok := service.floorPlans[id]
	service.mu.RUnlock()
	return floorPlan, ok
}

func (service *Service) GetFile(id string) ([]byte, bool) {
	service.mu.RLock()
	content, ok := service.files[id]
	service.mu.RUnlock()
	return append([]byte(nil), content...), ok
}

func (service *Service) persistLocked() error {
	if service.persistencePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(service.persistencePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(persistedState{Projects: service.projects, FloorPlans: service.floorPlans, Files: service.files}, "", "  ")
	if err != nil {
		return err
	}
	temporaryPath := service.persistencePath + ".tmp"
	if err := os.WriteFile(temporaryPath, data, 0o600); err != nil {
		return err
	}
	if err := os.Remove(service.persistencePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(temporaryPath, service.persistencePath)
}

func isSupportedFile(fileName, contentType string, content []byte) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if !supportedTypes[mediaType] {
		mediaType = http.DetectContentType(content)
	}
	if !supportedTypes[mediaType] {
		return false
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	return (mediaType == "application/pdf" && extension == ".pdf") || (mediaType == "image/png" && extension == ".png") || (mediaType == "image/jpeg" && (extension == ".jpg" || extension == ".jpeg"))
}

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
