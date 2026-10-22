package plans

import (
	"context"
	"database/sql"
	"errors"
	"sync"
)

type Feature string

const (
	AIAnalysis   Feature = "aiAnalysis"
	Model3D      Feature = "model3d"
	Materials    Feature = "materials"
	Exports      Feature = "exports"
	DesignStudio Feature = "designStudio"
)

type Plan struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PriceLabel  string    `json:"priceLabel"`
	Description string    `json:"description"`
	Features    []Feature `json:"features"`
}

func Catalog() []Plan {
	return []Plan{
		{ID: "free", Name: "Free", PriceLabel: "Free", Description: "Explore the estimating workflow with sample analysis.", Features: []Feature{}},
		{ID: "pro", Name: "Pro", PriceLabel: "Coming soon", Description: "Analyze drawings and prepare material budgets and exports.", Features: []Feature{AIAnalysis, Model3D, Materials, Exports}},
		{ID: "studio", Name: "Studio", PriceLabel: "Coming soon", Description: "Everything in Pro, plus room interior concepts.", Features: []Feature{AIAnalysis, Model3D, Materials, Exports, DesignStudio}},
	}
}

func Find(id string) (Plan, bool) {
	for _, plan := range Catalog() {
		if plan.ID == id {
			return plan, true
		}
	}
	return Plan{}, false
}

func (plan Plan) Allows(feature Feature) bool {
	for _, available := range plan.Features {
		if available == feature {
			return true
		}
	}
	return false
}

type Assignment struct {
	PlanID string
	Source string
}

type ManualAssignment struct {
	UserID string
	PlanID string
}

type Account struct {
	Plan                 Plan   `json:"plan"`
	AssignmentSource     string `json:"assignmentSource"`
	DemoSwitchingEnabled bool   `json:"demoSwitchingEnabled"`
}

type Store struct {
	db          *sql.DB
	mutex       sync.Mutex
	assignments map[string]Assignment
}

func NewStore(db *sql.DB) (*Store, error) {
	store := &Store{db: db, assignments: make(map[string]Assignment)}
	if db != nil {
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS account_plans (
 user_id TEXT PRIMARY KEY,
 plan_id TEXT NOT NULL CHECK (plan_id IN ('free', 'pro', 'studio')),
 assignment_source TEXT NOT NULL CHECK (assignment_source IN ('demo', 'manual')),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`)
		if err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (store *Store) Account(ctx context.Context, userID string, demo bool) (Account, error) {
	if userID == "" {
		return Account{}, errors.New("account owner is required")
	}
	var assignment Assignment
	if store.db != nil {
		err := store.db.QueryRowContext(ctx, `SELECT plan_id, assignment_source FROM account_plans WHERE user_id=$1`, userID).Scan(&assignment.PlanID, &assignment.Source)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Account{}, err
		}
	} else {
		store.mutex.Lock()
		assignment = store.assignments[userID]
		store.mutex.Unlock()
	}
	plan, valid := Find(assignment.PlanID)
	if !valid || (assignment.Source == "demo" && !demo) {
		plan, _ = Find("free")
	}
	if assignment.Source == "" {
		assignment.Source = "default"
	}
	return Account{Plan: plan, AssignmentSource: assignment.Source, DemoSwitchingEnabled: demo}, nil
}

func (store *Store) SwitchDemo(ctx context.Context, userID, planID string, demo bool) (Account, error) {
	if !demo {
		return Account{}, errors.New("self-service plan changes are disabled")
	}
	if userID == "" {
		return Account{}, errors.New("account owner is required")
	}
	if _, ok := Find(planID); !ok {
		return Account{}, errors.New("unknown plan")
	}
	if store.db != nil {
		_, err := store.db.ExecContext(ctx, `INSERT INTO account_plans (user_id, plan_id, assignment_source) VALUES ($1,$2,'demo')
ON CONFLICT (user_id) DO UPDATE SET plan_id=EXCLUDED.plan_id, assignment_source='demo', updated_at=NOW()`, userID, planID)
		if err != nil {
			return Account{}, err
		}
	} else {
		store.mutex.Lock()
		store.assignments[userID] = Assignment{PlanID: planID, Source: "demo"}
		store.mutex.Unlock()
	}
	return store.Account(ctx, userID, demo)
}

func (store *Store) AssignManualBatch(ctx context.Context, assignments []ManualAssignment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(assignments) == 0 {
		return errors.New("at least one account assignment is required")
	}
	seen := make(map[string]bool)
	for _, assignment := range assignments {
		if assignment.UserID == "" || seen[assignment.UserID] {
			return errors.New("account assignments must have distinct, nonempty owners")
		}
		if _, valid := Find(assignment.PlanID); !valid {
			return errors.New("unknown plan")
		}
		seen[assignment.UserID] = true
	}
	if store.db != nil {
		transaction, err := store.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer transaction.Rollback()
		for _, assignment := range assignments {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO account_plans (user_id,plan_id,assignment_source) VALUES ($1,$2,'manual')
ON CONFLICT (user_id) DO UPDATE SET plan_id=EXCLUDED.plan_id, assignment_source='manual', updated_at=NOW()`, assignment.UserID, assignment.PlanID); err != nil {
				return err
			}
		}
		return transaction.Commit()
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	for _, assignment := range assignments {
		store.assignments[assignment.UserID] = Assignment{PlanID: assignment.PlanID, Source: "manual"}
	}
	return nil
}
