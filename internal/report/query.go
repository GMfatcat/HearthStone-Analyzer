package report

import (
	"context"
	"time"

	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

type StoredReport struct {
	ID                string    `json:"id"`
	DeckID            string    `json:"deck_id"`
	BasedOnSnapshotID *string   `json:"based_on_snapshot_id,omitempty"`
	ReportType        string    `json:"report_type"`
	ReportJSON        string    `json:"report_json"`
	ReportText        *string   `json:"report_text,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type QueryService struct {
	repo *sqliteStore.AnalysisReportsRepository
}

func NewQueryService(repo *sqliteStore.AnalysisReportsRepository) *QueryService {
	return &QueryService{repo: repo}
}

func (s *QueryService) ListReports(ctx context.Context, limit int) ([]StoredReport, error) {
	items, err := s.repo.ListRecent(ctx, limit)
	if err != nil {
		return nil, err
	}

	out := make([]StoredReport, 0, len(items))
	for _, item := range items {
		out = append(out, StoredReport{
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
