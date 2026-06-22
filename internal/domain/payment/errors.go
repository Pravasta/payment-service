package payment

import "errors"

var (
	ErrNotFound            = errors.New("payment: transaction not found")
	ErrInvalidTransition   = errors.New("payment: invalid status transition")
	ErrIdempotencyConflict = errors.New("payment: idempotency key reused with different payload")
	ErrRefundExceedsAmount = errors.New("payment: refund exceeds refundable amount")
	ErrNotRefundable       = errors.New("payment: transaction is not in a refundable state")
	ErrUnsupportedCurrency = errors.New("payment: unsupported currency")
)
