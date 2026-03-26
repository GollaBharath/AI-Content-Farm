package trends

import (
	"context"
	"fmt"
)

type Provider interface {
	TopTopics(ctx context.Context, category, countryCode string, limit int) ([]string, error)
}

type StaticProvider struct{}

func NewStaticProvider() *StaticProvider {
	return &StaticProvider{}
}

func (p *StaticProvider) TopTopics(_ context.Context, category, countryCode string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be > 0")
	}

	seed := []string{
		"breaking tech updates",
		"unexpected history facts",
		"sports moments explained",
		"finance myth busting",
		"daily science in 60 seconds",
	}

	if limit > len(seed) {
		limit = len(seed)
	}

	_ = category
	_ = countryCode
	return seed[:limit], nil
}
