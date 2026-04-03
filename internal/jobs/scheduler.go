package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

type Engine struct {
	jobsRepo *sqliteStore.ScheduledJobsRepository
	logsRepo *sqliteStore.JobExecutionLogsRepository
	runners  map[string]Runner
	gate     *ExecutionGate
}

func NewEngine(
	jobsRepo *sqliteStore.ScheduledJobsRepository,
	logsRepo *sqliteStore.JobExecutionLogsRepository,
	runners map[string]Runner,
) *Engine {
	if runners == nil {
		runners = map[string]Runner{}
	}

	return &Engine{
		jobsRepo: jobsRepo,
		logsRepo: logsRepo,
		runners:  runners,
		gate:     NewExecutionGate(),
	}
}

func (e *Engine) SetExecutionGate(gate *ExecutionGate) {
	if gate != nil {
		e.gate = gate
	}
}

func (e *Engine) Reload(ctx context.Context, now time.Time) error {
	jobs, err := e.loadJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		var nextRunAt *time.Time
		if job.Enabled {
			next, err := nextScheduledTime(job.CronExpr, now)
			if err != nil {
				return fmt.Errorf("compute next run for %q: %w", job.Key, err)
			}
			nextRunAt = &next
		}

		if err := e.jobsRepo.Upsert(ctx, sqliteStore.ScheduledJob{
			Key:       job.Key,
			CronExpr:  job.CronExpr,
			Enabled:   job.Enabled,
			LastRunAt: job.LastRunAt,
			NextRunAt: nextRunAt,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) RunDue(ctx context.Context, now time.Time) error {
	jobs, err := e.loadJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if !job.Enabled || job.NextRunAt == nil || job.NextRunAt.After(now) {
			continue
		}

		if err := e.runScheduledJob(ctx, job, now); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) RunLoop(ctx context.Context, ticks <-chan time.Time, onError func(error)) {
	for {
		select {
		case <-ctx.Done():
			return
		case tick, ok := <-ticks:
			if !ok {
				return
			}

			if err := e.RunDue(ctx, tick); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}

func (e *Engine) loadJobs(ctx context.Context) ([]Job, error) {
	stored, err := e.jobsRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	storedByKey := make(map[string]sqliteStore.ScheduledJob, len(stored))
	for _, job := range stored {
		storedByKey[job.Key] = job
	}

	out := make([]Job, 0, len(builtInJobs))
	for _, def := range builtInJobs {
		if storedJob, ok := storedByKey[def.Key]; ok {
			out = append(out, toJob(storedJob))
			continue
		}

		out = append(out, Job{
			Key:      def.Key,
			CronExpr: def.CronExpr,
			Enabled:  def.Enabled,
		})
	}

	return out, nil
}

func (e *Engine) runScheduledJob(ctx context.Context, job Job, now time.Time) error {
	if !e.gate.TryAcquire(job.Key) {
		return nil
	}
	defer e.gate.Release(job.Key)

	if err := e.jobsRepo.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       job.Key,
		CronExpr:  job.CronExpr,
		Enabled:   job.Enabled,
		LastRunAt: job.LastRunAt,
		NextRunAt: job.NextRunAt,
	}); err != nil {
		return err
	}

	runner, ok := e.runners[job.Key]
	if !ok || runner == nil {
		return fmt.Errorf("job %q runner unavailable", job.Key)
	}

	startedAt := now.UTC().Truncate(time.Second)
	result, runErr := runner.Run(ctx)
	finishedAt := startedAt
	status := StatusSuccess
	var errorMessage *string
	if runErr != nil {
		status = StatusFailed
		msg := runErr.Error()
		errorMessage = &msg
	}

	if err := e.logsRepo.Create(ctx, sqliteStore.JobExecutionLog{
		JobKey:          job.Key,
		Status:          status,
		StartedAt:       startedAt,
		FinishedAt:      &finishedAt,
		RecordsAffected: result.RecordsAffected,
		ErrorMessage:    errorMessage,
	}); err != nil {
		return err
	}

	nextRunAt, err := nextScheduledTime(job.CronExpr, startedAt)
	if err != nil {
		return err
	}

	if err := e.jobsRepo.Upsert(ctx, sqliteStore.ScheduledJob{
		Key:       job.Key,
		CronExpr:  job.CronExpr,
		Enabled:   job.Enabled,
		LastRunAt: &startedAt,
		NextRunAt: &nextRunAt,
	}); err != nil {
		return err
	}

	logJobExecution("scheduled", job.Key, status, startedAt, finishedAt, result.RecordsAffected, errorMessage)
	slog.Info("scheduled job advanced", "job_key", job.Key, "next_run_at", nextRunAt.Format(time.RFC3339))

	return nil
}

func nextScheduledTime(expr string, now time.Time) (time.Time, error) {
	schedule, err := parseSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}

	cursor := now.UTC().Truncate(time.Minute).Add(time.Minute)
	limit := cursor.Add(366 * 24 * time.Hour)
	for !cursor.After(limit) {
		if schedule.matches(cursor) {
			return cursor, nil
		}
		cursor = cursor.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("could not find next scheduled time for %q", expr)
}

type schedule struct {
	minute fieldMatcher
	hour   fieldMatcher
}

func (s schedule) matches(t time.Time) bool {
	return s.minute.matches(t.Minute()) && s.hour.matches(t.Hour())
}

type fieldMatcher interface {
	matches(v int) bool
}

type anyField struct{}

func (anyField) matches(v int) bool { return true }

type exactField struct {
	value int
}

func (f exactField) matches(v int) bool { return v == f.value }

type stepField struct {
	step int
}

func (f stepField) matches(v int) bool { return v%f.step == 0 }

func parseSchedule(expr string) (schedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return schedule{}, fmt.Errorf("invalid cron expression %q", expr)
	}

	minute, err := parseField(fields[0], 0, 59)
	if err != nil {
		return schedule{}, fmt.Errorf("parse minute field: %w", err)
	}

	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return schedule{}, fmt.Errorf("parse hour field: %w", err)
	}

	for i := 2; i < len(fields); i++ {
		if fields[i] != "*" {
			return schedule{}, fmt.Errorf("unsupported cron field %q", fields[i])
		}
	}

	return schedule{
		minute: minute,
		hour:   hour,
	}, nil
}

func parseField(raw string, min int, max int) (fieldMatcher, error) {
	switch {
	case raw == "*":
		return anyField{}, nil
	case strings.HasPrefix(raw, "*/"):
		step, err := strconv.Atoi(strings.TrimPrefix(raw, "*/"))
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step field %q", raw)
		}
		return stepField{step: step}, nil
	default:
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid exact field %q", raw)
		}
		if value < min || value > max {
			return nil, fmt.Errorf("field %q out of range", raw)
		}
		return exactField{value: value}, nil
	}
}
