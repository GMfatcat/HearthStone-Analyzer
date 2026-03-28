package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"hearthstone-analyzer/internal/analysis"
	"hearthstone-analyzer/internal/cards"
	comparepkg "hearthstone-analyzer/internal/compare"
	"hearthstone-analyzer/internal/decks"
	"hearthstone-analyzer/internal/jobs"
	"hearthstone-analyzer/internal/meta"
	reportpkg "hearthstone-analyzer/internal/report"
	"hearthstone-analyzer/internal/settings"
)

type SettingsService interface {
	List(ctx context.Context) ([]settings.Setting, error)
	Get(ctx context.Context, key string) (settings.Setting, error)
	Upsert(ctx context.Context, input settings.Input) error
}

type Dependencies struct {
	Settings SettingsService
	Cards    CardsService
	Decks    DecksService
	Analysis AnalysisService
	Compare  CompareService
	Reports  ReportsService
	Jobs     JobsService
	Meta     MetaService
}

type cardsResponse struct {
	ID       string `json:"id"`
	Class    string `json:"class"`
	CardType string `json:"card_type"`
	Cost     int    `json:"cost"`
	Name     string `json:"name"`
	Text     string `json:"text"`
}

type CardsService interface {
	List(ctx context.Context, filter cards.ListFilter) ([]cards.Summary, error)
	GetByID(ctx context.Context, id string) (cards.Summary, error)
}

type DecksService interface {
	Parse(ctx context.Context, deckCode string) (decks.ParseResult, error)
}

type AnalysisService interface {
	AnalyzeDeck(ctx context.Context, deckCode string) (analysis.Result, error)
}

type CompareService interface {
	CompareDeck(ctx context.Context, deckCode string, limit int) (comparepkg.Result, error)
}

type ReportsService interface {
	GenerateDeckReport(ctx context.Context, deckCode string, language string) (reportpkg.Result, error)
	ListReports(ctx context.Context, limit int) ([]reportpkg.StoredReport, error)
	GetReport(ctx context.Context, id string) (reportpkg.ReportDetail, error)
}

type JobsService interface {
	List(ctx context.Context) ([]jobs.Job, error)
	Get(ctx context.Context, key string) (jobs.Job, error)
	Update(ctx context.Context, input jobs.UpdateInput) (jobs.Job, error)
	RunNow(ctx context.Context, key string) error
	History(ctx context.Context, key string, limit int) ([]jobs.Execution, error)
}

type MetaService interface {
	GetLatestSnapshot(ctx context.Context, format string) (meta.Snapshot, error)
	ListSnapshots(ctx context.Context, format string, limit int) ([]meta.Snapshot, error)
	GetSnapshotByID(ctx context.Context, id string) (meta.SnapshotDetail, error)
}

type settingsResponse struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Sensitive   bool   `json:"sensitive"`
	Description string `json:"description"`
}

type upsertSettingRequest struct {
	Value string `json:"value"`
}

type parseDeckRequest struct {
	DeckCode string `json:"deck_code"`
}

type parseDeckResponse struct {
	Class      string           `json:"class"`
	Format     int              `json:"format"`
	TotalCount int              `json:"total_count"`
	DeckHash   string           `json:"deck_hash"`
	Legality   decks.Legality   `json:"legality"`
	Cards      []decks.DeckCard `json:"cards"`
}

type analysisFeaturesResponse struct {
	AvgCost            float64        `json:"avg_cost"`
	ManaCurve          map[string]int `json:"mana_curve"`
	MinionCount        int            `json:"minion_count"`
	SpellCount         int            `json:"spell_count"`
	WeaponCount        int            `json:"weapon_count"`
	EarlyCurveCount    int            `json:"early_curve_count"`
	TopHeavyCount      int            `json:"top_heavy_count"`
	DrawCount          int            `json:"draw_count"`
	DiscoverCount      int            `json:"discover_count"`
	SingleRemovalCount int            `json:"single_removal_count"`
	AoeCount           int            `json:"aoe_count"`
	HealCount          int            `json:"heal_count"`
	BurnCount          int            `json:"burn_count"`
	TauntCount         int            `json:"taunt_count"`
	TokenCount         int            `json:"token_count"`
	DeathrattleCount   int            `json:"deathrattle_count"`
	BattlecryCount     int            `json:"battlecry_count"`
	ManaCheatCount     int            `json:"mana_cheat_count"`
	ComboPieceCount    int            `json:"combo_piece_count"`
	EarlyGameScore     float64        `json:"early_game_score"`
	MidGameScore       float64        `json:"mid_game_score"`
	LateGameScore      float64        `json:"late_game_score"`
	CurveBalanceScore  float64        `json:"curve_balance_score"`
}

type analyzeDeckResponse struct {
	Archetype             string                          `json:"archetype"`
	Confidence            float64                         `json:"confidence"`
	ConfidenceReasons     []string                        `json:"confidence_reasons,omitempty"`
	Features              analysisFeaturesResponse        `json:"features"`
	FunctionalRoleSummary []functionalRoleSummaryResponse `json:"functional_role_summary,omitempty"`
	Strengths             []string                        `json:"strengths"`
	Weaknesses            []string                        `json:"weaknesses"`
	StructuralTags        []string                        `json:"structural_tags,omitempty"`
	StructuralTagDetails  []structuralTagDetailResponse   `json:"structural_tag_details,omitempty"`
	PackageDetails        []packageDetailResponse         `json:"package_details,omitempty"`
	SuggestedAdds         []string                        `json:"suggested_adds,omitempty"`
	SuggestedCuts         []string                        `json:"suggested_cuts,omitempty"`
}

type structuralTagDetailResponse struct {
	Tag         string `json:"tag"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
}

type packageDetailResponse struct {
	Package     string `json:"package"`
	Parent      string `json:"parent,omitempty"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Slots       int    `json:"slots"`
	TargetMin   int    `json:"target_min,omitempty"`
	TargetMax   int    `json:"target_max,omitempty"`
	Explanation string `json:"explanation"`
}

type functionalRoleSummaryResponse struct {
	Role        string `json:"role"`
	Label       string `json:"label"`
	Count       int    `json:"count"`
	Explanation string `json:"explanation"`
}

type reportGenerateResponse struct {
	ReportID    string                    `json:"report_id,omitempty"`
	Report      string                    `json:"report"`
	Model       string                    `json:"model"`
	GeneratedAt time.Time                 `json:"generated_at"`
	Analysis    analyzeDeckResponse       `json:"analysis"`
	Structured  *structuredReportResponse `json:"structured,omitempty"`
	Compare     *compareDeckResponse      `json:"compare,omitempty"`
}

type structuredReportResponse struct {
	DeckIdentity             []string `json:"deck_identity,omitempty"`
	WhatTheDeckIsDoingWell   []string `json:"what_the_deck_is_doing_well,omitempty"`
	MainRisks                []string `json:"main_risks,omitempty"`
	PracticalNextAdjustments []string `json:"practical_next_adjustments,omitempty"`
}

type reportListItemResponse struct {
	ID                string    `json:"id"`
	DeckID            string    `json:"deck_id"`
	BasedOnSnapshotID *string   `json:"based_on_snapshot_id,omitempty"`
	ReportType        string    `json:"report_type"`
	ReportText        *string   `json:"report_text,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type reportDetailResponse struct {
	ID                string                 `json:"id"`
	DeckID            string                 `json:"deck_id"`
	BasedOnSnapshotID *string                `json:"based_on_snapshot_id,omitempty"`
	ReportType        string                 `json:"report_type"`
	CreatedAt         time.Time              `json:"created_at"`
	Result            reportGenerateResponse `json:"result"`
	Compare           *compareDeckResponse   `json:"compare,omitempty"`
}

type compareDeckRequest struct {
	DeckCode string `json:"deck_code"`
	Limit    int    `json:"limit"`
}

type generateReportRequest struct {
	DeckCode string `json:"deck_code"`
	Language string `json:"language,omitempty"`
}

type compareCandidateResponse struct {
	DeckID           string                    `json:"deck_id"`
	Name             string                    `json:"name"`
	Class            string                    `json:"class"`
	Archetype        string                    `json:"archetype,omitempty"`
	Similarity       float64                   `json:"similarity"`
	Breakdown        compareSimilarityResponse `json:"breakdown"`
	Summary          []string                  `json:"summary,omitempty"`
	Winrate          *float64                  `json:"winrate,omitempty"`
	Playrate         *float64                  `json:"playrate,omitempty"`
	SampleSize       *int                      `json:"sample_size,omitempty"`
	Tier             *string                   `json:"tier,omitempty"`
	SharedCards      []compareCardDiffResponse `json:"shared_cards,omitempty"`
	MissingFromInput []compareCardDiffResponse `json:"missing_from_input,omitempty"`
	MissingFromMeta  []compareCardDiffResponse `json:"missing_from_meta,omitempty"`
	SuggestedAdds    []string                  `json:"suggested_adds,omitempty"`
	SuggestedCuts    []string                  `json:"suggested_cuts,omitempty"`
}

type compareCardDiffResponse struct {
	CardID     string `json:"card_id"`
	Name       string `json:"name"`
	InputCount int    `json:"input_count"`
	MetaCount  int    `json:"meta_count"`
}

type compareSimilarityResponse struct {
	Total    float64 `json:"total"`
	Overlap  float64 `json:"overlap"`
	Curve    float64 `json:"curve"`
	CardType float64 `json:"card_type"`
}

type compareDeckResponse struct {
	SnapshotID          string                     `json:"snapshot_id"`
	PatchVersion        string                     `json:"patch_version"`
	Format              string                     `json:"format"`
	MergedSummary       []string                   `json:"merged_summary,omitempty"`
	MergedSuggestedAdds []string                   `json:"merged_suggested_adds,omitempty"`
	MergedSuggestedCuts []string                   `json:"merged_suggested_cuts,omitempty"`
	MergedGuidance      *compareGuidanceResponse   `json:"merged_guidance,omitempty"`
	Candidates          []compareCandidateResponse `json:"candidates"`
}

type compareGuidanceResponse struct {
	Summary []compareRecommendationResponse `json:"summary,omitempty"`
	Adds    []compareRecommendationResponse `json:"adds,omitempty"`
	Cuts    []compareRecommendationResponse `json:"cuts,omitempty"`
}

type compareRecommendationResponse struct {
	Key        string                                 `json:"key"`
	Kind       string                                 `json:"kind"`
	Package    string                                 `json:"package,omitempty"`
	Source     string                                 `json:"source"`
	Message    string                                 `json:"message"`
	Confidence float64                                `json:"confidence"`
	Support    []compareRecommendationSupportResponse `json:"support,omitempty"`
}

type compareRecommendationSupportResponse struct {
	Source          string  `json:"source"`
	CandidateDeckID string  `json:"candidate_deck_id,omitempty"`
	CandidateName   string  `json:"candidate_name,omitempty"`
	CandidateRank   int     `json:"candidate_rank,omitempty"`
	Weight          float64 `json:"weight,omitempty"`
	Evidence        string  `json:"evidence"`
}

type apiErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type jobResponse struct {
	Key       string     `json:"key"`
	CronExpr  string     `json:"cron_expr"`
	Enabled   bool       `json:"enabled"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
}

type updateJobRequest struct {
	CronExpr string `json:"cron_expr"`
	Enabled  bool   `json:"enabled"`
}

type jobExecutionResponse struct {
	ID              int64      `json:"id"`
	JobKey          string     `json:"job_key"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	RecordsAffected *int64     `json:"records_affected,omitempty"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
}

type metaSnapshotResponse struct {
	ID           string    `json:"id"`
	Source       string    `json:"source"`
	PatchVersion string    `json:"patch_version"`
	Format       string    `json:"format"`
	RankBracket  *string   `json:"rank_bracket,omitempty"`
	Region       *string   `json:"region,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
}

type metaSnapshotDetailResponse struct {
	ID           string    `json:"id"`
	Source       string    `json:"source"`
	PatchVersion string    `json:"patch_version"`
	Format       string    `json:"format"`
	RankBracket  *string   `json:"rank_bracket,omitempty"`
	Region       *string   `json:"region,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
	RawPayload   string    `json:"raw_payload"`
}

func NewRouter(frontendFS fs.FS, deps Dependencies) http.Handler {
	frontendFS = distRoot(frontendFS)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/api/cards", cardsCollectionHandler(deps.Cards))
	mux.HandleFunc("/api/cards/", cardItemHandler(deps.Cards))
	mux.HandleFunc("/api/decks/analyze", analyzeDeckHandler(deps.Analysis))
	mux.HandleFunc("/api/decks/compare", compareDeckHandler(deps.Compare))
	mux.HandleFunc("/api/decks/parse", parseDeckHandler(deps.Decks))
	mux.HandleFunc("/api/reports", reportsCollectionHandler(deps.Reports))
	mux.HandleFunc("/api/reports/", reportItemHandler(deps.Reports))
	mux.HandleFunc("/api/reports/generate", generateReportHandler(deps.Reports))
	mux.HandleFunc("/api/jobs", jobsCollectionHandler(deps.Jobs))
	mux.HandleFunc("/api/jobs/", jobItemHandler(deps.Jobs))
	mux.HandleFunc("/api/meta", metaCollectionHandler(deps.Meta))
	mux.HandleFunc("/api/meta/", metaItemHandler(deps.Meta))
	mux.HandleFunc("/api/meta/latest", latestMetaSnapshotHandler(deps.Meta))
	mux.HandleFunc("/api/settings", settingsCollectionHandler(deps.Settings))
	mux.HandleFunc("/api/settings/", settingItemHandler(deps.Settings))
	mux.Handle("/", frontendHandler(frontendFS))

	return mux
}

func distRoot(frontendFS fs.FS) fs.FS {
	subFS, err := fs.Sub(frontendFS, "dist")
	if err == nil {
		return subFS
	}

	return frontendFS
}

func frontendHandler(frontendFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(frontendFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))

		switch requestPath {
		case ".", "":
			serveIndex(frontendFS, w, r)
			return
		}

		if _, err := fs.Stat(frontendFS, requestPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		serveIndex(frontendFS, w, r)
	})
}

func serveIndex(frontendFS fs.FS, w http.ResponseWriter, r *http.Request) {
	index, err := fs.ReadFile(frontendFS, "index.html")
	if err != nil {
		http.Error(w, "frontend assets unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}

func settingsCollectionHandler(svc SettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "settings service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		settingsList, err := svc.List(r.Context())
		if err != nil {
			http.Error(w, "failed to list settings", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, mapSettings(settingsList))
	}
}

func settingItemHandler(svc SettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "settings service unavailable", http.StatusServiceUnavailable)
			return
		}

		key := strings.TrimPrefix(r.URL.Path, "/api/settings/")
		if key == "" || strings.Contains(key, "/") {
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			setting, err := svc.Get(r.Context(), key)
			if err != nil {
				if strings.Contains(err.Error(), "unknown setting key") || errors.Is(err, fs.ErrNotExist) {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				http.Error(w, "failed to get setting", http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, toSettingsResponse(setting))
		case http.MethodPut:
			var payload upsertSettingRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid json payload", http.StatusBadRequest)
				return
			}

			if err := svc.Upsert(r.Context(), settings.Input{
				Key:   key,
				Value: payload.Value,
			}); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			setting, err := svc.Get(r.Context(), key)
			if err != nil {
				http.Error(w, "failed to fetch updated setting", http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, toSettingsResponse(setting))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func mapSettings(in []settings.Setting) []settingsResponse {
	out := make([]settingsResponse, 0, len(in))
	for _, setting := range in {
		out = append(out, toSettingsResponse(setting))
	}

	return out
}

func toSettingsResponse(setting settings.Setting) settingsResponse {
	return settingsResponse{
		Key:         setting.Key,
		Value:       setting.Value,
		Sensitive:   setting.Sensitive,
		Description: setting.Description,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func cardsCollectionHandler(svc CardsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "cards service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		filter := cards.ListFilter{
			Class: r.URL.Query().Get("class"),
			Set:   r.URL.Query().Get("set"),
		}
		if costParam := r.URL.Query().Get("cost"); costParam != "" {
			cost, err := strconv.Atoi(costParam)
			if err != nil {
				http.Error(w, "invalid cost filter", http.StatusBadRequest)
				return
			}
			filter.Cost = &cost
		}

		items, err := svc.List(r.Context(), filter)
		if err != nil {
			http.Error(w, "failed to list cards", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, mapCards(items))
	}
}

func cardItemHandler(svc CardsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "cards service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/cards/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}

		card, err := svc.GetByID(r.Context(), id)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				http.Error(w, "card not found", http.StatusNotFound)
				return
			}

			http.Error(w, "failed to get card", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, toCardResponse(card))
	}
}

func mapCards(in []cards.Summary) []cardsResponse {
	out := make([]cardsResponse, 0, len(in))
	for _, item := range in {
		out = append(out, toCardResponse(item))
	}
	return out
}

func toCardResponse(card cards.Summary) cardsResponse {
	return cardsResponse{
		ID:       card.ID,
		Class:    card.Class,
		CardType: card.CardType,
		Cost:     card.Cost,
		Name:     card.Name,
		Text:     card.Text,
	}
}

func parseDeckHandler(svc DecksService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "decks service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload parseDeckRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{
				Code:    "invalid_json",
				Message: "invalid json payload",
			})
			return
		}
		if strings.TrimSpace(payload.DeckCode) == "" {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{
				Code:    "missing_deck_code",
				Message: "deck_code is required",
			})
			return
		}

		result, err := svc.Parse(r.Context(), payload.DeckCode)
		if err != nil {
			if parseErr, ok := decks.AsParseError(err); ok {
				writeJSON(w, http.StatusBadRequest, apiErrorResponse{
					Code:    string(parseErr.Code),
					Message: parseErr.Message,
				})
				return
			}

			writeJSON(w, http.StatusBadRequest, apiErrorResponse{
				Code:    "parse_failed",
				Message: err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, parseDeckResponse{
			Class:      result.Class,
			Format:     result.Format,
			TotalCount: result.TotalCount,
			DeckHash:   result.DeckHash,
			Legality:   result.Legality,
			Cards:      result.Cards,
		})
	}
}

func analyzeDeckHandler(svc AnalysisService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "analysis service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload parseDeckRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{
				Code:    "invalid_json",
				Message: "invalid json payload",
			})
			return
		}
		if strings.TrimSpace(payload.DeckCode) == "" {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{
				Code:    "missing_deck_code",
				Message: "deck_code is required",
			})
			return
		}

		result, err := svc.AnalyzeDeck(r.Context(), payload.DeckCode)
		if err != nil {
			if parseErr, ok := decks.AsParseError(err); ok {
				writeJSON(w, http.StatusBadRequest, apiErrorResponse{
					Code:    string(parseErr.Code),
					Message: parseErr.Message,
				})
				return
			}
			writeJSON(w, http.StatusInternalServerError, apiErrorResponse{
				Code:    "analysis_failed",
				Message: "failed to analyze deck",
			})
			return
		}

		writeJSON(w, http.StatusOK, toAnalyzeDeckResponse(result))
	}
}

func compareDeckHandler(svc CompareService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "compare service unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload compareDeckRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Code: "invalid_json", Message: "invalid json payload"})
			return
		}
		if strings.TrimSpace(payload.DeckCode) == "" {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Code: "missing_deck_code", Message: "deck_code is required"})
			return
		}

		result, err := svc.CompareDeck(r.Context(), payload.DeckCode, payload.Limit)
		if err != nil {
			if parseErr, ok := decks.AsParseError(err); ok {
				writeJSON(w, http.StatusBadRequest, apiErrorResponse{Code: string(parseErr.Code), Message: parseErr.Message})
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				http.Error(w, "meta snapshot not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to compare deck", http.StatusInternalServerError)
			return
		}

		out := compareDeckResponse{
			SnapshotID:          result.SnapshotID,
			PatchVersion:        result.PatchVersion,
			Format:              result.Format,
			MergedSummary:       result.MergedSummary,
			MergedSuggestedAdds: result.MergedSuggestedAdds,
			MergedSuggestedCuts: result.MergedSuggestedCuts,
			MergedGuidance:      toCompareGuidanceResponse(result.MergedGuidance),
			Candidates:          make([]compareCandidateResponse, 0, len(result.Candidates)),
		}
		for _, candidate := range result.Candidates {
			out.Candidates = append(out.Candidates, compareCandidateResponse{
				DeckID:     candidate.DeckID,
				Name:       candidate.Name,
				Class:      candidate.Class,
				Archetype:  candidate.Archetype,
				Similarity: candidate.Similarity,
				Breakdown: compareSimilarityResponse{
					Total:    candidate.Breakdown.Total,
					Overlap:  candidate.Breakdown.Overlap,
					Curve:    candidate.Breakdown.Curve,
					CardType: candidate.Breakdown.CardType,
				},
				Summary:          candidate.Summary,
				Winrate:          candidate.Winrate,
				Playrate:         candidate.Playrate,
				SampleSize:       candidate.SampleSize,
				Tier:             candidate.Tier,
				SharedCards:      toCompareCardDiffResponses(candidate.SharedCards),
				MissingFromInput: toCompareCardDiffResponses(candidate.MissingFromInput),
				MissingFromMeta:  toCompareCardDiffResponses(candidate.MissingFromMeta),
				SuggestedAdds:    candidate.SuggestedAdds,
				SuggestedCuts:    candidate.SuggestedCuts,
			})
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func generateReportHandler(svc ReportsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "report service unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload generateReportRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Code: "invalid_json", Message: "invalid json payload"})
			return
		}
		if strings.TrimSpace(payload.DeckCode) == "" {
			writeJSON(w, http.StatusBadRequest, apiErrorResponse{Code: "missing_deck_code", Message: "deck_code is required"})
			return
		}

		result, err := svc.GenerateDeckReport(r.Context(), payload.DeckCode, payload.Language)
		if err != nil {
			if parseErr, ok := decks.AsParseError(err); ok {
				writeJSON(w, http.StatusBadRequest, apiErrorResponse{Code: string(parseErr.Code), Message: parseErr.Message})
				return
			}
			writeJSON(w, http.StatusInternalServerError, apiErrorResponse{
				Code:    "report_generation_failed",
				Message: err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, reportGenerateResponse{
			ReportID:    result.ReportID,
			Report:      result.Report,
			Model:       result.Model,
			GeneratedAt: result.GeneratedAt,
			Analysis:    toAnalyzeDeckResponse(result.Analysis),
			Structured:  toStructuredReportResponse(result.Structured),
			Compare:     optionalCompareDeckResponse(result.Compare),
		})
	}
}

func reportsCollectionHandler(svc ReportsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "report service unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 20
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsed
		}

		items, err := svc.ListReports(r.Context(), limit)
		if err != nil {
			http.Error(w, "failed to list reports", http.StatusInternalServerError)
			return
		}

		out := make([]reportListItemResponse, 0, len(items))
		for _, item := range items {
			out = append(out, reportListItemResponse{
				ID:                item.ID,
				DeckID:            item.DeckID,
				BasedOnSnapshotID: item.BasedOnSnapshotID,
				ReportType:        item.ReportType,
				ReportText:        item.ReportText,
				CreatedAt:         item.CreatedAt,
			})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func reportItemHandler(svc ReportsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "report service unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/reports/")
		if id == "" || strings.Contains(id, "/") || id == "generate" {
			http.NotFound(w, r)
			return
		}

		item, err := svc.GetReport(r.Context(), id)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				http.Error(w, "report not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to get report", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, toReportDetailResponse(item))
	}
}

func toCompareCardDiffResponses(items []comparepkg.CardDiff) []compareCardDiffResponse {
	out := make([]compareCardDiffResponse, 0, len(items))
	for _, item := range items {
		out = append(out, compareCardDiffResponse{
			CardID:     item.CardID,
			Name:       item.Name,
			InputCount: item.InputCount,
			MetaCount:  item.MetaCount,
		})
	}
	return out
}

func toAnalyzeDeckResponse(result analysis.Result) analyzeDeckResponse {
	manaCurve := make(map[string]int, len(result.Features.ManaCurve))
	for cost, count := range result.Features.ManaCurve {
		manaCurve[strconv.Itoa(cost)] = count
	}

	return analyzeDeckResponse{
		Archetype:         result.Archetype,
		Confidence:        result.Confidence,
		ConfidenceReasons: result.ConfidenceReasons,
		Features: analysisFeaturesResponse{
			AvgCost:            result.Features.AvgCost,
			ManaCurve:          manaCurve,
			MinionCount:        result.Features.MinionCount,
			SpellCount:         result.Features.SpellCount,
			WeaponCount:        result.Features.WeaponCount,
			EarlyCurveCount:    result.Features.EarlyCurveCount,
			TopHeavyCount:      result.Features.TopHeavyCount,
			DrawCount:          result.Features.DrawCount,
			DiscoverCount:      result.Features.DiscoverCount,
			SingleRemovalCount: result.Features.SingleRemovalCount,
			AoeCount:           result.Features.AoeCount,
			HealCount:          result.Features.HealCount,
			BurnCount:          result.Features.BurnCount,
			TauntCount:         result.Features.TauntCount,
			TokenCount:         result.Features.TokenCount,
			DeathrattleCount:   result.Features.DeathrattleCount,
			BattlecryCount:     result.Features.BattlecryCount,
			ManaCheatCount:     result.Features.ManaCheatCount,
			ComboPieceCount:    result.Features.ComboPieceCount,
			EarlyGameScore:     result.Features.EarlyGameScore,
			MidGameScore:       result.Features.MidGameScore,
			LateGameScore:      result.Features.LateGameScore,
			CurveBalanceScore:  result.Features.CurveBalanceScore,
		},
		FunctionalRoleSummary: toFunctionalRoleSummaryResponses(result.FunctionalRoleSummary),
		Strengths:             result.Strengths,
		Weaknesses:            result.Weaknesses,
		StructuralTags:        result.StructuralTags,
		StructuralTagDetails:  toStructuralTagDetailResponses(result.StructuralTagDetails),
		PackageDetails:        toPackageDetailResponses(result.PackageDetails),
		SuggestedAdds:         result.SuggestedAdds,
		SuggestedCuts:         result.SuggestedCuts,
	}
}

func toFunctionalRoleSummaryResponses(items []analysis.FunctionalRoleSummaryItem) []functionalRoleSummaryResponse {
	if len(items) == 0 {
		return nil
	}
	out := make([]functionalRoleSummaryResponse, 0, len(items))
	for _, item := range items {
		out = append(out, functionalRoleSummaryResponse{
			Role:        item.Role,
			Label:       item.Label,
			Count:       item.Count,
			Explanation: item.Explanation,
		})
	}
	return out
}

func toStructuralTagDetailResponses(items []analysis.StructuralTagDetail) []structuralTagDetailResponse {
	if len(items) == 0 {
		return nil
	}

	out := make([]structuralTagDetailResponse, 0, len(items))
	for _, item := range items {
		out = append(out, structuralTagDetailResponse{
			Tag:         item.Tag,
			Title:       item.Title,
			Explanation: item.Explanation,
		})
	}
	return out
}

func toPackageDetailResponses(items []analysis.PackageDetail) []packageDetailResponse {
	if len(items) == 0 {
		return nil
	}

	out := make([]packageDetailResponse, 0, len(items))
	for _, item := range items {
		out = append(out, packageDetailResponse{
			Package:     item.Package,
			Parent:      item.Parent,
			Label:       item.Label,
			Status:      item.Status,
			Slots:       item.Slots,
			TargetMin:   item.TargetMin,
			TargetMax:   item.TargetMax,
			Explanation: item.Explanation,
		})
	}
	return out
}

func toReportDetailResponse(item reportpkg.ReportDetail) reportDetailResponse {
	out := reportDetailResponse{
		ID:                item.ID,
		DeckID:            item.DeckID,
		BasedOnSnapshotID: item.BasedOnSnapshotID,
		ReportType:        item.ReportType,
		CreatedAt:         item.CreatedAt,
		Result: reportGenerateResponse{
			ReportID:    item.Result.ReportID,
			Report:      item.Result.Report,
			Model:       item.Result.Model,
			GeneratedAt: item.Result.GeneratedAt,
			Analysis:    toAnalyzeDeckResponse(item.Result.Analysis),
			Structured:  toStructuredReportResponse(item.Result.Structured),
		},
	}
	out.Compare = optionalCompareDeckResponse(item.Result.Compare)
	return out
}

func toCompareDeckResponse(result comparepkg.Result) compareDeckResponse {
	out := compareDeckResponse{
		SnapshotID:          result.SnapshotID,
		PatchVersion:        result.PatchVersion,
		Format:              result.Format,
		MergedSummary:       result.MergedSummary,
		MergedSuggestedAdds: result.MergedSuggestedAdds,
		MergedSuggestedCuts: result.MergedSuggestedCuts,
		MergedGuidance:      toCompareGuidanceResponse(result.MergedGuidance),
		Candidates:          make([]compareCandidateResponse, 0, len(result.Candidates)),
	}
	for _, candidate := range result.Candidates {
		out.Candidates = append(out.Candidates, compareCandidateResponse{
			DeckID:     candidate.DeckID,
			Name:       candidate.Name,
			Class:      candidate.Class,
			Archetype:  candidate.Archetype,
			Similarity: candidate.Similarity,
			Breakdown: compareSimilarityResponse{
				Total:    candidate.Breakdown.Total,
				Overlap:  candidate.Breakdown.Overlap,
				Curve:    candidate.Breakdown.Curve,
				CardType: candidate.Breakdown.CardType,
			},
			Summary:          candidate.Summary,
			Winrate:          candidate.Winrate,
			Playrate:         candidate.Playrate,
			SampleSize:       candidate.SampleSize,
			Tier:             candidate.Tier,
			SharedCards:      toCompareCardDiffResponses(candidate.SharedCards),
			MissingFromInput: toCompareCardDiffResponses(candidate.MissingFromInput),
			MissingFromMeta:  toCompareCardDiffResponses(candidate.MissingFromMeta),
			SuggestedAdds:    candidate.SuggestedAdds,
			SuggestedCuts:    candidate.SuggestedCuts,
		})
	}
	return out
}

func toCompareGuidanceResponse(guidance comparepkg.StructuredGuidance) *compareGuidanceResponse {
	if len(guidance.Summary) == 0 && len(guidance.Adds) == 0 && len(guidance.Cuts) == 0 {
		return nil
	}
	return &compareGuidanceResponse{
		Summary: toCompareRecommendationResponses(guidance.Summary),
		Adds:    toCompareRecommendationResponses(guidance.Adds),
		Cuts:    toCompareRecommendationResponses(guidance.Cuts),
	}
}

func toCompareRecommendationResponses(items []comparepkg.Recommendation) []compareRecommendationResponse {
	if len(items) == 0 {
		return nil
	}
	out := make([]compareRecommendationResponse, 0, len(items))
	for _, item := range items {
		out = append(out, compareRecommendationResponse{
			Key:        item.Key,
			Kind:       item.Kind,
			Package:    item.Package,
			Source:     item.Source,
			Message:    item.Message,
			Confidence: item.Confidence,
			Support:    toCompareRecommendationSupportResponses(item.Support),
		})
	}
	return out
}

func toCompareRecommendationSupportResponses(items []comparepkg.RecommendationSupport) []compareRecommendationSupportResponse {
	if len(items) == 0 {
		return nil
	}
	out := make([]compareRecommendationSupportResponse, 0, len(items))
	for _, item := range items {
		out = append(out, compareRecommendationSupportResponse{
			Source:          item.Source,
			CandidateDeckID: item.CandidateDeckID,
			CandidateName:   item.CandidateName,
			CandidateRank:   item.CandidateRank,
			Weight:          item.Weight,
			Evidence:        item.Evidence,
		})
	}
	return out
}

func optionalCompareDeckResponse(result *comparepkg.Result) *compareDeckResponse {
	if result == nil {
		return nil
	}
	compareResponse := toCompareDeckResponse(*result)
	return &compareResponse
}

func toStructuredReportResponse(report *reportpkg.StructuredReport) *structuredReportResponse {
	if report == nil {
		return nil
	}
	return &structuredReportResponse{
		DeckIdentity:             report.DeckIdentity,
		WhatTheDeckIsDoingWell:   report.WhatTheDeckIsDoingWell,
		MainRisks:                report.MainRisks,
		PracticalNextAdjustments: report.PracticalNextAdjustments,
	}
}

func jobsCollectionHandler(svc JobsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "jobs service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		list, err := svc.List(r.Context())
		if err != nil {
			http.Error(w, "failed to list jobs", http.StatusInternalServerError)
			return
		}

		out := make([]jobResponse, 0, len(list))
		for _, job := range list {
			out = append(out, toJobResponse(job))
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func jobItemHandler(svc JobsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "jobs service unavailable", http.StatusServiceUnavailable)
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
		if trimmed == "" {
			http.NotFound(w, r)
			return
		}

		parts := strings.Split(trimmed, "/")
		if len(parts) == 1 {
			handleJobDetail(w, r, svc, parts[0])
			return
		}

		if len(parts) == 2 && parts[1] == "run" {
			handleJobRun(w, r, svc, parts[0])
			return
		}

		if len(parts) == 2 && parts[1] == "history" {
			handleJobHistory(w, r, svc, parts[0])
			return
		}

		http.NotFound(w, r)
	}
}

func handleJobDetail(w http.ResponseWriter, r *http.Request, svc JobsService, key string) {
	if key == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		job, err := svc.Get(r.Context(), key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusOK, toJobResponse(job))
	case http.MethodPut:
		var payload updateJobRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json payload", http.StatusBadRequest)
			return
		}

		job, err := svc.Update(r.Context(), jobs.UpdateInput{
			Key:      key,
			CronExpr: payload.CronExpr,
			Enabled:  payload.Enabled,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusOK, toJobResponse(job))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleJobRun(w http.ResponseWriter, r *http.Request, svc JobsService, key string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := svc.RunNow(r.Context(), key); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func handleJobHistory(w http.ResponseWriter, r *http.Request, svc JobsService, key string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	history, err := svc.History(r.Context(), key, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	out := make([]jobExecutionResponse, 0, len(history))
	for _, item := range history {
		out = append(out, jobExecutionResponse{
			ID:              item.ID,
			JobKey:          item.JobKey,
			Status:          item.Status,
			StartedAt:       item.StartedAt,
			FinishedAt:      item.FinishedAt,
			RecordsAffected: item.RecordsAffected,
			ErrorMessage:    item.ErrorMessage,
		})
	}

	writeJSON(w, http.StatusOK, out)
}

func toJobResponse(job jobs.Job) jobResponse {
	return jobResponse{
		Key:       job.Key,
		CronExpr:  job.CronExpr,
		Enabled:   job.Enabled,
		LastRunAt: job.LastRunAt,
		NextRunAt: job.NextRunAt,
	}
}

func latestMetaSnapshotHandler(svc MetaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "meta service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		format := r.URL.Query().Get("format")
		snapshot, err := svc.GetLatestSnapshot(r.Context(), format)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				http.Error(w, "meta snapshot not found", http.StatusNotFound)
				return
			}

			http.Error(w, "failed to get latest meta snapshot", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, metaSnapshotResponse{
			ID:           snapshot.ID,
			Source:       snapshot.Source,
			PatchVersion: snapshot.PatchVersion,
			Format:       snapshot.Format,
			RankBracket:  snapshot.RankBracket,
			Region:       snapshot.Region,
			FetchedAt:    snapshot.FetchedAt,
		})
	}
}

func metaCollectionHandler(svc MetaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "meta service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		format := r.URL.Query().Get("format")
		limit := 20
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsed
		}

		snapshots, err := svc.ListSnapshots(r.Context(), format, limit)
		if err != nil {
			http.Error(w, "failed to list meta snapshots", http.StatusInternalServerError)
			return
		}

		out := make([]metaSnapshotResponse, 0, len(snapshots))
		for _, snapshot := range snapshots {
			out = append(out, metaSnapshotResponse{
				ID:           snapshot.ID,
				Source:       snapshot.Source,
				PatchVersion: snapshot.PatchVersion,
				Format:       snapshot.Format,
				RankBracket:  snapshot.RankBracket,
				Region:       snapshot.Region,
				FetchedAt:    snapshot.FetchedAt,
			})
		}

		writeJSON(w, http.StatusOK, out)
	}
}

func metaItemHandler(svc MetaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			http.Error(w, "meta service unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/meta/")
		if id == "" || strings.Contains(id, "/") || id == "latest" {
			http.NotFound(w, r)
			return
		}

		snapshot, err := svc.GetSnapshotByID(r.Context(), id)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				http.Error(w, "meta snapshot not found", http.StatusNotFound)
				return
			}

			http.Error(w, "failed to get meta snapshot", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, metaSnapshotDetailResponse{
			ID:           snapshot.ID,
			Source:       snapshot.Source,
			PatchVersion: snapshot.PatchVersion,
			Format:       snapshot.Format,
			RankBracket:  snapshot.RankBracket,
			Region:       snapshot.Region,
			FetchedAt:    snapshot.FetchedAt,
			RawPayload:   snapshot.RawPayload,
		})
	}
}
