package app

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"hearthstone-analyzer/internal/analysis"
	"hearthstone-analyzer/internal/cardquery"
	"hearthstone-analyzer/internal/cards"
	comparepkg "hearthstone-analyzer/internal/compare"
	"hearthstone-analyzer/internal/deckanalysis"
	"hearthstone-analyzer/internal/decks"
	"hearthstone-analyzer/internal/jobs"
	"hearthstone-analyzer/internal/meta"
	reportpkg "hearthstone-analyzer/internal/report"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"hearthstone-analyzer/internal/httpapi"
	"hearthstone-analyzer/internal/settings"
	"hearthstone-analyzer/internal/storage/sqlite"
	"hearthstone-analyzer/web"
)

const defaultSettingsKey = "0123456789abcdef0123456789abcdef"

type Config struct {
	Addr                  string
	DBPath                string
	SettingsKey           string
	CardsSourceURL        string
	CardsLocale           string
	MetaFixture           string
	MetaFilePath          string
	MetaRemoteURL         string
	MetaRemoteToken       string
	MetaRemoteHeaderName  string
	MetaRemoteHeaderValue string
	MetaRemoteProfile     string
}

func LoadConfig() Config {
	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dbPath := os.Getenv("APP_DB_PATH")
	if dbPath == "" {
		dataDir := os.Getenv("APP_DATA_DIR")
		if dataDir == "" {
			dataDir = "data"
		}

		dbPath = filepath.Join(dataDir, "hearthstone.db")
	}

	settingsKey := os.Getenv("APP_SETTINGS_KEY")
	if settingsKey == "" {
		settingsKey = defaultSettingsKey
	}

	cardsSourceURL := os.Getenv("APP_CARDS_SOURCE_URL")
	if cardsSourceURL == "" {
		cardsSourceURL = "https://api.hearthstonejson.com/v1/latest/enUS/cards.collectible.json"
	}

	cardsLocale := os.Getenv("APP_CARDS_LOCALE")
	if cardsLocale == "" {
		cardsLocale = "enUS"
	}

	return Config{
		Addr:                  addr,
		DBPath:                dbPath,
		SettingsKey:           settingsKey,
		CardsSourceURL:        cardsSourceURL,
		CardsLocale:           cardsLocale,
		MetaFixture:           os.Getenv("APP_META_FIXTURE"),
		MetaFilePath:          os.Getenv("APP_META_FILE"),
		MetaRemoteURL:         os.Getenv("APP_META_REMOTE_URL"),
		MetaRemoteToken:       os.Getenv("APP_META_REMOTE_TOKEN"),
		MetaRemoteHeaderName:  os.Getenv("APP_META_REMOTE_HEADER_NAME"),
		MetaRemoteHeaderValue: os.Getenv("APP_META_REMOTE_HEADER_VALUE"),
		MetaRemoteProfile:     os.Getenv("APP_META_REMOTE_PROFILE"),
	}
}

func (c Config) DataDir() string {
	return filepath.Dir(c.DBPath)
}

func (c Config) UsesDefaultSettingsKey() bool {
	return c.SettingsKey == defaultSettingsKey
}

func (c Config) MetaSourceMode() string {
	switch {
	case c.MetaFilePath != "":
		return "file"
	case c.MetaRemoteURL != "" && c.MetaRemoteProfile != "":
		return "remote_profile:" + c.MetaRemoteProfile
	case c.MetaRemoteURL != "":
		return "remote"
	case c.MetaFixture != "":
		return "fixture"
	default:
		return "unconfigured"
	}
}

type Runtime struct {
	Server       *http.Server
	DB           *sql.DB
	Repositories *sqlite.Repositories
	Settings     *settings.Service
	Jobs         *jobs.Service
	Scheduler    *jobs.Engine
}

func Bootstrap(ctx context.Context, cfg Config) (*Runtime, error) {
	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := sqlite.Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	repos := sqlite.NewRepositories(db)
	settingsCodec, err := settings.NewAESGCMCodec(cfg.SettingsKey)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("build settings codec: %w", err)
	}
	settingsService := settings.NewService(repos.Settings, settingsCodec)
	cardsService := cardquery.NewService(repos.Cards)
	decksService := decks.NewParser(sqlite.NewCardLookupRepository(db))
	analysisService := deckanalysis.NewService(decksService, analysis.NewAnalyzer())
	compareService := comparepkg.NewService(decksService, repos.MetaSnapshots, repos.MetaDecks, repos.Cards, repos.DeckCards)
	reportService := reportpkg.NewServiceWithPersistence(
		decksService,
		analysisService,
		compareService,
		settingsService,
		reportpkg.NewOpenAICompatibleProvider(nil),
		reportDeckStore{repo: repos.Decks},
		reportStore{repo: repos.AnalysisReports},
	)
	metaQueryService := meta.NewQueryService(repos.MetaSnapshots)
	cardSyncRunner := jobs.RunnerFunc(func(ctx context.Context) (jobs.RunResult, error) {
		source := cards.NewHearthstoneJSONSource(cfg.CardsSourceURL, cfg.CardsLocale, nil)
		summary, err := cards.NewSyncService(repos.Cards, source).Sync(ctx)
		if err != nil {
			return jobs.RunResult{}, err
		}

		affected := int64(summary.CardsUpserted)
		return jobs.RunResult{RecordsAffected: &affected}, nil
	})
	metaSyncRunner := jobs.RunnerFunc(func(ctx context.Context) (jobs.RunResult, error) {
		source := buildMetaSource(cfg)
		summary, err := meta.NewSyncServiceWithDeckPersistence(repos.MetaSnapshots, repos.Decks, repos.MetaDecks, repos.Cards, repos.DeckCards, source).Sync(ctx)
		if err != nil {
			return jobs.RunResult{}, err
		}

		_ = summary
		affected := int64(1)
		return jobs.RunResult{RecordsAffected: &affected}, nil
	})
	runners := map[string]jobs.Runner{
		jobs.KeySyncCards: cardSyncRunner,
		jobs.KeySyncMeta:  metaSyncRunner,
	}
	executionGate := jobs.NewExecutionGate()
	schedulerEngine := jobs.NewEngine(repos.ScheduledJobs, repos.JobExecutionLogs, runners)
	jobsService := jobs.NewService(repos.ScheduledJobs, repos.JobExecutionLogs, runners)
	schedulerEngine.SetExecutionGate(executionGate)
	jobsService.SetExecutionGate(executionGate)
	jobsService.SetReloadHook(func(ctx context.Context) error {
		return schedulerEngine.Reload(ctx, time.Now().UTC())
	})

	if err := schedulerEngine.Reload(ctx, time.Now().UTC()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize scheduler: %w", err)
	}

	return &Runtime{
		Server:       NewServer(cfg, httpapi.Dependencies{Settings: settingsService, Cards: cardsService, Decks: decksService, Analysis: analysisService, Compare: compareService, Reports: reportService, Jobs: jobsService, Meta: metaQueryService}),
		DB:           db,
		Repositories: repos,
		Settings:     settingsService,
		Jobs:         jobsService,
		Scheduler:    schedulerEngine,
	}, nil
}

func NewServer(cfg Config, deps httpapi.Dependencies) *http.Server {
	return &http.Server{
		Addr:    cfg.Addr,
		Handler: httpapi.NewRouter(web.DistFS, deps),
	}
}

func buildMetaSource(cfg Config) meta.Source {
	if cfg.MetaFilePath != "" {
		return meta.NewFileSource(cfg.MetaFilePath)
	}

	if cfg.MetaRemoteURL != "" {
		if cfg.MetaRemoteProfile == "vicioussyndicate" {
			return meta.NewViciousSyndicateSource(cfg.MetaRemoteURL)
		}
		if cfg.MetaRemoteProfile == "hearthstonetopdecks" {
			return meta.NewHearthstoneTopDecksSource(cfg.MetaRemoteURL)
		}

		return meta.NewRemoteSourceWithOptions(cfg.MetaRemoteURL, meta.RemoteSourceOptions{
			BearerToken: cfg.MetaRemoteToken,
			HeaderName:  cfg.MetaRemoteHeaderName,
			HeaderValue: cfg.MetaRemoteHeaderValue,
		})
	}

	if cfg.MetaFixture != "" {
		return meta.NewFixtureSource(meta.FetchResult{
			Source:       "fixture",
			PatchVersion: cfg.MetaFixture,
			Format:       "standard",
			FetchedAt:    time.Now().UTC(),
			RawPayload:   `{"fixture":true}`,
		})
	}

	return meta.UnavailableSource{}
}

type reportDeckStore struct {
	repo *sqlite.DecksRepository
}

func (s reportDeckStore) UpsertReportDeck(ctx context.Context, deck reportpkg.DeckRecord) (string, error) {
	deckID := buildReportDeckID(deck.DeckHash)
	var deckCodePtr, archetypePtr, deckHashPtr *string
	if deck.DeckCode != "" {
		deckCodePtr = &deck.DeckCode
	}
	if deck.Archetype != "" {
		archetypePtr = &deck.Archetype
	}
	if deck.DeckHash != "" {
		deckHashPtr = &deck.DeckHash
	}

	return s.repo.UpsertReportDeck(ctx, sqlite.Deck{
		ID:        deckID,
		Source:    deck.Source,
		Class:     deck.Class,
		Format:    deck.Format,
		DeckCode:  deckCodePtr,
		Archetype: archetypePtr,
		DeckHash:  deckHashPtr,
	})
}

type reportStore struct {
	repo *sqlite.AnalysisReportsRepository
}

func (s reportStore) Create(ctx context.Context, record reportpkg.ReportRecord) error {
	return s.repo.Create(ctx, sqlite.AnalysisReport{
		ID:                record.ID,
		DeckID:            record.DeckID,
		BasedOnSnapshotID: record.BasedOnSnapshotID,
		ReportType:        record.ReportType,
		InputHash:         record.InputHash,
		ReportJSON:        record.ReportJSON,
		ReportText:        record.ReportText,
	})
}

func (s reportStore) ListReports(ctx context.Context, limit int) ([]reportpkg.StoredReport, error) {
	items, err := s.repo.ListRecent(ctx, limit)
	if err != nil {
		return nil, err
	}

	out := make([]reportpkg.StoredReport, 0, len(items))
	for _, item := range items {
		out = append(out, reportpkg.StoredReport{
			ID:                item.ID,
			DeckID:            item.DeckID,
			BasedOnSnapshotID: item.BasedOnSnapshotID,
			ReportType:        item.ReportType,
			ReportJSON:        item.ReportJSON,
			ReportText:        item.ReportText,
			CreatedAt:         item.CreatedAt,
		})
	}
	return out, nil
}

func (s reportStore) GetReport(ctx context.Context, id string) (reportpkg.StoredReport, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return reportpkg.StoredReport{}, err
	}

	return reportpkg.StoredReport{
		ID:                item.ID,
		DeckID:            item.DeckID,
		BasedOnSnapshotID: item.BasedOnSnapshotID,
		ReportType:        item.ReportType,
		ReportJSON:        item.ReportJSON,
		ReportText:        item.ReportText,
		CreatedAt:         item.CreatedAt,
	}, nil
}

func buildReportDeckID(deckHash string) string {
	sum := sha1.Sum([]byte(deckHash))
	return fmt.Sprintf("report_deck_%x", sum[:8])
}
