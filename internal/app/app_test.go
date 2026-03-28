package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hearthstone-analyzer/internal/jobs"
	"hearthstone-analyzer/internal/meta"
)

func TestLoadConfigUsesDefaultAddr(t *testing.T) {
	t.Setenv("APP_ADDR", "")

	cfg := LoadConfig()

	if cfg.Addr != ":8080" {
		t.Fatalf("expected default addr %q, got %q", ":8080", cfg.Addr)
	}
}

func TestLoadConfigUsesEnvAddr(t *testing.T) {
	t.Setenv("APP_ADDR", ":9090")

	cfg := LoadConfig()

	if cfg.Addr != ":9090" {
		t.Fatalf("expected env addr %q, got %q", ":9090", cfg.Addr)
	}
}

func TestLoadConfigUsesDefaultDBPath(t *testing.T) {
	t.Setenv("APP_DB_PATH", "")
	t.Setenv("APP_DATA_DIR", "")

	cfg := LoadConfig()

	if cfg.DBPath != filepath.Join("data", "hearthstone.db") {
		t.Fatalf("expected default db path %q, got %q", filepath.Join("data", "hearthstone.db"), cfg.DBPath)
	}
}

func TestLoadConfigUsesEnvDBPath(t *testing.T) {
	t.Setenv("APP_DB_PATH", filepath.Join("custom", "db.sqlite"))

	cfg := LoadConfig()

	if cfg.DBPath != filepath.Join("custom", "db.sqlite") {
		t.Fatalf("expected env db path %q, got %q", filepath.Join("custom", "db.sqlite"), cfg.DBPath)
	}
}

func TestLoadConfigUsesDefaultSettingsEncryptionKey(t *testing.T) {
	t.Setenv("APP_SETTINGS_KEY", "")

	cfg := LoadConfig()

	if cfg.SettingsKey != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("expected default settings key, got %q", cfg.SettingsKey)
	}
}

func TestConfigUsesDefaultSettingsKey(t *testing.T) {
	cfg := Config{SettingsKey: defaultSettingsKey}

	if !cfg.UsesDefaultSettingsKey() {
		t.Fatal("expected default settings key detection to be true")
	}
}

func TestConfigDataDirUsesDBPathDirectory(t *testing.T) {
	cfg := Config{DBPath: filepath.Join("custom", "data", "hearthstone.db")}

	if cfg.DataDir() != filepath.Join("custom", "data") {
		t.Fatalf("expected data dir %q, got %q", filepath.Join("custom", "data"), cfg.DataDir())
	}
}

func TestLoadConfigUsesDefaultCardsSourceSettings(t *testing.T) {
	t.Setenv("APP_CARDS_SOURCE_URL", "")
	t.Setenv("APP_CARDS_LOCALE", "")

	cfg := LoadConfig()

	if cfg.CardsSourceURL == "" {
		t.Fatal("expected default cards source url to be set")
	}

	if cfg.CardsLocale != "enUS" {
		t.Fatalf("expected default cards locale %q, got %q", "enUS", cfg.CardsLocale)
	}
}

func TestBootstrapInitializesSchedulerState(t *testing.T) {
	cfg := Config{
		Addr:           ":0",
		DBPath:         filepath.Join(t.TempDir(), "bootstrap.db"),
		SettingsKey:    "0123456789abcdef0123456789abcdef",
		CardsSourceURL: "https://example.test/cards.json",
		CardsLocale:    "enUS",
	}

	runtime, err := Bootstrap(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.DB.Close()
	})

	job, err := runtime.Repositories.ScheduledJobs.GetByKey(context.Background(), jobs.KeySyncCards)
	if err != nil {
		t.Fatalf("GetByKey(sync_cards) error = %v", err)
	}

	if job.NextRunAt == nil {
		t.Fatal("expected sync_cards next_run_at to be initialized during bootstrap")
	}
}

func TestBootstrapRegistersSyncMetaJobDefinition(t *testing.T) {
	cfg := Config{
		Addr:           ":0",
		DBPath:         filepath.Join(t.TempDir(), "bootstrap-meta.db"),
		SettingsKey:    "0123456789abcdef0123456789abcdef",
		CardsSourceURL: "https://example.test/cards.json",
		CardsLocale:    "enUS",
	}

	runtime, err := Bootstrap(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.DB.Close()
	})

	jobsList, err := runtime.Jobs.List(context.Background())
	if err != nil {
		t.Fatalf("Jobs.List() error = %v", err)
	}

	found := false
	for _, job := range jobsList {
		if job.Key == jobs.KeySyncMeta {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected sync_meta to be present in built-in jobs")
	}
}

func TestBootstrapSyncMetaCanUseFixtureSource(t *testing.T) {
	cfg := Config{
		Addr:           ":0",
		DBPath:         filepath.Join(t.TempDir(), "bootstrap-meta-fixture.db"),
		SettingsKey:    "0123456789abcdef0123456789abcdef",
		CardsSourceURL: "https://example.test/cards.json",
		CardsLocale:    "enUS",
		MetaFixture:    "32.1.0",
	}

	runtime, err := Bootstrap(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.DB.Close()
	})

	if err := runtime.Jobs.RunNow(context.Background(), jobs.KeySyncMeta); err != nil {
		t.Fatalf("RunNow(sync_meta) error = %v", err)
	}

	latest, err := runtime.Repositories.MetaSnapshots.GetLatestByFormat(context.Background(), "standard")
	if err != nil {
		t.Fatalf("GetLatestByFormat() error = %v", err)
	}

	if latest.Source != "fixture" || latest.PatchVersion != "32.1.0" {
		t.Fatalf("unexpected latest meta snapshot: %+v", latest)
	}
}

func TestBootstrapSyncMetaCanUseFileSource(t *testing.T) {
	metaFilePath := filepath.Join(t.TempDir(), "meta.json")
	if err := os.WriteFile(metaFilePath, []byte(`{
  "patch_version": "32.3.0",
  "format": "standard",
  "region": "APAC"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := Config{
		Addr:           ":0",
		DBPath:         filepath.Join(t.TempDir(), "bootstrap-meta-file.db"),
		SettingsKey:    "0123456789abcdef0123456789abcdef",
		CardsSourceURL: "https://example.test/cards.json",
		CardsLocale:    "enUS",
		MetaFilePath:   metaFilePath,
	}

	runtime, err := Bootstrap(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.DB.Close()
	})

	if err := runtime.Jobs.RunNow(context.Background(), jobs.KeySyncMeta); err != nil {
		t.Fatalf("RunNow(sync_meta) error = %v", err)
	}

	latest, err := runtime.Repositories.MetaSnapshots.GetLatestByFormat(context.Background(), "standard")
	if err != nil {
		t.Fatalf("GetLatestByFormat() error = %v", err)
	}

	if latest.Source != "file" || latest.PatchVersion != "32.3.0" {
		t.Fatalf("unexpected latest meta snapshot: %+v", latest)
	}
}

func TestLoadConfigReadsRemoteMetaURL(t *testing.T) {
	t.Setenv("APP_META_REMOTE_URL", "https://example.test/meta.json")

	cfg := LoadConfig()

	if cfg.MetaRemoteURL != "https://example.test/meta.json" {
		t.Fatalf("expected remote meta url to round-trip, got %q", cfg.MetaRemoteURL)
	}
}

func TestLoadConfigReadsRemoteMetaAuthSettings(t *testing.T) {
	t.Setenv("APP_META_REMOTE_TOKEN", "secret-token")
	t.Setenv("APP_META_REMOTE_HEADER_NAME", "X-Meta-Key")
	t.Setenv("APP_META_REMOTE_HEADER_VALUE", "meta-value")

	cfg := LoadConfig()

	if cfg.MetaRemoteToken != "secret-token" || cfg.MetaRemoteHeaderName != "X-Meta-Key" || cfg.MetaRemoteHeaderValue != "meta-value" {
		t.Fatalf("expected remote meta auth settings to round-trip, got %+v", cfg)
	}
}

func TestLoadConfigReadsRemoteMetaProfile(t *testing.T) {
	t.Setenv("APP_META_REMOTE_PROFILE", "vicioussyndicate")

	cfg := LoadConfig()

	if cfg.MetaRemoteProfile != "vicioussyndicate" {
		t.Fatalf("expected remote meta profile to round-trip, got %q", cfg.MetaRemoteProfile)
	}
}

func TestConfigMetaSourceMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "file", cfg: Config{MetaFilePath: "meta.json"}, want: "file"},
		{name: "remote profile", cfg: Config{MetaRemoteURL: "https://example.test", MetaRemoteProfile: "vicioussyndicate"}, want: "remote_profile:vicioussyndicate"},
		{name: "remote", cfg: Config{MetaRemoteURL: "https://example.test"}, want: "remote"},
		{name: "fixture", cfg: Config{MetaFixture: "32.1.0"}, want: "fixture"},
		{name: "unconfigured", cfg: Config{}, want: "unconfigured"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.MetaSourceMode(); got != tc.want {
				t.Fatalf("expected meta source mode %q, got %q", tc.want, got)
			}
		})
	}
}

func TestBuildMetaSourceUsesHearthstoneTopDecksProfile(t *testing.T) {
	cfg := Config{
		MetaRemoteURL:     "https://example.test/standard-meta/",
		MetaRemoteProfile: "hearthstonetopdecks",
	}

	source := buildMetaSource(cfg)
	if _, ok := source.(meta.HearthstoneTopDecksSource); !ok {
		t.Fatalf("expected hearthstonetopdecks profile source, got %T", source)
	}
}
