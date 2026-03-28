package settings

import (
	"context"
	"path/filepath"
	"testing"

	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

func TestServiceRejectsUnknownSettingKey(t *testing.T) {
	t.Parallel()

	svc, cleanup := newTestService(t)
	defer cleanup()

	err := svc.Upsert(context.Background(), Input{
		Key:   "unknown.key",
		Value: "value",
	})
	if err == nil {
		t.Fatal("expected error for unknown setting key")
	}
}

func TestServiceEncryptsSensitiveSettingsBeforeStorage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, cleanup := newTestService(t)
	defer cleanup()

	if err := svc.Upsert(ctx, Input{
		Key:   KeyLLMAPIKey,
		Value: "super-secret",
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	got, err := svc.Get(ctx, KeyLLMAPIKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Value != "super-secret" {
		t.Fatalf("expected decrypted value %q, got %q", "super-secret", got.Value)
	}

	raw, err := svc.repo.GetByKey(ctx, KeyLLMAPIKey)
	if err != nil {
		t.Fatalf("repo.GetByKey() error = %v", err)
	}

	if !raw.IsEncrypted {
		t.Fatal("expected raw stored setting to be marked encrypted")
	}

	if raw.Value == "super-secret" {
		t.Fatal("expected raw stored value to be encrypted, but it matched plaintext")
	}
}

func TestServiceStoresNonSensitiveSettingsAsPlaintext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, cleanup := newTestService(t)
	defer cleanup()

	if err := svc.Upsert(ctx, Input{
		Key:   KeyLLMBaseURL,
		Value: "https://api.example.test/v1",
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	got, err := svc.Get(ctx, KeyLLMBaseURL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Value != "https://api.example.test/v1" {
		t.Fatalf("unexpected value %q", got.Value)
	}

	raw, err := svc.repo.GetByKey(ctx, KeyLLMBaseURL)
	if err != nil {
		t.Fatalf("repo.GetByKey() error = %v", err)
	}

	if raw.IsEncrypted {
		t.Fatal("expected non-sensitive setting to remain plaintext")
	}
}

func TestServiceListReturnsDecryptedValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, cleanup := newTestService(t)
	defer cleanup()

	for _, input := range []Input{
		{Key: KeyLLMAPIKey, Value: "secret-1"},
		{Key: KeyLLMBaseURL, Value: "https://api.example.test/v1"},
		{Key: KeyLLMModel, Value: "gpt-4.1-mini"},
	} {
		if err := svc.Upsert(ctx, input); err != nil {
			t.Fatalf("Upsert(%s) error = %v", input.Key, err)
		}
	}

	settings, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(settings) != 3 {
		t.Fatalf("expected 3 settings, got %d", len(settings))
	}

	for _, s := range settings {
		if s.Key == KeyLLMAPIKey && s.Value != "secret-1" {
			t.Fatalf("expected decrypted list value for api key, got %q", s.Value)
		}
	}
}

func TestServiceListIncludesKnownUnsetKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, cleanup := newTestService(t)
	defer cleanup()

	if err := svc.Upsert(ctx, Input{
		Key:   KeyLLMBaseURL,
		Value: "https://api.example.test/v1",
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	settings, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(settings) != 3 {
		t.Fatalf("expected 3 settings including unset catalog entries, got %d", len(settings))
	}

	foundEmptyModel := false
	for _, setting := range settings {
		if setting.Key == KeyLLMModel && setting.Value == "" {
			foundEmptyModel = true
		}
	}

	if !foundEmptyModel {
		t.Fatal("expected known unset key to be included with empty value")
	}
}

func newTestService(t *testing.T) (*Service, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "settings.db")
	db, err := sqliteStore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := sqliteStore.Migrate(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("Migrate() error = %v", err)
	}

	repo := sqliteStore.NewSettingsRepository(db)
	codec, err := NewAESGCMCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		_ = db.Close()
		t.Fatalf("NewAESGCMCodec() error = %v", err)
	}

	svc := NewService(repo, codec)
	return svc, func() {
		_ = db.Close()
	}
}
