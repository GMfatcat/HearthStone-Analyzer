package meta

import "context"

type FixtureSource struct {
	result FetchResult
}

func NewFixtureSource(result FetchResult) FixtureSource {
	return FixtureSource{result: result}
}

func (s FixtureSource) FetchSnapshot(ctx context.Context) (FetchResult, error) {
	return s.result, nil
}
