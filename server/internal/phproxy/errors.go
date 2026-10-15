package phproxy

import "errors"

var (
	ErrInvalid             = errors.New("invalid ph-proxy enrichment request")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with another request")
	ErrPendingCapacity     = errors.New("pending enrichment capacity reached")
)

// ValidationError contains only fixed codes, allowlisted fields and a batch index.
// Neither Error nor its HTTP representation includes identities or payloads.
type ValidationError struct {
	Code        string `json:"code"`
	Field       string `json:"field,omitempty"`
	PlayerIndex *int   `json:"player_index,omitempty"`
}

func (e *ValidationError) Error() string { return ErrInvalid.Error() }
func (e *ValidationError) Unwrap() error { return ErrInvalid }
func validationError(code, field string, index int) *ValidationError {
	e := &ValidationError{Code: code, Field: field}
	if index >= 0 {
		e.PlayerIndex = &index
	}
	return e
}
func withPlayerIndex(err error, index int) error {
	var detail *ValidationError
	if errors.As(err, &detail) {
		copy := *detail
		copy.PlayerIndex = &index
		return &copy
	}
	return validationError("invalid_player", "", index)
}
