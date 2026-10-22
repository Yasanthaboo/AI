package plans

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPlanAssignmentsAreIsolatedAndDemoGrantsExpire(t *testing.T) {
	store, err := NewStore(nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	account, err := store.Account(ctx, "user-a", true)
	if err != nil || account.Plan.ID != "free" || account.Plan.Allows(DesignStudio) {
		t.Fatalf("default account: %+v %v", account, err)
	}
	account, err = store.SwitchDemo(ctx, "user-a", "studio", true)
	if err != nil || !account.Plan.Allows(DesignStudio) {
		t.Fatalf("studio assignment: %+v %v", account, err)
	}
	other, _ := store.Account(ctx, "user-b", true)
	if other.Plan.ID != "free" {
		t.Fatal("plan leaked between accounts")
	}
	account, _ = store.Account(ctx, "user-a", false)
	if account.Plan.ID != "free" {
		t.Fatal("demo upgrade granted production entitlements")
	}
	if _, err := store.SwitchDemo(ctx, "user-a", "studio", false); err == nil {
		t.Fatal("production self-upgrade accepted")
	}
	if _, err := store.SwitchDemo(ctx, "user-a", "invented", true); err == nil {
		t.Fatal("unknown plan accepted")
	}
	if _, err := store.SwitchDemo(ctx, "", "pro", true); err == nil {
		t.Fatal("missing owner accepted")
	}
	account, err = store.SwitchDemo(ctx, "user-a", "pro", true)
	if err != nil || !account.Plan.Allows(Materials) || account.Plan.Allows(DesignStudio) {
		t.Fatalf("pro permissions: %+v %v", account, err)
	}
	account, err = store.SwitchDemo(ctx, "user-a", "free", true)
	if err != nil || account.Plan.Allows(AIAnalysis) {
		t.Fatalf("downgrade: %+v %v", account, err)
	}
	for _, plan := range Catalog() {
		if plan.Allows(Feature("unknown")) {
			t.Fatal("unknown feature allowed")
		}
	}
}

func TestManualPlanBatchIsAtomicAndWorksWithoutDemo(t *testing.T) {
	store, _ := NewStore(nil)
	ctx := context.Background()
	if err := store.AssignManualBatch(ctx, []ManualAssignment{{UserID: "free-user", PlanID: "free"}, {UserID: "pro-user", PlanID: "pro"}, {UserID: "studio-user", PlanID: "studio"}}); err != nil {
		t.Fatal(err)
	}
	for _, assignment := range []ManualAssignment{{UserID: "free-user", PlanID: "free"}, {UserID: "pro-user", PlanID: "pro"}, {UserID: "studio-user", PlanID: "studio"}} {
		account, err := store.Account(ctx, assignment.UserID, false)
		if err != nil || account.Plan.ID != assignment.PlanID || account.AssignmentSource != "manual" {
			t.Fatalf("manual account=%+v error=%v", account, err)
		}
	}
	for _, batch := range [][]ManualAssignment{
		{{UserID: "pro-user", PlanID: "free"}, {UserID: "invalid-user", PlanID: "invalid"}},
		{{UserID: "pro-user", PlanID: "free"}, {UserID: "pro-user", PlanID: "studio"}},
		{{UserID: "", PlanID: "studio"}},
	} {
		if err := store.AssignManualBatch(ctx, batch); err == nil {
			t.Fatal("invalid batch accepted")
		}
		account, _ := store.Account(ctx, "pro-user", false)
		if account.Plan.ID != "pro" {
			t.Fatal("invalid batch partially changed a plan")
		}
	}
}

func TestPlanPostgresPersistence(t *testing.T) {
	databaseURL := os.Getenv("SURVEYOR_PLAN_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SURVEYOR_PLAN_TEST_DATABASE_URL to a disposable plan test database")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || !strings.HasPrefix(strings.TrimPrefix(parsed.Path, "/"), "surveyor_plan_test") {
		t.Fatal("refusing a database outside the surveyor_plan_test prefix")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal("could not open disposable plan database")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	userID := "plan-test-" + time.Now().UTC().Format("20060102150405.000000000")
	defer db.Exec(`DELETE FROM account_plans WHERE user_id=$1`, userID)
	if _, err := store.SwitchDemo(ctx, userID, "studio", true); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	account, err := reopened.Account(ctx, userID, true)
	if err != nil || account.Plan.ID != "studio" {
		t.Fatalf("saved plan not restored: %+v %v", account, err)
	}
	other, err := reopened.Account(ctx, userID+"-other", true)
	if err != nil || other.Plan.ID != "free" {
		t.Fatalf("cross-account plan leak: %+v %v", other, err)
	}
	account, err = reopened.Account(ctx, userID, false)
	if err != nil || account.Plan.ID != "free" {
		t.Fatalf("demo plan survived production switch: %+v %v", account, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE account_plans SET assignment_source='manual' WHERE user_id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	account, err = reopened.Account(ctx, userID, false)
	if err != nil || account.Plan.ID != "studio" {
		t.Fatalf("trusted manual grant unavailable: %+v %v", account, err)
	}
	if _, err := reopened.SwitchDemo(ctx, userID, "free", true); err != nil {
		t.Fatal(err)
	}
	account, err = store.Account(ctx, userID, true)
	if err != nil || account.Plan.ID != "free" {
		t.Fatalf("downgrade not persisted: %+v %v", account, err)
	}
}
