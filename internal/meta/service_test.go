package meta

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hearthstone-analyzer/internal/cards"
	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

func TestSyncServicePersistsSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMetaTestDB(t)
	repo := sqliteStore.NewMetaSnapshotsRepository(db)
	service := NewSyncService(repo, stubSource{
		result: FetchResult{
			Source:       "stub",
			PatchVersion: "32.0.1",
			Format:       "standard",
			FetchedAt:    time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
			RawPayload:   `{"decks":[]}`,
		},
	})

	summary, err := service.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if summary.Source != "stub" || summary.Format != "standard" {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	latest, err := repo.GetLatestByFormat(ctx, "standard")
	if err != nil {
		t.Fatalf("GetLatestByFormat() error = %v", err)
	}

	if latest.Source != "stub" || latest.PatchVersion != "32.0.1" {
		t.Fatalf("unexpected stored snapshot: %+v", latest)
	}
}

func TestSyncServiceReturnsSourceFailureWithoutPersisting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMetaTestDB(t)
	repo := sqliteStore.NewMetaSnapshotsRepository(db)
	service := NewSyncService(repo, stubSource{
		err: errors.New("meta source unavailable"),
	})

	if _, err := service.Sync(ctx); err == nil {
		t.Fatal("expected Sync() to return source error")
	}

	_, err := repo.GetLatestByFormat(ctx, "standard")
	if err == nil {
		t.Fatal("expected no snapshot to be persisted on source failure")
	}
}

func TestFixtureSourceReturnsConfiguredSnapshot(t *testing.T) {
	t.Parallel()

	source := NewFixtureSource(FetchResult{
		Source:       "fixture",
		PatchVersion: "32.1.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 9, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[{"name":"Tempo Rogue"}]}`,
	})

	result, err := source.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}

	if result.Source != "fixture" || result.PatchVersion != "32.1.0" {
		t.Fatalf("unexpected fixture result: %+v", result)
	}
}

func TestFileSourceReadsSnapshotFromJSONFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "meta.json")
	content := `{
  "patch_version": "32.2.0",
  "format": "standard",
  "rank_bracket": "diamond",
  "region": "APAC",
  "fetched_at": "2026-03-26T09:30:00Z"
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	source := NewFileSource(path)
	result, err := source.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}

	if result.Source != "file" || result.PatchVersion != "32.2.0" || result.Format != "standard" {
		t.Fatalf("unexpected file source result: %+v", result)
	}

	if result.RawPayload == "" {
		t.Fatal("expected raw payload to be preserved")
	}
}

func TestFileSourceReturnsErrorWhenFileMissing(t *testing.T) {
	t.Parallel()

	source := NewFileSource(filepath.Join(t.TempDir(), "missing.json"))
	if _, err := source.FetchSnapshot(context.Background()); err == nil {
		t.Fatal("expected FetchSnapshot() to fail for missing file")
	}
}

func TestRemoteSourceFetchesSnapshotFromHTTP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/meta.json" {
			t.Fatalf("expected request path /meta.json, got %q", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "source": "remote",
  "patch_version": "32.4.0",
  "format": "standard",
  "rank_bracket": "legend",
  "region": "APAC",
  "fetched_at": "2026-03-26T11:30:00Z",
  "decks": [{"name":"Cycle Rogue"}]
}`))
	}))
	defer server.Close()

	source := NewRemoteSource(server.URL + "/meta.json")
	result, err := source.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}

	if result.Source != "remote" || result.PatchVersion != "32.4.0" || result.Format != "standard" {
		t.Fatalf("unexpected remote source result: %+v", result)
	}

	if result.RawPayload == "" {
		t.Fatal("expected remote raw payload to be preserved")
	}
}

func TestRemoteSourceCanSendAuthorizationAndCustomHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("expected bearer token header, got %q", got)
		}
		if got := r.Header.Get("X-Meta-Key"); got != "meta-value" {
			t.Fatalf("expected custom header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"patch_version":"32.4.1","format":"standard"}`))
	}))
	defer server.Close()

	source := NewRemoteSourceWithOptions(server.URL, RemoteSourceOptions{
		BearerToken: "secret-token",
		HeaderName:  "X-Meta-Key",
		HeaderValue: "meta-value",
	})

	if _, err := source.FetchSnapshot(context.Background()); err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}
}

func TestViciousSyndicateSourceFetchesLatestReportSnapshot(t *testing.T) {
	t.Parallel()

	var baseURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tag/meta/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`
				<html><body>
					<h3><a href="` + baseURL + `/vs-data-reaper-report-344/">vS Data Reaper Report #344</a></h3>
					<h3><a href="` + baseURL + `/vs-data-reaper-report-343/">vS Data Reaper Report #343</a></h3>
				</body></html>
			`))
		case "/vs-data-reaper-report-344/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`
				<html><body>
					<h1>vS Data Reaper Report #344</h1>
					<time datetime="2026-02-19T00:00:00Z">February 19, 2026</time>
					<a href="` + baseURL + `/deck-library/demon-hunter-decks/broxigar-demon-hunter/">Broxigar Demon Hunter</a>
					<p>Overall 2,233,000</p>
				</body></html>
			`))
		case "/deck-library/demon-hunter-decks/broxigar-demon-hunter/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`
				<html><body>
					<h1>Broxigar Demon Hunter</h1>
					<a href="` + baseURL + `/decks/blob-broxigar-demon-hunter-2/">Blob Broxigar Demon Hunter</a>
				</body></html>
			`))
		case "/decks/blob-broxigar-demon-hunter-2/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`
				<html><body>
					<h1>Blob Broxigar Demon Hunter</h1>
					<p>POSTED BY: ZachO | PUBLISHED: February 18, 2026 | DUST COST: 10580</p>
					<p>CLASS:Demon Hunter | Format:Standard | Era:Across the Timeways</p>
					<p>Copy Deck</p>
					<ul>
						<li>1 Illidari Studies 2 CORE</li>
						<li>2 Broxigar CORE</li>
					</ul>
				</body></html>
			`))
		default:
			http.NotFound(w, r)
		}
	}))
	baseURL = server.URL
	defer server.Close()

	source := NewViciousSyndicateSource(server.URL + "/tag/meta/")
	result, err := source.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}

	if result.Source != "vicioussyndicate" || result.Format != "standard" {
		t.Fatalf("unexpected source result: %+v", result)
	}

	if result.RawPayload == "" || result.FetchedAt.IsZero() {
		t.Fatalf("expected metadata payload and fetched date, got %+v", result)
	}

	if !strings.Contains(result.RawPayload, "Blob Broxigar Demon Hunter") {
		t.Fatalf("expected raw payload to include extracted deck metadata, got %s", result.RawPayload)
	}
}

func TestHearthstoneTopDecksSourceFetchesTieredDeckSnapshot(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/standard-meta/" {
			t.Fatalf("expected request path /standard-meta/, got %q", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`
			<html><body>
				<h1>Best Hearthstone Decks - Standard Meta Tier List - Across the Timeways</h1>
				<h2>Tier 1 (Best) Decks</h2>
				<a href="/decks/cycle-rogue/" class="class-header rogue-header"><h2>Cycle Rogue - Standard Meta Tier List March 2026</h2></a>
				<ul>
					<li><span class="card-cost">0</span><a href="/cards/backstab/"><span class="card-name">Backstab</span></a><span class="card-count">2</span><img /></li>
					<li><span class="card-cost">2</span><a href="/cards/quick-pick/"><span class="card-name">Quick Pick</span></a><span class="card-count">2</span><img /></li>
				</ul>
				<h2>Tier 2 (Good) Decks</h2>
				<a href="/decks/rainbow-death-knight/" class="class-header death-knight-header"><h2>Rainbow Death Knight - Standard Meta Tier List March 2026</h2></a>
				<ul>
					<li><span class="card-cost">1</span><a href="/cards/morbid-swarm/"><span class="card-name">Morbid Swarm</span></a><span class="card-count">2</span><img /></li>
				</ul>
			</body></html>
		`))
	}))
	defer server.Close()

	source := NewHearthstoneTopDecksSource(server.URL + "/standard-meta/")
	result, err := source.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}

	if result.Source != "hearthstonetopdecks" || result.Format != "standard" {
		t.Fatalf("unexpected source result: %+v", result)
	}

	if !strings.Contains(result.RawPayload, `"tier":"T1"`) || !strings.Contains(result.RawPayload, `"tier":"T2"`) {
		t.Fatalf("expected tier labels in payload, got %s", result.RawPayload)
	}

	if !strings.Contains(result.RawPayload, `2x Backstab`) || !strings.Contains(result.RawPayload, `2x Quick Pick`) {
		t.Fatalf("expected card lines in payload, got %s", result.RawPayload)
	}

	if !strings.Contains(result.RawPayload, `"class":"ROGUE"`) || !strings.Contains(result.RawPayload, `"class":"DEATHKNIGHT"`) {
		t.Fatalf("expected inferred classes in payload, got %s", result.RawPayload)
	}
}

func TestHearthstoneTopDecksSourceIgnoresSideboardCards(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`
			<html><body>
				<h1>Best Hearthstone Decks - Standard Meta Tier List</h1>
				<h2>Tier 1 (Best) Decks</h2>
				<a href="/decks/aura-paladin/" class="class-header paladin-header"><h2>Aura Paladin - Standard Meta Tier List March 2026</h2></a>
				<ul>
					<li><span class="card-cost">1</span><a href="/cards/righteous-protector/"><span class="card-name">Righteous Protector</span></a><span class="card-count">2</span><img /></li>
					<li><span class="card-cost">0</span><a href="/cards/zilliax-deluxe-3000/"><span class="card-name">Zilliax Deluxe 3000</span></a><span class="card-count">1</span><img /></li>
				</ul>
				<h3>Sideboard</h3>
				<ul>
					<li><span class="card-cost">4</span><a href="/cards/twin-module/"><span class="card-name">Twin Module</span></a><span class="card-count">1</span><img /></li>
					<li><span class="card-cost">5</span><a href="/cards/perfect-module/"><span class="card-name">Perfect Module</span></a><span class="card-count">1</span><img /></li>
				</ul>
			</body></html>
		`))
	}))
	defer server.Close()

	source := NewHearthstoneTopDecksSource(server.URL)
	result, err := source.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}

	if strings.Contains(result.RawPayload, "Twin Module") || strings.Contains(result.RawPayload, "Perfect Module") {
		t.Fatalf("expected sideboard cards to be excluded from payload, got %s", result.RawPayload)
	}
}

func TestTrimHTDDeckTitleRemovesVariableMetaSuffix(t *testing.T) {
	t.Parallel()

	got := trimHTDDeckTitle("Cycle Rogue - Standard Meta Tier List March 2026")
	if got != "Cycle Rogue" {
		t.Fatalf("expected generic meta suffix to be trimmed, got %q", got)
	}
}

func TestQueryServiceListsSnapshotsByFormat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMetaTestDB(t)
	repo := sqliteStore.NewMetaSnapshotsRepository(db)
	service := NewQueryService(repo)

	for _, snapshot := range []sqliteStore.MetaSnapshot{
		{
			ID:           "meta_1",
			Source:       "fixture",
			PatchVersion: "32.0.0",
			Format:       "standard",
			FetchedAt:    time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC),
			RawPayload:   `{"decks":[1]}`,
		},
		{
			ID:           "meta_2",
			Source:       "fixture",
			PatchVersion: "32.0.1",
			Format:       "standard",
			FetchedAt:    time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
			RawPayload:   `{"decks":[2]}`,
		},
	} {
		if err := repo.Create(ctx, snapshot); err != nil {
			t.Fatalf("Create(%q) error = %v", snapshot.ID, err)
		}
	}

	list, err := service.ListSnapshots(ctx, "standard", 10)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}

	if len(list) != 2 || list[0].ID != "meta_2" || list[1].ID != "meta_1" {
		t.Fatalf("unexpected snapshot list: %+v", list)
	}
}

func TestQueryServiceGetsSnapshotByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMetaTestDB(t)
	repo := sqliteStore.NewMetaSnapshotsRepository(db)
	service := NewQueryService(repo)

	if err := repo.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           "meta_detail",
		Source:       "fixture",
		PatchVersion: "32.2.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[{"name":"Overheal Priest"}]}`,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	snapshot, err := service.GetSnapshotByID(ctx, "meta_detail")
	if err != nil {
		t.Fatalf("GetSnapshotByID() error = %v", err)
	}

	if snapshot.ID != "meta_detail" || snapshot.RawPayload == "" {
		t.Fatalf("unexpected snapshot detail: %+v", snapshot)
	}
}

func TestSyncServicePersistsMetaDeckMappingsFromPayload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMetaTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	service := NewSyncServiceWithDeckPersistence(repos.MetaSnapshots, repos.Decks, repos.MetaDecks, repos.Cards, repos.DeckCards, stubSource{
		result: FetchResult{
			Source:       "remote",
			PatchVersion: "32.4.0",
			Format:       "standard",
			FetchedAt:    time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC),
			RawPayload: `{
  "decks": [
    {
      "id": "cycle-rogue",
      "name": "Cycle Rogue",
      "class": "ROGUE",
      "archetype": "Combo",
      "playrate": 0.12,
      "winrate": 0.51,
      "sample_size": 2450,
      "tier": "T1"
    }
  ]
}`,
		},
	})

	summary, err := service.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	items, err := repos.MetaDecks.ListBySnapshotID(ctx, summary.ID)
	if err != nil {
		t.Fatalf("ListBySnapshotID() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 persisted meta deck, got %d", len(items))
	}

	if items[0].Playrate == nil || *items[0].Playrate != 12 {
		t.Fatalf("expected normalized playrate, got %+v", items[0])
	}
}

func TestSyncServiceNormalizesAlternateMetaDeckSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMetaTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	service := NewSyncServiceWithDeckPersistence(repos.MetaSnapshots, repos.Decks, repos.MetaDecks, repos.Cards, repos.DeckCards, stubSource{
		result: FetchResult{
			Source:       "remote",
			PatchVersion: "32.5.0",
			Format:       "standard",
			FetchedAt:    time.Date(2026, 3, 26, 15, 0, 0, 0, time.UTC),
			RawPayload: `{
  "meta_decks": [
    {
      "deck_id": "tempo-mage",
      "deck_name": "Tempo Mage",
      "cls": "MAGE",
      "deckCode": "AAEAAA==",
      "wr": 55.5,
      "pr": 8.2,
      "games": 3200,
      "tier_label": "T2"
    }
  ]
}`,
		},
	})

	summary, err := service.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	items, err := repos.MetaDecks.ListComparableBySnapshotID(ctx, summary.ID)
	if err != nil {
		t.Fatalf("ListComparableBySnapshotID() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 normalized meta deck, got %d", len(items))
	}

	if items[0].Name == nil || *items[0].Name != "Tempo Mage" || items[0].Class != "MAGE" {
		t.Fatalf("expected normalized name/class, got %+v", items[0])
	}

	if items[0].Winrate == nil || *items[0].Winrate != 55.5 || items[0].Playrate == nil || *items[0].Playrate != 8.2 {
		t.Fatalf("expected normalized rates, got %+v", items[0])
	}
}

func TestSyncServicePersistsDeckCardsFromSnapshotCardLines(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMetaTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		{
			ID:            "CARD_001",
			DBFID:         1,
			Class:         "DEMONHUNTER",
			CardType:      "SPELL",
			Set:           "CORE",
			Rarity:        "FREE",
			Cost:          1,
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales:       []cards.LocaleText{{Locale: "enUS", Name: "Illidari Studies"}},
		},
		{
			ID:            "CARD_002",
			DBFID:         2,
			Class:         "DEMONHUNTER",
			CardType:      "MINION",
			Set:           "CORE",
			Rarity:        "LEGENDARY",
			Cost:          2,
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales:       []cards.LocaleText{{Locale: "enUS", Name: "Broxigar"}},
		},
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	service := NewSyncServiceWithDeckPersistence(repos.MetaSnapshots, repos.Decks, repos.MetaDecks, repos.Cards, repos.DeckCards, stubSource{
		result: FetchResult{
			Source:       "vicioussyndicate",
			PatchVersion: "vS Data Reaper Report #344",
			Format:       "standard",
			FetchedAt:    time.Date(2026, 3, 26, 16, 0, 0, 0, time.UTC),
			RawPayload: `{
  "decks": [
    {
      "external_ref": "https://example.test/decks/blob-broxigar-demon-hunter-2/",
      "name": "Blob Broxigar Demon Hunter",
      "class": "DEMONHUNTER",
      "cards": ["2x Illidari Studies", "1x Broxigar"]
    }
  ]
}`,
		},
	})

	summary, err := service.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	comparable, err := repos.MetaDecks.ListComparableBySnapshotID(ctx, summary.ID)
	if err != nil {
		t.Fatalf("ListComparableBySnapshotID() error = %v", err)
	}
	if len(comparable) != 0 {
		t.Fatalf("expected no comparable deck_code rows, got %+v", comparable)
	}

	deckRows, err := repos.MetaDecks.ListBySnapshotID(ctx, summary.ID)
	if err != nil || len(deckRows) != 1 {
		t.Fatalf("expected one meta deck row, got rows=%+v err=%v", deckRows, err)
	}

	deckCards, err := repos.DeckCards.ListByDeckID(ctx, deckRows[0].DeckID)
	if err != nil {
		t.Fatalf("ListByDeckID() error = %v", err)
	}

	if len(deckCards) != 2 {
		t.Fatalf("expected 2 persisted deck cards, got %+v", deckCards)
	}
}

func TestSyncServicePersistsDeckCardsFromNormalizedSnapshotCardLines(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMetaTestDB(t)
	repos := sqliteStore.NewRepositories(db)
	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		{
			ID:            "CARD_010",
			DBFID:         10,
			Class:         "PRIEST",
			CardType:      "SPELL",
			Set:           "WORKSHOP",
			Rarity:        "RARE",
			Cost:          3,
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales:       []cards.LocaleText{{Locale: "enUS", Name: "Pop-Up Book"}},
		},
		{
			ID:            "CARD_011",
			DBFID:         11,
			Class:         "PRIEST",
			CardType:      "MINION",
			Set:           "CORE",
			Rarity:        "LEGENDARY",
			Cost:          4,
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales:       []cards.LocaleText{{Locale: "enUS", Name: "Maiev Shadowsong"}},
		},
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	service := NewSyncServiceWithDeckPersistence(repos.MetaSnapshots, repos.Decks, repos.MetaDecks, repos.Cards, repos.DeckCards, stubSource{
		result: FetchResult{
			Source:       "remote",
			PatchVersion: "32.6.0",
			Format:       "standard",
			FetchedAt:    time.Date(2026, 3, 26, 17, 0, 0, 0, time.UTC),
			RawPayload: `{
  "decks": [
    {
      "external_ref": "https://example.test/decks/priest/",
      "name": "Value Priest",
      "class": "PRIEST",
      "cards": ["2x Pop Up Book", "1x Maiev Shadowsong (Core)"]
    }
  ]
}`,
		},
	})

	summary, err := service.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	deckRows, err := repos.MetaDecks.ListBySnapshotID(ctx, summary.ID)
	if err != nil || len(deckRows) != 1 {
		t.Fatalf("expected one meta deck row, got rows=%+v err=%v", deckRows, err)
	}

	deckCards, err := repos.DeckCards.ListByDeckID(ctx, deckRows[0].DeckID)
	if err != nil {
		t.Fatalf("ListByDeckID() error = %v", err)
	}

	if len(deckCards) != 2 {
		t.Fatalf("expected normalized card lines to persist 2 deck cards, got %+v", deckCards)
	}
}

func openMetaTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "meta.db")
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

type stubSource struct {
	result FetchResult
	err    error
}

func (s stubSource) FetchSnapshot(ctx context.Context) (FetchResult, error) {
	return s.result, s.err
}
