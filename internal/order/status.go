package order

type OrderStatus string

const (
	StatusPending	OrderStatus = "pending"
	StatusPaid		OrderStatus = "paid"
	StatusFailed	OrderStatus = "failed"
)

