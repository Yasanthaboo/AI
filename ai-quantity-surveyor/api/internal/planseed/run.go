package planseed

import (
	"context"
	"database/sql"
	"errors"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/plans"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func Run(ctx context.Context, config Config, projects *project.PostgresService, analyses *analysis.Service, estimates *estimate.Service, planStore *plans.Store) ([]User, error) {
	if !config.Enabled {
		return nil, nil
	}
	if projects == nil || analyses == nil || estimates == nil || planStore == nil {
		return nil, errors.New("plan preview seeding requires PostgreSQL and initialized application stores")
	}
	transaction, err := projects.DB().BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, errors.New("could not acquire preview seed coordination")
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `SELECT pg_advisory_xact_lock(5932446431720001)`); err != nil {
		return nil, errors.New("could not lock preview seed coordination")
	}
	users, err := EnsureUsers(ctx, config)
	if err != nil {
		return nil, err
	}
	assignments := make([]plans.ManualAssignment, 0, len(users))
	for _, user := range users {
		assignments = append(assignments, plans.ManualAssignment{UserID: user.ID, PlanID: user.PlanID})
	}
	if err := planStore.AssignManualBatch(ctx, assignments); err != nil {
		return nil, errors.New("could not persist preview account plans")
	}
	registry, err := newWorkspaceRegistry(projects.DB())
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if err := ensureSample(ctx, user, projects, analyses, estimates, registry); err != nil {
			return nil, err
		}
	}
	if err := transaction.Commit(); err != nil {
		return nil, errors.New("preview seed coordination ended unexpectedly; rerun safely")
	}
	return users, nil
}
