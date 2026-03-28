package jobs

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

func TestServiceListIncludesBuiltInJobs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	svc := NewService(repos.ScheduledJobs, repos.JobExecutionLogs, nil)

	got, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 built-in jobs, got %d", len(got))
	}

	if got[0].Key != KeySyncCards || got[1].Key != KeySyncMeta || got[2].Key != KeyRebuildFeatures {
		t.Fatalf("unexpected built-in job order: %+v", got)
	}

	if got[0].CronExpr != "0 */6 * * *" || !got[0].Enabled {
		t.Fatalf("unexpected sync_cards defaults: %+v", got[0])
	}
}

func TestServiceUpdatePersistsJobOverride(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	svc := NewService(repos.ScheduledJobs, repos.JobExecutionLogs, nil)

	updated, err := svc.Update(ctx, UpdateInput{
		Key:      KeySyncCards,
		CronExpr: "*/15 * * * *",
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.Key != KeySyncCards || updated.CronExpr != "*/15 * * * *" || updated.Enabled {
		t.Fatalf("unexpected updated job: %+v", updated)
	}

	persisted, err := repos.ScheduledJobs.GetByKey(ctx, KeySyncCards)
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}

	if persisted.CronExpr != "*/15 * * * *" || persisted.Enabled {
		t.Fatalf("unexpected persisted job row: %+v", persisted)
	}
}

func TestServiceUpdateRejectsUnknownJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	svc := NewService(repos.ScheduledJobs, repos.JobExecutionLogs, nil)

	_, err := svc.Update(ctx, UpdateInput{
		Key:      "unknown_job",
		CronExpr: "0 * * * *",
		Enabled:  true,
	})
	if err == nil {
		t.Fatal("expected error for unknown job key")
	}
}

func TestServiceUpdateRejectsInvalidCronExpr(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	svc := NewService(repos.ScheduledJobs, repos.JobExecutionLogs, nil)

	_, err := svc.Update(ctx, UpdateInput{
		Key:      KeySyncCards,
		CronExpr: "not-a-cron",
		Enabled:  true,
	})
	if err == nil {
		t.Fatal("expected error for invalid cron expr")
	}
}

func TestServiceRunNowRecordsSuccessAndPreservesNextRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	nextRunAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       KeySyncCards,
		CronExpr:  "0 */6 * * *",
		Enabled:   true,
		NextRunAt: &nextRunAt,
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	affected := int64(42)
	svc := NewService(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{
		KeySyncCards: runnerFunc(func(ctx context.Context) (RunResult, error) {
			return RunResult{RecordsAffected: &affected}, nil
		}),
	})

	if err := svc.RunNow(ctx, KeySyncCards); err != nil {
		t.Fatalf("RunNow() error = %v", err)
	}

	job, err := svc.Get(ctx, KeySyncCards)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if job.LastRunAt == nil {
		t.Fatal("expected last_run_at to be set after manual run")
	}

	if job.NextRunAt == nil || !job.NextRunAt.Equal(nextRunAt) {
		t.Fatalf("expected next_run_at to stay unchanged, got %+v", job.NextRunAt)
	}

	history, err := svc.History(ctx, KeySyncCards, 10)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 execution log, got %d", len(history))
	}

	if history[0].Status != StatusSuccess {
		t.Fatalf("expected success status, got %+v", history[0])
	}

	if history[0].RecordsAffected == nil || *history[0].RecordsAffected != affected {
		t.Fatalf("expected records_affected=%d, got %+v", affected, history[0].RecordsAffected)
	}
}

func TestServiceRunNowRecordsFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	svc := NewService(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{
		KeySyncCards: runnerFunc(func(ctx context.Context) (RunResult, error) {
			return RunResult{}, errors.New("upstream timeout")
		}),
	})

	err := svc.RunNow(ctx, KeySyncCards)
	if err == nil {
		t.Fatal("expected RunNow() to return runner error")
	}

	history, histErr := svc.History(ctx, KeySyncCards, 10)
	if histErr != nil {
		t.Fatalf("History() error = %v", histErr)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 execution log, got %d", len(history))
	}

	if history[0].Status != StatusFailed {
		t.Fatalf("expected failed status, got %+v", history[0])
	}

	if history[0].ErrorMessage == nil || *history[0].ErrorMessage != "upstream timeout" {
		t.Fatalf("unexpected failure log: %+v", history[0])
	}
}

func TestServiceUpdateTriggersReloadHook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	svc := NewService(repos.ScheduledJobs, repos.JobExecutionLogs, nil)

	calls := 0
	svc.SetReloadHook(func(ctx context.Context) error {
		calls++
		return nil
	})

	_, err := svc.Update(ctx, UpdateInput{
		Key:      KeySyncCards,
		CronExpr: "*/20 * * * *",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected reload hook to be called once, got %d", calls)
	}
}

func TestServiceRunNowRejectsDuplicateConcurrentRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls int32

	svc := NewService(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{
		KeySyncCards: RunnerFunc(func(ctx context.Context) (RunResult, error) {
			atomic.AddInt32(&calls, 1)
			started <- struct{}{}
			<-release
			return RunResult{}, nil
		}),
	})

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- svc.RunNow(ctx, KeySyncCards)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected first run to start")
	}

	err := svc.RunNow(ctx, KeySyncCards)
	if !errors.Is(err, ErrJobAlreadyRunning) {
		t.Fatalf("expected ErrJobAlreadyRunning, got %v", err)
	}

	close(release)

	select {
	case err := <-firstErrCh:
		if err != nil {
			t.Fatalf("first RunNow() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected first run to finish")
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected runner to be called once, got %d", got)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "jobs.db")
	db, err := sqliteStore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := sqliteStore.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return db
}

type runnerFunc func(ctx context.Context) (RunResult, error)

func (f runnerFunc) Run(ctx context.Context) (RunResult, error) {
	return f(ctx)
}
