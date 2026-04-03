package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

const (
	KeySyncCards       = "sync_cards"
	KeySyncMeta        = "sync_meta"
	KeyRebuildFeatures = "rebuild_features"

	StatusSuccess = "success"
	StatusFailed  = "failed"
)

var ErrJobAlreadyRunning = errors.New("job already running")

type Definition struct {
	Key      string
	CronExpr string
	Enabled  bool
}

type Job struct {
	Key       string     `json:"key"`
	CronExpr  string     `json:"cron_expr"`
	Enabled   bool       `json:"enabled"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type UpdateInput struct {
	Key      string
	CronExpr string
	Enabled  bool
}

type Execution struct {
	ID              int64      `json:"id"`
	JobKey          string     `json:"job_key"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	RecordsAffected *int64     `json:"records_affected,omitempty"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
}

type RunResult struct {
	RecordsAffected *int64
}

type Runner interface {
	Run(ctx context.Context) (RunResult, error)
}

type RunnerFunc func(ctx context.Context) (RunResult, error)

func (f RunnerFunc) Run(ctx context.Context) (RunResult, error) {
	return f(ctx)
}

type Service struct {
	jobsRepo   *sqliteStore.ScheduledJobsRepository
	logsRepo   *sqliteStore.JobExecutionLogsRepository
	runners    map[string]Runner
	gate       *ExecutionGate
	reloadHook func(context.Context) error
}

var builtInJobs = []Definition{
	{Key: KeySyncCards, CronExpr: "0 */6 * * *", Enabled: true},
	{Key: KeySyncMeta, CronExpr: "0 */12 * * *", Enabled: false},
	{Key: KeyRebuildFeatures, CronExpr: "0 0 * * *", Enabled: false},
}

func NewService(
	jobsRepo *sqliteStore.ScheduledJobsRepository,
	logsRepo *sqliteStore.JobExecutionLogsRepository,
	runners map[string]Runner,
) *Service {
	if runners == nil {
		runners = map[string]Runner{}
	}

	return &Service{
		jobsRepo: jobsRepo,
		logsRepo: logsRepo,
		runners:  runners,
		gate:     NewExecutionGate(),
	}
}

func (s *Service) SetReloadHook(hook func(context.Context) error) {
	s.reloadHook = hook
}

func (s *Service) SetExecutionGate(gate *ExecutionGate) {
	if gate != nil {
		s.gate = gate
	}
}

func (s *Service) List(ctx context.Context) ([]Job, error) {
	stored, err := s.jobsRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	storedByKey := make(map[string]sqliteStore.ScheduledJob, len(stored))
	for _, job := range stored {
		storedByKey[job.Key] = job
	}

	out := make([]Job, 0, len(builtInJobs))
	for _, def := range builtInJobs {
		job := Job{
			Key:      def.Key,
			CronExpr: def.CronExpr,
			Enabled:  def.Enabled,
		}

		if storedJob, ok := storedByKey[def.Key]; ok {
			job = toJob(storedJob)
		}

		out = append(out, job)
	}

	return out, nil
}

func (s *Service) Get(ctx context.Context, key string) (Job, error) {
	def, err := lookupDefinition(key)
	if err != nil {
		return Job{}, err
	}

	stored, err := s.jobsRepo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{
				Key:      def.Key,
				CronExpr: def.CronExpr,
				Enabled:  def.Enabled,
			}, nil
		}

		return Job{}, err
	}

	return toJob(stored), nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (Job, error) {
	def, err := lookupDefinition(input.Key)
	if err != nil {
		return Job{}, err
	}

	if err := validateCronExpr(input.CronExpr); err != nil {
		return Job{}, err
	}

	job := sqliteStore.ScheduledJob{
		Key:      def.Key,
		CronExpr: input.CronExpr,
		Enabled:  input.Enabled,
	}

	existing, err := s.jobsRepo.GetByKey(ctx, input.Key)
	if err == nil {
		job.LastRunAt = existing.LastRunAt
		job.NextRunAt = existing.NextRunAt
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Job{}, err
	}

	if err := s.jobsRepo.Upsert(ctx, job); err != nil {
		return Job{}, err
	}

	if s.reloadHook != nil {
		if err := s.reloadHook(ctx); err != nil {
			return Job{}, err
		}
	}

	return s.Get(ctx, input.Key)
}

func (s *Service) RunNow(ctx context.Context, key string) error {
	job, err := s.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := s.jobsRepo.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       job.Key,
		CronExpr:  job.CronExpr,
		Enabled:   job.Enabled,
		LastRunAt: job.LastRunAt,
		NextRunAt: job.NextRunAt,
	}); err != nil {
		return err
	}

	runner, ok := s.runners[key]
	if !ok || runner == nil {
		return fmt.Errorf("job %q runner unavailable", key)
	}

	if !s.gate.TryAcquire(key) {
		return ErrJobAlreadyRunning
	}
	defer s.gate.Release(key)

	startedAt := time.Now().UTC().Truncate(time.Second)
	result, runErr := runner.Run(ctx)
	finishedAt := time.Now().UTC().Truncate(time.Second)

	status := StatusSuccess
	var errorMessage *string
	if runErr != nil {
		status = StatusFailed
		msg := runErr.Error()
		errorMessage = &msg
	}

	if err := s.logsRepo.Create(ctx, sqliteStore.JobExecutionLog{
		JobKey:          key,
		Status:          status,
		StartedAt:       startedAt,
		FinishedAt:      &finishedAt,
		RecordsAffected: result.RecordsAffected,
		ErrorMessage:    errorMessage,
	}); err != nil {
		return err
	}

	if err := s.jobsRepo.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       job.Key,
		CronExpr:  job.CronExpr,
		Enabled:   job.Enabled,
		LastRunAt: &startedAt,
		NextRunAt: job.NextRunAt,
	}); err != nil {
		return err
	}

	logJobExecution("manual", key, status, startedAt, finishedAt, result.RecordsAffected, errorMessage)

	return runErr
}

func (s *Service) History(ctx context.Context, key string, limit int) ([]Execution, error) {
	if _, err := lookupDefinition(key); err != nil {
		return nil, err
	}

	logs, err := s.logsRepo.ListByJobKey(ctx, key, limit)
	if err != nil {
		return nil, err
	}

	out := make([]Execution, 0, len(logs))
	for _, log := range logs {
		out = append(out, Execution{
			ID:              log.ID,
			JobKey:          log.JobKey,
			Status:          log.Status,
			StartedAt:       log.StartedAt,
			FinishedAt:      log.FinishedAt,
			RecordsAffected: log.RecordsAffected,
			ErrorMessage:    log.ErrorMessage,
		})
	}

	return out, nil
}

func lookupDefinition(key string) (Definition, error) {
	for _, def := range builtInJobs {
		if def.Key == key {
			return def, nil
		}
	}

	return Definition{}, fmt.Errorf("unknown job key %q", key)
}

func validateCronExpr(expr string) error {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return fmt.Errorf("invalid cron expression %q", expr)
	}

	return nil
}

func toJob(job sqliteStore.ScheduledJob) Job {
	return Job{
		Key:       job.Key,
		CronExpr:  job.CronExpr,
		Enabled:   job.Enabled,
		LastRunAt: job.LastRunAt,
		NextRunAt: job.NextRunAt,
		UpdatedAt: job.UpdatedAt,
	}
}

func logJobExecution(trigger string, jobKey string, status string, startedAt time.Time, finishedAt time.Time, recordsAffected *int64, errorMessage *string) {
	attrs := []any{
		"trigger", trigger,
		"job_key", jobKey,
		"status", status,
		"started_at", startedAt.Format(time.RFC3339),
		"finished_at", finishedAt.Format(time.RFC3339),
	}
	if recordsAffected != nil {
		attrs = append(attrs, "records_affected", *recordsAffected)
	}
	if errorMessage != nil && strings.TrimSpace(*errorMessage) != "" {
		attrs = append(attrs, "error", *errorMessage)
		slog.Error("job execution failed", attrs...)
		return
	}
	slog.Info("job execution finished", attrs...)
}
