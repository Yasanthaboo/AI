package materials

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) (*PostgresRepository, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS material_budget_versions (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id), estimate_id TEXT NOT NULL REFERENCES estimate_versions(id),
 version INTEGER NOT NULL CHECK(version>0), payload JSONB NOT NULL, UNIQUE(project_id,version));
CREATE TABLE IF NOT EXISTS material_comparison_versions (
 id TEXT PRIMARY KEY, budget_id TEXT NOT NULL REFERENCES material_budget_versions(id), version INTEGER NOT NULL CHECK(version>0), payload JSONB NOT NULL, UNIQUE(budget_id,version));
CREATE TABLE IF NOT EXISTS material_price_observations (id TEXT PRIMARY KEY, payload JSONB NOT NULL);`)
	if err == nil {
		for _, offer := range fixtureOffers() {
			payload, encodeErr := json.Marshal(offer)
			if encodeErr != nil {
				return nil, encodeErr
			}
			if _, err = db.Exec(`INSERT INTO material_price_observations(id,payload) VALUES($1,$2) ON CONFLICT(id) DO NOTHING`, offer.ID, payload); err != nil {
				break
			}
		}
	}
	return &PostgresRepository{db: db}, err
}
func (repository *PostgresRepository) SaveBudget(ctx context.Context, budget Budget) (Budget, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return Budget{}, err
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "material-budget:"+budget.ProjectID); err != nil {
		return Budget{}, err
	}
	if err := transaction.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM material_budget_versions WHERE project_id=$1`, budget.ProjectID).Scan(&budget.Version); err != nil {
		return Budget{}, err
	}
	payload, err := json.Marshal(budget)
	if err != nil {
		return Budget{}, err
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO material_budget_versions(id,project_id,estimate_id,version,payload) VALUES($1,$2,$3,$4,$5)`, budget.ID, budget.ProjectID, budget.EstimateID, budget.Version, payload); err != nil {
		return Budget{}, err
	}
	if err := transaction.Commit(); err != nil {
		return Budget{}, err
	}
	return clone(budget)
}
func (repository *PostgresRepository) GetBudget(ctx context.Context, id string) (Budget, bool, error) {
	var payload []byte
	err := repository.db.QueryRowContext(ctx, `SELECT payload FROM material_budget_versions WHERE id=$1`, id).Scan(&payload)
	if err == sql.ErrNoRows {
		return Budget{}, false, nil
	}
	if err != nil {
		return Budget{}, false, err
	}
	var result Budget
	err = json.Unmarshal(payload, &result)
	return result, err == nil, err
}
func (repository *PostgresRepository) ListBudgets(ctx context.Context, projectID string) ([]Budget, error) {
	rows, err := repository.db.QueryContext(ctx, `SELECT payload FROM material_budget_versions WHERE project_id=$1 ORDER BY version DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Budget, 0)
	for rows.Next() {
		var payload []byte
		var budget Budget
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &budget); err != nil {
			return nil, err
		}
		result = append(result, budget)
	}
	return result, rows.Err()
}

func listStoredOffers(ctx context.Context, query interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) ([]Offer, error) {
	rows, err := query.QueryContext(ctx, `SELECT payload FROM material_price_observations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Offer, 0)
	for rows.Next() {
		var payload []byte
		var value Offer
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (repository *PostgresRepository) ListOffers(ctx context.Context) ([]Offer, error) {
	return listStoredOffers(ctx, repository.db)
}
func (repository *PostgresRepository) SaveComparison(ctx context.Context, value Comparison) (Comparison, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return Comparison{}, err
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "material-comparison:"+value.BudgetID); err != nil {
		return Comparison{}, err
	}
	var budget Budget
	var budgetPayload []byte
	if err := transaction.QueryRowContext(ctx, `SELECT payload FROM material_budget_versions WHERE id=$1 FOR SHARE`, value.BudgetID).Scan(&budgetPayload); err != nil {
		return Comparison{}, err
	}
	if err := json.Unmarshal(budgetPayload, &budget); err != nil {
		return Comparison{}, err
	}
	offers, err := listStoredOffers(ctx, transaction)
	if err != nil {
		return Comparison{}, err
	}
	value.Basket, err = priceBasket(budget, offers, selectionsFrom(value), value.Conversion, time.Now().UTC(), true)
	if err != nil {
		return Comparison{}, err
	}
	if err := transaction.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM material_comparison_versions WHERE budget_id=$1`, value.BudgetID).Scan(&value.Version); err != nil {
		return Comparison{}, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return Comparison{}, err
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO material_comparison_versions(id,budget_id,version,payload) VALUES($1,$2,$3,$4)`, value.ID, value.BudgetID, value.Version, payload); err != nil {
		return Comparison{}, err
	}
	if err := transaction.Commit(); err != nil {
		return Comparison{}, err
	}
	return clone(value)
}
func (repository *PostgresRepository) GetComparison(ctx context.Context, id string) (Comparison, bool, error) {
	var payload []byte
	err := repository.db.QueryRowContext(ctx, `SELECT payload FROM material_comparison_versions WHERE id=$1`, id).Scan(&payload)
	if err == sql.ErrNoRows {
		return Comparison{}, false, nil
	}
	if err != nil {
		return Comparison{}, false, err
	}
	var result Comparison
	err = json.Unmarshal(payload, &result)
	return result, err == nil, err
}
