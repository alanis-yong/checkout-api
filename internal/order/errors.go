package order

import "errors"

var (
	ErrNotFound          = errors.New("order not found")
	ErrEmptyCart         = errors.New("cart is empty")
	ErrOrderNotEditable  = errors.New("order: cannot modify a non-pending order")
	ErrInvalidQuantity   = errors.New("order: quantity must be greater than zero")
	ErrInvalidPrice      = errors.New("order: price must be greater than zero")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrPaymentFailed     = errors.New("payment failed")
)
