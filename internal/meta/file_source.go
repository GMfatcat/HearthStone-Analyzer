package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type FileSource struct {
	path string
}

type fileSnapshotPayload struct {
	PatchVersion string  `json:"patch_version"`
	Format       string  `json:"format"`
	RankBracket  *string `json:"rank_bracket"`
	Region       *string `json:"region"`
	FetchedAt    string  `json:"fetched_at"`
}

func NewFileSource(path string) FileSource {
	return FileSource{path: path}
}

func (s FileSource) FetchSnapshot(ctx context.Context) (FetchResult, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return FetchResult{}, fmt.Errorf("read meta file: %w", err)
	}

	var payload fileSnapshotPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return FetchResult{}, fmt.Errorf("decode meta file: %w", err)
	}

	fetchedAt := time.Now().UTC()
	if payload.FetchedAt != "" {
		parsed, err := time.Parse(time.RFC3339, payload.FetchedAt)
		if err != nil {
			return FetchResult{}, fmt.Errorf("parse meta file fetched_at: %w", err)
		}
		fetchedAt = parsed
	}

	return FetchResult{
		Source:       "file",
		PatchVersion: payload.PatchVersion,
		Format:       payload.Format,
		RankBracket:  payload.RankBracket,
		Region:       payload.Region,
		FetchedAt:    fetchedAt,
		RawPayload:   string(raw),
	}, nil
}
