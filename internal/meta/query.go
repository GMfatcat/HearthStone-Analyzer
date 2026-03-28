package meta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

type Snapshot struct {
	ID           string
	Source       string
	PatchVersion string
	Format       string
	RankBracket  *string
	Region       *string
	FetchedAt    time.Time
}

type SnapshotDetail struct {
	Snapshot
	RawPayload string
}

type QueryService struct {
	repo *sqliteStore.MetaSnapshotsRepository
}

func NewQueryService(repo *sqliteStore.MetaSnapshotsRepository) *QueryService {
	return &QueryService{repo: repo}
}

func (s *QueryService) GetLatestSnapshot(ctx context.Context, format string) (Snapshot, error) {
	if format == "" {
		format = "standard"
	}

	item, err := s.repo.GetLatestByFormat(ctx, format)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Snapshot{}, fmt.Errorf("meta snapshot not found: %w", err)
		}
		return Snapshot{}, err
	}

	return Snapshot{
		ID:           item.ID,
		Source:       item.Source,
		PatchVersion: item.PatchVersion,
		Format:       item.Format,
		RankBracket:  item.RankBracket,
		Region:       item.Region,
		FetchedAt:    item.FetchedAt,
	}, nil
}

func (s *QueryService) ListSnapshots(ctx context.Context, format string, limit int) ([]Snapshot, error) {
	if format == "" {
		format = "standard"
	}

	items, err := s.repo.ListByFormat(ctx, format, limit)
	if err != nil {
		return nil, err
	}

	out := make([]Snapshot, 0, len(items))
	for _, item := range items {
		out = append(out, Snapshot{
			ID:           item.ID,
			Source:       item.Source,
			PatchVersion: item.PatchVersion,
			Format:       item.Format,
			RankBracket:  item.RankBracket,
			Region:       item.Region,
			FetchedAt:    item.FetchedAt,
		})
	}

	return out, nil
}

func (s *QueryService) GetSnapshotByID(ctx context.Context, id string) (SnapshotDetail, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SnapshotDetail{}, fmt.Errorf("meta snapshot not found: %w", err)
		}
		return SnapshotDetail{}, err
	}

	return SnapshotDetail{
		Snapshot: Snapshot{
			ID:           item.ID,
			Source:       item.Source,
			PatchVersion: item.PatchVersion,
			Format:       item.Format,
			RankBracket:  item.RankBracket,
			Region:       item.Region,
			FetchedAt:    item.FetchedAt,
		},
		RawPayload: item.RawPayload,
	}, nil
}
