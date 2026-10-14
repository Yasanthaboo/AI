package analysis

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresJobPersistenceAndQuota(t *testing.T) {
	connection := os.Getenv("SURVEYOR_JOB_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("disposable job test database not configured")
	}
	parsed, err := url.Parse(connection)
	if err != nil || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "surveyor_job_test") {
		t.Fatal("job tests require a disposable surveyor_job_test database")
	}
	db, err := sql.Open("pgx", connection)
	if err != nil {
		t.Fatal("could not open disposable test database")
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	repository, err := NewPostgresRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id, scope := newID(), newID()
	control := JobControl{OwnerID: newID(), AttemptToken: newID()}
	if err := repository.CreateJob(ctx, Candidate{ID: id, FloorPlanID: "test-drawing", State: StateAnalyzing}, control); err != nil {
		t.Fatal(err)
	}
	page, err := NewPagePreparer().Prepare(ctx, pageImage(t, 20, 10), "plan.png", 1, "image")
	if err != nil {
		t.Fatal(err)
	}
	page.Source.FloorPlanID = "test-drawing"
	if err := repository.PublishSource(ctx, id, control, page); err != nil {
		t.Fatal(err)
	}
	if err := repository.PublishSource(ctx, id, control, page); err == nil {
		t.Fatal("PostgreSQL source overwritten")
	}
	var accepted atomic.Int32
	var workers sync.WaitGroup
	errorsFound := make(chan error, 12)
	for index := 0; index < 12; index++ {
		workers.Add(1)
		go func(attempt int) {
			defer workers.Done()
			err := repository.ReserveCall(ctx, id, control, scope, "gemini", fmt.Sprintf("attempt-%d", attempt), 3)
			if err == nil {
				accepted.Add(1)
				return
			}
			var failure *RuntimeError
			if !errors.As(err, &failure) || failure.Failure.ErrorCode != "ProviderQuotaExceeded" {
				errorsFound <- err
			}
		}(index)
	}
	workers.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	if accepted.Load() != 3 {
		t.Fatalf("quota accepted %d calls", accepted.Load())
	}
	if err := repository.RecoverExpired(ctx); err != nil {
		t.Fatal(err)
	}
	current, found, err := repository.ReadJob(ctx, id)
	if err != nil || !found || current.State != StateAnalyzing {
		t.Fatalf("live job recovered: %+v %v", current, err)
	}
	if _, err := db.Exec(`UPDATE analysis_candidates SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecoverExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repository.FinishJob(ctx, Candidate{ID: id, State: StateReady}, control); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("late completion accepted: %v", err)
	}
	reopened, err := sql.Open("pgx", connection)
	if err != nil {
		t.Fatal("could not reopen disposable test database")
	}
	defer reopened.Close()
	again, err := NewPostgresRepository(reopened)
	if err != nil {
		t.Fatal(err)
	}
	stored, found, err := again.ReadSource(ctx, id)
	if err != nil || !found || stored.Source.ImageDigest != page.Source.ImageDigest {
		t.Fatalf("source not durable: %v", err)
	}
	current, found, err = again.ReadJob(ctx, id)
	if err != nil || !found || current.State != StateFailed || current.Error == nil || !current.Error.Retryable {
		t.Fatalf("recovery not durable: %+v %v", current, err)
	}
}
