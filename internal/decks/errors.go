package decks

import "errors"

type ErrorCode string

const (
	ErrCodeInvalidDeckCode ErrorCode = "invalid_deck_code"
	ErrCodeHeroNotFound    ErrorCode = "hero_not_found"
	ErrCodeCardNotFound    ErrorCode = "card_not_found"
	ErrCodeLookupFailed    ErrorCode = "lookup_failed"
)

type ParseError struct {
	Code    ErrorCode
	Message string
}

func NewParseError(code ErrorCode, message string) *ParseError {
	return &ParseError{
		Code:    code,
		Message: message,
	}
}

func (e *ParseError) Error() string {
	return e.Message
}

func AsParseError(err error) (*ParseError, bool) {
	var parseErr *ParseError
	if errors.As(err, &parseErr) {
		return parseErr, true
	}

	return nil, false
}
