package cart

import "errors"

var (
	ErrCartNotFound  = errors.New("cart not found")
	ErrPaymentFailed = errors.New("payment failed")
)
