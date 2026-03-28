package deckanalysis

import (
	"context"

	"hearthstone-analyzer/internal/analysis"
	"hearthstone-analyzer/internal/decks"
)

type Parser interface {
	Parse(ctx context.Context, deckCode string) (decks.ParseResult, error)
}

type Analyzer interface {
	Analyze(parsed decks.ParseResult) analysis.Result
}

type Service struct {
	parser   Parser
	analyzer Analyzer
}

func NewService(parser Parser, analyzer Analyzer) *Service {
	return &Service{
		parser:   parser,
		analyzer: analyzer,
	}
}

func (s *Service) AnalyzeDeck(ctx context.Context, deckCode string) (analysis.Result, error) {
	parsed, err := s.parser.Parse(ctx, deckCode)
	if err != nil {
		return analysis.Result{}, err
	}

	return s.analyzer.Analyze(parsed), nil
}
