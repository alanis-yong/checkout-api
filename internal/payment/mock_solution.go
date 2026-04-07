//go:build ignore

package payment

import (
	"context"
	"fmt"
	"time"
)

type Mock struct{}

func (m Mock) Charge(_ context.Context, amount int) (string, error) {
	if amount <= 0 || amount >= 1_000_000 {
		return "", ErrChargeFailed
	}
	return fmt.Sprintf("txn_%d", time.Now().UnixNano()), nil
}
