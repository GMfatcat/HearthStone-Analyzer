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

func TestEngineReloadSchedulesEnabledJobs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openSchedulerTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:      KeySyncCards,
		CronExpr: "*/15 * * * *",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("Upsert(sync_cards) error = %v", err)
	}

	engine := NewEngine(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{})
	now := time.Date(2026, 3, 25, 10, 7, 0, 0, time.UTC)

	if err := engine.Reload(ctx, now); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	job, err := repos.ScheduledJobs.GetByKey(ctx, KeySyncCards)
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}

	if job.NextRunAt == nil {
		t.Fatal("expected next_run_at to be set")
	}

	want := time.Date(2026, 3, 25, 10, 15, 0, 0, time.UTC)
	if !job.NextRunAt.Equal(want) {
		t.Fatalf("expected next_run_at=%s, got %s", want, job.NextRunAt)
	}
}

func TestEngineReloadClearsNextRunForDisabledJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openSchedulerTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	existingNext := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)

	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       KeySyncMeta,
		CronExpr:  "0 */12 * * *",
		Enabled:   false,
		NextRunAt: &existingNext,
	}); err != nil {
		t.Fatalf("Upsert(sync_meta) error = %v", err)
	}

	engine := NewEngine(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{})
	now := time.Date(2026, 3, 25, 10, 7, 0, 0, time.UTC)

	if err := engine.Reload(ctx, now); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	job, err := repos.ScheduledJobs.GetByKey(ctx, KeySyncMeta)
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}

	if job.NextRunAt != nil {
		t.Fatalf("expected next_run_at to be cleared for disabled job, got %v", *job.NextRunAt)
	}
}

func TestEngineRunDueExecutesRunnerAndAdvancesNextRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openSchedulerTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	initialNext := time.Date(2026, 3, 25, 10, 15, 0, 0, time.UTC)

	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       KeySyncCards,
		CronExpr:  "*/15 * * * *",
		Enabled:   true,
		NextRunAt: &initialNext,
	}); err != nil {
		t.Fatalf("Upsert(sync_cards) error = %v", err)
	}

	calls := 0
	affected := int64(7)
	engine := NewEngine(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{
		KeySyncCards: RunnerFunc(func(ctx context.Context) (RunResult, error) {
			calls++
			return RunResult{RecordsAffected: &affected}, nil
		}),
	})

	now := time.Date(2026, 3, 25, 10, 15, 0, 0, time.UTC)
	if err := engine.RunDue(ctx, now); err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected runner to be called once, got %d", calls)
	}

	job, err := repos.ScheduledJobs.GetByKey(ctx, KeySyncCards)
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}

	if job.LastRunAt == nil || !job.LastRunAt.Equal(now) {
		t.Fatalf("expected last_run_at=%s, got %+v", now, job.LastRunAt)
	}

	wantNext := time.Date(2026, 3, 25, 10, 30, 0, 0, time.UTC)
	if job.NextRunAt == nil || !job.NextRunAt.Equal(wantNext) {
		t.Fatalf("expected next_run_at=%s, got %+v", wantNext, job.NextRunAt)
	}

	history, err := repos.JobExecutionLogs.ListByJobKey(ctx, KeySyncCards, 10)
	if err != nil {
		t.Fatalf("ListByJobKey() error = %v", err)
	}

	if len(history) != 1 || history[0].Status != StatusSuccess {
		t.Fatalf("unexpected execution history: %+v", history)
	}
}

func TestEngineRunDueRecordsFailureAndStillAdvancesSchedule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openSchedulerTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	initialNext := time.Date(2026, 3, 25, 10, 15, 0, 0, time.UTC)

	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       KeySyncCards,
		CronExpr:  "*/15 * * * *",
		Enabled:   true,
		NextRunAt: &initialNext,
	}); err != nil {
		t.Fatalf("Upsert(sync_cards) error = %v", err)
	}

	engine := NewEngine(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{
		KeySyncCards: RunnerFunc(func(ctx context.Context) (RunResult, error) {
			return RunResult{}, errors.New("source timeout")
		}),
	})

	now := time.Date(2026, 3, 25, 10, 15, 0, 0, time.UTC)
	if err := engine.RunDue(ctx, now); err != nil {
		t.Fatalf("RunDue() error = %v", err)
	}

	job, err := repos.ScheduledJobs.GetByKey(ctx, KeySyncCards)
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}

	wantNext := time.Date(2026, 3, 25, 10, 30, 0, 0, time.UTC)
	if job.NextRunAt == nil || !job.NextRunAt.Equal(wantNext) {
		t.Fatalf("expected next_run_at=%s, got %+v", wantNext, job.NextRunAt)
	}

	history, err := repos.JobExecutionLogs.ListByJobKey(ctx, KeySyncCards, 10)
	if err != nil {
		t.Fatalf("ListByJobKey() error = %v", err)
	}

	if len(history) != 1 || history[0].Status != StatusFailed {
		t.Fatalf("unexpected execution history: %+v", history)
	}

	if history[0].ErrorMessage == nil || *history[0].ErrorMessage != "source timeout" {
		t.Fatalf("unexpected failure log: %+v", history[0])
	}
}

func TestEngineReloadAppliesUpdatedScheduleImmediately(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openSchedulerTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:      KeySyncCards,
		CronExpr: "0 */6 * * *",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("Upsert(initial) error = %v", err)
	}

	engine := NewEngine(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{})
	now := time.Date(2026, 3, 25, 10, 7, 0, 0, time.UTC)

	if err := engine.Reload(ctx, now); err != nil {
		t.Fatalf("Reload(initial) error = %v", err)
	}

	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:      KeySyncCards,
		CronExpr: "*/10 * * * *",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("Upsert(updated) error = %v", err)
	}

	if err := engine.Reload(ctx, now); err != nil {
		t.Fatalf("Reload(updated) error = %v", err)
	}

	job, err := repos.ScheduledJobs.GetByKey(ctx, KeySyncCards)
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}

	want := time.Date(2026, 3, 25, 10, 10, 0, 0, time.UTC)
	if job.NextRunAt == nil || !job.NextRunAt.Equal(want) {
		t.Fatalf("expected updated next_run_at=%s, got %+v", want, job.NextRunAt)
	}
}

func TestEngineRunLoopExecutesDueJobsOnTick(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := openSchedulerTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	initialNext := time.Date(2026, 3, 25, 10, 15, 0, 0, time.UTC)
	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       KeySyncCards,
		CronExpr:  "*/15 * * * *",
		Enabled:   true,
		NextRunAt: &initialNext,
	}); err != nil {
		t.Fatalf("Upsert(sync_cards) error = %v", err)
	}

	calls := make(chan struct{}, 1)
	engine := NewEngine(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{
		KeySyncCards: RunnerFunc(func(ctx context.Context) (RunResult, error) {
			calls <- struct{}{}
			return RunResult{}, nil
		}),
	})

	ticks := make(chan time.Time, 1)
	go engine.RunLoop(ctx, ticks, nil)

	ticks <- initialNext

	select {
	case <-calls:
	case <-time.After(2 * time.Second):
		t.Fatal("expected runner to be called from loop tick")
	}
}

func TestEngineRunDueSkipsJobAlreadyRunning(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openSchedulerTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	initialNext := time.Date(2026, 3, 25, 10, 15, 0, 0, time.UTC)
	if err := repos.ScheduledJobs.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       KeySyncCards,
		CronExpr:  "*/15 * * * *",
		Enabled:   true,
		NextRunAt: &initialNext,
	}); err != nil {
		t.Fatalf("Upsert(sync_cards) error = %v", err)
	}

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls int32

	engine := NewEngine(repos.ScheduledJobs, repos.JobExecutionLogs, map[string]Runner{
		KeySyncCards: RunnerFunc(func(ctx context.Context) (RunResult, error) {
			atomic.AddInt32(&calls, 1)
			started <- struct{}{}
			<-release
			return RunResult{}, nil
		}),
	})

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- engine.RunDue(ctx, initialNext)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected first scheduled run to start")
	}

	secondErr := engine.RunDue(ctx, initialNext)
	if secondErr != nil {
		t.Fatalf("expected duplicate scheduled run to be skipped, got %v", secondErr)
	}

	close(release)

	select {
	case err := <-firstErrCh:
		if err != nil {
			t.Fatalf("first RunDue() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected first scheduled run to finish")
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected runner to be called once, got %d", got)
	}
}

func openSchedulerTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "scheduler.db")
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
