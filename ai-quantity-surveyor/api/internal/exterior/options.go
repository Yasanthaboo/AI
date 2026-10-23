package exterior

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"
)

const Assumptions = "Conceptual rectangular single-storey envelope. Envelope dimensions, wall height, roof pitch and overhang are reviewed visualization assumptions, not verified elevations. Openings, additional levels, drainage, structure and code compliance are not modeled. Roof finish is appearance only. Quantities and estimates are unchanged."

type Input struct {
	AnalysisID           string  `json:"analysisId"`
	ConfirmationRevision int     `json:"confirmationRevision"`
	Roof                 string  `json:"roof"`
	PitchDegrees         float64 `json:"pitchDegrees"`
	OverhangM            float64 `json:"overhangM"`
	LengthM              float64 `json:"lengthM"`
	WidthM               float64 `json:"widthM"`
	WallHeightM          float64 `json:"wallHeightM"`
	Finish               string  `json:"finish"`
	AssumptionsReviewed  bool    `json:"assumptionsReviewed"`
}

func (input Input) Validate() error {
	bounded := func(value, minimum, maximum float64) bool {
		return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
	}
	if input.AnalysisID == "" || input.ConfirmationRevision < 1 || !input.AssumptionsReviewed {
		return errors.New("a confirmed revision and reviewed exterior assumptions are required")
	}
	if input.Roof != "flat" && input.Roof != "gable" && input.Roof != "hip" && input.Roof != "shed" {
		return errors.New("choose flat, gable, hip or shed")
	}
	if !bounded(input.LengthM, 1, 100) || !bounded(input.WidthM, 1, 100) || !bounded(input.WallHeightM, 1, 12) || !bounded(input.OverhangM, 0, 2) {
		return errors.New("envelope length/width must be 1-100 m, wall height 1-12 m and overhang 0-2 m")
	}
	if (input.Roof == "flat" && input.PitchDegrees != 0) || (input.Roof != "flat" && !bounded(input.PitchDegrees, 5, 60)) {
		return errors.New("flat roofs use zero schematic pitch; other roofs require 5-60 degrees")
	}
	if input.Finish != "metal" && input.Finish != "tile" && input.Finish != "membrane" {
		return errors.New("choose a supported roof appearance")
	}
	return nil
}

type Option struct {
	Input
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Assumptions string    `json:"assumptions"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Store struct {
	db      *sql.DB
	mutex   sync.Mutex
	options []Option
}

func NewStore(db *sql.DB) (*Store, error) {
	if db != nil {
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS exterior_options (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id), analysis_id TEXT NOT NULL,
 confirmation_revision INTEGER NOT NULL CHECK (confirmation_revision > 0), payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL);
CREATE INDEX IF NOT EXISTS exterior_options_revision ON exterior_options(project_id,analysis_id,confirmation_revision,created_at DESC);`)
		if err != nil {
			return nil, err
		}
	}
	return &Store{db: db}, nil
}

func (store *Store) Save(ctx context.Context, projectID string, input Input) (Option, error) {
	if err := input.Validate(); err != nil {
		return Option{}, err
	}
	if projectID == "" {
		return Option{}, errors.New("project is required")
	}
	if err := ctx.Err(); err != nil {
		return Option{}, err
	}
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return Option{}, err
	}
	option := Option{Input: input, ID: hex.EncodeToString(identifier), ProjectID: projectID, Assumptions: Assumptions, CreatedAt: time.Now().UTC()}
	if store.db != nil {
		payload, err := json.Marshal(option)
		if err != nil {
			return Option{}, err
		}
		_, err = store.db.ExecContext(ctx, `INSERT INTO exterior_options(id,project_id,analysis_id,confirmation_revision,payload,created_at) VALUES($1,$2,$3,$4,$5,$6)`, option.ID, projectID, input.AnalysisID, input.ConfirmationRevision, payload, option.CreatedAt)
		if err != nil {
			return Option{}, err
		}
	} else {
		store.mutex.Lock()
		store.options = append(store.options, option)
		store.mutex.Unlock()
	}
	return option, nil
}

func (store *Store) List(ctx context.Context, projectID, analysisID string, revision int) ([]Option, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make([]Option, 0)
	if store.db != nil {
		rows, err := store.db.QueryContext(ctx, `SELECT payload FROM exterior_options WHERE project_id=$1 AND analysis_id=$2 AND confirmation_revision=$3 ORDER BY created_at DESC LIMIT 100`, projectID, analysisID, revision)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var payload []byte
			var option Option
			if err := rows.Scan(&payload); err != nil {
				return nil, err
			}
			if err := json.Unmarshal(payload, &option); err != nil {
				return nil, err
			}
			result = append(result, option)
		}
		return result, rows.Err()
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	for index := len(store.options) - 1; index >= 0 && len(result) < 100; index-- {
		option := store.options[index]
		if option.ProjectID == projectID && option.AnalysisID == analysisID && option.ConfirmationRevision == revision {
			result = append(result, option)
		}
	}
	return result, nil
}
