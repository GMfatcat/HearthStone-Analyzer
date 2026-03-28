package meta

import (
	"context"
	"errors"
)

var ErrSourceUnavailable = errors.New("meta source unavailable")

type UnavailableSource struct{}

func (UnavailableSource) FetchSnapshot(ctx context.Context) (FetchResult, error) {
	return FetchResult{}, ErrSourceUnavailable
}
