package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"hearthstone-analyzer/internal/app"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := app.LoadConfig()
	runtime, err := app.Bootstrap(ctx, cfg)
	if err != nil {
		logger.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := runtime.DB.Close(); err != nil {
			logger.Error("database close error", "error", err)
		}
	}()

	logStartupConfiguration(logger, cfg)
	logSchedulerState(ctx, logger, runtime)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go runtime.Scheduler.RunLoop(ctx, ticker.C, func(err error) {
		logger.Error("scheduler tick failed", "error", err)
	})

	logger.Info("starting api server", "addr", cfg.Addr)
	if err := runtime.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func logStartupConfiguration(logger *slog.Logger, cfg app.Config) {
	logger.Info(
		"startup configuration",
		"addr", cfg.Addr,
		"db_path", cfg.DBPath,
		"data_dir", cfg.DataDir(),
		"cards_locale", cfg.CardsLocale,
		"meta_source_mode", cfg.MetaSourceMode(),
		"using_default_settings_key", cfg.UsesDefaultSettingsKey(),
	)

	if cfg.UsesDefaultSettingsKey() {
		logger.Warn("using development default APP_SETTINGS_KEY; override this in non-local deployments")
	}
}

func logSchedulerState(ctx context.Context, logger *slog.Logger, runtime *app.Runtime) {
	jobsList, err := runtime.Jobs.List(ctx)
	if err != nil {
		logger.Warn("failed to load scheduler state for startup log", "error", err)
		return
	}

	logger.Info("scheduler initialized", "job_count", len(jobsList))
	for _, job := range jobsList {
		logger.Info(
			"scheduler job",
			"key", job.Key,
			"enabled", job.Enabled,
			"cron_expr", job.CronExpr,
			"next_run_at", formatOptionalTime(job.NextRunAt),
			"last_run_at", formatOptionalTime(job.LastRunAt),
		)
	}
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return "null"
	}
	return value.UTC().Format(time.RFC3339)
}
