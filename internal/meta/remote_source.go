package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RemoteSource struct {
	url    string
	client *http.Client
	opts   RemoteSourceOptions
}

type RemoteSourceOptions struct {
	BearerToken string
	HeaderName  string
	HeaderValue string
}

type remoteSnapshotPayload struct {
	Source       string  `json:"source"`
	PatchVersion string  `json:"patch_version"`
	Format       string  `json:"format"`
	RankBracket  *string `json:"rank_bracket"`
	Region       *string `json:"region"`
	FetchedAt    string  `json:"fetched_at"`
}

func NewRemoteSource(url string) RemoteSource {
	return NewRemoteSourceWithOptions(url, RemoteSourceOptions{})
}

func NewRemoteSourceWithOptions(url string, opts RemoteSourceOptions) RemoteSource {
	return RemoteSource{
		url: url,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		opts: opts,
	}
}

func (s RemoteSource) FetchSnapshot(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("build meta request: %w", err)
	}
	if s.opts.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.opts.BearerToken)
	}
	if s.opts.HeaderName != "" && s.opts.HeaderValue != "" {
		req.Header.Set(s.opts.HeaderName, s.opts.HeaderValue)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("fetch remote meta snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FetchResult{}, fmt.Errorf("fetch remote meta snapshot: unexpected status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("read remote meta snapshot: %w", err)
	}

	var payload remoteSnapshotPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return FetchResult{}, fmt.Errorf("decode remote meta snapshot: %w", err)
	}

	fetchedAt := time.Now().UTC()
	if payload.FetchedAt != "" {
		parsed, err := time.Parse(time.RFC3339, payload.FetchedAt)
		if err != nil {
			return FetchResult{}, fmt.Errorf("parse remote meta fetched_at: %w", err)
		}
		fetchedAt = parsed
	}

	source := payload.Source
	if source == "" {
		source = "remote"
	}

	return FetchResult{
		Source:       source,
		PatchVersion: payload.PatchVersion,
		Format:       payload.Format,
		RankBracket:  payload.RankBracket,
		Region:       payload.Region,
		FetchedAt:    fetchedAt,
		RawPayload:   string(raw),
	}, nil
}
