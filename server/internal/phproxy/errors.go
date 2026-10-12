package phproxy

import "errors"

var (
	ErrInvalid             = errors.New("invalid ph-proxy enrichment request")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with another request")
	ErrPendingCapacity     = errors.New("pending enrichment capacity reached")
)
