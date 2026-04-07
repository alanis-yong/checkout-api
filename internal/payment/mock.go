package payment

import (
	"context"
	"errors"
)

// Mock satisfies Gateway for tests and live demos.
// It replaces mockProcessPayment() which was embedded in the handler.
type Mock struct{}

func (m Mock) Charge(_ context.Context, amount int) (string, error) {
	// TODO: return ErrChargeFailed if amount <= 0 or amount >= 1_000_000
	// TODO: return a unique transaction ID string
	// hint: fmt.Sprintf("txn_%d", time.Now().UnixNano())
	return "", errors.New("not implemented")
}
