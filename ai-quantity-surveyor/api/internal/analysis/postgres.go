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
	return repository, err
}

func (repository *PostgresRepository) SaveConfirmation(snapshot Confirmation) error {
	payload, err := json.Marshal(snapshot.Candidate)
	if err != nil {
		return err
	}
	_, err = repository.db.Exec(`INSERT INTO analysis_confirmations (analysis_id, revision, payload) VALUES ($1, $2, $3)`, snapshot.AnalysisID, snapshot.Revision, payload)
	return err
}

func (repository *PostgresRepository) LatestConfirmation(id string) (Confirmation, bool) {
	return repository.confirmation(id, `SELECT revision, payload FROM analysis_confirmations WHERE analysis_id = $1 ORDER BY revision DESC LIMIT 1`)
}

func (repository *PostgresRepository) ConfirmationRevision(id string, revision int) (Confirmation, bool) {
	return repository.confirmation(id, `SELECT revision, payload FROM analysis_confirmations WHERE analysis_id = $1 AND revision = $2`, revision)
}

func (repository *PostgresRepository) confirmation(id, query string, args ...any) (Confirmation, bool) {
	var revision int
	var payload []byte
	values := append([]any{id}, args...)
	if repository.db.QueryRow(query, values...).Scan(&revision, &payload) != nil {
		return Confirmation{}, false
	}
	var candidate Candidate
	if json.Unmarshal(payload, &candidate) != nil {
		return Confirmation{}, false
	}
	return Confirmation{AnalysisID: id, Revision: revision, Candidate: candidate}, true
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
