package order

import "context"

type Repository interface {
	FindByID(ctx context.Context, id int) (*Order, error)
	FindByUserID(ctx context.Context, userID int) ([]*Order, error)
	Save(ctx context.Context, order *Order) error
}
