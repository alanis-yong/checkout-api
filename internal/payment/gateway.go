package payment

import (
	"context"
	"errors"
)

var ErrChargeFailed = errors.New("payment charge failed")

type Gateway interface {
	Charge(ctx context.Context, amountCents int) (transactionID string, err error)
}
