package analysis

import (
	"database/sql"
	"encoding/json"
)

type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	repository := &PostgresRepository{db: db}
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS analysis_candidates (id TEXT PRIMARY KEY, floor_plan_id TEXT NOT NULL, state TEXT NOT NULL, confirmed BOOLEAN NOT NULL DEFAULT FALSE, payload JSONB NOT NULL)`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS analysis_confirmations (analysis_id TEXT NOT NULL, revision INTEGER NOT NULL, payload JSONB NOT NULL, PRIMARY KEY (analysis_id, revision))`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`INSERT INTO analysis_confirmations (analysis_id, revision, payload)
		SELECT id, 1, payload FROM analysis_candidates WHERE confirmed = TRUE
		ON CONFLICT (analysis_id, revision) DO NOTHING`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`ALTER TABLE analysis_confirmations ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ`)
	return repository, err
}

func (repository *PostgresRepository) Confirmations(id string) []Confirmation {
	rows, err := repository.db.Query(`SELECT revision, confirmed_at, payload FROM analysis_confirmations WHERE analysis_id = $1 ORDER BY revision`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []Confirmation
	for rows.Next() {
		var snapshot Confirmation
		var confirmedAt sql.NullTime
		var payload []byte
		if rows.Scan(&snapshot.Revision, &confirmedAt, &payload) != nil || json.Unmarshal(payload, &snapshot.Candidate) != nil {
			continue
		}
		snapshot.AnalysisID, snapshot.ConfirmedAt = id, confirmedAt.Time
		result = append(result, snapshot)
	}
	if rows.Err() != nil {
		return nil
	}
	return result
}

func (repository *PostgresRepository) LatestForFloorPlan(floorPlanID string) (Candidate, bool) {
	var payload []byte
	if repository.db.QueryRow(`SELECT payload FROM analysis_candidates WHERE floor_plan_id = $1 AND state = 'AnalysisReady' ORDER BY payload->>'startedAt' DESC NULLS LAST LIMIT 1`, floorPlanID).Scan(&payload) != nil {
		return Candidate{}, false
	}
	var candidate Candidate
	if json.Unmarshal(payload, &candidate) != nil {
		return Candidate{}, false
	}
	return candidate, true
}

func (repository *PostgresRepository) SaveConfirmation(snapshot Confirmation) error {
	payload, err := json.Marshal(snapshot.Candidate)
	if err != nil {
		return err
	}
	_, err = repository.db.Exec(`INSERT INTO analysis_confirmations (analysis_id, revision, confirmed_at, payload) VALUES ($1, $2, $3, $4)`, snapshot.AnalysisID, snapshot.Revision, snapshot.ConfirmedAt, payload)
	return err
}

func (repository *PostgresRepository) LatestConfirmation(id string) (Confirmation, bool) {
	return repository.confirmation(id, `SELECT revision, confirmed_at, payload FROM analysis_confirmations WHERE analysis_id = $1 ORDER BY revision DESC LIMIT 1`)
}

func (repository *PostgresRepository) ConfirmationRevision(id string, revision int) (Confirmation, bool) {
	return repository.confirmation(id, `SELECT revision, confirmed_at, payload FROM analysis_confirmations WHERE analysis_id = $1 AND revision = $2`, revision)
}

func (repository *PostgresRepository) confirmation(id, query string, args ...any) (Confirmation, bool) {
	var revision int
	var confirmedAt sql.NullTime
	var payload []byte
	values := append([]any{id}, args...)
	if repository.db.QueryRow(query, values...).Scan(&revision, &confirmedAt, &payload) != nil {
		return Confirmation{}, false
	}
	var candidate Candidate
	if json.Unmarshal(payload, &candidate) != nil {
		return Confirmation{}, false
	}
	return Confirmation{AnalysisID: id, Revision: revision, ConfirmedAt: confirmedAt.Time, Candidate: candidate}, true
}

func (repository *PostgresRepository) Save(candidate Candidate, confirmed bool) error {
	payload, err := json.Marshal(candidate)
	if err != nil {
		return err
	}
	_, err = repository.db.Exec(`INSERT INTO analysis_candidates (id, floor_plan_id, state, confirmed, payload) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO UPDATE SET state = EXCLUDED.state, confirmed = EXCLUDED.confirmed, payload = EXCLUDED.payload`, candidate.ID, candidate.FloorPlanID, candidate.State, confirmed, payload)
	return err
}

func (repository *PostgresRepository) Get(id string) (Candidate, bool) {
	return repository.get(id, false)
}
func (repository *PostgresRepository) Confirmed(id string) (Candidate, bool) {
	if snapshot, ok := repository.LatestConfirmation(id); ok {
		return snapshot.Candidate, true
	}
	return repository.get(id, true)
}

func (repository *PostgresRepository) get(id string, confirmed bool) (Candidate, bool) {
	var payload []byte
	query := `SELECT payload FROM analysis_candidates WHERE id = $1`
	if confirmed {
		query += ` AND confirmed = TRUE`
	}
	if repository.db.QueryRow(query, id).Scan(&payload) != nil {
		return Candidate{}, false
	}
	var candidate Candidate
	if json.Unmarshal(payload, &candidate) != nil {
		return Candidate{}, false
	}
	return candidate, true
}
