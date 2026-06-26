package payment

import "errors"

var (
	ErrNotFound            = errors.New("payment: transaction not found")
	ErrInvalidTransition   = errors.New("payment: invalid status transition")
	ErrIdempotencyConflict = errors.New("payment: idempotency key reused with different payload")
	ErrRefundExceedsAmount = errors.New("payment: refund exceeds refundable amount")
	ErrNotRefundable       = errors.New("payment: transaction is not in a refundable state")
	ErrUnsupportedCurrency = errors.New("payment: unsupported currency")
	ErrInvalidAmount       = errors.New("payment: amount must be greater than zero")
	ErrDuplicateReference  = errors.New("payment: external_reference already used for this app")
	ErrInvalidSignature    = errors.New("payment: webhook signature verification failed")
	ErrRateLimited         = errors.New("payment: too many requests, slow down")
)
