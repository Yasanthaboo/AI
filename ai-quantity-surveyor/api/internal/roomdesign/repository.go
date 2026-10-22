package roomdesign

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"
)

type storedConcept struct {
	concept Concept
	image   []byte
}

type MemoryRepository struct {
	mu       sync.Mutex
	concepts map[string]storedConcept
	usage    map[string]int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{concepts: make(map[string]storedConcept), usage: make(map[string]int)}
}

func (repository *MemoryRepository) ReserveDaily(ctx context.Context, ownerID string, at time.Time, count, limit int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	key := ownerID + ":" + at.UTC().Format("2006-01-02")
	if repository.usage[key]+count > limit {
		return ErrDailyLimit
	}
	repository.usage[key] += count
	return nil
}

func (repository *MemoryRepository) SaveBatch(ctx context.Context, concepts []Concept, images [][]byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(concepts) == 0 || len(concepts) != len(images) {
		return errors.New("invalid design batch")
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, concept := range concepts {
		if _, exists := repository.concepts[concept.ID]; exists {
			return errors.New("design id already exists")
		}
	}
	for index, concept := range concepts {
		repository.concepts[concept.ID] = storedConcept{concept: concept, image: append([]byte(nil), images[index]...)}
	}
	return nil
}

func (repository *MemoryRepository) List(ctx context.Context, projectID, roomID string, revision int) ([]Concept, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]Concept, 0)
	for _, item := range repository.concepts {
		if item.concept.ProjectID == projectID && item.concept.RoomID == roomID && item.concept.ConfirmationRevision == revision {
			result = append(result, item.concept)
		}
	}
	sort.Slice(result, func(first, second int) bool { return result[first].CreatedAt.After(result[second].CreatedAt) })
	return result, nil
}

func (repository *MemoryRepository) Get(ctx context.Context, id string) (Concept, bool, error) {
	if err := ctx.Err(); err != nil {
		return Concept{}, false, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	item, found := repository.concepts[id]
	return item.concept, found, nil
}

func (repository *MemoryRepository) Image(ctx context.Context, id string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	item, found := repository.concepts[id]
	return append([]byte(nil), item.image...), found, nil
}

func (repository *MemoryRepository) Select(ctx context.Context, projectID, roomID string, revision int, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	selected, found := repository.concepts[id]
	if !found || selected.concept.ProjectID != projectID || selected.concept.RoomID != roomID || selected.concept.ConfirmationRevision != revision {
		return ErrNotFound
	}
	for conceptID, item := range repository.concepts {
		if item.concept.ProjectID == projectID && item.concept.RoomID == roomID && item.concept.ConfirmationRevision == revision {
			item.concept.Selected = conceptID == id
			repository.concepts[conceptID] = item
		}
	}
	return nil
}

func (repository *MemoryRepository) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if _, found := repository.concepts[id]; !found {
		return ErrNotFound
	}
	delete(repository.concepts, id)
	return nil
}

type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS room_design_concepts (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id), room_id TEXT NOT NULL,
 confirmation_revision INTEGER NOT NULL CHECK (confirmation_revision > 0), selected BOOLEAN NOT NULL DEFAULT FALSE,
 payload JSONB NOT NULL, image BYTEA NOT NULL, created_at TIMESTAMPTZ NOT NULL);
CREATE INDEX IF NOT EXISTS room_design_concepts_room_revision ON room_design_concepts(project_id, room_id, confirmation_revision, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS room_design_concepts_one_selected ON room_design_concepts(project_id, room_id, confirmation_revision) WHERE selected;
CREATE TABLE IF NOT EXISTS room_design_daily_usage (
 user_id TEXT NOT NULL, usage_date DATE NOT NULL, image_count INTEGER NOT NULL CHECK (image_count >= 0), PRIMARY KEY(user_id, usage_date));`)
	if err != nil {
		return nil, err
	}
	return &PostgresRepository{db: db}, nil
}

func (repository *PostgresRepository) ReserveDaily(ctx context.Context, ownerID string, at time.Time, count, limit int) error {
	var countAfter int
	err := repository.db.QueryRowContext(ctx, `INSERT INTO room_design_daily_usage(user_id, usage_date, image_count)
VALUES($1, $2, $3)
ON CONFLICT(user_id, usage_date) DO UPDATE SET image_count = room_design_daily_usage.image_count + EXCLUDED.image_count
WHERE room_design_daily_usage.image_count + EXCLUDED.image_count <= $4
RETURNING image_count`, ownerID, at.UTC().Format("2006-01-02"), count, limit).Scan(&countAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDailyLimit
	}
	return err
}

func (repository *PostgresRepository) SaveBatch(ctx context.Context, concepts []Concept, images [][]byte) error {
	if len(concepts) == 0 || len(concepts) != len(images) {
		return errors.New("invalid design batch")
	}
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for index, concept := range concepts {
		payload, err := json.Marshal(concept)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO room_design_concepts(id,project_id,room_id,confirmation_revision,selected,payload,image,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, concept.ID, concept.ProjectID, concept.RoomID, concept.ConfirmationRevision, concept.Selected, payload, images[index], concept.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (repository *PostgresRepository) List(ctx context.Context, projectID, roomID string, revision int) ([]Concept, error) {
	rows, err := repository.db.QueryContext(ctx, `SELECT payload, selected FROM room_design_concepts WHERE project_id=$1 AND room_id=$2 AND confirmation_revision=$3 ORDER BY created_at DESC`, projectID, roomID, revision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Concept, 0)
	for rows.Next() {
		var payload []byte
		var concept Concept
		if err := rows.Scan(&payload, &concept.Selected); err != nil {
			return nil, err
		}
		selected := concept.Selected
		if err := json.Unmarshal(payload, &concept); err != nil {
			return nil, err
		}
		concept.Selected = selected
		result = append(result, concept)
	}
	return result, rows.Err()
}

func (repository *PostgresRepository) Get(ctx context.Context, id string) (Concept, bool, error) {
	var payload []byte
	var selected bool
	err := repository.db.QueryRowContext(ctx, `SELECT payload, selected FROM room_design_concepts WHERE id=$1`, id).Scan(&payload, &selected)
	if errors.Is(err, sql.ErrNoRows) {
		return Concept{}, false, nil
	}
	if err != nil {
		return Concept{}, false, err
	}
	var concept Concept
	err = json.Unmarshal(payload, &concept)
	concept.Selected = selected
	return concept, err == nil, err
}

func (repository *PostgresRepository) Image(ctx context.Context, id string) ([]byte, bool, error) {
	var image []byte
	err := repository.db.QueryRowContext(ctx, `SELECT image FROM room_design_concepts WHERE id=$1`, id).Scan(&image)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return image, err == nil, err
}

func (repository *PostgresRepository) Select(ctx context.Context, projectID, roomID string, revision int, id string) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, projectID+":"+roomID+":"+strconv.Itoa(revision)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE room_design_concepts SET selected=FALSE, payload=jsonb_set(payload,'{selected}','false'::jsonb) WHERE project_id=$1 AND room_id=$2 AND confirmation_revision=$3`, projectID, roomID, revision); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE room_design_concepts SET selected=TRUE, payload=jsonb_set(payload,'{selected}','true'::jsonb) WHERE id=$1 AND project_id=$2 AND room_id=$3 AND confirmation_revision=$4`, id, projectID, roomID, revision)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (repository *PostgresRepository) Delete(ctx context.Context, id string) error {
	result, err := repository.db.ExecContext(ctx, `DELETE FROM room_design_concepts WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return ErrNotFound
	}
	return nil
}
