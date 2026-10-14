package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const jobSchema = `
ALTER TABLE analysis_candidates ADD COLUMN IF NOT EXISTS owner_id TEXT;
ALTER TABLE analysis_candidates ADD COLUMN IF NOT EXISTS attempt_token TEXT;
ALTER TABLE analysis_candidates ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;
ALTER TABLE analysis_candidates ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ;
CREATE TABLE IF NOT EXISTS analysis_source_images (
 analysis_id TEXT PRIMARY KEY REFERENCES analysis_candidates(id), source JSONB NOT NULL,
 content BYTEA NOT NULL CHECK(octet_length(content)>0 AND octet_length(content)<=10485760)
);
CREATE TABLE IF NOT EXISTS provider_daily_usage (
 quota_scope TEXT NOT NULL, provider TEXT NOT NULL, utc_date DATE NOT NULL,
 reserved_calls INTEGER NOT NULL CHECK(reserved_calls>=0), daily_limit INTEGER NOT NULL CHECK(daily_limit>0),
 PRIMARY KEY(quota_scope,provider,utc_date)
);
CREATE TABLE IF NOT EXISTS provider_call_reservations (
 id TEXT PRIMARY KEY, quota_scope TEXT NOT NULL, provider TEXT NOT NULL,
 analysis_id TEXT NOT NULL REFERENCES analysis_candidates(id), attempt_id TEXT NOT NULL,
 utc_date DATE NOT NULL, owner_id TEXT NOT NULL, attempt_token TEXT NOT NULL,
 UNIQUE(quota_scope,provider,analysis_id,attempt_id)
);`

func (repository *PostgresRepository) CreateJob(ctx context.Context, candidate Candidate, control JobControl) error {
	if candidate.ID == "" || candidate.FloorPlanID == "" || candidate.State != StateAnalyzing || control.OwnerID == "" || control.AttemptToken == "" {
		return fmt.Errorf("invalid analysis job")
	}
	payload, err := json.Marshal(candidate)
	if err != nil {
		return err
	}
	_, err = repository.db.ExecContext(ctx, `INSERT INTO analysis_candidates(id,floor_plan_id,state,payload,owner_id,attempt_token,heartbeat_at,lease_expires_at)
 VALUES($1,$2,$3,$4,$5,$6,clock_timestamp(),clock_timestamp()+interval '60 seconds')`, candidate.ID, candidate.FloorPlanID, candidate.State, payload, control.OwnerID, control.AttemptToken)
	return err
}

func (repository *PostgresRepository) ReadJob(ctx context.Context, id string) (Candidate, bool, error) {
	var payload []byte
	if err := repository.db.QueryRowContext(ctx, `SELECT payload FROM analysis_candidates WHERE id=$1`, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Candidate{}, false, nil
		}
		return Candidate{}, false, err
	}
	var candidate Candidate
	err := json.Unmarshal(payload, &candidate)
	return candidate, err == nil, err
}

func (repository *PostgresRepository) RenewJob(ctx context.Context, id string, control JobControl) error {
	result, err := repository.db.ExecContext(ctx, `UPDATE analysis_candidates SET heartbeat_at=clock_timestamp(),lease_expires_at=clock_timestamp()+interval '60 seconds'
 WHERE id=$1 AND owner_id=$2 AND attempt_token=$3 AND state='Analyzing' AND lease_expires_at>clock_timestamp()`, id, control.OwnerID, control.AttemptToken)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrLeaseLost
	}
	return nil
}

func lockedJob(ctx context.Context, transaction *sql.Tx, id string, control JobControl) (Candidate, error) {
	var payload []byte
	err := transaction.QueryRowContext(ctx, `SELECT payload FROM analysis_candidates WHERE id=$1 AND owner_id=$2 AND attempt_token=$3
 AND state='Analyzing' AND lease_expires_at>clock_timestamp() FOR UPDATE`, id, control.OwnerID, control.AttemptToken).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return Candidate{}, ErrLeaseLost
	}
	if err != nil {
		return Candidate{}, err
	}
	var candidate Candidate
	err = json.Unmarshal(payload, &candidate)
	return candidate, err
}

func (repository *PostgresRepository) PublishSource(ctx context.Context, id string, control JobControl, page PreparedPage) error {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	original, err := lockedJob(ctx, transaction, id, control)
	if err != nil {
		return err
	}
	if err := validateSource(page, original.FloorPlanID); err != nil {
		return err
	}
	source, err := json.Marshal(page.Source)
	if err != nil {
		return err
	}
	if _, err = transaction.ExecContext(ctx, `INSERT INTO analysis_source_images(analysis_id,source,content) VALUES($1,$2,$3)`, id, source, page.Content); err != nil {
		return err
	}
	if _, err = transaction.ExecContext(ctx, `UPDATE analysis_candidates SET payload=jsonb_set(payload,'{source}',$2::jsonb,true) WHERE id=$1`, id, source); err != nil {
		return err
	}
	return transaction.Commit()
}

func (repository *PostgresRepository) ReadSource(ctx context.Context, id string) (PreparedPage, bool, error) {
	var source []byte
	var page PreparedPage
	err := repository.db.QueryRowContext(ctx, `SELECT source,content FROM analysis_source_images WHERE analysis_id=$1`, id).Scan(&source, &page.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return PreparedPage{}, false, nil
	}
	if err != nil {
		return PreparedPage{}, false, err
	}
	err = json.Unmarshal(source, &page.Source)
	return page, err == nil, err
}

func (repository *PostgresRepository) FinishJob(ctx context.Context, candidate Candidate, control JobControl) error {
	if candidate.State != StateReady && candidate.State != StateFailed {
		return fmt.Errorf("job completion must be terminal")
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	original, err := lockedJob(ctx, transaction, candidate.ID, control)
	if err != nil {
		return err
	}
	candidate.FloorPlanID, candidate.Source = original.FloorPlanID, original.Source
	payload, err := json.Marshal(candidate)
	if err != nil {
		return err
	}
	result, err := transaction.ExecContext(ctx, `UPDATE analysis_candidates SET payload=$2,state=$3,owner_id=NULL,attempt_token=NULL,lease_expires_at=NULL,heartbeat_at=NULL
 WHERE id=$1 AND owner_id=$4 AND attempt_token=$5 AND state='Analyzing' AND lease_expires_at>clock_timestamp()`, candidate.ID, payload, candidate.State, control.OwnerID, control.AttemptToken)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrLeaseLost
	}
	return transaction.Commit()
}

func (repository *PostgresRepository) RecoverExpired(ctx context.Context) error {
	_, err := repository.db.ExecContext(ctx, `UPDATE analysis_candidates SET state='AnalysisFailed',owner_id=NULL,attempt_token=NULL,lease_expires_at=NULL,heartbeat_at=NULL,
 payload=payload||jsonb_build_object('analysisState','AnalysisFailed','completedAt',clock_timestamp(),'errorMessage','Analysis interrupted; retry with a new analysis.',
 'error',jsonb_build_object('errorCode','AnalysisInterrupted','message','Analysis interrupted; retry with a new analysis.','retryable',true))
 WHERE state='Analyzing' AND owner_id IS NOT NULL AND attempt_token IS NOT NULL AND lease_expires_at<=clock_timestamp()`)
	return err
}

func (repository *PostgresRepository) ReserveCall(ctx context.Context, id string, control JobControl, scope, provider, attempt string, limit int) error {
	if limit <= 0 || scope == "" || provider == "" {
		return runtimeFailure(503, "ProviderQuotaNotConfigured", "Provider daily quota is not configured.", false)
	}
	if attempt == "" {
		return fmt.Errorf("provider attempt is required")
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if _, err = lockedJob(ctx, transaction, id, control); err != nil {
		return err
	}
	var exists bool
	err = transaction.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_call_reservations WHERE quota_scope=$1 AND provider=$2 AND analysis_id=$3 AND attempt_id=$4)`, scope, provider, id, attempt).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("provider attempt already dispatched")
	}
	var day string
	var reset time.Time
	err = transaction.QueryRowContext(ctx, `WITH clock AS(SELECT clock_timestamp() AS now) SELECT to_char(now AT TIME ZONE 'UTC','YYYY-MM-DD'),
 (date_trunc('day',now AT TIME ZONE 'UTC')+interval '1 day') AT TIME ZONE 'UTC' FROM clock`).Scan(&day, &reset)
	if err != nil {
		return err
	}
	_, err = transaction.ExecContext(ctx, `INSERT INTO provider_daily_usage(quota_scope,provider,utc_date,reserved_calls,daily_limit) VALUES($1,$2,$3,0,$4) ON CONFLICT DO NOTHING`, scope, provider, day, limit)
	if err != nil {
		return err
	}
	var configured int
	if err = transaction.QueryRowContext(ctx, `SELECT daily_limit FROM provider_daily_usage WHERE quota_scope=$1 AND provider=$2 AND utc_date=$3 FOR UPDATE`, scope, provider, day).Scan(&configured); err != nil {
		return err
	}
	if configured != limit {
		return runtimeFailure(503, "ProviderQuotaNotConfigured", "Provider quota differs between instances for this UTC day.", false)
	}
	var count int
	err = transaction.QueryRowContext(ctx, `UPDATE provider_daily_usage SET reserved_calls=reserved_calls+1 WHERE quota_scope=$1 AND provider=$2 AND utc_date=$3 AND reserved_calls<daily_limit RETURNING reserved_calls`, scope, provider, day).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		failure := runtimeFailure(429, "ProviderQuotaExceeded", "Provider daily quota reached; retry after the UTC reset.", true)
		failure.Failure.RetryAt = &reset
		return failure
	}
	if err != nil {
		return err
	}
	_, err = transaction.ExecContext(ctx, `INSERT INTO provider_call_reservations(id,quota_scope,provider,analysis_id,attempt_id,utc_date,owner_id,attempt_token) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, newID(), scope, provider, id, attempt, day, control.OwnerID, control.AttemptToken)
	if err != nil {
		return err
	}
	return transaction.Commit()
}
